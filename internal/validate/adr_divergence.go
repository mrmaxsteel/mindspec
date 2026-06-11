package validate

import (
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// CheckADRDivergence is the Spec 087 Bead 2 implementation of the
// ADR-divergence enforcement lane. It bridges the executor.Executor
// + diff-ref idiom used by callers in internal/{complete,approve} to
// the package-internal ValidateDivergence helper that does the actual
// work.
//
// Behavior:
//   - When `specDir` is empty the function cannot load citations or
//     impacted-domains; it returns a *Result with a single
//     `adr-divergence-load` error and nil findings.
//   - When `beadID` is non-empty (complete.Run path) the diff range is
//     base..HEAD — the bead's commits relative to the spec branch.
//     Proposed-only ADR coverage is an advisory WARNING on this lane
//     (panel condition C1 on mindspec-53qx).
//   - When `beadID` is empty (approve.ImplOptions impl backstop) the
//     diff range is base..<spec branch tip>; the spec branch is
//     derived from filepath.Base(specDir) via workspace.SpecBranch.
//     Proposed-only ADR coverage is an ERROR on this lane — the
//     implementation ships here, so the cited Proposed ADR must be
//     flipped to Accepted (or --override-adr / --supersede-adr used).
//
// The SubCommand label "adr-divergence" is preserved across the
// transition from the spec-086 stub for HC-3 traceability.
func CheckADRDivergence(
	root, diffRef string,
	exec executor.Executor,
	specDir string,
	beadID string,
) (*Result, []DivergenceFinding) {
	r := &Result{SubCommand: "adr-divergence", TargetID: beadID}
	if specDir == "" {
		r.AddError("adr-divergence-load", "specDir required")
		return r, nil
	}

	headRef := "HEAD"
	implApprove := beadID == ""
	if implApprove {
		// Impl backstop path: scan the full spec branch tip vs the
		// caller-supplied base (typically merge-base with main).
		headRef = workspace.SpecBranch(filepath.Base(specDir))
	}

	return ValidateDivergence(exec, root, specDir, beadID, diffRef, headRef, implApprove)
}
