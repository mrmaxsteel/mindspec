// Spec 127 R4 (bead 1): the shared work-destruction predicate. Every
// lifecycle merge producer (internal/executor's CompleteBead, FinalizeEpic,
// and the direct spec→main merge) and every verb-layer preflight
// (internal/complete, internal/approve, through the internal/lifecycle
// ADR-0030 boundary wrapper) consults EvaluateWorkDestruction before a
// merge mutation, so a candidate merge that would destroy work is refused
// before it happens rather than discovered after.
//
// Home: internal/gitutil, not internal/lifecycle and not internal/guard.
// internal/executor must not import internal/lifecycle (its own package
// doc's direct-import boundary), so a predicate homed in lifecycle would
// make the executor producers unable to consult it directly. internal/lint's
// ADR-0030 boundary separately bans internal/gitutil and os/exec from the
// enforcement packages (internal/{validate,approve,complete,state,phase}),
// so the verb layer cannot call this predicate directly either — it rides
// the thin wrapper internal/lifecycle/gitquery.go declares as an
// immutable `func` over an unexported package-level `var` seam, not an
// exported mutable `var` itself (spec 127 bead-1 fix round, G1-2 — this
// package doc comment previously said the opposite and was stale from
// before that fix; see gitquery.go's own doc comment for why the
// exported-var shape was withdrawn), so no consumer package can
// substitute a divergent implementation, and a pointer-equality test
// pins the in-package seam ≡ implementation (see gitquery_test.go).
// gitutil already hosts every decision primitive this predicate composes
// (NetEffectLanded, IsAncestor, PreviewDeletedPaths — ContentSubsumedOutcome
// is no longer one of them: spec 127 bead-1 fix round deleted this
// predicate's only call to it along with the targetConflict short-circuit;
// see EvaluateWorkDestruction's own doc comment, step 2) and imports only
// guard/termsafe/containment — no cycle. The closed outcome enum itself
// stays in internal/guard (already a direct import of both executor and
// lifecycle, and of gitutil).
//
// The stale-deletion discriminator, corrected (plan-approve ruling, spec
// amended c43e3c93): a computed merge-base is itself forged by branch
// recreation — on a branch recreated from the target's own tip carrying an
// old tree, `merge-base(branch, target)` trivially resolves to target's
// tip, so the reverting commit reads as authoring its own deletions under
// any definition grounded in that range. Authorship is grounded instead in
// the branch's OWN novel contribution: strip the paths the branch ADDS
// (only — spec 127 bead-1 fix round 2, ruling 2: a round-1 fix briefly
// also stripped/masked paths the branch EDITS in place, to cover the
// common case of a recreation whose novel work also touches an existing
// tracked path (O1-4/O2-4), but masking those paths out of candidate
// ancestors too made the comparison strictly weaker than tree equality —
// the more a branch touched, the more ancestors could match it — and it
// newly misclassified honest branches as destructive (O1c-B/NEW-O2-b).
// Reverted; see snapshotRevertMatch's doc comment for the mechanism and
// the invariant that ruled it out) relative to the target from the
// branch's tip tree; if the stripped tree exactly matches the tree of
// some commit in target's own history — "target's own history" meaning
// every commit `git rev-list target` reaches, including through a merged
// side branch, not only target's first-parent lineage (spec 127 bead-1
// fix round, O1-6): a match against such a commit is target having
// carried that tree at SOME point, even if not on its direct mainline, so
// the branch — net of its own authored additions — reconstructs a prior
// state of the target, and its preview-deletions are staleness
// artifacts, not authored changes. This widens the corner's false-refusal
// surface slightly (a merged side branch's own tree, never target's
// first-parent tip, can still trigger a match) but never its miss
// surface, and stays fail-closed in the same direction as the
// conservative corner below. See
// snapshotRevertMatch below for the mechanics, and workdestruction_test.go
// for the probed fixtures (single- and multi-commit recreation, the #218
// shape routed to DestructionSuperseded instead, the AC-8(ii)/(iii)
// boundary shapes, and the conservative corner where a cleanup's
// deletions exactly equal a whole ancestor delta).
//
// The GENERAL rule for the stale-deletion leg's miss surface (stated once
// here rather than left to be inferred from examples, spec 127 bead-1 fix
// round 3->4, NEW-O1r-B/O3r-1). CORRECTED fix round 5 (NEW-O2G-2/NEW-O3g-2):
// the rule as first stated here — "detects a recreation ONLY when the
// branch's ENTIRE novel diff against target is A-status" — is FALSE,
// falsified twice over by this file's own committed fixture table: every
// DETECTED row's novel diff against target necessarily carries the
// revert-induced D-status paths that make it a recreation at all (the
// deletion clause alone declares every positive row invisible), and
// ConflictMaskedRecreationNowCaught is detected with a non-A-status M
// (conflicted.txt) in its novel diff too (falsifying the in-place-edit
// clause specifically). It is not a status-bucket rule over the diff
// against target at all; it is a residual-tree-equality rule over what
// the strip actually leaves behind. Correctly stated: detection requires
// branch's tip tree, with ONLY the paths branch ADDS relative to target
// (A-status, and only those) removed, to be byte-identical to some commit
// tree in target's own history.
//
// CORRECTED AGAIN, in the over-claiming direction this time (spec 127
// bead-1 fix round 6, NEW-O1v-A): the sentence that stood here through
// fix round 5 — "a path blocks the match iff branch's content there
// differs from EVERY candidate ancestor's tree at that path" — is FALSE.
// It names only one of the two mechanisms that can block a match, and it
// says "content" where the comparison is over the whole tree entry.
// Stated correctly: for a given CANDIDATE, a path is a RESIDUE against
// it when the STRIPPED branch tree's entry there (blob AND mode — a
// mode-only difference blocks exactly like a content difference, see
// StatedLimit_ModeOnlyNovelWorkIsMissed) does not equal that candidate's
// tree entry at that path — INCLUDING the case where the strip has
// removed the path from branch's side entirely (leaving no entry there
// at all) while the candidate still has one. That second case is not a
// corner: the strip is UNCONDITIONAL over the A-status bucket, so it
// removes a path whether it is branch's OWN novel work (never existed in
// target's history at all — the ordinary case below) or a RESTORATION of
// content target has since deleted (exists in target's history, at that
// very path, in the candidate being compared against — see
// wdRestoredDeletedPathFixture in workdestruction_test.go): both are
// A-status relative to target's tip, so both are stripped alike, and a
// byte-identical recreation of a real ancestor can therefore still miss,
// because the strip took the restored path's entry away from the side of
// the comparison that needed it. A match requires ONE candidate to
// agree, simultaneously, at EVERY path — paths that each agree with a
// DIFFERENT candidate do not compose into a match.
//
// That is why a rename/copy, an in-place edit (content or mode-only), or
// a type change in branch's OWN novel work is a stated, fixtured miss:
// that tree entry exists nowhere in target's history at that path, so it
// is a residue against every candidate and blocks all of them at once
// (see novelPaths' and diffNameStatusBuckets' doc comments for why none
// of R/C, M, or T is stripped or masked). It is also why the class's own
// defining D-status paths never block detection on their own
// (StaleDeletionWitnessSingleCommit et al.: the added path is
// A-status-stripped, the deleted paths ARE the revert, and the row is
// still caught) and why an M-status path against target does not block
// it either, when branch's tree entry there merely coincides with a
// candidate ancestor's rather than being the branch's own edit
// (ConflictMaskedRecreationNowCaught: conflicted.txt is M-status against
// target because TARGET edited it after the ancestor branch
// reconstructs, but branch still carries that ancestor's own tree entry
// there, so it is not a residue against THAT candidate and does not
// block the match). StatedLimit_NovelDeletionIsMissed shows the
// deletion-shaped miss precisely: it is missed not because its novel
// diff contains a deletion (every caught row's does) but because the
// branch's OWN novel deletion is a residue against the candidates that
// still hold the deleted path (C1, C2) — and, against the one candidate
// that does not (the fixture's own root commit, which never carried
// stale-doc.md at all), it blocks for the ordinary reason, an unrelated
// path (a.txt) disagreeing instead — never a status exemption either
// way. The following shapes are fixtured, by name, in
// workdestruction_test.go's outcome table:
// StatedLimit_RenameOfNovelWorkIsMissed,
// StatedLimit_ModifiedNovelWorkIsMissed,
// StatedLimit_ModeOnlyNovelWorkIsMissed,
// StatedLimit_TypeChangeOfNovelWorkIsMissed,
// StatedLimit_ConflictOnRevertedPathScreensNothing,
// StatedLimit_NovelDeletionIsMissed,
// StatedLimit_MixedAddAndEditIsMissed, and (fix round 6)
// StatedLimit_RestoredDeletedPathIsMissed. They are INSTANCES of that
// rule, illustrations for the fixture table, never its extent: closing
// any one of them individually (e.g. by relocating rename content back
// to its original path) would still leave the general rule, and the
// other reachable members it predicts, open. Closing the novel-work
// misses fully would require either a structural signal (a deletion
// attributable to the branch's own commit, not merely to the diff
// against target) or re-admitting the over-match masking ruling 2 rolled
// back; closing the restored-path miss would require making the strip
// candidate-relative instead of unconditional — a real design change with
// its own over-refusal surface (a restoration that is genuinely novel
// relative to every candidate must still strip) — both deliberately out
// of scope for this bead, the latter filed as its own follow-up rather
// than folded into this fix round. See guard.DestructionClean's doc
// comment for the consumer-facing consequence: Clean means no destructive
// class was DETECTED, not that the merge is certified non-destructive.
//
// A third stated limitation (spec 127 bead-1 fix round 2, O3-2): when the
// candidate merge conflicts ON the very path the recreation reverts (a
// modify/delete conflict), git's merge resolution KEEPS the modified
// side rather than deleting it, so PreviewDeletedPaths' D-set is EMPTY
// and step 3 returns DestructionClean before the snapshot-revert scan
// ever runs — even though the branch's snapshot-revert signature would
// otherwise be positive. This is git-state-INDISTINGUISHABLE from an
// honest stale branch whose merge happens to conflict (both shapes:
// empty D-set, positive signature — removing the D-set gate to reach the
// signature unconditionally was tried and REJECTED, having been shown to
// flip two must-be-Clean rows, HonestStaleBranch and
// ModifyDeleteConflictIsClean, to StaleDeletion), so this is a
// documentation-completeness gap, not a closable fail-open: the real
// merge attempt still stops the operator on the conflict; hand-resolving
// toward the deletion afterward is unguarded. See
// StatedLimit_ConflictOnRevertedPathScreensNothing in
// workdestruction_test.go.
//
// Two whole-repository preconditions make EVERY evaluation return
// DestructionEvidenceError, fail-closed rather than silently wrong (spec
// 127 bead-1 fix round, O1-7/O2-7): a repo with no local `main` ref (the
// hardcoded second ancestry/supersession target in ancestryTargets), and
// branch/target histories with no common ancestor ("unrelated
// histories" — merge-base exits 1). Both surface as
// DestructionEvidenceError with a FailedProbe naming the git primitive
// that failed, never as a silent skip or a guessed outcome.
package gitutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// errTruncatedHistory is findAncestorWithTree's sentinel for a target
// history rev-list cannot fully see AS THE REPOSITORY'S REAL HISTORY, via
// either of TWO independent mechanisms (spec 127 bead-1 fix round, O1-3;
// fix round 2 added the second, then fix round 3->4 and fix round 5 each
// corrected how it is detected — see historyTruncated's doc comment): a
// depth-limited shallow clone (isShallowRepo), or an object-replacement
// mechanism — a refs/replace/* ref or a legacy .git/info/grafts file —
// that measurably shortens OR SUBSTITUTES what `git rev-list target`
// reaches (historyTruncated: fix round 5 widened this from a length-only
// comparison to a (commit, tree)-sequence comparison, so a replacement
// that swaps a real ancestor's tree for a different one without changing
// the count is caught too — see that function's doc comment). In every
// case "no matching ancestor found" is not a legitimate answer — it is an
// artifact of history that was never fully, faithfully available to scan.
// Wrapped, never returned bare, so callers can still see the underlying
// probe's own error text — distinguishing shallow from replace/grafts,
// but not distinguishing a replace ref from a grafts file within that
// second category, since a single listing comparison cannot tell which
// of the two moved it.
var errTruncatedHistory = errors.New("target's history is shallow/truncated: an ancestor scan cannot certify the absence of a match")

