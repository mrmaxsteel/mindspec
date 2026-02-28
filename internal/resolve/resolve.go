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
// Returns specs sorted by spec ID for deterministic output.
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

		lc, err := state.ReadLifecycle(specDir)
		if err != nil || lc == nil {
			continue
		}

		phase := lc.Phase
		if phase == "" || phase == "done" || phase == state.ModeIdle {
			continue
		}

		active = append(active, SpecStatus{
			SpecID: specID,
			Mode:   phase,
			Active: true,
		})
	}

	sort.Slice(active, func(i, j int) bool {
		return active[i].SpecID < active[j].SpecID
	})

	return active, nil
}

// ResolveMode returns the current lifecycle phase for a spec by reading lifecycle.yaml.
// Returns ModeIdle if no lifecycle file exists.
// Deprecated: callers should read lifecycle.yaml directly via state.ReadLifecycle.
func ResolveMode(root, specID string) (string, error) {
	specsDir := filepath.Join(workspace.DocsDir(root), "specs", specID)
	lc, err := state.ReadLifecycle(specsDir)
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
