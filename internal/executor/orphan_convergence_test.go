// Spec 127 bead 4 — AC-3(iii)'s convergence leg (R2(b)'s "normal
// unmerged" outcome: the hint stays exactly `mindspec complete <bead>`
// because an ordinary re-run safely converges). This drives the REAL
// in-process MindspecExecutor.CompleteBead end-to-end, under the fake-bd
// PATH shim (the landed_e2e_test.go:88-104 precedent this bead's own
// plan cites) — never a stub/mock of executor.Executor that fabricates
// the merge (the R6 stub-hollowing this plan's own bead prompt names).
//
// External `_test` package for the SAME reason as landed_e2e_test.go:
// internal/executor imports internal/lifecycle in production
// (mindspec_executor.go), so an internal (package executor) test file
// cannot ALSO import internal/lifecycle without inverting that
// boundary; from here, executor's own unexported binding seams stay
// unreachable, so this test drives the PRODUCTION seam defaults, not a
// stub.
package executor_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// TestOrphanConvergence_NormalUnmergedBeadMergesCleanly is AC-3(iii): a
// bead branch carrying genuinely novel, non-conflicting work — the
// DestructionClean outcome the shared work-destruction predicate maps
// to R2(b)'s "normal unmerged" hint — merges via PRODUCTION CompleteBead
// with no refusal, no conflict, and no destructive side effect:
//
//   - the merge succeeds (no error);
//   - every commit reachable from ANY ref before the merge (main, the
//     spec branch's old tip, the bead branch's tip) is still reachable
//     from the spec branch's NEW tip afterward — nothing the merge
//     landed became unreachable, and nothing pre-existing was lost;
//   - the spec branch's tree gains the bead's novel path and loses none
//     of the paths it carried before (no main/spec-tree regression).
func TestOrphanConvergence_NormalUnmergedBeadMergesCleanly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	installFakeBD(t)

	const beadID = "mindspec-conv1.1"
	const specBranch = "spec/127-conv"
	beadBranch := "bead/" + beadID

	dir := t.TempDir()
	gitE2E(t, dir, "init", "-b", "main")
	gitE2E(t, dir, "config", "user.email", "test@example.com")
	gitE2E(t, dir, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitE2E(t, dir, "add", ".")
	gitE2E(t, dir, "commit", "-m", "root")
	gitE2E(t, dir, "branch", specBranch)
	gitE2E(t, dir, "branch", beadBranch)

	// Spec worktree at the canonical path CompleteBead derives.
	specWt := filepath.Join(dir, ".worktrees", "worktree-spec-127-conv")
	if err := os.MkdirAll(filepath.Dir(specWt), 0o755); err != nil {
		t.Fatal(err)
	}
	gitE2E(t, dir, "worktree", "add", specWt, specBranch)
	t.Cleanup(func() { _, _ = gitE2ERaw(dir, "worktree", "remove", "--force", specWt) })

	// The bead's own novel commit — a NEW path, no overlap with anything
	// the spec branch already carries, so the merge cannot conflict —
	// authored via a temporary worktree removed before CompleteBead runs
	// (the same discipline landed_e2e_test.go uses so the branch survives
	// unchecked-out for CompleteBead's own worktree-removal step).
	beadWt := filepath.Join(dir, ".wt-bead-conv")
	gitE2E(t, dir, "worktree", "add", beadWt, beadBranch)
	if err := os.WriteFile(filepath.Join(beadWt, "deliverable.txt"), []byte("bead work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitE2E(t, beadWt, "add", ".")
	gitE2E(t, beadWt, "commit", "-m", "bead work")
	gitE2E(t, dir, "worktree", "remove", "--force", beadWt)

	beforeMain := gitE2E(t, dir, "rev-parse", "main")
	beforeSpec := gitE2E(t, dir, "rev-parse", specBranch)
	beforeBead := gitE2E(t, dir, "rev-parse", beadBranch)
	specTreeBefore := lsTreeNames(t, dir, specBranch)

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	g := executor.NewMindspecExecutor(dir)
	if err := g.CompleteBead(beadID, specBranch, "", "", false); err != nil {
		t.Fatalf("an ordinary, non-conflicting bead merge must converge with no refusal, got: %v", err)
	}

	afterSpec := gitE2E(t, dir, "rev-parse", specBranch)
	if afterSpec == beforeSpec {
		t.Fatal("the spec branch tip must advance — the merge did not land")
	}

	for name, sha := range map[string]string{"main": beforeMain, "spec (old tip)": beforeSpec, "bead": beforeBead} {
		if !isAncestorE2E(t, dir, sha, afterSpec) {
			t.Errorf("%s's pre-merge tip %s is no longer reachable from the spec branch's post-merge tip %s — the merge lost reachable history", name, sha, afterSpec)
		}
	}

	specTreeAfter := lsTreeNames(t, dir, specBranch)
	for _, p := range specTreeBefore {
		if !containsStr(specTreeAfter, p) {
			t.Errorf("path %q present in the spec branch before the merge is ABSENT after — the merge deleted pre-existing work", p)
		}
	}
	if !containsStr(specTreeAfter, "deliverable.txt") {
		t.Error("the bead's own novel path must be present in the spec branch's tree after the merge")
	}
}

func isAncestorE2E(t *testing.T, dir, ancestor, descendant string) bool {
	t.Helper()
	cmd := execCommandE2E(dir, "merge-base", "--is-ancestor", ancestor, descendant)
	err := cmd.Run()
	if err == nil {
		return true
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false
	}
	t.Fatalf("merge-base --is-ancestor %s %s: %v", ancestor, descendant, err)
	return false
}

func lsTreeNames(t *testing.T, dir, ref string) []string {
	t.Helper()
	out := gitE2E(t, dir, "ls-tree", "-r", "--name-only", ref)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func containsStr(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func execCommandE2E(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	return cmd
}
