package main

// repair_release_sanction_test.go — spec 128 (lifecycle-verb-state-
// honesty) Bead 2, AC-5(viii): "on the same released all-open state,
// release's internal guard-routed backward sync succeeds unforced while
// mindspec repair phase on that state still refuses without force+reason
// — the test distinguishes the sanctioned internal caller from the
// operator path." Under OQ-3 option 1, a release (the sole claimed
// lifecycle bead returning to open) is ALWAYS a genuine backward move in
// the total order (implement -> plan) — this is exactly why release
// routes through phase.SanctionedDerivedSync rather than the operator
// guard.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// TestAC5viii_ReleasedAllOpenState_SanctionedSyncSucceedsOperatorGuardRefuses
// builds ONE state — stored "implement", v2 marker present (round-3 G2
// fixture pinning: built via post-marker means, not the legacy path),
// one lifecycle child now back to "open" (the released bead) — and
// drives it through BOTH surfaces: phase.SanctionedDerivedSync (the
// sanctioned internal caller, release's own step 6.5) succeeds unforced;
// `mindspec repair phase` (the operator path, same state) refuses
// without --force-backward.
func TestAC5viii_ReleasedAllOpenState_SanctionedSyncSucceedsOperatorGuardRefuses(t *testing.T) {
	epicID, specID := "epic-92", "092-agent-contract-hardening"
	newEpics := func() []phase.EpicInfo {
		return []phase.EpicInfo{{
			ID: epicID, Title: "[SPEC 092-agent-contract-hardening] Hardening", Status: "open",
			IssueType: "epic", Metadata: map[string]interface{}{
				"spec_num":                 float64(92),
				"spec_title":               "agent-contract-hardening",
				"mindspec_phase":           "implement",
				"mindspec_phase_semantics": "v2",
			},
		}}
	}
	children := []phase.ChildInfo{{ID: "b1", Status: "open", IssueType: "task"}}

	wireFixture := func(t *testing.T) {
		t.Helper()
		restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
			for _, a := range args {
				if a == "--type=epic" {
					return json.Marshal(newEpics())
				}
				if strings.HasPrefix(a, "--parent") {
					return json.Marshal(children)
				}
			}
			return []byte("[]"), nil
		})
		t.Cleanup(restoreList)
		restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
			if len(args) >= 2 && args[0] == "show" && args[1] == epicID {
				return json.Marshal(newEpics())
			}
			return []byte("[]"), nil
		})
		t.Cleanup(restoreRun)
	}

	// --- Leg 1: the SANCTIONED internal caller (release's own sync)
	// succeeds unforced, writing the backward move implement -> plan.
	t.Run("sanctioned_sync_succeeds_unforced", func(t *testing.T) {
		wireFixture(t)
		root := t.TempDir()
		var gotID string
		var gotUpdates map[string]interface{}
		t.Cleanup(phase.SetMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
			gotID, gotUpdates = id, updates
			return nil
		}))

		if err := phase.SanctionedDerivedSync(root, epicID); err != nil {
			t.Fatalf("SanctionedDerivedSync must succeed unforced on a released all-open state: %v", err)
		}
		if gotID != epicID {
			t.Errorf("merge target = %q, want %q", gotID, epicID)
		}
		if gotUpdates["mindspec_phase"] != "plan" {
			t.Errorf("mindspec_phase = %v, want %q (unforced backward move)", gotUpdates["mindspec_phase"], "plan")
		}
		if _, hasReason := gotUpdates["mindspec_phase_force_reason"]; hasReason {
			t.Errorf("the sanctioned internal caller must never record a force reason (that is the operator-only escape): %v", gotUpdates)
		}
	})

	// --- Leg 2: the OPERATOR path (mindspec repair phase, same state,
	// no --force-backward) refuses.
	t.Run("operator_repair_phase_refuses", func(t *testing.T) {
		wireFixture(t)
		root := t.TempDir()
		stubRepairFindRoot(t, root)
		calls := 0
		origMerge := repairMergeMetadataFn
		repairMergeMetadataFn = func(id string, updates map[string]interface{}) error {
			calls++
			return nil
		}
		t.Cleanup(func() { repairMergeMetadataFn = origMerge })

		err := repairPhaseRunE(repairPhaseCmd, []string{specID})
		if err == nil {
			t.Fatal("expected mindspec repair phase to refuse the same released all-open state without --force-backward")
		}
		if !guard.HasFinalRecoveryLine(err.Error()) {
			t.Errorf("refusal must end with a recovery line: %v", err)
		}
		if !strings.Contains(err.Error(), "force-backward") {
			t.Errorf("refusal should name the --force-backward escape: %v", err)
		}
		if calls != 0 {
			t.Errorf("operator guard refusal must perform ZERO merge-write calls, got %d", calls)
		}
	})
}
