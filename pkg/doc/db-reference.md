---
Title: "Review Database Reference"
Slug: "db-reference"
Short: "Schema, tables, and query patterns for the code-review SQLite database."
Topics:
- code-review
- sqlite
- reference
Commands:
- review index
- review export
- review db create
IsTopLevel: false
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Overview

The `codebase-browser review` commands produce a SQLite database containing both indexed commit history and review markdown documents.

Two separate DB paths matter:

| DB | Produced by | Use |
|----|-------------|-----|
| **Source DB** | `review index` or `review db create` | Query with `sqlite3`, hand to an LLM, or use as input to `review export` |
| **Export DB** (`db/codebase.db`) | `review export` (copies and enriches the source DB) | The static browser opens this file with sql.js. Contains `static_review_pages` with structured markdown/widget blocks. |

`review export` copies the source DB to `db/codebase.db` in the output directory, then writes `static_review_pages` rows into the output DB. The source DB is never modified.

## Database structure

The review database is a standard SQLite file with two groups of tables:

1. **History tables** — per-commit snapshots of the codebase (from `internal/history/schema.go`)
2. **Review tables** — markdown documents and their resolved snippet references (from `internal/review/schema.go`)
3. **Static export tables** — export-time browser preparation tables written only to the copied export DB

## History tables

The review database uses a **normalized schema** where each unique entity (symbol, file, package, ref set) is stored once, and narrow mapping tables record which version appears in which commit. The old `snapshot_*` table shapes are available as views for backward-compatible querying.

### Base tables (stored once)

#### `commits`

One row per indexed commit.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment |
| `hash` | TEXT UNIQUE | Full 40-character SHA |
| `short_hash` | TEXT | 7-character abbreviation |
| `message` | TEXT | Commit message |
| `author_name` | TEXT | Author name |
| `author_email` | TEXT | Author email |
| `author_time` | INTEGER | Unix timestamp |
| `parent_hashes` | TEXT | JSON array of parent SHAs |
| `tree_hash` | TEXT | Git tree hash |
| `indexed_at` | INTEGER | When the row was inserted |
| `sequence` | INTEGER | Explicit review-range order; larger means later/latest |
| `branch` | TEXT | Branch name (if supplied) |
| `error` | TEXT | Empty unless indexing failed |

#### `packages`

One row per unique package (keyed by `stable_id`).

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment |
| `stable_id` | TEXT UNIQUE | `pkg:<importPath>` |
| `import_path` | TEXT | Go/TS import path |
| `name` | TEXT | Package name |
| `doc` | TEXT | Package comment |
| `language` | TEXT | `"go"` or `"ts"` |

#### `files`

One row per unique file version (keyed by `stable_id` + `sha256`).

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment |
| `stable_id` | TEXT | `file:<path>` |
| `path` | TEXT | Relative path |
| `package_id` | INTEGER FK | References `packages(id)` |
| `size` | INTEGER | File size in bytes |
| `line_count` | INTEGER | Number of lines |
| `sha256` | TEXT | File content hash |
| `language` | TEXT | `"go"` or `"ts"` |
| `build_tags_json` | TEXT | JSON array of build tags |

#### `symbols`

One row per unique symbol body+location version (keyed by `stable_id` + `body_hash` + `file_id` + `start_offset` + `end_offset`). A "symbol" is any top-level declaration: function, method, type, const, var. File/range identity is part of the key so an unchanged symbol that moves files or offsets remains historically tied to the file version that existed in each commit.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment |
| `stable_id` | TEXT | `sym:<importPath>.<kind>.<name>` |
| `kind` | TEXT | `func`, `method`, `type`, `var`, `const` |
| `name` | TEXT | Symbol name |
| `package_id` | INTEGER FK | References `packages(id)` |
| `file_id` | INTEGER FK | References `files(id)` |
| `start_line` / `end_line` | INTEGER | Line range |
| `start_col` / `end_col` | INTEGER | Column range |
| `start_offset` / `end_offset` | INTEGER | **Byte offsets** (authoritative for slicing) |
| `doc` | TEXT | Godoc / TSDoc |
| `signature` | TEXT | e.g. `func Merge(...) (*Index, error)` |
| `receiver_type` | TEXT | For methods: receiver type name |
| `receiver_pointer` | INTEGER | 1 if receiver is a pointer |
| `exported` | INTEGER | 1 if name starts with uppercase |
| `language` | TEXT | `"go"` or `"ts"` |
| `type_params_json` | TEXT | JSON array of type parameters |
| `tags_json` | TEXT | JSON array of struct tags |
| `body_hash` | TEXT | SHA-256 of function body bytes |

#### `ref_versions`

