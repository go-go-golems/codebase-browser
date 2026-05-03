---
title: Tasks - Static Browser Performance Hardening After sql.js Worker Migration
type: tasks
ticket: GCB-024
status: active
created: 2026-05-03
---

# Tasks

## Phase 1 — Query caching

- [x] H1. Cache `listCommits()` in `SqlJsQueryProvider`.
- [x] H2. Cache `resolveCommitRef(...)` results, especially `HEAD`.
- [x] H3. Cache `getCommit(...)` results.
- [x] H4. Add provider regression tests proving repeated calls do not rerun commit-list SQL.

## Phase 2 — Worker recovery

- [x] H5. Add explicit Worker reset/terminate helper.
- [x] H6. Add request timeout handling with actionable errors.
- [x] H7. Keep test reset behavior working.

## Phase 3 — Hot-path regression checks

- [x] H8. Add a regression test that frontend hot reference paths do not use `snapshot_refs`.

## Phase 4 — Browser smoke and export

- [x] H9. Add a reusable browser performance smoke script for source pages.
- [x] H10. Run UI typecheck/tests and Go tests.
- [x] H11. Rebuild the standalone binary.
- [x] H12. Export full Glazed static site for manual testing.
- [x] H13. Validate Worker route and `?noSqlWorker` fallback.
