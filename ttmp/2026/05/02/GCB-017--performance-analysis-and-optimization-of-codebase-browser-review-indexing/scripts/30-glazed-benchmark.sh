#!/usr/bin/env bash
# 30-glazed-benchmark.sh — Benchmark codebase-browser review index against glazed repo
#
# Measures: time, database size, row counts, redundancy ratios
# Repo: /home/manuel/code/wesen/go-go-golems/glazed
# Tool: codebase-browser review index (normalized schema)
#
set -euo pipefail

REPO="/home/manuel/code/wesen/go-go-golems/glazed"
TOOL="/home/manuel/code/wesen/corporate-headquarters/codebase-browser/bin/codebase-browser"
DOCS="/tmp/glazed-bench/dummy.md"
OUTDIR="/tmp/glazed-bench"
mkdir -p "$OUTDIR"

# Create dummy doc
cat > "$DOCS" << 'HEREDOC'
# Benchmark Dummy
Placeholder doc for benchmarking.
HEREDOC

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

benchmark_range() {
    local label="$1"
    local range="$2"
    local db="$OUTDIR/glazed-${label}.db"

    echo -e "${CYAN}================================================================${NC}"
    echo -e "${CYAN}  BENCHMARK: $label  (range: $range)${NC}"
    echo -e "${CYAN}================================================================${NC}"

    # Count actual commits in range
    local commit_count
    commit_count=$(cd "$REPO" && git log --oneline "$range" | wc -l)
    echo -e "  Actual commits: ${YELLOW}$commit_count${NC}"

    # Remove old DB
    rm -f "$db"

    # Run indexing with time measurement
    echo -e "  Running index..."
    local start_sec end_sec elapsed
    start_sec=$(date +%s%N)
    "$TOOL" review index \
        --db "$db" \
        --commits "$range" \
        --docs "$DOCS" \
        "$REPO" 2>&1 | tail -3
    end_sec=$(date +%s%N)
    elapsed=$(( (end_sec - start_sec) / 1000000 ))
    echo -e "  Time: ${GREEN}$(( elapsed / 1000 )).$(( (elapsed % 1000) / 100 ))s${NC}"

    # Database size
    local db_size
    db_size=$(du -h "$db" | cut -f1)
    echo -e "  DB size: ${GREEN}$db_size${NC}"

    # Row counts
    echo ""
    echo -e "  ${YELLOW}Row counts:${NC}"
    for t in commits packages files symbols ref_versions file_contents \
             commit_packages commit_files commit_symbols commit_refs; do
        local count
        count=$(sqlite3 "$db" "SELECT COUNT(*) FROM $t;" 2>/dev/null || echo "?")
        printf "    %-20s %s\n" "$t" "$count"
    done

    # Redundancy analysis
    echo ""
    echo -e "  ${YELLOW}Redundancy ratios (unique / total):${NC}"

    # Symbols: unique by (stable_id, body_hash) vs total commit_symbols
    local sym_total sym_unique sym_red
    sym_total=$(sqlite3 "$db" "SELECT COUNT(*) FROM commit_symbols;")
    sym_unique=$(sqlite3 "$db" "SELECT COUNT(*) FROM symbols;")
    if [ "$sym_total" -gt 0 ]; then
        sym_red=$(sqlite3 "$db" "SELECT ROUND(100.0 * (1 - $sym_unique * 1.0 / $sym_total), 1);")
    else
        sym_red="N/A"
    fi
    printf "    %-20s total=%-8s unique=%-8s redundancy=%s%%\n" "symbols" "$sym_total" "$sym_unique" "$sym_red"

    # Refs
    local ref_total ref_unique ref_red
    ref_total=$(sqlite3 "$db" "SELECT COUNT(*) FROM commit_refs;")
    ref_unique=$(sqlite3 "$db" "SELECT COUNT(*) FROM ref_versions;")
    if [ "$ref_total" -gt 0 ]; then
        ref_red=$(sqlite3 "$db" "SELECT ROUND(100.0 * (1 - $ref_unique * 1.0 / $ref_total), 1);")
    else
        ref_red="N/A"
    fi
    printf "    %-20s total=%-8s unique=%-8s redundancy=%s%%\n" "refs" "$ref_total" "$ref_unique" "$ref_red"

    # Files
    local file_total file_unique file_red
    file_total=$(sqlite3 "$db" "SELECT COUNT(*) FROM commit_files;")
    file_unique=$(sqlite3 "$db" "SELECT COUNT(*) FROM files;")
    if [ "$file_total" -gt 0 ]; then
        file_red=$(sqlite3 "$db" "SELECT ROUND(100.0 * (1 - $file_unique * 1.0 / $file_total), 1);")
    else
        file_red="N/A"
    fi
    printf "    %-20s total=%-8s unique=%-8s redundancy=%s%%\n" "files" "$file_total" "$file_unique" "$file_red"

    # Packages
    local pkg_total pkg_unique pkg_red
    pkg_total=$(sqlite3 "$db" "SELECT COUNT(*) FROM commit_packages;")
    pkg_unique=$(sqlite3 "$db" "SELECT COUNT(*) FROM packages;")
    if [ "$pkg_total" -gt 0 ]; then
        pkg_red=$(sqlite3 "$db" "SELECT ROUND(100.0 * (1 - $pkg_unique * 1.0 / $pkg_total), 1);")
    else
        pkg_red="N/A"
    fi
    printf "    %-20s total=%-8s unique=%-8s redundancy=%s%%\n" "packages" "$pkg_total" "$pkg_unique" "$pkg_red"

    echo ""
    echo ""
}

