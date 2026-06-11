package main

// Spec 092 Bead 4 (mindspec-qxsy): coverage for the AC "qxsy unit
// (impl approve)" — invocation from inside the spec worktree ends with
// the process cwd at root, the cd-back NOTE as the last line of stdout,
// and a nil error (exit 0).
//
// Panel R3-1/R3-2: the tests below call the PRODUCTION tail functions
// (implApproveTail / completeTail — the exact code the RunE handlers
// run) rather than simulating their sequence, so deleting the chdir or
// the NOTE call from the production path, or moving the NOTE before the
// instruct tail, fails these tests. The withWorkingDir restore-failure
// half of the impl-approve AC lives in
// internal/executor/executor_test.go.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// chdirIntoDoomed creates root/<rel>, chdirs the process into it, and
// returns its path plus the captured invocation cwd — the entry state
// of a terminal command run from inside a worktree.
func chdirIntoDoomed(t *testing.T, root, rel string) (doomed, invocationCwd string) {
	t.Helper()
	doomed = filepath.Join(root, rel)
	if err := os.MkdirAll(doomed, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", doomed, err)
	}
	origWd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origWd) })
	if err := os.Chdir(doomed); err != nil {
		t.Fatalf("chdir into %s: %v", doomed, err)
	}
	invocationCwd = captureInvocationCwd()
	if invocationCwd == "" {
		t.Fatal("captureInvocationCwd returned empty for a valid cwd")
	}
	return doomed, invocationCwd
}

func assertCwdIsRoot(t *testing.T, root string) {
	t.Helper()
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

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[len(lines)-1]
}

// TestImplApproveTail_SuccessFromRemovedSpecWorktree exercises the
// production `impl approve` tail with the terminal mutation having
// removed the invocation directory (the spec worktree):
//   - the Req 3b chdir leaves the process at root (kills panel
//     mutation 1b — delete the chdir and the cwd stays deleted),
//   - the Req 4 NOTE is the LAST line of stdout, emitted AFTER the
//     instruct tail (kills mutation 2c — move emitCdBackNote before
//     instructFn and the last line becomes the instruct output),
//   - the tail returns nil → exit 0.
func TestImplApproveTail_SuccessFromRemovedSpecWorktree(t *testing.T) {
	root := t.TempDir()
	specWt, invocationCwd := chdirIntoDoomed(t, root, filepath.Join(".worktrees", "worktree-spec-091-x"))

	// Terminal mutation: FinalizeEpic removes the spec worktree —
	// including the process cwd.
	if err := os.RemoveAll(specWt); err != nil {
		t.Fatalf("removing spec worktree: %v", err)
	}

	var stdout, stderr bytes.Buffer
	instructed := false
	tailErr := implApproveTail(&stdout, &stderr, root, invocationCwd, "091-x",
		&approve.ImplResult{SpecID: "091-x", SpecBranch: "spec/091-x", CommitCount: 2},
		nil,
		func(r string) error {
			instructed = true
			if r != root {
				t.Errorf("instructFn root: got %q, want %q", r, root)
			}
			fmt.Fprintln(&stdout, "instruct guidance line")
			return nil
		})

	if tailErr != nil {
		t.Fatalf("tail returned error on success path: %v", tailErr)
	}
	if !instructed {
		t.Error("instruct tail was not emitted")
	}

	assertCwdIsRoot(t, root)

	want := "NOTE: your shell's working directory was removed — run: cd " + root
	if got := lastLine(stdout.String()); got != want {
		t.Errorf("last stdout line:\n  got  %q\n  want %q", got, want)
	}
	if !strings.Contains(stdout.String(), "Implementation for 091-x approved") {
		t.Errorf("success output missing; stdout:\n%s", stdout.String())
	}
}

// TestImplApproveTail_SuccessNoNoteWhileCwdExists: from a surviving
// invocation directory the tail stays NOTE-free.
func TestImplApproveTail_SuccessNoNoteWhileCwdExists(t *testing.T) {
	root := t.TempDir()
	_, invocationCwd := chdirIntoDoomed(t, root, "alive")

	var stdout, stderr bytes.Buffer
	tailErr := implApproveTail(&stdout, &stderr, root, invocationCwd, "091-x",
		&approve.ImplResult{SpecID: "091-x"},
		nil,
		func(string) error { fmt.Fprintln(&stdout, "instruct guidance line"); return nil })

	if tailErr != nil {
		t.Fatalf("tail returned error: %v", tailErr)
	}
	if strings.Contains(stdout.String(), "working directory was removed") {
		t.Errorf("NOTE emitted while invocation cwd exists; stdout:\n%s", stdout.String())
	}
}

