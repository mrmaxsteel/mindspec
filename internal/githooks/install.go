package githooks

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// backupMarker is written as the first line of any backup MindSpec creates,
// so CleanStaleGitHooks can identify which backup files belong to MindSpec.
// Leading "#" makes this a no-op shell comment if the backup is ever executed.
const backupMarker = "# MindSpec-created-backup v1"

// writeBackup reads src, prepends the MindSpec backup marker, writes the
// result to dst preserving src's permission bits, then removes src.
// This is the non-atomic equivalent of os.Rename used for backups so we
// can later verify provenance via isMindspecBackup.
func writeBackup(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	out := append([]byte(backupMarker+"\n"), data...)
	if err := os.WriteFile(dst, out, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Remove(src)
}

// isMindspecBackup returns true iff the file at path begins with backupMarker.
// It reads only the first len(backupMarker) bytes.
func isMindspecBackup(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, len(backupMarker))
	n, _ := io.ReadFull(f, buf)
	return n == len(backupMarker) && string(buf) == backupMarker
}

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
		// Existing hook — chain by writing a marker-prefixed backup
		backupPath := hookPath + ".pre-mindspec"
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			if err := writeBackup(hookPath, backupPath); err != nil {
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

// CleanStaleGitHooks removes MindSpec-created backup files and retired hooks
// left by previous MindSpec versions. Only files whose provenance can be
// verified are removed:
//   - *.pre-mindspec files are removed only if they begin with backupMarker.
//   - Hooks listed in retiredHooks are removed only if they contain "MindSpec".
//
// *.backup files are intentionally NOT matched: MindSpec has never written
// a *.backup file, so any such file is foreign and must be preserved.
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
		full := filepath.Join(hooksDir, name)

		if strings.HasSuffix(name, ".pre-mindspec") {
			if isMindspecBackup(full) {
				os.Remove(full)
			}
			// unmarked: leave alone (foreign or legacy pre-marker MindSpec)
			continue
		}

		for _, retired := range retiredHooks {
			if name == retired {
				// Only remove if it's a MindSpec hook (contains our marker)
				data, err := os.ReadFile(full)
				if err == nil && strings.Contains(string(data), "MindSpec") {
					os.Remove(full)
				}
			}
		}
	}
}
