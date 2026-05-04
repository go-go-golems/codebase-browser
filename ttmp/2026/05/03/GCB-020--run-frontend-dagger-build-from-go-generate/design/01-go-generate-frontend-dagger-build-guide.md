---
Title: Go Generate Frontend Dagger Build Guide
Ticket: GCB-020
Status: active
Topics:
  - go-generate
  - dagger
  - frontend
  - static-export
DocType: design
Intent: implementation-guide
Summary: "Design and implementation guide for making go generate build the Vite frontend through the Dagger pnpm pipeline before embedding static export assets."
LastUpdated: 2026-05-03T12:20:00-04:00
RelatedFiles:
  - Path: internal/staticapp/generate.go
    Note: go:generate entry point for embedded static SPA assets
  - Path: internal/staticapp/generate_build.go
    Note: current generator that only copies prebuilt ui/dist/public
  - Path: cmd/build-ts-index/main.go
    Note: existing Dagger + pnpm CacheVolume pattern used by TypeScript index generation
  - Path: Makefile
    Note: build/generate workflow
  - Path: .github/workflows/push.yml
    Note: CI workflow that exposed missing ui/dist/public before go generate
---

# Go Generate Frontend Dagger Build Guide

## Problem

`go generate ./...` currently fails in a clean checkout when it reaches `internal/staticapp` because `internal/staticapp/generate_build.go` only copies `ui/dist/public` into `internal/staticapp/embed/public`.

That means the caller must remember to run:

```bash
pnpm -C ui run build
```

before:

```bash
go generate ./...
```

This is fragile. It already broke CI with:

```text
SPA assets missing at .../ui/dist/public; run `pnpm -C ui run build` first
```

The repository already has the right Dagger/pnpm pattern in `cmd/build-ts-index/main.go`: use a Node container, activate a pinned pnpm via Corepack, mount a pnpm store cache, run a frozen-lockfile install, run the build, and export the generated artifact.

## Goal

Make `go generate ./internal/staticapp` build the frontend itself through a Dagger-first pnpm pipeline and then copy/export the resulting Vite build into `internal/staticapp/embed/public`.

After this change, a clean checkout should support:

```bash
go generate ./internal/staticapp
```

without any separate frontend build step.

## Design

Add a dedicated command:

```text
cmd/build-web/main.go
```

Responsibilities:

1. Find the repo root.
2. Locate the UI module at `ui/`.
3. Read or default to `pnpm@10.13.1` from `ui/package.json`.
4. Build with Dagger by default:
   - image: `node:22-bookworm`
   - cache: `codebase-browser-ui-pnpm-store`
   - workdir: `/module`
   - `corepack enable && corepack prepare pnpm@<version> --activate`
   - `pnpm install --frozen-lockfile --prefer-offline`
   - `pnpm run build`
   - export `/module/dist/public` to `internal/staticapp/embed/public`
5. Fall back to local pnpm when Dagger is unavailable or when `BUILD_WEB_LOCAL=1` is set:
   - `corepack enable` if available
   - `pnpm install --frozen-lockfile`
   - `pnpm run build`
   - copy `ui/dist/public` to `internal/staticapp/embed/public`

Update `internal/staticapp/generate_build.go` so it shells out to:

```bash
go run ./cmd/build-web
```

instead of checking for prebuilt assets.

## API / environment

| Variable | Meaning |
|----------|---------|
| `BUILD_WEB_LOCAL=1` | Force local pnpm instead of Dagger |
| `WEB_BUILDER_IMAGE` | Override builder image, default `node:22-bookworm` |
| `WEB_PNPM_VERSION` | Override pinned pnpm version, default from `ui/package.json` or `10.13.1` |
| `WEB_MODULE_ROOT` | Override UI module path, default `ui` |
| `WEB_EMBED_OUT` | Override embed output path, default `internal/staticapp/embed/public` |

## CI implication

Once `go generate` can build the UI itself, the push workflow should not need a separate `pnpm -C ui run build` step just to satisfy `go generate`. CI can run:

```bash
go generate ./...
go test ./...
make review-widget-smoke
```

`make review-widget-smoke` may still run `make build` if no binary exists; `make build` should use the same `go generate` path so there is one source of truth.

## Validation plan

Run:

```bash
rm -rf ui/dist internal/staticapp/embed/public
GOWORK=off go generate ./internal/staticapp
test -f internal/staticapp/embed/public/index.html
GOWORK=off go test ./internal/staticapp ./internal/reviewwidgets ./internal/docs ./internal/review
GOWORK=off make build
GCB_SKIP_BUILD=1 make review-widget-smoke
```

Then run the broader checks:

```bash
GOWORK=off go test ./...
pnpm -C ui run typecheck
```

## Implementation checklist

- [ ] Add `cmd/build-web/main.go` using the same Dagger/pnpm cache pattern as `cmd/build-ts-index`.
- [ ] Replace `internal/staticapp/generate_build.go` with a small wrapper that invokes `cmd/build-web`.
- [ ] Update `Makefile` so `build` does not have to run a separate `frontend-build` before `generate`.
- [ ] Simplify CI now that `go generate` owns the frontend build.
- [ ] Validate clean generation with `ui/dist` removed.
- [ ] Commit in small chunks.
