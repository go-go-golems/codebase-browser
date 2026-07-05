---
Title: 'Postmortem: sql.js Source Page Freeze on Large Static Exports'
Ticket: GCB-021
Status: active
Topics:
    - frontend
    - performance
    - sqlite
    - sqljs
    - static-export
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: internal/history/schema.go
      Note: GCB-021 postmortem architecture
    - Path: ttmp/2026/05/03/GCB-021--optimize-sql-js-frontend-query-performance-for-large-static-exports/scripts/01-source-page-cdp-smoke.py
      Note: GCB-021 postmortem architecture
    - Path: ui/src/api/sourceApi.ts
      Note: GCB-021 postmortem architecture
    - Path: ui/src/api/sqlJsQueryProvider.ts
      Note: GCB-021 postmortem architecture
    - Path: ui/src/api/sqljs/sqlJsDb.ts
      Note: GCB-021 postmortem architecture
    - Path: ui/src/api/sqljs/sqlRows.ts
      Note: GCB-021 postmortem architecture
    - Path: ui/src/features/source/FileXrefPanel.tsx
      Note: GCB-021 postmortem architecture
    - Path: ui/src/features/source/SourcePage.tsx
      Note: GCB-021 postmortem architecture
ExternalSources: []
Summary: Detailed postmortem of the full-Glazed static export source-page freeze, including measurements, root cause, fixes, validation, and Worker follow-up design.
LastUpdated: 2026-05-03T17:10:00-04:00
WhatFor: Teach new contributors how the static sql.js browser works, why the source page froze, how it was diagnosed and fixed, and how to extend it safely.
WhenToUse: Use when debugging frontend static-export performance, changing sql.js queries, or planning a Web Worker migration.
---









# Postmortem: sql.js Source Page Freeze on Large Static Exports

## Executive summary

A full static export of the Glazed repository exposed a serious frontend performance failure in codebase-browser. Opening one source file in the browser caused the tab to consume 100% CPU and appear hung. The problematic route was:

```text
http://127.0.0.1:4183/#/source/pkg/help/publish/sqlite_validator.go
```

The root cause was not raw file loading, syntax highlighting, React rendering, or the SQLite database size by itself. The root cause was a source-reference query that selected from the `snapshot_refs` compatibility view. That view expands reference locations with `json_each(ref_versions.locations_json)` and assigns synthetic IDs with `row_number()`. On a large export with 1,577 commits and more than 6 million commit-reference mappings, SQLite planned the query as a broad scan/expansion of `snapshot_refs` before applying the file filter effectively. Native SQLite took about 60 seconds to count 82 reference rows for one file. The same query shape inside sql.js/WebAssembly blocked the browser main thread.

The incident was fixed by changing hot frontend reference queries to use the normalized base tables directly. The fix constrains by commit and file or symbol first, then expands only the relevant `locations_json` rows. The source-reference query fell from roughly 60 seconds native SQLite to milliseconds native SQLite and about 19-28 ms in the browser. The fixed full Glazed export loaded the source page and xref panel in roughly 1.1-2.3 seconds in Playwright, with no page freeze.

A follow-up idea is to move sql.js query execution into a Web Worker. A Worker would not make bad SQL fast, but it would prevent future slow queries from blocking the UI. This postmortem includes a detailed Worker design so a new intern can implement it safely.

## Audience and goals

This document is written for a new intern or engineer who has not worked on codebase-browser before. It explains:

- what the static browser is,
- how data flows from React to sql.js,
- what the history database schema looks like,
- why a seemingly selective query became catastrophically slow,
- how the problem was measured,
- what code changed,
- how the fix was validated,
- what risks remain,
- how to design the next Web Worker improvement.

The goal is to make future performance work evidence-based rather than guess-based.

## Incident timeline

### 1. Large Glazed export created

A full Glazed export was generated to estimate codebase-browser behavior on a normal large repository rather than on a small demo repository.

Important export facts:

```text
Correct repo: /home/manuel/code/wesen/corporate-headquarters/glazed
Branch: main
Git commits in repo: 1578
Indexed commits: 1577
Export DB: /tmp/glazed-full-export/db/codebase.db
DB size: 198.92 MB / about 199 MB
Static export size: 211 MB
Original served URL: http://127.0.0.1:4183/
Fixed served URL: http://127.0.0.1:4184/
```

The indexed commit count is one less than the repository commit count because the two `A..B` ranges used for indexing exclude the root side of the range. The oldest indexed commit was still effectively the initial project commit:

```text
2430ff5210070420cf1bbf66f6c4cb154ecca683
:sparkles: Initial code to get RUM events
```

### 2. User clicked a source file

The problematic route was:

```text
/source/pkg/help/publish/sqlite_validator.go
```

User-visible symptoms:

- browser CPU went to 100%,
- page appeared stuck,
- no clear UI feedback indicated whether it was still working,
- DevTools profiling showed the browser was busy inside sql.js/WebAssembly.

### 3. Firefox/DevTools profiler identified the hot path

The user provided profiler screenshots. The hot path was:

