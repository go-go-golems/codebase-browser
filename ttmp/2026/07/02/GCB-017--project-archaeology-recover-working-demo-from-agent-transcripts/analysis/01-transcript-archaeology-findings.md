---
Title: Transcript Archaeology Findings
Ticket: GCB-017
Status: active
Topics:
    - codebase-browser
    - go-minitrace
    - project-archeology
    - demo-recovery
    - transcript-analysis
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/codebase-browser/cmds/serve/run.go
      Note: Live Go server CLI command
    - Path: cmd/codebase-browser/main.go
      Note: Register live serve command
    - Path: examples/01-pr-review-static-export.md
      Note: Hardcoded stable staticapp.Export diff range
    - Path: examples/02-symbol-history-and-impact.md
      Note: Hardcoded stable history diff range
    - Path: examples/03-commit-walk-walkthrough.md
      Note: Hardcoded stable commit walk range
    - Path: internal/docs/renderer.go
      Note: Supports package-local short symbol refs in review docs
    - Path: internal/server/api.go
      Note: Go-backed SQLite index/source/symbol APIs
    - Path: internal/server/api_history.go
      Note: Live Go history diff endpoint for diff widgets
    - Path: internal/server/api_review.go
      Note: Go-backed review doc APIs
    - Path: internal/server/server.go
      Note: Live server router and static fallback
    - Path: internal/server/server_test.go
      Note: Live server API regression tests
    - Path: ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/scripts/01-stage-and-convert.sh
      Note: Stages and converts exactly the six requested Pi transcripts
    - Path: ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/scripts/02-session-overview.sql
      Note: Raw DuckDB session overview query
    - Path: ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/scripts/03-tool-frequency.sql
      Note: Raw DuckDB tool-frequency query
    - Path: ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/scripts/05-demo-recovery-files.sql
      Note: Raw DuckDB demo-recovery evidence scan
    - Path: ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/scripts/archaeology/extract.js
      Note: Structured JS query commands for prompts
    - Path: ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/scripts/archaeology/introspect.js
      Note: Structured JS schema introspection helper
    - Path: ui/src/api/historyApi.ts
      Note: Uses live Go diff API when available
ExternalSources: []
Summary: Initial go-minitrace archaeology pass over six project transcripts to recover the path to a working codebase-browser demo.
LastUpdated: 2026-07-02T23:35:00Z
WhatFor: Use this to understand which historical sessions contain the live-server/static-demo evidence and which reproducible scripts generated the findings.
WhenToUse: When resuming codebase-browser demo recovery, deciding whether to restore the old Go live server, or validating the current static sql.js demo.
---




# Transcript Archaeology Findings

## Executive summary

This ticket converts the six named Pi transcripts into a reproducible `go-minitrace` archive and adds SQL/JS query scripts under `scripts/`. The archive confirms two important historical facts:

1. **There used to be a Go live-server demo path.** The 2026-04-20 deployment session read and exercised `internal/server/*` and `cmd/codebase-browser/cmds/serve/run.go`, including `./bin/codebase-browser serve --addr :3001` and `/api/index` checks.
2. **The current working demo path is static sql.js export.** Later sessions pivoted toward SQLite/sql.js and static export, matching the current README and the demo we ran locally at `/tmp/gcb-run-static` on port `8784`.

The immediate recovery recommendation is: **use the static export demo as the working baseline first**, then decide separately whether the old Go live server should be resurrected as a compatibility/dev convenience.

## Source transcripts

All six source files were found under:

`/home/manuel/.pi/agent/sessions/--home-manuel-code-wesen-2026-04-19--go-codebase-browser--/`

| Started | Session id | Title preview | Turns | Tool calls |
|---|---:|---|---:|---:|
| 2026-04-20 16:08 | `8664fb89-aa66-4563-826b-0dbe8c78019e` | Deploy this as an example page | 270 | 390 |
| 2026-04-23 23:42 | `019dbcb9-a9c4-7449-9d4f-dc260c37eaba` | Why does this software need a backend? single page rendered static | 1420 | 1441 |
| 2026-04-25 11:48 | `019dc477-dd22-76d9-8e6a-c8ab7bef4b3b` | Embeddable semantic diff widgets | 618 | 579 |
| 2026-04-30 15:11 | `019ddef2-5b5d-72fb-b679-1d10972a0515` | GCB-015 SQL.js Static Frontend | 1057 | 1030 |
| 2026-05-02 14:40 | `019de921-f893-73dc-8bfa-9087aa65cbfe` | Public website documentation | 124 | 162 |
| 2026-07-02 23:18 | `019f2520-ac47-7a6e-8372-23242db70cad` | Current “how does this work?” session | 44 | 50 |

