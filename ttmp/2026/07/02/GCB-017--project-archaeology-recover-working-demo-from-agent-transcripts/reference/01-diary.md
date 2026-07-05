---
Title: Diary
Ticket: GCB-017
Status: active
Topics:
    - codebase-browser
    - go-minitrace
    - project-archeology
    - demo-recovery
    - transcript-analysis
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/codebase-browser/cmds/serve/run.go
      Note: Step 4 restored serve command
    - Path: examples/02-symbol-history-and-impact.md
      Note: Step 6 stable diff refs
    - Path: internal/docs/renderer.go
      Note: Step 6 short-ref resolver fix
    - Path: internal/server/api.go
      Note: Step 4 restored Go-side SQLite APIs
    - Path: internal/server/api_history.go
      Note: Step 6 live diff API
    - Path: internal/server/api_xref.go
      Note: Live backend endpoints replacing sql.js-only xref/ref queries
    - Path: internal/server/server.go
      Note: Registered xref/ref routes
    - Path: internal/server/server_test.go
      Note: |-
        Step 4 validation coverage
        Fixture coverage for xref/ref endpoints
    - Path: ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/analysis/01-transcript-archaeology-findings.md
      Note: Primary findings synthesized from the transcript archaeology
    - Path: ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/scripts/05-demo-recovery-files.sql
      Note: DuckDB schema mismatch and fix recorded in diary
    - Path: ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/scripts/archaeology/extract.js
      Note: JS command API issues and fixes recorded in diary
    - Path: ui/package.json
      Note: Removed sql.js runtime dependencies
    - Path: ui/src/api/codebaseProvider.ts
      Note: Backend-only provider helper after removing sql.js fallback selection
    - Path: ui/src/api/liveApiProvider.ts
      Note: Live HTTP provider now covers xref/ref endpoints
    - Path: ui/src/api/xrefApi.ts
      Note: Xref slice migrated from sql.js provider to live API
ExternalSources: []
Summary: Chronological diary for the GCB-017 go-minitrace transcript archaeology work.
LastUpdated: 2026-07-02T23:35:00Z
WhatFor: Use this to resume or review the transcript archaeology workflow.
WhenToUse: When continuing the demo recovery investigation or rerunning the minitrace scripts.
---






# Diary

## Goal

Capture the transcript archaeology work for `codebase-browser`: creating a docmgr ticket, converting the named Pi transcripts with `go-minitrace`, writing reusable SQL/JS query scripts, and preserving evidence that helps recover a working demo.

## Step 1: Create the archaeology ticket and read minitrace guidance

I created a dedicated ticket workspace for the project archaeology instead of mixing the investigation into the existing implementation tickets. The goal was to make the transcript conversion, query scripts, generated artifacts, and recovery notes reproducible from one place.

I also read `go-minitrace help --all` and the focused help pages for JS command handlers and DuckDB queries, then saved those help outputs under the ticket `sources/` directory so later runs can see which API shape the scripts were written against.

### Prompt Context

**User prompt (verbatim):** "Go through the transcripts in 2026-04-20T16-08-01-580Z_8664fb89-aa66-4563-826b-0dbe8c78019e.jsonl  2026-04-30T15-11-58-306Z_019ddef2-5b5d-72fb-b679-1d10972a0515.jsonl
2026-04-23T23-42-57-477Z_019dbcb9-a9c4-7449-9d4f-dc260c37eaba.jsonl  2026-05-02T14-40-10-900Z_019de921-f893-73dc-8bfa-9087aa65cbfe.jsonl
2026-04-25T11-48-02-978Z_019dc477-dd22-76d9-8e6a-c8ab7bef4b3b.jsonl  2026-07-02T23-18-46-087Z_019f2520-ac47-7a6e-8372-23242db70cad.jsonl 

with go-minitrace.

First, create a new ticket about this archeology of the project to get back a working demo.

Read `go-minitrace help --all` and read up on writing js scripts and sql queries and write those and store them in the ticket scripts/ folder. 

Keep a diary as you work."

**Assistant interpretation:** Create a new docmgr ticket, use `go-minitrace` to analyze the specified Pi transcript JSONL files, write reusable SQL and JS query scripts in the ticket, and keep a running diary.

**Inferred user intent:** Recover project history and demo-recovery evidence from prior agent sessions without relying on raw transcript spelunking.

### What I did
- Loaded the `go-minitrace-transcript-analysis` skill and its query reference.
- Ran `go-minitrace help --all` and saved it to `sources/go-minitrace-help/help-all.txt`.
- Saved focused help pages:
  - `sources/go-minitrace-help/js-api-reference.txt`
  - `sources/go-minitrace-help/structured-query-commands.txt`
  - `sources/go-minitrace-help/writing-duckdb-queries.txt`
  - `sources/go-minitrace-help/query-duckdb.txt`
  - `sources/go-minitrace-help/duckdb-query-recipes.txt`
- Created `GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts`.

### Why
- The requested work is research-heavy and benefits from docmgr bookkeeping, reusable scripts, and durable generated artifacts.
- Saving the help output makes the query-script API assumptions auditable.

### What worked
- All six requested transcript filenames were found under the Pi session directory for this repository.
- `go-minitrace help --all` was available from `/home/manuel/go/bin/go-minitrace`.

### What didn't work
- I first tried ticket ids `GCB-015` and `GCB-016`, but both already existed historically. `docmgr` reported ambiguous ticket indexes:
  - `Error: failed to find ticket directory: ambiguous ticket index doc for GCB-015 (got 2)`
  - `Error: failed to find ticket directory: ambiguous ticket index doc for GCB-016 (got 2)`
- I removed the duplicate ticket directories I had just created and used `GCB-017`, which was available.

### What I learned
- This repository already has completed tickets through `GCB-016`; new archaeology work should start at `GCB-017`.
- The current `go-minitrace` JS command API is builder-composed and prefers `mt.db().RuntimeArchives().QueryCommandDefaults().Build()` over older examples that call `mt.query()` directly.

### What was tricky to build
- Ticket ids were not monotonically obvious from the visible tail of `docmgr ticket list`; the old `GCB-015` and `GCB-016` tickets were present earlier in the list. The symptom was ambiguous doc lookup after ticket creation. I resolved this by checking existing ticket ids, deleting only the duplicate directories created during this step, and recreating the work as `GCB-017`.

### What warrants a second pair of eyes
- Confirm that removing the mistakenly created duplicate `GCB-015`/`GCB-016` July ticket directories is acceptable. They contained only fresh scaffold files from this session, not historical ticket data.

### What should be done in the future
- Use `docmgr ticket list | rg '^### GCB-'` before allocating a new GCB ticket id.

### Code review instructions
- Start with `ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/index.md` and `tasks.md`.
- Validate help snapshots by checking `sources/go-minitrace-help/*.txt`.

### Technical details
- Ticket base: `ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts`.
- Main analysis doc: `analysis/01-transcript-archaeology-findings.md`.
- Diary: `reference/01-diary.md`.

## Step 2: Convert the requested transcripts and write reusable queries

I staged only the six requested Pi JSONL transcripts into the ticket archive and converted that subset with `go-minitrace convert pi`. This avoids converting unrelated sessions while preserving a rerunnable source tree and minitrace archive inside the ticket.

I then wrote both raw DuckDB SQL and structured JS query commands. The SQL scripts cover overview and frequency analysis; the JS commands use the normalized query-command database to inspect prompts, shell commands, and demo-recovery signals.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Convert the specified transcripts, then write and run reusable SQL/JS minitrace queries from the ticket scripts folder.

**Inferred user intent:** Build a repeatable archaeology toolkit that can be rerun and extended while recovering evidence for a working demo.

### What I did
- Wrote `scripts/01-stage-and-convert.sh`.
- Wrote SQL scripts:
  - `scripts/02-session-overview.sql`
  - `scripts/03-tool-frequency.sql`
  - `scripts/05-demo-recovery-files.sql`
- Wrote JS structured query commands:
  - `scripts/archaeology/introspect.js`
  - `scripts/archaeology/extract.js`
- Ran conversion and generated:
  - `archive/minitrace/active/2026-04/*.minitrace.json`
  - `archive/minitrace/active/2026-05/*.minitrace.json`
  - `archive/minitrace/active/2026-07/*.minitrace.json`
- Ran SQL and JS queries, producing JSON outputs in `archive/`.

### Why
- The staged conversion makes the transcript subset explicit and prevents manifest drift from repeated `--source-session` conversions.
- SQL is good for compact aggregate analysis; JS commands are better for normalized-table queries and reusable project-specific extraction.

### What worked
- `go-minitrace convert pi --source-dir <staged> --output-dir <archive/minitrace>` converted all six sessions with quality `A`.
- `scripts/02-session-overview.sql` and `scripts/03-tool-frequency.sql` ran successfully via `go-minitrace query duckdb`.
- The JS command repository was discovered after moving the JS file under `scripts/archaeology/extract.js`; commands became available as `archaeology extract prompts`, `archaeology extract shell-commands`, and `archaeology extract demo-signals`.

### What didn't work
- My first JS script used stale API examples (`mt.query()` against `sessions_base`) and failed with:
  - `Error: json: unsupported type: func() (*goja.Object, error)`
- A first introspection attempt queried `sqlite_master` directly and failed because the JS query DB enforces allowed tables/views:
  - `Error: GoError: query references disallowed table/view "sqlite_master" at github.com/go-go-golems/go-minitrace/pkg/minitracejs.handleObject.func1 (native)`
- A first version of `05-demo-recovery-files.sql` assumed normalized `files`/`sessions` tables existed in raw DuckDB mode and failed with:
  - `Error: executing query: Catalog Error: Table with name files does not exist!`

### What I learned
- `go-minitrace query duckdb` exposes the raw archive as `sessions_base`.
- JS query commands expose a normalized SQLite-style schema with tables such as `sessions`, `turns`, `tool_calls`, `files`, `events`, and `metrics`.
- The JS command DB provides safe introspection via `db.tables()` and `db.schema()`, not arbitrary `sqlite_master` queries.

