package next

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/config"
	"github.com/mindspec/mindspec/internal/gitops"
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
	runBDFn          = bead.RunBD
	runBDCombFn      = bead.RunBDCombined
	worktreeList     = bead.WorktreeList
	worktreeCreate   = bead.WorktreeCreate
	readFocusFn  = state.ReadFocus
	writeFocusFn = state.WriteFocus
	loadConfigFn     = config.Load
	createBranchFn   = gitops.CreateBranch
	branchExistsFn   = gitops.BranchExists
	ensureGitignore  = gitops.EnsureGitignoreEntry
)

// QueryReady discovers ready work via global bd ready.
func QueryReady() ([]BeadInfo, error) {
	out, err := runBDFn("ready", "--json")
	if err != nil {
		return nil, fmt.Errorf("bd ready failed: %w", err)
	}

	return ParseBeadsJSON(out)
}

// QueryReadyForEpic queries ready work scoped to a specific epic (parent issue).
func QueryReadyForEpic(epicID string) ([]BeadInfo, error) {
	out, err := runBDFn("ready", "--parent", epicID, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd ready for epic %s failed: %w", epicID, err)
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
			return nil, fmt.Errorf("parsing beads ready JSON: %w", err)
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

// ResolveActiveBead finds the currently in-progress bead for a spec by reading
// the epic_id from lifecycle.yaml and querying bd for in-progress children.
// Returns empty string (no error) if no bead is in progress.
func ResolveActiveBead(root, specID string) (string, error) {
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", specID)
	if _, err := os.Stat(specDir); err != nil {
		// Try legacy docs path
		specDir = filepath.Join(root, "docs", "specs", specID)
	}

	lc, err := state.ReadLifecycle(specDir)
	if err != nil {
		return "", fmt.Errorf("reading lifecycle for %s: %w", specID, err)
	}
	if lc == nil || lc.EpicID == "" {
		return "", nil
	}

	out, err := runBDFn("list", "--parent", lc.EpicID, "--status=in_progress", "--json")
	if err != nil {
		return "", nil // No in-progress beads
	}

	// Parse directly — don't use filterReadyItems which excludes in_progress status
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return "", nil
	}

	var items []BeadInfo
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return "", nil
	}
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id != "" && !strings.EqualFold(strings.TrimSpace(item.IssueType), "epic") {
			return id, nil
		}
	}

	return "", nil
}

// ClaimBead marks a bead as in_progress via bd update.
func ClaimBead(id string) error {
	_, err := runBDCombFn("update", id, "--status=in_progress")
	return err
}

// FetchBeadByID retrieves a single bead by its ID via bd show --json.
func FetchBeadByID(id string) (BeadInfo, error) {
	out, err := runBDFn("show", id, "--json")
	if err != nil {
		return BeadInfo{}, fmt.Errorf("bd show %s failed: %w", id, err)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return BeadInfo{}, fmt.Errorf("bd show %s returned empty output", id)
	}

	// bd show --json returns an array with one element
	if strings.HasPrefix(trimmed, "[") {
		var items []BeadInfo
		if err := json.Unmarshal(out, &items); err != nil {
			return BeadInfo{}, fmt.Errorf("parsing bead %s JSON: %w", id, err)
		}
		if len(items) == 0 {
			return BeadInfo{}, fmt.Errorf("bead %s not found", id)
		}
		return items[0], nil
	}

	// Single object
	var item BeadInfo
	if err := json.Unmarshal(out, &item); err != nil {
		return BeadInfo{}, fmt.Errorf("parsing bead %s JSON: %w", id, err)
	}
	return item, nil
}

// EnsureWorktree checks for an existing worktree for the bead, or creates one.
// It reads state for SpecBranch (to branch from spec instead of main) and config
// for WorktreeRoot (canonical .worktrees/ directory).
// Returns the worktree path. Returns empty string if worktree creation is not
// applicable (e.g., working on main).
func EnsureWorktree(root, beadID string) (string, error) {
	entries, err := worktreeList()
	if err != nil {
		return "", fmt.Errorf("listing worktrees: %w", err)
	}

	// Load config for worktree root path.
	cfg, cfgErr := loadConfigFn(root)
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}

	// Check for existing worktree matching this bead
	wtName := "worktree-" + beadID
	branchName := "bead/" + beadID
	for _, e := range entries {
		if e.Name == wtName || e.Branch == branchName {
			return e.Path, nil
		}
	}

	// Determine the base branch: use spec branch from focus if available.
	baseBranch := "HEAD"
	mc, mcErr := readFocusFn(root)
	if mcErr == nil && mc.SpecBranch != "" {
		baseBranch = mc.SpecBranch
	}

	// Create the bead branch from the spec branch (or HEAD).
	if !branchExistsFn(branchName) {
		if err := createBranchFn(branchName, baseBranch); err != nil {
			return "", fmt.Errorf("creating branch %s from %s: %w", branchName, baseBranch, err)
		}
	}

	// Ensure .worktrees/ directory exists and is gitignored.
	if err := os.MkdirAll(filepath.Join(root, cfg.WorktreeRoot), 0755); err != nil {
		return "", fmt.Errorf("creating %s directory: %w", cfg.WorktreeRoot, err)
	}
	if err := ensureGitignore(root, cfg.WorktreeRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	// Create new worktree via bd worktree create under .worktrees/
	relWtPath := filepath.Join(cfg.WorktreeRoot, wtName)
	if err := worktreeCreate(relWtPath, branchName); err != nil {
		return "", fmt.Errorf("creating worktree: %w", err)
	}

	wtPath := cfg.WorktreePath(root, wtName)

	// Read back path from worktree list to confirm
	entries, err = worktreeList()
	if err == nil {
		for _, e := range entries {
			if e.Name == wtName || strings.HasSuffix(e.Path, wtName) {
				wtPath = e.Path
				break
			}
		}
	}

	// Propagate focus into the bead worktree so commands work from it.
	if mcErr == nil && mc != nil {
		beadMC := *mc // shallow copy
		beadMC.ActiveWorktree = wtPath
		beadMC.ActiveBead = beadID
		beadMC.Mode = state.ModeImplement
		if err := writeFocusFn(wtPath, &beadMC); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write focus to bead worktree: %v\n", err)
		}
	}

	return wtPath, nil
}
