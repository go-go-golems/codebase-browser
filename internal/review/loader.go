package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/wesen/codebase-browser/internal/browser"
	"github.com/wesen/codebase-browser/internal/indexer"
)

// LoadLatestSnapshot reconstructs a *browser.Loaded from the most recent
// commit's snapshot data in the review database.
func LoadLatestSnapshot(ctx context.Context, store *Store) (*browser.Loaded, error) {
	hash, err := LatestCommitHash(ctx, store.DB())
	if err != nil {
		return nil, fmt.Errorf("find latest commit: %w", err)
	}

	idx, err := loadSnapshotIndex(ctx, store.DB(), hash)
	if err != nil {
		return nil, fmt.Errorf("load snapshot index: %w", err)
	}

	return browser.LoadIndex(idx), nil
}

// loadSnapshotIndex rebuilds an *indexer.Index from snapshot_* tables for a single commit.
func loadSnapshotIndex(ctx context.Context, db *sql.DB, commitHash string) (*indexer.Index, error) {
	idx := &indexer.Index{Version: "review-snapshot"}

	// Load packages
	pkgRows, err := db.QueryContext(ctx, `
		SELECT id, import_path, name, doc, language
		FROM snapshot_packages WHERE commit_hash = ?`, commitHash)
	if err != nil {
		return nil, fmt.Errorf("query packages: %w", err)
	}
	defer pkgRows.Close()
	for pkgRows.Next() {
		var p indexer.Package
		var lang string
		if err := pkgRows.Scan(&p.ID, &p.ImportPath, &p.Name, &p.Doc, &lang); err != nil {
			return nil, fmt.Errorf("scan package: %w", err)
		}
		p.Language = lang
		idx.Packages = append(idx.Packages, p)
	}
	if err := pkgRows.Err(); err != nil {
		return nil, fmt.Errorf("package rows: %w", err)
	}

	// Load files
	fileRows, err := db.QueryContext(ctx, `
		SELECT id, path, package_id, size, line_count, sha256, language, build_tags_json
		FROM snapshot_files WHERE commit_hash = ?`, commitHash)
	if err != nil {
		return nil, fmt.Errorf("query files: %w", err)
	}
	defer fileRows.Close()
	for fileRows.Next() {
		var f indexer.File
		var lang, buildTagsJSON string
		if err := fileRows.Scan(&f.ID, &f.Path, &f.PackageID, &f.Size, &f.LineCount,
			&f.SHA256, &lang, &buildTagsJSON); err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}
		f.Language = lang
		json.Unmarshal([]byte(buildTagsJSON), &f.BuildTags)
		idx.Files = append(idx.Files, f)
	}
	if err := fileRows.Err(); err != nil {
		return nil, fmt.Errorf("file rows: %w", err)
	}

	// Load symbols
	symRows, err := db.QueryContext(ctx, `
		SELECT id, kind, name, package_id, file_id,
			start_line, start_col, end_line, end_col, start_offset, end_offset,
			doc, signature, receiver_type, receiver_pointer, exported, language,
			type_params_json, tags_json
		FROM snapshot_symbols WHERE commit_hash = ?`, commitHash)
	if err != nil {
		return nil, fmt.Errorf("query symbols: %w", err)
	}
	defer symRows.Close()
	for symRows.Next() {
		var s indexer.Symbol
		var lang, typeParamsJSON, tagsJSON string
		var receiverType string
		var receiverPointer, exported int
		if err := symRows.Scan(
			&s.ID, &s.Kind, &s.Name, &s.PackageID, &s.FileID,
			&s.Range.StartLine, &s.Range.StartCol, &s.Range.EndLine, &s.Range.EndCol,
			&s.Range.StartOffset, &s.Range.EndOffset,
			&s.Doc, &s.Signature, &receiverType, &receiverPointer, &exported, &lang,
			&typeParamsJSON, &tagsJSON); err != nil {
			return nil, fmt.Errorf("scan symbol: %w", err)
		}
		s.Language = lang
		s.Exported = exported != 0
		if receiverType != "" {
			s.Receiver = &indexer.Receiver{
				TypeName: receiverType,
				Pointer:  receiverPointer != 0,
			}
		}
		json.Unmarshal([]byte(typeParamsJSON), &s.TypeParams)
		json.Unmarshal([]byte(tagsJSON), &s.Tags)
		idx.Symbols = append(idx.Symbols, s)
	}
	if err := symRows.Err(); err != nil {
		return nil, fmt.Errorf("symbol rows: %w", err)
	}

	// Load refs
	refRows, err := db.QueryContext(ctx, `
		SELECT from_symbol_id, to_symbol_id, kind, file_id,
			start_line, start_col, end_line, end_col, start_offset, end_offset
		FROM snapshot_refs WHERE commit_hash = ?`, commitHash)
	if err != nil {
		return nil, fmt.Errorf("query refs: %w", err)
	}
	defer refRows.Close()
	for refRows.Next() {
		var r indexer.Ref
		if err := refRows.Scan(
			&r.FromSymbolID, &r.ToSymbolID, &r.Kind, &r.FileID,
			&r.Range.StartLine, &r.Range.StartCol, &r.Range.EndLine, &r.Range.EndCol,
			&r.Range.StartOffset, &r.Range.EndOffset); err != nil {
			return nil, fmt.Errorf("scan ref: %w", err)
		}
		idx.Refs = append(idx.Refs, r)
	}
	if err := refRows.Err(); err != nil {
		return nil, fmt.Errorf("ref rows: %w", err)
	}

	return idx, nil
}
