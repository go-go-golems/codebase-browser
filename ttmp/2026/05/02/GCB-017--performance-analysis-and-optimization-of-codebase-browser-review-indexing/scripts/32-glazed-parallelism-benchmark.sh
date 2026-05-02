#!/usr/bin/env bash
# 32-glazed-parallelism-benchmark.sh
# Benchmark codebase-browser review index against glazed repo at different parallelism levels
#
set -euo pipefail

REPO="/home/manuel/code/wesen/go-go-golems/glazed"
TOOL="/home/manuel/code/wesen/corporate-headquarters/codebase-browser/bin/codebase-browser"
DOCS="/tmp/glazed-bench/dummy.md"
OUTDIR="/tmp/glazed-bench"
mkdir -p "$OUTDIR"

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Create dummy doc if missing
[ -f "$DOCS" ] || echo "# Benchmark Dummy" > "$DOCS"

RANGE="HEAD~50..HEAD"
COMMIT_COUNT=$(cd "$REPO" && git log --oneline "$RANGE" | wc -l)

echo ""
echo -e "${CYAN}GLAZED PARALLELISM BENCHMARK${NC}"
echo "  Range: $RANGE ($COMMIT_COUNT commits)"
echo "  Repo: $REPO"
echo ""

printf "%-14s %-10s %-10s %-12s %-12s %-10s\n" "Parallelism" "Wall time" "User time" "DB size" "Throughput" "Speedup"
printf "%-14s %-10s %-10s %-12s %-12s %-10s\n" "-----------" "---------" "---------" "-------" "----------" "-------"

BASE_WALL=""

for P in 1 2 4 8; do
    DB="$OUTDIR/glazed-50fp-p${P}.db"
    rm -f "$DB"

    # Use /usr/bin/time for precise measurements
    OUTPUT=$(/usr/bin/time -f "%e\t%U\t%S" \
        "$TOOL" review index \
            --db "$DB" \
            --commits "$RANGE" \
            --docs "$DOCS" \
            --parallelism "$P" \
            "$REPO" 2>&1)

    # Parse timing from "Done in XmYs" line
    WALL=$(echo "$OUTPUT" | grep -oP 'Done in \K[^:]+' | tail -1)
    WALL_SEC=$(echo "$OUTPUT" | tail -1 | awk '{print $1}')
    USER_SEC=$(echo "$OUTPUT" | tail -1 | awk '{print $2}')
    DB_SIZE=$(du -h "$DB" | cut -f1)

    # Throughput
    if [ -n "$WALL_SEC" ]; then
        THROUGHPUT=$(echo "scale=1; $COMMIT_COUNT / $WALL_SEC * 60" | bc)
    else
        THROUGHPUT="?"
    fi

    # Speedup relative to p=1
    if [ -z "$BASE_WALL" ]; then
        BASE_WALL="$WALL_SEC"
        SPEEDUP="1.0×"
    else
        SPEEDUP=$(echo "scale=1; $BASE_WALL / $WALL_SEC" | bc)
        SPEEDUP="${SPEEDUP}×"
    fi

    printf "%-14s %-10s %-10s %-12s %-12s %-10s\n" \
        "p=$P" "${WALL_SEC}s" "${USER_SEC}s" "$DB_SIZE" "${THROUGHPUT} c/m" "$SPEEDUP"
done

echo ""
echo "Data integrity check:"
for P in 1 2 4 8; do
    DB="$OUTDIR/glazed-50fp-p${P}.db"
    printf "  p=%-2s  commits=%-4s symbols=%-4s refs=%-5s commit_sym=%-6s commit_ref=%-6s\n" \
        "$P" \
        "$(sqlite3 "$DB" 'SELECT COUNT(*) FROM commits;')" \
        "$(sqlite3 "$DB" 'SELECT COUNT(*) FROM symbols;')" \
        "$(sqlite3 "$DB" 'SELECT COUNT(*) FROM ref_versions;')" \
        "$(sqlite3 "$DB" 'SELECT COUNT(*) FROM commit_symbols;')" \
        "$(sqlite3 "$DB" 'SELECT COUNT(*) FROM commit_refs;')"
done
