package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// TestRelease_RemoveBeadWorktreeAndRestore_RecoversCwd is the cwd-safety AC for
// Spec 101 R2 (the spec-092 Req 3c / mindspec-qxsy bug class). It uses a REAL
// MindspecExecutor against a real temp git repo (newRepoExecutor) and a fake
// WorktreeOps whose onRemove ACTUALLY deletes the bead worktree directory —
// only real deletion can exercise the footgun (the recording fake by itself
// never deletes, so it structurally cannot catch a deleted-cwd regression).
//
// The process cwd is set INSIDE the bead worktree before the call. After
// RemoveBeadWorktreeAndRestore the process MUST be back at the repo root (never
// stranded in the deleted directory) so that any subsequent bd/git subprocess
// runs from a live cwd.
func TestRelease_RemoveBeadWorktreeAndRestore_RecoversCwd(t *testing.T) {
	g, fake, root := newRepoExecutor(t)

	beadID := "mindspec-3cj0"
	// Build a real on-disk directory standing in for the bead worktree, at the
	// path BeadWorktreeName resolves under the repo's worktrees dir.
	beadWtName, err := workspace.BeadWorktreeName(beadID)
	if err != nil {
		t.Fatalf("BeadWorktreeName: %v", err)
	}
	wtDir := filepath.Join(workspace.DefaultWorktreesDir(root), beadWtName)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatalf("mkdir bead worktree: %v", err)
	}

	// The fake's Remove reifies the real `bd worktree remove` side effect:
	// it deletes the directory. This is what makes the cwd-recovery footgun
	// reachable in the test.
	fake.onRemove = func(name string) {
		_ = os.RemoveAll(wtDir)
	}

	// Sit the process INSIDE the bead worktree — the natural place an agent
	// realizes it mis-claimed and runs `mindspec release`.
	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(wtDir); err != nil {
		t.Fatalf("chdir into bead worktree: %v", err)
	}

	if err := g.RemoveBeadWorktreeAndRestore(beadID); err != nil {
		t.Fatalf("RemoveBeadWorktreeAndRestore returned error: %v", err)
	}

	// Remove must have been routed through the WorktreeOps seam (ADR-0030),
	// for the canonical bead worktree name.
	if len(fake.removeCalls) != 1 || fake.removeCalls[0] != beadWtName {
		t.Fatalf("Remove calls = %v, want exactly [%s]", fake.removeCalls, beadWtName)
	}

	// cwd MUST have recovered to the repo root — never the deleted worktree.
	wd, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatalf("process left in unresolvable cwd after removal: %v", wdErr)
	}
	realWd, _ := filepath.EvalSymlinks(wd)
	realRoot, _ := filepath.EvalSymlinks(root)
	if realWd != realRoot {
		t.Errorf("cwd after removal: got %q, want repo root %q", wd, root)
	}

	// A post-removal filesystem read from the recovered cwd must succeed —
	// proving the process is no longer stranded in a deleted directory (the
	// stand-in for a post-removal bd read that would otherwise degrade).
	if _, err := os.Stat("."); err != nil {
		t.Errorf("post-removal stat of cwd failed (stranded in deleted dir?): %v", err)
	}
}
