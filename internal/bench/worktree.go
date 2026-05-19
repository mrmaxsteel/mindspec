package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// CreateWorktree creates a git worktree at wtPath from the given commit,
// then checks out a new branch with the given name.
func CreateWorktree(repoRoot, branch, wtPath, commit string) error {
	if err := gitutil.WorktreeAddDetach(repoRoot, wtPath, commit); err != nil {
		return fmt.Errorf("git worktree add: %w", err)
	}
	if err := gitutil.CheckoutNewBranch(wtPath, branch); err != nil {
		return fmt.Errorf("git checkout -b: %w", err)
	}
	return nil
}

// CheckoutWorktree creates a git worktree at wtPath from an existing branch.
func CheckoutWorktree(repoRoot, branch, wtPath string) error {
	if err := gitutil.WorktreeAdd(repoRoot, wtPath, branch); err != nil {
		return fmt.Errorf("git worktree add (existing branch): %w", err)
	}
	return nil
}

// RemoveWorktree force-removes a git worktree and prunes.
func RemoveWorktree(repoRoot, wtPath string) error {
	if err := gitutil.WorktreeRemoveForce(repoRoot, wtPath); err != nil {
		return fmt.Errorf("git worktree remove: %w", err)
	}
	return nil
}

// NeutralizeBaseline removes MindSpec-specific files from a worktree while
// keeping docs/ and non-MindSpec .claude/ settings intact.
func NeutralizeBaseline(wtPath string) error {
	// Remove CLAUDE.md
	os.Remove(filepath.Join(wtPath, "CLAUDE.md"))

	// Preserve canonical docs as legacy docs for baseline sessions.
	canonicalDocs := filepath.Join(wtPath, ".mindspec", "docs")
	legacyDocs := filepath.Join(wtPath, "docs")
	if info, err := os.Stat(canonicalDocs); err == nil && info.IsDir() {
		if _, err := os.Stat(legacyDocs); os.IsNotExist(err) {
			if err := os.Rename(canonicalDocs, legacyDocs); err != nil {
				return fmt.Errorf("preserving canonical docs for baseline: %w", err)
			}
		}
	}

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
	os.RemoveAll(filepath.Join(wtPath, ".mindspec", "docs"))
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
