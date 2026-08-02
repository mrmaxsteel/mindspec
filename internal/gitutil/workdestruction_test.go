package gitutil

// Spec 127 bead 1: real-git table fixtures for the shared work-destruction
// predicate (EvaluateWorkDestruction), over ALL FIVE guard.DestructionOutcome
// variants (B-r4-3's sentinel discipline — the model every later consumer
// copies). Reuses this package's existing initGitRepo/neRunGit/neWriteFile
// fixture helpers (neteffect_test.go).
//
// The stale-deletion fixtures below are the Go transcription of the
// plan-time probes committed at /tmp/core1-planrev-scratch/probe.sh
// (shapes A-G): both destructive recreation shapes (A, E) are RED against
// the prior set-subtraction discriminator and GREEN (correctly flagged)
// against this one; the honest shapes (B, C, D) stay clean under both;
// shape F (a genuine content conflict) is outside this leg entirely; shape
// G is the conservative fail-closed corner.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// --- fixture builders -------------------------------------------------------

// wdBranchOffMain creates a new branch off main's current tip. Returns
// nothing; callers checkout onto it themselves via neRunGit for clarity at
// each call site.

// wdAncestorFixture: branch == main's current tip (trivially an ancestor
// of main); target is an UNRELATED orphan history that never contained
// main's commits at all — so branch is an ancestor of main but NOT of
// target. Exercises the "ancestor of main, not of target" leg.
func wdAncestorOfMainFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neRunGit(t, dir, "branch", "stale-ancestor")
	neRunGit(t, dir, "checkout", "--orphan", "unrelated-target")
	neRunGit(t, dir, "rm", "-rf", "--cached", ".")
	neWriteFile(t, dir, "unrelated.txt", "unrelated history\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "unrelated orphan root")
	neRunGit(t, dir, "checkout", "main")
	return dir, "stale-ancestor", "unrelated-target"
}

// wdAncestorOfTargetFixture: branch == target's current tip (an ancestor
// of target directly — the common no-op-merge case).
func wdAncestorOfTargetFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "spec-target")
	neWriteFile(t, dir, "spec.txt", "spec work\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "spec work")
	neRunGit(t, dir, "branch", "already-merged-bead")
	neRunGit(t, dir, "checkout", "main")
	return dir, "already-merged-bead", "spec-target"
}

// wdSupersededFixture is the #218 shape: a stale branch whose content
// already landed in target via ANOTHER route (a squash merge) — a
// genuinely superseded snapshot, distinct from the stale-deletion witness
// below (that shape's net-effect mass deletion never lands anywhere, so
// supersession does not catch it — this one's content is positively
// re-findable at target's tip).
func wdSupersededFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "spec-target")
	neRunGit(t, dir, "checkout", "-b", "stale-bead", "spec-target")
	neWriteFile(t, dir, "feature.txt", "feature content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "spec-target")
	neRunGit(t, dir, "merge", "--squash", "stale-bead")
	neRunGit(t, dir, "commit", "-m", "squash merge feature (another route)")
	neRunGit(t, dir, "checkout", "main")
	return dir, "stale-bead", "spec-target"
}

// wdSupersededViaMainFixture: superseded not against target directly, but
// against main — target itself never saw the content.
func wdSupersededViaMainFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	// spec-target-2 forks BEFORE the squash lands and never receives it —
	// target itself must NOT see the content, only main does.
	neRunGit(t, dir, "branch", "spec-target-2")
	neRunGit(t, dir, "checkout", "-b", "stale-bead")
	neWriteFile(t, dir, "feature.txt", "feature content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feature work")
	neRunGit(t, dir, "checkout", "main")
	neRunGit(t, dir, "merge", "--squash", "stale-bead")
	neRunGit(t, dir, "commit", "-m", "squash merge feature into main directly")
	neRunGit(t, dir, "checkout", "main")
	return dir, "stale-bead", "spec-target-2"
}

// wdStaleDeletionSingleCommitFixture is probe A: a spec branch recreated
// from the target's OWN tip, reverting to an older tree state in ONE
// commit, plus a novel commit on top.
func wdStaleDeletionSingleCommitFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "old.txt", "old1\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 landed work")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated", "main")
	neRunGit(t, dir, "rm", "landed.txt")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts to C1's tree")
	neWriteFile(t, dir, "spec-work.txt", "specwork\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "--amend", "-m", "recreated spec branch: old tree + novel work")
	neRunGit(t, dir, "checkout", "main")
	return dir, "spec-recreated", "main"
}

// wdStaleDeletionMultiCommitFixture is probe E: the same recreation shape,
// but the revert-to-old-tree and the novel work land as TWO SEPARATE
// commits, so a range-based ("did commit X author this deletion")
// discriminator would see a different-shaped history than the
// single-commit variant even though the destructive signature is
// identical.
func wdStaleDeletionMultiCommitFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "base.txt", "base\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "mid.txt", "mid\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C3 landed work")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated-multi", "main")
	neRunGit(t, dir, "rm", "landed.txt", "mid.txt")
	neRunGit(t, dir, "commit", "-m", "restore old snapshot (reverts to C1's tree)")
	neWriteFile(t, dir, "novel.txt", "novel\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "novel work (separate commit)")
	neRunGit(t, dir, "checkout", "main")
	return dir, "spec-recreated-multi", "main"
}

// wdLargeRenameFixture is AC-8(ii): a large directory rename/move with no
// net content loss.
func wdLargeRenameFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	body := "content line one\nmore lines here for similarity\nline three\nline four\n"
	for _, n := range []string{"1", "2", "3", "4", "5"} {
		neWriteFile(t, dir, "dir/f"+n+".txt", body+n+"\n")
	}
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "add dir/")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "landed work")
	neRunGit(t, dir, "checkout", "-b", "mover")
	neRunGit(t, dir, "mv", "dir", "newdir")
	neRunGit(t, dir, "commit", "-m", "move dir -> newdir")
	neRunGit(t, dir, "checkout", "main")
	return dir, "mover", "main"
}

// wdHonestCleanupFixture is AC-8(iii): the generic honest-cleanup shape —
// an older file deleted with later content landed since, in the branch's
// own authored range.
func wdHonestCleanupFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "keep.txt", "keep\n")
	neWriteFile(t, dir, "obsolete.txt", "obsolete\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2")
	neRunGit(t, dir, "checkout", "-b", "cleanup")
	neRunGit(t, dir, "rm", "obsolete.txt")
	neRunGit(t, dir, "commit", "-m", "cleanup: remove obsolete")
	neRunGit(t, dir, "checkout", "main")
	return dir, "cleanup", "main"
}

// wdHonestStaleBranchFixture: a branch forked before some unrelated
// content landed on main — never conflicts, never deletes anything.
func wdHonestStaleBranchFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neRunGit(t, dir, "checkout", "-b", "feat")
	neWriteFile(t, dir, "work.txt", "w\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feat work")
	neRunGit(t, dir, "checkout", "main")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2")
	return dir, "feat", "main"
}

// wdModifyDeleteConflictFixture is probe F: a genuine modify/delete
// conflict — outside the stale-deletion leg entirely. The real merge
// attempt surfaces this conflict on its own (R5(d)); the predicate must
// return Clean, not refuse.
func wdModifyDeleteConflictFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "f.txt", "v1\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neRunGit(t, dir, "checkout", "-b", "del")
	neRunGit(t, dir, "rm", "f.txt")
	neRunGit(t, dir, "commit", "-m", "delete f")
	neRunGit(t, dir, "checkout", "main")
	neWriteFile(t, dir, "f.txt", "v2\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "modify f")
	return dir, "del", "main"
}

// wdConservativeCornerFixture is probe G: a cleanup whose deletions are
// EXACTLY the whole delta since main's previous commit — the result tree
// equals that ancestor's tree exactly. Byte-indistinguishable from
// un-landing the tip's own change; the ruling breaks the tie fail-closed
// (DestructionStaleDeletion, override available), not because intent is
// provably bad but because it is NOT decidable from git state alone.
func wdConservativeCornerFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "a.txt", "a\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 adds only landed.txt")
	neRunGit(t, dir, "checkout", "-b", "cleanup")
	neRunGit(t, dir, "rm", "landed.txt")
	neRunGit(t, dir, "commit", "-m", "cleanup deletes the only thing C2 added")
	neRunGit(t, dir, "checkout", "main")
	return dir, "cleanup", "main"
}

// wdConflictOnRevertedPathFixture is O3-2's third stated limitation
// (spec 127 bead-1 fix round 2): a genuine recreation — the branch, net
// of its own novel work (spec-work.txt), reconstructs C1's tree exactly,
// a positive snapshot-revert signature — whose candidate merge ALSO
// conflicts, but the conflict lands ON the very path the recreation
// reverts (landed.txt): a modify/delete conflict. git's merge resolution
// for modify/delete KEEPS the modified side (with conflict-marker
// content) rather than deleting, so the preview's tree still HAS that
// path — the D-set PreviewDeletedPaths computes is EMPTY, and
// EvaluateWorkDestruction's own D-set-non-empty gate (step 3) returns
// DestructionClean before ever reaching the snapshot-revert scan that
// would otherwise have found the match. This is git-state-
// INDISTINGUISHABLE from an honest stale branch whose merge happens to
// conflict (both: empty D-set, positive signature — O3's own downgrade
// of this finding from MAJOR to MINOR, having tried and failed to find a
// discriminator), so it is a documentation-completeness gap, not a
// closable fail-open: the real merge attempt still stops the operator on
// the conflict; hand-resolving toward the deletion afterward is
// unguarded. See the package doc comment's third stated limitation.
func wdConflictOnRevertedPathFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "old.txt", "old\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "landed.txt", "landed v1\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 landed work")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated-conflict-on-reverted", "main")
	neRunGit(t, dir, "rm", "landed.txt")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts to C1's tree")
	neWriteFile(t, dir, "spec-work.txt", "specwork\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "--amend", "-m", "recreated spec branch: old tree + novel work")
	neRunGit(t, dir, "checkout", "main")
	neWriteFile(t, dir, "landed.txt", "landed v2 (target advances the SAME path the branch reverts)\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C3 target modifies landed.txt")
	return dir, "spec-recreated-conflict-on-reverted", "main"
}

// wdSideBranchReachableAncestorFixture is O1-6's stated corner (spec 127
// bead-1 fix round 2): the reconstructed ancestor findAncestorWithTree
// matches against may be reachable only through a MERGED SIDE BRANCH,
// never through target's own first-parent lineage — target genuinely
// carried that tree at some point in its history, just not on its
// mainline. main adds b.txt, then merges a side branch that (on its own,
// unrelated line of development) added s.txt; an HONEST branch off the
// resulting tip that simply deletes b.txt reconstructs the side branch's
// OWN tip tree {base.txt, s.txt} — a real match, at a real commit target
// actually contains, but not one `git rev-list --first-parent main`
// would ever visit.
func wdSideBranchReachableAncestorFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "base.txt", "base\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neRunGit(t, dir, "checkout", "-b", "side")
	neWriteFile(t, dir, "s.txt", "side work\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "side branch adds s.txt")
	neRunGit(t, dir, "checkout", "main")
	neWriteFile(t, dir, "b.txt", "b\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "main adds b.txt")
	neRunGit(t, dir, "merge", "--no-ff", "-m", "merge side branch into main", "side")
	neRunGit(t, dir, "checkout", "-b", "honest-delete-b")
	neRunGit(t, dir, "rm", "b.txt")
	neRunGit(t, dir, "commit", "-m", "honest: remove b.txt")
	neRunGit(t, dir, "checkout", "main")

	// Fixture invariant: the side branch's own tip must be reachable in
	// `rev-list main` but NOT in `rev-list --first-parent main` —
	// otherwise this fixture would not exercise O1-6's shape at all.
	sideSHA := strings.TrimSpace(neRunGit(t, dir, "rev-parse", "side"))
	fullHistory := neRunGit(t, dir, "rev-list", "main")
	if !strings.Contains(fullHistory, sideSHA) {
		t.Fatalf("fixture invariant broken: side branch's tip %s must be reachable in rev-list main", sideSHA)
	}
	firstParentHistory := neRunGit(t, dir, "rev-list", "--first-parent", "main")
	if strings.Contains(firstParentHistory, sideSHA) {
		t.Fatalf("fixture invariant broken: side branch's tip %s must NOT be reachable via --first-parent main — otherwise this is not a side-branch-only match", sideSHA)
	}
	return dir, "honest-delete-b", "main"
}

// --- spec 127 bead-1 fix round fixtures -------------------------------------

// wdConflictMaskedRecreationFixture is O1-1/O2-1/O3-2's RED-on-the-prior-
// bug shape, REBUILT (spec 127 bead-1 fix round 2, O1c-A/NEW-O2-a): the
// round-1 version never actually conflicted. Its branch left f.txt
// UNTOUCHED at the value it already had when cut, so the candidate
// merge's three-way for that path was base=v1/ours=v2/theirs=v1 — a
// CLEAN fast-forward, not a conflict (ContentSubsumedOutcome on it
// returned SubsumptionCleanDivergence, never SubsumptionConflict) — so
// re-injecting the deleted targetConflict short-circuit left the ENTIRE
// suite green: the BLOCKING fix had NO regression pin.
//
// A REAL conflict needs BOTH sides to diverge from a common base on the
// SAME path, which necessarily makes that path part of the branch's own
// novel diff too (an M-status entry in target..branch) — so this shape
// cannot be built by merely leaving a path untouched. This fixture's
// branch instead actively REVERTS conflicted.txt back to an EARLIER real
// commit's (C0's) exact content — distinct from both C2's ("initial")
// and C3's ("main side") values — while main independently advances
// conflicted.txt to a THIRD value. Once spec-work.txt (the branch's only
// ADDED path) is stripped, the branch's tip tree exactly equals C0's
// tree — a NATURAL, unmasked match (ruling 2 rolled back M-status
// ancestor-side masking, so this fixture does not and must not depend on
// it): merge-base(spec-recreated-conflict, main) = C2 (conflicted.txt=
// "initial"), ours(main)="main side content", theirs(branch)="branch
// side content" — both diverge from base, so the preview genuinely
// conflicts (asserted below, not merely narrated).
func wdConflictMaskedRecreationFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "old.txt", "old\n")
	neWriteFile(t, dir, "conflicted.txt", "branch side content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C0")
	neWriteFile(t, dir, "conflicted.txt", "initial\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1 conflicted.txt diverges from C0's content")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 landed work")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated-conflict", "main")
	neRunGit(t, dir, "rm", "landed.txt")
	neWriteFile(t, dir, "conflicted.txt", "branch side content\n")
	neRunGit(t, dir, "add", "conflicted.txt")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts landed.txt+conflicted.txt to C0's tree")
	neWriteFile(t, dir, "spec-work.txt", "specwork\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "--amend", "-m", "recreated spec branch: old tree + novel work")
	neRunGit(t, dir, "checkout", "main")
	neWriteFile(t, dir, "conflicted.txt", "main side content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C3 target advances conflicted.txt independently")

	// Fixture invariant (NEW-O2-a's own required_change: "Assert
	// res.conflict inside the fixture ... so the fixture can never
	// silently stop conflicting again"), verified via the exact
	// primitive the deleted targetConflict short-circuit used to call.
	base := strings.TrimSpace(neRunGit(t, dir, "merge-base", "spec-recreated-conflict", "main"))
	outcome, err := ContentSubsumedOutcome(dir, base, "spec-recreated-conflict", "main")
	if err != nil {
		t.Fatalf("fixture invariant: ContentSubsumedOutcome: %v", err)
	}
	if outcome != SubsumptionConflict {
		t.Fatalf("fixture invariant broken: the candidate merge must genuinely CONFLICT (ContentSubsumedOutcome=%v, want SubsumptionConflict) — otherwise this is not a red-on-the-prior-bug pin for the deleted targetConflict short-circuit", outcome)
	}
	return dir, "spec-recreated-conflict", "main"
}

