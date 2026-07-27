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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// TestRun_TargetDriftBackstop_ProducerRefusesAfterSection1Passes is the
// mandatory target-drift backstop fixture: the producer (executor-level)
// preflight refuses a merge that the §1-phase preflight evaluated
// permissively moments earlier, because the TARGET drifted in between —
// proving the two-layer siting argument empirically, not by inspection.
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

	ex := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}

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
