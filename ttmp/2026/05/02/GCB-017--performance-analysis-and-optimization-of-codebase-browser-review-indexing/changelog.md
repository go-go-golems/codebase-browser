# Changelog

## 2026-05-02

- Initial workspace created


## 2026-05-02

Created GCB-017 ticket, diary, and design document. Analyzed 264MB production database. Found 99%+ redundancy in snapshot tables. Designed normalized schema projected to reduce DB from 264MB to ~5MB. Saved 14 SQL analysis scripts and 2 shell scripts.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ttmp/2026/05/02/GCB-017--performance-analysis-and-optimization-of-codebase-browser-review-indexing/design/01-performance-analysis-and-design-guide-for-review-indexing.md — Full design document (1258 lines)
- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/ttmp/2026/05/02/GCB-017--performance-analysis-and-optimization-of-codebase-browser-review-indexing/reference/01-investigation-diary.md — Investigation diary with Step 1 and Step 2


## 2026-05-02

Uploaded bundled design doc + diary to reMarkable at /ai/2026/05/02/GCB-017


## 2026-05-02

Root-caused worktree extraction bug: packages.Config needs GOWORK=off when parent go.work exists. One-line fix in internal/indexer/extractor.go.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/codebase-browser/internal/indexer/extractor.go — packages.Config needs GOWORK=off env var for worktree extraction


## 2026-05-02

Phase A (incremental indexing) complete: --incremental flag added, tested with 5+5+0 pattern, 12ms skip for all-cached run


## 2026-05-02

Phase B complete: Normalized schema implemented. 50 commits: 32.3MB -> 1.4MB (23x smaller). All tests pass. Views recreate old table shapes for browser compatibility.


## 2026-05-02

All phases complete. Design doc updated with actual results, re-uploaded to reMarkable.

