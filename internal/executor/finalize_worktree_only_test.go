package executor

// Spec 115 Bead 1 (AC12, Fact 2): FinalizeEpic's bead→spec auto-merge loop
// keys ONLY off g.WorktreeOps.List() — the same production enumeration
// source `bd worktree list --json` backs (bdcli.go) — never off a raw scan
// of git refs named bead/<id>. TestFinalizeEpic_MergesOnlyWorktreeRealBranches
// pins this invariant (the blp6 constraint, spec Non-Goals) with real-git
// subtests, beside the finalize_orphan_test.go / merge_conflict_test.go
// conventions: a merge candidate must be BOTH a real bead/<id> branch AND
// enumerated by WorktreeOps.List() — anything else (an unenumerated branch
// ref, a tag shadowing a deleted branch surfaced as a detached worktree
// entry, or a symref so dangling `git worktree add` itself refuses it) must
// never be merged.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

func TestFinalizeEpic_MergesOnlyWorktreeRealBranches(t *testing.T) {
	// (a) An unmerged bead/<id> branch REF with NO worktree entry is never
	// merged. FinalizeEpic's bead-merge loop (mindspec_executor.go:383)
	// iterates g.WorktreeOps.List() — a branch whose worktree was already
	// removed (a prior `bd worktree remove`, or a crashed run) leaves no
	// entry for the loop to see, so the branch is left exactly alone: not
	// merged into the spec branch, not merged into main, not deleted.
	t.Run("unmerged bead branch ref with no worktree is never merged", func(t *testing.T) {
		g, fake, dir := newRepoExecutor(t)
		runGitIn(t, dir, "branch", "spec/115-ac12a")

		runGitIn(t, dir, "branch", "bead/orphan-ac12a.1", "main")
		beadWt := filepath.Join(dir, ".wt-orphan-ac12a-1")
		runGitIn(t, dir, "worktree", "add", beadWt, "bead/orphan-ac12a.1")
		if err := os.WriteFile(filepath.Join(beadWt, "orphan.txt"), []byte("orphan content\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		runGitIn(t, beadWt, "add", ".")
		runGitIn(t, beadWt, "commit", "-m", "orphan bead change")
		// Remove the WORKTREE only — the branch ref survives. WorktreeOps
		// (the fake) reports NO entry for it: exactly the enumeration gap
		// Fact 2 pins against a raw ref scan.
		runGitIn(t, dir, "worktree", "remove", "--force", beadWt)
		fake.listEntries = nil

		if _, err := g.FinalizeEpic("epic-1", "115-ac12a", "spec/115-ac12a"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !branchExistsIn(t, dir, "bead/orphan-ac12a.1") {
			t.Error("the unenumerated bead branch must be left alone (never cleaned up)")
		}
		// The spec branch itself is deleted by FinalizeEpic's own cleanup
		// once its (no-op, since it never received the orphan's commits)
		// direct merge into main lands — so the merge-prevention proof is
		// checked against main, the surviving ref, plus the tell-tale file
		// the orphan's commit alone would have introduced.
		if isAnc, ancErr := gitutil.IsAncestor(dir, "bead/orphan-ac12a.1", "main"); ancErr != nil || isAnc {
			t.Errorf("an unenumerated bead branch must NEVER reach main (isAncestor=%v, err=%v)", isAnc, ancErr)
		}
		if _, statErr := os.Stat(filepath.Join(dir, "orphan.txt")); !os.IsNotExist(statErr) {
			t.Error("the unenumerated bead branch's content must never be merged")
		}
	})

	// (b) A bead/<id> TAG shadowing a DELETED branch of the same name is
	// not enumerated as a bead/ branch. The worktree that was checked out
	// on the branch is detached before the branch is deleted (mirroring
	// how a real crashed/rebuilt worktree can end up pointing at a bare
	// commit once its branch is gone); the enumerated WorktreeListEntry for
	// a detached checkout carries an EMPTY Branch field (no branch line) —
	// the HasPrefix(e.Branch, "bead/") filter (mindspec_executor.go:385)
	// drops it regardless of what a same-named tag happens to point at.
	t.Run("a bead/<id> tag shadowing a deleted branch is not enumerated as a bead/ branch", func(t *testing.T) {
		g, fake, dir := newRepoExecutor(t)
		runGitIn(t, dir, "branch", "spec/115-ac12b")

		runGitIn(t, dir, "branch", "bead/tagshadow-ac12b.1", "main")
		wt := filepath.Join(dir, ".wt-tagshadow-ac12b-1")
		runGitIn(t, dir, "worktree", "add", wt, "bead/tagshadow-ac12b.1")
		t.Cleanup(func() {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", wt).Run()
		})
		if err := os.WriteFile(filepath.Join(wt, "shadow.txt"), []byte("shadow content\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		runGitIn(t, wt, "add", ".")
		runGitIn(t, wt, "commit", "-m", "shadow bead change")
		sha := refHash(t, dir, "bead/tagshadow-ac12b.1")

		// Detach the worktree from the branch, THEN delete the branch
		// (git refuses to delete a branch checked out in a worktree), THEN
		// tag the same commit under the branch's former name — the tag now
		// occupies the "bead/tagshadow-ac12b.1" name in a namespace no
		// branch claims any more.
		runGitIn(t, wt, "checkout", "--detach", sha)
		runGitIn(t, dir, "branch", "-D", "bead/tagshadow-ac12b.1")
		runGitIn(t, dir, "tag", "bead/tagshadow-ac12b.1", sha)

		if branchExistsIn(t, dir, "bead/tagshadow-ac12b.1") {
			t.Fatal("test setup: the branch must be gone, leaving only the tag")
		}

		// The enumerated entry mirrors a real detached `bd worktree list`
		// row: Branch is empty (no branch line) — never the tag's name.
		fake.listEntries = []bead.WorktreeListEntry{
			{Name: "worktree-tagshadow-ac12b-1", Path: wt, Branch: ""},
		}

		if _, err := g.FinalizeEpic("epic-1", "115-ac12b", "spec/115-ac12b"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// The tag-shadowed detached worktree's content must never reach
		// the spec branch or main: the filter keys off e.Branch, not any
		// ref-name guess.
		if _, statErr := os.Stat(filepath.Join(dir, "shadow.txt")); !os.IsNotExist(statErr) {
			t.Error("the tag-shadowed detached worktree's content must never be merged")
		}
	})

	// (c) A dangling bead/<id> symref cannot be `git worktree add`-ed at
	// all. This is a pure git-behavior pin (no FinalizeEpic call): it
	// demonstrates why WorktreeOps.List() (backed by `bd worktree list`,
	// itself backed by real `git worktree` state) can never surface an
	// entry for a broken ref in the first place — reinforcing that the
	// merge-prevention leg's enumeration source (real, checked-out
	// worktrees) is safe by construction.
	t.Run("a dangling bead/<id> symref cannot be worktree-added", func(t *testing.T) {
		_, _, dir := newRepoExecutor(t)

		symrefCmd := exec.Command("git", "-C", dir, "symbolic-ref",
			"refs/heads/bead/dangling-ac12c.1", "refs/heads/does-not-exist-ac12c")
		if out, err := symrefCmd.CombinedOutput(); err != nil {
			t.Fatalf("creating the dangling symref: %v\n%s", err, out)
		}

		addCmd := exec.Command("git", "-C", dir, "worktree", "add",
			filepath.Join(dir, ".wt-dangling-ac12c-1"), "bead/dangling-ac12c.1")
		if out, err := addCmd.CombinedOutput(); err == nil {
			t.Fatalf("expected `git worktree add` on a dangling symref to fail; it succeeded:\n%s", out)
		}
	})
}
