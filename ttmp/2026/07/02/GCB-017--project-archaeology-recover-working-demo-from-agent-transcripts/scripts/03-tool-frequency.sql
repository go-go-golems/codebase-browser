-- Tool-call frequency by session and tool. This identifies whether a session
-- was primarily coding, browsing, shell-driven, or documentation-oriented.

SELECT
  id,
  title,
  REPLACE(CAST(json_extract(tc, '$.tool_name') AS VARCHAR), '"', '') AS tool_name,
  COUNT(*) AS calls
FROM sessions_base, UNNEST(tool_calls) AS t(tc)
GROUP BY id, title, tool_name
ORDER BY id, calls DESC, tool_name;
