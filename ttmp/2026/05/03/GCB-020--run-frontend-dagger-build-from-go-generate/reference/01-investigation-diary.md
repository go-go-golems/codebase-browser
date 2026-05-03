# Investigation Diary

## Step 1: CI failure and desired direction

The linked GitHub Actions job failed during `go generate ./...` because the staticapp generator expected prebuilt Vite assets at `ui/dist/public/index.html`.

The important failure line was:

```text
SPA assets missing at .../ui/dist/public; run `pnpm -C ui run build` first
```

I first fixed CI by adding an explicit frontend build before `go generate`, but the requested direction is better: `go generate` itself should run the existing Dagger/pnpm pipeline. The repository already has a Dagger + CacheVolume pnpm pattern in `cmd/build-ts-index/main.go`; `internal/staticapp` simply was not using it for the SPA build.

Created GCB-020 to make `go generate ./internal/staticapp` self-contained: build the UI with Dagger by default, fall back to local pnpm when requested/unavailable, and export/copy the result into `internal/staticapp/embed/public`.
