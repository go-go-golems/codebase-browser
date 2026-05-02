---
Title: Review Widget Rendering Contract Analysis and Implementation Guide
Ticket: GCB-018
Status: active
Topics:
    - review
    - frontend
    - sqlite
    - static-export
    - architecture
DocType: design
Intent: long-term
Owners: []
RelatedFiles:
    - Path: internal/docs/renderer.go
      Note: Current Go markdown preprocessing and widget stub generation
    - Path: internal/review/strict_docs.go
      Note: Current strict commit-ref validation added after GCB-017
    - Path: ui/src/api/sqlJsQueryProvider.ts
      Note: Browser-side SQLite query provider and runtime commit-ref resolution
    - Path: ui/src/features/doc/DocSnippet.tsx
      Note: Current widget dispatch layer
    - Path: ui/src/features/doc/widgets/CommitWalkWidget.tsx
      Note: Current commit-walk widget and step dispatch
    - Path: ui/src/features/review/ReviewDocPage.tsx
      Note: Current React DOM scanning and portal hydration for review docs
ExternalSources: []
Summary: "Clean-cut design for making review-widget rendering simple, explicit, strictly validated, and robust across Go, SQLite, and React."
LastUpdated: 2026-05-02T19:36:38.666767904-04:00
WhatFor: "Use this when refactoring review markdown widgets, static review export rendering, strict docs validation, or sql.js-backed widget queries."
WhenToUse: "Before adding or changing a codebase-* directive, review widget, markdown renderer behavior, or exported review document schema."
---

# Review Widget Rendering Contract Analysis and Implementation Guide

## 1. Executive summary

GCB-017 made review indexing dramatically faster and smaller, but it also exposed a separate class of bugs: the review browser can produce a valid SQLite database and still render review widgets incorrectly. During validation we hit failures such as unresolved short symbol refs, `HEAD~5` runtime errors in a two-commit export, commit-walk steps that existed in markdown but not in React, `codebase-file` blocks that were malformed by Markdown rendering, and widgets that displayed generic or indefinite loading states.

These were not isolated frontend mistakes. They were symptoms of an under-specified contract between three subsystems:

- the Go markdown renderer in `internal/docs/renderer.go`,
- the exported SQLite review data and static render tables,
- the React/sql.js frontend in `ui/src/features/...` and `ui/src/api/sqlJsQueryProvider.ts`.

The current design uses an implicit handshake: Go converts `codebase-*` fences into raw HTML stubs plus snippet metadata, React finds those stubs by scanning the DOM, then React decides whether to replace the fallback HTML with a richer widget that may query sql.js at runtime. That can work, but only if every directive is consistently implemented in Go, SQLite, React dispatch, widget code, strict validation, and tests. Today that consistency is enforced by convention rather than by a central schema.

This ticket proposes a clean-cut redesign. We should not preserve compatibility with the fragile stub contract. We should introduce an explicit, structured review document model and a central directive registry that defines each widget once: required params, validated params, render mode, frontend component, backend payload, and runtime query requirements. Go should validate and export that model. React should render that model directly. DOM scanning and hidden `data-*` coupling should go away.

The target end state is:

- Every directive has a single schema entry.
- `--strict-docs` means “this exported review page will render without widget errors.”
- Full `sym:` IDs are the only accepted symbol refs.
- Commit refs are validated before publishing.
- Widget params are normalized once and stored as structured JSON.
- React renders from structured blocks, not from parsed HTML stubs.
- Browser-side errors remain rich, but they should be safety nets, not the main validation path.
- A Playwright smoke test opens every exported review doc and fails on visible widget failures.

## 2. Terms and mental model

A new intern should know the following terms before touching this system.

**Review database**: SQLite database produced by `codebase-browser review index`. It contains normalized commit snapshots, review markdown documents, resolved snippets, and source blobs.

**Static export database**: Copy of the review database at `db/codebase.db` inside an exported static site. `review export` also writes `static_review_rendered_docs` rows used by the browser.

**Directive**: A fenced Markdown block whose info string starts with `codebase-`, for example:

```markdown
```codebase-diff sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export from=HEAD~1 to=HEAD
```
```

