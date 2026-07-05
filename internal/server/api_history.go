package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type fileDiffRow struct {
	FileID     string `json:"FileID"`
	Path       string `json:"Path"`
	ChangeType string `json:"ChangeType"`
	OldSHA256  string `json:"OldSHA256"`
	NewSHA256  string `json:"NewSHA256"`
}

type symbolDiffRow struct {
	SymbolID     string `json:"SymbolID"`
	Name         string `json:"Name"`
	Kind         string `json:"Kind"`
	PackageID    string `json:"PackageID"`
	ChangeType   string `json:"ChangeType"`
	OldStartLine int    `json:"OldStartLine"`
	OldEndLine   int    `json:"OldEndLine"`
	NewStartLine int    `json:"NewStartLine"`
	NewEndLine   int    `json:"NewEndLine"`
	OldSignature string `json:"OldSignature"`
	NewSignature string `json:"NewSignature"`
	OldBodyHash  string `json:"OldBodyHash"`
	NewBodyHash  string `json:"NewBodyHash"`
}

type diffStatsRow struct {
	FilesAdded       int `json:"FilesAdded"`
	FilesRemoved     int `json:"FilesRemoved"`
	FilesModified    int `json:"FilesModified"`
	SymbolsAdded     int `json:"SymbolsAdded"`
	SymbolsRemoved   int `json:"SymbolsRemoved"`
	SymbolsModified  int `json:"SymbolsModified"`
	SymbolsMoved     int `json:"SymbolsMoved"`
	SymbolsUnchanged int `json:"SymbolsUnchanged"`
}

type commitDiffResponse struct {
	OldHash string          `json:"OldHash"`
	NewHash string          `json:"NewHash"`
	Files   []fileDiffRow   `json:"Files"`
	Symbols []symbolDiffRow `json:"Symbols"`
	Stats   diffStatsRow    `json:"Stats"`
}

type commitRow struct {
	Hash        string `json:"Hash"`
	ShortHash   string `json:"ShortHash"`
	Message     string `json:"Message"`
	AuthorName  string `json:"AuthorName"`
	AuthorEmail string `json:"AuthorEmail"`
	AuthorTime  int64  `json:"AuthorTime"`
	IndexedAt   int64  `json:"IndexedAt"`
	Branch      string `json:"Branch"`
	Error       string `json:"Error"`
}

type symbolHistoryRow struct {
	CommitHash string `json:"commitHash"`
	ShortHash  string `json:"shortHash"`
	Message    string `json:"message"`
	AuthorTime int64  `json:"authorTime"`
	BodyHash   string `json:"bodyHash"`
	StartLine  int    `json:"startLine"`
	EndLine    int    `json:"endLine"`
	Signature  string `json:"signature"`
	Kind       string `json:"kind"`
}