// WorkDestructionEvidence carries the facts EvaluateWorkDestruction
// gathered while deciding — always populated with whatever was resolved
// before the deciding leg fired, so a refusal (or a DestructionEvidenceError)
// can name the evidence, not just the outcome.
type WorkDestructionEvidence struct {
	// MergeBase is the merge-base of branch and target, when resolved.
	MergeBase string
	// AncestorOf names the ref (target or "main") the branch was found to
	// be an already-merged ancestor of. Populated only for
	// DestructionAncestor. LOAD-BEARING for a consumer's disposition, not
	// merely diagnostic (spec 127 bead-1 fix round, O1-5): only
	// AncestorOf == target is a true no-op (nothing to merge into target).
	// AncestorOf == "main" (target itself not yet an ancestor) is NOT a
	// no-op — merging branch into target is still a real, tree-changing
	// merge — so a consumer that skips the merge on DestructionAncestor
	// without checking this field against target risks silently dropping
	// that branch's work. See guard.DestructionOutcome's doc comment for
	// the full rationale.
	AncestorOf string
	// SupersededVia names the ref (target or "main") NetEffectLanded found
	// the branch's content already landed against. Populated only for
	// DestructionSuperseded.
	SupersededVia string
	// DeletedPaths is the D-set PreviewDeletedPaths computed (the merge
	// preview's deletions relative to target's tip). Populated whenever a
	// non-empty D-set was resolved — that includes DestructionClean when
	// the deletions turned out to be the branch's own authored work, NOT
	// only DestructionStaleDeletion (spec 127 bead-1 fix round,
	// F1-2/G1-5: the prior doc comment claimed "only for
	// DestructionStaleDeletion", but the assignment already ran before the
	// snapshot-revert match check — the code's actual behavior, not the
	// comment, is what is now pinned and documented).
	DeletedPaths []string
	// ReconstructedAncestor is the target-history commit whose tree
	// exactly matched the branch's tip tree once its own novel paths were
	// stripped — the evidence that the D-set is a staleness artifact.
	// Populated only for DestructionStaleDeletion.
	ReconstructedAncestor string
	// FailedProbe names the primitive that failed. Populated only for
	// DestructionEvidenceError.
	FailedProbe string

	// LandedMergeSHA, LandedMergeFirstParent, and LandedMergeSecondParent
	// are bead 4's enrichment slot: for a DestructionSuperseded outcome,
	// internal/lifecycle additionally calls lifecycle.FindLandedMerge (a
	// symbol gitutil cannot import without a cycle) and writes the
	// identified landed merge's identity back into the already-returned
	// evidence value one layer up. This is enrichment, never the decision
	// — EvaluateWorkDestruction itself leaves these fields zero-valued;
	// the SupersededVia field above is what the decision is grounded in.
	LandedMergeSHA          string
	LandedMergeFirstParent  string
	LandedMergeSecondParent string
}