**Widget**: The browser-side component that renders a directive interactively. Examples include `DiffStatsWidget`, `ChangedFilesWidget`, `CommitWalkWidget`, and `AnnotationWidget`.

**SnippetRef**: Current Go/TypeScript data object that describes one directive instance. In Go it is `docs.SnippetRef` in `internal/docs/renderer.go`; in TypeScript it is `SnippetRef` in `ui/src/api/docApi.ts`.

**Stub**: Current raw HTML `<div data-codebase-snippet ...>` emitted by Go into rendered Markdown. React later scans for those stubs and hydrates widgets into them.

**Strict docs**: CLI mode (`--strict-docs`) intended to fail when a review doc contains broken directives. The design goal is for strict mode to catch all widget failures that can be known at index/export time.

## 3. Current architecture

The current flow is easiest to understand as a two-stage render pipeline followed by runtime hydration.

```text
Markdown review doc
   |
   v
Go renderer: internal/docs/renderer.go
   - parse codebase-* fences
   - resolve some symbols/files/source snippets
   - emit HTML stub fallback
   - emit SnippetRef metadata
   |
   v
SQLite review_docs + review_doc_snippets
   |
   v
review export
   - reload latest snapshot
   - render markdown again
   - write static_review_rendered_docs
   |
   v
React review page
   - dangerouslySetInnerHTML(rendered HTML)
   - querySelectorAll([data-codebase-snippet])
   - parse data-* attributes
   - clear innerHTML
   - createPortal(<DocSnippet ...>)
   |
   v
Widget-specific rendering + sql.js queries
```

Important current files:

- `internal/docs/renderer.go`
  - Parses directives and emits `SnippetRef` plus stub HTML.
  - Contains `resolveDirective`, `parseCommitWalkSteps`, `stubHTML`, and `resolveSymbol`.
- `internal/review/indexer.go`
  - Calls the renderer during `review index`.
  - Stores raw markdown and snippets.
- `internal/staticapp/reviewdocs.go`
  - Calls the renderer during `review export`.
  - Writes rendered docs into `static_review_rendered_docs`.
- `internal/review/strict_docs.go`
  - Current post-GCB-017 strict validation for browser-resolved commit refs.
- `ui/src/features/review/ReviewDocPage.tsx`
  - Renders exported review docs.
  - Scans DOM stubs and portals React widgets into them.
- `ui/src/features/doc/DocSnippet.tsx`
  - Dispatches one directive to the correct widget component.
- `ui/src/features/doc/widgets/*.tsx`
  - Individual widget implementations.
- `ui/src/api/sqlJsQueryProvider.ts`
  - Browser SQLite provider for symbols, files, commits, diffs, snippets, xrefs, and review docs.

## 4. Current directive behavior matrix

The following table documents the current behavior after GCB-017 hotfixes. This is useful because it shows why a central registry is needed.

| Directive | Needs symbol | Needs file path | Needs commit refs | Go resolves content | React widget queries sql.js | Recent failure mode |
|-----------|--------------|-----------------|-------------------|---------------------|-----------------------------|---------------------|
| `codebase-snippet` | yes | via symbol | optional `commit` | yes for fallback/latest | maybe for commit-specific snippet | loading/error when symbol missing |
| `codebase-signature` | yes | no | optional `commit` | yes | maybe for commit-specific signature | short refs failed |
| `codebase-doc` | yes | no | no | yes | static symbol query | short refs failed |
| `codebase-file` | no | yes | no | yes | now rendered from snippet text | Markdown fallback corrupted source |
| `codebase-diff` | yes | no | `from`, `to` | validates symbol only | yes | commit refs failed at runtime |
| `codebase-diff-stats` | no | no | `from`, `to` | params only | yes | blank/generic failure when refs invalid or zero diff |
| `codebase-changed-files` | no | no | `from`, `to` | params only | yes | raw JSON error for invalid refs |
| `codebase-symbol-history` | yes | no | optional implied range | validates symbol | yes | missing symbol/empty history |
| `codebase-impact` | yes | no | optional `commit` | validates symbol | yes | invalid commit only caught later before strict fix |
| `codebase-annotation` | yes | no | optional `commit` | validates symbol | yes/source snippet | loading for missing runtime text |
| `codebase-commit-walk` | mixed | mixed | top-level and per-step refs | parses step DSL | child widgets query sql.js | unimplemented step kinds; inherited refs missing |

