// Package lifecycle holds cross-command lifecycle-invariant detectors shared
// by `mindspec next`, `mindspec complete`, and `mindspec doctor`.
//
// The bead-loop guardrails (AGENTS.md) require that a lifecycle bead is closed
// ONLY via `mindspec complete`, which merges the bead/<id> branch into the spec
// branch and then removes the worktree + branch. An agent that finishes a bead
// with a bare `bd close <id>` leaves the bead marked done in Dolt while its
// bead/<id> branch is still unmerged — the lifecycle can no longer see the
// pending work (the "bd_close lifecycle-bypass", bead mindspec-4gsz).
//
// FindOrphanedClosedBeads is the ONE shared predicate that detects this state,
// reused by all three commands so the trigger and the message stay in lockstep.
package lifecycle

import (
	"encoding/json"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Injectable seams (test stubs). The defaults call the real packages; tests
// override these to drive the predicate without a live repo or `bd`.
var (
	findEpicBySpecIDFn = phase.FindEpicBySpecID
	listClosedBeadsFn  = func(epicID string) ([]bead.BeadInfo, error) {
		out, err := bead.RunBD("list", "--parent", epicID, "--status=closed", "--json")
		if err != nil {
			return nil, err
		}
		var items []bead.BeadInfo
		if err := json.Unmarshal(out, &items); err != nil {
			return nil, err
		}
		return items, nil
	}
	branchExistsFn = gitutil.BranchExists
	isAncestorFn   = gitutil.IsAncestor
)

// Orphan describes a closed lifecycle bead whose bead/<id> branch still exists
// and is NOT an ancestor of the spec branch — i.e. it was closed without
// `mindspec complete` and its work is unmerged and ungated.
type Orphan struct {
	BeadID     string // the closed bead id (without the bead/ prefix)
	BeadBranch string // bead/<id>
	SpecBranch string // spec/<specID>
}

// RecoveryCommand is the converging re-run that clears the orphaned state.
func (o Orphan) RecoveryCommand() string { return "mindspec complete " + o.BeadID }

// FindOrphanedClosedBeads returns the closed sibling beads under specID's epic
// that were closed without `mindspec complete`.
//
// A closed bead is ORPHANED iff its bead/<id> branch EXISTS (the cheap trigger)
// AND is NOT an ancestor of the spec branch (the confirmation). The IsAncestor
// guard is the key refinement: `mindspec complete` deletes the bead branch
// best-effort after the merge (mindspec_executor.go), so a benign
// "merged-but-branch-undeleted" branch can legitimately exist — that branch IS
// an ancestor of the spec branch and must NOT be flagged.
//
// workdir is the git working directory the ancestry check runs in (the repo
// root or a worktree). excludeBeadID, when non-empty, is skipped — callers that
// are themselves completing a bead must not flag the very bead in flight
// (chicken-and-egg).
//
// Detection is best-effort and read-only: an absent epic, a `bd` failure, or an
// ancestry-check error yields no orphans rather than a hard error, so a
// transient infra problem never masks the original command.
func FindOrphanedClosedBeads(specID, workdir, excludeBeadID string) []Orphan {
	epicID, err := findEpicBySpecIDFn(specID)
	if err != nil || epicID == "" {
		return nil
	}

	items, err := listClosedBeadsFn(epicID)
	if err != nil {
		return nil
	}

	specBranch := workspace.SpecBranch(specID)

	var orphans []Orphan
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" || id == excludeBeadID {
			continue
		}
		beadBranch := workspace.BeadBranch(id)
		// Cheap trigger: a closed bead with no branch is cleanly completed.
		if !branchExistsFn(beadBranch) {
			continue
		}
		// Confirmation: a branch that IS an ancestor of the spec branch is the
		// benign merged-but-undeleted state — skip it. Treat an ancestry error
		// as "cannot confirm orphaned" and skip (read-only, never false-block).
		isAnc, ancErr := isAncestorFn(workdir, beadBranch, specBranch)
		if ancErr != nil || isAnc {
			continue
		}
		orphans = append(orphans, Orphan{
			BeadID:     id,
			BeadBranch: beadBranch,
			SpecBranch: specBranch,
		})
	}
	return orphans
}
