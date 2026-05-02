package history

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wesen/codebase-browser/internal/gitutil"
	"github.com/wesen/codebase-browser/internal/indexer"
)

// LoadSnapshot bulk-loads a single commit's index into the normalized database.
// Each unique entity (package, file, symbol, ref set) is stored once; mapping
// tables record which version appears in which commit.
func (s *Store) LoadSnapshot(ctx context.Context, commit gitutil.Commit, idx *indexer.Index, worktreeDir string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin load tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Insert commit → get integer ID.
	now := time.Now().Unix()
	parentJSON, _ := json.Marshal(commit.ParentHashes)
	var commitID int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO commits(hash, short_hash, message, author_name, author_email,
                    author_time, parent_hashes, tree_hash, indexed_at, sequence)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id`,
		commit.Hash, commit.ShortHash, commit.Message,
		commit.AuthorName, commit.AuthorEmail,
		commit.AuthorTime.Unix(), string(parentJSON),
		commit.TreeHash, now, commit.Sequence,
	).Scan(&commitID)
	if err != nil {
		return fmt.Errorf("insert commit %s: %w", commit.ShortHash, err)
	}

	// Load packages.
	if err := loadPackages(ctx, tx, commitID, idx.Packages); err != nil {
		return err
	}

	// Load files.
	fileIDMap, err := loadFiles(ctx, tx, commitID, idx.Files)
	if err != nil {
		return err
	}

	// Load symbols.
	symbolIDMap, err := loadSymbols(ctx, tx, commitID, idx.Symbols, worktreeDir)
	if err != nil {
		return err
	}

	// Load refs.
	if err := loadRefs(ctx, tx, commitID, idx.Refs, symbolIDMap, fileIDMap); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit load tx: %w", err)
	}
	return nil
}

// loadPackages inserts packages (deduplicated by stable_id) and their commit mapping.
func loadPackages(ctx context.Context, tx *sql.Tx, commitID int64, packages []indexer.Package) error {
	pkgStmt, err := tx.PrepareContext(ctx, `
INSERT INTO packages(stable_id, import_path, name, doc, language)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(stable_id) DO NOTHING
RETURNING id`)
	if err != nil {
		return fmt.Errorf("prepare package insert: %w", err)
	}
	defer pkgStmt.Close()

	lookupStmt, err := tx.PrepareContext(ctx, `SELECT id FROM packages WHERE stable_id = ?`)
	if err != nil {
		return fmt.Errorf("prepare package lookup: %w", err)
	}
	defer lookupStmt.Close()

	mapStmt, err := tx.PrepareContext(ctx, `
INSERT OR IGNORE INTO commit_packages(commit_id, package_id) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare commit_packages insert: %w", err)
	}
	defer mapStmt.Close()

	for _, p := range packages {
		pkgID, err := upsertPkg(ctx, pkgStmt, lookupStmt, p)
		if err != nil {
			return fmt.Errorf("insert package %s: %w", p.ID, err)
		}
		if _, err := mapStmt.ExecContext(ctx, commitID, pkgID); err != nil {
			return fmt.Errorf("map package %s: %w", p.ID, err)
		}
	}
	return nil
}

// loadFiles inserts file versions (deduplicated by stable_id+sha256) and their commit mapping.
// Returns a map from file stable_id → integer file ID.
func loadFiles(ctx context.Context, tx *sql.Tx, commitID int64, files []indexer.File) (map[string]int64, error) {
	fileStmt, err := tx.PrepareContext(ctx, `
INSERT INTO files(stable_id, path, package_id, size, line_count, sha256, language, build_tags_json)
VALUES (?, ?, (SELECT id FROM packages WHERE stable_id = ?), ?, ?, ?, ?, ?)
ON CONFLICT(stable_id, sha256) DO NOTHING
RETURNING id`)
	if err != nil {
		return nil, fmt.Errorf("prepare file insert: %w", err)
	}
	defer fileStmt.Close()

	lookupStmt, err := tx.PrepareContext(ctx, `SELECT id FROM files WHERE stable_id = ? AND sha256 = ?`)
	if err != nil {
		return nil, fmt.Errorf("prepare file lookup: %w", err)
	}
	defer lookupStmt.Close()

	mapStmt, err := tx.PrepareContext(ctx, `
INSERT OR IGNORE INTO commit_files(commit_id, file_id) VALUES (?, ?)`)
	if err != nil {
		return nil, fmt.Errorf("prepare commit_files insert: %w", err)
	}
	defer mapStmt.Close()

	result := make(map[string]int64, len(files))
	for _, f := range files {
		buildTags, _ := json.Marshal(f.BuildTags)
		lang := f.Language
		if lang == "" {
			lang = "go"
		}
		var fileID int64
		err := fileStmt.QueryRowContext(ctx,
			f.ID, f.Path, f.PackageID,
			f.Size, f.LineCount, f.SHA256, lang, string(buildTags),
		).Scan(&fileID)
		if err == sql.ErrNoRows {
			err = lookupStmt.QueryRowContext(ctx, f.ID, f.SHA256).Scan(&fileID)
		}
		if err != nil {
			return nil, fmt.Errorf("insert/lookup file %s: %w", f.ID, err)
		}
		result[f.ID] = fileID
		if _, err := mapStmt.ExecContext(ctx, commitID, fileID); err != nil {
			return nil, fmt.Errorf("map file %s: %w", f.ID, err)
		}
	}
	return result, nil
}

