package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- Result tests ---

func TestResult_HasFailures_NoIssues(t *testing.T) {
	r := &Result{SubCommand: "spec"}
	if r.HasFailures() {
		t.Error("expected no failures for empty result")
	}
}

func TestResult_HasFailures_WithError(t *testing.T) {
	r := &Result{SubCommand: "spec"}
	r.AddError("test", "something broke")
	if !r.HasFailures() {
		t.Error("expected failures when error present")
	}
}

func TestResult_HasFailures_OnlyWarnings(t *testing.T) {
	r := &Result{SubCommand: "spec"}
	r.AddWarning("test", "just a heads up")
	if r.HasFailures() {
		t.Error("expected no failures when only warnings present")
	}
}

func TestResult_ToJSON(t *testing.T) {
	r := &Result{SubCommand: "spec", TargetID: "005-next"}
	r.AddError("test", "error message")
	r.AddWarning("warn", "warning message")

	out, err := r.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["sub_command"] != "spec" {
		t.Errorf("expected sub_command=spec, got %v", parsed["sub_command"])
	}
	if parsed["target_id"] != "005-next" {
		t.Errorf("expected target_id=005-next, got %v", parsed["target_id"])
	}
}

func TestResult_FormatText_NoIssues(t *testing.T) {
	r := &Result{SubCommand: "spec", TargetID: "005-next"}
	out := r.FormatText()
	if out != "005-next: all checks passed\n" {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestResult_FormatText_WithIssues(t *testing.T) {
	r := &Result{SubCommand: "spec", TargetID: "005-next"}
	r.AddError("test", "broken")
	out := r.FormatText()
	if !contains(out, "[ERROR]") || !contains(out, "broken") {
		t.Errorf("expected error in output: %s", out)
	}
}

// --- Vague criterion tests ---

func TestIsVagueCriterion(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"The system works correctly under load", true},
		{"The API is fast enough", true},
		{"The handler properly handles edge cases", true},
		{"Functions as expected in all scenarios", true},
		{"`mindspec validate spec <id>` reports missing/empty required sections", false},
		{"All sub-commands return non-zero exit code on validation failure", false},
		{"make test passes with tests covering each validation check", false},
	}
	for _, tt := range tests {
		result := IsVagueCriterion(tt.text)
		if result != tt.expected {
			t.Errorf("IsVagueCriterion(%q) = %v, want %v", tt.text, result, tt.expected)
		}
	}
}

// --- Spec validation tests ---

func TestValidateSpec_WellFormed(t *testing.T) {
	// Use a real spec from the project
	root := findProjectRoot(t)
	r := ValidateSpec(root, "005-next")
	if r.HasFailures() {
		for _, issue := range r.Issues {
			t.Logf("[%s] %s: %s", issue.Severity, issue.Name, issue.Message)
		}
		t.Error("expected 005-next to pass validation")
	}
}

func TestValidateSpec_MissingSections(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# Spec 999\n\n## Goal\n\nDo something.\n"), 0644)

	r := ValidateSpec(tmp, "999-test")
	if !r.HasFailures() {
		t.Error("expected failures for spec with missing sections")
	}

	// Should report multiple missing sections
	errorCount := 0
	for _, issue := range r.Issues {
		if issue.Severity == SevError {
			errorCount++
		}
	}
	if errorCount < 3 {
		t.Errorf("expected at least 3 errors for minimal spec, got %d", errorCount)
	}
}

func TestValidateSpec_UnresolvedOpenQuestions(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)

	spec := `# Spec 999: Test

## Goal

Do something useful.

## Impacted Domains

- **core**: something

## ADR Touchpoints

- [ADR-0001](../../adr/ADR-0001.md): relevant

## Requirements

1. First requirement
2. Second requirement

## Scope

### In Scope
- something

### Out of Scope
- something else

## Acceptance Criteria

- [ ] First criterion
- [ ] Second criterion
- [ ] Third criterion

## Open Questions

- [ ] What about this thing?
- [ ] And this other thing?

## Approval

- **Status**: DRAFT
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0644)

	r := ValidateSpec(tmp, "999-test")
	if !r.HasFailures() {
		t.Error("expected failures for unresolved open questions")
	}

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "open-question" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected open-question issue")
	}
}

func TestValidateSpec_VagueCriteria(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)

	spec := `# Spec 999: Test

## Goal

Do something useful.

## Impacted Domains

- **core**: something

## ADR Touchpoints

- [ADR-0001](../../adr/ADR-0001.md): relevant

## Requirements

1. First requirement
2. Second requirement

## Scope

### In Scope
- something

### Out of Scope
- something else

## Acceptance Criteria

- [ ] The system works correctly under all conditions
- [ ] The API is fast enough for production use
- [ ] Third specific criterion about exit codes

## Open Questions

None — all resolved.

## Approval

- **Status**: DRAFT
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0644)

	r := ValidateSpec(tmp, "999-test")

	vagueCount := 0
	for _, issue := range r.Issues {
		if issue.Name == "criteria-vague" {
			vagueCount++
		}
	}
	if vagueCount != 2 {
		t.Errorf("expected 2 vague criteria warnings, got %d", vagueCount)
	}
}

func TestValidateSpec_NonexistentSpec(t *testing.T) {
	r := ValidateSpec("/nonexistent", "does-not-exist")
	if !r.HasFailures() {
		t.Error("expected failure for nonexistent spec")
	}
}

func TestValidateSpec_PlaceholderContent(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)

	// Copy the template as-is (all placeholders)
	spec := `# Spec 999: Test

## Goal

<Brief description of what this spec achieves and the target user outcome>

## Impacted Domains

- <domain-1>: <how it is impacted>

## ADR Touchpoints

- [ADR-NNNN](../../adr/ADR-NNNN.md): <why this ADR is relevant>

## Requirements

1. <Requirement 1>
2. <Requirement 2>

## Scope

### In Scope
- <File or component 1>

### Out of Scope
- <Explicitly excluded items>

## Acceptance Criteria

- [ ] <Specific, measurable criterion 1>
- [ ] <Specific, measurable criterion 2>
- [ ] <Specific, measurable criterion 3>

## Open Questions

- [ ] <Question that must be resolved before planning>

## Approval

- **Status**: DRAFT
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0644)

	r := ValidateSpec(tmp, "999-test")
	if !r.HasFailures() {
		t.Error("expected failures for placeholder-only spec")
	}
}

// --- parseSections tests ---

func TestParseSections(t *testing.T) {
	content := "# Title\n\n## Goal\n\nDo something.\n\n## Requirements\n\n1. First\n2. Second\n"
	sections := parseSections(content)

	if _, ok := sections["Goal"]; !ok {
		t.Error("expected Goal section")
	}
	if _, ok := sections["Requirements"]; !ok {
		t.Error("expected Requirements section")
	}
	if !contains(sections["Goal"], "Do something") {
		t.Errorf("Goal section should contain content: %q", sections["Goal"])
	}
}

// --- helpers ---

func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from test file to find project root (has .mindspec/ or .git)
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".mindspec")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root")
		}
		dir = parent
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstr(s, substr)
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
