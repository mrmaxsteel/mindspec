package panel

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// FinalizeArtifactPath reports whether a path can change during finalization.
func FinalizeArtifactPath(name, specDir string, panelDirs ...string) bool {
	if name == ToolArtifactPaths(specDir)[0] {
		return true
	}
	own := strings.TrimSuffix(Subject("spec_approve", specDir).Roots[0], "/")
	if strings.HasPrefix(name, own+"/") {
		return true
	}
	for _, dir := range panelDirs {
		if strings.HasPrefix(name, strings.TrimSuffix(dir, "/")+"/") {
			return true
		}
	}
	return false
}

// PendingPaths expands status entries, including untracked directories.
// The status producer must disable rename detection so both endpoints are listed.
func PendingPaths(worktree, status string) ([]string, error) {
	var paths []string
	for _, line := range strings.Split(status, "\n") {
		name, err := PorcelainPath(line)
		if err != nil {
			return nil, err
		}
		if name == "" {
			continue
		}
		if !strings.HasSuffix(name, "/") {
			paths = append(paths, name)
			continue
		}
		err = filepath.WalkDir(filepath.Join(worktree, filepath.FromSlash(name)), func(full string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(worktree, full)
			if err == nil {
				paths = append(paths, filepath.ToSlash(rel))
			}
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// FinalizeSubjectDirt lists pending content outside the finalization artifact set.
func FinalizeSubjectDirt(worktree, specDir, status string, panelDirs ...string) ([]string, error) {
	paths, err := PendingPaths(worktree, status)
	if err != nil {
		return nil, err
	}
	var dirt []string
	for _, name := range paths {
		if !FinalizeArtifactPath(name, specDir, panelDirs...) {
			dirt = append(dirt, name)
		}
	}
	return dirt, nil
}
