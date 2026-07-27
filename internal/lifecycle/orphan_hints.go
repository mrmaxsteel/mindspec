// Spec 127 R2 — the single orphan-recovery-hint derivation. Every
// production consumer that names a recovery action for a closed-but-
// unmerged (orphaned) bead branch — `mindspec complete`'s and
// `mindspec impl approve`'s sibling-orphan refusals, `mindspec doctor`'s
// orphaned-closed-bead check, and `mindspec impl adopt`'s composite-
// incident refusal (R1(g)) — renders its recovery line(s) through
// DeriveOrphanHint below, never by composing "mindspec complete <bead>"
// (or a merge/deletion command) itself. The five outcomes of the shared
// work-destruction predicate (internal/guard.DestructionOutcome) map 1:1
// onto exactly the five hint shapes this file renders (R2(b)).
package lifecycle

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// OrphanHint is DeriveOrphanHint's output: the outcome it was derived
// from, a free-form sentence naming the evidence class that licenses
// Lines (R2(b)'s "destructive hints carry their proof" — empty only for
// the normal-unmerged outcome, where no extra framing is needed), and
// the ordered recovery-command sequence itself. Lines is never empty —
// every outcome names at least one recovery action, even when that
// action is "inspect first" rather than a mutation.
type OrphanHint struct {
	Outcome      guard.DestructionOutcome
	EvidenceNote string
	Lines        []string
}

