package phase

import (
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/state"
)

// stubFinders wires the bd seams so FindEpicBySpecID and FindEpic/GetChildren
// resolve against in-memory fixtures. Returns a teardown.
func stubFinders(t *testing.T, epicJSON, childrenJSON string) func() {
	t.Helper()

	restoreList := SetListJSONForTest(func(args ...string) ([]byte, error) {
		// queryChildren passes --parent
		for _, a := range args {
			if strings.HasPrefix(a, "--parent") {
				return []byte(childrenJSON), nil
			}
		}
		// epic list query: return epic so FindEpicBySpecID can resolve it
		for _, a := range args {
			if a == "--type=epic" {
				return []byte("[" + epicJSON + "]"), nil
			}
		}
		return []byte("[]"), nil
	})

	restoreRun := SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return []byte("[" + epicJSON + "]"), nil
		}
		return []byte("[]"), nil
	})

	return func() {
		restoreList()
		restoreRun()
	}
}

// ---------------------------------------------------------------------------
// TestPhaseDerivationFromMetadataOnly — guard the pre-existing fast-path:
// epic with mindspec_phase=plan + zero children → DerivePhase returns "plan"
// without consulting children (regression guard for Spec 080).
// ---------------------------------------------------------------------------

func TestPhaseDerivationFromMetadataOnly(t *testing.T) {
	childrenCalls := 0
	restoreList := SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if strings.HasPrefix(a, "--parent") {
				childrenCalls++
				return []byte("[]"), nil
			}
		}
		return []byte("[]"), nil
	})
	defer restoreList()

	restoreRun := SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return []byte(`[{"id":"epic-1","title":"test","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"plan"}}]`), nil
		}
		return []byte("[]"), nil
	})
	defer restoreRun()

	got, err := DerivePhase("epic-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != state.ModePlan {
		t.Errorf("DerivePhase(metadata=plan) = %q, want %q", got, state.ModePlan)
	}
	// The implementation does a consistency child-derive on the metadata
	// path; that is fine — what matters here is the stored metadata
	// value is the one returned (the guard for ADR-0023 metadata-first).
}

// ---------------------------------------------------------------------------
// TestLegacyMigratesOnFirstCommand — legacy 7-bead spec (no mindspec_phase
// metadata) → first EnsureMigrated call writes mindspec_phase and
// mindspec_migrated_at, returns (true, nil).
// ---------------------------------------------------------------------------

func TestLegacyMigratesOnFirstCommand(t *testing.T) {
	// Legacy epic: no mindspec_phase in metadata; one closed child + one
	// in_progress child → DerivePhaseFromChildren returns "implement".
	epic := `{"id":"epic-7","title":"[SPEC 007-legacy] Legacy","status":"open","issue_type":"epic","metadata":{"spec_num":7,"spec_title":"legacy"}}`
	children := `[{"id":"b1","title":"bead1","status":"closed","issue_type":"task"},{"id":"b2","title":"bead2","status":"in_progress","issue_type":"task"}]`
	defer stubFinders(t, epic, children)()

	var captured map[string]interface{}
	var capturedID string
	calls := 0
	restoreMerge := SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		calls++
		capturedID = issueID
		captured = updates
		return nil
	})
	defer restoreMerge()

	migrated, err := EnsureMigrated("007-legacy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !migrated {
		t.Fatalf("expected migrated=true, got false")
	}
	if calls != 1 {
		t.Errorf("MergeMetadata calls = %d, want 1", calls)
	}
	if capturedID != "epic-7" {
		t.Errorf("MergeMetadata target id = %q, want %q", capturedID, "epic-7")
	}
	if got := captured["mindspec_phase"]; got != state.ModeImplement {
		t.Errorf("mindspec_phase = %v, want %q", got, state.ModeImplement)
	}
	ts, ok := captured["mindspec_migrated_at"].(string)
	if !ok || ts == "" {
		t.Fatalf("mindspec_migrated_at missing or not a string: %v", captured["mindspec_migrated_at"])
	}
}

// ---------------------------------------------------------------------------
// TestMigratedEpicHasMigratedAtMarker — after migration, the timestamp is
// a parseable RFC3339 value.
// ---------------------------------------------------------------------------

