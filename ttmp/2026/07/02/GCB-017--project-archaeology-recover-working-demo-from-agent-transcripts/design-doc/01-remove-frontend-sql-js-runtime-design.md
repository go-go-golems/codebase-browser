---
Title: Remove Frontend sql.js Runtime Design
Ticket: GCB-017
Status: active
Topics:
    - codebase-browser
    - demo-recovery
    - frontend-runtime
    - live-go-api
    - sqlite
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: internal/server/api.go
      Note: Existing live index/source/snippet endpoints
    - Path: internal/server/api_history.go
      Note: Existing live history/diff/impact endpoints
    - Path: internal/server/server.go
      Note: Route registration for live backend APIs
    - Path: ui/src/api/liveApiProvider.ts
      Note: Target frontend HTTP provider
    - Path: ui/src/api/sourceApi.ts
      Note: Source/snippet refs API slice with remaining sql.js paths
    - Path: ui/src/api/sqlJsQueryProvider.ts
      Note: Frontend SQLite provider to remove
    - Path: ui/src/api/xrefApi.ts
      Note: Xref API slice currently using sql.js directly
ExternalSources: []
Summary: ""
LastUpdated: 2026-07-05T00:00:00Z
WhatFor: ""
WhenToUse: ""
---


# Remove Frontend sql.js Runtime Design

## Executive summary

The current `codebase-browser` demo has two query runtimes: a live Go HTTP API and a frontend sql.js runtime that downloads `db/codebase.db` and evaluates SQLite queries in the browser. That dual runtime was useful while the project supported fully static exports, but it is now actively harmful for the yolo live demo: users can download a very large SQLite database, frontend behavior differs between live and static mode, and query ownership is split across TypeScript and Go.

This design removes all frontend sql.js migration paths from the live application code. After the implementation, React data APIs should call backend HTTP endpoints only. The SQLite database remains an implementation detail of the Go server. The browser may render static assets, call JSON/text endpoints, and navigate routes, but it must not import `sql.js`, request `sql-wasm.wasm`, request `db/codebase.db`, or instantiate `SqlJsQueryProvider`.

The removal is not just a delete operation. Several features currently have backend endpoints (`index`, `symbol`, `search`, `source`, `snippet`, review docs, history, diff, body diff, impact), while other features still depend on sql.js-only paths (`xref`, snippet refs, source refs, file xrefs, package lites depending on implementation details). The implementation must first close those API gaps, then simplify the frontend provider layer, then remove sql.js artifacts from build/runtime packaging.

## Problem statement and scope

### Problem

The yolo live demo is served by a Go process with a SQLite database mounted inside the container. The browser should not need to download the same database. However, the frontend still has code paths that can initialize sql.js and fetch the static database.

Observed evidence:

- `ui/src/api/sqljs/sqlJsDb.ts` imports `sql.js`, locates `sql-wasm.wasm`, fetches `manifest.json`, fetches `db/codebase.db`, and creates an in-browser SQLite database.
- `ui/src/api/sqlJsQueryProvider.ts` contains a full TypeScript implementation of most query operations against that database.
- `ui/src/api/codebaseProvider.ts` exposes `liveOrSql(...)`, `liveWithSqlFallback(...)`, and `sqlProvider()`, which keeps sql.js as an explicit runtime fallback.
- `ui/src/api/xrefApi.ts` still calls `getSqlJsProvider().getXref(...)` directly.
- `ui/src/api/sourceApi.ts` still uses sql.js-only providers for snippet refs, source refs, and file xrefs.
- `ui/src/api/historyApi.ts` still carries sql.js fallbacks for all history endpoints.

### Scope

In scope:

1. Add backend endpoints for every frontend query that is still sql.js-only.
2. Update frontend API slices to use live backend endpoints only.
3. Remove the `liveOrSql` and `sqlProvider` abstraction from production frontend code.
4. Remove `sql.js` imports and package dependency.
5. Stop packaging or advertising frontend SQLite runtime assets for the live demo.
6. Update tests, smoke checks, and docs so regressions are caught.
7. Keep a detailed diary and commit in reviewable increments.

