package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Package-level function variables for testability.
var execCommand = exec.Command

// CurrentBranch returns the name of the current git branch.
func CurrentBranch() (string, error) {
	cmd := execCommand("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// BranchExists returns true if the named branch exists locally.
func BranchExists(name string) bool {
	cmd := execCommand("git", "rev-parse", "--verify", "refs/heads/"+name)
	return cmd.Run() == nil
}

// CreateBranch creates a new branch from the given base.
func CreateBranch(name, from string) error {
	cmd := execCommand("git", "branch", name, from)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating branch %s from %s: %s", name, from, strings.TrimSpace(string(out)))
	}
	return nil
}

// MergeBranch merges source into target using --no-ff (from the given workdir).
// If workdir is empty, uses the current directory.
func MergeBranch(workdir, source, target string) error {
	// Checkout target
	checkoutCmd := execCommand("git", "-C", workdir, "checkout", target)
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkout %s: %s", target, strings.TrimSpace(string(out)))
	}

	// Merge source
	mergeCmd := execCommand("git", "-C", workdir, "merge", "--no-ff", source, "-m",
		fmt.Sprintf("Merge %s into %s", source, target))
	if out, err := mergeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("merge %s into %s: %s", source, target, strings.TrimSpace(string(out)))
	}

	return nil
}

// MergeInto merges sourceBranch into the current branch of targetWorkdir.
// Unlike MergeBranch, this does not checkout — it assumes targetWorkdir already
// has the desired branch checked out (e.g. a spec worktree).
func MergeInto(targetWorkdir, sourceBranch string) error {
	mergeCmd := execCommand("git", "-C", targetWorkdir, "merge", "--no-ff", sourceBranch, "-m",
		fmt.Sprintf("Merge %s", sourceBranch))
	if out, err := mergeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("merge %s in %s: %s", sourceBranch, targetWorkdir, strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteBranch deletes a local branch.
func DeleteBranch(name string) error {
	cmd := execCommand("git", "branch", "-d", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("deleting branch %s: %s", name, strings.TrimSpace(string(out)))
	}
	return nil
}

// MainWorktreePath returns the path of the main (non-linked) worktree.
func MainWorktreePath() (string, error) {
	cmd := execCommand("git", "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("listing worktrees: %w", err)
	}

	// The first "worktree <path>" line is always the main worktree.
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			return strings.TrimPrefix(line, "worktree "), nil
		}
	}
	return "", fmt.Errorf("no worktree found in git output")
}

// IsMainWorktree returns true if the given path is the main (non-linked) worktree.
func IsMainWorktree(path string) (bool, error) {
	mainPath, err := MainWorktreePath()
	if err != nil {
		return false, err
	}
	return path == mainPath, nil
}

// HasRemote returns true if at least one git remote is configured.
func HasRemote() bool {
	cmd := execCommand("git", "remote")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// PushBranch pushes a branch to origin.
func PushBranch(branch string) error {
	cmd := execCommand("git", "push", "-u", "origin", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pushing %s: %s", branch, strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureGitignoreEntry adds an entry to .gitignore if not already present.
func EnsureGitignoreEntry(root, entry string) error {
	gitignorePath := root + "/.gitignore"

	// Read existing content
	data, err := readFile(gitignorePath)
	if err != nil {
		data = nil // File doesn't exist yet
	}

	// Check if already present
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == entry || trimmed == entry+"/" {
			return nil // Already present
		}
	}

	// Append
	content := string(data)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "# mindspec worktrees\n" + entry + "/\n"

	return writeFile(gitignorePath, []byte(content), 0o644)
}

// DiffStat returns a short diffstat summary between two refs.
// workdir specifies the git repository path.
func DiffStat(workdir, base, head string) (string, error) {
	cmd := execCommand("git", "-C", workdir, "diff", "--stat", base+".."+head)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("diffstat %s..%s: %w", base, head, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CommitCount returns the number of commits between base and head.
func CommitCount(workdir, base, head string) (int, error) {
	cmd := execCommand("git", "-C", workdir, "rev-list", "--count", base+".."+head)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("commit count %s..%s: %w", base, head, err)
	}
	var count int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count); err != nil {
		return 0, fmt.Errorf("parsing commit count: %w", err)
	}
	return count, nil
}

// IsAncestor returns true if ancestor is an ancestor of descendant.
// Uses git merge-base --is-ancestor.
func IsAncestor(workdir, ancestor, descendant string) (bool, error) {
	cmd := execCommand("git", "-C", workdir, "merge-base", "--is-ancestor", ancestor, descendant)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	// Exit code 1 means not an ancestor; other errors are real failures.
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("checking ancestry %s..%s: %w", ancestor, descendant, err)
}

// CommitAll stages all changes in workdir and commits with the given message.
// Used for auto-commits at lifecycle boundaries (spec-init, approvals) to ensure
// artifacts are on the branch before downstream worktrees branch from it.
// Returns nil if there are no changes to commit.
func CommitAll(workdir, message string) error {
	statusCmd := execCommand("git", "-C", workdir, "status", "--porcelain")
	statusOut, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("checking git status: %w", err)
	}
	if strings.TrimSpace(string(statusOut)) == "" {
		return nil // nothing to commit
	}

	addCmd := execCommand("git", "-C", workdir, "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("staging changes: %s", strings.TrimSpace(string(out)))
	}

	commitCmd := execCommand("git", "-C", workdir, "commit", "-m", message)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("committing: %s", strings.TrimSpace(string(out)))
	}

	return nil
}

// File I/O wrappers for testability.
var (
	readFile  = os.ReadFile
	writeFile = os.WriteFile
)
