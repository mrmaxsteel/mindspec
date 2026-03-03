package resolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mindspec/mindspec/internal/state"
)

func TestActiveSpecs_ScansLifecycle(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".mindspec", "docs", "specs")

	// Spec in implement phase — active
	specA := filepath.Join(specsDir, "038-alpha")
	os.MkdirAll(specA, 0755)
	state.WriteLifecycle(specA, &state.Lifecycle{Phase: state.ModeImplement, EpicID: "epic-a"})

	// Spec in spec phase — active
	specB := filepath.Join(specsDir, "039-beta")
	os.MkdirAll(specB, 0755)
	state.WriteLifecycle(specB, &state.Lifecycle{Phase: state.ModeSpec})

	// Spec in idle phase — NOT active
	specC := filepath.Join(specsDir, "040-gamma")
	os.MkdirAll(specC, 0755)
	state.WriteLifecycle(specC, &state.Lifecycle{Phase: state.ModeIdle})

	// Spec done — NOT active
	specD := filepath.Join(specsDir, "041-delta")
	os.MkdirAll(specD, 0755)
	state.WriteLifecycle(specD, &state.Lifecycle{Phase: "done"})

	active, err := ActiveSpecs(root)
	if err != nil {
		t.Fatalf("ActiveSpecs() error: %v", err)
	}

	if len(active) != 2 {
		t.Fatalf("expected 2 active specs, got %d: %+v", len(active), active)
	}

	// Should be sorted by spec ID
	if active[0].SpecID != "038-alpha" {
		t.Errorf("first active spec: got %q, want %q", active[0].SpecID, "038-alpha")
	}
	if active[0].Mode != state.ModeImplement {
		t.Errorf("first active mode: got %q, want %q", active[0].Mode, state.ModeImplement)
	}
	if active[1].SpecID != "039-beta" {
		t.Errorf("second active spec: got %q, want %q", active[1].SpecID, "039-beta")
	}
}

func TestActiveSpecs_NoLifecycle(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".mindspec", "docs", "specs")

	// Spec directory with no lifecycle.yaml — should be skipped
	os.MkdirAll(filepath.Join(specsDir, "005-legacy"), 0755)

	active, err := ActiveSpecs(root)
	if err != nil {
		t.Fatalf("ActiveSpecs() error: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active specs, got %d", len(active))
	}
}

func TestActiveSpecs_EmptyPhase(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".mindspec", "docs", "specs")

	// Spec with empty phase — NOT active
	specDir := filepath.Join(specsDir, "042-empty")
	os.MkdirAll(specDir, 0755)
	state.WriteLifecycle(specDir, &state.Lifecycle{Phase: ""})

	active, err := ActiveSpecs(root)
	if err != nil {
		t.Fatalf("ActiveSpecs() error: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active specs for empty phase, got %d", len(active))
	}
}

func TestActiveSpecs_AllPhases(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".mindspec", "docs", "specs")

	tests := []struct {
		specID     string
		phase      string
		wantActive bool
	}{
		{"002-spec", state.ModeSpec, true},
		{"003-plan", state.ModePlan, true},
		{"004-implement", state.ModeImplement, true},
		{"005-review", state.ModeReview, true},
		{"006-idle", state.ModeIdle, false},
		{"007-done", "done", false},
	}

	for _, tt := range tests {
		specDir := filepath.Join(specsDir, tt.specID)
		os.MkdirAll(specDir, 0755)
		state.WriteLifecycle(specDir, &state.Lifecycle{Phase: tt.phase})
	}

	active, err := ActiveSpecs(root)
	if err != nil {
		t.Fatalf("ActiveSpecs() error: %v", err)
	}

	activeSet := make(map[string]bool)
	for _, a := range active {
		activeSet[a.SpecID] = true
	}

	for _, tt := range tests {
		got := activeSet[tt.specID]
		if got != tt.wantActive {
			t.Errorf("spec %s (phase=%s): active=%v, want %v", tt.specID, tt.phase, got, tt.wantActive)
		}
	}
}

func TestActiveSpecs_SortOrder(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".mindspec", "docs", "specs")

	// Create in reverse order
	for _, id := range []string{"039-beta", "040-gamma", "038-alpha"} {
		specDir := filepath.Join(specsDir, id)
		os.MkdirAll(specDir, 0755)
		state.WriteLifecycle(specDir, &state.Lifecycle{Phase: state.ModeSpec})
	}

	active, err := ActiveSpecs(root)
	if err != nil {
		t.Fatalf("ActiveSpecs() error: %v", err)
	}

	if len(active) != 3 {
		t.Fatalf("expected 3, got %d", len(active))
	}
	if active[0].SpecID != "038-alpha" || active[1].SpecID != "039-beta" || active[2].SpecID != "040-gamma" {
		t.Errorf("not sorted: %v", active)
	}
}

func TestFormatActiveList_Empty(t *testing.T) {
	got := FormatActiveList(nil)
	if got != "No active specs found.\n" {
		t.Errorf("FormatActiveList(nil) = %q", got)
	}
}

func TestFormatActiveList_Multiple(t *testing.T) {
	specs := []SpecStatus{
		{SpecID: "001-alpha", Mode: "spec"},
		{SpecID: "002-beta", Mode: "plan"},
	}
	got := FormatActiveList(specs)
	if got == "No active specs found.\n" {
		t.Error("expected non-empty list")
	}
}

func TestResolveSpecBranch(t *testing.T) {
	got := ResolveSpecBranch("053-drop-state-json")
	if got != "spec/053-drop-state-json" {
		t.Errorf("ResolveSpecBranch() = %q, want %q", got, "spec/053-drop-state-json")
	}
}

func TestResolveWorktree(t *testing.T) {
	got := ResolveWorktree("/project", "053-drop-state-json")
	want := filepath.Join("/project", ".worktrees", "worktree-spec-053-drop-state-json")
	if got != want {
		t.Errorf("ResolveWorktree() = %q, want %q", got, want)
	}
}
