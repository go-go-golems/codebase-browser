# File and Annotation Examples

This example demonstrates file-level widgets and inline annotations.

## Export source file (first 60 lines)

```codebase-file path=internal/staticapp/export.go range=1-60
```

## Annotation on the Export function

```codebase-annotation sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export lines=40-60 note="Options field set by the CLI command; RepoRoot defaults to '.'"
```

## Signature and doc for Export

```codebase-signature sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export
```

```codebase-doc sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export
```

## Notes

- File widgets can show partial ranges using `range=1-60`.
- Annotations overlay notes on specific line ranges.
