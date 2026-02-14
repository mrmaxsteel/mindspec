package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CreateWorktree creates a git worktree at wtPath from the given commit,
// then checks out a new branch with the given name.
func CreateWorktree(repoRoot, branch, wtPath, commit string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "add", "--detach", wtPath, commit)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add: %w\n%s", err, out)
	}

	cmd = exec.Command("git", "-C", wtPath, "checkout", "-b", branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout -b: %w\n%s", err, out)
	}
	return nil
}

// CheckoutWorktree creates a git worktree at wtPath from an existing branch.
func CheckoutWorktree(repoRoot, branch, wtPath string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "add", wtPath, branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add (existing branch): %w\n%s", err, out)
	}
	return nil
}

// RemoveWorktree force-removes a git worktree and prunes.
func RemoveWorktree(repoRoot, wtPath string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "remove", "--force", wtPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, out)
	}
	return nil
}

// NeutralizeBaseline removes MindSpec-specific files from a worktree while
// keeping docs/ and non-MindSpec .claude/ settings intact.
func NeutralizeBaseline(wtPath string) error {
	// Remove CLAUDE.md
	os.Remove(filepath.Join(wtPath, "CLAUDE.md"))

	// Remove .mindspec/
	os.RemoveAll(filepath.Join(wtPath, ".mindspec"))

	// Remove MindSpec-specific commands
	commands := []string{
		"spec-init.md", "spec-approve.md", "plan-approve.md",
		"impl-approve.md", "spec-status.md",
	}
	for _, cmd := range commands {
		os.Remove(filepath.Join(wtPath, ".claude", "commands", cmd))
	}

	// Strip hooks from settings.json
	settingsPath := filepath.Join(wtPath, ".claude", "settings.json")
	if err := stripHooks(settingsPath); err != nil {
		// Non-fatal: settings.json may not exist
		_ = err
	}

	return nil
}

// NeutralizeNoDocs removes MindSpec-specific files AND docs/ from a worktree.
func NeutralizeNoDocs(wtPath string) error {
	if err := NeutralizeBaseline(wtPath); err != nil {
		return err
	}
	os.RemoveAll(filepath.Join(wtPath, "docs"))
	return nil
}

// stripHooks reads a settings.json file, removes the "hooks" key, and writes it back.
// Uses encoding/json — no python3 dependency.
func stripHooks(settingsPath string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return err
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("parsing settings.json: %w", err)
	}

	delete(obj, "hooks")

	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings.json: %w", err)
	}
	out = append(out, '\n')

	return os.WriteFile(settingsPath, out, 0644)
}
