-- 11-consecutive-commit-overlap.sql
-- What fraction of symbols are unchanged between consecutive commits?
-- High overlap = big deduplication opportunity.
-- Usage: sqlite3 -header -column <db> < scripts/11-consecutive-commit-overlap.sql

.headers on
.mode column

-- Distribution of overlap percentages
WITH ordered_commits AS (
    SELECT hash, short_hash,
           LAG(hash) OVER (ORDER BY author_time) as prev_hash
    FROM commits
),
pair_stats AS (
    SELECT
        oc.short_hash,
        oc.hash,
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
SELECT
    CASE
        WHEN pct >= 95 THEN '95-100%'
        WHEN pct >= 90 THEN '90-95%'
        WHEN pct >= 85 THEN '85-90%'
        WHEN pct >= 80 THEN '80-85%'
        WHEN pct >= 70 THEN '70-80%'
        WHEN pct >= 50 THEN '50-70%'
        ELSE '<50%'
    END as overlap_bucket,
    COUNT(*) as commit_pairs
FROM (
    SELECT short_hash, curr_symbols, prev_symbols, unchanged_symbols,
           ROUND(100.0 * unchanged_symbols / NULLIF(curr_symbols, 0), 1) as pct
    FROM pair_stats
)
GROUP BY overlap_bucket
ORDER BY overlap_bucket DESC;

-- Average overlap
SELECT ROUND(AVG(100.0 * unchanged / NULLIF(total, 0)), 1) as avg_pct_unchanged
FROM (
    SELECT
        (SELECT COUNT(*) FROM snapshot_symbols curr
         JOIN snapshot_symbols prev ON curr.id = prev.id AND curr.body_hash = prev.body_hash
         WHERE curr.commit_hash = oc.hash AND prev.commit_hash = oc.prev_hash
        ) as unchanged,
        (SELECT COUNT(*) FROM snapshot_symbols WHERE commit_hash = oc.hash) as total
    FROM (
        SELECT hash,
               LAG(hash) OVER (ORDER BY author_time) as prev_hash
        FROM commits
    ) oc
    WHERE oc.prev_hash IS NOT NULL
);
