package resolve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/state"
)

// --- Multi-spec integration tests ---

func TestActiveSpecs_MultiSpec_Independent(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".mindspec", "docs", "specs")

	// Alpha in implement, Beta in spec — both active, independent phases
	specA := filepath.Join(specsDir, "038-alpha")
	os.MkdirAll(specA, 0755)
	state.WriteLifecycle(specA, &state.Lifecycle{Phase: state.ModeImplement})

	specB := filepath.Join(specsDir, "039-beta")
	os.MkdirAll(specB, 0755)
	state.WriteLifecycle(specB, &state.Lifecycle{Phase: state.ModeSpec})

	active, err := ActiveSpecs(root)
	if err != nil {
		t.Fatalf("ActiveSpecs() error: %v", err)
	}

	if len(active) != 2 {
		t.Fatalf("expected 2 active specs, got %d", len(active))
	}

	// Verify independent phases
	phases := map[string]string{}
	for _, a := range active {
		phases[a.SpecID] = a.Mode
	}
	if phases["038-alpha"] != state.ModeImplement {
		t.Errorf("alpha: got %q, want %q", phases["038-alpha"], state.ModeImplement)
	}
	if phases["039-beta"] != state.ModeSpec {
		t.Errorf("beta: got %q, want %q", phases["039-beta"], state.ModeSpec)
	}
}

func TestActiveSpecsWorktreeOnly(t *testing.T) {
	root := t.TempDir()
	// Create .mindspec/docs/specs with no specs in it
	os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs"), 0755)

	// Create a spec that only exists in a worktree (not in main repo)
	specID := "099-worktree-only"
	wtSpecDir := filepath.Join(root, ".worktrees", "worktree-spec-"+specID,
		".mindspec", "docs", "specs", specID)
	os.MkdirAll(wtSpecDir, 0755)
	state.WriteLifecycle(wtSpecDir, &state.Lifecycle{Phase: state.ModeSpec})

	active, err := ActiveSpecs(root)
	if err != nil {
		t.Fatalf("ActiveSpecs() error: %v", err)
	}

	if len(active) != 1 {
		t.Fatalf("expected 1 active spec from worktree, got %d: %+v", len(active), active)
	}
	if active[0].SpecID != specID {
		t.Errorf("expected specID %q, got %q", specID, active[0].SpecID)
	}
	if active[0].Mode != state.ModeSpec {
		t.Errorf("expected mode %q, got %q", state.ModeSpec, active[0].Mode)
	}
}

func TestActiveSpecsWorktreeWinsOverMain(t *testing.T) {
	root := t.TempDir()
	specID := "099-dual"

	// Create spec in main repo with phase=spec
	mainSpecDir := filepath.Join(root, ".mindspec", "docs", "specs", specID)
	os.MkdirAll(mainSpecDir, 0755)
	state.WriteLifecycle(mainSpecDir, &state.Lifecycle{Phase: state.ModeSpec})

	// Create same spec in worktree with phase=plan (should win)
	wtSpecDir := filepath.Join(root, ".worktrees", "worktree-spec-"+specID,
		".mindspec", "docs", "specs", specID)
	os.MkdirAll(wtSpecDir, 0755)
	state.WriteLifecycle(wtSpecDir, &state.Lifecycle{Phase: state.ModePlan})

	active, err := ActiveSpecs(root)
	if err != nil {
		t.Fatalf("ActiveSpecs() error: %v", err)
	}

	if len(active) != 1 {
		t.Fatalf("expected 1 active spec (deduplicated), got %d: %+v", len(active), active)
	}
	if active[0].Mode != state.ModePlan {
		t.Errorf("worktree should win: expected mode %q, got %q", state.ModePlan, active[0].Mode)
	}
}

func TestAmbiguousTarget_RefusesToGuess(t *testing.T) {
	err := &ErrAmbiguousTarget{
		Active: []SpecStatus{
			{SpecID: "038-alpha", Mode: "implement"},
			{SpecID: "039-beta", Mode: "spec"},
		},
	}

	msg := err.Error()
	if !strings.Contains(msg, "--spec") {
		t.Errorf("ambiguous error should mention --spec: %s", msg)
	}
	if !strings.Contains(msg, "038-alpha") {
		t.Errorf("ambiguous error should list 038-alpha: %s", msg)
	}
	if !strings.Contains(msg, "039-beta") {
		t.Errorf("ambiguous error should list 039-beta: %s", msg)
	}
}

