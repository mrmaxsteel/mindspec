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
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Injectable seams (test stubs). The defaults call the real packages; tests
// override these to drive the predicate without a live repo or `bd`.
var (
	findEpicBySpecIDFn = phase.FindEpicBySpecID
	listClosedBeadsFn  = func(epicID string) ([]bead.BeadInfo, error) {
		// Gate-all-ids (ADR-0042 §1, round 8/9): epicID feeds a
		// `bd list --parent` argv build directly — bd ids are
		// agent-writable (bd create --force --id=<arbitrary> proven
		// unsafe), so it is validated BEFORE any bd spawn, ZERO bd argv
		// on a malformed id.
		if err := idvalidate.BeadID(epicID); err != nil {
			return nil, fmt.Errorf("invalid epic id %s: %w", epicID, err)
		}
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
// The bead ID is rendered via idrender.Bead: byte-identical for a
// genuine, waist-valid bead ID, but forced through strconv.Quote for a
// malformed-but-printable value so this display/copy-paste surface can
// never be mistaken for — or forge output around — a real recovery
// command (R4 display-forgery sweep, spec 120).
func (o Orphan) RecoveryCommand() string { return "mindspec complete " + idrender.Bead(o.BeadID) }

// ClosedEpicBeadIDs returns the ids of every closed bead under specID's epic
// — the SAME `bd list --parent <epic> --status=closed` enumeration
// ScanOrphanedClosedBeads walks — as a standalone, error-preserving export
// (Bead 2's worktree-enumeration merge-prevention leg consumes this so the
// gate and the orphan scan can never disagree on the epic's closed-bead
// set; single home).
//
// An absent epic (findEpicBySpecIDFn succeeds with an empty id — a spec
// whose epic has not been created yet, a legitimate state) yields (nil,
// nil): not itself an infra failure. A genuine epic-lookup or bd-list
// failure is PROPAGATED, never swallowed — callers that need fail-closed
// behavior (the gate) can tell "no closed beads" from "could not check"; a
// caller that wants the old fail-open behavior (see FindOrphanedClosedBeads
// below) chooses to ignore the error itself.
func ClosedEpicBeadIDs(specID string) ([]string, error) {
	epicID, err := findEpicBySpecIDFn(specID)
	if err != nil {
		return nil, fmt.Errorf("finding epic for spec %s: %w", specID, err)
	}
	if epicID == "" {
		return nil, nil
	}

	items, err := listClosedBeadsFn(epicID)
	if err != nil {
		return nil, fmt.Errorf("listing closed beads for epic %s: %w", epicID, err)
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ScanOrphanedClosedBeads is the error-preserving core behind
// FindOrphanedClosedBeads (Bead 2's impl-approve gate consumes this
// directly, fail-closed): it performs the IDENTICAL orphan-detection walk
// but PROPAGATES the three cleanly-signaled infra errors — epic-lookup and
// bd-list (both surfaced via ClosedEpicBeadIDs above) and the per-bead
// ancestry check — instead of silently swallowing them into an empty
// result. An unreadable store must never be confused with "no orphans".
//
// A closed bead is ORPHANED iff its bead/<id> branch EXISTS (the cheap
// trigger, branchExistsFn — unchanged, a probe failure there still reads as
// "no trigger" by design, round-6 C+B) AND is NOT an ancestor of the spec
// branch (the confirmation). The IsAncestor guard is the key refinement:
// `mindspec complete` deletes the bead branch best-effort after the merge
// (mindspec_executor.go), so a benign "merged-but-branch-undeleted" branch
// can legitimately exist — that branch IS an ancestor of the spec branch
// and must NOT be flagged.
//
// workdir is the git working directory the ancestry check runs in (the repo
// root or a worktree). excludeBeadID, when non-empty, is skipped — callers
// that are themselves completing a bead must not flag the very bead in
// flight (chicken-and-egg).
//
// MIXED-list parity (the round-3 G2 case): when the ancestry check errors
// for one bead but a LATER bead in the same list is a provable orphan, this
// function does NOT abandon the scan — it records the first ancestry error
// but keeps walking the remaining beads, so the later orphan is still
// collected. It returns (orphans-found-so-far, the first error) rather than
// (nil, error): the error alone tells a fail-closed caller (the gate) to
// refuse, while the accompanying orphans list lets the fail-open wrapper
// below reproduce its EXACT historical behavior (skip the erroring bead,
// keep scanning) merely by ignoring the error.
func ScanOrphanedClosedBeads(specID, workdir, excludeBeadID string) ([]Orphan, error) {
	ids, err := ClosedEpicBeadIDs(specID)
	if err != nil {
		return nil, err
	}

	specBranch, err := workspace.SpecBranch(specID)
	if err != nil {
		return nil, fmt.Errorf("composing spec branch for %s: %w", specID, err)
	}

	var orphans []Orphan
	var firstErr error
	for _, id := range ids {
		if id == "" || id == excludeBeadID {
			continue
		}
		// Gate-all-ids (ADR-0042 §1): id is bd-sourced and agent-writable
		// (bd create --force --id=<arbitrary> is empirically unsafe) —
		// validate before composing a branch name. A malformed id is
		// recorded via the same firstErr discipline as an ancestry-check
		// failure (fail-closed signal for the gate) but the scan keeps
		// walking (MIXED-list parity for the fail-open wrapper).
		beadBranch, err := workspace.BeadBranch(id)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("invalid closed-bead id %s: %w", id, err)
			}
			continue
		}
		// Cheap trigger: a closed bead with no branch is cleanly completed.
		if !branchExistsFn(beadBranch) {
			continue
		}
		// Confirmation: a branch that IS an ancestor of the spec branch is the
		// benign merged-but-undeleted state — skip it. An ancestry-check
		// error is recorded (fail-closed signal for the gate) but does NOT
		// stop the scan — the fail-open wrapper's parity depends on later
		// beads in the same list still being reachable.
		isAnc, ancErr := isAncestorFn(workdir, beadBranch, specBranch)
		if ancErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("checking ancestry of %s: %w", beadBranch, ancErr)
			}
			continue
		}
		if isAnc {
			continue
		}
		orphans = append(orphans, Orphan{
			BeadID:     id,
			BeadBranch: beadBranch,
			SpecBranch: specBranch,
		})
	}
	return orphans, firstErr
}

// FindOrphanedClosedBeads returns the closed sibling beads under specID's epic
// that were closed without `mindspec complete`.
//
// Detection is best-effort and read-only: an absent epic, a `bd` failure, or
// an ancestry-check error yields no orphans rather than a hard error, so a
// transient infra problem never masks the original command. This is now a
// thin fail-open wrapper over ScanOrphanedClosedBeads — it delegates the
// identical walk and discards the error, reproducing the exact historical
// behavior byte-for-byte (including the MIXED-list case: a later provable
// orphan survives an earlier bead's ancestry-check error) for its three
// existing callers (`mindspec complete`, `mindspec next`, `mindspec
// doctor`). Bead 2's impl-approve gate uses ScanOrphanedClosedBeads directly
// instead, fail-closed on the propagated error.
func FindOrphanedClosedBeads(specID, workdir, excludeBeadID string) []Orphan {
	orphans, _ := ScanOrphanedClosedBeads(specID, workdir, excludeBeadID)
	return orphans
}
