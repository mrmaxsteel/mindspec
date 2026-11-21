package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestS134PanelRoots_Layouts(t *testing.T) {
	root := t.TempDir()
	flat := filepath.Join(root, ".mindspec", "specs", "001-main")
	wt := filepath.Join(root, ".worktrees", "worktree-spec-002-x")
	wtFlat := filepath.Join(wt, ".mindspec", "specs", "002-flat")
	canonical := filepath.Join(wt, ".mindspec", "docs", "specs", "002-x")
	external := t.TempDir()
	supplied := filepath.Join(external, ".mindspec", "docs", "specs", "003-y")
	for _, dir := range []string{flat, wtFlat, canonical, supplied} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	roots := PanelReviewRoots(root, supplied, flat)
	counts := map[string]int{}
	for _, dir := range roots {
		counts[dir]++
	}
	for _, want := range []string{root, MindspecDir(root), flat, wt, MindspecDir(wt), wtFlat, canonical, supplied, external, MindspecDir(external)} {
		if counts[want] != 1 {
			t.Errorf("root %s occurs %d times in %v", want, counts[want], roots)
		}
	}
}

func TestS134PanelRoots_MissingSpecs(t *testing.T) {
	root := t.TempDir()
	wt := filepath.Join(root, ".worktrees", "worktree-spec-002-x")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatal(err)
	}
	counts := map[string]bool{}
	for _, dir := range PanelReviewRoots(root) {
		counts[dir] = true
	}
	for _, want := range []string{root, MindspecDir(root), wt, MindspecDir(wt)} {
		if !counts[want] {
			t.Errorf("missing root %s", want)
		}
	}
}