### What was tricky to build
- The main sharp edge was that `go-minitrace` has two query surfaces with different schemas. Raw DuckDB uses `sessions_base` and JSON extraction; JS commands use normalized tables and a builder-composed DB handle. I identified this by saving and reading the help pages, then adding `scripts/archaeology/introspect.js` to inspect the normalized schema safely.

### What warrants a second pair of eyes
- Review `scripts/archaeology/extract.js` for SQL injection safety. It uses `mt.sql.like()` for string filters and numeric coercion for limits, but the generated SQL is still string-composed.
- Review `scripts/05-demo-recovery-files.sql` because it uses broad text matching and intentionally returns noisy evidence.

### What should be done in the future
- Add narrower JS commands for exact evidence classes: old live-server evidence, static export evidence, deployment evidence, and failing-demo evidence.

### Code review instructions
- Start with `scripts/01-stage-and-convert.sh` to verify the transcript subset.
- Run:
  - `scripts/01-stage-and-convert.sh`
  - `go-minitrace query duckdb --archive-glob 'archive/minitrace/active/*/*.minitrace.json' --sql-file scripts/02-session-overview.sql`
  - `go-minitrace query commands --query-repository scripts archaeology extract demo-signals --archive-glob 'archive/minitrace/active/*/*.minitrace.json' --limit 1000`

### Technical details
- The converted sessions total 3,533 turns and 3,652 tool calls.
- Overall top tool families in the six transcripts: `bash` (1796), `read` (780), `edit` (412), `write` (275), and Playwright tooling.

## Step 3: Write the initial archaeology findings

I wrote the first pass of the analysis document to capture the highest-signal evidence for recovering a working demo. The key result is that the project has had two demo architectures: an older live Go server with `/api/*`, and the current static sql.js export.

The findings recommend using the current static export as the baseline working demo before deciding whether to resurrect the old live server. That keeps demo recovery grounded in the current code instead of immediately reintroducing a historical backend API the UI no longer calls.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Summarize the transcript archaeology in a durable ticket document with evidence and next steps.

**Inferred user intent:** Get back to a working demo with enough historical context to avoid rebuilding the wrong architecture.

### What I did
- Wrote `analysis/01-transcript-archaeology-findings.md`.
- Recorded the six session summaries and generated artifact paths.
- Captured evidence for the old live server:
  - `internal/server/spa.go`
  - `internal/server/server.go`
  - `internal/server/api_doc.go`
  - `cmd/codebase-browser/cmds/serve/run.go`
  - `./bin/codebase-browser serve --addr :3001`
  - `/api/index`
- Captured evidence for the later static/sql.js pivot.

### Why
- The investigation needs a human-readable synthesis, not just query outputs.
- The demo recovery question depends on distinguishing historical live-server code from the current static export architecture.

### What worked
- `archive/js-demo-signals-1000.json` surfaced direct old-server evidence from the 2026-04-20 session.
- The current repo already demonstrated a working static export flow earlier in this session using `review index`, `review export`, and `python3 -m http.server`.

### What didn't work
- The current repository no longer has a `serve` command wired into `cmd/codebase-browser/main.go`, so old live-server evidence is historical rather than directly runnable from HEAD.

### What I learned
- The 2026-04-20 session is the main source for old live-server/deployment recovery.
- The 2026-04-23 and 2026-04-30 sessions are the main sources for the static/sql.js redesign.

### What was tricky to build
- The transcript evidence is noisy because broad demo terms like `static`, `server`, and `deploy` match many tool calls. I handled this by combining aggregate queries with targeted term sampling and by recording caveats in the analysis doc.

### What warrants a second pair of eyes
- Validate the recommendation to prioritize static export over live-server restoration.
- Inspect old transcript evidence before any code resurrection, especially if the old `/api/*` routes were intentionally removed during the sql.js redesign.

### What should be done in the future
- Add a focused “restore serve?” decision record after extracting the exact old route shapes and comparing them to current `ui/src/api/sqlJsQueryProvider.ts`.

### Code review instructions
- Review `analysis/01-transcript-archaeology-findings.md` first.
- Then inspect `archive/demo-recovery-files.json` and `archive/js-demo-signals-1000.json` for cited evidence.

### Technical details
- Current known-good demo command sequence:
  - `./bin/codebase-browser review index --commits HEAD~5..HEAD --docs ./examples --db /tmp/gcb-run.db`
  - `./bin/codebase-browser review export --db /tmp/gcb-run.db --out /tmp/gcb-run-static`
  - `python3 -m http.server 8784 --directory /tmp/gcb-run-static`

## Step 4: Restore the live Go server command

I restored a live Go server path in the current codebase, but deliberately targeted the current SQLite review database instead of resurrecting the old embedded `index.json`/WASM runtime wholesale. The new `serve` command opens the same `codebase.db` that static export ships, serves optional static assets, and exposes Go-backed `/api/*` endpoints for health, index data, review docs, symbol search, source, and snippets.

This gives us a working live-Go demo surface again while keeping the current static sql.js architecture intact. The static export remains shareable and canonical; the live server is now a local/demo/development API runtime for people who want the queries executed in Go.

### Prompt Context

**User prompt (verbatim):** "restore live go server."

**Assistant interpretation:** Reintroduce a `codebase-browser serve` command and Go HTTP API support so the project can run a live server again.

**Inferred user intent:** Recover the old demo capability where codebase-browser could be launched as a Go process and queried over HTTP, especially for Go-side query execution instead of browser-only sql.js.

### What I did
- Added `cmd/codebase-browser/cmds/serve/run.go` with a Cobra `serve` command.
- Registered `serve.Register(rootCmd)` in `cmd/codebase-browser/main.go`.
- Added `internal/server/server.go` for routing, common JSON helpers, optional static-dir SPA fallback, and `/api/health`.
- Added `internal/server/api.go` for Go-backed SQLite endpoints:
  - `GET /api/index`
  - `GET /api/search?q=...&kind=...`
  - `GET /api/symbol?id=...`
  - `GET /api/source?path=...`
  - `GET /api/snippet?symbol=...`
- Added `internal/server/api_review.go` for review document endpoints:
  - `GET /api/review-docs`
  - `GET /api/review-docs/{slug}`
- Added `internal/server/server_test.go` with SQLite fixture tests for index/search/source/review-doc endpoints.
- Rebuilt `bin/codebase-browser`.

### Why
- The transcript archaeology showed the old live server existed, but the old implementation depended on removed `internal/web`, embedded index, and legacy frontend runtime pieces.
- Rebuilding against the current review SQLite database avoids reviving obsolete WASM/browser API assumptions while satisfying the live-Go requirement.

### What worked
- Focused tests pass:
  - `go test ./internal/server ./cmd/codebase-browser/cmds/serve ./cmd/codebase-browser -count=1`
- `go build -o bin/codebase-browser ./cmd/codebase-browser` succeeds.
- Manual live-server smoke test succeeds:
  - `./bin/codebase-browser serve --addr :3002 --db /tmp/gcb-run-static/db/codebase.db --static-dir /tmp/gcb-run-static`
  - `GET /api/health` returns `{"ok":true,"mode":"live-go",...}`.
  - `GET /api/review-docs` returns the four exported example review docs.
  - `GET /api/search?q=Export&kind=func` returns `internal/staticapp.Export`.
  - `GET /api/source?path=cmd/codebase-browser/main.go` returns source text from the SQLite DB.

### What didn't work
- `go test ./... -count=1` still fails because generated source snapshots under `internal/sourcefs/embed/source/...` contain old `github.com/go-go-golems/codebase-browser/...` import paths. This appears unrelated to the restored live server and predates this step as generated/source snapshot hygiene.

### What I learned
- The current SQLite DB has enough data to support a useful live server without reintroducing the deleted `internal/web` package.
- Serving the current static export through Go is possible via `--static-dir`, but the React UI still uses sql.js directly; the new `/api/*` endpoints are available for demos, scripts, and future UI switching.

### What was tricky to build
- The old server from history was not a clean drop-in because it was deleted as “obsolete Go server runtime” and depended on removed packages such as `internal/web`. The solution was to restore the product behavior rather than the exact old implementation: same `serve` command concept, but backed by current `codebase.db` tables (`commits`, `snapshot_*`, `file_contents`, and `static_review_rendered_docs`).

### What warrants a second pair of eyes
- The API shape is intentionally pragmatic and may not match the old `/api/*` contract exactly. Review whether future UI work should consume these endpoints or whether this should remain a demo/API compatibility layer.
- Review path handling in `--static-dir` fallback and `/api/source` before exposing the server beyond local development.

### What should be done in the future
- Add `/api/history/*` live endpoints if the UI should use Go for history diffs again.
- Add concept-query execution endpoints if we want to restore the old structured-query page behavior.
- Decide whether the React app should support a “live API provider” in addition to the current sql.js provider.

### Code review instructions
- Start with `cmd/codebase-browser/cmds/serve/run.go` for CLI behavior.
- Then review `internal/server/server.go`, `internal/server/api.go`, and `internal/server/api_review.go`.
- Validate with:
  - `go test ./internal/server ./cmd/codebase-browser/cmds/serve ./cmd/codebase-browser -count=1`
  - `go build -o bin/codebase-browser ./cmd/codebase-browser`
  - `./bin/codebase-browser serve --addr :3002 --db /tmp/gcb-run-static/db/codebase.db --static-dir /tmp/gcb-run-static`
  - `curl -fsS http://127.0.0.1:3002/api/health`
  - `curl -fsS http://127.0.0.1:3002/api/review-docs`

### Technical details
- The command defaults to `--addr :3001` and `--db internal/sqlite/embed/codebase.db`.
- `--static-dir` is optional. Without it, `/` serves a tiny live-server landing page; with it, Go serves the static export directory and falls back to `index.html` for SPA routes.

## Step 5: Prefer the live Go API from the React data layer

I changed the React data layer so the browser detects a live Go server by probing `/api/health`. When that endpoint reports the live server, the UI uses Go-backed endpoints for the high-traffic index, package, symbol, search, source, snippet, and review-document paths. When `/api/health` is absent, the static export behavior remains unchanged and the UI falls back to sql.js.

