package phase_test

// route_248_test.go — spec 128 (lifecycle-verb-state-honesty) Bead 2,
// plan step 5: the two #248 routes. OQ-3 option 1 makes stored and
// derived AGREE at every point of the plan-approve -> first-claim ->
// release window (plan.md's OQ-3 rationale), which dissolves #248's
// self-worsening-recovery-hint warning entirely on BOTH routes rather
// than merely suppressing it. Each probe below constructs the epic
// metadata + children shape a real run would leave at that point and
// asserts (a) stored == derived via phase.DerivePhaseDetail and (b) NO
// stored-vs-derived warning (nor a `repair phase` recovery line) is
// emitted on stderr — AC-5(i)'s "warning is absent" arm, both routes.
// External package (mirrors derive_recovery_test.go / derive_v2_test.go)
// so the same captureStderrPhase helper applies.

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// probeEpicState wires phase's bd seams to a single epic/children
// snapshot and asserts AC-5(i)'s zero-warning, stored==derived contract
// via a state-show-shaped probe (phase.DerivePhaseDetail, the same
// stored-vs-derived pair `state show`'s underlying resolution uses).
func probeEpicState(t *testing.T, label, epicJSON, childrenJSON, wantPhase string) {
	t.Helper()
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if strings.HasPrefix(a, "--parent") {
				return []byte(childrenJSON), nil
			}
		}
		return []byte(epicJSON), nil
	})
	defer restoreList()
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return []byte(epicJSON), nil
		}
		return []byte("[]"), nil
	})
	defer restore()

	var detail phase.PhaseDetail
	var derr error
	stderr := captureStderrPhase(t, func() {
		detail, derr = phase.DerivePhaseDetail("epic-1")
	})
	if derr != nil {
		t.Fatalf("%s: DerivePhaseDetail: %v", label, derr)
	}
	if detail.Stored != wantPhase {
		t.Errorf("%s: stored = %q, want %q", label, detail.Stored, wantPhase)
	}
	if detail.Derived != wantPhase {
		t.Errorf("%s: derived = %q, want %q (stored == derived is the OQ-3 option-1 invariant)", label, detail.Derived, wantPhase)
	}
	if stderr != "" {
		t.Errorf("%s: expected ZERO stderr output (AC-5(i) warning-absent arm), got %q", label, stderr)
	}
	// Belt-and-suspenders: even if some unrelated warning slipped
	// through, the specific self-worsening recovery command must never
	// appear here (AC-5(i)'s literal text).
	if strings.Contains(stderr, "recovery: mindspec repair phase") {
		t.Errorf("%s: must never emit the repair-phase recovery line on an agreeing state: %q", label, stderr)
	}
}

// TestRoute248_PlanApproveShaped: plan approve -> state probes.
func TestRoute248_PlanApproveShaped(t *testing.T) {
	// Probe 1: immediately after `plan approve` (OQ-3 option 1 writes
	// `plan`), all lifecycle children open, none claimed.
	epicPostApprove := `[{"id":"epic-1","title":"[SPEC 010-test] Test","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"plan","spec_num":10,"spec_title":"test","mindspec_phase_semantics":"v2"}}]`
	childrenAllOpen := `[{"id":"b1","status":"open","issue_type":"task"},{"id":"b2","status":"open","issue_type":"task"}]`
	probeEpicState(t, "post-plan-approve", epicPostApprove, childrenAllOpen, state.ModePlan)

	// Probe 2: after the first LIFECYCLE claim (`mindspec next` advances
	// stored to implement), one child in_progress.
	epicPostClaim := `[{"id":"epic-1","title":"[SPEC 010-test] Test","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"implement","spec_num":10,"spec_title":"test","mindspec_phase_semantics":"v2"}}]`
	childrenOneClaimed := `[{"id":"b1","status":"in_progress","issue_type":"task"},{"id":"b2","status":"open","issue_type":"task"}]`
	probeEpicState(t, "post-first-claim", epicPostClaim, childrenOneClaimed, state.ModeImplement)
}

// TestRoute248_ReleaseShaped: claim -> release -> state probes.
func TestRoute248_ReleaseShaped(t *testing.T) {
	// Probe 1: claim (same shape as the plan-approve route's second
	// probe — the two routes converge here).
	epicPostClaim := `[{"id":"epic-2","title":"[SPEC 011-test] Test","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"implement","spec_num":11,"spec_title":"test","mindspec_phase_semantics":"v2"}}]`
	childrenOneClaimed := `[{"id":"c1","status":"in_progress","issue_type":"task"}]`
	probeEpicState(t, "claim", epicPostClaim, childrenOneClaimed, state.ModeImplement)

	// Probe 2: release (the sanctioned sync returns stored to plan via
	// phase.SanctionedDerivedSync — proven to succeed unforced by
	// TestAC5viii in cmd/mindspec; here the state AFTER that sync has
	// already landed is what gets probed), the released bead back to
	// open.
	epicPostRelease := `[{"id":"epic-2","title":"[SPEC 011-test] Test","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"plan","spec_num":11,"spec_title":"test","mindspec_phase_semantics":"v2"}}]`
	childrenAllOpen := `[{"id":"c1","status":"open","issue_type":"task"}]`
	probeEpicState(t, "post-release", epicPostRelease, childrenAllOpen, state.ModePlan)
}

// TestRoute248_ComplexTriggerTwo_PlanOverPlanNeverBackward is the
// inherited round-6 R3 advisory, discharged by OQ-3 option 1 (plan.md's
// "(a) round-6 R3 (trigger-2 / OQ-3 interlock)"): a non-lifecycle child
// (e.g. a `bug`) completed while ALL lifecycle tasks remain open and
// none was ever claimed — `complete`'s step-6.5 NextMode=plan write in
// this state is plan-over-plan, not backward, and needs no sanctioning.
func TestRoute248_ComplexTriggerTwo_PlanOverPlanNeverBackward(t *testing.T) {
	epic := `[{"id":"epic-3","title":"[SPEC 012-test] Test","status":"open","issue_type":"epic","metadata":{"mindspec_phase":"plan","spec_num":12,"spec_title":"test","mindspec_phase_semantics":"v2"}}]`
	// A closed non-lifecycle bug child + all-open lifecycle children:
	// the lifecycle-only derivation (spec 095) ignores the bug entirely.
	children := `[{"id":"bug-1","status":"closed","issue_type":"bug"},{"id":"t1","status":"open","issue_type":"task"}]`
	probeEpicState(t, "trigger-2 complete-to-plan", epic, children, state.ModePlan)

	if phase.IsBackwardMove(state.ModePlan, state.ModePlan) {
		t.Error("plan -> plan must never classify as a backward move")
	}
}
