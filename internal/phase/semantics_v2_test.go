package phase

// semantics_v2_test.go — spec 128 (lifecycle-verb-state-honesty) Bead 2,
// plan step 7's six fixtures for EnsurePhaseSemanticsV2 /
// LegacyPhaseSignature: upgrade-before-first-claim; upgrade-then-first-
// claim; non-lifecycle claim (no advance, marker written); release
// (returns to plan, marker present, no re-reconcile); already-progressed
// epics (some-closed / in_progress / review / done — phase untouched,
// marker written); and the READ-path fixture (covered separately in
// derive_v2_test.go, which needs the external phase_test package to
// capture stderr the same way derive_recovery_test.go does).

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/state"
)

// v2FixtureStore is a minimal fake bd tracker distinguishing an
// epic-metadata `bd show`/`--type=epic` response from a `--parent`
// children response — the same two-query shape SanctionedDerivedSync's
// own tests (sanctioned_sync_test.go) stub.
type v2FixtureStore struct {
	epicID   string
	metadata map[string]interface{}
	children []ChildInfo
}

func (s *v2FixtureStore) epicJSON() []byte {
	b, _ := json.Marshal([]EpicInfo{{ID: s.epicID, Title: "fixture", Status: "open", IssueType: "epic", Metadata: s.metadata}})
	return b
}

func (s *v2FixtureStore) stub(t *testing.T) {
	t.Helper()
	t.Cleanup(SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == s.epicID {
			return s.epicJSON(), nil
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return s.epicJSON(), nil
			}
			if strings.HasPrefix(a, "--parent") {
				b, _ := json.Marshal(s.children)
				return b, nil
			}
		}
		return []byte("[]"), nil
	}))
}

func lifecycleChild(id, status string) ChildInfo {
	return ChildInfo{ID: id, Status: status, IssueType: "task"}
}

// TestEnsurePhaseSemanticsV2_UpgradeBeforeFirstClaim: the legacy
// plan-approve signature (marker absent, stored `implement`, all
// lifecycle children open, none claimed) rewrites stored to `plan` AND
// writes the marker, in one critical section.
func TestEnsurePhaseSemanticsV2_UpgradeBeforeFirstClaim(t *testing.T) {
	root := t.TempDir()
	store := &v2FixtureStore{
		epicID:   "epic-1",
		metadata: map[string]interface{}{"mindspec_phase": state.ModeImplement},
		children: []ChildInfo{lifecycleChild("b1", "open"), lifecycleChild("b2", "open")},
	}
	store.stub(t)

	var gotID string
	var gotUpdates map[string]interface{}
	t.Cleanup(SetMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
		gotID, gotUpdates = id, updates
		return nil
	}))

	rewrote, err := EnsurePhaseSemanticsV2(root, "epic-1")
	if err != nil {
		t.Fatalf("EnsurePhaseSemanticsV2: %v", err)
	}
	if !rewrote {
		t.Error("expected rewrote=true for the legacy all-open signature")
	}
	if gotID != "epic-1" {
		t.Errorf("merge target = %q, want epic-1", gotID)
	}
	if gotUpdates["mindspec_phase"] != state.ModePlan {
		t.Errorf("mindspec_phase = %v, want %q", gotUpdates["mindspec_phase"], state.ModePlan)
	}
	if gotUpdates[PhaseSemanticsV2MarkerKey] != PhaseSemanticsV2Value {
		t.Errorf("marker = %v, want %q", gotUpdates[PhaseSemanticsV2MarkerKey], PhaseSemanticsV2Value)
	}
	if len(gotUpdates) != 2 {
		t.Errorf("updates = %v, want exactly mindspec_phase + marker (merge semantics)", gotUpdates)
	}
}

