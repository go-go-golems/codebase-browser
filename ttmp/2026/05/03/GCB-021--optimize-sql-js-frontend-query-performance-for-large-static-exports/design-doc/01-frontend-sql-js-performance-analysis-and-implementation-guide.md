---
Title: Frontend sql.js Performance Analysis and Implementation Guide
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
      Note: GCB-021 sql.js frontend performance architecture evidence
    - Path: ui/src/api/sourceApi.ts
      Note: GCB-021 sql.js frontend performance architecture evidence
    - Path: ui/src/api/sqlJsQueryProvider.ts
      Note: GCB-021 sql.js frontend performance architecture evidence
    - Path: ui/src/api/sqljs/sqlRows.ts
      Note: GCB-021 sql.js frontend performance architecture evidence
    - Path: ui/src/features/source/FileXrefPanel.tsx
      Note: GCB-021 sql.js frontend performance architecture evidence
    - Path: ui/src/features/source/SourcePage.tsx
      Note: GCB-021 sql.js frontend performance architecture evidence
ExternalSources: []
Summary: Analysis and implementation guide for fixing large-export sql.js source/xref query stalls.
LastUpdated: 2026-05-03T16:38:04.98423457-04:00
WhatFor: Onboard an engineer to the static frontend sql.js query stack and guide implementation of large-export performance fixes.
WhenToUse: Use when optimizing source, symbol, xref, or review-browser queries in static codebase-browser exports.
---







# Frontend sql.js Performance Analysis and Implementation Guide

## Executive Summary

The static codebase-browser frontend runs entirely in the browser. It downloads a SQLite database, loads it through `sql.js`, and serves source, symbol, xref, history, and review pages through TypeScript query provider methods. This architecture is valuable because exported review sites are portable and require only a static HTTP server. The tradeoff is that every SQLite query runs inside WebAssembly on the browser main thread today. A query that is acceptable in native SQLite can freeze the browser when it expands millions of rows in `sql.js`.

A full Glazed export exposed this boundary. Opening:

```text
http://127.0.0.1:4183/#/source/pkg/help/publish/sqlite_validator.go
```

pegged the browser CPU. Firefox profiling showed the hot path was:

```text
getRefRecordsInFile
  queryAll(...)
    stmt.step()
      sql-wasm-browser.wasm
```

The blocking query comes from `SqlJsQueryProvider.getSourceRefs()`, which asks for all references in the opened source file so the `SourceView` can linkify identifiers. The implementation currently queries the compatibility view `snapshot_refs`. That view expands `ref_versions.locations_json` with `json_each()` and assigns synthetic IDs with `row_number()`. On the full Glazed export, a native SQLite count through `snapshot_refs` for one file took roughly 60 seconds even though it returned only 82 rows.

The same lookup against normalized base tables took roughly 9 ms because it constrained the query by `commit_id` and `file_id` before expanding `locations_json`. The proposed fix is therefore not to abandon sql.js. The proposed fix is to treat `snapshot_refs` as a broad compatibility/query-browser interface and add purpose-built frontend queries over normalized base tables for hot source/xref paths.

This guide explains the system from first principles, then gives a phased implementation plan for an intern:

1. Add low-overhead frontend SQL timing instrumentation behind `?debugSql`.
2. Rewrite file-local source reference lookup to use base tables directly.
3. Rewrite file xref lookups to avoid `snapshot_refs` scans.
4. Add regression tests for query SQL shape and result parity.
5. Validate against the full Glazed export and record timings.
6. Consider Web Worker migration as a later resilience step, not the first fix.

## Problem Statement and Scope

### User-visible problem

On large static exports, clicking a source file can appear to hang. The browser CPU goes to 100%, and the UI does not tell the user whether anything is still working. In the observed Glazed run, the page was opened at:

```text
/source/pkg/help/publish/sqlite_validator.go
```

The source text itself is cheap to load. The slow part is the source reference overlay: the frontend tries to fetch every reference occurrence in that file so identifiers can become links to target symbols.

### Technical problem