// loadSymbols inserts symbol versions (deduplicated by stable_id+body_hash) and their commit mapping.
// Returns a map from symbol stable_id -> integer symbol ID.
func loadSymbols(ctx context.Context, tx *sql.Tx, commitID int64, symbols []indexer.Symbol, worktreeDir string) (map[string]int64, error) {
	// Insert symbols. Use a two-step approach: first look up the integer IDs for
	// package and file from the mapping tables we just populated, then insert.
	symStmt, err := tx.PrepareContext(ctx, `
INSERT INTO symbols(stable_id, kind, name, package_id, file_id,
    start_line, start_col, end_line, end_col, start_offset, end_offset,
    doc, signature, receiver_type, receiver_pointer, exported, language,
    type_params_json, tags_json, body_hash)
SELECT ?, ?, ?,
    (SELECT id FROM packages WHERE stable_id = ?),
    cf.file_id,
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
FROM commit_files cf
JOIN files f ON f.id = cf.file_id
WHERE cf.commit_id = ? AND f.stable_id = ?
LIMIT 1
ON CONFLICT(stable_id, body_hash) DO NOTHING
RETURNING id`)
	if err != nil {
		return nil, fmt.Errorf("prepare symbol insert: %w", err)
	}
	defer symStmt.Close()

	lookupStmt, err := tx.PrepareContext(ctx, `SELECT id FROM symbols WHERE stable_id = ? AND body_hash = ?`)
	if err != nil {
		return nil, fmt.Errorf("prepare symbol lookup: %w", err)
	}
	defer lookupStmt.Close()

	mapStmt, err := tx.PrepareContext(ctx, `
INSERT OR IGNORE INTO commit_symbols(commit_id, symbol_id) VALUES (?, ?)`)
	if err != nil {
		return nil, fmt.Errorf("prepare commit_symbols insert: %w", err)
	}
	defer mapStmt.Close()

	result := make(map[string]int64, len(symbols))
	seen := make(map[string]bool, len(symbols))
	for _, sym := range symbols {
		if seen[sym.ID] {
			continue
		}
		seen[sym.ID] = true

		lang := sym.Language
		if lang == "" {
			lang = "go"
		}
		receiverType := ""
		receiverPointer := 0
		if sym.Receiver != nil {
			receiverType = sym.Receiver.TypeName
			if sym.Receiver.Pointer {
				receiverPointer = 1
			}
		}
		typeParams, _ := json.Marshal(sym.TypeParams)
		tags, _ := json.Marshal(sym.Tags)

		bodyHash := computeBodyHash(worktreeDir, sym)

		var symID int64
		// Params: stable_id, kind, name, pkg_stable_id, then 15 data fields,
		// then commit_id, file_stable_id for the subquery.
		err := symStmt.QueryRowContext(ctx,
			sym.ID, sym.Kind, sym.Name, sym.PackageID,
			sym.Range.StartLine, sym.Range.StartCol, sym.Range.EndLine, sym.Range.EndCol,
			sym.Range.StartOffset, sym.Range.EndOffset,
			sym.Doc, sym.Signature, receiverType, receiverPointer,
			boolInt(sym.Exported), lang, string(typeParams), string(tags), bodyHash,
			commitID, sym.FileID,
		).Scan(&symID)
		if err == sql.ErrNoRows {
			err = lookupStmt.QueryRowContext(ctx, sym.ID, bodyHash).Scan(&symID)
		}
		if err != nil {
			return nil, fmt.Errorf("insert/lookup symbol %s: %w", sym.ID, err)
		}
		result[sym.ID] = symID
		if _, err := mapStmt.ExecContext(ctx, commitID, symID); err != nil {
			return nil, fmt.Errorf("map symbol %s: %w", sym.ID, err)
		}
	}
	return result, nil
}

