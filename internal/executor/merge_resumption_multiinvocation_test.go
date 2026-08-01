package executor

// Spec 127 bead-6 fix round 4: proves (and closes) the confirm-round
// question round 3 raised but did not answer — completeDriftedResumedMerge's
// STATED RESIDUAL 1 (a drift catch-up that itself re-conflicts, resolved by
// a LATER, SEPARATE invocation) produces the identical structural shape
// (two adjacent, same-subject, two-parent merge commits) that
// detectStrandedDriftTopology's ADJACENCY+SUBJECT check alone cannot tell
// apart from a genuinely stranded ResetSoft failure — but this shape is
// reached via two entirely successful `git commit --no-edit` calls,
// NEVER commitTreeMergeFn/resetSoftFn. Round 3's own reasoning ("the repair
// cannot even run against it") only covers the invocation immediately
// adjacent to the re-conflict, while MERGE_HEAD is still live; it does not
// follow the chain to what happens once the SECOND commit lands and
// MERGE_HEAD clears.
//
// Two tests below construct (never merely reason about) both reachable
// sub-cases of that later state — an ensureLandedBinding write failure, and
// a binding SUCCESS followed by a best-effort branch-delete failure — and
// prove repairStrandedDriftCollapse never collapses either: the dangling-
// object corroboration (gitops.go's DanglingCollapsedMergeExists) only
// answers "stranded" when an interrupted CommitTreeMerge call actually left
// its own output object behind, which neither sub-case here ever produces.

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// twoInvocationDriftReconflictFixture drives CompleteBead through a
// PRESERVED conflict, a drift commit that ALSO conflicts with the
// resolution (so the drift catch-up itself re-conflicts — STATED RESIDUAL
// 1's precondition), then resolves BOTH conflicts across two separate
// --resolve-merge invocations. By the time it returns, the spec branch
// carries C1 (the completed, now-stale original merge) and C2 (the
// completed drift catch-up) as two adjacent, same-subject, two-parent
// merge commits — produced entirely by real `git commit --no-edit` calls,
// with NO commitTreeMergeFn/resetSoftFn call ever made. Returns the tip
// SHAs of both.
func twoInvocationDriftReconflictFixture(t *testing.T) (g *MindspecExecutor, dir, specWtPath, beadWtDir string, c1, c2, driftedBeadTip string) {
	t.Helper()
	g, _, dir, specWtPath, beadWtDir, c1, c2, driftedBeadTip = twoInvocationDriftReconflictFixtureWithFake(t)
	return g, dir, specWtPath, beadWtDir, c1, c2, driftedBeadTip
}

// twoInvocationDriftReconflictFixtureWithFake is
// twoInvocationDriftReconflictFixture's own implementation, additionally
// exposing the *fakeWorktreeOps so a test can chain further behavior onto
// its onRemove callback (the branch-delete-failure sub-case below).
func twoInvocationDriftReconflictFixtureWithFake(t *testing.T) (g *MindspecExecutor, fake *fakeWorktreeOps, dir, specWtPath, beadWtDir string, c1, c2, driftedBeadTip string) {
	t.Helper()
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

	// 1. Plain invocation: conflicts on c.txt, preserved (also writes the
	// merge-source marker — producer-written evidence, not unforgeable;
	// see gitutil.MergeSourceMarkerRef's doc comment — at merge start).
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false); err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}

	// 2. DRIFT: a second commit on the bead branch that ALSO touches c.txt
	// — the drift catch-up (step 4 below) will conflict too, unlike
	// driftedResolveMergeFixture's non-conflicting drift.txt.
	if err := os.WriteFile(beadWtDir+"/c.txt", []byte("bead side v2\n"), 0o644); err != nil {
		t.Fatalf("write drift: %v", err)
	}
	runGitIn(t, beadWtDir, "add", "c.txt")
	runGitIn(t, beadWtDir, "commit", "-m", "drift: bead branch advances again, touching the SAME file")
	driftedBeadTip = refHash(t, dir, "bead/mindspec-x.1")

	// 3. Resolve + stage the ORIGINAL conflict.
	if err := os.WriteFile(specWtPath+"/c.txt", []byte("resolved-v1\n"), 0o644); err != nil {
		t.Fatalf("write resolution v1: %v", err)
	}
	runGitIn(t, specWtPath, "add", "c.txt")

	// 4. --resolve-merge: completes C1 (the stale resumed merge, a plain
	// `git commit --no-edit`), then attempts the drift catch-up — which
	// ALSO conflicts (STATED RESIDUAL 1's exact precondition).
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err == nil {
		t.Fatal("expected the drift catch-up to ALSO conflict")
	}
	c1 = refHash(t, dir, "spec/077-test")
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("fixture invariant broken: expected the catch-up conflict to leave a fresh MERGE_HEAD")
	}

	// 5. Resolve the catch-up conflict.
	if err := os.WriteFile(specWtPath+"/c.txt", []byte("resolved-v2\n"), 0o644); err != nil {
		t.Fatalf("write resolution v2: %v", err)
	}
	runGitIn(t, specWtPath, "add", "c.txt")

	return g, fake, dir, specWtPath, beadWtDir, c1, "", driftedBeadTip
}

