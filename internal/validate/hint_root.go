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

	"github.com/mrmaxsteel/mindspec/internal/executor"
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

// domainsRootLabelAtRef is the ref-CONSISTENT sibling of
// domainsRootLabel (spec 122 Bead 3, FX-2). The bead-time divergence
// and per-bead doc-sync lanes enumerate ownership from a git ref
// (ownerRef — beadHead / the spec-branch tip), NOT the ambient
// checkout. If that ref's docs layout differs from the ambient working
// tree (e.g. a flat ref while the local checkout is still canonical, or
// vice versa), rendering the ambient domainsRootLabel(root) would send
// the operator to the WRONG root. This resolves the label from the SAME
// tree the ownership enumeration read, mirroring the flat -> canonical
// -> legacy precedence of workspace.DomainsDir by reusing the EXISTING
// ref-anchored seam (domainsTreeRoots + executor.TreeDirsAtRef — the
// same mechanism listDomainDirsAtRef uses); no new machinery.
//
// It returns the FIRST domains-enumeration root (in flat -> canonical
// -> legacy precedence) that holds domain directories in ref's tree.
// The authoring / working-tree call sites (ownerRef == "" — e.g.
// ownership_resolve.go's Requirement-1 error, or a nil executor) keep
// the ambient domainsRootLabel(root) behavior byte-identically. A git
// read failure, or a ref with no populated domains tree at all, also
// falls back to the ambient label rather than guessing a root that
// isn't there.
func domainsRootLabelAtRef(exec executor.Executor, root, ownerRef string) string {
	if ownerRef == "" || exec == nil {
		return domainsRootLabel(root)
	}
	for _, treeRoot := range domainsTreeRoots {
		dirs, err := exec.TreeDirsAtRef(ownerRef, treeRoot)
		if err != nil {
			// Git read failure: fall back to the ambient label rather
			// than pick a root the ref may not actually carry.
			return domainsRootLabel(root)
		}
		if len(dirs) > 0 {
			// domainsTreeRoots entries are already forward-slash,
			// workspace-relative git paths — the same label shape
			// domainsRootLabel emits.
			return treeRoot
		}
	}
	// No domains tree populated at the ref: ambient label is the best
	// available "create your first domain dir here" hint.
	return domainsRootLabel(root)
}
