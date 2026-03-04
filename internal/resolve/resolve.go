// Package resolve provides spec targeting and active-spec discovery.
package resolve

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mindspec/mindspec/internal/phase"
	"github.com/mindspec/mindspec/internal/state"
)

// SpecStatus holds the resolved state for a single spec.
type SpecStatus struct {
	SpecID string `json:"spec_id"`
	Mode   string `json:"mode"`
	Active bool   `json:"active"`
}

// ActiveSpecs discovers all specs with active lifecycles by querying beads for open epics.
// Returns specs sorted by spec ID for deterministic output.
func ActiveSpecs(root string) ([]SpecStatus, error) {
	activeSpecs, err := phase.DiscoverActiveSpecs()
	if err != nil {
		return nil, err
	}

	result := make([]SpecStatus, 0, len(activeSpecs))
	for _, as := range activeSpecs {
		if as.Phase == "" || as.Phase == "done" || as.Phase == state.ModeIdle {
			continue
		}
		result = append(result, SpecStatus{SpecID: as.SpecID, Mode: as.Phase, Active: true})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].SpecID < result[j].SpecID
	})

	return result, nil
}

// ResolveMode returns the current lifecycle phase for a spec by querying beads.
// Returns ModeIdle if no epic exists for the spec.
func ResolveMode(root, specID string) (string, error) {
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		return state.ModeIdle, nil
	}
	derivedPhase, err := phase.DerivePhase(epicID)
	if err != nil {
		return state.ModeIdle, err
	}
	return derivedPhase, nil
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