func TestExplicitTarget_BypassesAmbiguity(t *testing.T) {
	got, err := ResolveTarget("/nonexistent", "038-alpha")
	if err != nil {
		t.Fatalf("explicit target should not error: %v", err)
	}
	if got != "038-alpha" {
		t.Errorf("got %q, want %q", got, "038-alpha")
	}
}

func TestSingleActiveSpec_AutoSelects(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".mindspec", "docs", "specs")

	specDir := filepath.Join(specsDir, "038-alpha")
	os.MkdirAll(specDir, 0755)
	state.WriteLifecycle(specDir, &state.Lifecycle{Phase: state.ModePlan})

	got, err := ResolveTarget(root, "")
	if err != nil {
		t.Fatalf("ResolveTarget() error: %v", err)
	}
	if got != "038-alpha" {
		t.Errorf("auto-select: got %q, want %q", got, "038-alpha")
	}
}

func TestMixedRepo_ActiveAndDone(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".mindspec", "docs", "specs")

	// Active spec
	activeDir := filepath.Join(specsDir, "038-active")
	os.MkdirAll(activeDir, 0755)
	state.WriteLifecycle(activeDir, &state.Lifecycle{Phase: state.ModeImplement})

	// Done spec
	doneDir := filepath.Join(specsDir, "005-done")
	os.MkdirAll(doneDir, 0755)
	state.WriteLifecycle(doneDir, &state.Lifecycle{Phase: "done"})

	// Legacy spec (no lifecycle.yaml)
	legacyDir := filepath.Join(specsDir, "001-legacy")
	os.MkdirAll(legacyDir, 0755)

	active, err := ActiveSpecs(root)
	if err != nil {
		t.Fatalf("ActiveSpecs() error: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active, got %d: %+v", len(active), active)
	}
	if active[0].SpecID != "038-active" {
		t.Errorf("got %q, want %q", active[0].SpecID, "038-active")
	}
}

func TestLegacyRepo_NoLifecycle(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".mindspec"), 0755)
	os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs"), 0755)

	_, err := ResolveTarget(root, "")
	if err == nil {
		t.Fatal("expected error when no active specs exist")
	}
	if !strings.Contains(err.Error(), "--spec") {
		t.Errorf("error should suggest --spec flag, got: %v", err)
	}
}

// --- Focus cursor tests ---

func TestFocusCursor_WritesReadBack(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".mindspec"), 0755)

	mc := &state.Focus{
		Mode:       state.ModeImplement,
		ActiveSpec: "038-test",
		ActiveBead: "bead-1",
	}
	if err := state.WriteFocus(root, mc); err != nil {
		t.Fatalf("WriteFocus failed: %v", err)
	}

	got, err := state.ReadFocus(root)
	if err != nil {
		t.Fatalf("ReadFocus failed: %v", err)
	}
	if got.Mode != state.ModeImplement {
		t.Errorf("mode: got %q, want %q", got.Mode, state.ModeImplement)
	}
	if got.ActiveSpec != "038-test" {
		t.Errorf("activeSpec: got %q, want %q", got.ActiveSpec, "038-test")
	}
	if got.ActiveBead != "bead-1" {
		t.Errorf("activeBead: got %q, want %q", got.ActiveBead, "bead-1")
	}
}

func TestFocusCursor_UpdateOnNext(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".mindspec"), 0755)

	state.WriteFocus(root, &state.Focus{
		Mode:       state.ModeImplement,
		ActiveSpec: "038-test",
		ActiveBead: "bead-1",
	})

	state.WriteFocus(root, &state.Focus{
		Mode:       state.ModeImplement,
		ActiveSpec: "038-test",
		ActiveBead: "bead-2",
	})

	got, err := state.ReadFocus(root)
	if err != nil {
		t.Fatalf("ReadFocus failed: %v", err)
	}
	if got.ActiveBead != "bead-2" {
		t.Errorf("activeBead: got %q, want %q", got.ActiveBead, "bead-2")
	}
}

func TestFormatActiveList_Ordering(t *testing.T) {
	specs := []SpecStatus{
		{SpecID: "039-beta", Mode: "spec"},
		{SpecID: "038-alpha", Mode: "implement"},
		{SpecID: "040-gamma", Mode: "plan"},
	}

	output := FormatActiveList(specs)

	if !strings.Contains(output, "Active specs (3)") {
		t.Errorf("expected 'Active specs (3)' header, got: %s", output)
	}
	for _, id := range []string{"038-alpha", "039-beta", "040-gamma"} {
		if !strings.Contains(output, id) {
			t.Errorf("expected %q in output: %s", id, output)
		}
	}
}