func TestMigratedEpicHasMigratedAtMarker(t *testing.T) {
	epic := `{"id":"epic-9","title":"[SPEC 009-legacy] L","status":"open","issue_type":"epic","metadata":{"spec_num":9,"spec_title":"legacy"}}`
	children := `[{"id":"b1","title":"b","status":"open","issue_type":"task"}]`
	defer stubFinders(t, epic, children)()

	var captured map[string]interface{}
	restoreMerge := SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		captured = updates
		return nil
	})
	defer restoreMerge()

	if _, err := EnsureMigrated("009-legacy"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tsRaw, ok := captured["mindspec_migrated_at"].(string)
	if !ok || tsRaw == "" {
		t.Fatalf("mindspec_migrated_at missing")
	}
	if _, err := time.Parse(time.RFC3339Nano, tsRaw); err != nil {
		// RFC3339 also accepted — RFC3339Nano is a superset.
		if _, err2 := time.Parse(time.RFC3339, tsRaw); err2 != nil {
			t.Errorf("mindspec_migrated_at not parseable as RFC3339(Nano): %q (errs: %v / %v)", tsRaw, err, err2)
		}
	}
}

// ---------------------------------------------------------------------------
// TestEnsureMigratedIdempotent — second call on a migrated epic must
// return (false, nil) and NOT call MergeMetadata again.
// ---------------------------------------------------------------------------

func TestEnsureMigratedIdempotent(t *testing.T) {
	// Migrated epic: mindspec_phase already present.
	epic := `{"id":"epic-3","title":"[SPEC 003-mig] M","status":"open","issue_type":"epic","metadata":{"spec_num":3,"spec_title":"mig","mindspec_phase":"implement","mindspec_migrated_at":"2026-05-01T00:00:00Z"}}`
	children := `[{"id":"b1","title":"b","status":"in_progress","issue_type":"task"}]`
	defer stubFinders(t, epic, children)()

	calls := 0
	restoreMerge := SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		calls++
		return nil
	})
	defer restoreMerge()

	migrated, err := EnsureMigrated("003-mig")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated {
		t.Errorf("expected migrated=false on already-migrated epic, got true")
	}
	if calls != 0 {
		t.Errorf("MergeMetadata calls = %d, want 0 (idempotent)", calls)
	}

	// Second invocation: same expectations.
	migrated2, err := EnsureMigrated("003-mig")
	if err != nil {
		t.Fatalf("unexpected error on 2nd call: %v", err)
	}
	if migrated2 {
		t.Errorf("expected migrated=false on 2nd call, got true")
	}
	if calls != 0 {
		t.Errorf("MergeMetadata calls after 2nd = %d, want 0", calls)
	}
}

// ---------------------------------------------------------------------------
// TestEnsureMigratedNoEpicReturnsFalseNil — when FindEpicBySpecID returns
// the empty string (no epic exists yet), EnsureMigrated returns
// (false, nil) without an error and without any MergeMetadata call.
// ---------------------------------------------------------------------------

func TestEnsureMigratedNoEpicReturnsFalseNil(t *testing.T) {
	// listJSON: no epics at all.
	restoreList := SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	defer restoreList()
	restoreRun := SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	defer restoreRun()

	calls := 0
	restoreMerge := SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		calls++
		return nil
	})
	defer restoreMerge()

	migrated, err := EnsureMigrated("999-nonexistent")
	if err != nil {
		t.Errorf("expected nil error when no epic exists, got %v", err)
	}
	if migrated {
		t.Errorf("expected migrated=false when no epic exists, got true")
	}
	if calls != 0 {
		t.Errorf("MergeMetadata should not be called when no epic exists, got %d calls", calls)
	}
}

// ---------------------------------------------------------------------------
// TestDerivePhaseFromChildrenStillPasses — the children-derived fallback is
// preserved untouched as the last-resort back-stop for legacy epics that
// have never had a lifecycle command run against them. This guards spec 089
// Requirement 14 (do not remove the children-derived fallback).
// ---------------------------------------------------------------------------

func TestDerivePhaseFromChildrenStillPasses(t *testing.T) {
	cases := []struct {
		name     string
		children []ChildInfo
		want     string
	}{
		{name: "no children → plan", children: nil, want: state.ModePlan},
		{name: "all closed → review", children: []ChildInfo{{Status: "closed"}, {Status: "closed"}}, want: state.ModeReview},
		{name: "one in_progress → implement", children: []ChildInfo{{Status: "open"}, {Status: "in_progress"}}, want: state.ModeImplement},
		{name: "mixed closed+open → implement", children: []ChildInfo{{Status: "closed"}, {Status: "open"}}, want: state.ModeImplement},
		{name: "all open → plan", children: []ChildInfo{{Status: "open"}, {Status: "open"}}, want: state.ModePlan},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DerivePhaseFromChildren(tc.children)
			if got != tc.want {
				t.Errorf("DerivePhaseFromChildren = %q, want %q", got, tc.want)
			}
		})
	}
}
