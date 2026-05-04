/* codebase-browser concept
name: commits-timeline
short: List indexed commits chronologically
params:
  - name: limit
    type: int
    default: 50
    help: Max commits
  - name: branch
    type: string
    default: ""
    help: Filter by branch name
tags: [history, commits]
*/
SELECT hash, short_hash, message, author_name,
       datetime(author_time, 'unixepoch') AS author_date,
       (SELECT COUNT(1) FROM snapshot_symbols ss WHERE ss.commit_hash = c.hash) AS symbol_count
FROM   commits c
WHERE  c.branch LIKE '%' || {{sqlString .branch}} || '%'
ORDER BY sequence DESC, author_time DESC
LIMIT  {{.limit}};
