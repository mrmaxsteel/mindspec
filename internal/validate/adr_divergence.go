package validate

import (
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// CheckADRDivergence is a named-symbol stub for the ADR-divergence lane that
// spec-087 (F1) will fill in. The body is intentionally empty; this stub
// exists so spec-086 Bead 3 can wire the call site and so AST-based
// call-order tests have an anchor symbol to assert against.
//
// Contract (forward-looking, not implemented here):
//   - Walk ADRs under .mindspec/docs/adr/**.md
//   - Detect divergence between accepted ADRs and code/docs that contradict
//     them in the current diff (diffRef)
//   - Emit findings under the "adr-divergence" sub-command
//
// Until spec-087 lands, callers receive an empty *Result so the gate stays
// neutral.
func CheckADRDivergence(root, diffRef string, exec executor.Executor) *Result {
	_ = root
	_ = diffRef
	_ = exec
	return &Result{SubCommand: "adr-divergence"}
}
