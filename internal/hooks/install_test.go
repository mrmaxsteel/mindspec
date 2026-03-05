package hooks

import (
	"os"
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
	if !strings.Contains(content, "MindSpec pre-commit hook v5") {
		t.Error("hook should contain v5 marker")
	}
	if !strings.Contains(content, "mindspec hook pre-commit") {
		t.Error("hook should delegate to mindspec hook pre-commit")
	}
	if !strings.Contains(content, "MINDSPEC_ALLOW_MAIN") {
		t.Error("hook should mention escape hatch")
	}
}

func TestInstallPreCommit_Idempotent(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

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

	existing := "#!/bin/bash\necho 'existing hook'\n"
	os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(existing), 0755)

	if err := InstallPreCommit(root); err != nil {
		t.Fatal(err)
	}

	backup, err := os.ReadFile(filepath.Join(hooksDir, "pre-commit.pre-mindspec"))
	if err != nil {
		t.Fatal("backup not created")
	}
	if string(backup) != existing {
		t.Error("backup content doesn't match original")
	}

	data, _ := os.ReadFile(filepath.Join(hooksDir, "pre-commit"))
	content := string(data)
	if !strings.Contains(content, "MindSpec pre-commit hook v5") {
		t.Error("new hook should contain v5 marker")
	}
}

func TestInstallPreCommit_UpgradesOldVersion(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	// Write an old v4 hook
	oldHook := "#!/usr/bin/env bash\n# MindSpec pre-commit hook v4 (Layer 1 enforcement)\necho old\n"
	os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(oldHook), 0755)

	if err := InstallPreCommit(root); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(hooksDir, "pre-commit"))
	content := string(data)
	if !strings.Contains(content, "pre-commit hook v5") {
		t.Errorf("expected v5 after upgrade, got: %s", content)
	}
}

func TestInstallPreCommit_NoGitDir(t *testing.T) {
	root := t.TempDir()
	if err := InstallPreCommit(root); err != nil {
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

	if _, err := os.Stat(filepath.Join(hooksDir, "pre-commit")); err != nil {
		t.Error("expected pre-commit hook to exist")
	}
	// post-checkout should NOT be installed
	if _, err := os.Stat(filepath.Join(hooksDir, "post-checkout")); !os.IsNotExist(err) {
		t.Error("post-checkout should NOT be installed")
	}
}
