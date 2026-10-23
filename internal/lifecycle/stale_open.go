// The stale-open cross-check finds open beads whose work has landed.
// A valid naming merge permits verification before close. Reverted merges
// need restoration. Other landed evidence retains the existing reconcile
// finding. A fresh branch with no naming merge gets no finding.
package lifecycle

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
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
	out, err := bead.ListJSON("--parent", epicID, "--status=open,in_progress")
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
	// ValidMerge means git proves one naming merge at the current bead tip
	// with its changes still present. The operator must verify before close.
	ValidMerge bool
}

// RecoveryCommand names completion after the operator verifies the merge.
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
	if s.ValidMerge {
		return guard.FormatFailure(fmt.Sprintf("bead %s remains open after valid merge %s on %s. Verify the merged work before you close the bead.", idrender.Bead(s.BeadID), s.LandedSHA, s.SpecBranch), s.RecoveryCommand())
	}
	return fmt.Sprintf(
		"bead %s is OPEN/in_progress in the tracker but its work already landed as merge %s on %s — the tracker never converged. Run `%s` to recover.",
		idrender.Bead(s.BeadID), s.LandedSHA, s.SpecBranch, s.RecoveryCommand(),
	)
}

// FindStaleOpenBeads scans specID's epic for OPEN/in_progress beads whose
// work already landed on the spec branch (Spec 119 R5, Bead 2 Step 1).
//
// A valid merge sets ValidMerge. Otherwise MergedUnclosed supplies the
// existing reconcile evidence. Reverted merges do not enter this list;
// ScanIntegrityFindings reports them in RevertedMerges.
//
// Detection is best-effort and read-only, mirroring FindOrphanedClosedBeads:
// an absent epic, a tracker failure, or a per-bead git read error yields
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

// staleOpenLanded returns the stale-open subset of the shared classifier.
// The aggregate scan also uses its reverted-merge findings.
func staleOpenLanded(workdir, specID string, beadIDs []string) []StaleOpenBead {
	stale, _ := openMergeFindings(workdir, specID, "", beadIDs)
	return stale
}

// RevertedMergeBead is an open bead whose naming merge lost one or more
// changed paths. Message contains the read-only restore guidance.
type RevertedMergeBead struct {
	BeadID  string
	Message string
}

// openMergeFindings classifies the open enumeration once. Reverted beads
// never enter the stale-open list. No marker metadata affects severity.
func openMergeFindings(workdir, specID, epicID string, beadIDs []string) ([]StaleOpenBead, []RevertedMergeBead) {
	specBranch, err := workspace.SpecBranch(specID)
	if err != nil {
		// Best-effort ambient scan: an invalid specID yields no findings
		// rather than a hard error (ADR-0042 degrade-vs-error policy).
		return nil, nil
	}
	var out []StaleOpenBead
	var reverted []RevertedMergeBead
	for _, id := range beadIDs {
		merges, err := FindNamingMerges(workdir, specBranch, id)
		if err == nil && len(merges) > 0 {
			specDir, dirErr := mergeSpecDir(workdir, specBranch)
			if dirErr != nil {
				goto fallback
			}
			m := namingMergeIdentity(merges[0])
			presence, presenceErr := CheckMergePresence(workdir, specBranch, specDir, m)
			if presenceErr != nil {
				goto fallback
			}
			if len(presence.RevertedPaths) > 0 {
				wt, wtErr := gitutil.WorktreeForBranch(workdir, specBranch)
				outcome := guard.DestructionEvidenceError
				if wtErr == nil && wt != "" {
					outcome = MergeRestoreOutcome(wt, specDir)
				}
				if wt == "" {
					wt = "<spec worktree>"
				}
				parent := epicID
				if parent == "" {
					parent = "<epic>"
				}
				message, recoveryErr := RestoreMergeRecovery(wt, id, parent, m, presence, outcome)
				if recoveryErr != nil {
					goto fallback
				}
				reverted = append(reverted, RevertedMergeBead{BeadID: id, Message: message})
				continue
			}
			valid, validErr := FindValidMerge(workdir, specBranch, id)
			if validErr == nil && valid != nil {
				out = append(out, StaleOpenBead{BeadID: id, SpecBranch: specBranch, LandedSHA: valid.SHA, ValidMerge: true})
				continue
			}
		}
	fallback:
		// A merge-state read error must not hide existing landed evidence.
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
	return out, reverted
}