# ===== Run benchmarks =====
echo ""
echo -e "${RED}****************************************************************${NC}"
echo -e "${RED}*  GLAZED REPO BENCHMARK SUITE                                  *${NC}"
echo -e "${RED}*  $(date)                                   *${NC}"
echo -e "${RED}****************************************************************${NC}"
echo ""
echo "  Repo: $REPO"
echo "  Tool: $TOOL"
echo ""

# Benchmark 1: Small range (10 first-parent = ~83 actual commits)
benchmark_range "10fp" "HEAD~10..HEAD"

# Benchmark 2: Medium range (50 first-parent = ~123 actual commits)
benchmark_range "50fp" "HEAD~50..HEAD"

# Benchmark 3: Large range (100 first-parent = ~252 actual commits)
benchmark_range "100fp" "HEAD~100..HEAD"

# ===== Incremental benchmark =====
echo -e "${CYAN}================================================================${NC}"
echo -e "${CYAN}  BENCHMARK: Incremental (add 10 more to existing 50fp)${NC}"
echo -e "${CYAN}================================================================${NC}"

# Copy the 50fp DB and add more
cp "$OUTDIR/glazed-50fp.db" "$OUTDIR/glazed-incremental.db"
echo "  Adding HEAD~60..HEAD~50 on top of HEAD~50..HEAD..."
start_sec=$(date +%s%N)
"$TOOL" review index \
    --db "$OUTDIR/glazed-incremental.db" \
    --commits "HEAD~60..HEAD~50" \
    --docs "$DOCS" \
    --incremental \
    "$REPO" 2>&1 | tail -3
end_sec=$(date +%s%N)
elapsed=$(( (end_sec - start_sec) / 1000000 ))
echo -e "  Incremental time: ${GREEN}$(( elapsed / 1000 )).$(( (elapsed % 1000) / 100 ))s${NC}"

# No-incremental control (same range, fresh DB)
echo ""
echo "  Control: same 60..50 range, fresh DB..."
rm -f "$OUTDIR/glazed-60-50-fresh.db"
start_sec=$(date +%s%N)
"$TOOL" review index \
    --db "$OUTDIR/glazed-60-50-fresh.db" \
    --commits "HEAD~60..HEAD~50" \
    --docs "$DOCS" \
    "$REPO" 2>&1 | tail -3
end_sec=$(date +%s%N)
elapsed=$(( (end_sec - start_sec) / 1000000 ))
echo -e "  Fresh DB time: ${GREEN}$(( elapsed / 1000 )).$(( (elapsed % 1000) / 100 ))s${NC}"

echo ""
echo -e "${GREEN}All benchmarks complete.${NC}"
echo ""
echo "Database files:"
ls -lh "$OUTDIR"/glazed-*.db
