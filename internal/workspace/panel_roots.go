package workspace

import (
	"os"
	"path/filepath"
)

// PanelReviewRoots lists directories that can contain review or reviews panels.
// It includes the main checkout, default worktrees, and any supplied spec directories.
// Missing or unreadable directories do not prevent discovery at other locations.
func PanelReviewRoots(root string, specDirs ...string) []string {
	roots := []string{root, MindspecDir(root)}
	seen := map[string]bool{filepath.Clean(root): true, filepath.Clean(MindspecDir(root)): true}
	add := func(dir string) {
		key := filepath.Clean(dir)
		if !seen[key] {
			roots = append(roots, dir)
			seen[key] = true
		}
	}
	addTree := func(tree string) {
		add(tree)
		add(MindspecDir(tree))
	}
	addSpecs := func(parent string) {
		entries, err := os.ReadDir(parent)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() {
				add(filepath.Join(parent, entry.Name()))
			}
		}
	}
	addSpecs(SpecsDir(root))
	for _, dir := range specDirs {
		if dir == "" {
			continue
		}
		add(dir)
		if tree := TreeRootForSpecDir(dir); tree != "" {
			addTree(tree)
		}
	}
	entries, _ := os.ReadDir(DefaultWorktreesDir(root))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		tree := filepath.Join(DefaultWorktreesDir(root), entry.Name())
		addTree(tree)
		addSpecs(filepath.Join(tree, ".mindspec", "specs"))
		addSpecs(filepath.Join(tree, ".mindspec", "docs", "specs"))
	}
	return roots
}
