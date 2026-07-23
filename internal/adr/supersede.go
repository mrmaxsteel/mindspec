package adr

import (
	"fmt"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Supersede updates the old ADR's Superseded-by field to point to newID.
// SEC-1 (bead mindspec-x1qr): both IDs are validated before path construction.
// newID is normally derived from internal NextID output (safe), but Supersede
// is also exported via FileStore.Supersede so we validate defensively.
//
// oldID resolution is READ-resolution (spec 123 R5(c)): oldID may be a
// canonical ID whose on-disk file is slugged (e.g. superseding
// "ADR-0001" when the file is "ADR-0001-integrate-at-contracts.md"), so
// it routes through workspace.ResolveADRFile rather than the exact-join
// ADRFilePath. The resolved path is then read AND written back to in
// place — updating an existing file's Superseded-by field is not new
// file emission.
func Supersede(root, oldID, newID string) error {
	if err := idvalidate.ADRID(oldID); err != nil {
		return fmt.Errorf("invalid old ADR ID: %w", err)
	}
	if err := idvalidate.ADRID(newID); err != nil {
		return fmt.Errorf("invalid new ADR ID: %w", err)
	}
	oldPath, err := workspace.ResolveADRFile(root, oldID)
	if err != nil {
		return err
	}
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
// SEC-1: validates id before path construction (the join previously enabled
// a read-arbitrary-*.md primitive via the same traversal pattern as
// --supersedes). Resolution routes through workspace.ResolveADRFile (spec
// 123 R5(c)): this is a READ of a possibly-slugged existing file, not new
// file emission, so the superseded ADR's domain list is found even when
// its filename carries a slug.
func CopyDomains(root, id string) ([]string, error) {
	path, err := workspace.ResolveADRFile(root, id)
	if err != nil {
		return nil, err
	}
	a, err := ParseADR(path)
	if err != nil {
		return nil, err
	}
	return a.Domains, nil
}
