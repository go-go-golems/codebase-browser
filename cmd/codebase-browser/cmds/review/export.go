package review

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wesen/codebase-browser/internal/staticapp"
)

func newExportCmd() *cobra.Command {
	var (
		dbPath     string
		outDir     string
		repoRoot   string
		strictDocs bool
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export a review database to a standalone sql.js static directory",
		Long: `Export a review database to a standalone static directory.

The exported app has no Go runtime server. It contains the built React SPA,
a manifest, the sql.js WASM runtime, and the SQLite database at db/codebase.db.
The browser opens that database directly with sql.js.

The export requires a review database produced by 'review index' or 'review db create'.

Examples:
  codebase-browser review export --db pr-42.db --out ./pr-42-export/
  codebase-browser review export --db review.db --out /tmp/export/`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				return fmt.Errorf("--db is required")
			}
			if outDir == "" {
				return fmt.Errorf("--out is required")
			}

			return staticapp.Export(cmd.Context(), staticapp.Options{
				DBPath:           dbPath,
				OutDir:           outDir,
				RepoRoot:         repoRoot,
				BuildSPA:         true,
				RenderReviewDocs: true,
				StrictDocs:       strictDocs,
			})
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Path to review database (required)")
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory for the static export (required)")
	cmd.Flags().StringVar(&repoRoot, "repo-root", ".", "Repository root label used in the export manifest")
	cmd.Flags().BoolVar(&strictDocs, "strict-docs", false, "Fail if markdown review docs contain unresolved codebase-* directives")

	_ = cmd.MarkFlagRequired("db")
	_ = cmd.MarkFlagRequired("out")

	return cmd
}
