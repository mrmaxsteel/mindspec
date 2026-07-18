package instruct

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// setupRunTestProject extends setupTestProject into a HERMETIC sandbox for
// Run (Spec 119 R12 / mindspec-z4ps).
//
// z4ps: the prior version only faked a `.git` MARKER — an empty directory,
// not a real repository — and left the test process's own working
// directory untouched. Two independent leaks followed:
//
//  1. phase.ResolveContext/ResolveContextWithCache (internal/phase,
//     called from Run's no-active-target fallback) derive their own scan
//     directory via filepath.Abs(".") — the PROCESS's actual working
//     directory — never this fixture's `root`. When `go test` runs from
//     inside an ACTIVE bead worktree (this very package's source
//     directory lives under
//     .worktrees/worktree-mindspec-<id>/internal/instruct), that real
//     path carries `.worktrees/worktree-<...>` segments;
//     workspace.DetectWorktreeContext parses THOSE segments and resolves
//     a REAL bead ID.
//  2. bead.RunBD/bead.ListJSON shell `bd` with no explicit `-C`/Dir — they
//     also inherit the process's actual cwd, so once (1) hands a real
//     bead ID downstream, the subsequent `bd show <id> --json` reads the
//     REAL, enclosing project's live Dolt state — the exact z4ps failure
//     (TestRun_IdleNoBeads renders live "implement" guidance instead of
//     the idle template).
//
// The fix makes the SANDBOX itself the process's cwd for the test's
// duration — `root` is genuinely "." from every downstream cwd-relative
// resolution — via three legs:
//
//  1. A REAL `git init` in root (not a fake `.git` marker): any
//     repository discovery that DOES walk finds a genuinely valid
//     repository AT root and stops there.
//  2. os.Chdir(root) for the test's duration, restored via t.Cleanup —
//     this is what actually closes leaks (1)/(2) above: every
//     filepath.Abs(".")-based and inherited-cwd-based resolution now
//     anchors at the sandbox, never the enclosing real worktree.
//  3. GIT_CEILING_DIRECTORIES pinned at root's parent as defense in
//     depth — even a discovery walk that somehow starts below root can
//     never cross into whatever lies above the sandbox.
//
// Sequential-only: no test in this package calls t.Parallel, so a
// process-global os.Chdir/os.Setenv is safe here — each test's
// t.Cleanup runs to completion before the next Test function starts.
func setupRunTestProject(t *testing.T) string {
	t.Helper()
	root := setupTestProject(t)

	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init %s failed: %v\n%s", root, err, out)
	}

	origCeiling, hadCeiling := os.LookupEnv("GIT_CEILING_DIRECTORIES")
	if err := os.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root)); err != nil {
		t.Fatalf("setenv GIT_CEILING_DIRECTORIES: %v", err)
	}
	t.Cleanup(func() {
		if hadCeiling {
			_ = os.Setenv("GIT_CEILING_DIRECTORIES", origCeiling)
		} else {
			_ = os.Unsetenv("GIT_CEILING_DIRECTORIES")
		}
	})

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir %s: %v", root, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	return root
}

// TestRun_IdleNoBeads verifies that a workspace with no .beads/ falls back to
// the idle template via the handleNoState path.
func TestRun_IdleNoBeads(t *testing.T) {
	root := setupRunTestProject(t)

	var buf bytes.Buffer
	if err := Run(context.Background(), root, "", "", &buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No Active Work") {
		t.Errorf("expected idle template heading 'No Active Work', got:\n%s", out)
	}
}

