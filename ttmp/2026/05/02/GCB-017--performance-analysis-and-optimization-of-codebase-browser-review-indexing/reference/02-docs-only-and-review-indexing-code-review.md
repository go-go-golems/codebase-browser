---
Title: "Docs-only and Review Indexing Code Review"
Ticket: "GCB-017"
Status: "active"
Topics:
  - review-indexing
  - docs-only
  - sqlite
  - code-review
DocType: "review"
Intent: "Audit the docs-only implementation and related review indexing changes after the glazed export exercise."
Owners: []
RelatedFiles:
  - Path: cmd/codebase-browser/cmds/review/index.go
    Note: review index CLI flags and result handling
  - Path: cmd/codebase-browser/cmds/review/db.go
    Note: review db create CLI and error handling
  - Path: internal/review/indexer.go
    Note: IndexReview orchestration, docs-only mode, markdown doc upsert
  - Path: internal/review/store.go
    Note: Open/Create/OpenOrCreate behavior for existing review DBs
  - Path: internal/history/indexer.go
    Note: worker pool cleanup and failed-commit behavior
---

# Docs-only and Review Indexing Code Review

## Purpose

This review records the handoff assessment after adding `--docs-only` during the glazed static export work. The feature direction is correct: editing markdown review guides should not require re-extracting 252 commits. However, the debugging path exposed several correctness, reliability, and documentation issues that should be fixed before treating the implementation as production quality.

## Executive summary

The right product behavior is:

```text
existing review DB + edited markdown docs -> updated review_docs/review_doc_snippets
```

No commit extraction should run. Ideally, docs-only mode should not even need to resolve a git range. The existing implementation gets close, but it still has sharp edges:

1. `LastInsertId()` was used after `INSERT ... ON CONFLICT DO UPDATE`, which is unsafe for SQLite upserts.
2. `--docs-only` still resolves the git commit range before skipping extraction.
3. `--docs-only` opens with `OpenOrCreate`, even though it needs an existing DB with commits.
4. Snippet counts are not incremented, so the CLI reports `0 snippets` even when snippets were written.
5. The CLI prints per-doc/per-commit errors but returns success.
6. Snippet rendering reads source from the live checkout, while offsets come from the indexed DB snapshot.
7. `--patterns` docs were updated with a comma example that does not match `StringArrayVar` behavior.
8. The worktree worker pool keeps worktree cleanup deferred until worker exit and returns from a worker on individual commit failures.
9. There are untracked artifacts and some Go files need `gofmt`.

## What went well

### The feature idea was correct

The original user question was whether editing a review markdown file requires full commit re-indexing. It should not. Adding a docs-only mode is the right boundary: commit/symbol history is immutable for a given DB; review documents are another layer that can be updated quickly.

### The FK investigation eventually found the real symptom

The decisive debug line was:

```text
DBG indexDoc: slug="help-browser-build" docID=32
DBG insert snippet FAILED: docID=32 ... err=FOREIGN KEY constraint failed
```

The DB row was actually:

```sql
SELECT id, slug FROM review_docs WHERE slug = 'help-browser-build';
-- 2|help-browser-build
```

So the snippet insert failed because it used a bogus `doc_id` that did not reference any `review_docs.id`.

### The transactional shape is right

Each document update should be atomic:

1. Upsert the doc row.
2. Delete stale snippets for that doc.
3. Insert the new snippets.
4. Commit.

If any snippet insert fails, the previous doc/snippet state should remain intact.

## Findings and cleanup sketches

### 1. Do not use `LastInsertId()` after SQLite upserts

Problem: `LastInsertId()` is not a reliable way to retrieve the row affected by `INSERT ... ON CONFLICT DO UPDATE`. The observed run returned `32`, but the real row was `2`. The temporary fix checked whether the returned ID exists, but that is still unsafe if the stale ID happens to exist for another doc.

Where to look: `internal/review/indexer.go:indexDoc`

Cleanup sketch:

```go
var docID int64
err := tx.QueryRowContext(ctx, `
    INSERT INTO review_docs (slug, title, path, content, frontmatter_json, indexed_at)
    VALUES (?, ?, ?, ?, ?, ?)
    ON CONFLICT(slug) DO UPDATE SET
        title = excluded.title,
        path = excluded.path,
        content = excluded.content,
        frontmatter_json = excluded.frontmatter_json,
        indexed_at = excluded.indexed_at
    RETURNING id
`, slug, page.Title, path, string(data), frontmatter, now).Scan(&docID)
```

If `RETURNING` compatibility is a concern, always select by slug after the upsert. Never trust `LastInsertId()` for this path.

### 2. Docs-only should skip git range resolution

Problem: `IndexReview()` still calls `gitutil.LogCommits()` before checking `DocsOnly`. Updating docs should not require a valid git range or even a live repo history.

