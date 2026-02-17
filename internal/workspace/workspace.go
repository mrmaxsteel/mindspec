package workspace

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrNoRoot is returned when no project root marker is found.
var ErrNoRoot = errors.New("no mindspec project root found (looked for .mindspec/, .git)")

// FindRoot walks up from startDir looking for .mindspec/ or .git at each level.
// It checks .mindspec/ first, then .git, to identify the project root.
func FindRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		if exists(filepath.Join(dir, ".mindspec")) {
			return dir, nil
		}
		if exists(filepath.Join(dir, ".git")) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", ErrNoRoot
}

// DocsDir returns the path to the docs directory under root.
func DocsDir(root string) string {
	canonical := CanonicalDocsDir(root)
	if exists(canonical) {
		return canonical
	}
	return LegacyDocsDir(root)
}

// CanonicalDocsDir returns the canonical docs root under .mindspec.
func CanonicalDocsDir(root string) string {
	return filepath.Join(root, ".mindspec", "docs")
}

// LegacyDocsDir returns the legacy docs root under project root.
func LegacyDocsDir(root string) string {
	return filepath.Join(root, "docs")
}

// GlossaryPath returns the canonical glossary path with legacy fallback.
func GlossaryPath(root string) string {
	canonical := filepath.Join(CanonicalDocsDir(root), "glossary.md")
	if exists(canonical) {
		return canonical
	}
	return filepath.Join(root, "GLOSSARY.md")
}

// SpecDir returns the path to a specific spec directory under root.
func SpecDir(root, specID string) string {
	return filepath.Join(DocsDir(root), "specs", specID)
}

// ContextMapPath returns the path to docs/context-map.md under root.
func ContextMapPath(root string) string {
	return filepath.Join(DocsDir(root), "context-map.md")
}

// ADRDir returns the path to docs/adr/ under root.
func ADRDir(root string) string {
	return filepath.Join(DocsDir(root), "adr")
}

// PoliciesPath returns the canonical path to .mindspec/policies.yml under root.
func PoliciesPath(root string) string {
	return filepath.Join(root, ".mindspec", "policies.yml")
}

// LegacyPoliciesPath returns the legacy path to architecture/policies.yml under root.
// Kept for brownfield read fallback during migration.
func LegacyPoliciesPath(root string) string {
	return filepath.Join(root, "architecture", "policies.yml")
}

// DomainDir returns the path to a specific domain's doc directory under root.
func DomainDir(root, domain string) string {
	return filepath.Join(DocsDir(root), "domains", domain)
}

// RecordingDir returns the path to a spec's recording directory.
func RecordingDir(root, specID string) string {
	return filepath.Join(SpecDir(root, specID), "recording")
}

// MindspecDir returns the path to the .mindspec directory under root.
func MindspecDir(root string) string {
	return filepath.Join(root, ".mindspec")
}

// StatePath returns the path to .mindspec/state.json under root.
func StatePath(root string) string {
	return filepath.Join(root, ".mindspec", "state.json")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
