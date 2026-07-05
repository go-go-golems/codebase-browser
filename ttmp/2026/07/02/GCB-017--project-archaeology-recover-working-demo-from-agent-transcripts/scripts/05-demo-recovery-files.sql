-- Tool calls whose JSON arguments/results mention files relevant to getting a
-- working demo back: server, static export, docs, deployment, and frontend paths.
-- This uses the raw DuckDB sessions_base JSON archive table.

WITH tool_rows AS (
  SELECT
    id,
    title,
    timing->>'started_at' AS started_at,
    REPLACE(CAST(json_extract(tc, '$.tool_name') AS VARCHAR), '"', '') AS tool_name,
    CAST(json_extract(tc, '$.input') AS VARCHAR) AS input_json,
    CAST(json_extract(tc, '$.output') AS VARCHAR) AS output_json
  FROM sessions_base, UNNEST(tool_calls) AS t(tc)
)
SELECT
  id,
  started_at,
  title,
  tool_name,
  LEFT(COALESCE(input_json, ''), 1200) AS input_preview,
  LEFT(COALESCE(output_json, ''), 1200) AS output_preview
FROM tool_rows
WHERE lower(COALESCE(input_json, '') || ' ' || COALESCE(output_json, '')) LIKE '%server%'
   OR lower(COALESCE(input_json, '') || ' ' || COALESCE(output_json, '')) LIKE '%static%'
   OR lower(COALESCE(input_json, '') || ' ' || COALESCE(output_json, '')) LIKE '%review export%'
   OR lower(COALESCE(input_json, '') || ' ' || COALESCE(output_json, '')) LIKE '%deploy%'
   OR lower(COALESCE(input_json, '') || ' ' || COALESCE(output_json, '')) LIKE '%ui/%'
   OR lower(COALESCE(input_json, '') || ' ' || COALESCE(output_json, '')) LIKE '%docker%'
   OR lower(COALESCE(input_json, '') || ' ' || COALESCE(output_json, '')) LIKE '%makefile%'
   OR lower(COALESCE(input_json, '') || ' ' || COALESCE(output_json, '')) LIKE '%cmd/codebase-browser/cmds%'
ORDER BY started_at, id, tool_name
LIMIT 300;
