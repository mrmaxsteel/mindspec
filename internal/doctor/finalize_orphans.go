package doctor

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Seams (mirroring findOrphanedClosedBeadsFn's pattern) so the
// finalize-orphan doctor checks are unit-testable without a live repo or
// `bd`. Both are pinned by the AC-12 anti-drift test to be the SAME
// exported symbols the generated `mindspec instruct` guidance invokes (P8).
var (
	findOutstandingFinalizeBranchesFn = lifecycle.FindOutstandingFinalizeBranches
	staleTrackerOnMainFn              = lifecycle.StaleTrackerOnMain
)

// findEpicForFinalizeCheckFn resolves a spec ID to its epic ID. Separate
// seam name from checkOrphanedBeads' internals (this one is doctor-package
// local; lifecycle's own findEpicBySpecIDFn is unexported to that package)
// so StaleTrackerOnMain's liveClosed argument can be derived here.
var findEpicForFinalizeCheckFn = phase.FindEpicBySpecID

// findEpicStatusFn resolves an epic's LIVE bd status (open/in_progress/
// closed). A nil *phase.Cache is safe (falls back to an uncached bd show).
var findEpicStatusFn = func(epicID string) (string, error) {
	epic, err := phase.NewCache().FindEpic(epicID)
	if err != nil {
		return "", err
	}
	if epic == nil {
		return "", nil
	}
	return epic.Status, nil
}

// checkFinalizeOrphans reports leftover artifacts from an interrupted
// protected-main finalize recovery (bug wu7t, Spec 119 R7): an outstanding,
// unmerged chore/finalize-<specID> branch, and main's committed
// .beads/issues.jsonl staying stale relative to bd's live epic status (the
// tell-tale left when that recovery's PR is never opened/merged). Both
// findings are rendered from the SAME internal/lifecycle predicates the
// generated `mindspec instruct` guidance consumes (P8, AC-12/AC-15) — never
// a doctor-private reimplementation.
func checkFinalizeOrphans(r *Report, root string) {
	if branches, err := findOutstandingFinalizeBranchesFn(root); err == nil {
		for _, o := range branches {
			addFinalizeOrphanCheck(r, o)
		}
	}

	specsRoot := workspace.SpecsDir(root)
	entries, err := os.ReadDir(specsRoot)
	if err != nil {
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
		epicID, err := findEpicForFinalizeCheckFn(specID)
		if err != nil || epicID == "" {
			continue
		}
		status, err := findEpicStatusFn(epicID)
		if err != nil {
			continue
		}
		liveClosed := strings.EqualFold(strings.TrimSpace(status), "closed")

		orphan, err := staleTrackerOnMainFn(root, specID, epicID, liveClosed)
		if err != nil || orphan == nil {
			continue
		}
		addFinalizeOrphanCheck(r, *orphan)
	}
}

func addFinalizeOrphanCheck(r *Report, o lifecycle.FinalizeOrphan) {
	r.Checks = append(r.Checks, Check{
		Name:    fmt.Sprintf("finalize orphan (%s): %s", o.Kind, o.SpecID),
		Status:  Error,
		Message: o.FullMessage(),
	})
}
