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

// treeSHAFn/commitMessageBodyFn/commitTreeMergeFn/resetSoftFn are
// package-level indirections over their gitutil counterparts (spec 127
// bead-6 fix round 3): the drift-collapse mechanics
// (completeDriftedResumedMerge/repairStrandedDriftCollapse) call these,
// not gitutil directly, so a fault-injection test can force any ONE step
// to fail — e.g. a successful commit-tree followed by a ResetSoft
// failure (S1-1/G1-3/O1-1/F1-1's real-git reproduction) — without
// needing to break real git itself (ref-lock contention, disk errors,
// etc. are not portably reproducible in a test).
var (
	treeSHAFn           = gitutil.TreeSHA
	commitMessageBodyFn = gitutil.CommitMessageBody
	commitTreeMergeFn   = gitutil.CommitTreeMerge
	resetSoftFn         = gitutil.ResetSoft
)

// classifySourceTipFn/classifyIsAncestorFn are package-level indirections
// over their gitutil counterparts (spec 127 bead-6 fix round 4, item 3):
// classifyPreservedMergeBinding's own two git-I/O calls, seamed for the
// same reason as the quartet above — a genuine, non-"ref not found" git
// failure at either call (a corrupted object, ref-lock contention) is not
// portably reproducible in a test, so a fault-injection test forces
// EITHER to fail and confirms the result is the fail-closed
// bindingIndeterminateRefusal leg — never silently treated as any
// confident classification.
var (
	classifySourceTipFn  = gitutil.RevParseRef
	classifyIsAncestorFn = gitutil.IsAncestor
)

// danglingCollapsedMergeExistsFn is a package-level indirection over
// gitutil.DanglingCollapsedMergeExists (spec 127 bead-6 fix round 4): the
// same fault-injection rationale as the quartet above — a test can force
// the "was this genuinely stranded, or does it only LOOK stranded"
// dangling-object proof to answer either way without needing to
// orchestrate real interrupted/non-interrupted git sequences for every
// case detectStrandedDriftTopology's caller needs to distinguish.
var danglingCollapsedMergeExistsFn = gitutil.DanglingCollapsedMergeExists

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
// still recognize). Round 2 anchored this on the subject git itself
// seeded at merge start (gitutil.MergeMsgSubject).
//
// Bead-6 fix round 3 (G1's confirm-round finding on round 2): MERGE_MSG
// is a plain file git itself invites an operator to hand-edit before
// finishing a merge — matching its subject is a FRESH forgeable proxy for
// identity, not durable evidence. An operator who rewrites MERGE_MSG's
// first line to "Merge <expectedSource>" over a preserved FOREIGN merge
// gets it classified bindingDrifted and completed. The subject check
// stays as a cheap, necessary first filter (a subject that doesn't even
// CLAIM expectedSource is confidently bindingForeign, unchanged from
// round 2), but a subject that DOES claim expectedSource must now also be
// corroborated by mergeSourceMarkerMatches — a ref MergeInto/MergeBranch
// themselves wrote, at (or very near — see recordMergeSourceMarker's own
// doc comment) the moment THIS TOOL'S OWN merge attempt of expectedSource
// began (gitutil.MergeSourceMarkerRef), never something the ORDINARY
// conflict-resolution workflow touches.
//
// Bead-6 fix round 5 (G1's confirm-round finding on round 3): this marker
// is producer-written evidence, not unforgeable evidence — see
// MergeSourceMarkerRef's own doc comment (internal/gitutil/gitops.go) for
// the full statement of what it catches (a hand-edited MERGE_MSG, a stale
// marker from an earlier run, another bead's preserved conflict sharing a
// worktree) versus what it is explicitly out of scope against (an
// operator deliberately running `git update-ref` against
// refs/mindspec/ — no marker living inside the repository can defend
// against that actor, who could equally rewrite any other part of the
// repository's history). A marker that cannot be read at all (absent, or
// a genuine git failure) is NOT evidence either way — it fails closed as
// indeterminate, never as a confident classification in either
// direction.
func classifyPreservedMergeBinding(workdir, expectedSource string) (class bindingClass, preservedTip, sourceTip string, err error) {
	preservedTip, err = gitutil.RevParseRef(workdir, "MERGE_HEAD")
	if err != nil {
		return 0, "", "", fmt.Errorf("resolving the preserved MERGE_HEAD in %s: %w", workdir, err)
	}
	sourceTip, err = classifySourceTipFn(workdir, expectedSource)
	if err != nil {
		return 0, preservedTip, "", fmt.Errorf("resolving the current tip of %s: %w", expectedSource, err)
	}
	if preservedTip == sourceTip {
		return bindingExact, preservedTip, sourceTip, nil
	}
	isAnc, ancErr := classifyIsAncestorFn(workdir, preservedTip, sourceTip)
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
	// Round 3: the subject CLAIMS expectedSource — but the subject is
	// operator-editable, so it is corroborated (never simply trusted)
	// against the merge-start marker (producer-written evidence, not
	// unforgeable — see MergeSourceMarkerRef's doc comment) before this
	// preserved merge is completed as expectedSource's own drifted
	// history.
	matched, markErr := mergeSourceMarkerMatches(workdir, expectedSource, preservedTip)
	if markErr != nil {
		return 0, preservedTip, sourceTip, fmt.Errorf("could not confirm the preserved merge in %s is %s's own recorded merge attempt (%w)", workdir, expectedSource, markErr)
	}
	if !matched {
		return bindingForeign, preservedTip, sourceTip, nil
	}
	return bindingDrifted, preservedTip, sourceTip, nil
}

