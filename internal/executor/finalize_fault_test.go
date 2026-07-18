package executor

// Spec 119 Bead 6 (AC-26 i4 / ADR-0041): FinalizeEpic is a single COMMIT-
// phase mutation chain with no existing seam separating its internal
// steps, so mechanism C (the stage-labeled finalizeStepHookFn, mindspec_
// executor.go) exists purely for this fault-injection matrix. Each test
// below sets the hook to fail at ONE stage, confirms the REAL mutations up
// to that stage genuinely landed (a real merge, a real push, a real
// branch/worktree removal), then clears the hook and re-invokes FinalizeEpic
// to confirm convergence — to completion for stages (a)/(b)/(c)/(d), and to
// a clean named refusal for stage (e), whose kill point is the LAST
// mutation in the chain (nothing is left to converge TO — the state is
// already fully finalized, and FinalizeEpic's own first check, "spec branch
// does not exist", is the honest recoverable refusal a second invocation
// hits).

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

// clearFinalizeStepHook is t.Cleanup-registered by every test below so a
// failing hook never leaks into another test.
func clearFinalizeStepHook(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { finalizeStepHookFn = nil })
}

// TestFinalizeStepHookFn_DefaultsToNil pins the production default this
// package doc comment promises: the hook is nil unless a test sets it, so
// finalizeStepHook is a pure no-op in production.
func TestFinalizeStepHookFn_DefaultsToNil(t *testing.T) {
	if finalizeStepHookFn != nil {
		t.Fatal("finalizeStepHookFn must default to nil (test-only seam)")
	}
	if err := finalizeStepHook("any-stage"); err != nil {
		t.Fatalf("finalizeStepHook with a nil hook must be a no-op, got: %v", err)
	}
}

var errFinalizeFault = errors.New("fault-injection: simulated kill at this stage")

// failAtStage returns a finalizeStepHookFn that fails ONLY at the named
// stage — every other stage the real FinalizeEpic invokes is a no-op pass.
func failAtStage(stage string) func(string) error {
	return func(s string) error {
		if s == stage {
			return errFinalizeFault
		}
		return nil
	}
}

// --- stage (a): after the bead-branch auto-merge leg ----------------------

func TestFinalizeFault_AutoMerge_KillThenConverge(t *testing.T) {
	clearFinalizeStepHook(t)
	g, fake, dir := newRepoExecutor(t)
	stubBeadExport(t)

	runGitIn(t, dir, "branch", "spec/077-test", "main")
	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-077-test")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/077-test")

	runGitIn(t, dir, "branch", "bead/b1", "spec/077-test")
	scratchPath := filepath.Join(dir, ".worktrees", "worktree-bead-scratch")
	runGitIn(t, dir, "worktree", "add", scratchPath, "bead/b1")
	if err := os.WriteFile(filepath.Join(scratchPath, "b1.txt"), []byte("b1"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, scratchPath, "add", ".")
	runGitIn(t, scratchPath, "commit", "-m", "b1 work")
	runGitIn(t, dir, "worktree", "remove", scratchPath)

	fake.listEntries = []bead.WorktreeListEntry{{Name: "worktree-b1", Path: "", Branch: "bead/b1"}}
	allowSet := []string{"b1"}

	// The fake WorktreeOps.Remove is a no-op by default; FinalizeEpic's
	// cleanup leg later needs the REAL linked spec worktree actually gone
	// (a linked worktree still checked out on spec/077-test blocks the real
	// `git branch -D`), so wire onRemove to perform the real removal.
	fake.onRemove = func(name string) {
		if name == "worktree-spec-077-test" {
			runGitIn(t, dir, "worktree", "remove", "--force", specWtPath)
		}
	}

	finalizeStepHookFn = failAtStage("auto_merge")
	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", allowSet)
	if err == nil || !errors.Is(err, errFinalizeFault) {
		t.Fatalf("expected the auto_merge kill, got: %v", err)
	}
	if !isAncestorIn(t, dir, "bead/b1", "spec/077-test") {
		t.Fatal("expected the real auto-merge to have landed bead/b1 into spec/077-test despite the kill")
	}

	finalizeStepHookFn = nil
	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", allowSet)
	if err != nil {
		t.Fatalf("expected re-invocation to converge to completion, got: %v", err)
	}
	if branchExistsIn(t, dir, "spec/077-test") {
		t.Error("expected the spec branch to be gone after full convergence (no-remote direct finalize)")
	}
	if result.MergeStrategy != "direct" {
		t.Errorf("MergeStrategy = %q, want direct (no remote configured)", result.MergeStrategy)
	}
}

// --- stage (b): after the unconditional spec-branch push ------------------

func TestFinalizeFault_Push_KillThenConverge(t *testing.T) {
	clearFinalizeStepHook(t)
	g, fake, dir := newRepoExecutor(t)
	stubBeadExport(t)
	origin := setupOrphanOrigin(t, dir, false)
	fake.listEntries = nil

	finalizeStepHookFn = failAtStage("push")
	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err == nil || !errors.Is(err, errFinalizeFault) {
		t.Fatalf("expected the push kill, got: %v", err)
	}
	if !branchExistsIn(t, origin, "spec/077-test") {
		t.Fatal("expected the real push to have landed spec/077-test on origin despite the kill")
	}
	preKillTip := refHash(t, origin, "spec/077-test")

	finalizeStepHookFn = nil
	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err != nil {
		t.Fatalf("expected re-invocation to converge (the push is idempotent), got: %v", err)
	}
	if result.MergeStrategy != "pr" {
		t.Errorf("MergeStrategy = %q, want pr", result.MergeStrategy)
	}
	if got := refHash(t, origin, "spec/077-test"); got != preKillTip {
		t.Errorf("expected the re-pushed tip to be unchanged (idempotent push), got %s want %s", got, preKillTip)
	}
}

