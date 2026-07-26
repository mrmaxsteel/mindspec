package lifecycle

import (
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

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

// evaluateWorkDestructionFn is the UNEXPORTED seam pointer-pinned to the
// real implementation (gitquery_test.go's
// TestEvaluateWorkDestruction_WrapperPinnedToImplementation) — spec 127
// bead-1 fix round, G1-2: this used to be the EXPORTED wrapper itself, a
// package-level `var`. A pointer-equality snapshot test only proves the
// two sides matched at the INSTANT the test ran; it neither prevents nor
// detects a consumer package assigning a divergent implementation
// afterward, and an *exported* mutable func-valued var additionally lets
// any package under the module repoint it — G1 proved both: an
// external-package probe compiled `lifecycle.EvaluateWorkDestruction =
// otherImpl` successfully, and running the suite with `-race` reported a
// genuine DATA RACE between that write and EvaluateWorkDestruction's own
// read. Keeping this seam UNEXPORTED closes the rewiring hole — no
// package outside internal/lifecycle can even name it — while the
// pointer-equality test below still catches accidental DRIFT (someone
// changing this line to call a different function) at build-and-test
// time, in-package, same as before.
var evaluateWorkDestructionFn = gitutil.EvaluateWorkDestruction

// EvaluateWorkDestruction is the ADR-0030 boundary wrapper over the spec
// 127 shared work-destruction predicate (internal/gitutil.
// EvaluateWorkDestruction): the enforcement packages (internal/approve,
// internal/complete) consult work-destruction facts through this symbol
// rather than importing internal/gitutil directly.
//
// Deliberately an immutable function declaration, NOT a package-level
// `var` (spec 127 bead-1 fix round, G1-2 — see evaluateWorkDestructionFn's
// doc comment for why the prior exported-var shape was withdrawn): no
// consumer package can substitute a divergent implementation behind this
// name, because there is no assignable symbol to substitute. It simply
// calls through to the pinned in-package seam.
func EvaluateWorkDestruction(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
	return evaluateWorkDestructionFn(workdir, branch, target)
}
