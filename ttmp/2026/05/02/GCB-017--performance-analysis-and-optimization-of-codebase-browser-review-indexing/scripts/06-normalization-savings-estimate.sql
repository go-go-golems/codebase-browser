-- 06-normalization-savings-estimate.sql
-- Estimate how much space we'd save by normalizing (storing each entity once,
-- with a separate commit→entity mapping table)
-- Usage: sqlite3 -header -column <db> < scripts/06-normalization-savings-estimate.sql

.headers on
.mode column

-- Current vs normalized row counts
SELECT
    'snapshot_symbols' as table_name,
    COUNT(*) as current_rows,
    COUNT(DISTINCT id || ':' || COALESCE(body_hash, '')) as normalized_rows,
    ROUND(100.0 * (1.0 - 1.0 * COUNT(DISTINCT id || ':' || COALESCE(body_hash, '')) / NULLIF(COUNT(*), 0)), 1) as savings_pct
FROM snapshot_symbols
UNION ALL
SELECT
    'snapshot_refs',
    COUNT(*),
    COUNT(DISTINCT from_symbol_id || ':' || to_symbol_id || ':' || kind),
    ROUND(100.0 * (1.0 - 1.0 * COUNT(DISTINCT from_symbol_id || ':' || to_symbol_id || ':' || kind) / NULLIF(COUNT(*), 0)), 1)
FROM snapshot_refs
UNION ALL
SELECT
    'snapshot_files',
    COUNT(*),
    COUNT(DISTINCT sha256),
    ROUND(100.0 * (1.0 - 1.0 * COUNT(DISTINCT sha256) / NULLIF(COUNT(*), 0)), 1)
FROM snapshot_files
UNION ALL
SELECT
    'snapshot_packages',
    COUNT(*),
    COUNT(DISTINCT id),
    ROUND(100.0 * (1.0 - 1.0 * COUNT(DISTINCT id) / NULLIF(COUNT(*), 0)), 1)
FROM snapshot_packages;
