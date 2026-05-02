#!/usr/bin/env bash
# 20-benchmark-parallelism.sh — Compare parallelism=1 vs parallelism=2
# Run from repo root after `make build`.
set -euo pipefail

for p in 1 2; do
    rm -f /tmp/gcb017-par${p}.db
    echo "=== parallelism=${p} ==="
    time ./bin/codebase-browser review index \
        --commits "HEAD~20..HEAD" \
        --docs ./README.md \
        --db /tmp/gcb017-par${p}.db \
        --repo-root . \
        --patterns "./cmd/..." "./internal/..." \
        --parallelism ${p} 2>&1 | grep "Done in"
    echo "  Symbols: $(sqlite3 /tmp/gcb017-par${p}.db 'SELECT COUNT(*) FROM snapshot_symbols;')"
    echo "  DB size: $(du -h /tmp/gcb017-par${p}.db | cut -f1)"
    echo ""
done
