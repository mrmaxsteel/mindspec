package harness

import (
	"os"
	"path/filepath"
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

func TestSandboxWriteFocus(t *testing.T) {
	s := NewSandbox(t)

	s.WriteFocus(`{"mode":"spec","activeSpec":"001-test"}`)

	content := s.ReadFile(".mindspec/focus")
	if content == "" {
		t.Error("focus file is empty")
	}
}

func TestSandboxWriteLifecycle(t *testing.T) {
	s := NewSandbox(t)

	s.WriteLifecycle("001-test", "phase: spec\nepic_id: mindspec-abc\n")

	content := s.ReadFile(".mindspec/docs/specs/001-test/lifecycle.yaml")
	if content == "" {
		t.Error("lifecycle.yaml is empty")
	}
}