// wdModifiedNovelWorkFixture is O1-4/O2-4's "modify-novel-work recreation":
// the same destructive recreation shape, but the branch's own novel
// contribution EDITS an existing target-present path (a.txt) in place,
// rather than adding a new one. novelPaths' strip set is A-status
// (added) ONLY (spec 127 bead-1 fix round 2, ruling 2: a round-1 fix
// briefly widened it to also strip/mask M-status paths, closing this
// shape, but masking those paths out of every candidate ancestor's tree
// too made the comparison strictly weaker than tree equality and
// misclassified HONEST branches as destructive — O1c-B/NEW-O2-b — so it
// was rolled back), so a.txt's edited content stays in the stripped tree,
// blocking the tree-equality match, and this shape is a STATED,
// FIXTURED miss (see StatedLimit_ModifiedNovelWorkIsMissed in the outcome
// table below) — probably the most common real shape a stated miss could
// have, since most bead branches touch tracked files; accepted per
// ruling 2's tradeoff (a smaller number of undetected destructive
// recreations, bounded by the D-set/evidence-error legs upstream and the
// real merge attempt's own conflict surfacing, over a false refusal of
// honest work).
func wdModifiedNovelWorkFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "a.txt", "original\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 landed work")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated-modified", "main")
	neRunGit(t, dir, "rm", "landed.txt")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts to C1's tree")
	neWriteFile(t, dir, "a.txt", "edited in place by novel work\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "--amend", "-m", "recreated spec branch: old tree + novel EDIT of a.txt")
	neRunGit(t, dir, "checkout", "main")
	return dir, "spec-recreated-modified", "main"
}

// wdModeOnlyNovelWorkFixture is O1-4/O2-4's mode-only variant: the
// branch's novel work changes a.txt's MODE only (chmod +x), no content
// change — git's plumbing reports this as "M" too (same blob OID,
// different mode). Same disposition as wdModifiedNovelWorkFixture, and
// for the identical reason (spec 127 bead-1 fix round 2, ruling 2's
// rollback): a stated, fixtured miss, not a catch.
func wdModeOnlyNovelWorkFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "a.txt", "original\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 landed work")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated-modeonly", "main")
	neRunGit(t, dir, "rm", "landed.txt")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts to C1's tree")
	if err := os.Chmod(filepath.Join(dir, "a.txt"), 0o755); err != nil {
		t.Fatalf("chmod a.txt: %v", err)
	}
	neRunGit(t, dir, "add", "a.txt")
	neRunGit(t, dir, "commit", "--amend", "-m", "recreated spec branch: old tree + novel MODE-ONLY change to a.txt")
	neRunGit(t, dir, "checkout", "main")
	return dir, "spec-recreated-modeonly", "main"
}

// wdTypeChangeNovelWorkFixture is G1-N1's shape (spec 127 bead-1 fix
// round 2): the same destructive recreation, but the branch's own novel
// contribution changes an existing target-present path's TYPE — a
// regular file (kind.txt) becomes a symlink — rather than editing its
// content. `git diff --name-status` reports this as "T" (typechange), a
// status diffNameStatusBuckets did not classify at all before fix round
// 2 (it fell through a bare `default` to "none of the three buckets"
// with no record that a decision had been made — the BLOCKING finding).
// T now joins R/C's deliberate exclusion (see diffNameStatusBuckets' doc
// comment for why NOT the modified bucket: masking a type-changed path
// the same way M-status paths briefly were reopens ruling 2's
// over-match failure), so this remains a STATED, FIXTURED miss — see
// StatedLimit_TypeChangeOfNovelWorkIsMissed in the outcome table below —
// rather than a silent one.
func wdTypeChangeNovelWorkFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "kind.txt", "a regular file\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 landed work")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated-typechange", "main")
	neRunGit(t, dir, "rm", "landed.txt")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts to C1's tree")
	if err := os.Remove(filepath.Join(dir, "kind.txt")); err != nil {
		t.Fatalf("remove kind.txt: %v", err)
	}
	if err := os.Symlink("landed.txt", filepath.Join(dir, "kind.txt")); err != nil {
		t.Fatalf("symlink kind.txt: %v", err)
	}
	neRunGit(t, dir, "add", "kind.txt")
	neRunGit(t, dir, "commit", "--amend", "-m", "recreated spec branch: old tree + novel TYPE CHANGE of kind.txt")
	neRunGit(t, dir, "checkout", "main")

	// Fixture invariant: git must actually report kind.txt as a T-status
	// (typechange) path in the branch's own novel diff, not as an M
	// (content-only) or some other status — otherwise this fixture would
	// not exercise G1-N1's shape at all.
	statusOut := neRunGit(t, dir, "diff", "--name-status", "--find-renames", "main", "spec-recreated-typechange", "--", "kind.txt")
	if !strings.HasPrefix(strings.TrimSpace(statusOut), "T") {
		t.Fatalf("fixture invariant broken: expected a T-status (typechange) record for kind.txt, got %q", statusOut)
	}
	return dir, "spec-recreated-typechange", "main"
}

// wdMixedAddAndEditFixture is NEW-O1r-B's first previously-undocumented,
// previously-unfixtured member of the general miss rule (spec 127
// bead-1 fix round 3->4): the branch's own novel contribution MIXES a
// genuine addition (novel.txt, A-status, correctly stripped) WITH an
// in-place edit of a target-present path (a.txt, M-status, never
// stripped — see novelPaths' doc comment). All four pre-existing
// StatedLimit_* fixtures above pin only the NARROW reading — each one's
// branch has ZERO added paths (novelPaths returns added=[] on every one
// of them) — so none of them exercises the everyday shape where a
// branch's novel work both adds something new AND touches an existing
// file. Stripping novel.txt alone cannot rescue the match here: a.txt's
// edited content still blocks exact tree-equality against C1, for the
// identical reason a bare edit does in wdModifiedNovelWorkFixture. See
// this file's package doc comment and snapshotRevertMatch's for the
// general rule this is an instance of.
func wdMixedAddAndEditFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "a.txt", "original\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 landed work")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated-mixed", "main")
	neRunGit(t, dir, "rm", "landed.txt")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts to C1's tree")
	neWriteFile(t, dir, "novel.txt", "genuinely new work\n")
	neWriteFile(t, dir, "a.txt", "edited in place by novel work\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "--amend", "-m", "recreated spec branch: old tree + novel ADD (novel.txt) mixed with novel EDIT (a.txt)")
	neRunGit(t, dir, "checkout", "main")
	return dir, "spec-recreated-mixed", "main"
}

// wdNovelDeletionFixture is NEW-O1r-B's second previously-undocumented,
// previously-unfixtured member (spec 127 bead-1 fix round 3->4): the
// branch's own novel contribution is itself a DELETION of a path present
// since target's own root (stale-doc.md, D-status) — not merely the
// revert-induced deletion of landed.txt that makes this a recreation in
// the first place. novelPaths' strip bucket is A-status only; D-status,
// like R/C/M/T, is not in it (diffNameStatusBucketsFn's own third return
// is discarded entirely by novelPaths — see its doc comment), so the
// stripped tree is short one path (stale-doc.md) relative to EVERY
// candidate ancestor's tree and matches none — even though this is a
// genuine destructive recreation: the real merge would delete BOTH
// landed.txt and stale-doc.md, two target-present paths, and branch
// reconstructs no state target's history ever exactly held.
func wdNovelDeletionFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "a.txt", "original\n")
	neWriteFile(t, dir, "stale-doc.md", "an old doc\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1: a.txt + stale-doc.md")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 landed work")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated-noveldeletion", "main")
	neRunGit(t, dir, "rm", "landed.txt")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts to C1's tree")
	neRunGit(t, dir, "rm", "stale-doc.md")
	neRunGit(t, dir, "commit", "--amend", "-m", "recreated spec branch: old tree + novel DELETION of stale-doc.md")
	neRunGit(t, dir, "checkout", "main")

	// Fixture invariant: no commit in target's history may have a tree
	// exactly equal to branch's tip ({README.md, a.txt}) — otherwise this
	// fixture would accidentally land on a real ancestor match and stop
	// exercising the miss it is named for.
	branchTree := strings.TrimSpace(neRunGit(t, dir, "rev-parse", "spec-recreated-noveldeletion^{tree}"))
	for _, rev := range []string{"main", "main~1", "main~2"} {
		out, err := exec.Command("git", "-C", dir, "rev-parse", rev+"^{tree}").Output()
		if err != nil {
			continue // main~2 (the initial commit) has no further parent; ignore
		}
		if strings.TrimSpace(string(out)) == branchTree {
			t.Fatalf("fixture invariant broken: %s's tree equals branch's stripped tip tree — this fixture would be CAUGHT, not missed", rev)
		}
	}
	return dir, "spec-recreated-noveldeletion", "main"
}

// wdRestoredDeletedPathFixture is NEW-O1v-A's shape (spec 127 bead-1 fix
// round 6): the everyday form of the "strip asymmetry" mechanism the
// package doc comment's corrected rule now names, as opposed to the
// novel-work misses above. Target's cleanup commit (C3) deletes
// stale.txt AND bumps keep.txt in the SAME commit — a cleanup, a rename
// split across paths, or a delete-plus-edit all take this shape. Branch
// reverts to the tree BEFORE that cleanup — byte-identical to C1's tree,
// asserted below, not a near-miss or a partial revert — restoring
// stale.txt and keep.txt's old content. Relative to target's tip (C3),
// stale.txt reads as A-status (C3 does not have it) even though it is
// not novel content at all: it is a byte-for-byte restoration of a path
// target's own history held. snapshotRevertMatch's strip is
// unconditional over the A bucket, so it removes stale.txt from branch's
// side regardless of which of the two reasons put it there — and that
// single removal IS the whole miss (CORRECTED fix round 7, NEW-O1f-A;
// the prior version of this sentence claimed the stripped tree also
// disagreed with every candidate on keep.txt, which is false). Against
// C1 — the ancestor this branch recreates exactly — the stripped
// stale.txt entry is the ONLY residue: branch carries keep.txt=v1, which
// is exactly what C1 AND C2 carry, so keep.txt AGREES with both of them;
// it disagrees only with C3, and the root commit has no keep.txt at all.
// That is what makes this row a PURE instance of strip asymmetry — the
// match is destroyed by the strip alone, not by a disagreement on a
// modified path — and therefore a distinct class from the
// StatedLimit_MixedAddAndEditIsMissed row, where an M-status novel edit
// is what blocks against every candidate. C3's delete-AND-edit-in-ONE-
// commit shape is what makes the fixture the everyday cleanup form
// guard.DestructionClean's doc comment names; it is not what causes the
// miss (measured fix round 7: with C3 deleting stale.txt alone and
// keep.txt never bumped, the stripped tree still equals no ancestor's
// tree and the row still reads Clean, because branch's own deletion of
// landed.txt keeps it off every post-C2 candidate). No candidate
// matches, so this genuine, byte-identical recreation reads
// DestructionClean.
//
// FOR THE FILED CANDIDATE-RELATIVE-STRIP FOLLOW-UP: this row FLIPS when
// that lands, so updating it is part of that change and not a regression
// it caused. stale.txt is not novel RELATIVE TO C1, so a candidate-
// relative strip would not remove it; branch's tip tree IS C1's tree
// (asserted below), so the scan would match and the expected outcome
// here becomes DestructionStaleDeletion.
//
// See guard.DestructionClean's doc comment for the consumer-facing
// enumeration this class was added to.
func wdRestoredDeletedPathFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "keep.txt", "v1\n")
	neWriteFile(t, dir, "stale.txt", "stale content\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1: keep.txt + stale.txt")
	ancestorTree := strings.TrimSpace(neRunGit(t, dir, "rev-parse", "HEAD^{tree}"))
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 landed work")
	neRunGit(t, dir, "rm", "stale.txt")
	neWriteFile(t, dir, "keep.txt", "v2\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C3 cleanup: remove stale.txt AND bump keep.txt (one commit)")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated-restoreddeletion", "main")
	neRunGit(t, dir, "rm", "landed.txt")
	neWriteFile(t, dir, "stale.txt", "stale content\n")
	neWriteFile(t, dir, "keep.txt", "v1\n")
	neRunGit(t, dir, "add", "-A")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts to C1's tree (restores stale.txt, keep.txt)")
	neRunGit(t, dir, "checkout", "main")

	// Fixture invariant, the one NEW-O1v-A required: branch's tip tree
	// must be BYTE-IDENTICAL to the recreated ancestor's (C1's) tree —
	// this is not a near-miss or a partial revert, it is the modal,
	// everyday recreation shape, and the miss it pins is real precisely
	// because the match is exact before the strip runs.
	branchTree := strings.TrimSpace(neRunGit(t, dir, "rev-parse", "spec-recreated-restoreddeletion^{tree}"))
	if branchTree != ancestorTree {
		t.Fatalf("fixture invariant broken: branch tip tree %s != recreated ancestor's (C1's) tree %s — this fixture must recreate C1 exactly, or it does not exercise NEW-O1v-A's shape", branchTree, ancestorTree)
	}
	return dir, "spec-recreated-restoreddeletion", "main"
}

// wdRenameNovelWorkFixture is O2-4's STATED, still-uncaught miss: the
// branch's own novel contribution is a RENAME of a target-present path
// (never a content edit or an add). novelPaths deliberately excludes R/C
// from both buckets (see its doc comment), so this shape still reads
// DestructionClean — named and fixtured so the boundary is machine-visible
// rather than a silent, undocumented gap.
func wdRenameNovelWorkFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "src/a.txt", "original\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 landed work")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated-rename", "main")
	neRunGit(t, dir, "rm", "landed.txt")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts to C1's tree")
	neRunGit(t, dir, "mv", "src/a.txt", "src/b.txt")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: old tree + novel RENAME of a.txt->b.txt")
	neRunGit(t, dir, "checkout", "main")
	return dir, "spec-recreated-rename", "main"
}

