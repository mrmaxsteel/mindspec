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
func resumeAwareMerge(workdir string, resolveMerge bool, reentryHint string, mergeFn func() error, conflictFailure func(mergeErr error) error) error {
	switch mergeResumptionStep(workdir) {
	case resumeStillConflicted:
		return stillConflictedRefusal(workdir, reentryHint)
	case resumeReadyToComplete:
		if !resolveMerge {
			return guard.NewFailure(
				fmt.Sprintf("a resolved merge is already staged in %s but was not completed.", workdir),
				fmt.Sprintf("re-run with %s to finish it: %s", ResolveMergeFlag, reentryHint),
			)
		}
		return completeResumedMerge(workdir)
	}
	// resumeNoMergeInProgress: attempt a fresh merge.
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