// DeriveOrphanHint is R2(a)'s single derivation. It takes a
// guard.DestructionOutcome and its gitutil.WorkDestructionEvidence as
// mandatory parameters — Go has no default arguments, so no call site
// can invoke this without naming both (O2-7's structural leg: the
// SIGNATURE cannot be satisfied by omission).
//
// That structural leg is deliberately NOT an evidentiary one, and this
// comment states the distinction precisely rather than overclaiming it
// (the same correction bead 5 recorded for guard.NewDestructiveCommand's
// identically-shaped mandatory outcome parameter — see that
// constructor's doc comment): guard.DestructionAncestor is this type's
// OWN ZERO VALUE (guard/outcome.go), so
// `DeriveOrphanHint(guard.DestructionOutcome(0), gitutil.WorkDestructionEvidence{}, ...)`
// compiles and runs — it renders the ancestor-deletion hint, not a
// panic or a refusal, because a zero DestructionOutcome IS a real,
// legitimate outcome value (bead 1's own outcome.go doc comment: "mere
// possession of a DestructionOutcome value is not proof the predicate
// ran"). What this function's signature actually guarantees is narrower
// and real: a caller cannot render ANY orphan hint without writing down
// SOME outcome value at the call site. The evidentiary guarantee — that
// the outcome passed in was actually produced by a real
// EvaluateWorkDestruction evaluation, not fabricated or defaulted — is
// NOT enforced by this function or its type signature at all; it is
// enforced by review + the emitter-enumeration anti-drift test
// (orphan_hint_emitters_test.go), which pins that every production
// caller of this function reaches it only through EvaluateOrphanHint
// below (which always performs a real evaluation first) — never by
// constructing an outcome value ad hoc. DeriveOrphanHint itself stays a
// PURE function (no I/O) precisely so the AC-3 oracle and this file's
// own fixture table can exercise every outcome hermetically, over
// fixtures built through the real predicate — never a fabricated stub
// value standing in for one.
//
// Pinned per outcome (R2(b)):
//   - DestructionAncestor: the branch is already an ancestor of main
//     (the reachable case for an Orphan — orphans.go:55-57/:181-190: an
//     ancestor-of-SPEC-branch bead branch is never even constructed as
//     an Orphan in the first place, so that sub-case is not fixtured
//     here as a live path; see evidence.AncestorOf's own doc comment).
//     Nothing of the branch's own work is missing from main, so
//     deletion is safe and carries no preserve-first clause — ancestry
//     of main is exactly "every commit is already reachable from
//     main".
//   - DestructionSuperseded: the branch's content already landed via
//     another route. EvaluateWorkDestruction's own evaluation order
//     checks ancestry BEFORE supersession (its doc comment, step 1
//     before step 2), so reaching this outcome at all already proves
//     the branch's tip is NOT an ancestor of target or of main — its
//     own commits are therefore never guaranteed reachable once its ref
//     is deleted, unconditionally (not merely "sometimes"), which is
//     why the preserve-first tag line always precedes the deletion line
//     for this outcome (no extra ancestry probe needed: the outcome
//     itself is the proof). The hint additionally names the full
//     `mindspec impl adopt` invocation, since a superseded branch's
//     home spec is exactly R1's terminal-transition scenario.
//   - DestructionStaleDeletion: merging would delete target-present
//     content the branch never authored. Inspection-first — a concrete
//     `git diff` command — never a destructive line: this predicate
//     leg does not, by itself, license deleting anything (see
//     guard.DestructionOutcome's own doc comment on DestructionClean
//     for the adjacent leg's stated miss surface; this leg's own
//     evidence — evidence.DeletedPaths — names what is at risk).
//   - DestructionEvidenceError: the predicate could not be evaluated.
//     Inspection-first, never the merge — absence of evidence is never
//     treated as safety (the same discipline gitutil.EvaluateWorkDestruction
//     itself states).
//   - DestructionClean: the ordinary, converging case — the hint stays
//     EXACTLY `mindspec complete <bead>`, byte-identical to
//     Orphan.RecoveryCommand()'s pre-existing text (R2(b): "the hint
//     remains exactly mindspec complete <bead>, unchanged"), because
//     this IS the case an ordinary `mindspec complete` re-run safely
//     converges once R4's preflight guards the merge it triggers.
//
// beadID/beadBranch/specID/targetRef are display/composition inputs
// only, never re-derived from evidence: targetRef names the ref
// EvaluateWorkDestruction actually evaluated branch against (the spec
// branch for complete/impl-approve/doctor's consumers, or "main" for
// adopt's R1(g) consumer, which has no surviving spec branch to name).
func DeriveOrphanHint(outcome guard.DestructionOutcome, evidence gitutil.WorkDestructionEvidence, beadID, beadBranch, specID, targetRef string) OrphanHint {
	safeBead := idrender.Bead(beadID)
	safeBranch := termsafe.Escape(beadBranch)
	safeTarget := termsafe.Escape(targetRef)

	switch outcome {
	case guard.DestructionAncestor:
		via := evidence.AncestorOf
		if via == "" {
			via = targetRef
		}
		return deletionHint(outcome, specID, safeBead, safeBranch, termsafe.Escape(via),
			fmt.Sprintf("bead %s's branch %s is already merged — an ancestor of %s per the shared work-destruction predicate — so nothing of its own work is missing; deleting it is safe", safeBead, safeBranch, termsafe.Escape(via)),
			false)
	case guard.DestructionSuperseded:
		via := evidence.SupersededVia
		if via == "" {
			via = targetRef
		}
		note := fmt.Sprintf("bead %s's branch %s is superseded — its content already landed in %s via another route per the shared work-destruction predicate", safeBead, safeBranch, termsafe.Escape(via))
		if evidence.LandedMergeSHA != "" {
			note = fmt.Sprintf("%s (landed via merge %s)", note, termsafe.Escape(evidence.LandedMergeSHA))
		}
		return deletionHint(outcome, specID, safeBead, safeBranch, termsafe.Escape(via), note, true)
	case guard.DestructionStaleDeletion:
		return OrphanHint{
			Outcome: outcome,
			EvidenceNote: fmt.Sprintf(
				"bead %s's branch %s previews a stale-deletion against %s per the shared work-destruction predicate — merging it now would delete work %s already carries that %s never authored; inspect before doing anything else",
				safeBead, safeBranch, safeTarget, safeTarget, safeBranch,
			),
			Lines: []string{fmt.Sprintf("git diff %s %s   (inspect the preview deletions before doing anything else — merging would delete landed work)", safeTarget, safeBranch)},
		}
	case guard.DestructionEvidenceError:
		probe := evidence.FailedProbe
		if probe == "" {
			probe = "an unnamed probe"
		}
		return OrphanHint{
			Outcome: outcome,
			EvidenceNote: fmt.Sprintf(
				"bead %s's branch %s could not be evaluated against %s (%s failed) — evidence is unavailable, and absence of evidence is never treated as safety; inspect before doing anything else",
				safeBead, safeBranch, safeTarget, termsafe.Escape(probe),
			),
			Lines: []string{fmt.Sprintf("git diff %s %s   (inspect once the probe failure above is resolved)", safeTarget, safeBranch)},
		}
	case guard.DestructionClean:
		return OrphanHint{
			Outcome:      guard.DestructionClean,
			EvidenceNote: "",
			Lines:        []string{"mindspec complete " + safeBead},
		}
	}

	// Unreached for any of the five pinned outcomes above (B-r4-3's
	// exhaustiveness discipline: this switch carries no `default` arm
	// over guard.DestructionOutcome, per the type's own doc comment —
	// orphan_hints_test.go's fixture table separately asserts
	// len(table) == guard.DestructionOutcomeCount, so a SIXTH variant
	// added later reds that table rather than silently falling through
	// here unnoticed). A value outside the five pinned outcomes reaches
	// this point only via a future enum change with no fixture yet
	// written for it; fail-closed rather than silently safe.
	return OrphanHint{
		Outcome: outcome,
		EvidenceNote: fmt.Sprintf(
			"bead %s's branch %s evaluated to an unrecognized outcome (%v) — evidence unavailable, never treated as safety; inspect before doing anything else",
			safeBead, safeBranch, outcome,
		),
		Lines: []string{fmt.Sprintf("git diff %s %s   (inspect before doing anything else)", safeTarget, safeBranch)},
	}
}

