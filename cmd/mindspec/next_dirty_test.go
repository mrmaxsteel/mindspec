package main

// Bead 9 punch-list M6 (Bead 7 panel, surviving mutant): cmd-layer pin
// for `mindspec next`'s dirty-tree path. internal/next's unit tests only
// see the DirtyTreeFailure MESSAGE — reverting the old direct-to-stderr
// "Recovery steps: 1..3" block in cmd/mindspec/next.go ALONGSIDE the new
// guard failure (double emission) would survive them. This subprocess
// test runs the real binary on a real dirty repo and asserts the old
// block (including its destructive `git restore .` advice) is ABSENT
// from the command's output, while the Req 12 guard failure is present.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runGitInDir runs git with args in dir, with a pinned test identity so
// commits work without host config.
func runGitInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-c", "user.email=test@example.invalid",
		"-c", "user.name=mindspec-test",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestNextCmd_DirtyTree_NoLegacyRecoveryStepsBlock(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	bin := buildMindspecBinary(t)

	// A workspace with committed baseline content, then user dirt.
	dir := t.TempDir()
	runGitInDir(t, dir, "init", "-b", "main")
	if err := os.MkdirAll(filepath.Join(dir, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("clean baseline\n"), 0o644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}
	runGitInDir(t, dir, "add", "-A")
	runGitInDir(t, dir, "commit", "-m", "baseline")
	// The user's uncommitted dirt — what the old block advised discarding.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("user WIP\n"), 0o644); err != nil {
		t.Fatalf("dirty notes.txt: %v", err)
	}

	cmd := exec.Command(bin, "next")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	combined := stdout.String() + stderr.String()

	if err == nil {
		t.Fatalf("mindspec next must fail on a dirty tree; output:\n%s", combined)
	}

	// The new guard failure is emitted (Reqs 8/12)...
	if !strings.Contains(combined, "cannot claim work") {
		t.Errorf("output missing the dirty-tree guard failure; got:\n%s", combined)
	}
	if !strings.Contains(combined, "recovery: ") {
		t.Errorf("output missing a `recovery:` line (Req 12); got:\n%s", combined)
	}
	if !strings.Contains(combined, "you are in the ") {
		t.Errorf("output missing the worktree-context line (Req 8); got:\n%s", combined)
	}

	// ...and the pre-092 stderr block is GONE — not emitted alongside
	// (the M6 double-emission mutant) or instead of the guard failure.
	for _, banned := range []string{"Recovery steps:", "git restore ."} {
		if strings.Contains(combined, banned) {
			t.Errorf("output still contains the legacy %q advice (M6 double-emission); got:\n%s", banned, combined)
		}
	}
}
