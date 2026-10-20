package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// blockingSpecBranchPreCommit mirrors the decision internal/hook's
// runPreCommit makes in implement mode: a commit on a spec/ branch is
// refused unless MINDSPEC_ALLOW_MAIN=1. The installed shim
// (`exec mindspec hook pre-commit`) needs a mindspec binary on PATH, so
// the decision is inlined here to keep the test hermetic.
const blockingSpecBranchPreCommit = `#!/bin/sh
[ "$MINDSPEC_ALLOW_MAIN" = "1" ] && exit 0
branch=$(git rev-parse --abbrev-ref HEAD)
case "$branch" in
  spec/*) echo "mindspec: commits on spec branch '$branch' are blocked during implement mode" >&2; exit 1 ;;
esac
exit 0
`

// TestCompleteBead_ResolveMerge_ConcludesUnderBlockingPreCommitHook:
// with a pre-commit hook that blocks spec-branch commits (implement
// mode) and no MINDSPEC_ALLOW_MAIN escape, `--resolve-merge` must still
// conclude the operator's resolved merge. The merge-conclusion commit is
// the tool's own, like a conflict-free `git merge`, which never fires
// pre-commit.
func TestCompleteBead_ResolveMerge_ConcludesUnderBlockingPreCommitHook(t *testing.T) {
	t.Setenv("MINDSPEC_ALLOW_MAIN", "")
	g, fake, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)

	hooksDir := filepath.Join(t.TempDir(), "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(blockingSpecBranchPreCommit), 0o755); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, dir, "config", "core.hooksPath", hooksDir)

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

	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", false); err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the conflict must be preserved after the first attempt")
	}

	// Sanity: the hook really blocks a plain commit on the spec branch.
	if err := os.WriteFile(filepath.Join(specWtPath, "c.txt"), []byte("resolved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, specWtPath, "add", "c.txt")
	probe := exec.Command("git", "-C", specWtPath, "commit", "--no-edit")
	probe.Env = append(os.Environ(), "MINDSPEC_ALLOW_MAIN=")
	if out, err := probe.CombinedOutput(); err == nil || !strings.Contains(string(out), "blocked during implement mode") {
		t.Fatalf("fixture hook must block a plain spec-branch commit; err=%v out=%s", err, out)
	}
	if !gitutil.MergeInProgress(specWtPath) {
		t.Fatal("the refused probe commit must leave the merge in progress")
	}

	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", "", "", true); err != nil {
		t.Fatalf("--resolve-merge must conclude the merge despite the pre-commit hook, got: %v", err)
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("the merge must be complete — no MERGE_HEAD should remain")
	}
	if got, err := os.ReadFile(filepath.Join(specWtPath, "c.txt")); err != nil || string(got) != "resolved\n" {
		t.Errorf("spec branch tip must carry the resolved content; got %q, err=%v", got, err)
	}
}
