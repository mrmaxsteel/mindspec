package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSandboxCreatesValidRepo(t *testing.T) {
	s := NewSandbox(t)

	// .git exists
	if !s.FileExists(".git") {
		t.Error(".git directory missing")
	}

	// .mindspec structure
	for _, path := range []string{
		".mindspec",
		".mindspec/config.yaml",
		".mindspec/docs",
		".mindspec/docs/specs",
		".mindspec/docs/adr",
	} {
		if !s.FileExists(path) {
			t.Errorf("%s missing", path)
		}
	}

	// Claude Code setup files (from setup.RunClaude)
	for _, path := range []string{
		"CLAUDE.md",
		".claude/settings.json",
	} {
		if !s.FileExists(path) {
			t.Errorf("%s missing (setup.RunClaude)", path)
		}
	}

	// config.yaml has expected content
	config := s.ReadFile(".mindspec/config.yaml")
	if config == "" {
		t.Error("config.yaml is empty")
	}

	// Git branch is main (or master)
	branch := s.GitBranch()
	if branch != "main" && branch != "master" {
		t.Errorf("branch = %q, want main or master", branch)
	}

	// Has at least one commit
	out, err := s.Run("git", "log", "--oneline", "-1")
	if err != nil {
		t.Errorf("git log failed: %v", err)
	}
	if out == "" {
		t.Error("no commits in sandbox")
	}
}

func TestSandboxShimsInstalled(t *testing.T) {
	s := NewSandbox(t)

	// Shim dir exists
	if _, err := os.Stat(s.ShimBinDir); err != nil {
		t.Errorf("shim dir missing: %v", err)
	}

	// At least git shim should exist
	gitShim := filepath.Join(s.ShimBinDir, "git")
	if _, err := os.Stat(gitShim); err != nil {
		t.Errorf("git shim missing: %v", err)
	}
}

func TestSandboxWriteAndReadFile(t *testing.T) {
	s := NewSandbox(t)

	s.WriteFile("internal/foo.go", "package foo\n")

	content := s.ReadFile("internal/foo.go")
	if content != "package foo\n" {
		t.Errorf("ReadFile = %q, want %q", content, "package foo\n")
	}
}

func TestSandboxReadEventsEmpty(t *testing.T) {
	s := NewSandbox(t)
	events := s.ReadEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestSandboxCommit(t *testing.T) {
	s := NewSandbox(t)

	s.WriteFile("hello.txt", "hello\n")
	s.Commit("add hello")

	// Verify commit exists
	out, err := s.Run("git", "log", "--oneline", "-1")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if out == "" {
		t.Error("no commit after Commit()")
	}
}

func TestSandboxCommitAllowsMainInSetup(t *testing.T) {
	s := NewSandbox(t)

	s.WriteFile("setup.txt", "setup\n")
	s.Commit("setup: commit")

	out, err := s.Run("git", "log", "--oneline", "-1")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if out == "" {
		t.Error("no commit after setup Commit()")
	}
}

func TestSandboxWriteFocusIsNoOp(t *testing.T) {
	s := NewSandbox(t)

	// WriteFocus is a no-op after ADR-0023
	s.WriteFocus(`{"mode":"spec","activeSpec":"001-test"}`)

	// Focus file should NOT be created
	if s.FileExists(".mindspec/focus") {
		t.Error("focus file should not be created (ADR-0023)")
	}
}

func TestSandboxListWorktreesExcludesMainAcrossPathAliases(t *testing.T) {
	s := NewSandbox(t)

	// Force an alias path form when available (/var vs /private/var on macOS),
	// so ListWorktrees must normalize both sides before comparing.
	if strings.HasPrefix(s.Root, "/private/var/") {
		alias := strings.Replace(s.Root, "/private/var/", "/var/", 1)
		if _, err := os.Stat(alias); err == nil {
			s.Root = alias
		}
	}

	if _, err := s.Run("git", "branch", "spec/001-test"); err != nil {
		t.Fatalf("create spec branch: %v", err)
	}
	if _, err := s.Run("git", "worktree", "add", ".worktrees/worktree-spec-001-test", "spec/001-test"); err != nil {
		t.Fatalf("create spec worktree: %v", err)
	}

	worktrees := s.ListWorktrees()
	if len(worktrees) != 1 || worktrees[0] != "worktree-spec-001-test" {
		t.Fatalf("expected only linked spec worktree, got: %v", worktrees)
	}
}
