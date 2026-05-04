import type initSqlJs from 'sql.js';

type SqlValue = initSqlJs.SqlValue;
type Database = initSqlJs.Database;

export type SqlRow = Record<string, SqlValue>;

const slowQueryThresholdMs = 1000;

function isSqlDebugEnabled(): boolean {
  const location = globalThis.location;
  return !!location && new URLSearchParams(location.search).has('debugSql');
}

function compactSql(sql: string): string {
  return sql.replace(/\s+/g, ' ').trim().slice(0, 500);
}

function compactParams(params: SqlValue[]): SqlValue[] {
  return params.map((param) => {
    if (param instanceof Uint8Array) return `<blob:${param.byteLength}>`;
    return param;
  });
}

export function queryAll<T extends SqlRow = SqlRow>(
  db: Database,
  sql: string,
  params: SqlValue[] = [],
): T[] {
  const debug = isSqlDebugEnabled();
  const started = performance.now();
  const sqlPreview = debug ? compactSql(sql) : '';
  if (debug) {
    console.warn('[sql.js:start]', { sql: sqlPreview, params: compactParams(params) });
  }

  const stmt = db.prepare(sql);
  try {
    stmt.bind(params);
    const rows: T[] = [];
    while (stmt.step()) {
      rows.push(stmt.getAsObject() as T);
    }
    const elapsedMs = performance.now() - started;
    if (debug || elapsedMs >= slowQueryThresholdMs) {
      console.warn('[sql.js:done]', {
        elapsedMs: Math.round(elapsedMs),
        rows: rows.length,
        sql: sqlPreview || compactSql(sql),
        params: debug ? compactParams(params) : undefined,
      });
    }
    return rows;
  } catch (error) {
    console.error('[sql.js:error]', {
      elapsedMs: Math.round(performance.now() - started),
      sql: sqlPreview || compactSql(sql),
      params: compactParams(params),
      error,
    });
    throw error;
  } finally {
    stmt.free();
  }
}

export function queryOne<T extends SqlRow = SqlRow>(
  db: Database,
  sql: string,
  params: SqlValue[] = [],
): T | null {
  return queryAll<T>(db, sql, params)[0] ?? null;
}

const utf8Decoder = new TextDecoder('utf-8');

export function sqlBlobToBytes(value: SqlValue | number[] | undefined): Uint8Array {
  if (value instanceof Uint8Array) return value;
  if (Array.isArray(value)) return new Uint8Array(value);
  if (typeof value === 'string') return new TextEncoder().encode(value);
  return new Uint8Array();
}

export function sqlBlobToText(value: SqlValue | number[] | undefined): string {
  return utf8Decoder.decode(sqlBlobToBytes(value));
}

export function extractUtf8Range(bytes: Uint8Array, startOffset: number, endOffset: number): string {
  if (startOffset < 0 || endOffset > bytes.length || startOffset > endOffset) {
    throw new Error(`invalid byte range ${startOffset}-${endOffset} for content length ${bytes.length}`);
  }
  return utf8Decoder.decode(bytes.slice(startOffset, endOffset));
}