This keeps the static export deployable as a standalone artifact while making `codebase-browser serve --static-dir ...` genuinely exercise the restored Go API. Playwright network traces confirmed that the live-served UI calls `/api/health`, `/api/index`, `/api/review-docs`, and `/api/search` instead of loading `db/codebase.db` for the main page/search flow.

### Prompt Context

**User prompt (verbatim):** "yes, use go api for react ui"

**Assistant interpretation:** Update the frontend so it prefers the restored Go `/api/*` runtime when served by `codebase-browser serve`, instead of always querying SQLite in the browser via sql.js.

**Inferred user intent:** Make the restored live server visible in the product, not merely available as side-channel curl endpoints.

### What I did
- Added `ui/src/api/liveApiProvider.ts`:
  - probes `/api/health`,
  - normalizes live Go API rows into existing frontend types,
  - fetches `/api/index`, `/api/search`, `/api/symbol`, `/api/source`, `/api/snippet`, and review-doc endpoints.
- Added `ui/src/api/codebaseProvider.ts` with live-or-sql provider selection helpers.
- Updated `ui/src/api/indexApi.ts`, `ui/src/api/docApi.ts`, and `ui/src/api/sourceApi.ts` to prefer the live Go API where the server now supports it.
- Left unsupported live paths (snippet refs, source refs, file xrefs, history APIs) on sql.js for now, so the static export and advanced views still work.

### Why
- The frontend previously used only `SqlJsQueryProvider`, so restoring a Go server did not change UI runtime behavior.
- Runtime probing allows one build to support both modes:
  - static export: `/api/health` missing → sql.js,
  - live server: `/api/health` available → Go API.

### What worked
- `pnpm -C ui run typecheck` passes.
- `pnpm -C ui run build` passes.
- Focused Go tests still pass:
  - `go test ./internal/server ./cmd/codebase-browser/cmds/serve ./cmd/codebase-browser -count=1`
- Live UI smoke test via Playwright showed these network requests:
  - `GET /api/health` → 200
  - `GET /api/index` → 200
  - `GET /api/review-docs` → 200
  - after typing `Export`, `GET /api/search?q=Export` → 200

### What didn't work
- Initial TypeScript pass failed because `QueryError` only accepts the known `QueryErrorCode` union, not arbitrary HTTP status strings:
  - `Argument of type 'string' is not assignable to parameter of type 'QueryErrorCode'.`
- The first source API fallback attempt inferred `undefined` because `Promise.reject(...)` was used as a generic live branch. I simplified unsupported live endpoints to call sql.js directly for now.

### What I learned
- The existing RTK Query setup is already provider-oriented, so switching providers is localized to the API modules.
- The minimal reliable live-provider boundary is `/api/health`; it avoids false positives when the app is served by a generic static server.

### What was tricky to build
- The live Go server returns a flatter symbol shape (`startLine`, `endLine`, etc.) than the frontend `Symbol` type expects (`range`). I added normalization in `liveApiProvider.ts` so existing components do not need to know which runtime served the data.

### What warrants a second pair of eyes
- Review the provider-selection policy. It currently caches the `/api/health` result for the session; if a live API appears after the first failed probe, the page needs reload/test reset to detect it.
- Review the partial-live behavior: main index/search/source/review paths use Go, while history/xref helper paths still use sql.js.

### What should be done in the future
- Add Go endpoints and live-provider methods for xrefs, source refs, file xrefs, and history diff APIs.
- Add a visible runtime badge (`live-go` vs `sqljs-static`) so demos can prove which query backend is active without opening devtools.

### Code review instructions
- Start with `ui/src/api/liveApiProvider.ts` and `ui/src/api/codebaseProvider.ts`.
- Then review API module integration in:
  - `ui/src/api/indexApi.ts`
  - `ui/src/api/docApi.ts`
  - `ui/src/api/sourceApi.ts`
- Validate with:
  - `pnpm -C ui run typecheck`
  - `pnpm -C ui run build`
  - live-server Playwright/network check for `/api/index` and `/api/search`.

### Technical details
- Live mode command used for validation:
  - `./bin/codebase-browser serve --addr :3003 --db /tmp/gcb-live-ui/db/codebase.db --static-dir /tmp/gcb-live-ui`
- The validation bundle copied the newly built `ui/dist/public` into `/tmp/gcb-live-ui` and reused the previously exported DB/manifest files.

## Step 6: Build a stable multi-commit history demo

The review-doc errors were not normal for a polished demo. They came from two issues at once: the demo DB only contained a short moving `HEAD~N` window, and the review markdown used moving refs (`HEAD~5`, `HEAD~1`) plus package-short symbol refs that were too strict for `staticapp.Export`. I rebuilt the demo around a larger fixed commit range and replaced moving refs with explicit commit hashes.

The live server at `http://127.0.0.1:3003/` now serves `/tmp/gcb-history-demo`, which contains 43 indexed commits from `b91c6a3` through `79af1b0`. After a hard browser reload, the review docs resolve `staticapp.Export`, render diff stats, and show changed files without the stale `HEAD~5` errors.

### Prompt Context

**User prompt (verbatim):** "can you index enough commits to have decent history review docs, and also hardcode the versions so the reports don't go out of date?"

**Assistant interpretation:** Rebuild the demo database with a sufficiently deep history window and edit the example review docs to use stable commit hashes instead of moving `HEAD~N` refs.

**Inferred user intent:** Make the demo reliable over time and avoid reports breaking when HEAD moves or when the indexed range is too shallow.

### What I did
- Updated example review docs:
  - `examples/01-pr-review-static-export.md`: `staticapp.Export` diff now uses `b91c6a3 → 83dbe40`.
  - `examples/02-symbol-history-and-impact.md`: diff stats and changed files now use `b91c6a3 → 79af1b0`.
  - `examples/03-commit-walk-walkthrough.md`: commit walk now uses fixed hashes and a stable `staticapp.Export` diff step.
- Loosened short symbol resolution in `internal/docs/renderer.go` so package-local refs like `staticapp.Export` match package name/import-path suffixes, while still erroring on ambiguity.
- Added live Go `/api/history/diff` in `internal/server/api_history.go` and routed it from `internal/server/server.go`.
- Updated `ui/src/api/liveApiProvider.ts` and `ui/src/api/historyApi.ts` so diff stats/changed-files widgets use the live Go diff API when available.
- Rebuilt and exported the demo:
  - `./bin/codebase-browser review index --commits 0c3aace..79af1b0 --docs ./examples --db /tmp/gcb-history-demo.db`
  - `./bin/codebase-browser review export --db /tmp/gcb-history-demo.db --out /tmp/gcb-history-demo`
- Restarted the live server on port `3003` with `/tmp/gcb-history-demo`.

### Why
- Moving refs like `HEAD~5` are fragile in rendered reports because the report may be opened after HEAD changes or against a database with fewer indexed commits.
- The examples are intended as durable demo material, so they should use fixed commit boundaries.

### What worked
- The rebuilt DB contains 43 commits.
- `static_review_rendered_docs` has no non-empty `errors_json` rows after the rebuild.
- Playwright verified `#/review/01-pr-review-static-export` resolves `staticapp.Export` snippets.
- Playwright verified `#/review/02-symbol-history-and-impact` shows diff stats and changed files for `b91c6a3 → 79af1b0`.
- Validation commands passed:
  - `go test ./internal/server ./cmd/codebase-browser/cmds/serve ./cmd/codebase-browser ./internal/docs ./internal/staticapp -count=1`
  - `pnpm -C ui run typecheck`

### What didn't work
- Before a hard reload, the browser still showed the old errors because the already-loaded SPA kept old in-memory query state. A hard reload loaded the new hashed JS bundle and the rebuilt DB.

### What I learned
- The review renderer's old short-ref resolver only accepted full package IDs/import paths; example prose used package-local refs such as `staticapp.Export`. Supporting package names/import suffixes is better for authored review guides, as long as ambiguity remains an error.
- The live UI needed `/api/history/diff`; otherwise diff widgets in live mode still depended on sql.js and could hit stale in-memory DB state.

### What was tricky to build
- There were two superficially similar failures: pre-rendered symbol resolution errors in `static_review_rendered_docs`, and runtime diff-widget failures from commit resolution. The first required renderer/doc fixes and re-exporting the DB; the second required live history diff support and a browser hard reload.

### What warrants a second pair of eyes
- Review the package-short symbol resolution behavior for ambiguous package names across large multi-language repos.
- Review whether the hardcoded commit hashes should be documented in the example text so readers know the range is intentionally fixed.

### What should be done in the future
- Add a small command or docs-smoke variant that fails when `static_review_rendered_docs.errors_json != '[]'`.
- Add a visible runtime/DB build stamp to the UI so stale browser state is obvious during demos.

### Code review instructions
- Start with `examples/*.md` to review fixed commit boundaries.
- Then review `internal/docs/renderer.go` short-ref matching.
- Then review `internal/server/api_history.go` and `ui/src/api/historyApi.ts` for live diff routing.
- Validate with:
  - `sqlite3 /tmp/gcb-history-demo/db/codebase.db "select slug, errors_json from static_review_rendered_docs where errors_json != '[]';"`
  - open `http://127.0.0.1:3003/#/review/02-symbol-history-and-impact` and hard reload.

### Technical details
- Stable indexed range command: `--commits 0c3aace..79af1b0`.
- Stable report range: `b91c6a3 → 79af1b0`.
- Stable focused `Export` diff: `b91c6a3 → 83dbe40`.

## Step 7: Reindex a larger solid demo database

I rebuilt the live demo with a deeper history window so the history page and review widgets have enough real project evolution to be useful. The new demo DB covers the git-aware history implementation through the current stable demo HEAD, rather than only the later static-export tail.

The live server on port `3003` now serves `/tmp/gcb-solid-demo`, backed by `/tmp/gcb-solid-demo/db/codebase.db`. This DB contains 118 commits, no rendered review-doc errors, and enough file/symbol snapshots for substantial diff, changed-file, symbol-history, and impact widgets.