This table is the beginning of the future directive registry.

## 5. What went wrong in GCB-017 validation

### 5.1 Short refs were convenience sugar with hidden semantics

Examples used `staticapp.Export`, but the indexed symbol ID was:

```text
sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export
```

The short-ref resolver tried to infer package paths and names. That made examples look friendly, but it hid the real identity model. In external repos or packages with similar names, it was ambiguous or simply wrong. We removed short refs. This was the correct clarity-first move.

Lesson: symbol identity should be one explicit format: full `sym:` IDs.

### 5.2 Strict mode did not validate browser-resolved refs

A doc could contain:

```markdown
```codebase-changed-files from=HEAD~5 to=HEAD
```
```

If the export indexed only two commits, Go strict mode originally passed because Go did not resolve those commit refs. The browser later failed with `commit ref not found: HEAD~5`.

Lesson: strict validation must cover every parameter that can make a widget fail, even if the widget ultimately queries in React.

### 5.3 Commit-walk had two sources of truth

The example DSL used:

```text
step kind=overview ...
step kind=symbol ...
step kind=diff-stats ...
```

But React implemented only some step kinds. The Go parser accepted unknown step kinds and passed them through. React failed later.

Additionally, the outer block had `from=HEAD~1 to=HEAD`, but the child `diff-stats` step did not inherit those values, so it rendered empty.

Lesson: nested DSLs need the same schema validation and param inheritance as top-level directives.

### 5.4 Fallback HTML was not safe for source code

`codebase-file` emitted a fallback `<pre><code>...</code></pre>` string inside a raw HTML stub. Goldmark still processed portions of that source text in surprising ways, inserting `<p>` tags and nested `<pre>` blocks and displaying escaped quote entities.

Lesson: source text should not rely on Markdown fallback HTML as the authoritative rendering path. Store source text as data and render it through React `Code`, or serialize it into a safe inert script/JSON block.

### 5.5 DOM scanning made ownership unclear

React currently does:

```ts
article.querySelectorAll('[data-codebase-snippet]').forEach((el) => {
  parse attributes;
  el.innerHTML = '';
  createPortal(<DocSnippet ...>, el);
});
```

This means the same stub has two meanings:

- It is fallback content for non-JS or failed hydration.
- It is also a mounting point that React may erase.

For `codebase-file`, erasing was wrong in one version and necessary in another version. That should not be an ad hoc per-directive decision hidden in `ReviewDocPage.tsx`.

Lesson: a structured render model should say whether a block is static HTML, a widget, or both.

## 6. Desired architecture: structured review document model

The proposed clean-cut design is to stop treating rendered review pages as opaque HTML with hidden stubs. Instead, export a structured page model.

```text
Markdown source
   |
   v
Go parser + directive registry
   |
   +--> prose blocks (rendered markdown HTML)
   +--> widget blocks (typed JSON payloads)
   +--> strict validation errors
   |
   v
SQLite static_review_pages
   - slug
   - title
   - blocks_json
   - errors_json
   - rendered_at
   |
   v
React ReviewPage
   - load blocks_json
   - map blocks to components
   - no DOM scanning
   - no data-* parsing
```

A page becomes an ordered list of blocks:

```json
{
  "slug": "03-commit-walk-walkthrough",
  "title": "Commit Walk: Review the Static Export Pipeline",
  "blocks": [
    { "type": "markdown", "html": "<p>A step-by-step...</p>" },
    {
      "type": "widget",
      "id": "w1",
      "directive": "codebase-commit-walk",
      "props": {
        "from": "HEAD~1",
        "to": "HEAD",
        "steps": [
          { "kind": "overview", "title": "Review scope", "body": "..." },
          { "kind": "diff-stats", "title": "Change summary", "from": "HEAD~1", "to": "HEAD" }
        ]
      }
    }
  ],
  "errors": []
}
```

