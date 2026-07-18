// Spec 119 final-review F1: ONE shared, cache-aware aggregate lifecycle
// scan consumed by BOTH `mindspec doctor` and the generated `mindspec
// instruct` idle guidance.
//
// The shipped Bead-2 wiring iterated every on-disk spec directory (~118 in
// this repo) and resolved each one's epic + status + open children through
// per-spec, uncached `bd` subprocesses — measured at minutes-to-never for
// a full `doctor --ci` run, on EVERY SessionStart once wired into idle
// instruct. This scan replaces that shape with a semantic, tracker-driven
// enumeration whose bd cost is 1 + (number of ACTIVE epics) subprocesses:
//
//   - `cache.AllEpics()` is called exactly ONCE; the specID for each epic
//     comes from its own metadata (phase.ExtractSpecMetadata /
//     phase.SpecIDFromMetadata) — never a per-spec-dir FindEpicBySpecID.
//   - stale-OPEN candidates come from ACTIVE (open/in_progress) epics
//     only: one `cache.GetChildren` per active epic, filtered in-process;
//     the git-side MergedUnclosed confirmation runs only for their open
//     children. A healthy completed historical epic cannot acquire a
//     newly-stale OPEN child without first becoming live-divergent, so
//     historical epics need no children query at all.
//   - stale-tracker divergence reads main's committed .beads/issues.jsonl
//     ONCE into an id→status map and compares every live-closed epic
//     against it in memory.
//   - `chore/finalize-*` carriers are enumerated once, git-only, with the
//     G1 origin/main ancestry confirmation.
//
// The pure classifiers (staleOpenLanded, staleTrackerFinding, the
// FindOutstandingFinalizeBranches predicate) remain the single homes of
// the trigger + message text (P8/AC-12/AC-15): this aggregate only owns
// the ENUMERATION, so doctor and instruct still cannot drift on either
// the finding condition or its wording.
package lifecycle

import (
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// IntegrityFindings aggregates every lifecycle-divergence finding the
// shared scan produces, in the order both consumers render them.
type IntegrityFindings struct {
	// FinalizeBranches are outstanding, provably-unmerged
	// chore/finalize-<specID> carriers (Kind == "finalize_branch").
	FinalizeBranches []FinalizeOrphan
	// StaleOpen are open/in_progress beads whose work already landed as a
	// bead→spec merge (the tracker never converged).
	StaleOpen []StaleOpenBead
	// StaleTrackers are live-closed epics whose committed status on main
	// is still non-terminal (Kind == "stale_tracker").
	StaleTrackers []FinalizeOrphan
}

// ScanIntegrityFindings runs the shared cache-aware aggregate scan over
// root. cache carries the invocation's memoized bd state — `mindspec
// instruct` threads its EXISTING per-invocation phase.Cache so the epic
// list is commonly already loaded; `mindspec doctor` creates one for this
// check. A nil cache degrades to uncached (but still batched) bd calls.
//
// Best-effort and read-only, mirroring the individual predicates' doctor/
// instruct contract: an infra failure on one leg yields fewer findings for
// that leg (provable findings from other legs survive — including the
// mixed-list survivors FindOutstandingFinalizeBranches returns alongside
// an ancestry-check error), never a hard failure of the caller.
func ScanIntegrityFindings(root string, cache *phase.Cache) IntegrityFindings {
	var out IntegrityFindings

	// chore/finalize-* carriers: one git-only enumeration (G1-refined).
	// The error, if any, is the mixed-list first-error — the returned
	// survivors are provable and kept.
	branches, _ := FindOutstandingFinalizeBranches(root)
	out.FinalizeBranches = branches

	// The single bd epic enumeration of the whole scan.
	epics, err := cache.AllEpics()
	if err != nil {
		return out
	}

	// main's committed export, read + decoded ONCE for every epic below.
	var committed map[string]string
	if data, jErr := fileAtRefFn(root, "main", ".beads/issues.jsonl"); jErr == nil {
		committed = issueStatusesInJSONL(data)
	}

	for _, epic := range epics {
		num, title := phase.ExtractSpecMetadata(epic)
		if num <= 0 || title == "" {
			continue // no spec lineage — not a lifecycle epic
		}
		specID := phase.SpecIDFromMetadata(num, title)

		switch strings.ToLower(strings.TrimSpace(epic.Status)) {
		case "open", "in_progress":
			// ACTIVE epic: one children query, filtered in-process to the
			// open/in_progress candidates the stale-OPEN classifier needs.
			children, cErr := cache.GetChildren(epic.ID)
			if cErr != nil {
				continue // best-effort: fewer findings, never a hard error
			}
			var openIDs []string
			for _, ch := range children {
				id := strings.TrimSpace(ch.ID)
				if id == "" || id == epic.ID {
					continue
				}
				switch strings.ToLower(strings.TrimSpace(ch.Status)) {
				case "open", "in_progress":
					openIDs = append(openIDs, id)
				}
			}
			out.StaleOpen = append(out.StaleOpen, staleOpenLanded(root, specID, openIDs)...)
		case "closed":
			if committed == nil {
				continue // main's export unreadable — cannot check, never guess
			}
			if o := staleTrackerFinding(specID, epic.ID, committed); o != nil {
				out.StaleTrackers = append(out.StaleTrackers, *o)
			}
		}
	}
	return out
}
