package resolve

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mindspec/mindspec/internal/phase"
	"github.com/mindspec/mindspec/internal/state"
)

// stubActiveEpics stubs phase.runBDFn to return the given epics.
func stubActiveEpics(t *testing.T, epics []phase.EpicInfo, childrenByEpic map[string][]phase.ChildInfo) {
	t.Helper()
	// Build index by ID for bd show lookups (used by hasDoneMarker)
	epicByID := map[string]phase.EpicInfo{}
	for _, e := range epics {
		epicByID[e.ID] = e
	}
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "list" && args[1] == "--type=epic" {
			return json.Marshal(epics)
		}
		if len(args) >= 2 && args[0] == "list" && args[1] == "--parent" {
			epicID := args[2]
			if children, ok := childrenByEpic[epicID]; ok {
				return json.Marshal(children)
			}
			return []byte("[]"), nil
		}
		if len(args) >= 2 && args[0] == "show" {
			epicID := args[1]
			if e, ok := epicByID[epicID]; ok {
				return json.Marshal([]phase.EpicInfo{e})
			}
			return []byte("[]"), nil
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restore)
}

func TestActiveSpecs_DeriveFromBeads(t *testing.T) {
	root := t.TempDir()

	// Two active epics: alpha in implement, beta in plan
	epics := []phase.EpicInfo{
		{ID: "epic-a", Title: "[SPEC 038-alpha] Alpha", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(38), "spec_title": "alpha"}},
		{ID: "epic-b", Title: "[SPEC 039-beta] Beta", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(39), "spec_title": "beta"}},
		{ID: "epic-c", Title: "[SPEC 040-gamma] Gamma", Status: "closed", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(40), "spec_title": "gamma", "mindspec_done": true}},
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
		t.Fatalf("expected 2 active specs, got %d: %+v", len(active), active)
	}

	if active[0].SpecID != "038-alpha" {
		t.Errorf("first active spec: got %q, want %q", active[0].SpecID, "038-alpha")
	}
	if active[0].Mode != state.ModeImplement {
		t.Errorf("first active mode: got %q, want %q", active[0].Mode, state.ModeImplement)
	}
	if active[1].SpecID != "039-beta" {
		t.Errorf("second active spec: got %q, want %q", active[1].SpecID, "039-beta")
	}
	if active[1].Mode != state.ModePlan {
		t.Errorf("second active mode: got %q, want %q", active[1].Mode, state.ModePlan)
	}
}

func TestActiveSpecs_NoEpics(t *testing.T) {
	root := t.TempDir()
	stubActiveEpics(t, nil, nil)

	active, err := ActiveSpecs(root)
	if err != nil {
		t.Fatalf("ActiveSpecs() error: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active specs, got %d", len(active))
	}
}

func TestActiveSpecs_SortOrder(t *testing.T) {
	root := t.TempDir()

	// Create in reverse order — should be sorted in output
	epics := []phase.EpicInfo{
		{ID: "epic-c", Title: "[SPEC 040-gamma] Gamma", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(40), "spec_title": "gamma"}},
		{ID: "epic-a", Title: "[SPEC 038-alpha] Alpha", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(38), "spec_title": "alpha"}},
		{ID: "epic-b", Title: "[SPEC 039-beta] Beta", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(39), "spec_title": "beta"}},
	}
	childrenByEpic := map[string][]phase.ChildInfo{
		"epic-a": {{ID: "b1", Status: "open", IssueType: "task"}},
		"epic-b": {{ID: "b2", Status: "open", IssueType: "task"}},
		"epic-c": {{ID: "b3", Status: "open", IssueType: "task"}},
	}
	stubActiveEpics(t, epics, childrenByEpic)

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
