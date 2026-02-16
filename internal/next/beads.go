package next

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/state"
)

// BeadInfo represents a work item from Beads.
type BeadInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  int    `json:"priority"`
	IssueType string `json:"issue_type"`
	Owner     string `json:"owner"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Package-level function variables for testability.
var (
	runBDFn        = bead.RunBD
	runBDCombFn    = bead.RunBDCombined
	worktreeList   = bead.WorktreeList
	worktreeCreate = bead.WorktreeCreate
	readStateFn    = state.Read
)

// QueryReady discovers ready work. If an active molecule exists in state,
// queries its ready children. Otherwise falls back to global bd ready.
func QueryReady() ([]BeadInfo, error) {
	// Check state for active molecule
	root, err := findRoot()
	if err == nil {
		s, err := readStateFn(root)
		if err == nil && s.ActiveMolecule != "" {
			out, err := runBDFn("ready", "--parent", s.ActiveMolecule, "--json")
			if err == nil {
				items, err := ParseBeadsJSON(out)
				if err == nil && len(items) > 0 {
					return items, nil
				}
			}
		}
	}

	// Fall back to global bd ready
	out, err := runBDFn("ready", "--json")
	if err != nil {
		return nil, fmt.Errorf("bd ready failed: %w", err)
	}

	return ParseBeadsJSON(out)
}

// findRoot attempts to find the workspace root for state reading.
func findRoot() (string, error) {
	out, err := runBDFn("worktree", "info", "--json")
	if err != nil {
		// Not in a worktree — try current directory
		return ".", nil
	}
	var info struct {
		MainRepo string `json:"main_repo"`
		Path     string `json:"path"`
	}
	if json.Unmarshal(out, &info) == nil && info.MainRepo != "" {
		return info.MainRepo, nil
	}
	return ".", nil
}

// ParseBeadsJSON parses the JSON output from bd commands into BeadInfo slices.
func ParseBeadsJSON(data []byte) ([]BeadInfo, error) {
	var items []BeadInfo
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parsing beads JSON: %w", err)
	}
	return items, nil
}

// ClaimBead marks a bead as in_progress via bd update.
func ClaimBead(id string) error {
	_, err := runBDCombFn("update", id, "--status=in_progress")
	return err
}

// EnsureWorktree checks for an existing worktree for the bead, or creates one.
// Returns the worktree path. Returns empty string if worktree creation is not
// applicable (e.g., working on main).
func EnsureWorktree(beadID string) (string, error) {
	entries, err := worktreeList()
	if err != nil {
		return "", fmt.Errorf("listing worktrees: %w", err)
	}

	// Check for existing worktree matching this bead
	wtName := "worktree-" + beadID
	branchName := "bead/" + beadID
	for _, e := range entries {
		if e.Name == wtName || e.Branch == branchName {
			return e.Path, nil
		}
	}

	// Create new worktree via bd worktree create
	if err := worktreeCreate(wtName, branchName); err != nil {
		return "", fmt.Errorf("creating worktree: %w", err)
	}

	// Read back path from worktree list
	entries, err = worktreeList()
	if err != nil {
		return "", fmt.Errorf("reading worktree path: %w", err)
	}
	for _, e := range entries {
		if e.Name == wtName || strings.HasSuffix(e.Path, wtName) {
			return e.Path, nil
		}
	}

	// Fallback: return the name (relative path)
	return wtName, nil
}
