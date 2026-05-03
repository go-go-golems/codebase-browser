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
      Note: Loader conflict and lookup identity fix
    - Path: internal/history/loader_test.go
      Note: Moved unchanged symbol regression test
    - Path: internal/history/schema.go
      Note: Symbol version identity schema fix
ExternalSources: []
Summary: ""
LastUpdated: 0001-01-01T00:00:00Z
WhatFor: ""
WhenToUse: ""
---




# Preserve Moved Symbol Versions in Normalized History

## Problem statement

The full Glazed static export exposes a history/body-diff failure for:

```text
http://127.0.0.1:4186/#/history?symbol=sym%3Agithub.com%2Fgo-go-golems%2Fglazed%2Fpkg%2Fcmds%2Ffields.const.TypeChoice
```

The UI reports:

```text
Failed to load body diff: symbol sym:github.com/go-go-golems/glazed/pkg/cmds/fields.const.TypeChoice not found at 140fae2
```

This is not a sql.js Worker issue and not a frontend caching issue. It is a normalized-history correctness bug. The database has a `snapshot_symbols` row for the symbol at commit `140fae2`, but that symbol row points at a file version that is not mapped into that commit.

## Concrete evidence from Glazed

The failing commit exists:

```text
140fae2eb554e08f4a86a276e394c71199956c25
Docs: close GL-001 ticket
```

The symbol exists in `symbol_history` and in `snapshot_symbols` at that commit:

```text
symbol:    sym:github.com/go-go-golems/glazed/pkg/cmds/fields.const.TypeChoice
file_id:   file:pkg/cmds/fields/field-type.go
signature: TypeChoice Type = "choice"
```

But Git shows that file did not exist at the commit:

```bash
git cat-file -e 140fae2:pkg/cmds/fields/field-type.go
# missing
```

The symbol did exist, but in a different file:

```bash
git grep -n "TypeChoice" 140fae2 -- '*.go'
# pkg/cmds/fields/parameter-type.go:45: TypeChoice Type = "choice"
```

The DB also contains the old file in `snapshot_files`:

```text
file:pkg/cmds/fields/parameter-type.go
```

So the indexed data has enough file/source history, but the symbol version for this commit points at the wrong file.

## Root cause

The current normalized schema defines symbol version identity as:

```sql
UNIQUE(stable_id, body_hash)
```

This is too coarse. It collapses two historically distinct symbol versions when a symbol keeps the same body text but moves to a different file, or potentially to a different range in the same file.

The Glazed case is:

```text
same stable_id: sym:...fields.const.TypeChoice
same body_hash: 9955a673...
old file:       pkg/cmds/fields/parameter-type.go
new file:       pkg/cmds/fields/field-type.go
```

Because `stable_id + body_hash` matched, the loader reused one symbol row. That row pointed at the newer file. Older commits then mapped to that same row, so `snapshot_symbols` reported a file that was not present in the older commit.

## Why the frontend error is correct

The frontend body-diff query joins symbols to files within the same commit:

```sql
FROM snapshot_symbols s
JOIN snapshot_files f
  ON f.commit_hash = s.commit_hash
 AND f.id = s.file_id
WHERE s.commit_hash = ? AND s.id = ?
```

That join is correct. A body diff must slice source bytes from the file version that existed in that commit. If the symbol points at a file not mapped into the commit, the safe behavior is to fail rather than show a body from the wrong file.

## Correct design

Symbol version identity must include location identity, not just body identity. The minimal fix is to change the unique key to include:

```text
stable_id
body_hash
file_id
start_offset
end_offset
```

This distinguishes:

- same symbol body moved to another file,
- same symbol body moved within the same file,
- empty body-hash fallback cases where file/range still identify the indexed version.

The schema change is a clean cutover because the project already chose clean schema cutovers over legacy migration wrappers.

## Implementation plan

1. Update `internal/history/schema.go`:
   - change the symbol table comment,
   - change `UNIQUE(stable_id, body_hash)` to `UNIQUE(stable_id, body_hash, file_id, start_offset, end_offset)`,
   - bump `history_schema_version` from `2` to `3`.
2. Update `internal/history/loader.go`:
   - update `ON CONFLICT(...)` target for symbol inserts,
   - update symbol lookup SQL to use the same identity,
   - pass file/range values as lookup arguments.
3. Add a regression test in `internal/history/loader_test.go`:
   - commit 1 has a symbol in `old.go`,
   - commit 2 has the same stable symbol with the same body text in `new.go`,
   - assert that two symbol versions are stored,
   - assert each commit's `snapshot_symbols.file_id` points at the commit-local file.
4. Run the normal test suite.
5. Rebuild the standalone binary.
6. Reindex enough data to validate the fix. Prefer the small regression test for correctness first. A partial repair of the existing Glazed DB is not appropriate because already-collapsed symbol rows need to be split across historical commits; that requires replaying extraction for affected commits.
7. If manual Glazed validation is required, rebuild a new Glazed DB/export from history. The current `/tmp/glazed-full.db` cannot be repaired reliably with a simple SQL update.

## Partial reindex assessment

A true partial reindex would need to identify every commit where a symbol was mapped to a symbol row whose `file_id` is not in `commit_files`, delete those commits' mappings and dependent ref mappings, re-extract those commits with the new schema identity, and ensure all downstream rows are consistent. That is more complex and riskier than a clean rebuild.

For local validation, a small purpose-built test is enough to prove the fix. For the full Glazed export, a clean reindex is the safe path. If runtime is a concern, we can first reindex a narrow Glazed range spanning the file move to validate the specific `TypeChoice` scenario before spending hours on the full history.
