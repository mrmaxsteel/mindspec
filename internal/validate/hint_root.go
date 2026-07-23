// Package validate.
//
// hint_root.go provides domainsRootLabel, the single layout-aware helper
// that renders the domains-enumeration root as a workspace-RELATIVE label
// for operator-facing gate messages (spec 122 R1/R4). It exists so a gate
// message can print the domains root that ACTUALLY resolves in the
// operator's workspace — flat `.mindspec/domains` (spec 106), canonical
// `.mindspec/docs/domains` pre-flatten, or legacy `docs/domains` — instead
// of a hard-coded pre-flatten literal.
//
// This bead (spec 122 Bead 1) introduces the helper because Requirement
// 1's forward-only reject error is the FIRST gate message that must print
// a true domains root; Bead 3 wires the three remaining hint sites
// (divergence.go's unowned-claim remedy, docsync.go's two internal-docs
// templates) through this same helper and adds the hard-coded-literal
// sweep guard (R4a).
package validate

import (
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// domainsRootLabel returns the workspace-relative label of the domains
// enumeration root that workspace.DomainsDir(root) resolves to in this
// workspace, mirroring DomainsDir's own flat -> canonical -> legacy
// precedence (workspace.go:617) exactly — no new resolution order is
// introduced here, only a relative rendering of the existing one.
//
// The label always uses forward slashes (filepath.ToSlash) so a gate
// message reads identically on Windows and POSIX, matching the existing
// operator-facing path conventions elsewhere in this package.
func domainsRootLabel(root string) string {
	abs := workspace.DomainsDir(root)
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		// Unreachable in practice: DomainsDir always joins its result onto
		// root, so Rel(root, abs) cannot fail for a well-formed root. If it
		// ever does, fall back to the absolute path rather than print an
		// empty or misleading label.
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}
