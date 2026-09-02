package gitutil

// localbranchnames_test.go: spec 128 Bead 3 round-3 panel (ADV-1
// remainder) — pins LocalBranchNames' disambiguation-safety, the property
// LocalBranchRefs's %(refname:short) lacks.

import (
	"os/exec"
	"testing"
)

// TestLocalBranchNames_RealGit_SurvivesShadowTag is the direct-primitive
// regression: a same-named tag must not make LocalBranchNames drop or
// rename the branch, unlike LocalBranchRefs under the identical fixture.
func TestLocalBranchNames_RealGit_SurvivesShadowTag(t *testing.T) {
	dir := initGitRepo(t)
	restore := swapRealExecCommand(t)
	defer restore()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("branch", "spec/128-foo")

	names, err := LocalBranchNames(dir)
	if err != nil {
		t.Fatalf("LocalBranchNames() error: %v", err)
	}
	if !containsStr(names, "spec/128-foo") {
		t.Fatalf("LocalBranchNames() = %v, want to contain %q", names, "spec/128-foo")
	}

	// The shadow: a tag with the SAME name as the branch.
	runGit("tag", "spec/128-foo", "main")

	// LocalBranchRefs (the existing, %(refname:short)-based helper) is
	// disambiguated by git under the shadow — this is the bug
	// LocalBranchNames exists to avoid. Documented here as a control,
	// not a claim about LocalBranchRefs being wrong for ITS callers.
	refs, err := LocalBranchRefs(dir)
	if err != nil {
		t.Fatalf("LocalBranchRefs() error: %v", err)
	}
	if containsStr(refs, "spec/128-foo") {
		t.Skip("this git version's :short did not disambiguate under the shadow tag as expected — the control assumption does not hold here, skipping rather than asserting a version-specific git quirk")
	}

	names, err = LocalBranchNames(dir)
	if err != nil {
		t.Fatalf("LocalBranchNames() error: %v", err)
	}
	if !containsStr(names, "spec/128-foo") {
		t.Fatalf("LocalBranchNames() = %v, want still to contain %q under the shadow tag — a same-named tag must never hide a real branch", names, "spec/128-foo")
	}
	for _, n := range names {
		if n == "heads/spec/128-foo" {
			t.Fatalf("LocalBranchNames() emitted the disambiguated form %q instead of the true branch name", n)
		}
	}
}

func containsStr(items []string, want string) bool {
	for _, it := range items {
		if it == want {
			return true
		}
	}
	return false
}