The frontend executes this query path synchronously:

```text
SourcePage
  useGetSourceRefsQuery(path)
    sourceApi.getSourceRefs
      SqlJsQueryProvider.getSourceRefs(path)
        getRefRecordsInFile(commit, fileId)
          queryAll(db, SELECT ... FROM snapshot_refs WHERE commit_hash=? AND file_id=?)
            while stmt.step()
```

The `snapshot_refs` view is convenient but expensive. It expands the normalized reference table into one row per reference location for every commit. SQLite does not push the file filter deeply enough through the view, so it scans a very large commit/ref space before returning a small file-specific result.

### Scope

This ticket focuses on frontend static-export runtime performance for sql.js queries. It includes:

- SQL timing instrumentation in TypeScript.
- Query rewrites for source file references and file xrefs.
- Tests and validation using the large Glazed export.
- Documentation and a diary of commands, errors, and measurements.

This ticket does not initially include:

- Changing the normalized history schema.
- Moving all sql.js work into a Web Worker.
- Adding server-side APIs for static exports.
- Removing `snapshot_refs` globally.

Those may become follow-up work, but the immediate fix is to make hot queries avoid the expensive view.

## Glossary

- **Static export**: A directory containing the built React app, `manifest.json`, and a SQLite database. It can be served with `python3 -m http.server`.
- **sql.js**: SQLite compiled to WebAssembly. It runs SQLite queries inside the browser.
- **Main thread**: The browser thread that handles rendering, input, JavaScript, and current sql.js execution. Long synchronous queries freeze it.
- **Normalized schema**: Database shape where commits map to deduplicated packages, files, symbols, references, and file contents.
- **Compatibility view**: A SQLite view named like `snapshot_refs` that reconstructs old snapshot table shapes for query compatibility.
- **Reference version**: A row in `ref_versions` representing a logical source symbol, target symbol, kind, file, and a JSON array of locations.
- **Location expansion**: `json_each(ref_versions.locations_json)`, which turns a JSON array into one row per source-code occurrence.

## Current-State Architecture

### High-level runtime

```text
Browser route
  ↓
React page component
  ↓
RTK Query API slice
  ↓
SqlJsQueryProvider method
  ↓
queryAll / queryOne helper
  ↓
sql.js Database.prepare / Statement.step
  ↓
SQLite tables and views inside exported codebase.db
```

The important architectural point is that every layer above SQLite is synchronous once `stmt.step()` begins. Even though the TypeScript methods are `async`, they call synchronous sql.js APIs after the DB promise resolves. The browser cannot repaint or respond to clicks until `stmt.step()` finishes.

### Low-level query helper

`ui/src/api/sqljs/sqlRows.ts` contains the shared SQL execution helper. Lines 8-24 show the current `queryAll` implementation:

```ts
export function queryAll<T extends SqlRow = SqlRow>(
  db: Database,
  sql: string,
  params: SqlValue[] = [],
): T[] {
  const stmt = db.prepare(sql);
  try {
    stmt.bind(params);
    const rows: T[] = [];
    while (stmt.step()) {
      rows.push(stmt.getAsObject() as T);
    }
    return rows;
  } finally {
    stmt.free();
  }
}
```

This is the right central location for instrumentation because every provider query flows through it. It currently does not log query duration, row count, or slow-query warnings. `queryOne` delegates to `queryAll` at lines 26-32, so instrumentation in `queryAll` covers both one-row and many-row queries.

### Source page data flow

`ui/src/features/source/SourcePage.tsx` is the route component for `/source/...`.

Key evidence:

- Lines 15-18 request both raw source and source refs.
- Lines 32-42 pass `refs` into `SourceView` for linkification.
- Line 43 renders `FileXrefPanel`, which triggers separate file-level xref queries.

Simplified flow:

```tsx
const { data } = useGetSourceQuery(path);
const { data: refs } = useGetSourceRefsQuery(path);
...
<SourceView source={data} refs={refs} renderRefLink={...} />
<FileXrefPanel path={path} />
```

This means opening a source file currently starts at least three categories of work:

1. load source bytes,
2. load source refs for inline identifier links,
3. load file xrefs for the lower panel.

The first category is cheap. The second and third can be expensive because they query references.

### Source API boundary

`ui/src/api/sourceApi.ts` defines RTK Query endpoints. Lines 55-69 wire the route to provider methods:

```ts
getSource:     getSqlJsProvider().getSource(path)
getSourceRefs: getSqlJsProvider().getSourceRefs(path)
getFileXref:   getSqlJsProvider().getFileXref(path)
```

This API boundary is useful because the UI does not know whether refs come from a compatibility view, base tables, a precomputed table, or a future Web Worker. The implementation can change inside `SqlJsQueryProvider` without changing the React component contract.

### Provider methods for source refs and file xrefs

`ui/src/api/sqlJsQueryProvider.ts` contains the frontend query provider.

Source refs:

- Lines 263-272 implement `getSourceRefs(path)`.
- It resolves `HEAD`, resolves the file to a stable file ID, calls `getRefRecordsInFile`, and maps rows to `SourceRefView`.

File xrefs:

- Lines 275-285 begin `getFileXref(path)`.
- It calls `getRefRecordsToFileSymbols` and `getRefRecordsFromFileSymbols`.
- The result is grouped into `usedBy` and `uses` for `FileXrefPanel`.

Reference record helpers:

- Lines 694-699: `getRefRecordsFrom` uses `snapshot_refs`.
- Lines 702-707: `getRefRecordsTo` uses `snapshot_refs`.
- Lines 710-715: `getRefRecordsInFile` uses `snapshot_refs`.
- Lines 718-726: `getRefRecordsInFileRange` uses `snapshot_refs`.
- Lines 729-742: `getRefRecordsToFileSymbols` joins `snapshot_refs` to `snapshot_symbols`.
- Lines 745-758: `getRefRecordsFromFileSymbols` joins `snapshot_refs` to `snapshot_symbols`.

The shared SQL fragment at lines 784-800 confirms both ref helper families select from `snapshot_refs`.

### Normalized schema

The normalized history schema is in `internal/history/schema.go`.

Important pieces:

- `ref_versions` stores unique ref shapes and `locations_json` (lines 101-115).
- `commit_refs` maps a commit to ref versions (lines 141-145 and nearby mapping-table section).
- `snapshot_refs` expands refs with `json_each` (lines 214-235).

The compatibility view is useful because many browser queries can treat the database as if it still had one snapshot table per commit. But its shape is hazardous for selective ref lookups because it expands every matching `commit_refs` row into location rows and then filters at the outer query level.

## Observed Evidence From the Glazed Export

### Export scale

The full Glazed static export used for investigation has:

```text
DB path: /tmp/glazed-full-export/db/codebase.db
DB size: 198.92 MB
commits: 1577
packages: 199
files_unique: 2783
symbols_unique: 8659
refs_unique: 127927
commit_refs_rows: 6074525
```

Redundancy remained high, which confirms the normalized backend is doing the right thing:

```text
files:   98.39% redundant across snapshots
symbols: 99.50% redundant across snapshots
refs:    97.89% redundant across snapshots
```

### Slow query evidence

The query shape used by `getRefRecordsInFile` is effectively:

```sql
SELECT ...
FROM snapshot_refs
WHERE commit_hash = ?
  AND file_id = ?
ORDER BY start_offset, end_offset;
```

For `pkg/help/publish/sqlite_validator.go`, native SQLite returned 82 rows but took about 60 seconds through `snapshot_refs`.

The query plan showed a scan and view expansion path:

```text
CO-ROUTINE snapshot_refs
  SCAN cr
  SEARCH rv USING INTEGER PRIMARY KEY
  SEARCH s USING INTEGER PRIMARY KEY
  SEARCH c USING INTEGER PRIMARY KEY
  SEARCH f USING INTEGER PRIMARY KEY
  SCAN json_each(...)
  USE TEMP B-TREE FOR ORDER BY
SCAN snapshot_refs
USE TEMP B-TREE FOR ORDER BY
```

