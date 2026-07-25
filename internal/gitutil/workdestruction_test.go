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
	"reflect"
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

// --- the outcome table ------------------------------------------------------

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
	}

	// Every non-error outcome above must be represented; the fixture below
	// (ProbeForcedEvidenceError) covers the fifth. This assertion is the
	// B-r4-3 sentinel discipline every later consumer's table copies:
	// DestructionOutcomeCount is 5 (four here + the forced-error fixture).
	seen := map[guard.DestructionOutcome]bool{guard.DestructionEvidenceError: true}
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
			if err != nil {
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
			_, rangeDeleted, err := diffNameStatusBucketsFn(dir, base, branch)
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

func TestEvaluateWorkDestruction_SubsumedOutcomeProbeErrorPropagates(t *testing.T) {
	dir, branch, target := wdHonestStaleBranchFixture(t)

	orig := workDestructionSubsumedFn
	t.Cleanup(func() { workDestructionSubsumedFn = orig })
	simulated := errors.New("simulated ContentSubsumedOutcome probe failure")
	workDestructionSubsumedFn = func(workdir, base, ref, target string) (Subsumption, error) {
		return SubsumptionCleanDivergence, simulated
	}

	outcome, evidence, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatalf("expected the forced ContentSubsumedOutcome failure to propagate, got outcome=%s, nil error", outcome)
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
	workDestructionNovelPathsFn = func(workdir, target, branch string) ([]string, error) {
		return nil, simulated
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

// TestEvaluateWorkDestruction_EvidenceErrorNeverFoldedIntoBoolean asserts
// the mechanism the plan pins for AC-2(ix)/AC-3/AC-7 consumers: the
// outcome on a probe failure is the NAMED evidence-error variant, never
// silently coerced to DestructionClean/DestructionAncestor or any other
// "safe-looking" value. Table-driven across every seam above.
func TestEvaluateWorkDestruction_EvidenceErrorNeverFoldedIntoBoolean(t *testing.T) {
	dir, branch, target := wdHonestStaleBranchFixture(t)

	origIsAncestor := workDestructionIsAncestorFn
	t.Cleanup(func() { workDestructionIsAncestorFn = origIsAncestor })
	workDestructionIsAncestorFn = func(workdir, ancestor, descendant string) (bool, error) {
		return false, errors.New("boom")
	}

	outcome, _, err := EvaluateWorkDestruction(dir, branch, target)
	if err == nil {
		t.Fatal("expected a non-nil error")
	}
	for _, unsafe := range []guard.DestructionOutcome{
		guard.DestructionClean, guard.DestructionAncestor,
		guard.DestructionSuperseded, guard.DestructionStaleDeletion,
	} {
		if outcome == unsafe {
			t.Fatalf("an evidence-computation error must never be folded into %s", unsafe)
		}
	}
	if outcome != guard.DestructionEvidenceError {
		t.Errorf("outcome = %s, want DestructionEvidenceError", outcome)
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
// file forces above must default to the REAL production symbol.
func TestEvaluateWorkDestruction_SeamsPinnedToRealSymbols(t *testing.T) {
	if reflect.ValueOf(workDestructionIsAncestorFn).Pointer() != reflect.ValueOf(IsAncestor).Pointer() {
		t.Error("workDestructionIsAncestorFn must default to IsAncestor")
	}
	if reflect.ValueOf(workDestructionNetEffectFn).Pointer() != reflect.ValueOf(NetEffectLanded).Pointer() {
		t.Error("workDestructionNetEffectFn must default to NetEffectLanded")
	}
	if reflect.ValueOf(workDestructionSubsumedFn).Pointer() != reflect.ValueOf(ContentSubsumedOutcome).Pointer() {
		t.Error("workDestructionSubsumedFn must default to ContentSubsumedOutcome")
	}
	if reflect.ValueOf(workDestructionPreviewDeletedFn).Pointer() != reflect.ValueOf(PreviewDeletedPaths).Pointer() {
		t.Error("workDestructionPreviewDeletedFn must default to PreviewDeletedPaths")
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
