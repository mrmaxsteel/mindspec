package instruct

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/state"
)

// TestExecutableCdRendersShellSafe is AC-12 (this package's slice):
// CheckWorktree's recovery hint and templates/implement.md's
// "Run `cd ...`" line (routed through the new `shellsafe` template func)
// both render through the single shell-safe emitter — a space-bearing active
// worktree is quoted; a clean one renders byte-identical to today. The
// CWD-redirect message (run.go) shares the same emitter and is exercised
// indirectly via cmd/mindspec's slice of this AC (Run's own plumbing
// needs a live beads/git harness it isn't worth mocking here).
func TestExecutableCdRendersShellSafe(t *testing.T) {
	t.Run("CheckWorktree: space-bearing active worktree is quoted", func(t *testing.T) {
		orig := getwdFn
		t.Cleanup(func() { getwdFn = orig })
		getwdFn = func() (string, error) { return "/somewhere/else", nil }

		wt := "/repo/.worktrees/worktree bead abc with spaces"
		got := CheckWorktree(wt)
		want := "cd '" + wt + "'"
		if !strings.Contains(got, want) {
			t.Errorf("CheckWorktree message not shell-safe quoted; got %q, want substring %q", got, want)
		}
	})

	t.Run("CheckWorktree: clean active worktree renders byte-identical", func(t *testing.T) {
		orig := getwdFn
		t.Cleanup(func() { getwdFn = orig })
		getwdFn = func() (string, error) { return "/somewhere/else", nil }

		wt := "/repo/.worktrees/worktree-bead-abc"
		got := CheckWorktree(wt)
		want := "cd " + wt
		if !strings.Contains(got, want) {
			t.Errorf("CheckWorktree message changed for a clean path; got %q, want substring %q", got, want)
		}
		if strings.Contains(got, "cd '"+wt) {
			t.Errorf("clean path must NOT be quoted; got %q", got)
		}
	})

	t.Run("templates/implement.md: space-bearing active worktree is quoted", func(t *testing.T) {
		wt := "/repo/.worktrees/worktree bead abc with spaces"
		ctx := &Context{
			Mode:           state.ModeImplement,
			ActiveSpec:     "001-x",
			ActiveBead:     "mindspec-abc.1",
			ActiveWorktree: wt,
			InWorktree:     false,
		}
		out, err := Render(ctx)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		want := "Run `cd '" + wt + "'`"
		if !strings.Contains(out, want) {
			t.Errorf("implement.md render missing quoted cd line %q; got:\n%s", want, out)
		}
	})

	t.Run("templates/implement.md: clean active worktree renders byte-identical", func(t *testing.T) {
		wt := "/repo/.worktrees/worktree-bead-abc"
		ctx := &Context{
			Mode:           state.ModeImplement,
			ActiveSpec:     "001-x",
			ActiveBead:     "mindspec-abc.1",
			ActiveWorktree: wt,
			InWorktree:     false,
		}
		out, err := Render(ctx)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		want := "Run `cd " + wt + "`"
		if !strings.Contains(out, want) {
			t.Errorf("implement.md render changed for a clean path; got:\n%s\nwant substring: %s", out, want)
		}
		if strings.Contains(out, "cd '"+wt) {
			t.Errorf("clean path must NOT be quoted; got:\n%s", out)
		}
	})
}
