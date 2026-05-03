package review

import (
	"context"
	"database/sql"
	"fmt"
)

// IntegrityReport summarizes consistency checks for a review/history database.
type IntegrityReport struct {
	SchemaVersions       map[string]string
	BadSymbolFileJoins   int
	BadRefFileJoins      int
	BadRefFromSymbolJoins int
}

// HasFailures reports whether any integrity check found inconsistent rows.
func (r IntegrityReport) HasFailures() bool {
	return r.BadSymbolFileJoins > 0 || r.BadRefFileJoins > 0 || r.BadRefFromSymbolJoins > 0
}

// ValidateIntegrity checks invariants that must hold for static review browser
// routes. These checks intentionally query the public snapshot_* views because
// that is the shape consumed by review rendering and parts of the browser.
func ValidateIntegrity(ctx context.Context, db *sql.DB) (*IntegrityReport, error) {
	report := &IntegrityReport{SchemaVersions: map[string]string{}}

	rows, err := db.QueryContext(ctx, `SELECT key, value FROM schema_info ORDER BY key`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var key, value string
			if err := rows.Scan(&key, &value); err != nil {
				return nil, fmt.Errorf("scan schema_info: %w", err)
			}
			report.SchemaVersions[key] = value
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("schema_info rows: %w", err)
		}
	}

	checks := []struct {
		name string
		dest *int
		sql  string
	}{
		{
			name: "symbol/file joins",
			dest: &report.BadSymbolFileJoins,
			sql: `
SELECT COUNT(*)
FROM snapshot_symbols s
LEFT JOIN snapshot_files f
  ON f.commit_hash = s.commit_hash
 AND f.id = s.file_id
WHERE f.id IS NULL`,
		},
		{
			name: "ref/file joins",
			dest: &report.BadRefFileJoins,
			sql: `
SELECT COUNT(*)
FROM snapshot_refs r
LEFT JOIN snapshot_files f
  ON f.commit_hash = r.commit_hash
 AND f.id = r.file_id
WHERE f.id IS NULL`,
		},
		{
			name: "ref/from-symbol joins",
			dest: &report.BadRefFromSymbolJoins,
			sql: `
SELECT COUNT(*)
FROM snapshot_refs r
LEFT JOIN snapshot_symbols s
  ON s.commit_hash = r.commit_hash
 AND s.id = r.from_symbol_id
WHERE s.id IS NULL`,
		},
	}

	for _, check := range checks {
		if err := db.QueryRowContext(ctx, check.sql).Scan(check.dest); err != nil {
			return nil, fmt.Errorf("check %s: %w", check.name, err)
		}
	}
	return report, nil
}