// loadRefs groups refs by (from, to, kind, file), stores one ref_version per group
// with locations_json, and maps it to the commit.
func loadRefs(ctx context.Context, tx *sql.Tx, commitID int64, refs []indexer.Ref, symbolIDMap map[string]int64, fileIDMap map[string]int64) error {
	// Group refs by (from, to, kind, file).
	type refKey struct {
		fromSymID int64
		toStable  string
		kind      string
		fileID    int64
	}
	type location struct {
		StartLine   int `json:"start_line"`
		StartCol    int `json:"start_col"`
		EndLine     int `json:"end_line"`
		EndCol      int `json:"end_col"`
		StartOffset int `json:"start_offset"`
		EndOffset   int `json:"end_offset"`
	}
	groups := make(map[refKey][]location)
	for _, ref := range refs {
		fromID, ok := symbolIDMap[ref.FromSymbolID]
		if !ok {
			continue
		}
		fileID, ok := fileIDMap[ref.FileID]
		if !ok {
			continue
		}
		key := refKey{fromSymID: fromID, toStable: ref.ToSymbolID, kind: ref.Kind, fileID: fileID}
		groups[key] = append(groups[key], location{
			StartLine: ref.Range.StartLine, StartCol: ref.Range.StartCol,
			EndLine: ref.Range.EndLine, EndCol: ref.Range.EndCol,
			StartOffset: ref.Range.StartOffset, EndOffset: ref.Range.EndOffset,
		})
	}

	refStmt, err := tx.PrepareContext(ctx, `
INSERT INTO ref_versions(from_symbol_id, to_stable_id, kind, file_id, locations_json)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(from_symbol_id, to_stable_id, kind, file_id) DO NOTHING
RETURNING id`)
	if err != nil {
		return fmt.Errorf("prepare ref insert: %w", err)
	}
	defer refStmt.Close()

	lookupStmt, err := tx.PrepareContext(ctx, `
SELECT id FROM ref_versions
WHERE from_symbol_id = ? AND to_stable_id = ? AND kind = ? AND file_id = ?`)
	if err != nil {
		return fmt.Errorf("prepare ref lookup: %w", err)
	}
	defer lookupStmt.Close()

	mapStmt, err := tx.PrepareContext(ctx, `
INSERT OR IGNORE INTO commit_refs(commit_id, ref_version_id) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare commit_refs insert: %w", err)
	}
	defer mapStmt.Close()

	for key, locs := range groups {
		locsJSON, _ := json.Marshal(locs)
		var refID int64
		err := refStmt.QueryRowContext(ctx,
			key.fromSymID, key.toStable, key.kind, key.fileID, string(locsJSON),
		).Scan(&refID)
		if err == sql.ErrNoRows {
			err = lookupStmt.QueryRowContext(ctx,
				key.fromSymID, key.toStable, key.kind, key.fileID,
			).Scan(&refID)
		}
		if err != nil {
			return fmt.Errorf("insert/lookup ref (%d→%s %s): %w", key.fromSymID, key.toStable, key.kind, err)
		}
		if _, err := mapStmt.ExecContext(ctx, commitID, refID); err != nil {
			return fmt.Errorf("map ref: %w", err)
		}
	}
	return nil
}

// computeBodyHash reads the file from the worktree and hashes the byte range
// for the symbol body. Returns empty string on any error (non-fatal).
func computeBodyHash(worktreeDir string, sym indexer.Symbol) string {
	relPath := sym.FileID
	if len(relPath) > 5 && relPath[:5] == "file:" {
		relPath = relPath[5:]
	}
	absPath := filepath.Join(worktreeDir, relPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}
	start := sym.Range.StartOffset
	end := sym.Range.EndOffset
	if start < 0 || end > len(data) || start > end {
		return ""
	}
	body := data[start:end]
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// upsertPkg inserts a package or returns the existing ID.
func upsertPkg(ctx context.Context, insertStmt, lookupStmt *sql.Stmt, p indexer.Package) (int64, error) {
	var id int64
	err := insertStmt.QueryRowContext(ctx, p.ID, p.ImportPath, p.Name, p.Doc, p.Language).Scan(&id)
	if err == sql.ErrNoRows {
		err = lookupStmt.QueryRowContext(ctx, p.ID).Scan(&id)
	}
	return id, err
}
