package executor

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// workDestructionFn is the R4 preflight's seam over the shared predicate
// (gitutil.EvaluateWorkDestruction). Tests override this to force
// DestructionEvidenceError/specific outcomes without a real git repo.
//
// Bead-6 fix round 1 (O2-2 correction): this is NOT a member of spec
// 121's AC-17 anti-drift consumer set — that set has exactly two pinned
// members (this package's netEffectLandedFn, pinned to
// gitutil.NetEffectLanded by TestNetEffectLandedFn_IsGitutilNetEffect
// Landed; internal/lifecycle's finalizeOrphanNetEffectFn, pinned the same
// way). workDestructionFn points at a DIFFERENT, higher-level function
// (gitutil.EvaluateWorkDestruction) that only reaches NetEffectLanded
// transitively, through yet another, independently-rewirable seam one
// level further out (gitutil's own workDestructionNetEffectFn, bead 1,
// pinned by internal/gitutil/workdestruction_test.go). This var follows
// the SAME pointer-pin PATTERN as the AC-17 pair (a package-level seam
// pointer-pinned to its real implementation by its own test,
// TestWorkDestructionFn_DefaultsToRealImplementation) — that pattern is
// the actual governing mechanism (there is no single enumerated AC-17
// registry/table to "join"; S3's ruling on this bead's own panel
// confirms it) — but it is honestly a fourth, additional,
// independently-rewirable seam layered above the two AC-17 members, not
// a fifth member of their set. No live divergence exists today (every
// link in the chain is independently pinned to the real
// gitutil.NetEffectLanded), but a reviewer who updates only the two
// AC-17 tests when auditing anti-drift coverage would incorrectly
// believe this seam is covered by that audit too — it is not; its own
// pin test is the only thing guarding it.
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
// others AMONG THE SHAPES ITS AST VOCABULARY RESOLVES — direct calls,
// alias chains, import aliases, struct fields, and the signature-matched
// parameter/var shapes it fails closed on; a dot-import, an embedded
// field, a map/slice element, or a multi-hop named struct type is
// review-caught, not scanned — see AC-7(iv), amended, and this file's own
// merge_chokepoint_test.go package doc comment).
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
// internal/executor/merge_chokepoint_test.go verifies via a preceding,
// operand-corresponding call to preflightMergeDestruction found BY NAME in
// the same span (an AST match against this function's own name, never a
// dataflow trace of workDestructionFn's evaluation — bead-6 fix round 8:
// a fifth surviving copy of the same mechanism misdescription plan.md:1378
// carried, caught by this round's own sweep; this file never built the
// dataflow-tracing mechanism the prior wording here claimed), not by mere
// call-site proximity.
//
// Walking every route an unevaluated outcome could take to reach a merge:
//   - A producer that never calls this function at all: the merge runs
//     unguarded. This is NOT prevented by any runtime mechanism — for a
//     call site merge_chokepoint_test.go's AST vocabulary can resolve
//     (a direct call, an alias chain, an import alias, or one of the
//     fail-closed parameter/var/field shapes), it fails the build's test
//     suite (not a runtime guard) if the call site is added with no
//     preceding call into this function; for a shape that vocabulary does
//     not attempt (a dot-import, an embedded field, a map/slice element,
//     a multi-hop named struct type — AC-7(iv), amended), this is instead
//     review-caught, under the same in-diff extension obligation R5(a)/
//     R5(b) already rely on, never machine-verified.
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
//
// # CROSS-LAYER DIVERGENCE — WHAT IS MECHANICALLY PINNED AND WHAT IS NOT
//
// Bead-6 fix round 1 (O1/G1's cross-layer-authority finding): this
// function and internal/lifecycle.EvaluateWorkDestructionPreflight are
// two INDEPENDENTLY-CODED copies of the same "only
// Ancestor/Clean proceed" decision (ADR-0030's boundary forces the
// duplication — internal/executor may not import internal/lifecycle in
// production). Stated honestly, not overclaimed:
//
//   - The PREDICATE cannot diverge: this file's workDestructionFn and
//     internal/lifecycle's evaluateWorkDestructionFn are BOTH
//     pointer-pinned (by their own package's test) to the identical
//     gitutil.EvaluateWorkDestruction — a genuinely different predicate
//     answer between the two layers is therefore not a code-drift risk,
//     only an operand/timing-drift risk (the target-drift and
//     branch-drift backstop fixtures, merge_resumption_test.go/
//     internal/complete's own tests, cover that class).
//   - The DISPOSITION SWITCH is NOT mechanically cross-checked: this
//     function's switch (Ancestor/Clean permissive, no default) and
//     lifecycle.EvaluateWorkDestructionPreflight's switch are two
//     separately-written, separately-maintained pieces of Go source. A
//     future edit to ONE that adds, removes, or reclassifies a case
//     without making the identical edit to the OTHER would silently
//     diverge — nothing in this codebase asserts the two switches agree,
//     beyond each one's own guard.DestructionOutcomeCount-length
//     disposition-table test (this package's
//     TestPreflightMergeDestruction_FullDispositionTable;
//     internal/lifecycle's TestEvaluateWorkDestructionPreflight_
//     FullDispositionTable) independently proving each one's OWN
//     internal completeness, not their mutual agreement. This is the
//     honest, currently-unclosed residual G1 named; a future spec that
//     wants it mechanically closed would need a single shared,
//     cross-imported disposition table (which the current ADR-0030
//     boundary forbids) or a generated/reflection-based cross-package
//     conformance test.
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
