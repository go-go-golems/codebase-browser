package staticapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wesen/codebase-browser/internal/docs"
	"github.com/wesen/codebase-browser/internal/review"
)

const createRenderedReviewDocsSQL = `
CREATE TABLE IF NOT EXISTS static_review_rendered_docs (
    slug TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    html TEXT NOT NULL,
    snippets_json TEXT NOT NULL DEFAULT '[]',
    errors_json TEXT NOT NULL DEFAULT '[]',
    rendered_at INTEGER NOT NULL DEFAULT 0
);
`

// AddRenderedReviewDocs renders review markdown into HTML and stores the result
// inside the exported SQLite database. This keeps markdown/directive resolution
// in Go while making the static browser read review pages through sql.js.
func AddRenderedReviewDocs(ctx context.Context, dbPath, repoRoot string, strict bool) error {
	store, err := review.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open copied DB: %w", err)
	}
	defer store.Close()

	if _, err := store.DB().ExecContext(ctx, createRenderedReviewDocsSQL); err != nil {
		return fmt.Errorf("create rendered review docs table: %w", err)
	}

	loaded, err := review.LoadLatestSnapshot(ctx, store)
	if err != nil {
		return fmt.Errorf("load latest snapshot: %w", err)
	}
	latestHash, err := review.LatestCommitHash(ctx, store.DB())
	if err != nil {
		return fmt.Errorf("load latest commit hash: %w", err)
	}

	rows, err := store.DB().QueryContext(ctx, `
		SELECT slug, title, content
		FROM review_docs
		ORDER BY slug
	`)
	if err != nil {
		return fmt.Errorf("query review docs: %w", err)
	}
	defer rows.Close()

	type reviewDoc struct {
		slug    string
		title   string
		content string
	}
	var reviewDocs []reviewDoc
	for rows.Next() {
		var doc reviewDoc
		if err := rows.Scan(&doc.slug, &doc.title, &doc.content); err != nil {
			return err
		}
		reviewDocs = append(reviewDocs, doc)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	sourceFS := review.NewSnapshotFS(ctx, store.DB(), latestHash)
	renderedAt := time.Now().Unix()
	for _, doc := range reviewDocs {
		slug, title, content := doc.slug, doc.title, doc.content
		page, renderErr := docs.Render(slug, []byte(content), loaded, sourceFS)
		html := ""
		snippetsJSON := "[]"
		errorsJSON := "[]"
		if renderErr != nil {
			errorsJSON = mustJSON([]string{renderErr.Error()})
		} else {
			if strict && len(page.Errors) > 0 {
				return fmt.Errorf("render doc %s: %d directive error(s): %s", slug, len(page.Errors), strings.Join(page.Errors, "; "))
			}
			html = page.HTML
			if page.Title != "" {
				title = page.Title
			}
			snippets := page.Snippets
			if snippets == nil {
				snippets = []docs.SnippetRef{}
			}
			errs := page.Errors
			if errs == nil {
				errs = []string{}
			}
			snippetsJSON = mustJSON(snippets)
			errorsJSON = mustJSON(errs)
		}

		if _, err := store.DB().ExecContext(ctx, `
			INSERT INTO static_review_rendered_docs(slug, title, html, snippets_json, errors_json, rendered_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(slug) DO UPDATE SET
				title = excluded.title,
				html = excluded.html,
				snippets_json = excluded.snippets_json,
				errors_json = excluded.errors_json,
				rendered_at = excluded.rendered_at
		`, slug, title, html, snippetsJSON, errorsJSON, renderedAt); err != nil {
			return fmt.Errorf("upsert rendered doc %s: %w", slug, err)
		}
	}
	return nil
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(data)
}
