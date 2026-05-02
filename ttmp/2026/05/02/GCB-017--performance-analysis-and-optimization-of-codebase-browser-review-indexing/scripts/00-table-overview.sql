-- 00-table-overview.sql
-- Quick row counts and table sizes for a review database
-- Usage: sqlite3 <db> < scripts/00-table-overview.sql

SELECT 'commits' as tbl, COUNT(*) as cnt FROM commits
UNION ALL SELECT 'snapshot_packages', COUNT(*) FROM snapshot_packages
UNION ALL SELECT 'snapshot_files', COUNT(*) FROM snapshot_files
UNION ALL SELECT 'snapshot_symbols', COUNT(*) FROM snapshot_symbols
UNION ALL SELECT 'snapshot_refs', COUNT(*) FROM snapshot_refs
UNION ALL SELECT 'file_contents', COUNT(*) FROM file_contents
UNION ALL SELECT 'review_docs', COUNT(*) FROM review_docs
UNION ALL SELECT 'review_doc_snippets', COUNT(*) FROM review_doc_snippets;
