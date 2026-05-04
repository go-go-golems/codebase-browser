package review

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateAndResetSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a fresh review database.
	store, err := Create(dbPath)
	if err != nil {
		t.Fatalf("create review db: %v", err)
	}
	defer store.Close()

	// Verify we can query the review_docs table.
	var count int
	if err := store.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM review_docs`).Scan(&count); err != nil {
		t.Fatalf("query review_docs: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 review docs, got %d", count)
	}

	// Verify history tables exist by querying commits.
	if err := store.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM commits`).Scan(&count); err != nil {
		t.Fatalf("query commits: %v", err)
	}

	// Verify we can open an existing database.
	store2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open review db: %v", err)
	}
	defer store2.Close()

	// Verify history store is accessible.
	if store2.History == nil {
		t.Fatal("expected history store to be initialized")
	}
	if store2.History.DB() == nil {
		t.Fatal("expected history DB to be non-nil")
	}
}

func TestResetSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Create(dbPath)
	if err != nil {
		t.Fatalf("create review db: %v", err)
	}
	defer store.Close()

	// Insert a dummy doc.
	_, err = store.DB().ExecContext(context.Background(),
		`INSERT INTO review_docs (slug, title, content) VALUES (?, ?, ?)`,
		"test", "Test", "hello")
	if err != nil {
		t.Fatalf("insert doc: %v", err)
	}

	// Reset schema.
	if err := store.ResetSchema(context.Background()); err != nil {
		t.Fatalf("reset schema: %v", err)
	}

	// Verify doc is gone.
	var count int
	if err := store.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM review_docs`).Scan(&count); err != nil {
		t.Fatalf("query review_docs after reset: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 review docs after reset, got %d", count)
	}
}

func TestOpenOrCreateInitializesEmptyDatabase(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "new.db")
	store, err := OpenOrCreate(dbPath)
	if err != nil {
		t.Fatalf("OpenOrCreate empty DB: %v", err)
	}
	defer store.Close()
	if err := store.ValidateSchemaCompatibility(ctx); err != nil {
		t.Fatalf("new DB schema should be compatible: %v", err)
	}
}

func TestOpenOrCreateRejectsLegacyCommitsTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if _, err := store.DB().ExecContext(context.Background(), `CREATE TABLE commits (hash TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create legacy commits table: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	store, err = OpenOrCreate(dbPath)
	if err == nil {
		_ = store.Close()
		t.Fatal("OpenOrCreate should reject incompatible legacy schema")
	}
	if !strings.Contains(err.Error(), "schema is incompatible") || !strings.Contains(err.Error(), "schema_info") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenNonexistent(t *testing.T) {
	tmpDir := t.TempDir()

	// sqlite3 creates the file if it doesn't exist, but the directory must exist.
	okDir := filepath.Join(tmpDir, "exists")
	if err := os.MkdirAll(okDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	okPath := filepath.Join(okDir, "test.db")
	store, err := Open(okPath)
	if err != nil {
		t.Fatalf("open new db: %v", err)
	}
	store.Close()

	// Nonexistent directory should fail.
	badPath := filepath.Join(tmpDir, "nonexistent_dir", "test.db")
	_, err = Open(badPath)
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestCloseIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Create(dbPath)
	if err != nil {
		t.Fatalf("create review db: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}

	// Second close may error but should not panic.
	_ = store.Close()
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Create(dbPath)
	if err != nil {
		t.Fatalf("create review db: %v", err)
	}
	store.Close()

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat db file: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty db file")
	}
}
