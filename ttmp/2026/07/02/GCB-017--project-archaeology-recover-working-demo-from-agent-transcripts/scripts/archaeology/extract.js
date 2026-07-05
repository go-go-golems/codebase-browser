// Structured go-minitrace query commands for the GCB-017 archaeology work.
// Use as an external query repository:
//   go-minitrace query commands --query-repository scripts archaeology extract prompts --archive-glob 'archive/minitrace/active/*/*.minitrace.json'
//   go-minitrace query commands --query-repository scripts archaeology extract shell-commands --archive-glob 'archive/minitrace/active/*/*.minitrace.json'

__section__('filters', {
  title: 'Filters',
  fields: {
    session: { type: 'string', help: 'Substring filter for session id or title' },
    limit: { type: 'int', default: 100, help: 'Maximum rows to return' },
  },
});

function sessionWhere(filters, mt, alias) {
  if (!filters.session) return '';
  const needle = mt.sql.like('%' + filters.session + '%');
  return `AND (${alias}.session_id LIKE ${needle} OR s.title LIKE ${needle})`;
}

function openDb() {
  const mt = require('minitrace');
  return mt.db().RuntimeArchives().QueryCommandDefaults().Build();
}

function prompts(filters) {
  const mt = require('minitrace');
  const db = openDb();
  try {
    return db.query(`
      SELECT
        t.session_id,
        s.title,
        s.started_at,
        t.turn_index,
        t.role,
        substr(t.content, 1, 500) AS content_preview
      FROM turns t
      JOIN sessions s USING (session_id)
      WHERE t.role IN ('user', 'assistant')
        AND t.content IS NOT NULL
        ${sessionWhere(filters, mt, 't')}
      ORDER BY s.started_at, t.session_id, t.turn_index
      LIMIT ${Number(filters.limit || 100)}
    `);
  } finally { db.close(); }
}

function shellCommands(filters) {
  const mt = require('minitrace');
  const db = openDb();
  try {
    return db.query(`
      SELECT
        tc.session_id,
        s.title,
        s.started_at,
        tc.emitting_turn_index,
        tc.tool_name,
        tc.operation_type,
        substr(COALESCE(tc.command, tc.arguments_json, ''), 1, 1000) AS command_preview,
        substr(COALESCE(tc.result, tc.error, ''), 1, 500) AS result_preview,
        tc.success,
        tc.exit_code
      FROM tool_calls tc
      JOIN sessions s USING (session_id)
      WHERE lower(COALESCE(tc.tool_name, '')) LIKE '%bash%'
         OR lower(COALESCE(tc.tool_name, '')) LIKE '%shell%'
         OR lower(COALESCE(tc.tool_name, '')) LIKE '%exec%'
         OR tc.command IS NOT NULL
        ${sessionWhere(filters, mt, 'tc')}
      ORDER BY s.started_at, tc.session_id, tc.emitting_turn_index
      LIMIT ${Number(filters.limit || 100)}
    `);
  } finally { db.close(); }
}

function demoSignals(filters) {
  const mt = require('minitrace');
  const db = openDb();
  const terms = ['serve', 'http.server', 'review export', 'docs-smoke', 'make build', 'playwright', 'localhost', 'static', 'sql.js', 'wasm'];
  const contentPattern = terms.map((term) => `lower(COALESCE(t.content, '')) LIKE ${mt.sql.like('%' + term.toLowerCase() + '%')}`).join(' OR ');
  const toolPattern = terms.map((term) => `lower(COALESCE(tc.command, tc.arguments_json, tc.result, '')) LIKE ${mt.sql.like('%' + term.toLowerCase() + '%')}`).join(' OR ');
  try {
    return db.query(`
      SELECT * FROM (
        SELECT
          t.session_id,
          s.title,
          s.started_at,
          t.turn_index AS ordinal,
          'turn:' || t.role AS source,
          substr(t.content, 1, 1200) AS evidence
        FROM turns t
        JOIN sessions s USING (session_id)
        WHERE (${contentPattern})
          ${sessionWhere(filters, mt, 't')}
        UNION ALL
        SELECT
          tc.session_id,
          s.title,
          s.started_at,
          tc.emitting_turn_index AS ordinal,
          'tool:' || COALESCE(tc.tool_name, '') AS source,
          substr(COALESCE(tc.command, tc.arguments_json, tc.result, ''), 1, 1200) AS evidence
        FROM tool_calls tc
        JOIN sessions s USING (session_id)
        WHERE (${toolPattern})
          ${sessionWhere(filters, mt, 'tc')}
      )
      ORDER BY started_at, session_id, ordinal
      LIMIT ${Number(filters.limit || 100)}
    `);
  } finally { db.close(); }
}

__verb__('prompts', {
  name: 'prompts',
  short: 'List user/assistant prompt previews from archaeology sessions',
  fields: { filters: { bind: 'filters' } },
});

__verb__('shellCommands', {
  name: 'shell-commands',
  short: 'List shell-like tool command previews from archaeology sessions',
  fields: { filters: { bind: 'filters' } },
});

__verb__('demoSignals', {
  name: 'demo-signals',
  short: 'Find message evidence related to build/export/serve demo recovery',
  fields: { filters: { bind: 'filters' } },
});
