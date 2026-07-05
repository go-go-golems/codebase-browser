// Package server provides the live Go HTTP API for codebase-browser SQLite
// databases. It is intentionally independent from the static sql.js runtime:
// callers can point it at the same review database and query it through /api/*.
package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Server holds the shared state for live HTTP handlers.
type Server struct {
	DB        *sql.DB
	StaticDir string
	mux       *http.ServeMux
}

// New constructs a live server around an opened SQLite database. staticDir is
// optional; when present, non-/api requests are served from that directory with
// SPA fallback to index.html.
func New(db *sql.DB, staticDir string) *Server {
	return &Server{DB: db, StaticDir: staticDir}
}

// Handler returns an http.Handler with API routes and the optional SPA/static
// file handler mounted.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.mux = mux
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/index", s.handleIndex)
	mux.HandleFunc("GET /api/review-docs", s.handleReviewDocList)
	mux.HandleFunc("GET /api/review-docs/{slug}", s.handleReviewDoc)
	mux.HandleFunc("GET /api/symbol", s.handleSymbol)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/source", s.handleSource)
	mux.HandleFunc("GET /api/snippet", s.handleSnippet)
	mux.HandleFunc("GET /api/history/commits", s.handleHistoryCommits)
	mux.HandleFunc("GET /api/history/symbol", s.handleSymbolHistory)
	mux.HandleFunc("GET /api/history/diff", s.handleHistoryDiff)
	mux.HandleFunc("GET /api/history/symbol-body-diff", s.handleSymbolBodyDiff)
	mux.Handle("/", s.staticHandler())
	return withCommonHeaders(mux)
}

func withCommonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) staticHandler() http.Handler {
	if strings.TrimSpace(s.StaticDir) == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, `<!doctype html>
<html><head><meta charset="utf-8"><title>codebase-browser live server</title></head>
<body>
<h1>codebase-browser live server</h1>
<p>Go API is running. Try <a href="/api/health">/api/health</a>, <a href="/api/index">/api/index</a>, or <a href="/api/review-docs">/api/review-docs</a>.</p>
</body></html>`)
		})
	}

	fsys := http.Dir(s.StaticDir)
	fileServer := http.FileServer(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(filepath.Clean(r.URL.Path), string(filepath.Separator))
		if name == "." || name == "" {
			name = "index.html"
		}
		if _, err := os.Stat(filepath.Join(s.StaticDir, name)); err == nil {
			if name == "index.html" || name == "manifest.json" || strings.HasPrefix(name, "db/") {
				w.Header().Set("Cache-Control", "no-store")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, err := os.Stat(filepath.Join(s.StaticDir, "index.html")); err == nil {
			r2 := new(http.Request)
			*r2 = *r
			r2.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r2)
			return
		}
		http.NotFound(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) requireDB(w http.ResponseWriter) (*sql.DB, bool) {
	if s.DB == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "database is not configured")
		return nil, false
	}
	return s.DB, true
}

func optionalString(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func isMissingTable(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no such table")
}

func firstExistingTable(db *sql.DB, names ...string) (string, error) {
	for _, name := range names {
		var found string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type IN ('table','view') AND name = ?`, name).Scan(&found)
		if err == nil {
			return found, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}
	return "", sql.ErrNoRows
}