// wdTabAndNonASCIIAndNewlineNovelPathFixture is S1-1/O1-2/G1-1's RED-on-
// the-prior-bug shape for the ADDED (novel) bucket: the same destructive
// recreation, but the branch's own novel work is added at THREE paths that
// each require special git handling under the pre-`-z` line-oriented
// parser — a tab-containing name, a non-ASCII (accented) name, and a
// literal-newline-containing name. Before this fix round, git's
// `--name-status` (no `-z`) C-quoted the non-ASCII and tab names (a tab
// ALSO breaks the '\t' field split itself) and split the newline-named
// record across two garbage lines, so novelPaths returned garbled or
// truncated tokens, stripNovelPaths silently no-opped on the real paths
// (git update-index --force-remove exits 0 on an unmatched path), and the
// tree-equality match failed — misclassifying a destructive recreation as
// DestructionClean.
func wdTabAndNonASCIIAndNewlineNovelPathFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "old.txt", "old1\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 landed work")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated-quoting-novel", "main")
	neRunGit(t, dir, "rm", "landed.txt")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts to C1's tree")
	neWriteFile(t, dir, "novel\twork.txt", "novel-tab\n")
	neWriteFile(t, dir, "résumé.txt", "novel-nonascii\n")
	neWriteFile(t, dir, "novel\nline.txt", "novel-newline\n")
	neRunGit(t, dir, "add", "-A")
	neRunGit(t, dir, "commit", "--amend", "-m", "recreated spec branch: old tree + novel tab/non-ascii/newline paths")
	neRunGit(t, dir, "checkout", "main")
	return dir, "spec-recreated-quoting-novel", "main"
}

// wdTabAndNonASCIIAndNewlineDeletedPathFixture is the same class on the
// DELETED-path/evidence side (O1-2's "cosmetic" DeletedPaths corruption,
// made behaviorally load-bearing here since it feeds novelPaths' sibling
// diffNameStatusBuckets call too): the paths the recreation reverts away
// (and that PreviewDeletedPaths reports in evidence.DeletedPaths) carry a
// tab, a non-ASCII name, and a literal newline.
func wdTabAndNonASCIIAndNewlineDeletedPathFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neWriteFile(t, dir, "old.txt", "old1\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "landed\ttab.txt", "landed-tab\n")
	neWriteFile(t, dir, "résumé-landed.txt", "landed-nonascii\n")
	neWriteFile(t, dir, "landed\nline.txt", "landed-newline\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 landed work (tab/non-ascii/newline names)")
	neRunGit(t, dir, "checkout", "-b", "spec-recreated-quoting-deleted", "main")
	neRunGit(t, dir, "rm", "landed\ttab.txt", "résumé-landed.txt", "landed\nline.txt")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts to C1's tree")
	neWriteFile(t, dir, "novel.txt", "novel\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "--amend", "-m", "recreated spec branch: old tree + novel work")
	neRunGit(t, dir, "checkout", "main")
	return dir, "spec-recreated-quoting-deleted", "main"
}

// wdNoMainRefFixture is a repo whose trunk is named `trunk`, not `main`.
// ancestryTargets checks for a local "main" ref before adding it as a
// second ancestry/supersession target; here it is absent, so evaluation
// narrows to target ("trunk") alone rather than hard-failing (spec 127
// bead-6 fix round 11, S2-2/F1-0 — this fixture previously backed a test
// asserting the opposite, that main's absence made EVERY evaluation fail
// closed with DestructionEvidenceError; see
// TestEvaluateWorkDestruction_NoMainRefNarrowsToTargetAlone below for why
// that was corrected).
func wdNoMainRefFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neRunGit(t, dir, "branch", "-m", "main", "trunk")
	neRunGit(t, dir, "checkout", "-b", "feat")
	neWriteFile(t, dir, "work.txt", "w\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "feat work")
	neRunGit(t, dir, "checkout", "trunk")
	return dir, "feat", "trunk"
}

// wdUnrelatedHistoriesFixture is O1-7/O2-7's other undocumented-but-
// fixtured disposition: branch and target share NO common ancestor at
// all (an orphan root), so merge-base exits 1 and every probe that relies
// on it — here, NetEffectLanded's internal merge-base call — fails
// closed.
func wdUnrelatedHistoriesFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir = initGitRepo(t)
	neRunGit(t, dir, "checkout", "--orphan", "island")
	neRunGit(t, dir, "rm", "-rf", "--cached", ".")
	neWriteFile(t, dir, "island.txt", "unrelated history\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "unrelated orphan root")
	neRunGit(t, dir, "checkout", "main")
	return dir, "island", "main"
}

// wdShallowHistoryFixture is O1-3's RED-on-the-prior-bug shape: builds the
// exact single-commit stale-deletion recreation (padded with extra
// commits so a shallow clone can genuinely truncate BEFORE reaching the
// reconstructable ancestor's commit), verifies the FULL-history origin
// classifies DestructionStaleDeletion (the positive control — otherwise
// this fixture would prove nothing about truncation specifically), then
// returns a real `git clone --depth 1` of it. Before this fix round,
// findAncestorWithTree's `git rev-list target` silently stopped at the
// graft boundary, so "no match found in the (truncated) history I could
// see" read as the definite answer "the deletions are authored" —
// DestructionClean, evidence.DeletedPaths fully populated, no error.
func wdShallowHistoryFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	origin := initGitRepo(t)
	for i := 0; i < 3; i++ {
		neWriteFile(t, origin, fmt.Sprintf("pad%d.txt", i), "pad\n")
		neRunGit(t, origin, "add", ".")
		neRunGit(t, origin, "commit", "-m", fmt.Sprintf("padding commit %d", i))
	}
	neWriteFile(t, origin, "old.txt", "old\n")
	neRunGit(t, origin, "add", ".")
	neRunGit(t, origin, "commit", "-m", "adds old.txt (the reconstructable ancestor)")
	oldSHA := strings.TrimSpace(neRunGit(t, origin, "rev-parse", "HEAD"))
	// Padding BETWEEN old.txt and the landed-work tip: a depth-2 shallow
	// clone truncates from EACH ref's own tip independently, so main's
	// tip needs to sit far enough from old.txt's commit that depth 2
	// (just enough for merge-base(spec-recreated, main) to still resolve
	// spec-recreated's parent — main's own tip — within its OWN depth-2
	// slice) does not incidentally still reach it.
	for i := 0; i < 3; i++ {
		neWriteFile(t, origin, fmt.Sprintf("mid%d.txt", i), "mid\n")
		neRunGit(t, origin, "add", ".")
		neRunGit(t, origin, "commit", "-m", fmt.Sprintf("mid commit %d", i))
	}
	neWriteFile(t, origin, "landed.txt", "landed\n")
	neRunGit(t, origin, "add", ".")
	neRunGit(t, origin, "commit", "-m", "landed work")
	neRunGit(t, origin, "checkout", "-b", "spec-recreated", "main")
	neRunGit(t, origin, "rm", "landed.txt")
	neRunGit(t, origin, "commit", "-m", "recreated: reverts to old.txt-adding commit's tree")
	neWriteFile(t, origin, "novel.txt", "novel\n")
	neRunGit(t, origin, "add", ".")
	neRunGit(t, origin, "commit", "--amend", "-m", "recreated: old tree + novel work")
	neRunGit(t, origin, "checkout", "main")

	outcome, _, err := EvaluateWorkDestruction(origin, "spec-recreated", "main")
	if err != nil {
		t.Fatalf("fixture invariant: unexpected error evaluating the FULL-history origin: %v", err)
	}
	if outcome != guard.DestructionStaleDeletion {
		t.Fatalf("fixture invariant: full-history origin must classify DestructionStaleDeletion (the positive control), got %s", outcome)
	}

	shallow := t.TempDir()
	cloneCmd := exec.Command("git", "clone", "--depth", "2", "--no-single-branch", "--branch", "spec-recreated", "file://"+origin, shallow)
	cloneCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone --depth 2: %s: %v", out, err)
	}
	neRunGit(t, shallow, "branch", "main", "origin/main")

	shallowNow, serr := isShallowRepo(shallow)
	if serr != nil {
		t.Fatalf("fixture invariant: isShallowRepo: %v", serr)
	}
	if !shallowNow {
		t.Fatalf("fixture invariant: the clone must actually be shallow")
	}
	// The load-bearing invariant: main's OWN visible history in the
	// shallow clone must NOT reach old.txt's commit — otherwise this
	// fixture would not actually exercise truncation.
	revList := neRunGit(t, shallow, "rev-list", "main")
	if strings.Contains(revList, oldSHA) {
		t.Fatalf("fixture invariant broken: old.txt's commit (%s) is still reachable in the shallow clone's rev-list main:\n%s", oldSHA, revList)
	}

	return shallow, "spec-recreated", "main"
}

// wdReplaceRefTruncatedHistoryFixture is O1-3's OTHER RED-on-the-prior-
// bug shape (spec 127 bead-1 fix round 2): a replace ref truncates
// rev-list's view of target's history exactly like a shallow clone does
// — but --is-shallow-repository never reports true for it, so the
// round-1 fix (isShallowRepo alone) still failed open on this mechanism,
// in the SAME direction (Clean, no error, D-set fully enumerated) via
// the OTHER truncation vector. `git replace --graft <tip>` with NO
// parent arguments makes git report the tip as having no parents at all,
// hiding the reconstructable ancestor (old.txt's commit) from rev-list.
func wdReplaceRefTruncatedHistoryFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir, branch, target = wdStaleDeletionSingleCommitFixture(t)

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("fixture invariant: unexpected error evaluating the pre-replace repo: %v", err)
	}
	if outcome != guard.DestructionStaleDeletion {
		t.Fatalf("fixture invariant: pre-replace repo must classify DestructionStaleDeletion (the positive control), got %s", outcome)
	}

	before := strings.Fields(neRunGit(t, dir, "rev-list", target))
	tip := strings.TrimSpace(neRunGit(t, dir, "rev-parse", target))
	neRunGit(t, dir, "replace", "--graft", tip)

	shallowNow, serr := isShallowRepo(dir)
	if serr != nil {
		t.Fatalf("fixture invariant: isShallowRepo: %v", serr)
	}
	if shallowNow {
		t.Fatalf("fixture invariant broken: a replace ref must NOT make is-shallow-repository report true — otherwise this fixture does not exercise the DISTINCT mechanism O1-3's still-open finding named")
	}
	replaceRefs := neRunGit(t, dir, "for-each-ref", "refs/replace/")
	if strings.TrimSpace(replaceRefs) == "" {
		t.Fatalf("fixture invariant broken: the repo must actually have a refs/replace/* ref")
	}
	truncatedNow, terr := historyTruncated(dir, target)
	if terr != nil {
		t.Fatalf("fixture invariant: historyTruncated: %v", terr)
	}
	if !truncatedNow {
		t.Fatalf("fixture invariant broken: historyTruncated must detect the truncation this replace ref just introduced (spec 127 bead-1 fix round 3->4: the differential replacing hasReplaceRefs)")
	}
	afterCmd := exec.Command("git", "-C", dir, "rev-list", target)
	afterOut, aerr := afterCmd.Output()
	if aerr != nil {
		t.Fatalf("rev-list %s: %v", target, aerr)
	}
	after := strings.Fields(string(afterOut))
	if len(after) >= len(before) {
		t.Fatalf("fixture invariant broken: the replace ref must actually TRUNCATE rev-list's view of %s (before=%d after=%d) — otherwise this fixture does not exercise truncation", target, len(before), len(after))
	}
	return dir, branch, target
}

// wdGraftsTruncatedHistoryFixture is O1-3's THIRD truncation mechanism
// (spec 127 bead-1 fix round 2): a legacy `.git/info/grafts` file
// truncates rev-list identically to a replace ref, via a wholly separate
// (deprecated but still-supported) git mechanism — detected, alongside a
// replace ref, by the single differential measurement historyTruncated
// performs (spec 127 bead-1 fix round 3->4: the differential replacing
// hasGraftsFile; corrected again fix round 5, NEW-G1sub-6/O1g-A/O2G-1/
// O3g-1, to a full (commit,tree) comparison, made order-insensitive — a
// MULTISET — fix round 6, NEW-G1sub-11 — see historyTruncated's doc
// comment), independently of isShallowRepo. A
// prior version of this comment (spec 127 bead-1 fix round 2) described
// this fixture against the two now-deleted presence probes
// (--is-shallow-repository and refs/replace/*) that predated the
// differential — corrected here, the tenth instance of this bead's
// prose-surviving-deleted-code class (NEW-G1sub-8).
func wdGraftsTruncatedHistoryFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir, branch, target = wdStaleDeletionSingleCommitFixture(t)

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("fixture invariant: unexpected error evaluating the pre-grafts repo: %v", err)
	}
	if outcome != guard.DestructionStaleDeletion {
		t.Fatalf("fixture invariant: pre-grafts repo must classify DestructionStaleDeletion (the positive control), got %s", outcome)
	}

	before := strings.Fields(neRunGit(t, dir, "rev-list", target))
	tip := strings.TrimSpace(neRunGit(t, dir, "rev-parse", target))
	neRunGit(t, dir, "config", "advice.graftFileDeprecated", "false")
	graftsPath := strings.TrimSpace(neRunGit(t, dir, "rev-parse", "--git-path", "info/grafts"))
	if !filepath.IsAbs(graftsPath) {
		graftsPath = filepath.Join(dir, graftsPath)
	}
	if err := os.MkdirAll(filepath.Dir(graftsPath), 0o755); err != nil {
		t.Fatalf("mkdir info/: %v", err)
	}
	if err := os.WriteFile(graftsPath, []byte(tip+"\n"), 0o644); err != nil {
		t.Fatalf("writing info/grafts: %v", err)
	}

	shallowNow, serr := isShallowRepo(dir)
	if serr != nil {
		t.Fatalf("fixture invariant: isShallowRepo: %v", serr)
	}
	if shallowNow {
		t.Fatalf("fixture invariant broken: a grafts file must NOT make is-shallow-repository report true")
	}
	replaceRefs := neRunGit(t, dir, "for-each-ref", "refs/replace/")
	if strings.TrimSpace(replaceRefs) != "" {
		t.Fatalf("fixture invariant broken: this fixture must exercise the GRAFTS mechanism, not a replace ref")
	}
	if _, gerr := os.Stat(graftsPath); gerr != nil {
		t.Fatalf("fixture invariant broken: info/grafts must exist at %s: %v", graftsPath, gerr)
	}
	truncatedNow, terr := historyTruncated(dir, target)
	if terr != nil {
		t.Fatalf("fixture invariant: historyTruncated: %v", terr)
	}
	if !truncatedNow {
		t.Fatalf("fixture invariant broken: historyTruncated must detect the truncation this info/grafts file just introduced (spec 127 bead-1 fix round 3->4: the differential replacing hasGraftsFile)")
	}
	afterCmd := exec.Command("git", "-C", dir, "rev-list", target)
	afterOut, aerr := afterCmd.Output()
	if aerr != nil {
		t.Fatalf("rev-list %s: %v", target, aerr)
	}
	after := strings.Fields(string(afterOut))
	if len(after) >= len(before) {
		t.Fatalf("fixture invariant broken: the grafts file must actually TRUNCATE rev-list's view of %s (before=%d after=%d)", target, len(before), len(after))
	}
	return dir, branch, target
}

