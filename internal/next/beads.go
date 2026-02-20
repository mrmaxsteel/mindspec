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
			items, err := QueryReadyForMolecule(s.ActiveMolecule)
			if err == nil && len(items) > 0 {
				return items, nil
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

// QueryReadyForMolecule queries ready work for a specific molecule.
func QueryReadyForMolecule(moleculeID string) ([]BeadInfo, error) {
	out, err := runBDFn("ready", "--mol", moleculeID, "--json")
	if err != nil {
		// Compatibility fallback for older beads versions.
		out, err = runBDFn("ready", "--parent", moleculeID, "--json")
		if err != nil {
			return nil, fmt.Errorf("bd ready for molecule %s failed: %w", moleculeID, err)
		}
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
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var items []BeadInfo
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("parsing beads JSON: %w", err)
		}
		return filterReadyItems(items), nil
	}

	if strings.HasPrefix(trimmed, "{") {
		var payload struct {
			Steps []struct {
				Issue BeadInfo `json:"issue"`
			} `json:"steps"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, fmt.Errorf("parsing molecule-ready JSON: %w", err)
		}
		items := make([]BeadInfo, 0, len(payload.Steps))
		for _, step := range payload.Steps {
			items = append(items, step.Issue)
		}
		return filterReadyItems(items), nil
	}

	return nil, fmt.Errorf("parsing beads JSON: unsupported payload shape")
}

func filterReadyItems(items []BeadInfo) []BeadInfo {
	seen := map[string]struct{}{}
	var filtered []BeadInfo
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.IssueType), "epic") {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if status == "closed" {
			continue
		}
		if status != "" && status != "open" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		filtered = append(filtered, item)
	}
	return filtered
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
