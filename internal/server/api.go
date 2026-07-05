package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
)

type healthResponse struct {
	OK        bool   `json:"ok"`
	Mode      string `json:"mode"`
	StaticDir string `json:"staticDir,omitempty"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, healthResponse{OK: true, Mode: "live-go", StaticDir: s.StaticDir})
}

type indexResponse struct {
	Version     string       `json:"version"`
	GeneratedAt string       `json:"generatedAt"`
	Module      string       `json:"module"`
	GoVersion   string       `json:"goVersion"`
	Commit      string       `json:"commit"`
	Packages    []packageRow `json:"packages"`
	Files       []fileRow    `json:"files"`
	Symbols     []symbolRow  `json:"symbols"`
}

type packageRow struct {
	ID         string   `json:"id"`
	ImportPath string   `json:"importPath"`
	Name       string   `json:"name"`
	Doc        string   `json:"doc"`
	Language   string   `json:"language"`
	FileIDs    []string `json:"fileIds"`
	SymbolIDs  []string `json:"symbolIds"`
}

type fileRow struct {
	ID            string   `json:"id"`
	Path          string   `json:"path"`
	PackageID     string   `json:"packageId"`
	Size          int      `json:"size"`
	LineCount     int      `json:"lineCount"`
	SHA256        string   `json:"sha256"`
	Language      string   `json:"language"`
	BuildTagsJSON string   `json:"buildTagsJson,omitempty"`
	BuildTags     []string `json:"buildTags,omitempty"`
}

type symbolRow struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	PackageID       string `json:"packageId"`
	FileID          string `json:"fileId"`
	StartLine       int    `json:"startLine"`
	StartCol        int    `json:"startCol"`
	EndLine         int    `json:"endLine"`
	EndCol          int    `json:"endCol"`
	StartOffset     int    `json:"startOffset"`
	EndOffset       int    `json:"endOffset"`
	Doc             string `json:"doc"`
	Signature       string `json:"signature"`
	ReceiverType    string `json:"receiverType"`
	ReceiverPointer bool   `json:"receiverPointer"`
	Exported        bool   `json:"exported"`
	Language        string `json:"language"`
	BodyHash        string `json:"bodyHash"`
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	commit, err := s.resolveCommit(r.URL.Query().Get("commit"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	packages, err := queryPackages(r.Context(), db, commit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	files, err := queryFiles(r.Context(), db, commit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	symbols, err := querySymbols(r.Context(), db, commit, "", "", 0)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkgByID := map[string]*packageRow{}
	for i := range packages {
		pkgByID[packages[i].ID] = &packages[i]
	}
	for _, file := range files {
		if pkg := pkgByID[file.PackageID]; pkg != nil {
			pkg.FileIDs = append(pkg.FileIDs, file.ID)
		}
	}
	for _, sym := range symbols {
		if pkg := pkgByID[sym.PackageID]; pkg != nil {
			pkg.SymbolIDs = append(pkg.SymbolIDs, sym.ID)
		}
	}
	writeJSON(w, indexResponse{Version: "live-go", Commit: commit, Packages: packages, Files: files, Symbols: symbols})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	commit, err := s.resolveCommit(r.URL.Query().Get("commit"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	limit := 100
	symbols, err := querySymbols(r.Context(), db, commit, r.URL.Query().Get("q"), r.URL.Query().Get("kind"), limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, symbols)
}

func (s *Server) handleSymbol(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing id")
		return
	}
	commit, err := s.resolveCommit(r.URL.Query().Get("commit"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	rows, err := querySymbols(r.Context(), db, commit, "", "", 0, "id = ?", id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(rows) == 0 {
		writeJSONError(w, http.StatusNotFound, "symbol not found")
		return
	}
	writeJSON(w, rows[0])
}

func (s *Server) handleSource(w http.ResponseWriter, r *http.Request) {
	db, ok := s.requireDB(w)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" || strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
		writeJSONError(w, http.StatusBadRequest, "bad or missing path")
		return
	}
	commit, err := s.resolveCommit(r.URL.Query().Get("commit"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	content, err := fileContentByPath(r.Context(), db, commit, path)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(content)
}

func (s *Server) handleSnippet(w http.ResponseWriter, r *http.Request) {
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
	body, err := snippetBySymbol(r.Context(), db, commit, id, r.URL.Query().Get("kind"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(body))
}

func (s *Server) resolveCommit(ref string) (string, error) {
	if s.DB == nil {
		return "", fmt.Errorf("database is not configured")
	}
	if ref == "" || ref == "HEAD" {
		var hash string
		err := s.DB.QueryRow(`SELECT hash FROM commits WHERE error = '' ORDER BY author_time DESC LIMIT 1`).Scan(&hash)
		if err != nil {
			return "", fmt.Errorf("resolve HEAD: %w", err)
		}
		return hash, nil
	}
	var hash string
	err := s.DB.QueryRow(`SELECT hash FROM commits WHERE hash = ? OR short_hash = ? OR hash LIKE ? ORDER BY length(hash) LIMIT 1`, ref, ref, ref+"%").Scan(&hash)
	if err != nil {
		return "", fmt.Errorf("commit ref not found: %s", ref)
	}
	return hash, nil
}

func queryPackages(ctx context.Context, db *sql.DB, commit string) ([]packageRow, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, import_path, name, doc, language
FROM snapshot_packages
WHERE commit_hash = ?
ORDER BY import_path`, commit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []packageRow{}
	for rows.Next() {
		var r packageRow
		if err := rows.Scan(&r.ID, &r.ImportPath, &r.Name, &r.Doc, &r.Language); err != nil {
			return nil, err
		}
		r.FileIDs = []string{}
		r.SymbolIDs = []string{}
		out = append(out, r)
	}
	return out, rows.Err()
}

