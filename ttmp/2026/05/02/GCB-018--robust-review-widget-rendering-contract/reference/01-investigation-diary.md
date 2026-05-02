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