React no longer needs to discover stubs. It simply renders:

```tsx
function ReviewPage({ page }) {
  return <article>
    {page.blocks.map(block => {
      switch (block.type) {
        case 'markdown': return <MarkdownHTML html={block.html} />
        case 'widget': return <ReviewWidget block={block} />
      }
    })}
  </article>
}
```

## 7. Directive registry design

The central piece is a directive registry in Go. This should live in a new package, for example:

```text
internal/reviewwidgets/
  registry.go
  schema.go
  parse.go
  validate.go
  render_model.go
```

A directive definition should declare:

```go
type DirectiveDefinition struct {
    Name string
    RequiredParams []string
    OptionalParams []string
    RequiresSymbol bool
    RequiresFile bool
    CommitRefParams []string
    RenderMode RenderMode
    Parse func(ctx ParseContext, raw RawDirective) (WidgetBlock, []Diagnostic)
    Validate func(ctx ValidateContext, block WidgetBlock) []Diagnostic
}

type RenderMode string

const (
    RenderReactWidget RenderMode = "react-widget"
    RenderStaticCode  RenderMode = "static-code"
    RenderMarkdown    RenderMode = "markdown"
)
```

Example entries:

```go
registry.Register(DirectiveDefinition{
    Name: "codebase-file",
    RequiredParams: []string{"path"},
    OptionalParams: []string{"range"},
    RequiresFile: true,
    RenderMode: RenderReactWidget,
    Parse: parseFileDirective,
    Validate: validateFileDirective,
})

registry.Register(DirectiveDefinition{
    Name: "codebase-diff-stats",
    RequiredParams: []string{"from", "to"},
    CommitRefParams: []string{"from", "to"},
    RenderMode: RenderReactWidget,
    Parse: parseDiffStatsDirective,
    Validate: validateCommitRefs,
})
```

The goal is that no directive can be partially implemented. If a directive is in the registry, tests can assert it has:

- parser behavior,
- strict validation behavior,
- JSON schema snapshot,
- frontend renderer mapping,
- example coverage.

## 8. Proposed exported page schema

Introduce a new table. Since the user explicitly allows clean cutoffs, we do not need to preserve the old `static_review_rendered_docs` shape.

```sql
CREATE TABLE static_review_pages (
    slug TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    blocks_json TEXT NOT NULL,
    diagnostics_json TEXT NOT NULL DEFAULT '[]',
    rendered_at INTEGER NOT NULL DEFAULT 0
);
```

`blocks_json` contains typed blocks:

```ts
type ReviewPageBlock =
  | { type: 'markdown'; html: string }
  | { type: 'widget'; id: string; directive: DirectiveName; props: WidgetProps };
```

Widget prop examples:

```ts
type FileWidgetProps = {
  path: string;
  range?: string;
  language: string;
  text: string;
};

type DiffStatsWidgetProps = {
  from: string;
  to: string;
};

type SymbolWidgetProps = {
  symbolId: string;
  language: string;
  commit?: string;
  text?: string;
};

type CommitWalkWidgetProps = {
  title?: string;
  from?: string;
  to?: string;
  steps: CommitWalkStep[];
};
```

## 9. Frontend rendering design

Replace `ReviewDocPage` DOM scanning with a typed renderer.

Current:

```tsx
<div dangerouslySetInnerHTML={{ __html: data.html }} />
{stubs.map(stub => createPortal(<DocSnippet ... />, stub.el))}
```

Target:

```tsx
function ReviewDocPage() {
  const { data } = useGetReviewPageQuery(slug)
  return <article>
    {data.blocks.map(block => <ReviewBlock key={block.id} block={block} />)}
  </article>
}

function ReviewBlock({ block }: { block: ReviewPageBlock }) {
  if (block.type === 'markdown') {
    return <div dangerouslySetInnerHTML={{ __html: block.html }} />
  }
  return <ReviewWidget directive={block.directive} props={block.props} />
}
```

Widget dispatch becomes explicit:

