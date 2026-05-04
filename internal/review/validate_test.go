package review

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/wesen/codebase-browser/internal/gitutil"
	"github.com/wesen/codebase-browser/internal/indexer"
)

func TestValidateIntegrityCleanDatabase(t *testing.T) {
	store := createIntegrityStore(t)
	defer store.Close()

	report, err := ValidateIntegrity(context.Background(), store.DB())
	if err != nil {
		t.Fatalf("validate integrity: %v", err)
	}
	if report.HasFailures() {
		t.Fatalf("expected clean report, got %+v", report)
	}
	if got := report.SchemaVersions["history_schema_version"]; got != "3" {
		t.Fatalf("history schema version = %q, want 3", got)
	}
}

func TestValidateIntegrityFindsBrokenSymbolFileJoin(t *testing.T) {
	store := createIntegrityStore(t)
	defer store.Close()

	if _, err := store.DB().ExecContext(context.Background(), `DELETE FROM commit_files`); err != nil {
		t.Fatalf("delete commit_files: %v", err)
	}

	report, err := ValidateIntegrity(context.Background(), store.DB())
	if err != nil {
		t.Fatalf("validate integrity: %v", err)
	}
	if report.BadSymbolFileJoins != 1 {
		t.Fatalf("bad symbol/file joins = %d, want 1", report.BadSymbolFileJoins)
	}
	if !report.HasFailures() {
		t.Fatalf("expected failures in report")
	}
}

func createIntegrityStore(t *testing.T) *Store {
	t.Helper()
	store, err := Create(filepath.Join(t.TempDir(), "review.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	idx := &indexer.Index{
		Version: "1",
		Module:  "example.com/validate",
		Packages: []indexer.Package{{
			ID:         "pkg:example.com/validate",
			ImportPath: "example.com/validate",
			Name:       "validate",
			FileIDs:    []string{"file:validate.go"},
			SymbolIDs:  []string{"sym:example.com/validate.func.Check"},
		}},
		Files: []indexer.File{{
			ID:        "file:validate.go",
			Path:      "validate.go",
			PackageID: "pkg:example.com/validate",
			SHA256:    "sha-validate",
			Language:  "go",
		}},
		Symbols: []indexer.Symbol{{
			ID:        "sym:example.com/validate.func.Check",
			Kind:      "func",
			Name:      "Check",
			PackageID: "pkg:example.com/validate",
			FileID:    "file:validate.go",
			Range:     indexer.Range{StartLine: 1, EndLine: 3, StartOffset: 0, EndOffset: 10},
			Language:  "go",
		}},
	}
	commit := gitutil.Commit{Hash: "1111111111111111111111111111111111111111", ShortHash: "1111111", AuthorTime: time.Unix(1, 0), Sequence: 1}
	if err := store.History.LoadSnapshot(context.Background(), commit, idx, ""); err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	return store
}
