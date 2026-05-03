# Investigation Diary

## Step 1: CI failure and desired direction

The linked GitHub Actions job failed during `go generate ./...` because the staticapp generator expected prebuilt Vite assets at `ui/dist/public/index.html`.

The important failure line was:

```text
SPA assets missing at .../ui/dist/public; run `pnpm -C ui run build` first
```

I first fixed CI by adding an explicit frontend build before `go generate`, but the requested direction is better: `go generate` itself should run the existing Dagger/pnpm pipeline. The repository already has a Dagger + CacheVolume pnpm pattern in `cmd/build-ts-index/main.go`; `internal/staticapp` simply was not using it for the SPA build.

Created GCB-020 to make `go generate ./internal/staticapp` self-contained: build the UI with Dagger by default, fall back to local pnpm when requested/unavailable, and export/copy the result into `internal/staticapp/embed/public`.

## Step 2: Implement Dagger-first frontend build for go generate

### What changed

- Added `cmd/build-web`, a Dagger-first frontend builder for the `ui/` Vite app.
- The command mirrors the existing `cmd/build-ts-index` pattern:
  - `node:22-bookworm` builder image by default
  - Corepack-pinned pnpm from `ui/package.json`
  - Dagger `CacheVolume` named `codebase-browser-ui-pnpm-store`
  - frozen-lockfile `pnpm install`
  - `pnpm run build`
  - export `/module/dist/public` into `internal/staticapp/embed/public`
- Added local fallback with `BUILD_WEB_LOCAL=1` or when Dagger is unavailable:
  - `corepack enable`
  - `corepack prepare pnpm@... --activate`
  - `pnpm install --frozen-lockfile --prefer-offline`
  - `pnpm run build`
  - copy `ui/dist/public` to `internal/staticapp/embed/public`
- Replaced `internal/staticapp/generate_build.go` with a small wrapper that runs `go run ./cmd/build-web`.
- Changed `Makefile build` to depend on `generate` only. `generate` now owns the frontend build, so a separate `frontend-build` prerequisite is no longer required.
- Simplified `.github/workflows/push.yml`: removed the explicit `pnpm -C ui install` and `pnpm -C ui run build` steps. `go generate ./...` now builds frontend assets itself.

### Validation

Validated from a clean frontend/staticapp output state:

```bash
rm -rf ui/dist internal/staticapp/embed/public
BUILD_WEB_LOCAL=1 GOWORK=off go generate ./internal/staticapp
test -f internal/staticapp/embed/public/index.html
GOWORK=off go test ./cmd/build-web ./internal/staticapp ./internal/review ./internal/reviewwidgets ./internal/docs
GCB_SKIP_BUILD=1 make review-widget-smoke
GOWORK=off go test ./...
```

All commands passed.

### Notes

I validated the local fallback path explicitly because it is deterministic and does not depend on Docker availability. The Dagger path uses the same SDK and CacheVolume pattern as `cmd/build-ts-index`, so CI should now build the SPA during `go generate` instead of requiring prebuilt `ui/dist/public`.

## Step 3: Validate actual Dagger path

After committing the initial implementation I also validated the default Dagger path, not only the local fallback:

```bash
rm -rf internal/staticapp/embed/public ui/dist
GOWORK=off go generate ./internal/staticapp
test -f internal/staticapp/embed/public/index.html
GOWORK=off go test ./cmd/build-web ./internal/staticapp
```

This succeeded and printed:

```text
build-web: wrote .../internal/staticapp/embed/public via Dagger
generate_staticapp: wrote internal/staticapp/embed/public
```

So `go generate ./internal/staticapp` now actually runs the Dagger pnpm frontend build and exports the Vite output into the embed directory from a clean state.

## Step 4: Address CI lint failure and PR review comments

### What changed

- Fixed the GitHub Actions lint failure in `cmd/build-web/main.go` by checking both input and output file close errors in `copyFile` with `errors.Join`.
- Addressed PR comment P1 on incremental review indexing sequences:
  - Added `history.Store.MaxSequence`.
  - Incremental review indexing now assigns new commit sequences above the existing maximum instead of restarting at `len(batch)`.
  - Added a unit test for sequence assignment.
- Addressed PR comment P1 on normalized ref versions:
  - Included `locations_json` in the `ref_versions` uniqueness key.
  - Updated ref insert conflict handling and lookup to include `locations_json`.
  - Sorted ref locations before marshaling so the JSON identity is deterministic.
  - Added a history loader test proving the same `(from,to,kind,file)` ref at a moved location produces a distinct version and the later snapshot exposes the new range.
- Addressed PR comment P2 on review widget/snippet matching:
  - Structured review widget block IDs now use the same `stub-N` namespace as stored snippet rows.
  - `docs.Render` increments the snippet ID counter for every directive, including failed resolutions, so later successful snippets keep their stable block ID.
  - `ReviewDocPage` now maps snippets by `stubId` instead of array index, avoiding shifted widgets in non-strict exports.

### Validation

- `GOWORK=off go test ./cmd/build-web ./internal/history ./internal/review ./internal/reviewwidgets ./internal/staticapp`
- `pnpm -C ui run typecheck`
- `GOWORK=off go test ./...`
- `BUILD_WEB_LOCAL=1 GOWORK=off make build`
- `GCB_SKIP_BUILD=1 make review-widget-smoke`

All commands passed.
