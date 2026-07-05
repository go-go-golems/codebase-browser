package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openFixtureDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", t.TempDir()+"/fixture.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stmts := []string{
		`CREATE TABLE commits (hash TEXT, short_hash TEXT, author_time INTEGER, error TEXT DEFAULT '')`,
		`CREATE TABLE snapshot_packages (commit_hash TEXT, id TEXT, import_path TEXT, name TEXT, doc TEXT, language TEXT)`,
		`CREATE TABLE snapshot_files (commit_hash TEXT, id TEXT, path TEXT, package_id TEXT, size INTEGER, line_count INTEGER, sha256 TEXT, content_hash TEXT, language TEXT, build_tags_json TEXT)`,
		`CREATE TABLE snapshot_symbols (commit_hash TEXT, id TEXT, kind TEXT, name TEXT, package_id TEXT, file_id TEXT, start_line INTEGER, start_col INTEGER, end_line INTEGER, end_col INTEGER, start_offset INTEGER, end_offset INTEGER, doc TEXT, signature TEXT, receiver_type TEXT, receiver_pointer INTEGER, exported INTEGER, language TEXT, body_hash TEXT)`,
		`CREATE TABLE file_contents (content_hash TEXT, content BLOB)`,
		`CREATE TABLE static_review_rendered_docs (slug TEXT, title TEXT, html TEXT, snippets_json TEXT, errors_json TEXT)`,
		`INSERT INTO commits(hash, short_hash, author_time, error) VALUES ('abcdef123456', 'abcdef1', 1, '')`,
		`INSERT INTO snapshot_packages VALUES ('abcdef123456', 'pkg:example.com/demo', 'example.com/demo', 'demo', '', 'go')`,
		`INSERT INTO snapshot_files VALUES ('abcdef123456', 'file:main.go', 'main.go', 'pkg:example.com/demo', 28, 3, 'h1', 'h1', 'go', '[]')`,
		`INSERT INTO snapshot_symbols VALUES ('abcdef123456', 'sym:example.com/demo.func.Hello', 'func', 'Hello', 'pkg:example.com/demo', 'file:main.go', 3, 1, 3, 15, 14, 28, '', 'func Hello() {}', '', 0, 1, 'go', 'b1')`,
		"INSERT INTO file_contents VALUES ('h1', CAST('package main\n\nfunc Hello() {}\n' AS BLOB))",
		`INSERT INTO static_review_rendered_docs VALUES ('intro', 'Intro', '<h1>Intro</h1>', '[]', '[]')`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	return db
}

func TestLiveServerAPI(t *testing.T) {
	h := New(openFixtureDB(t), "").Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/index", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/api/index code=%d body=%s", w.Code, w.Body.String())
	}
	var idx indexResponse
	if err := json.Unmarshal(w.Body.Bytes(), &idx); err != nil {
		t.Fatal(err)
	}
	if got := len(idx.Symbols); got != 1 {
		t.Fatalf("symbols=%d, want 1", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/search?q=Hello&kind=func", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/api/search code=%d body=%s", w.Code, w.Body.String())
	}
	var symbols []symbolRow
	if err := json.Unmarshal(w.Body.Bytes(), &symbols); err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 || symbols[0].Name != "Hello" {
		t.Fatalf("symbols=%+v", symbols)
	}
}

func TestLiveServerSourceAndReviewDocs(t *testing.T) {
	h := New(openFixtureDB(t), "").Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/source?path=main.go", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "package main\n\nfunc Hello() {}\n" {
		t.Fatalf("source code=%d body=%q", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/review-docs/intro", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("review doc code=%d body=%s", w.Code, w.Body.String())
	}
	var doc reviewDoc
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.HTML != "<h1>Intro</h1>" {
		t.Fatalf("doc=%+v", doc)
	}
}