// TestImplApproveTail_ErrorPathEmitsNoteOnStderr (panel R2-3):
// FinalizeEpic can fail AFTER removing the spec worktree. The error
// path must still chdir to root and emit the NOTE — as the LAST line of
// stderr (stdout is the success channel) — before the caller exits 1.
func TestImplApproveTail_ErrorPathEmitsNoteOnStderr(t *testing.T) {
	root := t.TempDir()
	specWt, invocationCwd := chdirIntoDoomed(t, root, filepath.Join(".worktrees", "worktree-spec-091-x"))

	// FinalizeEpic removed the worktree, then failed.
	if err := os.RemoveAll(specWt); err != nil {
		t.Fatalf("removing spec worktree: %v", err)
	}
	approveErr := errors.New("finalize: direct merge conflict")

	var stdout, stderr bytes.Buffer
	tailErr := implApproveTail(&stdout, &stderr, root, invocationCwd, "091-x",
		nil, // result is nil on error
		approveErr,
		func(string) error {
			t.Error("instruct tail must not run on the error path")
			return nil
		})

	if !errors.Is(tailErr, approveErr) {
		t.Errorf("tail must return the approve error for the exit-1 path; got %v", tailErr)
	}

	assertCwdIsRoot(t, root)

	if stdout.Len() != 0 {
		t.Errorf("error path must not write success output to stdout; got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "error: finalize: direct merge conflict") {
		t.Errorf("stderr missing the error line; got:\n%s", stderr.String())
	}
	want := "NOTE: your shell's working directory was removed — run: cd " + root
	if got := lastLine(stderr.String()); got != want {
		t.Errorf("last stderr line:\n  got  %q\n  want %q", got, want)
	}
}

// TestCompleteTail_NoteIsLastStdoutLine exercises the production
// `mindspec complete` tail with the bead worktree (the invocation cwd)
// removed by the terminal mutation: the NOTE is the LAST line of
// stdout, after FormatResult and the instruct tail (panel R3-2 —
// moving emitCdBackNote before instructFn fails this test).
func TestCompleteTail_NoteIsLastStdoutLine(t *testing.T) {
	root := t.TempDir()
	beadWt, invocationCwd := chdirIntoDoomed(t, root, filepath.Join(".worktrees", "worktree-bead-doom"))

	// complete.Run's CompleteBead removed the bead worktree; its Req 3c
	// chdir moved the process to root.
	if err := os.RemoveAll(beadWt); err != nil {
		t.Fatalf("removing bead worktree: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir to root: %v", err)
	}

	var stdout, stderr bytes.Buffer
	completeTail(&stdout, &stderr, root, invocationCwd,
		&complete.Result{
			BeadID:          "bead-doom",
			BeadClosed:      true,
			WorktreeRemoved: true,
			NextMode:        state.ModeReview,
			NextSpec:        "009-doomed",
		},
		func(r string) error {
			if r != root {
				t.Errorf("instructFn root: got %q, want %q", r, root)
			}
			fmt.Fprintln(&stdout, "instruct guidance line")
			return nil
		})

	out := stdout.String()
	if !strings.Contains(out, "Bead bead-doom closed.") {
		t.Errorf("FormatResult output missing; stdout:\n%s", out)
	}
	if !strings.Contains(out, "instruct guidance line") {
		t.Errorf("instruct tail missing; stdout:\n%s", out)
	}
	want := "NOTE: your shell's working directory was removed — run: cd " + root
	if got := lastLine(out); got != want {
		t.Errorf("last stdout line:\n  got  %q\n  want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}

// TestCompleteTail_NoNoteWhileCwdExists: the happy path (complete run
// from the repo root) stays NOTE-free.
func TestCompleteTail_NoNoteWhileCwdExists(t *testing.T) {
	root := t.TempDir()
	_, invocationCwd := chdirIntoDoomed(t, root, "alive")

	var stdout, stderr bytes.Buffer
	completeTail(&stdout, &stderr, root, invocationCwd,
		&complete.Result{BeadID: "bead-1", BeadClosed: true},
		func(string) error { fmt.Fprintln(&stdout, "instruct guidance line"); return nil })

	if strings.Contains(stdout.String(), "working directory was removed") {
		t.Errorf("NOTE emitted while invocation cwd exists; stdout:\n%s", stdout.String())
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
