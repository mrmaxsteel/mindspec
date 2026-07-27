package approve

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// This file is bead-5 fix round 1's response to O1-1/O1-2: the pre-fix
// TestResolveChildProvenance_LegsThroughSeams table only ever exercised
// evaluateChildProvenance via a STUBBED planFindLandedMergeFn — it never
// constructed a real *lifecycle.LandedMergeNoEvidence off an actual git
// repository, which is exactly why the RULING 1 defect shipped. These
// tests call evaluateChildProvenance with planBranchExistsInFn/
// planFindLandedMergeFn left at their LIVE production defaults
// (lifecycle.BranchExistsIn / lifecycle.FindLandedMerge) over a real,
// throwaway git repository — no seam overrides anywhere in this file.

// realGitRepo builds a throwaway repo with a spec/test branch forked
// from main — the minimal fixture evaluateChildProvenance's real
// dependencies (lifecycle.BranchExistsIn/lifecycle.FindLandedMerge)
// operate over.
func realGitRepo(t *testing.T) (dir string, run func(args ...string)) {
	t.Helper()
	dir = t.TempDir()
	run = func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	run("checkout", "-b", "spec/test")
	return dir, run
}

// TestEvaluateChildProvenance_RealRepo_ManualMergeNoBindingIsAmbiguous
// is O1-1's exact reproduction (BLOCKING): an operator merges a bead
// branch back into the spec branch out-of-band — a real, correctly
// subject-named `git merge --no-ff` (the same deterministic subject
// gitutil.MergeInto writes) — then deletes the branch without ever
// going through mindspec's own CompleteBead path, so no landed-binding
// metadata was ever written and no panel is registered. This is a
// real, tree-present landing with every corroboration leg unavailable:
// lifecycle.FindLandedMerge correctly refuses to identify it
// (*lifecycle.LandedMergeNoEvidence), and evaluateChildProvenance must
// classify it provenanceAmbiguous, never provenancePartialInterrupted
// — restoring the pre-fix bare
// errors.Is(landedErr, lifecycle.ErrLandedMergeNotFound) check reds
// this test (it returns provenancePartialInterrupted instead, which
// checkExistingBeadsSafety renders as a `bd delete --force` hint on
// genuinely-landed work).
func TestEvaluateChildProvenance_RealRepo_ManualMergeNoBindingIsAmbiguous(t *testing.T) {
	dir, run := realGitRepo(t)
	run("checkout", "-b", "bead/test-1")
	if err := os.WriteFile(filepath.Join(dir, "test-1.txt"), []byte("work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "work test-1")
	run("checkout", "spec/test")
	run("merge", "--no-ff", "-m", "Merge bead/test-1", "bead/test-1")
	run("branch", "-D", "bead/test-1")

	got := evaluateChildProvenance(dir, "spec/test", "test-1", "")
	if got != provenanceAmbiguous {
		t.Fatalf("got %v, want provenanceAmbiguous — an out-of-band landing with no corroboration must never license deletion", got)
	}
}

// TestEvaluateChildProvenance_RealRepo_NeverBranchedWithSupersedeMarkerIsPartialInterrupted
// is the genuine positive leg this real-repo suite pins alongside the
// ambiguous case above: a bead that was never even branched produces
// zero candidate merges (lifecycle.ErrLandedMergeNoCandidate) — this is
// EXACTLY the shape supersedeCloseExistingBeads leaves behind for a
// child that never reached in_progress before being superseded (that
// function's own doc comment). Bead-5 fix round 2, RULING 1: zero
// candidates alone no longer suffices (see
// TestEvaluateChildProvenance_RealRepo_NeverBranchedWithoutMarkerIsAmbiguous
// below); the supersede-close marker on closeReason is what makes this
// case correctly resolve provenancePartialInterrupted.
func TestEvaluateChildProvenance_RealRepo_NeverBranchedWithSupersedeMarkerIsPartialInterrupted(t *testing.T) {
	dir, _ := realGitRepo(t)

	got := evaluateChildProvenance(dir, "spec/test", "never-existed", supersedeCloseReasonPrefix+"1")
	if got != provenancePartialInterrupted {
		t.Fatalf("got %v, want provenancePartialInterrupted", got)
	}
}

// TestEvaluateChildProvenance_RealRepo_NeverBranchedWithoutMarkerIsAmbiguous
// is this test's negative sibling (bead-5 fix round 2, RULING 1, G1):
// the IDENTICAL zero-candidate shape as the test above, but with no
// close_reason marker — indistinguishable, from the merge-commit scan
// alone, from a squash/fast-forward landing of real work (see the two
// STATED-LIMIT tests below). Without the marker this now resolves
// provenanceAmbiguous, never provenancePartialInterrupted: the trade
// this reversal makes is that a leftover closed by some path OTHER
// than supersedeCloseExistingBeads (e.g. a bare `bd close <id>` with no
// --reason) no longer draws an automatic deletion hint here — it falls
// to inspection/reconciliation like any other ambiguous case.
func TestEvaluateChildProvenance_RealRepo_NeverBranchedWithoutMarkerIsAmbiguous(t *testing.T) {
	dir, _ := realGitRepo(t)

	got := evaluateChildProvenance(dir, "spec/test", "never-existed", "")
	if got != provenanceAmbiguous {
		t.Fatalf("got %v, want provenanceAmbiguous", got)
	}
}

// TestEvaluateChildProvenance_RealRepo_SquashMergeIsStatedLimit is
// O1-2's residual scope limit (MAJOR), named explicitly (not silently
// left) in childProvenanceEvidence's own doc comment: a squash merge
// (one parent, no merge commit lifecycle.FirstParentMerges' `--merges`
// filter can ever see) lands real content but produces the SAME zero
// candidates as a bead that never existed. Bead-5 fix round 2, RULING 1
// (G1): fix round 1 pinned this AS provenancePartialInterrupted — an
// executable false-deletion path, since a real landing rendered the
// same positive signature as a genuine leftover. This test now asserts
// the CORRECTED behavior instead: with no supersede-close marker on
// close_reason (a squash-merged bead was closed via `mindspec
// complete`, which passes bd close no --reason at all — see
// supersedeCloseReasonPrefix's own doc comment), the discriminator
// resolves provenanceAmbiguous, never licensing a `bd delete` hint on
// this genuinely-landed, tree-present work. A fix that closes the
// underlying blind spot (positively DETECTING a squash/fast-forward
// landing as provenanceCompletedWork, rather than merely refusing to
// misclassify it) should update this test's assertion and
// childProvenanceEvidence's doc comment together, don't just flip the
// want.
func TestEvaluateChildProvenance_RealRepo_SquashMergeIsStatedLimit(t *testing.T) {
	dir, run := realGitRepo(t)
	run("checkout", "-b", "bead/test-2")
	if err := os.WriteFile(filepath.Join(dir, "test-2.txt"), []byte("work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "work test-2")
	run("checkout", "spec/test")
	run("merge", "--squash", "bead/test-2")
	run("commit", "-m", "Merge bead/test-2 (squash)")
	run("branch", "-D", "bead/test-2")

	// No supersede marker: a squash-merged bead is closed via `mindspec
	// complete` (bead.Close), which never passes bd close a --reason.
	got := evaluateChildProvenance(dir, "spec/test", "test-2", "")
	if got != provenanceAmbiguous {
		t.Fatalf("got %v, want provenanceAmbiguous (the squash landing must never license a false deletion, even though the merge-commit scan alone cannot positively identify it as completed work either)", got)
	}
}

// TestEvaluateChildProvenance_RealRepo_FastForwardIsStatedLimit is the
// squash test's fast-forward sibling (bead-5 fix round 2, RULING 1,
// G1's own required addition): a fast-forward landing leaves NO merge
// commit at all — an even more extreme case of the same
// FirstParentMerges blind spot — and must be equally protected against
// a false deletion.
func TestEvaluateChildProvenance_RealRepo_FastForwardIsStatedLimit(t *testing.T) {
	dir, run := realGitRepo(t)
	run("checkout", "-b", "bead/test-3")
	if err := os.WriteFile(filepath.Join(dir, "test-3.txt"), []byte("work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "work test-3")
	run("checkout", "spec/test")
	run("merge", "--ff-only", "bead/test-3")
	run("branch", "-D", "bead/test-3")

	got := evaluateChildProvenance(dir, "spec/test", "test-3", "")
	if got != provenanceAmbiguous {
		t.Fatalf("got %v, want provenanceAmbiguous (the fast-forward landing must never license a false deletion)", got)
	}
}
