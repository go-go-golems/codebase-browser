-- 02-deduplication-analysis.sql
-- Measure redundancy: how many rows are exact duplicates across commits?
-- Usage: sqlite3 -header -column <db> < scripts/02-deduplication-analysis.sql

.headers on
.mode column

-- 1) Symbols: unique body hashes vs total rows
SELECT 'symbols' as entity,
       COUNT(*) as total_rows,
       COUNT(DISTINCT body_hash) as unique_body_hashes,
       ROUND(100.0 * COUNT(DISTINCT body_hash) / NULLIF(COUNT(*), 0), 1) as pct_unique
FROM snapshot_symbols WHERE body_hash != '';

-- 2) Files: unique SHA256 vs total rows
SELECT 'files' as entity,
       COUNT(*) as total_rows,
       COUNT(DISTINCT sha256) as unique_sha256s,
       ROUND(100.0 * COUNT(DISTINCT sha256) / NULLIF(COUNT(*), 0), 1) as pct_unique
FROM snapshot_files;

-- 3) Refs: unique (from, to, kind) triples vs total rows
SELECT 'refs' as entity,
       COUNT(*) as total_rows,
       COUNT(DISTINCT from_symbol_id || ':' || to_symbol_id || ':' || kind) as unique_pairs,
       ROUND(100.0 * COUNT(DISTINCT from_symbol_id || ':' || to_symbol_id || ':' || kind) / NULLIF(COUNT(*), 0), 1) as pct_unique
FROM snapshot_refs;

-- 4) Packages: unique vs total
SELECT 'packages' as entity,
       COUNT(*) as total_rows,
       COUNT(DISTINCT id) as unique_pkgs,
       ROUND(100.0 * COUNT(DISTINCT id) / NULLIF(COUNT(*), 0), 1) as pct_unique
FROM snapshot_packages;

-- 5) Symbol versions: unique (id, body_hash) combinations
SELECT 'symbol_versions' as entity,
       COUNT(*) as total_rows,
       COUNT(DISTINCT id || ':' || COALESCE(body_hash, '')) as unique_versions,
       ROUND(100.0 * COUNT(DISTINCT id || ':' || COALESCE(body_hash, '')) / NULLIF(COUNT(*), 0), 1) as pct_unique
FROM snapshot_symbols;
