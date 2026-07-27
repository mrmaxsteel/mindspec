package lifecycle

// Spec 127 bead-6 fix round 1 (G1-4): EvaluateWorkDestructionPreflight
// (merge_preflight.go) had NO direct outcome-table test at all before
// this file — internal/executor's own preflightMergeDestruction has
// hand-written per-outcome tests, but this package's §1-phase wrapper
// (the SEPARATE, independently-coded copy of the identical decision —
// merge_preflight.go's own doc comment) was unjoined from the spec's
// enum-exhaustiveness mechanism entirely. This file closes that gap: a
// complete disposition table asserted against guard.DestructionOutcomeCount,
// stubbing the in-package evaluateWorkDestructionFn seam (gitquery.go) so
// no real git repo is needed.

import (
	"errors"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// TestEvaluateWorkDestructionPreflight_FullDispositionTable is the §1-
// phase wrapper's own join of the spec's enum-exhaustiveness mechanism
// (spec 127 B-r4-3): every DestructionOutcome variant gets a named row,
// and the table's length is asserted against guard.DestructionOutcomeCount
// so an appended variant with no row here goes red at development time —
// not merely fail-closed at runtime (which the switch's no-default-arm
// discipline already guarantees; this test proves the OTHER half, that a
// human is forced to make a conscious proceed/refuse decision for it).
func TestEvaluateWorkDestructionPreflight_FullDispositionTable(t *testing.T) {
	type row struct {
		outcome  guard.DestructionOutcome
		proceeds bool
	}
	table := []row{
		{guard.DestructionAncestor, true},
		{guard.DestructionSuperseded, false},
		{guard.DestructionStaleDeletion, false},
		{guard.DestructionClean, true},
		{guard.DestructionEvidenceError, false},
	}
	if len(table) != int(guard.DestructionOutcomeCount) {
		t.Fatalf("disposition table has %d row(s), want %d (guard.DestructionOutcomeCount) — a new DestructionOutcome variant needs a row here", len(table), int(guard.DestructionOutcomeCount))
	}

	for _, r := range table {
		t.Run(r.outcome.String(), func(t *testing.T) {
			orig := evaluateWorkDestructionFn
			t.Cleanup(func() { evaluateWorkDestructionFn = orig })
			evaluateWorkDestructionFn = func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
				if r.outcome == guard.DestructionEvidenceError {
					return r.outcome, gitutil.WorkDestructionEvidence{}, errors.New("simulated evidence-error")
				}
				return r.outcome, gitutil.WorkDestructionEvidence{}, nil
			}
			err := EvaluateWorkDestructionPreflight("/nonexistent", "bead/x", "spec/y", "", "mindspec complete x")
			if r.proceeds && err != nil {
				t.Errorf("outcome %s must proceed (no override needed), got: %v", r.outcome, err)
			}
			if !r.proceeds && err == nil {
				t.Errorf("outcome %s must refuse without an override, got nil", r.outcome)
			}
		})
	}
}

// TestEvaluateWorkDestructionPreflight_OverrideCompletesEveryRefusingOutcome
// is this package's own copy of AC-7(ii): every outcome that refuses
// above proceeds when overrideReason is non-empty, proving the override
// is consulted at THIS layer too, not only internal/executor's.
func TestEvaluateWorkDestructionPreflight_OverrideCompletesEveryRefusingOutcome(t *testing.T) {
	for _, outcome := range []guard.DestructionOutcome{guard.DestructionSuperseded, guard.DestructionStaleDeletion, guard.DestructionEvidenceError} {
		t.Run(outcome.String(), func(t *testing.T) {
			orig := evaluateWorkDestructionFn
			t.Cleanup(func() { evaluateWorkDestructionFn = orig })
			evaluateWorkDestructionFn = func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
				if outcome == guard.DestructionEvidenceError {
					return outcome, gitutil.WorkDestructionEvidence{}, errors.New("simulated evidence-error")
				}
				return outcome, gitutil.WorkDestructionEvidence{}, nil
			}
			if err := EvaluateWorkDestructionPreflight("/nonexistent", "bead/x", "spec/y", "genuinely reviewed and safe", "mindspec complete x"); err != nil {
				t.Errorf("outcome %s with a non-empty override reason must proceed, got: %v", outcome, err)
			}
		})
	}
}
