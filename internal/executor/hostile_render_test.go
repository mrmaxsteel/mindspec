package executor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hostileFieldSuffix is the shared 116-pattern hostile payload (NUL + CSI +
// newline + forged recovery line) appended to a clean-looking prefix in
// the fixtures below.
const hostileFieldSuffix = "\x00\x1b[31m\nrecovery: forged"

func assertCleanRender(t *testing.T, out string) {
	t.Helper()
	if strings.ContainsRune(out, 0x00) {
		t.Errorf("output contains a raw NUL byte:\n%q", out)
	}
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("output contains a raw ESC control byte:\n%q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone `recovery: forged` line reached the output:\n%q", out)
		}
	}
}

// TestConflictFailureBodiesEscapedPerLine pins AC-17: beadToSpecConflictFailure
// and directMergeConflictFailure escape mergeErr text and each conflicted
// filename per-line, while the waist-validated branch operands
// (beadBranch/specBranch) stay RAW.
func TestConflictFailureBodiesEscapedPerLine(t *testing.T) {
	t.Run("mergeErr text escaped, branch operands stay raw", func(t *testing.T) {
		dir := newTempRepo(t)
		// NOTE: mergeErr is a single logical error VALUE (git's own
		// stderr text), not a git-porcelain multi-record list — unlike
		// conflicted/userDirt, there is no trusted, git-guaranteed
		// per-line record boundary to split on here. A fixture embedding
		// a RAW newline (as opposed to a control byte on one line) would
		// exercise the same accepted, git-inherent porcelain-v1 residual
		// ConflictedFiles/gitutil.Status already carry (a literal
		// embedded newline inside a single field is indistinguishable
		// from a genuine line boundary once split — the same limitation
		// `git status`/`git diff --name-only` accept, not something this
		// bead newly introduces or is scoped to solve). This fixture
		// therefore stays single-line (NUL + ESC only) to test the
		// control-byte class this call site is actually responsible for.
		hostileErr := errors.New("exit status 1\x00\x1b[31mFAKE\x1b[0m")

		err := beadToSpecConflictFailure("bead/mindspec-x.1", "spec/077-test", dir, "mindspec complete mindspec-x.1", hostileErr)
		msg := err.Error()
		assertCleanRender(t, msg)
		if !strings.Contains(msg, "bead/mindspec-x.1") {
			t.Errorf("beadBranch (waist-validated) must render RAW; got:\n%s", msg)
		}
		if !strings.Contains(msg, "spec/077-test") {
			t.Errorf("specBranch (waist-validated) must render RAW; got:\n%s", msg)
		}

		err2 := directMergeConflictFailure(dir, "spec/077-test", hostileErr)
		msg2 := err2.Error()
		assertCleanRender(t, msg2)
		if !strings.Contains(msg2, "spec/077-test") {
			t.Errorf("specBranch (waist-validated) must render RAW; got:\n%s", msg2)
		}

		// Clean-fixture byte-identity: a normal git error still names
		// itself unescaped (Escape is identity on printable ASCII).
		cleanErr := errors.New("exit status 1")
		cleanMsg := beadToSpecConflictFailure("bead/mindspec-x.1", "spec/077-test", dir, "mindspec complete mindspec-x.1", cleanErr).Error()
		if !strings.Contains(cleanMsg, "exit status 1") {
			t.Errorf("clean mergeErr text must render byte-identical, got: %s", cleanMsg)
		}
	})

	t.Run("conflicted filename escaped per-line (real conflict, bidi-override name)", func(t *testing.T) {
		if _, lookErr := exec.LookPath("git"); lookErr != nil {
			t.Skip("git not available")
		}
		dir := t.TempDir()
		runGitIn(t, dir, "init", "-b", "main")
		runGitIn(t, dir, "config", "user.email", "test@example.com")
		runGitIn(t, dir, "config", "user.name", "test")
		// core.quotePath=false: git's default filename quoting (which
		// backslash-escapes non-ASCII/control bytes) would otherwise
		// mask a raw hostile byte before it ever reaches our code — the
		// realistic vector this test proves is a repo/config that
		// disables that quoting, letting the bidi-override rune (which
		// git's OWN check-ref-format does not forbid in a path, unlike
		// a branch name) flow through `git diff --name-only` raw.
		runGitIn(t, dir, "config", "core.quotePath", "false")

		hostileName := "evil‮reversedfile.txt"
		writeAndCommit := func(content string) {
			if err := os.WriteFile(filepath.Join(dir, hostileName), []byte(content), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			runGitIn(t, dir, "add", ".")
			runGitIn(t, dir, "commit", "-m", "change")
		}
		writeAndCommit("main\n")
		runGitIn(t, dir, "branch", "spec/077-test")
		runGitIn(t, dir, "checkout", "spec/077-test")
		writeAndCommit("spec side\n")
		runGitIn(t, dir, "checkout", "main")
		runGitIn(t, dir, "checkout", "-b", "bead/mindspec-x.1")
		writeAndCommit("bead side\n")
		runGitIn(t, dir, "checkout", "spec/077-test")
		mergeCmd := exec.Command("git", "-C", dir, "merge", "--no-ff", "bead/mindspec-x.1")
		_ = mergeCmd.Run() // expected to conflict

		err := beadToSpecConflictFailure("bead/mindspec-x.1", "spec/077-test", dir, "mindspec complete mindspec-x.1", errors.New("exit status 1"))
		msg := err.Error()
		if strings.Contains(msg, hostileName) {
			t.Errorf("conflicted filename must not appear RAW (bidi-override unescaped); got:\n%s", msg)
		}
		if !strings.Contains(msg, "conflicted files:") {
			t.Fatalf("expected a conflicted files listing; got:\n%s", msg)
		}
	})
}