// --- in-package seams (error-forcing for tests; default pointer-pinned to
// the real symbols in workdestruction_test.go — spec 127 bead 1: "no
// cross-package seam exists for a decision leg at all", so these seams are
// exactly as reachable as the unexported functions they wrap, never wider).

var workDestructionIsAncestorFn = IsAncestor
var workDestructionNetEffectFn = NetEffectLanded
var workDestructionPreviewDeletedFn = PreviewDeletedPaths
var workDestructionNovelPathsFn = novelPaths
var workDestructionStripPathsFn = stripNovelPaths
var workDestructionFindAncestorTreeFn = findAncestorWithTree
var workDestructionIsShallowFn = isShallowRepo
var workDestructionHistoryTruncatedFn = historyTruncated

// ancestryTargets returns the refs EvaluateWorkDestruction checks a
// candidate branch against for ancestry and supersession, in order:
// target first, then "main" — except when target IS "main", in which case
// there is only one distinct ref to check (checking it twice would be a
// wasted duplicate git call, not a behavior change).
func ancestryTargets(target string) []string {
	if target == "main" {
		return []string{"main"}
	}
	return []string{target, "main"}
}

// EvaluateWorkDestruction is the shared work-destruction predicate (spec
// 127 R4): for a candidate merge of branch into target, it classifies the
// merge into one of guard's five closed DestructionOutcome variants. See
// the package doc comment above for the discriminator mechanics and
// per-consumer outcome table pointer: R4(c)/F3-r2-1 pins that a preflight
// (internal/complete, internal/approve) refuses on
// DestructionSuperseded/DestructionStaleDeletion and fails closed
// (retryable, override available) on DestructionEvidenceError, while
// DestructionAncestor proceeds as a no-op ONLY when evidence.AncestorOf
// == target (spec 127 bead-1 fix round, O1-5 — this comment previously
// stated the no-op disposition flatly, with no caveat, contradicting
// guard.DestructionOutcome's own doc comment; see WorkDestructionEvidence
// .AncestorOf's doc comment for why the two sub-cases are not
// interchangeable): the documented post-conflict recovery flow's
// convergence depends on the target-ancestor case specifically, not on
// DestructionAncestor as a whole. The direct spec→main producer applies
// the identical table, with the identical caveat.
//
// Evaluation order, read-only throughout (mutates no refs, index, or
// worktree — every underlying primitive shares that discipline):
//
//  1. Ancestry: is branch already an ancestor of target, or of main?
//  2. Supersession: has branch's content already landed in target, or in
//     main, via another route (NetEffectLanded)?
//  3. Stale-deletion: does the merge preview delete target-present
//     content (PreviewDeletedPaths), and does branch — net of its own
//     novel contribution — reconstruct a prior state of target
//     (snapshotRevertMatch)? Evaluated UNCONDITIONALLY whenever the D-set
//     is non-empty — never skipped merely because the target-side preview
//     also conflicts elsewhere (spec 127 bead-1 fix round, O1-1/O2-1/O3-2:
//     a prior version of this leg short-circuited to DestructionClean the
//     moment the preview conflicted, on the premise that "a preview here
//     would only re-discover the identical conflict" — false, the preview
//     discovers the DELETIONS, which is different information than the
//     conflict, and PreviewDeletedPaths itself no longer treats a
//     conflicted preview as carrying no D-set; see its doc comment). A
//     conflict on one path must never mask a genuine, unrelated deletion
//     on another — the "certifying the destruction" failure this
//     predicate exists to prevent, reappearing at narrower width, if it
//     did.
//  4. Otherwise: DestructionClean — including the case where the
//     candidate merge's own content genuinely conflicts (a real conflict
//     is handled by the merge attempt itself, R5(d), not by this
//     predicate).
//
// ANY git/infra failure at any probe returns
// (DestructionEvidenceError, evidence naming the failed probe, non-nil
// err) — absence of evidence is never treated as safety. Two
// whole-repository preconditions make every probe fail this way: no
// local `main` ref, and unrelated branch/target histories (see the
// package doc comment above).
func EvaluateWorkDestruction(workdir, branch, target string) (guard.DestructionOutcome, WorkDestructionEvidence, error) {
	var evidence WorkDestructionEvidence

	// 1. Ancestry.
	for _, anc := range ancestryTargets(target) {
		isAnc, err := workDestructionIsAncestorFn(workdir, branch, anc)
		if err != nil {
			evidence.FailedProbe = fmt.Sprintf("IsAncestor(%s, %s)", branch, anc)
			return guard.DestructionEvidenceError, evidence, err
		}
		if isAnc {
			evidence.AncestorOf = anc
			return guard.DestructionAncestor, evidence, nil
		}
	}

	// 2. Supersession, against target then main.
	for i, against := range ancestryTargets(target) {
		landed, err := workDestructionNetEffectFn(workdir, branch, against)
		if err != nil {
			evidence.FailedProbe = fmt.Sprintf("NetEffectLanded(%s, %s)", branch, against)
			return guard.DestructionEvidenceError, evidence, err
		}
		if landed {
			evidence.SupersededVia = against
			return guard.DestructionSuperseded, evidence, nil
		}
		if i == 0 {
			// evidence.MergeBase is populated here for downstream
			// consumers (e.g. a copy-pasteable `git diff` hint) — it no
			// longer feeds a target-conflict short-circuit into step 3
			// (deleted along with that short-circuit; see this
			// function's doc comment).
			base, err := mergeBaseFn(workdir, branch, against)
			if err != nil {
				evidence.FailedProbe = fmt.Sprintf("merge-base(%s, %s)", branch, against)
				return guard.DestructionEvidenceError, evidence, err
			}
			evidence.MergeBase = base
		}
	}

	// 3. Stale-deletion — unconditional (see doc comment above: a
	// co-occurring target-side conflict must never mask this leg).
	deleted, err := workDestructionPreviewDeletedFn(workdir, target, branch)
	if err != nil {
		evidence.FailedProbe = "PreviewDeletedPaths"
		return guard.DestructionEvidenceError, evidence, err
	}
	// Populated whenever resolved, on EVERY outcome from here on (not only
	// DestructionStaleDeletion — see the field's doc comment, spec 127
	// bead-1 fix round F1-2/G1-5).
	evidence.DeletedPaths = deleted
	if len(deleted) == 0 {
		return guard.DestructionClean, evidence, nil
	}

	matched, ancestorSHA, err := snapshotRevertMatch(workdir, branch, target)
	if err != nil {
		evidence.FailedProbe = "snapshot-revert scan"
		return guard.DestructionEvidenceError, evidence, err
	}
	if matched {
		evidence.ReconstructedAncestor = ancestorSHA
		return guard.DestructionStaleDeletion, evidence, nil
	}
	return guard.DestructionClean, evidence, nil
}