Where to look: `internal/review/indexer.go:IndexReview`

Cleanup sketch:

```go
if !opts.DocsOnly {
    commits, err := gitutil.LogCommits(...)
    // incremental filtering and commit extraction
}

// Always allowed if !SkipDocs:
loaded, err := LoadLatestSnapshot(ctx, store)
render docs
```

### 3. Docs-only should require an existing DB

Problem: `cmd review index --docs-only` uses `OpenOrCreate`. That can create an empty DB, which then fails later with `no commits in review database`.

Where to look: `cmd/codebase-browser/cmds/review/index.go`

Cleanup sketch:

```go
switch {
case docsOnly:
    store, err = review.Open(dbPath)
case incremental:
    store, err = review.OpenOrCreate(dbPath)
default:
    store, err = review.Create(dbPath)
}
```

Then fail clearly if no commits exist.

### 4. Count snippets correctly

Problem: `IndexResult.SnippetsIndexed` exists but is never incremented. CLI output says `0 snippets` even when snippets are present in the DB.

Where to look: `internal/review/indexer.go:indexDoc` and the doc indexing loop.

Cleanup sketch:

```go
count, err := indexDoc(...)
if err == nil {
    result.DocsIndexed++
    result.SnippetsIndexed += count
}
```

### 5. Return non-zero when indexing errors occur

Problem: The CLI prints `ERROR ...` lines and still returns nil. CI and scripts will treat a failed doc render as success.

Where to look: `cmd/codebase-browser/cmds/review/index.go`, `cmd/codebase-browser/cmds/review/db.go`

Cleanup sketch:

```go
if len(result.Errors) > 0 {
    for _, idxErr := range result.Errors { ... }
    return fmt.Errorf("index completed with %d error(s)", len(result.Errors))
}
```

### 6. Read snippet source from the indexed DB, not the live checkout

Problem: `docs.Render()` receives `os.DirFS(opts.RepoRoot)`, then slices files using offsets from the latest indexed snapshot. If the working tree moved since indexing, byte offsets can point into the wrong file.

Where to look: `internal/review/indexer.go`, `internal/docs/renderer.go`

Cleanup sketch:

```go
type snapshotFS struct { db *sql.DB; commitHash string }
func (s snapshotFS) Open(name string) (fs.File, error) {
    SELECT fc.content
    FROM snapshot_files f
    JOIN file_contents fc ON fc.content_hash = f.content_hash
    WHERE f.commit_hash = ? AND f.path = ?
}
```

Use this FS for review rendering so snippet bytes and symbol offsets come from the same snapshot.

### 7. Fix `--patterns` documentation or flag type

Problem: The docs mention `--patterns ./...,./pkg/...`, but Cobra `StringArrayVar` does not split comma-separated values.

Where to look: `cmd/codebase-browser/cmds/review/index.go`, `cmd/codebase-browser/cmds/review/db.go`, docs.

Cleanup options:

- Change flags to `StringSliceVar`, which supports commas and repeated flags.
- Or update docs to use repeated flags only.

Prefer `StringSliceVar` because it is friendlier.

### 8. Fix worker-pool cleanup and error handling

Problem: `defer RemoveWorktree` is inside a worker loop, so worktrees live until the worker exits. Also, per-commit failures use `return`, stopping the worker and potentially abandoning queued jobs.

Where to look: `internal/history/indexer.go:indexWithWorktrees`

Cleanup sketch:

```go
for job := range jobs {
    func() {
        wt, err := CreateWorktree(...)
        if err != nil { record; return }
        defer RemoveWorktree(...)
        ...
    }()
}
```

Use `continue` behavior for one failed commit rather than terminating the whole worker.

### 9. Hygiene: gofmt and artifacts

Problem: `gofmt -l` reports unformatted files, and `git status` shows untracked artifacts (`.git-worktrees/`, a binary, screenshot, `lefthook.yml`).

Cleanup sketch:

```bash
gofmt -w internal/review/indexer.go internal/history/indexer.go internal/review/loader.go
rm -rf .git-worktrees codebase-browser glazed-review-screenshot.png
# decide whether lefthook.yml is intended, otherwise remove or ignore
```

## Recommended implementation order

1. Fix doc upsert ID lookup (`RETURNING id`) and snippet counts.
2. Make docs-only skip git and require an existing DB.
3. Make CLI return non-zero on accumulated errors.
4. Add tests for docs-only single doc, multiple docs, stale snippet deletion, and missing DB/empty DB failure.
5. Fix snippet source to read from DB snapshot.
6. Fix `--patterns` flag/docs.
7. Fix worker pool cleanup/error handling.
8. Run `gofmt`, tests, and clean artifacts.
