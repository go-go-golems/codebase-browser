---
Title: Investigation diary
Ticket: GCB-021
Status: active
Topics:
    - frontend
    - performance
    - sqlite
    - sqljs
    - static-export
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: internal/history/schema.go
    - Path: ttmp/2026/05/03/GCB-021--optimize-sql-js-frontend-query-performance-for-large-static-exports/scripts/01-source-page-cdp-smoke.py
      Note: GCB-021 implementation and validation evidence
    - Path: ui/src/api/sqlJsQueryProvider.test.ts
      Note: GCB-021 implementation and validation evidence
    - Path: ui/src/api/sqlJsQueryProvider.ts
      Note: |-
        GCB-021 measured source ref bottleneck in provider ref helpers
        GCB-021 implementation and validation evidence
    - Path: ui/src/api/sqljs/sqlRows.ts
      Note: GCB-021 implementation and validation evidence
    - Path: ui/src/features/source/SourcePage.tsx
ExternalSources: []
Summary: Chronological investigation diary for frontend sql.js large-export performance work.
LastUpdated: 2026-05-03T16:38:05.058894386-04:00
WhatFor: Record what was measured, changed, validated, and learned while optimizing sql.js frontend queries.
WhenToUse: Use during GCB-021 implementation, review, and handoff.
---






# Investigation diary

## Goal

Improve frontend performance for large static exports where sql.js queries can block the browser main thread. The immediate reproduction is the full Glazed export at `http://127.0.0.1:4183/` and the source route `/source/pkg/help/publish/sqlite_validator.go`.

## Chronological entries

### 2026-05-03 — Step 1: Create ticket and capture initial evidence

#### What changed

- Created ticket `GCB-021` with title `Optimize sql.js Frontend Query Performance for Large Static Exports`.
- Created the primary design document:
  - `design-doc/01-frontend-sql-js-performance-analysis-and-implementation-guide.md`
- Created this diary:
  - `reference/01-investigation-diary.md`

#### Evidence and commands

The user provided profiler screenshots after opening:

```text
http://127.0.0.1:4183/#/source/pkg/help/publish/sqlite_validator.go
```

The profile showed a hot path through:

```text
getRefRecordsInFile
  p.prototype.step
  sql-wasm-browser.wasm
```

I inspected source files and line references with:

```bash
nl -ba ui/src/api/sqljs/sqlRows.ts | sed -n '1,80p'
nl -ba ui/src/api/sqlJsQueryProvider.ts | sed -n '240,285p;690,760p;780,805p'
nl -ba ui/src/features/source/SourcePage.tsx | sed -n '1,70p'
nl -ba internal/history/schema.go | sed -n '90,145p;185,245p'
```

I also measured the problematic query against the exported Glazed DB:

```bash
sqlite3 /tmp/glazed-full-export/db/codebase.db <<'SQL'
.timer on
.eqp on
SELECT COUNT(*)
FROM snapshot_refs
WHERE commit_hash=(SELECT hash FROM commits ORDER BY sequence DESC LIMIT 1)
  AND file_id='file:pkg/help/publish/sqlite_validator.go';
SQL
```

Result:

```text
82
Run Time: real 60.285 user 45.081491 sys 5.700637
```

Then I tested the normalized-base-table shape:

```bash
sqlite3 /tmp/glazed-full-export/db/codebase.db <<'SQL'
.timer on
.eqp on
WITH latest AS (SELECT id FROM commits ORDER BY sequence DESC LIMIT 1),
file AS (SELECT id FROM files WHERE stable_id='file:pkg/help/publish/sqlite_validator.go')
SELECT COUNT(*)
FROM commit_refs cr
JOIN latest ON latest.id=cr.commit_id
JOIN ref_versions rv ON rv.id=cr.ref_version_id
JOIN file ON file.id=rv.file_id,
     json_each(rv.locations_json) j;
SQL
```

Result:

```text
82
Run Time: real 0.009 user 0.008321 sys 0.000374
```

#### What worked

- The profiler screenshots identified the exact frontend method: `getRefRecordsInFile`.
- Native SQLite reproduced the bad query shape and showed why the browser was blocked.
- A direct normalized query proved that the database has enough structure to answer the query quickly.