### Prompt Context

**User prompt (verbatim):** "index enough commits to have a solid working demo page."

**Assistant interpretation:** Rebuild the demo artifact with a larger commit range and restart the live server against that artifact.

**Inferred user intent:** Make the running demo page robust and impressive enough for browsing history/review functionality, not merely a minimal smoke test.

### What I did
- Indexed `025e4c6..79af1b0` into `/tmp/gcb-solid-demo.db`.
- Exported the static/live bundle to `/tmp/gcb-solid-demo`.
- Restarted `codebase-browser serve` on `:3003` using:
  - `--db /tmp/gcb-solid-demo/db/codebase.db`
  - `--static-dir /tmp/gcb-solid-demo`
- Checked rendered review docs for errors.
- Checked live `/api/history/diff?from=b91c6a3&to=79af1b0`.

### Why
- The previous 43-commit DB was clean but only covered the static-export era. Starting at `025e4c6` includes the GCB-009 git-aware history implementation and later docs/review/static-export evolution.

### What worked
- The new DB contains:
  - 118 commits
  - 3,761 snapshot package rows
  - 53,655 snapshot symbol rows
- `static_review_rendered_docs` has no rows where `errors_json != '[]'`.
- Live history diff for `b91c6a3 → 79af1b0` returns 42 changed files and populated stats.
- The live server reports healthy at `http://127.0.0.1:3003/api/health`.

### What didn't work
- N/A for this step.

### What I learned
- Indexing 118 commits took just under three minutes and produced a usable 87MB-class SQLite-backed demo artifact, which is acceptable for a local rich demo.

### What was tricky to build
- Picking the range mattered: all 191 commits would include early project phases but cost more and include more churn; `025e4c6..79af1b0` starts at the git-aware history work, which is a better demo boundary for history/review pages.

### What warrants a second pair of eyes
- Confirm whether `025e4c6..79af1b0` should become the canonical documented demo range, or whether CI should generate a shorter public artifact.

### What should be done in the future
- Add a make target such as `make demo-solid` that builds `/tmp/gcb-solid-demo` with this stable range and checks review-doc errors.

### Code review instructions
- Validate with:
  - `sqlite3 /tmp/gcb-solid-demo/db/codebase.db "select count(*) from commits;"`
  - `sqlite3 /tmp/gcb-solid-demo/db/codebase.db "select slug, errors_json from static_review_rendered_docs where errors_json != '[]';"`
  - `curl -fsS 'http://127.0.0.1:3003/api/history/diff?from=b91c6a3&to=79af1b0'`

### Technical details
- Build command:
  - `./bin/codebase-browser review index --commits 025e4c6..79af1b0 --docs ./examples --db /tmp/gcb-solid-demo.db`
  - `./bin/codebase-browser review export --db /tmp/gcb-solid-demo.db --out /tmp/gcb-solid-demo`

## Step 8: Repair broken review demo widgets in the live bundle

I investigated the broken review pages directly in the running browser. The commit-walk page was rendering `overview` and `symbol` steps as unknown widget kinds, and the PR review page was still trying to resolve the body diff through the static sql.js path where short commit `b91c6a3` was not available.

I fixed those as real widget/API gaps rather than just restarting the server. The rebuilt bundle now serves body diffs through the live Go API when available, allows commit-walk steps to inherit the parent `from`/`to` range, renders overview/note/symbol steps, and treats `codebase-file` directives as file snippets instead of accidentally querying symbol `:0`.

### Prompt Context

**User prompt (verbatim):** "the many demo pages are kinda broken. see for the example the walk one."

**Assistant interpretation:** Inspect the live demo pages, identify why the example review widgets are broken, repair the implementation, rebuild/restart the demo, and validate in-browser.

**Inferred user intent:** Make all shipped demo review pages clean enough to present without visible widget errors or stale commit-ref failures.

### What I did
- Reproduced `03-commit-walk-walkthrough` in Playwright and saw `Unknown commit walk step kind: overview`.
- Reproduced `01-pr-review-static-export` and saw `Failed to load diff: {"status":"NOT_FOUND","data":"commit ref not found: b91c6a3"}`.
- Added live Go endpoint `GET /api/history/symbol-body-diff`.
- Updated the React live provider and history API to prefer that endpoint for body diffs.
- Updated commit-walk parsing so step params inherit top-level `from`, `to`, and `commit` values.
- Updated the commit-walk widget to render `overview`, `note`, and `symbol` steps.
- Updated `codebase-file` rendering metadata and the React doc snippet hydrator to render actual file snippets.
- Re-exported `/tmp/gcb-solid-demo` and restarted the live server on port `3003`.

### Why
- A restart alone would not fix these because the failures were implementation gaps: some widgets only knew the static sql.js path, and some rendered directive kinds had no React component behavior.

### What worked
- `go test ./internal/docs ./internal/server ./internal/staticapp -count=1` passed.
- `pnpm -C ui run typecheck` passed.
- `GET /api/history/symbol-body-diff?from=b91c6a3&to=83dbe40&symbol=...Export` returns a populated diff.
- Playwright checks for all four review pages found no visible `Failed`, `Unknown`, `not found`, `No history found`, or lingering `Loading…` states.
- Playwright checked all six commit-walk steps and found no visible widget errors.
- Browser console had no new errors after the rebuild.

### What didn't work
- The first rebuild fixed the `overview` and body-diff errors but exposed `Unknown commit walk step kind: symbol` on step 3.
- The file/annotation example also emitted a hidden console error for `/api/symbol?id=:0` because `codebase-file` fell through to symbol hydration despite having no symbol id.

### What I learned
- The static rendered docs can be clean at the SQLite/rendering layer while still having broken hydrated widgets in React. Browser-level validation needs to click through interactive states, not just query `static_review_rendered_docs.errors_json`.

### What was tricky to build
- The failures were split across layers: renderer metadata, React widget dispatch, and live-vs-sql.js provider selection. The symptom looked like stale commits, but the root cause for `b91c6a3` was that the symbol body diff was not live-provider aware.

### What warrants a second pair of eyes
- `simpleUnifiedDiff` is intentionally minimal; review whether the body-diff API should use a richer diff library or existing renderer before this becomes a long-term API contract.
- `codebase-file` now fetches full source and slices client-side; that is fine for demo pages but may need a range-aware endpoint for large files.

### What should be done in the future
- Add an automated Playwright smoke test that visits every rendered review doc and clicks through commit-walk steps looking for visible error/loading text and console errors.

### Code review instructions
- Start with:
  - `internal/server/api_history.go` for the new body-diff endpoint.
  - `internal/docs/renderer.go` for commit-walk inheritance and file metadata.
  - `ui/src/features/doc/widgets/CommitWalkWidget.tsx` for supported step kinds.
  - `ui/src/features/doc/DocSnippet.tsx` for `codebase-file` hydration.
- Validate with:
  - `go test ./internal/docs ./internal/server ./internal/staticapp -count=1`
  - `pnpm -C ui run typecheck`
  - Browser smoke pages under `http://127.0.0.1:3003/?v=<n>#/review/...`

### Technical details
- Live server health after rebuild:
  - `{"ok":true,"mode":"live-go","staticDir":"/tmp/gcb-solid-demo"}`

## Step 9: Make symbol-history demos use the full live history and a more interesting symbol

I investigated why the symbol-history page appeared to have only a handful of commits even though the solid demo database contained 118 indexed commits. The root cause was not insufficient indexing: the SQLite artifact already had 117 rows for `cmd/codebase-browser.func.main`; the UI was still reading commit lists and symbol history from the static sql.js database path, which could be stale inside a running browser tab.

I moved commit lists and symbol-history reads to the live Go API when available, kept sql.js as the fallback, and changed the second review demo to use `history.newScanCmd`, which has visible body changes in the first 20 history rows. I also made commit-walk bodies render as explicit “Guide note” callouts so the notes are visible instead of looking like ordinary muted prose.

### Prompt Context

**User prompt (verbatim):** "http://127.0.0.1:3003/?v=5#/review/02-symbol-history-and-impact <- can you index enough commits or choose other symbols for this to show something interesting (some diffs, etc...)"

**Assistant interpretation:** Ensure the symbol-history review page demonstrates meaningful history and diffs, either by indexing enough commits or by choosing a better symbol.

**Inferred user intent:** Make the demo compelling: visible body changes, an explicit diff, impact data, and enough commit history to prove the history feature works.

### What I did
- Confirmed `/tmp/gcb-solid-demo/db/codebase.db` contains 118 commits.
- Confirmed `cmd/codebase-browser.func.main` has 117 symbol-history rows in SQLite.
- Added live endpoints:
  - `GET /api/history/commits`
  - `GET /api/history/symbol?symbol=...&limit=...`
- Wired React history APIs to prefer live Go for commit lists and symbol history.
- Added no-store headers for `index.html`, `manifest.json`, and `db/*` static files to reduce stale bundle/database confusion.
- Updated `examples/02-symbol-history-and-impact.md` to demonstrate `history.newScanCmd` with:
  - 20 visible history rows
  - 3 body changes
  - an explicit body diff from `05f3ffe` to `7c095d0`
  - impact from callers of `newScanCmd`
- Reindexed the full `025e4c6..79af1b0` range and re-exported `/tmp/gcb-solid-demo`.
- Restarted the live server on port `3003`.

### Why
- The problem was partly demo content and partly runtime data freshness. The old `AddRenderedReviewDocs` symbol only had a few visible rows, and the standalone `/history?symbol=...` route was vulnerable to stale sql.js state.

### What worked
- `GET /api/history/commits` returns 118 commits.
- `GET /api/history/symbol?...cmd/codebase-browser.func.main` returns 117 rows.
- The updated review page shows `history.newScanCmd` with 20 commits and 3 body changes.
- The explicit diff for `newScanCmd` renders successfully.
- Playwright smoke checks found no visible errors across all four review pages.
- `docmgr doctor --ticket GCB-017 --stale-after 30` passed before this step; rerun after final doc updates if continuing.

