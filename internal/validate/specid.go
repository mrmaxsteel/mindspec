package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
)

// SpecID validates that id is a well-formed spec identifier (e.g. "033-security-hardening").
// Thin re-export of idvalidate.SpecID; kept here for API stability of the
// existing `validate.SpecID` call sites.
func SpecID(id string) error {
	return idvalidate.SpecID(id)
}

// ADRID re-exports idvalidate.ADRID for convenience at validate package call sites.
func ADRID(id string) error {
	return idvalidate.ADRID(id)
}

// BeadID re-exports idvalidate.BeadID for convenience at validate package call sites.
func BeadID(id string) error {
	return idvalidate.BeadID(id)
}

// DomainName re-exports idvalidate.DomainName for convenience at validate package call sites.
func DomainName(name string) error {
	return idvalidate.DomainName(name)
}

// SafePath checks that resolved is contained under root after symlink resolution.
// Both paths are resolved to their real (symlink-free) absolute forms before comparison.
//
// This is filesystem-aware (unlike the pure-string validators in idvalidate)
// so it stays in package validate.
func SafePath(root, resolved string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolving root path: %w", err)
	}
	realResolved, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		return fmt.Errorf("resolving target path: %w", err)
	}

	realRoot = filepath.Clean(realRoot) + string(filepath.Separator)
	realResolved = filepath.Clean(realResolved)

	if !strings.HasPrefix(realResolved+string(filepath.Separator), realRoot) && realResolved != filepath.Clean(realRoot[:len(realRoot)-1]) {
		return fmt.Errorf("path %q escapes project root %q", resolved, root)
	}
	return nil
}