Out of scope for this ticket unless explicitly requested:

- Designing a new static/offline export mode with a non-sql.js data format.
- Replacing the server-side SQLite database.
- Changing the underlying index schema.

## Current-state architecture

### Runtime overview

```text
Current live deployment

Browser React app
  ├─ LiveApiProvider  ───────────────► Go server /api/* ─────► SQLite DB in container
  └─ SqlJsQueryProvider fallback ────► fetch db/codebase.db ─► SQLite DB in browser
```

The live backend already owns the authoritative database. The frontend fallback duplicates query logic and can load the full DB into the browser.

### Server API surface already present

`internal/server/server.go` registers the current live routes:

- `GET /api/health`
- `GET /api/index`
- `GET /api/review-docs`
- `GET /api/review-docs/{slug}`
- `GET /api/symbol`
- `GET /api/search`
- `GET /api/source`
- `GET /api/snippet`
- `GET /api/history/commits`
- `GET /api/history/symbol`
- `GET /api/history/impact`
- `GET /api/history/diff`
- `GET /api/history/symbol-body-diff`

The generic code/source endpoints live in `internal/server/api.go`. History and impact endpoints live in `internal/server/api_history.go`. Review-doc endpoints live in `internal/server/api_review.go`.

### Frontend provider split

`ui/src/api/liveApiProvider.ts` is the desired direction: methods call HTTP endpoints with `fetchJSON` and `fetchText`.

`ui/src/api/sqlJsQueryProvider.ts` is the runtime to remove. It duplicates these classes of query logic:

- current index queries (`getIndex`, `getSymbol`, `searchSymbols`),
- source/snippet queries (`getSource`, `getSnippet`, `getSnippetRefs`, `getSourceRefs`, `getFileXref`),
- xref queries (`getXref`),
- history queries (`listCommits`, `getSymbolHistory`, `getSymbolBodyDiff`, `getCommitDiff`, `getImpact`),
- review doc queries (`listReviewDocs`, `getReviewDoc`).

`ui/src/api/sqljs/sqlJsDb.ts` is the point where the browser downloads the DB:

```text
getStaticDb()
  ├─ getSqlJs() -> import sql.js runtime and locate sql-wasm.wasm
  ├─ getStaticManifest() -> fetch manifest.json
  ├─ fetch manifest.db.path or db/codebase.db
  └─ new SQL.Database(bytes)
```

### Known remaining sql.js-dependent feature gaps

The following frontend API endpoints still need backend implementations before deleting sql.js:

| Frontend feature | Current frontend API | Current backend equivalent | Gap |
|---|---|---|---|
| Symbol xrefs | `xrefApi.getXref` | none | Add `/api/xref` |
| Snippet refs | `sourceApi.getSnippetRefs` | none | Add `/api/snippet-refs` or `/api/source/snippet-refs` |
| Source refs | `sourceApi.getSourceRefs` | none | Add `/api/source-refs` |
| File xref summary | `sourceApi.getFileXref` | none | Add `/api/file-xref` |
| Package lites | live provider derives from `/api/index` | works but heavy | Optional: add `/api/packages` |
| Commit refs in source/snippet | live provider supports `commit=` | works | Keep and test |

## Target architecture

```text
Target live deployment

Browser React app
  ├─ RTK Query API slices
  └─ LiveApiProvider only
       │
       ▼
Go server /api/*
  ├─ api.go           index, symbol, search, source, snippet
  ├─ api_review.go    review docs
  ├─ api_history.go   commits, symbol history, impact, diffs
  └─ api_xref.go      xrefs, snippet refs, source refs, file xrefs
       │
       ▼
SQLite DB in server container
```

In the target runtime:

- The frontend never imports `sql.js`.
- The frontend never fetches `db/codebase.db`.
- The frontend never fetches `sql-wasm.wasm`.
- Query logic lives in Go and can be tested without a browser.
- Static export can still serve the SPA, but interactive data requires a live backend unless a new explicit static-data runtime is designed later.

## Proposed backend API additions

### `GET /api/xref?id=<symbol>&commit=<ref>`

