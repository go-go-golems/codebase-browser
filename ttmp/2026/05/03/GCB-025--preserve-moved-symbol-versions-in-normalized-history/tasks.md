---
title: Tasks - Preserve Moved Symbol Versions in Normalized History
type: tasks
ticket: GCB-025
status: active
created: 2026-05-03
---

# Tasks

## Analysis

- [x] T1. Reproduce and diagnose the Glazed `TypeChoice` history/body-diff failure.
- [x] T2. Determine whether the error is missing data, query failure, or schema/indexing corruption.
- [x] T3. Write detailed implementation plan.

## Fix

- [x] T4. Update symbol version identity in `internal/history/schema.go` to include file/range location.
- [x] T5. Update history loader symbol insert/lookup conflict identity.
- [x] T6. Add regression test for same stable symbol and same body moved to a different file.
- [x] T7. Run UI and Go validation.

## Export/reindex validation

- [x] T8. Build standalone binary with embedded SPA assets.
- [x] T9. Decide whether to run narrow Glazed range validation or full Glazed reindex.
- [ ] T10. Export a validation static site if a new DB is produced.
- [ ] T11. Retest the failing history URL or equivalent narrow-range URL.