func (s *Server) handleHistoryCommits(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	rows, err := db.QueryContext(r.Context(), `
SELECT hash, short_hash, message, author_name, author_email, author_time, indexed_at, branch, error
FROM commits
WHERE error = ''
ORDER BY author_time DESC`)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []commitRow{}
	for rows.Next() {
		var row commitRow
		if err := rows.Scan(&row.Hash, &row.ShortHash, &row.Message, &row.AuthorName, &row.AuthorEmail, &row.AuthorTime, &row.IndexedAt, &row.Branch, &row.Error); err != nil {
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

func (s *Server) handleSymbolHistory(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	symbolID := r.URL.Query().Get("symbol")
	if symbolID == "" {
		symbolID = r.URL.Query().Get("id")
	}
	if symbolID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing symbol")
		return
	}
	limit := 0
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			writeJSONError(w, http.StatusBadRequest, "bad limit")
			return
		}
		limit = parsed
	}
	query := `
SELECT c.hash, c.short_hash, c.message, c.author_time, s.body_hash, s.start_line, s.end_line, s.signature, s.kind
FROM snapshot_symbols s
JOIN commits c ON c.hash = s.commit_hash
WHERE s.id = ? AND c.error = ''
ORDER BY c.author_time DESC`
	args := []any{symbolID}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []symbolHistoryRow{}
	for rows.Next() {
		var row symbolHistoryRow
		if err := rows.Scan(&row.CommitHash, &row.ShortHash, &row.Message, &row.AuthorTime, &row.BodyHash, &row.StartLine, &row.EndLine, &row.Signature, &row.Kind); err != nil {
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

func (s *Server) handleHistoryDiff(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		writeJSONError(w, http.StatusBadRequest, "from and to query params required")
		return
	}
	oldHash, err := s.resolveCommit(from)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	newHash, err := s.resolveCommit(to)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	files, err := queryFileDiff(r.Context(), db, oldHash, newHash)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	symbols, err := querySymbolDiff(r.Context(), db, oldHash, newHash)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, commitDiffResponse{OldHash: oldHash, NewHash: newHash, Files: files, Symbols: symbols, Stats: computeDiffStats(files, symbols)})
}

func queryFileDiff(ctx context.Context, db *sql.DB, oldHash, newHash string) ([]fileDiffRow, error) {
	rows, err := db.QueryContext(ctx, `
SELECT b.id AS FileID, b.path AS Path, 'added' AS ChangeType, '' AS OldSHA256, b.sha256 AS NewSHA256
FROM snapshot_files b
LEFT JOIN snapshot_files a ON a.commit_hash = ? AND a.id = b.id
WHERE b.commit_hash = ? AND a.id IS NULL
UNION ALL
SELECT a.id AS FileID, a.path AS Path, 'removed' AS ChangeType, a.sha256 AS OldSHA256, '' AS NewSHA256
FROM snapshot_files a
LEFT JOIN snapshot_files b ON b.commit_hash = ? AND b.id = a.id
WHERE a.commit_hash = ? AND b.id IS NULL
UNION ALL
SELECT b.id AS FileID, b.path AS Path, 'modified' AS ChangeType, a.sha256 AS OldSHA256, b.sha256 AS NewSHA256
FROM snapshot_files a
JOIN snapshot_files b ON b.id = a.id
WHERE a.commit_hash = ? AND b.commit_hash = ? AND a.sha256 != b.sha256
ORDER BY Path`, oldHash, newHash, newHash, oldHash, oldHash, newHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []fileDiffRow{}
	for rows.Next() {
		var r fileDiffRow
		if err := rows.Scan(&r.FileID, &r.Path, &r.ChangeType, &r.OldSHA256, &r.NewSHA256); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func querySymbolDiff(ctx context.Context, db *sql.DB, oldHash, newHash string) ([]symbolDiffRow, error) {
	rows, err := db.QueryContext(ctx, `
SELECT b.id AS SymbolID, b.name AS Name, b.kind AS Kind, b.package_id AS PackageID,
       'added' AS ChangeType, 0 AS OldStartLine, 0 AS OldEndLine,
       b.start_line AS NewStartLine, b.end_line AS NewEndLine,
       '' AS OldSignature, b.signature AS NewSignature, '' AS OldBodyHash, b.body_hash AS NewBodyHash
FROM snapshot_symbols b
LEFT JOIN snapshot_symbols a ON a.commit_hash = ? AND a.id = b.id
WHERE b.commit_hash = ? AND a.id IS NULL
UNION ALL
SELECT a.id AS SymbolID, a.name AS Name, a.kind AS Kind, a.package_id AS PackageID,
       'removed' AS ChangeType, a.start_line AS OldStartLine, a.end_line AS OldEndLine,
       0 AS NewStartLine, 0 AS NewEndLine,
       a.signature AS OldSignature, '' AS NewSignature, a.body_hash AS OldBodyHash, '' AS NewBodyHash
FROM snapshot_symbols a
LEFT JOIN snapshot_symbols b ON b.commit_hash = ? AND b.id = a.id
WHERE a.commit_hash = ? AND b.id IS NULL
UNION ALL
SELECT b.id AS SymbolID, b.name AS Name, b.kind AS Kind, b.package_id AS PackageID,
       CASE
         WHEN a.body_hash != b.body_hash AND a.body_hash != '' AND b.body_hash != '' THEN 'modified'
         WHEN a.signature != b.signature THEN 'signature-changed'
         WHEN a.start_line != b.start_line OR a.end_line != b.end_line THEN 'moved'
         ELSE 'unchanged'
       END AS ChangeType,
       a.start_line AS OldStartLine, a.end_line AS OldEndLine,
       b.start_line AS NewStartLine, b.end_line AS NewEndLine,
       a.signature AS OldSignature, b.signature AS NewSignature,
       a.body_hash AS OldBodyHash, b.body_hash AS NewBodyHash
FROM snapshot_symbols a
JOIN snapshot_symbols b ON b.id = a.id
WHERE a.commit_hash = ? AND b.commit_hash = ?
  AND (a.body_hash != b.body_hash OR a.signature != b.signature OR a.start_line != b.start_line OR a.end_line != b.end_line)
ORDER BY Name`, oldHash, newHash, newHash, oldHash, oldHash, newHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []symbolDiffRow{}
	for rows.Next() {
		var r symbolDiffRow
		if err := rows.Scan(&r.SymbolID, &r.Name, &r.Kind, &r.PackageID, &r.ChangeType, &r.OldStartLine, &r.OldEndLine, &r.NewStartLine, &r.NewEndLine, &r.OldSignature, &r.NewSignature, &r.OldBodyHash, &r.NewBodyHash); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func computeDiffStats(files []fileDiffRow, symbols []symbolDiffRow) diffStatsRow {
	var stats diffStatsRow
	for _, file := range files {
		switch file.ChangeType {
		case "added":
			stats.FilesAdded++
		case "removed":
			stats.FilesRemoved++
		case "modified":
			stats.FilesModified++
		}
	}
	for _, sym := range symbols {
		switch sym.ChangeType {
		case "added":
			stats.SymbolsAdded++
		case "removed":
			stats.SymbolsRemoved++
		case "modified":
			stats.SymbolsModified++
		case "moved":
			stats.SymbolsMoved++
		case "unchanged":
			stats.SymbolsUnchanged++
		}
	}
	return stats
}

type bodyDiffResponse struct {
	SymbolID    string `json:"symbolId"`
	Name        string `json:"name"`
	OldCommit   string `json:"oldCommit"`
	NewCommit   string `json:"newCommit"`
	OldBody     string `json:"oldBody"`
	NewBody     string `json:"newBody"`
	UnifiedDiff string `json:"unifiedDiff"`
	OldRange    string `json:"oldRange"`
	NewRange    string `json:"newRange"`
}

func (s *Server) handleSymbolBodyDiff(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	symbolID := r.URL.Query().Get("symbol")
	if from == "" || to == "" || symbolID == "" {
		writeJSONError(w, http.StatusBadRequest, "from, to, and symbol query params required")
		return
	}
	oldHash, err := s.resolveCommit(from)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	newHash, err := s.resolveCommit(to)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	oldBody, oldRange, name, err := symbolBodyWithRange(r.Context(), db, oldHash, symbolID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	newBody, newRange, newName, err := symbolBodyWithRange(r.Context(), db, newHash, symbolID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	if newName != "" {
		name = newName
	}
	writeJSON(w, bodyDiffResponse{
		SymbolID:    symbolID,
		Name:        name,
		OldCommit:   oldHash,
		NewCommit:   newHash,
		OldBody:     oldBody,
		NewBody:     newBody,
		UnifiedDiff: simpleUnifiedDiff(oldBody, newBody),
		OldRange:    oldRange,
		NewRange:    newRange,
	})
}

func symbolBodyWithRange(ctx context.Context, db *sql.DB, commitHash, symbolID string) (body, lineRange, name string, err error) {
	var startLine, endLine int
	err = db.QueryRowContext(ctx, `
SELECT name, start_line, end_line
FROM snapshot_symbols
WHERE commit_hash = ? AND id = ?`, commitHash, symbolID).Scan(&name, &startLine, &endLine)
	if err != nil {
		return "", "", "", err
	}
	body, err = snippetBySymbol(ctx, db, commitHash, symbolID, "declaration")
	if err != nil {
		return "", "", "", err
	}
	return body, fmt.Sprintf("lines %d-%d", startLine, endLine), name, nil
}

func simpleUnifiedDiff(oldText, newText string) string {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix && oldLines[len(oldLines)-1-suffix] == newLines[len(newLines)-1-suffix] {
		suffix++
	}
	out := []string{}
	for i := 0; i < prefix; i++ {
		out = append(out, "  "+oldLines[i])
	}
	for i := prefix; i < len(oldLines)-suffix; i++ {
		out = append(out, "- "+oldLines[i])
	}
	for i := prefix; i < len(newLines)-suffix; i++ {
		out = append(out, "+ "+newLines[i])
	}
	for i := len(oldLines) - suffix; i < len(oldLines); i++ {
		out = append(out, "  "+oldLines[i])
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
