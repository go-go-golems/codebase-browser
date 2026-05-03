package reviewwidgets

import (
	"strings"
	"testing"
)

func TestBuildPageStructuredBlocks(t *testing.T) {
	page, err := BuildPage("smoke", []byte(`# Smoke

Intro prose.

`+"```codebase-diff-stats from=HEAD~1 to=HEAD\n```"+`

Outro prose.
`))
	if err != nil {
		t.Fatalf("BuildPage() error = %v", err)
	}
	if page.Title != "Smoke" {
		t.Fatalf("Title = %q, want Smoke", page.Title)
	}
	if len(page.Diagnostics) != 0 {
		t.Fatalf("Diagnostics = %#v, want none", page.Diagnostics)
	}
	if len(page.Blocks) != 3 {
		t.Fatalf("len(Blocks) = %d, want 3", len(page.Blocks))
	}
	if page.Blocks[0].Type != "markdown" || !strings.Contains(page.Blocks[0].HTML, "Intro prose") {
		t.Fatalf("first block = %#v", page.Blocks[0])
	}
	widget := page.Blocks[1]
	if widget.Type != "widget" || widget.Directive != "codebase-diff-stats" {
		t.Fatalf("widget block = %#v", widget)
	}
	if widget.ID != "widget-1" || widget.Props["from"] != "HEAD~1" || widget.Props["to"] != "HEAD" {
		t.Fatalf("widget props = %#v", widget)
	}
	if page.Blocks[2].Type != "markdown" || !strings.Contains(page.Blocks[2].HTML, "Outro prose") {
		t.Fatalf("last block = %#v", page.Blocks[2])
	}
}

func TestBuildPageReportsDirectiveDiagnostics(t *testing.T) {
	page, err := BuildPage("bad", []byte(`# Bad

`+"```codebase-diff-stats from=HEAD~1\n```"))
	if err != nil {
		t.Fatalf("BuildPage() error = %v", err)
	}
	if len(page.Diagnostics) != 1 {
		t.Fatalf("Diagnostics = %#v, want one", page.Diagnostics)
	}
	if page.Diagnostics[0].Message != "codebase-diff-stats requires to=" {
		t.Fatalf("diagnostic message = %q", page.Diagnostics[0].Message)
	}
	if len(page.Blocks) != 1 || page.Blocks[0].Type != "markdown" {
		t.Fatalf("Blocks = %#v, want only leading markdown", page.Blocks)
	}
}

func TestBuildPageReportsCommitWalkStepDiagnostics(t *testing.T) {
	page, err := BuildPage("bad-walk", []byte(`# Bad Walk

`+"```codebase-commit-walk from=HEAD~1 to=HEAD\nstep kind=magic title=Nope\n```"))
	if err != nil {
		t.Fatalf("BuildPage() error = %v", err)
	}
	if len(page.Diagnostics) != 1 {
		t.Fatalf("Diagnostics = %#v, want one", page.Diagnostics)
	}
	if page.Diagnostics[0].Message != "unknown commit-walk step kind \"magic\"" {
		t.Fatalf("diagnostic message = %q", page.Diagnostics[0].Message)
	}
}

func TestSplitFieldsQuotedValues(t *testing.T) {
	fields := SplitFields(`codebase-annotation sym=sym:x note="hello world" lines=1-2`)
	want := []string{"codebase-annotation", "sym=sym:x", "note=hello world", "lines=1-2"}
	if len(fields) != len(want) {
		t.Fatalf("fields = %#v", fields)
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Fatalf("fields[%d] = %q, want %q", i, fields[i], want[i])
		}
	}
}
