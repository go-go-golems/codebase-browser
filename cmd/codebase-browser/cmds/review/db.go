package review

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/wesen/codebase-browser/internal/review"
)

func newDBCmd() *cobra.Command {
	var (
		dbPath       string
		repoRoot     string
		commitRange  string
		patterns     []string
		includeTests bool
		parallelism  int
	)

	cmd := &cobra.Command{
		Use:   "db create",
		Short: "Create a review SQLite database from a commit range (no docs)",
		Long: `Create a review database containing per-commit snapshots.

This is the artifact you hand to an LLM for code review analysis.
The database contains commits, snapshot_symbols, snapshot_files, snapshot_refs,
and file_contents — but no review documents.

For multi-commit ranges, review database creation automatically uses git
worktrees so source, symbol, reference, and body-hash snapshots match each
commit. A single commit is indexed directly from the current checkout.

Examples:
  codebase-browser review db create --commits HEAD~10..HEAD --db pr-42.db
  codebase-browser review db create --commits HEAD --db current.db`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if repoRoot == "" {
				repoRoot = "."
			}
			if commitRange == "" {
				commitRange = "HEAD"
			}
			if len(patterns) == 0 {
				patterns = defaultPatterns()
			}

			store, err := review.Create(dbPath)
			if err != nil {
				return fmt.Errorf("create review db: %w", err)
			}
			defer func() { _ = store.Close() }()

			opts := review.IndexOptions{
				RepoRoot:     repoRoot,
				CommitRange:  commitRange,
				Patterns:     patterns,
				IncludeTests: includeTests,
				Parallelism:  parallelism,
				SkipDocs:     true,
				OnProgress: func(phase string, done, total int, detail string) {
					fmt.Fprintf(os.Stderr, "  [%s %d/%d] %s\n", phase, done, total, detail)
				},
			}

			result, err := review.IndexReview(ctx, store, opts)
			if err != nil {
				return fmt.Errorf("index review: %w", err)
			}

			fmt.Fprintf(os.Stderr, "\nDone in %s: %d commits indexed\n",
				result.Duration.Round(time.Millisecond), result.CommitsIndexed)
			for _, idxErr := range result.Errors {
				fmt.Fprintf(os.Stderr, "  ERROR %s: %v\n", idxErr.Detail, idxErr.Err)
			}
			if len(result.Errors) > 0 {
				return fmt.Errorf("index completed with %d error(s)", len(result.Errors))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "review.db", "Path to review database")
	cmd.Flags().StringVar(&repoRoot, "repo-root", ".", "Path to git repository root")
	cmd.Flags().StringVar(&commitRange, "commits", "", "Git log range spec (e.g. HEAD~10..HEAD)")
	cmd.Flags().StringArrayVar(&patterns, "patterns", nil, "Go package patterns for extraction")
	cmd.Flags().BoolVar(&includeTests, "include-tests", true, "Include test files")
	cmd.Flags().IntVar(&parallelism, "parallelism", 1, "Max concurrent worktrees for multi-commit indexing")

	return cmd
}
