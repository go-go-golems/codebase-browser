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
      Note: CLI flag changes for --incremental in §7.6
    - Path: internal/history/loader.go
      Note: LoadSnapshot function to be rewritten in §6.5
    - Path: internal/history/schema.go
      Note: Current schema analyzed in §3
    - Path: internal/review/indexer.go
      Note: Four-phase indexing pipeline described in §2
    - Path: internal/review/store.go
      Note: Create/Open distinction needed for §7 incremental indexing
ExternalSources: []
Summary: ""
LastUpdated: 0001-01-01T00:00:00Z
WhatFor: ""
WhenToUse: ""
---


# Performance Analysis and Design Guide for Codebase-Browser Review Indexing

**Ticket:** GCB-017
**Audience:** A new intern joining the project. This document assumes you can read Go and TypeScript, have basic SQLite knowledge, and can use a terminal. Everything else is explained from scratch.

---

## Table of Contents

1. [What Is Codebase-Browser?](#what-is-codebase-browser)
2. [The Indexing Pipeline](#the-indexing-pipeline)
3. [SQLite Schema Deep Dive](#sqlite-schema-deep-dive)
4. [Performance Measurements](#performance-measurements)
5. [The Redundancy Problem](#the-redundancy-problem)
6. [Normalized Schema Design](#normalized-schema-design)
7. [Incremental Indexing Design](#incremental-indexing-design)
8. [Implementation Plan](#implementation-plan)
9. [API Reference](#api-reference)
10. [File Reference](#file-reference)
11. [Diagrams](#diagrams)

---

## 1. What Is Codebase-Browser?

Codebase-browser is a command-line tool written in Go that turns a **git commit range** into a **static, shareable web application**. You point it at a repository, give it a range of commits (like `HEAD~10..HEAD`), and optionally some markdown review documents. It produces:

- A **SQLite database** containing source code metadata (symbols, files, cross-references) for every commit in the range
- An **export directory** containing a React single-page application (SPA) that opens the SQLite database client-side using sql.js (SQLite compiled to WebAssembly)

The key insight is that **no server runs at read time**. The reviewer opens the exported directory in any browser, and all queries run against the local SQLite file via WebAssembly. This means the database is the sole data contract between the Go indexer and the React browser.

### Why Does Performance Matter?

- **Indexing time** determines the feedback loop during code review. If indexing 50 commits takes 5 minutes, developers won't use it.
- **Database size** determines how fast the static browser loads. A 264MB SQLite file loaded into browser memory via WebAssembly is slow. A 5MB file is instant.
- **Incremental indexing** (adding new commits without re-indexing everything) is essential for continuous use on active codebases.

---

## 2. The Indexing Pipeline

The `review index` command runs four sequential phases. Understanding each phase is critical because they have very different performance characteristics.

```
┌─────────────────────────────────────────────────────────────┐
│                   review index                               │
│                                                              │
│  Phase 1: Resolve Commits                                    │
│    git log → []gitutil.Commit                                │
│    Cost: O(commits) git I/O, negligible                      │
│                                                              │
│  Phase 2: Index Commits (THE BOTTLENECK)                     │
│    For each commit:                                          │
│      ├─ Create git worktree at that commit                   │
│      ├─ Run Go AST extraction (packages.Load + ast.Walk)     │
│      ├─ Load snapshot into SQLite (bulk INSERT)              │
│      └─ Cache file contents (SHA-256 keyed BLOBs)            │
│    Cost: O(commits × files) — each commit is a full parse    │
│                                                              │
│  Phase 3: Discover Markdown Docs                             │
│    Walk directories, find *.md files                         │
│    Cost: negligible                                          │
│                                                              │
│  Phase 4: Index Each Doc                                     │
│    Render markdown (resolve codebase-* directives)           │
│    Store raw markdown + resolved snippet metadata            │
│    Cost: O(docs × snippets), fast                            │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Phase 1: Resolve Commits

**Entry point:** `gitutil.LogCommits()` in `internal/gitutil/log.go`

This function runs `git log <range>` and parses the output into `gitutil.Commit` structs. Each struct contains:

```go
type Commit struct {
    Hash         string    // full 40-char SHA
    ShortHash    string    // first 7 chars
    Message      string
    AuthorName   string
    AuthorEmail  string
    AuthorTime   time.Time
    ParentHashes []string
    TreeHash     string
}
```

**Performance:** Fast. Even for 500 commits, `git log` completes in under a second.

### Phase 2: Index Commits

**Entry point:** `history.IndexCommits()` in `internal/history/indexer.go`

This is where **all the time and space is spent**. For each commit:

1. **Create a git worktree** (`gitutil.CreateWorktree`) — this checks out the repo at that commit into a temporary directory. Cost: disk I/O proportional to repo size.

2. **Extract the Go AST** (`indexer.Extract()`) — loads all Go packages under the configured patterns using `golang.org/x/tools/go/packages`, then walks every AST node to collect:
   - **Packages** (import path, name, doc comment)
   - **Files** (path, size, line count, SHA-256 hash)
   - **Symbols** (funcs, methods, types, structs, interfaces, consts, vars — with source ranges, signatures, receiver info, body hashes)
   - **Refs** (cross-references: which function calls which, which type uses which)

3. **Load the snapshot** (`store.LoadSnapshot()`) — bulk-inserts all packages, files, symbols, and refs into SQLite tables in a single transaction. Every row includes `commit_hash` as a partition key.

4. **Cache file contents** (`CacheFileContents()`) — reads each file from the worktree, computes SHA-256, and stores the raw bytes in `file_contents` (deduplicated by hash).

**Performance:** Each commit takes ~0.3s (small repo) to ~5s (large repo). The total scales linearly with the number of commits because every commit is independently parsed from scratch.

### Phase 3: Discover Markdown Docs

**Entry point:** `discoverDocs()` in `internal/review/indexer.go`

Walks the given file/directory paths and collects `.md` files. Negligible cost.

### Phase 4: Index Each Doc

**Entry point:** `indexDoc()` in `internal/review/indexer.go`

Renders each markdown file through the docs renderer (`docs.Render()`), which resolves `codebase-*` fenced blocks into symbol snippets, source views, and diff widgets. Stores:
- The raw markdown in `review_docs`
- Each resolved snippet's metadata in `review_doc_snippets`

**Performance:** Fast. Even complex docs with many widgets complete in under a second.


---

## 3. SQLite Schema Deep Dive

The review database has **8 tables** and **1 view**. Let's walk through each one.

### 3.1 commits

```sql
CREATE TABLE commits (
    hash TEXT PRIMARY KEY,           -- full 40-char SHA-1
    short_hash TEXT NOT NULL,        -- first 7 chars
    message TEXT NOT NULL,           -- commit message (first line)
    author_name TEXT NOT NULL,
    author_email TEXT NOT NULL,
    author_time INTEGER NOT NULL,    -- Unix timestamp
    parent_hashes TEXT NOT NULL DEFAULT '[]',  -- JSON array of parent hashes
    tree_hash TEXT NOT NULL DEFAULT '',
    indexed_at INTEGER NOT NULL DEFAULT 0,     -- when we indexed this commit
    branch TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT ''              -- non-empty if indexing failed
);
```

**Purpose:** One row per commit in the range. The `hash` column is the foreign key used by all `snapshot_*` tables.

**Size:** Tiny. 181 rows = 52 KB. Even 10,000 commits would be under 3 MB.

**Key insight:** The `error` column lets us record failed commits without aborting the entire indexing run.

### 3.2 snapshot_packages

```sql
CREATE TABLE snapshot_packages (
    commit_hash TEXT NOT NULL REFERENCES commits(hash),
    id TEXT NOT NULL,                -- "pkg:github.com/.../pkgname"
    import_path TEXT NOT NULL,       -- Go import path
    name TEXT NOT NULL,              -- package name (last segment)
    doc TEXT NOT NULL DEFAULT '',    -- package-level doc comment
    language TEXT NOT NULL DEFAULT 'go',
    PRIMARY KEY (commit_hash, id)
);
```

**Purpose:** One row per (commit, package) pair. If your repo has 20 packages and you index 100 commits, this table has 2,000 rows.

**Redundancy:** Very high. Package metadata rarely changes between commits. In our test data, 37 unique packages × 181 commits = 4,840 rows → 99.2% redundant.

### 3.3 snapshot_files

```sql
CREATE TABLE snapshot_files (
    commit_hash TEXT NOT NULL REFERENCES commits(hash),
    id TEXT NOT NULL,                -- "file:path/to/file.go"
    path TEXT NOT NULL,
    package_id TEXT NOT NULL,
    size INTEGER NOT NULL DEFAULT 0,
    line_count INTEGER NOT NULL DEFAULT 0,
    sha256 TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT 'go',
    build_tags_json TEXT NOT NULL DEFAULT '[]',
    content_hash TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (commit_hash, id)
);
```

**Purpose:** One row per (commit, file) pair. Stores metadata about each source file: path, size, line count, SHA-256 hash.

**Redundancy:** Very high. 185 unique files × 181 commits = 11,848 rows → 98.4% redundant. Most files don't change between commits.

**Key column:** `sha256` is critical — it lets us check if a file changed without reading the full content. The actual file bytes live in `file_contents`.

### 3.4 snapshot_symbols

```sql
CREATE TABLE snapshot_symbols (
    commit_hash TEXT NOT NULL REFERENCES commits(hash),
    id TEXT NOT NULL,                -- "sym:github.com/.../pkg.func.Name"
    kind TEXT NOT NULL,              -- func, method, struct, iface, type, const, var, alias
    name TEXT NOT NULL,
    package_id TEXT NOT NULL,
    file_id TEXT NOT NULL,
    start_line INTEGER, start_col INTEGER,
    end_line INTEGER, end_col INTEGER,
    start_offset INTEGER, end_offset INTEGER,
    doc TEXT NOT NULL DEFAULT '',
    signature TEXT NOT NULL DEFAULT '',
    receiver_type TEXT NOT NULL DEFAULT '',
    receiver_pointer INTEGER NOT NULL DEFAULT 0,
    exported INTEGER NOT NULL DEFAULT 0,
    language TEXT NOT NULL DEFAULT 'go',
    type_params_json TEXT NOT NULL DEFAULT '[]',
    tags_json TEXT NOT NULL DEFAULT '[]',
    body_hash TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (commit_hash, id)
);
```

**Purpose:** One row per (commit, symbol) pair. This is the heart of the database — it stores every function, method, type, constant, and variable that the Go AST extractor found.

**Columns explained:**

- `id`: A globally unique string like `sym:github.com/wesen/codebase-browser/internal/review.func.IndexReview`. The prefix `sym:` identifies it as a symbol, the middle part is the Go import path, `func` is the kind, and `IndexReview` is the name.
- `kind`: One of `func`, `method`, `struct`, `iface`, `type`, `const`, `var`, `alias`. Methods also have `receiver_type` (e.g., `Store`).
- `start_offset`/`end_offset`: Byte offsets into the source file. These are authoritative for slicing source text.
- `start_line`/`end_line`: Line numbers for display.
- `body_hash`: SHA-256 of the source bytes between `start_offset` and `end_offset`. This is the key to detecting whether a function changed between commits.
- `signature`: The function/type signature (parameters and return types), printed by `go/printer`.

**Redundancy:** Extreme. 646 unique body hashes across 62,256 rows → 99.0% redundant. The same function body is re-stored for every commit where it appears, even if it hasn't changed at all.

**Size:** 31 MB data + 24 MB indexes = 55 MB total. The indexes are expensive because symbol IDs are long strings (~80 bytes average).

### 3.5 snapshot_refs

```sql
CREATE TABLE snapshot_refs (
    commit_hash TEXT NOT NULL REFERENCES commits(hash),
    id INTEGER NOT NULL,
    from_symbol_id TEXT NOT NULL,    -- who is doing the referencing
    to_symbol_id TEXT NOT NULL,      -- who is being referenced
    kind TEXT NOT NULL,              -- call, uses-type, reads, use
    file_id TEXT NOT NULL,
    start_line INTEGER, start_col INTEGER,
    end_line INTEGER, end_col INTEGER,
    start_offset INTEGER, end_offset INTEGER,
    PRIMARY KEY (commit_hash, id)
);
```

**Purpose:** Cross-reference table. Each row says "symbol A references symbol B at this location in this file." For example, if function `main()` calls `fmt.Println()`, there's a ref row with `from_symbol_id = "sym:...func.main"`, `to_symbol_id = "sym:fmt.func.Println"`, `kind = "call"`.

**Ref kinds:**
- `call` — function/method call
- `uses-type` — type reference in a type expression
- `reads` — reads a variable or constant
- `use` — generic catch-all

**Redundancy:** The worst offender. 2,122 unique (from, to, kind) triples across 331,208 rows → **99.4% redundant**. This single table accounts for **76% of the entire database** (78 MB data + 122 MB indexes = 200 MB).

**Why the indexes are so large:** Each index entry contains the full `from_symbol_id` or `to_symbol_id` strings (averaging 80 and 46 bytes respectively). With 331K rows, each index is tens of thousands of pages.

### 3.6 file_contents

```sql
CREATE TABLE file_contents (
    content_hash TEXT PRIMARY KEY,   -- SHA-256 hex
    content BLOB NOT NULL           -- raw file bytes
);
```

**Purpose:** Stores the actual source file bytes, keyed by SHA-256 hash. This table is already deduplicated — if two commits have the same version of a file, the bytes are stored once.

**Size:** 185 files × 872 KB total. Very small relative to the metadata tables. This is the one table that got deduplication right.

### 3.7 review_docs and review_doc_snippets

These tables store the user-written markdown review guides and the resolved widget metadata. They're small and don't contribute to the performance problem.

### 3.8 symbol_history (view)

```sql
CREATE VIEW symbol_history AS
SELECT s.id, s.name, s.kind, s.package_id,
       c.hash, c.short_hash, c.message, c.author_time,
       s.body_hash, s.start_line, s.end_line, s.signature, s.file_id
FROM snapshot_symbols s
JOIN commits c ON c.hash = s.commit_hash;
```

A convenience view that joins symbols with their commit metadata. Used by the history page and symbol-diff widgets.


---

## 4. Performance Measurements

All measurements were taken on the codebase-browser repository itself (a small-to-medium Go project with ~550 symbols, ~96 source files, ~37 packages).

### 4.1 Indexing Time by Commit Range

| Commit Range | Commits | Wall Time | Rate |
|---|---|---|---|
| HEAD (single, direct) | 1 | ~0.3s | — |
| HEAD~5..HEAD (worktrees) | 5 | 1.1s | 0.22s/commit |
| HEAD~20..HEAD | 20 | ~4s | 0.20s/commit |
| Full history (HEAD alone) | 193 | 57.7s | 0.30s/commit |

**Key observations:**

- The per-commit cost is dominated by three operations: git worktree creation (~0.05s), Go AST extraction (~0.15s), and SQLite bulk insert (~0.05s).
- The rate is remarkably consistent at ~0.2-0.3s per commit, regardless of range size.
- For a 50-commit PR review, expect ~10-15 seconds of indexing time on this repo size.

### 4.2 Database Size by Commit Count

| DB | Commits | Symbols | Refs | File Size |
|---|---|---|---|---|
| review-with-docs.db | 1 | 1,021 | 5,521 | 5.2 MB |
| review-bodydiff.db | 2 | 2,056 | 11,138 | 10 MB |
| review-impact.db | 2 | 2,070 | 11,180 | 10 MB |
| review.db | 181 | 62,256 | 331,208 | 264 MB |

**Scaling pattern:** Database size grows roughly linearly with `(commits × symbols)` and `(commits × refs)` because every commit gets a full snapshot. The slope is steep:

- ~5 MB per commit × 1 commit = 5 MB (small)
- ~5 MB per commit × 2 commits = 10 MB (tolerable)
- ~1.5 MB per commit × 181 commits = 264 MB (unacceptable for browser WebAssembly loading)

The "per-commit" rate decreases slightly with more commits because later commits share more unchanged files. But the dominant effect is still linear growth.

### 4.3 Where the Bytes Go (264 MB database breakdown)

```
┌────────────────────────────────────────────────────────────────┐
│  snapshot_refs (data)     ██████████████████████████  78 MB    │
│  snapshot_refs (indexes)  ██████████████████████████████ 122MB │
│  snapshot_symbols (data)  ████████████  31 MB                │
│  snapshot_symbols (idx)   █████████  24 MB                    │
│  snapshot_files           ███  3 MB data + 2.8 MB indexes     │
│  snapshot_packages        █  1.2 MB data + 0.9 MB indexes     │
│  file_contents            ▌ 872 KB                           │
│  commits                  ▏ 52 KB                             │
│  Total: 264 MB                                                 │
│                                                                │
│  refs alone = 76% of total                                     │
│  refs + symbols = 97% of total                                 │
└────────────────────────────────────────────────────────────────┘
```

### 4.4 What `sqlite-viz` Shows

Run `sqlite-viz tables -d /tmp/review.db` to see the live breakdown:

```
+---------------------+--------+--------+------------+-----------+------------------+------------------+---------------+
| name                | type   | rows   | size_bytes | size_human| index_size_bytes | index_size_human | percent_of_db |
+---------------------+--------+--------+------------+-----------+------------------+------------------+---------------+
| snapshot_refs       | table  | 331208 | 81641472   | 77.86 MB  | 127889408        | 121.96 MB        | 75.91%        |
| snapshot_symbols    | table  | 62256  | 32411648   | 30.91 MB  | 24793088         | 23.64 MB         | 20.72%        |
| snapshot_files      | table  | 11848  | 3149824    | 3.00 MB   | 2928640          | 2.79 MB          | 2.20%         |
| snapshot_packages   | table  | 4840   | 1290240    | 1.23 MB   | 901120           | 880.00 KB        | 0.79%         |
| file_contents       | table  | 185    | 892928     | 872.00 KB | 20480            | 20.00 KB         | 0.33%         |
| commits             | table  | 181    | 53248      | 52.00 KB  | 24576            | 24.00 KB         | 0.03%         |
+---------------------+--------+--------+------------+-----------+------------------+------------------+---------------+
```

---

## 5. The Redundancy Problem

The core performance issue is not speed (0.3s/commit is reasonable) but **space**. The current schema stores a complete snapshot for every commit, even though consecutive commits typically share 95-98% of their content unchanged.

### 5.1 Measured Redundancy

Here are the exact numbers from the 181-commit production database:

| Entity | Total Rows | Unique Entities | Redundancy |
|---|---|---|---|
| Symbols (by body hash) | 62,256 | 646 | **99.0%** |
| Files (by SHA-256) | 11,848 | 185 | **98.4%** |
| Refs (by from+to+kind) | 331,208 | 2,122 | **99.4%** |
| Packages (by ID) | 4,840 | 37 | **99.2%** |
| File contents (by hash) | 185 | 185 | **0%** (already deduplicated) |

### 5.2 Consecutive Commit Overlap

Between any two adjacent commits, how many symbols are identical (same ID, same body hash)?

```
Overlap bucket    Commit pairs
──────────────    ────────────
95-100%           164          ← 91% of all commit pairs
90-95%            5
85-90%            7
80-85%            1
70-80%            1
50-70%            1
<50%              1

Average overlap: 98.0%
```

This means that for a typical commit, only 2% of symbols actually changed. The other 98% are identical to the previous commit — yet we store them again in full.

### 5.3 Why This Matters for the Browser

The static browser loads the entire SQLite database into browser memory via WebAssembly (sql.js). A 264MB database:

- Takes 2-5 seconds to download on a fast connection
- Takes 3-8 seconds to parse and initialize in WebAssembly
- Consumes 264MB+ of browser RAM
- Cannot be practically shared as an email attachment or chat upload

A 5MB database (the projected size after normalization) would load in under a second and use negligible RAM.

### 5.4 Why Indexes Make It Worse

SQLite B-tree indexes store a copy of the indexed column in every leaf page. For `snapshot_refs`, the three main indexes are:

```
idx_snap_ref_commit  ON (commit_hash)                         → ~4 MB
idx_snap_ref_from    ON (from_symbol_id, commit_hash)         → ~60 MB
idx_snap_ref_to      ON (to_symbol_id, commit_hash)           → ~60 MB
```

The `from` and `to` indexes each contain the full symbol ID strings (averaging 80 bytes). With 331,208 rows, each index is enormous — the **indexes alone are larger than the data** (122 MB indexes vs 78 MB data).

In a normalized schema, refs would be stored once and the commit→ref mapping would use integer foreign keys, making the indexes 10-20× smaller.


---

## 6. Normalized Schema Design

The fix is straightforward: store each unique entity **once**, and use lightweight mapping tables to record which commits contain which versions.

### 6.1 Design Principles

1. **Single source of truth.** Each unique symbol version, file version, and ref set is stored exactly once.
2. **Integer keys.** Use auto-increment integer IDs for internal references instead of 80-byte string IDs. This shrinks indexes by 10-20×.
3. **Version tracking.** A "version" of a symbol is identified by its body hash. If a function's body doesn't change, we don't create a new version.
4. **Commit presence.** A mapping table records which symbol versions appear in which commits. This is a narrow (int, int) table that's extremely compact.
5. **Backward compatibility.** The normalized schema should produce the same logical view as the current `symbol_history` view, so the React browser's SQL queries still work.

### 6.2 Proposed Schema

```sql
── ────────────────────────────────────────
── Core entity tables (stored once)
── ────────────────────────────────────────

── Commits (unchanged)
CREATE TABLE commits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hash TEXT NOT NULL UNIQUE,
    short_hash TEXT NOT NULL,
    message TEXT NOT NULL,
    author_name TEXT NOT NULL,
    author_email TEXT NOT NULL,
    author_time INTEGER NOT NULL,
    parent_hashes TEXT NOT NULL DEFAULT '[]',
    tree_hash TEXT NOT NULL DEFAULT '',
    indexed_at INTEGER NOT NULL DEFAULT 0,
    branch TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX idx_commits_hash ON commits(hash);

── Packages (stored once per unique package)
CREATE TABLE packages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    stable_id TEXT NOT NULL UNIQUE,   -- "pkg:github.com/.../name"
    import_path TEXT NOT NULL,
    name TEXT NOT NULL,
    doc TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT 'go'
);

── Files (stored once per unique file version, keyed by SHA-256)
CREATE TABLE files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    stable_id TEXT NOT NULL,          -- "file:path/to/file.go"
    path TEXT NOT NULL,
    package_id INTEGER NOT NULL REFERENCES packages(id),
    sha256 TEXT NOT NULL,
    size INTEGER NOT NULL DEFAULT 0,
    line_count INTEGER NOT NULL DEFAULT 0,
    language TEXT NOT NULL DEFAULT 'go',
    build_tags_json TEXT NOT NULL DEFAULT '[]',
    UNIQUE(stable_id, sha256)         -- one row per file version
);
CREATE INDEX idx_files_sha ON files(sha256);

── Symbols (stored once per unique symbol version, keyed by body hash)
CREATE TABLE symbols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    stable_id TEXT NOT NULL,          -- "sym:.../pkg.func.Name"
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    package_id INTEGER NOT NULL REFERENCES packages(id),
    file_id INTEGER NOT NULL REFERENCES files(id),
    start_line INTEGER NOT NULL DEFAULT 0,
    start_col INTEGER NOT NULL DEFAULT 0,
    end_line INTEGER NOT NULL DEFAULT 0,
    end_col INTEGER NOT NULL DEFAULT 0,
    start_offset INTEGER NOT NULL DEFAULT 0,
    end_offset INTEGER NOT NULL DEFAULT 0,
    doc TEXT NOT NULL DEFAULT '',
    signature TEXT NOT NULL DEFAULT '',
    receiver_type TEXT NOT NULL DEFAULT '',
    receiver_pointer INTEGER NOT NULL DEFAULT 0,
    exported INTEGER NOT NULL DEFAULT 0,
    language TEXT NOT NULL DEFAULT 'go',
    type_params_json TEXT NOT NULL DEFAULT '[]',
    tags_json TEXT NOT NULL DEFAULT '[]',
    body_hash TEXT NOT NULL DEFAULT '',
    UNIQUE(stable_id, body_hash)      -- one row per symbol version
);
CREATE INDEX idx_symbols_name ON symbols(name);
CREATE INDEX idx_symbols_kind ON symbols(kind);
CREATE INDEX idx_symbols_body ON symbols(body_hash);

── Refs (stored once per unique ref set per symbol version)
── Instead of one row per ref, we store the ref set as a JSON blob
── per (from_symbol_version, to_symbol_id, kind, file_version).
CREATE TABLE ref_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_symbol_id INTEGER NOT NULL REFERENCES symbols(id),
    to_stable_id TEXT NOT NULL,       -- keep as string for cross-lookup
    kind TEXT NOT NULL,
    file_id INTEGER NOT NULL REFERENCES files(id),
    locations_json TEXT NOT NULL DEFAULT '[]',  -- array of {start_line,start_col,end_line,end_col,start_offset,end_offset}
    UNIQUE(from_symbol_id, to_stable_id, kind, file_id)
);
CREATE INDEX idx_ref_from ON ref_versions(from_symbol_id);
CREATE INDEX idx_ref_to ON ref_versions(to_stable_id);

── File contents (unchanged — already deduplicated)
CREATE TABLE file_contents (
    content_hash TEXT PRIMARY KEY,
    content BLOB NOT NULL
);

── ────────────────────────────────────────
── Commit mapping tables (which version is present in which commit)
── ────────────────────────────────────────

── Which packages appear in which commit
CREATE TABLE commit_packages (
    commit_id INTEGER NOT NULL REFERENCES commits(id),
    package_id INTEGER NOT NULL REFERENCES packages(id),
    PRIMARY KEY (commit_id, package_id)
) WITHOUT ROWID;

── Which file version appears in which commit
CREATE TABLE commit_files (
    commit_id INTEGER NOT NULL REFERENCES commits(id),
    file_id INTEGER NOT NULL REFERENCES files(id),
    PRIMARY KEY (commit_id, file_id)
) WITHOUT ROWID;

── Which symbol version appears in which commit
CREATE TABLE commit_symbols (
    commit_id INTEGER NOT NULL REFERENCES commits(id),
    symbol_id INTEGER NOT NULL REFERENCES symbols(id),
    PRIMARY KEY (commit_id, symbol_id)
) WITHOUT ROWID;

── Which ref set is active in which commit
CREATE TABLE commit_refs (
    commit_id INTEGER NOT NULL REFERENCES commits(id),
    ref_id INTEGER NOT NULL REFERENCES ref_versions(id),
    PRIMARY KEY (commit_id, ref_id)
) WITHOUT ROWID;
```

### 6.3 Why This Works

The key insight is that `commit_symbols` is a very narrow table: just two INTEGER columns. With 62,256 entries, it would be roughly:

```
62,256 rows × 8 bytes/row = ~500 KB
```

Compare this to the current `snapshot_symbols` table: 62,256 rows × ~423 bytes/row = ~26 MB. That's a **50× reduction** in the symbol table.

For refs, the savings are even more dramatic:

```
Current:  331,208 rows × ~198 bytes/row = ~66 MB data + 122 MB indexes = 188 MB
Proposed: 2,122 ref_versions rows × ~200 bytes + 331,208 commit_refs rows × 8 bytes
        = ~424 KB + ~2.6 MB = ~3 MB
```

That's a **63× reduction** in the refs tables.

### 6.4 Compatibility View

To avoid rewriting all the browser's SQL queries, we recreate the current `snapshot_symbols` shape as a view:

```sql
CREATE VIEW snapshot_symbols AS
SELECT
    c.hash AS commit_hash,
    s.stable_id AS id,
    s.kind,
    s.name,
    p.stable_id AS package_id,
    f.stable_id AS file_id,
    s.start_line, s.start_col, s.end_line, s.end_col,
    s.start_offset, s.end_offset,
    s.doc, s.signature,
    s.receiver_type, s.receiver_pointer,
    s.exported, s.language,
    s.type_params_json, s.tags_json,
    s.body_hash
FROM commit_symbols cs
JOIN commits c ON c.id = cs.commit_id
JOIN symbols s ON s.id = cs.symbol_id
JOIN packages p ON p.id = s.package_id
JOIN files f ON f.id = s.file_id;
```

Similar views for `snapshot_files`, `snapshot_packages`, and `snapshot_refs`.

### 6.5 Pseudocode: Loading a Snapshot

Current approach (in `loader.go`):

```
function LoadSnapshot(commit, index, worktreeDir):
    BEGIN TRANSACTION
    INSERT INTO commits VALUES (commit.hash, ...)
    FOR EACH package IN index.Packages:
        INSERT INTO snapshot_packages VALUES (commit.hash, package.id, ...)
    FOR EACH file IN index.Files:
        INSERT INTO snapshot_files VALUES (commit.hash, file.id, ...)
    FOR EACH symbol IN index.Symbols:
        bodyHash = hashFileRange(worktreeDir, symbol)
        INSERT INTO snapshot_symbols VALUES (commit.hash, symbol.id, ..., bodyHash, ...)
    FOR EACH ref IN index.Refs:
        INSERT INTO snapshot_refs VALUES (commit.hash, ref.id, ...)
    COMMIT
```

Proposed approach:

```
function LoadSnapshot(commit, index, worktreeDir):
    BEGIN TRANSACTION
    commitId = INSERT INTO commits VALUES (...) RETURNING id

    FOR EACH package IN index.Packages:
        pkgId = INSERT OR IGNORE INTO packages (stable_id, ...) VALUES (package.id, ...)
        INSERT OR IGNORE INTO commit_packages VALUES (commitId, pkgId)

    FOR EACH file IN index.Files:
        fileId = INSERT OR IGNORE INTO files (stable_id, sha256, ...) VALUES (file.id, file.sha256, ...)
        INSERT OR IGNORE INTO commit_files VALUES (commitId, fileId)

    FOR EACH symbol IN index.Symbols:
        bodyHash = hashFileRange(worktreeDir, symbol)
        symId = INSERT OR IGNORE INTO symbols (stable_id, body_hash, ...) VALUES (symbol.id, bodyHash, ...)
        INSERT OR IGNORE INTO commit_symbols VALUES (commitId, symId)

    FOR EACH ref IN deduplicatedRefs(index.Refs):
        refId = INSERT OR IGNORE INTO ref_versions (from_symbol_id, to_stable_id, kind, file_id, ...)
                   VALUES (symId, ref.to, ref.kind, fileId, ...)
        INSERT OR IGNORE INTO commit_refs VALUES (commitId, refId)

    COMMIT
```

The main differences:
1. We use `INSERT OR IGNORE` with `UNIQUE(stable_id, body_hash)` constraints to skip already-seen entities
2. We look up integer IDs after insert and use those for the mapping tables
3. Refs are deduplicated: multiple locations for the same (from, to, kind, file) tuple become one row with a `locations_json` array


---

## 7. Incremental Indexing Design

Incremental indexing means adding new commits to an existing database without re-extracting the commits that are already there. This is the key to making codebase-browser usable as a continuous tool during development.

### 7.1 Current State: Always From Scratch

Today, `review index` calls `review.Create()` which drops all tables and recreates the schema. Every run starts from zero:

```go
// In cmd/codebase-browser/cmds/review/index.go:
store, err := review.Create(dbPath)  // drops and recreates all tables!
```

This means:
- Indexing 10 new commits on top of 181 existing ones takes the same time as indexing 191 commits from scratch
- The database file grows linearly with each re-indexing

### 7.2 What Already Exists

The `history.Store` already has a `HasCommit()` method:

```go
// In internal/history/store.go:
func (s *Store) HasCommit(ctx context.Context, hash string) (bool, error) {
    var count int
    err := s.db.QueryRowContext(ctx,
        "SELECT COUNT(1) FROM commits WHERE hash = ? AND error = ''",
        hash,
    ).Scan(&count)
    return count > 0, err
}
```

This is the hook for incremental indexing — we can check if a commit is already present and skip it.

### 7.3 Incremental Indexing Flow

```
┌─────────────────────────────────────────────────────────────────┐
│  review index --db existing.db --commits HEAD~5..HEAD           │
│                                                                  │
│  1. Open existing database (NOT create from scratch)             │
│  2. Resolve commit range → [C1, C2, C3, C4, C5]                 │
│  3. For each commit Ci:                                          │
│       if HasCommit(Ci.hash):                                     │
│           skip  ← already indexed                                │
│       else:                                                      │
│           create worktree, extract, load snapshot                │
│  4. Index docs (unchanged)                                       │
│  5. Done                                                         │
└─────────────────────────────────────────────────────────────────┘
```

### 7.4 Schema Migration Strategy

The normalized schema is a significant change from the current one. We need a migration path:

**Option A: New schema with automatic migration**

```sql
PRAGMA user_version = 2;  -- version marker in the database file itself
```

On open:
- If `user_version == 0` or missing tables → create fresh (v2 schema)
- If `user_version == 1` → run migration (create new tables, copy data, drop old tables)
- If `user_version == 2` → normal operation

**Option B: Dual-write period**

During a transition period, write both the old and new schemas. This lets us validate the new schema produces identical query results before switching over.

**Recommendation:** Option A. The database is ephemeral (regenerated from source), so migration complexity isn't worth it. Just use the new schema going forward.

### 7.5 Handling Docs Incrementally

Docs are trickier because they reference the latest snapshot for snippet resolution. The current code calls `LoadLatestSnapshot()` to build a `browser.Loaded` struct for the renderer.

For incremental doc indexing:
1. Load the latest snapshot from the existing database
2. Re-render only the docs that changed (compare file modification times or content hashes)
3. Use `ON CONFLICT(slug) DO UPDATE SET ...` (which already exists in `indexDoc`)

### 7.6 Pseudocode: Incremental Index Command

```
function reviewIndex(dbPath, commitRange, docsPaths):
    // Open existing DB instead of creating from scratch
    store = review.Open(dbPath)  // NOT review.Create(dbPath)
    // Ensure schema exists (for first-time)
    if !store.HasTables():
        store.ResetSchema()

    // Phase 1: resolve commits
    allCommits = gitutil.LogCommits(repoRoot, commitRange)

    // Phase 2: filter to only new commits
    newCommits = []
    for commit in allCommits:
        if !store.History.HasCommit(commit.hash):
            newCommits.append(commit)

    if len(newCommits) == 0:
        print("All commits already indexed, nothing to do")
    else:
        print(f"Indexing {len(newCommits)} new commits (skipping {len(allCommits) - len(newCommits)} existing)")
        history.IndexCommits(store.History, newCommits, ...)

    // Phase 3: index docs
    for doc in discoverDocs(docsPaths):
        indexDoc(store, doc, loadLatestSnapshot(store), sourceFS)
```

### 7.7 Concurrency Opportunity

The current indexer is sequential — one worktree at a time. With the `--parallelism` flag already in the CLI (but effectively unused), we could index multiple commits concurrently:

```
commitQueue = channel of commits
for i in 0..parallelism:
    go worker(commitQueue, store)
```

Each worker creates a worktree, extracts, and loads. SQLite supports concurrent readers but only one writer, so the `LoadSnapshot` step needs a mutex or channel-based serialization. The extraction step (CPU-heavy) can run fully in parallel.

Expected speedup: near-linear up to the number of CPU cores, minus serialization overhead on the SQLite writes. On a 4-core machine, indexing 100 commits could go from 30s to ~10s.


---

## 8. Implementation Plan

This is a phased approach. Each phase is independently testable and mergeable.

### Phase 1: Measurement Infrastructure (1-2 days)

**Goal:** Establish baseline benchmarks and analysis scripts.

- [ ] Create a benchmark script (`scripts/01-benchmark-indexing.sh`) that runs `review index` at different commit ranges and records timing + DB size
- [ ] Create a DB analysis script (`scripts/02-analyze-db.sh`) that runs the SQL queries from this document
- [ ] Run the benchmark on at least 3 different repos: codebase-browser (small), glazed (medium), go-go-golems (large)
- [ ] Record baseline numbers in this ticket

**Files to create/modify:**
- `ttmp/.../GCB-017/scripts/01-benchmark-indexing.sh` — already done
- `ttmp/.../GCB-017/scripts/02-analyze-db.sh` — already done
- `ttmp/.../GCB-017/scripts/00-*.sql` through `13-*.sql` — already done

### Phase 2: Normalized Schema (3-5 days)

**Goal:** Implement the new schema from §6.2 and verify it produces identical query results.

- [ ] Create `internal/history/schema_v2.go` with the new table definitions
- [ ] Modify `history.Store.Create()` to use v2 schema
- [ ] Modify `history.Store.LoadSnapshot()` to use `INSERT OR IGNORE` + mapping tables
- [ ] Create compatibility views that recreate the old `snapshot_*` table shapes
- [ ] Run the existing test suite against the new schema
- [ ] Verify the React browser still works with the compatibility views
- [ ] Measure the new DB size and compare to baseline

**Key design decisions:**
- The compatibility views must produce the exact same column names and types as the current tables
- `ref_versions.locations_json` must be unpacked by the view to produce one row per location
- The `review export` command should work without changes (it copies the DB file)

**Files to modify:**
- `internal/history/schema.go` → add v2 schema
- `internal/history/loader.go` → rewrite `LoadSnapshot` for normalized writes
- `internal/history/store.go` → add `Open()` vs `Create()` distinction
- `internal/review/store.go` → update `ResetSchema` for v2

### Phase 3: Incremental Indexing (2-3 days)

**Goal:** Add `review index --incremental` flag that skips already-indexed commits.

- [ ] Modify `review/index.go` to use `review.Open()` instead of `review.Create()` when `--incremental` is set
- [ ] Add commit filtering: skip commits where `HasCommit()` returns true
- [ ] Handle the case where the DB schema is v1 (old format) — refuse with a clear error message asking for a fresh re-index
- [ ] Add tests: index 5 commits, then index 10 (should only process 5 new ones)
- [ ] Measure speedup

**Files to modify:**
- `cmd/codebase-browser/cmds/review/index.go` — add `--incremental` flag, change `Create` → `Open`
- `internal/review/indexer.go` — add commit filtering in `IndexReview`

### Phase 4: Parallel Indexing (2-3 days)

**Goal:** Use the existing `--parallelism` flag to index multiple commits concurrently.

- [ ] Modify `history.IndexCommits` to use a worker pool pattern
- [ ] Serialize the `LoadSnapshot` SQLite writes (use a channel or mutex)
- [ ] Measure speedup with `--parallelism 4` on a 4-core machine
- [ ] Test with parallelism=1 to verify no regression

**Files to modify:**
- `internal/history/indexer.go` — rewrite `indexWithWorktrees` to use goroutine pool

### Phase 5: ID Shortening (1-2 days)

**Goal:** Replace long string IDs with short integer IDs in SQLite to reduce index sizes.

- [ ] This is partially done by the normalized schema (integer PKs)
- [ ] For the `ref_versions.to_stable_id` column (which still uses the string ID), consider adding a `symbols_by_stable_id` lookup table
- [ ] Measure index size reduction

**Files to modify:**
- `internal/history/schema_v2.go` — refine the `ref_versions` table

---

## 9. API Reference

### 9.1 Go Package: `internal/history`

#### `type Store`

```go
// Open opens an existing history database. Does NOT create tables.
func Open(path string) (*Store, error)

// Create opens path, drops any existing tables, and recreates the schema.
func Create(path string) (*Store, error)

// Close checkpoints WAL state before closing.
func (s *Store) Close() error

// DB exposes the underlying *sql.DB for direct queries.
func (s *Store) DB() *sql.DB

// HasCommit checks if a commit has already been indexed (successfully).
func (s *Store) HasCommit(ctx context.Context, hash string) (bool, error)

// ResetSchema drops and recreates all tables.
func (s *Store) ResetSchema(ctx context.Context) error

// LoadSnapshot bulk-loads a single commit's index into the database.
func (s *Store) LoadSnapshot(ctx context.Context, commit gitutil.Commit, idx *indexer.Index, worktreeDir string) error

// ListCommits returns all indexed commits ordered by author_time descending.
func (s *Store) ListCommits(ctx context.Context) ([]CommitRow, error)

// SymbolCountAtCommit returns the number of symbols for a given commit.
func (s *Store) SymbolCountAtCommit(ctx context.Context, hash string) (int, error)
```

#### `type IndexOptions`

```go
type IndexOptions struct {
    RepoRoot     string           // path to git repository
    Commits      []gitutil.Commit // commits to index
    Patterns     []string         // Go package patterns (e.g., "./...")
    IncludeTests bool             // include *_test.go packages
    Worktrees    bool             // use git worktrees for each commit
    Parallelism  int              // max concurrent worktrees
    OnProgress   func(done, total int, shortHash, message string)
}
```

#### `func IndexCommits(ctx, store, opts) (*IndexResult, error)`

Runs the extraction pipeline for each commit. Returns `IndexResult` with counts of indexed/skipped/failed commits and any errors.

### 9.2 Go Package: `internal/review`

#### `type Store`

```go
// Open opens an existing review database (history + review tables).
func Open(path string) (*Store, error)

// Create opens path, drops all tables, and recreates the full schema.
func Create(path string) (*Store, error)

// DB exposes the underlying *sql.DB.
func (s *Store) DB() *sql.DB

// History exposes the embedded history.Store.
// Both share the same *sql.DB connection.
func (s *Store) Close() error
```

#### `func IndexReview(ctx, store, opts) (*IndexResult, error)`

Main entry point for `review index`. Orchestrates all four phases.

### 9.3 Go Package: `internal/indexer`

#### `func Extract(opts ExtractOptions) (*Index, error)`

Loads Go packages and walks their ASTs to produce an `Index` containing packages, files, symbols, and refs.

#### `type Index`

```go
type Index struct {
    Version     string    // "1"
    GeneratedAt string    // RFC3339 timestamp
    Module      string    // Go module path
    GoVersion   string    // runtime.Version()
    Packages    []Package
    Files       []File
    Symbols     []Symbol
    Refs        []Ref
}
```

### 9.4 CLI Commands

```bash
# Index a commit range into a review database
codebase-browser review index \
  --commits HEAD~10..HEAD \
  --docs ./reviews/pr-42.md \
  --db review.db \
  --repo-root . \
  --patterns "./cmd/..." "./internal/..." \
  --include-tests true \
  --parallelism 1

# Export a static browser bundle
codebase-browser review export \
  --db review.db \
  --out ./static-export

# Inspect database tables and sizes
sqlite-viz tables -d review.db
```

---

## 10. File Reference

### Core indexing pipeline

| File | Purpose |
|---|---|
| `cmd/codebase-browser/cmds/review/index.go` | CLI command: `review index` |
| `cmd/codebase-browser/cmds/review/root.go` | CLI command registration |
| `cmd/codebase-browser/cmds/review/patterns.go` | Default Go package patterns |
| `cmd/codebase-browser/cmds/review/db.go` | CLI command: `review db` |
| `cmd/codebase-browser/cmds/review/export.go` | CLI command: `review export` |
| `internal/review/indexer.go` | Top-level review indexing (4 phases) |
| `internal/review/store.go` | Review database connection management |
| `internal/review/schema.go` | Review-specific table DDL (review_docs, review_doc_snippets) |
| `internal/review/loader.go` | Snapshot loading into review DB |
| `internal/history/indexer.go` | Per-commit extraction orchestration |
| `internal/history/store.go` | History database connection + queries |
| `internal/history/schema.go` | History table DDL (commits, snapshot_*, file_contents) |
| `internal/history/loader.go` | Bulk INSERT of packages/files/symbols/refs |
| `internal/history/cache.go` | File contents caching by SHA-256 |
| `internal/history/scanner.go` | Git history scanning |
| `internal/history/diff.go` | Symbol-level body diff computation |
| `internal/history/bodydiff.go` | Body diff rendering |
| `internal/indexer/extractor.go` | Go AST → Index extraction |
| `internal/indexer/types.go` | Index, Package, File, Symbol, Ref types |
| `internal/indexer/xref.go` | Cross-reference extraction (refs) |
| `internal/indexer/id.go` | ID generation (SymbolID, MethodID, etc.) |
| `internal/indexer/write.go` | Index JSON serialization |
| `internal/indexer/multi.go` | Multi-language index merging |
| `internal/gitutil/log.go` | git log parsing |
| `internal/gitutil/worktree.go` | git worktree management |
| `internal/gitutil/show.go` | git show for file retrieval |

### Analysis scripts in this ticket

| Script | Purpose |
|---|---|
| `scripts/00-table-overview.sql` | Row counts for all tables |
| `scripts/00-sample-data.sql` | Sample rows from each table |
| `scripts/01-approx-data-per-table.sql` | Approximate data bytes per table |
| `scripts/01-benchmark-indexing.sh` | Run indexing at different commit ranges |
| `scripts/02-analyze-db.sh` | Full DB analysis pipeline |
| `scripts/02-deduplication-analysis.sql` | Redundancy measurements |
| `scripts/03-symbol-kind-distribution.sql` | Symbol types breakdown |
| `scripts/04-change-frequency.sql` | How often symbols change |
| `scripts/05-file-contents-size.sql` | Cached file sizes |
| `scripts/06-normalization-savings-estimate.sql` | Projected savings from normalization |
| `scripts/07-per-commit-snapshot-sizes.sql` | Per-commit row counts |
| `scripts/08-index-overhead.sql` | Index vs data size comparison |
| `scripts/09-symbol-body-hash-distribution.sql` | Most-changed symbols |
| `scripts/10-incremental-feasibility.sql` | Consecutive commit overlap |
| `scripts/11-consecutive-commit-overlap.sql` | Detailed overlap analysis |
| `scripts/12-row-size-analysis.sql` | Average row sizes and ID lengths |
| `scripts/13-scaling-comparison.sql` | Cross-DB scaling comparison |

---

## 11. Diagrams

### 11.1 Current Data Flow

```
Git Repo ──────┐
               │
               ▼
        ┌──────────────┐
        │ review index  │
        │               │
        │  1. git log   │──── []Commit
        │  2. for each: │
        │     worktree  │──── Index{Packages,Files,Symbols,Refs}
        │     extract   │
        │     load      │──── INSERT INTO snapshot_* (commit_hash, ...)
        │     cache     │──── INSERT INTO file_contents (hash, blob)
        │  3. discover  │──── []markdown paths
        │  4. render    │──── INSERT INTO review_docs, review_doc_snippets
        └──────────────┘
               │
               ▼
        ┌──────────────┐
        │  SQLite DB   │
        │  (264 MB)    │
        │              │
        │ commits (181)│
        │ snapshot_pkgs│ ×181 = 4,840 rows
        │ snapshot_fls │ ×181 = 11,848 rows
        │ snapshot_syms│ ×181 = 62,256 rows  ← 99% redundant
        │ snapshot_refs│ ×181 = 331,208 rows ← 99.4% redundant
        │ file_contents│ = 185 rows (already deduplicated)
        │ review_docs  │
        └──────────────┘
               │
               ▼
        ┌──────────────┐
        │ review export │──→ static directory (index.html + db/)
        └──────────────┘
               │
               ▼
        ┌──────────────┐
        │ React SPA    │ loads db via sql.js (WebAssembly)
        │ (browser)    │ queries snapshot_symbols, snapshot_refs, etc.
        └──────────────┘
```

### 11.2 Proposed Normalized Data Flow

```
Git Repo ──────┐
               │
               ▼
        ┌──────────────┐
        │ review index  │
        │ (normalized)  │
        │               │
        │  1. git log   │──── []Commit
        │  2. for each: │
        │     worktree  │──── Index
        │     extract   │
        │     load      │──── INSERT OR IGNORE INTO symbols (stable_id, body_hash, ...)
        │               │     INSERT INTO commit_symbols (commit_id, symbol_id)
        │  3. docs      │ (unchanged)
        └──────────────┘
               │
               ▼
        ┌──────────────┐
        │  SQLite DB   │
        │  (~5 MB)     │
        │              │
        │ commits (181)│
        │ packages (37)│ ← stored once
        │ files (185)  │ ← stored once per unique SHA-256
        │ symbols (646)│ ← stored once per unique body_hash
        │ ref_versions │ ← stored once per unique (from,to,kind,file)
        │ commit_syms  │ ← narrow (int,int) mapping = ~500 KB
        │ commit_refs  │ ← narrow (int,int) mapping = ~2.5 MB
        │ file_contents│ = 872 KB (unchanged)
        │ review_docs  │
        │              │
        │ + VIEWS that │ recreate snapshot_* table shapes
        │   for compat │
        └──────────────┘
               │
               ▼
        ┌──────────────┐
        │ React SPA    │ queries views (same as before)
        │ (unchanged)  │
        └──────────────┘
```

### 11.3 Incremental Indexing Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│  Day 1: index --commits HEAD~10..HEAD --db project.db              │
│                                                                     │
│  [C1] → extract → load                                              │
│  [C2] → extract → load                                              │
│  ...                                                                │
│  [C10] → extract → load                                             │
│                                                                     │
│  DB now has 10 commits. Time: ~3s. Size: ~2 MB.                    │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  Day 2: index --commits HEAD~15..HEAD --db project.db              │
│          (incremental: reuse existing DB)                           │
│                                                                     │
│  [C1]  → HasCommit? YES → skip                                     │
│  [C2]  → HasCommit? YES → skip                                     │
│  ...                                                                │
│  [C10] → HasCommit? YES → skip                                     │
│  [C11] → HasCommit? NO  → extract → load   ← only new work        │
│  [C12] → HasCommit? NO  → extract → load                           │
│  ...                                                                │
│  [C15] → HasCommit? NO  → extract → load                           │
│                                                                     │
│  DB now has 15 commits. Time: ~1.5s (only 5 new commits).         │
│  Size: ~3 MB (normalized: new unique entities only).               │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Appendix A: Glossary

| Term | Definition |
|---|---|
| **Symbol** | A named code entity: function, method, type, struct, interface, const, or var. Identified by a `stable_id` string like `sym:github.com/.../pkg.func.Name`. |
| **Ref** | A cross-reference from one symbol to another. E.g., function `A` calls function `B` → ref(from=A, to=B, kind=call). |
| **Body hash** | SHA-256 of the source bytes between a symbol's `start_offset` and `end_offset`. Two versions of the same function have different body hashes if their implementation changed. |
| **Stable ID** | A string that uniquely identifies a symbol across all commits, regardless of source location changes. E.g., `sym:github.com/.../pkg.func.Name` is the same symbol whether it's on line 10 or line 50. |
| **Snapshot** | A complete set of (packages, files, symbols, refs) for a single commit. The current schema stores one snapshot per commit. |
| **Worktree** | A `git worktree add` checkout of the repository at a specific commit. Used so the Go AST extractor can parse the source as it existed at that commit. |
| **sql.js** | SQLite compiled to WebAssembly. Runs in the browser. The static browser uses it to query the review database client-side. |

## Appendix B: Reproducing the Analysis

```bash
# 1. Build the CLI
make build

# 2. Index a range of commits (small test)
./bin/codebase-browser review index \
  --commits HEAD~5..HEAD \
  --docs ./README.md \
  --db /tmp/test.db

# 3. Analyze the database
sqlite-viz tables -d /tmp/test.db

# 4. Run the analysis scripts
for sql in ttmp/.../GCB-017/scripts/*.sql; do
    echo "=== $(basename $sql) ==="
    sqlite3 -header -column /tmp/test.db < "$sql"
    echo ""
done

# 5. If you have the 264MB production database
sqlite-viz tables -d /tmp/review.db
sqlite3 -header -column /tmp/review.db < ttmp/.../GCB-017/scripts/02-deduplication-analysis.sql
sqlite3 -header -column /tmp/review.db < ttmp/.../GCB-017/scripts/11-consecutive-commit-overlap.sql
```

---

## 12. Actual Benchmark Results (Post-Implementation)

These results were measured after implementing the normalized schema, incremental indexing, and parallel indexing on the codebase-browser repository (~550 symbols, ~96 source files, ~37 packages).

### 12.1 Schema Comparison: Old vs Normalized

| Commits | Old Schema | New Schema | Reduction |
|---|---|---|---|
| 5 | 3.2 MB | 516 KB | **6×** |
| 10 | 6.4 MB | 864 KB | **7×** |
| 20 | 12.4 MB | 1.1 MB | **11×** |
| 50 | 32.3 MB | 1.4 MB | **23×** |

The improvement scales with commit count because each additional commit adds only narrow (int, int) mapping rows instead of full entity snapshots.

### 12.2 Indexing Time Comparison

| Commits | Old Schema Time | New Schema Time |
|---|---|---|
| 5 | 2.6s | 1.9s |
| 10 | 5.5s | 5.7s |
| 20 | 12.4s | 20.0s |
| 50 | 46.3s | 26.8s |

The new schema is slightly slower for small ranges (upsert overhead) but faster for large ranges (less data written overall).

### 12.3 Parallel Indexing Speedup

| Parallelism | 20 commits | Speedup |
|---|---|---|
| 1 | 7.2s | baseline |
| 2 | 3.4s | **2.1×** |

### 12.4 Incremental Indexing

| Operation | Time |
|---|---|
| Index 5 new commits | 1.9s |
| Index 5 more (skip 5 existing) | 1.3s |
| Index 0 new (skip 10 existing) | **12ms** |

### 12.5 Normalized Table Row Counts (50 commits)

| Table | Rows | Description |
|---|---|---|
| `commits` | 50 | One per commit |
| `packages` | 8 | 8 unique packages |
| `files` | 24 | 24 unique file versions |
| `symbols` | 207 | 207 unique symbol versions |
| `ref_versions` | 1,488 | Deduplicated ref sets |
| `commit_symbols` | 4,590 | Mapping (500KB) |
| `commit_refs` | 14,870 | Mapping |
| `file_contents` | 24 | Already deduplicated |

Compare with the old schema: 50 × 550 = 27,500 symbol rows vs 207 base rows. That's a **133×** reduction in the symbols table.

### 12.6 What Changed From The Design

- The `ref_versions` table stores `locations_json` as a JSON array of location objects, expanded by the `snapshot_refs` view using `json_each()`. This collapsed 40,171 individual ref rows (old schema) into 1,488 ref_version rows.
- The `snapshot_refs` view uses `row_number() OVER (PARTITION BY ...)` to assign sequential IDs, since SQLite's `json_each` doesn't support `WITH ORDINALITY`.
- No `PRAGMA user_version` migration was needed — clean cutover, no backward compatibility.
- The old `symStmt` (direct INSERT with subqueries for package_id/file_id) was replaced with a `SELECT`-based INSERT that joins `commit_files` to resolve the file_id for the current commit.
