package docs

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/wesen/codebase-browser/internal/browser"
	"github.com/wesen/codebase-browser/internal/indexer"
)

func fixtureLoaded(t *testing.T) (*browser.Loaded, fstest.MapFS) {
	t.Helper()
	src := "package foo\n\n// Hello greets.\nfunc Hello(name string) string {\n\treturn \"hi \" + name\n}\n"
	// Offsets below were computed for the source above.
	idx := &indexer.Index{
		Version: "1",
		Module:  "example.com/foo",
		Packages: []indexer.Package{{
			ID: indexer.PackageID("example.com/foo"), ImportPath: "example.com/foo", Name: "foo",
		}},
		Files: []indexer.File{{
			ID: indexer.FileID("foo.go"), Path: "foo.go",
			PackageID: indexer.PackageID("example.com/foo"),
			Size:      len(src),
		}},
		Symbols: []indexer.Symbol{{
			ID:        indexer.SymbolID("example.com/foo", "func", "Hello", ""),
			Kind:      "func",
			Name:      "Hello",
			PackageID: indexer.PackageID("example.com/foo"),
			FileID:    indexer.FileID("foo.go"),
			Signature: "func Hello(name string) string",
			Doc:       "Hello greets.",
			Range:     indexer.Range{StartOffset: 29, EndOffset: int(len(src) - 1), StartLine: 4, EndLine: 6},
		}},
	}
	data, _ := json.Marshal(idx)
	l, err := browser.LoadFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	return l, fstest.MapFS{"foo.go": &fstest.MapFile{Data: []byte(src)}}
}

func TestRender_Signature(t *testing.T) {
	l, srcFS := fixtureLoaded(t)
	md := "# title\n\n```codebase-signature sym=sym:example.com/foo.func.Hello\n```\n"
	page, err := Render("p", []byte(md), l, srcFS)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Errors) > 0 {
		t.Fatalf("errors: %v", page.Errors)
	}
	if strings.Contains(page.HTML, "data-codebase-snippet") {
		t.Errorf("html still contains deprecated hydration stub: %s", page.HTML)
	}
	if !strings.Contains(page.HTML, "Resolved") {
		t.Errorf("html missing inert resolution marker: %s", page.HTML)
	}
	if len(page.Snippets) != 1 || page.Snippets[0].Kind != "signature" || page.Snippets[0].Text != "func Hello(name string) string" {
		t.Errorf("snippets=%+v", page.Snippets)
	}
}

func TestRender_DocDirective(t *testing.T) {
	l, srcFS := fixtureLoaded(t)
	md := "```codebase-doc sym=sym:example.com/foo.func.Hello\n```\n"
	page, err := Render("p", []byte(md), l, srcFS)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(page.HTML, "data-codebase-snippet") {
		t.Errorf("html still contains deprecated hydration stub: %s", page.HTML)
	}
	if len(page.Snippets) != 1 || page.Snippets[0].Text != "Hello greets." {
		t.Errorf("snippet missing doc text: %+v", page.Snippets)
	}
}

func TestRender_RejectsShortSymbolRef(t *testing.T) {
	l, srcFS := fixtureLoaded(t)
	md := "```codebase-snippet sym=example.com/foo.Hello\n```\n"
	page, err := Render("p", []byte(md), l, srcFS)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Errors) == 0 {
		t.Fatal("expected error for non-sym reference")
	}
	if !strings.Contains(page.Errors[0], "must use a full sym: ID") {
		t.Fatalf("unexpected errors: %v", page.Errors)
	}
}

func TestRender_MissingSymbol_ReportsError(t *testing.T) {
	l, srcFS := fixtureLoaded(t)
	md := "```codebase-snippet sym=sym:example.com/foo.func.Nope\n```\n"
	page, err := Render("p", []byte(md), l, srcFS)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Errors) == 0 {
		t.Fatal("expected error for missing symbol")
	}
	if !strings.Contains(page.HTML, "doc error") {
		t.Errorf("expected inline marker, got: %s", page.HTML)
	}
}

func TestFirstH1(t *testing.T) {
	if got := firstH1([]byte("# Hello\n\nblah")); got != "Hello" {
		t.Errorf("firstH1=%q", got)
	}
}

func TestRender_DoesNotEmitHydrationStubs(t *testing.T) {
	l, srcFS := fixtureLoaded(t)
	md := "# t\n\n```codebase-snippet sym=sym:example.com/foo.func.Hello\n```\n\n" +
		"```codebase-signature sym=sym:example.com/foo.func.Hello\n```\n"
	page, err := Render("p", []byte(md), l, srcFS)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Errors) > 0 {
		t.Fatalf("errors: %v", page.Errors)
	}
	if len(page.Snippets) != 2 {
		t.Fatalf("want 2 snippets, got %d", len(page.Snippets))
	}
	for _, s := range page.Snippets {
		if s.StubID == "" {
			t.Errorf("snippet %s has no StubID", s.SymbolID)
		}
	}
	if strings.Contains(page.HTML, `data-codebase-snippet`) || strings.Contains(page.HTML, `data-stub-id`) {
		t.Errorf("html still contains deprecated hydration attributes: %s", page.HTML)
	}
	if !strings.Contains(page.HTML, "Resolved") {
		t.Errorf("html missing inert resolution marker: %s", page.HTML)
	}
}
