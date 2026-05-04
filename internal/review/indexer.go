package review

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wesen/codebase-browser/internal/browser"
	"github.com/wesen/codebase-browser/internal/docs"
	"github.com/wesen/codebase-browser/internal/gitutil"
	"github.com/wesen/codebase-browser/internal/history"
)

// IndexOptions controls the review indexing process.
type IndexOptions struct {
	RepoRoot     string
	CommitRange  string
	DocsPaths    []string
	Patterns     []string
	IncludeTests bool
	Parallelism  int
	OnProgress   func(phase string, done, total int, detail string)
	SkipDocs     bool
	StrictDocs   bool

	// Incremental skips commits that are already present in the database.
	// The store must be opened with Open (not Create) so existing data is
	// preserved. New commits are appended; existing snapshots are not
	// re-extracted.
	Incremental bool

	// DocsOnly skips commit indexing entirely and only re-indexes markdown
	// docs. Useful when you've updated a review doc but the commits haven't
	// changed. Requires an existing database with commits already indexed.
	DocsOnly bool
}

// IndexResult describes what the indexer did.
type IndexResult struct {
	CommitsIndexed  int
	DocsIndexed     int
	SnippetsIndexed int
	Duration        time.Duration
	Errors          []IndexError
}

// IndexError records a failure for a specific phase.
type IndexError struct {
	Phase  string
	Detail string
	Err    error
}

// IndexReview builds a review database from commits and markdown docs.
func IndexReview(ctx context.Context, store *Store, opts IndexOptions) (*IndexResult, error) {
	start := time.Now()
	result := &IndexResult{}

	if !opts.DocsOnly {
		// ── Phase 1: resolve commit range ──
		commits, err := gitutil.LogCommits(ctx, opts.RepoRoot, opts.CommitRange)
		if err != nil {
			return nil, fmt.Errorf("parse commit range %q: %w", opts.CommitRange, err)
		}

		// ── Phase 2: index commits ──
		// Multi-commit review databases must contain source/symbol/ref snapshots for
		// each commit, not the current checkout repeated N times. The current
		// extractor is filesystem-oriented, so use git worktrees automatically for
		// commit ranges and keep direct indexing only for single-commit snapshots.

		if opts.Incremental {
			if err := assignIncrementalSequences(ctx, store, commits); err != nil {
				return nil, err
			}
		} else {
			assignBatchSequences(commits, 0)
		}

		// Filter out already-indexed commits when running in incremental mode after
		// assigning sequences, so retry batches keep the original per-range IDs.
		toIndex := commits
		skipped := 0
		if opts.Incremental {
			var filtered []gitutil.Commit
			for _, c := range commits {
				present, err := store.History.HasCommit(ctx, c.Hash)
				if err != nil {
					return nil, fmt.Errorf("check commit %s: %w", c.ShortHash, err)
				}
				if present {
					skipped++
					continue
				}
				filtered = append(filtered, c)
			}
			toIndex = filtered
		}

		if len(toIndex) == 0 && len(commits) > 0 {
			fmt.Fprintf(os.Stderr, "All %d commits already indexed (skipping)\n", len(commits))
		} else if skipped > 0 {
			fmt.Fprintf(os.Stderr, "Indexing %d new commits (skipping %d existing)\n", len(toIndex), skipped)
		}

		// Decide from the originally requested range, not the filtered incremental
		// batch. If --incremental filters a multi-commit range down to one missing
		// commit, that remaining commit may not be the current checkout; direct
		// filesystem extraction would index the wrong revision under that hash.
		useWorktrees := len(commits) > 1
		histOpts := history.IndexOptions{
			RepoRoot:     opts.RepoRoot,
			Commits:      toIndex,
			Patterns:     opts.Patterns,
			IncludeTests: opts.IncludeTests,
			Worktrees:    useWorktrees,
			Parallelism:  opts.Parallelism,
			OnProgress: func(done, total int, shortHash, message string) {
				result.CommitsIndexed = done
				if opts.OnProgress != nil {
					opts.OnProgress("commits", done, total, shortHash)
				}
			},
		}

		histResult, err := history.IndexCommits(ctx, store.History, histOpts)
		if err != nil {
			return nil, fmt.Errorf("index commits: %w", err)
		}
		result.CommitsIndexed = histResult.Indexed
		for _, e := range histResult.Errors {
			result.Errors = append(result.Errors, IndexError{
				Phase:  "commit",
				Detail: e.ShortHash,
				Err:    e.Err,
			})
		}
	}

	if opts.SkipDocs {
		result.Duration = time.Since(start)
		return result, nil
	}

	// ── Phase 3: discover markdown files ──
	docPaths, err := discoverDocs(opts.DocsPaths)
	if err != nil {
		return nil, fmt.Errorf("discover docs: %w", err)
	}

	// Load the latest snapshot for snippet resolution.
	loaded, err := LoadLatestSnapshot(ctx, store)
	if err != nil {
		return nil, fmt.Errorf("load latest snapshot: %w", err)
	}
	latestHash, err := LatestCommitHash(ctx, store.DB())
	if err != nil {
		return nil, fmt.Errorf("load latest commit hash: %w", err)
	}

	// Use indexed file contents for snippet slicing. Symbol ranges and source
	// bytes must come from the same snapshot; the live checkout may have moved.
	sourceFS := NewSnapshotFS(ctx, store.DB(), latestHash)

	// ── Phase 4: index each markdown file ──
	for i, path := range docPaths {
		snippetCount, err := indexDoc(ctx, store, path, loaded, sourceFS, opts.StrictDocs)
		if err != nil {
			result.Errors = append(result.Errors, IndexError{
				Phase:  "doc",
				Detail: path,
				Err:    err,
			})
			continue
		}
		result.DocsIndexed++
		result.SnippetsIndexed += snippetCount
		if opts.OnProgress != nil {
			opts.OnProgress("docs", i+1, len(docPaths), filepath.Base(path))
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// discoverDocs resolves a list of file/directory paths into a flat list of .md files.
func discoverDocs(paths []string) ([]string, error) {
	var result []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			entries, err := os.ReadDir(p)
			if err != nil {
				return nil, err
			}
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".md") {
					result = append(result, filepath.Join(p, e.Name()))
				}
			}
		} else {
			result = append(result, p)
		}
	}
	return result, nil
}

