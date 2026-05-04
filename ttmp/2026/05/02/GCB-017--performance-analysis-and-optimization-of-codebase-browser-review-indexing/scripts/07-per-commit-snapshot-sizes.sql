-- 07-per-commit-snapshot-sizes.sql
-- How many symbols/files/refs per commit? Identify bloat patterns.
-- Usage: sqlite3 -header -column <db> < scripts/07-per-commit-snapshot-sizes.sql

.headers on
.mode column

SELECT
    c.short_hash,
    substr(c.message, 1, 50) as message,
    (SELECT COUNT(*) FROM snapshot_symbols ss WHERE ss.commit_hash = c.hash) as symbols,
    (SELECT COUNT(*) FROM snapshot_files sf WHERE sf.commit_hash = c.hash) as files,
    (SELECT COUNT(*) FROM snapshot_refs sr WHERE sr.commit_hash = c.hash) as refs,
    (SELECT COUNT(*) FROM snapshot_packages sp WHERE sp.commit_hash = c.hash) as packages
FROM commits c
ORDER BY c.author_time DESC
LIMIT 30;