// TestEnsurePhaseSemanticsV2_UpgradeThenFirstClaim proves the SEQUENCE:
// once upgraded (marker present, stored now `plan`), a subsequent claim
// (simulated here as next's own writer would run it: an unconditional
// forward write to `implement`) still lands cleanly, and a SECOND
// EnsurePhaseSemanticsV2 call against the now-claimed state is a pure
// marker-present no-op (never re-fires the legacy rewrite).
func TestEnsurePhaseSemanticsV2_UpgradeThenFirstClaim(t *testing.T) {
	root := t.TempDir()
	store := &v2FixtureStore{
		epicID:   "epic-1",
		metadata: map[string]interface{}{"mindspec_phase": state.ModeImplement},
		children: []ChildInfo{lifecycleChild("b1", "open")},
	}
	store.stub(t)

	applied := map[string]interface{}{}
	t.Cleanup(SetMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
		for k, v := range updates {
			applied[k] = v
			store.metadata[k] = v
		}
		return nil
	}))

	if _, err := EnsurePhaseSemanticsV2(root, "epic-1"); err != nil {
		t.Fatalf("first EnsurePhaseSemanticsV2: %v", err)
	}
	if applied["mindspec_phase"] != state.ModePlan {
		t.Fatalf("after upgrade, stored = %v, want %q", applied["mindspec_phase"], state.ModePlan)
	}

	// Simulate next's claim advance: stored implement, one child now
	// in_progress (the claim).
	store.metadata["mindspec_phase"] = state.ModeImplement
	store.children = []ChildInfo{lifecycleChild("b1", "in_progress")}

	calls := 0
	t.Cleanup(SetMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
		calls++
		return nil
	}))
	rewrote, err := EnsurePhaseSemanticsV2(root, "epic-1")
	if err != nil {
		t.Fatalf("second EnsurePhaseSemanticsV2: %v", err)
	}
	if rewrote {
		t.Error("second call must not report a rewrite (marker already present)")
	}
	if calls != 0 {
		t.Errorf("marker-present epic must no-op with ZERO merge calls, got %d", calls)
	}
}

// TestEnsurePhaseSemanticsV2_NonLifecycleClaimNoAdvanceMarkerWritten: an
// epic whose only children are non-lifecycle (e.g. a `bug`) is NOT the
// legacy signature (LegacyPhaseSignature only counts lifecycle
// children) — the marker is written but the phase is left untouched.
func TestEnsurePhaseSemanticsV2_NonLifecycleClaimNoAdvanceMarkerWritten(t *testing.T) {
	root := t.TempDir()
	store := &v2FixtureStore{
		epicID:   "epic-1",
		metadata: map[string]interface{}{"mindspec_phase": state.ModeImplement},
		children: []ChildInfo{{ID: "bug-1", Status: "open", IssueType: "bug"}},
	}
	store.stub(t)

	var gotUpdates map[string]interface{}
	t.Cleanup(SetMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
		gotUpdates = updates
		return nil
	}))

	rewrote, err := EnsurePhaseSemanticsV2(root, "epic-1")
	if err != nil {
		t.Fatalf("EnsurePhaseSemanticsV2: %v", err)
	}
	if rewrote {
		t.Error("a non-lifecycle-only child set must never be treated as the legacy signature")
	}
	if _, ok := gotUpdates["mindspec_phase"]; ok {
		t.Errorf("phase must be left untouched, got updates %v", gotUpdates)
	}
	if gotUpdates[PhaseSemanticsV2MarkerKey] != PhaseSemanticsV2Value {
		t.Errorf("marker must still be written, got %v", gotUpdates)
	}
}

// TestEnsurePhaseSemanticsV2_ReleaseReturnsToPlanMarkerPresentNoReReconcile
// covers the release fixture: marker ALREADY present (post-upgrade,
// released state built by post-marker means per round-3 G2's fixture
// pinning) — a no-op regardless of what stored/children currently say.
func TestEnsurePhaseSemanticsV2_ReleaseReturnsToPlanMarkerPresentNoReReconcile(t *testing.T) {
	root := t.TempDir()
	store := &v2FixtureStore{
		epicID: "epic-1",
		metadata: map[string]interface{}{
			"mindspec_phase":          state.ModePlan,
			PhaseSemanticsV2MarkerKey: PhaseSemanticsV2Value,
		},
		children: []ChildInfo{lifecycleChild("b1", "open")},
	}
	store.stub(t)

	calls := 0
	t.Cleanup(SetMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
		calls++
		return nil
	}))

	rewrote, err := EnsurePhaseSemanticsV2(root, "epic-1")
	if err != nil {
		t.Fatalf("EnsurePhaseSemanticsV2: %v", err)
	}
	if rewrote {
		t.Error("marker present must never rewrite")
	}
	if calls != 0 {
		t.Errorf("marker present must be a ZERO-call no-op, got %d calls", calls)
	}
}