func queryFiles(ctx context.Context, db *sql.DB, commit string) ([]fileRow, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, path, package_id, size, line_count, sha256, language, build_tags_json
FROM snapshot_files
WHERE commit_hash = ?
ORDER BY path`, commit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []fileRow{}
	for rows.Next() {
		var r fileRow
		if err := rows.Scan(&r.ID, &r.Path, &r.PackageID, &r.Size, &r.LineCount, &r.SHA256, &r.Language, &r.BuildTagsJSON); err != nil {
			return nil, err
		}
		r.BuildTags = []string{}
		out = append(out, r)
	}
	return out, rows.Err()
}

func querySymbols(ctx context.Context, db *sql.DB, commit, q, kind string, limit int, extra ...any) ([]symbolRow, error) {
	where := []string{"commit_hash = ?"}
	args := []any{commit}
	if kind != "" {
		where = append(where, "kind = ?")
		args = append(args, kind)
	}
	if q != "" {
		like := "%" + q + "%"
		where = append(where, "(name LIKE ? OR id LIKE ? OR signature LIKE ?)")
		args = append(args, like, like, like)
	}
	if len(extra) == 2 {
		where = append(where, extra[0].(string))
		args = append(args, extra[1])
	}
	query := `
SELECT id, kind, name, package_id, file_id, start_line, start_col, end_line, end_col,
       start_offset, end_offset, doc, signature, receiver_type, receiver_pointer,
       exported, language, body_hash
FROM snapshot_symbols
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY name COLLATE NOCASE, id`
	if limit > 0 {
		query += "\nLIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []symbolRow{}
	for rows.Next() {
		var r symbolRow
		var receiverPointer, exported int
		if err := rows.Scan(&r.ID, &r.Kind, &r.Name, &r.PackageID, &r.FileID, &r.StartLine, &r.StartCol, &r.EndLine, &r.EndCol, &r.StartOffset, &r.EndOffset, &r.Doc, &r.Signature, &r.ReceiverType, &receiverPointer, &exported, &r.Language, &r.BodyHash); err != nil {
			return nil, err
		}
		r.ReceiverPointer = receiverPointer != 0
		r.Exported = exported != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

func fileContentByPath(ctx context.Context, db *sql.DB, commit, path string) ([]byte, error) {
	var content []byte
	err := db.QueryRowContext(ctx, `
SELECT fc.content
FROM snapshot_files f
JOIN file_contents fc ON fc.content_hash = COALESCE(NULLIF(f.content_hash, ''), f.sha256)
WHERE f.commit_hash = ? AND f.path = ?`, commit, path).Scan(&content)
	if err == nil {
		return content, nil
	}
	return nil, fmt.Errorf("source file not found: %s", path)
}

func snippetBySymbol(ctx context.Context, db *sql.DB, commit, symbolID, kind string) (string, error) {
	var name, signature, filePath string
	var start, end int
	err := db.QueryRowContext(ctx, `
SELECT s.name, s.signature, f.path, s.start_offset, s.end_offset
FROM snapshot_symbols s
JOIN snapshot_files f ON f.commit_hash = s.commit_hash AND f.id = s.file_id
WHERE s.commit_hash = ? AND s.id = ?`, commit, symbolID).Scan(&name, &signature, &filePath, &start, &end)
	if err != nil {
		return "", fmt.Errorf("symbol not found: %s", symbolID)
	}
	if kind == "signature" {
		if signature != "" {
			return signature, nil
		}
		return name, nil
	}
	content, err := fileContentByPath(ctx, db, commit, filePath)
	if err != nil {
		return "", err
	}
	if start < 0 || end < start || end > len(content) {
		sum := sha256.Sum256(content)
		return "", fmt.Errorf("invalid symbol byte range %d..%d for %s (%x)", start, end, filePath, sum[:4])
	}
	return string(content[start:end]), nil
}