Returns the existing frontend `XrefResponse` shape.

```ts
interface RefRecord {
  fromSymbolId: string;
  toSymbolId: string;
  kind: string;
  fileId: string;
  range: Range;
}

interface XrefUseTarget {
  toSymbolId: string;
  kind: string;
  count: number;
  occurrences: RefRecord[];
}

interface XrefResponse {
  id: string;
  usedBy: RefRecord[];
  uses: XrefUseTarget[];
}
```

Server query sketch:

```sql
-- usedBy
SELECT from_symbol_id, to_symbol_id, kind, file_id,
       start_line, start_col, end_line, end_col, start_offset, end_offset
FROM snapshot_refs
WHERE commit_hash = ? AND to_symbol_id = ?
ORDER BY from_symbol_id, kind;

-- uses
SELECT ...
FROM snapshot_refs
WHERE commit_hash = ? AND from_symbol_id = ?
ORDER BY to_symbol_id, kind;
```

Grouping pseudocode:

```go
func handleXref(w, r):
    commit = resolveCommit(r.query["commit"] or "HEAD")
    id = required query id/symbol
    usedBy = queryRefRecordsTo(commit, id)
    usesFlat = queryRefRecordsFrom(commit, id)
    uses = group by (toSymbolId, kind)
    writeJSON(XrefResponse{id, usedBy, uses})
```

### `GET /api/snippet-refs?symbol=<symbol>&commit=<ref>`

Returns `SnippetRefView[]` for references inside a symbol declaration/body snippet.

```ts
interface SnippetRefView {
  toSymbolId: string;
  kind: string;
  offsetInSnippet: number;
  length: number;
}
```

Server query sketch:

```text
1. Resolve symbol body metadata at commit:
   - file_id
   - start_offset
   - end_offset
2. Query refs in same file whose byte range is inside that symbol range.
3. Convert file offsets into snippet-relative offsets.
```

Pseudocode:

```go
func handleSnippetRefs(w, r):
    meta = queryBodyMeta(commit, symbolID)
    refs = queryRefRecordsInFileRange(commit, meta.fileID, meta.startOffset, meta.endOffset)
    views = []SnippetRefView{}
    for ref in refs:
        views.append({
          toSymbolId: ref.ToSymbolID,
          kind: ref.Kind,
          offsetInSnippet: max(0, ref.StartOffset - meta.StartOffset),
          length: max(0, ref.EndOffset - ref.StartOffset),
        })
    writeJSON(views)
```

### `GET /api/source-refs?path=<path>&commit=<ref>`

Returns `SourceRefView[]` for all references in a file.

```ts
interface SourceRefView {
  toSymbolId: string;
  kind: string;
  offset: number;
  length: number;
}
```

Server query sketch:

```text
1. Resolve file metadata by path at commit.
2. Query refs where commit_hash=? and file_id=? ordered by byte offset.
3. Map each ref to target symbol, kind, offset, and length.
```

### `GET /api/file-xref?path=<path>&commit=<ref>`

Returns file-level inbound and outbound references, matching `FileXrefResponse`.

```ts
interface FileXrefResponse {
  path: string;
  usedBy: FileXrefRef[];
  uses: FileXrefUseTarget[];
}
```

Server query sketch:

```sql
-- inbound: refs from symbols outside this file to symbols in this file
SELECT r.*
FROM snapshot_refs r
JOIN snapshot_symbols target
  ON target.commit_hash = r.commit_hash AND target.id = r.to_symbol_id
LEFT JOIN snapshot_symbols source
  ON source.commit_hash = r.commit_hash AND source.id = r.from_symbol_id
WHERE r.commit_hash = ?
  AND target.file_id = ?
  AND COALESCE(source.file_id, '') != ?;

-- outbound: refs from symbols in this file to symbols outside this file
SELECT r.*
FROM snapshot_refs r
JOIN snapshot_symbols source
  ON source.commit_hash = r.commit_hash AND source.id = r.from_symbol_id
LEFT JOIN snapshot_symbols target
  ON target.commit_hash = r.commit_hash AND target.id = r.to_symbol_id
WHERE r.commit_hash = ?
  AND source.file_id = ?
  AND COALESCE(target.file_id, '') != ?;
```

