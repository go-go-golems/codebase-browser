---
Title: ""
Ticket: ""
Status: ""
Topics: []
DocType: ""
Intent: ""
Owners: []
RelatedFiles:
    - Path: cmd/codebase-browser/cmds/review/db.go
      Note: GCB-026 review db command group
    - Path: cmd/codebase-browser/cmds/review/db_validate.go
      Note: GCB-026 validation CLI
    - Path: internal/review/validate.go
      Note: GCB-026 integrity validator
    - Path: internal/staticapp/manifest.go
      Note: GCB-026 schema versions in manifest
    - Path: scripts/review-browser-smoke.py
      Note: GCB-026 reusable browser smoke
ExternalSources: []
Summary: ""
LastUpdated: 0001-01-01T00:00:00Z
WhatFor: ""
WhenToUse: ""
---






# Implementation Diary

## 2026-05-03 — Start

GCB-026 turns the GCB-025 moved-symbol incident into permanent guardrails: add a DB validator for commit-local symbol/file consistency, expose schema versions in exports, and promote the source-page browser smoke script into the repository.

## 2026-05-03 — Implementation

Implemented the integrity validator in `internal/review/validate.go`. The validator checks three invariants against the snapshot views:

```sql
snapshot_symbols.file_id -> snapshot_files.id in the same commit
snapshot_refs.file_id -> snapshot_files.id in the same commit
snapshot_refs.from_symbol_id -> snapshot_symbols.id in the same commit
```

Added `review db validate --db <path>` as a command. During validation I discovered that the earlier command shape used separate Cobra commands with `Use: "db create"` and `Use: "db validate"`, which made `review db validate` resolve incorrectly. It started the old create behavior with default flags when run from an old binary. I fixed this by turning `review db` into a real parent command and adding `create` and `validate` as subcommands.

Because the bad old command briefly overwrote `./glazed-full-gcb025.db` with a local codebase-browser index, I restored it from the already-good exported DB at `/tmp/glazed-full-export-gcb025/db/codebase.db` and re-ran validation. The restored DB has 1577 Glazed commits and newest commit `58e0bd0`.

Validation result:

```text
Review DB integrity report for ./glazed-full-gcb025.db
  history_schema_version: 3
  review_schema_version: 2
  bad_symbol_file_joins: 0
  bad_ref_file_joins: 0
  bad_ref_from_symbol_joins: 0
OK
```

Added schema versions to the static manifest. The GCB-026 export manifest now contains:

```json
{
  "historySchemaVersion": "3",
  "reviewSchemaVersion": "2",
  "schemaVersions": {
    "history_schema_version": "3",
    "review_schema_version": "2"
  }
}
```

Promoted the browser smoke script to `scripts/review-browser-smoke.py` and added a Make target:

```bash
make review-browser-smoke URL='http://127.0.0.1:4188/?debugSql&v=gcb026b#/source/pkg/help/publish/sqlite_validator.go'
```

The smoke test passed in 2.274 seconds with source and xref readiness.

Validation commands run:

```bash
GOWORK=off go test ./internal/review ./internal/staticapp
pnpm --dir ui typecheck
pnpm --dir ui test
GOWORK=off go test ./...
GOWORK=off make build
bin/codebase-browser review db validate --db ./glazed-full-gcb025.db
bin/codebase-browser review export --db ./glazed-full-gcb025.db --out /tmp/glazed-full-export-gcb026 --repo-root /home/manuel/code/wesen/corporate-headquarters/glazed --strict-docs
make review-browser-smoke URL='http://127.0.0.1:4188/?debugSql&v=gcb026b#/source/pkg/help/publish/sqlite_validator.go'
```
