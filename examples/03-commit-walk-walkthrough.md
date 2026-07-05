# Commit Walk: Review the Static Export Pipeline

A step-by-step walkthrough of the commits affecting the static export pipeline.

```codebase-commit-walk from=b91c6a3 to=79af1b0
step kind=overview title="Review scope" body="This walkthrough covers commits touching the static export pipeline."
step kind=diff-stats title="Change summary"
step kind=symbol sym=staticapp.Export title="Inspect the Export function"
step kind=diff sym=staticapp.Export from=b91c6a3 to=83dbe40 title="Diff across static review rendering changes"
step kind=impact sym=staticapp.Export dir=usedby depth=2 title="Callers"
step kind=note title="Key observation" body="The static export flow gained rendered review docs while staying a standalone sql.js bundle."
```
