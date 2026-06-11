package executor

// Spec 092 Bead 6 — merge-conflict hardening fixtures (Reqs 14, 18 +
// the 2026-06-11 incident amendment extending Req 14(a) to
// CompleteBead's MergeInto site).
//
// Three forced-conflict sites, each asserting the POST-ABORT state, the
// conflicted-file naming, and the Req 12 final-recovery-line contract
// (guard.HasFinalRecoveryLine) per site:
//
//   1. CompleteBead bead→spec merge (the site that fired in the
//      incident — previously downgraded to a stderr warning).
//   2. FinalizeEpic bead→spec auto-merge (previously warn-and-continue
//      into worktree removal + direct merge + branch deletion).
//   3. FinalizeEpic direct spec→main merge (Req 18: previously
//      warn-then-DeleteBranch, destroying the recovery source).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// setupConflictingSpecAndBead builds, in the repo at dir:
//   - spec/077-test with a spec worktree at
//     .worktrees/worktree-spec-077-test carrying a commit that writes
//     c.txt with "spec side",
//   - bead/mindspec-x.1 with a bead worktree at .wt-bead-x1 carrying a
//     commit that writes c.txt with "bead side".
//
// Both branch from main's root commit, so merging the bead branch into
// the spec branch conflicts on c.txt.
func setupConflictingSpecAndBead(t *testing.T, dir string) (specWtPath, beadWtDir string) {
	t.Helper()

	runGitIn(t, dir, "branch", "spec/077-test")
	runGitIn(t, dir, "branch", "bead/mindspec-x.1")

	specWtPath = filepath.Join(dir, ".worktrees", "worktree-spec-077-test")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/077-test")
	t.Cleanup(func() {
		_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run()
	})
	if err := os.WriteFile(filepath.Join(specWtPath, "c.txt"), []byte("spec side\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, specWtPath, "add", ".")
	runGitIn(t, specWtPath, "commit", "-m", "spec change")

	beadWtDir = filepath.Join(dir, ".wt-bead-x1")
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-x.1")
	t.Cleanup(func() {
		_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
	})
	if err := os.WriteFile(filepath.Join(beadWtDir, "c.txt"), []byte("bead side\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, beadWtDir, "add", ".")
	runGitIn(t, beadWtDir, "commit", "-m", "bead change")

	return specWtPath, beadWtDir
}

// TestCompleteBead_MergeConflictAbortsAndPreserves pins the incident
// amendment: a bead→spec merge failure in CompleteBead must abort the
// in-progress merge in the spec worktree, preserve the bead branch +
// bead worktree (no cleanup), and return a non-zero guard failure
// naming the conflicted files with resolve-in-spec-worktree recovery —
// never the old warn-and-continue.
func TestCompleteBead_MergeConflictAbortsAndPreserves(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-x.1",
	}}

	specHashBefore := refHash(t, dir, "spec/077-test")

	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "")
	if err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}
	msg := err.Error()

	// Names the conflicted file.
	if !strings.Contains(msg, "c.txt") {
		t.Errorf("error should name the conflicted file c.txt; got:\n%s", msg)
	}
	// Req 12: final recovery line (per-site HasFinalRecoveryLine test).
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("error must end with a `recovery:` line; got:\n%s", msg)
	}
	// Resolve-in-spec-worktree recovery: the spec worktree (which still
	// exists) and the converging re-run command are both named.
	if !strings.Contains(msg, specWtPath) {
		t.Errorf("recovery should reference the spec worktree %s; got:\n%s", specWtPath, msg)
	}
	if !strings.Contains(msg, "recovery: mindspec complete mindspec-x.1") {
		t.Errorf("recovery should include the converging re-run `mindspec complete mindspec-x.1`; got:\n%s", msg)
	}

	// No warn-and-continue cleanup: bead worktree + branch preserved.
	if len(fake.removeCalls) != 0 {
		t.Errorf("no worktree removal may happen on conflict; got removeCalls=%v", fake.removeCalls)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("bead branch must be preserved on conflict")
	}
	if _, statErr := os.Stat(beadWtDir); statErr != nil {
		t.Errorf("bead worktree must be preserved on conflict: %v", statErr)
	}

	// The in-progress merge was aborted: spec worktree clean, spec
	// branch unchanged.
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("the in-progress merge in the spec worktree must be aborted")
	}
	if got := refHash(t, dir, "spec/077-test"); got != specHashBefore {
		t.Errorf("spec branch must be unchanged after abort; was %s, now %s", specHashBefore, got)
	}
}

