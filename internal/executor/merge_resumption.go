package executor

// Spec 127 R5(d): the conflict-recovery re-entry surface. Neither
// conflict emitter (beadToSpecConflictFailure, directMergeConflictFailure)
// prints a raw `git merge` line any longer — the printed recovery names a
// `--resolve-merge` re-entry invocation of the owning lifecycle verb
// instead. This file holds the shared resumption mechanics both
// `mindspec complete --resolve-merge` and `mindspec impl approve
// --resolve-merge` drive: the preserved-merge precondition every
// committing/merging path checks BEFORE mutating, and the resumption
// logic that completes (or re-reports) an already-in-progress merge
// rather than blindly starting a new one.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// ResolveMergeFlag is the exact flag spelling every layer (cmd-layer help
// text, the printed recovery invocations, and every AC-11 resolution
// site) must render byte-identically.
const ResolveMergeFlag = "--resolve-merge"

// preservedMergeError is the R5(d)(v) preserved-merge precondition's own
// error type: a merge is already in progress in a worktree that THIS
// call did not create. Callers that need to distinguish this class from
// an ordinary commit/export failure (e.g. FinalizeEpic's :576 leg, which
// warns-and-continues on an ordinary failure but must hard-refuse on
// exactly this class — E-r5-2) match it via errors.As.
type preservedMergeError struct {
	workdir    string
	conflicted []string
	msg        string
}

func (e *preservedMergeError) Error() string { return e.msg }

// isPreservedMergeError reports whether err is (or wraps) a
// preservedMergeError.
func isPreservedMergeError(err error) bool {
	var pme *preservedMergeError
	return errors.As(err, &pme)
}

// checkNoPreservedMerge is the R5(d)(v) preserved-merge precondition:
// before ANY commit or merge in workdir, refuse fail-closed if a merge is
// ALREADY in progress there — this call did not start it, so it must
// never commit over it (gitutil.CommitAll would silently produce a
// two-parent merge commit under an unrelated subject, conflict markers
// included — the exact bug spec 125 shipped to fix) and never abort it
// (a hand resolution may already be staged). reentryHint names the
// resolution invocation the refusal points to.
func checkNoPreservedMerge(workdir, reentryHint string) error {
	if !gitutil.MergeInProgress(workdir) {
		return nil
	}
	conflicted := gitutil.ConflictedFiles(workdir)
	return preservedMergeRefusal(workdir, conflicted, reentryHint)
}

// preservedMergeRefusal renders the R5(d)(v) refusal: names the
// preserved conflict, its paths, and the re-entry invocation. Mutates
// nothing — the caller has not yet committed or merged.
func preservedMergeRefusal(workdir string, conflicted []string, reentryHint string) *preservedMergeError {
	var b strings.Builder
	fmt.Fprintf(&b, "refusing to commit or merge in %s: a merge is already in progress there that this run did not start.", workdir)
	if len(conflicted) > 0 {
		escaped := make([]string, len(conflicted))
		for i, f := range conflicted {
			escaped[i] = termsafe.Escape(f)
		}
		fmt.Fprintf(&b, "\nconflicted files:\n  %s", strings.Join(escaped, "\n  "))
	}
	fmt.Fprintf(&b, "\nnothing here was touched: resolve the preserved merge first, or re-run %s", reentryHint)
	return &preservedMergeError{workdir: workdir, conflicted: conflicted, msg: b.String()}
}

// noMergeStateError is R5(d)(vi)'s worktree-state failure leg: a merge
// attempt failed to even START (no MERGE_HEAD, no conflicted files) —
// typically a dirty index/worktree blocking `git merge`/`git checkout`.
// This is NOT a predicate outcome and NOT a conflict (E-r5-4): the R4
// preflight is commit-level (merge-tree) and cannot observe a worktree
// blocker, so a bare re-invocation loops forever unless this leg is
// named distinctly.
type noMergeStateError struct {
	msg string
}