```tsx
const widgetRegistry = {
  'codebase-file': FileWidget,
  'codebase-diff': SymbolDiffInlineWidget,
  'codebase-diff-stats': DiffStatsWidget,
  'codebase-changed-files': ChangedFilesWidget,
  'codebase-symbol-history': SymbolHistoryInlineWidget,
  'codebase-impact': ImpactInlineWidget,
  'codebase-annotation': AnnotationWidget,
  'codebase-commit-walk': CommitWalkWidget,
} satisfies Record<DirectiveName, React.ComponentType<any>>
```

If a directive is missing from `widgetRegistry`, TypeScript should fail. That requires `DirectiveName` to be generated from the Go registry or maintained in a single TS source file that tests compare with Go.

## 10. Strict validation design

Strict validation should run in Go during both:

- `review index --strict-docs`,
- `review export --strict-docs`.

Strict validation should validate:

- directive name is known,
- required params are present,
- no unsupported params are present unless explicitly allowed,
- full `sym:` refs only,
- symbol exists in latest snapshot or requested commit,
- file path exists in snapshot,
- file range is valid,
- commit refs resolve against indexed commits,
- commit-walk step kinds are known,
- commit-walk inherited params are normalized,
- runtime widget queries have required data.

Pseudocode:

```go
func BuildReviewPage(ctx context.Context, input MarkdownDoc, env RenderEnv) (*ReviewPage, []Diagnostic) {
    tokens := ParseMarkdownWithDirectives(input.Content)
    var blocks []Block
    var diagnostics []Diagnostic

    for _, token := range tokens {
        switch token.Kind {
        case MarkdownText:
            blocks = append(blocks, RenderMarkdownBlock(token.Markdown))
        case DirectiveFence:
            def, ok := registry.Lookup(token.Directive)
            if !ok {
                diagnostics = append(diagnostics, Error(token.Line, "unknown directive"))
                continue
            }
            block, ds := def.Parse(ParseContext{Env: env}, token)
            diagnostics = append(diagnostics, ds...)
            diagnostics = append(diagnostics, def.Validate(ValidateContext{Env: env}, block)...)
            if !HasFatal(ds) {
                blocks = append(blocks, block)
            }
        }
    }
    return &ReviewPage{Blocks: blocks, Diagnostics: diagnostics}, diagnostics
}
```

Commit ref validation should be central:

```go
func ResolveCommitRef(ref string, commits []Commit) (Commit, error) {
    if ref == "" || ref == "HEAD" { return newest(commits), nil }
    if strings.HasPrefix(ref, "HEAD~") { ... }
    if exactHashOrShortHash(ref) { ... }
    if uniquePrefix(ref) { ... }
    return Commit{}, DetailedNotFoundError{...}
}
```

The Go and TypeScript versions should have identical behavior. Prefer generating a test fixture file of commit refs and expected outcomes to test both implementations.

## 11. Commit-walk redesign

Commit-walk is currently a mini DSL inside a directive body. It should remain small, but it needs first-class schema.

Current syntax:

```text
step kind=overview title="Review scope" body="..."
step kind=diff-stats title="Change summary"
step kind=symbol sym=sym:... title="Inspect the Export function"
```

Problems:

- step kinds were not centrally enumerated,
- inherited `from`/`to` was not normalized,
- unknown kinds passed through to React,
- prose rendering duplicated `body`,
- `symbol` semantics were unclear.

Target normalized model:

```json
{
  "directive": "codebase-commit-walk",
  "props": {
    "from": "HEAD~1",
    "to": "HEAD",
    "steps": [
      { "kind": "overview", "title": "Review scope", "body": "..." },
      { "kind": "diff-stats", "title": "Change summary", "from": "HEAD~1", "to": "HEAD" },
      { "kind": "symbol", "title": "Inspect Export", "symbolId": "sym:..." }
    ]
  }
}
```

The Go normalizer should apply inheritance before export:

```go
func normalizeCommitWalk(top Params, steps []Step) []Step {
    for i := range steps {
        if steps[i].From == "" { steps[i].From = top.From }
        if steps[i].To == "" { steps[i].To = top.To }
        if steps[i].Commit == "" { steps[i].Commit = top.Commit }
    }
    return steps
}
```

