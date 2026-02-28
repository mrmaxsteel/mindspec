package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrossValidate_SpecMode_OK(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "docs", "specs", "004-instruct"), 0755)
	os.WriteFile(filepath.Join(tmp, "docs", "specs", "004-instruct", "spec.md"),
		[]byte("# Spec\n\n## Approval\n\n- **Status**: DRAFT\n"), 0644)

	s := &Focus{Mode: ModeSpec, ActiveSpec: "004-instruct"}
	warnings := CrossValidate(tmp, s)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestCrossValidate_SpecMode_PlanExistsSkippedGate(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "004-instruct")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Spec\n\n## Approval\n\n- **Status**: DRAFT\n"), 0644)
	os.WriteFile(filepath.Join(specDir, "plan.md"),
		[]byte("---\nstatus: Draft\n---\n# Plan\n"), 0644)

	s := &Focus{Mode: ModeSpec, ActiveSpec: "004-instruct"}
	warnings := CrossValidate(tmp, s)

	found := false
	for _, w := range warnings {
		if w.Field == "mode" && strings.Contains(w.Message, "SKIPPED GATE") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SKIPPED GATE warning when plan.md exists in spec mode, got %v", warnings)
	}
}

func TestCrossValidate_SpecMode_AlreadyApproved(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "docs", "specs", "004-instruct"), 0755)
	os.WriteFile(filepath.Join(tmp, "docs", "specs", "004-instruct", "spec.md"),
		[]byte("# Spec\n\n## Approval\n\n- **Status**: APPROVED\n"), 0644)

	s := &Focus{Mode: ModeSpec, ActiveSpec: "004-instruct"}
	warnings := CrossValidate(tmp, s)

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if warnings[0].Field != "mode" {
		t.Errorf("expected field 'mode', got %q", warnings[0].Field)
	}
}

func TestCrossValidate_SpecMode_NoSpec(t *testing.T) {
	tmp := t.TempDir()

	s := &Focus{Mode: ModeSpec, ActiveSpec: "004-instruct"}
	warnings := CrossValidate(tmp, s)

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if warnings[0].Field != "activeSpec" {
		t.Errorf("expected field 'activeSpec', got %q", warnings[0].Field)
	}
}

func TestCrossValidate_SpecMode_NoActiveSpec(t *testing.T) {
	tmp := t.TempDir()

	s := &Focus{Mode: ModeSpec, ActiveSpec: ""}
	warnings := CrossValidate(tmp, s)

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if warnings[0].Field != "activeSpec" {
		t.Errorf("expected field 'activeSpec', got %q", warnings[0].Field)
	}
}

func TestCrossValidate_PlanMode_OK(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "004-instruct")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Spec\n\n## Approval\n\n- **Status**: APPROVED\n"), 0644)
	os.WriteFile(filepath.Join(specDir, "plan.md"),
		[]byte("---\nstatus: Draft\n---\n# Plan\n"), 0644)

	s := &Focus{Mode: ModePlan, ActiveSpec: "004-instruct"}
	warnings := CrossValidate(tmp, s)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestCrossValidate_PlanMode_SpecNotApproved(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "004-instruct")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Spec\n\n## Approval\n\n- **Status**: DRAFT\n"), 0644)
	os.WriteFile(filepath.Join(specDir, "plan.md"),
		[]byte("---\nstatus: Draft\n---\n# Plan\n"), 0644)

	s := &Focus{Mode: ModePlan, ActiveSpec: "004-instruct"}
	warnings := CrossValidate(tmp, s)

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

func TestCrossValidate_PlanMode_NoPlan(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "004-instruct")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Spec\n\n## Approval\n\n- **Status**: APPROVED\n"), 0644)

	s := &Focus{Mode: ModePlan, ActiveSpec: "004-instruct"}
	warnings := CrossValidate(tmp, s)

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

func TestCrossValidate_ImplementMode_NoBead(t *testing.T) {
	tmp := t.TempDir()

	s := &Focus{Mode: ModeImplement, ActiveSpec: "004-instruct", ActiveBead: ""}
	warnings := CrossValidate(tmp, s)

	hasBeadWarning := false
	for _, w := range warnings {
		if w.Field == "activeBead" {
			hasBeadWarning = true
		}
	}
	if !hasBeadWarning {
		t.Error("expected activeBead warning")
	}
}

func TestCrossValidate_IdleMode(t *testing.T) {
	tmp := t.TempDir()

	s := &Focus{Mode: ModeIdle}
	warnings := CrossValidate(tmp, s)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for idle mode, got %v", warnings)
	}
}

func TestReadSpecApprovalStatus(t *testing.T) {
	tmp := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"approved", "# Spec\n\n## Approval\n\n- **Status**: APPROVED\n", "APPROVED"},
		{"draft", "# Spec\n\n## Approval\n\n- **Status**: DRAFT\n", "DRAFT"},
		{"no approval section", "# Spec\n\n## Goal\n\nSomething\n", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmp, tt.name+".md")
			os.WriteFile(path, []byte(tt.content), 0644)

			got := readSpecApprovalStatus(path)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadPlanFrontmatterStatus(t *testing.T) {
	tmp := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"draft", "---\nstatus: Draft\nspec_id: 004\n---\n# Plan\n", "Draft"},
		{"approved", "---\nstatus: Approved\napproved_at: 2026-02-12\n---\n# Plan\n", "Approved"},
		{"no frontmatter", "# Plan\n\nSome content\n", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmp, tt.name+".md")
			os.WriteFile(path, []byte(tt.content), 0644)

			got := readPlanFrontmatterStatus(path)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
