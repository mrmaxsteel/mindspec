package executor

// Spec 127 bead 6 (R5(d)): end-to-end resumption tests over a REAL git
// repo, driving PRODUCTION CompleteBead through the full conflict →
// preserve → resolve → --resolve-merge lifecycle. These are the
// mechanism-level proof AC-9(v) requires: no raw `git merge` line is
// ever printed, a plain re-run never touches a preserved conflict, and
// --resolve-merge both re-prints steps (still conflicted) and completes
// (resolved+staged), landing the SAME cleanup a fresh merge success
// reaches.

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// TestCompleteBead_ResolveMerge_FullLifecycle drives the whole R5(d)
// resumption arc through production CompleteBead:
//  1. plain invocation conflicts → preserved (not aborted), no raw
//     `git merge` line, --resolve-merge named.
//  2. plain re-run (no flag) while the conflict is preserved → refuses
//     WITHOUT touching anything (AC-9(v)(δ)'s "plain no-flag re-run
//     refuses without aborting").
//  3. --resolve-merge while STILL conflicted → re-prints resolution
//     steps, still refuses, still does not touch the preserved merge.
//  4. operator resolves + stages (no commit).
//  5. --resolve-merge with the index resolved → completes the merge
//     (git commit --no-edit, preserving the original subject) and
//     proceeds through the SAME cleanup a fresh merge success would
//     reach (worktree removal, branch deletion).
func TestCompleteBead_ResolveMerge_FullLifecycle(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-x.1",
	}}
	// Make the fake's Remove reify a REAL `git worktree remove` so the
	// post-completion branch deletion below (which requires the bead
	// worktree to no longer be checked out) can actually succeed.
	fake.onRemove = func(name string) {
		if name == "worktree-mindspec-x.1" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
		}
	}

	// 1. Plain invocation: conflicts, preserved.
	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false)
	if err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}
	if strings.Contains(err.Error(), "git merge") {
		t.Fatalf("no raw `git merge` line may ever be printed; got:\n%s", err.Error())
	}
	if !strings.Contains(err.Error(), ResolveMergeFlag) {
		t.Fatalf("recovery must name %s; got:\n%s", ResolveMergeFlag, err.Error())
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the conflict must be PRESERVED (not aborted) after the first attempt")
	}

	// 2. Plain re-run (no --resolve-merge) while the conflict is
	// preserved: must refuse WITHOUT touching the preserved state
	// (AC-9(v)(delta)) — never abort, never checkout, never commit over it.
	preservedMergeHeadBefore := mergeHeadSHA(t, specWtPath)
	err = g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false)
	if err == nil {
		t.Fatal("a plain re-run over a preserved conflict must refuse")
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("a plain re-run must NOT abort the preserved conflict")
	}
	if got := mergeHeadSHA(t, specWtPath); got != preservedMergeHeadBefore {
		t.Fatalf("MERGE_HEAD must be untouched by a plain re-run; was %s, now %s", preservedMergeHeadBefore, got)
	}

	// 3. --resolve-merge while STILL conflicted: re-prints steps, still
	// refuses, still does not touch the preserved merge.
	err = g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true)
	if err == nil {
		t.Fatal("--resolve-merge over an unresolved conflict must still refuse")
	}
	if !strings.Contains(err.Error(), "conflicted") {
		t.Errorf("the still-conflicted refusal should mention the conflict; got:\n%s", err.Error())
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("--resolve-merge must NOT abort an unresolved conflict")
	}
	if got := mergeHeadSHA(t, specWtPath); got != preservedMergeHeadBefore {
		t.Fatalf("MERGE_HEAD must be untouched while still conflicted; was %s, now %s", preservedMergeHeadBefore, got)
	}

	// 4. Operator resolves and STAGES (no commit) — resumeReadyToComplete.
	if err := os.WriteFile(specWtPath+"/c.txt", []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	runGitIn(t, specWtPath, "add", "c.txt")

	// 5. --resolve-merge with the index resolved: completes the merge
	// and proceeds through cleanup.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("--resolve-merge with a resolved index must complete the merge, got: %v", err)
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("the merge must be COMPLETE — no MERGE_HEAD should remain")
	}
	if got, readErr := os.ReadFile(specWtPath + "/c.txt"); readErr != nil || string(got) != "resolved\n" {
		t.Errorf("the spec branch tip must carry the resolved content; got %q, err=%v", got, readErr)
	}
	// Cleanup ran (bead worktree/branch removed, same as a fresh-merge
	// success) — proof the merge completion fell through to the SAME
	// post-merge cleanup path, not a special-cased short circuit.
	// (IsAncestor cannot be checked here: cleanup already deleted the
	// bead branch by the time CompleteBead returns.)
	if len(fake.removeCalls) == 0 {
		t.Error("cleanup (worktree removal) must run after the resumed merge completes, same as a fresh merge success")
	}
	if branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("the bead branch must be deleted after the resumed merge completes and cleanup runs")
	}
}

// mergeHeadSHA reads MERGE_HEAD's resolved SHA in workdir (empty if
// absent), used to prove a plain re-run or a still-conflicted
// --resolve-merge invocation never touches the preserved merge state.
func mergeHeadSHA(t *testing.T, workdir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", workdir, "rev-parse", "MERGE_HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// TestResumeAwareMerge_NoMergeStateFailureIsDistinctFromConflict is
// AC-9(v)(zeta)/R5(d)(vi): a merge that fails to even START (a dirty
// worktree blocking the checkout/merge, no MERGE_HEAD ever created) is
// diagnosed as a WORKTREE-STATE refusal — never the word "conflict",
// naming the blocking paths — never the still-conflicted resolution
// steps, which would loop forever over a worktree that was never
// mid-merge.
func TestResumeAwareMerge_NoMergeStateFailureIsDistinctFromConflict(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-x.1",
	}}

	// Dirty the spec worktree on the SAME path the bead branch also
	// touches, uncommitted — `git merge` refuses to even START (no
	// MERGE_HEAD is ever created), the exact worktree-state class.
	if err := os.WriteFile(specWtPath+"/c.txt", []byte("uncommitted local edit\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false)
	if err == nil {
		t.Fatal("expected a worktree-state refusal, got nil")
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Fatal("fixture invariant broken: a merge that never started must leave no MERGE_HEAD")
	}
	msg := err.Error()
	if strings.Contains(msg, "conflict") {
		t.Errorf("a no-merge-state failure must never use the word \"conflict\"; got:\n%s", msg)
	}
	if !strings.Contains(msg, "c.txt") {
		t.Errorf("the blocking path (c.txt) must be named; got:\n%s", msg)
	}
}
