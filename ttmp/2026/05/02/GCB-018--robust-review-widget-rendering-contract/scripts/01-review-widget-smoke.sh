#!/usr/bin/env bash
set -euo pipefail

# Review widget smoke test for GCB-018.
#
# This script builds (or uses) the codebase-browser binary, indexes the
# all-widget fixture with strict docs, exports a static site with strict docs,
# serves it locally, and optionally runs a Playwright browser check when the
# `playwright` package is available to Node.
#
# Environment:
#   GCB_BIN=/path/to/codebase-browser   Use an existing binary instead of ./bin/codebase-browser
#   GCB_SKIP_BUILD=1                    Skip `make build`
#   GCB_SMOKE_KEEP=1                    Keep temp DB/export/server after success
#   GCB_SMOKE_PORT=4179                 HTTP port

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../../.." && pwd)"
cd "$ROOT"

PORT="${GCB_SMOKE_PORT:-$(python3 - <<'PY'
import random
print(random.randint(20000, 45000))
PY
)}"
DB="$(mktemp /tmp/gcb-widget-smoke-XXXXXX.db)"
OUT="$(mktemp -d /tmp/gcb-widget-smoke-export-XXXXXX)"
LOG="$(mktemp /tmp/gcb-widget-smoke-http-XXXXXX.log)"
PID=""
BIN="${GCB_BIN:-$ROOT/bin/codebase-browser}"

cleanup() {
  if [[ -n "${PID:-}" ]] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" || true
  fi
  if [[ "${GCB_SMOKE_KEEP:-}" != "1" ]]; then
    rm -f "$DB" "$LOG"
    rm -rf "$OUT"
  else
    echo "KEEP: db=$DB"
    echo "KEEP: out=$OUT"
    echo "KEEP: log=$LOG"
  fi
}
trap cleanup EXIT

if [[ "${GCB_SKIP_BUILD:-}" != "1" ]]; then
  echo "[smoke] building standalone binary"
  GOWORK=off make build
fi

if [[ ! -x "$BIN" ]]; then
  echo "binary not found or not executable: $BIN" >&2
  exit 1
fi

echo "[smoke] strict index all-widget fixture"
"$BIN" review index \
  --commits HEAD~2..HEAD \
  --docs ./examples/all-widgets-smoke.md \
  --db "$DB" \
  --parallelism 2 \
  --strict-docs

echo "[smoke] strict export"
(
  cd /tmp
  "$BIN" review export \
    --db "$DB" \
    --out "$OUT" \
    --repo-root codebase-browser-widget-smoke \
    --strict-docs
)

echo "[smoke] SQLite validation"
test "$(sqlite3 "$OUT/db/codebase.db" "SELECT COUNT(*) FROM static_review_rendered_docs WHERE json_array_length(errors_json)>0;")" = "0"
test "$(sqlite3 "$OUT/db/codebase.db" "SELECT COUNT(*) FROM review_doc_snippets;")" -ge "11"

echo "[smoke] serving $OUT on http://127.0.0.1:$PORT"
(
  cd "$OUT"
  python3 -m http.server "$PORT" >"$LOG" 2>&1
) &
PID=$!
for _ in $(seq 1 50); do
  if curl -fsS "http://127.0.0.1:$PORT/manifest.json" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done
curl -fsS "http://127.0.0.1:$PORT/manifest.json" >/dev/null

if node -e "require.resolve('playwright')" >/dev/null 2>&1; then
  echo "[smoke] running Playwright visible-error scan"
  node "$(dirname "${BASH_SOURCE[0]}")/02-review-widget-smoke.mjs" "http://127.0.0.1:$PORT"
else
  echo "[smoke] playwright package not available; skipped browser scan" >&2
  echo "[smoke] install playwright or run the companion script in an environment with playwright" >&2
fi

echo "[smoke] PASS"
