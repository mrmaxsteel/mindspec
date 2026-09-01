package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepoCfg creates a git repo with a committed file and a configured
// identity (so commits work without per-call env) and returns its path.
func initGitRepoCfg(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

func TestGitMv_PreservesHistory(t *testing.T) {
	repo := initGitRepoCfg(t)
	sub := filepath.Join(repo, "old")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitPaths(repo, "add old/a.txt", []string{"old/a.txt"}); err != nil {
		t.Fatalf("CommitPaths: %v", err)
	}

	if err := GitMv(repo, "old", "new"); err != nil {
		t.Fatalf("GitMv: %v", err)
	}
	if err := CommitPaths(repo, "move old -> new", nil); err != nil {
		t.Fatalf("CommitPaths (rename): %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "new", "a.txt")); err != nil {
		t.Errorf("new/a.txt missing after GitMv: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "old")); err == nil {
		t.Error("old/ should be gone after GitMv")
	}
	// The rename commit is a pure 100% rename → log --follow survives.
	cmd := exec.Command("git", "-C", repo, "log", "--follow", "--format=%s", "--", "new/a.txt")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log --follow: %s", out)
	}
	if !strings.Contains(string(out), "add old/a.txt") {
		t.Errorf("log --follow did not survive the rename:\n%s", out)
	}
}

func TestResetHardAndCleanForce(t *testing.T) {
	repo := initGitRepoCfg(t)
	head, err := RevParseHEAD(repo)
	if err != nil {
		t.Fatal(err)
	}

	// Create a commit and an untracked file, then roll back.
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitPaths(repo, "add tracked", []string{"tracked.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ResetHard(repo, head); err != nil {
		t.Fatalf("ResetHard: %v", err)
	}
	if err := CleanForce(repo); err != nil {
		t.Fatalf("CleanForce: %v", err)
	}
	if now, _ := RevParseHEAD(repo); now != head {
		t.Errorf("ResetHard did not restore HEAD: %s != %s", now, head)
	}
	if _, err := os.Stat(filepath.Join(repo, "tracked.txt")); err == nil {
		t.Error("tracked.txt should be gone after ResetHard")
	}
	if _, err := os.Stat(filepath.Join(repo, "untracked.txt")); err == nil {
		t.Error("untracked.txt should be removed by CleanForce")
	}
}

// TestCleanForcePaths_ScopedToRoots asserts the SCOPED clean removes untracked
// residue only UNDER the given pathspecs and leaves user-untracked files
// OUTSIDE the move set in place (the mover's rollback safety property).
func TestCleanForcePaths_ScopedToRoots(t *testing.T) {
	repo := initGitRepoCfg(t)
	// Untracked residue inside a touched root, and an untracked user file
	// outside it.
	if err := os.MkdirAll(filepath.Join(repo, ".mindspec", "migrations", "run-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".mindspec", "migrations", "run-1", "state.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "user-scratch.txt"), []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CleanForcePaths(repo, []string{".mindspec", "project-docs", "review"}); err != nil {
		t.Fatalf("CleanForcePaths: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, ".mindspec", "migrations")); err == nil {
		t.Error(".mindspec residue should have been cleaned")
	}
	if _, err := os.Stat(filepath.Join(repo, "user-scratch.txt")); err != nil {
		t.Errorf("untracked user file OUTSIDE the scoped roots must be preserved: %v", err)
	}
}

func TestCommitPaths_NoopWhenNothingStaged(t *testing.T) {
	repo := initGitRepoCfg(t)
	before, _ := RevParseHEAD(repo)
	if err := CommitPaths(repo, "empty", nil); err != nil {
		t.Fatalf("CommitPaths empty: %v", err)
	}
	if after, _ := RevParseHEAD(repo); after != before {
		t.Errorf("CommitPaths created a commit with nothing staged: %s -> %s", before, after)
	}
}

// TestCommitPaths_ScopedCommitLeavesUnrelatedStagedContentAlone is
// G1-B3-01 (spec 127 bead 3 fix round): when OTHER content is already
// staged before CommitPaths runs, the commit it makes must include ONLY
// the given paths — the pre-existing staged content must remain staged
// (untouched, uncommitted) afterward, not silently swept into this
// commit just because it shares the same index. This is the property
// the adopt surface's finalize-export commit depends on to never sweep
// an operator's unrelated dirty/staged work into a terminal transition.
func TestCommitPaths_ScopedCommitLeavesUnrelatedStagedContentAlone(t *testing.T) {
	repo := initGitRepoCfg(t)

	// Something else already staged BEFORE CommitPaths ever runs.
	if err := os.WriteFile(filepath.Join(repo, "unrelated.txt"), []byte("unrelated work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	addCmd := exec.Command("git", "-C", repo, "add", "unrelated.txt")
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add unrelated.txt: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(repo, "export.txt"), []byte("export content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitPaths(repo, "export only", []string{"export.txt"}); err != nil {
		t.Fatalf("CommitPaths: %v", err)
	}

	// The commit must touch ONLY export.txt.
	showCmd := exec.Command("git", "-C", repo, "show", "--name-only", "--format=", "HEAD")
	out, err := showCmd.Output()
	if err != nil {
		t.Fatalf("git show: %v", err)
	}
	changed := strings.Fields(strings.TrimSpace(string(out)))
	if len(changed) != 1 || changed[0] != "export.txt" {
		t.Fatalf("commit must touch only export.txt, got %v", changed)
	}

	// unrelated.txt must remain STAGED but uncommitted.
	statusCmd := exec.Command("git", "-C", repo, "status", "--porcelain")
	statusOut, err := statusCmd.Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if !strings.Contains(string(statusOut), "A  unrelated.txt") {
		t.Fatalf("expected unrelated.txt to remain staged (git status --porcelain):\n%s", statusOut)
	}

	// A second call with nothing NEW at export.txt (but unrelated.txt
	// still staged) must be a true no-op — scoped to paths, not the
	// whole index.
	beforeSHA, _ := RevParseHEAD(repo)
	if err := CommitPaths(repo, "export only again", []string{"export.txt"}); err != nil {
		t.Fatalf("CommitPaths (idempotent resume): %v", err)
	}
	if afterSHA, _ := RevParseHEAD(repo); afterSHA != beforeSHA {
		t.Errorf("CommitPaths must be a no-op when the given paths have nothing new staged, even with unrelated content staged elsewhere: %s -> %s", beforeSHA, afterSHA)
	}
}

func TestLocalAndRemoteTrackingRefs(t *testing.T) {
	repo := initGitRepoCfg(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("branch", "spec/106")

	// Simulate a remote-tracking ref by writing the packed/loose ref directly.
	refDir := filepath.Join(repo, ".git", "refs", "remotes", "origin")
	if err := os.MkdirAll(refDir, 0o755); err != nil {
		t.Fatal(err)
	}
	head, _ := RevParseHEAD(repo)
	if err := os.WriteFile(filepath.Join(refDir, "main"), []byte(head+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	locals, err := LocalBranchRefs(repo)
	if err != nil {
		t.Fatalf("LocalBranchRefs: %v", err)
	}
	if !contains(locals, "main") || !contains(locals, "spec/106") {
		t.Errorf("LocalBranchRefs missing branches: %v", locals)
	}

	remotes, err := RemoteTrackingRefs(repo)
	if err != nil {
		t.Fatalf("RemoteTrackingRefs: %v", err)
	}
	if !contains(remotes, "origin/main") {
		t.Errorf("RemoteTrackingRefs missing origin/main: %v", remotes)
	}
}

func TestLockedWorktreeBranches(t *testing.T) {
	repo := initGitRepoCfg(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	wt := filepath.Join(t.TempDir(), "wt-locked")
	run("worktree", "add", "-b", "agent/locked", wt)
	run("worktree", "lock", wt)

	branches, err := LockedWorktreeBranches(repo)
	if err != nil {
		t.Fatalf("LockedWorktreeBranches: %v", err)
	}
	if !contains(branches, "agent/locked") {
		t.Errorf("expected agent/locked among locked worktree branches, got %v", branches)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
