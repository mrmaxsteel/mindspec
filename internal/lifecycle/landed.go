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

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// ErrLandedMergeNotFound is returned by FindLandedMerge when no first-parent
// merge commit on specBranch positively identifies as beadID's landed
// bead->spec merge. It is a typed sentinel (test with errors.Is) so callers
// can distinguish "positively evaluated, no match" from a genuine git/read
// failure that must NOT be silently treated the same way.
var ErrLandedMergeNotFound = errors.New("no landed merge commit positively identified for bead")

// LandedMergeNoEvidence is returned by FindLandedMerge, wrapping
// ErrLandedMergeNotFound (test via errors.Is), when a subject-scan candidate
// merge commit exists on specBranch but NO admissible datum — a registered
// panel, a surviving bead/<id> branch, or the merge-time landed-binding —
// confirms it belongs to THIS bead (spec 121 R5(a)/(c), ADR-0041 §2(ii)).
// The subject text alone is never sufficient (AC-11: a hand-crafted "Merge
// bead/<id>" commit over an unrelated second parent must refuse). Callers
// extract the candidate's SHAs via errors.As to render the R5(c)
// attested-restore forward exit — the one deliberately non-mechanical
// recovery line in the tree: the operator must VERIFY the named merge
// before recreating the branch against it.
type LandedMergeNoEvidence struct {
	BeadID       string
	SpecBranch   string
	MergeSHA     string
	SecondParent string
}

func (e *LandedMergeNoEvidence) Error() string {
	return fmt.Sprintf("%s: %s on %s (candidate merge %s, second parent %s — no admissible datum confirms it)",
		ErrLandedMergeNotFound, e.BeadID, e.SpecBranch, e.MergeSHA, e.SecondParent)
}

// Unwrap makes errors.Is(err, ErrLandedMergeNotFound) succeed for a
// LandedMergeNoEvidence, preserving every existing caller's classification
// while still letting callers that need the candidate detail reach it via
// errors.As.
func (e *LandedMergeNoEvidence) Unwrap() error { return ErrLandedMergeNotFound }

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
	// landedBindingMetadataFn reads a bead's bd metadata for the merge-time
	// landed-binding datum (spec 121 R5(b), ADR-0041 §2(ii)) — the third
	// admissible corroboration FindLandedMerge consults, alongside the
	// registered panel and the surviving branch tip. Seamed so tests can
	// stub a binding without a real bd process.
	landedBindingMetadataFn = bead.GetMetadata
	// landedContentSubsumedFn is Bead 1's shared net-effect primitive
	// (gitutil.ContentSubsumedOutcome — final-review r2 F2-2r: the
	// trichotomy form, NOT the boolean projection NetEffectLanded's leg
	// (a) consumes), consumed here for R5(d) revert/reapply-awareness: a
	// positively-corroborated candidate is identified while its own
	// content is subsumed at specBranch's CURRENT tip OR superseded by
	// later work built on it (a three-way CONFLICT); only the CLEAN
	// not-subsumed shape — a genuine backout — refuses.
	landedContentSubsumedFn = gitutil.ContentSubsumedOutcome
)

// landedBinding is the merge-time landed-binding datum read back from bd
// metadata: the three keys internal/executor's mergeBindingFn writes
// (via bead.MergeMetadata) immediately after a MergeInto succeeds and
// before any cleanup.
type landedBinding struct {
	mergeSHA     string
	secondParent string
}