// --- the outcome table ------------------------------------------------------

// wdForcedEvidenceErrorFixture is the outcome table's own
// DestructionEvidenceError row (spec 127 bead-1 fix round, O2-5(b)): the
// prior version SEEDED the sentinel's `seen` map with
// {DestructionEvidenceError: true} by fiat — a literal, not a fixture —
// so deleting every real evidence-error test elsewhere would still leave
// this sentinel green. Forcing a seam and registering its restoration via
// t.Cleanup from INSIDE the fixture builder makes this row behave like
// every other: the outcome table loop calls EvaluateWorkDestruction
// exactly once, same as any other row, and gets a real
// DestructionEvidenceError back from an actually-forced probe failure.
func wdForcedEvidenceErrorFixture(t *testing.T) (dir, branch, target string) {
	t.Helper()
	dir, branch, target = wdHonestStaleBranchFixture(t)
	orig := workDestructionIsAncestorFn
	t.Cleanup(func() { workDestructionIsAncestorFn = orig })
	workDestructionIsAncestorFn = func(workdir, ancestor, descendant string) (bool, error) {
		return false, errors.New("forced evidence-error fixture (spec 127 O2-5(b) sentinel-coverage row)")
	}
	return dir, branch, target
}

func TestEvaluateWorkDestruction_OutcomeTable(t *testing.T) {
	type row struct {
		name    string
		build   func(t *testing.T) (dir, branch, target string)
		want    guard.DestructionOutcome
		checkFn func(t *testing.T, evidence WorkDestructionEvidence)
	}
	table := []row{
		{"AncestorOfTarget", wdAncestorOfTargetFixture, guard.DestructionAncestor, func(t *testing.T, e WorkDestructionEvidence) {
			if e.AncestorOf != "spec-target" {
				t.Errorf("evidence.AncestorOf = %q, want spec-target", e.AncestorOf)
			}
		}},
		{"AncestorOfMainNotTarget", wdAncestorOfMainFixture, guard.DestructionAncestor, func(t *testing.T, e WorkDestructionEvidence) {
			if e.AncestorOf != "main" {
				t.Errorf("evidence.AncestorOf = %q, want main", e.AncestorOf)
			}
		}},
		{"SupersededTheHashtag218Shape", wdSupersededFixture, guard.DestructionSuperseded, func(t *testing.T, e WorkDestructionEvidence) {
			if e.SupersededVia != "spec-target" {
				t.Errorf("evidence.SupersededVia = %q, want spec-target", e.SupersededVia)
			}
		}},
		{"SupersededViaMain", wdSupersededViaMainFixture, guard.DestructionSuperseded, func(t *testing.T, e WorkDestructionEvidence) {
			if e.SupersededVia != "main" {
				t.Errorf("evidence.SupersededVia = %q, want main", e.SupersededVia)
			}
		}},
		{"StaleDeletionWitnessSingleCommit", wdStaleDeletionSingleCommitFixture, guard.DestructionStaleDeletion, func(t *testing.T, e WorkDestructionEvidence) {
			if len(e.DeletedPaths) == 0 {
				t.Error("evidence.DeletedPaths must be populated for a stale-deletion outcome")
			}
			if e.ReconstructedAncestor == "" {
				t.Error("evidence.ReconstructedAncestor must be populated for a stale-deletion outcome")
			}
		}},
		{"StaleDeletionWitnessMultiCommit", wdStaleDeletionMultiCommitFixture, guard.DestructionStaleDeletion, func(t *testing.T, e WorkDestructionEvidence) {
			if len(e.DeletedPaths) == 0 {
				t.Error("evidence.DeletedPaths must be populated for a stale-deletion outcome")
			}
			if e.ReconstructedAncestor == "" {
				t.Error("evidence.ReconstructedAncestor must be populated for a stale-deletion outcome")
			}
		}},
		{"LargeRenameAC8ii", wdLargeRenameFixture, guard.DestructionClean, nil},
		{"HonestCleanupAC8iii", wdHonestCleanupFixture, guard.DestructionClean, nil},
		{"HonestStaleBranch", wdHonestStaleBranchFixture, guard.DestructionClean, nil},
		{"ModifyDeleteConflictIsClean", wdModifyDeleteConflictFixture, guard.DestructionClean, nil},
		{"ConservativeCorner", wdConservativeCornerFixture, guard.DestructionStaleDeletion, func(t *testing.T, e WorkDestructionEvidence) {
			if e.ReconstructedAncestor == "" {
				t.Error("evidence.ReconstructedAncestor must be populated for the conservative-corner stale-deletion outcome")
			}
		}},

		// --- spec 127 bead-1 fix round: RED-on-the-prior-bug rows -----------

		{"ConflictMaskedRecreationNowCaught", wdConflictMaskedRecreationFixture, guard.DestructionStaleDeletion, func(t *testing.T, e WorkDestructionEvidence) {
			if len(e.DeletedPaths) == 0 {
				t.Error("evidence.DeletedPaths must be populated (O1-1/O2-1/O3-2: a co-occurring conflict must not mask the deletion)")
			}
			if e.ReconstructedAncestor == "" {
				t.Error("evidence.ReconstructedAncestor must be populated")
			}
		}},
		{"TabNonASCIINewlineNovelPathNowCaught", wdTabAndNonASCIIAndNewlineNovelPathFixture, guard.DestructionStaleDeletion, func(t *testing.T, e WorkDestructionEvidence) {
			if e.ReconstructedAncestor == "" {
				t.Error("evidence.ReconstructedAncestor must be populated (S1-1/O1-2/G1-1: tab/non-ASCII/newline novel paths must not survive C-quoting corruption)")
			}
		}},
		{"TabNonASCIINewlineDeletedPathNowCaught", wdTabAndNonASCIIAndNewlineDeletedPathFixture, guard.DestructionStaleDeletion, func(t *testing.T, e WorkDestructionEvidence) {
			if len(e.DeletedPaths) != 3 {
				t.Errorf("evidence.DeletedPaths = %v, want 3 raw (unescaped) tab/non-ASCII/newline paths", e.DeletedPaths)
			}
			if e.ReconstructedAncestor == "" {
				t.Error("evidence.ReconstructedAncestor must be populated")
			}
		}},

		// --- stated, fixtured misses (named so the boundary is machine-
		// visible, not a silent gap) -----------------------------------------

		{"StatedLimit_RenameOfNovelWorkIsMissed", wdRenameNovelWorkFixture, guard.DestructionClean, nil},
		{"StatedLimit_ModifiedNovelWorkIsMissed", wdModifiedNovelWorkFixture, guard.DestructionClean, nil},
		{"StatedLimit_ModeOnlyNovelWorkIsMissed", wdModeOnlyNovelWorkFixture, guard.DestructionClean, nil},
		{"StatedLimit_TypeChangeOfNovelWorkIsMissed", wdTypeChangeNovelWorkFixture, guard.DestructionClean, nil},
		{"StatedLimit_MixedAddAndEditIsMissed", wdMixedAddAndEditFixture, guard.DestructionClean, nil},
		{"StatedLimit_NovelDeletionIsMissed", wdNovelDeletionFixture, guard.DestructionClean, nil},
		{"StatedLimit_RestoredDeletedPathIsMissed", wdRestoredDeletedPathFixture, guard.DestructionClean, func(t *testing.T, e WorkDestructionEvidence) {
			if len(e.DeletedPaths) == 0 {
				t.Error("evidence.DeletedPaths must be populated — the merge preview genuinely deletes landed.txt, so this is a real miss on a genuine deletion, not a vacuous row (NEW-O1v-A)")
			}
		}},
		{"StatedLimit_ConflictOnRevertedPathScreensNothing", wdConflictOnRevertedPathFixture, guard.DestructionClean, func(t *testing.T, e WorkDestructionEvidence) {
			if len(e.DeletedPaths) != 0 {
				t.Errorf("evidence.DeletedPaths = %v, want empty (O3-2: a modify/delete conflict on the reverted path keeps the modified side, so the preview deletes nothing)", e.DeletedPaths)
			}
		}},

		// --- O1-6: the reconstructed ancestor may be reachable only through
		// a merged side branch, never target's first-parent lineage ---------

		{"ReconstructedAncestorViaMergedSideBranch", wdSideBranchReachableAncestorFixture, guard.DestructionStaleDeletion, func(t *testing.T, e WorkDestructionEvidence) {
			if e.ReconstructedAncestor == "" {
				t.Error("evidence.ReconstructedAncestor must be populated")
			}
		}},

		// --- the sentinel-coverage row (O2-5(b)) ----------------------------

		{"ForcedEvidenceError", wdForcedEvidenceErrorFixture, guard.DestructionEvidenceError, func(t *testing.T, e WorkDestructionEvidence) {
			if e.FailedProbe == "" {
				t.Error("evidence.FailedProbe must be populated")
			}
		}},
	}

	// Every outcome, INCLUDING DestructionEvidenceError, must be
	// represented by an actual row's `want` above — no literal seed (spec
	// 127 bead-1 fix round, O2-5(b)). This is the B-r4-3 sentinel
	// discipline every later consumer's table copies:
	// len(seen) == guard.DestructionOutcomeCount.
	seen := map[guard.DestructionOutcome]bool{}
	for _, r := range table {
		seen[r.want] = true
	}
	if len(seen) != int(guard.DestructionOutcomeCount) {
		t.Fatalf("outcome table covers %d distinct outcomes, want all %d (guard.DestructionOutcomeCount)", len(seen), guard.DestructionOutcomeCount)
	}

	for _, r := range table {
		t.Run(r.name, func(t *testing.T) {
			dir, branch, target := r.build(t)
			outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
			if r.want == guard.DestructionEvidenceError {
				if err == nil {
					t.Fatal("expected a non-nil error for the forced evidence-error row")
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if outcome != r.want {
				t.Errorf("EvaluateWorkDestruction(%s, %s) = %s, want %s", branch, target, outcome, r.want)
			}
			if r.checkFn != nil {
				r.checkFn(t, evidence)
			}
		})
	}
}

// TestEvaluateWorkDestruction_DeletedLineFloorWouldMisclassifyAC8iii is the
// named deviation-target pin (plan step 4: "these two make a bare
// deleted-line floor unimplementable — red against a numeric-floor impl"):
// a naive floor of the shape "any preview-deletion at all refuses" WOULD
// flag the honest AC-8(iii) cleanup (its preview genuinely deletes
// obsolete.txt) even though the deletion is the branch's own authored
// work. This test asserts BOTH halves directly: the raw D-set is
// non-empty (so a bare floor really would trip), and the predicate's own
// outcome is Clean anyway (so this implementation does not regress to
// that floor).
func TestEvaluateWorkDestruction_DeletedLineFloorWouldMisclassifyAC8iii(t *testing.T) {
	dir, branch, target := wdHonestCleanupFixture(t)

	deleted, err := PreviewDeletedPaths(dir, target, branch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) == 0 {
		t.Fatal("fixture invariant broken: the honest cleanup must have a non-empty raw D-set (obsolete.txt) — otherwise this deviation-target comparison is vacuous")
	}

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != guard.DestructionClean {
		t.Errorf("a bare deleted-line floor would refuse here (D-set=%v non-empty); the predicate must still classify Clean, got %s", deleted, outcome)
	}
}

// TestEvaluateWorkDestruction_DeletedLineFloorWouldMisclassifyAC8ii is
// AC-8(ii)'s own half of the same deviation-target pin: raw preview
// deletions must be EMPTY for a large rename (proving --find-renames is
// doing its job one layer down), and the outcome is Clean.
func TestEvaluateWorkDestruction_DeletedLineFloorWouldMisclassifyAC8ii(t *testing.T) {
	dir, branch, target := wdLargeRenameFixture(t)

	deleted, err := PreviewDeletedPaths(dir, target, branch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("fixture invariant broken: a large rename must have an EMPTY raw D-set, got %v", deleted)
	}

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != guard.DestructionClean {
		t.Errorf("EvaluateWorkDestruction = %s, want Clean", outcome)
	}
}

// TestEvaluateWorkDestruction_StaleDeletionRedOnRevertedDiscriminator
// mechanically demonstrates the PRIOR (empty-by-construction) discriminator
// against the two destructive fixtures: the OLD set-subtraction form —
// D-paths(preview vs target) MINUS D-paths(merge-base(branch,target)..branch)
// — is provably empty on both, meaning it certifies the destruction as
// CLEAN. This is the red-on-revert witness for the corrected snapshot-
// revert discriminator: reverting to the old mechanic must make these two
// fixtures wrongly read as clean.
func TestEvaluateWorkDestruction_StaleDeletionRedOnRevertedDiscriminator(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(t *testing.T) (dir, branch, target string)
	}{
		{"SingleCommitRecreation", wdStaleDeletionSingleCommitFixture},
		{"MultiCommitRecreation", wdStaleDeletionMultiCommitFixture},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, branch, target := tc.build(t)

			deleted, err := PreviewDeletedPaths(dir, target, branch)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(deleted) == 0 {
				t.Fatal("fixture invariant broken: the recreation shape must have a non-empty raw D-set")
			}

			// The OLD discriminator's subtraction set: D-paths in the
			// range merge-base(branch,target)..branch. On a branch
			// recreated FROM target's own tip, merge-base(branch,target)
			// computes to target's tip itself (the forging effect named
			// in this file's package doc comment) — so every deleted path
			// in `deleted` is trivially also a deletion inside that range,
			// making the subtraction (and therefore the old discriminator)
			// EMPTY. Reproduced directly here (not merely asserted) via
			// the same primitives this package already exposes.
			base, err := mergeBaseFn(dir, branch, target)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			targetTip, err := RevParseRef(dir, target)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if base != targetTip {
				t.Fatalf("fixture invariant broken: merge-base(branch,target) must equal target's own tip on a recreated-from-tip branch (the forging effect), got base=%s target=%s", base, targetTip)
			}
			_, rangeDeleted, _, err := diffNameStatusBucketsFn(dir, base, branch)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			oldDiscriminatorResult := subtractPaths(deleted, rangeDeleted)
			if len(oldDiscriminatorResult) != 0 {
				t.Fatalf("test invariant broken: the OLD discriminator was expected to be EMPTY (and thus CLEAN) on this shape, got %v", oldDiscriminatorResult)
			}

			// The CORRECTED discriminator (this package's actual
			// EvaluateWorkDestruction) must still fire on the exact same
			// fixture the old one went blind on.
			outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if outcome != guard.DestructionStaleDeletion {
				t.Errorf("EvaluateWorkDestruction = %s, want DestructionStaleDeletion (red-on-revert to the old discriminator)", outcome)
			}
		})
	}
}

func subtractPaths(a, b []string) []string {
	inB := make(map[string]bool, len(b))
	for _, p := range b {
		inB[p] = true
	}
	var out []string
	for _, p := range a {
		if !inB[p] {
			out = append(out, p)
		}
	}
	return out
}

