package adr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Supersede updates the old ADR's Superseded-by field to point to newID.
func Supersede(root, oldID, newID string) error {
	oldPath := filepath.Join(workspace.ADRDir(root), oldID+".md")
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", oldID, err)
	}

	content := string(data)
	updated := strings.Replace(content, "**Superseded-by**: n/a", fmt.Sprintf("**Superseded-by**: %s", newID), 1)
	if updated == content {
		// Try without n/a — maybe the field has a different value
		// Just find the line and replace the value
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.Contains(line, "**Superseded-by**:") {
				// Replace everything after the key
				idx := strings.Index(line, "**Superseded-by**:")
				lines[i] = line[:idx] + "**Superseded-by**: " + newID
				break
			}
		}
		updated = strings.Join(lines, "\n")
	}

	return os.WriteFile(oldPath, []byte(updated), 0o644)
}

// CopyDomains reads an ADR and returns its domain list.
func CopyDomains(root, id string) ([]string, error) {
	path := filepath.Join(workspace.ADRDir(root), id+".md")
	a, err := ParseADR(path)
	if err != nil {
		return nil, err
	}
	return a.Domains, nil
}
