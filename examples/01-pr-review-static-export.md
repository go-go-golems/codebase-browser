# PR Review: Static Export Packaging

This example demonstrates the core review workflow for a small PR that touches the static export pipeline.

## Motivation

The `staticapp.Export` function is the boundary between the indexer and the static browser. This PR refines its error handling.

## Changes

### Export function signature

```codebase-signature sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export
```

### Export function body diff

```codebase-diff sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export from=HEAD~1 to=HEAD
```

### Impact: callers of Export

```codebase-impact sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export dir=usedby depth=2
```

## Notes

- The `Export` function is called by the `review export` command.
- The static export model means no Go server runs when reading the review.
