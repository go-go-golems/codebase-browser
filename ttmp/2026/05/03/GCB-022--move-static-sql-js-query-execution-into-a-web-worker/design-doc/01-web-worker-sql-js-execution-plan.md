---
Title: Web Worker sql.js Execution Plan
Ticket: GCB-022
Status: active
Topics:
    - frontend
    - performance
    - sqlite
    - sqljs
    - static-export
    - wasm
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ui/src/api/docApi.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
    - Path: ui/src/api/historyApi.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
    - Path: ui/src/api/indexApi.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
    - Path: ui/src/api/queryProvider.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
    - Path: ui/src/api/sourceApi.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
    - Path: ui/src/api/sqlJsProviderRegistry.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
    - Path: ui/src/api/sqlJsQueryProvider.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
    - Path: ui/src/api/sqljs/sqlJsDb.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
    - Path: ui/src/api/sqljs/sqlJsQueryWorker.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
    - Path: ui/src/api/sqljs/sqlRows.ts
    - Path: ui/src/api/sqljs/workerClient.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
    - Path: ui/src/api/sqljs/workerProtocol.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
    - Path: ui/src/api/workerSqlJsQueryProvider.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
    - Path: ui/src/api/xrefApi.ts
      Note: GCB-022 Worker-backed sql.js provider architecture
ExternalSources: []
Summary: Plan for moving static sql.js query execution off the browser main thread into a Web Worker.
LastUpdated: 2026-05-03T17:30:00-04:00
WhatFor: Guide implementation of a Worker-backed query provider for static codebase-browser exports.
WhenToUse: Use while implementing or reviewing GCB-022.
---














# Web Worker sql.js Execution Plan

## Executive summary

GCB-021 fixed the immediate large-export source-page freeze by rewriting hot reference queries. GCB-022 addresses the next architectural risk: sql.js still runs on the browser main thread. Even optimized queries can create long tasks, and a future bad query can freeze the UI again. The goal is to move SQLite execution into a Web Worker while preserving the existing provider API used by React and RTK Query.

The design has three important constraints:

1. The Worker should own the `SQL.Database` instance so the 199 MB Glazed database is not loaded twice.
2. The main-thread API should continue to look like `getSqlJsProvider().getSourceRefs(path)` so React components and API slices remain stable.
3. A `?noSqlWorker` fallback should keep the current direct provider available for debugging and compatibility.

## Current architecture

```text
React component
  ↓
RTK Query endpoint
  ↓
SqlJsQueryProvider
  ↓
queryAll/queryOne
  ↓
sql.js stmt.step()
  ↓
SQLite database
```

The blocking point is `stmt.step()`, which is synchronous. If it runs on the main thread, the UI cannot repaint while a query is executing.

## Target architecture

```text
React component
  ↓
RTK Query endpoint
  ↓
WorkerSqlJsQueryProvider proxy
  ↓ postMessage
sql.js Worker
  ↓
SqlJsQueryProvider
  ↓
queryAll/queryOne
  ↓
SQL.Database
  ↓ postMessage result
RTK Query resolves
```

The Worker does not make slow SQL fast. It changes the failure mode from "frozen page" to "loading state while background work continues." Query optimization remains mandatory.

## Provider contract

Add a `CodebaseQueryProvider` interface that captures the public methods consumed by API slices and widgets. `SqlJsQueryProvider` implements it directly; `WorkerSqlJsQueryProvider` implements it by RPC.

Pseudocode:

```ts
export interface CodebaseQueryProvider {
  getIndex(): Promise<IndexSummary>;
  getPackageLites(): Promise<PackageLite[]>;
  getSymbol(id: string): Promise<Symbol>;
  searchSymbols(query: string, kind?: string): Promise<Symbol[]>;
  getSource(path: string, commitRef?: string): Promise<string>;
  getSnippet(symbolId: string, kind?: string, commitRef?: string): Promise<string>;
  getSnippetRefs(symbolId: string, commitRef?: string): Promise<SnippetRefView[]>;
  getSourceRefs(path: string, commitRef?: string): Promise<SourceRefView[]>;
  getFileXref(path: string, commitRef?: string): Promise<FileXrefResponse>;
  getXref(symbolId: string, commitRef?: string): Promise<XrefResponse>;
  listCommits(): Promise<CommitRow[]>;
  resolveCommitRef(ref: string): Promise<string>;
  getCommit(ref: string): Promise<CommitRow>;
  getSymbolHistory(symbolId: string): Promise<SymbolHistoryEntry[]>;
  getSymbolBodyDiff(from: string, to: string, symbolId: string): Promise<BodyDiffResult>;
  getCommitDiff(from: string, to: string): Promise<CommitDiff>;
  getImpact(options: ImpactOptions): Promise<ImpactResponse>;
  listReviewDocs(): Promise<ReviewDocMeta[]>;
  getReviewDoc(slug: string): Promise<DocPage>;
}
```

## RPC protocol

Requests use numeric IDs so multiple calls can be in flight:

```ts
type WorkerRequest = {
  id: number;
  method: string;
  args: unknown[];
};
```

Responses are either success or failure:

```ts
type WorkerResponse =
  | { id: number; ok: true; result: unknown; timing?: WorkerTiming }
  | { id: number; ok: false; error: SerializedProviderError };
```

Errors must preserve `QueryError` fields:

```ts
type SerializedProviderError = {
  name?: string;
  message: string;
  code?: string;
  details?: unknown;
  stack?: string;
};
```

## Worker DB loading

The Worker needs a base URL because relative paths inside a bundled Worker can differ from document-relative paths. The main thread sends `document.baseURI` or `window.location.href` at Worker initialization. The Worker resolves:

- `manifest.json`,
- `db/codebase.db`,
- `sql-wasm.wasm`.

Pseudocode:

```ts
const dbLoader = createStaticDbLoader({ baseUrl });
const provider = new SqlJsQueryProvider(dbLoader);
```

## Rollout plan

1. Add interface and registry without changing behavior.
2. Add Worker RPC and proxy.
3. Enable Worker by default when `Worker` exists.
4. Keep `?noSqlWorker` for fallback.
5. Validate the fixed Glazed source route and compare long tasks.

## Risks

- Worker bundling can break asset paths for `sql-wasm.wasm`; use explicit `baseUrl` resolution.
- Error objects lose prototypes through structured clone; serialize and deserialize `QueryError` explicitly.
- Loading the DB twice wastes memory; ensure direct provider is not initialized when Worker is active.
- True SQLite cancellation is not solved. Terminating and recreating the Worker is the first reset strategy.