// deletionHint is the shared render for the two outcomes whose safe
// disposition is deleting beadBranch (DestructionAncestor,
// DestructionSuperseded): it constructs the `git branch -D` line
// through bead 2's evidence-carrying constructor (never a bare string —
// R5(b)), preceded by a preserve-first `git tag` line when preserveFirst
// is set (R2(b)'s "destructive hints carry their proof": for
// DestructionSuperseded this is unconditional, per this file's own
// DeriveOrphanHint doc comment on why no extra ancestry probe is
// needed), and followed by the full `mindspec impl adopt` invocation
// when preserveFirst is set (a superseded branch's home spec is R1's
// terminal-transition scenario; an ancestor-of-main branch has no such
// follow-on — its spec may still be mid-lifecycle).
func deletionHint(outcome guard.DestructionOutcome, specID, safeBead, safeBranch, safeVia, note string, preserveFirst bool) OrphanHint {
	// safeBranch (already termsafe.Escape'd by the caller), never the
	// raw beadBranch, feeds every rendered command line below — a
	// hostile bd-sourced branch name (lifecycle.Orphan.BeadBranch is
	// bd-list free text, never idvalidate'd) must not leak a raw
	// control byte or ANSI escape into a pasteable recovery line, the
	// same discipline this file's EvidenceNote sentences already apply.
	// termsafe.Escape is a no-op for any ordinary branch name, so this
	// changes nothing for the common case.
	deleteCmd, ctorErr := guard.NewDestructiveCommand(fmt.Sprintf("git branch -D %s", safeBranch), outcome)
	if ctorErr != nil {
		// Fail-closed (spec 127 R5(b)): never fall through to an
		// unconstructed deletion string. `git branch -D <branch>` is on
		// the reviewed floor (FamilyGitBranchDeleteForce) so this is
		// unreachable in practice; if the floor ever stopped matching
		// this exact family, refusing to render ANY deletion line here —
		// falling back to inspection-first — is the fail-closed choice.
		return OrphanHint{
			Outcome:      outcome,
			EvidenceNote: fmt.Sprintf("%s (could not construct the deletion recovery: %s)", note, termsafe.Escape(ctorErr.Error())),
			Lines:        []string{fmt.Sprintf("git diff %s %s   (inspect before doing anything else)", safeVia, safeBranch)},
		}
	}

	lines := make([]string, 0, 3)
	if preserveFirst {
		lines = append(lines, fmt.Sprintf(
			"git tag preserve/%s %s   (preserve %s's commits before deleting — its content is not guaranteed reachable from %s)",
			safeBead, safeBranch, safeBead, safeVia,
		))
	}
	lines = append(lines, deleteCmd.String())
	if preserveFirst {
		lines = append(lines, fmt.Sprintf(
			`mindspec impl adopt %s --reason "<why %s's content already reached main outside the lifecycle>"`,
			idrender.Spec(specID), safeBead,
		))
	}
	return OrphanHint{Outcome: outcome, EvidenceNote: note, Lines: lines}
}

