# codebase-browser

Turn a code review into a live, shareable browser. `codebase-browser` indexes a commit range, source files, symbols, references, and markdown review notes into a SQLite database. Then `review export` packages a React application and that SQLite database into a directory served by `codebase-browser serve`. The browser calls the Go `/api/*` runtime; it does not open SQLite directly.

Reviewers can read prose, inspect symbol diffs, follow callers and callees, browse source files, and query the same database with SQL or an LLM.

## Why

- **Live, shareable artifacts.** Export a directory that `codebase-browser serve` can publish as a single review application.
- **SQLite as the server-side runtime boundary.** The indexer writes SQLite; the Go API reads SQLite; an LLM can inspect the same SQLite file. One artifact feeds human and machine consumers.
- **Symbol-level diffs and history.** Instead of reviewing only file-level patches, embed symbol-level diffs, impact graphs, and history timelines directly in prose review guides.
- **Review guides as markdown.** Write review notes in markdown with special fenced blocks that become interactive widgets in the exported browser.

## Feature tour

### Literate review guides

Markdown docs can embed guided, interactive review flows. The `codebase-commit-walk`
widget composes smaller semantic widgets into a step-by-step walkthrough:
start with change size, inspect files, drill into a symbol diff, zoom in on an
annotated snippet, check history, and finish with impact.

![Guided commit walk widget](docs/readme-assets/commit-walk.png)

### Semantic symbol diffs

Instead of reviewing only file-level patches, docs can embed symbol-level diffs
for a specific function or method across two commits. Diff rendering is powered
by `@pierre/diffs`, with word-level highlighting, a unified/split toggle, and
lazy-loaded Shiki syntax highlighting scoped at runtime to Go/TypeScript/TSX.

![Embedded symbol diff widget](docs/readme-assets/symbol-diff-widget.png)

### Impact analysis

Impact widgets show callers/callees around a symbol directly inside the review
narrative, with local symbols linked back into history-backed routes.

![Impact analysis widget](docs/readme-assets/impact-widget.png)

### History-backed symbol pages

Deep links such as `/history?symbol=...` open a focused symbol-history view with
per-commit body hashes and the same Diffs-powered body diff renderer used in
embedded review guides.

![History-backed symbol page](docs/readme-assets/symbol-history.png)

## Quick start

Prerequisites: Go 1.22+ and optional Docker for the hermetic Dagger build path. Node 22+ and pnpm 10.x are needed when building the static browser frontend.

```bash
# 1) Optional: install UI deps
pnpm -C ui install

# 2) Build the standalone CLI
#    This builds the React UI, embeds the static assets, and compiles with -tags embed.
make build

# 3) Write a review guide with embedded widgets
mkdir -p ./reviews
cat > ./reviews/pr-42.md << 'EOF'
# PR #42: Add strict mode to Extract

## Changes

```codebase-snippet sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract
```

### Diff

```codebase-diff sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract from=HEAD~1 to=HEAD
```
EOF

# 4) Index commits and review docs into SQLite
#    NOTE: --patterns defaults to ./cmd/...,./internal/...
#    Use --patterns ./... to index all Go packages
./bin/codebase-browser review index \
  --commits HEAD~10..HEAD \
  --docs ./reviews/pr-42.md \
  --parallelism 4 \
  --db /tmp/pr-42.db

# 5) Export a live-server browser bundle
./bin/codebase-browser review export \
  --db /tmp/pr-42.db \
  --out /tmp/pr-42-export

# 6) Serve through the live Go API
./bin/codebase-browser serve \
  --addr :8784 \
  --db /tmp/pr-42-export/db/codebase.db \
  --static-dir /tmp/pr-42-export
# open http://localhost:8784/#/
```

The exported browser loads the SPA assets and sends data requests to `/api/*`. The Go server opens `db/codebase.db` and answers code navigation, source, xref, history, impact, and review-document queries.

## Architecture

```
Git commit range ──▶ indexer (Go) ──▶ SQLite review DB
                                     │
                                     ▼
                    review export ──▶ live-server bundle
                                        ├── index.html
                                        ├── manifest.json
                                        └── db/codebase.db   ◀── Go server opens this file
```

The exported directory is a React SPA plus SQLite database. A Go server runs at read time and owns all SQLite access. All browser data requests go to `/api/*` endpoints.

For documentation on writing review guides, run:

```bash
./bin/codebase-browser help user-guide          # tutorial
./bin/codebase-browser help db-reference         # schema reference
./bin/codebase-browser help markdown-block-reference  # directive reference
```

## Adding a doc page

Drop a markdown file under `internal/docs/embed/pages/`. Any fenced block with an info string of `codebase-snippet`, `codebase-signature`, or `codebase-doc` is replaced at render time with the named symbol's body, signature, or godoc.

**Always use full `sym:` IDs** (e.g. `sym:github.com/.../indexer.func.Extract`). Short refs like `indexer.Extract` are intentionally unsupported because they are ambiguous across packages. Query the review database to discover symbol IDs:

```sql
sqlite3 review.db "SELECT DISTINCT stable_id FROM symbols ORDER BY 1;"
```

## Repo layout

```
cmd/codebase-browser/     Main CLI (glazed commands: review, history, index, query, symbol)
cmd/build-ts-index/       Dagger orchestrator for the Node TS extractor
internal/indexer/         Go AST → Index JSON + Merge
internal/browser/         Index loader shared by CLI and indexing paths
internal/review/          Git/review document indexing into SQLite
internal/staticapp/       Live-server export packaging
internal/sourcefs/        Source tree embed (for snippet slicing)
internal/indexfs/         index.json embed + go:generate wiring
internal/docs/            Markdown renderer + embedded doc pages
internal/history/         Git-aware history: per-commit snapshots, diffs
tools/ts-indexer/         Node + TS Compiler API extractor
ui/                       React SPA (RTK Query + live Go API provider)
pkg/doc/                   Glazed help pages (user-guide, db-reference, markdown-block-reference)
ttmp/                      Ticket workspaces (docmgr)
```

## Testing

```bash
make test                 # go test ./... (indexer, docs, review, static export)
pnpm -C ui run typecheck  # tsc --noEmit for the SPA
pnpm -C tools/ts-indexer test  # vitest (extractor + xref + JSX fixtures)
make smoke                # build the CLI and run --help
make docs-smoke           # smoke-test docs examples (create DB, export, verify)
```

## Documentation

Help pages embedded in the binary (run `./bin/codebase-browser help <topic>`):

| Topic | Description |
|-------|-------------|
| `user-guide` | Tutorial: write review markdown guides with ` ```codebase-* ``` ` blocks |
| `db-reference` | Schema reference and SQL query patterns for the review SQLite database |
| `markdown-block-reference` | Canonical reference for every `codebase-*` directive |

Tickets and design docs live under `ttmp/`:

- **GCB-017** — live demo recovery and backend-only frontend runtime.
- **GCB-015** — earlier static sql.js browser implementation.
- **GCB-014** — earlier architecture redesign from embedded-index to browser-side SQLite export.
- **GCB-001** — original design and 10-phase implementation plan.
