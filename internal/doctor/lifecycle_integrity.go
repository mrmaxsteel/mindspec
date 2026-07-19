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
// shared aggregate scan produces, in the same order the generated
// `mindspec instruct` idle guidance renders them (AC-15):
//
//   - outstanding chore/finalize-<specID> carriers (bug wu7t residue),
//   - stale-OPEN beads (open in the tracker, work already landed as a
//     bead→spec merge — the inverse of checkOrphanedBeads), each with a
//     FixFunc that re-invokes `mindspec complete <id>` under --fix,
//   - stale-committed-tracker epics (closed in bd, still non-terminal in
//     main's committed .beads/issues.jsonl).
//
// One phase.Cache is created for this aggregate check (F1) — the scan's
// entire bd cost is one epic list plus one children query per ACTIVE epic.
func checkLifecycleIntegrity(r *Report, root string) {
	findings := scanIntegrityFindingsFn(root, phase.NewCache())

	for _, o := range findings.FinalizeBranches {
		addFinalizeOrphanCheck(r, o)
	}
	for _, s := range findings.StaleOpen {
		beadID := s.BeadID
		r.Checks = append(r.Checks, Check{
			Name:    fmt.Sprintf("stale-open bead: %s", idrender.Bead(beadID)),
			Status:  Error,
			Message: s.Message(),
			FixFunc: func() error { return runMindspecCompleteFn(root, beadID) },
		})
	}
	for _, o := range findings.StaleTrackers {
		addFinalizeOrphanCheck(r, o)
	}
}

func addFinalizeOrphanCheck(r *Report, o lifecycle.FinalizeOrphan) {
	r.Checks = append(r.Checks, Check{
		Name:    fmt.Sprintf("finalize orphan (%s): %s", o.Kind, idrender.Spec(o.SpecID)),
		Status:  Error,
		Message: o.FullMessage(),
	})
}
