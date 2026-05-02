---
Title: "Markdown Block Reference"
Slug: "markdown-block-reference"
Short: "Canonical reference for every codebase-* fenced block directive."
Topics:
- code-review
- markdown
- reference
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
SectionType: GeneralTopic
---

## Overview

Review guides are markdown files under `reviews/` (or any directory) containing normal prose plus special fenced code blocks. These blocks use info strings starting with `codebase-` and are resolved at index/export time into interactive widgets in the exported browser.

This page is the canonical reference for every directive. For a tutorial on writing review guides, see `user-guide`.

## Directives quick reference

| Directive | Purpose | Required params |
|-----------|---------|-----------------|
| `codebase-snippet` | Full symbol body | `sym=` |
| `codebase-signature` | Function/type signature only | `sym=` |
| `codebase-doc` | Godoc/TSDoc comment | `sym=` |
| `codebase-file` | Whole or partial file | `path=` |
| `codebase-diff` | Symbol body diff between two commits | `sym=`, `from=`, `to=` |
| `codebase-symbol-history` | Per-commit symbol timeline | `sym=` |
| `codebase-impact` | Transitive caller/callee list | `sym=` |
| `codebase-commit-walk` | Guided multi-step narrative | `from=`, `to=` |
| `codebase-annotation` | Inline highlights and notes on a symbol | `sym=` |
| `codebase-changed-files` | File-level diff summary | `from=`, `to=` |
| `codebase-diff-stats` | Compact numeric diff summary | `from=`, `to=` |

## Symbol references

Most directives take a `sym=` parameter identifying a symbol in the index.

### Full symbol IDs

Full IDs are prefixed with `sym:` and are stable across renames:

```
sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export
sym:github.com/wesen/codebase-browser/internal/indexer.method.Store.LoadSnapshot
```

### Short symbol references

Short refs omit the `sym:` prefix and use a `package.Name` or `package.Recv.Method` pattern. The resolver finds the symbol by matching the name against the indexed package's **import path**:

```text
staticapp.Export          → top-level function in github.com/.../staticapp
indexer.Extract           → top-level function in github.com/.../indexer
indexer.Store.LoadSnapshot → method on Store type in github.com/.../indexer
```

> [!CAUTION]
> Short refs are fragile. The first segment must match a **suffix** of the
> package's full import path. For example, `indexer.Extract` matches
> `github.com/foo/bar/internal/indexer` but NOT `github.com/foo/bar/cmd/indexer`
> if both packages exist. In external or multi-package repos, short refs often
> fail silently — the directive is simply skipped with no error message.
>
> **Recommendation:** Always use full `sym:` IDs in review markdown files.
> Discover them with:
> ```sql
> sqlite3 review.db "SELECT DISTINCT stable_id FROM symbols ORDER BY 1;"
> ```

Short refs also fail if ambiguous (two symbols with the same name in the same package). Use the full ID in that case.

### Commit parameters

Many directives accept `from=` and `to=` to specify commits for diffs, walkthroughs, and file change summaries:

```text
HEAD             → current commit
HEAD~1           → one commit before HEAD
main..HEAD       → commits on HEAD not on main
abc123           → unique SHA prefix
```

The `commit=` parameter on a directive shows the symbol at a specific commit:

````markdown
Before this change:
```codebase-snippet sym=staticapp.Export commit=HEAD~3
```

After this change:
```codebase-snippet sym=staticapp.Export
```
````

## codebase-snippet

Embeds the full body of a symbol (the declaration plus its implementation body).

**Required params:** `sym=<symbol-ref>`

**Optional params:**
- `commit=<ref>` — show the symbol snapshot at a specific commit
- `kind=declaration` — show declaration only (no body)
- `kind=body` — show function body only (after `{`)
- `dedent=true` — remove common leading whitespace

**Example:**

````markdown
```codebase-snippet sym=staticapp.Export
```
````

**Rendered:** A code block showing the full function/type declaration and its body.

## codebase-signature

Embeds only the signature line of a symbol — the function signature without the body, or the type definition without its fields.

**Required params:** `sym=<symbol-ref>`

**Optional params:**
- `commit=<ref>` — show the signature at a specific commit

**Example:**

````markdown
```codebase-signature sym=staticapp.Export
```
````

**Rendered:** A compact code block showing just the signature:

```go
func Export(ctx context.Context, opts Options) error
```

## codebase-doc

Embeds the godoc or TSDoc comment on a symbol.

**Required params:** `sym=<symbol-ref>`

**Optional params:**
- `commit=<ref>` — show the doc at a specific commit

**Example:**

````markdown
```codebase-doc sym=staticapp.Export
```
````

**Rendered:** A blockquote containing the comment text.

## codebase-file

Embeds the contents of a source file, or a line range within it.

**Required params:** `path=<relative-path>`

**Optional params:**
- `range=<start>-<end>` — 1-indexed line range (e.g. `range=1-80`)
- `commit=<ref>` — show the file at a specific commit

**Example:**

````markdown
```codebase-file path=internal/staticapp/export.go range=1-40
```
````

**Rendered:** A code block showing the specified file or range.

## codebase-diff

Shows a semantic diff of a symbol's body between two commits. Uses word-level highlighting with added/removed line markers.

**Required params:** `sym=<symbol-ref>`, `from=<commit-ref>`, `to=<commit-ref>`

**Optional params:**
- none

**Example:**

````markdown
```codebase-diff sym=staticapp.Export from=HEAD~1 to=HEAD
```
````

**Rendered:** A split or unified diff view of the symbol body, with additions in green and removals in red.

**Common failure:** If `from` and `to` have the same `body_hash` for this symbol (no actual code change), the widget shows no diff. Use `codebase-symbol-history` to confirm which commits actually changed the symbol.

## codebase-symbol-history

Shows a timeline of every commit that touched a symbol, with commit metadata and a per-commit body diff link.

**Required params:** `sym=<symbol-ref>`

**Optional params:**
- `limit=<N>` — maximum number of commits to show (default: all)

**Example:**

````markdown
```codebase-symbol-history sym=staticapp.Export limit=8
```
````

**Rendered:** A table listing commits (short hash, message, author, date) for the symbol, with links to the per-commit symbol snapshot.

## codebase-impact

Shows a transitive caller/callee graph around a symbol. Callers are functions that reference the symbol; callees are symbols the function references.

**Required params:** `sym=<symbol-ref>`

**Optional params:**
- `dir=usedby` — show callers (functions that call/import this symbol) [default]
- `dir=uses` — show callees (functions/symbols this symbol references)
- `depth=<N>` — traversal depth (default: 1)
- `commit=<ref>` — show impact at a specific commit

**Example — who calls `staticapp.Export`?**

````markdown
```codebase-impact sym=staticapp.Export dir=usedby depth=2
```
````

**Example — what does `staticapp.Export` call?**

````markdown
```codebase-impact sym=staticapp.Export dir=uses depth=1
```
````

**Rendered:** A table of symbol names, signatures, and file paths, linked to history-backed symbol pages.

## codebase-commit-walk

Composes multiple directives into a guided, step-by-step narrative. The author writes a fenced block with a line-oriented step DSL in the body, and the widget renders each step as a navigable section.

**Required params:** `from=<commit-ref>`, `to=<commit-ref>`

**Optional params:**
- `title=<text>` — title for the whole walkthrough

**Body DSL syntax:**

Each non-comment line in the fenced block is a step:

```text
step kind=<kind> [title=<text>] [sym=<symbol-ref>] [body=<text>] [<extra-params>]
```

**Step kinds:**

| Kind | Description |
|------|-------------|
| `overview` | Brief text describing the scope of the walk |
| `diff-stats` | File and symbol change counts between `from` and `to` |
| `changed-files` | File-level diff list |
| `symbol` | Show a specific symbol (requires `sym=`) |
| `diff` | Semantic diff for a symbol (requires `sym=`, `from=`, `to=`) |
| `snippet` | Symbol snippet (requires `sym=`) |
| `signature` | Symbol signature (requires `sym=`) |
| `impact` | Impact graph (requires `sym=`) |
| `history` | Symbol history (requires `sym=`) |
| `note` | Free-text note (requires `body=`) |

**Example — complete walkthrough:**

