-- 10-incremental-feasibility.sql
-- For incremental indexing: which commits already exist, and what's the gap?
-- Also: overlap between consecutive commits (what fraction of symbols are unchanged?)
-- Usage: sqlite3 -header -column <db> < scripts/10-incremental-feasibility.sql

.headers on
.mode column

-- Consecutive commit overlap: what fraction of symbols are unchanged?
-- (same id, same body_hash in both commits)
WITH ordered_commits AS (
    SELECT hash, short_hash,
           LAG(hash) OVER (ORDER BY author_time) as prev_hash
    FROM commits
),
pair_stats AS (
    SELECT
        oc.short_hash,
        oc.prev_hash,
        (SELECT COUNT(*) FROM snapshot_symbols WHERE commit_hash = oc.hash) as curr_symbols,
        (SELECT COUNT(*) FROM snapshot_symbols WHERE commit_hash = oc.prev_hash) as prev_symbols,
        (SELECT COUNT(*) FROM snapshot_symbols curr
         JOIN snapshot_symbols prev ON curr.id = prev.id AND curr.body_hash = prev.body_hash
         WHERE curr.commit_hash = oc.hash AND prev.commit_hash = oc.prev_hash
        ) as unchanged_symbols
    FROM ordered_commits oc
    WHERE oc.prev_hash IS NOT NULL
)
SELECT short_hash, curr_symbols, prev_symbols, unchanged_symbols,
       ROUND(100.0 * unchanged_symbols / NULLIF(curr_symbols, 0), 1) as pct_unchanged
FROM pair_stats
ORDER BY pct_unchanged
LIMIT 20;
