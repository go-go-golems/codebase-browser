package review

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/wesen/codebase-browser/internal/review"
)

func newDBValidateCmd() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate review database integrity",
		Long: `Validate review database integrity.

The validator checks invariants needed by the static browser, including that
snapshot_symbols rows point to files present in the same commit and that
snapshot_refs rows point to commit-local files and from-symbols. This catches
corrupt historical symbol/file mappings such as unchanged symbols that moved
files but were incorrectly deduplicated during indexing.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := review.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open review db: %w", err)
			}
			defer func() { _ = store.Close() }()

			report, err := review.ValidateIntegrity(ctx, store.DB())
			if err != nil {
				return fmt.Errorf("validate integrity: %w", err)
			}

			fmt.Fprintf(os.Stdout, "Review DB integrity report for %s\n", dbPath)
			if len(report.SchemaVersions) > 0 {
				keys := make([]string, 0, len(report.SchemaVersions))
				for key := range report.SchemaVersions {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					fmt.Fprintf(os.Stdout, "  %s: %s\n", key, report.SchemaVersions[key])
				}
			}
			fmt.Fprintf(os.Stdout, "  bad_symbol_file_joins: %d\n", report.BadSymbolFileJoins)
			fmt.Fprintf(os.Stdout, "  bad_ref_file_joins: %d\n", report.BadRefFileJoins)
			fmt.Fprintf(os.Stdout, "  bad_ref_from_symbol_joins: %d\n", report.BadRefFromSymbolJoins)

			if report.HasFailures() {
				return fmt.Errorf("review database integrity check failed")
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "review.db", "Path to review database")
	return cmd
}
