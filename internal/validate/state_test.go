package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/state"
)

func TestCrossValidate_SpecMode_OK(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "docs", "specs", "004-instruct"), 0755)
	os.WriteFile(filepath.Join(tmp, "docs", "specs", "004-instruct", "spec.md"),
		[]byte("---\nstatus: Draft\n---\n# Spec\n"), 0644)

	s := &state.Focus{Mode: state.ModeSpec, ActiveSpec: "004-instruct"}
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
		[]byte("---\nstatus: Draft\n---\n# Spec\n"), 0644)
	os.WriteFile(filepath.Join(specDir, "plan.md"),
		[]byte("---\nstatus: Draft\n---\n# Plan\n"), 0644)

	s := &state.Focus{Mode: state.ModeSpec, ActiveSpec: "004-instruct"}
	warnings := CrossValidate(tmp, s)

	found := false
	for _, w := range warnings {
		if w.Field == "mode" && strings.Contains(w.Message, "SKIPPED GATE") {
			found = true
			// Bead 9 punch-list B17 (spec 092 Req 11): the advised gate
			// command is the CANONICAL noun-verb form. This kill was
			// previously only cross-package (the instruct fixture);
			// pin it locally so a regression to the deprecated
			// `approve spec` order fails in this package's own tests.
			if !strings.Contains(w.Message, "mindspec spec approve 004-instruct") {
				t.Errorf("SKIPPED GATE warning must advise the canonical `mindspec spec approve <id>`; got %q", w.Message)
			}
			if strings.Contains(w.Message, "approve spec") {
				t.Errorf("SKIPPED GATE warning must not use the deprecated `approve spec` order; got %q", w.Message)
			}
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
		[]byte("---\nstatus: Approved\n---\n# Spec\n"), 0644)

	s := &state.Focus{Mode: state.ModeSpec, ActiveSpec: "004-instruct"}
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

	s := &state.Focus{Mode: state.ModeSpec, ActiveSpec: "004-instruct"}
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

	s := &state.Focus{Mode: state.ModeSpec, ActiveSpec: ""}
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
		[]byte("---\nstatus: Approved\n---\n# Spec\n"), 0644)
	os.WriteFile(filepath.Join(specDir, "plan.md"),
		[]byte("---\nstatus: Draft\n---\n# Plan\n"), 0644)

	s := &state.Focus{Mode: state.ModePlan, ActiveSpec: "004-instruct"}
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
		[]byte("---\nstatus: Draft\n---\n# Spec\n"), 0644)
	os.WriteFile(filepath.Join(specDir, "plan.md"),
		[]byte("---\nstatus: Draft\n---\n# Plan\n"), 0644)

	s := &state.Focus{Mode: state.ModePlan, ActiveSpec: "004-instruct"}
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
		[]byte("---\nstatus: Approved\n---\n# Spec\n"), 0644)

	s := &state.Focus{Mode: state.ModePlan, ActiveSpec: "004-instruct"}
	warnings := CrossValidate(tmp, s)

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

func TestCrossValidate_ImplementMode_NoBead(t *testing.T) {
	tmp := t.TempDir()

	s := &state.Focus{Mode: state.ModeImplement, ActiveSpec: "004-instruct", ActiveBead: ""}
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

	s := &state.Focus{Mode: state.ModeIdle}
	warnings := CrossValidate(tmp, s)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for idle mode, got %v", warnings)
	}
}

// TestReadSpecApprovalStatus pins the spec-approval status source after spec
// 108 R6: the value now comes from the YAML frontmatter `status:` field via
// SpecStatusAt (the deleted readSpecApprovalStatus prose scan is gone). Case is
// preserved; the CrossValidate callers compare case-insensitively.
func TestReadSpecApprovalStatus(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"approved", "---\nstatus: Approved\n---\n# Spec\n", "Approved"},
		{"draft", "---\nstatus: Draft\n---\n# Spec\n", "Draft"},
		{"no frontmatter", "# Spec\n\n## Goal\n\nSomething\n", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}
			got := SpecStatusAt(specDir)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSpecApprovalStatusFromFrontmatter is the spec 108 R6 ZFC-fix proof: when
// the YAML frontmatter `status:` field and the `## Approval` prose disagree,
// the frontmatter value decides. The prose scan that previously drove the
// approval gate (readSpecApprovalStatus) is deleted, so a stale/contradictory
// `## Approval` block can no longer flip the declared status.
func TestSpecApprovalStatusFromFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			// Frontmatter says Draft; prose falsely claims APPROVED.
			name:    "frontmatter draft overrides approved prose",
			content: "---\nstatus: Draft\n---\n# Spec\n\n## Approval\n\n- **Status**: APPROVED\n",
			want:    "Draft",
		},
		{
			// Frontmatter says Approved; prose still says DRAFT.
			name:    "frontmatter approved overrides draft prose",
			content: "---\nstatus: Approved\n---\n# Spec\n\n## Approval\n\n- **Status**: DRAFT\n",
			want:    "Approved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}
			if got := SpecStatusAt(specDir); got != tt.want {
				t.Errorf("SpecStatusAt = %q, want %q (frontmatter must decide over prose)", got, tt.want)
			}
		})
	}
}