One row per unique ref set (keyed by `from_symbol_id` + `to_stable_id` + `kind` + `file_id`). Multiple locations for the same ref are collapsed into a JSON array.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment |
| `from_symbol_id` | INTEGER FK | References `symbols(id)` — caller |
| `to_stable_id` | TEXT | Stable ID of the callee |
| `kind` | TEXT | `call`, `uses-type`, `reads`, `use` |
| `file_id` | INTEGER FK | References `files(id)` |
| `locations_json` | TEXT | JSON array of `{start_line, start_col, end_line, end_col, start_offset, end_offset}` objects |

### Mapping tables (commit ↔ entity)

These narrow tables record which entity version appears in which commit. They use `WITHOUT ROWID` for compact storage.

#### `commit_packages`

| Column | Type | Description |
|--------|------|-------------|
| `commit_id` | INTEGER FK | References `commits(id)` |
| `package_id` | INTEGER FK | References `packages(id)` |

#### `commit_files`

| Column | Type | Description |
|--------|------|-------------|
| `commit_id` | INTEGER FK | References `commits(id)` |
| `file_id` | INTEGER FK | References `files(id)` |

#### `commit_symbols`

| Column | Type | Description |
|--------|------|-------------|
| `commit_id` | INTEGER FK | References `commits(id)` |
| `symbol_id` | INTEGER FK | References `symbols(id)` |

#### `commit_refs`

| Column | Type | Description |
|--------|------|-------------|
| `commit_id` | INTEGER FK | References `commits(id)` |
| `ref_version_id` | INTEGER FK | References `ref_versions(id)` |

### Content table

#### `file_contents`

Deduplicated file content blobs (keyed by SHA-256).

| Column | Type | Description |
|--------|------|-------------|
| `content_hash` | TEXT PK | SHA-256 of content |
| `content` | BLOB | Raw file bytes |

### Schema metadata

#### `schema_info`

Small key/value metadata table for identifying the clean-cut schema version in exported or source review databases.

| Column | Type | Description |
|--------|------|-------------|
| `key` | TEXT PK | Metadata key, e.g. `history_schema_version` |
| `value` | TEXT | Metadata value |

### Views (compatibility layer)

The `snapshot_packages`, `snapshot_files`, `snapshot_symbols`, and `snapshot_refs` views recreate the old "one row per (commit, entity)" table shape by joining mapping tables with base tables. The browser's SQL queries use these views.

**`snapshot_packages`** — joins `commit_packages → packages`, projecting `commits.hash` as `commit_hash` and `packages.stable_id` as `id`.

**`snapshot_files`** — joins `commit_files → files`, projecting integer IDs back to stable string IDs. File content is joined via `sha256 = file_contents.content_hash`.

**`snapshot_symbols`** — joins `commit_symbols → symbols`, projecting all symbol columns with `commit_hash` and stable string IDs.

**`snapshot_refs`** — joins `commit_refs → ref_versions → symbols, files` and expands `locations_json` into individual rows using `json_each()`. Each location becomes one row with `start_line`, `start_col`, `end_line`, `end_col`, `start_offset`, `end_offset` columns and a synthetic `id` via `row_number()`.

**`symbol_history`** — convenience view joining `snapshot_symbols` with `commits`.

> [!NOTE]
> All queries in this guide use the `snapshot_*` views. They produce the same column names and types as the original tables, so existing SQL queries continue to work unchanged.

## Review tables

### `review_docs`

One row per markdown review document.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment |
| `slug` | TEXT UNIQUE | Derived from filename (`pr-42.md` → `pr-42`) |
| `title` | TEXT | From H1 or frontmatter |
| `path` | TEXT | Original file path |
| `content` | TEXT | Raw markdown |
| `frontmatter_json` | TEXT | JSON object of YAML frontmatter |
| `indexed_at` | INTEGER | Unix timestamp |

### `review_doc_snippets`

One row per resolved `codebase-*` directive in a review doc.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment |
| `doc_id` | INTEGER FK | References `review_docs(id)` |
| `stub_id` | TEXT | Stable per-document snippet key such as `stub-1` (retained as a snippet identifier, not an HTML mount stub) |
| `directive` | TEXT | `codebase-snippet`, `codebase-diff`, etc. |
| `symbol_id` | TEXT | Resolved symbol ID |
| `file_path` | TEXT | Source file path |
| `kind` | TEXT | `func`, `declaration`, `diff`, etc. |
| `language` | TEXT | `"go"`, `"ts"`, `"text"` |
| `text` | TEXT | Pre-resolved snippet text |
| `params_json` | TEXT | Directive parameters |
| `start_line` / `end_line` | INTEGER | Line range |
| `commit_hash` | TEXT | If `commit=` was specified |

