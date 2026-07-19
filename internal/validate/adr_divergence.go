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
//     base..headRef where headRef is the bead branch tip the caller
//     resolved (or the canonical workspace.BeadBranch(beadID) when
//     headRef is empty). The range is anchored to the BEAD's work —
//     never the ambient HEAD of whatever checkout the process runs
//     from, which measured main-side drift from the repo root (false
//     blocks, mindspec-aqey) and an empty range from the spec worktree
//     (vacuous passes, mindspec-perm). Proposed-only ADR coverage is
//     an advisory WARNING on this lane (panel condition C1 on
//     mindspec-53qx).
//   - When `beadID` is empty (approve.ImplOptions impl backstop) the
//     diff range is base..<spec branch tip>; the spec branch is
//     derived from filepath.Base(specDir) via workspace.SpecBranch.
//     Proposed-only ADR coverage is an ERROR on this lane — the
//     implementation ships here, so the cited Proposed ADR must be
//     flipped to Accepted (or --override-adr / --supersede-adr used).
//
// The SubCommand label "adr-divergence" is preserved across the
// transition from the spec-086 stub for HC-3 traceability.
// ownerRef (spec 095 / mindspec-vvs9) is the git ref the OWNERSHIP
// attribution input is read from — INDEPENDENT of the diff range. The
// per-bead caller passes beadHead; the impl-approve backstop passes the
// spec-branch tip; "" preserves the on-disk working-tree read. Do not
// assume ownerRef == the diff head: they are wired separately at each
// call site even when they presently coincide.
func CheckADRDivergence(
	root, diffRef string,
	exec executor.Executor,
	specDir string,
	beadID string,
	headRef string,
	ownerRef string,
) (*Result, []DivergenceFinding) {
	r := &Result{SubCommand: "adr-divergence", TargetID: beadID}
	if specDir == "" {
		r.AddError("adr-divergence-load", "specDir required")
		return r, nil
	}

	implApprove := beadID == ""
	if headRef == "" {
		var branchErr error
		if implApprove {
			// Impl backstop path: scan the full spec branch tip vs the
			// caller-supplied base (typically merge-base with main).
			headRef, branchErr = workspace.SpecBranch(filepath.Base(specDir))
		} else {
			// Per-bead path: default to the canonical bead branch so
			// the lane is self-anchoring even when the caller does not
			// resolve a head ref explicitly.
			headRef, branchErr = workspace.BeadBranch(beadID)
		}
		if branchErr != nil {
			r.AddError("adr-divergence-load", "invalid id composing diff head ref: "+branchErr.Error())
			return r, nil
		}
	}

	return ValidateDivergence(exec, root, specDir, beadID, diffRef, headRef, ownerRef, implApprove)
}
