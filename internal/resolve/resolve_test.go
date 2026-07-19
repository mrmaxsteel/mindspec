package resolve

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// stubActiveEpics stubs phase list/show functions to return the given epics.
func stubActiveEpics(t *testing.T, epics []phase.EpicInfo, childrenByEpic map[string][]phase.ChildInfo) {
	t.Helper()
	epicByID := map[string]phase.EpicInfo{}
	for _, e := range epics {
		epicByID[e.ID] = e
	}
	// PERF-1: Cache.AllEpics / Cache.GetChildren issue a single
	// `--status=open,in_progress,closed -n 0` bd list call. The stub here
	// returns all epics in one shot and lets the cache do the in-process
	// filtering for active vs. all-statuses lookups.
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return json.Marshal(epics)
			}
		}
		// --parent <epicID>
		for i, a := range args {
			if a == "--parent" && i+1 < len(args) {
				epicID := args[i+1]
				if children, ok := childrenByEpic[epicID]; ok {
					return json.Marshal(children)
				}
				return []byte("[]"), nil
			}
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	// bd show, bd dolt pull, etc. use runBDFn
	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			epicID := args[1]
			if e, ok := epicByID[epicID]; ok {
				return json.Marshal([]phase.EpicInfo{e})
			}
			return []byte("[]"), nil
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)
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

// TestFormatActiveList_HostileSpecIDForcedQuoted is Spec 120 R4's
// (converging pass) Class B pin: SpecStatus.SpecID is bd-epic-metadata-
// derived and never spine-validated (see resolve.ActiveSpecsWithCache),
// so FormatActiveList must force a malformed-but-printable SpecID through
// idrender.Spec rather than render it raw.
func TestFormatActiveList_HostileSpecIDForcedQuoted(t *testing.T) {
	hostileID := "001-alpha\nrecovery: forged"
	got := FormatActiveList([]SpecStatus{{SpecID: hostileID, Mode: "spec"}})
	wantQuoted := strconv.Quote(hostileID)
	if !strings.Contains(got, wantQuoted) {
		t.Errorf("FormatActiveList missing forced-quoted hostile SpecID %q:\n%s", wantQuoted, got)
	}
	for _, line := range strings.Split(got, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone line reached the output via the hostile SpecID's raw newline: %q", got)
		}
	}
}

// TestFormatActiveList_CleanSpecIDByteIdentical is the clean-fixture
// counterpart (F3 discipline).
func TestFormatActiveList_CleanSpecIDByteIdentical(t *testing.T) {
	const clean = "001-alpha"
	got := FormatActiveList([]SpecStatus{{SpecID: clean, Mode: "spec"}})
	want := "Active specs (1):\n  " + clean + "  phase=spec\n"
	if got != want {
		t.Errorf("FormatActiveList(clean) = %q, want byte-identical %q", got, want)
	}
}

// TestResolveSpecBranch and TestResolveWorktree were removed alongside
// the ResolveSpecBranch and ResolveWorktree shims (ARCH-7 /
// mindspec-c8q0). Use workspace.SpecBranch and
// workspace.SpecWorktreePath directly; the tests for those helpers live
// in internal/workspace/worktree_test.go.