### Fast equivalent query evidence

The equivalent base-table query first constrains the latest commit and file, then expands only matching `locations_json`:

```sql
WITH latest AS (
  SELECT id FROM commits ORDER BY sequence DESC LIMIT 1
),
file AS (
  SELECT id FROM files WHERE stable_id = 'file:pkg/help/publish/sqlite_validator.go'
)
SELECT COUNT(*)
FROM commit_refs cr
JOIN latest ON latest.id = cr.commit_id
JOIN ref_versions rv ON rv.id = cr.ref_version_id
JOIN file ON file.id = rv.file_id,
     json_each(rv.locations_json) j;
```

Native SQLite returned the same count in about 9 ms.

The rule is therefore:

> For hot reference queries, constrain by integer IDs before calling `json_each`.

## Proposed Architecture

### Design principle

Keep the compatibility views for broad browser features and SQL-query discoverability, but stop using `snapshot_refs` in hot frontend source/xref paths.

The frontend provider should have two query families:

```text
Compatibility queries
  - simple package/file/symbol lists
  - docs and review pages
  - ad hoc query browser concepts
  - acceptable when row counts are modest

Hot-path normalized queries
  - refs in one file
  - refs in one symbol body range
  - refs to symbols declared in one file
  - refs from symbols declared in one file
  - symbol xrefs and impact graph traversal
```

### Proposed helper vocabulary

Add named SQL fragments or helper functions to make intent obvious:

```ts
const normalizedRefRecordSelectSQL = `... FROM commits c JOIN commit_refs cr ...`;

private async getRefRecordsInFile(commitHash: string, fileStableID: string): Promise<RefRecord[]>;
private async getRefRecordsInFileRange(commitHash: string, fileStableID: string, start: number, end: number): Promise<RefRecord[]>;
private async getRefRecordsToFileSymbols(commitHash: string, fileStableID: string): Promise<RefRecord[]>;
private async getRefRecordsFromFileSymbols(commitHash: string, fileStableID: string): Promise<RefRecord[]>;
```

Keep the public API unchanged:

```ts
getSourceRefs(path: string, commitRef = 'HEAD'): Promise<SourceRefView[]>
getFileXref(path: string, commitRef = 'HEAD'): Promise<FileXrefResponse>
```

The UI should not care how references are fetched.

### Query rewrite: refs in one file

Current problematic shape:

```sql
SELECT ... FROM snapshot_refs
WHERE commit_hash = ? AND file_id = ?
ORDER BY start_offset, end_offset;
```

Proposed shape:

```sql
SELECT
  s.stable_id AS fromSymbolId,
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
JOIN files f
  ON f.id = rv.file_id
JOIN symbols s
  ON s.id = rv.from_symbol_id
JOIN json_each(rv.locations_json) j
WHERE c.hash = ?
  AND f.stable_id = ?
ORDER BY startOffset, endOffset;
```

This query uses the `commit_refs` primary key on `(commit_id, ref_version_id)` and the `files` primary key after resolving the stable file ID. The critical difference is that only refs for one commit and one file are expanded.

### Query rewrite: refs in a symbol body range

`getSnippetRefs` uses `getRefRecordsInFileRange`. The same normalized query can be extended:

```sql
WHERE c.hash = ?
  AND f.stable_id = ?
  AND json_extract(j.value, '$.start_offset') >= ?
  AND json_extract(j.value, '$.end_offset') <= ?
ORDER BY startOffset, endOffset;
```

This still expands only one file's refs for one commit.

### Query rewrite: file used-by

The `usedBy` side asks: Which external symbols reference symbols declared in this file?

Pseudocode:

```text
1. Resolve commit hash to commit ID.
2. Resolve file stable ID to file row ID.
3. Find target symbols in that file at that commit.
4. Join refs whose to_stable_id equals those target symbol stable IDs.
5. Join source symbol metadata to exclude refs whose source file is the same file.
6. Expand locations_json only for matching refs.
```

SQL sketch:

```sql
WITH target_symbols AS (
  SELECT s.stable_id
  FROM commits c
  JOIN commit_symbols cs ON cs.commit_id = c.id
  JOIN symbols s ON s.id = cs.symbol_id
  JOIN files f ON f.id = s.file_id
  WHERE c.hash = ? AND f.stable_id = ?
)
SELECT ...
FROM commits c
JOIN commit_refs cr ON cr.commit_id = c.id
JOIN ref_versions rv ON rv.id = cr.ref_version_id
JOIN symbols source ON source.id = rv.from_symbol_id
JOIN files source_file ON source_file.id = source.file_id
JOIN target_symbols target ON target.stable_id = rv.to_stable_id
JOIN files ref_file ON ref_file.id = rv.file_id
JOIN json_each(rv.locations_json) j
WHERE c.hash = ?
  AND source_file.stable_id != ?
ORDER BY source.stable_id, rv.kind;
```

### Query rewrite: file uses

The `uses` side asks: Which external symbols are referenced by symbols declared in this file?

Pseudocode:

```text
1. Resolve commit hash to commit ID.
2. Resolve file stable ID to file row ID.
3. Find source symbols in that file at that commit.
4. Join refs whose from_symbol_id is one of those symbols.
5. Exclude refs whose target symbol, if known locally, is in the same file.
6. Expand matching locations_json.
```

SQL sketch:

```sql
WITH source_symbols AS (
  SELECT s.id, s.stable_id
  FROM commits c
  JOIN commit_symbols cs ON cs.commit_id = c.id
  JOIN symbols s ON s.id = cs.symbol_id
  JOIN files f ON f.id = s.file_id
  WHERE c.hash = ? AND f.stable_id = ?
)
SELECT ...
FROM commits c
JOIN commit_refs cr ON cr.commit_id = c.id
JOIN ref_versions rv ON rv.id = cr.ref_version_id
JOIN source_symbols source ON source.id = rv.from_symbol_id
JOIN files ref_file ON ref_file.id = rv.file_id
LEFT JOIN commit_symbols target_cs
  ON target_cs.commit_id = c.id
LEFT JOIN symbols target
  ON target.id = target_cs.symbol_id
 AND target.stable_id = rv.to_stable_id
LEFT JOIN files target_file
  ON target_file.id = target.file_id
JOIN json_each(rv.locations_json) j
WHERE c.hash = ?
  AND COALESCE(target_file.stable_id, '') != ?
ORDER BY rv.to_stable_id, rv.kind;
```

The exact SQL can be simplified during implementation. The key is the same: identify the small symbol/file set first, then expand locations.

## Instrumentation Design

### Goals

Instrumentation should answer these questions without needing a profiler every time:

- Which provider query started?
- Which SQL did it run?
- Which parameters were bound?
- How many rows came back?
- How many milliseconds did it take?
- Did it exceed a slow-query threshold?

### Debug flag

Use URL query parameters so production exports remain quiet:

```text
?debugSql
?debugSql=1
?debugSql=verbose
```

The simplest implementation is boolean:

```ts
function isSqlDebugEnabled(): boolean {
  return new URLSearchParams(window.location.search).has('debugSql');
}
```

### Query logging API sketch

```ts
interface SqlQueryDebugInfo {
  sql: string;
  params: SqlValue[];
  elapsedMs: number;
  rows: number;
}

function compactSql(sql: string): string {
  return sql.replace(/\s+/g, ' ').trim().slice(0, 500);
}

function logSqlStart(sql: string, params: SqlValue[]): void;
function logSqlDone(info: SqlQueryDebugInfo): void;
function logSqlError(info: SqlQueryDebugInfo & { error: unknown }): void;
```

### `queryAll` pseudocode