React should not infer missing defaults. It should receive already-normalized props.

## 12. Testing strategy

### 12.1 Go unit tests

Add tests around the registry and parser:

```text
internal/reviewwidgets/registry_test.go
internal/reviewwidgets/render_model_test.go
internal/reviewwidgets/strict_validation_test.go
```

Test cases:

- unknown directive fails,
- missing required param fails,
- unsupported param fails,
- non-`sym:` symbol ref fails,
- missing symbol fails,
- missing file fails,
- invalid line range fails,
- invalid `HEAD~N` fails with commit count context,
- commit-walk unknown step kind fails,
- commit-walk inherited refs are normalized,
- `codebase-file` text is stored as text, not HTML.

### 12.2 TypeScript unit tests

Add widget-dispatch and error-rendering tests:

```text
ui/src/features/review/reviewPageModel.test.tsx
ui/src/features/doc/widgets/CommitWalkWidget.test.tsx
ui/src/api/sqlJsQueryProvider.test.ts
```

### 12.3 Export smoke test

Add a fixture review doc containing every directive:

```text
examples/all-widgets-smoke.md
```

Then add a make target or Go integration test:

```bash
codebase-browser review index \
  --commits HEAD~2..HEAD \
  --docs examples/all-widgets-smoke.md \
  --db /tmp/gcb-widget-smoke.db \
  --strict-docs

codebase-browser review export \
  --db /tmp/gcb-widget-smoke.db \
  --out /tmp/gcb-widget-smoke \
  --strict-docs
```

### 12.4 Playwright smoke test

Open every review doc and fail if the visible page contains:

- `doc error`,
- `Failed`,
- `Unknown`,
- `not found`,
- `outside this export`,
- `Loading…` after a timeout,
- escaped HTML artifacts such as `&#34;`.

Pseudocode:

```ts
test('all exported review docs render cleanly', async ({ page }) => {
  await page.goto(baseURL)
  const links = await page.locator('a[href^="#/review/"]').evaluateAll(...)
  for (const href of links) {
    await page.goto(baseURL + href)
    await expect(page.locator('main')).not.toContainText(/doc error|Failed|Unknown|Loading…/)
  }
})
```

## 13. Migration plan: clean cut, no compatibility shims

Because we are free to cut compatibility, implement this as a new schema and delete old paths once the new path works.

### Phase 1: Document and freeze current behavior

- Keep current implementation stable.
- Add regression tests for the exact bugs from GCB-017/GCB-018.
- Add `examples/all-widgets-smoke.md`.
- Add Playwright smoke validation.

### Phase 2: Introduce review page model side-by-side internally

- Add Go types:

```go
type ReviewPage struct {
    Slug string `json:"slug"`
    Title string `json:"title"`
    Blocks []ReviewBlock `json:"blocks"`
    Diagnostics []Diagnostic `json:"diagnostics"`
}
```

- Build this model from markdown.
- Continue writing old tables only until tests pass.

### Phase 3: Switch export to `static_review_pages`

- Add `static_review_pages`.
- Update `sqlJsQueryProvider.getReviewDoc` to read blocks.
- Update React to render blocks directly.
- Stop writing `static_review_rendered_docs`.

### Phase 4: Delete DOM scanning and stub HTML

- Delete `stubHTML`.
- Delete `ReviewDocPage` querySelector hydration.
- Delete fallback assumptions from `DocSnippet`.
- Rename `DocSnippet` to `ReviewWidget` or `WidgetBlockRenderer`.

### Phase 5: Make strict mode default for examples and CI

- `docs-smoke` should use `--strict-docs`.
- CI should run the widget smoke export and Playwright check.
- Consider making `--strict-docs` default for `review export` with a `--allow-doc-errors` escape hatch only if truly needed.

## 14. Suggested new files and APIs

### Go

```text
internal/reviewwidgets/schema.go
internal/reviewwidgets/registry.go
internal/reviewwidgets/parser.go
internal/reviewwidgets/validator.go
internal/reviewwidgets/render_model.go
internal/reviewwidgets/commit_refs.go
internal/reviewwidgets/diagnostics.go
```

