package review

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wesen/codebase-browser/internal/browser"
	"github.com/wesen/codebase-browser/internal/gitutil"
	"github.com/wesen/codebase-browser/internal/indexer"
)

func TestIndexDocUpdatesExistingDocAndSnippetCount(t *testing.T) {
	ctx := context.Background()
	store, sourceRoot, loaded := setupDocIndexTest(t, ctx)

	docPath := writeReviewDoc(t, sourceRoot, "review.md", "Original")
	snippets, err := indexDoc(ctx, store, docPath, loaded, os.DirFS(sourceRoot))
	if err != nil {
		t.Fatalf("index doc first time: %v", err)
	}
	if snippets != 1 {
		t.Fatalf("expected 1 snippet, got %d", snippets)
	}

	docPath = writeReviewDoc(t, sourceRoot, "review.md", "Edited")
	snippets, err = indexDoc(ctx, store, docPath, loaded, os.DirFS(sourceRoot))
	if err != nil {
		t.Fatalf("index doc second time: %v", err)
	}
	if snippets != 1 {
		t.Fatalf("expected 1 snippet after update, got %d", snippets)
	}

	var docCount, snippetCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM review_docs WHERE slug = 'review'`).Scan(&docCount); err != nil {
		t.Fatalf("count docs: %v", err)
	}
	if docCount != 1 {
		t.Fatalf("expected one review doc row, got %d", docCount)
	}
	if err := store.DB().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM review_doc_snippets s
		JOIN review_docs d ON d.id = s.doc_id
		WHERE d.slug = 'review'
	`).Scan(&snippetCount); err != nil {
		t.Fatalf("count snippets: %v", err)
	}
	if snippetCount != 1 {
		t.Fatalf("expected stale snippets to be replaced, got %d", snippetCount)
	}

	var content string
	if err := store.DB().QueryRowContext(ctx, `SELECT content FROM review_docs WHERE slug = 'review'`).Scan(&content); err != nil {
		t.Fatalf("read content: %v", err)
	}
	if !strings.Contains(content, "Edited") {
		t.Fatalf("expected updated content, got %q", content)
	}
}

func TestIndexReviewDocsOnlyIndexesMultipleDocsAndCountsSnippets(t *testing.T) {
	ctx := context.Background()
	store, sourceRoot, _ := setupDocIndexTest(t, ctx)

	docA := writeReviewDoc(t, sourceRoot, "a.md", "A")
	docB := writeReviewDoc(t, sourceRoot, "b.md", "B")

	result, err := IndexReview(ctx, store, IndexOptions{
		RepoRoot:    sourceRoot,
		DocsPaths:   []string{docA, docB},
		DocsOnly:    true,
		CommitRange: "this-is-not-a-valid-range",
	})
	if err != nil {
		t.Fatalf("docs-only index should not resolve git range: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected indexing errors: %+v", result.Errors)
	}
	if result.CommitsIndexed != 0 {
		t.Fatalf("docs-only indexed commits: %d", result.CommitsIndexed)
	}
	if result.DocsIndexed != 2 {
		t.Fatalf("expected 2 docs, got %d", result.DocsIndexed)
	}
	if result.SnippetsIndexed != 2 {
		t.Fatalf("expected 2 snippets, got %d", result.SnippetsIndexed)
	}
}

func TestHasCommits(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	store, err := Create(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	hasCommits, err := store.HasCommits(ctx)
	if err != nil {
		t.Fatalf("has commits empty: %v", err)
	}
	if hasCommits {
		t.Fatal("empty DB should not have commits")
	}

	loadTestSnapshot(t, ctx, store, tmpDir)
	hasCommits, err = store.HasCommits(ctx)
	if err != nil {
		t.Fatalf("has commits after load: %v", err)
	}
	if !hasCommits {
		t.Fatal("expected DB to have commits after loading snapshot")
	}
}

func setupDocIndexTest(t *testing.T, ctx context.Context) (*Store, string, *browser.Loaded) {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := Create(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	loadTestSnapshot(t, ctx, store, tmpDir)
	loaded, err := LoadLatestSnapshot(ctx, store)
	if err != nil {
		t.Fatalf("load latest snapshot: %v", err)
	}
	return store, tmpDir, loaded
}

func loadTestSnapshot(t *testing.T, ctx context.Context, store *Store, sourceRoot string) {
	t.Helper()
	fileContent := "package test\n\nfunc Target() {\n\tprintln(\"hello\")\n}\n"
	if err := os.WriteFile(filepath.Join(sourceRoot, "target.go"), []byte(fileContent), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	start := strings.Index(fileContent, "func Target")
	end := len(fileContent)
	commit := gitutil.Commit{
		Hash:        "abc123def456",
		ShortHash:   "abc123",
		Message:     "test commit",
		AuthorName:  "Test",
		AuthorEmail: "test@example.com",
		AuthorTime:  time.Now(),
	}
	idx := &indexer.Index{
		Version: "1",
		Module:  "github.com/test/module",
		Packages: []indexer.Package{{
			ID:         "pkg:github.com/test/module",
			ImportPath: "github.com/test/module",
			Name:       "test",
			FileIDs:    []string{"file:target.go"},
			SymbolIDs:  []string{"sym:github.com/test/module.func.Target"},
		}},
		Files: []indexer.File{{
			ID:        "file:target.go",
			Path:      "target.go",
			PackageID: "pkg:github.com/test/module",
			Size:      len(fileContent),
			LineCount: strings.Count(fileContent, "\n"),
			SHA256:    "test-sha",
			Language:  "go",
		}},
		Symbols: []indexer.Symbol{{
			ID:        "sym:github.com/test/module.func.Target",
			Kind:      "func",
			Name:      "Target",
			PackageID: "pkg:github.com/test/module",
			FileID:    "file:target.go",
			Range: indexer.Range{
				StartLine:   3,
				StartCol:    1,
				EndLine:     5,
				EndCol:      1,
				StartOffset: start,
				EndOffset:   end,
			},
			Signature: "func Target()",
			Language:  "go",
		}},
	}
	if err := store.History.LoadSnapshot(ctx, commit, idx, sourceRoot); err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
}

func writeReviewDoc(t *testing.T, dir, name, marker string) string {
	t.Helper()
	content := "# " + marker + "\n\n```codebase-snippet sym=sym:github.com/test/module.func.Target\n```\n"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	return path
}
