-- 13-scaling-comparison.sql
-- How does DB size scale with commit count?
-- Run this against multiple DBs and compare the output.
-- Usage: sqlite3 -header -column <db> < scripts/13-scaling-comparison.sql

.headers on
.mode column

SELECT
    (SELECT COUNT(*) FROM commits) as commit_count,
    (SELECT COUNT(*) FROM snapshot_symbols) as symbol_rows,
    (SELECT COUNT(DISTINCT id) FROM snapshot_symbols) as unique_symbols,
    (SELECT COUNT(*) FROM snapshot_refs) as ref_rows,
    (SELECT COUNT(DISTINCT from_symbol_id || ':' || to_symbol_id || ':' || kind) FROM snapshot_refs) as unique_refs,
    (SELECT COUNT(*) FROM snapshot_files) as file_rows,
    (SELECT COUNT(DISTINCT sha256) FROM snapshot_files) as unique_files,
    (SELECT COUNT(*) FROM snapshot_packages) as pkg_rows,
    (SELECT COUNT(DISTINCT id) FROM snapshot_packages) as unique_pkgs,
    (SELECT ROUND(SUM(LENGTH(content)) / 1024.0 / 1024.0, 2) FROM file_contents) as file_contents_mb;
