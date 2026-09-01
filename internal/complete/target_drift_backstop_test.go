package complete

// Spec 127 bead-6 fix round 1: the MANDATORY target-drift backstop
// fixture plan.md's Steps section 6 names ("P1-4's dissolution,
// demonstrated not asserted") — unanimous across S1, S3, O3, and G1's
// panel review of the first fix round. R4(a)'s own siting argument is
// that the §1-phase preflight (this package's completeWorkDestructionPreflightFn,
// evaluated BEFORE step 2.5's materialization) and the executor's live
// producer-level preflight (mindspec_executor.go, evaluated immediately
// before its own gitutil.MergeInto) independently re-derive the SAME
// predicate — so if the TARGET (specBranch) drifts into a
// destruction-shaped state BETWEEN the two evaluations, the producer's
// own live re-check is the backstop that catches it. That claim had
// never been fixtured at the producer level in either direction before
// this fix round (O1-2 covers the paired branch-side case) — only argued
// in the doc comments across merge_preflight.go/lifecycle's own copy.
//
// This fixture forces the drift via the completeWorkDestructionPreflightFn
// seam itself: the real implementation is called FIRST (a genuine,
// non-vacuous Clean/Ancestor evaluation — the bead branch has not yet
// been merged by anything), and ONLY THEN, as a side effect of that same
// call, the target is advanced to a destruction-shaped state (a squash
// merge of the bead branch's own content landing on specBranch via
// ANOTHER route — gitutil's own wdSupersededFixture recipe) — precisely
// "seam/hook-forced," per the plan's own wording, never asserted by
// fiat.

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// TestRun_TargetDriftBackstop_ProducerRefusesAfterSection1Passes is the
// mandatory target-drift backstop fixture: the producer (executor-level)
// preflight refuses a merge that the §1-phase preflight evaluated
// permissively moments earlier, because the TARGET drifted in between —
// proving the two-layer siting argument empirically, not by inspection.
//
// Bead-6 fix round 2 (G1's confirm-round finding): the plan's own Steps
// §6 wording for this fixture is "the producer refuses, bead N is
// unmerged, PRIOR MERGES AND BINDINGS ARE INTACT, and the re-run
// converges" — fix round 1 demonstrated only the first, third, and
// fourth clauses. This adds a PRIOR bead (M), landed for real via
// ordinary Run() completion BEFORE bead N's drift/refusal sequence
// begins, and asserts M's own landed merge commit — and
// lifecycle.FindLandedMerge's identification outcome for it, whatever
// that outcome is — is BYTE-IDENTICAL before and after bead N's
// producer-level refusal and re-run: nothing about refusing/re-running
// bead N's drifted merge may touch a DIFFERENT bead's already-landed
// history or evidence trail.
//
// SCOPE CAVEAT (bead-6 fix round 3, O3-1/G1-4's panel finding — read
// before trusting the "PRIOR MERGES AND BINDINGS ARE INTACT" label
// below): this fixture proves the git-SHA/content leg of that phrase
// (bead M's landed merge commit stays an ancestor of specBranch, its
// content survives, and lifecycle.FindLandedMerge's own — possibly
// uncorroborated — outcome for it is unchanged). It does NOT exercise the
// merge-time landed-BINDING write/read path (mindspec_executor.go's
// ensureLandedBinding): this test's process cwd is the package checkout,
// not root, and gitutil.BranchExists(beadBranch) at mindspec_executor.go
// (the guard around BOTH the ancestor-safety check and ensureLandedBinding)
// resolves against the CALLING PROCESS's cwd — so that entire guard is
// silently skipped for every bead this fixture completes, prior bead M
// included. That is a pre-existing gap (specs 121/125, not introduced
// here), already filed as a P1 follow-up; this test's own "bindings"
// claim is scoped to what it actually exercises (SHA identity and
// FindLandedMerge's outcome stability), not a literal binding-metadata
// comparison.
func TestRun_TargetDriftBackstop_ProducerRefusesAfterSection1Passes(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "927-tdrift", "mindspec-119tdrift.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	// No bead worktree exists in this fixture (cleanup is irrelevant to
	// the drift assertion): stub worktreeListFn to empty rather than
	// depending on bead.WorktreeList's real `bd worktree list` shelling
	// behavior, which the c2/c3 real-git fixtures in
	// fault_injection_realgit_test.go do identically.
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	specWtPath := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
	ex := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}

	// PRIOR BEAD M: lands for real, via an ordinary (undrifted, un-hooked)
	// Run() completion, BEFORE the drift hook below is even installed —
	// so its own landing is genuinely unaffected by anything that
	// follows.
	const priorBeadID = "mindspec-119tdriftprior.1"
	priorBeadBranch := "bead/" + priorBeadID
	gitRun(t, root, "branch", priorBeadBranch, "main")
	priorWt := filepath.Join(root, ".wt-tdriftprior")
	gitRun(t, root, "worktree", "add", priorWt, priorBeadBranch)
	writeFile(t, priorWt, "internal/widget/prior.go", "package widget\n\nfunc Prior() {}\n")
	// This branch forks from "main", which carries NEITHER widget.go NOR
	// OWNERSHIP.yaml at all (setupRealGitFaultFixture seeds both ONLY on
	// beadBranch, in its own commit) — so this bead's own commit must
	// claim its file the identical way: an OWNERSHIP.yaml alongside the
	// source change, BYTE-IDENTICAL to setupRealGitFaultFixture's own
	// content, so the LATER squash-merge of bead N's branch below (which
	// introduces the SAME path with the SAME content from a DIFFERENT,
	// unrelated history) resolves as a clean add/add rather than a
	// conflict.
	writeFile(t, priorWt, ".mindspec/docs/domains/widget/OWNERSHIP.yaml", "paths:\n  - internal/widget/**\n")
	gitRun(t, priorWt, "add", "-A")
	gitRun(t, priorWt, "commit", "-q", "-m", "impl: prior bead's own unrelated work")
	gitRun(t, root, "worktree", "remove", "--force", priorWt)

	if _, err := Run(root, priorBeadID, specID, "", ex, CompleteOpts{}); err != nil {
		t.Fatalf("fixture invariant broken: the PRIOR bead must land cleanly before bead N's drift sequence begins, got: %v", err)
	}
	priorMergeSHA := gateRevParse(t, root, specBranch)
	if !fileExistsAtRefRealGit(t, root, specBranch, "internal/widget/prior.go") {
		t.Fatal("fixture invariant broken: the prior bead's content must have landed on specBranch")
	}
	priorLandedBefore, priorErrBefore := lifecycle.FindLandedMerge(root, specBranch, priorBeadID)

	origPreflight := completeWorkDestructionPreflightFn
	t.Cleanup(func() { completeWorkDestructionPreflightFn = origPreflight })
	driftApplied := false
	completeWorkDestructionPreflightFn = func(workdir, branch, target, overrideReason, rerun string) error {
		// The REAL §1 evaluation, run FIRST: the bead branch has not yet
		// landed anywhere, so this is a genuine, non-vacuous
		// Clean/Ancestor answer — never stubbed to a canned permissive
		// value.
		err := origPreflight(workdir, branch, target, overrideReason, rerun)
		if err == nil && !driftApplied {
			driftApplied = true
			// Seam/hook-forced target drift (plan.md Steps §6): AFTER
			// §1 evaluates permissively but BEFORE the producer's own
			// live re-check runs, land the bead branch's exact content
			// onto specBranch via ANOTHER route — a squash merge, never
			// a real MergeInto of beadBranch — the gitutil package's own
			// wdSupersededFixture recipe (#218 shape).
			gitRun(t, specWtPath, "merge", "--squash", beadBranch)
			gitRun(t, specWtPath, "commit", "-m", "chore: content lands via another route (simulated concurrent finalize)")
		}
		return err
	}

	specTipBeforeFirstRun := gateRevParse(t, root, specBranch)
	_, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err == nil {
		t.Fatal("the producer's own live preflight must refuse the drifted merge — the §1 evaluation observed a stale (pre-drift) state")
	}
	if !strings.Contains(err.Error(), "--allow-net-deletion") {
		t.Errorf("the producer-level refusal must name --allow-net-deletion, got: %v", err)
	}
	if !driftApplied {
		t.Fatal("fixture invariant broken: the drift hook never fired")
	}

	// Bead N (the drifted bead) stays unmerged: the drift commit above is
	// a squash (single-parent), never a MergeInto merge commit, so
	// beadBranch is not (and must not become) an ancestor of specBranch.
	if isAnc, ancErr := isAncestorRealGit(t, root, beadBranch, specBranch); ancErr != nil || isAnc {
		t.Errorf("bead branch must remain UNMERGED (no MergeInto commit), IsAncestor=%v err=%v", isAnc, ancErr)
	}
	if !branchExistsRealGit(t, root, beadBranch) {
		t.Fatal("the bead branch must survive the producer-level refusal — nothing is cleaned up on a refusal")
	}
	specTipAfterFirstRun := gateRevParse(t, root, specBranch)
	if specTipAfterFirstRun == specTipBeforeFirstRun {
		t.Fatal("fixture invariant broken: the drift commit itself must have landed on specBranch")
	}

	// Re-run: the drift is now PERMANENT (already landed before this
	// invocation even starts), so §1 ALSO observes it fresh and refuses
	// — the SAME class of refusal, and no NEW tool-generated commit
	// lands on specBranch (plan.md's "re-run converges with the §1
	// refusal and no new tool-generated commit").
	_, err2 := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err2 == nil {
		t.Fatal("the re-run must also refuse (the drifted/superseded state persists)")
	}
	if !strings.Contains(err2.Error(), "--allow-net-deletion") {
		t.Errorf("the re-run's refusal must also name --allow-net-deletion, got: %v", err2)
	}
	if got := gateRevParse(t, root, specBranch); got != specTipAfterFirstRun {
		t.Errorf("the re-run must not create any new commit on specBranch; tip was %s, now %s", specTipAfterFirstRun, got)
	}

	// PRIOR MERGE'S GIT-SHA IDENTITY AND FindLandedMerge's OUTCOME ARE
	// INTACT (plan.md Steps §6's "PRIOR MERGES AND BINDINGS ARE INTACT"
	// clause, scoped per this file's SCOPE CAVEAT above — the literal
	// landed-BINDING metadata is never written in this fixture, see
	// there): bead N's producer-level refusal and re-run — including the
	// drift/squash commit that landed alongside it — must never disturb
	// bead M's own, already-landed merge commit or FindLandedMerge's
	// identification outcome for it.
	if !isAncestorRealGit2(t, root, priorMergeSHA, specBranch) {
		t.Errorf("the prior bead's landed merge commit %s must still be an ancestor of specBranch after bead N's refusal/re-run sequence", priorMergeSHA)
	}
	if !fileExistsAtRefRealGit(t, root, specBranch, "internal/widget/prior.go") {
		t.Error("the prior bead's content must still be present on specBranch")
	}
	priorLandedAfter, priorErrAfter := lifecycle.FindLandedMerge(root, specBranch, priorBeadID)
	if diff := landedMergeOutcomeDiff(priorLandedBefore, priorErrBefore, priorLandedAfter, priorErrAfter); diff != "" {
		t.Errorf("bead N's refusal/re-run sequence must not change FindLandedMerge's identification outcome for the PRIOR bead M: %s", diff)
	}
}