// TestEnsurePhaseSemanticsV2_AlreadyProgressedEpicsPhaseUntouchedMarkerWritten
// is the already-progressed group: some-closed / in_progress / review /
// done, marker absent — none of these match LegacyPhaseSignature (which
// requires ALL lifecycle children open), so each writes the marker only,
// phase untouched.
func TestEnsurePhaseSemanticsV2_AlreadyProgressedEpicsPhaseUntouchedMarkerWritten(t *testing.T) {
	cases := []struct {
		name     string
		stored   string
		children []ChildInfo
	}{
		{"some-closed", state.ModeImplement, []ChildInfo{lifecycleChild("b1", "closed"), lifecycleChild("b2", "open")}},
		{"in-progress", state.ModeImplement, []ChildInfo{lifecycleChild("b1", "in_progress")}},
		{"review", state.ModeReview, []ChildInfo{lifecycleChild("b1", "closed"), lifecycleChild("b2", "closed")}},
		{"done", state.ModeDone, []ChildInfo{lifecycleChild("b1", "closed")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			store := &v2FixtureStore{
				epicID:   "epic-1",
				metadata: map[string]interface{}{"mindspec_phase": tc.stored},
				children: tc.children,
			}
			store.stub(t)

			var gotUpdates map[string]interface{}
			t.Cleanup(SetMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
				gotUpdates = updates
				return nil
			}))

			rewrote, err := EnsurePhaseSemanticsV2(root, "epic-1")
			if err != nil {
				t.Fatalf("EnsurePhaseSemanticsV2: %v", err)
			}
			if rewrote {
				t.Errorf("%s: an already-progressed epic must never be treated as the legacy signature", tc.name)
			}
			if _, ok := gotUpdates["mindspec_phase"]; ok {
				t.Errorf("%s: phase must be left untouched, got updates %v", tc.name, gotUpdates)
			}
			if gotUpdates[PhaseSemanticsV2MarkerKey] != PhaseSemanticsV2Value {
				t.Errorf("%s: marker must be written, got %v", tc.name, gotUpdates)
			}
		})
	}
}

// TestEnsurePhaseSemanticsV2_ChildReadFailurePropagatesNoWrite mirrors
// SanctionedDerivedSync's own named-hazard proof (round-1/round-2 panel
// hard-block driver): a children-read failure must propagate, with ZERO
// merge-write calls, never silently deriving "not legacy" from a nil
// slice.
func TestEnsurePhaseSemanticsV2_ChildReadFailurePropagatesNoWrite(t *testing.T) {
	root := t.TempDir()
	wantErr := errors.New("bd list --parent: transient failure")
	t.Cleanup(SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == "epic-1" {
			b, _ := json.Marshal([]EpicInfo{{ID: "epic-1", Title: "fixture", Status: "open", IssueType: "epic",
				Metadata: map[string]interface{}{"mindspec_phase": state.ModeImplement}}})
			return b, nil
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				b, _ := json.Marshal([]EpicInfo{{ID: "epic-1", Title: "fixture", Status: "open", IssueType: "epic",
					Metadata: map[string]interface{}{"mindspec_phase": state.ModeImplement}}})
				return b, nil
			}
			if strings.HasPrefix(a, "--parent") {
				return nil, wantErr
			}
		}
		return []byte("[]"), nil
	}))
	calls := 0
	t.Cleanup(SetMergeMetadataForTest(func(string, map[string]interface{}) error {
		calls++
		return nil
	}))

	_, err := EnsurePhaseSemanticsV2(root, "epic-1")
	if err == nil {
		t.Fatal("expected an error when the children read fails")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("error does not wrap the underlying children-read failure: %v", err)
	}
	if calls != 0 {
		t.Errorf("merge calls = %d, want 0 on a children-read failure", calls)
	}
}

// TestLegacyPhaseSignature_TableDriven pins the predicate directly
// (lock-free, no bd involved) across its boundary cases.
func TestLegacyPhaseSignature_TableDriven(t *testing.T) {
	cases := []struct {
		name     string
		stored   string
		children []ChildInfo
		want     bool
	}{
		{"implement all-open one child", state.ModeImplement, []ChildInfo{lifecycleChild("b1", "open")}, true},
		{"implement all-open two children", state.ModeImplement, []ChildInfo{lifecycleChild("b1", "open"), lifecycleChild("b2", "open")}, true},
		{"implement zero lifecycle children", state.ModeImplement, nil, false},
		{"implement with one closed", state.ModeImplement, []ChildInfo{lifecycleChild("b1", "closed"), lifecycleChild("b2", "open")}, false},
		{"implement with one in_progress", state.ModeImplement, []ChildInfo{lifecycleChild("b1", "in_progress")}, false},
		{"plan stored", state.ModePlan, []ChildInfo{lifecycleChild("b1", "open")}, false},
		{"review stored", state.ModeReview, []ChildInfo{lifecycleChild("b1", "closed")}, false},
		{"implement all-open but only non-lifecycle children", state.ModeImplement, []ChildInfo{{ID: "bug-1", Status: "open", IssueType: "bug"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LegacyPhaseSignature(tc.stored, tc.children); got != tc.want {
				t.Errorf("LegacyPhaseSignature(%q, %v) = %v, want %v", tc.stored, tc.children, got, tc.want)
			}
		})
	}
}
