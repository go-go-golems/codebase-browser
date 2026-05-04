import { readFileSync } from 'node:fs';
import initSqlJs from 'sql.js';
import type initSqlJsTypes from 'sql.js';
import { describe, expect, it } from 'vitest';

import { QueryError } from './queryErrors';
import { SqlJsQueryProvider } from './sqlJsQueryProvider';

type Database = initSqlJsTypes.Database;

async function withProvider<T>(fn: (provider: SqlJsQueryProvider, db: Database) => Promise<T>): Promise<T> {
  const SQL = await initSqlJs();
  const db = new SQL.Database();
  try {
    db.run(`
      CREATE TABLE commits (
        hash TEXT PRIMARY KEY,
        short_hash TEXT NOT NULL,
        message TEXT NOT NULL,
        author_name TEXT NOT NULL,
        author_email TEXT NOT NULL,
        author_time INTEGER NOT NULL,
        indexed_at INTEGER NOT NULL,
        sequence INTEGER NOT NULL DEFAULT 0,
        branch TEXT NOT NULL,
        error TEXT NOT NULL
      )
    `);
    const provider = new SqlJsQueryProvider(async () => db);
    return await fn(provider, db);
  } finally {
    db.close();
  }
}

function insertCommit(db: Database, hash: string, shortHash: string, authorTime: number, error = ''): void {
  db.run(
    `INSERT INTO commits (
      hash, short_hash, message, author_name, author_email, author_time, indexed_at, sequence, branch, error
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [hash, shortHash, `message ${shortHash}`, 'Author', 'author@example.test', authorTime, authorTime + 1000, authorTime, 'main', error],
  );
}

async function withRefProvider<T>(fn: (provider: SqlJsQueryProvider, db: Database) => Promise<T>): Promise<T> {
  const SQL = await initSqlJs();
  const db = new SQL.Database();
  try {
    db.run(`
      CREATE TABLE commits (
        id INTEGER PRIMARY KEY,
        hash TEXT NOT NULL UNIQUE,
        short_hash TEXT NOT NULL,
        message TEXT NOT NULL,
        author_name TEXT NOT NULL,
        author_email TEXT NOT NULL,
        author_time INTEGER NOT NULL,
        indexed_at INTEGER NOT NULL,
        sequence INTEGER NOT NULL DEFAULT 0,
        branch TEXT NOT NULL,
        error TEXT NOT NULL
      );
      CREATE TABLE files (
        id INTEGER PRIMARY KEY,
        stable_id TEXT NOT NULL,
        path TEXT NOT NULL,
        sha256 TEXT NOT NULL
      );
      CREATE TABLE symbols (
        id INTEGER PRIMARY KEY,
        stable_id TEXT NOT NULL,
        kind TEXT NOT NULL,
        name TEXT NOT NULL,
        file_id INTEGER NOT NULL,
        start_offset INTEGER NOT NULL DEFAULT 0,
        end_offset INTEGER NOT NULL DEFAULT 0,
        start_line INTEGER NOT NULL DEFAULT 0,
        end_line INTEGER NOT NULL DEFAULT 0,
        signature TEXT NOT NULL DEFAULT ''
      );
      CREATE TABLE commit_symbols (
        commit_id INTEGER NOT NULL,
        symbol_id INTEGER NOT NULL,
        PRIMARY KEY (commit_id, symbol_id)
      );
      CREATE TABLE ref_versions (
        id INTEGER PRIMARY KEY,
        from_symbol_id INTEGER NOT NULL,
        to_stable_id TEXT NOT NULL,
        kind TEXT NOT NULL,
        file_id INTEGER NOT NULL,
        locations_json TEXT NOT NULL
      );
      CREATE INDEX idx_ref_from ON ref_versions(from_symbol_id);
      CREATE INDEX idx_ref_to ON ref_versions(to_stable_id);
      CREATE TABLE commit_refs (
        commit_id INTEGER NOT NULL,
        ref_version_id INTEGER NOT NULL,
        PRIMARY KEY (commit_id, ref_version_id)
      );
      CREATE TABLE snapshot_files (
        commit_hash TEXT NOT NULL,
        id TEXT NOT NULL,
        path TEXT NOT NULL,
        sha256 TEXT NOT NULL
      );
      CREATE TABLE snapshot_symbols (
        commit_hash TEXT NOT NULL,
        id TEXT NOT NULL,
        name TEXT NOT NULL,
        start_offset INTEGER NOT NULL,
        end_offset INTEGER NOT NULL,
        start_line INTEGER NOT NULL,
        end_line INTEGER NOT NULL,
        file_id TEXT NOT NULL,
        signature TEXT NOT NULL
      );
    `);
    db.run(
      `INSERT INTO commits (id, hash, short_hash, message, author_name, author_email, author_time, indexed_at, sequence, branch, error)
       VALUES (1, 'cccc333333333333333333333333333333333333', 'cccc333', 'latest', 'Author', 'a@example.test', 300, 400, 1, 'main', '')`,
    );
    db.run(`INSERT INTO files (id, stable_id, path, sha256) VALUES
      (10, 'file:pkg/a.go', 'pkg/a.go', 'sha-a'),
      (11, 'file:pkg/b.go', 'pkg/b.go', 'sha-b')`);
    db.run(`INSERT INTO symbols (id, stable_id, kind, name, file_id, start_offset, end_offset, start_line, end_line, signature) VALUES
      (100, 'sym:pkg/a.func.Func', 'function', 'Func', 10, 10, 80, 1, 8, 'func Func()'),
      (101, 'sym:pkg/b.func.Target', 'function', 'Target', 11, 10, 30, 1, 3, 'func Target()'),
      (102, 'sym:pkg/b.func.Caller', 'function', 'Caller', 11, 40, 90, 5, 9, 'func Caller()'),
      (103, 'sym:pkg/a.func.Helper', 'function', 'Helper', 10, 90, 120, 10, 12, 'func Helper()')`);
    db.run(`INSERT INTO commit_symbols (commit_id, symbol_id) VALUES (1, 100), (1, 101), (1, 102), (1, 103)`);
    db.run(`INSERT INTO ref_versions (id, from_symbol_id, to_stable_id, kind, file_id, locations_json) VALUES
      (1, 100, 'sym:pkg/b.func.Target', 'call', 10, '[{"start_line":2,"start_col":3,"end_line":2,"end_col":9,"start_offset":20,"end_offset":26}]'),
      (2, 100, 'sym:pkg/a.func.Helper', 'call', 10, '[{"start_line":4,"start_col":3,"end_line":4,"end_col":9,"start_offset":40,"end_offset":46}]'),
      (3, 102, 'sym:pkg/a.func.Func', 'call', 11, '[{"start_line":6,"start_col":3,"end_line":6,"end_col":7,"start_offset":50,"end_offset":54}]')`);
    db.run(`INSERT INTO commit_refs (commit_id, ref_version_id) VALUES (1, 1), (1, 2), (1, 3)`);
    db.run(`INSERT INTO snapshot_files (commit_hash, id, path, sha256) VALUES
      ('cccc333333333333333333333333333333333333', 'file:pkg/a.go', 'pkg/a.go', 'sha-a'),
      ('cccc333333333333333333333333333333333333', 'file:pkg/b.go', 'pkg/b.go', 'sha-b')`);
    db.run(`INSERT INTO snapshot_symbols (commit_hash, id, name, start_offset, end_offset, start_line, end_line, file_id, signature) VALUES
      ('cccc333333333333333333333333333333333333', 'sym:pkg/a.func.Func', 'Func', 10, 80, 1, 8, 'file:pkg/a.go', 'func Func()'),
      ('cccc333333333333333333333333333333333333', 'sym:pkg/b.func.Target', 'Target', 10, 30, 1, 3, 'file:pkg/b.go', 'func Target()'),
      ('cccc333333333333333333333333333333333333', 'sym:pkg/b.func.Caller', 'Caller', 40, 90, 5, 9, 'file:pkg/b.go', 'func Caller()'),
      ('cccc333333333333333333333333333333333333', 'sym:pkg/a.func.Helper', 'Helper', 90, 120, 10, 12, 'file:pkg/a.go', 'func Helper()')`);

    const provider = new SqlJsQueryProvider(async () => db);
    return await fn(provider, db);
  } finally {
    db.close();
  }
}

describe('SqlJsQueryProvider commit refs', () => {
  it('lists successful commits newest first and ignores errored rows', async () => {
    await withProvider(async (provider, db) => {
      insertCommit(db, 'aaaa111111111111111111111111111111111111', 'aaaa111', 100);
      insertCommit(db, 'bbbb222222222222222222222222222222222222', 'bbbb222', 200);
      insertCommit(db, 'cccc333333333333333333333333333333333333', 'cccc333', 300, 'index failed');

      expect((await provider.listCommits()).map((commit) => commit.Hash)).toEqual([
        'bbbb222222222222222222222222222222222222',
        'aaaa111111111111111111111111111111111111',
      ]);
    });
  });

  it('caches commit list, resolved refs, and commit lookups for immutable static exports', async () => {
    await withProvider(async (provider, db) => {
      insertCommit(db, 'aaaa111111111111111111111111111111111111', 'aaaa111', 100);
      insertCommit(db, 'bbbb222222222222222222222222222222222222', 'bbbb222', 200);

      const originalPrepare = db.prepare.bind(db);
      let commitListQueries = 0;
      db.prepare = ((sql: string) => {
        if (sql.includes('FROM commits') && sql.includes('ORDER BY sequence DESC')) {
          commitListQueries++;
        }
        return originalPrepare(sql);
      }) as typeof db.prepare;

      expect(await provider.resolveCommitRef('HEAD')).toBe('bbbb222222222222222222222222222222222222');
      expect(await provider.resolveCommitRef('HEAD')).toBe('bbbb222222222222222222222222222222222222');
      expect((await provider.listCommits()).map((commit) => commit.Hash)).toEqual([
        'bbbb222222222222222222222222222222222222',
        'aaaa111111111111111111111111111111111111',
      ]);
      expect((await provider.getCommit('HEAD')).Hash).toBe('bbbb222222222222222222222222222222222222');
      expect((await provider.getCommit('HEAD')).Hash).toBe('bbbb222222222222222222222222222222222222');
      expect(commitListQueries).toBe(1);
    });
  });

  it('resolves HEAD, HEAD~N, exact hashes, short hashes, and unique prefixes', async () => {
    await withProvider(async (provider, db) => {
      insertCommit(db, 'aaaa111111111111111111111111111111111111', 'aaaa111', 100);
      insertCommit(db, 'bbbb222222222222222222222222222222222222', 'bbbb222', 200);
      insertCommit(db, 'cccc333333333333333333333333333333333333', 'cccc333', 300);

      await expect(provider.resolveCommitRef('HEAD')).resolves.toBe('cccc333333333333333333333333333333333333');
      await expect(provider.resolveCommitRef('HEAD~1')).resolves.toBe('bbbb222222222222222222222222222222222222');
      await expect(provider.resolveCommitRef('HEAD~2')).resolves.toBe('aaaa111111111111111111111111111111111111');
      await expect(provider.resolveCommitRef('bbbb222222222222222222222222222222222222')).resolves.toBe(
        'bbbb222222222222222222222222222222222222',
      );
      await expect(provider.resolveCommitRef('bbbb222')).resolves.toBe('bbbb222222222222222222222222222222222222');
      await expect(provider.resolveCommitRef('cccc33')).resolves.toBe('cccc333333333333333333333333333333333333');
    });
  });

  it('reports missing, empty, and ambiguous refs with structured query errors', async () => {
    await withProvider(async (provider) => {
      await expect(provider.resolveCommitRef('HEAD')).rejects.toMatchObject({ code: 'NOT_FOUND' });
    });

    await withProvider(async (provider, db) => {
      insertCommit(db, 'abc1111111111111111111111111111111111111', 'abc1111', 100);
      insertCommit(db, 'abc2222222222222222222222222222222222222', 'abc2222', 200);

      await expect(provider.resolveCommitRef('HEAD~9')).rejects.toMatchObject({
        code: 'NOT_FOUND',
        message: expect.stringContaining('outside this export'),
        details: expect.objectContaining({ indexedCommitCount: 2, requestedOffset: 9 }),
      });
      await expect(provider.resolveCommitRef('missing')).rejects.toMatchObject({
        code: 'NOT_FOUND',
        message: expect.stringContaining('was not found in this static export'),
        details: expect.objectContaining({ indexedCommitCount: 2 }),
      });
      await expect(provider.resolveCommitRef('abc')).rejects.toMatchObject({
        code: 'AMBIGUOUS_REF',
        details: expect.objectContaining({ indexedCommitCount: 2 }),
      });

      try {
        await provider.resolveCommitRef('abc');
      } catch (error) {
        expect(error).toBeInstanceOf(QueryError);
      }
    });
  });
});

describe('SqlJsQueryProvider hot-path SQL regressions', () => {
  it('does not use snapshot_refs in frontend provider hot paths', () => {
    const source = readFileSync(new URL('./sqlJsQueryProvider.ts', import.meta.url), 'utf8');

    expect(source).not.toContain('snapshot_refs');
  });
});

describe('SqlJsQueryProvider normalized ref queries', () => {
  it('loads source refs from normalized tables', async () => {
    await withRefProvider(async (provider) => {
      const refs = await provider.getSourceRefs('pkg/a.go');

      expect(refs).toEqual([
        { toSymbolId: 'sym:pkg/b.func.Target', kind: 'call', offset: 20, length: 6 },
        { toSymbolId: 'sym:pkg/a.func.Helper', kind: 'call', offset: 40, length: 6 },
      ]);
    });
  });

  it('loads snippet refs from normalized tables and clips to the symbol body range', async () => {
    await withRefProvider(async (provider) => {
      const refs = await provider.getSnippetRefs('sym:pkg/a.func.Func');

      expect(refs).toEqual([
        { toSymbolId: 'sym:pkg/b.func.Target', kind: 'call', offsetInSnippet: 10, length: 6 },
        { toSymbolId: 'sym:pkg/a.func.Helper', kind: 'call', offsetInSnippet: 30, length: 6 },
      ]);
    });
  });

  it('loads file xrefs from normalized tables and excludes intra-file refs', async () => {
    await withRefProvider(async (provider) => {
      const xref = await provider.getFileXref('pkg/a.go');

      expect(xref.usedBy.map((ref) => ref.fromSymbolId)).toEqual(['sym:pkg/b.func.Caller']);
      expect(xref.uses).toHaveLength(1);
      expect(xref.uses[0]).toMatchObject({
        toSymbolId: 'sym:pkg/b.func.Target',
        kind: 'call',
        count: 1,
      });
    });
  });
});
