# How the indexer extracts symbols

The indexer is a thin wrapper around `golang.org/x/tools/go/packages`. It
loads packages in typed-syntax mode and walks every top-level declaration
in every file, emitting a stable `Symbol` record per function, method,
type, const, or var.

The symbol ID scheme deliberately uses the import path rather than the
file path. This makes IDs survive file moves:

```codebase-signature sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.SymbolID
```

The actual implementation:

```codebase-snippet sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.SymbolID
```

Method IDs embed the receiver name because two different types can have
methods with the same name in the same package:

```codebase-snippet sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.MethodID
```

Entry point for extraction — note how the loader config requests
`NeedCompiledGoFiles` alongside `NeedSyntax` so `Syntax[i]` aligns with
`CompiledGoFiles[i]`:

```codebase-signature sym=sym:github.com/wesen/codebase-browser/internal/indexer.func.Extract
```