func (e *noMergeStateError) Error() string { return e.msg }

// worktreeStateRefusal is R5(d)(vi)'s refusal: names the blocking paths
// and the action that unblocks them, NEVER the word "conflict", and
// never a recovery that re-enters unchanged.
func worktreeStateRefusal(workdir string, blockingPaths []string, mergeErr error) *noMergeStateError {
	var b strings.Builder
	fmt.Fprintf(&b, "could not start the merge in %s: the worktree has uncommitted changes blocking it (no merge is in progress here): %s", workdir, escapeLines(fmt.Sprint(mergeErr)))
	if len(blockingPaths) > 0 {
		escaped := make([]string, len(blockingPaths))
		for i, f := range blockingPaths {
			escaped[i] = termsafe.Escape(f)
		}
		fmt.Fprintf(&b, "\nblocking path(s):\n  %s", strings.Join(escaped, "\n  "))
	}
	b.WriteString("\ncommit or discard the listed changes in that worktree, then re-run.")
	return &noMergeStateError{msg: b.String()}
}

// bindingClass classifies a preserved MERGE_HEAD against the source
// branch THIS invocation was asked to merge (spec 127 bead-6 fix round 1,
// G1-1): the resumption dispatch previously treated ANY preserved
// MERGE_HEAD as "the product-initiated merge for this request", binding
// nothing. See classifyPreservedMergeBinding's doc comment.
type bindingClass int

const (
	// bindingExact: the preserved MERGE_HEAD is exactly the expected
	// source's current tip — the ordinary case, no drift, no foreign
	// merge.
	bindingExact bindingClass = iota
	// bindingDrifted: the expected source has advanced PAST the
	// preserved MERGE_HEAD since the merge began (MERGE_HEAD is an
	// ancestor of the source's current tip) — the same branch, an older
	// snapshot of it.
	bindingDrifted
	// bindingForeign: the preserved MERGE_HEAD is neither the expected
	// source's tip nor an ancestor of it — a merge this invocation did
	// not start and does not recognize (an operator merge, or another
	// bead's preserved conflict in a shared spec worktree).
	bindingForeign
)

// classifyPreservedMergeBinding resolves the preserved MERGE_HEAD in
// workdir and the CURRENT tip of expectedSource, and classifies their
// relationship. Spec 127 bead-6 fix round 1 (G1-1): resumeAwareMerge
// consults this before EVER completing (committing) or re-diagnosing a
// preserved merge, so a merge that does not correspond to the branch this
// invocation was asked to merge is never silently completed or mistaken
// for this request's own conflict.
//
// Bead-6 fix round 2 (G1's confirm-round finding): ancestry ALONE cannot
// establish that a preserved merge is expectedSource, merely drifted.
// preservedTip being an ancestor of expectedSource's current tip is
// necessary but not sufficient for "this is expectedSource's own history,
// just an older snapshot" — a FOREIGN branch's tip can, by pure git
// topology, ALSO be an ancestor of expectedSource's current tip (e.g.
// expectedSource later merged, or was rebased across, that foreign
// branch's history) without the preserved merge being expectedSource's
// own stale attempt at all. Tip equality cannot be required either
// (preservedTip is necessarily frozen at whatever it was when the
// conflict began, while expectedSource's OWN branch may have since
// moved — that is exactly the legitimate drifted case this function must
// still recognize). What durably identifies WHICH branch a preserved
// merge actually started against is the subject git itself seeded at
// merge start (mergeInto's `-m "Merge <source>"` / MergeBranch's
// `-m "Merge <source> into <target>"`, gitutil.MergeMsgSubject) — so an
// ancestor-but-not-identical preserved tip is only bindingDrifted when
// the seeded subject ALSO names expectedSource; otherwise it is
// bindingForeign, exactly like the non-ancestor case, never silently
// trusted on ancestry alone.
func classifyPreservedMergeBinding(workdir, expectedSource string) (class bindingClass, preservedTip, sourceTip string, err error) {
	preservedTip, err = gitutil.RevParseRef(workdir, "MERGE_HEAD")
	if err != nil {
		return 0, "", "", fmt.Errorf("resolving the preserved MERGE_HEAD in %s: %w", workdir, err)
	}
	sourceTip, err = gitutil.RevParseRef(workdir, expectedSource)
	if err != nil {
		return 0, preservedTip, "", fmt.Errorf("resolving the current tip of %s: %w", expectedSource, err)
	}
	if preservedTip == sourceTip {
		return bindingExact, preservedTip, sourceTip, nil
	}
	isAnc, ancErr := gitutil.IsAncestor(workdir, preservedTip, sourceTip)
	if ancErr != nil {
		return 0, preservedTip, sourceTip, fmt.Errorf("could not determine whether the preserved merge in %s corresponds to %s (ancestry check failed: %w)", workdir, expectedSource, ancErr)
	}
	if !isAnc {
		return bindingForeign, preservedTip, sourceTip, nil
	}
	named, subjErr := mergeSubjectNamesSource(workdir, expectedSource)
	if subjErr != nil {
		return 0, preservedTip, sourceTip, fmt.Errorf("could not verify which branch the preserved merge in %s belongs to (%w)", workdir, subjErr)
	}
	if !named {
		return bindingForeign, preservedTip, sourceTip, nil
	}
	return bindingDrifted, preservedTip, sourceTip, nil
}

