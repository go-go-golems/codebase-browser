import initSqlJs from 'sql.js';
import type initSqlJsTypes from 'sql.js';
import { queryOne } from './sqlRows';

type Database = initSqlJsTypes.Database;
type SqlJsStatic = initSqlJsTypes.SqlJsStatic;

export interface StaticManifest {
  schemaVersion?: number;
  kind?: string;
  generatedAt?: string;
  db?: {
    path?: string;
    sizeBytes?: number;
    schemaVersion?: number;
  };
  features?: Record<string, boolean>;
  runtime?: Record<string, unknown>;
}

export interface StaticDbLoaderOptions {
  baseUrl?: string;
}

let sqlJsPromise: Promise<SqlJsStatic> | null = null;
let manifestPromise: Promise<StaticManifest> | null = null;
let dbPromise: Promise<Database> | null = null;

function resolveStaticAsset(path: string, baseUrl?: string): string {
  return baseUrl ? new URL(path, baseUrl).toString() : path;
}

export async function getSqlJs(baseUrl?: string): Promise<SqlJsStatic> {
  if (!sqlJsPromise) {
    sqlJsPromise = initSqlJs({
      locateFile: (file) => (file === 'sql-wasm.wasm' ? resolveStaticAsset('sql-wasm.wasm', baseUrl) : resolveStaticAsset(file, baseUrl)),
    });
  }
  return sqlJsPromise;
}

export async function getStaticManifest(baseUrl?: string): Promise<StaticManifest> {
  if (!manifestPromise) {
    manifestPromise = (async () => {
      const response = await fetch(resolveStaticAsset('manifest.json', baseUrl));
      if (!response.ok) {
        return { db: { path: 'db/codebase.db' } } satisfies StaticManifest;
      }
      return (await response.json()) as StaticManifest;
    })();
  }
  return manifestPromise;
}

export function createStaticDbLoader(options: StaticDbLoaderOptions = {}): () => Promise<Database> {
  let localSqlJsPromise: Promise<SqlJsStatic> | null = null;
  let localManifestPromise: Promise<StaticManifest> | null = null;
  let localDbPromise: Promise<Database> | null = null;
  const { baseUrl } = options;

  return async () => {
    if (!localDbPromise) {
      localDbPromise = (async () => {
        localSqlJsPromise ??= initSqlJs({
          locateFile: (file) => (file === 'sql-wasm.wasm' ? resolveStaticAsset('sql-wasm.wasm', baseUrl) : resolveStaticAsset(file, baseUrl)),
        });
        localManifestPromise ??= (async () => {
          const response = await fetch(resolveStaticAsset('manifest.json', baseUrl));
          if (!response.ok) {
            return { db: { path: 'db/codebase.db' } } satisfies StaticManifest;
          }
          return (await response.json()) as StaticManifest;
        })();

        const [SQL, manifest] = await Promise.all([localSqlJsPromise, localManifestPromise]);
        const dbPath = manifest.db?.path ?? 'db/codebase.db';
        const response = await fetch(resolveStaticAsset(dbPath, baseUrl));
        if (!response.ok) {
          throw new Error(`failed to fetch SQLite DB ${dbPath}: ${response.status} ${response.statusText}`);
        }
        const bytes = new Uint8Array(await response.arrayBuffer());
        return new SQL.Database(bytes);
      })();
    }
    return localDbPromise;
  };
}

export async function getStaticDb(): Promise<Database> {
  if (!dbPromise) {
    dbPromise = createStaticDbLoader()();
  }
  return dbPromise;
}

export async function smokeCountCommits(): Promise<number> {
  const db = await getStaticDb();
  const row = queryOne<{ count: number }>(db, 'SELECT COUNT(*) AS count FROM commits');
  return Number(row?.count ?? 0);
}

export function resetStaticDbForTests(): void {
  dbPromise = null;
  manifestPromise = null;
  sqlJsPromise = null;
}
