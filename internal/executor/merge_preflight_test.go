package executor

// Spec 127 bead 6 (R4/AC-7/AC-8): unit-level tests for
// preflightMergeDestruction — the merge preflight gate every producer
// consults. These stub the in-package workDestructionFn seam directly
// (no real git repo needed), isolating the DECISION logic from
// gitutil.EvaluateWorkDestruction's own git plumbing (which
// internal/gitutil's own tests already cover).

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

func stubWorkDestructionFn(t *testing.T, fn func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error)) {
	t.Helper()
	orig := workDestructionFn
	t.Cleanup(func() { workDestructionFn = orig })
	workDestructionFn = fn
}

// TestWorkDestructionFn_DefaultsToRealImplementation is the AC-17
// anti-drift pointer-pin (S3-r2-6): the seam must default to the real
// gitutil.EvaluateWorkDestruction, never a private reimplementation.
func TestWorkDestructionFn_DefaultsToRealImplementation(t *testing.T) {
	if reflect.ValueOf(workDestructionFn).Pointer() != reflect.ValueOf(gitutil.EvaluateWorkDestruction).Pointer() {
		t.Fatal("workDestructionFn must default to gitutil.EvaluateWorkDestruction (spec 127 R4(a), S3-r2-6)")
	}
}

// --- THE ZERO-VALUE TRAP ---

// TestPreflightMergeDestruction_ZeroValueOutcomeWithErrorRefuses is the
// central hazard this bead exists to close: guard.DestructionAncestor
// is guard.DestructionOutcome's ZERO VALUE, and Ancestor is ALSO the
// gate's PERMISSIVE disposition. A seam that returns the zero-value
// outcome ALONGSIDE a non-nil error (the shape a buggy or degenerate
// evaluator might produce — "I don't actually know, here's a zero
// value") must NOT be read as "ancestor, proceed" — the err != nil
// check must dominate, unconditionally, regardless of what the
// accompanying outcome value says.
func TestPreflightMergeDestruction_ZeroValueOutcomeWithErrorRefuses(t *testing.T) {
	stubWorkDestructionFn(t, func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
		var zeroOutcome guard.DestructionOutcome // == guard.DestructionAncestor
		return zeroOutcome, gitutil.WorkDestructionEvidence{}, errors.New("simulated evaluator failure")
	})
	g := &MindspecExecutor{Root: "/nonexistent"}
	err := g.preflightMergeDestruction("bead/x", "spec/y", "", "mindspec complete x")
	if err == nil {
		t.Fatal("a zero-value DestructionOutcome accompanied by a non-nil error must refuse, never proceed as if it were a genuine Ancestor answer")
	}
	if !strings.Contains(err.Error(), "could not determine") {
		t.Errorf("expected the evidence-error refusal wording, got: %v", err)
	}
}

// TestPreflightMergeDestruction_UnrecognizedOutcomeRefusesNotDefaultsToProceed
// proves the switch's "no default arm" discipline (spec 127 B-r4-3):
// an outcome value that matches NEITHER of the two explicit permissive
// cases (Ancestor, Clean) — including a value outside the guard
// package's own closed enum, simulating a future variant nobody has
// added a case for yet — must fall through to the FAIL-CLOSED refusal,
// never be silently treated as safe to merge.
func TestPreflightMergeDestruction_UnrecognizedOutcomeRefusesNotDefaultsToProceed(t *testing.T) {
	const futureUnknownOutcome guard.DestructionOutcome = 99 // outside guard.DestructionOutcomeCount
	stubWorkDestructionFn(t, func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
		return futureUnknownOutcome, gitutil.WorkDestructionEvidence{}, nil
	})
	g := &MindspecExecutor{Root: "/nonexistent"}
	err := g.preflightMergeDestruction("bead/x", "spec/y", "", "mindspec complete x")
	if err == nil {
		t.Fatal("an outcome value with no explicit permissive case must refuse — a switch with no default arm must not silently proceed on an unhandled value")
	}
}