```ts
export function queryAll<T>(db, sql, params = []): T[] {
  const debug = isSqlDebugEnabled();
  const start = performance.now();
  if (debug) console.warn('[sql.js:start]', { sql: compactSql(sql), params });

  const stmt = db.prepare(sql);
  try {
    stmt.bind(params);
    const rows = [];
    while (stmt.step()) rows.push(stmt.getAsObject());
    const elapsedMs = performance.now() - start;
    if (debug || elapsedMs > 1000) {
      console.warn('[sql.js:done]', { elapsedMs, rows: rows.length, sql: compactSql(sql) });
    }
    return rows;
  } catch (error) {
    console.error('[sql.js:error]', { elapsedMs: performance.now() - start, sql: compactSql(sql), params, error });
    throw error;
  } finally {
    stmt.free();
  }
}
```

This does not make slow queries non-blocking, but it makes future performance bugs self-identifying.

## Implementation Plan

### Phase 1: Add SQL timing instrumentation

Files:

- `ui/src/api/sqljs/sqlRows.ts`

Steps:

1. Add `isSqlDebugEnabled()` and `compactSql()` helpers.
2. Wrap `queryAll` with start/done/error logs.
3. Log slow queries above a fixed threshold, for example 1000 ms, even without `?debugSql`.
4. Keep logs compact. Do not print entire multi-kilobyte SQL strings.
5. Verify with the large export URL:

```text
http://127.0.0.1:4183/?debugSql#/source/pkg/help/publish/sqlite_validator.go
```

Expected result before query rewrite: a slow `snapshot_refs` query is clearly visible.

### Phase 2: Rewrite source refs query

Files:

- `ui/src/api/sqlJsQueryProvider.ts`

Steps:

1. Replace `refRecordSelectSQL` use inside `getRefRecordsInFile` with a normalized SQL fragment.
2. Keep the returned aliases identical to `RefRecordSQL`.
3. Ensure `toRefRecord` does not change.
4. Rebuild and export.
5. Re-test source page.

Expected result: opening the source file should render source text and identifier links quickly. The debug log should show the source refs query returning 82 rows in milliseconds or low tens of milliseconds in sql.js, not tens of seconds.

### Phase 3: Rewrite snippet refs range query

Files:

- `ui/src/api/sqlJsQueryProvider.ts`

Steps:

1. Apply the same normalized base-table query to `getRefRecordsInFileRange`.
2. Add start/end offset predicates using aliases or repeated `json_extract` expressions.
3. Verify symbol pages and review snippets still link identifiers.

Expected result: symbol snippets remain correct and no longer pay the `snapshot_refs` view cost.

### Phase 4: Rewrite file xref panel queries

Files:

- `ui/src/api/sqlJsQueryProvider.ts`
- `ui/src/features/source/FileXrefPanel.tsx` if UI behavior changes are needed

Steps:

1. Rewrite `getRefRecordsToFileSymbols` against base tables.
2. Rewrite `getRefRecordsFromFileSymbols` against base tables.
3. Consider adding a `LIMIT` or lazy-load toggle if large files still produce too many refs for comfortable rendering.
4. Preserve `FileXrefResponse` shape.

Expected result: the lower `FileXrefPanel` no longer causes a second long stall after source refs are fixed.

### Phase 5: Tests

Recommended test layers:

1. Unit-level SQL string tests if existing test infrastructure can instantiate sql.js fixtures.
2. Provider-level tests with a small SQLite fixture that has:
   - two commits,
   - two files,
   - source and target symbols,
   - one intra-file ref,
   - one cross-file ref,
   - one external target ref.
3. Static smoke test against the full Glazed DB using a Node script or browser smoke script.

Provider-level assertions:

```text
getSourceRefs(fileA) returns refs whose locations are in fileA.
getSnippetRefs(symbolInFileA) returns only refs inside the symbol byte range.
getFileXref(fileA).usedBy excludes intra-file references.
getFileXref(fileA).uses groups by target symbol and kind.
```

### Phase 6: Validation against Glazed

Commands:

```bash
# native timing baseline
sqlite3 /tmp/glazed-full-export/db/codebase.db < ttmp/.../scripts/<new-analysis-script>.sql

# rebuild frontend/export after code changes
GOWORK=off make build
/tmp/codebase-browser-demo review export --db /tmp/glazed-full.db --out /tmp/glazed-full-export --repo-root /home/manuel/code/wesen/corporate-headquarters/glazed --strict-docs

# serve
cd /tmp/glazed-full-export
python3 -m http.server 4183
```

