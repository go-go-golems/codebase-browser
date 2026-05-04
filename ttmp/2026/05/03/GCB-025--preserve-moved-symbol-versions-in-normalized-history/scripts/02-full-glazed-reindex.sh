#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="/home/manuel/code/wesen/corporate-headquarters/codebase-browser"
GLAZED_ROOT="/home/manuel/code/wesen/corporate-headquarters/glazed"
DB="$REPO_ROOT/glazed-full-gcb025.db"
DOCS="/tmp/glazed-full-reviews"
LOG="$REPO_ROOT/ttmp/2026/05/03/GCB-025--preserve-moved-symbol-versions-in-normalized-history/scripts/02-full-glazed-reindex.log"

cd "$REPO_ROOT"
rm -f "$DB" "$DB-shm" "$DB-wal"

echo "GCB-025 full Glazed reindex started at $(date -Is)" | tee "$LOG"
echo "repo=$GLAZED_ROOT" | tee -a "$LOG"
echo "db=$DB" | tee -a "$LOG"
echo "parallelism=5" | tee -a "$LOG"
echo "binary=$(./bin/codebase-browser --version 2>/dev/null || true)" | tee -a "$LOG"

/usr/bin/time -f 'GCB025_FULL_INDEX elapsed=%E maxrss=%MKB' \
  ./bin/codebase-browser review index \
    --repo-root "$GLAZED_ROOT" \
    --db "$DB" \
    --commits 94461071372daeebf7ba025132b149d24962e544..HEAD \
    --docs "$DOCS" \
    --parallelism 5 \
    --patterns ./... \
    --strict-docs \
  2>&1 | tee -a "$LOG"

sqlite3 "$DB" <<'SQL' | tee -a "$LOG"
.headers on
.mode column
SELECT COUNT(*) AS commits FROM commits;
SELECT COUNT(*) AS symbols FROM symbols;
SELECT COUNT(*) AS files FROM files;
SELECT key, value FROM schema_info ORDER BY key;
SELECT COUNT(*) AS bad_symbol_file_joins
FROM snapshot_symbols s
LEFT JOIN snapshot_files f ON f.commit_hash = s.commit_hash AND f.id = s.file_id
WHERE f.id IS NULL;
SELECT COUNT(*) AS typechoice_bad_joins
FROM snapshot_symbols s
LEFT JOIN snapshot_files f ON f.commit_hash = s.commit_hash AND f.id = s.file_id
WHERE s.id = 'sym:github.com/go-go-golems/glazed/pkg/cmds/fields.const.TypeChoice'
  AND f.id IS NULL;
SQL

echo "GCB-025 full Glazed reindex finished at $(date -Is)" | tee -a "$LOG"
