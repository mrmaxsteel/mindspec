package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoRoot is returned when no project root marker is found.
var ErrNoRoot = errors.New("no mindspec project root found (looked for .mindspec/, .git)")

// FindRoot walks up from startDir looking for .mindspec/ or .git at each level.
// It checks .mindspec/ first, then .git, to identify the project root.
// If the candidate is a git worktree (.git is a file, not a directory),
// it resolves to the main repository root instead.
func FindRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		if exists(filepath.Join(dir, ".mindspec")) || exists(filepath.Join(dir, ".git")) {
			if resolved := resolveWorktreeRoot(dir); resolved != "" {
				return resolved, nil
			}
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", ErrNoRoot
}

// resolveWorktreeRoot checks if dir is a git worktree and returns the main
// repo root. Git worktrees have a .git file (not directory) containing
// "gitdir: <path>". The linked gitdir contains a "commondir" file pointing
// back to the main .git directory, whose parent is the main repo root.
// Returns "" if dir is not a worktree or resolution fails.
func resolveWorktreeRoot(dir string) string {
	gitPath := filepath.Join(dir, ".git")
	fi, err := os.Lstat(gitPath)
	if err != nil || fi.IsDir() {
		return "" // real repo or no .git — not a worktree
	}

	// .git is a file → parse "gitdir: <path>"
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, "gitdir: ") {
		return ""
	}
	gitdir := strings.TrimPrefix(content, "gitdir: ")
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(dir, gitdir)
	}
	gitdir = filepath.Clean(gitdir)

	// Read commondir to find the shared .git directory
	commondirData, err := os.ReadFile(filepath.Join(gitdir, "commondir"))
	if err != nil {
		return ""
	}
	commondir := strings.TrimSpace(string(commondirData))
	if !filepath.IsAbs(commondir) {
		commondir = filepath.Join(gitdir, commondir)
	}
	commondir = filepath.Clean(commondir)

	// commondir is the main .git directory; its parent is the main repo root
	mainRoot := filepath.Dir(commondir)
	if exists(filepath.Join(mainRoot, ".mindspec")) || exists(filepath.Join(mainRoot, ".git")) {
		return mainRoot
	}
	return ""
}

// DocsDir returns the path to the docs directory under root.
func DocsDir(root string) string {
	canonical := CanonicalDocsDir(root)
	if exists(canonical) {
		return canonical
	}
	return LegacyDocsDir(root)
}

// CanonicalDocsDir returns the canonical docs root under .mindspec.
func CanonicalDocsDir(root string) string {
	return filepath.Join(root, ".mindspec", "docs")
}

// LegacyDocsDir returns the legacy docs root under project root.
func LegacyDocsDir(root string) string {
	return filepath.Join(root, "docs")
}

// SpecDir returns the path to a specific spec directory under root.
func SpecDir(root, specID string) string {
	return filepath.Join(DocsDir(root), "specs", specID)
}

// ContextMapPath returns the path to docs/context-map.md under root.
func ContextMapPath(root string) string {
	return filepath.Join(DocsDir(root), "context-map.md")
}

// ADRDir returns the path to docs/adr/ under root.
func ADRDir(root string) string {
	return filepath.Join(DocsDir(root), "adr")
}

// DomainDir returns the path to a specific domain's doc directory under root.
func DomainDir(root, domain string) string {
	return filepath.Join(DocsDir(root), "domains", domain)
}

// RecordingDir returns the path to a spec's recording directory.
func RecordingDir(root, specID string) string {
	return filepath.Join(SpecDir(root, specID), "recording")
}

// MindspecDir returns the path to the .mindspec directory under root.
func MindspecDir(root string) string {
	return filepath.Join(root, ".mindspec")
}

// SessionPath returns the path to .mindspec/session.json under root.
func SessionPath(root string) string {
	return filepath.Join(root, ".mindspec", "session.json")
}

// FocusPath returns the path to .mindspec/focus under root.
func FocusPath(root string) string {
	return filepath.Join(root, ".mindspec", "focus")
}

// LifecyclePath returns the path to lifecycle.yaml in a spec directory.
func LifecyclePath(root, specID string) string {
	return filepath.Join(SpecDir(root, specID), "lifecycle.yaml")
}

// EffectiveSpecRoot returns the worktree root for a spec if one exists,
// otherwise returns mainRoot. Use this for reading spec artifacts that
// may only exist in the worktree (plan.md, lifecycle.yaml, etc.).
func EffectiveSpecRoot(mainRoot, specID string) string {
	wtPath := filepath.Join(mainRoot, ".worktrees", "worktree-spec-"+specID)
	if exists(filepath.Join(wtPath, ".mindspec")) {
		return wtPath
	}
	return mainRoot
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
