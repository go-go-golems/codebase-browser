package review

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
)

// snapshotFS exposes indexed file_contents for a single commit as an fs.FS.
// This keeps markdown snippet rendering consistent with the symbol offsets in
// the selected snapshot instead of reading potentially changed files from the
// live working tree.
type snapshotFS struct {
	db         *sql.DB
	commitHash string
}

func (s snapshotFS) Open(name string) (fs.File, error) {
	clean := path.Clean(strings.TrimPrefix(name, "/"))
	if clean == "." || strings.HasPrefix(clean, "../") {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	var content []byte
	err := s.db.QueryRowContext(context.Background(), `
SELECT fc.content
FROM snapshot_files f
JOIN file_contents fc ON fc.content_hash = f.sha256
WHERE f.commit_hash = ? AND f.path = ?
`, s.commitHash, clean).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return &snapshotFile{name: path.Base(clean), Reader: bytes.NewReader(content), size: int64(len(content))}, nil
}

type snapshotFile struct {
	name string
	*bytes.Reader
	size int64
}

func (f *snapshotFile) Close() error { return nil }

func (f *snapshotFile) Stat() (fs.FileInfo, error) {
	return snapshotFileInfo{name: f.name, size: f.size}, nil
}

func (f *snapshotFile) Read(p []byte) (int, error) {
	if f.Reader == nil {
		return 0, io.ErrClosedPipe
	}
	return f.Reader.Read(p)
}

type snapshotFileInfo struct {
	name string
	size int64
}

func (i snapshotFileInfo) Name() string       { return i.name }
func (i snapshotFileInfo) Size() int64        { return i.size }
func (i snapshotFileInfo) Mode() fs.FileMode  { return 0o444 }
func (i snapshotFileInfo) ModTime() time.Time { return time.Time{} }
func (i snapshotFileInfo) IsDir() bool        { return false }
func (i snapshotFileInfo) Sys() any           { return nil }

func latestCommitHash(ctx context.Context, db *sql.DB) (string, error) {
	var hash string
	if err := db.QueryRowContext(ctx, `SELECT hash FROM commits ORDER BY author_time DESC LIMIT 1`).Scan(&hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no commits in review database")
		}
		return "", err
	}
	return hash, nil
}
