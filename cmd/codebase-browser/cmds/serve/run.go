// Package serve wires the `codebase-browser serve` command.
package serve

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/wesen/codebase-browser/internal/server"
	cbsqlite "github.com/wesen/codebase-browser/internal/sqlite"
)

type options struct {
	addr      string
	dbPath    string
	staticDir string
}

// Register attaches `serve` to the root command.
func Register(root *cobra.Command) error {
	opts := &options{}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the live Go codebase-browser HTTP server",
		Long: strings.TrimSpace(`Run a live Go HTTP server around a codebase-browser SQLite database.

The static export remains the canonical shareable artifact, but this command is
useful for local demos and for consumers that want Go-side /api/* queries instead
of browser-side sql.js queries.

Examples:
  codebase-browser serve --db /tmp/pr-42.db --addr :3001
  codebase-browser serve --db /tmp/pr-42-static/db/codebase.db --static-dir /tmp/pr-42-static

API examples:
  GET /api/health
  GET /api/index
  GET /api/review-docs
  GET /api/search?q=Export&kind=func
  GET /api/source?path=cmd/codebase-browser/main.go
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), opts)
		},
	}
	cmd.Flags().StringVar(&opts.addr, "addr", ":3001", "Bind address")
	cmd.Flags().StringVar(&opts.dbPath, "db", "internal/sqlite/embed/codebase.db", "Path to codebase-browser SQLite DB")
	cmd.Flags().StringVar(&opts.staticDir, "static-dir", "", "Optional static SPA/export directory to serve with fallback to index.html")
	root.AddCommand(cmd)
	return nil
}

func run(ctx context.Context, opts *options) error {
	store, err := cbsqlite.Open(opts.dbPath)
	if err != nil {
		return fmt.Errorf("open db %q: %w", opts.dbPath, err)
	}
	defer func() { _ = store.Close() }()

	srv := server.New(store.DB(), opts.staticDir)
	httpSrv := &http.Server{Addr: opts.addr, Handler: srv.Handler()}
	log.Info().Str("addr", opts.addr).Str("db", opts.dbPath).Str("static_dir", opts.staticDir).Msg("codebase-browser live server listening")

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		_ = httpSrv.Close()
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