// assertLegitimateTwoMergeChain pins the fixture invariant every test below
// depends on: C1 and C2 adjacent, both exactly-two-parent, sharing the
// identical seeded subject, C2's second parent the drifted tip and C1's
// the STALE original tip — and (the whole point of this file) that no
// dangling commit-tree object with the would-be-collapsed shape exists,
// because nothing in this sequence ever called commitTreeMergeFn.
func assertLegitimateTwoMergeChain(t *testing.T, dir, specWtPath, c1, c2, driftedBeadTip string) {
	t.Helper()
	merges, err := gitutil.FirstParentMerges(dir, "spec/077-test")
	if err != nil {
		t.Fatalf("FirstParentMerges: %v", err)
	}
	if len(merges) < 2 || merges[0].SHA != c2 || merges[1].SHA != c1 {
		t.Fatalf("fixture invariant broken: expected C2 at tip with C1 as its first parent; got %+v (c1=%s c2=%s)", merges, c1, c2)
	}
	if merges[0].Parents[1] != driftedBeadTip {
		t.Fatalf("C2's second parent should be the drifted tip; got %s want %s", merges[0].Parents[1], driftedBeadTip)
	}
	if merges[0].Subject != merges[1].Subject {
		t.Fatalf("C1/C2 must share the identical seeded subject: %q vs %q", merges[1].Subject, merges[0].Subject)
	}
	proven, derr := gitutil.DanglingCollapsedMergeExists(specWtPath, mustTree(t, specWtPath), merges[1].Parents[0], driftedBeadTip, merges[0].Subject)
	if derr != nil {
		t.Fatalf("DanglingCollapsedMergeExists: %v", derr)
	}
	if proven {
		t.Fatal("fixture invariant broken: a dangling collapsed-shape commit unexpectedly exists — this sequence must never call commitTreeMergeFn")
	}
}

func mustTree(t *testing.T, workdir string) string {
	t.Helper()
	tree, err := gitutil.TreeSHA(workdir, "HEAD")
	if err != nil {
		t.Fatalf("TreeSHA: %v", err)
	}
	return tree
}

// TestRepairStrandedDriftCollapse_NeverCollapsesLegitimateChain_BindingFailureRetry
// is the confirm-round question's central acceptance test, sub-case (a):
// ensureLandedBinding itself fails on the invocation that lands C2 — the
// bead branch SURVIVES (ADR-0041 §2(ii)) with NO binding yet recorded. A
// later retry reaches resumeNoMergeInProgress (MERGE_HEAD is already clear)
// and must NOT collapse C1+C2 away — there is no dangling proof of a real
// interruption, only a legitimately-produced multi-invocation chain.
func TestRepairStrandedDriftCollapse_NeverCollapsesLegitimateChain_BindingFailureRetry(t *testing.T) {
	g, dir, specWtPath, _, c1, _, driftedBeadTip := twoInvocationDriftReconflictFixture(t)

	// 6. --resolve-merge again: bindingExact this time (MERGE_HEAD equals
	// expectedSource's current tip exactly) — a plain completeResumedMerge
	// produces C2 via a REAL `git commit --no-edit`. Fault-inject
	// mergeBindingFn to fail exactly here.
	origBind := mergeBindingFn
	t.Cleanup(func() { mergeBindingFn = origBind })
	mergeBindingFn = func(string, map[string]interface{}) error {
		return errors.New("simulated transient bd/Dolt write failure")
	}
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err == nil {
		t.Fatal("expected the injected binding-write failure to surface and suppress cleanup")
	}
	c2 := refHash(t, dir, "spec/077-test")
	if gitutil.MergeInProgress(specWtPath) {
		t.Fatal("MERGE_HEAD should already be clear — the plain completeResumedMerge already committed C2")
	}
	if !branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Fatal("the bead branch must SURVIVE a binding-write failure (ADR-0035)")
	}
	assertLegitimateTwoMergeChain(t, dir, specWtPath, c1, c2, driftedBeadTip)

	// 7. Retry, binding write now fixed: this is the invocation under
	// investigation. It must converge WITHOUT rewriting C1/C2 away.
	mergeBindingFn = origBind
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("retry with the binding write fixed should converge, got: %v", err)
	}
	finalTip := refHash(t, dir, "spec/077-test")
	if finalTip != c2 {
		t.Fatalf("the legitimate two-invocation chain must NOT be collapsed: tip moved from C2 (%s) to %s", c2, finalTip)
	}
	finalMerges, err := gitutil.FirstParentMerges(dir, "spec/077-test")
	if err != nil {
		t.Fatalf("FirstParentMerges (final): %v", err)
	}
	if len(finalMerges) < 2 || finalMerges[0].SHA != c2 || finalMerges[1].SHA != c1 {
		t.Fatalf("C1 and C2 must both survive as separate commits; got %+v", finalMerges)
	}
	if branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("the bead branch must be deleted once the retry converges")
	}
}

