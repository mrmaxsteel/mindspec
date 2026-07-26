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
// the branch's OWN novel contribution: strip the paths the branch adds OR
// edits (in place, including a mode-only change — spec 127 bead-1 fix
// round, O1-4/O2-4: an ADD-only novelPaths misses the common case of a
// recreation whose novel work also touches an existing tracked path)
// relative to the target from the branch's tip tree; if the stripped tree
// exactly matches the tree of some commit in the target's own history —
// itself similarly stripped of the SAME edited paths, so a candidate
// ancestor's own (different) content there never blocks the match — the
// branch, net of its own novel work, reconstructs a prior state of the
// target, and its preview-deletions are staleness artifacts, not authored
// changes. See snapshotRevertMatch below for the mechanics, and
// workdestruction_test.go for the probed fixtures (single- and
// multi-commit recreation, the #218 shape routed to DestructionSuperseded
// instead, the AC-8(ii)/(iii) boundary shapes, the conservative corner
// where a cleanup's deletions exactly equal a whole ancestor delta, and
// the modified-novel-work recreation now caught by the same leg). A
// recreation whose OWN novel contribution is a rename/copy of a
// target-present path is still a stated, fixtured miss (see novelPaths'
// doc comment) — the strip mechanic would need to relocate content back
// to the path's ORIGINAL location, not merely mask it, and that is
// deliberately out of scope for this bead.
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
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// errTruncatedHistory is findAncestorWithTree's sentinel for a shallow or
// otherwise truncated target history (spec 127 bead-1 fix round, O1-3): on
// a depth-limited clone, `git rev-list target` silently stops at the
// graft boundary, so "no matching ancestor found" is not a legitimate
// answer — it is an artifact of history that was never fully available to
// scan. Wrapped, never returned bare, so callers can still see the
// underlying probe's own error text.
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
	// DestructionAncestor.
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
	added, modified, err := workDestructionNovelPathsFn(workdir, target, branch)
	if err != nil {
		return false, "", fmt.Errorf("finding %s's novel paths relative to %s: %w", branch, target, err)
	}
	strip := make([]string, 0, len(added)+len(modified))
	strip = append(strip, added...)
	strip = append(strip, modified...)
	strippedTree, err := workDestructionStripPathsFn(workdir, branch, strip)
	if err != nil {
		return false, "", fmt.Errorf("stripping %s's novel paths from its tip tree: %w", branch, err)
	}
	// modified is passed through as the ancestor-side mask: an ancestor's
	// OWN (necessarily different, since it predates the edit) content at
	// an edited-in-place path must never block the match — only added
	// paths need no such mask, since they are by definition absent at
	// target's tip and are not expected to reappear stripped-for-stripped
	// at an OLDER ancestor either (spec 127 bead-1 fix round, O1-4/O2-4).
	sha, err := workDestructionFindAncestorTreeFn(workdir, target, strippedTree, modified)
	if err != nil {
		return false, "", fmt.Errorf("scanning %s's history for a reconstructed ancestor tree: %w", target, err)
	}
	return sha != "", sha, nil
}

// novelPaths returns the paths branch's own novel contribution touches
// relative to target's tip: added (A-status) and modified-in-place
// (M-status, which also carries a pure mode-only change — git's plumbing
// does not distinguish the two) `git diff --name-status --find-renames
// target branch` buckets.
//
// Renamed/copied content (R/C) is deliberately excluded from BOTH buckets
// and stays a stated, fixtured miss (spec 127 bead-1 fix round, O2-4):
// unlike an added or edited-in-place path, reconstructing "this path's
// prior state" for a rename would require RELOCATING content back to its
// original path in the strip mechanic (stripNovelPaths removes/masks a
// path in place; it does not move content between paths), which is a
// materially different — and materially riskier — mechanic than the
// mask-in-place trick modified paths use. Given the choice between
// shipping that additional mechanic unreviewed in this bead or naming the
// miss precisely and fixturing it (see
// StatedLimit_RenameOfNovelWorkIsMissed in workdestruction_test.go), this
// bead takes the latter.
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
// candidate ANCESTOR commit-ish, to mask the same edited-in-place paths
// out of a candidate's tree before comparing (spec 127 bead-1 fix round,
// O1-4/O2-4).
//
// Hardened (spec 127 bead-1 fix round, S1-2): `git update-index
// --force-remove -- <path>` exits 0 SILENTLY when path is not present in
// the index at all — upstream git behavior, not a bug this function
// introduces, but it means a future path-identity mismatch between the
// caller's `strip` list and the temp index's real contents (S1-1's
// concrete instance was one such mismatch; there could be others) would
// silently no-op the strip with no test failing unless a specific
// regression fixture happens to still be in place. After write-tree,
// verify via `git ls-tree -z` (NUL-delimited, unquoted — the same
// quoting-proof discipline as diffNameStatusBuckets) that none of the
// intended-to-strip paths survive in the resulting tree, and fail loudly,
// naming the mismatch, if any do.
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

// findAncestorWithTree scans target's own history (`git rev-list
// --format='%H %T' target`) for a commit whose tree OID — or, when
// maskPaths is non-empty, whose tree OID once maskPaths is ALSO stripped
// from it via stripNovelPaths (spec 127 bead-1 fix round, O1-4/O2-4: an
// ancestor's own, necessarily different, content at a path branch edited
// in place must never block the match) — equals wantTree, returning its
// SHA; or ("", nil) when no such commit exists (not an error: "no match"
// is a legitimate, common answer). Before scanning, checks that target's
// history is not shallow/truncated (isShallowRepo) — see
// errTruncatedHistory's doc comment. `git rev-list --format=` (unlike
// `git log --format=`) precedes each formatted line with a "commit <sha>"
// header line; those are skipped by rejecting any line whose first field
// is the literal string "commit" (a real "%H %T" data line's first field
// is always a 40-or-64-hex OID, never that literal).
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
