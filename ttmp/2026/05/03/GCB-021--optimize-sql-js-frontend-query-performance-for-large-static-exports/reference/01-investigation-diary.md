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
    - Path: ui/src/api/sqlJsQueryProvider.ts
      Note: GCB-021 measured source ref bottleneck in provider ref helpers
    - Path: ui/src/api/sqljs/sqlRows.ts
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
