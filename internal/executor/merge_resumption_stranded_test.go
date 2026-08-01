package executor

// Spec 127 bead-6 fix round 3: BLOCKING 1's acceptance tests
// (S1-1/G1-3/O1-1/F1-1's panel review). completeDriftedResumedMerge's
// ResetSoft call can fail AFTER its CommitTreeMerge already succeeded,
// leaving the branch at the un-collapsed two-merge topology with
// MERGE_HEAD already cleared — the exact ambiguous shape spec 125's
// FindLandedMerge refuses. Before this fix, a BARE re-invocation would
// silently no-op ("already up to date") over that stranded shape and
// report apparent success. These tests fault-inject the ResetSoft
// failure via resetSoftFn (merge_resumption.go's fault-injection seam)
// and prove: (1) the stranding itself surfaces as a loud error, never
// silent success; (2) a re-invocation repairs the collapse and converges
// once the underlying failure clears; (3) a repair attempt that ITSELF
// fails is also loud and distinguishable, never silently retried as a
// fresh merge.

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// driftedResolveMergeFixture drives CompleteBead through a conflict,
// drift, and resolved-conflict setup identical to
// TestCompleteBead_ResolveMerge_SourceDriftIsIncorporated, stopping just
// before the --resolve-merge invocation that will trigger
// completeDriftedResumedMerge's collapse — shared by both tests below.
func driftedResolveMergeFixture(t *testing.T) (g *MindspecExecutor, dir, specWtPath, beadWtDir, driftedBeadTip string) {
	t.Helper()
	var fake *fakeWorktreeOps
	g, fake, dir = newRepoExecutor(t)
	specWtPath, beadWtDir = setupConflictingSpecAndBead(t, dir)

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-x.1",
	}}
	fake.onRemove = func(name string) {
		if name == "worktree-mindspec-x.1" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
		}
	}

	// 1. Plain invocation: conflicts, preserved (gitutil.MergeInto writes
	// the merge-start marker for bead/mindspec-x.1 here).
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false); err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}

	// 2. DRIFT: a new commit lands on the bead branch after the conflict
	// began, on a path the resolved conflict never touches (a clean
	// catch-up, not a second conflict).
	if err := os.WriteFile(beadWtDir+"/drift.txt", []byte("drifted work\n"), 0o644); err != nil {
		t.Fatalf("write drift file: %v", err)
	}
	runGitIn(t, beadWtDir, "add", "drift.txt")
	runGitIn(t, beadWtDir, "commit", "-m", "drift: bead branch advances after the conflict began")
	driftedBeadTip = refHash(t, dir, "bead/mindspec-x.1")

	// 3. Resolve + stage the ORIGINAL conflict.
	if err := os.WriteFile(specWtPath+"/c.txt", []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	runGitIn(t, specWtPath, "add", "c.txt")

	return g, dir, specWtPath, beadWtDir, driftedBeadTip
}

