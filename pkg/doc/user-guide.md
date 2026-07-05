---
Title: "Writing Live Code Review Guides"
Slug: "user-guide"
Short: "How to write markdown review guides and export them as a live Go API codebase browser."
Topics:
- code-review
- markdown
- tutorial
Commands:
- review index
- review export
- review db create
Flags:
- commits
- docs
- db
IsTopLevel: false
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

## Quick start

Write a markdown file with embedded code widgets, then index and export it:

```bash
# 1. Create a review guide
cat > ./reviews/pr-42.md << 'EOF'
# PR #42: Add strict mode to Extract

## Motivation
The `Extract` function needs to support build tag filtering.

## Changes

### 1. New parameter
```codebase-diff sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract from=HEAD~1 to=HEAD
```

### 2. Updated callers
```codebase-impact sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract dir=usedby depth=2
```
EOF

# 2. Index commits and docs into a review database
codebase-browser review index \
  --commits HEAD~5..HEAD \
  --docs ./reviews/pr-42.md \
  --db ./reviews/pr-42.db

# 3. Export a live-server browser bundle
codebase-browser review export \
  --db ./reviews/pr-42.db \
  --out ./reviews/pr-42-export

# 4. Serve the export through the live Go API
codebase-browser serve \
  --addr :3002 \
  --db ./reviews/pr-42-export/db/codebase.db \
  --static-dir ./reviews/pr-42-export

# 5. Open http://localhost:3002/#/review/pr-42 in a browser
```

The exported browser loads the SPA assets and sends data requests to `/api/*`. The Go server opens `db/codebase.db` and owns all SQLite access.

## Writing review markdown files

Review guides are regular markdown files with special fenced code blocks that the renderer replaces with interactive widgets during export.

### Available directives

| Directive | Purpose | Example |
|-----------|---------|---------|
| `codebase-snippet` | Full symbol body | `` ```codebase-snippet sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract``` `` |
| `codebase-signature` | Just the signature | `` ```codebase-signature sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract``` `` |
| `codebase-doc` | Godoc/TSDoc comment | `` ```codebase-doc sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract``` `` |
| `codebase-file` | Whole or partial file | `` ```codebase-file path=internal/staticapp/export.go range=1-80``` `` |
| `codebase-diff` | Symbol body diff between commits | `` ```codebase-diff sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract from=HEAD~1 to=HEAD``` `` |
| `codebase-symbol-history` | Timeline of commits touching a symbol | `` ```codebase-symbol-history sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Merge limit=8``` `` |
| `codebase-impact` | Transitive caller/callee list | `` ```codebase-impact sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract dir=usedby depth=2``` `` |
| `codebase-commit-walk` | Guided narrative through commits | `` ```codebase-commit-walk from=HEAD~4 to=HEAD``` `` |
| `codebase-annotation` | Inline highlights and notes | `` ```codebase-annotation sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract commit=HEAD``` `` |
| `codebase-changed-files` | File-level diff summary | `` ```codebase-changed-files from=main to=HEAD``` `` |
| `codebase-diff-stats` | Compact numeric summary | `` ```codebase-diff-stats from=main to=HEAD``` `` |

### Symbol references

Symbols must be referenced with full `sym:` IDs:

```markdown
sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract
sym:github.com/wesen/codebase-browser/internal/indexer.method.Store.LoadSnapshot
```

Short refs such as `indexer.Extract` are intentionally not supported. They are ambiguous across packages and can look correct while pointing at no indexed symbol.

To discover full symbol IDs for your repo, query the review database:

```sql
sqlite3 review.db "SELECT DISTINCT stable_id FROM symbols ORDER BY 1;"
```

### Commit parameters

Most directives accept an optional `commit=` parameter to show the symbol at a specific commit:

````markdown
Before this PR:
```codebase-snippet sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract commit=HEAD~3
```

After this PR:
```codebase-snippet sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract
```
````

When `commit=` is present, the static browser resolves that commit ref against the exported SQLite database and reads the symbol snapshot at that commit.

## Commit range syntax

The `--commits` flag accepts any git log range:

| Example | Meaning |
|---------|---------|
| `HEAD~10..HEAD` | Last 10 commits |
| `main..feature` | Commits on `feature` not on `main` |
| `abc123..def456` | Between two SHAs |
| `HEAD` | Just the current commit |
| `--all` | All reachable commits |

For PR reviews, `HEAD~N..HEAD` is usually what you want, where `N` is the number of commits in the PR.

## Sharing review artifacts

A review export is a static directory plus a SQLite database. You can:

- **Publish it:** Upload the export directory to any static file host.
- **Share it as an artifact:** Zip the export directory and attach it to a PR or CI run.
- **Query the DB with an LLM:** Give `db/codebase.db` to an LLM with instructions to run SQL against it. The schema is documented in `db-reference`.

