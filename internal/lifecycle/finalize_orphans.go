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
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Injectable seams (mirroring orphans.go's pattern) so finalize-orphan
// detection is unit-testable without a live repo.
var (
	localBranchRefsFn           = gitutil.LocalBranchRefs
	finalizeOrphanCommitCountFn = gitutil.CommitCount
	finalizeOrphanDiffStatFn    = gitutil.DiffStat
	finalizeOrphanIsAncestorFn  = gitutil.IsAncestor
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
//
// R4: o.Branch is the spine-validated `chore/finalize-<specID>` branch
// operand (workspace.FinalizeBranchPrefix) and stays RAW, matching the
// `spec/<id>`/`bead/<id>` convention; o.SpecID is an ID-typed position —
// idrender.Spec.
func (o FinalizeOrphan) RecoveryCommand() string {
	if o.Kind == "finalize_branch" {
		return fmt.Sprintf("open a PR for %s and merge it (or delete the branch if it is superseded)", o.Branch)
	}
	return fmt.Sprintf("mindspec impl approve %s", idrender.Spec(o.SpecID))
}

// FullMessage combines Message and RecoveryCommand into the single rendered
// line `mindspec doctor` (internal/doctor) and the generated `mindspec
// instruct` guidance (internal/instruct) both surface verbatim (Spec 119
// Bead 2, AC-15/P8) — one template, defined once here, so the two
// consumers cannot drift into differently-worded renderings of the same
// finding.
func (o FinalizeOrphan) FullMessage() string {
	return fmt.Sprintf("%s Run `%s`.", o.Message, o.RecoveryCommand())
}

// FindOutstandingFinalizeBranches scans workdir's LOCAL branches for a
// surviving chore/finalize-<specID> carrier (workspace.FinalizeBranchPrefix).
// finalizeOrphanedSpecBranch leaves this branch behind LOCALLY on success —
// a retry deletes and recreates it fresh from origin/main, per its own doc
// comment — so a branch surviving past the run that created it means the PR
// it carries was never opened, merged, and cleaned up. Stats are computed
// against origin/main (Requirement 7), never local main — this predicate
// never even reads local main.
//
// Merged-carrier suppression (spec 119 final-review G1): because the
// recovery flow deliberately leaves the carrier branch behind LOCALLY even
// after its PR merges, "the branch exists" alone is NOT proof of stranded
// work. A carrier that IS an ancestor of origin/main is the benign
// merged-but-undeleted state and is skipped — the same IsAncestor
// confirmation ScanOrphanedClosedBeads (orphans.go) applies to bead
// branches. When the ancestry of one branch CANNOT be checked, that branch
// is never asserted "unmerged" from absence of proof: it is skipped and the
// first such error is returned alongside the provable findings (the
// mixed-list contract ScanOrphanedClosedBeads pioneered — later provable
// findings survive an earlier branch's ancestry error). The check reads
// only the locally available origin/main remote-tracking ref; no network
// fetch is performed.
func FindOutstandingFinalizeBranches(workdir string) ([]FinalizeOrphan, error) {
	branches, err := localBranchRefsFn(workdir)
	if err != nil {
		return nil, fmt.Errorf("listing local branches: %w", err)
	}
	var out []FinalizeOrphan
	var firstErr error
	for _, b := range branches {
		if !strings.HasPrefix(b, workspace.FinalizeBranchPrefix) {
			continue
		}
		// Confirmation before assertion: a carrier already merged into
		// origin/main is benign residue, not an orphan. An ancestry-check
		// failure is recorded and the branch skipped — never reported as
		// "unmerged" without proof.
		isAnc, ancErr := finalizeOrphanIsAncestorFn(workdir, b, "origin/main")
		if ancErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("checking ancestry of %s against origin/main: %w", b, ancErr)
			}
			continue
		}
		if isAnc {
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
		// R4: b is the spine-validated finalize-branch operand (stays RAW);
		// specID is an ID-typed position (idrender.Spec).
		o.Message = fmt.Sprintf(
			"finalize branch %s is unmerged (%d commit(s) ahead of origin/main) — spec %s's epic-close export never reached main",
			b, o.CommitCount, idrender.Spec(specID),
		)
		out = append(out, o)
	}
	return out, firstErr
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
	return staleTrackerFinding(specID, epicID, issueStatusesInJSONL(data)), nil
}

// staleTrackerFinding is the pure classification core shared by
// StaleTrackerOnMain and ScanIntegrityFindings (spec 119 final-review F1):
// given main's ALREADY-PARSED committed id→status map, it reports the
// stale-tracker divergence for one live-closed epic, or nil. The single
// home of the finding's message template — neither consumer re-derives it.
func staleTrackerFinding(specID, epicID string, committed map[string]string) *FinalizeOrphan {
	committedStatus, found := committed[epicID]
	if !found || strings.EqualFold(committedStatus, "closed") {
		return nil
	}
	// R4: epicID/specID are ID-typed positions (idrender.Bead/idrender.Spec);
	// committedStatus already renders through %q (strconv.Quote-equivalent),
	// which is inherently forced-safe.
	return &FinalizeOrphan{
		Kind:   "stale_tracker",
		SpecID: specID,
		Message: fmt.Sprintf(
			"epic %s (spec %s) is closed in bd but main's committed .beads/issues.jsonl still shows status %q — the finalize export never reached main",
			idrender.Bead(epicID), idrender.Spec(specID), committedStatus,
		),
	}
}

// issueStatusesInJSONL parses a .beads/issues.jsonl blob (one JSON object
// per line) ONCE into an id→status map, so an aggregate scan over many
// epics reads and decodes main's committed export a single time (F1).
func issueStatusesInJSONL(data []byte) map[string]string {
	out := make(map[string]string)
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
		if _, dup := out[rec.ID]; !dup {
			out[rec.ID] = rec.Status
		}
	}
	return out
}
