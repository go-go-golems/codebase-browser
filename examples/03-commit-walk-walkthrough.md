# Commit Walk: Review the Static Export Pipeline

A step-by-step walkthrough of the commits affecting the static export pipeline.

```codebase-commit-walk from=HEAD~1 to=HEAD
step kind=overview title="Review scope" body="This walkthrough covers commits touching the static export pipeline."
step kind=diff-stats title="Change summary"
step kind=symbol sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export title="Inspect the Export function"
step kind=diff sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export from=HEAD~1 to=HEAD title="Diff across recent changes"
step kind=impact sym=sym:github.com/wesen/codebase-browser/internal/staticapp.func.Export dir=usedby depth=2 title="Callers"
step kind=note title="Key observation" body="The static export pipeline keeps generated browser assets and database packaging behind the Options struct without changing the CLI-facing Export function signature."
```