// landedMergeOutcomeDiff compares two FindLandedMerge outcomes (each a
// *lifecycle.LandedMerge plus its error) and returns a non-empty
// description of any difference, or "" if they are equivalent. Used to
// prove a DIFFERENT bead's refusal/re-run sequence left an unrelated
// bead's own landed-merge identification byte-for-byte unchanged,
// whatever that identification outcome is (positively identified,
// uncorroborated, or otherwise) — this test does not need FindLandedMerge
// to succeed for bead M, only to answer IDENTICALLY before and after.
func landedMergeOutcomeDiff(beforeLanded *lifecycle.LandedMerge, beforeErr error, afterLanded *lifecycle.LandedMerge, afterErr error) string {
	beforeOK := beforeErr == nil
	afterOK := afterErr == nil
	if beforeOK != afterOK {
		return fmt.Sprintf("success changed: before err=%v, after err=%v", beforeErr, afterErr)
	}
	if beforeOK {
		if beforeLanded.SHA != afterLanded.SHA || beforeLanded.SecondParent != afterLanded.SecondParent {
			return fmt.Sprintf("identified merge changed: before %+v, after %+v", beforeLanded, afterLanded)
		}
		return ""
	}
	if beforeErr.Error() != afterErr.Error() {
		return fmt.Sprintf("error text changed: before %q, after %q", beforeErr.Error(), afterErr.Error())
	}
	return ""
}