// findLandedMergeFn is the enrichment seam (pointer-pinned default,
// mirroring internal/executor's netEffectLandedFn AC-17 pattern) —
// enrichSupersededEvidence below is the ONLY caller.
var findLandedMergeFn = FindLandedMerge

// enrichSupersededEvidence is bead 4's landed-merge attribution slot
// (WorkDestructionEvidence.LandedMergeSHA/FirstParent/SecondParent,
// gitutil/workdestruction.go's own doc comment on those fields):
// enrichment, never the decision — evidence.SupersededVia alone already
// grounds the DestructionSuperseded outcome; this only names WHICH
// merge landed the content, when one can be found, for a richer
// EvidenceNote. Best-effort and silent on failure (never fails the
// whole hint): specBranch is empty, or FindLandedMerge itself errors,
// exactly when adopt's R1(g) consumer calls this — its spec branch does
// not survive at all in that composite-incident shape (AdoptSpec only
// reaches the orphan scan once the branch-present legs above it have
// already established the local spec branch is ABSENT), so there is
// nothing to scan and enrichment correctly no-ops.
func enrichSupersededEvidence(workdir, specBranch, beadID string, outcome guard.DestructionOutcome, evidence gitutil.WorkDestructionEvidence) gitutil.WorkDestructionEvidence {
	if outcome != guard.DestructionSuperseded || specBranch == "" {
		return evidence
	}
	lm, err := findLandedMergeFn(workdir, specBranch, beadID)
	if err != nil || lm == nil {
		return evidence
	}
	evidence.LandedMergeSHA = lm.SHA
	evidence.LandedMergeFirstParent = lm.FirstParent
	evidence.LandedMergeSecondParent = lm.SecondParent
	return evidence
}

// EvaluateOrphanHint is the impure convenience wrapper every production
// consumer actually calls (complete.go, impl.go, orphaned_beads.go; NOT
// adopt.go — R1(g)'s composite-incident refusal evaluates against
// "main" with no surviving spec branch to enrich from, so it calls
// EvaluateWorkDestruction + DeriveOrphanHint directly rather than through
// this specBranch-shaped wrapper): it runs the shared work-destruction
// predicate for beadBranch against specBranch, best-effort enriches a
// DestructionSuperseded outcome via FindLandedMerge, then calls the pure
// DeriveOrphanHint above. workdir is the git working directory the
// predicate and enrichment probes run in.
//
// The predicate's own error return is deliberately discarded here, not
// swallowed silently: EvaluateWorkDestruction's contract (its own doc
// comment) is that ANY git/infra failure returns
// (DestructionEvidenceError, evidence naming the failed probe, non-nil
// err) and NO OTHER path ever returns a non-nil err — so outcome alone
// already carries every bit of information the error would, and
// DeriveOrphanHint's DestructionEvidenceError case renders the
// inspection-first hint from evidence.FailedProbe regardless.
func EvaluateOrphanHint(workdir, beadID, beadBranch, specID, specBranch string) OrphanHint {
	outcome, evidence, _ := EvaluateWorkDestruction(workdir, beadBranch, specBranch)
	evidence = enrichSupersededEvidence(workdir, specBranch, beadID, outcome, evidence)
	return DeriveOrphanHint(outcome, evidence, beadID, beadBranch, specID, specBranch)
}

// EvaluateOrphanHintAgainstMain is adopt.go's R1(g) composite-incident
// consumer's own entrypoint: unlike EvaluateOrphanHint above, it
// evaluates beadBranch against "main" directly, with no spec-branch
// enrichment attempt (R1(g) is reached only once AdoptSpec has already
// established the local spec branch does not survive at all — there is
// nothing for FindLandedMerge to scan). This is also the mechanism that
// lets internal/approve (an ADR-0030 enforcement package, banned from
// importing internal/gitutil directly) obtain a fully-rendered
// OrphanHint without ever needing to name gitutil.WorkDestructionEvidence
// in its own source: every gitutil-typed value stays inside this
// function's own frame.
func EvaluateOrphanHintAgainstMain(workdir, beadID, beadBranch, specID string) OrphanHint {
	outcome, evidence, _ := EvaluateWorkDestruction(workdir, beadBranch, "main")
	return DeriveOrphanHint(outcome, evidence, beadID, beadBranch, specID, "main")
}
