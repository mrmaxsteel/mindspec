package main

// Spec 092 Bead 4 (mindspec-qxsy): unit coverage for the AC "qxsy unit
// (impl approve)" — invocation from inside the spec worktree ends with
// the process cwd at root, the cd-back NOTE as the last line of stdout,
// and a nil error from the command tail (exit 0). The withWorkingDir
// restore-failure half of the same AC lives in
// internal/executor/executor_test.go
// (TestWithWorkingDir_RestoreFailureRemainsAtDirAndWarns).

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCdBackNote_InvocationFromRemovedSpecWorktree simulates the exact
// `impl approve` sequence the cmd layer performs when invoked from
// inside the spec worktree FinalizeEpic removes:
//
//  1. capture the invocation cwd at entry (before any auto-chdir),
//  2. the terminal mutation removes the spec worktree,
//  3. the cmd layer chdirs to root (Req 3b) before tail output,
//  4. the cd-back NOTE is the LAST line of stdout (Req 4).
func TestCdBackNote_InvocationFromRemovedSpecWorktree(t *testing.T) {
	root := t.TempDir()
	specWt := filepath.Join(root, ".worktrees", "worktree-spec-091-x")
	if err := os.MkdirAll(specWt, 0o755); err != nil {
		t.Fatalf("mkdir spec worktree: %v", err)
	}

	origWd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origWd) })
	if err := os.Chdir(specWt); err != nil {
		t.Fatalf("chdir into spec worktree: %v", err)
	}

	// (1) Entry: capture before any auto-chdir.
	invocationCwd := captureInvocationCwd()
	if invocationCwd == "" {
		t.Fatal("captureInvocationCwd returned empty for a valid cwd")
	}

	// (2) Terminal mutation: FinalizeEpic removes the spec worktree.
	if err := os.RemoveAll(specWt); err != nil {
		t.Fatalf("removing spec worktree: %v", err)
	}

	// (3) Req 3b: the cmd layer moves to root so all tail output runs
	// from a valid cwd and the command can return nil (exit 0).
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir to root: %v", err)
	}

	// (4) Tail output, then the NOTE as the final stdout write.
	var stdout bytes.Buffer
	fmt.Fprintln(&stdout, "Implementation for 091-x approved. Mode: idle.")
	fmt.Fprintln(&stdout, "instruct tail output")
	emitCdBackNote(&stdout, invocationCwd, root)

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	last := lines[len(lines)-1]
	want := "NOTE: your shell's working directory was removed — run: cd " + root
	if last != want {
		t.Errorf("last stdout line:\n  got  %q\n  want %q", last, want)
	}

	// Process cwd is root.
	wd, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatalf("process ended in unresolvable cwd: %v", wdErr)
	}
	realWd, _ := filepath.EvalSymlinks(wd)
	realRoot, _ := filepath.EvalSymlinks(root)
	if realWd != realRoot {
		t.Errorf("process cwd: got %q, want root %q", wd, root)
	}
}

// TestEmitCdBackNote_SilentWhileInvocationCwdExists: the NOTE is a
// removed-directory recovery channel, not noise on the happy path.
func TestEmitCdBackNote_SilentWhileInvocationCwdExists(t *testing.T) {
	root := t.TempDir()
	alive := filepath.Join(root, "alive")
	if err := os.MkdirAll(alive, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var stdout bytes.Buffer
	emitCdBackNote(&stdout, alive, root)
	if stdout.Len() != 0 {
		t.Errorf("NOTE emitted while invocation cwd exists: %q", stdout.String())
	}
}

// TestEmitCdBackNote_NoOpWithoutCapturedCwd: an empty capture (cwd was
// unresolvable at entry) disables the NOTE instead of pointing the
// agent at a stat of "".
func TestEmitCdBackNote_NoOpWithoutCapturedCwd(t *testing.T) {
	var stdout bytes.Buffer
	emitCdBackNote(&stdout, "", "/some/root")
	if stdout.Len() != 0 {
		t.Errorf("NOTE emitted for empty invocation cwd: %q", stdout.String())
	}
}

// TestCaptureInvocationCwd_DeletedCwdDegradesGracefully: when the
// process starts in an already-deleted directory, the capture+NOTE
// pipeline must never error or emit garbage. Platform behavior differs
// — Linux getcwd fails (capture returns "", NOTE disabled) while macOS
// returns the stale path (capture non-empty, stat fails, NOTE emitted)
// — so the invariant is: the output is either empty or exactly the
// cd-back NOTE pointing at root.
func TestCaptureInvocationCwd_DeletedCwdDegradesGracefully(t *testing.T) {
	base := t.TempDir()
	doomed := filepath.Join(base, "doomed")
	if err := os.MkdirAll(doomed, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	origWd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origWd) })
	if err := os.Chdir(doomed); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.RemoveAll(doomed); err != nil {
		t.Fatalf("remove cwd: %v", err)
	}

	got := captureInvocationCwd()

	root := base
	var stdout bytes.Buffer
	emitCdBackNote(&stdout, got, root)

	out := stdout.String()
	want := "NOTE: your shell's working directory was removed — run: cd " + root + "\n"
	switch {
	case got == "" && out != "":
		t.Errorf("empty capture must disable the NOTE; got %q", out)
	case got != "" && out != want:
		t.Errorf("non-empty capture of a deleted cwd must emit the NOTE:\n  got  %q\n  want %q", out, want)
	}
}
