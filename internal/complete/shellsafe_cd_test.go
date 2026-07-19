package complete

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/state"
)

// TestExecutableCdRendersShellSafe is AC-12 (this package's slice):
// FormatResult's three "Run: `cd ...`" lines route through the single
// shell-safe emitter — a space-bearing spec worktree path is quoted; a
// clean path renders byte-identical to today.
func TestExecutableCdRendersShellSafe(t *testing.T) {
	cases := []struct {
		mode string
	}{
		{state.ModeImplement},
		{state.ModePlan},
		{state.ModeReview},
	}

	for _, c := range cases {
		t.Run(c.mode+": space-bearing spec worktree is quoted", func(t *testing.T) {
			wt := "/repo/.worktrees/worktree spec 001 with spaces"
			r := &Result{
				BeadID:          "bead-1",
				WorktreeRemoved: true,
				NextMode:        c.mode,
				NextSpec:        "001-x",
				SpecWorktree:    wt,
			}
			out := FormatResult(r)
			want := "Run: `cd '" + wt + "'`"
			if !strings.Contains(out, want) {
				t.Errorf("FormatResult output missing quoted cd line %q; got:\n%s", want, out)
			}
		})

		t.Run(c.mode+": clean spec worktree renders byte-identical", func(t *testing.T) {
			wt := "/repo/.worktrees/worktree-spec-001-x"
			r := &Result{
				BeadID:          "bead-1",
				WorktreeRemoved: true,
				NextMode:        c.mode,
				NextSpec:        "001-x",
				SpecWorktree:    wt,
			}
			out := FormatResult(r)
			want := "Run: `cd " + wt + "`"
			if !strings.Contains(out, want) {
				t.Errorf("FormatResult output changed for a clean path; got:\n%s\nwant substring: %s", out, want)
			}
			if strings.Contains(out, "cd '"+wt) {
				t.Errorf("clean path must NOT be quoted; got:\n%s", out)
			}
		})
	}
}
