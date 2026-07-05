package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wesen/codebase-browser/internal/browser"
	"github.com/wesen/codebase-browser/internal/gitutil"
	"github.com/wesen/codebase-browser/internal/history"
	"github.com/wesen/codebase-browser/internal/indexer"
)

func TestIndexDocUpdatesExistingDocAndSnippetCount(t *testing.T) {
	ctx := context.Background()
	store, sourceRoot, loaded := setupDocIndexTest(t, ctx)

	docPath := writeReviewDoc(t, sourceRoot, "review.md", "Original")
	snippets, err := indexDoc(ctx, store, docPath, loaded, os.DirFS(sourceRoot), false)
	if err != nil {
		t.Fatalf("index doc first time: %v", err)
	}
	if snippets != 1 {
		t.Fatalf("expected 1 snippet, got %d", snippets)
	}

	docPath = writeReviewDoc(t, sourceRoot, "review.md", "Edited")
	snippets, err = indexDoc(ctx, store, docPath, loaded, os.DirFS(sourceRoot), false)
	if err != nil {
		t.Fatalf("index doc second time: %v", err)
	}
	if snippets != 1 {
		t.Fatalf("expected 1 snippet after update, got %d", snippets)
	}

	var docCount, snippetCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM review_docs WHERE slug = 'review'`).Scan(&docCount); err != nil {
		t.Fatalf("count docs: %v", err)
	}
	if docCount != 1 {
		t.Fatalf("expected one review doc row, got %d", docCount)
	}
	if err := store.DB().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM review_doc_snippets s
		JOIN review_docs d ON d.id = s.doc_id
		WHERE d.slug = 'review'
	`).Scan(&snippetCount); err != nil {
		t.Fatalf("count snippets: %v", err)
	}
	if snippetCount != 1 {
		t.Fatalf("expected stale snippets to be replaced, got %d", snippetCount)
	}

	var content string
	if err := store.DB().QueryRowContext(ctx, `SELECT content FROM review_docs WHERE slug = 'review'`).Scan(&content); err != nil {
		t.Fatalf("read content: %v", err)
	}
	if !strings.Contains(content, "Edited") {
		t.Fatalf("expected updated content, got %q", content)
	}
}

func TestIndexReviewDocsOnlyUsesIndexedContentNotLiveCheckout(t *testing.T) {
	ctx := context.Background()
	store, sourceRoot, _ := setupDocIndexTest(t, ctx)

	// Move the live checkout forward after indexing. Docs-only rendering should
	// still slice the source bytes stored in file_contents for the indexed commit.
	changed := "package test\n\nfunc Target() {\n\tprintln(\"changed live checkout\")\n}\n"
	if err := os.WriteFile(filepath.Join(sourceRoot, "target.go"), []byte(changed), 0o644); err != nil {
		t.Fatalf("change live source: %v", err)
	}
	docPath := writeReviewDoc(t, sourceRoot, "review.md", "DB backed")

	result, err := IndexReview(ctx, store, IndexOptions{
		RepoRoot:  sourceRoot,
		DocsPaths: []string{docPath},
		DocsOnly:  true,
	})
	if err != nil {
		t.Fatalf("docs-only index: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected indexing errors: %+v", result.Errors)
	}

	var text string
	if err := store.DB().QueryRowContext(ctx, `SELECT text FROM review_doc_snippets LIMIT 1`).Scan(&text); err != nil {
		t.Fatalf("read snippet: %v", err)
	}
	if strings.Contains(text, "changed live checkout") {
		t.Fatalf("snippet used live checkout content: %q", text)
	}
	if !strings.Contains(text, "hello") {
		t.Fatalf("snippet did not use indexed content: %q", text)
	}
}

func TestIndexReviewDocsOnlyIndexesMultipleDocsAndCountsSnippets(t *testing.T) {
	ctx := context.Background()
	store, sourceRoot, _ := setupDocIndexTest(t, ctx)

	docA := writeReviewDoc(t, sourceRoot, "a.md", "A")
	docB := writeReviewDoc(t, sourceRoot, "b.md", "B")

	result, err := IndexReview(ctx, store, IndexOptions{
		RepoRoot:    sourceRoot,
		DocsPaths:   []string{docA, docB},
		DocsOnly:    true,
		CommitRange: "this-is-not-a-valid-range",
	})
	if err != nil {
		t.Fatalf("docs-only index should not resolve git range: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected indexing errors: %+v", result.Errors)
	}
	if result.CommitsIndexed != 0 {
		t.Fatalf("docs-only indexed commits: %d", result.CommitsIndexed)
	}
	if result.DocsIndexed != 2 {
		t.Fatalf("expected 2 docs, got %d", result.DocsIndexed)
	}
	if result.SnippetsIndexed != 2 {
		t.Fatalf("expected 2 snippets, got %d", result.SnippetsIndexed)
	}
}

