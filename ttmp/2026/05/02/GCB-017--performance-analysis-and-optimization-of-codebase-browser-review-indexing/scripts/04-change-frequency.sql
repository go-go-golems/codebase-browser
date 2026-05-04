-- 04-change-frequency.sql
-- How many commits does each symbol appear in?
-- High-frequency symbols are stable; low-frequency ones changed rarely.
-- Usage: sqlite3 -header -column <db> < scripts/04-change-frequency.sql

.headers on
.mode column

-- Distribution: for each commit_count, how many symbols have that count
SELECT commit_count, COUNT(*) as symbols_with_that_count
FROM (
    SELECT id, COUNT(DISTINCT commit_hash) as commit_count
    FROM snapshot_symbols
    GROUP BY id
)
GROUP BY commit_count
ORDER BY commit_count
LIMIT 30;
