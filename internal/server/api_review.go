package server

import (
	"context"
	"database/sql"
	"net/http"
)

type reviewDocMeta struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type reviewDoc struct {
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	HTML         string `json:"html,omitempty"`
	Markdown     string `json:"markdown,omitempty"`
	SnippetsJSON string `json:"snippetsJson,omitempty"`
	ErrorsJSON   string `json:"errorsJson,omitempty"`
}

func (s *Server) handleReviewDocList(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	table, err := firstExistingTable(db, "static_review_rendered_docs", "review_docs")
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
SELECT slug, title, markdown
FROM review_docs
WHERE slug = ?`, slug).Scan(&doc.Slug, &doc.Title, &doc.Markdown)
	return doc, err
}
