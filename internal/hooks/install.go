package hooks

import (
	"os"
	"path/filepath"
	"strings"
)

const preCommitScript = `#!/usr/bin/env bash
# MindSpec pre-commit hook v5 (thin shim)
# Delegates to mindspec CLI for branch protection logic.
# Escape hatch: MINDSPEC_ALLOW_MAIN=1 git commit
exec mindspec hook pre-commit "$@"
`

// InstallPreCommit installs the MindSpec pre-commit hook.
// It uses the git hooks path and chains with any existing pre-commit hook.
func InstallPreCommit(root string) error {
	hooksDir := filepath.Join(root, ".git", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")
	marker := "# MindSpec pre-commit hook"

	if data, err := os.ReadFile(hookPath); err == nil {
		content := string(data)
		if strings.Contains(content, marker) {
			if !strings.Contains(content, "pre-commit hook v5") {
				return os.WriteFile(hookPath, []byte(preCommitScript), 0755)
			}
			return nil // already installed and current
		}
		// Existing hook — chain by renaming
		backupPath := hookPath + ".pre-mindspec"
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			if err := os.Rename(hookPath, backupPath); err != nil {
				return err
			}
		}
	}

	return os.WriteFile(hookPath, []byte(preCommitScript), 0755)
}

// InstallAll installs all MindSpec git hooks and cleans stale artifacts.
func InstallAll(root string) error {
	if err := InstallPreCommit(root); err != nil {
		return err
	}
	CleanStaleGitHooks(root)
	return nil
}

// retiredHooks lists git hook files that MindSpec previously installed but no longer uses.
var retiredHooks = []string{"post-checkout"}

// CleanStaleGitHooks removes backup files and retired hooks left by previous MindSpec versions.
// Stale artifacts: *.backup, *.pre-mindspec, and hooks listed in retiredHooks.
func CleanStaleGitHooks(root string) {
	hooksDir := filepath.Join(root, ".git", "hooks")
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return // no hooks dir — nothing to clean
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		remove := false
		if strings.HasSuffix(name, ".backup") || strings.HasSuffix(name, ".pre-mindspec") {
			remove = true
		}
		for _, retired := range retiredHooks {
			if name == retired {
				// Only remove if it's a MindSpec hook (contains our marker)
				data, err := os.ReadFile(filepath.Join(hooksDir, name))
				if err == nil && strings.Contains(string(data), "MindSpec") {
					remove = true
				}
			}
		}
		if remove {
			os.Remove(filepath.Join(hooksDir, name))
		}
	}
}
