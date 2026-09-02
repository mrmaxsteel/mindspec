package gitutil

// fileexists_test.go: spec 128 Bead 3 round-1 panel G3 (MAJOR) — pins
// FileExistsAtRef, the tree-scoped probe that lets the OQ-2 draft
// discovery see a spec whose spec.md was committed only to the spec/<id>
// branch, never checked out into the invoking checkout's own working
// tree (the real shape `spec create` produces).

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestFileExistsAtRef_RunsAtExplicitWorkdir pins the `-C workdir cat-file
// -e <ref>:<path>` argv shape and the exit-0/non-zero dispatch.
func TestFileExistsAtRef_RunsAtExplicitWorkdir(t *testing.T) {
	calls := swapExec(t, "", 0)
	exists, err := FileExistsAtRef("/tmp/some-root", "main", ".mindspec/specs/foo/spec.md")
	if err != nil {
		t.Fatalf("FileExistsAtRef: unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("FileExistsAtRef: expected true on a zero-exit cat-file -e")
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	assertArgs(t, (*calls)[0].args, "-C", "/tmp/some-root", "cat-file", "-e", "main:.mindspec/specs/foo/spec.md")

	// git cat-file -e's exit code does not distinguish "path absent" from
	// "ref absent" or "malformed ref" (all exit 128, verified
	// empirically) — this helper treats ANY non-zero exit as absence,
	// which is only safe once the caller has independently confirmed the
	// ref exists (e.g. via BranchExistsIn).
	swapExec(t, "fatal: path 'nope.txt' does not exist in 'main'", 128)
	exists, err = FileExistsAtRef("/tmp/some-root", "main", "nope.txt")
	if err != nil {
		t.Fatalf("FileExistsAtRef: unexpected error on a non-zero exit: %v", err)
	}
	if exists {
		t.Fatal("FileExistsAtRef: expected false on a non-zero exit")
	}
}

// TestFileExistsAtRef_RejectsOptionLikeArgs mirrors BranchExistsIn's own
// SEC-5 hygiene for both operands.
func TestFileExistsAtRef_RejectsOptionLikeArgs(t *testing.T) {
	calls := swapExec(t, "", 0)
	if _, err := FileExistsAtRef("/tmp/some-root", "--upload-pack=x", "path"); err == nil {
		t.Fatal("FileExistsAtRef: expected an error on an option-like ref")
	}
	if _, err := FileExistsAtRef("/tmp/some-root", "main", "--evil"); err == nil {
		t.Fatal("FileExistsAtRef: expected an error on an option-like path")
	}
	if len(*calls) != 0 {
		t.Fatalf("expected no git invocation on a rejected operand, got %d", len(*calls))
	}
}

// TestFileExistsAtRef_NonExitErrorPropagates is G1-B3-04's discipline
// applied here: a failure that is NOT even a completed process run (git
// itself unrunnable) must be a genuine ERROR, never folded into absence.
func TestFileExistsAtRef_NonExitErrorPropagates(t *testing.T) {
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command(filepath.Join(t.TempDir(), "definitely-not-a-real-git-binary"), args...)
	}
	exists, err := FileExistsAtRef("/tmp/some-root", "main", "spec.md")
	if err == nil {
		t.Fatal("FileExistsAtRef: expected an error when the git binary itself cannot run")
	}
	if exists {
		t.Fatal("FileExistsAtRef: a structural failure must never report true")
	}
}

// TestFileExistsAtRef_RealGit is the real-git regression fixture (mirroring
// TestBranchExistsIn_RealGitIsWorkdirScoped's real-git-not-mock
// discipline): a genuinely committed file is found via its branch's own
// tree, and a genuinely absent path is not.
func TestFileExistsAtRef_RealGit(t *testing.T) {
	dir := initGitRepo(t)
	restore := swapRealExecCommand(t)
	defer restore()

	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# draft\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("add", "-A")
	runGit("commit", "-m", "add spec.md")

	if exists, err := FileExistsAtRef(dir, "main", "spec.md"); err != nil || !exists {
		t.Errorf("FileExistsAtRef must find a real committed file: exists=%v err=%v", exists, err)
	}
	if exists, err := FileExistsAtRef(dir, "main", "does-not-exist.txt"); err != nil || exists {
		t.Errorf("FileExistsAtRef must not find a nonexistent path: exists=%v err=%v", exists, err)
	}
}