// mergeSubjectNamesSource reports whether workdir's seeded MERGE_MSG
// subject names expectedSource as the branch being merged — the
// MergeInto ("Merge <source>") or MergeBranch ("Merge <source> into
// <target>") shape gitutil seeds at merge start (gitops.go's own `-m`
// calls). Bead-6 fix round 2: this is the ONE piece of evidence that
// survives expectedSource's OWN tip moving after the merge began —
// ancestry alone cannot distinguish "this IS expectedSource, just an
// older snapshot of it" from "this is some OTHER branch that happens to
// be an ancestor of expectedSource's current tip". Matching is anchored
// on "Merge <expectedSource>" as either the WHOLE subject (MergeInto's
// form) or a prefix immediately followed by " into " (MergeBranch's
// form) — never a bare substring — so a branch name that is a textual
// prefix of another branch's name (e.g. "bead/x" vs "bead/x-extra")
// cannot be confused for a match.
func mergeSubjectNamesSource(workdir, expectedSource string) (bool, error) {
	subject, err := gitutil.MergeMsgSubject(workdir)
	if err != nil {
		return false, err
	}
	want := "Merge " + expectedSource
	if subject == want {
		return true, nil
	}
	return strings.HasPrefix(subject, want+" into "), nil
}

// bindingIndeterminateError is the fail-closed leg when
// classifyPreservedMergeBinding itself cannot resolve a ref or evaluate
// ancestry — an infra failure, not a classification, so this must never
// be read as "safe to complete".
type bindingIndeterminateError struct{ msg string }

func (e *bindingIndeterminateError) Error() string { return e.msg }

// bindingIndeterminateRefusal renders the fail-closed refusal for a
// classification failure: nothing is committed, nothing is touched.
func bindingIndeterminateRefusal(workdir, expectedSource string, cause error, reentryHint string) error {
	return &bindingIndeterminateError{msg: fmt.Sprintf(
		"could not verify that the merge preserved in %s belongs to %s (%s) — refusing to touch it until this is resolved.\nre-run %s once resolved.",
		workdir, termsafe.Escape(expectedSource), termsafe.Escape(cause.Error()), reentryHint,
	)}
}

// foreignMergeError is G1-1's central fix: a preserved MERGE_HEAD that is
// neither this request's expected source tip nor an ancestor of it — a
// merge this invocation did not start and must never commit over or
// treat as its own conflict.
type foreignMergeError struct{ msg string }

func (e *foreignMergeError) Error() string { return e.msg }

