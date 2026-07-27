package executor

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// workDestructionFn is the R4 preflight's seam over the shared predicate
// (gitutil.EvaluateWorkDestruction), pointer-pinned to the AC-17 anti-drift
// consumer set alongside netEffectLandedFn above (spec 127 R4(a), S3-r2-6):
// "the preflight consumes gitutil.NetEffectLanded/lifecycle.FindLandedMerge
// through the same symbols the existing netEffectLandedFn seam pins" — no
// private reimplementation, no second independently-rewirable seam. Tests
// override this to force DestructionEvidenceError/specific outcomes without
// a real git repo.
var workDestructionFn = gitutil.EvaluateWorkDestruction

// AllowNetDeletionFlag is the exact flag spelling every layer (cmd-layer
// help text, the friction registry, the redact escape-hatch token table,
// and every refusal message below) must render byte-identically, so a
// re-spelling in one place can never silently diverge from another.
const AllowNetDeletionFlag = "--allow-net-deletion"

// preflightMergeDestruction is the ONE function every lifecycle merge
// producer in this package calls IMMEDIATELY before its
// gitutil.MergeInto/MergeBranch — spec 127 R4(a)'s mandatory preflight,
// consulted at CompleteBead's bead→spec merge, FinalizeEpic's bead→spec
// auto-merge, and the direct spec→main merge (the three enumerated call
// sites; internal/executor/merge_chokepoint_test.go asserts there are no
// others).
//
// # THE ZERO-VALUE TRAP
//
// guard.DestructionAncestor is guard.DestructionOutcome's zero value
// (outcome.go), and Ancestor is ALSO this gate's PERMISSIVE disposition —
// the exact inversion of every prior spec-127 bead, where the zero value
// was the FAIL-CLOSED answer. An unevaluated, defaulted, or stale
// DestructionOutcome variable therefore reads identically to "safe to
// merge". This function closes that hole STRUCTURALLY rather than by
// convention or by a non-zero check (a non-zero check cannot distinguish a
// genuine Ancestor answer from an unevaluated zero value — they are the
// same bit pattern): it is the ONLY function in this file that calls
// workDestructionFn, and it ALWAYS calls it itself, from g.Root, over the
// CURRENT branch/target tips — there is no (guard.DestructionOutcome)
// parameter for a caller to pass in, so there is no zero-value or stale-
// value argument for a caller to forget to populate. Every producer's
// obligation reduces to "call preflightMergeDestruction before your merge,
// with the real branch/target operands" — which
// internal/executor/merge_chokepoint_test.go verifies by dataflow (a real
// evaluation feeding the merge call), not by call-site proximity.
//
// Walking every route an unevaluated outcome could take to reach a merge:
//   - A producer that never calls this function at all: the merge runs
//     unguarded. This is NOT prevented by any runtime mechanism — it is
//     caught by merge_chokepoint_test.go's enumeration of every
//     gitutil.MergeInto/MergeBranch call site in this package, which
//     fails the build's test suite (not a runtime guard) if a call site
//     is added with no preceding call into this function.
//   - A struct literal or "var outcome guard.DestructionOutcome" defaulting
//     silently: impossible for a CALLER to construct, because outcome is
//     never a parameter here — it is always the fresh return value of
//     workDestructionFn(g.Root, branch, target), called on the line below.
//   - An error path that forgets to return: workDestructionFn's own
//     contract (gitutil.EvaluateWorkDestruction's doc comment) is that a
//     non-nil err always accompanies guard.DestructionEvidenceError, never
//     a permissive outcome — but this function does not TRUST that
//     contract silently: it checks err first, unconditionally, before the
//     outcome switch, so any err short-circuits to the fail-closed
//     refusal regardless of what the (possibly buggy) accompanying
//     outcome value says.
//   - A seam var reassigned to nil or a stub that panics: workDestructionFn
//     is a package-level var — if some future change ever left it nil, the
//     call below panics rather than silently returning a zero
//     guard.DestructionOutcome; a panic surfaces as a hard test/CLI
//     failure, not a silent merge.
//   - A new DestructionOutcome variant added to package guard without
//     updating this function: the switch below names ONLY the two
//     PERMISSIVE outcomes (Ancestor, Clean) as explicit cases and carries
//     NO default arm (spec 127 B-r4-3's discipline) — a value that
//     matches neither falls through to the refusal below. A new variant
//     is therefore fail-CLOSED by construction, not merely by discipline:
//     the permissive path requires an explicit, named case; everything
//     else — known destructive outcomes, evidence-error, and any future
//     outcome nobody has written a case for yet — refuses.
func (g *MindspecExecutor) preflightMergeDestruction(branch, target, overrideReason, rerun string) error {
	outcome, evidence, err := workDestructionFn(g.Root, branch, target)
	if err != nil {
		return evidenceErrorRefusal(branch, target, err, overrideReason, rerun)
	}

	switch outcome {
	case guard.DestructionAncestor, guard.DestructionClean:
		// The only two PERMISSIVE dispositions: let the caller's normal
		// MergeInto/MergeBranch call proceed exactly as it would without
		// this preflight existing at all.
		//
		// DestructionAncestor's own doc comment (guard/outcome.go) and
		// gitutil.EvaluateWorkDestruction's doc comment both flag that
		// "ancestor" has two sub-cases (evidence.AncestorOf == target, a
		// true no-op; evidence.AncestorOf == "main" but not target, a
		// REAL tree-changing merge) that a consumer must not conflate by
		// SKIPPING the merge call on the strength of the outcome alone.
		// This function never skips the merge call in either case — it
		// only decides proceed-vs-refuse; the actual MergeInto/MergeBranch
		// call the caller makes right after this returns nil is what
		// performs (or naturally no-ops) the real git operation. So both
		// sub-cases are handled correctly by construction, without this
		// function needing to inspect evidence.AncestorOf at all.
		_ = evidence
		return nil
	}

	// Every other value — DestructionSuperseded, DestructionStaleDeletion,
	// DestructionEvidenceError (defense-in-depth: workDestructionFn's
	// contract already routed a non-nil err through the branch above, so
	// reaching this outcome with err == nil would itself be a contract
	// violation — still refused, never silently proceeded), and any
	// outcome value this switch has no case for — falls through here.
	if outcome == guard.DestructionEvidenceError {
		return evidenceErrorRefusal(branch, target, fmt.Errorf("evaluated as evidence-error with no accompanying error value from workDestructionFn"), overrideReason, rerun)
	}

	if overrideReason != "" {
		// Audited override (R4(b)): the caller (internal/complete,
		// internal/approve) is responsible for recording the reason on
		// bead/epic metadata and the friction journal AFTER this producer
		// call returns nil — this function only decides proceed-vs-refuse.
		return nil
	}

	return destructionRefusal(outcome, branch, target, evidence, rerun)
}

