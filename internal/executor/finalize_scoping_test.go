package executor

// Spec 119 Bead 3 (R6, AC-13, AC-14) — FinalizeEpic lifecycle-scoping
// fixtures. Both bead-branch enumerations (the auto-merge leg and the
// worktree/branch cleanup leg) must be scoped to the passed
// lifecycleAllowSet: a candidate bead/<id> is admitted iff its id is a
// member. The exclusion boundary is lifecycle IDENTITY, not open-status —
// a same-epic non-lifecycle follow-up (open OR closed) and a foreign-epic
// bead must survive BOTH legs untouched, while the allow-set's own
// lifecycle beads merge and get cleaned up. lifecycleAllowSet itself is
// computed upstream (internal/approve, via the new internal/phase
// classifier ∩ the plan-declared bead_ids) — this executor-level fixture
// exercises the SAME scoping logic FinalizeEpic applies once handed that
// set, independent of how it was computed.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

// plantBeadWorktree creates branch (from main) with a real worktree at
// dir/.wt-<name>, containing one commit that writes a distinctly-named
// file. Returns the worktree path. Mirrors the pattern used throughout
// merge_conflict_test.go / finalize_worktree_only_test.go.
func plantBeadWorktree(t *testing.T, dir, branch, fileName string) string {
	t.Helper()
	runGitIn(t, dir, "branch", branch, "main")
	wt := filepath.Join(dir, ".wt-"+filepath.Base(branch))
	if err := os.MkdirAll(filepath.Dir(wt), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", wt, branch)
	if err := os.WriteFile(filepath.Join(wt, fileName), []byte("content of "+fileName+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", fileName, err)
	}
	runGitIn(t, wt, "add", ".")
	runGitIn(t, wt, "commit", "-m", "commit for "+branch)
	return wt
}

// TestFinalizeEpic_ScopesBothLegsToLifecycleAllowSet is the AC-13 four-plant
// fixture: only the allow-set's lifecycle bead is merged, worktree-removed,
// and branch-deleted; a foreign-epic in_progress bead, a closed orphan
// branch of another spec, an OPEN non-lifecycle child of the SAME epic, and
// a CLOSED non-lifecycle child of the SAME epic all survive BOTH legs
// (not merged, not worktree-removed, not branch-deleted) — proving the
// exclusion boundary is lifecycle identity (the allow-set membership),
// never open-status or same-epic parentage alone.
func TestFinalizeEpic_ScopesBothLegsToLifecycleAllowSet(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/119-scope")

	// A REAL spec worktree, at the conventional path FinalizeEpic's
	// bead-merge leg reads (workspace.SpecWorktreePath), is required for
	// the auto-merge leg to run at all — mirroring
	// merge_conflict_test.go's setupConflictingSpecAndBead.
	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-119-scope")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/119-scope")

	lifecycleWt := plantBeadWorktree(t, dir, "bead/mindspec-scope.1", "lifecycle.txt")
	foreignWt := plantBeadWorktree(t, dir, "bead/mindspec-other.1", "foreign.txt")
	otherSpecWt := plantBeadWorktree(t, dir, "bead/mindspec-legacy.1", "otherspec.txt")
	openFollowupWt := plantBeadWorktree(t, dir, "bead/mindspec-scope-bug.1", "openbug.txt")
	closedFollowupWt := plantBeadWorktree(t, dir, "bead/mindspec-scope-chore.1", "closedchore.txt")

	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-scope.1", Path: lifecycleWt, Branch: "bead/mindspec-scope.1"},
		{Name: "worktree-mindspec-other.1", Path: foreignWt, Branch: "bead/mindspec-other.1"},
		{Name: "worktree-mindspec-legacy.1", Path: otherSpecWt, Branch: "bead/mindspec-legacy.1"},
		{Name: "worktree-mindspec-scope-bug.1", Path: openFollowupWt, Branch: "bead/mindspec-scope-bug.1"},
		{Name: "worktree-mindspec-scope-chore.1", Path: closedFollowupWt, Branch: "bead/mindspec-scope-chore.1"},
	}
	// onRemove reifies the real `git worktree remove` a live `bd worktree
	// remove` would perform — os.RemoveAll alone leaves git's internal
	// worktree registration behind, which would then block the spec
	// branch's own DeleteBranch below.
	fake.onRemove = func(name string) {
		p := map[string]string{
			"worktree-mindspec-scope.1": lifecycleWt,
			"worktree-spec-119-scope":   specWtPath,
		}[name]
		if p != "" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", p).Run()
		}
	}

	result, err := g.FinalizeEpic("epic-1", "119-scope", "spec/119-scope", []string{"mindspec-scope.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeStrategy != "direct" {
		t.Fatalf("MergeStrategy = %q, want direct", result.MergeStrategy)
	}

	// The lifecycle bead's work reached main (merged into the spec branch,
	// which then direct-merged into main); its branch is gone.
	if _, statErr := os.Stat(filepath.Join(dir, "lifecycle.txt")); statErr != nil {
		t.Errorf("lifecycle bead's content must reach main: %v", statErr)
	}
	if branchExistsIn(t, dir, "bead/mindspec-scope.1") {
		t.Error("lifecycle bead branch must be deleted after finalize")
	}
	if _, statErr := os.Stat(lifecycleWt); !os.IsNotExist(statErr) {
		t.Error("lifecycle bead worktree must be removed after finalize")
	}

	// All four non-lifecycle-scope candidates survive BOTH legs: content
	// never reaches main, branches survive, worktrees survive.
	survivors := map[string]string{
		"foreign.txt":     "bead/mindspec-other.1",
		"otherspec.txt":   "bead/mindspec-legacy.1",
		"openbug.txt":     "bead/mindspec-scope-bug.1",
		"closedchore.txt": "bead/mindspec-scope-chore.1",
	}
	survivorWts := map[string]string{
		"bead/mindspec-other.1":       foreignWt,
		"bead/mindspec-legacy.1":      otherSpecWt,
		"bead/mindspec-scope-bug.1":   openFollowupWt,
		"bead/mindspec-scope-chore.1": closedFollowupWt,
	}
	for file, branch := range survivors {
		if _, statErr := os.Stat(filepath.Join(dir, file)); !os.IsNotExist(statErr) {
			t.Errorf("%s's content (branch %s) must NEVER reach main", file, branch)
		}
		if !branchExistsIn(t, dir, branch) {
			t.Errorf("branch %s must survive (out of lifecycle scope)", branch)
		}
	}
	for branch, wt := range survivorWts {
		if _, statErr := os.Stat(wt); statErr != nil {
			t.Errorf("worktree for %s must survive (out of lifecycle scope): %v", branch, statErr)
		}
	}
}

