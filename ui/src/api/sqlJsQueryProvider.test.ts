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
    await withProvider(async (provider, db) => {
      await expect(provider.resolveCommitRef('HEAD')).rejects.toMatchObject({ code: 'NOT_FOUND' });

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
