---
Title: Tasks
Ticket: GCB-021
Status: active
Topics:
    - frontend
    - performance
    - sqlite
    - sqljs
    - static-export
DocType: tasks
Intent: operational
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Implementation task list for frontend sql.js large-export performance fixes."
LastUpdated: 2026-05-03T16:45:00-04:00
WhatFor: "Track GCB-021 implementation progress."
WhenToUse: "Use during implementation and review."
---

# Tasks

## Documentation and ticket setup

- [x] T1. Create GCB-021 ticket workspace.
- [x] T2. Write intern-oriented analysis/design/implementation guide.
- [x] T3. Upload the design guide bundle to reMarkable.

## Instrumentation

- [x] T4. Add `?debugSql` timing around sql.js `queryAll` / `queryOne`.
- [x] T5. Add slow-query warning threshold for queries over 1000 ms.
- [x] T6. Validate instrumentation in the browser on the full Glazed export.

## Hot query rewrites

- [x] T7. Rewrite `getRefRecordsInFile` to query normalized base tables instead of `snapshot_refs`.
- [x] T8. Rewrite `getRefRecordsInFileRange` to query normalized base tables instead of `snapshot_refs`.
- [x] T9. Rewrite `getRefRecordsToFileSymbols` for file used-by xrefs.
- [x] T10. Rewrite `getRefRecordsFromFileSymbols` for file uses xrefs.
- [x] T11. Review symbol-level `getRefRecordsFrom` / `getRefRecordsTo` and decide whether they need the same treatment in this ticket.

## Tests and validation

- [x] T12. Add or update provider tests for source refs, snippet refs, and file xrefs.
- [x] T13. Run frontend tests and Go tests as applicable.
- [x] T14. Rebuild static assets and export the full Glazed site.
- [x] T15. Validate `/source/pkg/help/publish/sqlite_validator.go` with `?debugSql` and record timings.

## Delivery

- [x] T16. Update diary with every implementation step and measured result.
- [x] T17. Update changelog and relate key files through docmgr.
- [x] T18. Commit at appropriate intervals.
