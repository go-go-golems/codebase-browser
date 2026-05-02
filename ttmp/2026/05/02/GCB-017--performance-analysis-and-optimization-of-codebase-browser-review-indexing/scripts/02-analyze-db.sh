#!/usr/bin/env bash
# 02-analyze-db.sh — Analyze a review database for size and deduplication opportunities
# Usage: bash scripts/02-analyze-db.sh PATH_TO_DB
set -euo pipefail

DB="${1:?Usage: $0 <path-to-db>}"

echo "=== Database Analysis: $DB ==="
echo "File size: $(du -h "$DB" | cut -f1)"
echo ""

echo "--- Table overview (sqlite-viz) ---"
sqlite-viz tables -d "$DB" 2>&1

echo ""
echo "--- Row counts and approximate data size ---"
sqlite3 "$DB" "
SELECT tbl, cnt FROM (
    SELECT 'commits' as tbl, COUNT(*) as cnt FROM commits
    UNION ALL SELECT 'snapshot_packages', COUNT(*) FROM snapshot_packages
    UNION ALL SELECT 'snapshot_files', COUNT(*) FROM snapshot_files
    UNION ALL SELECT 'snapshot_symbols', COUNT(*) FROM snapshot_symbols
    UNION ALL SELECT 'snapshot_refs', COUNT(*) FROM snapshot_refs
    UNION ALL SELECT 'file_contents', COUNT(*) FROM file_contents
    UNION ALL SELECT 'review_docs', COUNT(*) FROM review_docs
    UNION ALL SELECT 'review_doc_snippets', COUNT(*) FROM review_doc_snippets
);
"

echo ""
echo "--- Deduplication analysis ---"
echo "Symbols: unique body hashes vs total rows"
sqlite3 "$DB" "
SELECT COUNT(*) as total_symbol_rows,
       COUNT(DISTINCT body_hash) as unique_body_hashes,
       ROUND(100.0 * COUNT(DISTINCT body_hash) / NULLIF(COUNT(*), 0), 1) as pct_unique
FROM snapshot_symbols WHERE body_hash != '';
" 2>/dev/null || echo "(no symbols with body_hash)"

echo ""
echo "Files: unique SHA256 vs total rows"
sqlite3 "$DB" "
SELECT COUNT(*) as total_file_rows,
       COUNT(DISTINCT sha256) as unique_sha256s,
       ROUND(100.0 * COUNT(DISTINCT sha256) / NULLIF(COUNT(*), 0), 1) as pct_unique
FROM snapshot_files;
" 2>/dev/null || echo "(no files)"

echo ""
echo "Refs: unique ref pairs vs total rows"
sqlite3 "$DB" "
SELECT COUNT(*) as total_refs,
       COUNT(DISTINCT from_symbol_id || ':' || to_symbol_id || ':' || kind) as unique_ref_pairs,
       ROUND(100.0 * COUNT(DISTINCT from_symbol_id || ':' || to_symbol_id || ':' || kind) / NULLIF(COUNT(*), 0), 1) as pct_unique
FROM snapshot_refs;
" 2>/dev/null || echo "(no refs)"

echo ""
echo "Packages: unique vs total"
sqlite3 "$DB" "
SELECT COUNT(*) as total,
       COUNT(DISTINCT id) as unique_pkgs
FROM snapshot_packages;
" 2>/dev/null || echo "(no packages)"

echo ""
echo "--- Symbol kind distribution ---"
sqlite3 "$DB" "
SELECT kind, COUNT(*) as cnt FROM snapshot_symbols GROUP BY kind ORDER BY cnt DESC;
" 2>/dev/null || echo "(no symbols)"

echo ""
echo "--- Commits per symbol (change frequency) ---"
sqlite3 "$DB" "
SELECT commit_count, COUNT(*) as symbols_with_that_count
FROM (SELECT id, COUNT(DISTINCT commit_hash) as commit_count FROM snapshot_symbols GROUP BY id)
GROUP BY commit_count
ORDER BY commit_count
LIMIT 20;
" 2>/dev/null || echo "(no symbols)"

echo ""
echo "--- File contents total size ---"
sqlite3 "$DB" "
SELECT COUNT(*) as cached_files,
       SUM(LENGTH(content)) as total_bytes,
       ROUND(SUM(LENGTH(content)) / 1024.0 / 1024.0, 2) as total_mb
FROM file_contents;
" 2>/dev/null || echo "(no file_contents)"

echo ""
echo "--- Potential savings from normalization ---"
sqlite3 "$DB" "
-- If we stored symbols in a base table and only commit→symbol_version mapping:
SELECT 
    'Current symbol storage' as metric,
    COUNT(*) as rows,
    'N/A' as estimated_savings_pct
FROM snapshot_symbols
UNION ALL
SELECT 
    'Unique symbol versions (id+body_hash)',
    COUNT(DISTINCT id || ':' || COALESCE(body_hash, '')),
    ROUND(100.0 - 100.0 * COUNT(DISTINCT id || ':' || COALESCE(body_hash, '')) / NULLIF(COUNT(*), 0), 1) || '%'
FROM snapshot_symbols
UNION ALL
SELECT
    'Unique ref pairs',
    COUNT(DISTINCT from_symbol_id || ':' || to_symbol_id || ':' || kind),
    ROUND(100.0 - 100.0 * COUNT(DISTINCT from_symbol_id || ':' || to_symbol_id || ':' || kind) / NULLIF(COUNT(*), 0), 1) || '%'
FROM snapshot_refs;
" 2>/dev/null || echo "(no data)"