Generated artifacts live in `archive/`:

- `archive/minitrace/active/*/*.minitrace.json` — converted minitrace sessions.
- `archive/session-overview.json` — SQL session summary output.
- `archive/tool-frequency.json` — SQL tool-frequency output.
- `archive/js-prompts.json` — JS command output for prompt previews.
- `archive/js-shell-commands.json` — JS command output for shell commands.
- `archive/js-demo-signals-1000.json` — JS command output for build/export/server/demo signals.

## Reproducible scripts

| Script | Purpose |
|---|---|
| `scripts/01-stage-and-convert.sh` | Stages exactly the six requested Pi transcripts and converts them with `go-minitrace convert pi`. |
| `scripts/02-session-overview.sql` | Raw DuckDB overview of session timing, model, turns, and tool counts. |
| `scripts/03-tool-frequency.sql` | Raw DuckDB tool-call frequency by session/tool. |
| `scripts/05-demo-recovery-files.sql` | Raw DuckDB scan for server/static/export/deploy/frontend evidence in tool-call JSON. |
| `scripts/archaeology/introspect.js` | JS command repository helper to inspect normalized go-minitrace tables. |
| `scripts/archaeology/extract.js` | JS command repository with `prompts`, `shell-commands`, and `demo-signals` verbs. |

Example rerun:

```bash
BASE=/home/manuel/code/wesen/2026-04-19--go-codebase-browser/ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts
$BASE/scripts/01-stage-and-convert.sh
GLOB="$BASE/archive/minitrace/active/*/*.minitrace.json"
go-minitrace query duckdb --archive-glob "$GLOB" --sql-file "$BASE/scripts/02-session-overview.sql" --output json

go-minitrace query commands \
  --query-repository "$BASE/scripts" \
  archaeology extract demo-signals \
  --archive-glob "$GLOB" \
  --limit 1000 \
  --output json
```

## Evidence relevant to the demo

### Old live server existed

The 2026-04-20 transcript contains direct evidence for a live Go server implementation:

- Read evidence: `internal/server/spa.go`, `internal/server/server.go`, `internal/server/api_doc.go`.
- Read evidence: `cmd/codebase-browser/cmds/serve/run.go`.
- Shell evidence: `./bin/codebase-browser serve --addr :3001`.
- Endpoint evidence: repeated `curl http://127.0.0.1:3001/api/index` checks.

A recovered snippet in `archive/demo-recovery-files.json` also points to the old GCB-001 diary saying Phase 2 added `internal/server` with `/api/index`, `/api/packages`, `/api/symbol/{id}`, `/api/source`, `/api/snippet`, `/api/search`, and `codebase-browser serve`.

### Static/sql.js direction superseded it

The 2026-04-23 and 2026-04-30 transcripts are the high-signal sessions for the later architecture pivot:

- 2026-04-23 asks why a backend is needed and researches browser-side SQLite / `sql.js`.
- 2026-04-30 is titled `GCB-015 SQL.js Static Frontend`, matching the current implementation direction.
- The current README states there is no Go runtime server at read time and no `/api/*` requests.

### Current working demo baseline

Before this ticket was created, we successfully indexed and served a current static demo:

```bash
./bin/codebase-browser review index --commits HEAD~5..HEAD --docs ./examples --db /tmp/gcb-run.db
./bin/codebase-browser review export --db /tmp/gcb-run.db --out /tmp/gcb-run-static
python3 -m http.server 8784 --directory /tmp/gcb-run-static
```

The browser loaded at `http://127.0.0.1:8784/#/`, with the only observed console error being a missing `favicon.ico`. The generated DB contained four review docs.

## Recommended next steps

1. Treat `/tmp/gcb-run-static` / `review export` as the known-good demo path.
2. Use the archaeology scripts to extract exact old `serve` implementation decisions before attempting restoration.
3. Decide whether a restored Go `serve` command should be:
   - a thin static-file server for exported bundles, or
   - a true live API server rebuilding the old `/api/*` surface.
