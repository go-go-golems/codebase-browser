# Changelog

## 2026-07-02

- Initial workspace created


## 2026-07-02

Created transcript archaeology workspace, converted six Pi transcripts with go-minitrace, added reusable SQL/JS scripts, and wrote initial demo-recovery findings.

### Related Files

- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/analysis/01-transcript-archaeology-findings.md — Initial findings
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/reference/01-diary.md — Chronological work record
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/scripts/01-stage-and-convert.sh — Reproducible conversion entrypoint
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/scripts/archaeology/extract.js — Reusable JS query commands


## 2026-07-02

Restored live Go server command backed by the current review SQLite database, with /api health/index/search/source/snippet/review-doc endpoints and regression tests.

### Related Files

- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/cmd/codebase-browser/cmds/serve/run.go — New serve command
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/internal/server/api.go — Live API handlers
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/internal/server/server_test.go — Regression tests


## 2026-07-04

Indexed a stable 43-commit demo DB, hardcoded example review doc commit refs, fixed package-local symbol refs, and added live Go history diff support for review widgets.

### Related Files

- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/examples/01-pr-review-static-export.md — Stable commit refs
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/internal/docs/renderer.go — Short symbol refs
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/internal/server/api_history.go — Live diff API


## 2026-07-04

Reindexed and served a solid 118-commit demo artifact at /tmp/gcb-solid-demo on port 3003, with clean rendered review docs and working live history diff.

### Related Files

- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/reference/01-diary.md — Step 7 demo indexing diary
- /tmp/gcb-solid-demo/db/codebase.db — Generated solid demo SQLite artifact


## 2026-07-04

Fixed broken hydrated demo widgets: live symbol body diffs, commit-walk overview/note/symbol steps, inherited walk refs, and codebase-file snippets; rebuilt and restarted /tmp/gcb-solid-demo.

### Related Files

- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/internal/docs/renderer.go — Commit-walk inheritance and file directive metadata
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/internal/server/api_history.go — Live symbol body diff endpoint
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ui/src/features/doc/DocSnippet.tsx — File snippet hydration
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ui/src/features/doc/widgets/CommitWalkWidget.tsx — Commit-walk step rendering fixes


## 2026-07-04

Made symbol-history demos use live history data and a more interesting history.newScanCmd example; reindexed 118-commit demo and made commit-walk notes visible.

### Related Files

- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/examples/02-symbol-history-and-impact.md — Updated to a richer history.newScanCmd demo
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/internal/server/api_history.go — Live commit and symbol-history endpoints
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ui/src/api/historyApi.ts — History APIs prefer live Go when available
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ui/src/features/doc/widgets/CommitWalkWidget.tsx — Visible guide-note callouts


## 2026-07-04

Added Makefile targets to reproduce, serve, and smoke-test the stable 118-commit demo; aligned docs-smoke with the stable historical range.

### Related Files

- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/Makefile — demo-solid/demo-serve/demo-smoke and stable docs-smoke targets
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/reference/01-diary.md — Step 10 reproducible demo workflow notes


## 2026-07-04

Deployed the repaired live demo to yolo via GitOps: built/pushed ghcr.io/go-go-golems/codebase-browser:yolo-20260704-solid-demo and rolled out codebase-browser.yolo.scapegoat.dev.

### Related Files

- /home/manuel/code/wesen/2026-03-27--hetzner-k3s/gitops/kustomize/codebase-browser/deployment.yaml — Yolo GitOps deployment updated in commit bc92125
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/.dockerignore — Includes bin/static in deploy image context
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/Dockerfile — Updated image runtime to serve embedded static export with live Go API
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/reference/01-diary.md — Step 11 yolo deployment diary


## 2026-07-05

Migrated impact and review-page snippet hydration to live backend APIs, rebuilt/pushed yolo-20260705-backend-impact, and verified public review pages no longer request db/codebase.db/sql-wasm.

### Related Files

- /home/manuel/code/wesen/2026-03-27--hetzner-k3s/gitops/kustomize/codebase-browser/deployment.yaml — Yolo image updated to backend-impact build
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/internal/server/api_history.go — Live impact endpoint and SQL-backed BFS
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ui/src/api/historyApi.ts — Impact widget now prefers live backend
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ui/src/api/liveApiProvider.ts — Live impact and commit-aware snippet/source requests
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ui/src/features/doc/widgets/AnnotationWidget.tsx — Annotation snippets use provider-backed API


## 2026-07-05

Wrote and uploaded the backend-only frontend data runtime design guide for removing sql.js from the React app.

### Related Files

- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/design-doc/01-remove-frontend-sql-js-runtime-design.md — Implementation guide uploaded to reMarkable
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ttmp/2026/07/02/GCB-017--project-archaeology-recover-working-demo-from-agent-transcripts/tasks.md — Added and checked task 7


## 2026-07-05

Added live backend xref, snippet-ref, source-ref, and file-xref endpoints with tests.

### Related Files

- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/internal/server/api_xref.go — New backend handlers and SQLite queries
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/internal/server/server_test.go — Endpoint tests


## 2026-07-05

Migrated React API slices to backend-only data access and removed sql.js provider files/dependencies.

### Related Files

- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ui/package.json — Removed sql.js dependencies
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ui/src/api/codebaseProvider.ts — Removed live/sql fallback selection
- /home/manuel/code/wesen/2026-04-19--go-codebase-browser/ui/src/api/liveApiProvider.ts — Added xref/ref live methods

