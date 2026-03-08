package next

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
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
	runBDFn     = bead.RunBD
	runBDCombFn = bead.RunBDCombined
	listJSONFn  = bead.ListJSON
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

// ResolveActiveBead finds the currently in-progress bead for a spec by querying
// beads for the spec's epic and then finding in-progress children.
// Returns empty string (no error) if no bead is in progress.
func ResolveActiveBead(root, specID string) (string, error) {
	// Find epic via beads metadata query (ADR-0023).
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		return "", nil
	}

	out, err := listJSONFn("--parent", epicID, "--status=in_progress")
	if err != nil {
		return "", nil // No in-progress beads
	}

	var items []BeadInfo
	if err := json.Unmarshal(out, &items); err != nil {
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

// EnsureWorktree creates or reuses a workspace for the given bead via the
// Executor interface. The specID is passed so the executor can branch from
// the spec branch; the branching strategy is the executor's concern.
// Returns the workspace path.
func EnsureWorktree(root, beadID, specID string, exec executor.Executor) (string, error) {
	ws, err := exec.DispatchBead(beadID, specID)
	if err != nil {
		return "", err
	}
	return ws.Path, nil
}
