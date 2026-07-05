// Package reviewwidgets defines the supported review Markdown directives and
// their parameter contracts.
//
// It is intentionally small and dependency-free: Go renderers, strict validators,
// tests, and eventually generated TypeScript types should all be able to depend
// on this single source of truth instead of each maintaining a private directive
// list.
package reviewwidgets

import (
	"fmt"
	"sort"
	"strings"
)

// Definition describes one top-level fenced Markdown directive such as
// codebase-snippet or codebase-diff-stats.
type Definition struct {
	Name          string
	Required      []string
	Optional      []string
	CommitRefKeys []string
	RequiresSym   bool
	RequiresFile  bool
	Description   string
}

// StepDefinition describes one codebase-commit-walk step kind.
type StepDefinition struct {
	Kind          string
	Required      []string
	Optional      []string
	CommitRefKeys []string
	RequiresSym   bool
	Description   string
}

var definitions = []Definition{
	{
		Name:        "codebase-snippet",
		Required:    []string{"sym"},
		Optional:    []string{"commit", "dedent", "kind"},
		RequiresSym: true,
		Description: "Render a source snippet for a full sym: symbol ID.",
	},
	{
		Name:        "codebase-signature",
		Required:    []string{"sym"},
		Optional:    []string{"commit"},
		RequiresSym: true,
		Description: "Render a symbol signature.",
	},
	{
		Name:        "codebase-doc",
		Required:    []string{"sym"},
		Optional:    []string{"commit"},
		RequiresSym: true,
		Description: "Render a symbol documentation comment.",
	},
	{
		Name:         "codebase-file",
		Required:     []string{"path"},
		Optional:     []string{"range"},
		RequiresFile: true,
		Description:  "Render source text from an indexed file path.",
	},
	{
		Name:          "codebase-diff",
		Required:      []string{"sym", "from", "to"},
		CommitRefKeys: []string{"from", "to"},
		RequiresSym:   true,
		Description:   "Render a symbol diff between two indexed commit refs.",
	},
	{
		Name:          "codebase-diff-stats",
		Required:      []string{"from", "to"},
		CommitRefKeys: []string{"from", "to"},
		Description:   "Render repository-level diff statistics between two indexed commit refs.",
	},
	{
		Name:          "codebase-changed-files",
		Required:      []string{"from", "to"},
		CommitRefKeys: []string{"from", "to"},
		Description:   "Render changed files between two indexed commit refs.",
	},
	{
		Name:        "codebase-symbol-history",
		Required:    []string{"sym"},
		Optional:    []string{"limit"},
		RequiresSym: true,
		Description: "Render history for a symbol.",
	},
	{
		Name:          "codebase-impact",
		Required:      []string{"sym"},
		Optional:      []string{"commit", "depth", "dir"},
		CommitRefKeys: []string{"commit"},
		RequiresSym:   true,
		Description:   "Render callers/callees impact around a symbol.",
	},
	{
		Name:          "codebase-annotation",
		Required:      []string{"sym"},
		Optional:      []string{"commit", "lines", "note"},
		CommitRefKeys: []string{"commit"},
		RequiresSym:   true,
		Description:   "Render an annotated snippet around a symbol.",
	},
	{
		Name:          "codebase-commit-walk",
		Optional:      []string{"commit", "from", "title", "to"},
		CommitRefKeys: []string{"commit", "from", "to"},
		Description:   "Render an ordered guided walkthrough with typed child steps.",
	},
}

var stepDefinitions = []StepDefinition{
	{
		Kind:        "overview",
		Optional:    []string{"body", "title"},
		Description: "Prose overview step.",
	},
	{
		Kind:        "note",
		Optional:    []string{"body", "title"},
		Description: "Prose note step.",
	},
	{
		Kind:          "diff-stats",
		Optional:      []string{"from", "title", "to"},
		CommitRefKeys: []string{"from", "to"},
		Description:   "Nested diff-stats widget step; inherits top-level refs when absent.",
	},
	{
		Kind:        "symbol",
		Required:    []string{"sym"},
		Optional:    []string{"commit", "title"},
		RequiresSym: true,
		Description: "Nested symbol inspection step.",
	},
	{
		Kind:          "diff",
		Required:      []string{"sym"},
		Optional:      []string{"from", "title", "to"},
		CommitRefKeys: []string{"from", "to"},
		RequiresSym:   true,
		Description:   "Nested symbol diff step; inherits top-level refs when absent.",
	},
	{
		Kind:          "impact",
		Required:      []string{"sym"},
		Optional:      []string{"commit", "depth", "dir", "title"},
		CommitRefKeys: []string{"commit"},
		RequiresSym:   true,
		Description:   "Nested impact step.",
	},
}

var definitionByName = indexDefinitions(definitions)
var stepDefinitionByKind = indexStepDefinitions(stepDefinitions)

func indexDefinitions(defs []Definition) map[string]Definition {
	out := make(map[string]Definition, len(defs))
	for _, def := range defs {
		out[def.Name] = def
	}
	return out
}

func indexStepDefinitions(defs []StepDefinition) map[string]StepDefinition {
	out := make(map[string]StepDefinition, len(defs))
	for _, def := range defs {
		out[def.Kind] = def
	}
	return out
}

// Definitions returns the supported top-level directives in stable name order.
func Definitions() []Definition {
	out := append([]Definition(nil), definitions...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// StepDefinitions returns supported commit-walk step kinds in stable kind order.
func StepDefinitions() []StepDefinition {
	out := append([]StepDefinition(nil), stepDefinitions...)
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

func Lookup(name string) (Definition, bool) { def, ok := definitionByName[name]; return def, ok }
func LookupStep(kind string) (StepDefinition, bool) {
	def, ok := stepDefinitionByKind[kind]
	return def, ok
}

// ValidateParams enforces required and supported params for a top-level directive.
func ValidateParams(name string, params map[string]string) error {
	def, ok := Lookup(name)
	if !ok {
		return fmt.Errorf("unknown directive %q", name)
	}
	return validateParamSet(name, params, def.Required, def.Optional)
}

// ValidateStepParams enforces required and supported params for a commit-walk step.
// The params map must include kind= so error messages can point to the full step.
func ValidateStepParams(params map[string]string) error {
	kind := strings.TrimSpace(params["kind"])
	if kind == "" {
		return fmt.Errorf("missing kind=")
	}
	def, ok := LookupStep(kind)
	if !ok {
		return fmt.Errorf("unknown commit-walk step kind %q", kind)
	}
	required := append([]string{"kind"}, def.Required...)
	optional := append([]string(nil), def.Optional...)
	return validateParamSet("commit-walk step "+kind, params, required, optional)
}

func validateParamSet(label string, params map[string]string, required, optional []string) error {
	allowed := map[string]bool{}
	for _, key := range required {
		allowed[key] = true
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for _, key := range required {
		if strings.TrimSpace(params[key]) == "" {
			return fmt.Errorf("%s requires %s=", label, key)
		}
	}
	var unsupported []string
	for key := range params {
		if !allowed[key] {
			unsupported = append(unsupported, key)
		}
	}
	if len(unsupported) > 0 {
		sort.Strings(unsupported)
		return fmt.Errorf("%s has unsupported param(s): %s", label, strings.Join(unsupported, ", "))
	}
	return nil
}