// snapshotRevertMatch is the snapshot-revert signature: it strips branch's
// own novel contribution (the paths it ADDS relative to target — never
// its renames/copies, nor the paths it edits in place: see novelPaths'
// doc comment for why both stay stated misses) from branch's tip tree,
// then scans target's own history for a commit whose tree exactly equals
// the stripped result, with no ancestor-side masking. A match means
// branch, net of its own added work, reconstructs a prior state of
// target — the D-set the caller already found non-empty is therefore a
// staleness artifact of that reconstruction, not authored work.
//
// Corrected (spec 127 bead-1 fix round 3->4, O3r-1/NEW-O2R-b): this
// paragraph previously said the strip ALSO covered paths branch edits in
// place, and that the ancestor-side comparison was "similarly stripped of
// the SAME edited paths" — true of round 1's brief A+M widening, and
// left unrevised when ruling 2 (below) reverted that widening's CODE
// without reverting this paragraph's PROSE, so it spent this bead's
// second and third fix rounds describing behavior the shipped function
// does not have, at the primary site the package doc above points
// readers to twice. Restored to ab5aca11's original wording, which is
// what the code below has always actually done since that revert.
func snapshotRevertMatch(workdir, branch, target string) (matched bool, ancestorSHA string, err error) {
	added, _, err := workDestructionNovelPathsFn(workdir, target, branch)
	if err != nil {
		return false, "", fmt.Errorf("finding %s's novel paths relative to %s: %w", branch, target, err)
	}
	strippedTree, err := workDestructionStripPathsFn(workdir, branch, added)
	if err != nil {
		return false, "", fmt.Errorf("stripping %s's novel paths from its tip tree: %w", branch, err)
	}
	// No ancestor-side mask (spec 127 bead-1 fix round 2, ruling 2 —
	// O1c-B/NEW-O2-b, a mirror-image failure AND a behavior regression
	// from this bead's original delivery): the fix round briefly passed
	// novelPaths' MODIFIED bucket through here as findAncestorWithTree's
	// maskPaths, so an ancestor's own content at a branch-edited path
	// would never block the match. That made the comparison strictly
	// WEAKER than tree equality — the more paths a branch touched, the
	// more ancestors could match it — and it newly misclassified HONEST
	// branches (cut from target's own current tip, deleting a
	// recently-added file, editing one unrelated tracked path — an
	// everyday bead shape) as DestructionStaleDeletion. The invariant
	// this leg must hold is that the comparison never gets WEAKER as a
	// branch touches more paths; no discriminator was found that keeps
	// that invariant while also matching a branch whose own novel
	// contribution edits a path in place, so this reverts to
	// added-paths-only masking (nil here), matching this bead's original
	// delivery. A recreation whose OWN novel contribution edits an
	// existing tracked path (content or mode-only), or changes an
	// existing path's TYPE (spec 127 bead-1 fix round 2, G1-N1 — see
	// diffNameStatusBuckets' doc comment), is therefore again a STATED,
	// fixtured miss — see StatedLimit_ModifiedNovelWorkIsMissed,
	// StatedLimit_ModeOnlyNovelWorkIsMissed, and
	// StatedLimit_TypeChangeOfNovelWorkIsMissed in
	// workdestruction_test.go — the same disposition, and for the same
	// reason, as the pre-existing rename miss just below. Between a
	// smaller number of destructive recreations going undetected (bounded
	// by the D-set/evidence-error legs upstream and by the real merge
	// attempt's own conflict surfacing) and a false refusal of honest
	// work, this leg accepts the former.
	sha, err := workDestructionFindAncestorTreeFn(workdir, target, strippedTree, nil)
	if err != nil {
		return false, "", fmt.Errorf("scanning %s's history for a reconstructed ancestor tree: %w", target, err)
	}
	return sha != "", sha, nil
}

