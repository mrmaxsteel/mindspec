package doctor

import (
	"fmt"
	"os"
	"sort"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// findStaleOpenBeadsFn is the shared stale-OPEN cross-check predicate (Spec
// 119 R5) — the inverse of findOrphanedClosedBeadsFn. Injectable so the
// doctor check is unit-testable without a live repo or `bd`, and pinned by
// the AC-12 anti-drift test to be the SAME exported symbol the generated
// `mindspec instruct` guidance invokes (P8).
var findStaleOpenBeadsFn = lifecycle.FindStaleOpenBeads

// checkStaleOpenBeads reports lifecycle beads that are still OPEN or
// in_progress in the tracker even though their bead/<id> branch has already
// been merged into the spec branch — the tracker never converged after
// `mindspec complete` (or an out-of-band recovery merge) landed the work.
//
// It walks the tier-aware specs enumeration root (spec 106 Req 3) and runs
// the shared lifecycle.FindStaleOpenBeads predicate per spec — the same
// landed-merge-commit-identity mechanism (Bead 1's FindLandedMerge /
// MergedUnclosed) the reconcile path in `mindspec complete` consumes, so
// this check and that gate can never disagree. Each finding is reported as
// Status=Error with the `mindspec complete <id>` recovery line and a
// FixFunc that re-invokes completion under `--fix`.
func checkStaleOpenBeads(r *Report, root string) {
	specsRoot := workspace.SpecsDir(root)
	entries, err := os.ReadDir(specsRoot)
	if err != nil {
		// No specs dir = nothing to check.
		return
	}

	names := make([]string, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() {
			names = append(names, ent.Name())
		}
	}
	sort.Strings(names)

	for _, specID := range names {
		found, err := findStaleOpenBeadsFn(specID, root)
		if err != nil {
			// Best-effort: a transient infra problem on one spec must
			// never mask the rest of doctor's checks.
			continue
		}
		for _, s := range found {
			beadID := s.BeadID
			r.Checks = append(r.Checks, Check{
				Name:    fmt.Sprintf("stale-open bead: %s", beadID),
				Status:  Error,
				Message: s.Message(),
				FixFunc: func() error { return runMindspecCompleteFn(root, beadID) },
			})
		}
	}
}
