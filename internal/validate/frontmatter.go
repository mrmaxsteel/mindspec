package validate

import (
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/frontmatter"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// SpecStatus returns the status field from the YAML frontmatter of a spec.md
// file, or an empty string if the spec is missing / malformed / has no
// frontmatter. The returned value is trimmed of surrounding whitespace but
// its case is preserved — callers decide how to compare.
//
// This helper exists so any code that needs the declared spec status parses
// the contract (YAML frontmatter) rather than substring-matching raw markdown
// prose, which was the original ZFC violation in next/mode.go and
// approve/plan.go.
//
// Delegates to internal/frontmatter (ARCH-6 / mindspec-npd2).
func SpecStatus(root, specID string) string {
	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return ""
	}
	specPath := filepath.Join(specDir, "spec.md")
	return specStatusFromPath(specPath)
}

// SpecStatusAt is SpecStatus but keyed off an already-known spec directory
// (useful in callers that have a *workspace.SpecDir* value in hand and want
// to avoid re-derivation).
func SpecStatusAt(specDir string) string {
	return specStatusFromPath(filepath.Join(specDir, "spec.md"))
}

func specStatusFromPath(path string) string {
	return frontmatter.StatusFromPath(path)
}

// SpecStatusFromBytes parses the YAML frontmatter of already-loaded spec.md
// bytes and returns the status field. Empty string on any parse failure.
//
// Delegates to internal/frontmatter (ARCH-6 / mindspec-npd2).
func SpecStatusFromBytes(data []byte) string {
	return frontmatter.Status(data)
}

// SpecIsApproved reports whether a spec.md's YAML frontmatter has status
// set to Approved (case-insensitive). Convenience wrapper over SpecStatus.
func SpecIsApproved(root, specID string) bool {
	return strings.EqualFold(SpecStatus(root, specID), "Approved")
}