// foreignMergeRefusal names the mismatch: the expected source, and (best-
// effort, via gitutil.MergeMsgSubject) what the preserved merge's own
// seeded message says it actually is — so an operator whose invocation
// collides with another bead's preserved conflict in a shared spec
// worktree gets a diagnostic naming the FOREIGN merge, not a silent
// completion or an unexplained ancestry-mismatch failure downstream.
func foreignMergeRefusal(workdir, expectedSource, preservedTip, expectedSourceTip, reentryHint string) *foreignMergeError {
	var b strings.Builder
	fmt.Fprintf(&b, "refusing to touch the merge preserved in %s: it does not correspond to the requested source %s.", workdir, termsafe.Escape(expectedSource))
	if subject, err := gitutil.MergeMsgSubject(workdir); err == nil && subject != "" {
		fmt.Fprintf(&b, "\nthe preserved merge's own message: %s", termsafe.Escape(subject))
	}
	fmt.Fprintf(&b, "\npreserved merge head: %s", termsafe.Escape(preservedTip))
	fmt.Fprintf(&b, "\nrequested source (%s) current tip: %s", termsafe.Escape(expectedSource), termsafe.Escape(expectedSourceTip))
	fmt.Fprintf(&b, "\nnothing here was touched: resolve or complete the preserved (foreign) merge directly if it is expected, or re-run %s once it no longer occupies this worktree.", reentryHint)
	return &foreignMergeError{msg: b.String()}
}

// resumeOutcome is what mergeResumptionStep found on this invocation.
type resumeOutcome int

const (
	// resumeNoMergeInProgress: no MERGE_HEAD present — the caller should
	// proceed to evaluate the R4 preflight and start a fresh merge.
	resumeNoMergeInProgress resumeOutcome = iota
	// resumeStillConflicted: MERGE_HEAD present, conflicted files remain
	// — re-print resolution steps and stop; no merge, no checkout, no
	// abort.
	resumeStillConflicted
	// resumeReadyToComplete: MERGE_HEAD present, conflicted files are
	// gone (resolved and staged) — complete via `git commit --no-edit`.
	resumeReadyToComplete
)

// mergeResumptionStep is the R5(d)(iii) resumption check: on EVERY
// invocation of a --resolve-merge re-entry surface, it consults
// in-progress-merge state BEFORE any merge or checkout. It never starts
// a merge, never checks out, and never aborts — it only classifies.
func mergeResumptionStep(workdir string) resumeOutcome {
	if !gitutil.MergeInProgress(workdir) {
		return resumeNoMergeInProgress
	}
	if len(gitutil.ConflictedFiles(workdir)) > 0 {
		return resumeStillConflicted
	}
	return resumeReadyToComplete
}

// resolutionSteps renders the R5(d)(vii)/E-r5-3 pinned resolution-step
// template: edit, `git add <path>...` over EXACTLY the conflicted set
// (never -A/./−u), then `git commit --no-edit` (subject pre-seeded).
// AC-9(v)(η) asserts the emitted add-step operands equal the emitter-
// computed conflicted set.
func resolutionSteps(workdir string, conflicted []string, reentryHint string) string {
	escaped := make([]string, len(conflicted))
	for i, f := range conflicted {
		escaped[i] = termsafe.Escape(f)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "merge conflict preserved in %s — resolve it:", workdir)
	fmt.Fprintf(&b, "\n  1. edit the conflicted file(s):\n     %s", strings.Join(escaped, "\n     "))
	fmt.Fprintf(&b, "\n  2. git add %s", strings.Join(escaped, " "))
	b.WriteString("\n  3. git commit --no-edit")
	fmt.Fprintf(&b, "\n  4. re-run %s", reentryHint)
	return b.String()
}

// stillConflictedRefusal is the R5(d)(iii) leg where unmerged index
// entries are STILL present: re-print the resolution steps and exit —
// no new merge, no checkout, no abort.
func stillConflictedRefusal(workdir string, reentryHint string) error {
	conflicted := gitutil.ConflictedFiles(workdir)
	return guard.NewFailure(resolutionSteps(workdir, conflicted, reentryHint), reentryHint)
}