// --- spec 127 bead-1 fix round 2 (ruling 3): permanent red-on-revert
// pins for the original BLOCKING classes, using the same house convention
// as TestEvaluateWorkDestruction_StaleDeletionRedOnRevertedDiscriminator
// above (mechanically reconstruct the PRIOR algorithm inline via this
// package's own primitives, apply it to the fixture that now proves the
// fix, and show it gets the wrong answer) rather than an ephemeral
// `go test -overlay` probe run once and discarded. -----------------------

// TestEvaluateWorkDestruction_OrderingMaskRedOnRevertedShortCircuit is
// the permanent pin for the FIRST original BLOCKING class — the deleted
// targetConflict short-circuit (O1-1/O2-1/O3-2) — REBUILT this round
// because the round-1 regression fixture (O1c-A/NEW-O2-a) never actually
// exercised it: see wdConflictMaskedRecreationFixture's doc comment.
// Reconstructs the exact deleted mechanic (fold a genuine target-side
// conflict, via the 4-arg ContentSubsumedOutcome, into an unconditional
// DestructionClean BEFORE ever previewing deletions) and confirms it
// answers Clean on the rebuilt fixture, while the real
// EvaluateWorkDestruction answers StaleDeletion on the identical
// fixture. Independently confirmed via `go test -overlay` against a full
// reconstruction of the pre-fix workdestruction.go: the reinjected
// short-circuit failed ONLY the OutcomeTable's
// ConflictMaskedRecreationNowCaught row, with the rest of
// internal/gitutil green (27.6s) — the same standard S1 met for the
// range-based discriminator above.
func TestEvaluateWorkDestruction_OrderingMaskRedOnRevertedShortCircuit(t *testing.T) {
	dir, branch, target := wdConflictMaskedRecreationFixture(t)

	base, err := mergeBaseFn(dir, branch, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	subsumed, err := ContentSubsumedOutcome(dir, base, branch, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subsumed != SubsumptionConflict {
		t.Fatalf("fixture invariant broken: ContentSubsumedOutcome = %v, want SubsumptionConflict — this test's whole point is a genuinely conflicting target-side merge", subsumed)
	}
	// This IS the reverted algorithm's answer: the moment step 2 sees a
	// target-side conflict, it folds to DestructionClean unconditionally,
	// discarding the D-set/snapshot-revert legs entirely — never even
	// reaching PreviewDeletedPaths.
	oldAnswer := guard.DestructionClean

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != guard.DestructionStaleDeletion {
		t.Errorf("EvaluateWorkDestruction = %s, want DestructionStaleDeletion (red-on-revert to the deleted targetConflict short-circuit)", outcome)
	}
	if outcome == oldAnswer {
		t.Fatalf("test invariant broken: the corrected and reverted answers must differ on this fixture; both landed on %s", outcome)
	}
}

// oldDiffNameStatusLineOriented reconstructs the PRE-`-z` line-oriented
// parser (spec 127 bead-1 fix round, S1-1/O1-2/G1-1): git's own
// `--name-status --find-renames` WITHOUT `-z`, split on '\n' then on the
// first '\t'. core.quotepath defaults to true, so any path byte git
// needs to escape (every non-ASCII byte, and unconditionally any control
// character such as a literal tab or newline) comes back as a
// double-quoted, backslash/octal-escaped LITERAL TEXT token — never the
// real bytes — for any path this bug can corrupt.
func oldDiffNameStatusLineOriented(t *testing.T, workdir, from, to string) (added, deleted, modified []string) {
	t.Helper()
	out := neRunGit(t, workdir, "diff", "--name-status", "--find-renames", from, to)
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 2)
		if len(fields) != 2 {
			continue
		}
		status, path := fields[0], fields[1]
		switch status[0] {
		case 'A':
			added = append(added, path)
		case 'D':
			deleted = append(deleted, path)
		case 'M':
			modified = append(modified, path)
		}
	}
	return added, deleted, modified
}

// TestEvaluateWorkDestruction_PathQuotingRedOnRevertedLineOrientedParsing
// is the permanent pin for the SECOND original BLOCKING class — the
// pre-`-z` line-oriented parser (S1-1/O1-2/G1-1) — applied to the same
// tab/non-ASCII/newline novel-path fixture the real (`-z`) parser now
// classifies correctly. The old parser's output is the ESCAPED, not the
// raw-byte, representation, and stripNovelPaths silently no-ops on that
// escaped spelling (`git update-index --force-remove` exits 0 on an
// unmatched path), leaving the real novel paths IN the "stripped" tree —
// which blocks the tree-equality match and misclassifies the destructive
// recreation as DestructionClean.
func TestEvaluateWorkDestruction_PathQuotingRedOnRevertedLineOrientedParsing(t *testing.T) {
	dir, branch, target := wdTabAndNonASCIIAndNewlineNovelPathFixture(t)

	oldAdded, _, _ := oldDiffNameStatusLineOriented(t, dir, target, branch)
	if len(oldAdded) == 0 {
		t.Fatal("fixture invariant broken: the old parser must return at least one (garbled) added path")
	}
	realAdded, _, err := novelPaths(dir, target, branch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(realAdded) == 0 {
		t.Fatal("fixture invariant broken: the real (-z) parser must return the novel added paths")
	}
	oldSet := make(map[string]bool, len(oldAdded))
	for _, p := range oldAdded {
		oldSet[p] = true
	}
	for _, p := range realAdded {
		if oldSet[p] {
			t.Fatalf("fixture invariant broken: the old parser's output byte-matched a real raw path (%q) — this fixture no longer exercises the quoting bug (old=%v real=%v)", p, oldAdded, realAdded)
		}
	}

	// Reconstruct the old, broken pipeline: strip using the GARBLED
	// (old-parser) path list rather than the real, raw-byte one.
	oldStripped, err := stripNovelPaths(dir, branch, oldAdded)
	if err != nil {
		t.Fatalf("unexpected error building the old pipeline's stripped tree: %v", err)
	}
	oldSHA, err := findAncestorWithTree(dir, target, oldStripped, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if oldSHA != "" {
		t.Fatalf("test invariant broken: the old (garbled) strip was expected to MISS the ancestor match (that is the bug this test pins) — it matched %s", oldSHA)
	}

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != guard.DestructionStaleDeletion {
		t.Errorf("EvaluateWorkDestruction = %s, want DestructionStaleDeletion (red-on-revert to the pre-`-z` line-oriented parser)", outcome)
	}
}

// TestEvaluateWorkDestruction_AncestorMaskOvermatchRedOnRestoredMStatusMask
// is the mirror-image pin for the "A-status scope" class ruling 3 named
// (spec 127 bead-1 fix round 2): round 1 widened novelPaths' strip/mask
// from A-status-only to A-status-plus-M-status (O1-4/O2-4), which this
// round ROLLED BACK (ruling 2, O1c-B/NEW-O2-b) because masking M-status
// paths out of every candidate ancestor made the comparison strictly
// weaker than tree equality and misclassified HONEST branches as
// destructive. There is no "reinject the miss and see a catch go red"
// version of this class any more — the miss (M-status novel work) is
// once again the STATED limit the outcome table's own
// StatedLimit_ModifiedNovelWorkIsMissed/StatedLimit_ModeOnlyNovelWorkIsMissed
// rows pin. This test instead reconstructs round 1's A+M mask directly
// and shows it MISCLASSIFIES an honest branch (cut from target's own
// current tip, deleting a recently-added file, editing one unrelated
// tracked path — the shape NEW-O2-b found) as DestructionStaleDeletion,
// while the real (rolled-back) EvaluateWorkDestruction correctly answers
// DestructionClean on the identical fixture — the reverse direction of
// every other pin in this file, and exactly why ruling 2 rejected
// keeping the wider mask.
func TestEvaluateWorkDestruction_AncestorMaskOvermatchRedOnRestoredMStatusMask(t *testing.T) {
	dir := initGitRepo(t)
	neWriteFile(t, dir, "a.txt", "v1\n")
	neWriteFile(t, dir, "keep.txt", "keep\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C1")
	neWriteFile(t, dir, "obsolete.txt", "obsolete\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "C2 adds obsolete.txt")
	branch := "honest-cleanup"
	neRunGit(t, dir, "checkout", "-b", branch)
	neRunGit(t, dir, "rm", "obsolete.txt")
	neWriteFile(t, dir, "a.txt", "v2 (an ordinary, unrelated edit)\n")
	neRunGit(t, dir, "add", ".")
	neRunGit(t, dir, "commit", "-m", "honest cleanup: remove obsolete.txt, edit a.txt")
	neRunGit(t, dir, "checkout", "main")
	target := "main"

	// Fixture invariant: this branch is cut from target's CURRENT tip
	// (zero staleness) and its deletion is its own authored work, not a
	// recreation — the real predicate must answer Clean.
	realOutcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if realOutcome != guard.DestructionClean {
		t.Fatalf("EvaluateWorkDestruction = %s, want DestructionClean — this is an honest branch, not a recreation. Restoring the M-status strip/mask in snapshotRevertMatch (round 1's A+M widening) is FORBIDDEN by spec 127 bead-1 fix round 2 ruling 2 and bead 6's acceptance criteria: it misclassifies honest branches like this one as destructive (O1c-B/NEW-O2-b). Do not adjust this fixture or this expectation to make this test pass — fix the regression in snapshotRevertMatch instead", realOutcome)
	}

	// Reconstruct round 1's A+M mask directly: strip BOTH added and
	// modified novel paths from branch's tip, and mask the SAME modified
	// paths out of every candidate ancestor before comparing.
	added, modified, err := novelPaths(dir, target, branch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(modified) == 0 {
		t.Fatal("fixture invariant broken: the honest branch must have a non-empty M-status novel path (a.txt) for this test to exercise the over-match")
	}
	strip := append(append([]string{}, added...), modified...)
	restoredStrippedTree, err := stripNovelPaths(dir, branch, strip)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	restoredSHA, err := findAncestorWithTree(dir, target, restoredStrippedTree, modified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if restoredSHA == "" {
		t.Fatal("test invariant broken: round 1's A+M mask was expected to OVER-MATCH (wrongly find an ancestor) on this honest-branch shape — that is the bug this test pins")
	}
	// restoredSHA != "" here means round 1's A+M mask WOULD have matched an
	// ancestor — i.e. EvaluateWorkDestruction would have answered
	// DestructionStaleDeletion, not the DestructionClean asserted above —
	// on this honest branch, had that mask still been wired in. That
	// non-empty match, together with the real predicate's Clean answer
	// already asserted above, IS the discriminating pin (spec 127 bead-1
	// fix round 3->4, NEW-G1sub-3: a prior version of this test also
	// compared realOutcome against a hardcoded guard.DestructionStaleDeletion
	// literal here, which could only ever fail if the Fatalf above had
	// already fired — deleted as vacuous).
}

// --- evidence-error legs -----------------------------------------------------

func TestEvaluateWorkDestruction_AncestryProbeErrorPropagates(t *testing.T) {
	dir, branch, target := wdHonestStaleBranchFixture(t)

	orig := workDestructionIsAncestorFn
	t.Cleanup(func() { workDestructionIsAncestorFn = orig })
	simulated := errors.New("simulated ancestry probe failure")
	workDestructionIsAncestorFn = func(workdir, ancestor, descendant string) (bool, error) {
		return false, simulated
	}

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected the forced ancestry failure to propagate, got outcome=%s, nil error", outcome)
	}
	if !errors.Is(err, simulated) {
		t.Errorf("expected the propagated error to wrap the simulated failure, got: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError", outcome)
	}
	if evidence.FailedProbe == "" {
		t.Error("evidence.FailedProbe must name the failed probe")
	}
}

func TestEvaluateWorkDestruction_SupersessionProbeErrorPropagates(t *testing.T) {
	dir, branch, target := wdHonestStaleBranchFixture(t)

	orig := workDestructionNetEffectFn
	t.Cleanup(func() { workDestructionNetEffectFn = orig })
	simulated := errors.New("simulated supersession probe failure")
	workDestructionNetEffectFn = func(workdir, ref, target string) (bool, error) {
		return false, simulated
	}

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected the forced supersession failure to propagate, got outcome=%s, nil error", outcome)
	}
	if !errors.Is(err, simulated) {
		t.Errorf("expected the propagated error to wrap the simulated failure, got: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError", outcome)
	}
	if evidence.FailedProbe == "" {
		t.Error("evidence.FailedProbe must name the failed probe")
	}
}

// TestEvaluateWorkDestruction_MergeBaseProbeErrorPropagates is O2-2(a): the
// step-2 merge-base resolution (evidence.MergeBase) must propagate a probe
// failure from INSIDE EvaluateWorkDestruction itself, not merely at the
// raw mergeBaseFn primitive's own unit tests elsewhere — and FailedProbe
// must name the exact call, which no prior test read. NetEffectLanded is
// stubbed to bypass its OWN internal mergeBaseFn use (the same package
// seam) so the forced failure is unambiguously attributed to step 2's
// explicit call, not to supersession's.
func TestEvaluateWorkDestruction_MergeBaseProbeErrorPropagates(t *testing.T) {
	dir, branch, target := wdHonestStaleBranchFixture(t)

	origNetEffect := workDestructionNetEffectFn
	t.Cleanup(func() { workDestructionNetEffectFn = origNetEffect })
	workDestructionNetEffectFn = func(workdir, ref, target string) (bool, error) {
		return false, nil
	}

	orig := mergeBaseFn
	t.Cleanup(func() { mergeBaseFn = orig })
	simulated := errors.New("simulated merge-base probe failure")
	mergeBaseFn = func(workdir, ref, target string) (string, error) {
		return "", simulated
	}

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected the forced merge-base failure to propagate, got outcome=%s, nil error", outcome)
	}
	if !errors.Is(err, simulated) {
		t.Errorf("expected the propagated error to wrap the simulated failure, got: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError", outcome)
	}
	want := fmt.Sprintf("merge-base(%s, %s)", branch, target)
	if evidence.FailedProbe != want {
		t.Errorf("evidence.FailedProbe = %q, want %q", evidence.FailedProbe, want)
	}
}

// TestEvaluateWorkDestruction_StripNovelPathsProbeErrorPropagates is
// O2-2(b): stripNovelPaths' own failure, forced from INSIDE
// snapshotRevertMatch (the seam existed but no test forced it before this
// fix round) — stripNovelPaths is the only step that writes outside the
// repo (os.CreateTemp for GIT_INDEX_FILE), so a denied/full TMPDIR fails
// on exactly the destructive shape this predicate exists to catch.
func TestEvaluateWorkDestruction_StripNovelPathsProbeErrorPropagates(t *testing.T) {
	dir, branch, target := wdStaleDeletionSingleCommitFixture(t)

	orig := workDestructionStripPathsFn
	t.Cleanup(func() { workDestructionStripPathsFn = orig })
	simulated := errors.New("simulated stripNovelPaths probe failure")
	workDestructionStripPathsFn = func(workdir, ref string, strip []string) (string, error) {
		return "", simulated
	}

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected the forced stripNovelPaths failure to propagate, got outcome=%s, nil error", outcome)
	}
	if !errors.Is(err, simulated) {
		t.Errorf("expected the propagated error to wrap the simulated failure, got: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError", outcome)
	}
	if evidence.FailedProbe != "snapshot-revert scan" {
		t.Errorf("evidence.FailedProbe = %q, want %q", evidence.FailedProbe, "snapshot-revert scan")
	}
}

// TestEvaluateWorkDestruction_FindAncestorTreeProbeErrorPropagates is
// O2-2(c): findAncestorWithTree's own failure, forced from INSIDE
// snapshotRevertMatch.
func TestEvaluateWorkDestruction_FindAncestorTreeProbeErrorPropagates(t *testing.T) {
	dir, branch, target := wdStaleDeletionSingleCommitFixture(t)

	orig := workDestructionFindAncestorTreeFn
	t.Cleanup(func() { workDestructionFindAncestorTreeFn = orig })
	simulated := errors.New("simulated findAncestorWithTree probe failure")
	workDestructionFindAncestorTreeFn = func(workdir, target, wantTree string, maskPaths []string) (string, error) {
		return "", simulated
	}

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected the forced findAncestorWithTree failure to propagate, got outcome=%s, nil error", outcome)
	}
	if !errors.Is(err, simulated) {
		t.Errorf("expected the propagated error to wrap the simulated failure, got: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError", outcome)
	}
	if evidence.FailedProbe != "snapshot-revert scan" {
		t.Errorf("evidence.FailedProbe = %q, want %q", evidence.FailedProbe, "snapshot-revert scan")
	}
}