### What didn't work
- Re-exporting alone did not update the review prose because `/tmp/gcb-solid-demo.db` still contained the previously indexed `review_docs` rows. I had to rerun `review index --docs ./examples` for the full demo range before exporting.

### What I learned
- Review markdown lives in the demo SQLite artifact after indexing. Editing `examples/*.md` requires re-running `review index`, not only `review export`.
- A large history DB can still look tiny if the frontend route is served from an old in-memory sql.js database. Live history endpoints are a better default for the local demo server.

### What was tricky to build
- The confusing symptom was “only 4 commits,” but there were two different counts: the database had enough history, while the browser was holding older static/sql.js state. Adding live endpoints fixed the authoritative path, and no-store headers reduce future stale-asset surprises.

### What warrants a second pair of eyes
- Decide whether all history-related providers, including impact, should move to live Go when available. Impact still uses the sql.js provider today.
- Confirm whether the `history.newScanCmd` example is the right public-facing story or whether a less internal command-builder symbol would be clearer.

### What should be done in the future
- Add a reproducible `demo-solid` target that re-runs `review index`, `review export`, server restart, and browser smoke checks in one command.

### Code review instructions
- Start with:
  - `internal/server/api_history.go` for live commit/symbol history endpoints.
  - `ui/src/api/historyApi.ts` and `ui/src/api/liveApiProvider.ts` for live-provider selection.
  - `examples/02-symbol-history-and-impact.md` for the new demo symbol/story.
  - `ui/src/features/doc/widgets/CommitWalkWidget.tsx` for the visible guide-note callouts.
- Validate with:
  - `go test ./internal/server ./internal/docs ./internal/staticapp -count=1`
  - `pnpm -C ui run typecheck`
  - `curl -fsS http://127.0.0.1:3003/api/history/commits`
  - browser visits to `/review/02-symbol-history-and-impact`, `/review/03-commit-walk-walkthrough`, and `/history?symbol=...cmd...main`.

### Technical details
- Reindex command:
  - `./bin/codebase-browser review index --commits 025e4c6..79af1b0 --docs ./examples --db /tmp/gcb-solid-demo.db`
- Export command:
  - `./bin/codebase-browser review export --db /tmp/gcb-solid-demo.db --out /tmp/gcb-solid-demo`

## Step 10: Add reproducible demo build and smoke targets

I turned the manual demo rebuild sequence into Makefile targets so the solid 118-commit demo can be recreated, served, and checked without remembering the exact commands. This also updates `docs-smoke` to use the same stable commit range as the examples, because the review markdown now contains hardcoded historical refs that are not valid in a tiny `HEAD~5..HEAD` window.

The new targets make the local workflow explicit: build the stable demo artifact, restart the live server for it, and smoke-test both the running API and exported review-doc render state.

### Prompt Context

**User prompt (verbatim):** "continue"

**Assistant interpretation:** Continue with the next hardening step after repairing the live demo pages.

**Inferred user intent:** Make the demo recovery work reproducible and less fragile for future review/presentation sessions.

### What I did
- Added Makefile variables for the stable demo:
  - `DEMO_COMMITS ?= 025e4c6..79af1b0`
  - `DEMO_DB ?= /tmp/gcb-solid-demo.db`
  - `DEMO_OUT ?= /tmp/gcb-solid-demo`
  - `DEMO_ADDR ?= :3003`
- Added targets:
  - `make demo-solid`
  - `make demo-serve`
  - `make demo-solid-serve`
  - `make demo-smoke`
- Updated `make docs-smoke` to index the stable demo range and check rendered review-doc errors.

### Why
- The example docs now intentionally reference stable historical commits such as `b91c6a3`, `05f3ffe`, `7c095d0`, and `79af1b0`. A small moving `HEAD~5..HEAD` smoke DB cannot validate those docs reliably.

### What worked
- `make demo-smoke` passed:
  - 118 commits
  - 117 `main` symbol-history rows
  - 0 rendered review-doc errors
- `make docs-smoke` passed against the full stable range.

### What didn't work
- N/A for this step.

### What I learned
- The smoke target needs to validate both API history counts and rendered review-doc errors; either one alone can miss stale or broken demo behavior.

### What was tricky to build
- The Makefile target needed to avoid the old moving commit window while still remaining configurable. Using `DEMO_COMMITS` as the shared range keeps the docs smoke and rich local demo aligned.

### What warrants a second pair of eyes
- `docs-smoke` is now slower because it indexes 118 commits. Review whether CI should use this full range or whether we should add a separate lighter fixture for CI.

### What should be done in the future
- Add a true browser-level smoke script that opens each review page and clicks commit-walk steps, complementing the current API/artifact checks.

### Code review instructions
- Start with `Makefile` targets `docs-smoke`, `demo-solid`, `demo-serve`, and `demo-smoke`.
- Validate with:
  - `make demo-smoke`
  - `make docs-smoke`

### Technical details
- `demo-solid` removes and recreates `/tmp/gcb-solid-demo.db` and `/tmp/gcb-solid-demo`.
- `demo-smoke` checks live `/api/health`, `/api/history/commits`, `/api/history/symbol?...main`, and `static_review_rendered_docs.errors_json`.

## Step 11: Deploy the live demo to the yolo K3s cluster

I deployed the repaired live demo to the yolo cluster at `https://codebase-browser.yolo.scapegoat.dev`. The existing ArgoCD `codebase-browser` Application was present but unhealthy: it ran an old nginx-style image while the probes expected `/api/index`, so Kubernetes repeatedly restarted it with 404 probe failures.

I built a new container image that embeds the current `codebase-browser` binary plus the 118-commit static export under `/app/static`, pushed it to GHCR, updated the Hetzner K3s GitOps deployment to that image, and let ArgoCD roll it out. The live Application is now Synced and Healthy.

### Prompt Context

**User prompt (verbatim):** "deployed it to yolo (we removed it a while back), look at ~/code/wesen/2026-03-27--hetzner-k3s/ (potentially git history if we actually removed it)"

**Assistant interpretation:** Use the Hetzner K3s GitOps repository to redeploy the fixed codebase-browser demo to the yolo cluster, restoring or updating the previous codebase-browser deployment as needed.

**Inferred user intent:** Make the demo available publicly on the yolo cluster, not only on localhost.

### What I did
- Inspected `/home/manuel/code/wesen/2026-03-27--hetzner-k3s`.
- Found existing manifests:
  - `gitops/applications/codebase-browser.yaml`
  - `gitops/kustomize/codebase-browser/*`
- Checked the live cluster via `.cache/kubeconfig-tailnet.yaml`.
- Confirmed the existing deployment was unhealthy because the old image served nginx static files and returned 404 for `/api/index` probes.
- Updated the application image packaging:
  - `Dockerfile` now copies `bin/static` and runs `serve --db /app/static/db/codebase.db --static-dir /app/static`.
  - `.dockerignore` now includes `bin/static/**`.
- Built `/tmp/gcb-solid-demo`, copied it into `bin/static`, built and smoke-tested the image locally.
- Pushed image:
  - `ghcr.io/go-go-golems/codebase-browser:yolo-20260704-solid-demo`
  - digest `sha256:22a8fc682e28dc702f0c5ce7c3c78442e74c80b9015dee5d49fe41437ad3dddf`
- Updated GitOps deployment in `/home/manuel/code/wesen/2026-03-27--hetzner-k3s/gitops/kustomize/codebase-browser/deployment.yaml`:
  - new image tag
  - probes moved to `/api/health`
  - memory request/limit raised for the embedded 225MB DB
- Committed and pushed GitOps commit:
  - `bc92125f3f4fd8eec50f910656006f0fdf5793e2` — `Deploy codebase-browser live demo image`
- Annotated the ArgoCD Application for hard refresh and waited for rollout.

### Why
- The cluster already had a `codebase-browser` Application, but it was not serving the repaired live Go demo. Updating the image/probes through GitOps avoids ArgoCD self-healing a manual deployment patch back to the broken image.

### What worked
- Local Docker smoke passed:
  - `/api/health` OK
  - 118 commits
  - 117 `main` symbol-history rows
- Cluster rollout passed:
  - `deployment "codebase-browser" successfully rolled out`
  - ArgoCD reports `Synced` / `Healthy` at Git revision `bc92125f3f4fd8eec50f910656006f0fdf5793e2`.
- Public HTTPS checks passed:
  - `https://codebase-browser.yolo.scapegoat.dev/api/health`
  - `/api/history/commits` returns 118 rows
  - `/api/history/symbol?...cmd/codebase-browser.func.main` returns 117 rows
- Browser smoke on yolo showed the updated symbol-history page and visible commit-walk guide notes.

### What didn't work
- The previous live pod was stuck in CrashLoopBackOff/Progressing due liveness/readiness probes hitting `/api/index` against an nginx static image that did not provide the live Go API.
- The first public browser smoke still showed `Loading impact…` briefly because impact still hydrates through sql.js; after waiting, it resolved.

### What I learned
- The yolo deployment existed but was effectively broken rather than removed from Git. The GitOps repo still had the Application and namespace, and the cluster had a 75-day-old namespace/deployment.
- For the current demo, `/api/health` is the right probe path; `/api/index` is heavier and less stable as a Kubernetes health probe.

### What was tricky to build
- The Dockerfile had drifted from the current server CLI. It still invoked removed flags (`--history-db`, `--repo-root`) and expected `bin/history.db`; the new runtime needs a static export directory and a `codebase.db` path.

### What warrants a second pair of eyes
- The image embeds a 225MB SQLite database, so the pushed image is demo-specific and large. Review whether future deployments should use an init container/PVC/artifact download instead.
- The codebase-browser repo still has uncommitted Dockerfile/.dockerignore updates and many implementation changes; decide how to split commits.

### What should be done in the future
- Update GitHub Actions image publishing to build the new `demo-solid` artifact and use the current `serve --db --static-dir` CLI.
- Add a deployment smoke target that checks the public yolo URL after GitOps rollout.

