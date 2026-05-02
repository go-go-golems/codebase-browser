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

- [ ] E1: Replace `LastInsertId()` doc upsert path with `RETURNING id` or unconditional slug lookup.
- [ ] E2: Make `indexDoc` return snippet counts and increment `IndexResult.SnippetsIndexed`.
- [ ] E3: Make `--docs-only` skip git commit-range resolution and commit filtering entirely.
- [ ] E4: Make `--docs-only` require an existing review DB instead of using `OpenOrCreate`.
- [ ] E5: Return non-zero from CLI commands when `IndexReview` accumulates errors.
- [ ] E6: Add tests for docs-only single-doc update, multi-doc update, stale snippet deletion, snippet counts, missing DB, and empty DB failure.
- [ ] E7: Render snippets from DB-backed snapshot content instead of the live working tree.
- [ ] E8: Fix `--patterns` behavior/docs mismatch by switching to `StringSliceVar` or documenting repeated flags only.
- [ ] E9: Fix worker-pool worktree cleanup and per-commit failure behavior.
- [ ] E10: Run gofmt, tests, and clean accidental artifacts.