// mergeSubjectNamesSource reports whether workdir's seeded MERGE_MSG
// subject names expectedSource as the branch being merged — the
// MergeInto ("Merge <source>") or MergeBranch ("Merge <source> into
// <target>") shape gitutil seeds at merge start (gitops.go's own `-m`
// calls). Matching is anchored on "Merge <expectedSource>" as either the
// WHOLE subject (MergeInto's form) or a prefix immediately followed by
// " into " (MergeBranch's form) — never a bare substring — so a branch
// name that is a textual prefix of another branch's name (e.g. "bead/x"
// vs "bead/x-extra") cannot be confused for a match.
//
// Bead-6 fix round 3: this is now a NECESSARY but no longer SUFFICIENT
// signal — MERGE_MSG is hand-editable, so a subject naming expectedSource
// no longer alone justifies bindingDrifted (see
// classifyPreservedMergeBinding's doc comment and mergeSourceMarkerMatches
// below). A subject that does NOT name expectedSource remains a
// confident, unchanged bindingForeign signal — hand-editing MERGE_MSG to
// falsely DISCLAIM identity would only ever make this function MORE
// conservative, never less.
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

// mergeSourceMarkerMatches reports whether workdir holds the merge-start
// marker MergeInto/MergeBranch record for expectedSource
// (gitutil.MergeSourceMarkerRef, spec 127 bead-6 fix round 3 — producer-
// written evidence, not unforgeable; see that function's own doc comment
// for what it does and does not defend against), and whether its
// recorded value equals preservedTip — the corroboration
// classifyPreservedMergeBinding requires before trusting an
// ancestor-but-not-equal preserved merge as expectedSource's own drifted
// history. These are two DIFFERENT outcomes, not one:
//
//   - ABSENT (gitutil.ErrRefNotFound — e.g. a merge started by a build
//     predating this fix, or a hand-run `git merge` outside this tool
//     that never wrote a marker at all): this is NOT evidence of anything
//     either way — the caller CANNOT TELL — so it is returned as an
//     error and fails closed via bindingIndeterminateRefusal, never
//     silently trusted (permission) and never silently rejected (a
//     confident, possibly wrong, foreign verdict).
//   - PRESENT but MISMATCHED (recorded != preservedTip, err == nil): this
//     IS evidence, and it is CONTRADICTING — this tool itself recorded
//     starting a merge attempt of expectedSource from a specific tip, and
//     the currently preserved MERGE_HEAD is a DIFFERENT tip (bead-6 fix
//     round 4, item 3's central case: a stale marker from an earlier
//     tool-driven attempt, left behind by a LATER, hand-run `git merge`
//     that bypassed the tool and never updated it). This returns
//     (false, nil) — no error — so the caller takes the CONFIDENT
//     bindingForeign leg (foreignMergeRefusal), never
//     bindingIndeterminateRefusal's "cannot tell" phrasing.
//     TestClassifyPreservedMergeBinding_MismatchedMarkerIsConfidentForeignRefusal
//     (merge_resumption_classification_test.go) constructs this exact
//     shape and pins the two refusals never share a code path.
func mergeSourceMarkerMatches(workdir, expectedSource, preservedTip string) (bool, error) {
	recorded, err := gitutil.RevParseRef(workdir, gitutil.MergeSourceMarkerRef(expectedSource))
	if err != nil {
		return false, err
	}
	return recorded == preservedTip, nil
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
// STATED RESIDUALS (bead-6 fix round 3: the FIRST of these two was the
// only one this comment disclosed before round 3 — S1-1/G1-3/O1-1/F1-1's
// panel review found a SECOND, undisclosed way the same bad shape could
// land, and this fix round closes it rather than merely disclosing it;
// see below):
//
//  1. When the catch-up merge (mergeFn against the current source tip)
//     itself RE-CONFLICTS, this returns that failure as-is — C1 stays
//     committed, no collapse is attempted over an uncommitted state. The
//     next --resolve-merge resumption then resolves the re-conflict as an
//     ordinary bindingExact completion against C1 (expectedSource's tip
//     has not moved again in the meantime), producing a second, separate
//     merge commit — the two-merge topology this fix removes from the
//     single-invocation clean-catch-up path can still arise across two
//     SEPARATE invocations when the catch-up itself conflicts.
//
//     Bead-6 fix round 4 (G-question confirm-round): round 3 claimed
//     "resumeAwareMerge's stranded-topology repair below cannot even run
//     against it" — true only of the invocation immediately adjacent to
//     the re-conflict (mergeResumptionStep sees a merge IN PROGRESS there
//     — resumeStillConflicted/resumeReadyToComplete — never
//     resumeNoMergeInProgress). It did NOT follow the chain to the
//     invocation AFTER that one completes the re-conflict (an ordinary
//     bindingExact completion, producing C2 via a plain `git commit
//     --no-edit` — never touching commitTreeMergeFn/resetSoftFn at all).
//     Once C2 lands, MERGE_HEAD is gone: a LATER invocation for this same
//     bead reaches resumeNoMergeInProgress whenever expectedSource SURVIVES
//     that completion — which happens whenever either (a) ensureLandedBinding
//     itself failed (ADR-0041 §2(ii)'s disclosed "branch survives, re-run to
//     converge" recovery), or (b) ensureLandedBinding succeeded but the
//     best-effort branch/worktree cleanup that follows it did not (ADR-0035
//     recoverable posture: gitutil.DeleteBranch's own failure is printed as
//     a WARNING, never returned — see CompleteBead's cleanup closure). A
//     completion where BOTH the binding AND the cleanup succeed instead
//     deletes expectedSource in that SAME invocation, and
//     preflightMergeDestruction fail-closed-refuses on a since-deleted
//     branch before ever reaching resumeAwareMerge again (merge_preflight.go)
//     — so only these two sub-cases are reachable, not every re-invocation.
//     A real-git reproduction (merge_resumption_multiinvocation_test.go)
//     confirms the shape itself: C1+C2 land as two genuinely separate,
//     successful commits, no commit-tree object with the collapsed shape
//     ever created, and a subsequent invocation still fires
//     repairStrandedDriftCollapse below.
//
//     This is NOT treated as a bug to suppress: C1 and C2 share the
//     IDENTICAL seeded subject naming expectedSource with DIFFERENT second
//     parents — exactly the shape spec 125's own FindLandedMerge refuses
//     as genuine ownership ambiguity (FIX-2b), left uncollapsed forever.
//     Collapsing it is correct and desired IN THE COMMON CASE (the
//     ensureLandedBinding-FAILURE retry: a binding is only ever recorded
//     AFTER a successful resumeAwareMerge return, so nothing durable yet
//     names C2 when the collapse discards it) — but NOT in every case a
//     bare re-invocation can reach this exact structural shape. If C2's own
//     completion instead succeeds (binding recorded, pointing at C2 by
//     SHA) and ONLY the subsequent best-effort branch/worktree cleanup
//     fails (gitutil.DeleteBranch's own failure is a printed WARNING, not
//     a returned error — see CompleteBead's cleanup closure), expectedSource
//     survives with a DURABLE binding already naming C2. A later
//     re-invocation reaches this SAME structural shape (C1, C2 still
//     adjacent, same subject) with no way, from shape alone, to tell it
//     apart from the ensureLandedBinding-failure case — collapsing here
//     would orphan the very commit the binding names, turning a clean
//     no-op retry into a silently-broken binding. detectStrandedDriftTopology's
//     dangling-object proof (round 4, see below) is what actually
//     distinguishes these: NEITHER sub-case of this residual ever calls
//     commitTreeMergeFn, so the proof correctly reports "not stranded" for
//     BOTH — collapsing this shape only when there is affirmative
//     producer-written evidence of a genuinely interrupted collapse (never
//     unforgeable — round 5's own confirm-round finding; see
//     gitutil.DanglingCollapsedMergeExists' doc comment) means the FIX-2b
//     ambiguity for this residual stays exactly as disclosed (uncollapsed,
//     out of this fix's scope), rather than being "fixed" in a way that is
//     only safe in the more common of its two reachable sub-cases.
//
//  2. After the catch-up lands CLEANLY (C2 exists, MERGE_HEAD is gone),
//     four more calls run before the branch moves onto the collapsed
//     commit: TreeSHA, the second-parent read, CommitTreeMerge, and
//     ResetSoft. If ResetSoft fails after CommitTreeMerge already
//     succeeded (a ref-lock or other transient failure), the branch is
//     left sitting at the un-collapsed C1+C2 topology with MERGE_HEAD
//     already cleared. resumeAwareMerge's dispatch calls
//     repairStrandedDriftCollapse BEFORE ever reaching attemptFreshMerge on
//     a bare re-invocation's resumeNoMergeInProgress leg — never letting
//     that leg's mergeFn() silently no-op ("already up to date") over the
//     stranded shape. This sub-residual has three reachable outcomes, not
//     one:
//
//       (a) CLOSED, round 3 — a BARE re-invocation, nothing else having
//           touched the branch: detectStrandedDriftTopology finds the
//           stranded shape (two adjacent, exactly-two-parent merge
//           commits at HEAD/HEAD^1 sharing one subject naming
//           expectedSource) corroborated by the interrupted
//           CommitTreeMerge's own dangling output object still being
//           present, and repairStrandedDriftCollapse re-attempts the
//           identical collapse. If the repair's own ResetSoft fails
//           again, THAT is returned as a loud, distinct error
//           (strandedCollapseError) — never silently retried as a fresh
//           merge.
//
//       (b) CLOSED, round 5 (G1/O1/S1's confirm-round reproduction: a
//           single intervening commit — another bead's own merge, any
//           ordinary auto-commit — landing on the branch between the
//           interruption and a retry buries the stranded pair one commit
//           below the new tip): round 4's own dangling-object proof still
//           exists and can still be found (detectStrandedDriftTopology
//           now resolves the candidate pair's OWN tree, not literally
//           "HEAD", so it is unaffected by what has since landed on top),
//           but repairStrandedDriftCollapse must NOT run `git reset
//           --soft` here — that would silently discard whatever landed
//           on top. Doing nothing and falling through to attemptFreshMerge
//           is equally wrong (S1-3/O1-3/G1-3/F1-1's shared finding: the
//           fresh merge then genuinely no-ops "already up to date",
//           reporting apparent success over the still-uncollapsed,
//           ambiguous topology — "the silence is the defect, not the
//           missed repair"). detectStrandedDriftTopology now returns a
//           distinguishable, loud *strandedTopologyIndeterminateError in
//           this exact case instead: proven stranded, but not safe to
//           auto-repair, so refuse and name what a human needs to look at
//           rather than claim success or silently rewrite history.
//
//       (c) DISCLOSED, NOT closed (S3-3/F1-1's confirm-round finding): if
//           an external `git gc --prune=now` (or an aggressive
//           gc.pruneExpire) runs in the narrow window between the
//           interruption and a retry, AND nothing has landed on top of
//           the stranded pair, the dangling proof object is genuinely
//           gone — fsck can no longer find what no longer exists. This is
//           INDISTINGUISHABLE, by any in-repository signal, from
//           STATED RESIDUAL 1's own legitimately-produced multi-invocation
//           chain (which never created that object either) — the same
//           reasoning MergeSourceMarkerRef's own doc comment gives for why
//           bead 6 stopped chasing an "unforgeable" replacement applies
//           here too: nothing that lives inside the repository can survive
//           an operator or CI job that can also run `git gc`. This
//           function therefore proceeds exactly as it does for the
//           legitimate chain (no collapse, no error — a no-op fresh merge
//           reports success), an ACCEPTED residual, not a false
//           attestation: the un-collapsed, ambiguous topology remains
//           exactly what it is in git history, and any LATER attempt to
//           resolve "the landed merge" for this bead — spec 125's own
//           FindLandedMerge — still fails closed and refuses to pick a
//           side, exactly as it does for residual 1. No caller of this
//           function, at any point, is ever told a false landed-merge
//           identity.
func completeDriftedResumedMerge(workdir string, mergeFn func() error, conflictFailure func(mergeErr error) error) error {
	preTip, err := gitutil.RevParseRef(workdir, "HEAD")
	if err != nil {
		return fmt.Errorf("resolving %s's pre-resumption tip: %w", workdir, err)
	}
	if err := completeResumedMerge(workdir); err != nil {
		return err
	}
	subject, err := commitMessageBodyFn(workdir, "HEAD")
	if err != nil {
		return fmt.Errorf("reading the completed merge's own message in %s: %w", workdir, err)
	}
	if err := attemptFreshMerge(workdir, mergeFn, conflictFailure); err != nil {
		// The catch-up merge did not land cleanly (a genuine re-conflict,
		// or a worktree-state failure) — STATED RESIDUAL 1 above.
		return err
	}
	tree, err := treeSHAFn(workdir, "HEAD")
	if err != nil {
		return fmt.Errorf("resolving %s's post-catch-up tree: %w", workdir, err)
	}
	driftedTip, err := gitutil.RevParseRef(workdir, "HEAD^2")
	if err != nil {
		return fmt.Errorf("resolving the catch-up merge's second parent in %s: %w", workdir, err)
	}
	collapsed, err := commitTreeMergeFn(workdir, tree, preTip, driftedTip, subject)
	if err != nil {
		return fmt.Errorf("collapsing the drift catch-up into a single merge commit in %s: %w", workdir, err)
	}
	if err := resetSoftFn(workdir, collapsed); err != nil {
		// Leaves the un-collapsed C1+C2 topology at HEAD, MERGE_HEAD
		// already cleared — STATED RESIDUAL 2 above. A LATER invocation's
		// repairStrandedDriftCollapse detects and re-attempts this exact
		// collapse before it can be silently laundered as a no-op success.
		return fmt.Errorf("moving %s onto the collapsed merge commit: %w", workdir, err)
	}
	return nil
}

// strandedCollapseError is bead-6 fix round 3's loud, distinct failure
// when repairStrandedDriftCollapse finds a stranded two-merge drift
// topology (an earlier, interrupted completeDriftedResumedMerge's
// ResetSoft failed after its CommitTreeMerge already succeeded) but its
// OWN repair attempt also fails — never silently retried as a fresh
// merge, and never reported as success.
type strandedCollapseError struct{ msg string }

func (e *strandedCollapseError) Error() string { return e.msg }

// strandedTopologyIndeterminateError is bead-6 fix round 5's loud,
// distinguishable failure for the OTHER way detectStrandedDriftTopology
// can now conclude it must not silently proceed (see this function's own
// doc comment, sub-case (b)): the dangling-object proof PROVES a genuine
// interruption happened, but at least one further commit has landed on
// top of the stranded pair since — collapsing via `git reset --soft` now
// would silently discard whatever that is, and reporting "not stranded"
// (the pre-round-5 behavior) would let attemptFreshMerge's mergeFn()
// silently no-op over the still-uncollapsed, ambiguous topology. Neither
// is acceptable, so this surfaces loudly instead: nothing is touched, and
// a human is told exactly what to look at.
type strandedTopologyIndeterminateError struct{ msg string }

func (e *strandedTopologyIndeterminateError) Error() string { return e.msg }

// strandedTopologyIndeterminateRefusal renders
// strandedTopologyIndeterminateError's message: names the stranded pair's
// own (no-longer-current) tip, the branch's actual current tip, and what
// an operator needs to do — never an automatic rewrite.
func strandedTopologyIndeterminateRefusal(workdir, expectedSource, strandedTip, currentHead string) *strandedTopologyIndeterminateError {
	return &strandedTopologyIndeterminateError{msg: fmt.Sprintf(
		"%s holds a two-merge drift-collapse topology for %s from an earlier, interrupted run that this tool can positively confirm (a dangling commit-tree object with the exact would-be-collapsed shape survives at %s) — but at least one further commit has landed on top of it since (the branch's current tip is %s, not %s), so collapsing it automatically now would silently discard everything committed since.\nnothing here was touched: this needs manual inspection (`git log --first-parent %s`) before any collapse or rebase, since it must preserve what has landed on top.",
		workdir, termsafe.Escape(expectedSource), strandedTip, currentHead, strandedTip, termsafe.Escape(expectedSource),
	)}
}

// detectStrandedDriftTopology reports whether workdir's branch history
// holds the exact stranded shape completeDriftedResumedMerge's ResetSoft
// failure can leave behind: two commits that are BOTH plain (exactly
// two-parent) merge commits, immediately adjacent on the first-parent
// chain reachable from HEAD (the newer one's first parent IS the older
// one, not merely an ancestor of it — no unrelated history sits between
// them), sharing the IDENTICAL subject, and that subject names
// expectedSource (mergeSubjectNamesSource's own MergeInto/MergeBranch form
// check, applied here to a commit's own preserved subject rather than a
// live MERGE_MSG). When true, it returns exactly what the original
// collapse needed: preTip (the OLDER commit's own first parent — the
// pre-resumption tip, never itself a parent of the eventual collapsed
// result), driftedTip (the NEWER commit's second parent — the drift that
// was caught up), and the shared subject to preserve.
//
// Bead-6 fix round 4 (confirm-round question on round 3): this structural
// shape ALONE is necessary but not sufficient — a real-git reproduction
// (merge_resumption_multiinvocation_test.go) proves that STATED RESIDUAL 1
// in completeDriftedResumedMerge's doc comment (a drift catch-up that
// itself re-conflicts, resolved by a LATER, separate invocation) produces
// this EXACT shape via two genuinely separate, successful `git commit
// --no-edit` calls — never a stranding, never an interrupted collapse.
// Collapsing THAT case is desired too (it shares expectedSource's own
// subject with a different second parent, exactly the ambiguity spec 125's
// FindLandedMerge refuses — see completeDriftedResumedMerge's doc comment),
// but it must be told apart from a topology this function has already
// mistaken for stranded and would rewrite regardless — so a positive
// verdict additionally requires danglingCollapsedMergeExistsFn to prove a
// commit object with the EXACT would-be-collapsed shape (this tree, these
// two parents, this message — round 5 closed a false-positive gap here,
// see gitutil.DanglingCollapsedMergeExists' own doc comment) already
// exists, unreferenced by any ref. Nothing but a
// completeDriftedResumedMerge/repairStrandedDriftCollapse CommitTreeMerge
// call that never reached its own ResetSoft ever produces such an object.
//
// Bead-6 fix round 5 (S1-3/O1-3/G1-3/F1-1's confirm-round finding): TWO
// further, previously-conflated cases now get their own outcome instead
// of both silently falling through this function's earlier, single "not
// stranded" leg:
//
//   - The candidate pair's own tree is now read from the NEWER commit
//     itself (not the literal string "HEAD"): if one or more further
//     commits have landed on top of the stranded pair since the
//     interruption, HEAD's own tree would otherwise no longer match what
//     the dangling proof object was built from, producing a false
//     "not stranded" purely from reading the wrong tree — a bug, not a
//     disclosed limit, now fixed.
//   - Whether that further-landed state exists at all is now itself part
//     of the verdict: if the dangling proof DOES corroborate a genuine
//     interruption but the candidate pair is no longer the branch's own
//     tip, this returns an error (strandedTopologyIndeterminateError) —
//     proven stranded, but not safe for this function's caller to
//     auto-repair (see repairStrandedDriftCollapse's own doc comment).
//
// Absent the dangling proof AND nothing having landed on top, this still
// reports NOT stranded, unchanged from round 4: the caller falls through
// to a fresh merge attempt, which safely no-ops ("already up to date")
// over an already-fully-landed multi-invocation chain rather than
// rewriting it. See completeDriftedResumedMerge's own doc comment,
// STATED RESIDUAL 2(c), for the one remaining disclosed gap this
// still-absent-proof leg cannot close (an external prune erasing the
// proof for a GENUINE interruption that nothing has yet landed on top
// of) — indistinguishable from the legitimate chain by any signal this
// function, or any in-repository marker, can produce.
func detectStrandedDriftTopology(workdir, expectedSource string) (stranded bool, preTip, driftedTip, subject string, err error) {
	head, herr := gitutil.RevParseRef(workdir, "HEAD")
	if herr != nil {
		return false, "", "", "", fmt.Errorf("resolving %s's current tip while scanning for a stranded drift-collapse topology: %w", workdir, herr)
	}
	merges, ferr := gitutil.FirstParentMerges(workdir, "HEAD")
	if ferr != nil {
		return false, "", "", "", fmt.Errorf("scanning %s for a stranded drift-collapse topology: %w", workdir, ferr)
	}
	if len(merges) < 2 {
		return false, "", "", "", nil
	}
	top, prior := merges[0], merges[1]
	if len(top.Parents) != 2 || len(prior.Parents) != 2 {
		return false, "", "", "", nil
	}
	if top.Parents[0] != prior.SHA {
		// Not adjacent — some other, unrelated history sits between them;
		// not the shape this collapse's own interruption produces.
		return false, "", "", "", nil
	}
	if top.Subject != prior.Subject {
		return false, "", "", "", nil
	}
	want := "Merge " + expectedSource
	if top.Subject != want && !strings.HasPrefix(top.Subject, want+" into ") {
		return false, "", "", "", nil
	}
	wouldBePreTip, wouldBeDriftedTip := prior.Parents[0], top.Parents[1]
	// Round 5: resolve the candidate pair's OWN tree (top.SHA), never the
	// literal string "HEAD" — HEAD may since have moved past top if
	// something else has landed on top of the stranded pair.
	tree, terr := treeSHAFn(workdir, top.SHA)
	if terr != nil {
		return false, "", "", "", fmt.Errorf("resolving %s's candidate stranded-pair tree while confirming a stranded drift-collapse topology: %w", workdir, terr)
	}
	// Round 5 (G1-3's confirm-round finding, the false-positive half):
	// gitutil.DanglingCollapsedMergeExists now also checks the dangling
	// object's own full message — but the message a genuine
	// completeDriftedResumedMerge run would have committed the collapsed
	// object under is prior's OWN FULL raw commit message
	// (commitMessageBodyFn's %B, read there via `commitMessageBodyFn(workdir,
	// "HEAD")` right after completeResumedMerge committed prior/C1 — see
	// that function's own body). That can carry more than prior.Subject's
	// bare first line (e.g. a `# Conflicts:` comment block `git commit
	// --no-edit` leaves in a merge commit's message body by default) — so
	// the identity check here, and the message this function hands back
	// to repairStrandedDriftCollapse for the ACTUAL repair, must use
	// prior's full message, never the truncated %s subject line.
	wouldBeMessage, merr := commitMessageBodyFn(workdir, prior.SHA)
	if merr != nil {
		return false, "", "", "", fmt.Errorf("reading %s's own preserved message while confirming a stranded drift-collapse topology: %w", workdir, merr)
	}
	proven, derr := danglingCollapsedMergeExistsFn(workdir, tree, wouldBePreTip, wouldBeDriftedTip, wouldBeMessage)
	if derr != nil {
		return false, "", "", "", fmt.Errorf("confirming a stranded drift-collapse topology in %s: %w", workdir, derr)
	}
	if !proven {
		// The structural shape matches, but no dangling commit-tree object
		// corroborates a genuine interruption — this is (or may be) the
		// legitimately-produced multi-invocation chain, never rewritten on
		// shape alone. STATED RESIDUAL 2(c): if this is instead a genuine
		// interruption whose proof was externally pruned, this leg cannot
		// tell — a disclosed, accepted gap (see this function's own doc
		// comment).
		return false, "", "", "", nil
	}
	if top.SHA != head {
		// STATED RESIDUAL 2(b): the dangling proof DOES corroborate a
		// genuine interruption, but top is no longer the branch's own tip
		// — something has landed on it since. Never silently collapse
		// (would discard what landed on top) and never silently report
		// "not stranded" either (the caller's fresh-merge fallback would
		// then genuinely no-op over the still-uncollapsed topology).
		return false, "", "", "", strandedTopologyIndeterminateRefusal(workdir, expectedSource, top.SHA, head)
	}
	return true, wouldBePreTip, wouldBeDriftedTip, wouldBeMessage, nil
}

// repairStrandedDriftCollapse is bead-6 fix round 3's convergence fix for
// completeDriftedResumedMerge's STATED RESIDUAL 2 (see its doc comment):
// called at the top of resumeAwareMerge's resumeNoMergeInProgress leg,
// BEFORE attemptFreshMerge ever runs, so a bare re-invocation over a
// stranded two-merge topology repairs it (making the sequence convergent)
// rather than letting attemptFreshMerge's mergeFn() silently no-op over it
// (making the interruption look like success). Returns nil, untouched,
// when workdir is not in the stranded shape at all — the ordinary case on
// every normal invocation.
//
// Bead-6 fix round 4: detectStrandedDriftTopology's structural shape check
// (adjacency + shared subject) is ALSO the exact shape STATED RESIDUAL 1
// can produce across two separate, fully successful invocations (see
// completeDriftedResumedMerge's doc comment) — this function must never
// collapse that legitimately-produced pair merely because it looks the
// same. detectStrandedDriftTopology's dangling-object corroboration (proof
// that an earlier CommitTreeMerge call for THIS exact tree+parent pair
// already ran and never reached its own ResetSoft) is what keeps this
// function's action confined to genuine interruptions — never a topology
// that only structurally resembles one.
//
// Bead-6 fix round 5: detectStrandedDriftTopology can now also return a
// non-nil error (*strandedTopologyIndeterminateError) rather than a plain
// (false, nil) verdict — proven stranded, but with at least one further
// commit already landed on top of the candidate pair, so running `git
// reset --soft` here would silently discard whatever that is. This
// function's own `if err != nil { return err }` below propagates that
// refusal unchanged: it never attempts a collapse in that case, and never
// treats it as "nothing to repair" either.
func repairStrandedDriftCollapse(workdir, expectedSource string) error {
	stranded, preTip, driftedTip, subject, err := detectStrandedDriftTopology(workdir, expectedSource)
	if err != nil {
		return err
	}
	if !stranded {
		return nil
	}
	tree, err := treeSHAFn(workdir, "HEAD")
	if err != nil {
		return fmt.Errorf("resolving the stranded topology's tree in %s: %w", workdir, err)
	}
	collapsed, err := commitTreeMergeFn(workdir, tree, preTip, driftedTip, subject)
	if err != nil {
		return fmt.Errorf("repairing the stranded two-merge drift topology in %s: %w", workdir, err)
	}
	if err := resetSoftFn(workdir, collapsed); err != nil {
		return &strandedCollapseError{msg: fmt.Sprintf(
			"%s still holds an uncollapsed two-merge drift topology for %s from an earlier, interrupted %s run (the collapse's final step did not complete) — the repair attempt failed: %s\nnothing beyond the repair attempt itself was touched; re-run once the underlying failure clears (e.g. ref-lock contention) to retry.",
			workdir, termsafe.Escape(expectedSource), ResolveMergeFlag, termsafe.Escape(err.Error()),
		)}
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
	// resumeNoMergeInProgress: repair any stranded drift-collapse an
	// earlier, interrupted invocation left behind (bead-6 fix round 3 —
	// see completeDriftedResumedMerge's STATED RESIDUAL 2 and
	// repairStrandedDriftCollapse's doc comment) BEFORE attempting a
	// fresh merge — otherwise a bare re-invocation's mergeFn() would
	// silently no-op ("already up to date") over the un-collapsed
	// topology and report apparent success.
	if err := repairStrandedDriftCollapse(workdir, expectedSource); err != nil {
		return err
	}
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