### Code review instructions
- Deployment state to review:
  - `/home/manuel/code/wesen/2026-03-27--hetzner-k3s/gitops/kustomize/codebase-browser/deployment.yaml`
  - GitOps commit `bc92125f3f4fd8eec50f910656006f0fdf5793e2`
- Runtime packaging to review:
  - `/home/manuel/code/wesen/2026-04-19--go-codebase-browser/Dockerfile`
  - `/home/manuel/code/wesen/2026-04-19--go-codebase-browser/.dockerignore`
- Validate with:
  - `curl -fsS https://codebase-browser.yolo.scapegoat.dev/api/health`
  - `curl -fsS https://codebase-browser.yolo.scapegoat.dev/api/history/commits`
  - `kubectl --kubeconfig /home/manuel/code/wesen/2026-03-27--hetzner-k3s/.cache/kubeconfig-tailnet.yaml -n argocd get application codebase-browser`

### Technical details
- Public URL:
  - `https://codebase-browser.yolo.scapegoat.dev/`
- Image:
  - `ghcr.io/go-go-golems/codebase-browser:yolo-20260704-solid-demo`
- Image digest:
  - `sha256:22a8fc682e28dc702f0c5ce7c3c78442e74c80b9015dee5d49fe41437ad3dddf`

## Step 12: Move impact and commit snippet hydration off the frontend SQLite database

I migrated the remaining review-page impact lookup to the live Go backend so the browser no longer needs to fetch `db/codebase.db` just to render `02-symbol-history-and-impact`. While checking that path, I also found direct sql.js snippet calls in annotation/commit-snippet hydration, so I moved those through the existing provider layer and added commit-aware live snippet/source requests.

The yolo deployment now serves a new image with backend impact support. Browser network validation on the public review pages showed `/api/history/impact` and `/api/snippet` requests, with no `db/codebase.db` or `sql-wasm` requests for the review pages tested.

### Prompt Context

**User prompt (verbatim):** "We seem to still load the full DB on the frontend, for example here: https://codebase-browser.yolo.scapegoat.dev/#/review/02-symbol-history-and-impact to answer the impact query. That should also be migrated to the backend, there should be no need to access the sqlite DB at all in the frontend"

**Assistant interpretation:** Eliminate frontend SQLite loading for live-server review pages, starting with the impact widget, and redeploy the yolo demo.

**Inferred user intent:** The public demo should use the backend API for data queries rather than making users download a large SQLite database in the browser.

### What I did
- Added live backend endpoint:
  - `GET /api/history/impact?symbol=...&direction=usedby|uses&depth=N&commit=...`
- Ported the impact BFS from the sql.js provider into Go against `snapshot_refs` and `snapshot_symbols`.
- Wired `historyApi.getImpact` to prefer the live provider and fall back to sql.js only in purely static mode.
- Made live source/snippet provider methods accept optional `commit=`.
- Updated `AnnotationWidget` to use `sourceApi.useGetSnippetQuery` instead of `getSqlJsProvider()` directly.
- Updated `DocSnippet` commit signature/snippet hydration to use RTK/provider-backed snippet queries instead of direct sql.js access.
- Rebuilt the solid demo, rebuilt and pushed image:
  - `ghcr.io/go-go-golems/codebase-browser:yolo-20260705-backend-impact`
  - digest `sha256:240714c62ab47a11021d1c1cb9b10a01bcbea2e2d9139ad4439d5d1c4a75482c`
- Updated the yolo GitOps deployment image and pushed GitOps commit:
  - `cd435b7` — `Deploy backend impact codebase-browser demo`
- ArgoCD then advanced to `a996fae588c32d45b4d3d1ef1a5f20e8410db1cc` after another GitOps commit and remained `Synced` / `Healthy`.

### Why
- Downloading the 225MB SQLite database in the frontend defeats the purpose of running the live Go API. It is acceptable for static exports, but not for the yolo live deployment.

### What worked
- `go test ./internal/server ./internal/docs ./internal/staticapp -count=1` passed.
- `pnpm -C ui run typecheck` passed.
- Local `/api/history/impact` returned the expected two-node impact chain for `history.newScanCmd`.
- Public yolo `/api/history/impact` returned a 200 response with impact nodes.
- Playwright network checks on the public review pages saw no `db/codebase.db` or `sql-wasm` requests; review widgets used `/api/history/impact` and `/api/snippet` instead.
- ArgoCD reports `codebase-browser` as `Synced` / `Healthy`.

### What didn't work
- `make demo-serve` briefly failed because port `3003` was already occupied during the restart check. Starting the server manually confirmed the updated local server was healthy.

### What I learned
- The impact widget was not the only sql.js path on review pages: annotation and commit-snippet hydration had direct provider calls too.
- Static mode still needs sql.js fallback, but live mode can avoid database downloads for the review/demo paths we exercised.

### What was tricky to build
- The Go impact endpoint had to preserve the frontend response shape exactly (`root`, `direction`, `depth`, `commit`, `nodes`, `edges`) while translating the TypeScript BFS into SQL-backed Go helpers.
- Playwright request listeners can accumulate across repeated `run_code` calls, so the useful validation signal is whether any `db/codebase` or `sql-wasm` request appears, not the full repeated list of API requests.

### What warrants a second pair of eyes
- `xrefApi`, `getSnippetRefs`, `getSourceRefs`, and `getFileXref` still have sql.js fallback/provider code. They may still download the DB on non-review interactive symbol/source pages unless those paths are migrated too.
- The yolo image was built from the current working tree before the codebase-browser changes were committed; split and commit those changes for reproducibility.

### What should be done in the future
- Add live backend endpoints for xrefs, snippet refs, source refs, and file xrefs, then make the live UI never initialize sql.js unless the backend is unavailable.
- Add a browser smoke assertion that fails if `db/codebase.db` or `sql-wasm.wasm` is requested while `/api/health` reports `live-go`.

### Code review instructions
- Start with:
  - `internal/server/api_history.go` (`handleImpact`, `queryImpact`, `queryImpactEdges`)
  - `ui/src/api/liveApiProvider.ts` (`getImpact`, commit-aware source/snippet methods)
  - `ui/src/api/historyApi.ts` (`getImpact` live-first provider selection)
  - `ui/src/features/doc/widgets/AnnotationWidget.tsx` and `ui/src/features/doc/DocSnippet.tsx` (no direct sql.js snippet hydration)
- Validate with:
  - `go test ./internal/server ./internal/docs ./internal/staticapp -count=1`
  - `pnpm -C ui run typecheck`
  - `curl -fsS 'https://codebase-browser.yolo.scapegoat.dev/api/history/impact?symbol=sym%3Agithub.com%2Fwesen%2Fcodebase-browser%2Fcmd%2Fcodebase-browser%2Fcmds%2Fhistory.func.newScanCmd&direction=usedby&depth=2'`
  - Browser network check on `/review/02-symbol-history-and-impact` for absence of `db/codebase.db` and `sql-wasm`.

### Technical details
- Public impact API validation:
  - `impact=2 Register`
- Active yolo image:
  - `ghcr.io/go-go-golems/codebase-browser:yolo-20260705-backend-impact`

## Step 13: Add backend xref endpoints before deleting sql.js

This step closed the largest backend API gap that prevented removal of the frontend SQLite runtime. The live Go server now exposes xref, snippet-reference, source-reference, and file-xref endpoints whose response shapes match the existing React contracts.

I implemented this before touching frontend provider selection so the UI can move one endpoint family at a time from sql.js to HTTP without losing widget behavior.

### Prompt Context

**User prompt (verbatim):** "The conversation history before this point was compacted into the following summary:

<summary>
## Goal
Restore a working `codebase-browser` live Go demo, make the React UI use the Go API when available, build stable history-rich review docs that do not depend on moving `HEAD~N` refs, and deploy the demo publicly to yolo without frontend SQLite DB downloads for review pages.

## Constraints & Preferences
- Use `go-minitrace` transcript archaeology and keep work under docmgr ticket `GCB-017`.
- Store reusable SQL/JS scripts in the ticket `scripts/` folder.
- Keep a diary while working.
- Hardcode commit versions so reports do not go out of date.
- Prefer Go API queries for the React UI when served live.
- In live mode, review/demo pages should not fetch `db/codebase.db` or initialize `sql-wasm`.

## Progress
### Done
- [x] Created ticket `GCB-017` at `ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts`.
- [x] Converted six Pi transcripts into `archive/minitrace/active/*/*.minitrace.json`.
- [x] Wrote transcript analysis scripts under `ttmp/.../scripts/`.
- [x] Restored `codebase-browser serve` in `cmd/codebase-browser/cmds/serve/run.go` and registered it in `cmd/codebase-browser/main.go`.
- [x] Added live Go API in `internal/server/*`, including review docs, source/snippet, history diff, body diff, commit list, symbol history, and impact endpoints.
- [x] Added React live API provider:
  - `ui/src/api/liveApiProvider.ts`
  - `ui/src/api/codebaseProvider.ts`
  - updated `ui/src/api/indexApi.ts`, `docApi.ts`, `sourceApi.ts`, `historyApi.ts`
- [x] Fixed package-local refs like `staticapp.Export` in `internal/docs/renderer.go`.
- [x] Hardcoded stable demo refs in `examples/01-pr-review-static-export.md`, `examples/02-symbol-history-and-impact.md`, `examples/03-commit-walk-walkthrough.md`.
- [x] Built stable rich demo range `025e4c6..79af1b0`:
  - DB: `/tmp/gcb-solid-demo.db`
  - export: `/tmp/gcb-solid-demo`
  - 118 commits
  - 0 rendered review-doc errors
- [x] Restarted local live server on `http://127.0.0.1:3003` using `/tmp/gcb-solid-demo/db/codebase.db`.
- [x] Fixed broken hydrated review widgets:
  - commit-walk `overview`, `note`, `symbol`
  - inherited walk `from/to/commit`
  - live `/api/history/symbol-body-diff`
  - `codebase-file` hydration
  - visible commit-walk “Guide note” callouts
