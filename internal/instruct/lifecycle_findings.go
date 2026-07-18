// Spec 119 Bead 2 (R7): wires the stale-OPEN and finalize-orphan doctor
// checks into the generated `mindspec instruct` idle guidance. P8
// (SHARED-PREDICATE VISIBILITY) requires this file invoke the SAME
// EXPORTED internal/lifecycle predicates internal/doctor's wrappers call —
// never a private re-derivation — so `mindspec doctor` and `mindspec
// instruct` can never disagree on either the trigger or the wording
// (AC-12/AC-15).
package instruct

import (
	"os"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Seams (mirroring internal/doctor's pattern) so collectLifecycleFindings
// is unit-testable without a live repo or `bd`. Pinned by the AC-12
// anti-drift test (lifecycle_findings_drift_test.go) to be the SAME
// exported symbols internal/doctor's checkStaleOpenBeads /
// checkFinalizeOrphans invoke.
var (
	instructFindStaleOpenBeadsFn            = lifecycle.FindStaleOpenBeads
	instructFindOutstandingFinalizeBranches = lifecycle.FindOutstandingFinalizeBranches
	instructStaleTrackerOnMainFn            = lifecycle.StaleTrackerOnMain
	instructFindEpicBySpecIDFn              = phase.FindEpicBySpecID
)

// instructFindEpicStatusFn resolves an epic's LIVE bd status. Separate seam
// from internal/doctor's findEpicStatusFn (package-private, can't be
// shared across packages) but the SAME underlying phase.Cache call.
var instructFindEpicStatusFn = func(epicID string) (string, error) {
	epic, err := phase.NewCache().FindEpic(epicID)
	if err != nil {
		return "", err
	}
	if epic == nil {
		return "", nil
	}
	return epic.Status, nil
}

// collectLifecycleFindings scans root for stale-OPEN and finalize-orphan
// findings, rendering each as ONE verbatim line via the finding's own
// Message()/FullMessage() (message + recovery command combined by the
// SAME template `mindspec doctor` renders from — no second copy of the
// text). Best-effort and read-only: any predicate error yields fewer
// findings, never a hard failure of `mindspec instruct`.
func collectLifecycleFindings(root string) []string {
	var out []string

	if branches, err := instructFindOutstandingFinalizeBranches(root); err == nil {
		for _, o := range branches {
			out = append(out, o.FullMessage())
		}
	}

	specsRoot := workspace.SpecsDir(root)
	entries, err := os.ReadDir(specsRoot)
	if err != nil {
		return out
	}
	names := make([]string, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() {
			names = append(names, ent.Name())
		}
	}
	sort.Strings(names)

	for _, specID := range names {
		if found, err := instructFindStaleOpenBeadsFn(specID, root); err == nil {
			for _, s := range found {
				out = append(out, s.Message())
			}
		}

		epicID, err := instructFindEpicBySpecIDFn(specID)
		if err != nil || epicID == "" {
			continue
		}
		status, err := instructFindEpicStatusFn(epicID)
		if err != nil {
			continue
		}
		liveClosed := strings.EqualFold(strings.TrimSpace(status), "closed")

		orphan, err := instructStaleTrackerOnMainFn(root, specID, epicID, liveClosed)
		if err != nil || orphan == nil {
			continue
		}
		out = append(out, orphan.FullMessage())
	}
	return out
}
