package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// --- Vague criterion check (removed as ZFC violation) ---
//
// The deterministic `criteria-vague` warning was removed: it encoded an
// English-only keyword blocklist ("works correctly", "is fast", "properly
// handles", …), which is exactly the kind of keyword-based semantic
// classification Yegge flags as a Zero Framework Cognition violation.
// Criterion quality is now judged at spec-approve time by the AI reviewer.
// TestValidateSpec_VagueCriteriaNotFlagged below pins this behaviour.

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

// TestValidateSpec_GrillDeferredMarkerBlocks pins the mindspec-0uur backstop
// contract: the ms-spec-grill bare-headless deferral marker is an UNCHECKED
// open question, and checkOpenQuestions must hard-ERROR on it — the deferral
// deliberately blocks spec approval until a human (or a documented panel-cited
// resolution, per ms-spec-approve) resolves it. Weakening this to a warning
// would let an ungrilled spec through the approve gate silently.
func TestValidateSpec_GrillDeferredMarkerBlocks(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	os.MkdirAll(specDir, 0755)

	// The exact marker line the ms-spec-create/ms-spec-grill session
	// disposition writes in a bare-headless session.
	const marker = "- [ ] grill deferred: headless session — run /ms-spec-grill interactively before approval."

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

` + marker + `

## Approval

- **Status**: DRAFT
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0644)

	r := ValidateSpec(tmp, "999-test")
	if !r.HasFailures() {
		t.Error("expected the unchecked grill-deferred marker to fail validation")
	}

	found := false
	for _, issue := range r.Issues {
		if issue.Name == "open-question" && strings.Contains(issue.Message, "grill deferred: headless session") {
			if issue.Severity != SevError {
				t.Errorf("grill-deferred marker must be ERROR severity, got %v", issue.Severity)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("expected an open-question ERROR carrying the grill-deferred marker text")
	}
}

// TestValidateSpec_VagueCriteriaNotFlagged confirms the validator no longer
// fires a `criteria-vague` warning on English phrases that used to match the
// deleted keyword list. Reinstating the check would be a regression to a ZFC
// violation.
func TestValidateSpec_VagueCriteriaNotFlagged(t *testing.T) {
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

	for _, issue := range r.Issues {
		if issue.Name == "criteria-vague" {
			t.Errorf("criteria-vague must not be emitted (ZFC): %s", issue.Message)
		}
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

func TestValidateSpec_LifecycleBindingWarn(t *testing.T) {
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

None

## Approval

- **Status**: APPROVED
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0644)

	// No lifecycle.yaml — should warn about missing lifecycle binding.
	r := ValidateSpec(tmp, "999-test")
	found := false
	for _, issue := range r.Issues {
		if issue.Name == "lifecycle-binding" {
			found = true
			if issue.Severity != SevWarning {
				t.Errorf("expected warning severity, got %s", issue.Severity)
			}
		}
	}
	if !found {
		t.Error("expected lifecycle-binding warning when lifecycle.yaml is absent")
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

// TestResult_FormatText_EscapesHostileTarget is the spec 120 final-review
// O3-1 regression: FormatText's header lines render r.TargetID, which is
// set from the ungated CLI arg BEFORE the SpecID/BeadID gates run (spec.go,
// plan.go, adr_divergence.go) and survives intact on the gate-FAIL path. A
// control-bearing target must therefore be termsafe-escaped at BOTH render
// sites (the all-checks-passed header and the issue header) or it forges
// extra terminal lines through the very function spec 120 names as the
// terminal-render choke point.
func TestResult_FormatText_EscapesHostileTarget(t *testing.T) {
	hostile := "120-x\n[FORGED] fake terminal line"
	quoted := `"120-x\n[FORGED] fake terminal line"`

	t.Run("issue header", func(t *testing.T) {
		r := &Result{SubCommand: "spec", TargetID: hostile}
		r.AddError("specid", "not a valid spec ID")
		out := r.FormatText()
		if strings.Contains(out, "\n[FORGED]") {
			t.Fatalf("hostile TargetID forged a terminal line through the issue header:\n%s", out)
		}
		if !strings.Contains(out, quoted) {
			t.Errorf("expected termsafe-quoted target %s in output, got:\n%s", quoted, out)
		}
	})

	t.Run("all-checks-passed header", func(t *testing.T) {
		r := &Result{SubCommand: "spec", TargetID: hostile}
		out := r.FormatText()
		if strings.Contains(out, "\n[FORGED]") {
			t.Fatalf("hostile TargetID forged a terminal line through the all-passed header:\n%s", out)
		}
		if !strings.Contains(out, quoted) {
			t.Errorf("expected termsafe-quoted target %s in output, got:\n%s", quoted, out)
		}
	})

	t.Run("clean target renders byte-identical", func(t *testing.T) {
		r := &Result{SubCommand: "spec", TargetID: "005-next"}
		if got := r.FormatText(); !strings.HasPrefix(got, "005-next: all checks passed\n") {
			t.Errorf("clean all-passed header changed: %q", got)
		}
		r.AddError("test", "msg")
		if got := r.FormatText(); !strings.HasPrefix(got, "005-next: 1 issue(s) found\n") {
			t.Errorf("clean issue header changed: %q", got)
		}
	})
}