func TestEvaluateWorkDestruction_PreviewDeletedPathsProbeErrorPropagates(t *testing.T) {
	dir, branch, target := wdHonestStaleBranchFixture(t)

	orig := workDestructionPreviewDeletedFn
	t.Cleanup(func() { workDestructionPreviewDeletedFn = orig })
	simulated := errors.New("simulated PreviewDeletedPaths probe failure")
	workDestructionPreviewDeletedFn = func(workdir, target, branch string) ([]string, error) {
		return nil, simulated
	}

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected the forced PreviewDeletedPaths failure to propagate, got outcome=%s, nil error", outcome)
	}
	if !errors.Is(err, simulated) {
		t.Errorf("expected the propagated error to wrap the simulated failure, got: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError", outcome)
	}
	if evidence.FailedProbe == "" {
		t.Error("evidence.FailedProbe must name the failed probe")
	}
}

func TestEvaluateWorkDestruction_SnapshotRevertScanProbeErrorPropagates(t *testing.T) {
	dir, branch, target := wdHonestCleanupFixture(t)

	orig := workDestructionNovelPathsFn
	t.Cleanup(func() { workDestructionNovelPathsFn = orig })
	simulated := errors.New("simulated snapshot-revert scan failure")
	workDestructionNovelPathsFn = func(workdir, target, branch string) ([]string, []string, error) {
		return nil, nil, simulated
	}

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected the forced snapshot-revert failure to propagate, got outcome=%s, nil error", outcome)
	}
	if !errors.Is(err, simulated) {
		t.Errorf("expected the propagated error to wrap the simulated failure, got: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError", outcome)
	}
	if evidence.FailedProbe != "snapshot-revert scan" {
		t.Errorf("evidence.FailedProbe = %q, want %q", evidence.FailedProbe, "snapshot-revert scan")
	}
}

// TestEvaluateWorkDestruction_ShallowHistoryFailsClosed is O1-3's
// RED-on-the-prior-bug regression: on a real, genuinely shallow clone,
// the ancestor scan must fail closed with DestructionEvidenceError, never
// silently read the truncated history's "no match" as DestructionClean.
func TestEvaluateWorkDestruction_ShallowHistoryFailsClosed(t *testing.T) {
	dir, branch, target := wdShallowHistoryFixture(t)

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected a non-nil error on a shallow/truncated history, got outcome=%s", outcome)
	}
	if !errors.Is(err, errTruncatedHistory) {
		t.Errorf("expected the propagated error to wrap errTruncatedHistory, got: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError (absence of evidence from a truncated history must never read as DestructionClean)", outcome)
	}
	if evidence.FailedProbe != "snapshot-revert scan" {
		t.Errorf("evidence.FailedProbe = %q, want %q", evidence.FailedProbe, "snapshot-revert scan")
	}
}

// TestEvaluateWorkDestruction_ReplaceRefTruncatedHistoryFailsClosed is
// O1-3's still-open finding (spec 127 bead-1 fix round 2): a replace ref
// truncates history the SAME way a shallow clone does — Clean, no
// error, D-set fully enumerated — via a mechanism isShallowRepo alone
// cannot see.
func TestEvaluateWorkDestruction_ReplaceRefTruncatedHistoryFailsClosed(t *testing.T) {
	dir, branch, target := wdReplaceRefTruncatedHistoryFixture(t)

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected a non-nil error on a replace-ref-truncated history, got outcome=%s", outcome)
	}
	if !errors.Is(err, errTruncatedHistory) {
		t.Errorf("expected the propagated error to wrap errTruncatedHistory, got: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError (absence of evidence from a replace-ref-truncated history must never read as DestructionClean)", outcome)
	}
	if evidence.FailedProbe != "snapshot-revert scan" {
		t.Errorf("evidence.FailedProbe = %q, want %q", evidence.FailedProbe, "snapshot-revert scan")
	}
}

// TestEvaluateWorkDestruction_GraftsTruncatedHistoryFailsClosed is O1-3's
// third truncation mechanism (spec 127 bead-1 fix round 2): a legacy
// `.git/info/grafts` file — detected, alongside a replace ref, by the
// single differential measurement historyTruncated performs (spec 127
// bead-1 fix round 3->4), independently of isShallowRepo.
func TestEvaluateWorkDestruction_GraftsTruncatedHistoryFailsClosed(t *testing.T) {
	dir, branch, target := wdGraftsTruncatedHistoryFixture(t)

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected a non-nil error on a grafts-truncated history, got outcome=%s", outcome)
	}
	if !errors.Is(err, errTruncatedHistory) {
		t.Errorf("expected the propagated error to wrap errTruncatedHistory, got: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError (absence of evidence from a grafts-truncated history must never read as DestructionClean)", outcome)
	}
	if evidence.FailedProbe != "snapshot-revert scan" {
		t.Errorf("evidence.FailedProbe = %q, want %q", evidence.FailedProbe, "snapshot-revert scan")
	}
}

// TestHistoryTruncated_BenignReplaceRefDoesNotOverRefuse is the
// over-fire regression pin for spec 127 bead-1 fix round 3->4
// (NEW-O1r-A/NEW-O2R-a/NEW-G1sub-1's own confirming repro, NEW-G1sub-2):
// fix round 2's presence-only probes turned EVERY subsequent evaluation
// on a repo carrying a BENIGN replace ref — git's own documented use
// case, a corrected author line on a same-tree commit — into a
// PERMANENT DestructionEvidenceError, even though rev-list's view of
// the affected history is complete either way. RED on that prior
// design: this fixture's replace ref, injected via `git replace <root>
// <same-tree-corrected-message-commit>` after a positive
// DestructionStaleDeletion control, changes `rev-list --count main` by
// ZERO — verified below — so historyTruncated must report false and
// EvaluateWorkDestruction must still reach and answer
// DestructionStaleDeletion, not fall back to a refusal.
func TestHistoryTruncated_BenignReplaceRefDoesNotOverRefuse(t *testing.T) {
	dir, branch, target := wdStaleDeletionSingleCommitFixture(t)

	controlOutcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("fixture invariant: unexpected error evaluating the pre-replace repo: %v", err)
	}
	if controlOutcome != guard.DestructionStaleDeletion {
		t.Fatalf("fixture invariant: pre-replace repo must classify DestructionStaleDeletion (the positive control), got %s", controlOutcome)
	}

	before := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--count", target))
	root := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--max-parents=0", target))
	tree := strings.TrimSpace(neRunGit(t, dir, "rev-parse", root+"^{tree}"))
	corrected := strings.TrimSpace(neRunGit(t, dir, "commit-tree", tree, "-m", "root (author corrected)"))
	neRunGit(t, dir, "replace", root, corrected)
	after := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--count", target))
	if before != after {
		t.Fatalf("fixture invariant broken: this replace ref must NOT truncate rev-list's view of %s (before=%s after=%s) — otherwise this is not the BENIGN case NEW-O1r-A/NEW-G1sub-2 named", target, before, after)
	}

	truncated, terr := historyTruncated(dir, target)
	if terr != nil {
		t.Fatalf("unexpected error from historyTruncated: %v", terr)
	}
	if truncated {
		t.Fatalf("historyTruncated(%s) = true, want false — a benign replace ref that truncates nothing must not be reported as truncating (NEW-O1r-A's over-fire class)", target)
	}

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("EvaluateWorkDestruction returned an unexpected error with a benign replace ref present: %v", err)
	}
	if outcome != guard.DestructionStaleDeletion {
		t.Fatalf("outcome = %s, want DestructionStaleDeletion — a benign replace ref must not turn this into a permanent refusal (NEW-O1r-A's over-fire class, the false-refusal direction this predicate's contract forbids)", outcome)
	}
}

// TestHistoryTruncated_BenignGraftsFileDoesNotOverRefuse mirrors
// TestHistoryTruncated_BenignReplaceRefDoesNotOverRefuse for the OTHER
// truncation mechanism (spec 127 bead-1 fix round 3->4): a grafts file
// naming a commit's TRUE parents (i.e. one that changes nothing about
// what rev-list reaches) must not refuse either.
func TestHistoryTruncated_BenignGraftsFileDoesNotOverRefuse(t *testing.T) {
	dir, branch, target := wdStaleDeletionSingleCommitFixture(t)

	controlOutcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("fixture invariant: unexpected error evaluating the pre-grafts repo: %v", err)
	}
	if controlOutcome != guard.DestructionStaleDeletion {
		t.Fatalf("fixture invariant: pre-grafts repo must classify DestructionStaleDeletion (the positive control), got %s", controlOutcome)
	}

	before := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--count", target))
	root := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--max-parents=0", target))
	neRunGit(t, dir, "config", "advice.graftFileDeprecated", "false")
	graftsPath := strings.TrimSpace(neRunGit(t, dir, "rev-parse", "--git-path", "info/grafts"))
	if !filepath.IsAbs(graftsPath) {
		graftsPath = filepath.Join(dir, graftsPath)
	}
	if err := os.MkdirAll(filepath.Dir(graftsPath), 0o755); err != nil {
		t.Fatalf("mkdir info/: %v", err)
	}
	// A graft naming root's TRUE (i.e. its actual, unchanged) parents —
	// root has none, so this line changes nothing rev-list can see.
	if err := os.WriteFile(graftsPath, []byte(root+"\n"), 0o644); err != nil {
		t.Fatalf("writing info/grafts: %v", err)
	}
	after := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--count", target))
	if before != after {
		t.Fatalf("fixture invariant broken: this grafts entry must NOT truncate rev-list's view of %s (before=%s after=%s)", target, before, after)
	}

	truncated, terr := historyTruncated(dir, target)
	if terr != nil {
		t.Fatalf("unexpected error from historyTruncated: %v", terr)
	}
	if truncated {
		t.Fatalf("historyTruncated(%s) = true, want false — a benign grafts entry that truncates nothing must not be reported as truncating (NEW-O1r-A's over-fire class)", target)
	}

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("EvaluateWorkDestruction returned an unexpected error with a benign grafts file present: %v", err)
	}
	if outcome != guard.DestructionStaleDeletion {
		t.Fatalf("outcome = %s, want DestructionStaleDeletion — a benign grafts file must not turn this into a permanent refusal", outcome)
	}
}

// TestHistoryTruncated_RelocatedReplaceRefStillDetected is the
// under-fire regression pin (spec 127 bead-1 fix round 3->4,
// NEW-G1sub-1): fix round 2's hasReplaceRefs hardcoded the
// `refs/replace/` prefix, but git honors GIT_REPLACE_REF_BASE and
// resolves a replacement's ref name by concatenating that variable with
// the target OID, so a replacement can live at a ref path a scan of
// "refs/replace/" never visits. This fixture relocates the base to
// "refs/myreplace/", putting the replacement at
// "refs/myreplace/<hex>" — outside the hardcoded namespace, which the
// fixture invariants below assert directly rather than assume.
//
// GIT-VERSION NOTE (spec 127 CI-identity fix round): this fixture used to
// relocate to "refs/myreplace" with NO trailing slash, on the strength of
// an empirical check that git concatenated it unnormalized into the
// literal ref "refs/myreplace<hex>". That check was run against git
// 2.51 and no longer holds: git 2.54+ aborts outright on a slashless
// replace-ref base — `BUG: refs.c: ref pattern must end in a trailing
// slash when trimming` — killing even a bare `git for-each-ref`, which
// turned this test red on CI (git 2.54) while it stayed green locally
// (git 2.51). The trailing-slash base exercises the same relocation
// property this test is named for, on every git version. So a genuinely truncating
// relocated replace ref reported "not truncated" under the old design —
// the exact fail-open this predicate exists to refuse, reappearing
// inside the mechanism fix round 2 added to close it. historyTruncated's
// `--no-replace-objects` differential must catch this regardless of
// relocation, because that flag disables replacement lookup
// universally, not by ref-namespace enumeration.
func TestHistoryTruncated_RelocatedReplaceRefStillDetected(t *testing.T) {
	dir, branch, target := wdStaleDeletionSingleCommitFixture(t)

	controlOutcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("fixture invariant: unexpected error evaluating the pre-replace repo: %v", err)
	}
	if controlOutcome != guard.DestructionStaleDeletion {
		t.Fatalf("fixture invariant: pre-replace repo must classify DestructionStaleDeletion (the positive control), got %s", controlOutcome)
	}

	before := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--count", target))

	// Relocate the ref base BEFORE creating the replace ref, so the ref
	// this `git replace --graft` creates lives outside refs/replace/.
	// The trailing slash is REQUIRED by git 2.54+ (see the doc comment's
	// git-version note); the relocation itself is what this test needs,
	// and the fixture invariants below prove it actually took effect.
	t.Setenv("GIT_REPLACE_REF_BASE", "refs/myreplace/")
	tip := strings.TrimSpace(neRunGit(t, dir, "rev-parse", target))
	neRunGit(t, dir, "replace", "--graft", tip)

	// Fixture invariant: the relocated ref must be INVISIBLE to a
	// for-each-ref scan of the DEFAULT namespace — otherwise this does
	// not exercise the relocation this test is named for.
	defaultNamespace := neRunGit(t, dir, "for-each-ref", "refs/replace/")
	if strings.TrimSpace(defaultNamespace) != "" {
		t.Fatalf("fixture invariant broken: the relocated replace ref must not appear under refs/replace/, got %q", defaultNamespace)
	}
	after := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--count", target))
	if before == after {
		t.Fatalf("fixture invariant broken: the relocated replace ref must actually TRUNCATE rev-list's view of %s (before=%s after=%s)", target, before, after)
	}

	truncated, terr := historyTruncated(dir, target)
	if terr != nil {
		t.Fatalf("unexpected error from historyTruncated: %v", terr)
	}
	if !truncated {
		t.Fatalf("historyTruncated(%s) = false, want true — a relocated replace ref that genuinely truncates history must still be detected regardless of GIT_REPLACE_REF_BASE (NEW-G1sub-1's under-fire class)", target)
	}

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected a non-nil error on a relocated-replace-ref-truncated history, got outcome=%s", outcome)
	}
	if !errors.Is(err, errTruncatedHistory) {
		t.Errorf("expected the propagated error to wrap errTruncatedHistory, got: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError — a relocated replace ref must fail closed exactly like one at the default namespace (NEW-G1sub-1)", outcome)
	}
}