```text
getRefRecordsInFile
  queryAll(...)
    p.prototype.step
      sql-wasm-browser.wasm
```

A selected call node showed roughly 24 seconds in JavaScript/WebAssembly in one capture. This was enough to rule out simple rendering glitches. The page was blocked by synchronous sql.js query execution.

### 4. Native SQLite reproduced the bad query

The source-page reference query was tested against the exported SQLite DB. The `snapshot_refs` query returned only 82 rows but took about 60 seconds in native SQLite:

```sql
SELECT COUNT(*)
FROM snapshot_refs
WHERE commit_hash=(SELECT hash FROM commits ORDER BY sequence DESC LIMIT 1)
  AND file_id='file:pkg/help/publish/sqlite_validator.go';
```

Observed result:

```text
82
Run Time: real 60.285 user 45.081491 sys 5.700637
```

This proved the problem was not only WebAssembly overhead. The SQL query shape was bad even in native SQLite.

### 5. Normalized-table query proved the database could answer quickly

The equivalent direct normalized query constrained by commit and file before expanding JSON locations:

```sql
WITH latest AS (
  SELECT id FROM commits ORDER BY sequence DESC LIMIT 1
),
file AS (
  SELECT id FROM files WHERE stable_id='file:pkg/help/publish/sqlite_validator.go'
)
SELECT COUNT(*)
FROM commit_refs cr
JOIN latest ON latest.id=cr.commit_id
JOIN ref_versions rv ON rv.id=cr.ref_version_id
JOIN file ON file.id=rv.file_id,
     json_each(rv.locations_json) j;
```

Observed result:

```text
82
Run Time: real 0.009 user 0.008321 sys 0.000374
```

This was the key diagnostic result. It showed that the normalized schema had the right data and indexes. The bug was the frontend's choice to query the compatibility view in a hot path.

### 6. Hot frontend reference queries were rewritten

The implementation changed `ui/src/api/sqlJsQueryProvider.ts` so source and xref reference lookups query normalized base tables instead of `snapshot_refs`.

Changed helpers:

- `getRefRecordsInFile`
- `getRefRecordsInFileRange`
- `getRefRecordsToFileSymbols`
- `getRefRecordsFromFileSymbols`
- `getRefRecordsFrom`
- `getRefRecordsTo`

Instrumentation was added to `ui/src/api/sqljs/sqlRows.ts` so future query problems can be diagnosed with `?debugSql`.

### 7. Fixed export was validated

The fixed SPA assets were rebuilt and a new export was created:

```text
/tmp/glazed-full-export-gcb021
http://127.0.0.1:4184/
```

Playwright verified:

```json
{
  "elapsedMs": 2252,
  "hasSource": true,
  "usedBy": "6",
  "uses": "25"
}
```

Browser console `?debugSql` timings showed:

```text
source refs query: 82 rows in 19 ms
file used-by query: 6 rows in 2 ms
file uses query: 70 raw rows in 42 ms, grouped to Uses (25)
```

A second run with a Long Task observer showed readiness in about 1.16 seconds and max long task around 170 ms:

```json
{
  "elapsedMs": 1163,
  "body": [
    "pkg/help/publish/sqlite_validator.go",
    "Used by (6)",
    "Uses (25)"
  ],
  "maxLongTaskMs": 170
}
```

### 8. Old export remained a negative control

The old export at port 4183 still hung on the same route. A Playwright wait for source+xref readiness timed out, and the CDP smoke script also timed out while trying to evaluate in the page. That is consistent with the main thread being blocked by the old sql.js query.

## System overview

### What codebase-browser static export does

A static export bundles three things:

1. a built React application,
2. a SQLite database,
3. a `manifest.json` file.

It can be served by a plain static server:

```bash
cd /tmp/glazed-full-export-gcb021
python3 -m http.server 4184
```

The browser then loads:

```text
index.html
assets/*.js
assets/*.css
manifest.json
db/codebase.db
sql-wasm.wasm
```

No Go server is needed at runtime.

### Runtime data flow

The simplified runtime stack is:

```text
React route component
  ↓
RTK Query API slice
  ↓
SqlJsQueryProvider method
  ↓
queryAll / queryOne helper
  ↓
sql.js Database.prepare
  ↓
sql.js Statement.step loop
  ↓
SQLite tables/views in codebase.db
```

For the source page, the important flow is:

```text
SourcePage
  ├─ useGetSourceQuery(path)
  │    └─ provider.getSource(path)
  │         ├─ resolve HEAD
  │         ├─ find file content hash
  │         └─ fetch content BLOB from file_contents
  │
  ├─ useGetSourceRefsQuery(path)
  │    └─ provider.getSourceRefs(path)
  │         ├─ resolve HEAD
  │         ├─ find file ID
  │         └─ fetch references in that file
  │
  └─ FileXrefPanel(path)
       └─ useGetFileXrefQuery(path)
            └─ provider.getFileXref(path)
                 ├─ fetch refs to symbols declared in this file
                 └─ fetch refs from symbols declared in this file
```

### Why sql.js can block the UI

The TypeScript provider methods are `async`, but sql.js query execution is synchronous once the database is available. The core loop is:

```ts
while (stmt.step()) {
  rows.push(stmt.getAsObject() as T);
}
```

Because this loop runs on the browser main thread today, a slow query blocks:

- painting,
- scrolling,
- clicking,
- React rendering,
- console interaction,
- readiness checks.

This is why the old export looked frozen.

## Database schema overview

The full history index uses a normalized schema. Instead of storing full snapshots for every commit, it stores unique entity versions once and maps commits to them.

### Core tables

From `internal/history/schema.go`:

```text
commits
packages
files
symbols
ref_versions
file_contents
```

Important reference-related tables:

```text
ref_versions
  id
  from_symbol_id
  to_stable_id
  kind
  file_id
  locations_json

commit_refs
  commit_id
  ref_version_id
```

`ref_versions.locations_json` stores one or more source-code locations for a logical reference edge. A simplified row means:

```text
From symbol A, in file F, there are call/reference locations to symbol B.
```

### Mapping tables

```text
commit_packages
commit_files
commit_symbols
commit_refs
```

These answer questions like:

```text
Which file versions exist in this commit?
Which symbol versions exist in this commit?
Which reference versions exist in this commit?
```

### Compatibility views

The schema also exposes views named:

```text
snapshot_packages
snapshot_files
snapshot_symbols
snapshot_refs
symbol_history
```

These are convenient public query interfaces. They reconstruct old snapshot-shaped tables over the normalized storage. This is useful for simple browser queries and for human-readable SQL concepts.

The dangerous one for hot paths is `snapshot_refs`:

```sql
CREATE VIEW snapshot_refs AS
SELECT
    c.hash AS commit_hash,
    row_number() OVER (PARTITION BY c.id ORDER BY rv.id, j.key) AS id,
    s.stable_id AS from_symbol_id,
    rv.to_stable_id AS to_symbol_id,
    rv.kind,
    f.stable_id AS file_id,
    json_extract(j.value, '$.start_line') AS start_line,
    json_extract(j.value, '$.start_col') AS start_col,
    json_extract(j.value, '$.end_line') AS end_line,
    json_extract(j.value, '$.end_col') AS end_col,
    json_extract(j.value, '$.start_offset') AS start_offset,
    json_extract(j.value, '$.end_offset') AS end_offset
FROM commit_refs cr
JOIN commits c ON c.id = cr.commit_id
JOIN ref_versions rv ON rv.id = cr.ref_version_id
JOIN symbols s ON s.id = rv.from_symbol_id
JOIN files f ON f.id = rv.file_id,
    json_each(rv.locations_json) AS j;
```

The view expands every `locations_json` array into one row per location. That is exactly what the UI needs eventually, but the timing of the expansion matters. Expanding before constraining to one commit/file is expensive.

## Scale of the full Glazed export

The full Glazed export was large enough to expose planner and browser-runtime problems:

```text
DB size: 198.92 MB
commits: 1577
packages: 199
files_unique: 2783
file_contents: 2776
symbols_unique: 8659
refs_unique: 127927
commit_files_rows: 172776
commit_symbols_rows: 1733053
commit_refs_rows: 6074525
review_docs: 1
review_snippets: 3
```

Redundancy ratios:

```text
files    mapped=172,776    unique=2,783      redundancy=98.39%
symbols  mapped=1,733,053  unique=8,659      redundancy=99.50%
refs     mapped=6,074,525  unique=127,927    redundancy=97.89%
```

Largest database objects:

```text
commit_refs                      68.98 MB
sqlite_autoindex_ref_versions_1  38.27 MB
ref_versions                     30.02 MB
file_contents                    19.84 MB
commit_symbols                   19.37 MB
idx_ref_to                        9.57 MB
symbols                           3.69 MB
commit_files                      1.92 MB
idx_ref_from                      1.66 MB
```

The important takeaway is that references dominate the large-export footprint. Any source/xref feature must be careful with reference query shape.

## Detailed root cause analysis

### The old source-reference query

The old frontend helper was conceptually:

```ts
private async getRefRecordsInFile(commit: string, fileId: string): Promise<RefRecord[]> {
  const db = await this.getDb();
  return queryAll<RefRecordSQL>(db, refRecordSelectSQL + `
    WHERE commit_hash = ? AND file_id = ?
    ORDER BY start_offset, end_offset
  `, [commit, fileId]).map(toRefRecord);
}
```

where `refRecordSelectSQL` was:

```sql
SELECT ...
FROM snapshot_refs
```

At first glance this looks selective because it has both `commit_hash` and `file_id`. The query should only return refs in one file at one commit. But the query planner had to deal with a view that includes `row_number()` and `json_each()`. The observed plan scanned and expanded too much of the underlying reference data before the outer filter became useful.

### Why the view is expensive

The view asks SQLite to compute a table shaped like this:

```text
for each commit_ref row:
  join commit
  join ref_version
  join from-symbol
  join file
  for each location in locations_json:
    emit one row
  assign row_number within commit
```

The filter from the outer query is:

```sql
WHERE commit_hash = ? AND file_id = ?
```

