// Landed-merge-commit-identity predicate (Spec 119 R4, Bead 1).
//
// A landed bead is one whose bead/<id> branch has already been merged into
// its spec branch via the in-binary CompleteBead/FinalizeEpic merge path
// (gitutil.MergeInto — every bead->spec merge site, mindspec_executor.go).
// mindspec's forward-only lifecycle needs to recognize this state even when
// the bead itself is not yet closed in Dolt (a prior `mindspec complete` run
// that closed-but-did-not-finish, or an operator's out-of-band recovery
// merge) or when it IS closed but its obligations were never settled — both
// converge through the SAME reconcile path (internal/complete), which
// consumes FindLandedMerge/MergedUnclosed to decide whether it may safely
// skip the (now pointless, and for an absent branch, ERRORING) merge-base /
// merge / branch-cleanup git plumbing while still applying every gate.
package lifecycle

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// ErrLandedMergeNotFound is returned by FindLandedMerge when no first-parent
// merge commit on specBranch positively identifies as beadID's landed
// bead->spec merge. It is a typed sentinel (test with errors.Is) so callers
// can distinguish "positively evaluated, no match" from a genuine git/read
// failure that must NOT be silently treated the same way.
var ErrLandedMergeNotFound = errors.New("no landed merge commit positively identified for bead")

// LandedMerge is the positively-identified bead->spec merge commit for a
// bead.
type LandedMerge struct {
	// SHA is the merge commit itself (M).
	SHA string
	// FirstParent is M^1 — the spec branch's content immediately before
	// the merge. Together with SHA it is the M^1..M evidence range the
	// reconcile path's doc-sync / ADR-divergence gates evaluate.
	FirstParent string
	// SecondParent is the merged bead branch's tip at merge time.
	SecondParent string
}

// Seams (test stubs). Defaults call the real gitutil/panel packages.
// isAncestorFn is the SAME package-level seam orphans.go declares — one
// home per predicate, no duplicate binding. landedRevParseRefFn is used
// (rather than orphans.go's branchExistsFn/gitutil.BranchExists, which
// takes no workdir and always checks the CALLING PROCESS's cwd) because
// FindLandedMerge/MergedUnclosed must resolve the bead branch's existence
// and tip IN root — a specific repo or worktree, not wherever the process
// happens to be running.
var (
	firstParentMergesFn = gitutil.FirstParentMerges
	landedRevParseRefFn = gitutil.RevParseRef
	landedPanelScanFn   = panel.Scan
)