#### What did not work

- The `snapshot_refs` compatibility view is not suitable for hot browser source-ref lookups at full Glazed scale.
- The frontend currently has no built-in SQL timing logs, so diagnosing this required external profiling and manual SQLite queries.

#### What was tricky

- `snapshot_refs` looks selective at the outer query level (`WHERE commit_hash=? AND file_id=?`), but the query plan showed SQLite expanding/scanning too much inside the view before the filter becomes useful.
- The TypeScript methods are `async`, but sql.js `stmt.step()` is synchronous and blocks the main thread once entered.

#### Next actions

1. Add `?debugSql` instrumentation to `queryAll`.
2. Rewrite hot reference queries in `SqlJsQueryProvider` to use normalized base tables.
3. Validate against the full Glazed export.

## Code review instructions

When reviewing GCB-021 changes, check that:

- hot source/xref queries no longer select from `snapshot_refs`,
- result aliases still match `RefRecordSQL`,
- public API types in `sourceApi.ts` do not change unless necessary,
- debug SQL logging is disabled unless `?debugSql` is present or a query exceeds the slow threshold,
- large-export validation timings are recorded in this diary.

### 2026-05-03 — Step 2: Validate docs and upload guide to reMarkable

#### What changed

- Related the design document and diary to the key frontend and schema files through docmgr.
- Ran `docmgr doctor --ticket GCB-021 --stale-after 30`.
- Added missing vocabulary topics `frontend` and `sqljs`.
- Uploaded the design guide bundle to reMarkable.
- Marked task T3 complete.

#### Commands

```bash
docmgr doctor --ticket GCB-021 --stale-after 30
docmgr vocab add --category topics --slug frontend --description "Frontend browser application code, UI runtime behavior, and client-side data fetching."
docmgr vocab add --category topics --slug sqljs --description "sql.js SQLite WebAssembly runtime used by static browser exports."
remarquee upload bundle --dry-run ... --remote-dir /ai/2026/05/03/GCB-021
remarquee upload bundle ... --remote-dir /ai/2026/05/03/GCB-021
remarquee cloud ls /ai/2026/05/03/GCB-021 --long --non-interactive
```

#### Result

```text
## Doctor Report (1 findings)
### GCB-021
- ✅ All checks passed

OK: uploaded GCB-021 Frontend sql.js Performance Guide.pdf -> /ai/2026/05/03/GCB-021
[f] GCB-021 Frontend sql.js Performance Guide
```

#### What worked

- The ticket documentation validates cleanly after vocabulary updates.
- The reMarkable upload was verified by listing the destination folder.

#### Next actions

Start implementation with SQL timing instrumentation in `ui/src/api/sqljs/sqlRows.ts`.

### 2026-05-03 — Step 3: Implement sql.js instrumentation and normalized ref hot-path queries

#### What changed

- Added timing instrumentation to `ui/src/api/sqljs/sqlRows.ts`:
  - `?debugSql` logs query start/done/error records.
  - Queries over 1000 ms emit a slow-query warning even without `?debugSql`.
  - Blob parameters are compacted as `<blob:N>` to avoid dumping large data into the console.
- Rewrote reference hot paths in `ui/src/api/sqlJsQueryProvider.ts`:
  - `getRefRecordsInFile`
  - `getRefRecordsInFileRange`
  - `getRefRecordsToFileSymbols`
  - `getRefRecordsFromFileSymbols`
  - `getRefRecordsFrom`
  - `getRefRecordsTo`
- Added `ui/src/api/sqlJsQueryProvider.test.ts` fixture coverage for:
  - source refs loaded from normalized tables,
  - snippet refs clipped to a symbol body range,
  - file xrefs excluding intra-file refs.
- Added CDP smoke script:
  - `scripts/01-source-page-cdp-smoke.py`

#### Commands

