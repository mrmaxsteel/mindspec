// Package resolve derives per-spec lifecycle mode from Beads molecule state (ADR-0015).
package resolve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/specmeta"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
)

// SpecStatus holds the resolved state for a single spec.
type SpecStatus struct {
	SpecID     string `json:"spec_id"`
	MoleculeID string `json:"molecule_id"`
	Mode       string `json:"mode"`
	Active     bool   `json:"active"`
}

// molShowResult is the JSON output from `bd mol show <id> --json`.
type molShowResult struct {
	Issues []molIssue `json:"issues"`
	Root   molIssue   `json:"root"`
}

type molIssue struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	IssueType string `json:"issue_type"`
}

// ResolveMode derives the lifecycle mode for a spec from its molecule's step statuses.
// Returns the mode string (spec/plan/implement/review/idle) or an error.
// This makes exactly one Beads CLI call (bd mol show).
func ResolveMode(root, specID string) (string, error) {
	meta, err := specmeta.EnsureBound(root, specID)
	if err != nil {
		return "", fmt.Errorf("resolving molecule binding for %s: %w", specID, err)
	}
	if meta == nil || meta.MoleculeID == "" {
		return "", fmt.Errorf("spec %s has no molecule binding", specID)
	}

	stepStatuses, err := fetchStepStatuses(meta.MoleculeID)
	if err != nil {
		return "", fmt.Errorf("fetching molecule steps for %s: %w", specID, err)
	}

	return deriveMode(meta.StepMapping, stepStatuses), nil
}

// IsActive returns whether a spec is actively being worked on.
// A spec is inactive if:
//   - it has no molecule binding
//   - its review step is closed (lifecycle complete)
//
// Otherwise it is active if any lifecycle step is not closed.
func IsActive(root, specID string) (bool, error) {
	meta, err := specmeta.ReadForSpec(root, specID)
	if err != nil {
		return false, err
	}
	if meta.MoleculeID == "" {
		return false, nil
	}

	stepStatuses, err := fetchStepStatuses(meta.MoleculeID)
	if err != nil {
		return false, err
	}

	return isActive(meta.StepMapping, stepStatuses), nil
}

// ActiveSpecs discovers all specs with active molecules.
// Returns specs sorted by spec ID for deterministic output.
// Makes one Beads CLI call per spec that has a molecule binding.
func ActiveSpecs(root string) ([]SpecStatus, error) {
	specsDir := filepath.Join(workspace.DocsDir(root), "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil, fmt.Errorf("reading specs directory: %w", err)
	}

	var active []SpecStatus
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		specID := e.Name()
		specDir := filepath.Join(specsDir, specID)
		if _, err := os.Stat(filepath.Join(specDir, "spec.md")); err != nil {
			continue
		}

		meta, err := specmeta.Read(specDir)
		if err != nil || meta.MoleculeID == "" {
			continue
		}

		stepStatuses, err := fetchStepStatuses(meta.MoleculeID)
		if err != nil {
			continue // skip specs with inaccessible molecules
		}

		if !isActive(meta.StepMapping, stepStatuses) {
			continue
		}

		mode := deriveMode(meta.StepMapping, stepStatuses)
		active = append(active, SpecStatus{
			SpecID:     specID,
			MoleculeID: meta.MoleculeID,
			Mode:       mode,
			Active:     true,
		})
	}

	// Deterministic ordering by spec ID
	sort.Slice(active, func(i, j int) bool {
		return active[i].SpecID < active[j].SpecID
	})

	return active, nil
}

// fetchStepStatuses queries `bd mol show <molID> --json` and returns a map
// from issue ID to status. This is the single Beads CLI call per resolution.
func fetchStepStatuses(molID string) (map[string]string, error) {
	out, err := bead.RunBD("mol", "show", molID, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd mol show %s: %w", molID, err)
	}

	var result molShowResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parsing mol show output: %w", err)
	}

	statuses := make(map[string]string, len(result.Issues))
	for _, issue := range result.Issues {
		statuses[issue.ID] = issue.Status
	}
	return statuses, nil
}