// completeResumedMerge is the R5(d)(iii)/(iv) completion leg: the index
// is resolved and MERGE_HEAD is present — complete via `git commit
// --no-edit` (E-r5-5), which retains MERGE_MSG's original subject (the
// identity mechanism, not a courtesy — A-r4-5) unchanged. Never a
// checkout, never an abort.
func completeResumedMerge(workdir string) error {
	return gitutil.CommitNoEdit(workdir)
}

// completeDriftedResumedMerge is bead-6 fix round 2's single-merge
// collapse for the bindingDrifted ready-to-complete leg (G1's confirm-
// round finding on this bead's own fix round 1: the prior two-commit
// sequence — complete the STALE preserved merge, then re-merge the
// CURRENT source tip — was NOT downstream-neutral.
// internal/lifecycle.FindLandedMerge's subject scan sees TWO owned
// candidates naming the SAME bead with DIFFERENT second parents and
// fails closed as genuine ownership ambiguity (spec 125 FIX-2b) — and
// that check cannot safely be loosened to tell "one invocation's own
// drift catch-up" apart from "an operator's independent re-merge of an
// advancing branch" (proven by restoring the two-merge topology against
// TestFindLandedMerge_AmbiguousOwnedSecondParentsRefuses: the identical
// adjacency+ancestor shape is exactly what that test deliberately
// refuses). The fix is at the SOURCE instead: never produce the second,
// independently-identifiable commit.
//
// Mechanics: the preserved (stale) merge is completed for real first —
// completeResumedMerge, `git commit --no-edit` — so the operator's own
// hand-resolution is never discarded or re-asked for, producing an
// intermediate commit C1 (this call's own HEAD immediately after).
// mergeFn then runs the CURRENT source tip against C1 via
// attemptFreshMerge; when that catch-up is itself conflict-free, it
// produces C2. C2's TREE already carries everything the final commit
// needs (the original resolution AND the drift) — commit-tree builds a
// NEW, unreferenced commit object with that EXACT tree but the INTENDED
// two parents (the target's pre-resumption tip, and the source's
// current — post-drift — tip; C1 is never a parent of the final
// result), carrying C1's own preserved subject (E-r5-5/A-r4-5) verbatim.
// reset --soft then moves the branch onto it: safe because the target
// tree is BY CONSTRUCTION identical to what is already checked out, so
// this touches neither the index nor the working tree.
//
// STATED RESIDUAL: when the catch-up merge (mergeFn against the current
// source tip) itself RE-CONFLICTS, this returns that failure as-is — C1
// stays committed, no collapse is attempted over an uncommitted state.
// The next --resolve-merge resumption then resolves the re-conflict as
// an ordinary bindingExact completion against C1 (expectedSource's tip
// has not moved again in the meantime), producing a second, separate
// merge commit — the two-merge topology this fix removes from the
// single-invocation clean-catch-up path can still arise across two
// SEPARATE invocations when the catch-up itself conflicts. Collapsing
// that multi-invocation chain too is out of this fix's scope.
func completeDriftedResumedMerge(workdir string, mergeFn func() error, conflictFailure func(mergeErr error) error) error {
	preTip, err := gitutil.RevParseRef(workdir, "HEAD")
	if err != nil {
		return fmt.Errorf("resolving %s's pre-resumption tip: %w", workdir, err)
	}
	if err := completeResumedMerge(workdir); err != nil {
		return err
	}
	subject, err := gitutil.CommitMessageBody(workdir, "HEAD")
	if err != nil {
		return fmt.Errorf("reading the completed merge's own message in %s: %w", workdir, err)
	}
	if err := attemptFreshMerge(workdir, mergeFn, conflictFailure); err != nil {
		// The catch-up merge did not land cleanly (a genuine re-conflict,
		// or a worktree-state failure) — see the STATED RESIDUAL above.
		return err
	}
	tree, err := gitutil.TreeSHA(workdir, "HEAD")
	if err != nil {
		return fmt.Errorf("resolving %s's post-catch-up tree: %w", workdir, err)
	}
	driftedTip, err := gitutil.RevParseRef(workdir, "HEAD^2")
	if err != nil {
		return fmt.Errorf("resolving the catch-up merge's second parent in %s: %w", workdir, err)
	}
	collapsed, err := gitutil.CommitTreeMerge(workdir, tree, preTip, driftedTip, subject)
	if err != nil {
		return fmt.Errorf("collapsing the drift catch-up into a single merge commit in %s: %w", workdir, err)
	}
	if err := gitutil.ResetSoft(workdir, collapsed); err != nil {
		return fmt.Errorf("moving %s onto the collapsed merge commit: %w", workdir, err)
	}
	return nil
}

