package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPreCommit_NewHook(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	if err := InstallPreCommit(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(hooksDir, "pre-commit"))
	if err != nil {
		t.Fatalf("reading hook: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "MindSpec pre-commit hook") {
		t.Error("hook should contain MindSpec marker")
	}
	if !strings.Contains(content, "MINDSPEC_ALLOW_MAIN") {
		t.Error("hook should contain escape hatch")
	}
}

func TestInstallPreCommit_Idempotent(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	// Install twice
	if err := InstallPreCommit(root); err != nil {
		t.Fatal(err)
	}
	if err := InstallPreCommit(root); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(hooksDir, "pre-commit"))
	count := strings.Count(string(data), "MindSpec pre-commit hook")
	if count != 1 {
		t.Errorf("expected exactly 1 marker, got %d", count)
	}
}

func TestInstallPreCommit_ChainsExisting(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	// Write existing hook
	existing := "#!/bin/bash\necho 'existing hook'\n"
	os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(existing), 0755)

	if err := InstallPreCommit(root); err != nil {
		t.Fatal(err)
	}

	// Check backup exists
	backup, err := os.ReadFile(filepath.Join(hooksDir, "pre-commit.pre-mindspec"))
	if err != nil {
		t.Fatal("backup not created")
	}
	if string(backup) != existing {
		t.Error("backup content doesn't match original")
	}

	// Check new hook chains
	data, _ := os.ReadFile(filepath.Join(hooksDir, "pre-commit"))
	content := string(data)
	if !strings.Contains(content, "MindSpec pre-commit hook") {
		t.Error("new hook should contain MindSpec marker")
	}
	if !strings.Contains(content, "pre-commit.pre-mindspec") {
		t.Error("new hook should chain to backup")
	}
}

func TestInstallPreCommit_NoGitDir(t *testing.T) {
	root := t.TempDir()
	// No .git/hooks — should skip silently
	if err := InstallPreCommit(root); err != nil {
		t.Errorf("expected nil error for non-git dir, got: %v", err)
	}
}

func TestInstallPostCheckout_NewHook(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	if err := InstallPostCheckout(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(hooksDir, "post-checkout"))
	if err != nil {
		t.Fatalf("reading hook: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "MindSpec post-checkout hook v2") {
		t.Error("hook should contain MindSpec v2 marker")
	}
	if !strings.Contains(content, "exit 0") {
		t.Error("v2 hook should be a no-op (exit 0)")
	}
}

func TestInstallPostCheckout_Idempotent(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	if err := InstallPostCheckout(root); err != nil {
		t.Fatal(err)
	}
	if err := InstallPostCheckout(root); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(hooksDir, "post-checkout"))
	count := strings.Count(string(data), "MindSpec post-checkout hook")
	if count != 1 {
		t.Errorf("expected exactly 1 marker, got %d", count)
	}
}

func TestInstallPostCheckout_ChainsExisting(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	existing := "#!/bin/bash\necho 'existing hook'\n"
	os.WriteFile(filepath.Join(hooksDir, "post-checkout"), []byte(existing), 0755)

	if err := InstallPostCheckout(root); err != nil {
		t.Fatal(err)
	}

	backup, err := os.ReadFile(filepath.Join(hooksDir, "post-checkout.pre-mindspec"))
	if err != nil {
		t.Fatal("backup not created")
	}
	if string(backup) != existing {
		t.Error("backup content doesn't match original")
	}

	data, _ := os.ReadFile(filepath.Join(hooksDir, "post-checkout"))
	content := string(data)
	if !strings.Contains(content, "MindSpec post-checkout hook") {
		t.Error("new hook should contain MindSpec marker")
	}
	if !strings.Contains(content, "post-checkout.pre-mindspec") {
		t.Error("new hook should chain to backup")
	}
}

func TestInstallPostCheckout_NoGitDir(t *testing.T) {
	root := t.TempDir()
	if err := InstallPostCheckout(root); err != nil {
		t.Errorf("expected nil error for non-git dir, got: %v", err)
	}
}

