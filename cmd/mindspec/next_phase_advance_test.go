package main

// next_phase_advance_test.go — spec 128 (lifecycle-verb-state-honesty)
// Bead 2, plan step 2: unit tests for advancePhaseOnClaim (next.go's
// Step 5.2), extracted to its own named function for exactly this
// purpose (matching claimFailureError/warnWorktreeSetupFailure's
// established pattern).

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// wireAdvancePhaseFixture stubs phase's bd seams to resolve beadID's
// epic lineage (a `bd show <beadID>` dependency-parent lookup) to
// epicID, and epicID's own metadata to epicMeta.
func wireAdvancePhaseFixture(t *testing.T, beadID, epicID string, epicMeta map[string]interface{}) {
	t.Helper()
	beadShowJSON, _ := json.Marshal([]map[string]interface{}{{
		"title": "[010-test] a bead",
		"dependencies": []map[string]interface{}{
			{"id": epicID, "issue_type": "epic"},
		},
	}})
	epicJSON, _ := json.Marshal([]phase.EpicInfo{{
		ID: epicID, Title: "[SPEC 010-test] Test", Status: "open", IssueType: "epic", Metadata: epicMeta,
	}})
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == beadID {
			return beadShowJSON, nil
		}
		if len(args) >= 2 && args[0] == "show" && args[1] == epicID {
			return epicJSON, nil
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
}

// TestAdvancePhaseOnClaim_LifecycleClaimAdvancesToImplement: a
// task-typed (LIFECYCLE) claim on an epic stored at `plan` advances
// stored mindspec_phase to `implement`.
func TestAdvancePhaseOnClaim_LifecycleClaimAdvancesToImplement(t *testing.T) {
	root := t.TempDir()
	wireAdvancePhaseFixture(t, "mindspec-bead1", "epic-1", map[string]interface{}{
		"mindspec_phase":           state.ModePlan,
		"mindspec_phase_semantics": "v2",
	})
	// EnsurePhaseSemanticsV2's own self-heal write (marker already
	// present here, so it no-ops — but stub it anyway so a change to
	// that no-op contract fails loudly here instead of hitting real bd).
	t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
	var gotUpdates map[string]interface{}
	origMerge := nextPhaseMergeMetadataFn
	nextPhaseMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		gotUpdates = updates
		return nil
	}
	t.Cleanup(func() { nextPhaseMergeMetadataFn = origMerge })

	advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: "task"})

	if gotUpdates["mindspec_phase"] != state.ModeImplement {
		t.Errorf("mindspec_phase = %v, want %q", gotUpdates["mindspec_phase"], state.ModeImplement)
	}
}

// TestAdvancePhaseOnClaim_UntypedClaimAdvancesToImplement: an untyped
// (empty issue_type) bead is ALSO lifecycle per the ADR-0023
// discriminator.
func TestAdvancePhaseOnClaim_UntypedClaimAdvancesToImplement(t *testing.T) {
	root := t.TempDir()
	wireAdvancePhaseFixture(t, "mindspec-bead1", "epic-1", map[string]interface{}{
		"mindspec_phase":           state.ModePlan,
		"mindspec_phase_semantics": "v2",
	})
	t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
	var gotUpdates map[string]interface{}
	origMerge := nextPhaseMergeMetadataFn
	nextPhaseMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		gotUpdates = updates
		return nil
	}
	t.Cleanup(func() { nextPhaseMergeMetadataFn = origMerge })

	advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: ""})

	if gotUpdates["mindspec_phase"] != state.ModeImplement {
		t.Errorf("mindspec_phase = %v, want %q for an untyped (lifecycle-default) claim", gotUpdates["mindspec_phase"], state.ModeImplement)
	}
}

// TestAdvancePhaseOnClaim_NonLifecycleClaimAdvancesNothing: a `bug`-typed
// claim must never advance stored mindspec_phase.
func TestAdvancePhaseOnClaim_NonLifecycleClaimAdvancesNothing(t *testing.T) {
	root := t.TempDir()
	wireAdvancePhaseFixture(t, "mindspec-bead1", "epic-1", map[string]interface{}{
		"mindspec_phase":           state.ModePlan,
		"mindspec_phase_semantics": "v2",
	})
	t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
	calls := 0
	origMerge := nextPhaseMergeMetadataFn
	nextPhaseMergeMetadataFn = func(string, map[string]interface{}) error {
		calls++
		return nil
	}
	t.Cleanup(func() { nextPhaseMergeMetadataFn = origMerge })

	advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: "bug"})

	if calls != 0 {
		t.Errorf("a non-lifecycle claim must perform ZERO phase writes, got %d", calls)
	}
}

// TestAdvancePhaseOnClaim_AlreadyImplementIsNoOp: forward-only — no write
// when stored is already `implement`.
func TestAdvancePhaseOnClaim_AlreadyImplementIsNoOp(t *testing.T) {
	root := t.TempDir()
	wireAdvancePhaseFixture(t, "mindspec-bead1", "epic-1", map[string]interface{}{
		"mindspec_phase":           state.ModeImplement,
		"mindspec_phase_semantics": "v2",
	})
	t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
	calls := 0
	origMerge := nextPhaseMergeMetadataFn
	nextPhaseMergeMetadataFn = func(string, map[string]interface{}) error {
		calls++
		return nil
	}
	t.Cleanup(func() { nextPhaseMergeMetadataFn = origMerge })

	advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: "task"})

	if calls != 0 {
		t.Errorf("an already-implement epic must be a ZERO-write no-op, got %d calls", calls)
	}
}

// TestAdvancePhaseOnClaim_NoEpicLineageIsSilentNoOp: a bead with no
// discoverable epic lineage (phase.ErrNoEpicLineage) must never warn or
// write — this is the ordinary "standalone task, not part of a spec"
// case, not an infra failure.
func TestAdvancePhaseOnClaim_NoEpicLineageIsSilentNoOp(t *testing.T) {
	root := t.TempDir()
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil // `bd show` returns no items -> ErrNoEpicLineage
	}))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
	calls := 0
	t.Cleanup(phase.SetMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
		calls++
		return nil
	}))

	stderr := captureStderr(t, func() {
		advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-standalone", IssueType: "task"})
	})

	if calls != 0 {
		t.Errorf("no epic lineage must perform ZERO phase writes, got %d", calls)
	}
	if strings.Contains(stderr, "warning:") {
		t.Errorf("a genuinely epic-less bead must not warn (that is the ordinary case, not an infra failure): %q", stderr)
	}
}
