package review

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/wesen/codebase-browser/internal/history"
)

// Store owns a SQLite connection for the unified review database.
// It wraps the history store (which shares the same DB connection)
// and adds review-specific tables (review_docs, review_doc_snippets).
type Store struct {
	db      *sql.DB
	History *history.Store
}

// Open opens an existing review database.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open review db: %w", err)
	}
	if err := configure(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	hist, err := history.NewFromDB(db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init history store: %w", err)
	}

	return &Store{db: db, History: hist}, nil
}

// Create opens path, drops any existing tables, and recreates the full schema.
func Create(path string) (*Store, error) {
	store, err := Open(path)
	if err != nil {
		return nil, err
	}
	if err := store.ResetSchema(context.Background()); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

// OpenOrCreate opens an existing database or creates a new one. Used by
// incremental indexing to reuse an existing database.
func OpenOrCreate(path string) (*Store, error) {
	store, err := Open(path)
	if err != nil {
		return nil, err
	}
	if !store.HasTables() {
		if err := store.ResetSchema(context.Background()); err != nil {
			_ = store.Close()
			return nil, err
		}
		return store, nil
	}
	if err := store.ValidateSchemaCompatibility(context.Background()); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

// ValidateSchemaCompatibility checks that an existing DB is safe to reuse for
// incremental indexing. Older review databases may have a commits table but not
// the normalized schema (sequence/schema_info/mapping tables) that incremental
// indexing and static exports now require.
func (s *Store) ValidateSchemaCompatibility(ctx context.Context) error {
	var problems []string
	for key, want := range map[string]string{
		"history_schema_version": "3",
		"review_schema_version":  "2",
	} {
		var got string
		if err := s.db.QueryRowContext(ctx, `SELECT value FROM schema_info WHERE key = ?`, key).Scan(&got); err != nil {
			problems = append(problems, fmt.Sprintf("missing schema_info[%s]", key))
		} else if got != want {
			problems = append(problems, fmt.Sprintf("schema_info[%s] = %q, want %q", key, got, want))
		}
	}
	for _, table := range []string{
		"commits", "packages", "files", "symbols", "ref_versions", "file_contents",
		"commit_packages", "commit_files", "commit_symbols", "commit_refs",
		"review_docs", "review_doc_snippets",
	} {
		if !s.hasTable(ctx, table) {
			problems = append(problems, "missing table "+table)
		}
	}
	for table, columns := range map[string][]string{
		"commits":             {"sequence"},
		"symbols":             {"file_id", "start_offset", "end_offset", "body_hash"},
		"ref_versions":        {"locations_json"},
		"review_doc_snippets": {"stub_id", "params_json", "commit_hash"},
	} {
		for _, column := range columns {
			if !s.hasColumn(ctx, table, column) {
				problems = append(problems, fmt.Sprintf("missing column %s.%s", table, column))
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("existing review database schema is incompatible with incremental indexing; recreate the database without --incremental (problems: %s)", strings.Join(problems, "; "))
	}
	return nil
}

// HasTables returns true if the database contains the expected tables.
func (s *Store) HasTables() bool {
	return s.hasTable(context.Background(), "commits")
}

func (s *Store) hasTable(ctx context.Context, name string) bool {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name = ?`, name).Scan(&count)
	return err == nil && count > 0
}

func (s *Store) hasColumn(ctx context.Context, table, column string) bool {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+quoteSQLiteIdent(table)+`)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

func quoteSQLiteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// HasCommits returns true if the database contains at least one indexed commit.
func (s *Store) HasCommits(ctx context.Context) (bool, error) {
	if !s.HasTables() {
		return false, nil
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM commits`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// DB exposes the underlying database for direct queries.
func (s *Store) DB() *sql.DB { return s.db }

// Close checkpoints WAL state and closes the connection.
// The caller should close the Store, not the history.Store directly,
// since both share the same *sql.DB.
func (s *Store) Close() error {
	_, _ = s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`)
	return s.db.Close()
}

// ResetSchema drops and recreates all tables (history + review).
func (s *Store) ResetSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, history.DropSchemaSQL); err != nil {
		return fmt.Errorf("drop history schema: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, history.CreateSchemaSQL); err != nil {
		return fmt.Errorf("create history schema: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, history.CreateViewsSQL); err != nil {
		return fmt.Errorf("create history views: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, dropReviewSchemaSQL); err != nil {
		return fmt.Errorf("drop review schema: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, createReviewSchemaSQL); err != nil {
		return fmt.Errorf("create review schema: %w", err)
	}
	return nil
}

func configure(db *sql.DB) error {
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		return fmt.Errorf("enable WAL: %w", err)
	}
	return nil
}