- [x] Updated `examples/02-symbol-history-and-impact.md` to use richer `history.newScanCmd` demo with 20 rows, 3 body changes, explicit diff `05f3ffe → 7c095d0`, and impact.
- [x] Added Makefile targets:
  - `demo-solid`
  - `demo-serve`
  - `demo-solid-serve`
  - `demo-smoke`
  - updated `docs-smoke` to use `DEMO_COMMITS ?= 025e4c6..79af1b0`
- [x] Validated:
  - `go test ./internal/server ./internal/docs ./internal/staticapp -count=1`
  - `pnpm -C ui run typecheck`
  - `make demo-smoke`
  - `make docs-smoke`
  - Playwright review-page smoke checks
  - `docmgr doctor --ticket GCB-017 --stale-after 30`
- [x] Updated runtime packaging:
  - `Dockerfile` now serves embedded `bin/static` with `serve --db /app/static/db/codebase.db --static-dir /app/static`
  - `.dockerignore` includes `bin/static/**`
- [x] Deployed to yolo:
  - public URL: `https://codebase-browser.yolo.scapegoat.dev/`
  - GitOps repo: `/home/manuel/code/wesen/2026-03-27--hetzner-k3s`
  - app path: `gitops/kustomize/codebase-browser/deployment.yaml`
  - ArgoCD app: `codebase-browser`
- [x] Fixed old yolo deployment:
  - old pod was nginx/static and failed probes with `/api/index` 404
  - probes changed to `/api/health`
  - memory raised to request `256Mi`, limit `768Mi`
- [x] Pushed yolo images:
  - `ghcr.io/go-go-golems/codebase-browser:yolo-20260704-solid-demo`
  - `ghcr.io/go-go-golems/codebase-browser:yolo-20260705-backend-impact`
- [x] Pushed GitOps commits:
  - `bc92125 Deploy codebase-browser live demo image`
  - `cd435b7 Deploy backend impact codebase-browser demo`
  - current Argo revision later advanced to `a996fae588c32d45b4d3d1ef1a5f20e8410db1cc`
- [x] Migrated impact and review-page snippet hydration off frontend SQLite:
  - backend endpoint `GET /api/history/impact`
  - `historyApi.getImpact` live-first
  - `AnnotationWidget` uses provider-backed `useGetSnippetQuery`
  - `DocSnippet` commit snippets/signatures use provider-backed queries
  - live `/api/snippet` and `/api/source` support optional `commit=`
- [x] Verified yolo:
  - ArgoCD `Synced / Healthy`
  - `/api/health` OK
  - `/api/history/commits` returns 118
  - `main` symbol history returns 117
  - `/api/history/impact?...history.func.newScanCmd...` returns impact nodes
  - Playwright network checks on review pages saw no `db/codebase.db` or `sql-wasm` requests.

### In Progress
- [ ] Split and commit the large `codebase-browser` working tree changes cleanly.
- [ ] Migrate remaining non-review sql.js paths for live mode.

### Blocked
- Full `go test ./...` previously failed due stale generated source snapshots under `internal/sourcefs/embed/source/...` importing `github.com/go-go-golems/codebase-browser/...`; focused tests pass.

## Key Decisions
- **Restore behavior, not old implementation**: Live server is backed by current SQLite `codebase.db`, not the deleted embedded `index.json`/`internal/web` runtime.
- **Runtime provider detection**: React probes `/api/health`; if available, it uses Go API, otherwise falls back to sql.js.
- **Stable demo refs**: Replaced moving `HEAD~N` refs with fixed commits to keep reports reproducible.
- **Larger history window**: Canonical local demo range is now `025e4c6..79af1b0` with 118 commits.
- **Package-local symbol refs allowed**: `staticapp.Export` now resolves by package name/import suffix, with ambiguity still treated as an error.
- **Live demo should avoid frontend DB downloads**: Review pages now use backend impact/snippet APIs; sql.js remains fallback for static export mode.
- **Yolo deployment via GitOps**: Updated `/home/manuel/code/wesen/2026-03-27--hetzner-k3s` and let ArgoCD roll out instead of manual-only patches.
- **Health probe path**: Use `/api/health` for Kubernetes probes instead of heavier `/api/index`.

## Next Steps
1. Commit/split the `codebase-browser` changes, especially server/API, UI provider migration, docs renderer, examples, Makefile, Dockerfile, and `.dockerignore`.
2. Add live backend endpoints for remaining sql.js fallback paths:
   - `xrefApi`
   - `getSnippetRefs`
   - `getSourceRefs`
   - `getFileXref`
3. Add browser smoke test that fails if `db/codebase.db` or `sql-wasm.wasm` is requested while `/api/health` reports `live-go`.
4. Update GitHub Actions image publishing to build the new `demo-solid` artifact and current `serve --db --static-dir` image.
5. Decide whether yolo’s large embedded 225MB SQLite DB image is acceptable or should become init-container/PVC/artifact-download based.
6. Re-run final validation:
   - `go test ./internal/server ./internal/docs ./internal/staticapp -count=1`
   - `pnpm -C ui run typecheck`
   - `make demo-smoke`
   - public yolo curl checks
   - `docmgr doctor --ticket GCB-017 --stale-after 30`

## Critical Context
- Original user-visible errors included:
  - `line 13: symbol "staticapp.Export" not found`
  - `line 18: symbol "staticapp.Export" not found`
  - `line 23: symbol "staticapp.Export" not found`
  - `line 9: symbol "staticapp.AddRenderedReviewDocs" not found`
  - `Failed to load changed files: {"status":"NOT_FOUND","data":"commit ref not found: HEAD~5"}`
  - `Unknown commit walk step kind: overview`
  - `Unknown commit walk step kind: symbol`
  - `Failed to load diff: {"status":"NOT_FOUND","data":"commit ref not found: b91c6a3"}`
  - frontend downloading full SQLite DB for impact on `https://codebase-browser.yolo.scapegoat.dev/#/review/02-symbol-history-and-impact`
- Stable refs/ranges:
  - canonical demo range: `025e4c6..79af1b0`
  - broad review range: `b91c6a3 → 79af1b0`
  - focused `staticapp.Export` diff: `b91c6a3 → 83dbe40`
  - focused `history.newScanCmd` diff: `05f3ffe → 7c095d0`
- Local live server:
  ```bash
  ./bin/codebase-browser serve \
    --addr :3003 \
    --db /tmp/gcb-solid-demo/db/codebase.db \
    --static-dir /tmp/gcb-solid-demo
  ```
- Public yolo URL:
  - `https://codebase-browser.yolo.scapegoat.dev/`
- Active yolo image:
  - `ghcr.io/go-go-golems/codebase-browser:yolo-20260705-backend-impact`
- Important modified files include:
  - `.dockerignore`
  - `Dockerfile`
  - `Makefile`
  - `cmd/codebase-browser/main.go`
  - `cmd/codebase-browser/cmds/serve/run.go`
  - `examples/01-pr-review-static-export.md`
  - `examples/02-symbol-history-and-impact.md`
  - `examples/03-commit-walk-walkthrough.md`
  - `internal/docs/renderer.go`
  - `internal/server/api.go`
  - `internal/server/api_history.go`
  - `internal/server/api_review.go`
  - `internal/server/server.go`
  - `internal/server/server_test.go`
  - `ui/src/api/codebaseProvider.ts`
  - `ui/src/api/docApi.ts`
  - `ui/src/api/historyApi.ts`
  - `ui/src/api/indexApi.ts`
  - `ui/src/api/liveApiProvider.ts`
  - `ui/src/api/sourceApi.ts`
  - `ui/src/features/doc/DocSnippet.tsx`
  - `ui/src/features/doc/widgets/AnnotationWidget.tsx`
  - `ui/src/features/doc/widgets/CommitWalkWidget.tsx`
- Current `codebase-browser` repo still has uncommitted implementation/deployment packaging changes and ticket docs.
- Hetzner K3s repo is pushed; latest visible commits include:
  - `a996fae retro-obsidian-publish: pull public GHCR images anonymously`
  - `cd435b7 Deploy backend impact codebase-browser demo`
  - `0ebec8b Deploy publish-vault memory optimized image`

---

**Turn Context (split turn):**

## Original Request
User asked to remove all `sql.js` migration/fallback paths, first producing a detailed intern-oriented analysis/design/implementation guide stored in the ticket and uploaded to reMarkable, then implementing task-by-task with a detailed diary and commits at appropriate intervals.

## Early Progress
- Read project documentation guidance:
  - Writing style guide from `/home/manuel/.pi/agent/skills/ticket-research-docmgr-remarkable/references/writing-style.md`
  - Deliverable checklist from `/home/manuel/.pi/agent/skills/ticket-research-docmgr-remarkable/references/deliverable-checklist.md`
- Confirmed existing ticket `GCB-017` and current docs/tasks using:
  - `docmgr doc list --ticket GCB-017`
  - `docmgr task list --ticket GCB-017`
- Found existing docs:
  - `analysis/01-transcript-archaeology-findings.md`
  - `reference/01-diary.md`
- Existing completed tasks showed prior work had restored a Go live server while keeping React fallback to static `sql.js`.

## Context for Suffix
- Initial code inspection focused on frontend API provider paths:
  - `ui/src/api/codebaseProvider.ts` currently chooses live API or `sql.js` fallback via `liveOrSql`, `liveWithSqlFallback`, `liveProvider`, and `sqlProvider`.
  - `ui/src/api/sqlJsQueryProvider.ts` contains large static SQLite query provider backed by `sql.js`.
  - `ui/src/api/liveApiProvider.ts` probes `/api/health` and fetches live Go API JSON/text endpoints.
  - `ui/src/api/sqljs/sqlJsDb.ts` initializes `sql.js`, loads `manifest.json`, fetches static `db/codebase.db`, and exposes static DB helpers.
  - `ui/src/api/xrefApi.ts` still directly calls `getSqlJsProvider().getXref(id)`.
- Attempted to read `ui/src/api/db.ts`, but it does not exist.
- Important implementation direction inferred from prefix:
  - Remove `sql.js` runtime/static DB fallback from frontend.
  - Convert API consumers to use live Go provider only.
  - Delete or stop referencing `sqlJsQueryProvider` and `sqljs` helpers.
  - Update documentation and diary before/during implementation.

