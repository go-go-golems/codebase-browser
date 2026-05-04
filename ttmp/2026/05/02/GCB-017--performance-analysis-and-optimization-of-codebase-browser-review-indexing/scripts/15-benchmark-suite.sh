#!/usr/bin/env bash
# 15-benchmark-suite.sh — Run indexing benchmarks at different commit ranges
# Must be run from the codebase-browser repo root after `make build`.
# Usage: bash scripts/15-benchmark-suite.sh
set -euo pipefail

RESULTS="/tmp/gcb017-bench-results"
mkdir -p "$RESULTS"
> "$RESULTS/summary.txt"

run_bench() {
    local label="$1"
    local commit_range="$2"
    local db_path="$RESULTS/${label}.db"
    shift 2

    rm -f "$db_path"
    local start_ns end_ns duration_ms db_size
    start_ns=$(date +%s%N)

    ./bin/codebase-browser review index \
        --commits "$commit_range" \
        --docs ./README.md \
        --db "$db_path" \
        --repo-root . \
        --patterns "./cmd/..." "./internal/..." 2>&1 | grep "Done in"

    end_ns=$(date +%s%N)
    duration_ms=$(( (end_ns - start_ns) / 1000000 ))
    db_size=$(stat --printf="%s" "$db_path" 2>/dev/null || stat -f%z "$db_path")

    local commits pkgs files syms refs
    commits=$(sqlite3 "$db_path" "SELECT COUNT(*) FROM commits;")
    pkgs=$(sqlite3 "$db_path" "SELECT COUNT(*) FROM snapshot_packages;")
    files=$(sqlite3 "$db_path" "SELECT COUNT(*) FROM snapshot_files;")
    syms=$(sqlite3 "$db_path" "SELECT COUNT(*) FROM snapshot_symbols;")
    refs=$(sqlite3 "$db_path" "SELECT COUNT(*) FROM snapshot_refs;")

    printf "%-20s %8dms | %s bytes | commits=%s pkgs=%s files=%s syms=%s refs=%s\n" \
        "$label" "$duration_ms" "$db_size" "$commits" "$pkgs" "$files" "$syms" "$refs" \
        | tee -a "$RESULTS/summary.txt"
}

run_bench "02-5-commits" "HEAD~5..HEAD"
run_bench "03-10-commits" "HEAD~10..HEAD"
run_bench "04-20-commits" "HEAD~20..HEAD"
run_bench "05-50-commits" "HEAD~50..HEAD"

echo ""
echo "=== Full Summary ==="
cat "$RESULTS/summary.txt"
