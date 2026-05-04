# Changelog

## 2026-05-03

- Initial workspace created


## 2026-05-03

- Added static-browser provider caching for `listCommits()`, `resolveCommitRef(...)`, and `getCommit(...)` to avoid repeated immutable commit metadata queries in large exports.
- Added Worker request timeout/reset behavior through `workerClient.ts`, with pending-request rejection and Worker recreation on the next query.
- Added a frontend regression test that prevents `snapshot_refs` from re-entering `sqlJsQueryProvider.ts` hot paths.
- Added a reusable CDP browser smoke script for source-page validation.
- Rebuilt embedded SPA assets and exported the full corporate Glazed static site to `/tmp/glazed-full-export-gcb024`, served on port 4186.
