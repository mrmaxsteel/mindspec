package lifecycle

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// AllowNetDeletionFlag is the spec 127 R4(b) audited-override flag
// spelling, byte-identical to internal/executor's own copy
// (executor.AllowNetDeletionFlag) — internal/executor may not import this
// enforcement package (its own doc comment's boundary), so the two
// constants are independently declared. cmd/mindspec's
// TestAllowNetDeletionFlag_ExecutorAndLifecycleAgree (the natural home,
// since it already imports both packages) pins them equal.
const AllowNetDeletionFlag = "--allow-net-deletion"

// EvaluateWorkDestructionPreflight is the shared §1-phase (verb-layer)
// wrapper spec 127 R4(a) requires internal/complete and internal/approve
// to consult before their own terminal mutation: it evaluates the shared
// work-destruction predicate (EvaluateWorkDestruction, ADR-0030's boundary
// wrapper over gitutil.EvaluateWorkDestruction) for a candidate
// branch→target merge and returns a refusal error unless the outcome is
// permissive (DestructionAncestor/DestructionClean) or overrideReason is
// non-empty (the audited escape hatch).
//
// This is deliberately a SEPARATE decision from internal/executor's own
// preflightMergeDestruction (mindspec_executor.go): internal/executor may
// not import this package (the executor/enforcement boundary its own doc
// comment declares), so the two call sites are independent, both
// re-deriving the SAME predicate fresh at their own moment — the §1 call
// here runs BEFORE this verb's first mutation (an early, non-mutating
// refusal); the executor's own call runs immediately before its
// MergeInto/MergeBranch (the backstop that actually gates the mutation
// regardless of what this call observed). Rather than share logic across
// a boundary that must not be crossed, both copies independently apply
// the identical "only Ancestor/Clean proceed, everything else refuses
// unless overridden" decision — see internal/executor's
// preflightMergeDestruction doc comment for the full zero-value analysis,
// which applies identically here: outcome is never a parameter a caller
// supplies, it is always this function's own fresh call into
// EvaluateWorkDestruction below.
//
// Cross-layer divergence, stated honestly (bead-6 fix round 1, O1/G1):
// see internal/executor's preflightMergeDestruction doc comment's
// "CROSS-LAYER DIVERGENCE" section for the full accounting. In short —
// the PREDICATE cannot diverge (both layers' seams are independently
// pointer-pinned to the same gitutil.EvaluateWorkDestruction); the two
// DISPOSITION SWITCHES (this one, and the executor's) are separately
// coded and NOT mechanically cross-checked against each other — each
// side's own disposition-table test proves only its OWN internal
// completeness against guard.DestructionOutcomeCount, never mutual
// agreement. This is a stated, review-caught residual, not a claim that
// something here prevents it.
func EvaluateWorkDestructionPreflight(workdir, branch, target, overrideReason, rerun string) error {
	outcome, evidence, err := EvaluateWorkDestruction(workdir, branch, target)
	if err != nil {
		return workDestructionEvidenceErrorRefusal(branch, target, err, overrideReason, rerun)
	}

	switch outcome {
	case guard.DestructionAncestor, guard.DestructionClean:
		// The only two permissive dispositions. No default arm (spec 127
		// B-r4-3): any outcome this switch does not name — including a
		// future DestructionOutcome variant — falls through to the
		// refusal below rather than being silently absorbed here.
		_ = evidence
		return nil
	}

	if outcome == guard.DestructionEvidenceError {
		// Defense-in-depth: see the identical leg in internal/executor's
		// preflightMergeDestruction for why this is checked explicitly
		// rather than trusted to the err != nil branch above alone.
		return workDestructionEvidenceErrorRefusal(branch, target, fmt.Errorf("evaluated as evidence-error with no accompanying error value"), overrideReason, rerun)
	}

	if overrideReason != "" {
		return nil
	}
	return workDestructionRefusal(outcome, branch, target, evidence, rerun)
}

func workDestructionEvidenceErrorRefusal(branch, target string, cause error, overrideReason, rerun string) error {
	if overrideReason != "" {
		return nil
	}
	return guard.NewFailure(
		fmt.Sprintf("could not determine whether merging %s into %s would destroy work (evidence unavailable: %s) — refusing to merge without evidence.",
			termsafe.Escape(branch), termsafe.Escape(target), termsafe.Escape(cause.Error())),
		fmt.Sprintf("retry once the underlying failure is resolved, or re-run with %s \"<reason>\" to proceed without verified evidence — then %s", AllowNetDeletionFlag, rerun),
	)
}

func workDestructionRefusal(outcome guard.DestructionOutcome, branch, target string, evidence gitutil.WorkDestructionEvidence, rerun string) error {
	var detail string
	switch outcome {
	case guard.DestructionSuperseded:
		detail = fmt.Sprintf("its content already landed in %s (via %s) — merging this stale snapshot would regress landed work", termsafe.Escape(target), termsafe.Escape(evidence.SupersededVia))
	case guard.DestructionStaleDeletion:
		detail = fmt.Sprintf("the merge preview would delete content in %s that this branch's own authored work never carried (a staleness artifact, not an authored deletion) — merging would delete landed work", termsafe.Escape(target))
	default:
		detail = fmt.Sprintf("evaluated to an unrecognized work-destruction outcome (%v) — refusing rather than assuming it is safe to merge", outcome)
	}
	return guard.NewFailure(
		fmt.Sprintf("refusing to merge %s into %s: %s.", termsafe.Escape(branch), termsafe.Escape(target), detail),
		fmt.Sprintf("inspect the branch, then re-run with %s \"<reason>\" to proceed anyway — then %s", AllowNetDeletionFlag, rerun),
	)
}
