package validate

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// specIDPattern matches valid spec IDs: 3+ digits, hyphen, then kebab-case slug.
var specIDPattern = regexp.MustCompile(`^[0-9]{3,}-[a-z0-9]+(?:-[a-z0-9]+)*$`)

// SpecID validates that id is a well-formed spec identifier (e.g. "033-security-hardening").
// Returns an error if the ID is empty, contains path separators, or doesn't match the expected format.
func SpecID(id string) error {
	if id == "" {
		return fmt.Errorf("invalid spec ID: empty")
	}
	if id == "." || id == ".." {
		return fmt.Errorf("invalid spec ID %q: must match NNN-slug format", id)
	}
	if strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("invalid spec ID %q: contains path separator", id)
	}
	if !specIDPattern.MatchString(id) {
		return fmt.Errorf("invalid spec ID %q: must match NNN-slug format", id)
	}
	return nil
}

// SafePath checks that resolved is contained under root after symlink resolution.
// Both paths are resolved to their real (symlink-free) absolute forms before comparison.
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