// TestFinalizeEpic_NilAllowSetAbortsOnBeadCandidate pins the fail-closed
// "not computed" sentinel (Step 2 / AC-14): a nil lifecycleAllowSet
// alongside a real bead/<id> worktree candidate must abort the finalize
// with a named error, never silently skip the leg or silently admit the
// candidate.
func TestFinalizeEpic_NilAllowSetAbortsOnBeadCandidate(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/119-scope")
	wt := plantBeadWorktree(t, dir, "bead/mindspec-scope.1", "lifecycle.txt")

	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-scope.1", Path: wt, Branch: "bead/mindspec-scope.1"},
	}

	specHashBefore := refHash(t, dir, "spec/119-scope")

	_, err := g.FinalizeEpic("epic-1", "119-scope", "spec/119-scope", nil)
	if err == nil {
		t.Fatal("expected an error when lifecycleAllowSet is nil with a bead candidate present, got nil")
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("no worktree removal may happen on the nil-allow-set abort; got %v", fake.removeCalls)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-scope.1") {
		t.Error("bead branch must be preserved on the nil-allow-set abort")
	}
	if got := refHash(t, dir, "spec/119-scope"); got != specHashBefore {
		t.Errorf("spec branch must be unchanged on the nil-allow-set abort; was %s, now %s", specHashBefore, got)
	}
}

// TestFinalizeEpic_WorktreeListErrorAbortsBeforeAnyMutation pins AC-14's
// second leg: a WorktreeOps.List() failure during the auto-merge
// enumeration aborts the ENTIRE finalize before any merge or removal —
// today's `if listErr == nil` silently skipped the whole leg instead.
func TestFinalizeEpic_WorktreeListErrorAbortsBeforeAnyMutation(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "spec/119-scope")
	wt := plantBeadWorktree(t, dir, "bead/mindspec-scope.1", "lifecycle.txt")
	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-scope.1", Path: wt, Branch: "bead/mindspec-scope.1"},
	}
	fake.listErr = errors.New("simulated WorktreeOps.List failure")

	specHashBefore := refHash(t, dir, "spec/119-scope")

	_, err := g.FinalizeEpic("epic-1", "119-scope", "spec/119-scope", []string{"mindspec-scope.1"})
	if err == nil {
		t.Fatal("expected an error when WorktreeOps.List fails, got nil")
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("no worktree removal may happen when List() fails; got %v", fake.removeCalls)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-scope.1") {
		t.Error("bead branch must be preserved when List() fails")
	}
	if got := refHash(t, dir, "spec/119-scope"); got != specHashBefore {
		t.Errorf("spec branch must be unchanged when List() fails; was %s, now %s", specHashBefore, got)
	}
}