Important API sketch:

```go
type BuildOptions struct {
    Strict bool
    Latest *browser.Loaded
    SourceFS fs.FS
    DB *sql.DB
}

func BuildPage(ctx context.Context, slug string, md []byte, opts BuildOptions) (*ReviewPage, error)
```

### TypeScript

```text
ui/src/api/reviewPageTypes.ts
ui/src/features/review/ReviewBlockRenderer.tsx
ui/src/features/review/widgetRegistry.tsx
ui/src/features/review/MarkdownBlock.tsx
```

Important API sketch:

```ts
export type ReviewPage = {
  slug: string
  title: string
  blocks: ReviewPageBlock[]
  diagnostics: Diagnostic[]
}

export type ReviewPageBlock = MarkdownBlock | WidgetBlock

export function ReviewBlockRenderer({ block }: { block: ReviewPageBlock })
```

## 15. Risks and tradeoffs

### Risk: More JSON in SQLite

Storing `blocks_json` increases JSON payload size. That is acceptable because review docs are tiny compared with source snapshots, and the clarity gain is large.

### Risk: Losing no-JS fallback

The current stub model nominally has a fallback. In practice the fallback caused bugs and the static browser already requires JavaScript/sql.js for most features. We should optimize for correctness in the supported runtime. If no-JS fallback matters later, implement it deliberately from the same block model.

### Risk: Duplicate schemas in Go and TypeScript

If Go and TS both define directive names manually, they can drift. Mitigate with a small generated JSON schema from Go or a test comparing registry names to TS widget registry names.

### Risk: Scope creep

Do not redesign all markdown rendering. Only replace the `codebase-*` directive handoff contract. Regular markdown can still be rendered with Goldmark into HTML blocks.

## 16. Intern implementation checklist

Start here if you are implementing GCB-018.

1. Read these files:
   - `internal/docs/renderer.go`
   - `internal/review/strict_docs.go`
   - `internal/staticapp/reviewdocs.go`
   - `ui/src/features/review/ReviewDocPage.tsx`
   - `ui/src/features/doc/DocSnippet.tsx`
   - `ui/src/features/doc/widgets/CommitWalkWidget.tsx`
   - `ui/src/api/sqlJsQueryProvider.ts`
2. Add a fixture review doc with every directive.
3. Add a Playwright smoke script that fails on visible widget errors.
4. Create `internal/reviewwidgets` with directive definitions.
5. Implement `BuildPage` returning structured blocks.
6. Implement strict validation from registry metadata.
7. Add `static_review_pages` export table.
8. Update `sqlJsQueryProvider` to load the structured page model.
9. Replace `ReviewDocPage` DOM scanning with block rendering.
10. Delete old stub hydration code.
11. Run:
    - `GOWORK=off go test ./...`
    - `pnpm -C ui run typecheck`
    - `pnpm -C ui test`
    - strict index/export smoke
    - Playwright review-doc smoke
12. Commit in small chunks.

## 17. Success criteria

GCB-018 is done when:

- There is no DOM scanning for `[data-codebase-snippet]` in review pages.
- There is no raw source fallback HTML path that can be parsed by Markdown.
- Every directive is defined in one registry.
- Unknown directive names fail during strict render.
- Unknown commit-walk step kinds fail during strict render.
- Invalid symbol/file/commit refs fail during strict render.
- The exported browser renders all fixture docs without visible errors.
- A Playwright smoke test enforces that behavior in CI.
- Documentation tells authors to use full `sym:` IDs and shows how to discover them.

## 18. Summary for the intern

The current review-widget system works, but it works because Go, SQLite, and React all happen to agree on an implicit convention. GCB-017 showed that this convention is too fragile. The next step is not to patch one widget at a time. The next step is to make the contract explicit.

Think of the future review page as a typed document:

```text
[markdown paragraph]
[widget: diff-stats]
[markdown paragraph]
[widget: codebase-file]
[widget: commit-walk]
```

Go should build and validate that document. SQLite should store it. React should render it directly. Anything less leaves too many places where a directive can be half-supported and only fail after a human opens the exported site.
