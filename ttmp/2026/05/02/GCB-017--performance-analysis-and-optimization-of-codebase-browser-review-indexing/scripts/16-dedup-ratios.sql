-- 16-dedup-ratios.sql
-- Deduplication ratios for a review database
-- Usage: sqlite3 -header -column <db> < scripts/16-dedup-ratios.sql

.headers on
.mode column

SELECT 'symbols' as entity, COUNT(*) as total, COUNT(DISTINCT body_hash) as unique_versions,
       ROUND(100.0 * COUNT(DISTINCT body_hash) / NULLIF(COUNT(*), 0), 1) as pct_unique
FROM snapshot_symbols WHERE body_hash != ''
UNION ALL
SELECT 'files', COUNT(*), COUNT(DISTINCT sha256),
       ROUND(100.0 * COUNT(DISTINCT sha256) / NULLIF(COUNT(*), 0), 1)
FROM snapshot_files
UNION ALL
SELECT 'refs', COUNT(*), COUNT(DISTINCT from_symbol_id || ':' || to_symbol_id || ':' || kind),
       ROUND(100.0 * COUNT(DISTINCT from_symbol_id || ':' || to_symbol_id || ':' || kind) / NULLIF(COUNT(*), 0), 1)
FROM snapshot_refs
UNION ALL
SELECT 'packages', COUNT(*), COUNT(DISTINCT id),
       ROUND(100.0 * COUNT(DISTINCT id) / NULLIF(COUNT(*), 0), 1)
FROM snapshot_packages;