```bash
pnpm --dir ui test -- sqlJsQueryProvider.test.ts
pnpm --dir ui typecheck
GOWORK=off make build
bin/codebase-browser review export --db /tmp/glazed-full.db --out /tmp/glazed-full-export-gcb021 --repo-root /home/manuel/code/wesen/corporate-headquarters/glazed --strict-docs
GOWORK=off go test ./...
python3 ttmp/2026/05/03/GCB-021--optimize-sql-js-frontend-query-performance-for-large-static-exports/scripts/01-source-page-cdp-smoke.py 'http://127.0.0.1:4184/?debugSql&v=gcb021#/source/pkg/help/publish/sqlite_validator.go' --timeout 60
```

#### Native SQLite timing check

Before the rewrite, counting source refs through `snapshot_refs` took roughly 60 seconds for 82 rows.

After the query-shape rewrite, equivalent normalized native timings were:

```text
source_refs: 82 rows, 0.008s
used_by:      6 rows, 1.193s after join-order/index hint improvement
uses:        70 rows, 0.379s after join-order/index hint improvement
```

The source-page freeze was caused by the source refs query, so the critical path improved from roughly 60 seconds native to milliseconds native.

#### Browser validation result

The Chromium CDP smoke script loaded:

```text
http://127.0.0.1:4184/?debugSql&v=gcb021#/source/pkg/help/publish/sqlite_validator.go
```

Result excerpt:

```json
{
  "elapsedSeconds": 2.003,
  "ready": true,
  "bodyPreview": "Codebase Browser ... pkg/help/publish/sqlite_validator.go ... func ValidateSQLiteHelpDB ..."
}
```

The page reached a source/xref-ready state in about two seconds in headless Chromium instead of appearing hung.

#### What worked

- Query-shape changes were enough to make the source page usable without moving sql.js to a Web Worker.
- Provider tests caught an incorrect first version of the `getRefRecordsFromFileSymbols` rewrite: the query included an intra-file ref because a `LEFT JOIN commit_symbols` multiplied target candidates and produced null rows that passed the `COALESCE(..., '') != fileId` filter.
- Rewriting the file-xref queries to start from `idx_ref_to` / `idx_ref_from` improved native timings for the xref panel.

#### What did not work

- A first Chromium `--dump-dom` attempt did not wait for the React/sql.js app to finish rendering; it only printed the initial shell.
- The first CDP attempt failed with Chromium's remote origin protection:

```text
Handshake status 403 Forbidden ... Use the command line flag --remote-allow-origins=...
```

The script now launches Chromium with `--remote-allow-origins=*`.

#### What was tricky

- The CDP `Runtime.consoleAPICalled` events can be dropped when synchronous `Runtime.evaluate` calls are used for readiness polling; the script is still useful for route readiness and body validation, but console timing capture may need refinement if exact browser-side query logs are required.
- The file `usedBy` query is inherently more expensive than source-local refs because it starts from target symbols and searches incoming references across the commit. It is now much faster than the view-based shape, but still worth monitoring on very large files.

#### Code review instructions

Reviewers should pay special attention to the SQL in `getRefRecordsFromFileSymbols`: it intentionally builds a current-commit `target_symbols` CTE and compares target file IDs there, rather than joining all `commit_symbols` rows directly. This avoids false positives for intra-file refs.

### 2026-05-03 — Step 4: Commit implementation

#### What changed

- Committed the frontend performance implementation.
- Marked task T18 complete.

#### Command

```bash
git commit -m "Optimize sqljs ref queries for large exports"
```

#### Result

```text
[task/better-frontend-ui <amended>] Optimize sqljs ref queries for large exports
```

### 2026-05-03 — Step 5: Upload updated post-implementation bundle

#### What changed

- Uploaded the updated design guide, diary, and tasks after implementation and validation.

#### Commands

```bash
remarquee upload bundle --dry-run ... --name "GCB-021 Frontend sql.js Performance Guide - Updated" --remote-dir /ai/2026/05/03/GCB-021
remarquee upload bundle ... --name "GCB-021 Frontend sql.js Performance Guide - Updated" --remote-dir /ai/2026/05/03/GCB-021
remarquee cloud ls /ai/2026/05/03/GCB-021 --long --non-interactive
```

#### Result