The source review database produced by `review index` is also useful on its own as a SQLite artifact, but the browser runtime should use `review export` output.

Before publishing a long-history export, validate the database integrity:

```bash
./bin/codebase-browser review db validate --db review.db
```

The validator checks that snapshot symbols and references point to files and symbols present in the same commit. This catches historical consistency bugs such as an unchanged symbol moving files while retaining the same body hash. Schema v3 preserves those moved symbol versions by including file/range location in the symbol version identity, so databases produced with older schema versions should be rebuilt before publishing large history exports.

## Querying the DB with an LLM

After running `review db create --commits HEAD~10..HEAD --db review.db`, you have a queryable SQLite file. Here are example prompts for an LLM:

**Prompt:** "Which functions had signature changes in this PR?"

```sql
SELECT s1.name, s1.signature AS old, s2.signature AS new
FROM snapshot_symbols s1
JOIN snapshot_symbols s2 ON s1.id = s2.id
WHERE s1.commit_hash = (SELECT hash FROM commits ORDER BY author_time ASC LIMIT 1)
  AND s2.commit_hash = (SELECT hash FROM commits ORDER BY sequence DESC, author_time DESC LIMIT 1)
  AND s1.signature != s2.signature;
```

**Prompt:** "Which symbols were added in this PR?"

```sql
SELECT s.name, s.kind, s.signature
FROM snapshot_symbols s
WHERE s.commit_hash = (SELECT hash FROM commits ORDER BY sequence DESC, author_time DESC LIMIT 1)
  AND s.id NOT IN (
    SELECT id FROM snapshot_symbols
    WHERE commit_hash = (SELECT hash FROM commits ORDER BY author_time ASC LIMIT 1)
  );
```

**Prompt:** "Show me the impact graph for `indexer.Extract` — who calls it?"

```sql
SELECT r.from_symbol_id, s.name, s.signature, r.kind
FROM snapshot_refs r
JOIN snapshot_symbols s ON s.id = r.from_symbol_id
  AND s.commit_hash = r.commit_hash
WHERE r.to_symbol_id = 'sym:github.com/foo/bar.func.Target'
  AND r.commit_hash = (SELECT hash FROM commits ORDER BY sequence DESC, author_time DESC LIMIT 1);
```

## Workflow tips

### Iterative review writing

1. Write the markdown guide with placeholder text.
2. Run `review index --commits RANGE --docs ./reviews/ --db review.db`.
3. Run `review export --db review.db --out ./review-static`.
4. Serve `./review-static` with a static file server.
5. Edit the markdown, re-run `review index --docs-only --docs ./reviews/ --db review.db`, re-run `review export`, and refresh the browser.

### Team reviews

For team review meetings:

1. The PR author writes the review guide.
2. They run `review index` and `review export`.
3. They share the exported static directory, a zip of it, or a hosted URL.
4. Everyone sees the same interactive widgets with no Go process running at review time.

### Large commit ranges

For large PRs (50+ commits), use `--parallelism N` to extract multiple commits concurrently:

```bash
./bin/codebase-browser review index \
  --commits HEAD~100..HEAD \
  --docs ./reviews/ \
  --parallelism 4 \
  --db review.db
```

The database uses a normalized schema where each unique symbol is stored once,
so database size grows sub-linearly with commit count. A 250-commit range
for a medium Go project (80+ packages) typically produces a 5–7 MB database.

Multi-commit ranges automatically use git worktrees so each source/symbol/ref snapshot matches its commit.

Add `--strict-docs` to `review index` or `review export` in CI when unresolved `codebase-*` directives should fail the command instead of being stored as render errors in the review page.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Symbol not found in rendered doc | Commit range doesn't include the symbol | Widen `--commits` range |
| Widget shows "doc error" | Missing symbol, wrong commit range, or non-`sym:` reference | Use a full `sym:` ID and verify the symbol exists in the indexed range |
| **0 snippets resolved** | Directives reference symbols that are not in the DB | Query the DB to find full IDs: `sqlite3 review.db "SELECT DISTINCT stable_id FROM symbols"` |
| **Missing packages in index** | Default `--patterns` only covers `./cmd/...` and `./internal/...` | Pass `--patterns` explicitly: `--patterns ./...,./pkg/...` |
| Export shows no review docs | No docs in DB | Run `review index` with `--docs` before `review export` |
| Browser cannot load data | Live server is not running or cannot open the DB | Start `codebase-browser serve --db ./reviews/pr-42-export/db/codebase.db --static-dir ./reviews/pr-42-export` and check `/api/health` |
| Diff widget shows no changes | `from` and `to` commits have same `body_hash` | Check commit range |

## See Also

- `markdown-block-reference` — Canonical reference for every `codebase-*` directive with full parameter tables and examples
- `db-reference` — Complete schema reference and SQL query patterns
