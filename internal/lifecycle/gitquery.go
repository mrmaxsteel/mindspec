package lifecycle

import "github.com/mrmaxsteel/mindspec/internal/gitutil"

// IsAncestor reports whether ancestor is an ancestor of descendant in the
// git repo at workdir. Thin wrapper so ADR-0030 enforcement packages
// (e.g. internal/approve) can consult ancestry without importing
// internal/gitutil directly (internal/lint boundary).
func IsAncestor(workdir, ancestor, descendant string) (bool, error) {
	return gitutil.IsAncestor(workdir, ancestor, descendant)
}

// BranchExists reports whether a local branch <name> exists. Thin wrapper
// (same ADR-0030 boundary rationale as IsAncestor).
func BranchExists(name string) bool {
	return gitutil.BranchExists(name)
}