// resumeAwareMerge is the R5(d) merge dispatch every producer's own
// merge site calls INSTEAD of calling gitutil.MergeInto/MergeBranch
// directly. It subsumes the preserved-merge precondition (checking
// mergeResumptionStep covers the same ground checkNoPreservedMerge
// would) with resumption awareness layered on top:
//
//   - a merge already in progress with conflicted index entries →
//     re-print the resolution steps and refuse; never a new merge,
//     never a checkout, never an abort — regardless of resolveMerge.
//   - a merge already in progress, index RESOLVED and staged →
//     resolveMerge==true completes it (`git commit --no-edit`,
//     preserving the original subject) and returns nil (the caller
//     proceeds to its normal post-merge cleanup exactly as if mergeFn()
//     itself had returned nil); resolveMerge==false still refuses,
//     naming --resolve-merge as the way to finish it.
//   - no merge in progress → calls mergeFn() (the caller's real
//     MergeInto/MergeBranch attempt) fresh, over the CURRENT operand
//     tips. On conflict, the conflict is left IN PLACE (never aborted)
//     — conflictFailure renders the caller's own rich failure message
//     naming what is preserved; this function never calls
//     gitutil.AbortMerge.
//
// expectedSource is the branch this invocation was asked to merge (the
// caller's own operand — beadBranch, e.Branch, or specBranch at the
// three call sites). Spec 127 bead-6 fix round 1 (G1-1): BEFORE either
// resumption leg above acts on a preserved MERGE_HEAD, this function
// classifies it against expectedSource's CURRENT tip
// (classifyPreservedMergeBinding) — a preserved merge is no longer
// assumed to be "the product-initiated merge for this request" merely by
// existing.
//
//   - bindingExact: unchanged behavior (the cases above).
//   - bindingForeign: refuse in BOTH resumption legs, naming the
//     mismatch and (best-effort) the foreign merge's own seeded subject
//     — never re-printed as this request's own resolution steps, never
//     completed.
//   - bindingDrifted (still-conflicted leg): not yet actionable — no
//     commit has happened, so there is nothing to catch up on yet; the
//     operator finishes resolving what is already staged, and drift is
//     caught at completion, below.
//   - bindingDrifted (ready-to-complete leg): completeDriftedResumedMerge
//     finalizes the PRESERVED (now-stale) resolution first — the
//     operator's staged work is never discarded — then re-invokes
//     mergeFn() against expectedSource's CURRENT tip so the drift is
//     incorporated before this call reports success, closing the same
//     gap a resumed merge would otherwise silently leave open (the
//     completed commit's second parent would stay the OLD MERGE_HEAD,
//     omitting every commit authored on the source after the conflict
//     began) — and then COLLAPSES the two commits this produces into a
//     single merge commit (bead-6 fix round 2: two first-parent-adjacent
//     merge commits under the SAME seeded subject with different second
//     parents is indistinguishable, from internal/lifecycle.FindLandedMerge's
//     vantage point, from a genuine ownership ambiguity between two
//     independent re-merges — spec 125 FIX-2b deliberately refuses that
//     shape, so it cannot be told apart downstream; the fix is to never
//     produce it). See completeDriftedResumedMerge's doc comment for the
//     collapse mechanics and its one stated residual (a catch-up that
//     itself re-conflicts is not collapsed in this same call).
func resumeAwareMerge(workdir string, resolveMerge bool, expectedSource, reentryHint string, mergeFn func() error, conflictFailure func(mergeErr error) error) error {
	switch mergeResumptionStep(workdir) {
	case resumeStillConflicted:
		class, preservedTip, sourceTip, err := classifyPreservedMergeBinding(workdir, expectedSource)
		if err != nil {
			return bindingIndeterminateRefusal(workdir, expectedSource, err, reentryHint)
		}
		if class == bindingForeign {
			return foreignMergeRefusal(workdir, expectedSource, preservedTip, sourceTip, reentryHint)
		}
		return stillConflictedRefusal(workdir, reentryHint)
	case resumeReadyToComplete:
		if !resolveMerge {
			return guard.NewFailure(
				fmt.Sprintf("a resolved merge is already staged in %s but was not completed.", workdir),
				fmt.Sprintf("re-run with %s to finish it: %s", ResolveMergeFlag, reentryHint),
			)
		}
		class, preservedTip, sourceTip, err := classifyPreservedMergeBinding(workdir, expectedSource)
		if err != nil {
			return bindingIndeterminateRefusal(workdir, expectedSource, err, reentryHint)
		}
		if class == bindingForeign {
			return foreignMergeRefusal(workdir, expectedSource, preservedTip, sourceTip, reentryHint)
		}
		if class == bindingDrifted {
			// expectedSource has moved since the conflict began — complete
			// the stale resolution, incorporate the current tip, and
			// collapse both into ONE merge commit (see
			// completeDriftedResumedMerge's doc comment).
			return completeDriftedResumedMerge(workdir, mergeFn, conflictFailure)
		}
		return completeResumedMerge(workdir)
	}
	// resumeNoMergeInProgress: attempt a fresh merge.
	return attemptFreshMerge(workdir, mergeFn, conflictFailure)
}

