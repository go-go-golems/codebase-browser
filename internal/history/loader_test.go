package history

import (
	"context"
	"os"
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

func TestLoadSnapshotPreservesMovedSymbolWithSameBody(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := Create(filepath.Join(root, "history.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	worktree := filepath.Join(root, "worktree")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	body := []byte("package moved\n\nfunc Stable() {}\n")

	if err := os.WriteFile(filepath.Join(worktree, "old.go"), body, 0o644); err != nil {
		t.Fatalf("write old file: %v", err)
	}
	idx1 := movedSymbolFixtureIndex("file:old.go", "old.go", "sha-old", len(body))
	if err := store.LoadSnapshot(ctx, gitutil.Commit{Hash: "1111111111111111111111111111111111111111", ShortHash: "1111111", AuthorTime: time.Unix(1, 0), Sequence: 1}, idx1, worktree); err != nil {
		t.Fatalf("load old snapshot: %v", err)
	}

	if err := os.Remove(filepath.Join(worktree, "old.go")); err != nil {
		t.Fatalf("remove old file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "new.go"), body, 0o644); err != nil {
		t.Fatalf("write new file: %v", err)
	}
	idx2 := movedSymbolFixtureIndex("file:new.go", "new.go", "sha-new", len(body))
	if err := store.LoadSnapshot(ctx, gitutil.Commit{Hash: "2222222222222222222222222222222222222222", ShortHash: "2222222", AuthorTime: time.Unix(2, 0), Sequence: 2}, idx2, worktree); err != nil {
		t.Fatalf("load new snapshot: %v", err)
	}

	var symbolVersions int
	if err := store.DB().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM symbols
		WHERE stable_id = 'sym:example.com/moved.func.Stable'
	`).Scan(&symbolVersions); err != nil {
		t.Fatalf("count symbol versions: %v", err)
	}
	if symbolVersions != 2 {
		t.Fatalf("symbol version count = %d, want 2", symbolVersions)
	}

	for _, tc := range []struct {
		commit string
		fileID string
	}{
		{commit: "1111111111111111111111111111111111111111", fileID: "file:old.go"},
		{commit: "2222222222222222222222222222222222222222", fileID: "file:new.go"},
	} {
		var got string
		if err := store.DB().QueryRowContext(ctx, `
			SELECT file_id
			FROM snapshot_symbols
			WHERE commit_hash = ? AND id = 'sym:example.com/moved.func.Stable'
		`, tc.commit).Scan(&got); err != nil {
			t.Fatalf("query snapshot symbol for %s: %v", tc.commit[:7], err)
		}
		if got != tc.fileID {
			t.Fatalf("snapshot file for %s = %s, want %s", tc.commit[:7], got, tc.fileID)
		}

		var joined int
		if err := store.DB().QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM snapshot_symbols s
			JOIN snapshot_files f ON f.commit_hash = s.commit_hash AND f.id = s.file_id
			WHERE s.commit_hash = ? AND s.id = 'sym:example.com/moved.func.Stable'
		`, tc.commit).Scan(&joined); err != nil {
			t.Fatalf("query joined snapshot for %s: %v", tc.commit[:7], err)
		}
		if joined != 1 {
			t.Fatalf("joined symbol/file rows for %s = %d, want 1", tc.commit[:7], joined)
		}
	}
}

func movedSymbolFixtureIndex(fileID, path, sha string, bodyLen int) *indexer.Index {
	return &indexer.Index{
		Version: "1",
		Module:  "example.com/moved",
		Packages: []indexer.Package{{
			ID:         "pkg:example.com/moved",
			ImportPath: "example.com/moved",
			Name:       "moved",
			FileIDs:    []string{fileID},
			SymbolIDs:  []string{"sym:example.com/moved.func.Stable"},
		}},
		Files: []indexer.File{{
			ID:        fileID,
			Path:      path,
			PackageID: "pkg:example.com/moved",
			SHA256:    sha,
			Language:  "go",
		}},
		Symbols: []indexer.Symbol{{
			ID:        "sym:example.com/moved.func.Stable",
			Kind:      "func",
			Name:      "Stable",
			PackageID: "pkg:example.com/moved",
			FileID:    fileID,
			Range:     indexer.Range{StartLine: 3, EndLine: 3, StartOffset: 15, EndOffset: bodyLen},
			Language:  "go",
		}},
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
