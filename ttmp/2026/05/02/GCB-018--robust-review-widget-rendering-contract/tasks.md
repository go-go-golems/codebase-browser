# Tasks

## TODO

- [x] Create GCB-018 ticket and design document scaffold.
- [x] Document current Go/SQLite/React review-widget architecture.
- [x] Analyze GCB-017/GCB-018 widget failures and root causes.
- [x] Propose clean-cut structured review page model.
- [x] Propose central directive registry and strict validation design.
- [x] Propose frontend block renderer and widget registry design.
- [x] Upload design guide to reMarkable.

## Implementation phases

- [x] Phase 1: Add all-widget fixture review doc and Playwright smoke test for visible widget failures.
- [x] Phase 2: Add `internal/reviewwidgets` registry/schema/parser/validator package.
- [x] Phase 3: Implement `BuildPage` structured review page model with markdown and widget blocks.
- [x] Phase 4: Add `static_review_pages` table and export structured page data.
- [x] Phase 5: Update sql.js provider and React review page to render structured blocks directly.
- [x] Phase 6: Delete remaining legacy doc-page stub HTML path; review/static pages no longer emit or hydrate HTML stubs.
- [x] Phase 7: Make strict docs validation cover every directive and commit-walk child step.
- [x] Phase 8: Wire CI/docs-smoke to run strict export plus review-widget validation.
