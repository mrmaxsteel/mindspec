package kernel_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestS134Allowance_CheckoutCopyRoundCount(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	counted(t, f, 2)
	n, err := kernel.RoundsForGate(f.State(), gate)
	f.Must(err)
	if n != 2 {
		t.Fatalf("initial rounds=%d, want 2", n)
	}
	src := filepath.Join(f.SpecDir, "reviews", "panel")
	rel, err := filepath.Rel(f.Root, src)
	f.Must(err)
	for _, tree := range []string{"worktree-copy", "worktree-spec-999-stacked"} {
		dst := filepath.Join(f.Root, ".worktrees", tree, rel)
		for _, name := range []string{panel.FileName, "dispositions.jsonl"} {
			data, err := os.ReadFile(filepath.Join(src, name))
			f.Must(err)
			put(t, filepath.Join(dst, name), string(data))
		}
	}
	n, err = kernel.RoundsForGate(f.State(), gate)
	f.Must(err)
	if n != 2 {
		t.Fatalf("checkout copies changed rounds: got %d, want 2", n)
	}
}

func TestS134Allowance_PanelScope(t *testing.T) {
	for _, location := range []string{"other-spec", "other-worktree-root", "other-worktree-spec", "own-spec", "own-worktree-root", "shared"} {
		for _, name := range []string{panel.FileName, "dispositions.jsonl"} {
			t.Run(location+"/"+name, func(t *testing.T) {
				f := reducerFixture(t)
				dir := ""
				own := false
				switch location {
				case "other-spec":
					dir = filepath.Join(f.Root, ".mindspec/specs/999-other/reviews/bad")
				case "other-worktree-root":
					dir = filepath.Join(f.Root, ".worktrees/worktree-spec-999-other/review/bad")
				case "other-worktree-spec":
					dir = filepath.Join(f.Root, ".worktrees/worktree-spec-999-other/.mindspec/specs/999-other/reviews/bad")
				case "own-spec":
					dir = filepath.Join(f.SpecDir, "reviews/bad")
					own = true
				case "own-worktree-root":
					dir = filepath.Join(f.Root, ".worktrees", "worktree-spec-"+f.SpecID, "review/bad")
					own = true
				case "shared":
					dir = filepath.Join(f.Root, ".mindspec/reviews/bad")
					own = true
				}
				put(t, filepath.Join(dir, name), "{bad\n")
				_, err := kernel.ReadState(f.Store, f.SpecID, f.SpecDir)
				if !own {
					if err != nil {
						t.Fatalf("unrelated panel halted this spec: %v", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), dir) || (!strings.Contains(err.Error(), "\nrecovery: in ") || !strings.Contains(err.Error(), ", repair")) {
					t.Fatalf("damaged panel must name its directory and repair step: %v", err)
				}
			})
		}
	}
}