// resolveBranchTip resolves branch's tip commit in root, distinguishing "the
// branch genuinely does not exist" (survives=false, err=nil) from an
// operational git failure (err != nil) via gitutil.ErrRefNotFound.
func resolveBranchTip(root, branch string) (tip string, survives bool, err error) {
	tip, err = landedRevParseRefFn(root, branch)
	if err != nil {
		if errors.Is(err, gitutil.ErrRefNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return tip, true, nil
}

// FindLandedMerge positively identifies the bead->spec merge commit for
// beadID on specBranch (plan.md Bead 1 Step 4 / spec R4).
//
// Mechanism: scan specBranch's first-parent merge commits newest-first
// (gitutil.FirstParentMerges). The first commit whose subject is EXACTLY
// "Merge bead/<id>" (gitutil.MergeInto's deterministic message — the ONLY
// bead->spec merge producer) is accepted iff every AVAILABLE corroboration
// agrees:
//
//   - a registered panel's reviewed_head_sha (panel.Scan/ForBead over the
//     repo root and, when specID is derivable from specBranch, the spec's
//     own co-located reviews/ directory), when present and non-empty, must
//     equal or be an ancestor of the candidate's second parent;
//   - a surviving bead/<id> branch's tip, when the branch still exists,
//     must equal or be an ancestor of the candidate's second parent.
//
// A subject match contradicted by an available corroboration is NOT a
// positive identification — this function returns ErrLandedMergeNotFound
// (it does not keep scanning past a contradicted match: the deterministic
// "Merge bead/<id>" subject is produced by exactly one merge per bead in
// the normal lifecycle, so a second, older match would be a DIFFERENT
// bead's history collision, never the intended target).
//
// A fresh bead branch with zero own commits can never produce a matching
// merge commit by construction: `git merge --no-ff` of an already-ancestor
// branch performs no merge ("Already up to date") and creates no commit —
// so FindLandedMerge correctly reports ErrLandedMergeNotFound for it.
//
// Any git/read failure (a first-parent-merges scan failure, or a
// surviving-branch rev-parse failure) is returned as-is (not wrapped in
// ErrLandedMergeNotFound) so callers can fail closed on infra trouble
// rather than silently treating it as "not found".
func FindLandedMerge(root, specBranch, beadID string) (*LandedMerge, error) {
	if strings.TrimSpace(beadID) == "" {
		return nil, fmt.Errorf("%w: empty bead id", ErrLandedMergeNotFound)
	}
	beadBranch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid bead id %s: %v", ErrLandedMergeNotFound, beadID, err)
	}
	wantSubject := "Merge " + beadBranch

	merges, err := firstParentMergesFn(root, specBranch)
	if err != nil {
		return nil, fmt.Errorf("scanning %s for a landed merge of %s: %w", specBranch, beadID, err)
	}

	reviewedHeadSHA, haveReviewed := reviewedHeadSHAForBead(root, specBranch, beadID)

	branchTip, branchSurvives, tipErr := resolveBranchTip(root, beadBranch)
	if tipErr != nil {
		return nil, fmt.Errorf("resolving surviving branch %s: %w", beadBranch, tipErr)
	}

	for _, m := range merges {
		if m.Subject != wantSubject || len(m.Parents) < 2 {
			continue
		}
		firstParent, secondParent := m.Parents[0], m.Parents[1]

		if haveReviewed && reviewedHeadSHA != "" && reviewedHeadSHA != secondParent {
			anc, ancErr := isAncestorFn(root, reviewedHeadSHA, secondParent)
			if ancErr != nil || !anc {
				// Contradicted corroboration — not a positive
				// identification. The deterministic subject match is
				// unique per bead in the normal lifecycle, so there is
				// no "next candidate" to fall back to.
				return nil, fmt.Errorf("%w: %s on %s (reviewed_head_sha %s contradicts merge %s's second parent %s)",
					ErrLandedMergeNotFound, beadID, specBranch, reviewedHeadSHA, m.SHA, secondParent)
			}
		}
		if branchSurvives && branchTip != secondParent {
			anc, ancErr := isAncestorFn(root, branchTip, secondParent)
			if ancErr != nil || !anc {
				return nil, fmt.Errorf("%w: %s on %s (surviving branch %s tip %s contradicts merge %s's second parent %s)",
					ErrLandedMergeNotFound, beadID, specBranch, beadBranch, branchTip, m.SHA, secondParent)
			}
		}

		return &LandedMerge{SHA: m.SHA, FirstParent: firstParent, SecondParent: secondParent}, nil
	}

	return nil, fmt.Errorf("%w: %s on %s", ErrLandedMergeNotFound, beadID, specBranch)
}

// reviewedHeadSHAForBead looks up the reviewed_head_sha recorded by a
// registered panel targeting beadID, scanning the repo root and (when
// resolvable) the owning spec's co-located reviews/ directory. Best-effort:
// no registered panel, or one with an empty reviewed_head_sha, yields
// ("", false) — an UNAVAILABLE corroboration, never treated as a
// contradiction.
func reviewedHeadSHAForBead(root, specBranch, beadID string) (string, bool) {
	// Reverse-derivation gate (ADR-0042 §1 reverse): specID is parsed back
	// OUT of an agent-writable branch name via TrimPrefix. A malformed
	// result is never treated as an ID — corroboration simply proceeds
	// root-only (the same as when specID == "" today).
	specID := strings.TrimPrefix(specBranch, workspace.SpecBranchPrefix)
	if idvalidate.SpecID(specID) != nil {
		specID = ""
	}
	roots := []string{root}
	if specID != "" {
		if specDir, err := workspace.SpecDir(root, specID); err == nil && specDir != "" {
			roots = append(roots, specDir)
		}
	}
	regs := panel.ForBead(landedPanelScanFn(roots...), beadID)
	for _, r := range regs {
		if r.Err != nil {
			continue
		}
		if sha := strings.TrimSpace(r.Panel.ReviewedHeadSHA); sha != "" {
			return sha, true
		}
	}
	return "", false
}

// MergedUnclosed derives the "merged-unclosed" reconcile-eligibility state
// (Spec 119 R4, Bead 1 Step 4): FindLandedMerge positively identifies a
// landed merge AND — when bead/<id> still exists — it is an ancestor of
// specBranch's CURRENT tip. This second check is deliberately against the
// whole spec branch (not just the identified merge's second parent, which
// only pins the merge-TIME tip): a branch carrying NEW commits landed after
// the identified merge — genuinely unmerged, still-in-flight work — is
// correctly NOT flagged as merged-unclosed.
//
// Returns (landed, true, nil) when merged-unclosed; (nil, false, nil) when
// not (no landed merge found, OR the branch survives with new unlanded
// work — both are simply "not this state", not an error); and
// (nil, false, err) only on a genuine git/read failure the caller must not
// silently treat as "not merged-unclosed".
func MergedUnclosed(root, specBranch, beadID string) (*LandedMerge, bool, error) {
	landed, err := FindLandedMerge(root, specBranch, beadID)
	if err != nil {
		if errors.Is(err, ErrLandedMergeNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}

	beadBranch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return nil, false, fmt.Errorf("invalid bead id %s: %w", beadID, err)
	}
	_, survives, tipErr := resolveBranchTip(root, beadBranch)
	if tipErr != nil {
		return nil, false, fmt.Errorf("resolving branch %s: %w", beadBranch, tipErr)
	}
	if !survives {
		return landed, true, nil
	}
	anc, ancErr := isAncestorFn(root, beadBranch, specBranch)
	if ancErr != nil {
		return nil, false, fmt.Errorf("checking ancestry of %s against %s: %w", beadBranch, specBranch, ancErr)
	}
	if !anc {
		return nil, false, nil
	}
	return landed, true, nil
}
