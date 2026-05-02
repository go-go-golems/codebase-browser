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

## Step 3: Design document and reMarkable upload

Wrote the full design document (`design/01-performance-analysis-and-design-guide-for-review-indexing.md`, 1258 lines) covering:
- §1-2: What codebase-browser is and how the 4-phase indexing pipeline works
- §3: Deep dive into every SQLite table with column explanations
- §4: Performance measurements with real numbers
- §5: The redundancy problem (99%+ deduplication ratios, 98% average commit overlap)
- §6: Normalized schema design with full DDL, pseudocode, and compatibility views
- §7: Incremental indexing design with flow diagrams
- §8: 5-phase implementation plan
- §9-10: API and file reference tables
- §11: ASCII diagrams showing current vs proposed data flow
- Appendix A: Glossary, Appendix B: Reproduction instructions

Uploaded as a bundled PDF to reMarkable at `/ai/2026/05/02/GCB-017`.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Continue the analysis, write the full design document, upload to reMarkable.

**Inferred user intent:** Produce a comprehensive, intern-ready guide that covers everything needed to understand and optimize the system.

### What I did

- Wrote the design document in 6 append calls to avoid tool call length limits
- Created 14 numbered SQL analysis scripts (`scripts/00-*.sql` through `scripts/13-*.sql`)
- Created 2 shell scripts (`scripts/01-benchmark-indexing.sh`, `scripts/02-analyze-db.sh`)
- Related 15 source files to the diary and design documents
- Uploaded bundled PDF to reMarkable via `remarquee upload bundle`
- Committed all ticket work

### Why

The design document needs to be thorough enough for an intern to understand the entire system and implement the optimization without additional context.

### What worked

- Writing the document in 6 parts kept each tool call manageable
- `remarquee upload bundle` produced a clean PDF with a ToC
- The SQL scripts provide reproducible analysis

### What didn't work

- N/A

### What I learned

- The current schema has exactly the right structure for the initial use case (static export), but doesn't scale to continuous/incremental use
- The `file_contents` table is already well-designed (deduplicated by SHA-256) — it's the snapshot tables that need normalization

### What was tricky to build

- Ensuring the normalized schema's compatibility views produce identical column names/types to the current tables, so the React browser's SQL queries don't need changes

### What warrants a second pair of eyes

- The `ref_versions.locations_json` approach for collapsing multiple ref locations into one row — verify this doesn't break any browser queries that expect one row per location

### What should be done in the future

- Run benchmarks on a large repo (glazed, go-go-golems) to confirm the 99% dedup ratios hold at scale
- Investigate the worktree extraction bug (empty symbol/file/ref tables)
- Implement Phase 2 (normalized schema)

### Code review instructions

- Read the design document top-to-bottom; it's self-contained
- Run `sqlite-viz tables -d /tmp/review.db` to verify the table size numbers
- Run `sqlite3 -header -column /tmp/review.db < scripts/02-deduplication-analysis.sql` to reproduce the redundancy findings

### Technical details

- Design doc: `ttmp/.../GCB-017/design/01-performance-analysis-and-design-guide-for-review-indexing.md` (1258 lines)
- Diary: `ttmp/.../GCB-017/reference/01-investigation-diary.md`
- Scripts: `ttmp/.../GCB-017/scripts/` (16 files)
- Commit: a8d1a1d — "GCB-017: Performance analysis and design guide for review indexing"
- reMarkable: `/ai/2026/05/02/GCB-017/GCB-017 Performance Analysis - Codebase Browser Review Indexing.pdf`

## Step 4: Root-caused the worktree extraction bug

Investigated why multi-commit worktree-based indexing produces empty symbol/file/ref tables. Found the root cause.

### Prompt Context

**User prompt (verbatim):** "what's the worktree extraction bug?"

**Assistant interpretation:** Explain and root-cause the bug mentioned in the diary about empty tables from worktree-based extraction.

**Inferred user intent:** Understand the actual bug, not just that it exists.

### What I did

1. Tested with 1 commit (indexDirect) → works perfectly (8 packages, 23 files, 88 symbols, 758 refs)
2. Tested with 3 commits (indexWithWorktrees) → broken (3 packages with `id="pkg:./cmd/..."`, empty names, 0 files/symbols/refs)
3. Wrote a standalone Go test program to call `packages.Load` from a git worktree
4. Got the error: `"pattern ./cmd/...: directory prefix cmd does not contain modules listed in go.work or their selected dependencies"`
5. Found `go.work` in the **parent directory** (`corporate-headquarters/go.work`) which lists `./codebase-browser` as a workspace module
6. The worktree is at `.git-worktrees/<hash>` — still a subdirectory of `corporate-headquarters/`, so Go's workspace mode finds the parent `go.work` and rejects the worktree's packages
7. Tested with `GOWORK=off` env var → **33 packages loaded correctly**

### Why

Go 1.18+ introduced workspace mode (`go.work`). When `packages.Load` runs from a directory, it walks upward looking for a `go.work` file. If found, it restricts package resolution to only the modules listed in `go.work`. The worktree directory (`.git-worktrees/<hash>`) isn't listed in `go.work`, so Go refuses to load its packages.

### Root Cause

**File:** `internal/indexer/extractor.go` line ~51 (`packages.Config` initialization)

**Missing:** `Env: append(os.Environ(), "GOWORK=off")`

The `packages.Config` does not set `GOWORK=off`, so when extracting from a worktree subdirectory, Go workspace mode kicks in and rejects the packages.

### Fix

One-line fix in `internal/indexer/extractor.go`:

```go
cfg := &packages.Config{
    Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
        packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
        packages.NeedImports | packages.NeedModule,
    Dir:   absRoot,
    Fset:  token.NewFileSet(),
    Tests: opts.IncludeTests,
    Env:   append(os.Environ(), "GOWORK=off"), // ← ADD THIS LINE
}
```

### What worked

- Writing a standalone Go test program was the fastest way to isolate the issue
- The error message from `packages.Load` was actually very clear once we saw it

### What didn't work

- The extractor silently swallows `packages.Load` errors (the "Non-fatal" comment) — this hid the real error
- Running `go list` from the worktree worked fine because `go list` doesn't use workspace mode the same way (it has GOWORK set by the main repo's context)

### What I learned

- Go workspace mode (`go.work`) is inherited by child processes and `packages.Load`
- Worktrees created inside the repo directory inherit the parent's `go.work` context
- The single-commit path works because it uses the main working directory, which IS listed in `go.work`

### What was tricky to build

- N/A

### What warrants a second pair of eyes

- The fix (adding `GOWORK=off`) — verify this doesn't break anything in non-workspace setups
- Should we also set `GOFLAGS=-mod=mod` for robustness?

### What should be done in the future

- Apply the one-line fix and test with multi-commit indexing
- Add an integration test that indexes 3+ commits with worktrees and verifies non-empty symbol/file/ref tables
- Consider whether the extractor should log `packages.Load` errors as warnings instead of silently continuing

### Code review instructions

- Look at `internal/indexer/extractor.go:51` — the `packages.Config` struct
- The fix is adding `Env: append(os.Environ(), "GOWORK=off")`

### Technical details

- Error: `pattern ./cmd/...: directory prefix cmd does not contain modules listed in go.work or their selected dependencies`
- Root cause: `corporate-headquarters/go.work` lists `./codebase-browser` but worktrees are at `.git-worktrees/<hash>` which is NOT listed
- Fix: `GOWORK=off` in `packages.Config.Env`
- Verified: `GOWORK=off` → 33 packages loaded correctly from worktree
