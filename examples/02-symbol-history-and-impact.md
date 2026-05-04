# Symbol History and Impact Analysis

This example demonstrates symbol history and impact analysis widgets.

## Symbol history

Track how the `AddRenderedReviewDocs` function evolved:

```codebase-symbol-history sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.AddRenderedReviewDocs limit=5
```

## Impact: who calls AddRenderedReviewDocs?

```codebase-impact sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.AddRenderedReviewDocs dir=usedby depth=1
```

## Diff stats across recent commits

```codebase-diff-stats from=HEAD~1 to=HEAD
```

## Changed files in recent range

```codebase-changed-files from=HEAD~1 to=HEAD
```

## Notes

- Symbol history is per-commit; the DB stores a snapshot per commit.
- Impact analysis traces references from the `snapshot_refs` table.
