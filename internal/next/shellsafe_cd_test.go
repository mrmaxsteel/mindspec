package next

import (
	"strings"
	"testing"
)

// TestExecutableCdRendersShellSafe is AC-12 (this package's slice):
// DirtyTreeFailure's active-worktree recovery line routes through the
// single shell-safe emitter — a space-bearing active worktree is quoted;
// a clean one renders byte-identical to today.
func TestExecutableCdRendersShellSafe(t *testing.T) {
	t.Run("space-bearing active worktree is quoted", func(t *testing.T) {
		wt := "/repo/.worktrees/worktree bead abc with spaces"
		err := DirtyTreeFailure("/somewhere/else", []string{"foo.go"}, wt)
		want := "cd '" + wt + "' && mindspec next"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("DirtyTreeFailure recovery not shell-safe quoted; got:\n%s\nwant substring: %s", err.Error(), want)
		}
	})

	t.Run("clean active worktree renders byte-identical", func(t *testing.T) {
		wt := "/repo/.worktrees/worktree-bead-abc"
		err := DirtyTreeFailure("/somewhere/else", []string{"foo.go"}, wt)
		want := "cd " + wt + " && mindspec next"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("DirtyTreeFailure recovery changed for a clean path; got:\n%s\nwant substring: %s", err.Error(), want)
		}
		if strings.Contains(err.Error(), "cd '"+wt) {
			t.Errorf("clean path must NOT be quoted; got:\n%s", err.Error())
		}
	})
}
