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

- [ ] T4. Add `?debugSql` timing around sql.js `queryAll` / `queryOne`.
- [ ] T5. Add slow-query warning threshold for queries over 1000 ms.
- [ ] T6. Validate instrumentation in the browser on the full Glazed export.

## Hot query rewrites

- [ ] T7. Rewrite `getRefRecordsInFile` to query normalized base tables instead of `snapshot_refs`.
- [ ] T8. Rewrite `getRefRecordsInFileRange` to query normalized base tables instead of `snapshot_refs`.
- [ ] T9. Rewrite `getRefRecordsToFileSymbols` for file used-by xrefs.
- [ ] T10. Rewrite `getRefRecordsFromFileSymbols` for file uses xrefs.
- [ ] T11. Review symbol-level `getRefRecordsFrom` / `getRefRecordsTo` and decide whether they need the same treatment in this ticket.

## Tests and validation

- [ ] T12. Add or update provider tests for source refs, snippet refs, and file xrefs.
- [ ] T13. Run frontend tests and Go tests as applicable.
- [ ] T14. Rebuild static assets and export the full Glazed site.
- [ ] T15. Validate `/source/pkg/help/publish/sqlite_validator.go` with `?debugSql` and record timings.

## Delivery

- [ ] T16. Update diary with every implementation step and measured result.
- [ ] T17. Update changelog and relate key files through docmgr.
- [ ] T18. Commit at appropriate intervals.