// novelPaths returns the paths branch's own novel contribution touches
// relative to target's tip: added (A-status), from `git diff
// --name-status --find-renames target branch`, and — informationally
// only, see below — modified-in-place (M-status, which also carries a
// pure mode-only change — git's plumbing does not distinguish the two).
//
// ONLY added is used by snapshotRevertMatch's strip/mask (spec 127
// bead-1 fix round 2, ruling 2 rollback of O1-4/O2-4's brief M-status
// masking — see snapshotRevertMatch's doc comment for the over-match
// failure that caused the rollback). modified is still returned — every
// caller that wants to name the stated miss precisely (or a future
// caller that finds a mask-invariant-preserving use for it) can — but as
// of this fix round its only actual caller discards it. diffNameStatusBucketsFn's
// third return, deleted, is discarded here too (the `_` above) — a path
// branch's own novel work DELETES relative to target is, like an edit or
// a rename, not A-status, so it is likewise a reachable, undocumented-
// until-now instance of the general miss rule stated in this file's
// package doc comment and snapshotRevertMatch's (spec 127 bead-1 fix
// round 3->4, NEW-O1r-B) rather than a sixth independent mechanism.
//
// Renamed/copied content (R/C) and type-changed content (T — spec 127
// bead-1 fix round 2, G1-N1) are deliberately excluded from BOTH buckets
// and stay stated, fixtured misses (spec 127 bead-1 fix round, O2-4; fix
// round 2, G1-N1): unlike an added path, reconstructing "this path's
// prior state" for a rename would require RELOCATING content back to its
// original path in the strip mechanic (stripNovelPaths removes/masks a
// path in place; it does not move content between paths); a modified-
// in-place or type-changed path COULD be masked the same way M-status
// paths briefly were, but doing so reopens the exact over-match failure
// ruling 2 rolled back. Given the choice between shipping either
// mechanic unreviewed in this bead or naming the miss precisely and
// fixturing it (see StatedLimit_RenameOfNovelWorkIsMissed,
// StatedLimit_ModifiedNovelWorkIsMissed,
// StatedLimit_ModeOnlyNovelWorkIsMissed, and
// StatedLimit_TypeChangeOfNovelWorkIsMissed in workdestruction_test.go),
// this bead takes the latter for all four.
func novelPaths(workdir, target, branch string) (added, modified []string, err error) {
	added, _, modified, err = diffNameStatusBucketsFn(workdir, target, branch)
	return added, modified, err
}

// tempIndexPath allocates a fresh, private TEMPORARY DIRECTORY
// (os.MkdirTemp, mode 0700) and returns a path for a GIT_INDEX_FILE
// inside it that names no existing file — git creates the index file
// fresh at `read-tree` time. This is what keeps stripNovelPaths from
// ever touching the repo's REAL index: `git read-tree`/`update-index`/
// `write-tree` below run with GIT_INDEX_FILE pointed here instead.
//
// Corrected (spec 127 bead-1 fix round 3->4, NEW-G1sub-5): a prior
// version reserved the index path via os.CreateTemp + Close + Remove,
// which frees that exact name inside TMPDIR — on a host where TMPDIR is
// shared and world-writable (e.g. Linux CI's default /tmp; not macOS,
// where TMPDIR is a per-user 0700 directory), another local process can
// occupy the freed name before git opens it (a create-then-remove race,
// CWE-377 shape). A private 0700 directory removes the class outright
// rather than bounding it: nothing else can create a same-named entry
// inside a directory only this process can write to. Callers must remove
// the WHOLE returned directory when done, not merely the index path
// within it — see stripNovelPaths' defer.
func tempIndexPath() (dir, path string, err error) {
	dir, err = os.MkdirTemp("", "mindspec-workdestruction-idx-")
	if err != nil {
		return "", "", err
	}
	return dir, filepath.Join(dir, "index"), nil
}

// stripNovelPaths builds ref's tip tree with every path in strip removed,
// via a TEMPORARY index (GIT_INDEX_FILE) — the repo's real index, refs,
// and worktree are never touched, and the only objects written are
// unreferenced loose tree objects (the same non-mutating discipline as
// every merge-tree preview in this package): `git read-tree ref` loads
// ref's tree into the temp index; `git update-index --force-remove --
// <strip paths>` removes them from that temp index only; `git write-tree`
// writes the resulting tree and returns its OID. Returns ref's own tip
// tree OID unchanged when strip is empty (no-op). Despite the name (kept
// for the snapshotRevertMatch/novelPaths call site, its primary caller),
// ref need not be branch: findAncestorWithTree also calls this with a
// candidate ANCESTOR commit-ish, to mask paths out of a candidate's tree
// before comparing when its own maskPaths argument is non-empty — as of
// spec 127 bead-1 fix round 2 (ruling 2), findAncestorWithTree's only
// caller always passes an empty maskPaths, so this second call site is
// presently exercised only with strip==nil (a no-op tree-OID lookup); see
// findAncestorWithTree's doc comment for why.
//
// Defense-in-depth verification, corrected doc claim (spec 127 bead-1 fix
// round, S1-2; corrected fix round 2, O1c-C): `git update-index
// --force-remove -- <path>` exits 0 SILENTLY when path is not present in
// the index at all — upstream git behavior, not a bug this function
// introduces. After write-tree, this verifies via `git ls-tree -z`
// (NUL-delimited, unquoted — the same quoting-proof discipline as
// diffNameStatusBuckets) that none of the intended-to-strip paths survive
// in the resulting tree, and fails loudly, naming the mismatch, if any
// do. Fix round 2 found that this check CANNOT actually detect the class
// its prior doc comment promised ("a future path-identity mismatch
// between the caller's `strip` list and the temp index's real contents
// ... would silently no-op the strip with no test failing"): a mismatch
// (the caller's spelling not present in the index to begin with) and a
// genuinely successful removal are OBSERVATIONALLY IDENTICAL by this
// check — both leave the path absent from `survivors` — so no real
// mismatch input can ever make it fire; O1c-C could construct none. It
// DOES fire under a direct execCommand-seam injection that fabricates a
// stripNovelPaths implementation which fails to remove a path it claims
// to have removed (S1's confirmation) — forcible via a seam is not the
// same as reachable by real input, and this is kept as exactly that:
// defense-in-depth against a future regression in THIS function's own
// read-tree/update-index/write-tree sequence, not a guard against a
// caller/index spelling mismatch. A genuine mismatch must instead be
// caught by verifying PRESENCE before removal, which this function does
// not currently do.
func stripNovelPaths(workdir, ref string, strip []string) (treeOID string, err error) {
	if err := rejectOptionLike(ref); err != nil {
		return "", err
	}

	idxDir, idxPath, err := tempIndexPath()
	if err != nil {
		return "", fmt.Errorf("allocating temporary index: %w", err)
	}
	defer os.RemoveAll(idxDir)

	env := append(os.Environ(), "GIT_INDEX_FILE="+idxPath)

	readCmd := execCommand("git", gitArgs(workdir, "read-tree", ref)...)
	readCmd.Env = env
	if out, err := readCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("read-tree %s: %s: %w", ref, strings.TrimSpace(string(out)), err)
	}

	if len(strip) > 0 {
		args := append([]string{"update-index", "--force-remove", "--"}, strip...)
		rmCmd := execCommand("git", gitArgs(workdir, args...)...)
		rmCmd.Env = env
		if out, err := rmCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("update-index --force-remove: %s: %w", strings.TrimSpace(string(out)), err)
		}
	}

	writeCmd := execCommand("git", gitArgs(workdir, "write-tree")...)
	writeCmd.Env = env
	out, err := writeCmd.Output()
	if err != nil {
		return "", fmt.Errorf("write-tree: %w", err)
	}
	tree := strings.TrimSpace(string(out))

	if len(strip) > 0 {
		if survivors, verr := lsTreeSurvivors(workdir, tree, strip); verr != nil {
			return "", fmt.Errorf("verifying strip result: %w", verr)
		} else if len(survivors) > 0 {
			return "", fmt.Errorf("strip verification failed: %v still present in tree %s after --force-remove (this function's own read-tree/update-index/write-tree sequence did not remove a path it was asked to remove; see this function's doc comment — a caller/index spelling mismatch is NOT detectable here)", survivors, tree)
		}
	}

	return tree, nil
}