// --- stage (c): after finalizeOrphanedSpecBranch returns (bug wu7t path) --

func TestFinalizeFault_OrphanFinalize_KillThenConverge(t *testing.T) {
	clearFinalizeStepHook(t)
	g, fake, dir := newRepoExecutor(t)
	exportContent := stubBeadExport(t)
	origin := setupOrphanOrigin(t, dir, true)
	fake.listEntries = nil

	finalizeStepHookFn = failAtStage("orphan_finalize")
	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err == nil || !errors.Is(err, errFinalizeFault) {
		t.Fatalf("expected the orphan_finalize kill, got: %v", err)
	}
	if !branchExistsIn(t, origin, "chore/finalize-077-test") {
		t.Fatal("expected the real chore/finalize branch to have reached origin despite the kill")
	}
	run1Tip := refHash(t, origin, "chore/finalize-077-test")

	// The retry's export content changes (Dolt state moved) — the exact
	// shape that made a plain push non-fast-forward pre-wu7t-fix.
	*exportContent = []byte(`{"id":"epic-1","status":"closed","retry":2}` + "\n")

	finalizeStepHookFn = nil
	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err != nil {
		t.Fatalf("expected re-invocation to converge (retry-idempotent chore flow), got: %v", err)
	}
	if result.FinalizeBranch != "chore/finalize-077-test" {
		t.Fatalf("FinalizeBranch = %q, want chore/finalize-077-test", result.FinalizeBranch)
	}
	if run2Tip := refHash(t, origin, "chore/finalize-077-test"); run2Tip == run1Tip {
		t.Error("expected the retry to replace the remote chore tip with its fresh export commit")
	}
}

// --- stage (d): between the merge/push legs and the cleanup leg -----------

func TestFinalizeFault_PreCleanup_KillThenConverge(t *testing.T) {
	clearFinalizeStepHook(t)
	g, fake, dir := newRepoExecutor(t)
	stubBeadExport(t)
	runGitIn(t, dir, "branch", "spec/077-test", "main")
	fake.listEntries = nil

	finalizeStepHookFn = failAtStage("pre_cleanup")
	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err == nil || !errors.Is(err, errFinalizeFault) {
		t.Fatalf("expected the pre_cleanup kill, got: %v", err)
	}
	// Nothing from the cleanup leg has run yet: the spec branch must still
	// exist (no direct merge, no deletion attempted).
	if !branchExistsIn(t, dir, "spec/077-test") {
		t.Fatal("expected the spec branch to still exist before the cleanup leg has run")
	}

	finalizeStepHookFn = nil
	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err != nil {
		t.Fatalf("expected re-invocation to converge to completion, got: %v", err)
	}
	if result.MergeStrategy != "direct" {
		t.Errorf("MergeStrategy = %q, want direct", result.MergeStrategy)
	}
	if branchExistsIn(t, dir, "spec/077-test") {
		t.Error("expected the spec branch to be gone after full convergence")
	}
}

// --- stage (e): after the cleanup leg's mutations complete -----------------
//
// This is the LAST mutation in the chain: by the time this hook fires,
// every cleanup mutation (worktree/branch removals, the direct spec→main
// merge, spec-branch deletion) has already landed for real. There is
// nothing left to converge TO by re-invoking FinalizeEpic — the spec branch
// genuinely no longer exists — so the honest re-invocation outcome is
// FinalizeEpic's own first-line refusal ("spec branch does not exist"), a
// clean, named, recoverable message rather than a second attempt at
// mutations that already succeeded. This is the AC-26-accepted "clean
// recoverable refusal" convergence outcome, not "completion" (there is
// nothing left to complete).
func TestFinalizeFault_PostCleanup_KillThenCleanRefusal(t *testing.T) {
	clearFinalizeStepHook(t)
	g, fake, dir := newRepoExecutor(t)
	stubBeadExport(t)
	runGitIn(t, dir, "branch", "spec/077-test", "main")
	fake.listEntries = nil

	finalizeStepHookFn = failAtStage("post_cleanup")
	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err == nil || !errors.Is(err, errFinalizeFault) {
		t.Fatalf("expected the post_cleanup kill, got: %v", err)
	}
	// Every cleanup mutation already landed for real: the spec branch is
	// genuinely gone.
	if branchExistsIn(t, dir, "spec/077-test") {
		t.Fatal("expected the real cleanup to have already deleted the spec branch despite the kill")
	}

	finalizeStepHookFn = nil
	_, err = g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err == nil {
		t.Fatal("expected the re-invocation to hit FinalizeEpic's own clean 'spec branch does not exist' refusal")
	}
	if got := err.Error(); got != "spec branch spec/077-test does not exist" {
		t.Errorf("expected the clean named refusal, got: %q", got)
	}
}
