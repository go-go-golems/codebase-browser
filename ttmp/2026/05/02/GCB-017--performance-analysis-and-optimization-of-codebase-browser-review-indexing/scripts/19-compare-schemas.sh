#!/usr/bin/env bash
# 19-compare-schemas.sh — Benchmark old vs new schema (requires pre-existing old schema DBs)
# Run from repo root after `make build`.
set -euo pipefail

echo "=== Normalized Schema Benchmarks ==="
for n in 5 10 20 50; do
    rm -f /tmp/gcb017-v2-bench-${n}.db
    start=$(date +%s%N)
    ./bin/codebase-browser review index \
        --commits "HEAD~${n}..HEAD" \
        --docs ./README.md \
        --db /tmp/gcb017-v2-bench-${n}.db \
        --repo-root . \
        --patterns "./cmd/..." "./internal/..." 2>&1 | grep "Done in"
    end=$(date +%s%N)
    ms=$(( (end - start) / 1000000 ))
    echo "  ${n} commits: ${ms}ms, $(du -h /tmp/gcb017-v2-bench-${n}.db | cut -f1)"
done
