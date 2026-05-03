# Investigation Diary: GCB-018 Robust Review Widget Rendering Contract

## Step 1: Ticket creation and design scope

Created GCB-018 after GCB-017 exposed repeated review-widget failures during static export validation. The goal is a clean-cut redesign plan, not compatibility preservation. The system should prefer a simple, explicit, testable widget contract over the current implicit Go-rendered HTML plus React DOM-scanning handoff.

Initial observed failures from GCB-017:

- Short symbol refs such as `staticapp.Export` looked valid but did not resolve to full stable IDs.
- Runtime widgets accepted invalid commit refs like `HEAD~5` even when only two commits were indexed.
- Commit-walk child steps did not inherit top-level `from=`/`to=` params.
- Commit-walk examples referenced step kinds (`overview`, `symbol`) that the React renderer did not implement.
- `codebase-file` fallback HTML was malformed because raw source text passed through Markdown/HTML parsing.
- Some widgets showed generic `Failed` or indefinite `Loading…` states with little context.

The design document in `design/01-review-widget-rendering-contract-analysis-and-implementation-guide.md` is intended as an intern-ready architecture guide and implementation plan.

## Step 2: Phase 1 all-widget smoke fixture and script

### What changed

- Added `examples/all-widgets-smoke.md`, a strict all-widget review document that exercises every supported review directive with full `sym:` IDs and a small `HEAD~1..HEAD`-compatible range.
- Added `scripts/01-review-widget-smoke.sh` to build/use the standalone binary, strict-index the fixture, strict-export it, validate SQLite rendered-doc errors/snippet count, serve the export, and run the browser check if the `playwright` package is installed.
- Added `scripts/02-review-widget-smoke.mjs`, the Playwright visible-error scan used by the shell script when Playwright is available.
- Added `make review-widget-smoke` as the convenient entry point.
- Marked Phase 1 complete in `tasks.md`.

### Validation

- `GCB_SKIP_BUILD=1 make review-widget-smoke` passed.
- The smoke script created a strict review DB and strict static export for `examples/all-widgets-smoke.md`.
- SQLite checks passed: zero rendered-doc errors and at least 11 snippets.
- Playwright was not available as a local Node package in the repo, so the script correctly skipped the optional browser scan.
- I manually served the kept smoke export and used the harness Playwright browser to validate the page and all commit-walk steps:
  - no `doc error`
  - no `Failed`
  - no `Unknown`
  - no `not found`
  - no `outside this export`
  - no `Loading…`
  - no `&#34;`
  - no paragraph tags inside the `codebase-file` widget
  - `Resolved 11 snippet(s)`

### Notes

This is Phase 1 of the GCB-018 plan: it freezes expected visible behavior before the deeper clean-cut refactor. Deprecated DOM-scanning/stub code is not removed in this phase; the design intentionally removes it in Phase 6 after the structured page model is implemented.

## Step 3: Phase 2 directive registry foundation

### What changed

- Added `internal/reviewwidgets`, a dependency-free package that defines the supported top-level `codebase-*` directives and supported `codebase-commit-walk` step kinds.
- Added metadata for each directive:
  - required params
  - optional params
  - commit-ref params
  - whether the directive requires a symbol or file
  - short description
- Added `ValidateParams` and `ValidateStepParams` so renderers and tests no longer need private copies of the directive contract.
- Wired `internal/docs/renderer.go` through the registry before directive-specific rendering. Unknown directives, missing required params, and unsupported params now fail centrally.
- Wired commit-walk step parsing through the registry. Unknown step kinds and unsupported step params now fail before React sees them.
- Added unit tests in `internal/reviewwidgets/schema_test.go` for directive enumeration, required params, unsupported params, unknown directives, and commit-walk step validation.

### Validation

- `GOWORK=off go test ./internal/reviewwidgets ./internal/docs ./internal/review` passed.
- `GCB_SKIP_BUILD=1 make review-widget-smoke` passed after the renderer started using the registry.

### Notes

This is the first real removal of duplicated/deprecated contract logic: directive/step required-param checks are no longer scattered only inside `internal/docs/renderer.go`. The old HTML stub/DOM hydration path still exists and will be removed after the structured review page model is implemented.
