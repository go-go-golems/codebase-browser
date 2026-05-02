package staticapp

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/wesen/codebase-browser/internal/review"
)

func TestExportCopiesDBWritesManifestAndOmitsLegacyRuntimeFiles(t *testing.T) {
	ctx := context.Background()
	dbPath := createStaticAppFixtureDB(t, false)
	workDir := t.TempDir()
	writeFakeSPABuild(t, workDir)
	withWorkingDir(t, workDir)

	outDir := filepath.Join(workDir, "out")
	if err := Export(ctx, Options{DBPath: dbPath, OutDir: outDir, RepoRoot: workDir}); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
		t.Fatalf("index.html not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "db", "codebase.db")); err != nil {
		t.Fatalf("db/codebase.db not copied: %v", err)
	}
	for _, legacy := range []string{"precomputed.json", "search.wasm", "wasm_exec.js"} {
		if _, err := os.Stat(filepath.Join(outDir, legacy)); err == nil {
			t.Fatalf("legacy runtime file %s should not be exported", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy runtime file %s: %v", legacy, err)
		}
	}

	manifestBytes, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if manifest.Kind != manifestKind {
		t.Fatalf("manifest kind = %q, want %q", manifest.Kind, manifestKind)
	}
	if manifest.DB.Path != "db/codebase.db" {
		t.Fatalf("manifest db path = %q", manifest.DB.Path)
	}
	if manifest.Runtime.QueryEngine != "sql.js" || manifest.Runtime.HasGoRuntimeServer {
		t.Fatalf("unexpected runtime manifest: %+v", manifest.Runtime)
	}
	if manifest.Commits.Count != 1 || manifest.Features.ReviewDocs {
		t.Fatalf("unexpected manifest counts/features: commits=%+v features=%+v", manifest.Commits, manifest.Features)
	}
}

func TestAddRenderedReviewDocsCreatesStaticTableOnCopiedDB(t *testing.T) {
	ctx := context.Background()
	dbPath := createStaticAppFixtureDB(t, true)
	if err := AddRenderedReviewDocs(ctx, dbPath, t.TempDir(), false); err != nil {
		t.Fatalf("AddRenderedReviewDocs() error = %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var title, html, snippetsJSON, errorsJSON string
	if err := db.QueryRow(`
		SELECT title, html, snippets_json, errors_json
		FROM static_review_rendered_docs
		WHERE slug = 'fixture'
	`).Scan(&title, &html, &snippetsJSON, &errorsJSON); err != nil {
		t.Fatalf("query rendered doc: %v", err)
	}
	if title != "Fixture Review" {
		t.Fatalf("title = %q", title)
	}
	if html == "" {
		t.Fatalf("html is empty")
	}
	if snippetsJSON != "[]" || errorsJSON != "[]" {
		t.Fatalf("unexpected rendered metadata snippets=%s errors=%s", snippetsJSON, errorsJSON)
	}
}

func createStaticAppFixtureDB(t *testing.T, withReviewDoc bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "codebase.db")
	store, err := review.Create(path)
	if err != nil {
		t.Fatalf("create review db: %v", err)
	}
	defer store.Close()
	db := store.DB()
	if _, err := db.Exec(`
		INSERT INTO commits(hash, short_hash, message, author_name, author_email, author_time, parent_hashes, tree_hash, indexed_at, sequence, branch, error)
		VALUES ('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'aaaaaaa', 'fixture', 'Test', 'test@example.com', 100, '[]', '', 100, 1, '', '');
		INSERT INTO packages(stable_id, import_path, name, doc, language)
		VALUES ('pkg:fixture', 'fixture', 'fixture', '', 'go');
		INSERT INTO files(stable_id, path, package_id, size, line_count, sha256, language, build_tags_json)
		VALUES ('file:fixture.go', 'fixture.go', 1, 12, 1, 'hash-fixture', 'go', '[]');
		INSERT INTO file_contents(content_hash, content)
		VALUES ('hash-fixture', CAST('package fixture' AS BLOB));
		INSERT INTO commit_packages(commit_id, package_id) VALUES (1, 1);
		INSERT INTO commit_files(commit_id, file_id) VALUES (1, 1);
	`); err != nil {
		t.Fatalf("insert fixture history rows: %v", err)
	}
	if withReviewDoc {
		if _, err := db.Exec(`
			INSERT INTO review_docs(slug, title, path, content, frontmatter_json, indexed_at)
			VALUES (?, ?, ?, ?, '{}', 100)
		`, "fixture", "Fixture Review", "fixture.md", "# Fixture Review\n\nPlain text."); err != nil {
			t.Fatalf("insert review doc: %v", err)
		}
	}
	return path
}

func writeFakeSPABuild(t *testing.T, root string) {
	t.Helper()
	public := filepath.Join(root, "ui", "dist", "public")
	if err := os.MkdirAll(public, 0o755); err != nil {
		t.Fatalf("mkdir fake SPA build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(public, "index.html"), []byte("<div id=\"root\"></div>"), 0o644); err != nil {
		t.Fatalf("write fake index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(public, "sql-wasm.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatalf("write fake sql wasm: %v", err)
	}
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore working dir: %v", err)
		}
	})
}
