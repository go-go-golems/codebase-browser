# Commit Walk: Review the Live Export Pipeline

A step-by-step walkthrough of the commits affecting the live export pipeline.

```codebase-commit-walk from=b91c6a3 to=79af1b0
step kind=overview title="Review scope" body="This walkthrough covers commits touching the export pipeline used by the live Go API demo."
step kind=diff-stats title="Change summary"
step kind=symbol sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export title="Inspect the Export function"
step kind=diff sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export from=b91c6a3 to=83dbe40 title="Diff across review rendering changes"
step kind=impact sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export dir=usedby depth=2 title="Callers"
step kind=note title="Key observation" body="The export flow packages rendered review docs and the SQLite database for codebase-browser serve, while the browser queries data through the live Go API."
```
