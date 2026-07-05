#!/usr/bin/env bash
set -euo pipefail

# Stage the six requested Pi transcript JSONL files into a small Pi-shaped
# source directory and convert only that subset to go-minitrace archives.

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC_DIR="/home/manuel/.pi/agent/sessions/--home-manuel-code-wesen-2026-04-19--go-codebase-browser--"
STAGED="$ROOT/archive/staged-pi-sessions"
OUT="$ROOT/archive/minitrace"

mkdir -p "$STAGED" "$OUT"
rm -f "$STAGED"/*.jsonl

sessions=(
  "2026-04-20T16-08-01-580Z_8664fb89-aa66-4563-826b-0dbe8c78019e.jsonl"
  "2026-04-23T23-42-57-477Z_019dbcb9-a9c4-7449-9d4f-dc260c37eaba.jsonl"
  "2026-04-25T11-48-02-978Z_019dc477-dd22-76d9-8e6a-c8ab7bef4b3b.jsonl"
  "2026-04-30T15-11-58-306Z_019ddef2-5b5d-72fb-b679-1d10972a0515.jsonl"
  "2026-05-02T14-40-10-900Z_019de921-f893-73dc-8bfa-9087aa65cbfe.jsonl"
  "2026-07-02T23-18-46-087Z_019f2520-ac47-7a6e-8372-23242db70cad.jsonl"
)

for session in "${sessions[@]}"; do
  if [[ ! -f "$SRC_DIR/$session" ]]; then
    echo "missing transcript: $SRC_DIR/$session" >&2
    exit 1
  fi
  cp "$SRC_DIR/$session" "$STAGED/$session"
done

# Rebuild the minitrace output so query results are reproducible.
rm -rf "$OUT"
mkdir -p "$OUT"
go-minitrace convert pi --source-dir "$STAGED" --output-dir "$OUT"

find "$OUT" -name '*.minitrace.json' -print | sort
