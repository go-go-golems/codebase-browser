package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wesen/codebase-browser/internal/docs"
	"github.com/wesen/codebase-browser/internal/reviewwidgets"
)

type strictCommitRow struct {
	Hash      string
	ShortHash string
}

// ValidatePageCommitRefs checks browser-resolved commit references in rendered
// review widgets. The markdown renderer can validate symbols and files on its
// own, but widgets such as codebase-diff-stats and codebase-changed-files defer
// commit resolution to the static browser; strict docs mode should catch those
// invalid refs before publishing an export.
func ValidatePageCommitRefs(ctx context.Context, db *sql.DB, page *docs.Page) []error {
	if page == nil {
		return nil
	}
	commits, err := orderedCommits(ctx, db)
	if err != nil {
		return []error{err}
	}
	var errs []error
	for _, snippet := range page.Snippets {
		for _, ref := range snippetCommitRefs(snippet) {
			if _, err := resolveIndexedCommitRef(commits, ref); err != nil {
				errs = append(errs, fmt.Errorf("%s %s: %w", snippet.Directive, ref, err))
			}
		}
	}
	return errs
}

func orderedCommits(ctx context.Context, db *sql.DB) ([]strictCommitRow, error) {
	rows, err := db.QueryContext(ctx, `
SELECT hash, short_hash
FROM commits
WHERE error = ''
ORDER BY sequence ASC, author_time ASC`)
	if err != nil {
		return nil, fmt.Errorf("query indexed commits for strict docs: %w", err)
	}
	defer rows.Close()

	var commits []strictCommitRow
	for rows.Next() {
		var c strictCommitRow
		if err := rows.Scan(&c.Hash, &c.ShortHash); err != nil {
			return nil, fmt.Errorf("scan indexed commit for strict docs: %w", err)
		}
		commits = append(commits, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate indexed commits for strict docs: %w", err)
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("no indexed commits in review database")
	}
	return commits, nil
}

func snippetCommitRefs(snippet docs.SnippetRef) []string {
	seen := map[string]bool{}
	var refs []string
	add := func(ref string) {
		ref = strings.TrimSpace(ref)
		if ref == "" || seen[ref] {
			return
		}
		seen[ref] = true
		refs = append(refs, ref)
	}

	add(snippet.CommitHash)
	if snippet.Params != nil {
		if def, ok := reviewwidgets.Lookup(snippet.Directive); ok {
			for _, key := range def.CommitRefKeys {
				add(snippet.Params[key])
			}
		}
		if stepsJSON := snippet.Params["steps"]; stepsJSON != "" {
			var steps []struct {
				Kind   string            `json:"kind"`
				Params map[string]string `json:"params"`
			}
			if err := json.Unmarshal([]byte(stepsJSON), &steps); err == nil {
				for _, step := range steps {
					if def, ok := reviewwidgets.LookupStep(step.Kind); ok {
						for _, key := range def.CommitRefKeys {
							add(step.Params[key])
						}
					}
				}
			}
		}
	}
	return refs
}

func resolveIndexedCommitRef(commits []strictCommitRow, ref string) (string, error) {
	if len(commits) == 0 {
		return "", fmt.Errorf("no indexed commits in review database")
	}
	newestIndex := len(commits) - 1
	if ref == "" || ref == "HEAD" {
		return commits[newestIndex].Hash, nil
	}
	if strings.HasPrefix(ref, "HEAD~") {
		var offset int
		if _, err := fmt.Sscanf(ref, "HEAD~%d", &offset); err != nil {
			return "", fmt.Errorf("invalid HEAD offset ref %q", ref)
		}
		index := newestIndex - offset
		if index < 0 || index >= len(commits) {
			return "", fmt.Errorf("commit ref %s is outside this export's indexed range (%d commit(s) available; deepest supported ref is HEAD~%d)", ref, len(commits), len(commits)-1)
		}
		return commits[index].Hash, nil
	}
	for _, commit := range commits {
		if commit.Hash == ref || commit.ShortHash == ref {
			return commit.Hash, nil
		}
	}
	var matches []strictCommitRow
	for _, commit := range commits {
		if strings.HasPrefix(commit.Hash, ref) {
			matches = append(matches, commit)
		}
	}
	if len(matches) == 1 {
		return matches[0].Hash, nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous commit ref %q", ref)
	}
	return "", fmt.Errorf("commit ref %s was not found in this export (%d indexed commit(s))", ref, len(commits))
}
