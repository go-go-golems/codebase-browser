// Package docs renders Markdown documentation pages whose fenced code blocks
// are resolved into live snippets from the indexed source tree. See design §11.
//
// Strategy: two-pass pipeline.
//  1. preprocess — find lines like "```codebase-<directive> k=v ..." and
//     replace the fenced block body with the resolved snippet text, changing
//     the info string to "go" so goldmark syntax-highlights it as code.
//  2. render — feed the preprocessed markdown through goldmark.
//
// Snippet metadata (symbol id, file, line range) is also emitted on a
// parallel SnippetRef list so the frontend can offer "jump to source" links.
package docs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	htmlpkg "html"
	"io/fs"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/wesen/codebase-browser/internal/browser"
	"github.com/wesen/codebase-browser/internal/indexer"
)

// SnippetRef is one resolved snippet embedding inside a doc page.
type SnippetRef struct {
	// StubID matches the `data-stub-id` attribute on the stub <div> emitted
	// into Page.HTML — stable within a single rendered page so the frontend
	// can line each stub up with its metadata entry.
	StubID    string `json:"stubId"`
	Directive string `json:"directive"`
	SymbolID  string `json:"symbolId,omitempty"`
	FilePath  string `json:"filePath,omitempty"`
	Kind      string `json:"kind,omitempty"`
	// Language of the resolved symbol (go | ts | ...). Empty for codebase-file.
	Language string `json:"language,omitempty"`
	Text     string `json:"text"`
	// CommitHash is set when the author passes commit=<hash> on a directive.
	// The frontend uses it to fetch the snippet from the history API instead
	// of the static index. (GCB-010 Slice 0)
	CommitHash string            `json:"commitHash,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
	StartLine  int               `json:"startLine,omitempty"`
	EndLine    int               `json:"endLine,omitempty"`
}

// Page is a single rendered doc page.
type Page struct {
	Slug     string       `json:"slug"`
	Title    string       `json:"title"`
	HTML     string       `json:"html"`
	Snippets []SnippetRef `json:"snippets"`
	Errors   []string     `json:"errors,omitempty"`
}

// Render parses mdSource, resolves any codebase-* fenced blocks, and returns
// the rendered HTML + the list of resolved snippets.
func Render(slug string, mdSource []byte, loaded *browser.Loaded, sourceFS fs.FS) (*Page, error) {
	page := &Page{Slug: slug}
	page.Title = firstH1(mdSource)

	processed, snippets, errs := preprocess(mdSource, loaded, sourceFS)
	page.Snippets = snippets
	page.Errors = errs

	md := goldmark.New(goldmark.WithRendererOptions(html.WithUnsafe()))
	var buf bytes.Buffer
	if err := md.Convert(processed, &buf); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	page.HTML = buf.String()
	return page, nil
}

var h1Re = regexp.MustCompile(`(?m)^#\s+(.+)$`)

func firstH1(src []byte) string {
	m := h1Re.FindSubmatch(src)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

// fenceRe matches ``` or ~~~ fenced openers whose info string starts with
// "codebase-". We don't fold ``` and ~~~ across a single doc intentionally.
var fenceOpenRe = regexp.MustCompile("^(?P<fence>```+|~~~+)(codebase-[a-z-]+[^\\n]*)$")

func preprocess(src []byte, loaded *browser.Loaded, sourceFS fs.FS) ([]byte, []SnippetRef, []string) {
	lines := strings.Split(string(src), "\n")
	var out []string
	var snippets []SnippetRef
	var errs []string

	i := 0
	stubCounter := 0
	for i < len(lines) {
		line := lines[i]
		m := fenceOpenRe.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			i++
			continue
		}
		fence := m[1]
		info := m[2]
		// Find the closing fence.
		j := i + 1
		for j < len(lines) && !strings.HasPrefix(lines[j], fence) {
			j++
		}
		// Resolve the directive. Most directives ignore the fence body, but richer
		// widgets such as codebase-commit-walk use it as a small line-oriented DSL.
		body := []string(nil)
		if j <= len(lines) {
			body = lines[i+1 : j]
		}
		ref, err := resolveDirective(info, body, loaded, sourceFS)
		if err != nil {
			errs = append(errs, fmt.Sprintf("line %d: %s", i+1, err))
			// Emit a visible marker so authors see the error in the rendered page.
			out = append(out, fmt.Sprintf("> **doc error**: %s (`%s`)", err, info))
		} else {
			stubCounter++
			ref.StubID = "stub-" + strconv.Itoa(stubCounter)
			snippets = append(snippets, *ref)
			// Emit the stub as a raw-HTML block (blank line above + below so
			// goldmark treats it as a standalone HTML block rather than
			// inline HTML — this preserves the stub's attributes verbatim
			// through markdown rendering).
			out = append(out, "")
			out = append(out, stubHTML(ref))
			out = append(out, "")
		}
		if j < len(lines) {
			i = j + 1
		} else {
			i = len(lines)
		}
	}
	return []byte(strings.Join(out, "\n")), snippets, errs
}

// stubHTML renders a SnippetRef as a single self-contained <div> that the
// React frontend can hydrate. The stub's inner body is the pre-resolved
// plaintext fallback so JS-disabled readers still see something useful.
func stubHTML(ref *SnippetRef) string {
	var body string
	switch ref.Directive {
	case "codebase-signature":
		body = "<pre><code>" + htmlpkg.EscapeString(ref.Text) + "</code></pre>"
	case "codebase-doc":
		body = "<blockquote>" + htmlpkg.EscapeString(ref.Text) + "</blockquote>"
	default: // codebase-snippet, codebase-file
		lang := ref.Language
		if lang == "" {
			lang = "text"
		}
		body = `<pre><code class="language-` + lang + `">` +
			htmlpkg.EscapeString(ref.Text) + "</code></pre>"
	}
	commitAttr := ""
	if ref.CommitHash != "" {
		commitAttr = fmt.Sprintf(` data-commit=%q`, ref.CommitHash)
	}
	paramsAttr := ""
	if len(ref.Params) > 0 {
		paramsJSON, _ := json.Marshal(ref.Params)
		// HTML attributes cannot safely contain raw JSON quotes escaped with
		// backslashes: the browser still treats the quote as the end of the
		// attribute. Escape as HTML instead so getAttribute returns valid JSON.
		paramsAttr = ` data-params="` + htmlpkg.EscapeString(string(paramsJSON)) + `"`
	}
	return fmt.Sprintf(
		`<div class="codebase-snippet" data-codebase-snippet `+
			`data-stub-id=%q data-sym=%q data-directive=%q `+
			`data-kind=%q data-lang=%q%s%s>%s</div>`,
		ref.StubID, ref.SymbolID, ref.Directive, ref.Kind, ref.Language, commitAttr, paramsAttr, body,
	)
}

func resolveDirective(info string, body []string, loaded *browser.Loaded, sourceFS fs.FS) (*SnippetRef, error) {
	parts := splitFields(info)
	directive := parts[0]
	params := map[string]string{}
	for _, p := range parts[1:] {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			params[kv[0]] = kv[1]
		}
	}
	ref := &SnippetRef{Directive: directive}
	// Capture commit= param for history-aware resolution (GCB-010 Slice 0).
	// When set, the stub emits data-commit so the frontend fetches from the
	// history API instead of the static index.
	commitHash := params["commit"]
	ref.CommitHash = commitHash

	switch directive {
	case "codebase-diff":
		symRef := params["sym"]
		from := params["from"]
		to := params["to"]
		if symRef == "" {
			return nil, errors.New("missing sym= on codebase-diff")
		}
		if from == "" || to == "" {
			return nil, errors.New("codebase-diff requires from= and to=")
		}
		sym, err := resolveSymbol(symRef, loaded)
		if err != nil {
			return nil, err
		}
		ref.SymbolID = sym.ID
		ref.Language = sym.Language
		if ref.Language == "" {
			ref.Language = "go"
		}
		ref.Kind = "diff"
		ref.Params = map[string]string{"from": from, "to": to}
		ref.Text = fmt.Sprintf("Diff for %s from %s to %s", sym.ID, from, to)
		return ref, nil

	case "codebase-symbol-history":
		symRef := params["sym"]
		if symRef == "" {
			return nil, errors.New("missing sym= on codebase-symbol-history")
		}
		sym, err := resolveSymbol(symRef, loaded)
		if err != nil {
			return nil, err
		}
		ref.SymbolID = sym.ID
		ref.Language = sym.Language
		if ref.Language == "" {
			ref.Language = "go"
		}
		ref.Kind = "history"
		ref.Params = map[string]string{}
		if limit := params["limit"]; limit != "" {
			ref.Params["limit"] = limit
		}
		ref.Text = fmt.Sprintf("History for %s", sym.ID)
		return ref, nil

	case "codebase-impact":
		symRef := params["sym"]
		if symRef == "" {
			return nil, errors.New("missing sym= on codebase-impact")
		}
		sym, err := resolveSymbol(symRef, loaded)
		if err != nil {
			return nil, err
		}
		ref.SymbolID = sym.ID
		ref.Language = sym.Language
		if ref.Language == "" {
			ref.Language = "go"
		}
		ref.Kind = "impact"
		ref.Params = map[string]string{}
		if dir := params["dir"]; dir != "" {
			ref.Params["dir"] = dir
		}
		if depth := params["depth"]; depth != "" {
			ref.Params["depth"] = depth
		}
		if commit := params["commit"]; commit != "" {
			ref.Params["commit"] = commit
		}
		ref.Text = fmt.Sprintf("Impact for %s", sym.ID)
		return ref, nil

	case "codebase-diff-stats", "codebase-changed-files":
		from := params["from"]
		to := params["to"]
		if from == "" || to == "" {
			return nil, fmt.Errorf("%s requires from= and to=", directive)
		}
		ref.Kind = strings.TrimPrefix(directive, "codebase-")
		ref.Language = "text"
		ref.Params = map[string]string{"from": from, "to": to}
		ref.Text = fmt.Sprintf("%s from %s to %s", directive, from, to)
		return ref, nil

	case "codebase-annotation":
		symRef := params["sym"]
		if symRef == "" {
			return nil, errors.New("missing sym= on codebase-annotation")
		}
		sym, err := resolveSymbol(symRef, loaded)
		if err != nil {
			return nil, err
		}
		ref.SymbolID = sym.ID
		ref.Language = sym.Language
		if ref.Language == "" {
			ref.Language = "go"
		}
		ref.Kind = "annotation"
		ref.Params = map[string]string{}
		for _, key := range []string{"commit", "lines", "note"} {
			if value := params[key]; value != "" {
				ref.Params[key] = value
			}
		}
		ref.Text = fmt.Sprintf("Annotated snippet for %s", sym.ID)
		return ref, nil

	case "codebase-commit-walk":
		steps, err := parseCommitWalkSteps(body, loaded, params)
		if err != nil {
			return nil, err
		}
		if len(steps) == 0 {
			return nil, errors.New("codebase-commit-walk requires at least one step line")
		}
		stepsJSON, err := json.Marshal(steps)
		if err != nil {
			return nil, fmt.Errorf("marshal commit walk: %w", err)
		}
		ref.Kind = "commit-walk"
		ref.Language = "text"
		ref.Params = map[string]string{"steps": string(stepsJSON)}
		if title := params["title"]; title != "" {
			ref.Params["title"] = title
		}
		ref.Text = fmt.Sprintf("Commit walk with %d step(s)", len(steps))
		return ref, nil

	case "codebase-snippet", "codebase-signature", "codebase-doc":
		symRef := params["sym"]
		if symRef == "" {
			return nil, errors.New("missing sym= on " + directive)
		}
		sym, err := resolveSymbol(symRef, loaded)
		if err != nil {
			return nil, err
		}
		ref.SymbolID = sym.ID
		ref.StartLine = sym.Range.StartLine
		ref.EndLine = sym.Range.EndLine
		ref.Language = sym.Language
		if ref.Language == "" {
			ref.Language = "go"
		}
		switch directive {
		case "codebase-doc":
			ref.Kind = "doc"
			ref.Text = sym.Doc
			return ref, nil
		case "codebase-signature":
			ref.Kind = "signature"
			ref.Text = sym.Signature
			return ref, nil
		}
		file, ok := loaded.File(sym.FileID)
		if !ok {
			return nil, fmt.Errorf("file for %s not indexed", sym.ID)
		}
		data, err := fs.ReadFile(sourceFS, file.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file.Path, err)
		}
		start, end := sym.Range.StartOffset, sym.Range.EndOffset
		if start < 0 || end > len(data) || start > end {
			return nil, fmt.Errorf("range out of file for %s", sym.ID)
		}
		snippet := string(data[start:end])
		if params["kind"] == "body" {
			if open := strings.IndexByte(snippet, '{'); open >= 0 {
				snippet = strings.TrimSpace(snippet[open:])
			}
		}
		if params["dedent"] == "true" {
			snippet = dedent(snippet)
		}
		ref.Kind = params["kind"]
		if ref.Kind == "" {
			ref.Kind = "declaration"
		}
		ref.Text = snippet
		ref.FilePath = file.Path
		return ref, nil

	case "codebase-file":
		path := params["path"]
		if path == "" {
			return nil, errors.New("missing path= on codebase-file")
		}
		if _, ok := loaded.File("file:" + path); !ok {
			return nil, fmt.Errorf("file %s not in index", path)
		}
		data, err := fs.ReadFile(sourceFS, path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		text := string(data)
		if r := params["range"]; r != "" {
			if trimmed, err := sliceByLines(text, r); err == nil {
				text = trimmed
			}
		}
		ref.FilePath = path
		ref.Language = languageForPath(path)
		ref.Params = map[string]string{"path": path}
		if r := params["range"]; r != "" {
			ref.Params["range"] = r
		}
		ref.Text = text
		return ref, nil
	}
	return nil, fmt.Errorf("unknown directive %q", directive)
}

