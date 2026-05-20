package validate

import (
	"testing"
)

func TestClassifyChanges(t *testing.T) {
	files := []string{
		"internal/validate/spec.go",
		"internal/validate/spec_test.go",
		"cmd/mindspec/validate.go",
		"docs/domains/workflow/interfaces.md",
		"CLAUDE.md",
		"go.mod",
	}

	source, docs := classifyChanges(files)

	// spec_test.go should NOT be in source (test files excluded)
	// go.mod should NOT be in either
	if len(source) != 2 {
		t.Errorf("expected 2 source files, got %d: %v", len(source), source)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 doc files, got %d: %v", len(docs), docs)
	}
}

func TestIsDocFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"docs/core/MODES.md", true},
		{"docs/domains/workflow/overview.md", true},
		{"CLAUDE.md", true},
		{"AGENTS.md", true},
		{".mindspec/docs/core/ARCHITECTURE.md", true},
		{"internal/validate/spec.go", false},
		{"cmd/mindspec/validate.go", false},
		{"go.mod", false},
	}
	for _, tt := range tests {
		if isDocFile(tt.path) != tt.expected {
			t.Errorf("isDocFile(%q) = %v, want %v", tt.path, !tt.expected, tt.expected)
		}
	}
}

func TestIsSourceFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"internal/validate/spec.go", true},
		{"cmd/mindspec/validate.go", true},
		{"internal/validate/spec_test.go", false},
		{"docs/core/MODES.md", false},
		{"go.mod", false},
	}
	for _, tt := range tests {
		if isSourceFile(tt.path) != tt.expected {
			t.Errorf("isSourceFile(%q) = %v, want %v", tt.path, !tt.expected, tt.expected)
		}
	}
}

func TestParseChangedFiles(t *testing.T) {
	output := "internal/validate/spec.go\ncmd/mindspec/validate.go\ndocs/core/MODES.md\n"
	files := ParseChangedFiles(output)
	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}
}

func TestParseChangedFiles_Empty(t *testing.T) {
	files := ParseChangedFiles("")
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestCheckInternalPackages_WithDomainDocs(t *testing.T) {
	r := &Result{SubCommand: "docs"}
	source := []string{"internal/validate/spec.go"}
	docs := []string{"docs/domains/workflow/interfaces.md"}

	checkInternalPackages(r, source, docs)
	if r.HasFailures() {
		t.Error("expected no failures when domain docs updated")
	}
	// Should also have no warnings
	if len(r.Issues) > 0 {
		t.Errorf("expected no issues, got %d", len(r.Issues))
	}
}

func TestCheckInternalPackages_WithoutDomainDocs(t *testing.T) {
	r := &Result{SubCommand: "docs"}
	source := []string{"internal/validate/spec.go"}
	docs := []string{"CLAUDE.md"}

	checkInternalPackages(r, source, docs)
	found := false
	for _, issue := range r.Issues {
		if issue.Name == "internal-docs" {
			found = true
		}
	}
	if !found {
		t.Error("expected internal-docs warning")
	}
}

func TestCheckCmdChanges_WithRelevantDocs(t *testing.T) {
	r := &Result{SubCommand: "docs"}
	source := []string{"cmd/mindspec/validate.go"}
	docs := []string{"CLAUDE.md"}

	checkCmdChanges(r, source, docs)
	if len(r.Issues) > 0 {
		t.Error("expected no issues when CLAUDE.md updated")
	}
}

func TestCheckCmdChanges_WithoutRelevantDocs(t *testing.T) {
	r := &Result{SubCommand: "docs"}
	source := []string{"cmd/mindspec/validate.go"}
	docs := []string{"docs/domains/workflow/overview.md"}

	checkCmdChanges(r, source, docs)
	found := false
	for _, issue := range r.Issues {
		if issue.Name == "cmd-docs" {
			found = true
		}
	}
	if !found {
		t.Error("expected cmd-docs warning")
	}
}

func TestCheckCmdChanges_NoCmdFiles(t *testing.T) {
	r := &Result{SubCommand: "docs"}
	source := []string{"internal/validate/spec.go"}
	docs := []string{}

	checkCmdChanges(r, source, docs)
	if len(r.Issues) > 0 {
		t.Error("expected no issues when no cmd files changed")
	}
}

// TestValidateSpecArtifactSync covers the spec-doc-sync lane: a spec.md
// change must be accompanied in the same diff by plan.md, an ADR, or any
// sibling artifact under the same spec directory.
func TestValidateSpecArtifactSync(t *testing.T) {
	cases := []struct {
		name      string
		all       []string
		wantError bool
	}{
		{
			name:      "spec.md only -> error",
			all:       []string{".mindspec/docs/specs/086-doc-sync/spec.md"},
			wantError: true,
		},
		{
			name: "spec.md + plan.md -> no error",
			all: []string{
				".mindspec/docs/specs/086-doc-sync/spec.md",
				".mindspec/docs/specs/086-doc-sync/plan.md",
			},
			wantError: false,
		},
		{
			name: "spec.md + sibling notes.md -> no error",
			all: []string{
				".mindspec/docs/specs/086-doc-sync/spec.md",
				".mindspec/docs/specs/086-doc-sync/notes.md",
			},
			wantError: false,
		},
		{
			name: "spec.md + ADR .md -> no error",
			all: []string{
				".mindspec/docs/specs/086-doc-sync/spec.md",
				".mindspec/docs/adr/ADR-0031-doc-sync-gate.md",
			},
			wantError: false,
		},
		{
			name:      "no spec.md change -> no error",
			all:       []string{"internal/validate/spec.go", "docs/core/MODES.md"},
			wantError: false,
		},
		{
			name: "spec.md + unrelated source file (no companion) -> error",
			all: []string{
				".mindspec/docs/specs/086-doc-sync/spec.md",
				"internal/validate/docsync.go",
			},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Result{SubCommand: "docs"}
			changes := ClassifiedChanges{All: tc.all}
			validateSpecArtifactSync(r, changes)

			found := false
			for _, issue := range r.Issues {
				if issue.Name == "spec-doc-sync" {
					if issue.Severity != SevError {
						t.Errorf("spec-doc-sync severity = %v, want %v", issue.Severity, SevError)
					}
					found = true
				}
			}
			if found != tc.wantError {
				t.Errorf("spec-doc-sync error present = %v, want %v (issues=%+v)", found, tc.wantError, r.Issues)
			}
		})
	}
}

