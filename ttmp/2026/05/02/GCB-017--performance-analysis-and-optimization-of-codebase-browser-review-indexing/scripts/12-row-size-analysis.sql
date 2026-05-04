-- 12-row-size-analysis.sql
-- Average row sizes and ID length for the big tables
-- Usage: sqlite3 -header -column <db> < scripts/12-row-size-analysis.sql

.headers on
.mode column

-- Average row sizes
SELECT
    'snapshot_refs' as tbl,
    COUNT(*) as rows,
    ROUND(AVG(
        LENGTH(from_symbol_id) + LENGTH(to_symbol_id) + LENGTH(kind) + LENGTH(file_id) + 32
    ), 0) as avg_row_bytes,
    ROUND(AVG(LENGTH(from_symbol_id)), 0) as avg_from_id_len,
    ROUND(AVG(LENGTH(to_symbol_id)), 0) as avg_to_id_len,
    ROUND(AVG(LENGTH(file_id)), 0) as avg_file_id_len
FROM snapshot_refs
UNION ALL
SELECT
    'snapshot_symbols',
    COUNT(*),
    ROUND(AVG(
        LENGTH(id) + LENGTH(kind) + LENGTH(name) + LENGTH(package_id) + LENGTH(file_id) +
        24 + LENGTH(doc) + LENGTH(signature) + LENGTH(receiver_type) +
        LENGTH(body_hash) + LENGTH(type_params_json) + LENGTH(tags_json)
    ), 0),
    ROUND(AVG(LENGTH(id)), 0),
    ROUND(AVG(LENGTH(package_id)), 0),
    ROUND(AVG(LENGTH(file_id)), 0)
FROM snapshot_symbols
UNION ALL
SELECT
    'snapshot_files',
    COUNT(*),
    ROUND(AVG(LENGTH(id) + LENGTH(path) + LENGTH(package_id) + LENGTH(sha256) + 20), 0),
    ROUND(AVG(LENGTH(id)), 0),
    ROUND(AVG(LENGTH(package_id)), 0),
    ROUND(AVG(LENGTH(sha256)), 0)
FROM snapshot_files;

-- Sample IDs to understand the format
SELECT '--- Sample ref IDs ---' as info;
SELECT from_symbol_id, to_symbol_id, kind FROM snapshot_refs LIMIT 3;

SELECT '--- Sample symbol IDs ---' as info;
SELECT id, kind, name, package_id, file_id FROM snapshot_symbols LIMIT 3;
