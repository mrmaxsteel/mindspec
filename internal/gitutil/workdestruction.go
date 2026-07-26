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
// round 3->4, NEW-O1r-B/O3r-1): it detects a recreation ONLY when the
// branch's ENTIRE novel diff against target is A-status (added paths,
// and only those). ANY rename/copy, in-place edit (content or mode-only),
// type change, or DELETION anywhere in that novel diff — alone or mixed
// with genuine additions — makes the recreation invisible to this leg,
// because none of those statuses is stripped before the ancestor-tree
// comparison (see novelPaths' and diffNameStatusBuckets' doc comments for
// why each is excluded). The five shapes fixtured by name below
// (StatedLimit_RenameOfNovelWorkIsMissed,
// StatedLimit_ModifiedNovelWorkIsMissed,
// StatedLimit_ModeOnlyNovelWorkIsMissed,
// StatedLimit_TypeChangeOfNovelWorkIsMissed, and
// StatedLimit_ConflictOnRevertedPathScreensNothing above) — plus
// StatedLimit_NovelDeletionIsMissed and
// StatedLimit_MixedAddAndEditIsMissed in workdestruction_test.go — are
// INSTANCES of that rule, illustrations for the fixture table, never its
// extent: closing any one of them individually (e.g. by relocating
// rename content back to its original path) would still leave the
// general rule, and the other reachable members it predicts, open.
// Closing it fully would require either a structural signal (a deletion
// attributable to the branch's own commit, not merely to the diff
// against target) or re-admitting the over-match masking ruling 2 rolled
// back — both deliberately out of scope for this bead. See
// guard.DestructionClean's doc comment for the consumer-facing
// consequence: Clean means no destructive class was DETECTED, not that
// the merge is certified non-destructive.
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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// errTruncatedHistory is findAncestorWithTree's sentinel for a target
// history rev-list cannot fully see, via either of TWO independent
// mechanisms (spec 127 bead-1 fix round, O1-3; fix round 2 added the
// second, then fix round 3->4 corrected how it is detected — see
// historyTruncated's doc comment): a depth-limited shallow clone
// (isShallowRepo), or an object-replacement mechanism — a refs/replace/*
// ref or a legacy .git/info/grafts file — that measurably shortens what
// `git rev-list target` reaches (historyTruncated). In every case "no
// matching ancestor found" is not a legitimate answer — it is an
// artifact of history that was never fully available to scan. Wrapped,
// never returned bare, so callers can still see the underlying probe's
// own error text — distinguishing shallow from replace/grafts, but (as
// of fix round 3->4's single differential measurement replacing the two
// separate presence probes) no longer distinguishing a replace ref from
// a grafts file within that second category, since a single count
// comparison cannot tell which of the two moved it.
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
// predicate's contract forbids, and a permanent condition where AC-8(iv)
// requires a retryable one; and under-fire, because `for-each-ref
// refs/replace/` is blind to a replace ref created under a relocated
// GIT_REPLACE_REF_BASE (git honors that variable, concatenating it with
// the target OID with NO normalization — verified empirically: a base of
// "refs/myreplace" with no trailing slash still works as a real
// replacement, at the ref name "refs/myreplacemyreplace<hex>", which
// `for-each-ref refs/replace/` and `for-each-ref refs/myreplace/` both
// miss), so a genuinely truncating relocated ref reported "not
// truncated" — the exact fail-open this predicate exists to refuse,
// reappearing inside the mechanism added to close it.
//
// This measures the EFFECT instead of the mechanism, which answers both
// directions with one differential and needs no enumeration of where a
// replace ref might live: it compares `git rev-list --count target` as
// the real scan will run it against the SAME count with
// `--no-replace-objects` (a top-level git option that disables replace-
// object lookup UNIVERSALLY — verified: it bypasses a replace ref
// regardless of which ref namespace stores it, including the relocated,
// no-slash-normalized shape above) and `GIT_GRAFT_FILE=/dev/null` (grafts
// have no equivalent top-level flag — `-c core.graftFile=` does NOT
// work, verified; the environment variable is the only bypass) both
// applied together. Truncated iff the bypassed count is STRICTLY
// GREATER: a benign replacement (same reachable set either way) reports
// false, never a refusal; a genuinely truncating one, at ANY ref
// location or via grafts, reports true, because disabling it always
// restores the full count regardless of where it was hiding.
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
	asSeen, err := revListCount(workdir, target, nil)
	if err != nil {
		return false, fmt.Errorf("rev-list --count %s: %w", target, err)
	}
	full, err := revListCount(workdir, target, []string{"GIT_GRAFT_FILE=/dev/null"}, "--no-replace-objects")
	if err != nil {
		return false, fmt.Errorf("rev-list --count %s (bypassing replace-objects/grafts): %w", target, err)
	}
	return full > asSeen, nil
}

// revListCount runs `git rev-list --count target`, prefixed with any
// topLevelOpts (top-level git options — e.g. --no-replace-objects — which
// must precede the subcommand, hence the separate parameter rather than
// folding into a generic args list) and with extraEnv appended to the
// child's environment when non-empty (nil leaves Env unset, inheriting
// the process environment exactly like every other read-only probe in
// this file).
func revListCount(workdir, target string, extraEnv []string, topLevelOpts ...string) (int, error) {
	args := append(append([]string{}, topLevelOpts...), "rev-list", "--count", target)
	cmd := execCommand("git", gitArgs(workdir, args...)...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	n, perr := strconv.Atoi(strings.TrimSpace(string(out)))
	if perr != nil {
		return 0, fmt.Errorf("parsing rev-list --count output %q: %w", string(out), perr)
	}
	return n, nil
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
		return "", fmt.Errorf("%s: a replace ref or info/grafts file measurably shortens rev-list's view: %w", target, errTruncatedHistory)
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
