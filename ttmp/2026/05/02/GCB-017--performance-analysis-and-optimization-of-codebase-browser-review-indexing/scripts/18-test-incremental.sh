#!/usr/bin/env bash
# 18-test-incremental.sh — Test incremental indexing: index 5, then 10, then 10 again
# Must be run from repo root after `make build`.
set -euo pipefail

DB="/tmp/gcb017-incr-test.db"
rm -f "$DB"

echo "=== Run 1: index 5 commits ==="
./bin/codebase-browser review index \
  --commits "HEAD~5..HEAD" --docs ./README.md --db "$DB" \
  --repo-root . --patterns "./cmd/..." "./internal/..." --incremental 2>&1 | tail -3
C1=$(sqlite3 "$DB" "SELECT COUNT(*) FROM commits;")
S1=$(sqlite3 "$DB" "SELECT COUNT(*) FROM snapshot_symbols;")

echo "=== Run 2: index 10 commits (5 new, 5 existing) ==="
./bin/codebase-browser review index \
  --commits "HEAD~10..HEAD" --docs ./README.md --db "$DB" \
  --repo-root . --patterns "./cmd/..." "./internal/..." --incremental 2>&1 | tail -3
C2=$(sqlite3 "$DB" "SELECT COUNT(*) FROM commits;")
S2=$(sqlite3 "$DB" "SELECT COUNT(*) FROM snapshot_symbols;")

echo "=== Run 3: same 10 commits (all existing) ==="
./bin/codebase-browser review index \
  --commits "HEAD~10..HEAD" --docs ./README.md --db "$DB" \
  --repo-root . --patterns "./cmd/..." "./internal/..." --incremental 2>&1 | tail -3
C3=$(sqlite3 "$DB" "SELECT COUNT(*) FROM commits;")

echo ""
echo "Results:"
echo "  Run 1: commits=$C1 symbols=$S1"
echo "  Run 2: commits=$C2 symbols=$S2"
echo "  Run 3: commits=$C3 (should be same as run 2)"

if [ "$C1" -eq 5 ] && [ "$C2" -eq 10 ] && [ "$C3" -eq 10 ]; then
    echo "PASS: incremental indexing works correctly"
else
    echo "FAIL: expected 5,10,10 got $C1,$C2,$C3"
    exit 1
fi
