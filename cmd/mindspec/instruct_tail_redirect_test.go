package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

func stubRedirectSeams(t *testing.T, wtPath string, mainCWD bool) {
	t.Helper()
	origPath, origMain := activeWorktreePathFn, isMainCWDFn
	t.Cleanup(func() { activeWorktreePathFn, isMainCWDFn = origPath, origMain })
	activeWorktreePathFn = func(string) string { return wtPath }
	isMainCWDFn = func(string) bool { return mainCWD }
}

// TestWorktreeRedirect_SingleBeadUnchanged: from the main checkout with
// one active worktree, the redirect still names it.
func TestWorktreeRedirect_SingleBeadUnchanged(t *testing.T) {
	wt := "/repo/.worktrees/worktree-spec-001-x/.worktrees/worktree-bead-1"
	stubRedirectSeams(t, wt, true)
	got := worktreeRedirect("/repo", false)
	if !strings.Contains(got, "You are in the main worktree. Switch to:") || !strings.Contains(got, wt) {
		t.Errorf("single-bead redirect must name the active worktree; got:\n%s", got)
	}
	stubRedirectSeams(t, wt, false)
	if got := worktreeRedirect("/repo", false); got != "" {
		t.Errorf("no redirect when cwd is not the main worktree; got:\n%s", got)
	}
}

// TestWorktreeRedirect_AfterVerbCdNoPeerRedirect: after complete (parallel
// beads, a peer's bead worktree is the only in-progress one, and Run
// moved cwd to the repo root), the summary already gave the cd — the
// tail must not add a "main worktree" redirect to the peer's worktree.
func TestWorktreeRedirect_AfterVerbCdNoPeerRedirect(t *testing.T) {
	stubRedirectSeams(t, "/repo/.worktrees/worktree-spec-001-x/.worktrees/worktree-peer-2", true)
	if got := worktreeRedirect("/repo", true); got != "" {
		t.Errorf("no redirect after the verb's own cd; got:\n%s", got)
	}
}

// TestCompleteInstructFn_SkipsRedirectWhenSummaryGaveCd pins the
// complete wiring: a summary with a cd line selects the no-redirect tail.
func TestCompleteInstructFn_SkipsRedirectWhenSummaryGaveCd(t *testing.T) {
	ptr := func(f func(string) error) uintptr { return reflect.ValueOf(f).Pointer() }
	withCd := &complete.Result{WorktreeRemoved: true, SpecWorktree: "/repo/.worktrees/worktree-spec-001-x", NextMode: state.ModeImplement}
	if ptr(completeInstructFn(withCd)) != ptr(emitInstructAfterCd) {
		t.Error("summary with cd must select emitInstructAfterCd")
	}
	noCd := &complete.Result{NextMode: state.ModeIdle}
	if ptr(completeInstructFn(noCd)) != ptr(emitInstruct) {
		t.Error("summary without cd must keep emitInstruct")
	}
	if ptr(completeInstructFn(nil)) != ptr(emitInstruct) {
		t.Error("nil result must keep emitInstruct")
	}
}
