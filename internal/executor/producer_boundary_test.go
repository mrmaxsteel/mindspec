package executor

// Spec 127 bead-6 fix round 1 (O3-2): a real-git, PRODUCER-level (through
// production CompleteBead, not just the predicate directly) confirmation
// that the large-rename and honest-cleanup shapes complete cleanly with
// no refusal and no --allow-net-deletion needed.
//
// Provenance note (see this bead's fix-round report): spec.md line 155
// and plan.md's own Provenance table both assign AC-8(ii)/(iii) delivery
// to BEAD 1 ("with AC-8(ii)/(iii) as its own panel's fixtures" — at the
// PREDICATE level, internal/gitutil/workdestruction_test.go's
// wdLargeRenameFixture/wdHonestCleanupFixture, both already asserting
// guard.DestructionClean). preflightMergeDestruction (merge_preflight.go)
// adds NO secondary check beyond a direct pass-through of that same
// predicate — Clean is one of exactly two unconditionally-permissive
// outcomes in its switch, with no numeric/other tripwire layered on top
// — so there is no additional producer-level risk bead 6 introduces for
// these two shapes. These fixtures exist to remove all residual doubt
// with a real, producer-level demonstration, not because a live gap was
// found.

import (
	"os"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// TestCompleteBead_LargeRenameCompletesCleanly is the producer-level
// analog of gitutil's own wdLargeRenameFixture (AC-8(ii)): a bead branch
// that renames a whole directory (no net content loss) merges into the
// spec branch cleanly, through the REAL preflightMergeDestruction call,
// with no refusal and no override.
func TestCompleteBead_LargeRenameCompletesCleanly(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	runGitIn(t, dir, "branch", "spec/077-rename")
	specWtPath := dir + "/.worktrees/worktree-spec-077-rename"
	if err := os.MkdirAll(dir+"/.worktrees", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/077-rename")
	body := "content line one\nmore lines here for similarity\nline three\nline four\n"
	for _, n := range []string{"1", "2", "3", "4", "5"} {
		if err := os.MkdirAll(specWtPath+"/dir", 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(specWtPath+"/dir/f"+n+".txt", []byte(body+n+"\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	runGitIn(t, specWtPath, "add", ".")
	runGitIn(t, specWtPath, "commit", "-m", "add dir/")

	beadWtDir := dir + "/.wt-bead-rename1"
	runGitIn(t, dir, "branch", "bead/mindspec-rename.1", "spec/077-rename")
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-rename.1")
	runGitIn(t, beadWtDir, "mv", "dir", "newdir")
	runGitIn(t, beadWtDir, "commit", "-m", "move dir -> newdir")

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-rename.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-rename.1",
	}}

	if err := g.CompleteBead("mindspec-rename.1", "spec/077-rename", "", "", false); err != nil {
		t.Fatalf("AC-8(ii) producer leg: a large rename with no net content loss must complete cleanly, no --allow-net-deletion needed, got: %v", err)
	}
	if isAnc, ancErr := gitutil.IsAncestor(dir, "bead/mindspec-rename.1", "spec/077-rename"); ancErr != nil || !isAnc {
		t.Errorf("bead branch must be merged into spec/077-rename, IsAncestor=%v err=%v", isAnc, ancErr)
	}
}

// TestCompleteBead_HonestCleanupCompletesCleanly is the producer-level
// analog of gitutil's own wdHonestCleanupFixture (AC-8(iii)): a bead
// branch whose OWN authored range deletes a file (an honest cleanup, not
// a staleness artifact) merges into the spec branch cleanly, with no
// refusal and no override.
func TestCompleteBead_HonestCleanupCompletesCleanly(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	runGitIn(t, dir, "branch", "spec/077-cleanup")
	specWtPath := dir + "/.worktrees/worktree-spec-077-cleanup"
	if err := os.MkdirAll(dir+"/.worktrees", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/077-cleanup")
	if err := os.WriteFile(specWtPath+"/keep.txt", []byte("keep\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(specWtPath+"/obsolete.txt", []byte("obsolete\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, specWtPath, "add", ".")
	runGitIn(t, specWtPath, "commit", "-m", "spec side: keep + obsolete")

	beadWtDir := dir + "/.wt-bead-cleanup1"
	runGitIn(t, dir, "branch", "bead/mindspec-cleanup.1", "spec/077-cleanup")
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-cleanup.1")
	runGitIn(t, beadWtDir, "rm", "obsolete.txt")
	runGitIn(t, beadWtDir, "commit", "-m", "cleanup: remove obsolete.txt (honest, own authored range)")

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-cleanup.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-cleanup.1",
	}}

	if err := g.CompleteBead("mindspec-cleanup.1", "spec/077-cleanup", "", "", false); err != nil {
		t.Fatalf("AC-8(iii) producer leg: an honest cleanup within the bead's own authored range must complete cleanly, no --allow-net-deletion needed, got: %v", err)
	}
	if isAnc, ancErr := gitutil.IsAncestor(dir, "bead/mindspec-cleanup.1", "spec/077-cleanup"); ancErr != nil || !isAnc {
		t.Errorf("bead branch must be merged into spec/077-cleanup, IsAncestor=%v err=%v", isAnc, ancErr)
	}
	if _, statErr := os.Stat(specWtPath + "/obsolete.txt"); !os.IsNotExist(statErr) {
		t.Error("obsolete.txt must be gone from the spec branch after the honest cleanup merges")
	}
}
