# Changelog

## 2026-05-03

- Initial workspace created


## 2026-05-03

- Created GCB-022 for moving static sql.js query execution into a Web Worker.
- Added phased tasks and the Web Worker sql.js execution plan.
- Added `CodebaseQueryProvider`, Worker RPC protocol, Worker query provider proxy, and Worker-owned static DB loading.
- Moved frontend API imports to `sqlJsProviderRegistry`, with Worker enabled by default and `?noSqlWorker` fallback.
- Validated UI tests, typecheck, `GOWORK=off make build`, `GOWORK=off go test ./...`, full Glazed export, Playwright Worker route, and `?noSqlWorker` fallback.