type commitWalkStep struct {
	Kind     string            `json:"kind"`
	Title    string            `json:"title,omitempty"`
	Body     string            `json:"body,omitempty"`
	SymbolID string            `json:"symbolId,omitempty"`
	Language string            `json:"language,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
}

func parseCommitWalkSteps(lines []string, loaded *browser.Loaded, inherited map[string]string) ([]commitWalkStep, error) {
	var steps []commitWalkStep
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := splitFields(line)
		if len(parts) == 0 {
			continue
		}
		if parts[0] != "step" {
			return nil, fmt.Errorf("commit walk line %d: expected step, got %q", i+1, parts[0])
		}
		params := map[string]string{}
		for _, p := range parts[1:] {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) == 2 {
				params[kv[0]] = kv[1]
			}
		}
		kind := params["kind"]
		if kind == "" {
			return nil, fmt.Errorf("commit walk line %d: missing kind=", i+1)
		}
		step := commitWalkStep{
			Kind:   kind,
			Title:  params["title"],
			Body:   params["body"],
			Params: map[string]string{},
		}
		delete(params, "kind")
		delete(params, "title")
		delete(params, "body")
		if symRef := params["sym"]; symRef != "" {
			sym, err := resolveSymbol(symRef, loaded)
			if err != nil {
				return nil, fmt.Errorf("commit walk line %d: %w", i+1, err)
			}
			step.SymbolID = sym.ID
			step.Language = sym.Language
			delete(params, "sym")
		}
		if step.Language == "" {
			step.Language = "go"
		}
		for _, key := range []string{"from", "to", "commit"} {
			if params[key] == "" && inherited[key] != "" {
				params[key] = inherited[key]
			}
		}
		for k, v := range params {
			step.Params[k] = v
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func languageForPath(path string) string {
	switch {
	case strings.HasSuffix(path, ".go"):
		return "go"
	case strings.HasSuffix(path, ".ts"), strings.HasSuffix(path, ".tsx"):
		return "typescript"
	case strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".jsx"):
		return "javascript"
	case strings.HasSuffix(path, ".md"):
		return "markdown"
	default:
		return "text"
	}
}

func splitFields(s string) []string {
	var fields []string
	var b strings.Builder
	quote := rune(0)
	escaped := false
	flush := func() {
		if b.Len() == 0 {
			return
		}
		fields = append(fields, b.String())
		b.Reset()
	}
	for _, r := range s {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' {
			flush()
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		b.WriteRune('\\')
	}
	flush()
	return fields
}

// resolveSymbol accepts either a full "sym:..." ID or a short form
// "pkg/import/path.Name" / "pkg/import/path.Recv.Method". Ambiguous short
// forms return an error so authors notice.
func resolveSymbol(ref string, l *browser.Loaded) (*indexer.Symbol, error) {
	if strings.HasPrefix(ref, "sym:") {
		sym, ok := l.Symbol(ref)
		if !ok {
			return nil, fmt.Errorf("symbol %s not found", ref)
		}
		return sym, nil
	}
	// Short form: last segment is Name (or Recv.Method), rest is importPath.
	// In review prose we also allow package-local forms such as
	// "staticapp.Export". Those match by package import-path suffix or package
	// name and are rejected if ambiguous.
	dot := strings.LastIndex(ref, ".")
	if dot < 0 {
		return nil, fmt.Errorf("bad short ref %q", ref)
	}
	importPath := ref[:dot]
	name := ref[dot+1:]
	// Short form "pkg.Name" addresses top-level (package-scoped) symbols
	// only. Methods share the package ID of their receiver type, so without
	// the filter below a ref like "internal/indexer.Extract" matches both
	// `func Extract` and any `method X.Extract` in the same package. Force
	// method use to go through the "pkg.Recv.Name" form handled below.
	var candidates []*indexer.Symbol
	for i := range l.Index.Symbols {
		s := &l.Index.Symbols[i]
		if !packageRefMatches(importPath, s, l) {
			continue
		}
		if s.Kind == "method" {
			continue
		}
		if s.Name != name {
			continue
		}
		candidates = append(candidates, s)
	}
	// Also try treating it as a method: strip the "Recv." from name.
	if dot2 := strings.LastIndex(importPath, "."); dot2 >= 0 {
		pkg := importPath[:dot2]
		recv := importPath[dot2+1:]
		for i := range l.Index.Symbols {
			s := &l.Index.Symbols[i]
			if !packageRefMatches(pkg, s, l) {
				continue
			}
			if s.Kind != "method" || s.Name != name {
				continue
			}
			if s.Receiver != nil && s.Receiver.TypeName == recv {
				candidates = append(candidates, s)
			}
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("symbol %q not found", ref)
	}
	if len(candidates) > 1 {
		ids := make([]string, len(candidates))
		for i, c := range candidates {
			ids[i] = c.ID
		}
		return nil, fmt.Errorf("ambiguous %q: %s", ref, strings.Join(ids, ", "))
	}
	return candidates[0], nil
}

func packageRefMatches(ref string, sym *indexer.Symbol, l *browser.Loaded) bool {
	pkgID := string(sym.PackageID)
	pkgID = strings.TrimPrefix(pkgID, "pkg:")
	if pkgID == ref || strings.HasSuffix(pkgID, "/"+ref) {
		return true
	}
	if pkg, ok := l.Package(string(sym.PackageID)); ok {
		if pkg.ImportPath == ref || strings.HasSuffix(pkg.ImportPath, "/"+ref) || pkg.Name == ref {
			return true
		}
	}
	return false
}

func dedent(s string) string {
	lines := strings.Split(s, "\n")
	prefix := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		spaces := 0
		for _, c := range line {
			if c != ' ' && c != '\t' {
				break
			}
			spaces++
		}
		if prefix < 0 || spaces < prefix {
			prefix = spaces
		}
	}
	if prefix <= 0 {
		return s
	}
	for i, line := range lines {
		if len(line) >= prefix {
			lines[i] = line[prefix:]
		}
	}
	return strings.Join(lines, "\n")
}

func sliceByLines(text, spec string) (string, error) {
	dash := strings.IndexByte(spec, '-')
	if dash <= 0 {
		return text, errors.New("bad range")
	}
	var start, end int
	if _, err := fmt.Sscanf(spec[:dash], "%d", &start); err != nil {
		return text, err
	}
	if _, err := fmt.Sscanf(spec[dash+1:], "%d", &end); err != nil {
		return text, err
	}
	lines := strings.Split(text, "\n")
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start-1:end], "\n"), nil
}
