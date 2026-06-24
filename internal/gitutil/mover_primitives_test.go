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
