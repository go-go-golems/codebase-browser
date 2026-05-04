-- 05-file-contents-size.sql
-- Total cached file contents and their size
-- Usage: sqlite3 -header -column <db> < scripts/05-file-contents-size.sql

.headers on
.mode column

SELECT COUNT(*) as cached_files,
       SUM(LENGTH(content)) as total_bytes,
       ROUND(SUM(LENGTH(content)) / 1024.0 / 1024.0, 2) as total_mb
FROM file_contents;
