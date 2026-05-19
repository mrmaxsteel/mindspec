// Package idvalidate provides pure-string validators for the various
// identifier kinds used in MindSpec: spec IDs, ADR IDs, bead IDs, and
// domain names. It is a leaf package — it imports only the standard
// library — so other internal packages (workspace, validate, …) can
// depend on it without creating import cycles.
//
// These validators are the project-wide chokepoint for any user-supplied
// identifier that flows into filepath.Join, os.ReadFile, os.WriteFile, or
// filepath.Glob. Bypassing them is a SEC-1 hazard (see bead mindspec-x1qr).
package idvalidate

import (
	"fmt"
	"regexp"
	"strings"
)

// specIDPattern matches valid spec IDs: 3+ digits, hyphen, then kebab-case slug.
var specIDPattern = regexp.MustCompile(`^[0-9]{3,}-[a-z0-9]+(?:-[a-z0-9]+)*$`)

// adrIDPattern: ADR-NNNN (4+ digits), optionally with a kebab slug.
// Accepts ADR-0001, ADR-0001-descriptive-slug.
var adrIDPattern = regexp.MustCompile(`^ADR-[0-9]{4,}(?:-[a-z0-9]+(?:-[a-z0-9]+)*)?$`)

// beadIDPattern: <project-slug>-<4+ alphanum>. E.g. mindspec-x1qr.
// Mirrors bd CLI bead ID format. Current bd uses exactly 4 alphanumeric
// suffix characters; {4,} keeps us forward-compatible if that widens.
// The explicit segment form forbids leading hyphens and empty segments.
var beadIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*-[a-z0-9]{4,}$`)

// domainNamePattern: kebab-case, starts with letter.
var domainNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

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

// ADRID validates an ADR identifier (e.g. "ADR-0001" or "ADR-0001-descriptive-slug").
// Rejects path separators and glob metacharacters because adr.Show feeds the
// id directly into filepath.Glob — a bare `*` or `?` would inject a glob pattern.
func ADRID(id string) error {
	if id == "" {
		return fmt.Errorf("invalid ADR ID: empty")
	}
	if id == "." || id == ".." {
		return fmt.Errorf("invalid ADR ID %q: must match ADR-NNNN[-slug] format", id)
	}
	if strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("invalid ADR ID %q: contains path separator", id)
	}
	// Reject shell/glob metacharacters. * ? [ ] are interpreted by
	// filepath.Glob; \ escapes glob patterns on POSIX and is a path
	// separator on Windows. { } are not interpreted by Go's filepath.Glob
	// but bash callers may pre-expand them — defense in depth.
	if strings.ContainsAny(id, "*?[]{}") {
		return fmt.Errorf("invalid ADR ID %q: contains glob metacharacter", id)
	}
	if !adrIDPattern.MatchString(id) {
		return fmt.Errorf("invalid ADR ID %q: must match ADR-NNNN[-slug] format", id)
	}
	return nil
}

// BeadID validates a bead identifier (e.g. "mindspec-x1qr").
//
// Note: as of SEC-1 (bead mindspec-x1qr) implementation, no production code
// passes bead IDs into filepath.Join. This validator is preventative — it
// closes the door before any future code paths construct paths from bead
// IDs derived from external input.
func BeadID(id string) error {
	if id == "" {
		return fmt.Errorf("invalid bead ID: empty")
	}
	if id == "." || id == ".." {
		return fmt.Errorf("invalid bead ID %q: must match <project>-<4+alnum> format", id)
	}
	if strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("invalid bead ID %q: contains path separator", id)
	}
	if strings.ContainsAny(id, "*?[]{}") {
		return fmt.Errorf("invalid bead ID %q: contains glob metacharacter", id)
	}
	if !beadIDPattern.MatchString(id) {
		return fmt.Errorf("invalid bead ID %q: must match <project>-<4+alnum> format", id)
	}
	return nil
}

// DomainName validates a domain name (e.g. "security", "cli-handlers").
func DomainName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid domain name: empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid domain name %q: must match [a-z][a-z0-9-]*", name)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("invalid domain name %q: contains path separator", name)
	}
	if !domainNamePattern.MatchString(name) {
		return fmt.Errorf("invalid domain name %q: must match [a-z][a-z0-9-]*", name)
	}
	return nil
}