// lsTreeSurvivors returns the subset of paths still present in tree,
// checked via `git ls-tree -z -r --name-only` (NUL-delimited, unquoted —
// see diffNameStatusBuckets' doc comment for why that matters for any
// path containing a byte git would otherwise C-quote or misparse).
func lsTreeSurvivors(workdir, tree string, paths []string) ([]string, error) {
	cmd := execCommand("git", gitArgs(workdir, "ls-tree", "-z", "-r", "--name-only", tree)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ls-tree -z -r --name-only %s: %w", tree, err)
	}
	present := map[string]bool{}
	raw := string(out)
	if raw != "" {
		for _, p := range strings.Split(strings.TrimSuffix(raw, "\x00"), "\x00") {
			present[p] = true
		}
	}
	var survivors []string
	for _, p := range paths {
		if present[p] {
			survivors = append(survivors, p)
		}
	}
	return survivors, nil
}

// isShallowRepo reports whether workdir's repository is a shallow clone
// (`git rev-parse --is-shallow-repository`) — spec 127 bead-1 fix round,
// O1-3: on a depth-limited clone, `git rev-list target` silently stops at
// the graft boundary, so findAncestorWithTree's scan would read "no
// matching ancestor was found in the (truncated) history I could see" as
// the definite answer "the deletions are authored", the exact
// absence-of-evidence-as-safety failure this predicate exists to refuse.
func isShallowRepo(workdir string) (bool, error) {
	cmd := execCommand("git", gitArgs(workdir, "rev-parse", "--is-shallow-repository")...)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("rev-parse --is-shallow-repository: %w", err)
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

// historyTruncated reports whether target's history, as
// findAncestorWithTree's own rev-list scan below will see it, is
// truncated by git's object-replacement machinery — a refs/replace/*
// ref, or a legacy .git/info/grafts file — RELATIVE to the full history
// the same commits reach with every replacement mechanism disabled at
// once.
//
// Spec 127 bead-1 fix round 3->4 (NEW-O1r-A/NEW-O2R-a/NEW-G1sub-1),
// replacing this bead's fix round 2 probes (hasReplaceRefs, hasGraftsFile
// — deleted, along with their doc comments' now-corrected claims):
// those asked whether either mechanism EXISTS anywhere in the
// repository, never whether it actually shortens what rev-list reaches.
// That shape failed in BOTH directions at once, on the SAME
// unconditional-presence design: over-fire, because a benign replace ref
// or grafts file (git's own documented use case — a corrected author
// line on a same-tree commit) truncates nothing yet still turned every
// subsequent evaluation into a PERMANENT DestructionEvidenceError — the
// one failure direction ("a false refusal of honest work") this
// predicate's contract forbids; and under-fire, because `for-each-ref
// refs/replace/` is blind to a replace ref created under a relocated
// GIT_REPLACE_REF_BASE (git honors that variable, concatenating it with
// the target OID with NO normalization — verified empirically: a base of
// "refs/myreplace" with no trailing slash still works as a real
// replacement, at the ref name "refs/myreplace<hex>" — base concatenated
// with the target OID, ONE occurrence of the base (spec 127 bead-1 fix
// round 5, NEW-G1sub-7: a prior version of this comment, and of the
// illustration in workdestruction_test.go, doubled the base to
// "refs/myreplacemyreplace<hex>", caught by direct verification against
// `git for-each-ref`) — which `for-each-ref refs/replace/` and
// `for-each-ref refs/myreplace/` both miss), so a genuinely truncating
// relocated ref reported "not truncated" — the exact fail-open this
// predicate exists to refuse, reappearing inside the mechanism added to
// close it.
//
// This measures the EFFECT instead of the mechanism, which answers both
// directions with one differential and needs no enumeration of where a
// replace ref might live: it compares `git rev-list --format='%H %T'
// target` as the real scan will run it against the SAME listing with
// `--no-replace-objects` (a top-level git option that disables replace-
// object lookup UNIVERSALLY — verified: it bypasses a replace ref
// regardless of which ref namespace stores it, including the relocated,
// no-slash-normalized shape above) and `GIT_GRAFT_FILE=/dev/null` (grafts
// have no equivalent top-level flag — `-c core.graftFile=` does NOT
// work, verified; the environment variable is the only bypass) both
// applied together. Truncated iff the two (commit, tree) MULTISETS differ
// AT ALL — not iff the bypassed count is merely larger, and (fix round 6,
// see equalCommitTreeMultisets' doc comment) not merely because rev-list's
// own commit-date-driven ordering places the identical pairs in a
// different sequence.
//
// CORRECTED fix round 5 (NEW-G1sub-6/NEW-O1g-A/NEW-O2G-1/NEW-O3g-1): the
// prior design compared only `rev-list --count`, i.e. the LENGTH of the
// two traversals, on the premise (stated here and now known false) that
// "a benign replacement reports false because it leaves the same
// reachable set either way, and disabling it always restores the full
// count regardless of where it was hiding." Neither half of that premise
// holds: an object replacement can substitute a DIFFERENT tree onto an
// ancestor with the SAME parents — git-replace's own headline use case, a
// content/metadata correction that preserves ancestry by construction, is
// exactly this shape — leaving the count identical on both sides of the
// differential while the tree at that position is not the repository's
// real one. findAncestorWithTree's scan consumes trees, not counts, so a
// count-equal differential reported "not truncated" while the scan then
// matched (or failed to match) against a history that was not target's
// real one — reopening, inside this mechanism, the exact absence-of-
// evidence-as-safety failure O1-3 was filed to close. Comparing the full
// (commit, tree) multiset closes this for a COMMIT-object substitution or
// a grafts entry: either changes the tree half of at least one pair (a
// substituted commit keeps its %H but not its %T) or the count itself (a
// grafts entry rewrites the parent list, changing which ancestors rev-list
// reaches at all), so it differs from the as-seen multiset and is caught.
// It does NOT close a TREE- or BLOB-object substitution reachable from
// target (spec 127 bead-1 fix round 6, NEW-O1v-B, MINOR, not fixed): a
// commit's %T is the OID recorded in the commit object, so replacing the
// TREE or BLOB object that OID names leaves both %H and %T unchanged on
// both legs of the differential while the content the scan's tree-OID
// comparison stands on is doctored underneath it — an exotic class (no
// everyday git workflow replaces a tree or a blob directly; git-replace's
// real-world uses are commit-level, which this differential does catch)
// left as a stated, undefended gap rather than a claimed-closed one.
// Verified across seven shapes: silent on a pristine repo, on a benign
// same-tree author/message-only replace on a LINEAR target history
// (rev-list's %H reports the ORIGINAL commit's OID even through a
// replacement, and an author-only correction changes neither the OID nor
// the tree, so the two multisets stay identical — the no-over-fire
// property NEW-O1r-A/NEW-G1sub-2 pin stays closed), on a benign grafts
// entry naming a commit's TRUE parents, and (fix round 6,
// TestHistoryTruncated_BenignAuthorOnlyReplaceAcrossMergePermutesButDoesNotOverRefuse)
// on that same benign same-tree replace when target's history contains a
// MERGE — the shape that lets the replacement's own committer date (left
// unset in real use, so git defaults it to the current time; the pinning
// test sets an explicit, later, deterministic date instead, to reproduce
// the same later-than-its-sibling relationship without depending on real
// time) permute its position relative to a sibling commit in rev-list's
// date-ordered output, which an element-for-element comparison mistook
// for truncation (NEW-G1sub-11);
// fires on a same-parents/different-tree replace (the hole above), a
// longer or shorter decoy chain via `replace --graft`, a truncating
// info/grafts entry, and the relocated GIT_REPLACE_REF_BASE shape
// TestHistoryTruncated_RelocatedReplaceRefStillDetected pins.
//
// TRADEOFF, stated rather than left inferred (G1sub's ruling): this is a
// strictly NARROWER acceptance than the length-only design — a legitimate
// history-editing replacement that rewrites a commit's TREE on purpose
// (e.g. excising a bad blob via `git replace --edit` or an equivalent
// content-rewriting BFG/filter-repo-style pass) is now refused here too,
// where the length-only design would have let it through silently. That
// is the correct direction for a predicate whose contract is "absence of
// evidence is never treated as safety": the refusal is retryable via the
// audited override (AC-8(iv)), not permanent, and a caller that genuinely
// intends a tree-rewriting replacement can re-run under that override:
// none of the other legs are widened, so this changes no OTHER outcome.
//
// isShallowRepo above is untouched by this and stays checked first: a
// shallow clone's boundary (the $GIT_DIR/shallow file) is a distinct
// mechanism neither flag here touches, so it truncates both sides of
// this differential equally and the differential alone cannot see it.
func historyTruncated(workdir, target string) (bool, error) {
	// Redundant with findAncestorWithTree's own top-of-function guard (its
	// only caller), kept directly here anyway — SEC-5 discipline elsewhere
	// in this file guards every ref-bearing operand at the function that
	// spends it, not only transitively at a caller several frames up.
	if err := rejectOptionLike(target); err != nil {
		return false, err
	}
	asSeen, err := revListCommitTrees(workdir, target, nil)
	if err != nil {
		return false, fmt.Errorf("rev-list --format='%%H %%T' %s: %w", target, err)
	}
	// -c advice.graftFileDeprecated=false is belt-and-braces, not noise
	// suppression (spec 127 bead-1 fix round 6, NEW-O1v-C/NEW-G1sub-13
	// corrected the claim that stood here through fix round 5, which said
	// setting GIT_GRAFT_FILE makes git print its deprecation advice "on
	// stderr, on EVERY call" and called that "noise on a load-bearing
	// path" — false for THIS call: revListCommitTrees below runs via
	// cmd.Output(), which captures the child's stderr into a buffer and
	// discards it on a zero exit, so the advice never reached an operator
	// through this leg either with or without the flag, verified directly
	// by redirecting stderr to a file on both sides of the flag). What the
	// flag actually protects is the FAILURE path: revListCommitTrees wraps
	// a non-zero exit's stderr into its returned error (fix round 5,
	// NEW-G1sub-10), and without this flag that wrapped text would carry
	// the deprecation advice alongside git's real diagnostic whenever this
	// bypass leg's own exit is non-zero for some OTHER reason. The as-seen
	// leg two lines up carries no such flag and would surface the same
	// advice unsuppressed on ITS failure path if a real info/grafts file
	// is present — a live asymmetry, left as-is because as-seen's failure
	// path is not this leg's job to polish. The removal-dependency note
	// stands regardless of the noise correction: if a future git drops
	// grafts support outright, this leg's grafts coverage (though not its
	// replace-ref coverage) goes with it, and whoever meets that removal
	// should start here.
	full, err := revListCommitTrees(workdir, target, []string{"GIT_GRAFT_FILE=/dev/null"}, "-c", "advice.graftFileDeprecated=false", "--no-replace-objects")
	if err != nil {
		return false, fmt.Errorf("rev-list --format='%%H %%T' %s (bypassing replace-objects/grafts): %w", target, err)
	}
	return !equalCommitTreeMultisets(asSeen, full), nil
}

// equalCommitTreeMultisets reports whether a and b — each an unordered
// capture of "sha tree" strings from revListCommitTrees — carry the
// IDENTICAL MULTISET of pairs, ignoring order (spec 127 bead-1 fix round
// 6, NEW-G1sub-11; renamed from equalCommitTreeSequences, since a
// sequence — order-sensitive by definition — is exactly what this no
// longer compares; see below for why both the old name and the old
// behavior were wrong). A length mismatch (a shortened or lengthened
// traversal) or any pair present a different number of times in one
// multiset than the other (a same-length substitution, the shape fix
// round 5 closed) both count as a difference; a mere reordering of the
// identical pairs does not.
//
// CORRECTED fix round 6 (NEW-G1sub-11): the element-for-element sequence
// comparison fix round 5 shipped here was ITSELF a false refusal of
// honest work — the one failure direction this predicate's contract
// forbids, and the direction round 2's presence probes were rejected for
// (see historyTruncated's doc comment). `git rev-list`'s default order is
// commit-date driven, not purely topological: two commits with no
// ancestor relationship to each other (e.g. two side branches merged
// separately into target) are ordered by their own commit dates. A
// same-tree, same-parents author/message-only replacement — git-replace's
// own headline use case, and the exact shape the doc comment above names
// as staying silent — gives the REPLACEMENT commit a new committer date
// by default (`git replace --edit`, or any commit-tree recipe, both
// produce this), which can swap that commit's position relative to a
// SIBLING commit whenever target's history contains a merge. Neither
// commit's OID nor its tree changes, so the two CAPTURES hold exactly the
// same pairs — proved by TestHistoryTruncated_BenignAuthorOnlyReplaceAcrossMergePermutesButDoesNotOverRefuse,
// which pins a merge-bearing target where the old comparison went red.
// Order carries no information the predicate that consumes this result
// uses: findAncestorWithTree iterates every candidate looking for a tree
// match, so any permutation of the same (commit, tree) pairs yields the
// identical conclusion. Comparing order-sensitively was therefore
// STRICTLY STRONGER than the question actually being asked — the mirror
// image of this bead's round-1/round-2 ancestor masking (see
// snapshotRevertMatch's doc comment), which was strictly WEAKER than its
// question and refused honest branches via over-broad wildcards. THE
// COMPARISON MUST BE EXACTLY AS STRONG AS THE QUESTION IT ANSWERS, in
// both directions — narrower and it fails open on a real substitution;
// broader and it fails closed on honest work. Sorting cannot weaken the
// fire cases fix round 5 added: a shortened or lengthened traversal still
// changes the multiset's size, and a tree substitution still keeps the
// commit's OID but changes its tree, so that pair itself differs from
// anything in the other multiset regardless of position.
func equalCommitTreeMultisets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sortedA := append([]string(nil), a...)
	sortedB := append([]string(nil), b...)
	sort.Strings(sortedA)
	sort.Strings(sortedB)
	for i := range sortedA {
		if sortedA[i] != sortedB[i] {
			return false
		}
	}
	return true
}

