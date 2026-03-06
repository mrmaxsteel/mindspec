package speclist

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
		status := readFrontmatterStatus(filepath.Join(specsDir, specID, "spec.md"))
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

// readFrontmatterStatus reads the status field from YAML frontmatter.
func readFrontmatterStatus(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "unknown"
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			if inFrontmatter {
				break
			}
			inFrontmatter = true
			continue
		}
		if inFrontmatter && strings.HasPrefix(line, "status:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "status:"))
		}
	}
	return "unknown"
}
