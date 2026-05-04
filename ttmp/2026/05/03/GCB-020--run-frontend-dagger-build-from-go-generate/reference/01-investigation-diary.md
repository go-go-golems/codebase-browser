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

## Step 5: Full 240-commit incremental indexing demo found and fixed retry sequence edge case

### What happened

I indexed the codebase-browser repository in three 80-commit batches to exercise incremental behavior:

1. `HEAD~240..HEAD~160`
2. `HEAD~160..HEAD~80 --incremental`
3. `HEAD~80..HEAD --incremental --strict-docs`

During the first attempt, one worktree in step 2 failed because Git still had a stale registered worktree. After `git worktree prune`, retrying the same incremental range correctly skipped 79 existing commits and indexed the one missing commit, but this exposed a sequence edge case: assigning retry batches above `MAX(sequence)` makes a missing older commit look newer than HEAD.

### Fix

- `assignIncrementalSequences` now infers the original batch base sequence from any already-indexed commit in the requested range.
- If there is no overlap, it still appends above `MAX(sequence)` as before.
- The code now assigns sequences to the full requested commit range before filtering already-indexed commits, so retrying a partial failed range preserves the original sequence positions.
- Added `TestInferBatchBaseSequenceFromExistingCommitInRange` to cover this retry case.

### Validation/demo results after fix

Rebuilt `/tmp/codebase-browser-demo` and reran the full demo from scratch:

```bash
/tmp/codebase-browser-demo review index --db /tmp/gcb-full-incremental.db --commits 'HEAD~240..HEAD~160' --docs examples --patterns './...' --parallelism 8
/tmp/codebase-browser-demo review index --db /tmp/gcb-full-incremental.db --commits 'HEAD~160..HEAD~80' --docs examples --patterns './...' --parallelism 8 --incremental
/tmp/codebase-browser-demo review index --db /tmp/gcb-full-incremental.db --commits 'HEAD~80..HEAD' --docs examples --patterns './...' --parallelism 8 --incremental --strict-docs
/tmp/codebase-browser-demo review index --db /tmp/gcb-full-incremental.db --docs examples --docs-only --strict-docs
```

Results:

- Step 1: 80 commits in 11.82s, DB 3.21 MB.
- Step 2: 80 more commits in 19.25s, DB 6.81 MB.
- Step 3: 80 more commits in 56.88s, DB 11.84 MB.
- Skip check over already-indexed latest batch: 0 commits + 5 docs in 1.41s.
- Docs-only check: 0 commits + 5 docs in 1.36s.
- Final DB: 240 commits, sequence range 1..240, 5 review docs, 23 snippets, 11.84 MB.
- Static export: `/tmp/gcb-full-export`, served on `http://127.0.0.1:4182/`.
- Browser validation confirmed the all-widget review doc rendered with no visible widget errors and the history page showed 240 commits.
