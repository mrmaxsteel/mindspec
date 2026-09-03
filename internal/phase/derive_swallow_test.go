package phase_test

// Spec 128 final-review round 1 (R6 F2): regression pins for the
// swallowed child-fetch errors in derive.go. Under a PARTIAL bd outage
// (epic reads healthy, `bd list --parent` failing) the pre-fix code fed
// DerivePhaseFromChildren a nil child set and either RETURNED a
// fabricated `plan` with a nil error (the no-stored-phase fallback) or
// asserted a `child-derived phase` in the AC-5(iv) consistency warning
// for a derivation that never ran. These tests drive exactly that
// partial-outage shape through the real cache seams. External test
// package so captureStderrPhase (derive_recovery_test.go) is shared.

import (
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// failParentListJSON succeeds for the epic-enumeration query and fails
// for every `bd list --parent` children read — the partial-outage shape
// R6 reproduced live (precisely the residue state of the AC-4b fixture).
func failParentListJSON(epicsPayload string) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--parent" {
				return nil, errors.New("bd list --parent failed: dolt server gone")
			}
		}
		return []byte(epicsPayload), nil
	}
}

func stubEpicShow(t *testing.T, payload string) {
	t.Helper()
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return []byte(payload), nil
		}
		return nil, errors.New("unexpected runBD call: " + strings.Join(args, " "))
	})
	t.Cleanup(restore)
}

// TestDerivePhase_NoStoredPhase_ChildReadFailurePropagates: the
// no-stored-phase fallback must return the child-read error — never a
// fabricated `plan` derived from the nil child set the failed read
// yields.
func TestDerivePhase_NoStoredPhase_ChildReadFailurePropagates(t *testing.T) {
	stubEpicShow(t, `[{"id":"epic-sw1","status":"open","issue_type":"epic","metadata":{}}]`)
	restore := phase.SetListJSONForTest(failParentListJSON(`[]`))
	defer restore()

	got, err := phase.DerivePhaseWithStatusWithCache(phase.NewCache(), "epic-sw1", "")
	if err == nil {
		t.Fatalf("expected the failed child read to propagate; got phase %q with nil error", got)
	}
	if got != "" {
		t.Errorf("no phase may be returned alongside the error, got %q", got)
	}
	if !strings.Contains(err.Error(), "bd list --parent failed") {
		t.Errorf("the underlying bd failure must survive the wrap, got: %v", err)
	}
}

// TestDerivePhase_ClosedEpicNoDoneMarker_ChildReadFailurePropagates: the
// closed-status arm of the same fallback has the same obligation.
func TestDerivePhase_ClosedEpicNoDoneMarker_ChildReadFailurePropagates(t *testing.T) {
	stubEpicShow(t, `[{"id":"epic-sw2","status":"closed","issue_type":"epic","metadata":{}}]`)
	restore := phase.SetListJSONForTest(failParentListJSON(`[]`))
	defer restore()

	got, err := phase.DerivePhaseWithStatusWithCache(phase.NewCache(), "epic-sw2", "")
	if err == nil {
		t.Fatalf("expected the failed child read to propagate; got phase %q with nil error", got)
	}
}

// TestDerivePhase_StoredPhase_ChildReadFailureSkipsConsistencyWarning:
// with a stored phase present, a failed child read makes the AC-5(iv)
// consistency check SKIP — the stored phase is returned and NO warning
// asserting a "child-derived phase" is emitted, because no derivation
// ran (R6's executed repro: the warning previously asserted
// `child-derived phase "plan"` while the shim log showed both list
// calls FAILED).
func TestDerivePhase_StoredPhase_ChildReadFailureSkipsConsistencyWarning(t *testing.T) {
	stubEpicShow(t, `[{"id":"epic-sw3","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"implement"}}]`)
	restore := phase.SetListJSONForTest(failParentListJSON(`[]`))
	defer restore()

	var got string
	var err error
	stderr := captureStderrPhase(t, func() {
		got, err = phase.DerivePhaseWithStatusWithCache(phase.NewCache(), "epic-sw3", "")
	})
	if err != nil {
		t.Fatalf("the stored phase stays authoritative under a failed consistency read, got error: %v", err)
	}
	if got != "implement" {
		t.Errorf("expected the stored phase %q, got %q", "implement", got)
	}
	if strings.Contains(stderr, "child-derived phase") {
		t.Errorf("no consistency warning may assert a child-derived phase when the child read failed, got: %q", stderr)
	}
}

// TestDiscoverActiveSpecs_ChildReadFailurePropagates: the pre-080
// fallback inside the active-spec enumeration must propagate a failed
// child read rather than fabricating `plan` for the epic (or silently
// skipping it, which could serialize a false idle when it is the only
// active spec).
func TestDiscoverActiveSpecs_ChildReadFailurePropagates(t *testing.T) {
	epics := `[{"id":"epic-sw4","title":"[SPEC 004-delta] Delta","status":"open","issue_type":"epic","metadata":{"spec_num":4,"spec_title":"delta"}}]`
	restore := phase.SetListJSONForTest(failParentListJSON(epics))
	defer restore()

	active, err := phase.DiscoverActiveSpecsWithCache(phase.NewCache())
	if err == nil {
		t.Fatalf("expected the failed child read to propagate; got %d active specs with nil error", len(active))
	}
	if !strings.Contains(err.Error(), "bd list --parent failed") {
		t.Errorf("the underlying bd failure must survive the wrap, got: %v", err)
	}
}
