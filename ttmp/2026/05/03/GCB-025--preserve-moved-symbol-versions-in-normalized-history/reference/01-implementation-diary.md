---
Title: ""
Ticket: ""
Status: ""
Topics: []
DocType: ""
Intent: ""
Owners: []
RelatedFiles:
    - Path: internal/history/loader.go
      Note: GCB-025 implementation evidence
    - Path: internal/history/loader_test.go
      Note: GCB-025 regression evidence
    - Path: internal/history/schema.go
      Note: GCB-025 implementation evidence
ExternalSources: []
Summary: ""
LastUpdated: 0001-01-01T00:00:00Z
WhatFor: ""
WhenToUse: ""
---




# Implementation Diary

## 2026-05-03 — Diagnosis

The failing Glazed history URL reports that `sym:github.com/go-go-golems/glazed/pkg/cmds/fields.const.TypeChoice` is not found at commit `140fae2`. Native SQLite inspection showed that the symbol row exists in `snapshot_symbols`, but it points at `file:pkg/cmds/fields/field-type.go`, which is not present in `snapshot_files` for that commit. Git confirms that `field-type.go` did not exist at `140fae2`; the symbol lived in `pkg/cmds/fields/parameter-type.go` at that time.

The root cause is the `symbols` uniqueness key: `UNIQUE(stable_id, body_hash)`. It collapses a moved symbol when the body text does not change. The fix is to include file/range identity in the symbol version key.

## 2026-05-03 — Fix implementation

Changed the `symbols` uniqueness key in `internal/history/schema.go` from:

```sql
UNIQUE(stable_id, body_hash)
```

to:

```sql
UNIQUE(stable_id, body_hash, file_id, start_offset, end_offset)
```

The loader's `ON CONFLICT` target and fallback lookup now use the same identity. This preserves two rows for the same stable symbol and same body hash when the symbol moves to another file or range.

Added `TestLoadSnapshotPreservesMovedSymbolWithSameBody` in `internal/history/loader_test.go`. The test creates two snapshots with `sym:example.com/moved.func.Stable`: first in `old.go`, then in `new.go`, with identical body bytes. The test asserts that the normalized DB stores two symbol versions and that each commit's `snapshot_symbols.file_id` joins to a commit-local `snapshot_files` row.

Validation passed:

```bash
GOWORK=off go test ./internal/history
pnpm --dir ui typecheck
pnpm --dir ui test
GOWORK=off go test ./...
GOWORK=off make build
```

A narrow Glazed indexing experiment was run around commit `6844cbf`, where Git reports `pkg/cmds/fields/parameter-type.go` was renamed to `pkg/cmds/fields/field-type.go`:

```bash
bin/codebase-browser review index \
  --repo-root /home/manuel/code/wesen/corporate-headquarters/glazed \
  --db /tmp/gcb025-glazed-move.db \
  --commits 50db9f3^..6844cbf \
  --docs /tmp/gcb025-glazed-reviews \
  --parallelism 2 \
  --patterns ./... \
  --strict-docs
```

The run completed quickly but did not reproduce the exact full-history `TypeChoice` path because the move commit's extracted snapshot did not contain the expected symbol rows in that narrow range. The important conclusion is that the existing full Glazed DB cannot be reliably fixed with a SQL patch: already-collapsed symbol rows need to be split by replaying extraction with the new schema. For the full exported site, a clean reindex remains the safe correction path.
