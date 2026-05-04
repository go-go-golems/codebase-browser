---
Title: Tasks
Ticket: GCB-022
Status: active
Topics:
  - frontend
  - performance
  - sqlite
  - sqljs
  - static-export
  - wasm
DocType: tasks
Intent: operational
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Phased task list for moving static sql.js execution into a Web Worker."
LastUpdated: 2026-05-03T17:30:00-04:00
WhatFor: "Track Web Worker sql.js migration work."
WhenToUse: "Use during implementation and review of GCB-022."
---

# Tasks

## Phase 0 — Documentation and setup

- [x] W0. Create GCB-022 ticket workspace.
- [x] W1. Write Web Worker execution plan.
- [x] W2. Keep an implementation diary with commands, errors, and measurements.
- [x] W3. Relate implementation files with docmgr.

## Phase 1 — Provider contract and direct fallback

- [x] W4. Add `CodebaseQueryProvider` interface covering current public provider methods.
- [x] W5. Move provider singleton selection into `sqlJsProviderRegistry.ts`.
- [x] W6. Keep direct `SqlJsQueryProvider` behavior available and testable.
- [x] W7. Add `?noSqlWorker` fallback gate.

## Phase 2 — Worker RPC skeleton

- [x] W8. Add Worker request/response protocol types.
- [x] W9. Add `sqlJsQueryWorker.ts` Worker entrypoint.
- [x] W10. Add `workerClient.ts` for request IDs, pending promises, error handling, and timing logs.
- [x] W11. Add `WorkerSqlJsQueryProvider` proxy class.

## Phase 3 — Worker-owned database loading

- [x] W12. Add Worker-safe DB loader with `baseUrl` asset resolution.
- [x] W13. Ensure Worker fetches `manifest.json`, `db/codebase.db`, and `sql-wasm.wasm` itself.
- [x] W14. Ensure main thread does not instantiate `SQL.Database` when Worker provider is enabled.

## Phase 4 — Full provider method coverage

- [x] W15. Forward source methods: `getSource`, `getSourceRefs`, `getFileXref`, snippets.
- [x] W16. Forward index/search methods.
- [x] W17. Forward xref/history/impact methods.
- [x] W18. Forward review-doc methods.
- [x] W19. Preserve `QueryError` code/message/details across Worker serialization.

## Phase 5 — Validation and performance measurement

- [x] W20. Run UI tests and typecheck.
- [x] W21. Rebuild embedded SPA assets.
- [x] W22. Export full Glazed site and validate source route with Worker enabled.
- [x] W23. Validate `?noSqlWorker` fallback still works.
- [x] W24. Measure main-thread long tasks before/after Worker migration.
- [ ] W25. Add a deliberate slow-query smoke test or debug harness to prove UI responsiveness.

## Phase 6 — Follow-up polish

- [ ] W26. Add Worker reset/terminate helper for wedged queries.
- [ ] W27. Consider request cancellation semantics.
- [ ] W28. Deduplicate repeated commit-list queries observed during GCB-021.
- [ ] W29. Consider progressive source-page loading improvements.