Browser URL:

```text
http://127.0.0.1:4183/?debugSql#/source/pkg/help/publish/sqlite_validator.go
```

Record:

- source load time,
- source refs query time,
- file xref query times,
- whether the browser remains responsive,
- any console errors.

## Diagrams

### Current slow path

```text
SourcePage
  │
  ├─ getSource(path) ─────────────── fast: file_contents by hash
  │
  ├─ getSourceRefs(path)
  │    └─ getRefRecordsInFile
  │         └─ SELECT FROM snapshot_refs
  │              └─ expands commit_refs × ref_versions × json_each
  │                   └─ filters one file too late
  │                        └─ main thread blocked
  │
  └─ FileXrefPanel(path)
       └─ getFileXref
            ├─ getRefRecordsToFileSymbols    via snapshot_refs
            └─ getRefRecordsFromFileSymbols  via snapshot_refs
```

### Proposed fast path

```text
SourcePage
  │
  ├─ getSource(path)
  │    └─ unchanged
  │
  ├─ getSourceRefs(path)
  │    └─ normalized query
  │         ├─ commits.hash -> commits.id
  │         ├─ files.stable_id -> files.id
  │         ├─ commit_refs constrained by commit_id
  │         ├─ ref_versions constrained by file_id
  │         └─ json_each only on matching ref_versions
  │
  └─ FileXrefPanel(path)
       └─ normalized xref queries
            ├─ source/target symbols constrained by file_id + commit_id
            └─ refs expanded only after symbol set is small
```

## Risk Analysis

### Risk: query result shape changes subtly

Mitigation: preserve `RefRecordSQL` aliases and keep `toRefRecord` unchanged. Add fixture tests for source refs, snippet refs, and file xrefs.

### Risk: file xrefs are still heavy for very large files

Mitigation: first fix the SQL scan. If rendering thousands of results remains slow, add pagination or a `Load xrefs` button as a separate UI task.

### Risk: normalized SQL is harder to read than `snapshot_refs`

Mitigation: name fragments clearly and add comments explaining why hot paths avoid `snapshot_refs`.

### Risk: sql.js still blocks during DB initialization or large history queries

Mitigation: this ticket adds measurement. Moving sql.js to a Web Worker remains a valid follow-up if query optimization is not enough.

## Alternatives Considered

### Alternative 1: Move all sql.js to a Web Worker first

This would keep the UI responsive but would not make the query efficient. The same 60-second native query shape would still waste CPU. Worker migration is valuable for resilience, but it should follow query-shape fixes.

### Alternative 2: Precompute a flattened `static_refs` table during export

This could make frontend queries very simple and fast. It would increase export size because references are already the largest part of the database. It may be useful later for very large repos, but the normalized base tables already contain the needed data.

### Alternative 3: Add indexes to `snapshot_refs`

SQLite views cannot be indexed directly. Indexes on base tables help only if the planner can push predicates through the view. The observed plan shows it does not do that effectively for this query.

### Alternative 4: Remove source ref linkification

This would make source pages fast but remove an important feature. It is acceptable as an emergency fallback or user setting, not as the primary fix.

## API References

### `SourceRefView`

File: `ui/src/api/sourceApi.ts`

```ts
export interface SourceRefView {
  toSymbolId: string;
  kind: string;
  offset: number;
  length: number;
}
```

Used by `SourceView` to linkify identifier tokens by byte offset.

### `FileXrefResponse`

File: `ui/src/api/sourceApi.ts`

```ts
export interface FileXrefResponse {
  path: string;
  usedBy: FileXrefRef[];
  uses: FileXrefUseTarget[];
}
```

Used by `FileXrefPanel` to display two lists: external users of symbols in this file, and external symbols used by this file.

### `RefRecord`

File: `ui/src/api/xrefApi.ts`

Conceptual shape:

