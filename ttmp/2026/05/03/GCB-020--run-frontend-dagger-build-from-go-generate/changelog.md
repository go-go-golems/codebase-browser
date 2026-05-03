# Changelog

## 2026-05-03

- Initial workspace created


## 2026-05-03

Created design guide for making go generate build the frontend through a Dagger-first pnpm pipeline instead of requiring prebuilt ui/dist/public.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/cmd/build-ts-index/main.go — Existing Dagger pnpm CacheVolume pattern to mirror
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/staticapp/generate_build.go — Current prebuilt-assets-only generator

