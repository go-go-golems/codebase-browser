package history

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store owns a SQLite connection for the git-aware history database.
type Store struct {
	db *sql.DB
}

// Open opens an existing history database.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open history db: %w", err)
	}
	if err := configure(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// NewFromDB creates a Store from an existing *sql.DB connection.
// The caller retains ownership of the DB and is responsible for closing it.
// This is used when the history tables live in a shared database (e.g. a review DB).
func NewFromDB(db *sql.DB) (*Store, error) {
	if err := configure(db); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Create opens path, drops any existing tables, and recreates the schema.
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

// DB exposes the underlying database for direct queries.
func (s *Store) DB() *sql.DB { return s.db }

// Close checkpoints WAL state before closing so a copied *.db file is
// self-contained. This matters for published container images that include a
// generated history.db without copying sidecar -wal/-shm files.
func (s *Store) Close() error {
	_, _ = s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`)
	return s.db.Close()
}

// ResetSchema drops and recreates the history database schema.
func (s *Store) ResetSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, DropSchemaSQL); err != nil {
		return fmt.Errorf("drop history schema: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, CreateSchemaSQL); err != nil {
		return fmt.Errorf("create history schema: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, CreateViewsSQL); err != nil {
		return fmt.Errorf("create history views: %w", err)
	}
	return nil
}

// HasCommit checks if a commit has already been indexed.
func (s *Store) HasCommit(ctx context.Context, hash string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(1) FROM commits WHERE hash = ? AND error = ''",
		hash,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// MaxSequence returns the highest explicit commit sequence currently stored.
func (s *Store) MaxSequence(ctx context.Context) (int, error) {
	var max sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(sequence) FROM commits WHERE error = ''`).Scan(&max); err != nil {
		return 0, err
	}
	if !max.Valid {
		return 0, nil
	}
	return int(max.Int64), nil
}

// GetCommit retrieves a single commit by hash.
func (s *Store) GetCommit(ctx context.Context, hash string) (*CommitRow, error) {
	row := &CommitRow{}
	err := s.db.QueryRowContext(ctx, `
SELECT hash, short_hash, message, author_name, author_email,
       author_time, indexed_at, sequence, branch, error
FROM   commits
WHERE  hash = ?`, hash).Scan(
		&row.Hash, &row.ShortHash, &row.Message, &row.AuthorName,
		&row.AuthorEmail, &row.AuthorTime, &row.IndexedAt, &row.Sequence, &row.Branch, &row.Error,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

// ListCommits returns all indexed commits ordered by explicit sequence descending.
func (s *Store) ListCommits(ctx context.Context) ([]CommitRow, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT hash, short_hash, message, author_name, author_email,
       author_time, indexed_at, sequence, branch, error
FROM   commits
ORDER BY sequence DESC, author_time DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []CommitRow
	for rows.Next() {
		var r CommitRow
		if err := rows.Scan(
			&r.Hash, &r.ShortHash, &r.Message, &r.AuthorName,
			&r.AuthorEmail, &r.AuthorTime, &r.IndexedAt, &r.Sequence, &r.Branch, &r.Error,
		); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// SymbolCountAtCommit returns the number of symbols indexed for a commit.
func (s *Store) SymbolCountAtCommit(ctx context.Context, hash string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(1) FROM snapshot_symbols WHERE commit_hash = ?",
		hash,
	).Scan(&count)
	return count, err
}

// CommitRow is the Go representation of a commits table row.
type CommitRow struct {
	Hash        string
	ShortHash   string
	Message     string
	AuthorName  string
	AuthorEmail string
	AuthorTime  int64
	IndexedAt   int64
	Sequence    int
	Branch      string
	Error       string
}

func (r *CommitRow) AuthorTimeTime() time.Time {
	return time.Unix(r.AuthorTime, 0)
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