### Optional `GET /api/packages`

The live provider currently implements package lites by fetching the full `/api/index` and mapping packages. That avoids sql.js, but it is inefficient. If time permits, add:

```ts
interface PackageLite {
  id: string;
  importPath: string;
  name: string;
  files: number;
  symbols: number;
}
```

This is optional because it is not a sql.js removal blocker.

## Proposed frontend changes

### Replace dual provider helpers

Delete the current abstraction:

```ts
liveOrSql(liveFn, sqlFn)
liveWithSqlFallback(liveFn, sqlFn)
sqlProvider()
```

Replace with a smaller backend-only helper:

```ts
export function apiProvider() {
  return getLiveApiProvider();
}
```

If `isLiveApiAvailable()` is still useful, keep it only for UI badges or startup diagnostics. It must not select a sql.js fallback.

### Update API slices

Change each RTK slice to call live provider only:

```ts
// before
queryFn: () => providerResult(() => liveOrSql(
  () => liveProvider().getCommitDiff(from, to),
  () => sqlProvider().getCommitDiff(from, to),
))

// after
queryFn: () => providerResult(() => apiProvider().getCommitDiff(from, to))
```

Affected files:

- `ui/src/api/indexApi.ts`
- `ui/src/api/docApi.ts`
- `ui/src/api/sourceApi.ts`
- `ui/src/api/historyApi.ts`
- `ui/src/api/xrefApi.ts`

### Remove sql.js implementation files

After all imports are gone, delete:

- `ui/src/api/sqlJsQueryProvider.ts`
- `ui/src/api/sqljs/sqlJsDb.ts`
- `ui/src/api/sqljs/sqlRows.ts` if only sql.js used it

Then remove dependency entries from `ui/package.json`:

- `sql.js`
- `@types/sql.js`

Run package manager cleanup so `pnpm-lock.yaml` no longer includes sql.js.

### Prevent accidental DB fetches

Add a browser smoke script or Playwright test that visits live pages and fails if any request URL contains:

- `/db/codebase.db`
- `sql-wasm.wasm`
- `sql.js`

Pseudocode:

```ts
const forbidden = [/\/db\/codebase\.db/, /sql-wasm\.wasm/, /sql\.js/];
page.on('request', req => {
  if (forbidden.some(re => re.test(req.url()))) {
    throw new Error(`forbidden frontend DB request: ${req.url()}`);
  }
});

for (const route of reviewRoutes) {
  await page.goto(`${base}/#/${route}`);
  await expect(page.getByText(/Failed|Unknown|not found/)).not.toBeVisible();
}
```

## Decision records

### Decision: Live server owns all query execution

- **Context:** The live yolo deployment already runs a Go server with the SQLite DB in the container. Keeping a browser SQLite runtime duplicates query code and forces large downloads.
- **Options considered:** Keep sql.js fallback; keep sql.js only for static export; remove sql.js completely from frontend code.
- **Decision:** Remove sql.js completely from frontend production code for this ticket.
- **Rationale:** The user explicitly requested no frontend SQLite access. Backend APIs are easier to secure, test, profile, and deploy.
- **Consequences:** Static exports become non-interactive unless served with a backend or redesigned with a smaller static JSON data layer.
- **Status:** accepted

### Decision: Preserve frontend response shapes while moving implementations to Go

- **Context:** Widgets already consume TypeScript interfaces such as `XrefResponse`, `SnippetRefView`, and `FileXrefResponse`.
- **Options considered:** Redesign all frontend models; preserve current response shapes and only change transport; expose raw SQL-ish rows.
- **Decision:** Preserve current response shapes.
- **Rationale:** This keeps the UI migration small and makes it easy to compare Go output to the old sql.js behavior during tests.
- **Consequences:** Some Go response structs will intentionally use camelCase/legacy names to match frontend contracts.
- **Status:** accepted

### Decision: Add focused endpoints instead of expanding `/api/index`

- **Context:** `/api/index` can return many packages/files/symbols. xrefs and refs are local queries and should not require the whole index.
- **Options considered:** Fetch `/api/index` and compute in frontend; add focused endpoints; add a GraphQL-like query endpoint.
- **Decision:** Add focused REST endpoints.
- **Rationale:** Focused endpoints minimize payload size and keep query logic near SQLite.
- **Consequences:** More handlers exist, but each is simple and testable.
- **Status:** accepted

## Implementation plan

### Phase 0: Baseline and guardrails

1. Add tasks to `ttmp/.../tasks.md` or via `docmgr task add` for each phase.
2. Capture a baseline network trace on yolo and local live server.
3. Add a temporary grep check:

```bash
rg -n "sqlJs|sqljs|sql\.js|getSqlJsProvider|db/codebase|sql-wasm|liveOrSql|sqlProvider" ui/src ui/package.json
```

Expected before implementation: several matches.
Expected after implementation: no production matches, except possibly historical docs/diary.

### Phase 1: Backend xref endpoints

Files:

- `internal/server/server.go`
- new `internal/server/api_xref.go`
- `internal/server/server_test.go` or new focused tests

Tasks:

1. Add route registrations:
   - `/api/xref`
   - `/api/snippet-refs`
   - `/api/source-refs`
   - `/api/file-xref`
2. Implement shared `refRecord` response struct.
3. Implement helper queries:
   - `queryRefRecordsFrom`
   - `queryRefRecordsTo`
   - `queryRefRecordsInFile`
   - `queryRefRecordsInFileRange`
   - `queryRefRecordsToFileSymbols`
   - `queryRefRecordsFromFileSymbols`
4. Add unit tests using the server test DB fixtures.

Validation:

```bash
go test ./internal/server -count=1
curl -fsS 'http://127.0.0.1:3003/api/xref?id=...'
```

### Phase 2: Live provider methods

File:

- `ui/src/api/liveApiProvider.ts`

Add methods:

```ts
getXref(id: string): Promise<XrefResponse>
getSnippetRefs(symbolId: string, commit?: string): Promise<SnippetRefView[]>
getSourceRefs(path: string, commit?: string): Promise<SourceRefView[]>
getFileXref(path: string, commit?: string): Promise<FileXrefResponse>
```

Validation:

```bash
pnpm -C ui run typecheck
```

### Phase 3: Remove sql.js from API slices

Files:

- `ui/src/api/codebaseProvider.ts`
- `ui/src/api/indexApi.ts`
- `ui/src/api/docApi.ts`
- `ui/src/api/sourceApi.ts`
- `ui/src/api/historyApi.ts`
- `ui/src/api/xrefApi.ts`

Tasks:

1. Replace live/sql fallback calls with live provider calls.
2. Remove `sqlProvider` and `liveOrSql` helpers.
3. Ensure `xrefApi` no longer imports `getSqlJsProvider`.
4. Ensure `sourceApi` no longer calls `sqlProvider()`.
5. Ensure `historyApi` no longer calls `sqlProvider()`.

Validation:

```bash
rg -n "sqlProvider|liveOrSql|getSqlJsProvider|SqlJsQueryProvider" ui/src
pnpm -C ui run typecheck
```

Expected: no production references.

### Phase 4: Delete sql.js runtime and dependency

Files:

- delete `ui/src/api/sqlJsQueryProvider.ts`
- delete `ui/src/api/sqljs/sqlJsDb.ts`
- delete `ui/src/api/sqljs/sqlRows.ts` if unused
- edit `ui/package.json`
- update `pnpm-lock.yaml`

Validation:

```bash
rg -n "sql\.js|@types/sql\.js|sql-wasm|db/codebase|SqlJs" ui package.json pnpm-lock.yaml
pnpm -C ui install --lockfile-only
pnpm -C ui run typecheck
```

### Phase 5: Demo rebuild and no-DB browser smoke

Tasks:

1. Run focused Go and TS validation:

```bash
go test ./internal/server ./internal/docs ./internal/staticapp -count=1
pnpm -C ui run typecheck
```

2. Rebuild demo:

```bash
make demo-solid
make demo-serve
make demo-smoke
```

3. Browser network validation:

- Visit `/review/01-pr-review-static-export`.
- Visit `/review/02-symbol-history-and-impact`.
- Visit `/review/03-commit-walk-walkthrough` and click all steps.
- Visit `/review/04-file-and-annotation-examples`.
- Fail if network requests include `db/codebase.db`, `sql-wasm`, or sql.js chunks.

### Phase 6: Deploy yolo

Tasks:

1. Build and push a new image tag.
2. Update `/home/manuel/code/wesen/2026-03-27--hetzner-k3s/gitops/kustomize/codebase-browser/deployment.yaml`.
3. Commit and push GitOps.
4. Wait for ArgoCD healthy.
5. Run public browser network smoke.

## Testing strategy

### Unit tests

- Backend handler tests for new xref endpoints.
- Query helper tests for correct grouping of `uses` and `usedBy`.
- Tests for commit ref resolution on xref endpoints.

### Type checks

```bash
pnpm -C ui run typecheck
```

### Static grep tests

```bash
rg -n "sql\.js|@types/sql\.js|getSqlJsProvider|SqlJsQueryProvider|liveOrSql|sqlProvider|sql-wasm|db/codebase" ui/src ui/package.json pnpm-lock.yaml
```

Expected: no production references after deletion.

### Runtime smoke tests

- `make demo-smoke` for backend/API/artifact health.
- Browser network smoke to prove no frontend DB/wasm request.
- Public yolo curl checks:

```bash
curl -fsS https://codebase-browser.yolo.scapegoat.dev/api/health
curl -fsS https://codebase-browser.yolo.scapegoat.dev/api/history/commits
curl -fsS 'https://codebase-browser.yolo.scapegoat.dev/api/xref?id=<symbol>'
```

## Risks and mitigations

### Risk: Static export loses interactive mode

Removing sql.js means a bare static export cannot answer dynamic queries by itself. This is acceptable for this ticket because the explicit requirement is no frontend SQLite access. If static interactive exports are needed later, design a smaller static JSON API or pre-render more pages.

### Risk: Go endpoints do not exactly match old sql.js output

Mitigate by preserving response shapes and testing representative widgets before deleting sql.js.

### Risk: Hidden sql.js imports remain through package lock or lazy chunks

Mitigate with grep checks, package dependency removal, and browser network checks.

### Risk: yolo deployment image remains large

The current demo image embeds the static export and a large SQLite DB. This remains true after sql.js removal. It can be improved later by downloading the DB artifact at startup or using a PVC.

## References

Key implementation files:

- `internal/server/server.go` — route registration and static handler.
- `internal/server/api.go` — health, index, symbol, search, source, snippet endpoints.
- `internal/server/api_history.go` — history, diff, body diff, and impact endpoints.
- `internal/server/api_review.go` — review document endpoints.
- `ui/src/api/liveApiProvider.ts` — desired frontend transport layer.
- `ui/src/api/codebaseProvider.ts` — current dual-runtime selector to remove.
- `ui/src/api/sqlJsQueryProvider.ts` — current frontend SQLite query implementation to delete.
- `ui/src/api/sqljs/sqlJsDb.ts` — current DB/wasm loader to delete.
- `ui/src/api/sourceApi.ts` — snippet/source/xref-related API slice.
- `ui/src/api/xrefApi.ts` — direct sql.js xref user to migrate.
- `ui/src/api/historyApi.ts` — history API slice with fallback paths to remove.
- `ui/src/features/doc/widgets/ImpactInlineWidget.tsx` — review-page impact widget consumer.
- `ui/src/features/symbol/XrefPanel.tsx` — xref consumer.
- `ui/src/features/source/SourcePage.tsx` — source refs/file xref likely consumers.
- `Dockerfile` and `.dockerignore` — yolo image packaging.
- `/home/manuel/code/wesen/2026-03-27--hetzner-k3s/gitops/kustomize/codebase-browser/deployment.yaml` — yolo deployment manifest.
