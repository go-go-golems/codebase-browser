package server

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
)

type refRange struct {
	StartLine   int `json:"startLine"`
	StartCol    int `json:"startCol"`
	EndLine     int `json:"endLine"`
	EndCol      int `json:"endCol"`
	StartOffset int `json:"startOffset"`
	EndOffset   int `json:"endOffset"`
}

type refRecord struct {
	FromSymbolID string   `json:"fromSymbolId"`
	ToSymbolID   string   `json:"toSymbolId"`
	Kind         string   `json:"kind"`
	FileID       string   `json:"fileId"`
	Range        refRange `json:"range"`
}

type xrefUseTarget struct {
	ToSymbolID  string      `json:"toSymbolId"`
	Kind        string      `json:"kind"`
	Count       int         `json:"count"`
	Occurrences []refRecord `json:"occurrences"`
}

type xrefResponse struct {
	ID     string          `json:"id"`
	UsedBy []refRecord     `json:"usedBy"`
	Uses   []xrefUseTarget `json:"uses"`
}

type snippetRefView struct {
	ToSymbolID      string `json:"toSymbolId"`
	Kind            string `json:"kind"`
	OffsetInSnippet int    `json:"offsetInSnippet"`
	Length          int    `json:"length"`
}

type sourceRefView struct {
	ToSymbolID string `json:"toSymbolId"`
	Kind       string `json:"kind"`
	Offset     int    `json:"offset"`
	Length     int    `json:"length"`
}

type fileXrefResponse struct {
	Path   string          `json:"path"`
	UsedBy []refRecord     `json:"usedBy"`
	Uses   []xrefUseTarget `json:"uses"`
}

type fileContentMeta struct {
	FileID      string
	ContentHash string
}

type bodyMeta struct {
	SymbolID    string
	Name        string
	StartOffset int
	EndOffset   int
	StartLine   int
	EndLine     int
	FileID      string
	Signature   string
	FilePath    string
	ContentHash string
}