// TestRepairStrandedDriftCollapse_NeverCollapsesLegitimateChain_BranchDeleteFailureAfterBind
// is sub-case (b): ensureLandedBinding SUCCEEDS (a durable binding now names
// C2 by SHA) but the best-effort branch-delete that follows fails for real
// (the branch is still checked out in ANOTHER worktree — gitutil.DeleteBranch's
// own failure is only ever a printed WARNING, never a returned error). A
// later retry again reaches resumeNoMergeInProgress against the identical
// C1/C2 shape — and must not collapse it, because C2 is now named by a
// DURABLE binding: collapsing it would orphan the exact commit the binding
// points at, turning a clean convergent retry into a silently broken
// binding.
func TestRepairStrandedDriftCollapse_NeverCollapsesLegitimateChain_BranchDeleteFailureAfterBind(t *testing.T) {
	g, fake, dir, specWtPath, _, c1, _, driftedBeadTip := twoInvocationDriftReconflictFixtureWithFake(t)

	// A second, untracked worktree keeps bead/mindspec-x.1 checked out
	// even after the fixture's own (fake-mediated) bead worktree is
	// removed — a real, unforced reason `git branch -D` fails, no
	// fault-injection seam needed. Chained onto the SAME onRemove callback
	// that removes the original bead worktree, so the branch is free at
	// the moment this runs but checked out again before CompleteBead's own
	// DeleteBranch call fires immediately after.
	extraWt := dir + "-extra-checkout"
	origOnRemove := fake.onRemove
	fake.onRemove = func(name string) {
		origOnRemove(name)
		if name == "worktree-mindspec-x.1" {
			_ = exec.Command("git", "-C", dir, "worktree", "add", extraWt, "bead/mindspec-x.1").Run()
		}
	}
	t.Cleanup(func() {
		_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", extraWt).Run()
	})

	// A real (in-memory) binding store spanning both CompleteBead calls
	// below — newRepoExecutor's default stub write/read pair does not
	// persist between calls, so a bare read-back would always see "not yet
	// bound" regardless of what a prior write recorded.
	store := map[string]map[string]interface{}{}
	origBindWrite, origBindRead := mergeBindingFn, mergeBindingReadFn
	t.Cleanup(func() { mergeBindingFn, mergeBindingReadFn = origBindWrite, origBindRead })
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		store[id] = updates
		return nil
	}
	mergeBindingReadFn = func(id string) (map[string]interface{}, error) {
		if m, ok := store[id]; ok {
			return m, nil
		}
		return map[string]interface{}{}, nil
	}

	// 6. --resolve-merge again: binding succeeds this time (no fault
	// injection), but the branch-delete cleanup step fails for real.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("CompleteBead should still report success (DeleteBranch failure is a warning, not a returned error), got: %v", err)
	}
	// The fixture's fake.listEntries is static (Remove does not prune it),
	// so a later CompleteBead call would re-run WorktreeOps.Remove and
	// re-chain the extra-worktree-add above. Only the ONE invocation above
	// needs to force the branch-delete failure — restore the plain
	// onRemove now so the retry below does not re-create extraWt itself.
	fake.onRemove = origOnRemove
	c2 := refHash(t, dir, "spec/077-test")
	if gitutil.MergeInProgress(specWtPath) {
		t.Fatal("MERGE_HEAD should already be clear")
	}
	if !branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Fatal("fixture invariant broken: the bead branch must SURVIVE — DeleteBranch must have failed (still checked out in the extra worktree)")
	}
	assertLegitimateTwoMergeChain(t, dir, specWtPath, c1, c2, driftedBeadTip)

	existing, readErr := mergeBindingReadFn("mindspec-x.1")
	if readErr != nil {
		t.Fatalf("reading the recorded binding: %v", readErr)
	}
	if sha, _ := existing["mindspec_landed_merge_sha"].(string); sha != c2 {
		t.Fatalf("fixture invariant broken: expected a durable binding naming C2 (%s); got %q", c2, sha)
	}

	// 7. Retry (the extra worktree is now removed, so DeleteBranch can
	// succeed): must converge WITHOUT rewriting the already-bound C2 away.
	if err := exec.Command("git", "-C", dir, "worktree", "remove", "--force", extraWt).Run(); err != nil {
		t.Fatalf("removing the extra worktree: %v", err)
	}
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("retry should converge, got: %v", err)
	}
	finalTip := refHash(t, dir, "spec/077-test")
	if finalTip != c2 {
		t.Fatalf("the already-bound C2 must survive unrewritten: tip moved from %s to %s", c2, finalTip)
	}
	if branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("the bead branch must be deleted once the retry converges")
	}
}