// wellFormedGitObjectID reports whether s is shaped like a git object id:
// 7-64 hex digits (an abbreviated SHA-1 through a full SHA-256). Spec 121
// final-review G2-1 (ADR-0042 provenance): the landed-binding values are
// read from AGENT-WRITABLE bd metadata, and binding.secondParent flows
// into isAncestorFn as a GIT OPERAND — before any such value may reach a
// git argv it must be structurally incapable of parsing as a git option
// (hex-only can never start with '-') or a revision EXPRESSION (no '.',
// '~', '^', ':', '@'). A non-conforming value makes the binding datum
// ABSENT, never a git operand and never a confirmation.
func wellFormedGitObjectID(s string) bool {
	if len(s) < 7 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// landedBindingForBead reads beadID's recorded landed-binding metadata, if
// any. A missing/empty binding, or a metadata-read failure, yields
// (nil, false) — an UNAVAILABLE datum, never treated as a contradiction
// (the same best-effort discipline as reviewedHeadSHAForBead): the binding
// was only introduced in spec 121, so a pre-121 bead — or one whose binding
// write failed and was instead corroborated through a different datum —
// simply has none.
//
// G2-1 provenance gate: a present-but-malformed value (anything that is
// not a well-formed git object id — an option-like `--foo`, terminal
// control bytes, a rev expression) also yields (nil, false): the executor's
// own binding write only ever records rev-parse output, so a non-conforming
// value is a corrupted or crafted binding — it must never confirm a
// candidate, and above all never reach git as an operand.
func landedBindingForBead(beadID string) (*landedBinding, bool) {
	meta, err := landedBindingMetadataFn(beadID)
	if err != nil || meta == nil {
		return nil, false
	}
	sha, _ := meta["mindspec_landed_merge_sha"].(string)
	sp, _ := meta["mindspec_landed_second_parent"].(string)
	sha = strings.TrimSpace(sha)
	sp = strings.TrimSpace(sp)
	if sha == "" && sp == "" {
		return nil, false
	}
	if (sha != "" && !wellFormedGitObjectID(sha)) || (sp != "" && !wellFormedGitObjectID(sp)) {
		return nil, false
	}
	return &landedBinding{mergeSHA: sha, secondParent: sp}, true
}

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
// beadID on specBranch (plan.md Bead 1 Step 4 / spec R4; hardened by spec
// 121 R5(a)/(d), ADR-0041 §2(ii)).
//
// Mechanism: scan specBranch's first-parent merge commits newest-first
// (gitutil.FirstParentMerges). A commit whose subject is EXACTLY "Merge
// bead/<id>" (gitutil.MergeInto's deterministic message — the ONLY
// bead->spec merge producer) is a CANDIDATE. A candidate is positively
// identified ONLY when at least one admissible datum CONFIRMS it (equals
// or is an ancestor of the candidate's second parent) AND no available
// datum CONTRADICTS it. The subject text alone is never sufficient (spec
// 121 AC-11: this is the "never-subject-only" contract — the pre-121
// fall-through that accepted an uncorroborated subject match has been
// removed; no content heuristic over the second parent's commits
// substitutes for identity, per the spec's Non-Goals). Admissible data:
//
//   - a registered panel's reviewed_head_sha (panel.Scan/ForBead over the
//     repo root and, when specID is derivable from specBranch, the spec's
//     own co-located reviews/ directory);
//   - a surviving bead/<id> branch's tip, when the branch still exists;
//   - the merge-time landed-binding recorded on bd metadata (spec 121
//     R5(b) — the three mindspec_landed_* keys internal/executor's
//     mergeBindingFn writes immediately after a MergeInto succeeds and
//     before any cleanup): confirms by an equal merge SHA, or an
//     equal-or-ancestor second parent.
//
// A subject match CONTRADICTED by an available datum is NOT a positive
// identification — this function returns ErrLandedMergeNotFound (it does
// not keep scanning past a contradicted match: the deterministic "Merge
// bead/<id>" subject is produced by exactly one merge per bead in the
// normal lifecycle, so a second, older match would be a DIFFERENT bead's
// history collision, never the intended target). A subject match with NO
// confirming datum at all — every leg unavailable — returns a
// *LandedMergeNoEvidence carrying the candidate's SHAs (spec 121 R5(c),
// the attested-restore forward exit; AC-11's spoof case and AC-18's honest
// out-of-band merge both land here).
//
// R5(d) revert/reapply-awareness: once a candidate is positively
// corroborated, the three-way outcome of its OWN content-introducing
// change (M^1..M) against specBranch's CURRENT tip
// (gitutil.ContentSubsumedOutcome, Bead 1's shared net-effect primitive in
// its final-review-r2 trichotomy form) decides identification: content
// still subsumed → identified; a three-way CONFLICT (the tip itself
// advanced past M^1 on M's region — later honest work built on/superseding
// M) → identified (landed-then-evolved, the F2-2r fix); only a CLEAN
// divergence (the tip sits at the base state on M's paths — the change was
// genuinely backed out, exactly a `git revert M`'s shape) is NOT
// identified. A revert-then-reapply, in either shape (a later commit
// re-introducing the same net changes, or a cherry-pick of them), leaves
// the content subsumed, so M is identified by construction. "Ever
// reverted ⇒ reject" is structurally inexpressible under this mechanism.
//
// A fresh bead branch with zero own commits can never produce a matching
// merge commit by construction: `git merge --no-ff` of an already-ancestor
// branch performs no merge ("Already up to date") and creates no commit —
// so FindLandedMerge correctly reports the plain ErrLandedMergeNotFound
// (no candidate to name) for it.
//
// Any git/read failure (a first-parent-merges scan failure, a
// surviving-branch rev-parse failure, or a net-effect-check failure) is
// returned as-is (not wrapped in ErrLandedMergeNotFound) so callers can
// fail closed on infra trouble rather than silently treating it as "not
// found".
func FindLandedMerge(root, specBranch, beadID string) (*LandedMerge, error) {
	if strings.TrimSpace(beadID) == "" {
		return nil, fmt.Errorf("%w: empty bead id", ErrLandedMergeNotFound)
	}
	beadBranch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid bead id %s: %v", ErrLandedMergeNotFound, idrender.Bead(beadID), err)
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

	binding, haveBinding := landedBindingForBead(beadID)

	for _, m := range merges {
		if m.Subject != wantSubject || len(m.Parents) < 2 {
			continue
		}
		firstParent, secondParent := m.Parents[0], m.Parents[1]

		confirmed := false

		if haveReviewed && reviewedHeadSHA != "" {
			if reviewedHeadSHA == secondParent {
				confirmed = true
			} else {
				anc, ancErr := isAncestorFn(root, reviewedHeadSHA, secondParent)
				if ancErr != nil || !anc {
					// Contradicted corroboration — not a positive
					// identification. The deterministic subject match is
					// unique per bead in the normal lifecycle, so there is
					// no "next candidate" to fall back to.
					return nil, fmt.Errorf("%w: %s on %s (reviewed_head_sha %s contradicts merge %s's second parent %s)",
						ErrLandedMergeNotFound, beadID, specBranch, reviewedHeadSHA, m.SHA, secondParent)
				}
				confirmed = true
			}
		}
		if branchSurvives {
			if branchTip == secondParent {
				confirmed = true
			} else {
				anc, ancErr := isAncestorFn(root, branchTip, secondParent)
				if ancErr != nil || !anc {
					return nil, fmt.Errorf("%w: %s on %s (surviving branch %s tip %s contradicts merge %s's second parent %s)",
						ErrLandedMergeNotFound, beadID, specBranch, beadBranch, branchTip, m.SHA, secondParent)
				}
				confirmed = true
			}
		}
		if haveBinding {
			switch {
			case binding.mergeSHA != "" && binding.mergeSHA == m.SHA:
				confirmed = true
			case binding.secondParent != "" && binding.secondParent == secondParent:
				confirmed = true
			case binding.secondParent != "":
				// Panel O1-1 (spec 121 Bead 2 fix round): tightened to the
				// SAME fail-closed-contradiction shape as the panel and
				// surviving-branch legs above — an ancestry-check FAILURE
				// here is treated the same as a definitive non-ancestor
				// (never silently skipped), so a recorded binding whose
				// relationship to this candidate could not be determined
				// is a contradiction, not a silently-unconfirmed leg.
				anc, ancErr := isAncestorFn(root, binding.secondParent, secondParent)
				if ancErr != nil || !anc {
					// G2-1: binding.mergeSHA/binding.secondParent come from
					// agent-writable bd metadata — the provenance gate above
					// (wellFormedGitObjectID in landedBindingForBead) already
					// constrains them to hex, but render them escaped anyway
					// so this message can never carry a raw hostile value.
					return nil, fmt.Errorf("%w: %s on %s (landed-binding merge %s/second-parent %s contradicts merge %s's second parent %s)",
						ErrLandedMergeNotFound, beadID, specBranch, termsafe.Escape(binding.mergeSHA), termsafe.Escape(binding.secondParent), m.SHA, secondParent)
				}
				confirmed = true
			}
		}

		if !confirmed {
			// R5(c): a subject match with every corroboration leg
			// unavailable — not identified, but NAME the candidate so the
			// caller can render the attested-restore forward exit.
			return nil, &LandedMergeNoEvidence{
				BeadID: beadID, SpecBranch: specBranch,
				MergeSHA: m.SHA, SecondParent: secondParent,
			}
		}

		// R5(d): revert/reapply-awareness. The three-way
		// merge-tree(base=M^1, ours=tip, theirs=M) outcome is
		// DISCRIMINATED (final-review r2 F2-2r), not collapsed to a
		// boolean:
		//
		//   - SubsumptionLanded — M's content is present, net-effect, at
		//     specBranch's CURRENT tip (includes both AC-10(ii)
		//     revert-then-reapply shapes) → identified.
		//   - SubsumptionConflict — the tip has ITSELF advanced past M^1
		//     on M's own region, incompatibly with re-applying M. M is in
		//     the tip's first-parent history, so this means later honest
		//     work EVOLVED/superseded M's content (a conflict-resolution
		//     region edited again, a bead's file rewritten by a later
		//     bead) — landed-then-evolved → identified. Refusing here was
		//     the F2-2r permanent-refusal deadlock: no recovery converges
		//     (restoring the branch re-corroborates and re-refuses), and
		//     the only exits were the bd-close bypass or reverting
		//     legitimate later work — the §2(i) class ADR-0041 forbids.
		//   - SubsumptionCleanDivergence — the three-way applies M's
		//     change CLEANLY but the result differs from the tip: the tip
		//     sits at the base state on M's own paths, i.e. the change
		//     was genuinely BACKED OUT (exactly what a `git revert M`
		//     leaves behind) → NOT identified (AC-10(i) RED-on-revert).
		//     Conservative residual: content later removed cleanly and
		//     fully by honest work is content-indistinguishable from a
		//     revert, so it also refuses — by design, since any datum
		//     here that accepted clean removal would accept every real
		//     revert too.
		outcome, subErr := landedContentSubsumedFn(root, firstParent, m.SHA, specBranch)
		if subErr != nil {
			return nil, fmt.Errorf("checking net effect of merge %s since it landed: %w", m.SHA, subErr)
		}
		if outcome == gitutil.SubsumptionCleanDivergence {
			return nil, fmt.Errorf("%w: %s on %s (merge %s's content is no longer present at %s's current tip — it was reverted after landing)",
				ErrLandedMergeNotFound, beadID, specBranch, m.SHA, specBranch)
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
// not (no candidate merge exists at all, OR the branch survives with new
// unlanded work — both are simply "not this state", not an error); and
// (nil, false, err) on a genuine git/read failure OR a spec 121
// *LandedMergeNoEvidence (a subject-scan candidate exists but no admissible
// datum confirms it) — the latter is deliberately propagated as an error,
// not swallowed into "not this state", so the caller can render the R5(c)
// attested-restore refusal naming the candidate's SHAs (errors.As) instead
// of a generic "nothing found" message.
func MergedUnclosed(root, specBranch, beadID string) (*LandedMerge, bool, error) {
	landed, err := FindLandedMerge(root, specBranch, beadID)
	if err != nil {
		var noEvidence *LandedMergeNoEvidence
		if errors.As(err, &noEvidence) {
			return nil, false, err
		}
		if errors.Is(err, ErrLandedMergeNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}

	beadBranch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return nil, false, fmt.Errorf("invalid bead id %s: %w", idrender.Bead(beadID), err)
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
