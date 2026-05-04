#!/usr/bin/env bash
# 01-benchmark-indexing.sh — Measure review indexing performance at different scales
# Usage: bash scripts/01-benchmark-indexing.sh [REPO_ROOT]
set -euo pipefail

REPO="${1:-.}"
CLI="./bin/codebase-browser"
RESULTS_DIR="/tmp/gcb017-bench-results"
mkdir -p "$RESULTS_DIR"

echo "=== codebase-browser indexing benchmark ==="
echo "Repo: $REPO"
echo "Date: $(date -Iseconds)"
echo ""

build_cli() {
    echo "Building CLI..."
    make build 2>&1 | tail -1
}

run_bench() {
    local label="$1"
    local commit_range="$2"
    local db_path="$RESULTS_DIR/${label}.db"
    shift 2

    echo ""
    echo "--- Benchmark: $label (commits=$commit_range) ---"
    rm -f "$db_path"

    local start_ns end_ns duration_ms
    start_ns=$(date +%s%N)

    "$CLI" review index \
        --commits "$commit_range" \
        --docs ./README.md \
        --db "$db_path" \
        --repo-root "$REPO" \
        --patterns "./cmd/..." "./internal/..." 2>&1 | tail -5

    end_ns=$(date +%s%N)
    duration_ms=$(( (end_ns - start_ns) / 1000000 ))

    local db_size
    db_size=$(stat --printf="%s" "$db_path" 2>/dev/null || stat -f%z "$db_path" 2>/dev/null)

    echo "Duration: ${duration_ms}ms"
    echo "DB size: ${db_size} bytes"
    echo ""

    # Table stats
    echo "Table stats:"
    sqlite3 "$db_path" "
    SELECT tbl, cnt, ROUND(size_kb, 1) as size_kb FROM (
        SELECT 'commits' as tbl, COUNT(*) as cnt, 0 as size_kb FROM commits
        UNION ALL SELECT 'packages', COUNT(*), 0 FROM snapshot_packages
        UNION ALL SELECT 'files', COUNT(*), 0 FROM snapshot_files
        UNION ALL SELECT 'symbols', COUNT(*), 0 FROM snapshot_symbols
        UNION ALL SELECT 'refs', COUNT(*), 0 FROM snapshot_refs
        UNION ALL SELECT 'file_contents', COUNT(*), 0 FROM file_contents
        UNION ALL SELECT 'review_docs', COUNT(*), 0 FROM review_docs
    );
    " 2>/dev/null

    echo "$label: ${duration_ms}ms, ${db_size} bytes" >> "$RESULTS_DIR/summary.txt"
}

build_cli

# Benchmark different commit ranges
run_bench "single-commit" "HEAD"
run_bench "5-commits" "HEAD~5..HEAD"
run_bench "10-commits" "HEAD~10..HEAD"
run_bench "20-commits" "HEAD~20..HEAD"
run_bench "50-commits" "HEAD~50..HEAD"

echo ""
echo "=== Summary ==="
cat "$RESULTS_DIR/summary.txt"
