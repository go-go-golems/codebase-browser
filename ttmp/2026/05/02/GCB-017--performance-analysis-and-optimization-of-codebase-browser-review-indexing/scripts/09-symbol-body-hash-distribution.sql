-- 09-symbol-body-hash-distribution.sql
-- How many distinct versions (body hashes) does each symbol have across commits?
-- Symbols with 1 version never changed; those with many changed frequently.
-- Usage: sqlite3 -header -column <db> < scripts/09-symbol-body-hash-distribution.sql

.headers on
.mode column

-- Top 20 most-changed symbols (most distinct body hashes)
SELECT s.name, s.kind, COUNT(DISTINCT s.body_hash) as distinct_versions,
       COUNT(DISTINCT s.commit_hash) as commit_appearances
FROM snapshot_symbols s
WHERE s.body_hash != ''
GROUP BY s.id
ORDER BY distinct_versions DESC
LIMIT 20;
