-- Session-level overview for the six archaeology transcripts.
-- Run with:
--   go-minitrace query duckdb --archive-glob '<ticket>/archive/minitrace/active/*/*.minitrace.json' --sql-file scripts/02-session-overview.sql

SELECT
  id,
  title,
  environment->>'agent_framework' AS framework,
  environment->>'provider_hint' AS provider,
  environment->>'model' AS model,
  timing->>'started_at' AS started_at,
  timing->>'ended_at' AS ended_at,
  CAST(metrics->>'turn_count' AS INT) AS turns,
  CAST(metrics->>'tool_call_count' AS INT) AS tool_calls,
  CAST(metrics->>'read_count' AS INT) AS reads,
  CAST(metrics->>'modify_count' AS INT) AS modifies,
  CAST(metrics->>'create_count' AS INT) AS creates,
  CAST(metrics->>'execute_count' AS INT) AS executes
FROM sessions_base
ORDER BY started_at;
