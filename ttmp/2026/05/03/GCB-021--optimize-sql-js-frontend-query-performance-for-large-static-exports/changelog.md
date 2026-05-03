---
Title: Changelog
Ticket: GCB-021
Status: active
Topics:
    - frontend
    - performance
    - sqlite
    - sqljs
    - static-export
DocType: changelog
Intent: operational
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Chronological changelog for GCB-021."
LastUpdated: 2026-05-03T16:45:00-04:00
WhatFor: "Track externally visible ticket changes."
WhenToUse: "Use during review and handoff."
---

# Changelog

## 2026-05-03

- Created GCB-021 for frontend sql.js large-export performance work.
- Added intern-oriented design guide explaining the source/xref query stack, the `snapshot_refs` bottleneck, and normalized-query implementation plan.
- Added investigation diary with profiler evidence and native SQLite timings from the full Glazed export.
- Added implementation task list for instrumentation, query rewrites, validation, and delivery.
- Uploaded the design guide, diary, and tasks bundle to reMarkable at `/ai/2026/05/03/GCB-021/GCB-021 Frontend sql.js Performance Guide` after a dry run.
- Added `frontend` and `sqljs` to the docmgr topic vocabulary so GCB-021 validates cleanly.
- Added `?debugSql`/slow-query instrumentation around the shared sql.js query loop.
- Rewrote frontend ref hot paths to query normalized base tables instead of the expensive `snapshot_refs` view, including source refs, snippet refs, file xrefs, and symbol-level from/to refs.
- Added provider regression tests for normalized source refs, snippet refs, and file xrefs.
- Rebuilt embedded SPA assets, exported the full Glazed site to `/tmp/glazed-full-export-gcb021`, and validated the source page with a Chromium CDP smoke script.
- Committed the frontend sql.js performance implementation.
- Uploaded the updated post-implementation guide bundle to reMarkable as `GCB-021 Frontend sql.js Performance Guide - Updated`.
- Verified the fixed source page with Playwright: source+xref readiness in ~2.25s, source-ref query 82 rows in 19 ms, file xref queries 2 ms and 42 ms; the old 4183 export still times out/hangs on the same route.
