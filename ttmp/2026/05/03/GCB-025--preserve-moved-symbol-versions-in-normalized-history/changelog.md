# Changelog

## 2026-05-03

- Initial workspace created


## 2026-05-03

- Diagnosed the Glazed `TypeChoice` history/body-diff failure as normalized symbol-version identity corruption, not a frontend query failure.
- Changed symbol version identity from `stable_id + body_hash` to `stable_id + body_hash + file_id + start_offset + end_offset`.
- Updated loader conflict/lookup logic to use the same symbol version identity.
- Added a regression test for an unchanged stable symbol moving from one file to another.
- Updated the DB reference documentation to describe the new symbol identity.
- Ran UI typecheck/tests and Go tests successfully.
- Built the standalone binary with embedded SPA assets.
- Ran a narrow Glazed range indexing experiment around the `parameter-type.go -> field-type.go` rename; it did not reproduce the exact full-history UI path, so the durable validation is the unit regression test. A full or larger Glazed reindex is still required before the previously exported full Glazed site can be corrected.

## 2026-05-03 Full Glazed reindex/export validation

- Full Glazed reindex with schema v3 completed in 11m34.61s at parallelism 5.
- New DB: `glazed-full-gcb025.db`, 226M, 1577 commits, 32263 symbol versions, 2783 file versions.
- Post-index integrity checks passed: `bad_symbol_file_joins = 0`, `typechoice_bad_joins = 0`.
- Exported static site to `/tmp/glazed-full-export-gcb025`, served on `http://127.0.0.1:4187/`.
- Playwright validation of the previous failing `TypeChoice` history URL passed: no `Failed to load body diff`, body diff request completed in 3-7 ms after DB load.
