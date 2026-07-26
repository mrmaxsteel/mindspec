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
// deletions exactly equal a whole ancestor delta). A recreation whose OWN
// novel contribution is a rename/copy of a target-present path, an
// in-place edit (content or mode-only) of one, or a TYPE change to one
// (spec 127 bead-1 fix round 2, G1-N1) is a stated, fixtured miss (see
// novelPaths' and diffNameStatusBuckets' doc comments) — each would
// require either relocating content back to its original path, or
// re-admitting the over-match masking ruling 2 rolled back, and both are
// deliberately out of scope for this bead.
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
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// errTruncatedHistory is findAncestorWithTree's sentinel for a target
// history rev-list cannot fully see, via any of THREE independent git
// mechanisms (spec 127 bead-1 fix round, O1-3; fix round 2 added the
// latter two): a depth-limited shallow clone, a refs/replace/* replace
// ref, or a legacy .git/info/grafts file. In every case "no matching
// ancestor found" is not a legitimate answer — it is an artifact of
// history that was never fully available to scan. Wrapped, never
// returned bare, so callers can still see the underlying probe's own
// error text (and which of the three mechanisms triggered it).
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
var workDestructionHasReplaceRefsFn = hasReplaceRefs
var workDestructionHasGraftsFileFn = hasGraftsFile

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
// own novel contribution — both the paths it ADDS and the paths it EDITS
// in place relative to target (never its renames/copies, which are moved
// content, not new or edited content at either path — see novelPaths'
// doc comment for why that stays a stated miss) — from branch's tip tree,
// then scans target's own history for a commit whose tree, similarly
// stripped of the SAME edited paths, exactly equals the stripped branch
// tree. A match means branch, net of its own novel work, reconstructs a
// prior state of target — the D-set the caller already found non-empty is
// therefore a staleness artifact of that reconstruction, not authored
// work.
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
// of this fix round its only actual caller discards it.
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

// tempIndexPath allocates a unique path suitable for a temporary
// GIT_INDEX_FILE: the file itself is removed immediately (git creates the
// index file fresh at `read-tree` time), so the returned path names no
// existing file. This is what keeps stripNovelPaths from ever touching the
// repo's REAL index: `git read-tree`/`update-index`/`write-tree` below run
// with GIT_INDEX_FILE pointed here instead.
func tempIndexPath() (string, error) {
	f, err := os.CreateTemp("", "mindspec-workdestruction-idx-")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if cerr := f.Close(); cerr != nil {
		return "", cerr
	}
	if rerr := os.Remove(path); rerr != nil {
		return "", rerr
	}
	return path, nil
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

	idxPath, err := tempIndexPath()
	if err != nil {
		return "", fmt.Errorf("allocating temporary index: %w", err)
	}
	defer os.Remove(idxPath)

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
			return "", fmt.Errorf("strip verification failed: %v still present in tree %s after --force-remove (path-identity mismatch between the strip list and the temp index)", survivors, tree)
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

// hasReplaceRefs reports whether workdir has any refs/replace/* refs
// (`git for-each-ref refs/replace/`) — spec 127 bead-1 fix round 2,
// O1-3: a replace ref silently substitutes a DIFFERENT commit (and its
// entire ancestry) wherever git would otherwise read the original,
// truncating rev-list's view of target's real history exactly like a
// shallow clone does — but WITHOUT --is-shallow-repository ever
// reporting true. Verified: `git replace --graft <sha> <parent>` on an
// otherwise-full clone makes `rev-list target` count drop while
// is-shallow-repository stays false, and findAncestorWithTree's scan
// (which relied on isShallowRepo alone) read the resulting "no match in
// the (replaced) history I could see" as the definite DestructionClean
// answer on a fixture that classifies DestructionStaleDeletion on the
// same repo with the replace ref removed. `--no-replace-objects` would
// make the scan see past this specific mechanism, but there is no
// equivalent flag for the grafts file below, so detect-and-fail-closed
// (this function, plus hasGraftsFile) is the single mechanic that covers
// both truncation vectors the same way.
func hasReplaceRefs(workdir string) (bool, error) {
	cmd := execCommand("git", gitArgs(workdir, "for-each-ref", "refs/replace/")...)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("for-each-ref refs/replace/: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// hasGraftsFile reports whether workdir's repository has a legacy
// `.git/info/grafts` file — spec 127 bead-1 fix round 2, O1-3: like a
// replace ref, a graft silently rewrites a commit's reported parents (and
// therefore rev-list's reachability from target), without
// --is-shallow-repository ever reporting true. Resolved via `git
// rev-parse --git-path info/grafts` rather than a hardcoded
// `<workdir>/.git/info/grafts` join, so this also works inside a linked
// worktree (whose `.git` is a file, not a directory, and whose
// `info/grafts` lives under the MAIN repository's common dir, not the
// worktree's own).
func hasGraftsFile(workdir string) (bool, error) {
	cmd := execCommand("git", gitArgs(workdir, "rev-parse", "--git-path", "info/grafts")...)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("rev-parse --git-path info/grafts: %w", err)
	}
	path := strings.TrimSpace(string(out))
	if !filepath.IsAbs(path) {
		path = filepath.Join(workdir, path)
	}
	info, statErr := os.Stat(path)
	if statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", path, statErr)
	}
	return !info.IsDir(), nil
}

// findAncestorWithTree scans target's own history (`git rev-list
// --format='%H %T' target`) for a commit whose tree OID — or, when
// maskPaths is non-empty, whose tree OID once maskPaths is ALSO stripped
// from it via stripNovelPaths — equals wantTree, returning its SHA; or
// ("", nil) when no such commit exists (not an error: "no match" is a
// legitimate, common answer). Before scanning, checks that target's
// history is not truncated by any of THREE independent mechanisms — a
// shallow clone (isShallowRepo), a replace ref (hasReplaceRefs), or a
// legacy grafts file (hasGraftsFile), each wrapping errTruncatedHistory
// (spec 127 bead-1 fix round 2, O1-3: the round-1 fix caught only the
// shallow-clone case; replace refs and grafts fail open the SAME way —
// Clean, no error, D-set fully enumerated — via the other two mechanisms
// git offers for the identical effect, "rev-list sees less history than
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
	hasReplace, err := workDestructionHasReplaceRefsFn(workdir)
	if err != nil {
		return "", fmt.Errorf("checking %s's history for replace-ref truncation: %w", target, err)
	}
	if hasReplace {
		return "", fmt.Errorf("%s: refs/replace/* present: %w", target, errTruncatedHistory)
	}
	hasGrafts, err := workDestructionHasGraftsFileFn(workdir)
	if err != nil {
		return "", fmt.Errorf("checking %s's history for grafts truncation: %w", target, err)
	}
	if hasGrafts {
		return "", fmt.Errorf("%s: info/grafts present: %w", target, errTruncatedHistory)
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
