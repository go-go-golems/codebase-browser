---
Title: Implementation diary
Ticket: GCB-022
Status: active
Topics:
    - frontend
    - performance
    - sqlite
    - sqljs
    - static-export
    - wasm
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ui/src/api/docApi.ts
      Note: GCB-022 implementation evidence
    - Path: ui/src/api/historyApi.ts
      Note: GCB-022 implementation evidence
    - Path: ui/src/api/indexApi.ts
      Note: GCB-022 implementation evidence
    - Path: ui/src/api/queryProvider.ts
      Note: GCB-022 implementation evidence
    - Path: ui/src/api/sourceApi.ts
      Note: GCB-022 implementation evidence
    - Path: ui/src/api/sqlJsProviderRegistry.ts
      Note: GCB-022 implementation evidence
    - Path: ui/src/api/sqlJsQueryProvider.ts
      Note: GCB-022 implementation evidence
    - Path: ui/src/api/sqljs/sqlJsDb.ts
      Note: GCB-022 implementation evidence
    - Path: ui/src/api/sqljs/sqlJsQueryWorker.ts
      Note: GCB-022 implementation evidence
    - Path: ui/src/api/sqljs/workerClient.ts
      Note: GCB-022 implementation evidence
    - Path: ui/src/api/sqljs/workerProtocol.ts
      Note: GCB-022 implementation evidence
    - Path: ui/src/api/workerSqlJsQueryProvider.ts
      Note: GCB-022 implementation evidence
    - Path: ui/src/api/xrefApi.ts
      Note: GCB-022 implementation evidence
ExternalSources: []
Summary: Chronological diary for moving static sql.js execution into a Web Worker.
LastUpdated: 2026-05-03T17:30:00-04:00
WhatFor: Record implementation steps, commands, errors, and validation evidence for GCB-022.
WhenToUse: Use during review and follow-up work.
---














# Implementation diary

## Goal

Move static sql.js query execution off the browser main thread and into a Web Worker while preserving the existing provider API and retaining a `?noSqlWorker` fallback.

## Entries

### 2026-05-03 — Step 1: Ticket setup and plan

#### What changed

- Created ticket `GCB-022`.
- Added phased task list in `tasks.md`.
- Added `design-doc/01-web-worker-sql-js-execution-plan.md`.
- Created this diary.

#### Why

GCB-021 fixed the slow source/xref query shape, but sql.js still runs on the main thread. A Worker-backed provider is the next architectural improvement to prevent future slow queries from freezing the UI.

#### Next actions

Start Phase 1 by adding a provider interface and moving singleton selection into a registry module.

### 2026-05-03 — Step 2: Implement Worker-backed provider

#### What changed

- Added `ui/src/api/queryProvider.ts` with the `CodebaseQueryProvider` interface.
- Added `ui/src/api/sqlJsProviderRegistry.ts`, which selects a Worker-backed provider by default and preserves direct mode with `?noSqlWorker`.
- Added Worker RPC files:
  - `ui/src/api/sqljs/workerProtocol.ts`
  - `ui/src/api/sqljs/workerClient.ts`
  - `ui/src/api/sqljs/sqlJsQueryWorker.ts`
  - `ui/src/api/workerSqlJsQueryProvider.ts`
- Updated `ui/src/api/sqljs/sqlJsDb.ts` with `createStaticDbLoader({ baseUrl })` so the Worker can load `manifest.json`, `db/codebase.db`, and `sql-wasm.wasm` relative to the static export.
- Updated API slices and widgets to import `getSqlJsProvider` from the registry instead of the concrete provider file.

#### Commands

```bash
pnpm --dir ui typecheck
pnpm --dir ui test
GOWORK=off make build
bin/codebase-browser review export --db /tmp/glazed-full.db --out /tmp/glazed-full-export-gcb022 --repo-root /home/manuel/code/wesen/corporate-headquarters/glazed --strict-docs
GOWORK=off go test ./...
```

#### Results

- `pnpm --dir ui typecheck`: passed.
- `pnpm --dir ui test`: 4 files / 15 tests passed.
- `GOWORK=off make build`: passed and emitted `assets/sqlJsQueryWorker-*.js`, confirming Vite bundled the Worker.
- Full Glazed export completed at `/tmp/glazed-full-export-gcb022` and is served at `http://127.0.0.1:4185/`.
- `GOWORK=off go test ./...`: passed.

#### Browser validation

Worker-enabled URL:

```text
http://127.0.0.1:4185/?debugSql&v=gcb022#/source/pkg/help/publish/sqlite_validator.go
```

Playwright observed Worker method logs:

```text
[sql.js-worker:start] getIndex
[sql.js-worker:start] listReviewDocs
[sql.js-worker:start] getSource
[sql.js-worker:start] getSourceRefs
[sql.js-worker:done] listReviewDocs 828ms
[sql.js-worker:done] getIndex 885ms
[sql.js-worker:done] getSource 904ms
[sql.js-worker:done] getSourceRefs 904ms
[sql.js-worker:start] getFileXref
[sql.js-worker:done] getFileXref 42ms
```

The first four calls include initial Worker DB loading, so their timings are dominated by fetching and opening the ~199 MB SQLite file. Subsequent query calls are much faster.

A long-task Playwright run reached source+xref readiness in 968 ms and reported no main-thread long tasks:

```json
{
  "elapsedMs": 968,
  "longTasks": [],
  "maxLongTaskMs": 0,
  "textOk": true
}
```

Fallback URL:

```text
http://127.0.0.1:4185/?debugSql&noSqlWorker&v=gcb022-direct#/source/pkg/help/publish/sqlite_validator.go
```

Fallback validation result:

```json
{
  "elapsedMs": 916,
  "textOk": true
}
```

#### What worked

- The Worker provider preserved the existing RTK Query and React component contracts.
- Vite bundled the Worker as a separate `sqlJsQueryWorker-*.js` asset.
- Worker-owned DB loading works in the static export.
- `?noSqlWorker` fallback works.

#### What was tricky

- The provider singleton had to move out of `sqlJsQueryProvider.ts` to avoid Worker/main-thread import cycles. The concrete provider is now separate from the registry that chooses direct vs Worker mode.
- The Worker needs an explicit `baseUrl` so `manifest.json`, `db/codebase.db`, and `sql-wasm.wasm` resolve relative to the exported site rather than relative to the Worker asset.

#### Remaining work

- Add an intentional slow-query debug harness to prove the UI remains interactive during a long Worker query.
- Add Worker reset/terminate behavior for wedged queries.
- Consider caching repeated commit-list queries as a separate cleanup.
