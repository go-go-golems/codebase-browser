-- 08-index-overhead.sql
-- How much space do indexes consume vs data? (uses sqlite-viz output)
-- This is a companion to `sqlite-viz tables -d <db>` which gives the raw numbers.
-- Run: sqlite-viz tables -d <db>
--
-- For a deeper look, check the page counts per index:
.headers on
.mode column

SELECT
    name as index_or_table,
    tbl_name,
    CASE WHEN type = 'index' THEN 'index' ELSE 'table' END as obj_type
FROM sqlite_master
WHERE type IN ('table', 'index')
  AND name NOT LIKE 'sqlite_%'
ORDER BY type, name;