But because the view has a window function and JSON virtual table expansion, SQLite cannot always push those predicates into the deepest loops as desired. The result is a broad scan over `commit_refs` and `json_each`.

### Better query shape

The fast shape is:

```text
resolve current commit id
resolve target file id
walk commit_refs for that commit
join ref_versions
filter ref_versions.file_id
only then expand locations_json
```

Pseudocode:

```pseudo
commitID = SELECT id FROM commits WHERE hash = $commit
fileID = SELECT id FROM files WHERE stable_id = $fileStableID

for each ref_version mapped to commitID:
    if ref_version.file_id != fileID:
        continue
    for each location in json_each(ref_version.locations_json):
        emit RefRecord
```

This keeps the expensive JSON expansion local to a small set of reference versions.

### General rule learned

For hot paths, especially in sql.js:

> Do not use broad compatibility views when a query can be expressed against normalized base tables with integer IDs and selective joins.

Compatibility views remain useful, but they are not automatically safe for all runtime routes.

## Implementation details

### Query instrumentation

File:

```text
ui/src/api/sqljs/sqlRows.ts
```

Added responsibilities:

- Detect `?debugSql` in `window.location.search`.
- Log query start when debug is enabled.
- Log query completion with elapsed time and row count.
- Log slow queries above 1000 ms even without debug mode.
- Log errors with SQL preview and parameters.
- Compact BLOB parameters.

Conceptual implementation:

```ts
const slowQueryThresholdMs = 1000;

function isSqlDebugEnabled(): boolean {
  const location = globalThis.location;
  return !!location && new URLSearchParams(location.search).has('debugSql');
}

export function queryAll<T>(db, sql, params = []): T[] {
  const debug = isSqlDebugEnabled();
  const started = performance.now();

  if (debug) {
    console.warn('[sql.js:start]', { sql: compactSql(sql), params: compactParams(params) });
  }

  const stmt = db.prepare(sql);
  try {
    stmt.bind(params);
    const rows = [];
    while (stmt.step()) rows.push(stmt.getAsObject());

    const elapsedMs = performance.now() - started;
    if (debug || elapsedMs >= slowQueryThresholdMs) {
      console.warn('[sql.js:done]', { elapsedMs, rows: rows.length, sql: compactSql(sql) });
    }
    return rows;
  } catch (error) {
    console.error('[sql.js:error]', { elapsedMs: performance.now() - started, sql: compactSql(sql), params, error });
    throw error;
  } finally {
    stmt.free();
  }
}
```

This instrumentation is intentionally simple. It does not solve blocking by itself; it makes future blocking queries visible.

### Normalized reference record select

File:

```text
ui/src/api/sqlJsQueryProvider.ts
```

The old shared select selected from `snapshot_refs`. The new shared select joins normalized tables directly:

```sql
SELECT s.stable_id AS fromSymbolId,
       rv.to_stable_id AS toSymbolId,
       rv.kind,
       f.stable_id AS fileId,
       json_extract(j.value, '$.start_line') AS startLine,
       json_extract(j.value, '$.start_col') AS startCol,
       json_extract(j.value, '$.end_line') AS endLine,
       json_extract(j.value, '$.end_col') AS endCol,
       json_extract(j.value, '$.start_offset') AS startOffset,
       json_extract(j.value, '$.end_offset') AS endOffset
FROM commits c
JOIN commit_refs cr
  ON cr.commit_id = c.id
JOIN ref_versions rv
  ON rv.id = cr.ref_version_id
JOIN symbols s
  ON s.id = rv.from_symbol_id
JOIN files f
  ON f.id = rv.file_id
JOIN json_each(rv.locations_json) j
```

The selected aliases still match `RefRecordSQL`, so `toRefRecord()` does not need to know whether data came from a view or base tables.

### Source refs query

New shape:

```sql
SELECT ...
FROM normalized joins
WHERE c.hash = ?
  AND f.stable_id = ?
ORDER BY startOffset, endOffset
```

This is the critical source-page fix.

### Snippet refs query

Symbol snippets need refs only within a byte range:

```sql
SELECT *
FROM (
  SELECT ...
  FROM normalized joins
  WHERE c.hash = ?
    AND f.stable_id = ?
) refs
WHERE refs.startOffset >= ?
  AND refs.endOffset <= ?
ORDER BY refs.startOffset, refs.endOffset
```

This preserves the old API behavior but avoids `snapshot_refs`.

### File used-by query

The `usedBy` side asks:

```text
Which symbols outside this file reference symbols declared in this file?
```

Fast query strategy:

1. Build `current_commit` CTE.
2. Build `target_symbols` CTE for symbols declared in the file at that commit.
3. Use `idx_ref_to` to find incoming refs by target stable ID.
4. Join `commit_refs` to ensure the ref version exists in the current commit.
5. Exclude refs whose source symbol's file is the same file.
6. Expand only matching `locations_json`.

The implementation uses:

```sql
JOIN ref_versions rv INDEXED BY idx_ref_to
  ON rv.to_stable_id = target.stable_id
```

This nudges SQLite toward the incoming-reference index.

### File uses query