func (s *Server) handleXref(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		id = r.URL.Query().Get("symbol")
	}
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing symbol")
		return
	}
	commit, err := s.resolveCommit(r.URL.Query().Get("commit"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	usedBy, err := queryRefRecordsTo(r.Context(), db, commit, id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	usesFlat, err := queryRefRecordsFrom(r.Context(), db, commit, id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, xrefResponse{ID: id, UsedBy: usedBy, Uses: groupRefUses(usesFlat)})
}

func (s *Server) handleSnippetRefs(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	id := r.URL.Query().Get("symbol")
	if id == "" {
		id = r.URL.Query().Get("id")
	}
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing symbol")
		return
	}
	commit, err := s.resolveCommit(r.URL.Query().Get("commit"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	meta, err := queryBodyMeta(r.Context(), db, commit, id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	refs, err := queryRefRecordsInFileRange(r.Context(), db, commit, meta.FileID, meta.StartOffset, meta.EndOffset)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]snippetRefView, 0, len(refs))
	for _, ref := range refs {
		out = append(out, snippetRefView{
			ToSymbolID:      ref.ToSymbolID,
			Kind:            ref.Kind,
			OffsetInSnippet: maxInt(0, ref.Range.StartOffset-meta.StartOffset),
			Length:          maxInt(0, ref.Range.EndOffset-ref.Range.StartOffset),
		})
	}
	writeJSON(w, out)
}

func (s *Server) handleSourceRefs(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	if !validSourcePath(path) {
		writeJSONError(w, http.StatusBadRequest, "bad or missing path")
		return
	}
	commit, err := s.resolveCommit(r.URL.Query().Get("commit"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	meta, err := queryFileContentMeta(r.Context(), db, commit, path)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	refs, err := queryRefRecordsInFile(r.Context(), db, commit, meta.FileID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]sourceRefView, 0, len(refs))
	for _, ref := range refs {
		out = append(out, sourceRefView{
			ToSymbolID: ref.ToSymbolID,
			Kind:       ref.Kind,
			Offset:     ref.Range.StartOffset,
			Length:     maxInt(0, ref.Range.EndOffset-ref.Range.StartOffset),
		})
	}
	writeJSON(w, out)
}

func (s *Server) handleFileXref(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	if !validSourcePath(path) {
		writeJSONError(w, http.StatusBadRequest, "bad or missing path")
		return
	}
	commit, err := s.resolveCommit(r.URL.Query().Get("commit"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	meta, err := queryFileContentMeta(r.Context(), db, commit, path)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	usedBy, err := queryRefRecordsToFileSymbols(r.Context(), db, commit, meta.FileID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	usesFlat, err := queryRefRecordsFromFileSymbols(r.Context(), db, commit, meta.FileID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, fileXrefResponse{Path: path, UsedBy: usedBy, Uses: groupRefUses(usesFlat)})
}

func groupRefUses(refs []refRecord) []xrefUseTarget {
	byKey := map[string]int{}
	out := []xrefUseTarget{}
	for _, ref := range refs {
		key := ref.ToSymbolID + "\x00" + ref.Kind
		idx, ok := byKey[key]
		if !ok {
			idx = len(out)
			byKey[key] = idx
			out = append(out, xrefUseTarget{ToSymbolID: ref.ToSymbolID, Kind: ref.Kind, Occurrences: []refRecord{}})
		}
		out[idx].Count++
		out[idx].Occurrences = append(out[idx].Occurrences, ref)
	}
	return out
}

func validSourcePath(path string) bool {
	return path != "" && !strings.Contains(path, "..") && !strings.HasPrefix(path, "/")
}

func queryFileContentMeta(ctx context.Context, db *sql.DB, commit, path string) (fileContentMeta, error) {
	var meta fileContentMeta
	err := db.QueryRowContext(ctx, `
SELECT id, COALESCE(NULLIF(content_hash, ''), sha256)
FROM snapshot_files
WHERE commit_hash = ? AND path = ?`, commit, path).Scan(&meta.FileID, &meta.ContentHash)
	return meta, err
}

func queryBodyMeta(ctx context.Context, db *sql.DB, commit, symbolID string) (bodyMeta, error) {
	var meta bodyMeta
	err := db.QueryRowContext(ctx, `
SELECT s.id,
       s.name,
       s.start_offset,
       s.end_offset,
       s.start_line,
       s.end_line,
       s.file_id,
       s.signature,
       f.path,
       COALESCE(NULLIF(f.content_hash, ''), f.sha256)
FROM snapshot_symbols s
JOIN snapshot_files f
  ON f.commit_hash = s.commit_hash
 AND f.id = s.file_id
WHERE s.commit_hash = ? AND s.id = ?`, commit, symbolID).Scan(
		&meta.SymbolID,
		&meta.Name,
		&meta.StartOffset,
		&meta.EndOffset,
		&meta.StartLine,
		&meta.EndLine,
		&meta.FileID,
		&meta.Signature,
		&meta.FilePath,
		&meta.ContentHash,
	)
	return meta, err
}

func queryRefRecordsFrom(ctx context.Context, db *sql.DB, commit, symbolID string) ([]refRecord, error) {
	return queryRefRecords(ctx, db, refRecordSelectSQL+`
WHERE commit_hash = ? AND from_symbol_id = ?
ORDER BY to_symbol_id, kind`, commit, symbolID)
}

func queryRefRecordsTo(ctx context.Context, db *sql.DB, commit, symbolID string) ([]refRecord, error) {
	return queryRefRecords(ctx, db, refRecordSelectSQL+`
WHERE commit_hash = ? AND to_symbol_id = ?
ORDER BY from_symbol_id, kind`, commit, symbolID)
}

func queryRefRecordsInFile(ctx context.Context, db *sql.DB, commit, fileID string) ([]refRecord, error) {
	return queryRefRecords(ctx, db, refRecordSelectSQL+`
WHERE commit_hash = ? AND file_id = ?
ORDER BY start_offset, end_offset`, commit, fileID)
}

func queryRefRecordsInFileRange(ctx context.Context, db *sql.DB, commit, fileID string, startOffset, endOffset int) ([]refRecord, error) {
	return queryRefRecords(ctx, db, refRecordSelectSQL+`
WHERE commit_hash = ?
  AND file_id = ?
  AND start_offset >= ?
  AND end_offset <= ?
ORDER BY start_offset, end_offset`, commit, fileID, startOffset, endOffset)
}

func queryRefRecordsToFileSymbols(ctx context.Context, db *sql.DB, commit, fileID string) ([]refRecord, error) {
	return queryRefRecords(ctx, db, refRecordSelectSQLWithAlias+`
JOIN snapshot_symbols target
  ON target.commit_hash = r.commit_hash
 AND target.id = r.to_symbol_id
LEFT JOIN snapshot_symbols source
  ON source.commit_hash = r.commit_hash
 AND source.id = r.from_symbol_id
WHERE r.commit_hash = ?
  AND target.file_id = ?
  AND COALESCE(source.file_id, '') != ?
ORDER BY r.from_symbol_id, r.kind`, commit, fileID, fileID)
}

func queryRefRecordsFromFileSymbols(ctx context.Context, db *sql.DB, commit, fileID string) ([]refRecord, error) {
	return queryRefRecords(ctx, db, refRecordSelectSQLWithAlias+`
JOIN snapshot_symbols source
  ON source.commit_hash = r.commit_hash
 AND source.id = r.from_symbol_id
LEFT JOIN snapshot_symbols target
  ON target.commit_hash = r.commit_hash
 AND target.id = r.to_symbol_id
WHERE r.commit_hash = ?
  AND source.file_id = ?
  AND COALESCE(target.file_id, '') != ?
ORDER BY r.to_symbol_id, r.kind`, commit, fileID, fileID)
}

func queryRefRecords(ctx context.Context, db *sql.DB, query string, args ...any) ([]refRecord, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []refRecord{}
	for rows.Next() {
		var ref refRecord
		if err := rows.Scan(
			&ref.FromSymbolID,
			&ref.ToSymbolID,
			&ref.Kind,
			&ref.FileID,
			&ref.Range.StartLine,
			&ref.Range.StartCol,
			&ref.Range.EndLine,
			&ref.Range.EndCol,
			&ref.Range.StartOffset,
			&ref.Range.EndOffset,
		); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

const refRecordSelectSQL = `
SELECT from_symbol_id,
       to_symbol_id,
       kind,
       file_id,
       start_line,
       start_col,
       end_line,
       end_col,
       start_offset,
       end_offset
FROM snapshot_refs
`

const refRecordSelectSQLWithAlias = `
SELECT r.from_symbol_id,
       r.to_symbol_id,
       r.kind,
       r.file_id,
       r.start_line,
       r.start_col,
       r.end_line,
       r.end_col,
       r.start_offset,
       r.end_offset
FROM snapshot_refs r
`

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
