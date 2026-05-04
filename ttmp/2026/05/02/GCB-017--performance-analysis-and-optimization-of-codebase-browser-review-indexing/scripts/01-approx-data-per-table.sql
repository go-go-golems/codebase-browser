-- 01-approx-data-per-table.sql
-- Approximate data size per table (summing column lengths)
-- Usage: sqlite3 <db> < scripts/01-approx-data-per-table.sql

SELECT 'commits' as tbl, COUNT(*) as rows,
       SUM(LENGTH(hash) + LENGTH(short_hash) + LENGTH(message) + LENGTH(author_name) + LENGTH(author_email)) as approx_data_bytes
FROM commits
UNION ALL
SELECT 'snapshot_packages', COUNT(*),
       SUM(LENGTH(id) + LENGTH(import_path) + LENGTH(name) + LENGTH(doc))
FROM snapshot_packages
UNION ALL
SELECT 'snapshot_files', COUNT(*),
       SUM(LENGTH(id) + LENGTH(path) + LENGTH(package_id) + LENGTH(sha256))
FROM snapshot_files
UNION ALL
SELECT 'snapshot_symbols', COUNT(*),
       SUM(LENGTH(id) + LENGTH(kind) + LENGTH(name) + LENGTH(package_id) + LENGTH(file_id) + LENGTH(doc) + LENGTH(signature) + LENGTH(body_hash))
FROM snapshot_symbols
UNION ALL
SELECT 'snapshot_refs', COUNT(*),
       SUM(LENGTH(from_symbol_id) + LENGTH(to_symbol_id) + LENGTH(kind) + LENGTH(file_id))
FROM snapshot_refs
UNION ALL
SELECT 'file_contents', COUNT(*),
       SUM(LENGTH(content))
FROM file_contents;
