-- 03-symbol-kind-distribution.sql
-- Breakdown of symbols by kind
-- Usage: sqlite3 -header -column <db> < scripts/03-symbol-kind-distribution.sql

.headers on
.mode column

SELECT kind, COUNT(*) as cnt
FROM snapshot_symbols
GROUP BY kind
ORDER BY cnt DESC;