4. If restoring live APIs, start from transcript evidence around `internal/server/*` and compare against current `ui/src/api/sqlJsQueryProvider.ts` to avoid rebuilding APIs the UI no longer calls.

## Open questions

- Should the project support both static sql.js and live Go APIs, or only document the old server as historical?
- If a live server returns, should it query the review SQLite DB or the older embedded `index.json` model?
- Do we want to recover the deployment path to `codebase-browser.yolo.scapegoat.dev`, or only a local demo?

## Implementation update: live Go server restored

The project now has a restored `codebase-browser serve` command in current HEAD. This restoration is intentionally **current-architecture first**: it opens the review SQLite database used by static export and exposes Go-backed `/api/*` queries, rather than reinstating the removed embedded `index.json` + `internal/web` runtime exactly as it existed historically.

New entry points:

- `cmd/codebase-browser/cmds/serve/run.go`
- `internal/server/server.go`
- `internal/server/api.go`
- `internal/server/api_review.go`

Smoke-tested command:

```bash
./bin/codebase-browser serve \
  --addr :3002 \
  --db /tmp/gcb-run-static/db/codebase.db \
  --static-dir /tmp/gcb-run-static
```

Verified endpoints:

- `GET /api/health`
- `GET /api/index`
- `GET /api/review-docs`
- `GET /api/search?q=Export&kind=func`
- `GET /api/source?path=cmd/codebase-browser/main.go`
- `GET /api/snippet?symbol=...`

Validation status:

- ✅ `go test ./internal/server ./cmd/codebase-browser/cmds/serve ./cmd/codebase-browser -count=1`
- ✅ `go build -o bin/codebase-browser ./cmd/codebase-browser`
- ⚠️ `go test ./... -count=1` still fails on generated source snapshots under `internal/sourcefs/embed/source/...` with stale `github.com/go-go-golems/codebase-browser/...` imports; this is unrelated to the restored live server and should be handled as generated snapshot hygiene.

## Implementation update: stable history demo

The local live demo on port `3003` now uses `/tmp/gcb-history-demo`, rebuilt from a fixed 43-commit range:

```bash
./bin/codebase-browser review index \
  --commits 0c3aace..79af1b0 \
  --docs ./examples \
  --db /tmp/gcb-history-demo.db

./bin/codebase-browser review export \
  --db /tmp/gcb-history-demo.db \
  --out /tmp/gcb-history-demo

./bin/codebase-browser serve \
  --addr :3003 \
  --db /tmp/gcb-history-demo/db/codebase.db \
  --static-dir /tmp/gcb-history-demo
```

The example review docs now use fixed commit hashes instead of moving `HEAD~N` refs:

- `b91c6a3 → 83dbe40` for the focused `staticapp.Export` diff.
- `b91c6a3 → 79af1b0` for broad diff stats, changed files, and commit-walk scope.

The pre-rendered review docs are clean:

```sql
select slug, errors_json
from static_review_rendered_docs
where errors_json != '[]';
```

returns no rows for `/tmp/gcb-history-demo/db/codebase.db`.

A hard browser reload is required if the old SPA/DB had already been loaded in the same tab.

## Implementation update: solid 118-commit live demo

The running local demo has been rebuilt as `/tmp/gcb-solid-demo` and served on port `3003`:

```bash
./bin/codebase-browser review index \
  --commits 025e4c6..79af1b0 \
  --docs ./examples \
  --db /tmp/gcb-solid-demo.db

./bin/codebase-browser review export \
  --db /tmp/gcb-solid-demo.db \
  --out /tmp/gcb-solid-demo

./bin/codebase-browser serve \
  --addr :3003 \
  --db /tmp/gcb-solid-demo/db/codebase.db \
  --static-dir /tmp/gcb-solid-demo
```

Current artifact summary:

- 118 indexed commits, starting with GCB-009 git-aware history work.
- 3,761 snapshot package rows.
- 53,655 snapshot symbol rows.
- No rendered review-doc errors in `static_review_rendered_docs`.
- Live Go `/api/history/diff` works for the hardcoded review range `b91c6a3 → 79af1b0`.
