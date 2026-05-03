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