// evidenceErrorRefusal is the F3-3 fail-closed, retryable leg (the
// `:640-659` ancestry-error precedent): the predicate could not be
// evaluated at all. The override remains available per spec (R4(c)):
// an operator who has independently verified safety by inspection is not
// permanently blocked by an infra failure the tool cannot resolve itself.
func evidenceErrorRefusal(branch, target string, cause error, overrideReason, rerun string) error {
	if overrideReason != "" {
		return nil
	}
	return guard.NewFailure(
		fmt.Sprintf("could not determine whether merging %s into %s would destroy work (evidence unavailable: %s) — refusing to merge without evidence.",
			termsafe.Escape(branch), termsafe.Escape(target), termsafe.Escape(cause.Error())),
		fmt.Sprintf("retry once the underlying failure is resolved, or re-run with %s \"<reason>\" to proceed without verified evidence — then %s", AllowNetDeletionFlag, rerun),
	)
}

// destructionRefusal is the R4(b) block: names the evidence class and the
// audited override, before any mutation.
func destructionRefusal(outcome guard.DestructionOutcome, branch, target string, evidence gitutil.WorkDestructionEvidence, rerun string) error {
	var detail string
	switch outcome {
	case guard.DestructionSuperseded:
		detail = fmt.Sprintf("its content already landed in %s (via %s) — merging this stale snapshot would regress landed work", termsafe.Escape(target), termsafe.Escape(evidence.SupersededVia))
	case guard.DestructionStaleDeletion:
		detail = fmt.Sprintf("the merge preview would delete content in %s that this branch's own authored work never carried (a staleness artifact, not an authored deletion) — merging would delete landed work", termsafe.Escape(target))
	default:
		// Fail-closed for any outcome this switch does not name above —
		// including a future DestructionOutcome variant with no fixture
		// in this switch yet (spec 127 B-r4-3): refuse rather than guess.
		detail = fmt.Sprintf("evaluated to an unrecognized work-destruction outcome (%v) — refusing rather than assuming it is safe to merge", outcome)
	}
	return guard.NewFailure(
		fmt.Sprintf("refusing to merge %s into %s: %s.", termsafe.Escape(branch), termsafe.Escape(target), detail),
		fmt.Sprintf("inspect the branch, then re-run with %s \"<reason>\" to proceed anyway — then %s", AllowNetDeletionFlag, rerun),
	)
}
