package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
)

type reviewDocMeta struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type reviewDoc struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	HTML            string `json:"html,omitempty"`
	Markdown        string `json:"markdown,omitempty"`
	SnippetsJSON    string `json:"snippetsJson,omitempty"`
	ErrorsJSON      string `json:"errorsJson,omitempty"`
	BlocksJSON      string `json:"blocksJson,omitempty"`
	DiagnosticsJSON string `json:"diagnosticsJson,omitempty"`
}

func (s *Server) handleReviewDocList(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	table, err := firstExistingTable(db, "static_review_pages", "static_review_rendered_docs", "review_docs")
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "review document table not found")
		return
	}
	query := "SELECT slug, title FROM " + table + " ORDER BY slug"
	rows, err := db.QueryContext(r.Context(), query)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []reviewDocMeta{}
	for rows.Next() {
		var row reviewDocMeta
		if err := rows.Scan(&row.Slug, &row.Title); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, out)
}

func (s *Server) handleReviewDoc(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	slug := r.PathValue("slug")
	if slug == "" {
		writeJSONError(w, http.StatusBadRequest, "missing slug")
		return
	}
	if doc, err := structuredReviewDoc(r.Context(), db, slug); err == nil {
		writeJSON(w, doc)
		return
	} else if !isMissingTable(err) && err != sql.ErrNoRows {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if doc, err := renderedReviewDoc(r.Context(), db, slug); err == nil {
		writeJSON(w, doc)
		return
	} else if !isMissingTable(err) && err != sql.ErrNoRows {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if doc, err := rawReviewDoc(r.Context(), db, slug); err == nil {
		writeJSON(w, doc)
		return
	} else if err == sql.ErrNoRows || isMissingTable(err) {
		writeJSONError(w, http.StatusNotFound, "review doc not found")
		return
	} else {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func structuredReviewDoc(ctx context.Context, db *sql.DB, slug string) (reviewDoc, error) {
	var doc reviewDoc
	err := db.QueryRowContext(ctx, `
SELECT slug, title, blocks_json, diagnostics_json
FROM static_review_pages
WHERE slug = ?`, slug).Scan(&doc.Slug, &doc.Title, &doc.BlocksJSON, &doc.DiagnosticsJSON)
	if err == nil {
		doc.SnippetsJSON = reviewDocSnippetsJSON(ctx, db, slug)
		doc.ErrorsJSON = doc.DiagnosticsJSON
	}
	return doc, err
}

func renderedReviewDoc(ctx context.Context, db *sql.DB, slug string) (reviewDoc, error) {
	var doc reviewDoc
	err := db.QueryRowContext(ctx, `
SELECT slug, title, html, snippets_json, errors_json
FROM static_review_rendered_docs
WHERE slug = ?`, slug).Scan(&doc.Slug, &doc.Title, &doc.HTML, &doc.SnippetsJSON, &doc.ErrorsJSON)
	return doc, err
}

func rawReviewDoc(ctx context.Context, db *sql.DB, slug string) (reviewDoc, error) {
	var doc reviewDoc
	err := db.QueryRowContext(ctx, `
SELECT slug, title, content
FROM review_docs
WHERE slug = ?`, slug).Scan(&doc.Slug, &doc.Title, &doc.Markdown)
	return doc, err
}

type reviewDocSnippet struct {
	StubID     string            `json:"stubId"`
	Directive  string            `json:"directive"`
	SymbolID   string            `json:"symbolId,omitempty"`
	FilePath   string            `json:"filePath,omitempty"`
	Kind       string            `json:"kind,omitempty"`
	Language   string            `json:"language,omitempty"`
	Text       string            `json:"text"`
	CommitHash string            `json:"commitHash,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
	StartLine  int               `json:"startLine,omitempty"`
	EndLine    int               `json:"endLine,omitempty"`
}

func reviewDocSnippetsJSON(ctx context.Context, db *sql.DB, slug string) string {
	rows, err := db.QueryContext(ctx, `
SELECT s.stub_id,
       s.directive,
       COALESCE(s.symbol_id, ''),
       COALESCE(s.file_path, ''),
       COALESCE(s.kind, ''),
       COALESCE(s.language, ''),
       s.text,
       COALESCE(s.commit_hash, ''),
       COALESCE(s.params_json, '{}'),
       s.start_line,
       s.end_line
FROM review_doc_snippets s
JOIN review_docs d ON d.id = s.doc_id
WHERE d.slug = ?
ORDER BY s.id`, slug)
	if err != nil {
		return "[]"
	}
	defer rows.Close()
	out := []reviewDocSnippet{}
	for rows.Next() {
		var row reviewDocSnippet
		var paramsJSON string
		if err := rows.Scan(
			&row.StubID,
			&row.Directive,
			&row.SymbolID,
			&row.FilePath,
			&row.Kind,
			&row.Language,
			&row.Text,
			&row.CommitHash,
			&paramsJSON,
			&row.StartLine,
			&row.EndLine,
		); err != nil {
			return "[]"
		}
		_ = json.Unmarshal([]byte(paramsJSON), &row.Params)
		out = append(out, row)
	}
	data, err := json.Marshal(out)
	if err != nil {
		return "[]"
	}
	return string(data)
}
