# Changelog

## 2026-05-02

- Initial workspace created


## 2026-05-02

Created intern-oriented analysis/design/implementation guide for replacing the implicit Go/SQLite/React review-widget contract with a structured page model, central directive registry, strict validation, and Playwright smoke tests.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/docs/renderer.go — Current renderer analyzed as the source of implicit stub contract
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ui/src/api/sqlJsQueryProvider.ts — Runtime query provider considered for strict validation parity
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ui/src/features/review/ReviewDocPage.tsx — Current DOM-scanning hydration path targeted for replacement


## 2026-05-02

Implemented Phase 1 smoke coverage: added all-widget fixture, strict export smoke shell script, optional Playwright visible-error scan, and Makefile target.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/Makefile — review-widget-smoke target
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/examples/all-widgets-smoke.md — All-widget strict review fixture
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ttmp/2026/05/02/GCB-018--robust-review-widget-rendering-contract/scripts/01-review-widget-smoke.sh — Strict index/export smoke runner


## 2026-05-02

Implemented Phase 2 registry foundation: added internal/reviewwidgets directive/step schema, central param validation, tests, and renderer integration.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/docs/renderer.go — Now validates directives and commit-walk steps via reviewwidgets
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/reviewwidgets/schema.go — Central directive and commit-walk step contract


## 2026-05-02

Implemented Phase 3 structured page model foundation with markdown/widget blocks, diagnostics, shared directive parsing helpers, and tests.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/docs/renderer.go — Removed duplicate splitFields parser in favor of reviewwidgets helpers
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/reviewwidgets/page_model.go — Structured review page model and BuildPage


## 2026-05-02

Implemented Phase 4 structured export: static_review_pages is created and populated with blocks_json/diagnostics_json, strict export fails on structured diagnostics, and tests cover the new table.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/staticapp/export_test.go — Asserts structured review page export
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/staticapp/reviewdocs.go — Writes static_review_pages alongside temporary legacy rendered docs


## 2026-05-02

Cut static review pages over to structured blocks: frontend reads static_review_pages, ReviewDocPage renders blocks directly, static_review_rendered_docs was removed, and smoke validation confirms no review-page legacy stubs.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/staticapp/reviewdocs.go — Removed static_review_rendered_docs write path
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ui/src/api/sqlJsQueryProvider.ts — Loads review pages from static_review_pages and snippets from review_doc_snippets
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ui/src/features/review/ReviewDocPage.tsx — Direct structured block renderer with no DOM scanning/portals

