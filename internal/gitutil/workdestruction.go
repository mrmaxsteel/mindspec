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
// the thin wrapper internal/lifecycle/gitquery.go declares as a
// package-level `var`, not a `func`, precisely so a pointer-equality test
// can pin wrapper ≡ implementation (see gitquery_test.go). gitutil already
// hosts every decision primitive this predicate composes
// (NetEffectLanded, ContentSubsumedOutcome, IsAncestor,
// PreviewDeletedPaths) and imports only guard/termsafe/containment — no
// cycle. The closed outcome enum itself stays in internal/guard (already a
// direct import of both executor and lifecycle, and of gitutil).
//
// The stale-deletion discriminator, corrected (plan-approve ruling, spec
// amended c43e3c93): a computed merge-base is itself forged by branch
// recreation — on a branch recreated from the target's own tip carrying an
// old tree, `merge-base(branch, target)` trivially resolves to target's
// tip, so the reverting commit reads as authoring its own deletions under
// any definition grounded in that range. Authorship is grounded instead in
// the branch's OWN novel contribution: strip the paths the branch adds
// relative to the target from the branch's tip tree; if the stripped tree
// exactly matches the tree of some commit in the target's own history, the
// branch — net of its own novel work — reconstructs a prior state of the
// target, and its preview-deletions are staleness artifacts, not authored
// changes. See snapshotRevertMatch below for the mechanics, and
// workdestruction_test.go for the probed fixtures (single- and
// multi-commit recreation, the #218 shape routed to DestructionSuperseded
// instead, the AC-8(ii)/(iii) boundary shapes, and the conservative corner
// where a cleanup's deletions exactly equal a whole ancestor delta).
package gitutil

import (
	"fmt"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// WorkDestructionEvidence carries the facts EvaluateWorkDestruction
// gathered while deciding — always populated with whatever was resolved
// before the deciding leg fired, so a refusal (or a DestructionEvidenceError)
// can name the evidence, not just the outcome.
type WorkDestructionEvidence struct {
	// MergeBase is the merge-base of branch and target, when resolved.
	MergeBase string
	// AncestorOf names the ref (target or "main") the branch was found to
	// be an already-merged ancestor of. Populated only for
	// DestructionAncestor.
	AncestorOf string
	// SupersededVia names the ref (target or "main") NetEffectLanded found
	// the branch's content already landed against. Populated only for
	// DestructionSuperseded.
	SupersededVia string
	// DeletedPaths is the D-set PreviewDeletedPaths computed (the merge
	// preview's deletions relative to target's tip). Populated only for
	// DestructionStaleDeletion.
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
var workDestructionSubsumedFn = ContentSubsumedOutcome
var workDestructionPreviewDeletedFn = PreviewDeletedPaths
var workDestructionNovelPathsFn = novelPaths
var workDestructionStripPathsFn = stripNovelPaths
var workDestructionFindAncestorTreeFn = findAncestorWithTree

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
// DestructionAncestor proceeds as a no-op (the documented post-conflict
// recovery flow's convergence depends on it); the direct spec→main
// producer applies the identical table.
//
// Evaluation order, read-only throughout (mutates no refs, index, or
// worktree — every underlying primitive shares that discipline):
//
//  1. Ancestry: is branch already an ancestor of target, or of main?
//  2. Supersession: has branch's content already landed in target, or in
//     main, via another route (NetEffectLanded)? A genuine content
//     conflict against target (ContentSubsumedOutcome's SubsumptionConflict
//     leg) is recorded so the stale-deletion leg below can skip a preview
//     that would only re-discover the same conflict.
//  3. Stale-deletion: does the merge preview delete target-present
//     content (PreviewDeletedPaths), and does branch — net of its own
//     novel contribution — reconstruct a prior state of target
//     (snapshotRevertMatch)? Skipped entirely when step 2 already found a
//     genuine content conflict against target: that is not this leg's
//     concern, and the real merge attempt surfaces it on its own (R5(d)).
//  4. Otherwise: DestructionClean.
//
// ANY git/infra failure at any probe returns
// (DestructionEvidenceError, evidence naming the failed probe, non-nil
// err) — absence of evidence is never treated as safety.
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

	// 2. Supersession, against target then main. targetConflict records
	// whether the target-side preview merge genuinely CONFLICTS — a fact
	// step 3 needs to decide whether to even attempt its own preview.
	targetConflict := false
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
			base, err := mergeBaseFn(workdir, branch, against)
			if err != nil {
				evidence.FailedProbe = fmt.Sprintf("merge-base(%s, %s)", branch, against)
				return guard.DestructionEvidenceError, evidence, err
			}
			evidence.MergeBase = base
			outcome, err := workDestructionSubsumedFn(workdir, base, branch, against)
			if err != nil {
				evidence.FailedProbe = fmt.Sprintf("ContentSubsumedOutcome(%s, %s)", branch, against)
				return guard.DestructionEvidenceError, evidence, err
			}
			targetConflict = outcome == SubsumptionConflict
		}
	}

	// 3. Stale-deletion.
	if targetConflict {
		// A genuine content conflict at the target merge is not this
		// leg's concern (spec 127 R4(d)): the ordinary merge attempt
		// surfaces it, and R5(d) handles conflict-recovery re-entry. A
		// preview here would only re-discover the identical conflict.
		return guard.DestructionClean, evidence, nil
	}

	deleted, err := workDestructionPreviewDeletedFn(workdir, target, branch)
	if err != nil {
		evidence.FailedProbe = "PreviewDeletedPaths"
		return guard.DestructionEvidenceError, evidence, err
	}
	if len(deleted) == 0 {
		return guard.DestructionClean, evidence, nil
	}

	matched, ancestorSHA, err := snapshotRevertMatch(workdir, branch, target)
	if err != nil {
		evidence.FailedProbe = "snapshot-revert scan"
		return guard.DestructionEvidenceError, evidence, err
	}
	evidence.DeletedPaths = deleted
	if matched {
		evidence.ReconstructedAncestor = ancestorSHA
		return guard.DestructionStaleDeletion, evidence, nil
	}
	return guard.DestructionClean, evidence, nil
}

