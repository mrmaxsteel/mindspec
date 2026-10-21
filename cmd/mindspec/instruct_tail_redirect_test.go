package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

func stubRedirectSeams(t *testing.T, wtPath string, mainCWD bool) {
	t.Helper()
	origPath, origMain := activeWorktreePathFn, isMainCWDFn
	t.Cleanup(func() { activeWorktreePathFn, isMainCWDFn = origPath, origMain })
	activeWorktreePathFn = func(string) string { return wtPath }
	isMainCWDFn = func(string) bool { return mainCWD }
}

// stubResolveFromDir stubs both context reads: the spec-worktree read
// and the cwd read (fn with dir = the process cwd), so a regression to
// the cwd-based tail renders its real output.
func stubResolveFromDir(t *testing.T, fn func(root, dir string) (*phase.Context, error)) {
	t.Helper()
	orig, origCwd := resolveContextFromDirFn, resolveContextFn
	t.Cleanup(func() { resolveContextFromDirFn, resolveContextFn = orig, origCwd })
	resolveContextFromDirFn = fn
	resolveContextFn = func(root string) (*phase.Context, error) {
		cwd, _ := os.Getwd()
		return fn(root, cwd)
	}
}

// TestWorktreeRedirect_SingleBeadUnchanged: from the main checkout with
// one active worktree, the redirect still names it.
func TestWorktreeRedirect_SingleBeadUnchanged(t *testing.T) {
	wt := "/repo/.worktrees/worktree-spec-001-x/.worktrees/worktree-bead-1"
	stubRedirectSeams(t, wt, true)
	got := worktreeRedirect("/repo")
	if !strings.Contains(got, "You are in the main worktree. Switch to:") || !strings.Contains(got, wt) {
		t.Errorf("single-bead redirect must name the active worktree; got:\n%s", got)
	}
	stubRedirectSeams(t, wt, false)
	if got := worktreeRedirect("/repo"); got != "" {
		t.Errorf("no redirect when cwd is not the main worktree; got:\n%s", got)
	}
}

// runCompleteTail drives the production completeTail with the production
// completeInstructFn, capturing both the writer and os.Stdout (the
// instruct renderer prints to os.Stdout).
func runCompleteTail(t *testing.T, root string, r *complete.Result) string {
	t.Helper()
	var buf, stderr bytes.Buffer
	osOut, _ := captureStdout(t, func() error {
		completeTail(&buf, &stderr, root, root, r, completeInstructFn(r))
		return nil
	})
	return buf.String() + osOut
}

// assertNoBeadBootstrap: the post-complete output never calls a dir the
// bead worktree, never tells the agent to claim, never claims cwd is the
// main worktree.
func assertNoBeadBootstrap(t *testing.T, out string) {
	t.Helper()
	for _, bad := range []string{"main worktree", "You are in the bead worktree", "to enter the bead worktree", "to claim a bead", "Active Bead", "worktree-peer-2"} {
		if strings.Contains(out, bad) {
			t.Errorf("post-complete output must not contain %q; got:\n%s", bad, out)
		}
	}
}

// TestCompleteTail_ParallelPeerNoReadyBead: a successful complete while one
// peer bead is in progress and none is ready. Run has moved cwd to the
// repo root, so the cwd-based guard would claim "main worktree" and pick
// the peer's bead worktree. The full tail must not.
func TestCompleteTail_ParallelPeerNoReadyBead(t *testing.T) {
	root := t.TempDir()
	specWt := filepath.Join(root, ".worktrees", "worktree-spec-001-x")
	if err := os.MkdirAll(specWt, 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, root)
	stubRedirectSeams(t, filepath.Join(specWt, ".worktrees", "worktree-peer-2"), true)
	stubResolveFromDir(t, func(string, string) (*phase.Context, error) {
		return &phase.Context{Phase: state.ModeImplement, SpecID: "001-x", BeadID: "peer-2", WorktreePath: root}, nil
	})

	for _, next := range []string{"", "bead-3"} {
		out := runCompleteTail(t, root, &complete.Result{
			BeadID: "bead-1", BeadClosed: true, WorktreeRemoved: true,
			NextMode: state.ModeImplement, NextBead: next, NextSpec: "001-x", SpecWorktree: specWt,
		})
		assertNoBeadBootstrap(t, out)
		if !strings.Contains(out, "Run: `cd ") {
			t.Errorf("next=%q: the summary's cd line must remain; got:\n%s", next, out)
		}
		if next == "" && !strings.Contains(out, "No bead is ready now.") {
			t.Errorf("no-ready summary line missing; got:\n%s", out)
		}
	}
}

// TestCompleteTail_ResolvesFromSpecWorktreeNotCwd: for plan/review the tail
// resolves context from the spec worktree, never the process cwd, and an
// implement context (race) still prints no bead guidance.
func TestCompleteTail_ResolvesFromSpecWorktreeNotCwd(t *testing.T) {
	root := t.TempDir()
	specWt := filepath.Join(root, ".worktrees", "worktree-spec-001-x")
	if err := os.MkdirAll(specWt, 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, root)
	stubRedirectSeams(t, filepath.Join(specWt, ".worktrees", "worktree-peer-2"), true)
	var gotDir string
	stubResolveFromDir(t, func(_, dir string) (*phase.Context, error) {
		gotDir = dir
		return &phase.Context{Phase: state.ModeImplement, SpecID: "001-x", BeadID: "peer-2", WorktreePath: dir}, nil
	})
	out := runCompleteTail(t, root, &complete.Result{
		BeadID: "bead-1", BeadClosed: true, WorktreeRemoved: true,
		NextMode: state.ModeReview, NextSpec: "001-x", SpecWorktree: specWt,
	})
	if gotDir != specWt {
		t.Errorf("tail context dir = %q, want the spec worktree %q", gotDir, specWt)
	}
	assertNoBeadBootstrap(t, out)

	// A review context renders review guidance, still with no redirect.
	stubResolveFromDir(t, func(_, dir string) (*phase.Context, error) {
		return &phase.Context{Phase: state.ModeReview, SpecID: "001-x", WorktreePath: dir}, nil
	})
	out = runCompleteTail(t, root, &complete.Result{
		BeadID: "bead-1", BeadClosed: true, WorktreeRemoved: true,
		NextMode: state.ModeReview, NextSpec: "001-x", SpecWorktree: specWt,
	})
	if !strings.Contains(out, "Review Mode") {
		t.Errorf("review guidance must render; got:\n%s", out)
	}
	assertNoBeadBootstrap(t, out)
}

// TestCompleteTailDir pins when the post-complete tail renders.
func TestCompleteTailDir(t *testing.T) {
	specWt := t.TempDir()
	cases := []struct {
		name   string
		r      *complete.Result
		wantOK bool
	}{
		{"nil", nil, false},
		{"implement", &complete.Result{NextMode: state.ModeImplement, SpecWorktree: specWt}, false},
		{"review", &complete.Result{NextMode: state.ModeReview, SpecWorktree: specWt}, true},
		{"plan", &complete.Result{NextMode: state.ModePlan, SpecWorktree: specWt}, true},
		{"no spec worktree", &complete.Result{NextMode: state.ModeReview}, false},
		{"missing dir", &complete.Result{NextMode: state.ModeReview, SpecWorktree: filepath.Join(specWt, "gone")}, false},
	}
	for _, c := range cases {
		dir, ok := completeTailDir(c.r)
		if ok != c.wantOK || (ok && dir != specWt) {
			t.Errorf("%s: completeTailDir = (%q, %v), want ok=%v", c.name, dir, ok, c.wantOK)
		}
	}
}
