package reviewwidgets

import "testing"

func TestDefinitionsContainSupportedDirectives(t *testing.T) {
	want := []string{
		"codebase-annotation",
		"codebase-changed-files",
		"codebase-commit-walk",
		"codebase-diff",
		"codebase-diff-stats",
		"codebase-doc",
		"codebase-file",
		"codebase-impact",
		"codebase-signature",
		"codebase-snippet",
		"codebase-symbol-history",
	}
	got := Definitions()
	if len(got) != len(want) {
		t.Fatalf("got %d definitions, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Fatalf("definition[%d] = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestValidateParams(t *testing.T) {
	tests := []struct {
		name      string
		directive string
		params    map[string]string
		wantErr   string
	}{
		{
			name:      "valid diff",
			directive: "codebase-diff",
			params:    map[string]string{"sym": "sym:example", "from": "HEAD~1", "to": "HEAD"},
		},
		{
			name:      "missing required",
			directive: "codebase-diff",
			params:    map[string]string{"sym": "sym:example", "to": "HEAD"},
			wantErr:   "codebase-diff requires from=",
		},
		{
			name:      "unsupported param",
			directive: "codebase-file",
			params:    map[string]string{"path": "x.go", "sym": "sym:wrong"},
			wantErr:   "codebase-file has unsupported param(s): sym",
		},
		{
			name:      "unknown directive",
			directive: "codebase-magic",
			params:    map[string]string{},
			wantErr:   "unknown directive \"codebase-magic\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParams(tt.directive, tt.params)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateParams() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateParams() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateStepParams(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]string
		wantErr string
	}{
		{
			name:   "overview prose",
			params: map[string]string{"kind": "overview", "title": "Scope", "body": "Read this"},
		},
		{
			name:    "symbol requires sym",
			params:  map[string]string{"kind": "symbol", "title": "Export"},
			wantErr: "commit-walk step symbol requires sym=",
		},
		{
			name:    "unknown kind",
			params:  map[string]string{"kind": "magic"},
			wantErr: "unknown commit-walk step kind \"magic\"",
		},
		{
			name:    "unsupported param",
			params:  map[string]string{"kind": "diff-stats", "sym": "sym:not-used"},
			wantErr: "commit-walk step diff-stats has unsupported param(s): sym",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStepParams(tt.params)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateStepParams() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateStepParams() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