// snapshotRevertMatch is the snapshot-revert signature: it strips branch's
// own novel contribution (the paths it adds relative to target — never
// its renames/copies, which are moved content, not new content) from
// branch's tip tree, then scans target's own history for a commit whose
// tree exactly equals the stripped result. A match means branch, net of
// its own authored additions, reconstructs a prior state of target — the
// D-set the caller already found non-empty is therefore a staleness
// artifact of that reconstruction, not authored work.
func snapshotRevertMatch(workdir, branch, target string) (matched bool, ancestorSHA string, err error) {
	novel, err := workDestructionNovelPathsFn(workdir, target, branch)
	if err != nil {
		return false, "", fmt.Errorf("finding %s's novel paths relative to %s: %w", branch, target, err)
	}
	strippedTree, err := workDestructionStripPathsFn(workdir, branch, novel)
	if err != nil {
		return false, "", fmt.Errorf("stripping %s's novel paths from its tip tree: %w", branch, err)
	}
	sha, err := workDestructionFindAncestorTreeFn(workdir, target, strippedTree)
	if err != nil {
		return false, "", fmt.Errorf("scanning %s's history for a reconstructed ancestor tree: %w", target, err)
	}
	return sha != "", sha, nil
}

// novelPaths returns the paths branch adds relative to target's tip — an
// A-status `git diff --name-status --find-renames target branch` bucket.
// Renamed/copied content counts as neither novel nor deleted: it moved,
// it was not introduced.
func novelPaths(workdir, target, branch string) ([]string, error) {
	added, _, err := diffNameStatusBucketsFn(workdir, target, branch)
	return added, err
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

// stripNovelPaths builds branch's tip tree with every path in novel
// removed, via a TEMPORARY index (GIT_INDEX_FILE) — the repo's real index,
// refs, and worktree are never touched, and the only objects written are
// unreferenced loose tree objects (the same non-mutating discipline as
// every merge-tree preview in this package): `git read-tree branch` loads
// branch's tree into the temp index; `git update-index --force-remove --
// <novel paths>` removes them from that temp index only; `git write-tree`
// writes the resulting tree and returns its OID. Returns branch's own tip
// tree OID unchanged when novel is empty (no-op strip).
func stripNovelPaths(workdir, branch string, novel []string) (treeOID string, err error) {
	if err := rejectOptionLike(branch); err != nil {
		return "", err
	}

	idxPath, err := tempIndexPath()
	if err != nil {
		return "", fmt.Errorf("allocating temporary index: %w", err)
	}
	defer os.Remove(idxPath)

	env := append(os.Environ(), "GIT_INDEX_FILE="+idxPath)

	readCmd := execCommand("git", gitArgs(workdir, "read-tree", branch)...)
	readCmd.Env = env
	if out, err := readCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("read-tree %s: %s: %w", branch, strings.TrimSpace(string(out)), err)
	}

	if len(novel) > 0 {
		args := append([]string{"update-index", "--force-remove", "--"}, novel...)
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
	return strings.TrimSpace(string(out)), nil
}

// findAncestorWithTree scans target's own history (`git rev-list
// --format='%H %T' target`) for a commit whose tree OID equals wantTree,
// returning its SHA — or ("", nil) when no such commit exists (not an
// error: "no match" is a legitimate, common answer). `git rev-list
// --format=` (unlike `git log --format=`) precedes each formatted line
// with a "commit <sha>" header line; those are skipped by rejecting any
// line whose first field is the literal string "commit" (a real "%H %T"
// data line's first field is always a 40-or-64-hex OID, never that
// literal).
func findAncestorWithTree(workdir, target, wantTree string) (sha string, err error) {
	if err := rejectOptionLike(target); err != nil {
		return "", err
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
		if fields[1] == wantTree {
			return fields[0], nil
		}
	}
	return "", nil
}