<read-files>
.gitignore
/home/manuel/.pi/agent/skills/ticket-research-docmgr-remarkable/references/deliverable-checklist.md
/home/manuel/.pi/agent/skills/ticket-research-docmgr-remarkable/references/writing-style.md
/home/manuel/code/wesen/2026-03-27--hetzner-k3s/gitops/applications/codebase-browser.yaml
/home/manuel/code/wesen/2026-03-27--hetzner-k3s/gitops/kustomize/codebase-browser/ingress.yaml
/home/manuel/code/wesen/2026-03-27--hetzner-k3s/gitops/kustomize/codebase-browser/kustomization.yaml
cmd/codebase-browser/cmds/serve/run.go
examples/03-commit-walk-walkthrough.md
examples/04-file-and-annotation-examples.md
internal/server/api.go
internal/server/api_review.go
ui/package.json
ui/src/api/codebaseProvider.ts
ui/src/api/db.ts
ui/src/api/sqlJsQueryProvider.ts
ui/src/api/sqljs/sqlJsDb.ts
ui/src/api/xrefApi.ts
ui/src/features/doc/widgets/SymbolHistoryInlineWidget.tsx
ui/src/features/history/HistoryPage.tsx
</read-files>

<modified-files>
.dockerignore
/home/manuel/code/wesen/2026-03-27--hetzner-k3s/gitops/kustomize/codebase-browser/deployment.yaml
Dockerfile
Makefile
examples/02-symbol-history-and-impact.md
internal/docs/renderer.go
internal/server/api_history.go
internal/server/server.go
ui/src/api/historyApi.ts
ui/src/api/liveApiProvider.ts
ui/src/api/sourceApi.ts
ui/src/features/doc/DocSnippet.tsx
ui/src/features/doc/widgets/AnnotationWidget.tsx
ui/src/features/doc/widgets/CommitWalkWidget.tsx
</modified-files>
</summary>"

**Assistant interpretation:** Build the missing backend API surface needed to remove sql.js, after first producing and uploading the design guide.

**Inferred user intent:** Make the live demo backend-only from the browser's perspective, while preserving a reviewable trail of design, implementation, validation, and commits.

**Commit (code):** c0e7361f9a9957759d1106a473282d7d0a038eaf — "server: add live xref reference endpoints"

### What I did
- Added `internal/server/api_xref.go` with `/api/xref`, `/api/snippet-refs`, `/api/source-refs`, and `/api/file-xref` handlers.
- Registered those handlers in `internal/server/server.go`.
- Expanded `internal/server/server_test.go` with a `snapshot_refs` fixture and endpoint assertions.
- Ran `gofmt -w internal/server/api_xref.go internal/server/server.go internal/server/server_test.go`.
- Ran `go test ./internal/server -count=1` successfully.
- Checked docmgr task 8 and updated changelog/relations.

### Why
- `xrefApi`, `getSnippetRefs`, `getSourceRefs`, and `getFileXref` were still sql.js-only, so deleting the frontend SQLite runtime would have broken symbol/source pages.
- Matching existing frontend response shapes allows the UI migration to be mostly transport-only.

### What worked
- The backend fixture test passed with grouped `uses`, `usedBy`, snippet-relative offsets, file-relative offsets, and file-level inbound/outbound xref responses.
- The existing `resolveCommit` helper was reusable for the new endpoints.

### What didn't work
- N/A. The focused backend test passed on the first full run after adding the route and fixture coverage.

### What I learned
- The old sql.js provider already had clean query boundaries that could be ported almost one-for-one into Go.
- The file-xref endpoints need joins against `snapshot_symbols` to distinguish references crossing the file boundary from references internal to the same file.

### What was tricky to build
- The subtle part was preserving the old grouping semantics: `uses` are grouped by `(toSymbolId, kind)` while `usedBy` remains a flat occurrence list.
- Snippet reference offsets also require subtracting the symbol body start offset, whereas source reference offsets remain file-relative.

### What warrants a second pair of eyes
- Review the file boundary SQL in `queryRefRecordsToFileSymbols` and `queryRefRecordsFromFileSymbols`; correctness depends on `snapshot_refs.file_id`, `from_symbol_id`, and `to_symbol_id` being consistently populated.
- Review whether endpoint names should remain top-level (`/api/snippet-refs`) or be nested under `/api/source/*` before the frontend migration hardens them.

### What should be done in the future
- Wire these endpoints into `LiveApiProvider` and remove the sql.js calls from `sourceApi` and `xrefApi`.

### Code review instructions
- Start in `internal/server/api_xref.go`, especially `handleXref`, `handleSnippetRefs`, `handleSourceRefs`, `handleFileXref`, and the six query helpers.
- Then inspect `internal/server/server_test.go` to see expected response shapes.
- Validate with `go test ./internal/server -count=1`.

### Technical details
- Endpoint response shapes intentionally mirror `ui/src/api/sourceApi.ts` and `ui/src/api/xrefApi.ts`.
- `queryRefRecordsInFileRange` uses byte offsets to select references contained inside a symbol body.
- `groupRefUses` uses a `(toSymbolId, kind)` key and preserves occurrence order.

## Step 14: Remove frontend sql.js provider selection and runtime files

This step moved the React app from a dual live-or-sql.js data layer to a backend-only data layer. The browser API slices now call `LiveApiProvider` methods directly, and the sql.js provider implementation, tests, runtime helpers, and package dependencies were removed.

The key behavior change is intentional: if the Go API is unavailable, interactive data queries now fail visibly instead of silently downloading and opening `db/codebase.db` in the browser.

### Prompt Context

**User prompt (verbatim):** (same as Step 13)

**Assistant interpretation:** Remove the frontend sql.js runtime after backend API coverage exists, and keep the work committed with validation.

**Inferred user intent:** Prevent all browser-side SQLite database downloads and make the live Go server the single query runtime.

**Commit (code):** 862cee25abca7f645b36fb8c663019c2c2332e9b — "ui: remove frontend sqljs data runtime"

### What I did
- Simplified `ui/src/api/codebaseProvider.ts` to expose `apiProvider()`/`liveProvider()` only; removed `liveOrSql`, `liveWithSqlFallback`, and `sqlProvider`.
- Updated `indexApi`, `docApi`, `sourceApi`, `historyApi`, and `xrefApi` to call the live provider directly.
- Added live provider methods for `/api/xref`, `/api/snippet-refs`, `/api/source-refs`, and `/api/file-xref`.
- Removed `ui/src/api/sqlJsQueryProvider.ts`, `ui/src/api/sqlJsQueryProvider.test.ts`, and `ui/src/api/sqljs/*`.
- Removed `sql.js` and `@types/sql.js` from `ui/package.json` and refreshed `ui/pnpm-lock.yaml` with `pnpm -C ui install --lockfile-only`.
- Ran `pnpm -C ui run typecheck` successfully.
- Ran `go test ./internal/server ./internal/docs ./internal/staticapp -count=1` successfully.

### Why
- The remaining fallback helpers could still initialize sql.js and fetch the static database when the live API probe failed.
- Deleting the implementation and package dependency makes accidental fallback impossible in production frontend code.

### What worked
- `rg -n "liveOrSql|liveWithSqlFallback|sqlProvider|getSqlJsProvider|SqlJsQueryProvider|sqlJs|sqljs|sql\.js|sql-wasm|db/codebase" ui/src ui/package.json ui/pnpm-lock.yaml` returned no matches after the deletion.
- TypeScript accepted the backend-only provider surface.
- Focused Go tests continued to pass.

### What didn't work
- N/A. The dependency removal produced only existing peer-dependency warnings from Storybook packages:
  - `@storybook/react 8.6.18` expects `@storybook/test@8.6.18` but `8.6.15` is installed.
  - `@storybook/react-vite 8.6.18` expects `@storybook/test@8.6.18` but `8.6.15` is installed.

### What I learned
- `xrefApi` was the last direct `getSqlJsProvider()` caller.
- `sourceApi` was relying on sql.js-only methods for snippet refs, source refs, and file xrefs, which justified adding the backend endpoints first.

### What was tricky to build
- The frontend had a benign-looking provider abstraction, but it encoded an important runtime behavior: live API failure caused a silent fallback to browser SQLite.
- Removing the fallback required changing all RTK slices consistently; leaving a single import would have kept the sql.js provider reachable through the bundle.

### What warrants a second pair of eyes
- Review UI behavior when `/api/health` or a data endpoint is unavailable. The new behavior is fail-fast/backend-only, which is intended but changes the old static fallback semantics.
- Review whether `liveProvider()` should be kept as a compatibility alias or removed in a later cleanup once all call sites standardize on `apiProvider()`.

### What should be done in the future
- Add an explicit browser/network smoke test to fail on `db/codebase.db`, `sql-wasm.wasm`, or sql.js requests.
- Rebuild the demo bundle and redeploy yolo with the no-sql.js frontend.

### Code review instructions
- Start with `ui/src/api/codebaseProvider.ts` to confirm no fallback selector remains.
- Then inspect each API slice under `ui/src/api/` to confirm calls go through `apiProvider()`.
- Check `ui/src/api/liveApiProvider.ts` for the new xref/ref methods.
- Validate with:
  - `rg -n "liveOrSql|liveWithSqlFallback|sqlProvider|getSqlJsProvider|SqlJsQueryProvider|sqlJs|sqljs|sql\.js|sql-wasm|db/codebase" ui/src ui/package.json ui/pnpm-lock.yaml`
  - `pnpm -C ui run typecheck`
  - `go test ./internal/server ./internal/docs ./internal/staticapp -count=1`

### Technical details
- `getImpact` and commit-scoped snippet/source hydration are now backend-only too; the previously-added live impact endpoint is part of this removal commit because the UI fallback path disappeared.
- Removing `sql.js` from `ui/package.json` also removed the lockfile package entries for `sql.js@1.14.1` and `@types/sql.js@1.4.11`.
