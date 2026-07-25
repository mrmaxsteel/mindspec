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

// EvaluateWorkDestruction is the ADR-0030 boundary wrapper over the spec
// 127 shared work-destruction predicate (internal/gitutil.
// EvaluateWorkDestruction): the enforcement packages (internal/approve,
// internal/complete) consult work-destruction facts through this symbol
// rather than importing internal/gitutil directly.
//
// Deliberately a package-level `var`, NOT a `func` — a pointer-equality
// test in gitquery_test.go asserts wrapper ≡ implementation
// (`reflect.ValueOf(EvaluateWorkDestruction).Pointer() ==
// reflect.ValueOf(gitutil.EvaluateWorkDestruction).Pointer()`), the same
// no-second-rewirable-seam discipline the spec-121 AC-17 anti-drift
// consumer set already enforces for gitutil.NetEffectLanded: this pins,
// machine-checked, that no divergent reimplementation can ever be swapped
// in behind this name.
var EvaluateWorkDestruction = gitutil.EvaluateWorkDestruction
