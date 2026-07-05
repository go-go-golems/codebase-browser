# PR Review: Static Export Packaging

This example demonstrates the core review workflow for a small PR that touches the static export pipeline.

## Motivation

The `staticapp.Export` function is the boundary between the indexer and the static browser. This PR refines its error handling.

## Changes

### Export function signature

```codebase-signature sym=staticapp.Export
```

### Export function body diff

```codebase-diff sym=staticapp.Export from=b91c6a3 to=83dbe40
```

### Impact: callers of Export

```codebase-impact sym=staticapp.Export dir=usedby depth=2
```

## Notes

- The `Export` function is called by the `review export` command.
- The static export model means no Go server runs when reading the review.