```text
OK: uploaded GCB-021 Frontend sql.js Performance Guide - Updated.pdf -> /ai/2026/05/03/GCB-021
[f] GCB-021 Frontend sql.js Performance Guide
[f] GCB-021 Frontend sql.js Performance Guide - Updated
```

### 2026-05-03 — Step 6: Playwright/browser performance verification

#### What changed

- Verified the fixed full Glazed export in Playwright at `http://127.0.0.1:4184/`.
- Compared behavior with the pre-fix export at `http://127.0.0.1:4183/`.

#### Commands and observations

Playwright loaded the fixed source page:

```text
http://127.0.0.1:4184/?debugSql&v=gcb021-run2#/source/pkg/help/publish/sqlite_validator.go
```

Readiness result:

```json
{
  "elapsedMs": 2252,
  "hasSource": true,
  "usedBy": "6",
  "uses": "25"
}
```

The `?debugSql` console timings showed the formerly blocking source-ref query returning quickly:

```text
source refs: 82 rows in 19 ms
file used-by refs: 6 rows in 2 ms
file uses refs: 70 raw rows in 42 ms, grouped to Uses (25)
```

A second Playwright run with a Long Task observer reached source+xref readiness in about 1.16 seconds and reported a maximum long task of about 170 ms:

```json
{
  "elapsedMs": 1163,
  "body": ["pkg/help/publish/sqlite_validator.go", "Used by (6)", "Uses (25)"],
  "maxLongTaskMs": 170
}
```

The pre-fix export at port 4183 still reproduced the hang. A Playwright run waiting for the same source+xref readiness timed out, and the CDP smoke script against 4183 also timed out while trying to evaluate in the page, consistent with the browser main thread being blocked by the old sql.js query.

#### What worked

- The fixed export renders the source page and xref panel quickly enough for interactive use.
- `?debugSql` now gives actionable per-query timings directly in the browser console.
- The old export remains a useful negative control because it still hangs on the same route.

#### Remaining caveat

The fixed page still performs repeated commit-list queries while route data loads. They are not currently the bottleneck, but repeated `listCommits()` calls may be worth caching or deduplicating in a later frontend cleanup ticket.

### 2026-05-03 — Step 7: Write full postmortem report

#### What changed

- Added `design-doc/02-postmortem-sql-js-source-page-freeze-on-large-static-exports.md`.
- The report consolidates the full incident: large Glazed export scale, profiler evidence, native SQLite timings, browser `?debugSql` timings, Playwright verification, root cause, implementation details, and the proposed Web Worker follow-up architecture.
- Related the postmortem to the main frontend provider, sql.js DB/query helpers, source UI files, schema file, and CDP smoke script.

#### Why

The implementation guide explained the plan and the diary recorded chronological actions. The postmortem is meant to be the single onboarding artifact a new intern can read to understand the incident end-to-end and continue with Worker migration or further sql.js performance work.

#### Next actions

- Validate the ticket with `docmgr doctor`.
- Upload the postmortem bundle to reMarkable.

### 2026-05-03 — Step 8: Upload postmortem to reMarkable

#### What changed

- Uploaded the new postmortem report, diary, and tasks as a bundled PDF to reMarkable.

#### Commands

```bash
remarquee upload bundle --dry-run \
  design-doc/02-postmortem-sql-js-source-page-freeze-on-large-static-exports.md \
  reference/01-investigation-diary.md \
  tasks.md \
  --name "GCB-021 sql.js Source Freeze Postmortem" \
  --remote-dir /ai/2026/05/03/GCB-021 \
  --toc-depth 2

remarquee upload bundle ...
remarquee cloud ls /ai/2026/05/03/GCB-021 --long --non-interactive
```

#### Result

```text
OK: uploaded GCB-021 sql.js Source Freeze Postmortem.pdf -> /ai/2026/05/03/GCB-021
[f] GCB-021 Frontend sql.js Performance Guide
[f] GCB-021 Frontend sql.js Performance Guide - Updated
[f] GCB-021 sql.js Source Freeze Postmortem
```

#### What worked

- The dry-run succeeded before the real upload.
- The remote listing confirms the postmortem is present on reMarkable.
