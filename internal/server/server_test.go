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
		`CREATE TABLE snapshot_refs (commit_hash TEXT, from_symbol_id TEXT, to_symbol_id TEXT, kind TEXT, file_id TEXT, start_line INTEGER, start_col INTEGER, end_line INTEGER, end_col INTEGER, start_offset INTEGER, end_offset INTEGER)`,
		`CREATE TABLE file_contents (content_hash TEXT, content BLOB)`,
		`INSERT INTO snapshot_files VALUES ('abcdef123456', 'file:util.go', 'util.go', 'pkg:example.com/demo', 24, 3, 'h2', 'h2', 'go', '[]')`,
		`INSERT INTO snapshot_symbols VALUES ('abcdef123456', 'sym:example.com/demo.func.Helper', 'func', 'Helper', 'pkg:example.com/demo', 'file:util.go', 3, 1, 3, 16, 14, 30, '', 'func Helper() {}', '', 0, 1, 'go', 'b2')`,
		`INSERT INTO snapshot_refs VALUES ('abcdef123456', 'sym:example.com/demo.func.Hello', 'sym:example.com/demo.func.Helper', 'call', 'file:main.go', 3, 8, 3, 14, 22, 28)`,
		`INSERT INTO snapshot_refs VALUES ('abcdef123456', 'sym:example.com/demo.func.Helper', 'sym:example.com/demo.func.Hello', 'call', 'file:util.go', 3, 8, 3, 13, 22, 27)`,
		"INSERT INTO file_contents VALUES ('h2', CAST('package main\n\nfunc Helper() {}\n' AS BLOB))",
		`CREATE TABLE static_review_rendered_docs (slug TEXT, title TEXT, html TEXT, snippets_json TEXT, errors_json TEXT)`,
		`CREATE TABLE review_docs (slug TEXT, title TEXT, content TEXT)`,
		`INSERT INTO commits(hash, short_hash, author_time, error) VALUES ('abcdef123456', 'abcdef1', 1, '')`,
		`INSERT INTO snapshot_packages VALUES ('abcdef123456', 'pkg:example.com/demo', 'example.com/demo', 'demo', '', 'go')`,
		`INSERT INTO snapshot_files VALUES ('abcdef123456', 'file:main.go', 'main.go', 'pkg:example.com/demo', 28, 3, 'h1', 'h1', 'go', '[]')`,
		`INSERT INTO snapshot_symbols VALUES ('abcdef123456', 'sym:example.com/demo.func.Hello', 'func', 'Hello', 'pkg:example.com/demo', 'file:main.go', 3, 1, 3, 15, 14, 28, '', 'func Hello() {}', '', 0, 1, 'go', 'b1')`,
		"INSERT INTO file_contents VALUES ('h1', CAST('package main\n\nfunc Hello() {}\n' AS BLOB))",
		`INSERT INTO static_review_rendered_docs VALUES ('intro', 'Intro', '<h1>Intro</h1>', '[]', '[]')`,
		`INSERT INTO review_docs VALUES ('raw', 'Raw', '# Raw')`,
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
	if got := len(idx.Symbols); got != 2 {
		t.Fatalf("symbols=%d, want 2", got)
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

func TestLiveServerXrefEndpoints(t *testing.T) {
	h := New(openFixtureDB(t), "").Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/xref?id=sym:example.com/demo.func.Hello", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/api/xref code=%d body=%s", w.Code, w.Body.String())
	}
	var xref xrefResponse
	if err := json.Unmarshal(w.Body.Bytes(), &xref); err != nil {
		t.Fatal(err)
	}
	if len(xref.UsedBy) != 1 || len(xref.Uses) != 1 || xref.Uses[0].ToSymbolID != "sym:example.com/demo.func.Helper" {
		t.Fatalf("xref=%+v", xref)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/snippet-refs?symbol=sym:example.com/demo.func.Hello", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/api/snippet-refs code=%d body=%s", w.Code, w.Body.String())
	}
	var snippetRefs []snippetRefView
	if err := json.Unmarshal(w.Body.Bytes(), &snippetRefs); err != nil {
		t.Fatal(err)
	}
	if len(snippetRefs) != 1 || snippetRefs[0].OffsetInSnippet != 8 || snippetRefs[0].Length != 6 {
		t.Fatalf("snippetRefs=%+v", snippetRefs)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/source-refs?path=main.go", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/api/source-refs code=%d body=%s", w.Code, w.Body.String())
	}
	var sourceRefs []sourceRefView
	if err := json.Unmarshal(w.Body.Bytes(), &sourceRefs); err != nil {
		t.Fatal(err)
	}
	if len(sourceRefs) != 1 || sourceRefs[0].Offset != 22 {
		t.Fatalf("sourceRefs=%+v", sourceRefs)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/file-xref?path=main.go", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/api/file-xref code=%d body=%s", w.Code, w.Body.String())
	}
	var fileXref fileXrefResponse
	if err := json.Unmarshal(w.Body.Bytes(), &fileXref); err != nil {
		t.Fatal(err)
	}
	if len(fileXref.UsedBy) != 1 || len(fileXref.Uses) != 1 {
		t.Fatalf("fileXref=%+v", fileXref)
	}
}

func TestResolveCommitSupportsHeadOffsets(t *testing.T) {
	db := openFixtureDB(t)
	if _, err := db.Exec(`INSERT INTO commits(hash, short_hash, author_time, error) VALUES ('fedcba654321', 'fedcba6', 2, '')`); err != nil {
		t.Fatal(err)
	}
	s := New(db, "")
	got, err := s.resolveCommit("HEAD")
	if err != nil || got != "fedcba654321" {
		t.Fatalf("HEAD = %q, %v", got, err)
	}
	got, err = s.resolveCommit("HEAD~1")
	if err != nil || got != "abcdef123456" {
		t.Fatalf("HEAD~1 = %q, %v", got, err)
	}
	if _, err := s.resolveCommit("HEAD~9"); err == nil {
		t.Fatalf("HEAD~9 resolved, want error")
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

func TestLiveServerRawReviewDocFallbackUsesContentColumn(t *testing.T) {
	db := openFixtureDB(t)
	if _, err := db.Exec(`DROP TABLE static_review_rendered_docs`); err != nil {
		t.Fatal(err)
	}
	h := New(db, "").Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/review-docs/raw", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("review doc code=%d body=%s", w.Code, w.Body.String())
	}
	var doc reviewDoc
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Markdown != "# Raw" {
		t.Fatalf("doc=%+v", doc)
	}
}
