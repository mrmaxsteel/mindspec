// Spec 127 bead-6 fix round 1: AC-9(v) leg beta (S1-1/O3-3/G1-3's
// ac9-beta-gamma ruling) — the --resolve-merge resumption's completing
// merge must remain identifiable by lifecycle.FindLandedMerge (spec 125's
// ONLY admissible corroboration once a bead's branch and worktree are
// gone) even after the branch is deleted, and its own subject must be a
// parseMergeSubjectBeadBranch-recognized form naming the bead branch —
// not merely inferred from CompleteBead's own cleanup gate succeeding
// (gitutil.IsAncestor, which is completely subject-independent).
//
// External `_test` package for the SAME reason landed_e2e_test.go is:
// driving PRODUCTION CompleteBead end-to-end against a real bd-on-PATH
// shim, then through internal/lifecycle.FindLandedMerge — proving the
// cross-package contract, not a stubbed approximation of it.
package executor_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// TestLandedE2E_ResolveMergeResumptionPreservesSubjectIdentity drives a
// REAL add/add conflict through production CompleteBead's --resolve-merge
// resumption (never a manual abort+redo, unlike
// TestLandedE2E_ConflictRecoveryBindsAndFindLandedMergeIdentifies above,
// which deliberately reproduces the pre-fix DEFAULT-subject miss shape).
// This proves the NEW re-entry surface's own identity leg: the seeded
// subject (`Merge bead/<id>`) MergeInto's own `-m` seeds survives
// `git commit --no-edit` unchanged, and FindLandedMerge resolves the
// merge by that subject once the branch is deleted.
func TestLandedE2E_ResolveMergeResumptionPreservesSubjectIdentity(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	installFakeBD(t)

	const beadID = "mindspec-e2ebeta.1"
	const specBranch = "spec/125-e2ebeta"
	beadBranch := "bead/" + beadID

	dir := t.TempDir()
	gitE2E(t, dir, "init", "-b", "main")
	gitE2E(t, dir, "config", "user.email", "test@example.com")
	gitE2E(t, dir, "config", "user.name", "test")
	gitE2E(t, dir, "commit", "--allow-empty", "-m", "root")
	gitE2E(t, dir, "branch", specBranch)
	gitE2E(t, dir, "branch", beadBranch)

	specWt := filepath.Join(dir, ".worktrees", "worktree-spec-125-e2ebeta")
	if err := os.MkdirAll(filepath.Dir(specWt), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	gitE2E(t, dir, "worktree", "add", specWt, specBranch)
	t.Cleanup(func() { _, _ = gitE2ERaw(dir, "worktree", "remove", "--force", specWt) })
	if err := os.WriteFile(filepath.Join(specWt, "c.txt"), []byte("spec side\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitE2E(t, specWt, "add", ".")
	gitE2E(t, specWt, "commit", "-m", "spec change")

	beadWt := filepath.Join(dir, ".wt-bead-e2ebeta")
	gitE2E(t, dir, "worktree", "add", beadWt, beadBranch)
	if err := os.WriteFile(filepath.Join(beadWt, "c.txt"), []byte("bead side\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitE2E(t, beadWt, "add", ".")
	gitE2E(t, beadWt, "commit", "-m", "bead work")
	gitE2E(t, dir, "worktree", "remove", "--force", beadWt)

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	g := executor.NewMindspecExecutor(dir)

	// 1. First attempt: conflicts, preserved (never a raw `git merge`
	// line — the printed recovery names --resolve-merge).
	if err := g.CompleteBead(beadID, specBranch, "", "", false); err == nil {
		t.Fatal("expected the bead→spec add/add conflict to refuse")
	}

	// 2. Resolve + stage the PRESERVED merge directly (no fresh `git
	// merge`, no abort — the conflict MergeInto's own "Merge <beadBranch>"
	// subject started is still in place).
	if err := os.WriteFile(filepath.Join(specWt, "c.txt"), []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	gitE2E(t, specWt, "add", "c.txt")

	beadTip := gitE2E(t, dir, "rev-parse", beadBranch)

	// 3. --resolve-merge completes the merge, preserving MergeInto's own
	// seeded subject through `git commit --no-edit`.
	if err := g.CompleteBead(beadID, specBranch, "", "", true); err != nil {
		t.Fatalf("--resolve-merge must complete the resolved merge, got: %v", err)
	}

	mergeSHA := gitE2E(t, dir, "rev-parse", specBranch)
	subject := gitE2E(t, dir, "log", "--format=%s", "-1", mergeSHA)
	wantSubject := "Merge " + beadBranch
	if subject != wantSubject {
		t.Fatalf("the completed merge's subject must be the seeded, identity-load-bearing form; got %q, want %q", subject, wantSubject)
	}

	// Branch (and worktree) deleted — FindLandedMerge's subject leg is
	// now the ONLY way to identify this merge by name.
	if _, refErr := gitE2ERaw(dir, "rev-parse", "--verify", "refs/heads/"+beadBranch); refErr == nil {
		t.Fatal("the bead branch must be deleted after the resumed merge completes and cleanup runs")
	}

	landed, err := lifecycle.FindLandedMerge(dir, specBranch, beadID)
	if err != nil {
		t.Fatalf("AC-9(v)(beta): FindLandedMerge must identify the RESUMED merge by its preserved subject after branch deletion, got: %v", err)
	}
	if landed.SHA != mergeSHA {
		t.Errorf("LandedMerge.SHA = %q, want %q", landed.SHA, mergeSHA)
	}
	if landed.SecondParent != beadTip {
		t.Errorf("LandedMerge.SecondParent = %q, want %q", landed.SecondParent, beadTip)
	}

	if !strings.Contains(subject, strings.TrimPrefix(beadBranch, "bead/")) {
		t.Errorf("fixture sanity: subject %q should name the bead branch", subject)
	}
}
