# Changelog

## 2026-05-03

- Initial workspace created


## 2026-05-03

- Added `review db validate` as a real `review db validate` subcommand under a `review db` command group.
- Added `internal/review.ValidateIntegrity`, checking commit-local symbol/file and ref/file/from-symbol joins in the snapshot views.
- Added validator tests for clean DBs and broken symbol/file joins.
- Added schema version fields to `manifest.json` under `db.historySchemaVersion`, `db.reviewSchemaVersion`, and `db.schemaVersions`.
- Updated frontend manifest typing for schema versions.
- Promoted the CDP source-page smoke script to `scripts/review-browser-smoke.py` and added `make review-browser-smoke URL=...`.
- Updated user and DB reference docs with validation and schema v3 guidance.
- Validated the fixed full Glazed DB with `review db validate`, exported `/tmp/glazed-full-export-gcb026`, and smoke-tested the source page on port 4188.