````markdown
```codebase-commit-walk from=HEAD~4 to=HEAD
step kind=overview title="Review scope" body="This PR touches the export pipeline."
step kind=diff-stats title="Change summary"
step kind=symbol sym=staticapp.Export title="Inspect the Export function"
step kind=diff sym=staticapp.Export from=HEAD~4 to=HEAD title="Diff across the PR"
step kind=impact sym=staticapp.Export dir=usedby depth=2 title="Callers"
step kind=note title="Key observation" body="The new Options field is set in the CLI but never read in Export."
```
````

**Rendered:** A vertical step list — one section per step. The first step (`overview`) always appears as an intro paragraph. Each subsequent step renders the embedded widget inline. The reader scrolls through the steps in order.

## codebase-annotation

Overlays inline highlights and notes on a symbol's source code. Useful for pointing out specific lines or patterns.

**Required params:** `sym=<symbol-ref>`

**Optional params:**
- `commit=<ref>` — show annotation at a specific commit (default: latest in range)
- `lines=<start>-<end>` — line range to annotate (default: entire symbol)
- `note=<text>` — annotation text

**Example:**

````markdown
```codebase-annotation sym=staticapp.Export lines=20-35 note="This is the new error path added in this PR"
```
````

**Rendered:** The source snippet with the annotated lines highlighted and the note shown as a tooltip or callout.

## codebase-changed-files

Shows a file-level diff summary between two commits. Lists added, removed, and modified files.

**Required params:** `from=<commit-ref>`, `to=<commit-ref>`

**Optional params:** none

**Example:**

````markdown
```codebase-changed-files from=main to=HEAD
```
````

**Rendered:** A compact list of files grouped by change type (added / modified / deleted).

## codebase-diff-stats

Shows a compact numeric summary of changes between two commits.

**Required params:** `from=<commit-ref>`, `to=<commit-ref>`

**Optional params:** none

**Example:**

````markdown
```codebase-diff-stats from=HEAD~5 to=HEAD
```
````

**Rendered:** A small table or badge showing:
- commits in range
- files changed
- symbols added / removed / modified (if symbols are indexed)

## Commit reference syntax

All `from=`/`to=`/`commit=` parameters accept any git revision spec:

| Spec | Meaning |
|------|---------|
| `HEAD` | Current commit |
| `HEAD~3` | Three commits before HEAD |
| `main` | tip of main branch |
| `main..HEAD` | commits on HEAD but not on main |
| `abc123` | Unique SHA prefix (at least 4 chars) |
| `v1.2.3` | Tag |

For PR reviews, the most common pattern is `from=HEAD~N..HEAD` where `N` is the number of commits in the PR.

## Symbol ID format

Symbol IDs are stable across renames:

```
sym:<importPath>.func.<Name>          # top-level function
sym:<importPath>.type.<Name>          # type, const, var
sym:<importPath>.method.<Recv>.<Name>  # method
```

Examples from this codebase:

```
sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export
sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract
sym:github.com/wesen/codebase-browser/internal/indexer.method.Store.LoadSnapshot
sym:github.com/wesen/codebase-browser/internal/indexer.type.Matcher
```

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Widget shows "doc error: symbol not found" | Symbol not in indexed commit range | Use `sym:` ID (not short ref) and confirm the symbol exists in `--commits` range |
| Widget shows "doc error: ambiguous" | Short ref matches multiple symbols | Use full `sym:` ID |
| **Directive produces no snippet (silently skipped)** | Short ref doesn't match any package import path | Use full `sym:` IDs; query DB: `SELECT DISTINCT stable_id FROM symbols` |
| **Fewer packages than expected** | Default `--patterns` only covers `./cmd/...` and `./internal/...` | Pass `--patterns` explicitly: `--patterns ./...,./pkg/...` |
| Diff widget shows no changes | `from` and `to` commits have same body hash for this symbol | Use `codebase-symbol-history` to find which commits actually changed the symbol |
| File widget shows "doc error: file not in index" | File path is wrong or file was renamed | Use the correct relative path from the repo root |
| Commit-walk shows "doc error: expected step" | DSL body has a malformed step line | Each step line must start with `step kind=...` and have `kind=` as the first parameter |
| `commit=` parameter shows old version | The indexed range does not include the target commit | Extend `--commits` to include the desired commit |
| Export has no review docs | `review index` was not run with `--docs` before `review export` | Run `review index --docs ./reviews --db ...` before `review export` |

## See Also

- `user-guide` — Tutorial for writing review markdown guides
- `db-reference` — Schema reference and SQL query patterns