func TestAssignBatchSequencesAppendsAboveExistingMax(t *testing.T) {
	commits := []gitutil.Commit{{ShortHash: "newest"}, {ShortHash: "oldest"}}
	assignBatchSequences(commits, 7)
	if commits[0].Sequence != 9 || commits[1].Sequence != 8 {
		t.Fatalf("sequences = [%d, %d], want [9, 8]", commits[0].Sequence, commits[1].Sequence)
	}
}

func TestInferBatchBaseSequenceFromExistingCommitInRange(t *testing.T) {
	ctx := context.Background()
	store, err := Create(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	loadTestSnapshotWithCommit(t, ctx, store, t.TempDir(), gitutil.Commit{
		Hash:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ShortHash:  "bbbbbbb",
		AuthorTime: time.Now(),
		Sequence:   42,
	})
	commits := []gitutil.Commit{
		{Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ShortHash: "aaaaaaa"},
		{Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ShortHash: "bbbbbbb"},
		{Hash: "cccccccccccccccccccccccccccccccccccccccc", ShortHash: "ccccccc"},
	}
	base, err := inferBatchBaseSequence(ctx, store, commits)
	if err != nil {
		t.Fatalf("infer base: %v", err)
	}
	if base != 40 {
		t.Fatalf("base = %d, want 40", base)
	}
	assignBatchSequences(commits, base)
	if commits[0].Sequence != 43 || commits[1].Sequence != 42 || commits[2].Sequence != 41 {
		t.Fatalf("sequences = [%d, %d, %d], want [43, 42, 41]", commits[0].Sequence, commits[1].Sequence, commits[2].Sequence)
	}
}

func TestHasCommits(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	store, err := Create(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	hasCommits, err := store.HasCommits(ctx)
	if err != nil {
		t.Fatalf("has commits empty: %v", err)
	}
	if hasCommits {
		t.Fatal("empty DB should not have commits")
	}

	loadTestSnapshot(t, ctx, store, tmpDir)
	hasCommits, err = store.HasCommits(ctx)
	if err != nil {
		t.Fatalf("has commits after load: %v", err)
	}
	if !hasCommits {
		t.Fatal("expected DB to have commits after loading snapshot")
	}
}

func setupDocIndexTest(t *testing.T, ctx context.Context) (*Store, string, *browser.Loaded) {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := Create(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	loadTestSnapshot(t, ctx, store, tmpDir)
	loaded, err := LoadLatestSnapshot(ctx, store)
	if err != nil {
		t.Fatalf("load latest snapshot: %v", err)
	}
	return store, tmpDir, loaded
}

func loadTestSnapshot(t *testing.T, ctx context.Context, store *Store, sourceRoot string) {
	t.Helper()
	loadTestSnapshotWithCommit(t, ctx, store, sourceRoot, gitutil.Commit{
		Hash:        "abc123def456",
		ShortHash:   "abc123",
		Message:     "test commit",
		AuthorName:  "Test",
		AuthorEmail: "test@example.com",
		AuthorTime:  time.Now(),
	})
}

func loadTestSnapshotWithCommit(t *testing.T, ctx context.Context, store *Store, sourceRoot string, commit gitutil.Commit) {
	t.Helper()
	fileContent := "package test\n\nfunc Target() {\n\tprintln(\"hello\")\n}\n"
	if err := os.WriteFile(filepath.Join(sourceRoot, "target.go"), []byte(fileContent), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	start := strings.Index(fileContent, "func Target")
	end := len(fileContent)
	hash := sha256.Sum256([]byte(fileContent))
	sha := hex.EncodeToString(hash[:])

	idx := &indexer.Index{
		Version: "1",
		Module:  "github.com/test/module",
		Packages: []indexer.Package{{
			ID:         "pkg:github.com/test/module",
			ImportPath: "github.com/test/module",
			Name:       "test",
			FileIDs:    []string{"file:target.go"},
			SymbolIDs:  []string{"sym:github.com/test/module.func.Target"},
		}},
		Files: []indexer.File{{
			ID:        "file:target.go",
			Path:      "target.go",
			PackageID: "pkg:github.com/test/module",
			Size:      len(fileContent),
			LineCount: strings.Count(fileContent, "\n"),
			SHA256:    sha,
			Language:  "go",
		}},
		Symbols: []indexer.Symbol{{
			ID:        "sym:github.com/test/module.func.Target",
			Kind:      "func",
			Name:      "Target",
			PackageID: "pkg:github.com/test/module",
			FileID:    "file:target.go",
			Range: indexer.Range{
				StartLine:   3,
				StartCol:    1,
				EndLine:     5,
				EndCol:      1,
				StartOffset: start,
				EndOffset:   end,
			},
			Signature: "func Target()",
			Language:  "go",
		}},
	}
	if err := store.History.LoadSnapshot(ctx, commit, idx, sourceRoot); err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if err := history.CacheFileContents(ctx, store.History, commit.Hash, sourceRoot); err != nil {
		t.Fatalf("cache file contents: %v", err)
	}
}

func writeReviewDoc(t *testing.T, dir, name, marker string) string {
	t.Helper()
	content := "# " + marker + "\n\n```codebase-snippet sym=sym:github.com/test/module.func.Target\n```\n"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	return path
}