func TestInstallAll(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	if err := InstallAll(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both hooks should exist
	for _, name := range []string{"pre-commit", "post-checkout"} {
		if _, err := os.Stat(filepath.Join(hooksDir, name)); err != nil {
			t.Errorf("expected %s hook to exist", name)
		}
	}
}

// initGitRepo creates a real git repo with an initial commit and returns its path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	// Initial commit so HEAD exists
	dummy := filepath.Join(root, "README.md")
	os.WriteFile(dummy, []byte("init\n"), 0644)
	for _, args := range [][]string{
		{"add", "README.md"},
		{"commit", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	return root
}

func TestPostCheckout_AllowsNonProtectedBranch(t *testing.T) {
	root := initGitRepo(t)

	// Install hook and create focus file
	if err := InstallPostCheckout(root); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, ".mindspec"), 0755)
	os.WriteFile(filepath.Join(root, ".mindspec", "focus"), []byte(`{"mode":"idle"}`), 0644)

	// v2 hook is a no-op — checkout should succeed
	cmd := exec.Command("git", "checkout", "-b", "feature/foo")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout should succeed (v2 no-op hook): %s: %v", out, err)
	}

	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = root
	branchOut, _ := cmd.Output()
	if branch := strings.TrimSpace(string(branchOut)); branch != "feature/foo" {
		t.Errorf("expected to be on 'feature/foo', got '%s'", branch)
	}
}

func TestPostCheckout_AllowsProtectedBranch(t *testing.T) {
	root := initGitRepo(t)

	if err := InstallPostCheckout(root); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, ".mindspec"), 0755)
	os.WriteFile(filepath.Join(root, ".mindspec", "focus"), []byte(`{"mode":"idle"}`), 0644)

	// Create and switch to a second protected branch (master)
	cmd := exec.Command("git", "branch", "master")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch master: %s: %v", out, err)
	}

	cmd = exec.Command("git", "checkout", "master")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout to master should be allowed: %s: %v", out, err)
	}

	// Verify we're on master
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = root
	branchOut, _ := cmd.Output()
	if branch := strings.TrimSpace(string(branchOut)); branch != "master" {
		t.Errorf("expected to be on 'master', got '%s'", branch)
	}
}

func TestPostCheckout_UpgradesStaleV1Hook(t *testing.T) {
	root := initGitRepo(t)

	// Write a v1 hook (blocking version)
	hooksDir := filepath.Join(root, ".git", "hooks")
	hookPath := filepath.Join(hooksDir, "post-checkout")
	v1Script := "#!/usr/bin/env bash\n# MindSpec post-checkout hook v1 (Layer 1 enforcement)\nexit 1\n"
	os.WriteFile(hookPath, []byte(v1Script), 0755)

	// InstallPostCheckout should detect stale v1 and upgrade to v2
	if err := InstallPostCheckout(root); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "post-checkout hook v2") {
		t.Error("expected v2 hook after upgrade, got:", content)
	}
	if strings.Contains(content, "checkout blocked") {
		t.Error("v2 hook should not contain blocking logic")
	}
}

func TestPostCheckout_SkipsWithoutFocusFile(t *testing.T) {
	root := initGitRepo(t)

	if err := InstallPostCheckout(root); err != nil {
		t.Fatal(err)
	}
	// No .mindspec/focus — hook should not enforce

	cmd := exec.Command("git", "checkout", "-b", "feature/baz")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout without focus file should succeed: %s: %v", out, err)
	}

	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = root
	branchOut, _ := cmd.Output()
	if branch := strings.TrimSpace(string(branchOut)); branch != "feature/baz" {
		t.Errorf("expected to be on 'feature/baz', got '%s'", branch)
	}
}

func TestPostCheckout_SkipsLinkedWorktree(t *testing.T) {
	root := initGitRepo(t)

	if err := InstallPostCheckout(root); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, ".mindspec"), 0755)
	os.WriteFile(filepath.Join(root, ".mindspec", "focus"), []byte(`{"mode":"idle"}`), 0644)

	// Create a linked worktree
	wtPath := filepath.Join(t.TempDir(), "linked-wt")
	cmd := exec.Command("git", "worktree", "add", wtPath, "-b", "wt-branch")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %s: %v", out, err)
	}

	// In the linked worktree, checkout to another branch should be allowed
	// (because git-dir != git-common-dir in a linked worktree)
	cmd = exec.Command("git", "checkout", "-b", "feature/in-worktree")
	cmd.Dir = wtPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout in linked worktree should succeed: %s: %v", out, err)
	}

	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = wtPath
	branchOut, _ := cmd.Output()
	if branch := strings.TrimSpace(string(branchOut)); branch != "feature/in-worktree" {
		t.Errorf("expected to be on 'feature/in-worktree', got '%s'", branch)
	}
}
