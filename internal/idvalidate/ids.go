// Package idvalidate provides pure-string validators for the various
// identifier kinds used in MindSpec: spec IDs, ADR IDs, bead IDs, and
// domain names. It is a leaf package — it imports only the standard
// library — so other internal packages (workspace, validate, …) can
// depend on it without creating import cycles.
//
// These validators are the project-wide chokepoint for any user-supplied
// identifier that flows into filepath.Join, os.ReadFile, os.WriteFile, or
// filepath.Glob. Bypassing them is a SEC-1 hazard (see bead mindspec-x1qr).
//
// The spec ID and bead ID grammars are corrected to match the framework's
// real minting scheme — dotted numeric epic-children at any depth,
// any-length alnum segments, and one optional letter suffix on the spec
// number — as a contract WIDENING (every ID the prior, stricter patterns
// accepted still passes); see ADR-0042 and spec 120-trust-boundary-render-audit
// R1/AC-1 for the live-inventory proof and the rejection-preservation
// argument.
package idvalidate

import (
	"fmt"
	"regexp"
	"strings"
)

// specIDPattern matches valid spec IDs: 3+ digits, one optional letter
// suffix on the number (e.g. "008b", "008c" — MindSpec's own on-disk
// letter-suffixed spec dirs), hyphen, then kebab-case slug.
var specIDPattern = regexp.MustCompile(`^[0-9]{3,}[a-z]?-[a-z0-9]+(?:-[a-z0-9]+)*$`)

// adrIDPattern: ADR-NNNN (4+ digits), optionally with a kebab slug.
// Accepts ADR-0001, ADR-0001-descriptive-slug.
var adrIDPattern = regexp.MustCompile(`^ADR-[0-9]{4,}(?:-[a-z0-9]+(?:-[a-z0-9]+)*)?$`)

// beadIDPattern: <project-slug>-<any-length-alnum-segments>, optionally
// followed by one or more dotted NUMERIC epic-child suffixes at any
// depth. E.g. "mindspec-x1qr", "mindspec-0ke" (3-char suffix),
// "mindspec-mol-015" (multi-segment slug), "mindspec-9cyu.1" (epic
// child), "mindspec-69y.2.2" (nested epic child). Mirrors bd CLI's real
// minting scheme — bd does NOT enforce a minimum suffix length, and bd's
// dotted child-numbering is exactly this hierarchical form (it is not
// merely a flat slug). The explicit segment form still forbids leading
// hyphens and empty segments; the dotted suffix, if present, must be
// `.` followed by one or more digits, so `..` and non-numeric children
// (e.g. "a.b") can never match.
var beadIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)+(?:\.[0-9]+)*$`)

// domainNamePattern: kebab-case, starts with a letter. The explicit
// segment form forbids trailing hyphens (e.g. "cli-") and consecutive
// hyphens (e.g. "a--b") while still accepting valid kebab names.
var domainNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// SpecID validates that id is a well-formed spec identifier (e.g. "033-security-hardening").
// Returns an error if the ID is empty, contains path separators, or doesn't match the expected format.
func SpecID(id string) error {
	if id == "" {
		return fmt.Errorf("invalid spec ID: empty")
	}
	if id == "." || id == ".." {
		return fmt.Errorf("invalid spec ID %q: must match <NNN>-<slug> format", id)
	}
	if strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("invalid spec ID %q: contains path separator", id)
	}
	if !specIDPattern.MatchString(id) {
		return fmt.Errorf("invalid spec ID %q: must match <NNN>-<slug> format", id)
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
		return fmt.Errorf("invalid ADR ID %q: must match ADR-<NNNN>[-<slug>] format", id)
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
		return fmt.Errorf("invalid ADR ID %q: must match ADR-<NNNN>[-<slug>] format", id)
	}
	return nil
}

// BeadID validates a bead identifier (e.g. "mindspec-x1qr", "mindspec-9cyu.1").
//
// Format: beadIDPattern models a bead ID as a hyphen-separated
// <project-slug>-<segment> base (two or more hyphen-separated segments of
// one or more alphanumeric characters each — there is no minimum suffix
// length; bd mints short suffixes like "mindspec-0ke" and multi-segment
// slugs like "mindspec-mol-015"), optionally followed by one or more
// dotted NUMERIC epic-child suffixes at any nesting depth (e.g.
// "mindspec-9cyu.1", "mindspec-69y.2.2"). This mirrors bd's real, nested
// bead ID scheme — bd's own epic/child numbering IS a hierarchical
// format, and this validator models it directly rather than disclaiming
// it. The base segments must be non-empty, so leading, trailing, and
// consecutive hyphens are all rejected; each dotted child must be `.`
// followed by one or more digits, so `..` and non-numeric children (e.g.
// "a.b") can never match — traversal stays structurally impossible.
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
		return fmt.Errorf("invalid bead ID %q: must match <project>-<slug>[.<child>...] format", id)
	}
	if strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("invalid bead ID %q: contains path separator", id)
	}
	if strings.ContainsAny(id, "*?[]{}") {
		return fmt.Errorf("invalid bead ID %q: contains glob metacharacter", id)
	}
	if !beadIDPattern.MatchString(id) {
		return fmt.Errorf("invalid bead ID %q: must match <project>-<slug>[.<child>...] format", id)
	}
	return nil
}

// DomainName validates a domain name (e.g. "security", "cli-handlers").
func DomainName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid domain name: empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid domain name %q: must match <kebab-case> format", name)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("invalid domain name %q: contains path separator", name)
	}
	if !domainNamePattern.MatchString(name) {
		return fmt.Errorf("invalid domain name %q: must match <kebab-case> format", name)
	}
	return nil
}