// TestHistoryTruncated_CountPreservingSubstitutionCaught is the
// RED-on-the-round-4-bug pin for spec 127 bead-1 fix round 5
// (NEW-G1sub-6/NEW-O1g-A/NEW-O2G-1/NEW-O3g-1): a git-replace substitution
// that keeps the reconstructed ancestor's EXACT parents — git-replace's
// own documented PRIMARY use case, a content/metadata correction that
// preserves ancestry by construction — but swaps in a DIFFERENT tree
// changes neither `rev-list --count` nor the substituted commit's own
// reported OID (rev-list's %H reports the ORIGINAL commit's hash even
// through a replacement — verified directly below). The round-4
// length-only differential answered "not truncated" on exactly this
// shape, and findAncestorWithTree's scan then ran against a history
// whose trees were not target's real ones, silently answering
// DestructionClean on a genuine deletion. This pins the fixed
// (commit,tree) MULTISET differential in historyTruncated instead
// (order-insensitive as of fix round 6, NEW-G1sub-11): reverting it to a
// bare count comparison must turn this test red.
func TestHistoryTruncated_CountPreservingSubstitutionCaught(t *testing.T) {
	dir, branch, target := wdStaleDeletionSingleCommitFixture(t)

	controlOutcome, controlEvidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("fixture invariant: unexpected error evaluating the pre-replace repo: %v", err)
	}
	if controlOutcome != guard.DestructionStaleDeletion {
		t.Fatalf("fixture invariant: pre-replace repo must classify DestructionStaleDeletion (the positive control), got %s", controlOutcome)
	}
	anc := controlEvidence.ReconstructedAncestor
	if anc == "" {
		t.Fatal("fixture invariant: ReconstructedAncestor must be populated by the positive control")
	}

	before := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--count", target))
	beforeHash := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--format=%H %T", target))

	// Forge a same-parents, different-tree replacement for the
	// reconstructed ancestor: read its real parents, then commit-tree a
	// DIFFERENT (empty) tree onto those exact same parents.
	parentFields := strings.Fields(strings.TrimSpace(neRunGit(t, dir, "rev-list", "--parents", "-n", "1", anc)))
	var commitTreeArgs []string
	for _, p := range parentFields[1:] {
		commitTreeArgs = append(commitTreeArgs, "-p", p)
	}
	mktreeCmd := exec.Command("git", "-C", dir, "mktree")
	mktreeCmd.Stdin = strings.NewReader("")
	emptyTreeOut, merr := mktreeCmd.Output()
	if merr != nil {
		t.Fatalf("mktree: %v", merr)
	}
	emptyTree := strings.TrimSpace(string(emptyTreeOut))
	forgedArgs := append([]string{"commit-tree", emptyTree, "-m", "forged same-parents different-tree replacement"}, commitTreeArgs...)
	forged := strings.TrimSpace(neRunGit(t, dir, forgedArgs...))
	neRunGit(t, dir, "replace", anc, forged)

	// Fixture invariant: this replacement must be COUNT-PRESERVING (the
	// shape the round-4 differential could not see) and must NOT change
	// what rev-list reports for %H at any position (git-replace
	// transparently substitutes CONTENT, not the OID used to reach it).
	after := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--count", target))
	if before != after {
		t.Fatalf("fixture invariant broken: this replacement must NOT change rev-list --count (before=%s after=%s) — otherwise it does not exercise the count-preserving shape this test pins", before, after)
	}
	afterHash := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--format=%H %T", target))
	beforeSHAs := extractCommitSHAs(beforeHash)
	afterSHAs := extractCommitSHAs(afterHash)
	if strings.Join(beforeSHAs, ",") != strings.Join(afterSHAs, ",") {
		t.Fatalf("fixture invariant broken: the replacement must not change any commit's OID as reported by rev-list (before=%v after=%v) — otherwise this is not the same-OID/different-tree shape the round-4 bug missed", beforeSHAs, afterSHAs)
	}

	truncated, terr := historyTruncated(dir, target)
	if terr != nil {
		t.Fatalf("unexpected error from historyTruncated: %v", terr)
	}
	if !truncated {
		t.Fatalf("historyTruncated(%s) = false, want true — a count-preserving tree substitution must be detected (spec 127 bead-1 fix round 5, NEW-G1sub-6/NEW-O1g-A/NEW-O2G-1/NEW-O3g-1's fail-open)", target)
	}

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected a non-nil error on a count-preserving substitution, got outcome=%s (the exact silent-Clean fail-open fix round 5 closed)", outcome)
	}
	if !errors.Is(err, errTruncatedHistory) {
		t.Errorf("expected the propagated error to wrap errTruncatedHistory, got: %v", err)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError (absence of evidence from a count-preserving substitution must never read as DestructionClean)", outcome)
	}
	if evidence.FailedProbe != "snapshot-revert scan" {
		t.Errorf("evidence.FailedProbe = %q, want %q", evidence.FailedProbe, "snapshot-revert scan")
	}
}

// TestHistoryTruncated_BenignAuthorOnlyReplaceAcrossMergePermutesButDoesNotOverRefuse
// is the RED-on-round-5-bug pin for spec 127 bead-1 fix round 6
// (NEW-G1sub-11): rev-list's default ordering is commit-date driven, not
// purely topological, so two commits with no ancestor relationship to
// each other — here, two SIDE branches merged separately into target —
// are ordered by their own commit dates. git-replace's own headline use
// case — a same-tree, same-parent author/message correction — gives the
// REPLACEMENT commit a NEW committer date by default (`git replace
// --edit`, or any commit-tree recipe, both produce this), which can swap
// that commit's position relative to a SIBLING commit even though
// neither commit's OID nor its tree changes. Fix round 5's
// element-for-element sequence comparison treated that permutation as a
// truncation and refused a benign, honest branch — a false refusal of
// honest work, the one direction this predicate's contract forbids (see
// errTruncatedHistory's doc comment) — exactly because a merge is
// present in target's history; neither existing benign-replace pin above
// (TestHistoryTruncated_BenignReplaceRefDoesNotOverRefuse,
// TestHistoryTruncated_BenignGraftsFileDoesNotOverRefuse) exercises this,
// since both of those fixtures are LINEAR — no merge, so no two commits
// can swap. Reverting equalCommitTreeMultisets to an element-for-element
// comparison must turn this test red.
func TestHistoryTruncated_BenignAuthorOnlyReplaceAcrossMergePermutesButDoesNotOverRefuse(t *testing.T) {
	dir := initGitRepo(t)
	root := strings.TrimSpace(neRunGit(t, dir, "rev-parse", "main"))

	setDate := func(hhmmss string) {
		t.Setenv("GIT_AUTHOR_DATE", "2024-01-01T"+hhmmss)
		t.Setenv("GIT_COMMITTER_DATE", "2024-01-01T"+hhmmss)
	}

	neRunGit(t, dir, "checkout", "-b", "sideA")
	neWriteFile(t, dir, "a1.txt", "a1\n")
	neRunGit(t, dir, "add", ".")
	setDate("01:00:00")
	neRunGit(t, dir, "commit", "-m", "a1")
	a1 := strings.TrimSpace(neRunGit(t, dir, "rev-parse", "HEAD"))

	neRunGit(t, dir, "checkout", "-b", "sideB", root)
	neWriteFile(t, dir, "b1.txt", "b1\n")
	neRunGit(t, dir, "add", ".")
	setDate("02:00:00")
	neRunGit(t, dir, "commit", "-m", "b1")

	neRunGit(t, dir, "checkout", "main")
	setDate("03:00:00")
	neRunGit(t, dir, "merge", "--no-ff", "-m", "mergeA", "sideA")
	setDate("04:00:00")
	neRunGit(t, dir, "merge", "--no-ff", "-m", "mergeB", "sideB")
	mergeBTree := strings.TrimSpace(neRunGit(t, dir, "rev-parse", "main^{tree}"))

	neWriteFile(t, dir, "landed.txt", "landed\n")
	neRunGit(t, dir, "add", ".")
	setDate("05:00:00")
	neRunGit(t, dir, "commit", "-m", "C2 landed work")

	neRunGit(t, dir, "checkout", "-b", "spec-recreated-order", "main")
	neRunGit(t, dir, "rm", "landed.txt")
	setDate("06:00:00")
	neRunGit(t, dir, "commit", "-m", "recreated spec branch: reverts to mergeB's tree")
	branch, target := "spec-recreated-order", "main"
	neRunGit(t, dir, "checkout", "main")

	// Fixture invariant: branch's tip tree must exactly equal mergeB's
	// tree — the positive control this test's over-fire check depends on.
	branchTree := strings.TrimSpace(neRunGit(t, dir, "rev-parse", branch+"^{tree}"))
	if branchTree != mergeBTree {
		t.Fatalf("fixture invariant broken: branch tip tree %s != mergeB's tree %s", branchTree, mergeBTree)
	}

	controlOutcome, controlEvidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("fixture invariant: unexpected error evaluating the pre-replace repo: %v", err)
	}
	if controlOutcome != guard.DestructionStaleDeletion {
		t.Fatalf("fixture invariant: pre-replace repo must classify DestructionStaleDeletion (the positive control), got %s", controlOutcome)
	}
	if controlEvidence.ReconstructedAncestor == "" {
		t.Fatal("fixture invariant: ReconstructedAncestor must be populated by the positive control")
	}

	before := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--count", target))
	beforeHash := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--format=%H %T", target))

	// A same-tree, same-parent author-only correction to a1 — git-replace's
	// own headline use case. A real `git replace --edit` or commit-tree
	// recipe would leave the date unset, so git defaults it to the current
	// wall-clock time; this test instead sets an explicit, LATER,
	// deterministic date below (rather than depending on real time),
	// chosen to keep the permutation minimal (a1/b1 swap in place) rather
	// than reshuffling the whole history the way an arbitrarily-distant
	// "now" might.
	a1Tree := strings.TrimSpace(neRunGit(t, dir, "rev-parse", a1+"^{tree}"))
	a1Parent := strings.TrimSpace(neRunGit(t, dir, "rev-parse", a1+"^"))
	setDate("10:00:00")
	corrected := strings.TrimSpace(neRunGit(t, dir, "commit-tree", a1Tree, "-p", a1Parent, "-m", "a1 (author corrected)"))
	neRunGit(t, dir, "replace", a1, corrected)

	after := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--count", target))
	if before != after {
		t.Fatalf("fixture invariant broken: this replacement must NOT change rev-list --count (before=%s after=%s) — otherwise it is not the benign, count-preserving shape this test pins", before, after)
	}
	afterHash := strings.TrimSpace(neRunGit(t, dir, "rev-list", "--format=%H %T", target))
	beforeSHAs := extractCommitSHAs(beforeHash)
	afterSHAs := extractCommitSHAs(afterHash)
	if len(beforeSHAs) != len(afterSHAs) {
		t.Fatalf("fixture invariant broken: commit count changed (before=%v after=%v)", beforeSHAs, afterSHAs)
	}
	beforeSorted := append([]string(nil), beforeSHAs...)
	afterSorted := append([]string(nil), afterSHAs...)
	sort.Strings(beforeSorted)
	sort.Strings(afterSorted)
	if strings.Join(beforeSorted, ",") != strings.Join(afterSorted, ",") {
		t.Fatalf("fixture invariant broken: the SET of commit OIDs rev-list reports must be unchanged (before=%v after=%v) — git-replace transparently substitutes content, never the OID used to reach it", beforeSHAs, afterSHAs)
	}
	if strings.Join(beforeSHAs, ",") == strings.Join(afterSHAs, ",") {
		t.Fatalf("fixture invariant broken: this replacement must actually PERMUTE rev-list's ordering (before and after are identical sequences) — otherwise this does not exercise NEW-G1sub-11's shape at all; adjust the merge topology or commit dates")
	}

	truncated, terr := historyTruncated(dir, target)
	if terr != nil {
		t.Fatalf("unexpected error from historyTruncated: %v", terr)
	}
	if truncated {
		t.Fatalf("historyTruncated(%s) = true, want false — a benign, same-tree author-only replacement must not be reported as truncating merely because it permutes rev-list's date-ordered listing (spec 127 bead-1 fix round 6, NEW-G1sub-11's false-refusal class)", target)
	}

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("EvaluateWorkDestruction returned an unexpected error with a benign, order-permuting replace ref present: %v", err)
	}
	if outcome != guard.DestructionStaleDeletion {
		t.Fatalf("outcome = %s, want DestructionStaleDeletion — a benign replacement must not turn this into a permanent refusal merely because it reordered rev-list's listing", outcome)
	}
}

// extractCommitSHAs pulls the first field (the commit SHA) from each
// "%H %T" data line rev-list --format produces, skipping its own
// "commit <sha>" header lines — the same parsing findAncestorWithTree's
// scan and revListCommitTrees both perform, duplicated here rather than
// exported, since only this test needs it.
func extractCommitSHAs(revListFormatOutput string) []string {
	var shas []string
	for _, line := range strings.Split(revListFormatOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] == "commit" {
			continue
		}
		shas = append(shas, fields[0])
	}
	return shas
}

// TestEvaluateWorkDestruction_NoMainRefNarrowsToTargetAlone is spec 127
// bead-6 fix round 11's regression fix (S2-2/F1-0, git-bisected against
// 2b02be5c): a repo with no local `main` ref must NOT fail every
// evaluation closed — that turned preflightMergeDestruction (this bead's
// new, non-error-discarding consumer, wired directly into CompleteBead/
// FinalizeEpic/the spec→main merge) into an unconditional refusal of
// every bead→spec and spec→main merge in any repository whose trunk is
// not literally named main. Evaluation must instead narrow to target
// ("trunk" here) alone and reach its ordinary conclusion: branch adds a
// file target does not have and deletes nothing target has, so the D-set
// is empty and the outcome is DestructionClean, with no error at all.
func TestEvaluateWorkDestruction_NoMainRefNarrowsToTargetAlone(t *testing.T) {
	dir, branch, target := wdNoMainRefFixture(t)

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("no local main ref must not be treated as an evidence error: %v", err)
	}
	if outcome != guard.DestructionClean {
		t.Errorf("outcome = %s, want DestructionClean", outcome)
	}
}

