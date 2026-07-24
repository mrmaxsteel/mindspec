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

// porcelainStatus returns the trimmed `git status --porcelain` output at
// dir — the byte-state behind "main is clean" / "back to the pre-merge
// state" (Bead 9 punch-list B15).
func porcelainStatus(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git status --porcelain in %s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

// assertConflictedFilesListing asserts the structured `conflicted
// files:` listing independently of git's raw stderr (Bead 9 punch-list
// B14): the literal header AND the conflicted path on its own indented
// line — a bare Contains("c.txt") is satisfied by the raw merge stderr
// that mergeErr wraps.
func assertConflictedFilesListing(t *testing.T, msg, path string) {
	t.Helper()
	if !strings.Contains(msg, "conflicted files:") {
		t.Errorf("error must carry the structured `conflicted files:` header; got:\n%s", msg)
	}
	if !strings.Contains(msg, "\nconflicted files:\n  "+path) {
		t.Errorf("conflicted path %q must be listed on its own indented line under the header; got:\n%s", path, msg)
	}
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
	// B14: the STRUCTURED listing, not just git's raw stderr.
	assertConflictedFilesListing(t, msg, "c.txt")
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
	// Spec 125 R5/AC-1b: the printed recovery merge line now supplies
	// `-m "Merge <beadBranch>"` so an operator following it verbatim
	// produces an IDENTIFIABLE exact subject too — belt (second-parent
	// identity, subject-independent) and suspenders (an identifiable
	// recovery subject).
	if !strings.Contains(msg, `recovery: git merge --no-ff -m "Merge bead/mindspec-x.1" bead/mindspec-x.1`) {
		t.Errorf("recovery should print an identifiable exact-subject merge; got:\n%s", msg)
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
	// B15: the abort site is byte-clean — no residual staged/unstaged
	// changes survive the `git merge --abort`.
	if got := porcelainStatus(t, specWtPath); got != "" {
		t.Errorf("spec worktree must be byte-clean after abort; git status --porcelain:\n%s", got)
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

	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"mindspec-x.1"})
	if err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}
	msg := err.Error()

	if !strings.Contains(msg, "c.txt") {
		t.Errorf("error should name the conflicted file c.txt; got:\n%s", msg)
	}
	// B14: the STRUCTURED listing, not just git's raw stderr.
	assertConflictedFilesListing(t, msg, "c.txt")
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
	// B15: the abort site is byte-clean after the abort.
	if got := porcelainStatus(t, specWtPath); got != "" {
		t.Errorf("spec worktree must be byte-clean after abort; git status --porcelain:\n%s", got)
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

	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err == nil {
		t.Fatal("expected a merge-conflict error, got nil")
	}
	msg := err.Error()

	if !strings.Contains(msg, "c.txt") {
		t.Errorf("error should name the conflicted file c.txt; got:\n%s", msg)
	}
	// B14: the STRUCTURED listing, not just git's raw stderr.
	assertConflictedFilesListing(t, msg, "c.txt")
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
	// B15: "main is clean" as a byte-state — the abort site (the root
	// checkout on main) has an empty porcelain status.
	if got := porcelainStatus(t, dir); got != "" {
		t.Errorf("main must be byte-clean after abort; git status --porcelain:\n%s", got)
	}
}

// TestFinalizeEpic_PartialBeadMergeMatrix pins the partial-merge matrix
// of the FinalizeEpic auto-merge loop (Bead 9 punch-list B13): bead A
// merges cleanly, bead B conflicts. Post-failure: A's merge IS on the
// spec branch (its branch is an ancestor), B's branch + worktree are
// preserved, main is untouched — and after the operator resolves B's
// conflict in the spec worktree, the re-run CONVERGES to a successful
// finalize (direct merge to main, branches cleaned up).
func TestFinalizeEpic_PartialBeadMergeMatrix(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	specWtPath, beadWtDir := setupConflictingSpecAndBead(t, dir)

	// Bead A: branches from main's root commit and touches a.txt only —
	// merges into the spec branch cleanly.
	runGitIn(t, dir, "branch", "bead/mindspec-a.1", "main")
	beadAWt := filepath.Join(dir, ".wt-bead-a1")
	runGitIn(t, dir, "worktree", "add", beadAWt, "bead/mindspec-a.1")
	t.Cleanup(func() {
		_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadAWt).Run()
	})
	if err := os.WriteFile(filepath.Join(beadAWt, "a.txt"), []byte("bead A\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, beadAWt, "add", ".")
	runGitIn(t, beadAWt, "commit", "-m", "bead A change")

	// A first, B second: the loop merges A cleanly, then aborts on B.
	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-mindspec-a.1", Path: beadAWt, Branch: "bead/mindspec-a.1"},
		{Name: "worktree-mindspec-x.1", Path: beadWtDir, Branch: "bead/mindspec-x.1"},
	}
	// Make fake removals real so the convergence re-run can delete the
	// branches the worktrees pin (the executor_test.go onRemove pattern).
	fake.onRemove = func(name string) {
		p := map[string]string{
			"worktree-mindspec-a.1":  beadAWt,
			"worktree-mindspec-x.1":  beadWtDir,
			"worktree-spec-077-test": specWtPath,
		}[name]
		if p != "" {
			_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", p).Run()
		}
	}

	mainHashBefore := refHash(t, dir, "main")

	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"mindspec-a.1", "mindspec-x.1"})
	if err == nil {
		t.Fatal("expected a merge-conflict error (bead B), got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bead/mindspec-x.1") {
		t.Errorf("error should name the conflicting bead branch; got:\n%s", msg)
	}

	// Bead A's merge survived the failure: its branch is an ancestor of
	// the spec branch (the merge commit landed before B aborted).
	if isAnc, ancErr := gitutil.IsAncestor(dir, "bead/mindspec-a.1", "spec/077-test"); ancErr != nil || !isAnc {
		t.Errorf("bead A must be merged into the spec branch post-failure (isAncestor=%v, err=%v)", isAnc, ancErr)
	}
	// Bead B preserved: branch + worktree.
	if !branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("bead B's branch must be preserved on conflict")
	}
	if _, statErr := os.Stat(beadWtDir); statErr != nil {
		t.Errorf("bead B's worktree must be preserved on conflict: %v", statErr)
	}
	// Main untouched, abort site clean, no removals ran.
	if got := refHash(t, dir, "main"); got != mainHashBefore {
		t.Errorf("main must be untouched; was %s, now %s", mainHashBefore, got)
	}
	if gitutil.MergeInProgress(specWtPath) {
		t.Error("the in-progress merge in the spec worktree must be aborted")
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("no worktree removal may happen on conflict; got removeCalls=%v", fake.removeCalls)
	}

	// Operator recovery (the failure's own recovery commands): re-run
	// the merge in the spec worktree, resolve, commit the merge.
	mergeCmd := exec.Command("git", "-C", specWtPath, "merge", "--no-ff", "bead/mindspec-x.1")
	_ = mergeCmd.Run() // conflicts again, leaves the merge in progress
	if err := os.WriteFile(filepath.Join(specWtPath, "c.txt"), []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	runGitIn(t, specWtPath, "add", ".")
	runGitIn(t, specWtPath, "commit", "-m", "merge bead/mindspec-x.1 (resolved)")

	// The re-run converges: both bead branches are ancestors now, so the
	// loop skips them and finalize completes (direct merge to main).
	result, rerunErr := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"mindspec-a.1", "mindspec-x.1"})
	if rerunErr != nil {
		t.Fatalf("re-run after conflict resolution must converge, got: %v", rerunErr)
	}
	if result.MergeStrategy != "direct" {
		t.Errorf("MergeStrategy = %q, want %q", result.MergeStrategy, "direct")
	}
	// Main now carries both beads' work.
	for path, want := range map[string]string{"a.txt": "bead A\n", "c.txt": "resolved\n"} {
		got, readErr := os.ReadFile(filepath.Join(dir, path))
		if readErr != nil {
			t.Errorf("post-convergence main is missing %s: %v", path, readErr)
			continue
		}
		if string(got) != want {
			t.Errorf("post-convergence %s = %q, want %q", path, got, want)
		}
	}
}

// TestAbortMergeState_NoMergeInProgress covers the defensive no-
// MERGE_HEAD early return (Bead 9 punch-list B16): with no in-progress
// merge there is nothing to abort — no conflicted files, empty note.
func TestAbortMergeState_NoMergeInProgress(t *testing.T) {
	dir := newTempRepo(t)

	conflicted, note := abortMergeState(dir)
	if len(conflicted) != 0 {
		t.Errorf("conflicted = %v, want empty (no merge in progress)", conflicted)
	}
	if note != "" {
		t.Errorf("note = %q, want empty (nothing was aborted)", note)
	}
}
