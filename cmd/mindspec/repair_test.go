package main

// Spec 092 Bead 3 (Req 19): `mindspec repair phase <spec-id>` unit
// tests — re-derive from children, write via the bead.MergeMetadata
// seam (merge semantics; the preservation diff itself is pinned in
// internal/bead TestMergeMetadata_PreservesUnrelatedKeys), and emit
// guard-convention failures. The HasFinalRecoveryLine assertions are
// the per-site Req 21 mirror for this package (see
// internal/guard/recovery_convention_test.go).

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// stubRepairPhaseEnv wires the phase package stubs with one epic
// ("epic-92") for spec "092-agent-contract-hardening" whose stored
// phase is `stored` and whose children are all closed (derived =
// review), and swaps the repair merge seam for `merge`.
func stubRepairPhaseEnv(t *testing.T, stored string, merge func(string, map[string]interface{}) error) {
	t.Helper()
	meta := map[string]interface{}{
		"spec_num":             float64(92),
		"spec_title":           "agent-contract-hardening",
		"mindspec_migrated_at": "2026-01-01T00:00:00Z",
	}
	if stored != "" {
		meta["mindspec_phase"] = stored
	}
	epics := []phase.EpicInfo{{
		ID: "epic-92", Title: "[SPEC 092-agent-contract-hardening] Hardening", Status: "open",
		IssueType: "epic", Metadata: meta,
	}}
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return json.Marshal(epics)
			}
			if strings.HasPrefix(a, "--parent") {
				return json.Marshal([]phase.ChildInfo{{ID: "b1", Status: "closed", IssueType: "task"}})
			}
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == "epic-92" {
			return json.Marshal(epics)
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)

	origMerge := repairMergeMetadataFn
	repairMergeMetadataFn = merge
	t.Cleanup(func() { repairMergeMetadataFn = origMerge })
}

func TestRepairPhase_WritesDerivedViaMergeSeam(t *testing.T) {
	var gotID string
	var gotUpdates map[string]interface{}
	calls := 0
	stubRepairPhaseEnv(t, "implement", func(id string, updates map[string]interface{}) error {
		calls++
		gotID = id
		gotUpdates = updates
		return nil
	})

	if err := repairPhaseRunE(repairPhaseCmd, []string{"092-agent-contract-hardening"}); err != nil {
		t.Fatalf("repair phase: %v", err)
	}
	if calls != 1 {
		t.Fatalf("merge seam calls = %d, want 1", calls)
	}
	if gotID != "epic-92" {
		t.Errorf("merge target = %q, want epic-92", gotID)
	}
	// Exactly the derived phase, nothing else: merge semantics mean
	// the update set carries ONLY the key being repaired.
	if len(gotUpdates) != 1 || gotUpdates["mindspec_phase"] != "review" {
		t.Errorf("merge updates = %v, want only mindspec_phase=review", gotUpdates)
	}
}

func TestRepairPhase_UnknownSpecFailsWithRecoveryLine(t *testing.T) {
	stubRepairPhaseEnv(t, "implement", func(id string, updates map[string]interface{}) error {
		t.Error("merge seam must not be called for an unknown spec")
		return nil
	})

	err := repairPhaseRunE(repairPhaseCmd, []string{"099-nope"})
	if err == nil {
		t.Fatal("expected error for unknown spec")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("repair failure must end with a recovery line (Req 12/21): %v", err)
	}
	if strings.Contains(err.Error(), "bd update --metadata") {
		t.Errorf("emitted message contains banned raw metadata command (Req 19): %v", err)
	}
}

func TestRepairPhase_WriteFailureHasRecoveryLine(t *testing.T) {
	stubRepairPhaseEnv(t, "implement", func(id string, updates map[string]interface{}) error {
		return errMock
	})

	err := repairPhaseRunE(repairPhaseCmd, []string{"092-agent-contract-hardening"})
	if err == nil {
		t.Fatal("expected error when the merge-write fails")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("repair write failure must end with a recovery line: %v", err)
	}
	if !strings.Contains(err.Error(), "mindspec repair phase 092-agent-contract-hardening") {
		t.Errorf("recovery should be the re-runnable repair command: %v", err)
	}
	if strings.Contains(err.Error(), "bd update --metadata") {
		t.Errorf("emitted message contains banned raw metadata command (Req 19): %v", err)
	}
}

// errMock is a sentinel error for the write-failure path.
var errMock = &mockErr{}

type mockErr struct{}

func (*mockErr) Error() string { return "dolt offline" }
