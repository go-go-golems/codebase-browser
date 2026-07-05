# Live Go API runtime

The review browser is served by `codebase-browser serve`. Exporting a review copies
the Vite bundle, writes `manifest.json`, and places the SQLite artifact at
`db/codebase.db`.

At runtime the browser calls `/api/*` endpoints exposed by the Go server. The Go
server opens `db/codebase.db` and answers navigation, source, xref, history,
impact, and review-document queries. The browser does not import sql.js and does
not open the SQLite database directly.

Useful implementation entry points:

```codebase-signature sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export
```

```codebase-signature sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.AddRenderedReviewDocs
```

```codebase-signature sym=sym:github.com/wesen/codebase-browser/internal/server.func.New
```