// TestRun_IdleNoBeads_HermeticEscape is the Spec 119 AC-24 / mindspec-z4ps
// RED-on-revert proof, independent of where `go test` happens to be
// invoked from (unlike TestRun_IdleNoBeads above, which only reproduces
// z4ps when the test binary's OWN cwd happens to sit inside an active
// bead worktree).
//
// It manufactures exactly the shape that broke setupRunTestProject: an
// OUTER directory tree containing a REAL git repository and a path
// segment (`.worktrees/worktree-mindspec-<id>`) matching
// workspace.DetectWorktreeContext's bead-worktree convention, then makes
// the TEST PROCESS's actual working directory a directory NESTED inside
// that tree BEFORE setupRunTestProject builds the hermetic sandbox —
// simulating "go test invoked from inside an active-spec bead worktree"
// deterministically rather than relying on the ambient environment.
//
// If setupRunTestProject is hermetic, Run still renders the idle
// template: the process cwd during Run is the SANDBOX (setupRunTestProject
// chdirs into it), never this poisoned outer path. Revert
// setupRunTestProject to the old fake-`.git`-marker/no-chdir version and
// this test goes RED: workspace.DetectWorktreeContext(cwd) matches the
// poisoned path's `.worktrees/worktree-mindspec-poison9` segment,
// phase.ResolveContextFromDirWithCache's WorktreeBead branch defaults
// ctx.Phase to state.ModeImplement even when the bead ID is unknown to
// bd, and Run renders "implement" guidance instead of "No Active Work".
func TestRun_IdleNoBeads_HermeticEscape(t *testing.T) {
	outer := t.TempDir()
	poisonedDir := filepath.Join(outer, ".worktrees", "worktree-mindspec-poison9", "nested", "pkg")
	if err := os.MkdirAll(poisonedDir, 0755); err != nil {
		t.Fatalf("mkdir poisoned dir: %v", err)
	}
	// A real repo at `outer` too — the escape must be caught by
	// setupRunTestProject's own hermeticity, not by git simply refusing an
	// invalid/absent repository at the poisoned path.
	if out, err := exec.Command("git", "init", "-q", outer).CombinedOutput(); err != nil {
		t.Fatalf("git init outer failed: %v\n%s", err, out)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(poisonedDir); err != nil {
		t.Fatalf("chdir poisoned dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	// setupRunTestProject must itself escape the poisoned cwd above (via
	// its own os.Chdir(root) leg) for the rest of this test to prove
	// anything.
	root := setupRunTestProject(t)

	var buf bytes.Buffer
	if err := Run(context.Background(), root, "", "", &buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No Active Work") {
		t.Errorf("expected idle template heading 'No Active Work' — resolution must not escape the sandbox root into the poisoned outer worktree, got:\n%s", out)
	}
	if strings.Contains(out, "poison9") {
		t.Errorf("output leaked the poisoned outer bead id 'poison9' — resolution escaped the sandbox root:\n%s", out)
	}
}

// TestRun_JSONFormat verifies the json format path emits parseable JSON to the
// writer.
func TestRun_JSONFormat(t *testing.T) {
	root := setupRunTestProject(t)

	var buf bytes.Buffer
	if err := Run(context.Background(), root, "json", "", &buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	var parsed JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Run JSON output failed to parse: %v\noutput:\n%s", err, buf.String())
	}
	if parsed.Mode == "" {
		t.Errorf("expected non-empty mode in JSON output, got %+v", parsed)
	}
}

// TestRun_WriterIsHonored confirms that Run writes only to the provided writer
// (not to os.Stdout).
func TestRun_WriterIsHonored(t *testing.T) {
	root := setupRunTestProject(t)

	// Capture os.Stdout via a pipe.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	var captured bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&captured, r)
	}()

	var buf bytes.Buffer
	if err := Run(context.Background(), root, "", "", &buf); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Close the writer so the copier exits.
	_ = w.Close()
	wg.Wait()

	if buf.Len() == 0 {
		t.Errorf("expected output in writer buf, got empty")
	}
	if captured.Len() != 0 {
		t.Errorf("expected os.Stdout untouched, got %d bytes: %q", captured.Len(), captured.String())
	}
}

// TestRun_HonorsContext verifies that an already-canceled context causes Run
// to return ctx.Err() at the first step boundary without writing output.
func TestRun_HonorsContext(t *testing.T) {
	root := setupRunTestProject(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	var buf bytes.Buffer
	err := Run(ctx, root, "", "", &buf)
	if err == nil {
		t.Fatalf("expected error from canceled ctx, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output when ctx is canceled, got %q", buf.String())
	}
}

// TestRun_HonorsContextDeadline confirms that a deadline-expired context is
// reported as context.DeadlineExceeded.
func TestRun_HonorsContextDeadline(t *testing.T) {
	root := setupRunTestProject(t)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	var buf bytes.Buffer
	err := Run(ctx, root, "", "", &buf)
	if err == nil {
		t.Fatalf("expected error from expired ctx, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}
