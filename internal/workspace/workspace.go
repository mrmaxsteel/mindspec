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

// FindLocalRoot walks up from startDir looking for .mindspec/ or .git at each level.
// Unlike FindRoot, it does NOT resolve worktrees back to the main repo.
// When CWD is inside a worktree, this returns the worktree directory itself.
// This is used for per-worktree focus: each worktree maintains its own .mindspec/focus.
func FindLocalRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		if exists(filepath.Join(dir, ".mindspec")) || exists(filepath.Join(dir, ".git")) {
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
// Resolution order (ADR-0022): worktree → canonical → legacy.
// 1. root/.worktrees/worktree-spec-<specID>/.mindspec/docs/specs/<specID>/
// 2. root/.mindspec/docs/specs/<specID>/
// 3. root/docs/specs/<specID>/
// Returns the first path that exists on disk. If none exist, returns the
// canonical path (option 2) so that callers creating new specs write to
// the right location.
func SpecDir(root, specID string) string {
	// 1. Worktree path
	wtPath := filepath.Join(root, ".worktrees", "worktree-spec-"+specID,
		".mindspec", "docs", "specs", specID)
	if exists(wtPath) {
		return wtPath
	}
	// 2. Canonical path
	canonical := filepath.Join(CanonicalDocsDir(root), "specs", specID)
	if exists(canonical) {
		return canonical
	}
	// 3. Legacy path
	legacy := filepath.Join(LegacyDocsDir(root), "specs", specID)
	if exists(legacy) {
		return legacy
	}
	// Default: canonical (for new spec creation)
	return canonical
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

// WorktreeKind describes the type of worktree context.
const (
	WorktreeMain = "main" // Main repository root
	WorktreeSpec = "spec" // Spec worktree (.worktrees/worktree-spec-<id>)
	WorktreeBead = "bead" // Bead worktree (.worktrees/worktree-<bead-id>)
)

// DetectWorktreeContext identifies the worktree context from a directory path.
// It returns the kind (main/spec/bead) and any extracted spec or bead ID.
// Detection is based on the path containing .worktrees/worktree-spec-<id> or
// .worktrees/worktree-<bead-id>.
func DetectWorktreeContext(dir string) (kind, specID, beadID string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return WorktreeMain, "", ""
	}

	// Walk the path and use the LAST .worktrees match so that nested worktrees
	// (bead worktree inside spec worktree) resolve to the innermost type.
	parts := strings.Split(filepath.ToSlash(abs), "/")
	lastKind := WorktreeMain
	var lastSpecID, lastBeadID string
	for i, part := range parts {
		if part == ".worktrees" && i+1 < len(parts) {
			wtDir := parts[i+1]
			if strings.HasPrefix(wtDir, "worktree-spec-") {
				lastKind = WorktreeSpec
				lastSpecID = strings.TrimPrefix(wtDir, "worktree-spec-")
				lastBeadID = ""
			} else if strings.HasPrefix(wtDir, "worktree-") {
				lastKind = WorktreeBead
				lastBeadID = strings.TrimPrefix(wtDir, "worktree-")
			}
		}
	}
	return lastKind, lastSpecID, lastBeadID
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
