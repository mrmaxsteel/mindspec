package spec

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/mrmaxsteel/mindspec/internal/frontmatter"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// SpecEntry represents a single spec with its status and lifecycle phase.
type SpecEntry struct {
	SpecID string `json:"spec_id"`
	Status string `json:"status"`
	Phase  string `json:"phase"`
}

// List scans the specs directory and returns all specs with their status and phase.
func List(root string) ([]SpecEntry, error) {
	specsDir := filepath.Join(workspace.DocsDir(root), "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil, err
	}

	var specs []SpecEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		specID := e.Name()
		status := frontmatter.StatusFromPath(filepath.Join(specsDir, specID, "spec.md"))
		// Preserve the "unknown" sentinel for CLI display: a blank Status column
		// looks broken in tables. The parser drops the sentinel; the table keeps it.
		if status == "" {
			status = "unknown"
		}
		ph := derivePhase(specID)

		specs = append(specs, SpecEntry{
			SpecID: specID,
			Status: status,
			Phase:  ph,
		})
	}

	sort.Slice(specs, func(i, j int) bool {
		return specs[i].SpecID < specs[j].SpecID
	})

	return specs, nil
}

// derivePhase queries beads for the spec's lifecycle phase.
func derivePhase(specID string) string {
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		return "—"
	}
	ph, err := phase.DerivePhase(epicID)
	if err != nil || ph == "" {
		return "—"
	}
	return ph
}
