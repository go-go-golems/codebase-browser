package review

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/wesen/codebase-browser/internal/review"
)

func newIndexCmd() *cobra.Command {
	var (
		dbPath       string
		repoRoot     string
		commitRange  string
		docsPaths    []string
		patterns     []string
		includeTests bool
		parallelism  int
		incremental  bool
		docsOnly     bool
		strictDocs   bool
	)

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index commits and markdown docs into a review database",
		Long: `Index a git commit range and a set of markdown review guides into a single SQLite database.

The database contains:
  - Per-commit snapshots (commits, snapshot_symbols, snapshot_files, snapshot_refs)
  - Review documents (review_docs, review_doc_snippets)

This is the input for 'review export', which packages a live-server browser bundle.

For multi-commit ranges, review indexing automatically uses git worktrees so
source, symbol, reference, and body-hash snapshots match each commit. A single
commit is indexed directly from the current checkout.

Examples:
  codebase-browser review index --commits HEAD~10..HEAD --docs ./reviews/pr-42.md --db pr-42.db
  codebase-browser review index --commits HEAD --docs ./reviews/current.md --db review.db`,
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
			if len(docsPaths) == 0 {
				return fmt.Errorf("--docs is required (provide markdown files or directories)")
			}

			var store *review.Store
			var err error
			switch {
			case docsOnly:
				store, err = review.Open(dbPath)
			case incremental:
				store, err = review.OpenOrCreate(dbPath)
			default:
				store, err = review.Create(dbPath)
			}
			if err != nil {
				return fmt.Errorf("open review db: %w", err)
			}
			defer func() { _ = store.Close() }()
			if docsOnly {
				hasCommits, err := store.HasCommits(ctx)
				if err != nil {
					return fmt.Errorf("check existing review db: %w", err)
				}
				if !hasCommits {
					return fmt.Errorf("--docs-only requires an existing review database with at least one indexed commit")
				}
			}

			opts := review.IndexOptions{
				RepoRoot:     repoRoot,
				CommitRange:  commitRange,
				DocsPaths:    docsPaths,
				Patterns:     patterns,
				IncludeTests: includeTests,
				Parallelism:  parallelism,
				Incremental:  incremental,
				DocsOnly:     docsOnly,
				StrictDocs:   strictDocs,
				OnProgress: func(phase string, done, total int, detail string) {
					fmt.Fprintf(os.Stderr, "  [%s %d/%d] %s\n", phase, done, total, detail)
				},
			}

			result, err := review.IndexReview(ctx, store, opts)
			if err != nil {
				return fmt.Errorf("index review: %w", err)
			}

			fmt.Fprintf(os.Stderr, "\nDone in %s: %d commits, %d docs, %d snippets\n",
				result.Duration.Round(time.Millisecond), result.CommitsIndexed,
				result.DocsIndexed, result.SnippetsIndexed)
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
	cmd.Flags().StringArrayVar(&docsPaths, "docs", nil, "Markdown files or directories to index")
	cmd.Flags().StringSliceVar(&patterns, "patterns", nil, "Go package patterns for extraction (repeat flag or comma-separate values)")
	cmd.Flags().BoolVar(&includeTests, "include-tests", true, "Include test files")
	cmd.Flags().IntVar(&parallelism, "parallelism", 1, "Max concurrent worktrees for multi-commit indexing")
	cmd.Flags().BoolVar(&incremental, "incremental", false, "Append to existing database instead of recreating it")
	cmd.Flags().BoolVar(&docsOnly, "docs-only", false, "Only re-index markdown docs, skip commit indexing (requires existing DB)")
	cmd.Flags().BoolVar(&strictDocs, "strict-docs", false, "Fail if markdown review docs contain unresolved codebase-* directives")

	_ = cmd.MarkFlagRequired("docs")

	return cmd
}