// revListCommitTrees runs `git rev-list --format='%H %T' target`,
// prefixed with any topLevelOpts (top-level git options — e.g.
// --no-replace-objects — which must precede the subcommand, hence the
// separate parameter rather than folding into a generic args list) and
// with extraEnv appended to the child's environment when non-empty (nil
// leaves Env unset, inheriting the process environment exactly like every
// other read-only probe in this file). Returns the ordered "sha tree"
// pairs it reports, parsed the same way findAncestorWithTree's own scan
// below parses its identical output (skipping rev-list's own "commit
// <sha>" header lines — a real "%H %T" data line's first field is always
// a 40-or-64-hex OID, never that literal).
func revListCommitTrees(workdir, target string, extraEnv []string, topLevelOpts ...string) ([]string, error) {
	args := append(append([]string{}, topLevelOpts...), "rev-list", "--format=%H %T", target)
	cmd := execCommand("git", gitArgs(workdir, args...)...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	out, err := cmd.Output()
	if err != nil {
		// Include the command's own stderr when available (spec 127
		// bead-1 fix round 5, G1sub-10): this leg now carries the whole
		// replace/grafts verdict, so its error text is the operator's
		// only handle when a partial clone or similar cannot resolve the
		// objects rev-list needs — "exit status 128" alone is
		// undiagnosable, git's own explanation is not.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	var pairs []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] == "commit" {
			continue
		}
		pairs = append(pairs, fields[0]+" "+fields[1])
	}
	return pairs, nil
}

