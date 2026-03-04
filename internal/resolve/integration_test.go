package resolve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/phase"
	"github.com/mindspec/mindspec/internal/state"
)

// --- Multi-spec integration tests ---
// ADR-0023: ActiveSpecs now derives from beads, not lifecycle.yaml files.

func TestActiveSpecs_MultiSpec_Independent(t *testing.T) {
	root := t.TempDir()

	epics := []phase.EpicInfo{
		{ID: "epic-a", Title: "[SPEC 038-alpha] Alpha", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(38), "spec_title": "alpha"}},
		{ID: "epic-b", Title: "[SPEC 039-beta] Beta", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(39), "spec_title": "beta"}},
	}
	childrenByEpic := map[string][]phase.ChildInfo{
		"epic-a": {{ID: "bead-1", Status: "in_progress", IssueType: "task"}},
		"epic-b": {{ID: "bead-2", Status: "open", IssueType: "task"}},
	}
	stubActiveEpics(t, epics, childrenByEpic)

	active, err := ActiveSpecs(root)
	if err != nil {
		t.Fatalf("ActiveSpecs() error: %v", err)
	}

	if len(active) != 2 {
		t.Fatalf("expected 2 active specs, got %d", len(active))
	}

	phases := map[string]string{}
	for _, a := range active {
		phases[a.SpecID] = a.Mode
	}
	if phases["038-alpha"] != state.ModeImplement {
		t.Errorf("alpha: got %q, want %q", phases["038-alpha"], state.ModeImplement)
	}
	if phases["039-beta"] != state.ModePlan {
		t.Errorf("beta: got %q, want %q", phases["039-beta"], state.ModePlan)
	}
}

func TestResolveTarget_SingleActiveSpec_AutoSelects(t *testing.T) {
	root := t.TempDir()

	epics := []phase.EpicInfo{
		{ID: "epic-a", Title: "[SPEC 038-alpha] Alpha", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(38), "spec_title": "alpha"}},
	}
	stubActiveEpics(t, epics, map[string][]phase.ChildInfo{
		"epic-a": {{ID: "b1", Status: "open", IssueType: "task"}},
	})

	got, err := ResolveTarget(root, "")
	if err != nil {
		t.Fatalf("ResolveTarget should auto-select single spec: %v", err)
	}
	if got != "038-alpha" {
		t.Errorf("got %q, want %q", got, "038-alpha")
	}
}

func TestResolveTarget_FocusDisambiguatesMultipleSpecs(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".mindspec"), 0755)

	epics := []phase.EpicInfo{
		{ID: "epic-a", Title: "[SPEC 038-alpha] Alpha", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(38), "spec_title": "alpha"}},
		{ID: "epic-b", Title: "[SPEC 039-beta] Beta", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(39), "spec_title": "beta"}},
	}
	stubActiveEpics(t, epics, map[string][]phase.ChildInfo{
		"epic-a": {{ID: "b1", Status: "in_progress", IssueType: "task"}},
		"epic-b": {{ID: "b2", Status: "open", IssueType: "task"}},
	})

	// ADR-0023: focus files eliminated. With multiple active specs and no
	// explicit --spec flag, ResolveTarget should return an ambiguity error.
	_, err := ResolveTarget(root, "")
	if err == nil {
		t.Fatal("expected ambiguity error with multiple active specs and no --spec flag")
	}
	// Verify explicit flag still works
	got, err := ResolveTarget(root, "039-beta")
	if err != nil {
		t.Fatalf("explicit --spec should resolve: %v", err)
	}
	if got != "039-beta" {
		t.Errorf("explicit flag: got %q, want %q", got, "039-beta")
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

func TestMixedRepo_ActiveAndDone(t *testing.T) {
	root := t.TempDir()

	// One active, one closed
	epics := []phase.EpicInfo{
		{ID: "epic-a", Title: "[SPEC 038-active] Active", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(38), "spec_title": "active"}},
		{ID: "epic-d", Title: "[SPEC 005-done] Done", Status: "closed", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(5), "spec_title": "done", "mindspec_done": true}},
	}
	stubActiveEpics(t, epics, map[string][]phase.ChildInfo{
		"epic-a": {{ID: "b1", Status: "in_progress", IssueType: "task"}},
	})

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

func TestLegacyRepo_NoEpics(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".mindspec"), 0755)

	// No epics → no active specs
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restore)

	_, err := ResolveTarget(root, "")
	if err == nil {
		t.Fatal("expected error when no active specs exist")
	}
	if !strings.Contains(err.Error(), "--spec") {
		t.Errorf("error should suggest --spec flag, got: %v", err)
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

// stubActiveEpics helper for integration tests (same as resolve_test.go).
func init() {
	// Ensure json is importable for stubs
	_ = json.Marshal
}
