# Changelog

## 2026-05-03

- Initial workspace created


## 2026-05-03

Created design guide for making go generate build the frontend through a Dagger-first pnpm pipeline instead of requiring prebuilt ui/dist/public.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/cmd/build-ts-index/main.go — Existing Dagger pnpm CacheVolume pattern to mirror
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/staticapp/generate_build.go — Current prebuilt-assets-only generator


## 2026-05-03

Implemented cmd/build-web Dagger-first pnpm frontend builder, wired internal/staticapp go generate through it, simplified Makefile/CI, and validated clean generation plus tests/smoke.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/.github/workflows/push.yml — go generate owns frontend build in CI
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/Makefile — build now relies on generate for frontend assets
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/cmd/build-web/main.go — Dagger-first frontend build command
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/staticapp/generate_build.go — go generate wrapper now invokes cmd/build-web


## 2026-05-03

Validated the default Dagger path from a clean ui/dist/internal/staticapp/embed/public state; go generate ./internal/staticapp now builds the UI through Dagger and writes embed assets.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/cmd/build-web/main.go — Default Dagger build path validated


## 2026-05-03

Implemented and validated Dagger-backed frontend generation: go generate ./internal/staticapp now runs cmd/build-web, exports Vite assets into internal/staticapp/embed/public, Makefile/CI rely on go generate, and both local fallback plus default Dagger paths were validated.


## 2026-05-03

Addressed CI/PR feedback: checked build-web file close errors, preserved incremental sequence monotonicity, included ref locations in version identity, and matched review widgets to snippets by stable IDs.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/cmd/build-web/main.go — copyFile now checks Close errors
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/history/loader.go — ref version identity now includes locations_json
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/review/indexer.go — incremental sequence assignment now starts above existing max sequence
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ui/src/features/review/ReviewDocPage.tsx — review widgets now match snippets by stable block/snippet ID


## 2026-05-03

Full 240-commit incremental indexing demo found a partial-retry sequence edge case; fixed sequence inference from existing commits in the requested range and validated final static export at /tmp/gcb-full-export.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/review/indexer.go — Incremental sequence assignment now infers original batch base from existing commits in range
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/review/indexer_test.go — Added retry/overlap sequence inference test