// TestFinalizeEpic_BeadMergeConflictAbortsFinalize pins Req 14(a): a
// bead→spec conflict ABORTS FinalizeEpic — abort the in-progress merge,
// no worktree removal, no direct merge to main, no branch deletion,
// non-zero return — replacing the old warn-and-continue that merged to
// main without the conflicted bead's work and deleted the recovery
// target.
func TestFinalizeEpic_BeadMergeConflictAbortsFinalize(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-x.1",
	}}

	mainHashBefore := refHash(t, dir, "main")
	specHashBefore := refHash(t, dir, "spec/077-test")

	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test")
	if err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}
	msg := err.Error()

	if !strings.Contains(msg, "c.txt") {
		t.Errorf("error should name the conflicted file c.txt; got:\n%s", msg)
	}
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("error must end with a `recovery:` line; got:\n%s", msg)
	}
	// Recovery matches the post-abort reality: resolve in the spec
	// worktree (preserved by the abort), then re-run impl approve.
	if !strings.Contains(msg, specWtPath) {
		t.Errorf("recovery should reference the spec worktree %s; got:\n%s", specWtPath, msg)
	}
	if !strings.Contains(msg, "recovery: mindspec impl approve 077-test") {
		t.Errorf("recovery should include re-running `mindspec impl approve 077-test`; got:\n%s", msg)
	}

	// Post-abort state: spec worktree exists, both branches exist, main
	// untouched (no direct merge, no in-progress merge), no removals.
	if _, statErr := os.Stat(specWtPath); statErr != nil {
		t.Errorf("spec worktree must still exist: %v", statErr)
	}
	if !branchExistsIn(t, dir, "spec/077-test") {
		t.Error("spec branch must still exist")
	}
	if !branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("bead branch must still exist")
	}
	if got := refHash(t, dir, "main"); got != mainHashBefore {
		t.Errorf("main must be untouched; was %s, now %s", mainHashBefore, got)
	}
	if gitutil.MergeInProgress(dir) {
		t.Error("main must have no in-progress merge state")
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("the in-progress merge in the spec worktree must be aborted")
	}
	if got := refHash(t, dir, "spec/077-test"); got != specHashBefore {
		t.Errorf("spec branch must be unchanged after abort; was %s, now %s", specHashBefore, got)
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("no worktree removal may happen on conflict; got removeCalls=%v", fake.removeCalls)
	}
}

// TestFinalizeEpic_DirectMergeConflictPreservesSpecBranch pins
// Req 14(b) + Req 18: a direct spec→main conflict aborts the merge
// (main clean), SKIPS spec-branch deletion, returns non-zero, and the
// recovery is root-anchored — it references no worktree path (the spec
// worktree was already removed by this point in FinalizeEpic).
func TestFinalizeEpic_DirectMergeConflictPreservesSpecBranch(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	// Spec branch with one commit touching c.txt.
	runGitIn(t, dir, "checkout", "-b", "spec/077-test")
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("spec side\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "spec change")
	runGitIn(t, dir, "checkout", "main")

	// Conflicting commit on main.
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("main side\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "main change")

	fake.listEntries = nil // no bead worktrees
	mainHashBefore := refHash(t, dir, "main")

	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test")
	if err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}
	msg := err.Error()

	if !strings.Contains(msg, "c.txt") {
		t.Errorf("error should name the conflicted file c.txt; got:\n%s", msg)
	}
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("error must end with a `recovery:` line; got:\n%s", msg)
	}
	// Root-anchored recovery, no worktree references.
	if !strings.Contains(msg, "recovery: cd "+dir) {
		t.Errorf("recovery should operate at the repo root %s; got:\n%s", dir, msg)
	}
	if !strings.Contains(msg, "recovery: git merge --no-ff spec/077-test") {
		t.Errorf("recovery should re-run the merge at the root; got:\n%s", msg)
	}
	for _, banned := range []string{".worktrees", "worktree-spec-"} {
		if strings.Contains(msg, banned) {
			t.Errorf("direct-merge recovery must reference no worktree path; found %q in:\n%s", banned, msg)
		}
	}

	// Req 18 post-abort state: spec branch survives, main clean,
	// non-zero exit (err != nil above).
	if !branchExistsIn(t, dir, "spec/077-test") {
		t.Error("spec branch must survive a direct-merge conflict")
	}
	if gitutil.MergeInProgress(dir) {
		t.Error("main must have no in-progress merge state after the abort")
	}
	if got := refHash(t, dir, "main"); got != mainHashBefore {
		t.Errorf("main must be unchanged after abort; was %s, now %s", mainHashBefore, got)
	}
}
