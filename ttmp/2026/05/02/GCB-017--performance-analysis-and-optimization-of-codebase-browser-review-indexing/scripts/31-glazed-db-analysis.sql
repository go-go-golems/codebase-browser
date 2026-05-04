-- 31-glazed-db-analysis.sql
-- Run against any glazed benchmark DB to get full metrics
-- Usage: sqlite3 -header -column /tmp/glazed-bench/glazed-10fp.db < scripts/31-glazed-db-analysis.sql

.header on
.mode column
.width 20 12 12 12

-- 1. Table row counts
SELECT 'commits' as entity, COUNT(*) as rows FROM commits
UNION ALL SELECT 'packages', COUNT(*) FROM packages
UNION ALL SELECT 'files', COUNT(*) FROM files
UNION ALL SELECT 'symbols', COUNT(*) FROM symbols
UNION ALL SELECT 'ref_versions', COUNT(*) FROM ref_versions
UNION ALL SELECT 'file_contents', COUNT(*) FROM file_contents
UNION ALL SELECT 'commit_packages', COUNT(*) FROM commit_packages
UNION ALL SELECT 'commit_files', COUNT(*) FROM commit_files
UNION ALL SELECT 'commit_symbols', COUNT(*) FROM commit_symbols
UNION ALL SELECT 'commit_refs', COUNT(*) FROM commit_refs;

-- 2. Redundancy ratios
SELECT 
  'symbols' as entity,
  (SELECT COUNT(*) FROM commit_symbols) as total_mappings,
  (SELECT COUNT(*) FROM symbols) as unique_entities,
  ROUND(100.0 * (1 - (SELECT COUNT(*) FROM symbols) * 1.0 / NULLIF((SELECT COUNT(*) FROM commit_symbols), 0)), 1) as redundancy_pct
UNION ALL
SELECT 'refs', (SELECT COUNT(*) FROM commit_refs), (SELECT COUNT(*) FROM ref_versions),
  ROUND(100.0 * (1 - (SELECT COUNT(*) FROM ref_versions) * 1.0 / NULLIF((SELECT COUNT(*) FROM commit_refs), 0)), 1)
UNION ALL
SELECT 'files', (SELECT COUNT(*) FROM commit_files), (SELECT COUNT(*) FROM files),
  ROUND(100.0 * (1 - (SELECT COUNT(*) FROM files) * 1.0 / NULLIF((SELECT COUNT(*) FROM commit_files), 0)), 1)
UNION ALL
SELECT 'packages', (SELECT COUNT(*) FROM commit_packages), (SELECT COUNT(*) FROM packages),
  ROUND(100.0 * (1 - (SELECT COUNT(*) FROM packages) * 1.0 / NULLIF((SELECT COUNT(*) FROM commit_packages), 0)), 1);

-- 3. Per-table size estimate (approximate from sqlite_master)
SELECT 
  name as table_name,
  SUM(pgsize) / 1024 as size_kb
FROM dbstat 
WHERE name IN ('commits','packages','files','symbols','ref_versions','file_contents',
               'commit_packages','commit_files','commit_symbols','commit_refs')
GROUP BY name
ORDER BY SUM(pgsize) DESC;

-- 4. Average entities per commit
SELECT 
  ROUND(AVG(cnt), 1) as avg_symbols_per_commit
FROM (SELECT commit_id, COUNT(*) as cnt FROM commit_symbols GROUP BY commit_id);

SELECT 
  ROUND(AVG(cnt), 1) as avg_refs_per_commit
FROM (SELECT commit_id, COUNT(*) as cnt FROM commit_refs GROUP BY commit_id);

-- 5. Consecutive commit overlap (symbols)
WITH ordered_commits AS (
  SELECT id, hash, ROW_NUMBER() OVER (ORDER BY id) as rn
  FROM commits
),
commit_sym_sets AS (
  SELECT commit_id, symbol_id,
    LAG(commit_id) OVER (PARTITION BY symbol_id ORDER BY commit_id) as prev_commit
  FROM commit_symbols
),
overlap AS (
  SELECT 
    c.commit_id,
    COUNT(*) as shared_symbols,
    LAG(COUNT(*)) OVER (ORDER BY c.commit_id) as prev_total,
    total.total_symbols as curr_total
  FROM commit_sym_sets c
  JOIN (SELECT commit_id, COUNT(*) as total_symbols FROM commit_symbols GROUP BY commit_id) total
    ON total.commit_id = c.commit_id
  WHERE c.prev_commit IS NOT NULL
  GROUP BY c.commit_id
)
SELECT 
  ROUND(AVG(100.0 * shared_symbols / curr_total), 1) as avg_overlap_pct,
  MIN(ROUND(100.0 * shared_symbols / curr_total, 1)) as min_overlap_pct,
  MAX(ROUND(100.0 * shared_symbols / curr_total, 1)) as max_overlap_pct
FROM overlap;
