# Symbol History and Impact Analysis

This example demonstrates a symbol with real history: the `history scan` command builder. It changes in the indexed range as the history CLI grows, the old Go server runtime is removed, and CI/lint plumbing lands.

## Symbol history

Track how `newScanCmd` evolved across recent indexed commits:

```codebase-symbol-history sym=history.newScanCmd limit=20
```

## Body diff: server-runtime cleanup to CI-ready command

```codebase-diff sym=history.newScanCmd from=05f3ffe to=7c095d0
```

## Impact: who calls newScanCmd?

```codebase-impact sym=history.newScanCmd dir=usedby depth=2
```

## Diff stats across recent commits

```codebase-diff-stats from=b91c6a3 to=79af1b0
```

## Changed files in recent range

```codebase-changed-files from=b91c6a3 to=79af1b0
```

## Notes

- `newScanCmd` has multiple body changes inside the first 20 history rows, so the history widget shows changed rows without requiring a very long table.
- The explicit body diff shows a focused implementation change in a CLI command rather than only aggregate file stats.
- Impact analysis traces references from the `snapshot_refs` table.