The `uses` side asks:

```text
Which external symbols are referenced by symbols declared in this file?
```

Fast query strategy:

1. Build `current_commit` CTE.
2. Build `source_symbols` CTE for symbols declared in the file at that commit.
3. Build `target_symbols` CTE for local target-symbol file lookup.
4. Use `idx_ref_from` to find outgoing refs from each source symbol.
5. Join `commit_refs` to ensure current commit membership.
6. Exclude targets that resolve to symbols in the same file.
7. Expand only matching `locations_json`.

The implementation uses:

```sql
JOIN ref_versions rv INDEXED BY idx_ref_from
  ON rv.from_symbol_id = source.id
```

### Why tests mattered

A first version of the file `uses` rewrite produced a false positive: it returned an intra-file reference that should have been excluded. The bug came from a direct `LEFT JOIN commit_symbols` shape that could produce null rows passing this filter:

```sql
COALESCE(target_file.stable_id, '') != ?
```

The fix was to build a `target_symbols` CTE for the current commit and left join by target stable ID once. This made intra-file exclusion deterministic.

Regression tests were added to:

```text
ui/src/api/sqlJsQueryProvider.test.ts
```

They cover:

- source refs loaded from normalized tables,
- snippet refs clipped to a symbol range,
- file xrefs excluding intra-file refs.

## Measurements before and after

### Before fix

Native SQLite through `snapshot_refs`:

```text
source refs count for sqlite_validator.go: 82 rows
runtime: 60.285 s
```

Browser behavior:

```text
Old export: http://127.0.0.1:4183/
Route: /source/pkg/help/publish/sqlite_validator.go
Observed: browser CPU 100%, page appears hung
Playwright: readiness timed out
CDP script: timed out trying to evaluate in page
```

Profiler evidence:

```text
Hot path: getRefRecordsInFile → stmt.step → sql-wasm-browser.wasm
Selected call: about 24 seconds in JavaScript/WebAssembly in screenshot
```

### After fix

Native SQLite equivalent timings:

```text
source_refs: 82 rows, 0.008 s
used_by:      6 rows, 1.193 s after join-order/index hint improvement
uses:        70 rows, 0.379 s after join-order/index hint improvement
```

Browser `?debugSql` timings in fixed export:

```text
source refs: 82 rows in 19 ms
file used-by refs: 6 rows in 2 ms
file uses refs: 70 raw rows in 42 ms, grouped to Uses (25)
```

Playwright readiness:

```json
{
  "elapsedMs": 2252,
  "hasSource": true,
  "usedBy": "6",
  "uses": "25"
}
```

Long task observer:

```json
{
  "elapsedMs": 1163,
  "body": [
    "pkg/help/publish/sqlite_validator.go",
    "Used by (6)",
    "Uses (25)"
  ],
  "maxLongTaskMs": 170
}
```

### Remaining visible cost

The route still performs repeated commit-list queries:

```text
SELECT hash AS Hash, short_hash AS ShortHash, ... FROM commits WHERE error = '' ORDER BY sequence DESC, author_time DESC
```

Observed browser timings:

```text
30-40 ms per repeated call
1577 rows returned
called multiple times during route load
```

This is not the current bottleneck, but it is a future cleanup opportunity. The provider or RTK Query layer should eventually deduplicate or cache commit resolution/listing more aggressively.

## Validation commands

### Unit/frontend tests

```bash
pnpm --dir ui test
pnpm --dir ui typecheck
```

Observed:

```text
Test Files  4 passed
Tests       15 passed
```

### Go tests

```bash
GOWORK=off go test ./...
```

Observed: all packages passed.

### Build and export

```bash
GOWORK=off make build

bin/codebase-browser review export \
  --db /tmp/glazed-full.db \
  --out /tmp/glazed-full-export-gcb021 \
  --repo-root /home/manuel/code/wesen/corporate-headquarters/glazed \
  --strict-docs
```

Observed:

```text
Export complete: /tmp/glazed-full-export-gcb021
211M /tmp/glazed-full-export-gcb021
```

### Serve fixed export

```bash
cd /tmp/glazed-full-export-gcb021
python3 -m http.server 4184
```

### Playwright route check

Conceptual Playwright script:

```ts
const url = 'http://127.0.0.1:4184/?debugSql&v=gcb021-run2#/source/pkg/help/publish/sqlite_validator.go';
const t0 = Date.now();
await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });
await page.waitForSelector('[data-part="source-view"]', { timeout: 30000 });
await page.waitForFunction(() =>
  document.body.innerText.includes('Used by (') &&
  document.body.innerText.includes('Uses ('),
  null,
  { timeout: 30000 },
);
const elapsedMs = Date.now() - t0;
```

Observed:

```text
elapsedMs: 2252
hasSource: true
Used by: 6
Uses: 25
```

### CDP smoke script

Script path:

```text
ttmp/2026/05/03/GCB-021--optimize-sql-js-frontend-query-performance-for-large-static-exports/scripts/01-source-page-cdp-smoke.py
```

Purpose:

- launch Chromium headless,
- navigate to a static export URL,
- wait for source and xref readiness,
- capture body preview and some console events,
- provide a dependency-light browser smoke check.

## File reference map

### Frontend runtime

- `ui/src/api/sqljs/sqlJsDb.ts`
  - Loads `manifest.json`.
  - Loads `db/codebase.db`.
  - Initializes sql.js and creates `SQL.Database`.
  - Important for a future Worker migration because DB ownership should move to the Worker.

- `ui/src/api/sqljs/sqlRows.ts`
  - Central sql.js query execution helper.
  - Contains `queryAll`, `queryOne`, BLOB conversion, and UTF-8 range extraction.
  - Now contains `?debugSql` and slow-query instrumentation.

- `ui/src/api/sqlJsQueryProvider.ts`
  - Main static DB query provider.
  - Converts route-level provider calls into SQL queries.
  - Contains source, symbol, xref, history, impact, review-doc methods.
  - Now uses normalized-table SQL for hot reference helpers.

- `ui/src/api/sourceApi.ts`
  - RTK Query API slice for source features.
  - Public types include `SourceRefView`, `FileXrefResponse`, and `FileXrefUseTarget`.

- `ui/src/features/source/SourcePage.tsx`
  - Source route component.
  - Requests source text, source refs, index metadata, and file xrefs.

- `ui/src/features/source/FileXrefPanel.tsx`
  - Displays file-level used-by and uses lists.
  - Consumes `getFileXref` results.

### Backend/export schema

- `internal/history/schema.go`
  - Defines normalized tables and compatibility views.
  - `ref_versions`, `commit_refs`, `idx_ref_from`, `idx_ref_to`, and `snapshot_refs` are central to this incident.

### Ticket artifacts

- `ttmp/2026/05/03/GCB-021--optimize-sql-js-frontend-query-performance-for-large-static-exports/design-doc/01-frontend-sql-js-performance-analysis-and-implementation-guide.md`
  - Initial design/implementation guide.

- `ttmp/2026/05/03/GCB-021--optimize-sql-js-frontend-query-performance-for-large-static-exports/reference/01-investigation-diary.md`
  - Chronological diary with commands, errors, and measurements.

- `ttmp/2026/05/03/GCB-021--optimize-sql-js-frontend-query-performance-for-large-static-exports/scripts/01-source-page-cdp-smoke.py`
  - Chromium CDP smoke validator.

## Design lessons

### Lesson 1: Views are not free abstraction boundaries

A SQLite view can make queries readable, but it can also hide expensive work from the caller. `snapshot_refs` looked like a normal table from the frontend's perspective. In reality it expands JSON arrays and computes window functions.

Guideline:

```text
Use compatibility views for broad or low-volume queries.
Use normalized base tables for hot paths and large result domains.
```

### Lesson 2: Measure in native SQLite and in the browser

Native SQLite answered whether the SQL shape was fundamentally bad. Browser profiling answered whether sql.js was blocking the main thread. Both measurements were necessary.

Useful sequence:

```text
1. Browser profiler identifies hot method.
2. Add ?debugSql to identify exact query.
3. Run query in native sqlite3 with .timer and .eqp.
4. Rewrite query.
5. Re-measure native sqlite3.
6. Rebuild/export.
7. Re-measure browser with Playwright and console logs.
```

### Lesson 3: Preserve provider API shapes while changing SQL internals

The UI consumes `SourceRefView`, `FileXrefResponse`, and `RefRecord`-like shapes. The fix preserved those shapes and changed only provider internals. This kept React components stable.

### Lesson 4: Tests catch subtle SQL join bugs

The initial file `uses` rewrite looked plausible but incorrectly included an intra-file reference. A small fixture test caught it. SQL rewrites should have semantic tests, not only performance tests.

### Lesson 5: Worker migration is a resilience improvement, not a query optimizer

Moving sql.js into a Worker would have kept the UI responsive during the old 60-second query, but the result would still arrive after 60 seconds. The right order is:

```text
1. Fix query shape.
2. Add Worker to isolate future slow queries.
```

## Follow-up design: move sql.js into a Web Worker

### Why do this?

Even after fixing current hot queries, sql.js still runs on the main thread. Future features may introduce slow queries accidentally. A Web Worker would isolate SQLite execution from the UI thread.

Benefits:

- UI remains scrollable/clickable during slow queries.
- Loading indicators can animate while queries run.
- Long query timeouts can terminate/recreate the Worker.
- The architecture becomes safer for large exports.

Non-benefits:

- It does not make bad SQL fast.
- It does not remove the need for indexes and query-shape care.
- It may increase complexity around initialization, errors, and cancellation.

### Target architecture

```text
Main thread                                      Worker thread
───────────────────────────────────────          ─────────────────────────────

React component
  ↓
RTK Query endpoint
  ↓
WorkerSqlJsQueryProvider
  ↓ postMessage({ id, method, args }) ─────────▶ sqlJsQueryWorker
                                                   ↓
                                                 SqlJsQueryProvider
                                                   ↓
                                                 SQL.Database
                                                   ↓
                                                 stmt.step()
  ◀──────── postMessage({ id, ok, result }) ───── result serialized back
  ↓
RTK Query resolves
  ↓
React renders
```