```ts
interface RefRecord {
  fromSymbolId: string;
  toSymbolId: string;
  kind: string;
  fileId: string;
  range: {
    startLine: number;
    startCol: number;
    endLine: number;
    endCol: number;
    startOffset: number;
    endOffset: number;
  };
}
```

The provider should keep returning this canonical shape regardless of SQL implementation.

## File Reference Map

- `ui/src/api/sqljs/sqlRows.ts`
  - Shared sql.js query execution helper.
  - Add debug timing here.
- `ui/src/api/sqlJsQueryProvider.ts`
  - Main static DB query provider.
  - Rewrite hot ref helpers here.
- `ui/src/api/sourceApi.ts`
  - RTK Query source endpoints and public source/xref response types.
  - Should not require API shape changes for the first fix.
- `ui/src/features/source/SourcePage.tsx`
  - Source route component.
  - Starts source, source-ref, index, and file-xref data requests.
- `ui/src/features/source/FileXrefPanel.tsx`
  - Displays file-level xrefs.
  - May need lazy-load or result-limit UI later.
- `internal/history/schema.go`
  - Defines normalized tables and compatibility views.
  - Important for understanding why `snapshot_refs` is expensive.
- `ttmp/2026/05/02/GCB-017--performance-analysis-and-optimization-of-codebase-browser-review-indexing/scripts/40-glazed-corporate-full-index.sh`
  - Reproduction script for the full Glazed export.
- `ttmp/2026/05/02/GCB-017--performance-analysis-and-optimization-of-codebase-browser-review-indexing/scripts/41-glazed-corporate-full-analysis.sql`
  - Analysis query for full Glazed DB size and redundancy.

## Implementation Checklist

- [ ] Add `?debugSql` query timing to `queryAll`.
- [ ] Add slow-query warning threshold.
- [ ] Rewrite `getRefRecordsInFile` to base tables.
- [ ] Rewrite `getRefRecordsInFileRange` to base tables.
- [ ] Rewrite file used-by query to base tables.
- [ ] Rewrite file uses query to base tables.
- [ ] Add fixture/provider tests.
- [ ] Rebuild static assets.
- [ ] Re-export full Glazed site.
- [ ] Validate source page and record timings.
- [ ] Decide whether Web Worker migration is still necessary.

## Open Questions

1. Should xrefs be loaded automatically on source page open, or behind an explicit `Load xrefs` button for very large exports?
2. Should the static export include optional precomputed xref summary tables for very large repositories?
3. Should `queryAll` support cancellation or yielding? sql.js statement stepping is synchronous, so true cancellation likely requires a Web Worker.
4. Should query timings be visible in a small in-app debug panel rather than only in the console?

## References

- `ui/src/api/sqljs/sqlRows.ts:8-24` — central `queryAll` loop over `stmt.step()`.
- `ui/src/api/sqljs/sqlRows.ts:26-32` — `queryOne` delegates to `queryAll`.
- `ui/src/features/source/SourcePage.tsx:15-18` — source page loads source and source refs.
- `ui/src/features/source/SourcePage.tsx:32-43` — source refs are passed to `SourceView`; file xrefs are rendered below.
- `ui/src/features/source/FileXrefPanel.tsx:14-18` — file xref query starts as soon as the panel mounts.
- `ui/src/api/sourceApi.ts:55-69` — RTK Query endpoints call provider methods.
- `ui/src/api/sqlJsQueryProvider.ts:263-272` — source refs call `getRefRecordsInFile`.
- `ui/src/api/sqlJsQueryProvider.ts:275-285` — file xrefs call two ref helper queries.
- `ui/src/api/sqlJsQueryProvider.ts:694-758` — current ref helpers query `snapshot_refs`.
- `ui/src/api/sqlJsQueryProvider.ts:784-800` — shared `snapshot_refs` select fragments.
- `internal/history/schema.go:101-115` — `ref_versions` table and indexes.
- `internal/history/schema.go:214-235` — `snapshot_refs` view expands `locations_json` with `json_each`.
