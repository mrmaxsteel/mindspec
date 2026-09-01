// Spec 127 bead-1 fix round 2 (ruling 3): the compile-time probe for
// G1-2's wrapper-mutability fix — the FOURTH of the four original
// BLOCKING classes. This file's only purpose is to FAIL TO COMPILE:
// gitquery_test.go's TestEvaluateWorkDestruction_ExternalPackageCannotReassign
// shells `go build` on this directory and asserts a non-zero exit naming
// "cannot assign to lifecycle.EvaluateWorkDestruction".
//
// Manually verified as the red-on-revert case for this class: reverting
// internal/lifecycle/gitquery.go to the pre-fix shape (`var
// EvaluateWorkDestruction = gitutil.EvaluateWorkDestruction`, ab5aca11's
// shape) via `go build -overlay` and re-running this exact probe makes
// `go build` exit 0 — the wrapper-mutability class the permanent test
// pins closed against the CURRENT (immutable func) shape.
package g1rewireprobe

import (
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// Rewire attempts the exact reassignment G1 proved possible against the
// pre-fix exported `var`. It is never called — its only role is to make
// this package fail to compile against the fixed (func) shape.
func Rewire() {
	lifecycle.EvaluateWorkDestruction = func(workdir, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
		return guard.DestructionClean, gitutil.WorkDestructionEvidence{}, nil
	}
}
