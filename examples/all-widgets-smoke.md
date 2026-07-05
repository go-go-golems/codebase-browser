# All Widgets Smoke Review

This review document intentionally exercises every supported `codebase-*` review widget. It is used by the review-widget smoke test and should stay boring: full `sym:` IDs only, commit refs that fit the small smoke range, and no deprecated short refs.

## Snippet

```codebase-snippet sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export
```

## Signature

```codebase-signature sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export
```

## Documentation

```codebase-doc sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export
```

## File range

```codebase-file path=internal/staticapp/export.go range=1-40
```

## Symbol diff

```codebase-diff sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export from=HEAD~1 to=HEAD
```

## Diff stats

```codebase-diff-stats from=HEAD~1 to=HEAD
```

## Changed files

```codebase-changed-files from=HEAD~1 to=HEAD
```

## Symbol history

```codebase-symbol-history sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export limit=5
```

## Impact

```codebase-impact sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export dir=usedby depth=2
```

## Annotation

```codebase-annotation sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export lines=20-45 note="Smoke test annotation for the export boundary."
```

## Commit walk

```codebase-commit-walk from=HEAD~1 to=HEAD title="Smoke commit walk"
step kind=overview title="Scope" body="This smoke walk checks prose, stats, symbol, diff, impact, and note steps."
step kind=diff-stats title="Stats"
step kind=symbol sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export title="Export symbol"
step kind=diff sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export title="Export diff"
step kind=impact sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export dir=usedby depth=1 title="Export callers"
step kind=note title="Observation" body="If this page renders without visible widget errors, the review widget contract is healthy enough for a smoke pass."
```
