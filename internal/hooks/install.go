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

// InstallAll installs all MindSpec git hooks.
func InstallAll(root string) error {
	return InstallPreCommit(root)
}