// findAncestorWithTree scans target's own history (`git rev-list
// --format='%H %T' target`) for a commit whose tree OID — or, when
// maskPaths is non-empty, whose tree OID once maskPaths is ALSO stripped
// from it via stripNovelPaths — equals wantTree, returning its SHA; or
// ("", nil) when no such commit exists (not an error: "no match" is a
// legitimate, common answer). Before scanning, checks that target's
// history is not truncated by either of TWO independent mechanisms — a
// shallow clone (isShallowRepo), or git's object-replacement machinery
// (historyTruncated, covering both a replace ref and a legacy grafts
// file with one measurement — see its doc comment for why they are
// checked together rather than by separate presence probes as of spec
// 127 bead-1 fix round 3->4) — each wrapping errTruncatedHistory (spec
// 127 bead-1 fix round 2, O1-3: the round-1 fix caught only the
// shallow-clone case; replace refs and grafts fail open the SAME way —
// Clean, no error, D-set fully enumerated — via the other mechanism git
// offers for the identical effect, "rev-list sees less history than
// actually exists") — see errTruncatedHistory's doc comment. `git rev-list --format=` (unlike
// `git log --format=`) precedes each formatted line with a "commit <sha>"
// header line; those are skipped by rejecting any line whose first field
// is the literal string "commit" (a real "%H %T" data line's first field
// is always a 40-or-64-hex OID, never that literal).
//
// maskPaths is a general mechanic — mask ANY set of paths out of a
// candidate's tree before comparing — kept generic and exported to this
// file's tests rather than hardcoded to always-empty, but as of spec 127
// bead-1 fix round 2 (ruling 2) its ONLY caller (snapshotRevertMatch)
// always passes nil: masking a branch's edited-in-place paths out of
// every candidate here is exactly the mechanism that made the
// snapshot-revert comparison weaker than tree equality and misclassified
// honest branches (O1c-B/NEW-O2-b). A future caller that finds a
// different, narrower use for masking here — one that does not weaken as
// more paths are masked — is not precluded by this function; the
// invariant to preserve is snapshotRevertMatch's, not this function's own.
func findAncestorWithTree(workdir, target, wantTree string, maskPaths []string) (sha string, err error) {
	if err := rejectOptionLike(target); err != nil {
		return "", err
	}

	shallow, err := workDestructionIsShallowFn(workdir)
	if err != nil {
		return "", fmt.Errorf("checking %s's history for truncation: %w", target, err)
	}
	if shallow {
		return "", fmt.Errorf("%s: %w", target, errTruncatedHistory)
	}
	truncated, err := workDestructionHistoryTruncatedFn(workdir, target)
	if err != nil {
		return "", fmt.Errorf("checking %s's history for replace-object/grafts truncation: %w", target, err)
	}
	if truncated {
		// "changes what rev-list reaches", not "shortens": spec 127 bead-1
		// fix round 6 (NEW-O2V-1/NEW-O3v-1/NEW-G1sub-14) corrected this
		// operator-facing string, which round 5 left saying "measurably
		// shortens" — true of the original length-only design, false of
		// the count-preserving tree substitution round 5 exists to catch
		// (nothing is shortened there; a tree is swapped) — while
		// correcting the same claim everywhere else in this file
		// (errTruncatedHistory's own doc comment, historyTruncated's).
		// This is the one instance of that claim a user actually sees.
		return "", fmt.Errorf("%s: a replace ref or info/grafts file changes what rev-list reaches for this history (a shortened, lengthened, or substituted ancestry): %w", target, errTruncatedHistory)
	}

	cmd := execCommand("git", gitArgs(workdir, "rev-list", "--format=%H %T", target)...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("rev-list --format='%%H %%T' %s: %w", target, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] == "commit" {
			continue
		}
		candidateSHA, candidateTree := fields[0], fields[1]
		if len(maskPaths) == 0 {
			if candidateTree == wantTree {
				return candidateSHA, nil
			}
			continue
		}
		maskedTree, merr := workDestructionStripPathsFn(workdir, candidateSHA, maskPaths)
		if merr != nil {
			return "", fmt.Errorf("masking candidate %s's tree: %w", candidateSHA, merr)
		}
		if maskedTree == wantTree {
			return candidateSHA, nil
		}
	}
	return "", nil
}