func assignIncrementalSequences(ctx context.Context, store *Store, commits []gitutil.Commit) error {
	baseSequence, err := inferBatchBaseSequence(ctx, store, commits)
	if err != nil {
		return err
	}
	assignBatchSequences(commits, baseSequence)
	return nil
}

func inferBatchBaseSequence(ctx context.Context, store *Store, commits []gitutil.Commit) (int, error) {
	for i, commit := range commits {
		row, err := store.History.GetCommit(ctx, commit.Hash)
		if err != nil {
			return 0, fmt.Errorf("load existing commit sequence %s: %w", commit.ShortHash, err)
		}
		if row != nil {
			return row.Sequence - (len(commits) - i), nil
		}
	}
	baseSequence, err := store.History.MaxSequence(ctx)
	if err != nil {
		return 0, fmt.Errorf("load current max commit sequence: %w", err)
	}
	return baseSequence, nil
}

func assignBatchSequences(commits []gitutil.Commit, baseSequence int) {
	for i := range commits {
		// git log returns newest-first. Assign the newest commit in this batch the
		// highest sequence. Incremental runs either infer the original batch base
		// from existing commits in the same range or append above the DB max.
		commits[i].Sequence = baseSequence + len(commits) - i
	}
}

// indexDoc reads a markdown file, renders it to resolve snippets, and stores both.
// Each doc is wrapped in a transaction so that stale snippet cleanup and
// re-insert are atomic.
func indexDoc(ctx context.Context, store *Store, path string, loaded *browser.Loaded, sourceFS fs.FS, strict bool) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	slug := strings.TrimSuffix(filepath.Base(path), ".md")

	page, err := docs.Render(slug, data, loaded, sourceFS)
	if err != nil {
		return 0, fmt.Errorf("render doc: %w", err)
	}
	if strict && len(page.Errors) > 0 {
		return 0, fmt.Errorf("render doc %s: %d directive error(s): %s", slug, len(page.Errors), strings.Join(page.Errors, "; "))
	}
	if strict {
		if errs := ValidatePageCommitRefs(ctx, store.DB(), page); len(errs) > 0 {
			messages := make([]string, len(errs))
			for i, err := range errs {
				messages[i] = err.Error()
			}
			return 0, fmt.Errorf("render doc %s: %d commit-ref error(s): %s", slug, len(errs), strings.Join(messages, "; "))
		}
	}

	frontmatter := "{}"
	// TODO: parse YAML frontmatter from data if present

	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var docID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO review_docs (slug, title, path, content, frontmatter_json, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			title = excluded.title,
			path = excluded.path,
			content = excluded.content,
			frontmatter_json = excluded.frontmatter_json,
			indexed_at = excluded.indexed_at
		RETURNING id
	`, slug, page.Title, path, string(data), frontmatter, time.Now().Unix()).Scan(&docID); err != nil {
		return 0, fmt.Errorf("upsert review doc: %w", err)
	}

	// Delete stale snippets from a previous index of this doc.
	delRes, err := tx.ExecContext(ctx, `DELETE FROM review_doc_snippets WHERE doc_id = ?`, docID)
	if err != nil {
		return 0, fmt.Errorf("delete stale snippets: %w", err)
	}
	_, _ = delRes.RowsAffected()

	for _, snip := range page.Snippets {
		paramsJSON, _ := json.Marshal(snip.Params)
		_, err := tx.ExecContext(ctx, `
			INSERT INTO review_doc_snippets
				(doc_id, stub_id, directive, symbol_id, file_path, kind, language,
				 text, params_json, start_line, end_line, commit_hash)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, docID, snip.StubID, snip.Directive, snip.SymbolID, snip.FilePath,
			snip.Kind, snip.Language, snip.Text, string(paramsJSON),
			snip.StartLine, snip.EndLine, snip.CommitHash)
		if err != nil {
			return 0, fmt.Errorf("insert snippet: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit doc: %w", err)
	}
	return len(page.Snippets), nil
}
