package staticapp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const manifestKind = "codebase-browser-sqljs-static-export"

// Options controls the static-only sql.js export. The Go binary is an offline
// packager here: it copies a fully indexed SQLite DB next to the SPA and writes
// enough boot metadata for the browser to open that DB with sql.js.
type Options struct {
	DBPath           string
	OutDir           string
	RepoRoot         string
	BuildSPA         bool
	RenderReviewDocs bool
	StrictDocs       bool
}

// Export writes a static sql.js application bundle to Options.OutDir.
func Export(ctx context.Context, opts Options) error {
	if opts.DBPath == "" {
		return fmt.Errorf("DBPath is required")
	}
	if opts.OutDir == "" {
		return fmt.Errorf("OutDir is required")
	}
	if opts.RepoRoot == "" {
		opts.RepoRoot = "."
	}

	if opts.BuildSPA {
		fmt.Fprintln(os.Stderr, "Building SPA...")
		if err := buildSPA(ctx); err != nil {
			return fmt.Errorf("build SPA: %w", err)
		}
	}

	fmt.Fprintln(os.Stderr, "Copying SPA assets...")
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := copyTree("ui/dist/public", opts.OutDir); err != nil {
		return fmt.Errorf("copy SPA build: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Copying SQLite database...")
	dbOutPath := filepath.Join(opts.OutDir, "db", "codebase.db")
	if err := copyFile(opts.DBPath, dbOutPath); err != nil {
		return fmt.Errorf("copy SQLite DB: %w", err)
	}

	if opts.RenderReviewDocs {
		fmt.Fprintln(os.Stderr, "Rendering review docs into SQLite...")
		if err := AddRenderedReviewDocs(ctx, dbOutPath, opts.RepoRoot, opts.StrictDocs); err != nil {
			return fmt.Errorf("render review docs: %w", err)
		}
	}

	manifest, err := buildManifest(ctx, opts, dbOutPath)
	if err != nil {
		return fmt.Errorf("build manifest: %w", err)
	}
	if err := writeManifest(opts.OutDir, manifest); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Wrote manifest.json\n")
	fmt.Fprintf(os.Stderr, "Copied SQLite DB to %s\n", dbOutPath)
	fmt.Fprintf(os.Stderr, "\nExport complete: %s\n", opts.OutDir)
	fmt.Fprintf(os.Stderr, "Serve %s with a static file server and open /#/ in a browser\n", opts.OutDir)
	return nil
}

func buildSPA(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "pnpm", "-C", "ui", "run", "build")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "VITE_STATIC_EXPORT=1")
	return cmd.Run()
}

func buildManifest(ctx context.Context, opts Options, dbOutPath string) (*Manifest, error) {
	info, err := os.Stat(dbOutPath)
	if err != nil {
		return nil, fmt.Errorf("stat output DB: %w", err)
	}

	commits, err := inspectCommits(ctx, dbOutPath)
	if err != nil {
		return nil, err
	}

	return &Manifest{
		SchemaVersion: 1,
		Kind:          manifestKind,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		DB: DBManifest{
			Path:          "db/codebase.db",
			SizeBytes:     info.Size(),
			SchemaVersion: 1,
		},
		Features: FeatureManifest{
			CodebaseBrowser: true,
			ReviewDocs:      commits.hasReviewDocs,
			LLMDatabase:     true,
		},
		Repo: RepoManifest{
			RootLabel: filepath.Base(absOrClean(opts.RepoRoot)),
		},
		Commits: CommitManifest{
			Count:  commits.count,
			Oldest: commits.oldest,
			Newest: commits.newest,
		},
		Runtime: RuntimeManifest{
			QueryEngine:              "sql.js",
			RequiresStaticHTTPServer: true,
			HasGoRuntimeServer:       false,
		},
	}, nil
}

type commitInspection struct {
	count         int
	oldest        string
	newest        string
	hasReviewDocs bool
}

func inspectCommits(ctx context.Context, dbPath string) (*commitInspection, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open output DB for manifest: %w", err)
	}
	defer db.Close()

	out := &commitInspection{}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM commits WHERE error = ''`).Scan(&out.count); err != nil {
		return nil, fmt.Errorf("count commits: %w", err)
	}
	_ = db.QueryRowContext(ctx, `SELECT hash FROM commits WHERE error = '' ORDER BY sequence ASC, author_time ASC LIMIT 1`).Scan(&out.oldest)
	_ = db.QueryRowContext(ctx, `SELECT hash FROM commits WHERE error = '' ORDER BY sequence DESC, author_time DESC LIMIT 1`).Scan(&out.newest)

	var reviewCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM review_docs`).Scan(&reviewCount); err == nil {
		out.hasReviewDocs = reviewCount > 0
	}
	return out, nil
}

func writeManifest(outDir string, manifest *Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	path := filepath.Join(outDir, "manifest.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func absOrClean(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return abs
}
