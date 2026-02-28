package cleanup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/gitops"
	"github.com/mindspec/mindspec/internal/state"
)

// Package-level function variables for testability.
var (
	prStatusFn       = gitops.PRStatus
	worktreeRemoveFn = bead.WorktreeRemove
	deleteBranchFn   = gitops.DeleteBranch
)

// Result holds the output of a cleanup operation.
type Result struct {
	SpecID          string
	PRStatus        string // "merged", "open", "closed"
	PRURL           string
	WorktreeRemoved bool
	BranchDeleted   bool
	Warnings        []string
}

// Run cleans up worktree and branch resources for a completed spec.
// It checks PR status if a PR URL is known, and removes the worktree
// and branch when the spec has been merged.
func Run(root, specID string, force bool) (*Result, error) {
	result := &Result{SpecID: specID}

	// Determine the spec branch and worktree name from conventions.
	specBranch := state.SpecBranch(specID)
	specWtName := "worktree-spec-" + specID

	if force {
		return forceCleanup(result, specWtName, specBranch)
	}

	// If focus still has activeSpec matching, check mode.
	mc, _ := state.ReadFocus(root)
	if mc != nil && mc.ActiveSpec == specID && mc.Mode != state.ModeIdle {
		return nil, fmt.Errorf("spec %s is still active (mode: %s). Run `mindspec approve impl %s` first", specID, mc.Mode, specID)
	}

	// Try to detect PR status via gh CLI. Look for an open or merged PR
	// for the spec branch.
	prStatus, prURL, err := findPRForBranchFn(specBranch)
	if err == nil {
		result.PRStatus = prStatus
		result.PRURL = prURL
	}

	switch prStatus {
	case "merged":
		// Safe to clean up.
		fmt.Fprintf(os.Stderr, "PR %s is merged. Cleaning up...\n", prURL)

	case "open":
		return result, fmt.Errorf("PR %s is still open. Merge it first, or use --force to clean up anyway", prURL)

	case "closed":
		return result, fmt.Errorf("PR %s was closed without merging. Re-open it or use --force to clean up", prURL)

	default:
		// No PR found — branch may have been direct-merged or never pushed.
		// Check if branch still exists locally.
		if !gitops.BranchExists(specBranch) {
			fmt.Fprintf(os.Stderr, "Branch %s already deleted.\n", specBranch)
		}
	}

	// Remove worktree (best-effort).
	if err := worktreeRemoveFn(specWtName); err != nil {
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "does not exist") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not remove worktree: %v", err))
		}
	} else {
		result.WorktreeRemoved = true
	}

	// Delete branch (best-effort).
	if gitops.BranchExists(specBranch) {
		if err := deleteBranchFn(specBranch); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not delete branch: %v", err))
		} else {
			result.BranchDeleted = true
		}
	}

	return result, nil
}

// forceCleanup removes worktree and branch without checking PR status.
func forceCleanup(result *Result, wtName, branch string) (*Result, error) {
	if err := worktreeRemoveFn(wtName); err != nil {
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "does not exist") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not remove worktree: %v", err))
		}
	} else {
		result.WorktreeRemoved = true
	}

	if gitops.BranchExists(branch) {
		if err := deleteBranchFn(branch); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not delete branch: %v", err))
		} else {
			result.BranchDeleted = true
		}
	}

	return result, nil
}

var findPRForBranchFn = findPRForBranch

// findPRForBranch uses gh CLI to find a PR for the given branch.
func findPRForBranch(branch string) (status string, url string, err error) {
	if _, lookErr := exec.LookPath("gh"); lookErr != nil {
		return "", "", fmt.Errorf("gh CLI not found")
	}

	cmd := exec.Command("gh", "pr", "list", "--head", branch, "--json", "url,state", "--limit", "1", "--state", "all")
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("listing PRs for %s: %w", branch, err)
	}

	var prs []struct {
		URL   string `json:"url"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		return "", "", fmt.Errorf("parsing PR list: %w", err)
	}
	if len(prs) == 0 {
		return "", "", fmt.Errorf("no PR found for branch %s", branch)
	}

	return strings.ToLower(prs[0].State), prs[0].URL, nil
}
