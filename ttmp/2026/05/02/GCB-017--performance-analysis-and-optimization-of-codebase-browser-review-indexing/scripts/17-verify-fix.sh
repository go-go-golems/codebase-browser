#!/usr/bin/env bash
# 17-verify-fix.sh — Verify that multi-commit worktree extraction now works
# Must be run from the codebase-browser repo root after `make build`.
# Usage: bash scripts/17-verify-fix.sh
set -euo pipefail

DB="/tmp/gcb017-fixtest.db"
rm -f "$DB"

echo "Indexing 3 commits with worktrees..."
./bin/codebase-browser review index \
    --commits "HEAD~3..HEAD" \
    --docs ./README.md \
    --db "$DB" \
    --repo-root . \
    --patterns "./cmd/..." "./internal/..." 2>&1

echo ""
echo "=== Table counts ==="
sqlite3 "$DB" "
SELECT 'commits' as tbl, COUNT(*) as cnt FROM commits
UNION ALL SELECT 'snapshot_packages', COUNT(*) FROM snapshot_packages
UNION ALL SELECT 'snapshot_files', COUNT(*) FROM snapshot_files
UNION ALL SELECT 'snapshot_symbols', COUNT(*) FROM snapshot_symbols
UNION ALL SELECT 'snapshot_refs', COUNT(*) FROM snapshot_refs
UNION ALL SELECT 'file_contents', COUNT(*) FROM file_contents;
"

echo ""
echo "=== Sample packages (should have real import paths, not patterns) ==="
sqlite3 "$DB" "SELECT id, import_path, name FROM snapshot_packages LIMIT 3;"

echo ""
SYMS=$(sqlite3 "$DB" "SELECT COUNT(*) FROM snapshot_symbols;")
REFS=$(sqlite3 "$DB" "SELECT COUNT(*) FROM snapshot_refs;")
if [ "$SYMS" -gt 0 ] && [ "$REFS" -gt 0 ]; then
    echo "PASS: symbols=$SYMS refs=$REFS (both non-zero)"
else
    echo "FAIL: symbols=$SYMS refs=$REFS (expected both non-zero)"
    exit 1
fi
