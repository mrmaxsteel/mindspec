package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignoreEntry_New(t *testing.T) {
	root := t.TempDir()

	if err := EnsureGitignoreEntry(root, ".worktrees"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}

	if !strings.Contains(string(data), ".worktrees/") {
		t.Errorf("expected .worktrees/ in .gitignore, got: %s", data)
	}
}

func TestEnsureGitignoreEntry_Idempotent(t *testing.T) {
	root := t.TempDir()

	// Write twice
	if err := EnsureGitignoreEntry(root, ".worktrees"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureGitignoreEntry(root, ".worktrees"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	count := strings.Count(string(data), ".worktrees/")
	if count != 1 {
		t.Errorf("expected exactly 1 .worktrees/ entry, got %d in:\n%s", count, data)
	}
}

// initGitRepo creates a git repo with an initial commit and returns the path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644)
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

func TestDiffStat(t *testing.T) {
	dir := initGitRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	// Create a feature branch with changes
	run("checkout", "-b", "feature")
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello\n"), 0644)
	run("add", ".")
	run("commit", "-m", "add new file")

	stat, err := DiffStat(dir, "main", "feature")
	if err != nil {
		t.Fatalf("DiffStat: %v", err)
	}
	if !strings.Contains(stat, "new.txt") {
		t.Errorf("expected new.txt in diffstat, got: %s", stat)
	}
}

func TestCommitCount(t *testing.T) {
	dir := initGitRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	run("checkout", "-b", "feature")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644)
	run("add", ".")
	run("commit", "-m", "commit 1")
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0644)
	run("add", ".")
	run("commit", "-m", "commit 2")

	count, err := CommitCount(dir, "main", "feature")
	if err != nil {
		t.Fatalf("CommitCount: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 commits, got %d", count)
	}
}

func TestCommitAll_CleanTree(t *testing.T) {
	dir := initGitRepo(t)

	// Clean tree — should be a no-op
	err := CommitAll(dir, "should not commit")
	if err != nil {
		t.Fatalf("unexpected error on clean tree: %v", err)
	}

	// Verify no new commit was created (still just the initial commit)
	cmd := exec.Command("git", "-C", dir, "rev-list", "--count", "HEAD")
	out, _ := cmd.Output()
	if strings.TrimSpace(string(out)) != "1" {
		t.Errorf("expected 1 commit, got %s", strings.TrimSpace(string(out)))
	}
}

func TestCommitAll_DirtyTree(t *testing.T) {
	dir := initGitRepo(t)

	// Set local git config so CommitAll's real git commands work in CI
	// (initGitRepo uses env vars for its helper, but CommitAll calls git directly).
	for _, kv := range [][2]string{{"user.name", "test"}, {"user.email", "test@test.com"}} {
		cmd := exec.Command("git", "-C", dir, "config", kv[0], kv[1])
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git config %s: %s", kv[0], out)
		}
	}

	// Create an untracked file
	os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0644)

	err := CommitAll(dir, "chore: auto-commit spec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the commit happened
	cmd := exec.Command("git", "-C", dir, "rev-list", "--count", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-list: %v", err)
	}
	if strings.TrimSpace(string(out)) != "2" {
		t.Errorf("expected 2 commits (initial + auto), got %s", strings.TrimSpace(string(out)))
	}

	// Verify the commit message
	cmd = exec.Command("git", "-C", dir, "log", "-1", "--format=%s")
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if strings.TrimSpace(string(out)) != "chore: auto-commit spec" {
		t.Errorf("unexpected commit message: %s", strings.TrimSpace(string(out)))
	}

	// Verify tree is clean after
	cmd = exec.Command("git", "-C", dir, "status", "--porcelain")
	out, _ = cmd.Output()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("tree should be clean after CommitAll, got: %s", out)
	}
}

func TestEnsureGitignoreEntry_ExistingFile(t *testing.T) {
	root := t.TempDir()
	gitignorePath := filepath.Join(root, ".gitignore")

	// Pre-existing content without trailing newline
	if err := os.WriteFile(gitignorePath, []byte("node_modules/"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitignoreEntry(root, ".worktrees"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "node_modules/") {
		t.Error("existing content should be preserved")
	}
	if !strings.Contains(content, ".worktrees/") {
		t.Error("new entry should be appended")
	}
}
