// Package resolve provides spec targeting and active-spec discovery.
package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
)

// SpecStatus holds the resolved state for a single spec.
type SpecStatus struct {
	SpecID string `json:"spec_id"`
	Mode   string `json:"mode"`
	Active bool   `json:"active"`
}

// ActiveSpecs discovers all specs with active lifecycles by scanning lifecycle.yaml files.
// A spec is active if its lifecycle.yaml exists and phase is not "done" or "idle".
// Scans both the main repo specs directory and worktree specs (ADR-0022).
// Worktree results take priority when a spec exists in both locations.
// Returns specs sorted by spec ID for deterministic output.
func ActiveSpecs(root string) ([]SpecStatus, error) {
	seen := make(map[string]SpecStatus)

	// 1. Scan worktrees first (higher priority per ADR-0022).
	wtRoot := filepath.Join(root, ".worktrees")
	if wtEntries, err := os.ReadDir(wtRoot); err == nil {
		for _, wte := range wtEntries {
			if !wte.IsDir() || !strings.HasPrefix(wte.Name(), "worktree-spec-") {
				continue
			}
			specID := strings.TrimPrefix(wte.Name(), "worktree-spec-")
			specDir := workspace.SpecDir(root, specID)
			if ss, ok := readActiveSpec(specID, specDir); ok {
				seen[specID] = ss
			}
		}
	}

	// 2. Scan main repo specs directory (fallback for specs without worktrees).
	specsDir := filepath.Join(workspace.DocsDir(root), "specs")
	if entries, err := os.ReadDir(specsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			specID := e.Name()
			if _, exists := seen[specID]; exists {
				continue // worktree result takes priority
			}
			specDir := filepath.Join(specsDir, specID)
			if ss, ok := readActiveSpec(specID, specDir); ok {
				seen[specID] = ss
			}
		}
	}

	active := make([]SpecStatus, 0, len(seen))
	for _, ss := range seen {
		active = append(active, ss)
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].SpecID < active[j].SpecID
	})

	return active, nil
}

// readActiveSpec reads lifecycle.yaml from specDir and returns a SpecStatus
// if the spec is active (phase is not empty, done, or idle).
func readActiveSpec(specID, specDir string) (SpecStatus, bool) {
	lc, err := state.ReadLifecycle(specDir)
	if err != nil || lc == nil {
		return SpecStatus{}, false
	}
	phase := lc.Phase
	if phase == "" || phase == "done" || phase == state.ModeIdle {
		return SpecStatus{}, false
	}
	return SpecStatus{SpecID: specID, Mode: phase, Active: true}, true
}

// ResolveMode returns the current lifecycle phase for a spec by reading lifecycle.yaml.
// Returns ModeIdle if no lifecycle file exists.
// Deprecated: callers should read lifecycle.yaml directly via state.ReadLifecycle.
func ResolveMode(root, specID string) (string, error) {
	specDir := workspace.SpecDir(root, specID)
	lc, err := state.ReadLifecycle(specDir)
	if err != nil {
		return state.ModeIdle, err
	}
	if lc == nil || lc.Phase == "" {
		return state.ModeIdle, nil
	}
	return lc.Phase, nil
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
		sb.WriteString(fmt.Sprintf("  %s  phase=%s\n", s.SpecID, s.Mode))
	}
	return sb.String()
}
