package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/wesen/codebase-browser/internal/gitutil"
	"github.com/wesen/codebase-browser/internal/indexer"
)

func TestLoadSnapshotRefVersionsIncludeLocations(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := Create(filepath.Join(root, "history.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	idx1 := refVersionFixtureIndex(10, 11)
	idx2 := refVersionFixtureIndex(20, 21)
	if err := store.LoadSnapshot(ctx, gitutil.Commit{Hash: "1111111111111111111111111111111111111111", ShortHash: "1111111", AuthorTime: time.Unix(1, 0), Sequence: 1}, idx1, ""); err != nil {
		t.Fatalf("load first snapshot: %v", err)
	}
	if err := store.LoadSnapshot(ctx, gitutil.Commit{Hash: "2222222222222222222222222222222222222222", ShortHash: "2222222", AuthorTime: time.Unix(2, 0), Sequence: 2}, idx2, ""); err != nil {
		t.Fatalf("load second snapshot: %v", err)
	}

	var versions int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ref_versions`).Scan(&versions); err != nil {
		t.Fatalf("count ref versions: %v", err)
	}
	if versions != 2 {
		t.Fatalf("ref_versions count = %d, want 2", versions)
	}

	var startLine, endLine int
	if err := store.DB().QueryRowContext(ctx, `
		SELECT start_line, end_line
		FROM snapshot_refs
		WHERE commit_hash = '2222222222222222222222222222222222222222'
	`).Scan(&startLine, &endLine); err != nil {
		t.Fatalf("query second snapshot refs: %v", err)
	}
	if startLine != 20 || endLine != 21 {
		t.Fatalf("second snapshot ref range = %d-%d, want 20-21", startLine, endLine)
	}
}

func refVersionFixtureIndex(refStart, refEnd int) *indexer.Index {
	return &indexer.Index{
		Version: "1",
		Module:  "example.com/refs",
		Packages: []indexer.Package{{
			ID:         "pkg:example.com/refs",
			ImportPath: "example.com/refs",
			Name:       "refs",
			FileIDs:    []string{"file:refs.go"},
			SymbolIDs:  []string{"sym:example.com/refs.func.Caller", "sym:example.com/refs.func.Callee"},
		}},
		Files: []indexer.File{{
			ID:        "file:refs.go",
			Path:      "refs.go",
			PackageID: "pkg:example.com/refs",
			SHA256:    "same-content",
			Language:  "go",
		}},
		Symbols: []indexer.Symbol{
			{
				ID:        "sym:example.com/refs.func.Caller",
				Kind:      "func",
				Name:      "Caller",
				PackageID: "pkg:example.com/refs",
				FileID:    "file:refs.go",
				Range:     indexer.Range{StartLine: 1, EndLine: 3, StartOffset: 0, EndOffset: 10},
				Language:  "go",
			},
			{
				ID:        "sym:example.com/refs.func.Callee",
				Kind:      "func",
				Name:      "Callee",
				PackageID: "pkg:example.com/refs",
				FileID:    "file:refs.go",
				Range:     indexer.Range{StartLine: 5, EndLine: 7, StartOffset: 20, EndOffset: 30},
				Language:  "go",
			},
		},
		Refs: []indexer.Ref{{
			FromSymbolID: "sym:example.com/refs.func.Caller",
			ToSymbolID:   "sym:example.com/refs.func.Callee",
			Kind:         "call",
			FileID:       "file:refs.go",
			Range:        indexer.Range{StartLine: refStart, EndLine: refEnd, StartOffset: refStart * 10, EndOffset: refEnd * 10},
		}},
	}
}