### `static_review_pages`

One row per structured review document in the exported browser database (`db/codebase.db`). This table is populated by `review export`, not by `review index`. The browser renders the ordered block model directly; it does not scan HTML for widget placeholders.

| Column | Type | Description |
|--------|------|-------------|
| `slug` | TEXT PK | Review document slug |
| `title` | TEXT | Review document title |
| `blocks_json` | TEXT | JSON array of ordered blocks. Markdown blocks contain rendered HTML; widget blocks contain directive names and params. |
| `diagnostics_json` | TEXT | JSON array of structured render/validation diagnostics; `[]` when clean |
| `rendered_at` | INTEGER | Unix timestamp when export rendered the document |

## Common SQL queries

### List all indexed commits

```sql
SELECT short_hash, message, author_name, datetime(author_time, 'unixepoch') AS date
FROM commits
ORDER BY sequence DESC, author_time DESC;
```

### Find symbols whose signatures changed between the first and last commit

```sql
SELECT
    s1.name,
    s1.signature AS old_sig,
    s2.signature AS new_sig,
    c1.short_hash AS old_commit,
    c2.short_hash AS new_commit
FROM snapshot_symbols s1
JOIN snapshot_symbols s2 ON s1.id = s2.id
JOIN commits c1 ON c1.hash = s1.commit_hash
JOIN commits c2 ON c2.hash = s2.commit_hash
WHERE c1.author_time = (SELECT MIN(author_time) FROM commits)
  AND c2.author_time = (SELECT MAX(author_time) FROM commits)
  AND s1.signature != s2.signature;
```

### Count symbols per commit

```sql
SELECT c.short_hash, COUNT(s.id) AS symbol_count
FROM commits c
LEFT JOIN snapshot_symbols s ON s.commit_hash = c.hash
GROUP BY c.hash
ORDER BY c.author_time DESC;
```

### Find all callers of a specific symbol

```sql
SELECT
    r.from_symbol_id,
    s.name AS caller_name,
    s.signature AS caller_sig,
    f.path
FROM snapshot_refs r
JOIN snapshot_symbols s ON s.id = r.from_symbol_id AND s.commit_hash = r.commit_hash
JOIN snapshot_files f ON f.id = s.file_id AND f.commit_hash = s.commit_hash
WHERE r.to_symbol_id = 'sym:github.com/foo/bar.func.Target'
  AND r.commit_hash = (SELECT hash FROM commits ORDER BY sequence DESC, author_time DESC LIMIT 1);
```

### List review documents and their snippet counts

```sql
SELECT d.slug, d.title, COUNT(s.id) AS snippet_count
FROM review_docs d
LEFT JOIN review_doc_snippets s ON s.doc_id = d.id
GROUP BY d.id;
```

## Symbol ID scheme

Symbol IDs are stable across file moves. The format is:

```
sym:<importPath>.<kind>.<name>              # top-level declaration
sym:<importPath>.method.<Recv>.<name>       # method
```

Examples:
- `sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract`
- `sym:github.com/wesen/codebase-browser/internal/indexer.method.Store.LoadSnapshot`

Markdown directives must use full `sym:` IDs. Short refs are intentionally unsupported because they are ambiguous across packages.

## Commit range syntax

The `--commits` flag accepts any git log range specification:

| Example | Meaning |
|---------|---------|
| `HEAD~10..HEAD` | Last 10 commits |
| `main..feature` | Commits on `feature` not on `main` |
| `abc123..def456` | Commits between two SHAs |
| `HEAD` | Just the current commit |
| `--all` | All reachable commits |

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `UNIQUE constraint failed: snapshot_symbols` | Duplicate symbol IDs (e.g. blank identifiers) | Fixed in history loader — first occurrence wins |
| `no commits in review database` | `LoadLatestSnapshot` called on empty DB | Run `review index` or `review db create` first |
| `render doc: symbol not found` | Doc references a symbol not in indexed commits | Ensure the commit range includes the symbol |
| **0 snippets resolved in review doc** | Directives reference symbols that are missing from the indexed range or are not full `sym:` IDs | Use full `sym:` IDs; query DB: `SELECT DISTINCT stable_id FROM symbols` |
| **Fewer packages than expected** | Default `--patterns` only covers `./cmd/...` and `./internal/...` | Pass `--patterns` explicitly: `--patterns ./...,./pkg/...` |
| Byte offsets don't match JavaScript string indices | `start_offset`/`end_offset` are byte offsets, not UTF-16 code units | Always decode `file_contents.content` bytes to a string before indexing by position |

## See Also

- `user-guide` — Tutorial for writing review markdown files
- `markdown-block-reference` — Canonical reference for every `codebase-*` directive
