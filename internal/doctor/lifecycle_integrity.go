package doctor

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// scanIntegrityFindingsFn is the shared cache-aware aggregate lifecycle
// scan (Spec 119 R5/R7, final-review F1) — ONE enumeration replacing the
// per-spec-dir bd fan-out the original Bead-2 wiring performed. Injectable
// so the doctor check is unit-testable without a live repo or `bd`, and
// pinned by the AC-12 anti-drift test to be the SAME exported symbol the
// generated `mindspec instruct` guidance invokes (P8): doctor and instruct
// consume the identical aggregate, whose findings are classified and
// worded by the single-home internal/lifecycle predicates.
var scanIntegrityFindingsFn = lifecycle.ScanIntegrityFindings

// checkLifecycleIntegrity reports every lifecycle-divergence finding the
// shared aggregate scan produces. Existing findings keep the order used
// by idle guidance. Ancestor diagnoses are also rendered here:
//
//   - outstanding chore/finalize-<specID> carriers (bug wu7t residue),
//   - stale-open beads: valid merges give Info with no automatic close;
//     other stale-open findings retain Error and their completion fix,
//   - reverted naming merges on open beads: one Error with restore
//     guidance and no automatic fix,
//   - stale-committed-tracker epics (closed in bd, still non-terminal in
//     main's committed .beads/issues.jsonl),
//   - prerequisites with present work but no naming merge or binding, at Warn.
//
// One phase.Cache supplies one epic list and one global open-bead list.
func checkLifecycleIntegrity(r *Report, root string) {
	findings := scanIntegrityFindingsFn(root, phase.NewCache())

	for _, o := range findings.FinalizeBranches {
		addFinalizeOrphanCheck(r, o)
	}
	for _, s := range findings.StaleOpen {
		beadID := s.BeadID
		check := Check{
			Name:    fmt.Sprintf("stale-open bead: %s", idrender.Bead(beadID)),
			Status:  Error,
			Message: s.Message(),
			FixFunc: func() error { return runMindspecCompleteFn(root, beadID) },
		}
		if s.ValidMerge {
			check.Status = Info
			check.FixFunc = nil
		}
		r.Checks = append(r.Checks, check)
	}
	for _, s := range findings.RevertedMerges {
		r.Checks = append(r.Checks, Check{
			Name:    fmt.Sprintf("reverted bead merge: %s", idrender.Bead(s.BeadID)),
			Status:  Error,
			Message: s.Message,
		})
	}
	for _, o := range findings.StaleTrackers {
		addFinalizeOrphanCheck(r, o)
	}
	for _, ancestor := range findings.Ancestors {
		addAncestorCheck(r, ancestor)
	}
}

func addFinalizeOrphanCheck(r *Report, o lifecycle.FinalizeOrphan) {
	// Spec 121 R2(c) (Bead 1 review MINOR fix): a "pull_advisory" is NOT
	// a stranded-carrier finding — origin/main already carries the
	// correct export, only the LOCAL checkout lags — so it renders at
	// Warn severity under its own name, never mislabeled as a "finalize
	// orphan" at Error severity (which would overstate a harmless,
	// self-clearing local staleness as a stranded carrier).
	if o.Kind == "pull_advisory" {
		r.Checks = append(r.Checks, Check{
			Name:    fmt.Sprintf("local main behind origin (pull advisory): %s", idrender.Spec(o.SpecID)),
			Status:  Warn,
			Message: o.FullMessage(),
		})
		return
	}
	r.Checks = append(r.Checks, Check{
		Name:    fmt.Sprintf("finalize orphan (%s): %s", o.Kind, idrender.Spec(o.SpecID)),
		Status:  Error,
		Message: o.FullMessage(),
	})
}