// TestCompleteDriftedResumedMerge_ResetSoftFailureStrandsThenRepairs is
// BLOCKING 1's central acceptance test: a ResetSoft failure immediately
// after a successful CommitTreeMerge must surface loudly on the
// invocation that hits it, and a LATER bare re-invocation must repair the
// stranded topology and converge — never silently no-op over it.
func TestCompleteDriftedResumedMerge_ResetSoftFailureStrandsThenRepairs(t *testing.T) {
	g, dir, specWtPath, _, driftedBeadTip := driftedResolveMergeFixture(t)

	// Fault-inject: fail exactly the FIRST ResetSoft call (S1-1's real-git
	// repro) — every call after that is the real gitutil.ResetSoft, so
	// the LATER repair invocation can actually converge.
	origResetSoft := resetSoftFn
	t.Cleanup(func() { resetSoftFn = origResetSoft })
	failNext := true
	resetSoftFn = func(workdir, target string) error {
		if failNext {
			failNext = false
			return errors.New("simulated ref-lock contention")
		}
		return origResetSoft(workdir, target)
	}

	// 4. --resolve-merge: C1 (the stale resumed merge) and C2 (the clean
	// catch-up) both land for real, CommitTreeMerge succeeds, but the
	// injected ResetSoft failure leaves the branch STRANDED at C2 —
	// MERGE_HEAD already cleared, the collapsed commit dangling.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err == nil {
		t.Fatal("the injected ResetSoft failure must surface as an error, never silent success")
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Fatal("fixture invariant broken: MERGE_HEAD must already be cleared by the catch-up merge commit")
	}
	strandedTip := refHash(t, dir, "spec/077-test")
	merges, err := gitutil.FirstParentMerges(dir, "spec/077-test")
	if err != nil {
		t.Fatalf("FirstParentMerges: %v", err)
	}
	if len(merges) < 2 || merges[0].SHA != strandedTip || len(merges[0].Parents) != 2 || merges[0].Parents[1] != driftedBeadTip {
		t.Fatalf("fixture invariant broken: expected the stranded topology (C2 at the tip, second parent = drifted tip); got %+v", merges)
	}
	if merges[1].SHA != merges[0].Parents[0] || len(merges[1].Parents) != 2 {
		t.Fatalf("fixture invariant broken: expected C1 to be C2's immediate first parent, both two-parent merges; got %+v", merges)
	}
	if merges[0].Subject != merges[1].Subject {
		t.Fatalf("fixture invariant broken: C1 and C2 must share the identical seeded subject (the exact ambiguity spec 125's FindLandedMerge refuses); got %q vs %q", merges[0].Subject, merges[1].Subject)
	}

	// 5. Bare re-invocation (still --resolve-merge, matching how an
	// operator would retry): resumeAwareMerge's resumeNoMergeInProgress
	// leg now repairs the stranded topology FIRST, before ever reaching
	// attemptFreshMerge (whose mergeFn() would otherwise silently no-op
	// "already up to date" over it). ResetSoft succeeds this time
	// (failNext already consumed).
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("the re-invocation must repair the stranded collapse and converge, got: %v", err)
	}

	// The topology is now collapsed: the tip is exactly ONE merge commit
	// (not the two-merge shape above), its second parent the drifted
	// tip, its subject the original seeded form.
	collapsed, err := gitutil.FirstParentMerges(dir, "spec/077-test")
	if err != nil {
		t.Fatalf("FirstParentMerges (post-repair): %v", err)
	}
	if len(collapsed) < 1 || len(collapsed[0].Parents) != 2 || collapsed[0].Parents[1] != driftedBeadTip {
		t.Fatalf("post-repair: expected a single collapsed merge whose second parent is the drifted tip; got %+v", collapsed)
	}
	if collapsed[0].Parents[0] != merges[1].Parents[0] {
		t.Errorf("post-repair: the collapsed merge's first parent must be the ORIGINAL pre-resumption tip (never C1); got %s want %s", collapsed[0].Parents[0], merges[1].Parents[0])
	}
	wantSubject := "Merge bead/mindspec-x.1"
	if collapsed[0].Subject != wantSubject {
		t.Errorf("post-repair subject = %q, want %q", collapsed[0].Subject, wantSubject)
	}
	if got, readErr := os.ReadFile(specWtPath + "/c.txt"); readErr != nil || string(got) != "resolved\n" {
		t.Errorf("the resolved content must survive the repair; got %q, err=%v", got, readErr)
	}
	if got, readErr := os.ReadFile(specWtPath + "/drift.txt"); readErr != nil || string(got) != "drifted work\n" {
		t.Errorf("the drifted content must survive the repair; got %q, err=%v", got, readErr)
	}
	if branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("the bead branch must be deleted once the repaired merge fully converges")
	}
}

// TestRepairStrandedDriftCollapse_RepairFailureIsLoudNotSilent is
// BLOCKING 1's second acceptance leg: when the repair's OWN ResetSoft
// ALSO fails, that must be a loud, distinguishable failure
// (*strandedCollapseError) — never silently retried as a fresh merge,
// and the branch must be left exactly where it was (still stranded, not
// further corrupted), so a LATER retry (once the underlying failure
// clears) can still converge.
func TestRepairStrandedDriftCollapse_RepairFailureIsLoudNotSilent(t *testing.T) {
	g, dir, _, _, _ := driftedResolveMergeFixture(t)

	origResetSoft := resetSoftFn
	t.Cleanup(func() { resetSoftFn = origResetSoft })
	resetSoftFn = func(workdir, target string) error {
		return errors.New("simulated persistent ref-lock contention")
	}

	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err == nil {
		t.Fatal("expected the first ResetSoft failure to surface")
	}
	strandedTip := refHash(t, dir, "spec/077-test")

	// Bare re-invocation: the repair ALSO fails (ResetSoft is still
	// stubbed to always fail) — must be loud and distinguishable, never
	// a silent no-op success laundering the stranded topology.
	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true)
	if err == nil {
		t.Fatal("a repair attempt that itself fails must never report success")
	}
	var sce *strandedCollapseError
	if !errors.As(err, &sce) {
		t.Errorf("expected a *strandedCollapseError naming the stranded topology, got %T: %v", err, err)
	}
	if got := refHash(t, dir, "spec/077-test"); got != strandedTip {
		t.Errorf("a failed repair attempt must not move the branch; was %s, now %s", strandedTip, got)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("the bead branch must survive a repair failure — no cleanup runs on a refusal")
	}
}
