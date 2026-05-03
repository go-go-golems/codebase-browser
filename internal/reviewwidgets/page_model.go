package reviewwidgets

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

// Diagnostic is a structured page-build or validation message.
type Diagnostic struct {
	Severity  string `json:"severity"`
	Line      int    `json:"line,omitempty"`
	Directive string `json:"directive,omitempty"`
	Message   string `json:"message"`
}

// Page is the structured review document model that will replace opaque HTML
// plus data-codebase-snippet stub hydration.
type Page struct {
	Slug        string       `json:"slug"`
	Title       string       `json:"title,omitempty"`
	Blocks      []Block      `json:"blocks"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// Block is one ordered review-page block. Markdown blocks contain already
// rendered HTML; widget blocks contain a typed directive name and normalized
// props that React can render directly.
type Block struct {
	Type      string            `json:"type"`
	ID        string            `json:"id,omitempty"`
	HTML      string            `json:"html,omitempty"`
	Directive string            `json:"directive,omitempty"`
	Props     map[string]string `json:"props,omitempty"`
	Body      string            `json:"body,omitempty"`
	Line      int               `json:"line,omitempty"`
}

// BuildPage parses Markdown into an ordered structured model. It validates the
// directive contract but does not resolve repository-specific symbols/files;
// that happens in later strict validation phases with a loaded index/DB.
func BuildPage(slug string, mdSource []byte) (*Page, error) {
	page := &Page{Slug: slug, Title: FirstH1(mdSource)}
	lines := strings.Split(string(mdSource), "\n")
	var markdown []string
	flushMarkdown := func() error {
		if len(markdown) == 0 {
			return nil
		}
		html, err := renderMarkdown(strings.Join(markdown, "\n"))
		if err != nil {
			return err
		}
		if strings.TrimSpace(html) != "" {
			page.Blocks = append(page.Blocks, Block{Type: "markdown", HTML: html})
		}
		markdown = nil
		return nil
	}

	widgetCounter := 0
	for i := 0; i < len(lines); {
		line := lines[i]
		m := fenceOpenRe.FindStringSubmatch(line)
		if m == nil {
			markdown = append(markdown, line)
			i++
			continue
		}
		if err := flushMarkdown(); err != nil {
			return nil, err
		}
		fence := m[1]
		info := m[2]
		j := i + 1
		for j < len(lines) && !strings.HasPrefix(lines[j], fence) {
			j++
		}
		bodyLines := []string(nil)
		if j <= len(lines) {
			bodyLines = lines[i+1 : j]
		}
		block, diagnostics := buildWidgetBlock(widgetCounter+1, i+1, info, bodyLines)
		page.Diagnostics = append(page.Diagnostics, diagnostics...)
		if block != nil {
			widgetCounter++
			page.Blocks = append(page.Blocks, *block)
		}
		if j < len(lines) {
			i = j + 1
		} else {
			i = len(lines)
		}
	}
	if err := flushMarkdown(); err != nil {
		return nil, err
	}
	return page, nil
}

func buildWidgetBlock(id, line int, info string, bodyLines []string) (*Block, []Diagnostic) {
	directive, params := ParseInfo(info)
	if err := ValidateParams(directive, params); err != nil {
		return nil, []Diagnostic{{Severity: "error", Line: line, Directive: directive, Message: err.Error()}}
	}
	props := copyStringMap(params)
	body := strings.Join(bodyLines, "\n")
	if directive == "codebase-commit-walk" {
		if diagnostics := validateCommitWalkBody(line, bodyLines); len(diagnostics) > 0 {
			return nil, diagnostics
		}
	}
	return &Block{
		Type:      "widget",
		ID:        "stub-" + strconv.Itoa(id),
		Directive: directive,
		Props:     props,
		Body:      body,
		Line:      line,
	}, nil
}

func validateCommitWalkBody(startLine int, lines []string) []Diagnostic {
	var diagnostics []Diagnostic
	stepCount := 0
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := SplitFields(line)
		if len(parts) == 0 {
			continue
		}
		if parts[0] != "step" {
			diagnostics = append(diagnostics, Diagnostic{Severity: "error", Line: startLine + i + 1, Directive: "codebase-commit-walk", Message: fmt.Sprintf("expected step, got %q", parts[0])})
			continue
		}
		params := ParamsFromFields(parts[1:])
		stepCount++
		if err := ValidateStepParams(params); err != nil {
			diagnostics = append(diagnostics, Diagnostic{Severity: "error", Line: startLine + i + 1, Directive: "codebase-commit-walk", Message: err.Error()})
			continue
		}
	}
	if stepCount == 0 {
		diagnostics = append(diagnostics, Diagnostic{Severity: "error", Line: startLine, Directive: "codebase-commit-walk", Message: "codebase-commit-walk requires at least one step line"})
	}
	return diagnostics
}

func renderMarkdown(src string) (string, error) {
	md := goldmark.New(goldmark.WithRendererOptions(html.WithUnsafe()))
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "", fmt.Errorf("render markdown block: %w", err)
	}
	return buf.String(), nil
}

var h1Re = regexp.MustCompile(`(?m)^#\s+(.+)$`)

func FirstH1(src []byte) string {
	m := h1Re.FindSubmatch(src)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

var fenceOpenRe = regexp.MustCompile("^(?P<fence>```+|~~~+)(codebase-[a-z-]+[^\\n]*)$")

// ParseInfo splits a codebase fence info string into directive name and params.
func ParseInfo(info string) (string, map[string]string) {
	parts := SplitFields(info)
	if len(parts) == 0 {
		return "", map[string]string{}
	}
	return parts[0], ParamsFromFields(parts[1:])
}

// ParamsFromFields parses k=v fields. Fields without '=' are ignored for now;
// the directive schema then reports required/unsupported params.
func ParamsFromFields(fields []string) map[string]string {
	params := map[string]string{}
	for _, p := range fields {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			params[kv[0]] = kv[1]
		}
	}
	return params
}

// SplitFields splits directive info strings while preserving quoted values.
func SplitFields(s string) []string {
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

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
