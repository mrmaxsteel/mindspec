// Spec 119 Bead 2 (R5): the stale-OPEN cross-check — the inverse of
// FindOrphanedClosedBeads (orphans.go). Where an orphaned-closed bead is
// Dolt racing AHEAD of git (closed without `mindspec complete`), a
// stale-OPEN bead is Dolt lagging BEHIND git: the tracker still shows the
// bead open/in_progress even though its bead/<id> branch has already been
// merged into the spec branch via the in-binary CompleteBead merge path.
//
// Detection reuses Bead 1's exported landed-merge-commit-identity predicate
// (MergedUnclosed, landed.go) rather than a private reimplementation or any
// fork-point computation of its own (P8). This is load-bearing for the
// fresh-claim negative (R5): a freshly claimed bead branch has zero own
// commits, so `git merge --no-ff` of it is a no-op that produces no merge
// commit — MergedUnclosed correctly reports false for it BY CONSTRUCTION,
// even when the spec branch has since advanced past the fork point via
// OTHER beads' `--no-ff` merges (the round-1 false-flag scenario this
// predicate must never reproduce).
package lifecycle

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// listOpenBeadsFn is the injectable seam for enumerating an epic's
// open/in_progress children (mirroring orphans.go's listClosedBeadsFn seam
// pattern) so FindStaleOpenBeads is unit-testable without a live `bd`.
var listOpenBeadsFn = func(epicID string) ([]bead.BeadInfo, error) {
	// Gate-all-ids (ADR-0042 §1, round 8/9): epicID feeds a
	// `bd list --parent` argv build directly — validate BEFORE any bd
	// spawn, ZERO bd argv on a malformed id.
	if err := idvalidate.BeadID(epicID); err != nil {
		return nil, fmt.Errorf("invalid epic id %s: %w", idrender.Bead(epicID), err)
	}
	out, err := bead.RunBD("list", "--parent", epicID, "--status=open,in_progress", "--json")
	if err != nil {
		return nil, err
	}
	var items []bead.BeadInfo
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// StaleOpenBead describes a bead OPEN/in_progress in the tracker whose
// bead/<id> work has already landed as a merge commit on the spec branch —
// the tracker never converged after `mindspec complete` (or an operator's
// out-of-band recovery merge) landed the work.
type StaleOpenBead struct {
	// BeadID is the stale bead's id (without the bead/ prefix).
	BeadID string
	// SpecBranch is the spec branch the landed merge was found on.
	SpecBranch string
	// LandedSHA is the identified bead->spec merge commit.
	LandedSHA string
}

// RecoveryCommand names the forward re-invocation that clears the stale
// state (ADR-0035 recovery-line convention): re-running `mindspec complete`
// converges Dolt to the already-landed state.
func (s StaleOpenBead) RecoveryCommand() string {
	// R4: BeadID is an ID-typed position — idrender.Bead.
	return "mindspec complete " + idrender.Bead(s.BeadID)
}

// Message is the rendered human-readable finding text, recovery command
// included — the SAME string `mindspec doctor` and the generated `mindspec
// instruct` guidance must both surface verbatim (Spec 119 AC-15/P8): this
// is the single template, never re-derived by either consumer.
//
// R4: BeadID is idrender'd (ID-typed position); SpecBranch is the
// spine-validated `spec/<id>` branch operand (workspace.SpecBranch) and
// stays RAW; LandedSHA is a git-produced hex commit SHA, not agent-writable
// free text.
func (s StaleOpenBead) Message() string {
	return fmt.Sprintf(
		"bead %s is OPEN/in_progress in the tracker but its work already landed as merge %s on %s — the tracker never converged. Run `%s` to recover.",
		idrender.Bead(s.BeadID), s.LandedSHA, s.SpecBranch, s.RecoveryCommand(),
	)
}

// FindStaleOpenBeads scans specID's epic for OPEN/in_progress beads whose
// work already landed on the spec branch (Spec 119 R5, Bead 2 Step 1).
//
// A bead is flagged iff MergedUnclosed(workdir, specBranch, beadID)
// positively reports merged-unclosed: Bead 1's FindLandedMerge predicate
// identifies a landed bead->spec merge commit AND — when bead/<id> still
// exists — it carries no unlanded commits on top (IsAncestor holds against
// the spec branch's CURRENT tip). This is the exact same reconcile-
// eligibility state `mindspec complete`'s forward reconcile path consumes,
// so the doctor/instruct check and the gate can never disagree.
//
// Detection is best-effort and read-only, mirroring FindOrphanedClosedBeads:
// an absent epic, a `bd` failure, or a per-bead MergedUnclosed error yields
// fewer findings rather than a hard error, so a transient infra problem
// never masks `mindspec doctor`'s other checks.
func FindStaleOpenBeads(specID, workdir string) ([]StaleOpenBead, error) {
	epicID, err := findEpicBySpecIDFn(specID)
	if err != nil {
		return nil, fmt.Errorf("finding epic for spec %s: %w", idrender.Spec(specID), err)
	}
	if epicID == "" {
		return nil, nil
	}

	items, err := listOpenBeadsFn(epicID)
	if err != nil {
		return nil, fmt.Errorf("listing open beads for epic %s: %w", epicID, err)
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" || id == epicID {
			continue
		}
		ids = append(ids, id)
	}
	return staleOpenLanded(workdir, specID, ids), nil
}

// staleOpenLanded is the pure classification core shared by
// FindStaleOpenBeads and ScanIntegrityFindings (spec 119 final-review F1):
// given the ALREADY-ENUMERATED open/in_progress bead IDs of specID's epic,
// it runs MergedUnclosed per bead — no bd calls of its own — and returns
// the beads whose work provably landed. Best-effort: an ancestry/read
// error on one bead must not abort the scan for the rest.
func staleOpenLanded(workdir, specID string, beadIDs []string) []StaleOpenBead {
	specBranch, err := workspace.SpecBranch(specID)
	if err != nil {
		// Best-effort ambient scan: an invalid specID yields no findings
		// rather than a hard error (ADR-0042 degrade-vs-error policy).
		return nil
	}
	var out []StaleOpenBead
	for _, id := range beadIDs {
		landed, ok, mErr := MergedUnclosed(workdir, specBranch, id)
		if mErr != nil || !ok {
			continue
		}
		out = append(out, StaleOpenBead{
			BeadID:     id,
			SpecBranch: specBranch,
			LandedSHA:  landed.SHA,
		})
	}
	return out
}
