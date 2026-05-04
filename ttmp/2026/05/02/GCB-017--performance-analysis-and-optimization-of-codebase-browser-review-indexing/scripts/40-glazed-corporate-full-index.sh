#!/usr/bin/env bash
set -euo pipefail

# Full-ish Glazed indexing run for the corporate-headquarters/glazed worktree.
# Note: this checkout has .git as a file pointing at ../.git/modules/glazed.
# The ranges below cover all first-parent reachable history except the root commit:
#   git rev-list --count HEAD            => 1578
#   indexed via HEAD~500..HEAD~250 + HEAD~250..HEAD => 1577

REPO=${REPO:-/home/manuel/code/wesen/corporate-headquarters/glazed}
BIN=${BIN:-/tmp/codebase-browser-demo}
DB=${DB:-/tmp/glazed-full.db}
REVIEWS=${REVIEWS:-/tmp/glazed-full-reviews}
EXPORT=${EXPORT:-/tmp/glazed-full-export}
PARALLELISM=${PARALLELISM:-8}

mkdir -p "$REVIEWS"
cat > "$REVIEWS/glazed-full-overview.md" <<'DOC'
# Glazed Full History Smoke Review

This review document is intentionally repository-generic so it can render across the full Glazed history without depending on a symbol that may not exist in older commits.

## Recent full-repository change size

```codebase-diff-stats from=HEAD~20 to=HEAD
```

## Recent changed files

```codebase-changed-files from=HEAD~20 to=HEAD
```

## Guided walkthrough

```codebase-commit-walk from=HEAD~20 to=HEAD title="Glazed full-history smoke walkthrough"
step kind=overview title="Scope" body="This full-history export indexes the Glazed repository in incremental batches and uses the browser history view to inspect change shape."
step kind=diff-stats title="Twenty-commit change size"
step kind=note title="Next" body="Use the history page to select specific commits and inspect file and symbol changes."
```
DOC

cd "$REPO"
git worktree prune || true
rm -rf .git-worktrees

/usr/bin/time -f 'GLAZED_STEP1 elapsed=%E maxrss=%MKB' \
  "$BIN" review index --repo-root . --db "$DB" \
  --commits 'HEAD~500..HEAD~250' --patterns './...' \
  --parallelism "$PARALLELISM" --docs "$REVIEWS" --incremental --strict-docs

git worktree prune || true
rm -rf .git-worktrees

/usr/bin/time -f 'GLAZED_STEP2 elapsed=%E maxrss=%MKB' \
  "$BIN" review index --repo-root . --db "$DB" \
  --commits 'HEAD~250..HEAD' --patterns './...' \
  --parallelism "$PARALLELISM" --docs "$REVIEWS" --incremental --strict-docs

sqlite3 "$DB" < "$(dirname "$0")/41-glazed-corporate-full-analysis.sql"

rm -rf "$EXPORT"
/usr/bin/time -f 'GLAZED_EXPORT elapsed=%E maxrss=%MKB' \
  "$BIN" review export --db "$DB" --out "$EXPORT" --repo-root "$REPO" --strict-docs

du -sh "$EXPORT" "$EXPORT/db/codebase.db"