// TestEvaluateWorkDestruction_MainRefExistenceCheckFailureIsEvidenceError
// pins the OTHER half of fix round 11's fix: only main's mere ABSENCE is
// exempted from hard-erroring — a genuine structural failure checking
// for its existence (a corrupt repository, git itself missing) must still
// surface as DestructionEvidenceError, exactly like every other probe
// failure. Uses a fixture whose target is NOT "main" (wdAncestorOfTargetFixture)
// so ancestryTargets actually calls workDestructionBranchExistsFn rather
// than short-circuiting on target == "main".
func TestEvaluateWorkDestruction_MainRefExistenceCheckFailureIsEvidenceError(t *testing.T) {
	dir, branch, target := wdAncestorOfTargetFixture(t)

	orig := workDestructionBranchExistsFn
	t.Cleanup(func() { workDestructionBranchExistsFn = orig })
	workDestructionBranchExistsFn = func(workdir, name string) (bool, error) {
		return false, errors.New("boom")
	}

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected a non-nil error, got outcome=%s", outcome)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError", outcome)
	}
	if evidence.FailedProbe != "ancestryTargets(main existence)" {
		t.Errorf("evidence.FailedProbe = %q, want %q", evidence.FailedProbe, "ancestryTargets(main existence)")
	}
}

// TestEvaluateWorkDestruction_UnrelatedHistoriesFailsClosedAndIsDocumented
// is O1-7/O2-7's other fixtured disposition: branch and target sharing no
// common ancestor fails closed via NetEffectLanded's internal merge-base
// call.
func TestEvaluateWorkDestruction_UnrelatedHistoriesFailsClosedAndIsDocumented(t *testing.T) {
	dir, branch, target := wdUnrelatedHistoriesFixture(t)

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected a non-nil error on unrelated branch/target histories, got outcome=%s", outcome)
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError", outcome)
	}
	want := fmt.Sprintf("NetEffectLanded(%s, %s)", branch, target)
	if evidence.FailedProbe != want {
		t.Errorf("evidence.FailedProbe = %q, want %q", evidence.FailedProbe, want)
	}
}

// TestEvaluateWorkDestruction_EvidenceErrorNeverFoldedIntoBoolean asserts
// the mechanism the plan pins for AC-2(ix)/AC-3/AC-7 consumers: the
// outcome on a probe failure is the NAMED evidence-error variant, never
// silently coerced to DestructionClean/DestructionAncestor or any other
// "safe-looking" value. Genuinely table-driven across every forceable
// seam this file exercises (spec 127 bead-1 fix round, O2-2: the prior
// version forced exactly one seam — IsAncestor — while claiming in its own
// comment to be "table-driven across every seam above").
//
// Each row now also asserts evidence.FailedProbe (spec 127 bead-1 fix
// round 2, NEW-O2-c): the round-1 "MergeBase" row forced mergeBaseFn
// WITHOUT stubbing workDestructionNetEffectFn, and NetEffectLanded uses
// the SAME package seam internally (neteffect.go), so the forced failure
// was actually consumed at the SUPERSESSION leg — the row passed, but on
// a different leg's failure than its name claimed, making it a silent
// duplicate of "Supersession" rather than genuine coverage of step 2's
// OWN explicit merge-base call. Bypassing NetEffectLanded's internal use
// (the same technique
// TestEvaluateWorkDestruction_MergeBaseProbeErrorPropagates already
// uses) and asserting the exact FailedProbe string per row makes a row
// that passes on the wrong leg fail loudly instead of silently.
//
// Fix round 11 adds a "MainRefExistenceCheck" row (S2-2/F1-0): the new
// workDestructionBranchExistsFn seam, forced to fail here, must surface
// as DestructionEvidenceError exactly like every other probe — only
// main's mere ABSENCE (a false, nil return) is exempted from hard-
// erroring, never a genuine failure checking for its existence. Uses
// wdAncestorOfTargetFixture (target != "main", so ancestryTargets
// actually calls this seam rather than short-circuiting on target ==
// "main") — kept as its own dedicated test,
// TestEvaluateWorkDestruction_MainRefExistenceCheckFailureIsEvidenceError
// below, rather than folded into this table, since every other row here
// forces a seam that fires unconditionally regardless of fixture shape,
// while this one only fires for a target that is not literally "main".
func TestEvaluateWorkDestruction_EvidenceErrorNeverFoldedIntoBoolean(t *testing.T) {
	unsafeOutcomes := []guard.DestructionOutcome{
		guard.DestructionClean, guard.DestructionAncestor,
		guard.DestructionSuperseded, guard.DestructionStaleDeletion,
	}

	cases := []struct {
		name string
		// snapshotRevert requests a fixture that actually reaches the
		// stale-deletion leg (a non-empty D-set); the others fire on any
		// fixture that reaches THEIR leg, so the generic honest-stale
		// fixture suffices.
		snapshotRevert     bool
		wantFailedProbeHas string // substring evidence.FailedProbe must contain
		force              func(t *testing.T)
	}{
		{name: "Ancestry", wantFailedProbeHas: "IsAncestor(", force: func(t *testing.T) {
			orig := workDestructionIsAncestorFn
			t.Cleanup(func() { workDestructionIsAncestorFn = orig })
			workDestructionIsAncestorFn = func(workdir, ancestor, descendant string) (bool, error) {
				return false, errors.New("boom")
			}
		}},
		{name: "Supersession", wantFailedProbeHas: "NetEffectLanded(", force: func(t *testing.T) {
			orig := workDestructionNetEffectFn
			t.Cleanup(func() { workDestructionNetEffectFn = orig })
			workDestructionNetEffectFn = func(workdir, ref, target string) (bool, error) {
				return false, errors.New("boom")
			}
		}},
		{name: "MergeBase", wantFailedProbeHas: "merge-base(", force: func(t *testing.T) {
			// Bypass NetEffectLanded's OWN internal mergeBaseFn use (the
			// same package seam) so the forced mergeBaseFn failure below
			// is unambiguously attributed to step 2's EXPLICIT call, not
			// to supersession's (NEW-O2-c).
			origNetEffect := workDestructionNetEffectFn
			t.Cleanup(func() { workDestructionNetEffectFn = origNetEffect })
			workDestructionNetEffectFn = func(workdir, ref, target string) (bool, error) {
				return false, nil
			}
			orig := mergeBaseFn
			t.Cleanup(func() { mergeBaseFn = orig })
			mergeBaseFn = func(workdir, ref, target string) (string, error) {
				return "", errors.New("boom")
			}
		}},
		{name: "PreviewDeletedPaths", wantFailedProbeHas: "PreviewDeletedPaths", force: func(t *testing.T) {
			orig := workDestructionPreviewDeletedFn
			t.Cleanup(func() { workDestructionPreviewDeletedFn = orig })
			workDestructionPreviewDeletedFn = func(workdir, target, branch string) ([]string, error) {
				return nil, errors.New("boom")
			}
		}},
		{name: "StripNovelPaths", snapshotRevert: true, wantFailedProbeHas: "snapshot-revert scan", force: func(t *testing.T) {
			orig := workDestructionStripPathsFn
			t.Cleanup(func() { workDestructionStripPathsFn = orig })
			workDestructionStripPathsFn = func(workdir, ref string, strip []string) (string, error) {
				return "", errors.New("boom")
			}
		}},
		{name: "FindAncestorTree", snapshotRevert: true, wantFailedProbeHas: "snapshot-revert scan", force: func(t *testing.T) {
			orig := workDestructionFindAncestorTreeFn
			t.Cleanup(func() { workDestructionFindAncestorTreeFn = orig })
			workDestructionFindAncestorTreeFn = func(workdir, target, wantTree string, maskPaths []string) (string, error) {
				return "", errors.New("boom")
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var dir, branch, target string
			if tc.snapshotRevert {
				dir, branch, target = wdStaleDeletionSingleCommitFixture(t)
			} else {
				dir, branch, target = wdHonestStaleBranchFixture(t)
			}
			tc.force(t)

			outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
			if err == nil {
				t.Fatal("expected a non-nil error")
			}
			for _, unsafe := range unsafeOutcomes {
				if outcome == unsafe {
					t.Fatalf("an evidence-computation error must never be folded into %s", unsafe)
				}
			}
			if outcome != guard.DestructionEvidenceError {
				t.Errorf("outcome = %s, want DestructionEvidenceError", outcome)
			}
			if !strings.Contains(evidence.FailedProbe, tc.wantFailedProbeHas) {
				t.Errorf("evidence.FailedProbe = %q, want a string containing %q — a row must fail on the leg its own name claims, not a different one", evidence.FailedProbe, tc.wantFailedProbeHas)
			}
		})
	}
}

// --- non-mutation and wiring checks -----------------------------------------

// TestEvaluateWorkDestruction_MutatesNothing runs the predicate over the
// most git-plumbing-heavy fixture (the stale-deletion witness, which
// exercises the temporary-index strip-and-scan machinery) and asserts refs
// and worktree/index status are byte-identical before and after.
func TestEvaluateWorkDestruction_MutatesNothing(t *testing.T) {
	dir, branch, target := wdStaleDeletionSingleCommitFixture(t)

	refsBefore := neRunGit(t, dir, "for-each-ref")
	statusBefore := neRunGit(t, dir, "status", "--porcelain")

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome != guard.DestructionStaleDeletion {
		t.Fatalf("fixture invariant broken: outcome = %s, want DestructionStaleDeletion", outcome)
	}

	refsAfter := neRunGit(t, dir, "for-each-ref")
	statusAfter := neRunGit(t, dir, "status", "--porcelain")
	if refsBefore != refsAfter {
		t.Errorf("refs changed:\nbefore: %q\nafter:  %q", refsBefore, refsAfter)
	}
	if statusBefore != statusAfter {
		t.Errorf("worktree/index status changed:\nbefore: %q\nafter:  %q", statusBefore, statusAfter)
	}
}

// TestEvaluateWorkDestruction_SeamsPinnedToRealSymbols is the default-pin
// half of "Error-forcing for tests rides unexported in-package seam vars
// with pointer-equality default pins (no exported knob)": every seam this
// file forces above must default to the REAL production symbol. Extended
// to ALL EIGHT of this file's own seams (spec 127 bead-1 fix round,
// O2-6/O3-3: the prior version pinned four of seven —
// workDestructionNovelPathsFn, workDestructionStripPathsFn, and
// workDestructionFindAncestorTreeFn were declared, and the first is even
// FORCED by a test above, without ever being pinned; two of the three
// are the snapshot-revert discriminator itself, the most safety-critical
// leg in this file) plus diffNameStatusBucketsFn (neteffect.go) and
// workDestructionIsShallowFn; fix round 2 (O1-3) added
// workDestructionHasReplaceRefsFn and workDestructionHasGraftsFileFn
// (bringing this file's own count to nine, ten with diffNameStatusBucketsFn);
// fix round 3->4 (NEW-O1r-A/NEW-O2R-a/NEW-G1sub-1) collapsed both of
// those into the single differential seam
// workDestructionHistoryTruncatedFn, bringing this file's own count back
// to eight.
//
// CORRECTED fix round 5 (NEW-G1sub-9/NEW-O2G-4): the universal above —
// "every seam THIS FILE FORCES must default to the real production
// symbol" — was true of the eight workDestruction*Fn seams plus
// diffNameStatusBucketsFn, but this file also forces mergeBaseFn
// (declared in neteffect.go, forced at
// TestEvaluateWorkDestruction_MergeBaseProbeErrorPropagates and again in
// the table-driven evidence-error test above) without ever pinning it —
// present since fix round 1 introduced this test, missed when rounds 2
// and 4 both edited this comment's seam count without checking the pin
// list against the force list. Pinned below; the count was eight
// workDestruction*Fn seams plus diffNameStatusBucketsFn and mergeBaseFn
// (ten total), and the universal held against every seam this file
// forces, not merely against the ones already named here.
//
// Fix round 11 (S2-2/F1-0) added workDestructionBranchExistsFn
// (ancestryTargets' check for a local main ref, defaulting to
// BranchExistsIn) — pinned below too, bringing the count to nine
// workDestruction*Fn seams plus diffNameStatusBucketsFn and mergeBaseFn
// (eleven total).
func TestEvaluateWorkDestruction_SeamsPinnedToRealSymbols(t *testing.T) {
	if reflect.ValueOf(workDestructionIsAncestorFn).Pointer() != reflect.ValueOf(IsAncestor).Pointer() {
		t.Error("workDestructionIsAncestorFn must default to IsAncestor")
	}
	if reflect.ValueOf(workDestructionNetEffectFn).Pointer() != reflect.ValueOf(NetEffectLanded).Pointer() {
		t.Error("workDestructionNetEffectFn must default to NetEffectLanded")
	}
	if reflect.ValueOf(workDestructionPreviewDeletedFn).Pointer() != reflect.ValueOf(PreviewDeletedPaths).Pointer() {
		t.Error("workDestructionPreviewDeletedFn must default to PreviewDeletedPaths")
	}
	if reflect.ValueOf(workDestructionNovelPathsFn).Pointer() != reflect.ValueOf(novelPaths).Pointer() {
		t.Error("workDestructionNovelPathsFn must default to novelPaths")
	}
	if reflect.ValueOf(workDestructionStripPathsFn).Pointer() != reflect.ValueOf(stripNovelPaths).Pointer() {
		t.Error("workDestructionStripPathsFn must default to stripNovelPaths")
	}
	if reflect.ValueOf(workDestructionFindAncestorTreeFn).Pointer() != reflect.ValueOf(findAncestorWithTree).Pointer() {
		t.Error("workDestructionFindAncestorTreeFn must default to findAncestorWithTree")
	}
	if reflect.ValueOf(workDestructionIsShallowFn).Pointer() != reflect.ValueOf(isShallowRepo).Pointer() {
		t.Error("workDestructionIsShallowFn must default to isShallowRepo")
	}
	if reflect.ValueOf(workDestructionHistoryTruncatedFn).Pointer() != reflect.ValueOf(historyTruncated).Pointer() {
		t.Error("workDestructionHistoryTruncatedFn must default to historyTruncated")
	}
	if reflect.ValueOf(diffNameStatusBucketsFn).Pointer() != reflect.ValueOf(diffNameStatusBuckets).Pointer() {
		t.Error("diffNameStatusBucketsFn must default to diffNameStatusBuckets")
	}
	if reflect.ValueOf(mergeBaseFn).Pointer() != reflect.ValueOf(gitMergeBase).Pointer() {
		t.Error("mergeBaseFn must default to gitMergeBase")
	}
	if reflect.ValueOf(workDestructionBranchExistsFn).Pointer() != reflect.ValueOf(BranchExistsIn).Pointer() {
		t.Error("workDestructionBranchExistsFn must default to BranchExistsIn")
	}
}

// TestEvaluateWorkDestruction_RejectsOptionLikeOperands: the SEC-5
// argv-hygiene pin, same as every other gitutil ref-bearing entry point.
func TestEvaluateWorkDestruction_RejectsOptionLikeOperands(t *testing.T) {
	dir := initGitRepo(t)
	if _, _, err := EvaluateWorkDestruction(dir, "-x", "main"); err == nil {
		t.Error("expected a rejection for an option-like branch operand")
	}
	if _, _, err := EvaluateWorkDestruction(dir, "main", "-x"); err == nil {
		t.Error("expected a rejection for an option-like target operand")
	}
}