### Provider interface

Introduce an interface that both direct and Worker-backed providers implement:

```ts
export interface CodebaseQueryProvider {
  getIndex(): Promise<IndexSummary>;
  getPackageLites(): Promise<PackageLite[]>;
  getSymbol(id: string): Promise<Symbol>;
  searchSymbols(query: string, kind?: string): Promise<Symbol[]>;

  getSource(path: string, commitRef?: string): Promise<string>;
  getSourceRefs(path: string, commitRef?: string): Promise<SourceRefView[]>;
  getFileXref(path: string, commitRef?: string): Promise<FileXrefResponse>;

  getSnippet(symbolId: string, kind?: string, commitRef?: string): Promise<string>;
  getSnippetRefs(symbolId: string, commitRef?: string): Promise<SnippetRefView[]>;

  listCommits(): Promise<CommitRow[]>;
  getCommit(ref: string): Promise<CommitRow>;
  getCommitDiff(from: string, to: string): Promise<CommitDiff>;
  getSymbolHistory(symbolId: string): Promise<SymbolHistoryEntry[]>;
  getSymbolBodyDiff(from: string, to: string, symbolId: string): Promise<BodyDiffResult>;
  getImpact(request: ImpactRequest): Promise<ImpactResponse>;

  listReviewDocs(): Promise<ReviewDocMeta[]>;
  getReviewDoc(slug: string): Promise<DocPage>;
}
```

Then the existing API slices can keep calling:

```ts
getSqlJsProvider().getSourceRefs(path)
```

without caring whether the provider is direct or Worker-backed.

### Worker request/response protocol

Request:

```ts
type WorkerRequest = {
  id: number;
  method: string;
  args: unknown[];
};
```

Success response:

```ts
type WorkerSuccess = {
  id: number;
  ok: true;
  result: unknown;
  timing?: {
    method: string;
    elapsedMs: number;
  };
};
```

Error response:

```ts
type WorkerFailure = {
  id: number;
  ok: false;
  error: {
    name: string;
    message: string;
    code?: string;
    details?: unknown;
    stack?: string;
  };
};
```

### Worker entrypoint pseudocode

```ts
// sqlJsQueryWorker.ts

const provider = new SqlJsQueryProvider(workerDbLoader);

self.onmessage = async (event) => {
  const { id, method, args } = event.data;
  const started = performance.now();

  try {
    const fn = provider[method];
    if (typeof fn !== 'function') throw new Error(`unknown method ${method}`);

    const result = await fn.apply(provider, args);

    self.postMessage({
      id,
      ok: true,
      result,
      timing: { method, elapsedMs: performance.now() - started },
    });
  } catch (error) {
    self.postMessage({
      id,
      ok: false,
      error: serializeError(error),
    });
  }
};
```

### Main-thread Worker client pseudocode

```ts
let worker: Worker | null = null;
let nextID = 1;
const pending = new Map<number, PendingRequest>();

function getWorker(): Worker {
  if (!worker) {
    worker = new Worker(new URL('./sqlJsQueryWorker.ts', import.meta.url), {
      type: 'module',
    });

    worker.onmessage = (event) => {
      const { id, ok, result, error, timing } = event.data;
      const request = pending.get(id);
      if (!request) return;
      pending.delete(id);

      if (timing && debugSqlEnabled()) {
        console.warn('[sql.js-worker:done]', timing);
      }

      if (ok) request.resolve(result);
      else request.reject(deserializeError(error));
    };
  }
  return worker;
}

function callWorker<T>(method: string, args: unknown[]): Promise<T> {
  const id = nextID++;
  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject });
    getWorker().postMessage({ id, method, args });
  });
}
```

### Worker-backed provider pseudocode

```ts
class WorkerSqlJsQueryProvider implements CodebaseQueryProvider {
  getSource(path: string, commitRef = 'HEAD') {
    return callWorker<string>('getSource', [path, commitRef]);
  }

  getSourceRefs(path: string, commitRef = 'HEAD') {
    return callWorker<SourceRefView[]>('getSourceRefs', [path, commitRef]);
  }

  getFileXref(path: string, commitRef = 'HEAD') {
    return callWorker<FileXrefResponse>('getFileXref', [path, commitRef]);
  }

  // Repeat for the remaining provider methods.
}
```

### DB ownership in the Worker

The Worker should own the `SQL.Database` instance. The main thread should not load the 199 MB DB if the Worker is enabled.

Preferred Worker loading sequence:

```text
Worker starts
  ↓
fetch manifest.json
  ↓
fetch db/codebase.db
  ↓
init sql.js
  ↓
new SQL.Database(bytes)
  ↓
serve queries
```

The main thread may send a `baseURL` during initialization so relative asset paths resolve correctly:

```ts
worker.postMessage({
  type: 'init',
  baseURL: document.baseURI,
});
```

Worker helper:

```ts
function resolveAsset(path: string): string {
  return new URL(path, baseURL).toString();
}
```

### Worker fallback and debugging

Add URL toggles:

```text
?noSqlWorker   force direct main-thread provider
?debugSql      show SQL/Worker timings
```

Provider selection:

```ts
export function getSqlJsProvider(): CodebaseQueryProvider {
  if (!provider) {
    provider = shouldUseWorker()
      ? new WorkerSqlJsQueryProvider()
      : new SqlJsQueryProvider();
  }
  return provider;
}

function shouldUseWorker(): boolean {
  const params = new URLSearchParams(window.location.search);
  if (params.has('noSqlWorker')) return false;
  return typeof Worker !== 'undefined';
}
```

### Cancellation strategy

True SQLite interruption is not the first goal. Start with simple cancellation semantics:

1. If a caller no longer cares about a request, reject/forget the pending promise.
2. If the Worker is wedged, terminate and recreate it.

Pseudocode:

```ts
function resetWorker(reason: string): void {
  worker?.terminate();
  worker = null;

  for (const request of pending.values()) {
    request.reject(new Error(`sql.js worker reset: ${reason}`));
  }
  pending.clear();
}
```

This is heavy because the DB must reload, but it is predictable.

### Worker migration phases

Recommended implementation phases:

1. **Interface phase**
   - Add `CodebaseQueryProvider`.
   - Make current `SqlJsQueryProvider` implement it.
   - No behavior change.

2. **Worker skeleton phase**
   - Add Worker entrypoint.
   - Add RPC client.
   - Implement `listCommits`, `getSource`, and `getSourceRefs` first.

3. **Full provider phase**
   - Move all provider methods behind Worker proxy.
   - Keep `?noSqlWorker` fallback.

4. **Instrumentation phase**
   - Forward Worker method timings to main thread.
   - Preserve `?debugSql` SQL timings inside Worker.

5. **Validation phase**
   - Re-run Glazed source route.
   - Add intentional slow-query smoke test.
   - Verify UI remains responsive while Worker is busy.

## Future cleanup opportunities

### 1. Deduplicate commit-list queries

Observed repeated `listCommits()` calls on source-page load. They are currently fast enough but wasteful.

Possible fixes:

- Cache resolved commit refs inside provider.
- Avoid calling `listCommits()` from `resolveCommitRef()` repeatedly.
- Let RTK Query share a single commit-list result more explicitly.

### 2. Add source-page phased loading

Current source page triggers source text, refs, index, and file xrefs together. Consider progressive loading:

```text
Phase 1: source text
Phase 2: source refs / identifier links
Phase 3: file xref panel
```

This would improve perceived performance even when all queries are fast.

### 3. Add query budget tests

A script could assert that known hot queries stay below thresholds on the full Glazed DB:

```text
source refs <= 100 ms browser sql.js
file xrefs <= 500 ms browser sql.js
source+xref page readiness <= 3000 ms
```

### 4. Add explicit query-plan regression checks

For native SQLite, store `EXPLAIN QUERY PLAN` expectations for hot queries to prevent accidental reintroduction of `snapshot_refs` scans.

### 5. Consider precomputed static xref summaries

If very large repositories still produce heavy xref panels, static export could optionally add summary tables. This would trade DB size for frontend speed.

## Appendix A: Key commands from the investigation

Native slow query:

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

Native fast query:

```bash
sqlite3 /tmp/glazed-full-export/db/codebase.db <<'SQL'
.timer on
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

Frontend tests:

```bash
pnpm --dir ui test
pnpm --dir ui typecheck
```

Go tests:

```bash
GOWORK=off go test ./...
```

Build and export:

```bash
GOWORK=off make build
bin/codebase-browser review export \
  --db /tmp/glazed-full.db \
  --out /tmp/glazed-full-export-gcb021 \
  --repo-root /home/manuel/code/wesen/corporate-headquarters/glazed \
  --strict-docs
```

Serve:

```bash
cd /tmp/glazed-full-export-gcb021
python3 -m http.server 4184
```

Browser URL:

```text
http://127.0.0.1:4184/?debugSql&v=gcb021#/source/pkg/help/publish/sqlite_validator.go
```

## Appendix B: Quick checklist for future sql.js performance bugs

When a static export route feels slow or frozen:

1. Open the route with `?debugSql`.
2. Capture console query timings.
3. If the page freezes before logs complete, profile with browser DevTools.
4. Identify provider method and SQL fragment.
5. Run the query against the exported DB with:

```sql
.timer on
.eqp on
```

6. Check whether the query uses a compatibility view that expands too much.
7. Rewrite hot path against normalized base tables.
8. Add fixture tests for semantic correctness.
9. Rebuild/export.
10. Validate with Playwright and record timings in the ticket diary.

## Appendix C: Final status

As of this postmortem:

- GCB-021 has a design guide, diary, tasks, changelog, and this postmortem.
- The immediate source-page freeze is fixed.
- The fixed full Glazed export is available at:

```text
/tmp/glazed-full-export-gcb021
http://127.0.0.1:4184/
```

- The old export at port 4183 remains a useful negative control.
- The next recommended architecture improvement is a Worker-backed sql.js provider with `?noSqlWorker` fallback.
