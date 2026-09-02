package approve

// spec_replay_test.go — spec 128 (lifecycle-verb-state-honesty) Bead 7,
// R3.6/AC-16: `spec approve`'s existing-epic replay must never regress a
// later stored phase back to "plan", and its output must honestly
// distinguish a genuine backfill from a no-op. Tests drive
// backfillExistingEpicPhase directly (extracted from ApproveSpec's
// existing-epic branch for exactly this purpose — spec.go's doc comment)
// so no spec.md validation or executor commit machinery is needed.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// wireBackfillFixture stubs phase's bd `show` seam so
// backfillExistingEpicPhase's fresh, under-lock FindEpic read of epicID
// sees the given stored mindspec_phase. present=false with any stored
// value omits the metadata key entirely (a genuinely absent field);
// present=true sets it to stored (including stored=="" for the
// present-but-invalid case, though no AC-16 leg below exercises that
// combination — the absent-key leg is the one R3.6 names).
func wireBackfillFixture(t *testing.T, epicID, stored string, present bool) {
	t.Helper()
	metadata := map[string]interface{}{}
	if present {
		metadata["mindspec_phase"] = stored
	}
	epicJSON, err := json.Marshal([]phase.EpicInfo{{
		ID: epicID, Title: "[SPEC 010-test] Test", Status: "open", IssueType: "epic", Metadata: metadata,
	}})
	if err != nil {
		t.Fatalf("marshal epic fixture: %v", err)
	}
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == epicID {
			return epicJSON, nil
		}
		return []byte("[]"), nil
	}))
}

// AC-16's backfill leg: an existing epic with EMPTY stored phase (the
// mindspec_phase key genuinely absent) gets "plan" written under the
// R3.3 lock, and the output says so.
func TestBackfillExistingEpicPhase_EmptyStoredPhaseBackfillsPlan(t *testing.T) {
	root := t.TempDir()
	epicID := "epic-128-empty"
	wireBackfillFixture(t, epicID, "", false)
	var gotUpdates map[string]interface{}
	calls := 0
	restore := SetSpecMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
		calls++
		gotUpdates = updates
		return nil
	})
	defer restore()

	msg, err := backfillExistingEpicPhase(root, epicID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly ONE write for a genuine backfill, got %d", calls)
	}
	if gotUpdates["mindspec_phase"] != "plan" {
		t.Errorf("expected mindspec_phase=plan to be written, got %v", gotUpdates)
	}
	if !strings.Contains(msg, "backfilled") {
		t.Errorf("expected the message to name the backfill mutation, got %q", msg)
	}
}

// AC-16's same-value leg: stored phase is already "plan" — the replay's
// output identifies that nothing was mutated and does not claim a
// backfill occurred.
func TestBackfillExistingEpicPhase_SameValuePlanReplayIsNoOp(t *testing.T) {
	root := t.TempDir()
	epicID := "epic-128-plan"
	wireBackfillFixture(t, epicID, "plan", true)
	calls := 0
	restore := SetSpecMergeMetadataForTest(func(string, map[string]interface{}) error {
		calls++
		return nil
	})
	defer restore()

	msg, err := backfillExistingEpicPhase(root, epicID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 0 {
		t.Errorf("a same-value replay must be a ZERO-write no-op, got %d calls", calls)
	}
	if strings.Contains(msg, "backfilled") {
		t.Errorf("a same-value no-op must never claim a backfill occurred: %q", msg)
	}
	if !strings.Contains(msg, "plan") {
		t.Errorf("expected the message to name the same-value phase, got %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "nothing mutated") {
		t.Errorf("expected the message to say nothing was mutated, got %q", msg)
	}
}

// AC-16's core red-on-revert legs: an existing lifecycle epic with
// stored mindspec_phase=implement (and the review/done variants —
// including a closed-out epic's replay) leaves the stored phase at the
// later value, UNCHANGED — never regressed to "plan" — and the output
// names the untouched value rather than claiming a backfill.
func TestBackfillExistingEpicPhase_LaterPhaseReplayLeavesItUntouched(t *testing.T) {
	for _, stored := range []string{"implement", "review", "done"} {
		stored := stored
		t.Run(stored, func(t *testing.T) {
			root := t.TempDir()
			epicID := "epic-128-" + stored
			wireBackfillFixture(t, epicID, stored, true)
			calls := 0
			restore := SetSpecMergeMetadataForTest(func(string, map[string]interface{}) error {
				calls++
				return nil
			})
			defer restore()

			msg, err := backfillExistingEpicPhase(root, epicID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if calls != 0 {
				t.Errorf("a later-phase replay must be a ZERO-write no-op (never regress %q back to plan) — red-on-revert against the pre-128 unconditional write; got %d calls", stored, calls)
			}
			if strings.Contains(msg, "backfilled") {
				t.Errorf("a later-phase no-op must never claim a backfill occurred: %q", msg)
			}
			if !strings.Contains(msg, stored) {
				t.Errorf("expected the message to name the untouched stored phase %q, got %q", stored, msg)
			}
		})
	}
}