// isAncestorRealGit2 is isAncestorRealGit without the (bool, error)
// double-return noise at call sites that only need a plain bool and
// treat any probe error as "not confirmed ancestor" (t.Errorf already
// names the SHA on failure, so a probe error and a genuine non-ancestor
// answer are both worth failing the same assertion on).
func isAncestorRealGit2(t *testing.T, dir, ancestor, descendant string) bool {
	t.Helper()
	isAnc, err := isAncestorRealGit(t, dir, ancestor, descendant)
	return err == nil && isAnc
}

// fileExistsAtRefRealGit reports whether path exists in ref's tree in the
// real git repo at dir (`git cat-file -e <ref>:<path>`).
func fileExistsAtRefRealGit(t *testing.T, dir, ref, path string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "cat-file", "-e", ref+":"+path)
	return cmd.Run() == nil
}

// isAncestorRealGit reports whether ancestor is an ancestor of descendant
// in the real git repo at dir (`git merge-base --is-ancestor`).
func isAncestorRealGit(t *testing.T, dir, ancestor, descendant string) (bool, error) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "merge-base", "--is-ancestor", ancestor, descendant)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// branchExistsRealGit reports whether a local branch exists in the real
// git repo at dir.
func branchExistsRealGit(t *testing.T, dir, name string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--verify", "refs/heads/"+name)
	return cmd.Run() == nil
}
