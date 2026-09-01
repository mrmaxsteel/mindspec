package executor

// Spec 127 final review (mindspec-6f5p): CompleteBead's anti-data-loss
// ancestry check and its fail-closed landed-binding write are BOTH gated on
// a single bead-branch presence probe. That probe used to be the cwd-scoped
// gitutil.BranchExists, while CompleteBead's production caller
// (internal/complete.Run) does not os.Chdir(root) until AFTER CompleteBead
// returns — so an invocation standing anywhere outside this repository
// answered "branch absent" and silently skipped BOTH guarded legs.
//
// Every other executor fixture is built on newRepoExecutor, which chdirs
// INTO the repo precisely to reproduce the "callers run from the repo root"
// invariant — which is exactly why no existing test could see the skip.
// These two tests are the deliberate inverse: they run CompleteBead from a
// directory that is not inside any git repository, so they FAIL if the
// probe ever reverts to cwd scope.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

// chdirOutsideAnyRepo moves the process into a fresh temp directory that is
// not inside any git repository, for the duration of the test. This models
// the production cwd at the moment internal/complete.Run calls
// exec.CompleteBead — which is wherever the operator invoked `mindspec
// complete` from, NOT necessarily g.Root.
func chdirOutsideAnyRepo(t *testing.T) string {
	t.Helper()
	outside := t.TempDir()
	// Defensive: a temp dir nested inside a repository would silently
	// restore the very cwd resolution these tests exist to defeat.
	if err := exec.Command("git", "-C", outside, "rev-parse", "--git-dir").Run(); err == nil {
		t.Skipf("temp dir %s is inside a git repository; cannot model an outside-any-repo cwd", outside)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(outside); err != nil {
		t.Fatalf("chdir %s: %v", outside, err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return outside
}

// TestCompleteBead_UnmergedBranchStillRefusesFromOutsideTheRepo pins the
// ancestry leg. The bead branch carries a commit that is NOT on the spec
// branch and there is no spec worktree (so CompleteBead's own merge leg is
// skipped) — the ancestry check is therefore the ONLY thing standing
// between this call and the deletion of unmerged work.
//
// With a cwd-scoped presence probe this call returns nil and DELETES
// bead/mindspec-scope.1 along with its commit.
func TestCompleteBead_UnmergedBranchStillRefusesFromOutsideTheRepo(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	runGitIn(t, dir, "branch", "spec/127-scope")
	runGitIn(t, dir, "checkout", "-b", "bead/mindspec-scope.1")
	if err := os.WriteFile(filepath.Join(dir, "bead-work.txt"), []byte("unmerged\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "bead work that has NOT been merged")
	runGitIn(t, dir, "checkout", "main")

	// No spec worktree is created, so CompleteBead skips its merge leg
	// entirely and falls straight through to the ancestry guard.
	fake.listEntries = nil

	chdirOutsideAnyRepo(t)

	err := g.CompleteBead("mindspec-scope.1", "spec/127-scope", "", "", false)
	if err == nil {
		t.Fatal("CompleteBead returned nil for an UNMERGED bead branch — the ancestry guard was skipped, which is the data-loss outcome it exists to prevent (presence probe resolved against the process cwd instead of g.Root?)")
	}
	if !strings.Contains(err.Error(), "is NOT merged into") {
		t.Fatalf("expected the anti-data-loss ancestry refusal, got: %v", err)
	}

	// The unmerged branch must survive the refusal.
	if out, gitErr := exec.Command("git", "-C", dir, "rev-parse", "--verify", "refs/heads/bead/mindspec-scope.1").CombinedOutput(); gitErr != nil {
		t.Fatalf("the unmerged bead branch was deleted despite the refusal: %v (%s)", gitErr, strings.TrimSpace(string(out)))
	}
}

// TestCompleteBead_LandedBindingIsRecordedFromOutsideTheRepo pins the other
// leg gated by the same probe: the merge-time landed-merge binding (spec
// 121 R5(b), ADR-0041 §2(ii)) — the third durable datum
// internal/lifecycle.FindLandedMerge consults once the bead's branch and
// worktree are gone.
//
// With a cwd-scoped presence probe the merge still lands, cleanup still
// runs, CompleteBead still returns nil — and no binding is ever written,
// silently.
func TestCompleteBead_LandedBindingIsRecordedFromOutsideTheRepo(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	runGitIn(t, dir, "branch", "spec/127-bindscope")
	runGitIn(t, dir, "branch", "bead/mindspec-bindscope.1")

	beadWtDir := filepath.Join(dir, ".wt-bead-bindscope-1")
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-bindscope.1")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run() })
	if err := os.WriteFile(filepath.Join(beadWtDir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitIn(t, beadWtDir, "add", ".")
	runGitIn(t, beadWtDir, "commit", "-m", "bead work")

	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-127-bindscope")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/127-bindscope")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-bindscope.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-bindscope.1",
	}}
	fake.onRemove = func(name string) {
		_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
	}

	var bound []map[string]interface{}
	mergeBindingFn = func(id string, updates map[string]interface{}) error {
		if id != "mindspec-bindscope.1" {
			t.Errorf("mergeBindingFn called for %q, want mindspec-bindscope.1", id)
		}
		bound = append(bound, updates)
		return nil
	}

	chdirOutsideAnyRepo(t)

	if err := g.CompleteBead("mindspec-bindscope.1", "spec/127-bindscope", "", "", false); err != nil {
		t.Fatalf("CompleteBead: %v", err)
	}

	if len(bound) == 0 {
		t.Fatal("no landed-merge binding was recorded — the binding leg was skipped after a REAL merge landed (presence probe resolved against the process cwd instead of g.Root?)")
	}
	sha, _ := bound[0]["mindspec_landed_merge_sha"].(string)
	sp, _ := bound[0]["mindspec_landed_second_parent"].(string)
	if strings.TrimSpace(sha) == "" || strings.TrimSpace(sp) == "" {
		t.Fatalf("binding recorded with an empty merge SHA or second parent: %#v", bound[0])
	}
}
