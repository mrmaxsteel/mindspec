package githooks

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

	backupPath := filepath.Join(hooksDir, "pre-commit.pre-mindspec")
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal("backup not created")
	}
	backupContent := string(backup)
	expected := backupMarker + "\n" + existing
	if backupContent != expected {
		t.Errorf("backup content mismatch:\n got: %q\nwant: %q", backupContent, expected)
	}
	if !strings.HasPrefix(backupContent, backupMarker+"\n") {
		t.Error("backup should begin with backupMarker line")
	}
	if !strings.Contains(backupContent, existing) {
		t.Error("backup should contain the original script body")
	}

	data, _ := os.ReadFile(filepath.Join(hooksDir, "pre-commit"))
	content := string(data)
	if !strings.Contains(content, "MindSpec pre-commit hook v5") {
		t.Error("new hook should contain v5 marker")
	}

	// Round-trip: cleanup should remove our marker-prefixed backup.
	CleanStaleGitHooks(root)
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("CleanStaleGitHooks should have removed marker-prefixed backup")
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

func TestCleanStaleGitHooks(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	// Create stale files that should be removed.
	// Only marker-prefixed .pre-mindspec and MindSpec-marked retired hooks
	// are MindSpec's to clean. .backup files are never written by MindSpec
	// and must be preserved (see keepFiles).
	staleFiles := map[string]string{
		"pre-commit.pre-mindspec": backupMarker + "\n#!/bin/bash\noriginal hook",
		"post-checkout":           "#!/bin/bash\n# MindSpec post-checkout hook\nexec mindspec hook post-checkout",
	}
	for name, content := range staleFiles {
		os.WriteFile(filepath.Join(hooksDir, name), []byte(content), 0755)
	}

	// Create files that should NOT be removed.
	// pre-commit.backup is foreign by definition — MindSpec never writes one.
	keepFiles := map[string]string{
		"pre-commit":        preCommitScript,
		"commit-msg":        "#!/bin/bash\nsome other hook",
		"pre-commit.backup": "#!/bin/bash\nold hook",
	}
	for name, content := range keepFiles {
		os.WriteFile(filepath.Join(hooksDir, name), []byte(content), 0755)
	}

	CleanStaleGitHooks(root)

	for name := range staleFiles {
		if _, err := os.Stat(filepath.Join(hooksDir, name)); !os.IsNotExist(err) {
			t.Errorf("stale file %q should have been removed", name)
		}
	}
	for name := range keepFiles {
		if _, err := os.Stat(filepath.Join(hooksDir, name)); os.IsNotExist(err) {
			t.Errorf("file %q should have been kept", name)
		}
	}
}

// TestCleanStaleGitHooks_PreservesForeignBackup verifies that a *.backup
// file (which MindSpec never creates) is left alone, regardless of contents.
func TestCleanStaleGitHooks_PreservesForeignBackup(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	path := filepath.Join(hooksDir, "pre-commit.backup")
	original := "#!/bin/bash\nnot ours\n"
	os.WriteFile(path, []byte(original), 0755)

	CleanStaleGitHooks(root)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("foreign *.backup file should not have been removed: %v", err)
	}
	if string(data) != original {
		t.Errorf("foreign *.backup file content was modified:\n got: %q\nwant: %q", string(data), original)
	}
}

// TestCleanStaleGitHooks_PreservesForeignPreMindspec verifies that a
// .pre-mindspec file lacking the MindSpec backup marker is preserved.
func TestCleanStaleGitHooks_PreservesForeignPreMindspec(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	path := filepath.Join(hooksDir, "pre-commit.pre-mindspec")
	original := "#!/bin/bash\nnot ours\n"
	os.WriteFile(path, []byte(original), 0755)

	CleanStaleGitHooks(root)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unmarked *.pre-mindspec file should not have been removed: %v", err)
	}
	if string(data) != original {
		t.Errorf("unmarked *.pre-mindspec file content was modified:\n got: %q\nwant: %q", string(data), original)
	}
}

// TestCleanStaleGitHooks_RemovesMindspecPreMindspec verifies that a
// .pre-mindspec file beginning with backupMarker is removed.
func TestCleanStaleGitHooks_RemovesMindspecPreMindspec(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	// Plant via writeBackup so the marker is in canonical form.
	src := filepath.Join(hooksDir, "pre-commit")
	os.WriteFile(src, []byte("#!/bin/bash\noriginal\n"), 0755)
	dst := src + ".pre-mindspec"
	if err := writeBackup(src, dst); err != nil {
		t.Fatalf("writeBackup: %v", err)
	}
	if !isMindspecBackup(dst) {
		t.Fatal("planted backup should be recognized as a MindSpec backup")
	}

	CleanStaleGitHooks(root)

	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Errorf("marker-prefixed *.pre-mindspec should have been removed")
	}
}

func TestCleanStaleGitHooks_NonMindspecRetiredHook(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	// A post-checkout that is NOT a MindSpec hook should NOT be removed
	path := filepath.Join(hooksDir, "post-checkout")
	os.WriteFile(path, []byte("#!/bin/bash\nuser's own hook"), 0755)

	CleanStaleGitHooks(root)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("non-MindSpec post-checkout hook should NOT have been removed")
	}
}

func TestCleanStaleGitHooks_NoHooksDir(t *testing.T) {
	root := t.TempDir()
	CleanStaleGitHooks(root) // should not panic
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
