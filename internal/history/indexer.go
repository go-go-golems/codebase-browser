package history

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wesen/codebase-browser/internal/gitutil"
	"github.com/wesen/codebase-browser/internal/indexer"
)

// IndexOptions controls how per-commit indexing runs.
type IndexOptions struct {
	RepoRoot     string
	Commits      []gitutil.Commit
	Patterns     []string // Go package patterns for extraction
	IncludeTests bool
	Worktrees    bool // if false, indexes in-process without worktrees (for testing)
	Parallelism  int  // max concurrent worktrees (default 1)
	OnProgress   func(done, total int, shortHash, message string)
}

// IndexResult describes what the indexer did.
type IndexResult struct {
	Indexed  int
	Skipped  int
	Failed   int
	Errors   []IndexError
	Duration time.Duration
}

// IndexError records a failure for a specific commit.
type IndexError struct {
	ShortHash string
	Message   string
	Err       error
}

// IndexCommits runs the extraction pipeline for each commit.
// When Worktrees is true, it creates a git worktree for each commit and
// extracts the index from it. When false, it indexes the working directory
// directly (useful for single-commit testing).
func IndexCommits(ctx context.Context, store *Store, opts IndexOptions) (*IndexResult, error) {
	start := time.Now()
	result := &IndexResult{}

	if opts.Worktrees {
		result = indexWithWorktrees(ctx, store, opts)
	} else {
		result = indexDirect(ctx, store, opts)
	}

	result.Duration = time.Since(start)
	return result, nil
}

// indexWithWorktrees creates a worktree per commit and extracts.
func indexWithWorktrees(ctx context.Context, store *Store, opts IndexOptions) *IndexResult {
	result := &IndexResult{}
	var mu sync.Mutex

	workers := opts.Parallelism
	if workers < 1 {
		workers = 1
	}

	recordError := func(commit gitutil.Commit, err error) {
		mu.Lock()
		defer mu.Unlock()
		result.Errors = append(result.Errors, IndexError{
			ShortHash: commit.ShortHash,
			Message:   commit.Message,
			Err:       err,
		})
		result.Failed++
	}

	type commitJob struct {
		index  int
		commit gitutil.Commit
	}

	jobs := make(chan commitJob, len(opts.Commits))
	for i, c := range opts.Commits {
		jobs <- commitJob{index: i, commit: c}
	}
	close(jobs)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				func() {
					commit := job.commit

					// Create worktree.
					wt, err := gitutil.CreateWorktree(ctx, opts.RepoRoot, commit.Hash)
					if err != nil {
						recordError(commit, fmt.Errorf("worktree: %w", err))
						return
					}
					defer func() {
						_ = gitutil.RemoveWorktree(context.Background(), opts.RepoRoot, wt)
					}()

					// Extract index from worktree (CPU-heavy, runs in parallel).
					idx, err := indexer.Extract(indexer.ExtractOptions{
						ModuleRoot:   wt,
						Patterns:     opts.Patterns,
						IncludeTests: opts.IncludeTests,
					})
					if err != nil {
						recordError(commit, fmt.Errorf("extract: %w", err))
						return
					}

					// Load snapshot and cache file contents with serialized SQLite writes.
					mu.Lock()
					defer mu.Unlock()
					if err := store.LoadSnapshot(ctx, commit, idx, wt); err != nil {
						result.Errors = append(result.Errors, IndexError{
							ShortHash: commit.ShortHash,
							Message:   commit.Message,
							Err:       fmt.Errorf("load: %w", err),
						})
						result.Failed++
						return
					}
					if err := CacheFileContents(ctx, store, commit.Hash, wt); err != nil {
						result.Errors = append(result.Errors, IndexError{
							ShortHash: commit.ShortHash,
							Message:   commit.Message,
							Err:       fmt.Errorf("cache file contents: %w", err),
						})
						result.Failed++
						return
					}
					result.Indexed++
					if opts.OnProgress != nil {
						opts.OnProgress(result.Indexed, len(opts.Commits), commit.ShortHash, commit.Message)
					}
				}()
			}
		}()
	}

	wg.Wait()
	return result
}

// indexDirect indexes the working directory for each commit without worktrees.
// This is used for testing and for single-commit indexing where the working
// directory is already at the right commit.
func indexDirect(ctx context.Context, store *Store, opts IndexOptions) *IndexResult {
	result := &IndexResult{}

	for _, commit := range opts.Commits {
		idx, err := indexer.Extract(indexer.ExtractOptions{
			ModuleRoot:   opts.RepoRoot,
			Patterns:     opts.Patterns,
			IncludeTests: opts.IncludeTests,
		})
		if err != nil {
			result.Errors = append(result.Errors, IndexError{
				ShortHash: commit.ShortHash,
				Message:   commit.Message,
				Err:       fmt.Errorf("extract: %w", err),
			})
			result.Failed++
			continue
		}

		if err := store.LoadSnapshot(ctx, commit, idx, opts.RepoRoot); err != nil {
			result.Errors = append(result.Errors, IndexError{
				ShortHash: commit.ShortHash,
				Message:   commit.Message,
				Err:       fmt.Errorf("load: %w", err),
			})
			result.Failed++
			continue
		}
		if err := CacheFileContents(ctx, store, commit.Hash, opts.RepoRoot); err != nil {
			result.Errors = append(result.Errors, IndexError{
				ShortHash: commit.ShortHash,
				Message:   commit.Message,
				Err:       fmt.Errorf("cache file contents: %w", err),
			})
			result.Failed++
			continue
		}

		result.Indexed++
		if opts.OnProgress != nil {
			opts.OnProgress(result.Indexed, len(opts.Commits), commit.ShortHash, commit.Message)
		}
	}

	return result
}