// TestPromotedLaneSeverities pins the spec-086 Req 7 contract: the
// doc-sync and internal-docs lanes are ERRORS, while the operator-docs
// (cmd-docs) lane stays a WARNING.
func TestPromotedLaneSeverities(t *testing.T) {
	// doc-sync (line ~37 in docsync.go): source changed, no docs.
	// We can't easily synthesize a *Result via ValidateDocs without an
	// executor, so we exercise the helpers + struct directly to confirm
	// severity selection.
	r := &Result{SubCommand: "docs"}
	checkInternalPackages(r, []string{"internal/validate/spec.go"}, nil)
	for _, issue := range r.Issues {
		if issue.Name == "internal-docs" && issue.Severity != SevError {
			t.Errorf("internal-docs severity = %v, want %v", issue.Severity, SevError)
		}
	}

	r2 := &Result{SubCommand: "docs"}
	checkCmdChanges(r2, []string{"cmd/mindspec/validate.go"}, []string{"docs/domains/workflow/overview.md"})
	for _, issue := range r2.Issues {
		if issue.Name == "cmd-docs" && issue.Severity != SevWarning {
			t.Errorf("cmd-docs severity = %v, want %v (Req 7 contract)", issue.Severity, SevWarning)
		}
	}
}

// TestOperatorDocsAdditiveAcceptSet verifies the operator-docs lane accepts
// the spec-086 additive set (.mindspec/docs/user/** and
// .mindspec/docs/core/USAGE.md) in addition to the existing accept set
// (CLAUDE.md, CONVENTIONS.md). Severity stays at warning per Req 7.
func TestOperatorDocsAdditiveAcceptSet(t *testing.T) {
	cases := []struct {
		name        string
		source      []string
		docs        []string
		wantWarning bool
	}{
		{
			name:        "CLAUDE.md satisfies lane (existing, preserved)",
			source:      []string{"cmd/mindspec/validate.go"},
			docs:        []string{"CLAUDE.md"},
			wantWarning: false,
		},
		{
			name:        "CONVENTIONS.md satisfies lane (existing, preserved)",
			source:      []string{"cmd/mindspec/validate.go"},
			docs:        []string{"docs/CONVENTIONS.md"},
			wantWarning: false,
		},
		{
			name:        ".mindspec/docs/user/** satisfies lane (additive)",
			source:      []string{"cmd/mindspec/validate.go"},
			docs:        []string{".mindspec/docs/user/getting-started.md"},
			wantWarning: false,
		},
		{
			name:        ".mindspec/docs/core/USAGE.md satisfies lane (additive)",
			source:      []string{"cmd/mindspec/validate.go"},
			docs:        []string{".mindspec/docs/core/USAGE.md"},
			wantWarning: false,
		},
		{
			name:        "no operator-doc touch warns",
			source:      []string{"cmd/mindspec/validate.go"},
			docs:        []string{"docs/domains/workflow/overview.md"},
			wantWarning: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Result{SubCommand: "docs"}
			checkCmdChanges(r, tc.source, tc.docs)

			found := false
			for _, issue := range r.Issues {
				if issue.Name == "cmd-docs" {
					found = true
				}
			}
			if found != tc.wantWarning {
				t.Errorf("warning present = %v, want %v (issues=%+v)", found, tc.wantWarning, r.Issues)
			}
		})
	}
}
