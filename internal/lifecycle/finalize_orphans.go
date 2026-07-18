// Spec 119 Bead 3 (R7): finalize-orphan detection. Bug wu7t's protected-main
// recovery flow (executor.MindspecExecutor.finalizeOrphanedSpecBranch,
// internal/workspace.FinalizeBranchPrefix = "chore/finalize-") creates a
// fresh chore/finalize-<specID> carrier from origin/main when a spec branch
// is already a dead PR carrier. If that recovery is interrupted or its PR is
// never opened/merged, it leaves TWO kinds of residue behind:
//
//   - an outstanding, unmerged chore/finalize-<specID> branch, and
//   - main's committed .beads/issues.jsonl staying stale relative to bd's
//     live epic status, which leaves the shipped bd post-merge hook poised
//     to silently REVERT Dolt's close on the next merge/FF.
//
// These predicates are exported here — NOT executor-private, NOT
// doctor-private — so both `mindspec doctor` and the generated `mindspec
// instruct` guidance (internal/instruct) can import and render the SAME
// finding text (P8, AC-15). internal/lifecycle sits outside the
// enforcement-package boundary pin (ADR-0030), so it uses internal/gitutil
// directly, mirroring the shipped orphans.go precedent.
package lifecycle

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Injectable seams (mirroring orphans.go's pattern) so finalize-orphan
// detection is unit-testable without a live repo.
var (
	localBranchRefsFn           = gitutil.LocalBranchRefs
	finalizeOrphanCommitCountFn = gitutil.CommitCount
	finalizeOrphanDiffStatFn    = gitutil.DiffStat
	fileAtRefFn                 = gitutil.FileAtRef
)

// FinalizeOrphan describes a leftover artifact from an interrupted
// protected-main finalize recovery (bug wu7t). One shared shape (P8,
// AC-15) so `doctor` and the generated `instruct` guidance render
// IDENTICAL message + recovery text from THIS predicate — never a private
// reimplementation.
type FinalizeOrphan struct {
	// Kind is "finalize_branch" (an outstanding chore/finalize-<specID>
	// branch) or "stale_tracker" (main's committed .beads/issues.jsonl
	// disagrees with bd's live epic status).
	Kind string
	// SpecID is the spec this finalize orphan belongs to.
	SpecID string
	// Branch is the outstanding chore/finalize-<specID> carrier
	// (Kind == "finalize_branch" only; empty otherwise).
	Branch string
	// CommitCount / DiffStat summarize Branch's stranded work, computed
	// against origin/main (Requirement 7: NEVER possibly-stale local
	// main) — populated for Kind == "finalize_branch" only.
	CommitCount int
	DiffStat    string
	// Message is the rendered human-readable finding text — the SAME
	// string doctor and instruct must both surface (AC-15).
	Message string
}

// RecoveryCommand names the forward re-invocation that clears the orphan
// (ADR-0035 recovery-line convention).
func (o FinalizeOrphan) RecoveryCommand() string {
	if o.Kind == "finalize_branch" {
		return fmt.Sprintf("open a PR for %s and merge it (or delete the branch if it is superseded)", o.Branch)
	}
	return fmt.Sprintf("mindspec impl approve %s", o.SpecID)
}

// FindOutstandingFinalizeBranches scans workdir's LOCAL branches for a
// surviving chore/finalize-<specID> carrier (workspace.FinalizeBranchPrefix).
// finalizeOrphanedSpecBranch leaves this branch behind LOCALLY on success —
// a retry deletes and recreates it fresh from origin/main, per its own doc
// comment — so a branch surviving past the run that created it means the PR
// it carries was never opened, merged, and cleaned up. Stats are computed
// against origin/main (Requirement 7), never local main — this predicate
// never even reads local main.
func FindOutstandingFinalizeBranches(workdir string) ([]FinalizeOrphan, error) {
	branches, err := localBranchRefsFn(workdir)
	if err != nil {
		return nil, fmt.Errorf("listing local branches: %w", err)
	}
	var out []FinalizeOrphan
	for _, b := range branches {
		if !strings.HasPrefix(b, workspace.FinalizeBranchPrefix) {
			continue
		}
		specID := strings.TrimPrefix(b, workspace.FinalizeBranchPrefix)
		o := FinalizeOrphan{Kind: "finalize_branch", SpecID: specID, Branch: b}
		if count, cErr := finalizeOrphanCommitCountFn(workdir, "origin/main", b); cErr == nil {
			o.CommitCount = count
		}
		if stat, sErr := finalizeOrphanDiffStatFn(workdir, "origin/main", b); sErr == nil {
			o.DiffStat = stat
		}
		o.Message = fmt.Sprintf(
			"finalize branch %s is unmerged (%d commit(s) ahead of origin/main) — spec %s's epic-close export never reached main",
			b, o.CommitCount, specID,
		)
		out = append(out, o)
	}
	return out, nil
}

// StaleTrackerOnMain reports whether epicID's committed status inside
// main's .beads/issues.jsonl (read via `git show main:.beads/issues.jsonl`)
// is a non-terminal ("open"/"in_progress") status while liveClosed is true
// — bd's LIVE state already has the epic closed. That divergence is the
// tell-tale left by bug wu7t: main never received the finalize export that
// would have synced it, which leaves the shipped bd post-merge hook poised
// to silently revert Dolt's close on the next merge/FF.
//
// Returns (nil, nil) — not itself an error — when liveClosed is false, when
// epicID is not present in main's committed export, or when the two
// statuses already agree. A genuine git-read failure (bad ref, no such
// path) is propagated so a caller can distinguish "no finding" from
// "could not check".
func StaleTrackerOnMain(workdir, specID, epicID string, liveClosed bool) (*FinalizeOrphan, error) {
	if !liveClosed {
		return nil, nil
	}
	data, err := fileAtRefFn(workdir, "main", ".beads/issues.jsonl")
	if err != nil {
		return nil, fmt.Errorf("reading main:.beads/issues.jsonl: %w", err)
	}
	committedStatus, found := issueStatusInJSONL(data, epicID)
	if !found || strings.EqualFold(committedStatus, "closed") {
		return nil, nil
	}
	return &FinalizeOrphan{
		Kind:   "stale_tracker",
		SpecID: specID,
		Message: fmt.Sprintf(
			"epic %s (spec %s) is closed in bd but main's committed .beads/issues.jsonl still shows status %q — the finalize export never reached main",
			epicID, specID, committedStatus,
		),
	}, nil
}

// issueStatusInJSONL scans a .beads/issues.jsonl blob (one JSON object per
// line) for id == issueID and returns its status field.
func issueStatusInJSONL(data []byte, issueID string) (string, bool) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.ID == issueID {
			return rec.Status, true
		}
	}
	return "", false
}
