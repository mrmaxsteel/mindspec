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
		{"GLOSSARY.md", true},
		{".mindspec/docs/core/ARCHITECTURE.md", true},
		{".mindspec/policies.yml", true},
		{"architecture/policies.yml", true},
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
