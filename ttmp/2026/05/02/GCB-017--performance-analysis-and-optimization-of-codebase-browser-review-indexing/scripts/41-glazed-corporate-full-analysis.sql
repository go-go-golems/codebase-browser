.headers on
.mode column

SELECT 'db_size_mb' AS metric, ROUND(page_count*page_size/1048576.0,2) AS value
FROM pragma_page_count(), pragma_page_size();

SELECT 'commits' metric, COUNT(*) value FROM commits
UNION ALL SELECT 'packages', COUNT(*) FROM packages
UNION ALL SELECT 'files_unique', COUNT(*) FROM files
UNION ALL SELECT 'file_contents', COUNT(*) FROM file_contents
UNION ALL SELECT 'symbols_unique', COUNT(*) FROM symbols
UNION ALL SELECT 'refs_unique', COUNT(*) FROM ref_versions
UNION ALL SELECT 'commit_files_rows', COUNT(*) FROM commit_files
UNION ALL SELECT 'commit_symbols_rows', COUNT(*) FROM commit_symbols
UNION ALL SELECT 'commit_refs_rows', COUNT(*) FROM commit_refs
UNION ALL SELECT 'review_docs', COUNT(*) FROM review_docs
UNION ALL SELECT 'review_snippets', COUNT(*) FROM review_doc_snippets;

WITH m AS (
  SELECT 'files' entity, (SELECT COUNT(*) FROM commit_files) mapped, (SELECT COUNT(*) FROM files) unique_count UNION ALL
  SELECT 'symbols', (SELECT COUNT(*) FROM commit_symbols), (SELECT COUNT(*) FROM symbols) UNION ALL
  SELECT 'refs', (SELECT COUNT(*) FROM commit_refs), (SELECT COUNT(*) FROM ref_versions)
)
SELECT entity, mapped, unique_count,
       ROUND(100.0*(1.0 - CAST(unique_count AS REAL)/mapped), 2) AS redundancy_pct
FROM m;

SELECT name, ROUND(SUM(pgsize)/1048576.0,2) mb
FROM dbstat
GROUP BY name
ORDER BY SUM(pgsize) DESC
LIMIT 15;

SELECT 'latest' label, hash, sequence, datetime(author_time,'unixepoch') author_time, message
FROM commits ORDER BY sequence DESC LIMIT 1;

SELECT 'oldest_indexed' label, hash, sequence, datetime(author_time,'unixepoch') author_time, message
FROM commits ORDER BY sequence ASC LIMIT 1;
