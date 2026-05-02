# Changelog

## 2026-05-02

- Initial workspace created


## 2026-05-02

Created GCB-017 ticket, diary, and design document. Analyzed 264MB production database. Found 99%+ redundancy in snapshot tables. Designed normalized schema projected to reduce DB from 264MB to ~5MB. Saved 14 SQL analysis scripts and 2 shell scripts.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ttmp/2026/05/02/GCB-017--performance-analysis-and-optimization-of-codebase-browser-review-indexing/design/01-performance-analysis-and-design-guide-for-review-indexing.md — Full design document (1258 lines)
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ttmp/2026/05/02/GCB-017--performance-analysis-and-optimization-of-codebase-browser-review-indexing/reference/01-investigation-diary.md — Investigation diary with Step 1 and Step 2


## 2026-05-02

Uploaded bundled design doc + diary to reMarkable at /ai/2026/05/02/GCB-017


## 2026-05-02

Root-caused worktree extraction bug: packages.Config needs GOWORK=off when parent go.work exists. One-line fix in internal/indexer/extractor.go.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/indexer/extractor.go — packages.Config needs GOWORK=off env var for worktree extraction


## 2026-05-02

Phase A (incremental indexing) complete: --incremental flag added, tested with 5+5+0 pattern, 12ms skip for all-cached run


## 2026-05-02

Phase B complete: Normalized schema implemented. 50 commits: 32.3MB -> 1.4MB (23x smaller). All tests pass. Views recreate old table shapes for browser compatibility.


## 2026-05-02

All phases complete. Design doc updated with actual results, re-uploaded to reMarkable.


## 2026-05-02

Closed after completing prioritized review indexing performance work: normalized schema, incremental and parallel indexing, docs-only hardening, snapshot-backed rendering, static export cleanup, strict docs mode, schema metadata, and loader cleanup.


## 2026-05-02

After closing the ticket, added embedded SPA assets for portable review export binaries, updated Makefile build to run frontend build plus go generate and compile with -tags embed, and verified export from /tmp outside the repo using embedded assets.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/Makefile — Build target now creates an embedded standalone binary
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/staticapp/assets_embed.go — Embeds built Vite assets when compiled with -tags embed
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/staticapp/export.go — Uses embedded SPA assets for portable review export


## 2026-05-02

Removed short symbol refs, added strict commit-ref validation for browser-resolved review widgets, improved widget error rendering, and regenerated the example static site with strict docs and zero rendered-doc errors.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/examples/02-symbol-history-and-impact.md — Uses commit refs present in the small example export
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/review/strict_docs.go — Validates from/to/commit refs during strict review indexing/export
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ui/src/features/doc/widgets/WidgetError.tsx — Shows actionable widget errors with diagnostic details


## 2026-05-02

Fixed final review widget hydration issues: commit-walk overview steps now render as prose and codebase-file stubs keep their static rendered fallback instead of hydrating into an empty-symbol loading state.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ui/src/features/doc/widgets/CommitWalkWidget.tsx — Renders overview/note commit-walk steps
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ui/src/features/review/ReviewDocPage.tsx — Skips hydration for codebase-file stubs


## 2026-05-02

Fixed commit-walk symbol steps and duplicate overview/note body rendering, then regenerated and revalidated the example static site.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ui/src/features/doc/widgets/CommitWalkWidget.tsx — Supports symbol steps and avoids duplicated prose for overview/note steps


## 2026-05-02

Fixed codebase-file formatting by hydrating file widgets from stored snippet text instead of relying on markdown fallback HTML.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ui/src/features/doc/DocPage.tsx — Passes stored snippet text to hydrated widgets
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ui/src/features/doc/DocSnippet.tsx — Renders codebase-file with Code from snippet text
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ui/src/features/review/ReviewDocPage.tsx — Passes stored snippet text to hydrated widgets

