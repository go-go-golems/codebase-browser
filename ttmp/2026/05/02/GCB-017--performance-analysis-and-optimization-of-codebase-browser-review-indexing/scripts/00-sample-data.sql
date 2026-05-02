-- 00-sample-data.sql
-- Peek at sample rows from each table
-- Usage: sqlite3 -header -column <db> < scripts/00-sample-data.sql

.headers on
.mode column

SELECT '--- commits ---' as info;
SELECT hash, short_hash, substr(message, 1, 60) as message, author_time FROM commits LIMIT 5;

SELECT '--- snapshot_packages ---' as info;
SELECT commit_hash, id, import_path, name, language FROM snapshot_packages LIMIT 10;

SELECT '--- snapshot_files ---' as info;
SELECT commit_hash, id, path, package_id, size, line_count, sha256 FROM snapshot_files LIMIT 5;

SELECT '--- snapshot_symbols ---' as info;
SELECT commit_hash, kind, name, package_id, file_id, start_line, end_line, body_hash
FROM snapshot_symbols LIMIT 5;

SELECT '--- snapshot_refs ---' as info;
SELECT commit_hash, from_symbol_id, to_symbol_id, kind, file_id, start_line, end_line
FROM snapshot_refs LIMIT 5;

SELECT '--- file_contents ---' as info;
SELECT content_hash, LENGTH(content) as bytes FROM file_contents LIMIT 5;
