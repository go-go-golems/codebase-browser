# Tasks

## TODO

- [x] Add tasks here

- [x] Create measurement scripts (benchmark + SQL analysis)
- [x] Run benchmarks on 3+ repos of different sizes
- [x] Investigate why worktree extraction produces empty symbol/file/ref tables
- [x] Implement normalized v2 schema with compatibility views
- [x] Implement incremental indexing (--incremental flag)
- [x] Implement parallel indexing (worker pool)
- [x] Write design document for reMarkable upload
- [x] Upload to reMarkable
- [x] Phase A1: Add review.Open() path for incremental mode (don't drop tables)
- [x] Phase A2: Add commit filtering — skip already-indexed commits in IndexReview
- [x] Phase A3: Add --incremental CLI flag to review index command
- [x] Phase A4: Integration test — index 5, then index 10, verify only 5 new commits processed
- [x] Phase A5: Benchmark incremental vs full re-index
- [x] Phase B1: Create schema_v2.go with normalized tables + compatibility views
- [x] Phase B2: Rewrite LoadSnapshot for normalized inserts (INSERT OR IGNORE + mapping tables)
- [x] Phase B3: Update review store ResetSchema for v2
- [x] Phase B4: Run existing tests against v2 schema via compatibility views
- [x] Phase B5: Verify React browser still works with v2 views
- [x] Phase B6: Benchmark v2 DB size vs v1
- [x] Phase C1: Implement worker pool in indexWithWorktrees using channel-based concurrency
- [x] Phase C2: Serialize SQLite writes via mutex/channel in LoadSnapshot
- [x] Phase C3: Benchmark parallelism=1 vs parallelism=4
- [x] Phase D1: Update design doc with actual benchmark results
- [x] Phase D2: Re-upload to reMarkable

## Docs-only follow-up cleanup (from review 02)

- [x] E1: Replace `LastInsertId()` doc upsert path with `RETURNING id` or unconditional slug lookup.
- [x] E2: Make `indexDoc` return snippet counts and increment `IndexResult.SnippetsIndexed`.
- [x] E3: Make `--docs-only` skip git commit-range resolution and commit filtering entirely.
- [x] E4: Make `--docs-only` require an existing review DB instead of using `OpenOrCreate`.
- [x] E5: Return non-zero from CLI commands when `IndexReview` accumulates errors.
- [x] E6: Add tests for docs-only single-doc update, multi-doc update, stale snippet deletion, snippet counts, missing DB, and empty DB failure.
- [x] E7: Render snippets from DB-backed snapshot content instead of the live working tree.
- [x] E8: Fix `--patterns` behavior/docs mismatch by switching to `StringSliceVar` or documenting repeated flags only.
- [x] E9: Fix worker-pool worktree cleanup and per-commit failure behavior.
- [x] E10: Run gofmt, tests, and clean accidental artifacts.

## Full architecture review follow-up tasks (from review 03)

- [ ] F1: Make `internal/staticapp/reviewdocs.go` use DB-backed snapshot content instead of `os.DirFS(repoRoot)`.
- [ ] F2: Add direct `browser.LoadIndex(*indexer.Index)` API and remove JSON roundtrip in `LoadLatestSnapshot`.
- [ ] F3: Decide whether `files.content_hash` should be removed or populated with `sha256`; update views/docs accordingly.
- [ ] F4: Add a schema version table inside the SQLite DB.
- [ ] F5: Add browser-query benchmarks for `snapshot_refs` expansion under sql.js-like workloads.
- [ ] F6: Refactor normalized loader upsert helpers to reduce hand-counted SQL argument lists.
- [ ] F7: Split file-content caching into read/hash outside the SQLite writer lock and insert inside a short critical section.
- [ ] F8: Add `--strict-docs` to fail on unresolved `codebase-*` directive errors.
- [ ] F9: Clarify, rename, or replace `review export --include-source` for external repos.
- [ ] F10: Revisit default package patterns; consider `./...`, warnings, or an `--all-packages` shortcut.

## Priority cleanup tasks requested after review (clarity over compatibility)

### 1.x Address now / before considering GCB-017 done

- [ ] P1.1: Make static export review-doc rendering use the same DB-backed snapshot source as `review index` (no live checkout reads in `internal/staticapp/reviewdocs.go`).
- [ ] P1.2: Remove the ambiguous `files.content_hash` column and `snapshot_files.content_hash` projection; use `files.sha256` as the single content key everywhere.
- [ ] P1.3: Replace `author_time`-based latest snapshot selection with an explicit indexed sequence/range-order column, and use one helper for latest commit lookup.
- [ ] P1.4: Remove or rename `review export --include-source`; prefer deleting the misleading external-repo behavior over preserving compatibility.

### 2.x High-value soon

- [ ] P2.1: Add `browser.LoadIndex(*indexer.Index)` and remove the JSON marshal/unmarshal roundtrip in `review.LoadLatestSnapshot`.
- [ ] P2.2: Add `--strict-docs` to fail on unresolved `codebase-*` directive errors for CI/export reliability.
- [ ] P2.3: Refactor normalized loader upserts to reduce duplicated insert-or-lookup code and hand-counted SQL arguments.
- [ ] P2.4: Add a simple schema metadata/version table for clarity, not legacy migration compatibility.
