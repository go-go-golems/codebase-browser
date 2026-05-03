---
Title: ""
Ticket: ""
Status: ""
Topics: []
DocType: ""
Intent: ""
Owners: []
RelatedFiles:
    - Path: ttmp/2026/05/03/GCB-024--static-browser-performance-hardening-after-sql-js-worker-migration/scripts/01-source-page-browser-smoke.py
      Note: GCB-024 performance hardening implementation and validation
    - Path: ui/src/api/sqlJsQueryProvider.test.ts
      Note: GCB-024 performance hardening implementation and validation
    - Path: ui/src/api/sqlJsQueryProvider.ts
      Note: GCB-024 performance hardening implementation and validation
    - Path: ui/src/api/sqljs/workerClient.ts
      Note: GCB-024 performance hardening implementation and validation
ExternalSources: []
Summary: ""
LastUpdated: 0001-01-01T00:00:00Z
WhatFor: ""
WhenToUse: ""
---





# Implementation Diary

## 2026-05-03 — Start

GCB-024 continues the GCB-021/GCB-022 frontend performance work. The goal is to harden the static browser after moving sql.js into a Worker: reduce repeated commit metadata queries, add Worker recovery behavior, add a regression check for `snapshot_refs` on hot frontend paths, and export a fresh large Glazed bundle for manual testing.

## 2026-05-03 — Implementation and validation

### Changes

- `ui/src/api/sqlJsQueryProvider.ts` now caches immutable commit metadata at provider scope:
  - `listCommits()` runs the commit-list SQL once per provider instance.
  - `resolveCommitRef(...)` memoizes successful ref resolutions, including `HEAD`.
  - `getCommit(...)` memoizes successful commit lookups by ref.
- `ui/src/api/sqljs/workerClient.ts` now has explicit Worker reset behavior and per-request timeout handling. The timeout defaults to 60000 ms and can be overridden with `?sqlWorkerTimeoutMs=<ms>` for debugging. On timeout or Worker error, the client terminates the Worker, rejects pending requests, clears state, and lets the next request create a fresh Worker.
- `ui/src/api/sqlJsQueryProvider.test.ts` now verifies commit metadata caching and asserts that `sqlJsQueryProvider.ts` does not contain `snapshot_refs`.
- `scripts/01-source-page-browser-smoke.py` provides a reusable CDP smoke test for source routes and captures `?debugSql` console output.

### Validation commands

```bash
pnpm --dir ui typecheck
pnpm --dir ui test
GOWORK=off make build
GOWORK=off go test ./...
bin/codebase-browser review export --db /tmp/glazed-full.db --out /tmp/glazed-full-export-gcb024 --repo-root /home/manuel/code/wesen/corporate-headquarters/glazed --strict-docs
```

### Export result

```text
Export: /tmp/glazed-full-export-gcb024
Server: http://127.0.0.1:4186/
Manifest commits: 1577
Manifest DB size: 208592896 bytes
Export elapsed: 0:48.33
```

### Browser smoke results

Worker URL:

```text
http://127.0.0.1:4186/?debugSql&v=gcb024#/source/pkg/help/publish/sqlite_validator.go
```

Result:

```json
{
  "elapsedSeconds": 4.835,
  "ready": true
}
```

Captured Worker timings included:

```text
[sql.js-worker:done] listReviewDocs 873ms
[sql.js-worker:done] getIndex 973ms
[sql.js-worker:done] getSource 991ms
[sql.js-worker:done] getFileXref 37ms
```

Direct fallback URL:

```text
http://127.0.0.1:4186/?debugSql&noSqlWorker&v=gcb024-direct#/source/pkg/help/publish/sqlite_validator.go
```

Result:

```json
{
  "elapsedSeconds": 1.989,
  "ready": true
}
```

The direct fallback console showed only one commit-list query in the captured startup window, which is the expected result of provider-level commit caching.
