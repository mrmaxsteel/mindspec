package validate

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"gopkg.in/yaml.v3"
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
func SpecStatus(root, specID string) string {
	specPath := filepath.Join(workspace.SpecDir(root, specID), "spec.md")
	return specStatusFromPath(specPath)
}

// SpecStatusAt is SpecStatus but keyed off an already-known spec directory
// (useful in callers that have a *workspace.SpecDir* value in hand and want
// to avoid re-derivation).
func SpecStatusAt(specDir string) string {
	return specStatusFromPath(filepath.Join(specDir, "spec.md"))
}

func specStatusFromPath(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return SpecStatusFromBytes(data)
}

// SpecStatusFromBytes parses the YAML frontmatter of already-loaded spec.md
// bytes and returns the status field. Empty string on any parse failure.
func SpecStatusFromBytes(data []byte) string {
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	var fmLines []string
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		fmLines = append(fmLines, line)
	}
	var fm struct {
		Status string `yaml:"status"`
	}
	if err := yaml.Unmarshal([]byte(strings.Join(fmLines, "\n")), &fm); err != nil {
		return ""
	}
	return strings.TrimSpace(fm.Status)
}

// SpecIsApproved reports whether a spec.md's YAML frontmatter has status
// set to Approved (case-insensitive). Convenience wrapper over SpecStatus.
func SpecIsApproved(root, specID string) bool {
	return strings.EqualFold(SpecStatus(root, specID), "Approved")
}