// deriveMode maps molecule step statuses to a MindSpec mode.
// Steps are checked in forward lifecycle order; the earliest non-closed step
// determines the mode.
//
// Lifecycle order: spec → spec-approve → plan → plan-approve → implement → review
//
// If stepMapping is nil/empty, returns idle.
func deriveMode(stepMapping map[string]string, statuses map[string]string) string {
	// Forward lifecycle order: earliest non-closed step determines mode
	type stepMode struct {
		step string
		mode string
	}
	lifecycle := []stepMode{
		{"spec", state.ModeSpec},
		{"spec-approve", state.ModeSpec},
		{"plan", state.ModePlan},
		{"plan-approve", state.ModePlan},
		{"implement", state.ModeImplement},
		{"review", state.ModeReview},
	}

	if len(stepMapping) > 0 {
		for _, sm := range lifecycle {
			beadID, ok := stepMapping[sm.step]
			if !ok {
				continue
			}
			status, found := statuses[beadID]
			if !found {
				continue
			}
			if status != "closed" {
				return sm.mode
			}
		}
		// All steps closed → idle (done)
		return state.ModeIdle
	}

	return state.ModeIdle
}

// isActive implements the active-spec predicate:
//   - inactive if review step is closed (lifecycle complete)
//   - active if any lifecycle step is not closed
func isActive(stepMapping map[string]string, statuses map[string]string) bool {
	if len(stepMapping) == 0 {
		// No step mapping → check if any non-root issue is not closed
		for _, status := range statuses {
			if status != "closed" {
				return true
			}
		}
		return false
	}

	// Check review step — if closed, lifecycle is complete
	if reviewID, ok := stepMapping["review"]; ok {
		if status, found := statuses[reviewID]; found && status == "closed" {
			return false
		}
	}

	// Check if any lifecycle step is not closed
	lifecycleSteps := []string{"spec", "spec-approve", "plan", "plan-approve", "implement", "review"}
	for _, step := range lifecycleSteps {
		beadID, ok := stepMapping[step]
		if !ok {
			continue
		}
		status, found := statuses[beadID]
		if !found {
			continue
		}
		if status != "closed" {
			return true
		}
	}

	return false
}

// ResolveActiveBead returns the in-progress bead ID for a spec's implement step.
// Returns "" if no bead is in_progress, or an error if multiple are (ambiguous).
func ResolveActiveBead(root, specID string) (string, error) {
	meta, err := specmeta.EnsureFullyBound(root, specID)
	if err != nil {
		return "", fmt.Errorf("resolving molecule for %s: %w", specID, err)
	}

	implStepID, ok := meta.StepMapping["implement"]
	if !ok {
		return "", fmt.Errorf("spec %s has no implement step in molecule", specID)
	}

	out, err := bead.RunBD("list", "--status=in_progress", "--parent", implStepID, "--json")
	if err != nil {
		return "", fmt.Errorf("querying in-progress beads for %s: %w", specID, err)
	}

	var results []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &results); err != nil {
		return "", fmt.Errorf("parsing bead list output: %w", err)
	}

	switch len(results) {
	case 0:
		return "", nil
	case 1:
		return results[0].ID, nil
	default:
		ids := make([]string, len(results))
		for i, r := range results {
			ids[i] = r.ID
		}
		return "", fmt.Errorf("ambiguous: multiple in-progress beads for %s: %v", specID, ids)
	}
}

// ResolveSpecBranch returns the canonical branch name for a spec.
func ResolveSpecBranch(specID string) string {
	return state.SpecBranch(specID)
}

// ResolveWorktree returns the canonical worktree path for a spec.
func ResolveWorktree(root, specID string) string {
	return state.SpecWorktreePath(root, specID)
}

// FormatActiveList produces a human-readable list of active specs.
func FormatActiveList(specs []SpecStatus) string {
	if len(specs) == 0 {
		return "No active specs found.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Active specs (%d):\n", len(specs)))
	for _, s := range specs {
		sb.WriteString(fmt.Sprintf("  %s  mode=%s  molecule=%s\n", s.SpecID, s.Mode, s.MoleculeID))
	}
	return sb.String()
}