// TestPreflightMergeDestruction_PermissiveOutcomesProceed pins the two
// PERMISSIVE dispositions explicitly: only Ancestor and Clean return
// nil (permit the caller's normal merge call to proceed).
func TestPreflightMergeDestruction_PermissiveOutcomesProceed(t *testing.T) {
	for _, outcome := range []guard.DestructionOutcome{guard.DestructionAncestor, guard.DestructionClean} {
		t.Run(outcome.String(), func(t *testing.T) {
			stubWorkDestructionFn(t, func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
				return outcome, gitutil.WorkDestructionEvidence{AncestorOf: target}, nil
			})
			g := &MindspecExecutor{Root: "/nonexistent"}
			if err := g.preflightMergeDestruction("bead/x", "spec/y", "", "mindspec complete x"); err != nil {
				t.Errorf("outcome %s must proceed (return nil), got: %v", outcome, err)
			}
		})
	}
}

// TestPreflightMergeDestruction_DestructiveOutcomesRefuseWithoutOverride
// pins AC-7(i): Superseded and StaleDeletion refuse, naming the
// evidence and the --allow-net-deletion flag, when no override reason
// is supplied.
func TestPreflightMergeDestruction_DestructiveOutcomesRefuseWithoutOverride(t *testing.T) {
	for _, outcome := range []guard.DestructionOutcome{guard.DestructionSuperseded, guard.DestructionStaleDeletion} {
		t.Run(outcome.String(), func(t *testing.T) {
			stubWorkDestructionFn(t, func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
				return outcome, gitutil.WorkDestructionEvidence{SupersededVia: target}, nil
			})
			g := &MindspecExecutor{Root: "/nonexistent"}
			err := g.preflightMergeDestruction("bead/x", "spec/y", "", "mindspec complete x")
			if err == nil {
				t.Fatalf("outcome %s must refuse without an override", outcome)
			}
			if !strings.Contains(err.Error(), AllowNetDeletionFlag) {
				t.Errorf("refusal must name %s, got: %v", AllowNetDeletionFlag, err)
			}
			if !strings.Contains(err.Error(), "mindspec complete x") {
				t.Errorf("refusal must name the rerun invocation, got: %v", err)
			}
		})
	}
}

// TestPreflightMergeDestruction_OverrideCompletesDestructiveOutcomes is
// AC-7(ii)'s core: the SAME destructive outcomes proceed when an
// override reason is supplied — proving the override is consulted, not
// merely documented.
func TestPreflightMergeDestruction_OverrideCompletesDestructiveOutcomes(t *testing.T) {
	for _, outcome := range []guard.DestructionOutcome{guard.DestructionSuperseded, guard.DestructionStaleDeletion, guard.DestructionEvidenceError} {
		t.Run(outcome.String(), func(t *testing.T) {
			stubWorkDestructionFn(t, func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
				if outcome == guard.DestructionEvidenceError {
					return outcome, gitutil.WorkDestructionEvidence{}, errors.New("simulated infra failure")
				}
				return outcome, gitutil.WorkDestructionEvidence{}, nil
			})
			g := &MindspecExecutor{Root: "/nonexistent"}
			if err := g.preflightMergeDestruction("bead/x", "spec/y", "genuinely reviewed and safe", "mindspec complete x"); err != nil {
				t.Errorf("outcome %s with a non-empty override reason must proceed, got: %v", outcome, err)
			}
		})
	}
}

// TestPreflightMergeDestruction_EvidenceErrorRefusesRetryableWithOverrideNamed
// is AC-8(iv): a git/infra failure fails closed, retryable, naming the
// override as still available.
func TestPreflightMergeDestruction_EvidenceErrorRefusesRetryableWithOverrideNamed(t *testing.T) {
	stubWorkDestructionFn(t, func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
		return guard.DestructionEvidenceError, gitutil.WorkDestructionEvidence{FailedProbe: "IsAncestor"}, fmt.Errorf("simulated git failure")
	})
	g := &MindspecExecutor{Root: "/nonexistent"}
	err := g.preflightMergeDestruction("bead/x", "spec/y", "", "mindspec complete x")
	if err == nil {
		t.Fatal("evidence-error must refuse without an override")
	}
	if !strings.Contains(err.Error(), "retry") {
		t.Errorf("evidence-error refusal must name retry, got: %v", err)
	}
	if !strings.Contains(err.Error(), AllowNetDeletionFlag) {
		t.Errorf("evidence-error refusal must name the override as available, got: %v", err)
	}
}