// attemptFreshMerge runs mergeFn() (the caller's real
// MergeInto/MergeBranch attempt) once, over whatever operand tips it
// closes over at call time. On conflict, the conflict is left IN PLACE
// (never aborted) — conflictFailure renders the caller's own rich
// failure message. Factored out of resumeAwareMerge's bottom leg so the
// bindingDrifted catch-up (above) can invoke the identical fresh-merge-
// plus-conflict-handling logic after completing a stale resumed merge,
// not a copy of it.
func attemptFreshMerge(workdir string, mergeFn func() error, conflictFailure func(mergeErr error) error) error {
	if mergeErr := mergeFn(); mergeErr != nil {
		// R5(d)(vi)/E-r5-4: a merge failure that produced NO merge state
		// (no MERGE_HEAD — e.g. a dirty index/worktree blocking the
		// checkout or merge start) is not a conflict at all; diagnosing
		// it as one would print conflict-resolution steps over a
		// worktree that was never mid-merge, a non-terminating loop.
		// Distinguish by re-checking merge state AFTER the failure.
		if !gitutil.MergeInProgress(workdir) {
			return worktreeStateRefusal(workdir, blockingPaths(workdir), mergeErr)
		}
		return conflictFailure(mergeErr)
	}
	return nil
}

// blockingPaths reports the dirty (uncommitted-change) paths in workdir
// for the R5(d)(vi) worktree-state refusal — the porcelain status lines,
// which name what needs to be committed or discarded before a merge can
// even start (distinct from gitutil.ConflictedFiles, which is empty here
// by construction: no MERGE_HEAD exists in this leg).
func blockingPaths(workdir string) []string {
	out, err := gitutil.Status(workdir)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}
