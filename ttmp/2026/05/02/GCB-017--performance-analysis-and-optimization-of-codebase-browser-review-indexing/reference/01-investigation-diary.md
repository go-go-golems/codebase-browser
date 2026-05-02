---
Title: ""
Ticket: ""
Status: ""
Topics: []
DocType: ""
Intent: ""
Owners: []
RelatedFiles:
    - Path: cmd/codebase-browser/cmds/review/index.go
      Note: CLI entry point for review index command
    - Path: internal/history/cache.go
      Note: File contents caching (already deduplicated by SHA-256)
    - Path: internal/history/indexer.go
      Note: Per-commit extraction orchestration (worktree + extract + load)
    - Path: internal/history/loader.go
      Note: Bulk INSERT into snapshot_* tables (the normalization target)
    - Path: internal/history/schema.go
      Note: Current SQLite schema DDL (the tables we're analyzing)
    - Path: internal/indexer/extractor.go
      Note: Go AST → Index extraction (the expensive phase)
    - Path: internal/indexer/types.go
      Note: Index/Package/File/Symbol/Ref type definitions
    - Path: internal/indexer/xref.go
      Note: Cross-reference extraction
    - Path: internal/review/indexer.go
      Note: Top-level review indexing orchestrator (4 phases)
    - Path: internal/review/store.go
      Note: Review DB connection management (Create vs Open distinction needed for incremental)
ExternalSources: []
Summary: ""
LastUpdated: 0001-01-01T00:00:00Z
WhatFor: ""
WhenToUse: ""
---


# Diary

## Goal

Investigate codebase-browser review indexing performance: measure indexing time at different scales, analyze the resulting SQLite database sizes, identify the biggest contributors to bloat, and design an incremental indexing approach that avoids re-extracting unchanged data.

## Step 1: Initial exploration and ticket setup

Created ticket GCB-017, set up diary and design document placeholders. Read the core source files to understand the four-phase indexing pipeline:

1. **Phase 1 — Resolve commits** (`gitutil.LogCommits`): parses the commit range spec into a list of `gitutil.Commit` structs.
2. **Phase 2 — Index commits** (`history.IndexCommits`): for each commit, creates a git worktree, runs `indexer.Extract` (Go AST walk via `golang.org/x/tools/go/packages`), and bulk-loads the resulting `Index` into SQLite via `LoadSnapshot`. Also calls `CacheFileContents` to store file bodies keyed by SHA-256.
3. **Phase 3 — Discover markdown docs** (`discoverDocs`): resolves file/directory paths into `.md` files.
4. **Phase 4 — Index each markdown file** (`indexDoc`): renders each doc (resolving codebase-snippet directives) and stores the raw markdown + resolved snippets.

### Prompt Context

**User prompt (verbatim):** "Create a new docmgr ticket to examine the performance of codebase-browser, both in review indexing time and in the size of the resulting database. Keep a diary as you work."

**Assistant interpretation:** Create a docmgr ticket GCB-017, measure indexing performance, analyze database sizes, and investigate incremental indexing feasibility.

**Inferred user intent:** Understand where time and space are spent in the indexing pipeline, find optimization opportunities, and produce a thorough design document for an intern to implement.

### What I did

- Created ticket GCB-017 with `docmgr ticket create-ticket`
- Read `README.md`, `internal/review/indexer.go`, `internal/review/schema.go`, `internal/history/indexer.go`, `internal/history/schema.go`, `internal/indexer/extractor.go`, `internal/indexer/types.go`, `internal/indexer/xref.go`, `internal/history/store.go`, `internal/history/loader.go`, `internal/history/cache.go`, `internal/review/store.go`
- Mapped the data flow: `IndexReview → IndexCommits → Extract → LoadSnapshot → CacheFileContents`

### Why

Needed to understand the full pipeline before writing measurement scripts or proposing changes.

### What worked

- The source is well-structured and the four phases are cleanly separated.
- `history.Store.HasCommit()` already exists — it's the hook needed for incremental indexing.

### What didn't work

- Running `review index` with `--commits HEAD` resolved to 193 commits (the entire history), because `HEAD` as a range means "all ancestors of HEAD". A single-commit snapshot needs `--commits HEAD..HEAD` or equivalent.

### What I learned

- The default `--patterns` is `["./cmd/...", "./internal/..."]` — only Go code under those paths is extracted.
- Multi-commit indexing uses git worktrees. Each worktree creation + extraction + teardown is sequential.
- The TS indexer (`cmd/build-ts-index`) uses Dagger (Docker) which is very slow for single-file changes.

### What was tricky to build

- N/A (reading phase only).

### What warrants a second pair of eyes

- The worktree-based extraction only populates `snapshot_packages` but produces 0 rows for `snapshot_files`, `snapshot_symbols`, `snapshot_refs`. This might be a bug in how `packages.Load` resolves patterns from worktree directories that lack a module cache, or a silent failure in the extractor. Needs investigation.

### What should be done in the future

- Investigate why worktree extraction produces empty symbol/file/ref tables.
- Test with a larger Go codebase (e.g., glazed or go-go-golems) to see scaling behavior.

### Code review instructions

- Start with `internal/review/indexer.go:IndexReview()` to see the four phases.
- Follow the call chain: `IndexCommits → indexWithWorktrees → Extract → LoadSnapshot`.

### Technical details

- Key files: `internal/review/indexer.go`, `internal/history/indexer.go`, `internal/indexer/extractor.go`

## Step 2: Database size analysis on the production 264MB database

Analyzed `/tmp/review.db` (264MB, 181 commits, indexed from the codebase-browser repo itself using `./...` patterns) using `sqlite-viz` and direct SQL queries.

### What I did

- Ran `sqlite-viz tables -d /tmp/review.db` to get table + index sizes
- Wrote SQL queries to measure deduplication ratios, symbol change frequency, per-commit snapshot sizes, and consecutive commit overlap
- Saved all SQL queries as numbered scripts in `scripts/00-*.sql` through `scripts/13-*.sql`

### Key findings

**Table sizes (data + index):**

| Table | Rows | Data | Index | % of DB |
|---|---|---|---|---|
| `snapshot_refs` | 331,208 | 78 MB | 122 MB | 75.9% |
| `snapshot_symbols` | 62,256 | 31 MB | 24 MB | 20.7% |
| `snapshot_files` | 11,848 | 3.0 MB | 2.8 MB | 2.2% |
| `snapshot_packages` | 4,840 | 1.2 MB | 0.9 MB | 0.8% |
| `file_contents` | 185 | 872 KB | 20 KB | 0.3% |
| `commits` | 181 | 52 KB | 24 KB | 0.03% |

**Deduplication ratios (massive redundancy):**

- Symbols: 646 unique body hashes across 62,256 rows → **99.0% are duplicates**
- Files: 185 unique SHA-256s across 11,848 rows → **98.4% are duplicates**
- Refs: 2,122 unique (from,to,kind) triples across 331,208 rows → **99.4% are duplicates**
- Packages: 37 unique IDs across 4,840 rows → **99.2% are duplicates**

**Consecutive commit overlap:**

- **98% average overlap** between consecutive commits (symbols unchanged)
- 164 out of 180 commit pairs have ≥95% symbol overlap
- Only 1 pair has <50% overlap (the very first commit)

**Symbol change frequency:**

- Most symbols appear in many commits but have only 1-2 distinct body hashes (they don't change)
- `main` has 9 distinct versions across 180 commits (most-changed function)
- Typical function has 1 version across all its commits

### Why

These numbers tell us that the current "snapshot per commit" approach stores massive amounts of redundant data. A normalized schema would reduce the 264MB database by roughly **98%**.

### What worked

- `sqlite-viz tables` gave clean per-table size breakdowns
- The deduplication queries were straightforward with `COUNT(DISTINCT ...)` / `COUNT(*)`
- The consecutive-overlap query using `LAG()` window function worked well

### What didn't work

- The `dbstat` virtual table approach for per-index page counts was unreliable (returned 0.0 for some aggregations)

### What I learned

- `snapshot_refs` is the single biggest table (76% of DB) and is 99.4% redundant
- The ID scheme uses long fully-qualified strings like `sym:github.com/wesen/codebase-browser/internal/review.func.IndexReview` — averaging ~80 bytes per ID
- Indexes on `snapshot_refs` consume 122MB (more than the 78MB of data) because of the long string keys

### What was tricky to build

- The consecutive-commit overlap query needed careful CTE nesting with window functions

### What warrants a second pair of eyes

- The 99.4% ref dedup ratio — verify this isn't an artifact of the test data being a single repo indexed against itself.

### What should be done in the future

- Re-run this analysis on a larger codebase (e.g., 10K+ symbols) to confirm the ratios hold.

### Code review instructions

- Run the SQL scripts in order against `/tmp/review.db` to reproduce all findings.

### Technical details

- All SQL scripts saved as `scripts/00-*.sql` through `scripts/13-*.sql`
- Shell wrapper: `scripts/02-analyze-db.sh` runs the full analysis pipeline
