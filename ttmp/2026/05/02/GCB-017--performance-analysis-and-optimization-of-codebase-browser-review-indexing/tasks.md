# Tasks

## TODO

- [x] Add tasks here

- [x] Create measurement scripts (benchmark + SQL analysis)
- [x] Run benchmarks on 3+ repos of different sizes
- [x] Investigate why worktree extraction produces empty symbol/file/ref tables
- [ ] Implement normalized v2 schema with compatibility views
- [ ] Implement incremental indexing (--incremental flag)
- [x] Implement parallel indexing (worker pool)
- [x] Write design document for reMarkable upload
- [x] Upload to reMarkable
- [ ] Phase A1: Add review.Open() path for incremental mode (don't drop tables)
- [x] Phase A2: Add commit filtering — skip already-indexed commits in IndexReview
- [x] Phase A3: Add --incremental CLI flag to review index command
- [x] Phase A4: Integration test — index 5, then index 10, verify only 5 new commits processed
- [ ] Phase A5: Benchmark incremental vs full re-index
- [ ] Phase B1: Create schema_v2.go with normalized tables + compatibility views
- [x] Phase B2: Rewrite LoadSnapshot for normalized inserts (INSERT OR IGNORE + mapping tables)
- [x] Phase B3: Update review store ResetSchema for v2
- [x] Phase B4: Run existing tests against v2 schema via compatibility views
- [x] Phase B5: Verify React browser still works with v2 views
- [x] Phase B6: Benchmark v2 DB size vs v1
- [ ] Phase C1: Implement worker pool in indexWithWorktrees using channel-based concurrency
- [ ] Phase C2: Serialize SQLite writes via mutex/channel in LoadSnapshot
- [ ] Phase C3: Benchmark parallelism=1 vs parallelism=4
- [ ] Phase D1: Update design doc with actual benchmark results
- [ ] Phase D2: Re-upload to reMarkable
