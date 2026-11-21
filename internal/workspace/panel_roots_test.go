package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestS134PanelRoots_Layouts(t *testing.T) {
	for _, layout := range []string{".mindspec/specs", ".mindspec/docs/specs", "docs/specs"} {
		t.Run(layout, func(t *testing.T) {
			root := t.TempDir()
			wt, err := SpecWorktreePath(root, nil, "002-x")
			if err != nil {
				t.Fatal(err)
			}
			own := filepath.Join(wt, filepath.FromSlash(layout), "002-x")
			other := filepath.Join(root, ".worktrees", "worktree-spec-999-other")
			for _, dir := range []string{own, filepath.Join(other, ".mindspec/specs/002-x"), filepath.Join(SpecsDir(root), "999-other")} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			roots := PanelReviewRoots(root, own, own)
			counts := map[string]int{}
			for _, dir := range roots {
				counts[dir]++
			}
			want := []string{root, MindspecDir(root), own, wt, MindspecDir(wt)}
			if len(roots) != len(want) {
				t.Fatalf("unexpected roots: %v", roots)
			}
			for _, dir := range want {
				if counts[dir] != 1 {
					t.Errorf("root %s occurs %d times in %v", dir, counts[dir], roots)
				}
			}
		})
	}
}

func TestS134PanelRoots_MissingSpecs(t *testing.T) {
	root := t.TempDir()
	wt, err := SpecWorktreePath(root, nil, "002-x")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatal(err)
	}
	// A main-checkout spec still needs its own worktree-root panels.
	own := filepath.Join(root, ".mindspec/specs/002-x")
	roots := PanelReviewRoots(root, own)
	counts := map[string]bool{}
	for _, dir := range roots {
		counts[dir] = true
	}
	for _, want := range []string{root, MindspecDir(root), own, wt, MindspecDir(wt)} {
		if !counts[want] {
			t.Errorf("missing root %s", want)
		}
	}
	if roots := PanelReviewRoots(root); len(roots) != 2 {
		t.Fatalf("unscoped roots include worktrees: %v", roots)
	}
}

func TestS134PanelRoots_ExternalSpec(t *testing.T) {
	root, external := t.TempDir(), t.TempDir()
	dir := filepath.Join(external, ".mindspec/docs/specs/003-y")
	roots := PanelReviewRoots(root, dir)
	found := map[string]bool{}
	for _, path := range roots {
		found[path] = true
	}
	for _, want := range []string{dir, external, MindspecDir(external)} {
		if !found[want] {
			t.Errorf("missing external root %s", want)
		}
	}
}
