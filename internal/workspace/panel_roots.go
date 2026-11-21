package workspace

import "path/filepath"

// PanelReviewRoots lists the shared checkout roots and the supplied specs' roots.
// Each spec adds its directory and its own worktree, even when the directory is
// in the main checkout. It does not enumerate other specs or worktrees.
func PanelReviewRoots(root string, specDirs ...string) []string {
	var roots []string
	seen := map[string]bool{}
	add := func(dir string) {
		key := filepath.Clean(dir)
		if !seen[key] {
			roots = append(roots, dir)
			seen[key] = true
		}
	}
	add(root)
	add(MindspecDir(root))
	for _, dir := range specDirs {
		if dir == "" {
			continue
		}
		add(dir)
		if tree := TreeRootForSpecDir(dir); tree != "" {
			add(tree)
			add(MindspecDir(tree))
		}
		if tree, err := SpecWorktreePath(root, nil, filepath.Base(dir)); err == nil {
			add(tree)
			add(MindspecDir(tree))
		}
	}
	return roots
}
