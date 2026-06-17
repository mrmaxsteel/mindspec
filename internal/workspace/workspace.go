package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
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
//
// SpecDir deliberately scans the default ".worktrees" root rather than
// honoring cfg.WorktreeRoot — it must locate existing on-disk worktrees,
// which may have been created with a different config at the time. A
// follow-up bead may extend this to accept *config.Config and probe both
// the configured root and the default for portability across configs.
//
// Returns an error if specID is not a well-formed spec identifier. This
// turns a previously silent vulnerability (user-controlled IDs reaching
// filepath.Join) into a compile-time obligation for every caller. See
// SEC-1 (bead mindspec-x1qr).
func SpecDir(root, specID string) (string, error) {
	if err := idvalidate.SpecID(specID); err != nil {
		return "", err
	}
	// 1. Worktree path (scan token uses default worktrees dir name).
	wtPath := filepath.Join(DefaultWorktreesDir(root), SpecWorktreeName(specID),
		".mindspec", "docs", "specs", specID)
	if exists(wtPath) {
		return wtPath, nil
	}
	// 2. Canonical path
	canonical := filepath.Join(CanonicalDocsDir(root), "specs", specID)
	if exists(canonical) {
		return canonical, nil
	}
	// 3. Legacy path
	legacy := filepath.Join(LegacyDocsDir(root), "specs", specID)
	if exists(legacy) {
		return legacy, nil
	}
	// Default: canonical (for new spec creation)
	return canonical, nil
}

// ContextMapPath returns the path to docs/context-map.md under root.
func ContextMapPath(root string) string {
	return filepath.Join(DocsDir(root), "context-map.md")
}

// ADRDir returns the path to docs/adr/ under root.
//
// This takes no user input so its signature is unchanged. However, every
// site that joins ADRDir with a user-supplied ADR ID (e.g.
// `filepath.Join(workspace.ADRDir(root), id+".md")`) must validate the id
// first. Use ADRFilePath, which bundles validation + path construction.
func ADRDir(root string) string {
	return filepath.Join(DocsDir(root), "adr")
}

// TreeRootForSpecDir returns the root of the checkout tree that a
// SpecDir-resolved spec directory lives in. SpecDir is worktree-aware
// (ADR-0022 resolution order: worktree → canonical → legacy), but
// ADRDir is not — it always resolves under the primary checkout. This
// helper lets validators that received a spec dir inside a spec
// worktree build an ADR store rooted in the SAME tree, so ADRs
// committed only on the spec branch are visible (mindspec-ew79).
//
// Recognized layouts:
//   - <tree>/.mindspec/docs/specs/<id>  → returns <tree> (4 levels up)
//   - <tree>/docs/specs/<id>            → returns <tree> (3 levels up, legacy)
//
// Returns "" when specDir does not match either layout; callers should
// fall back to the primary root.
func TreeRootForSpecDir(specDir string) string {
	abs, err := filepath.Abs(specDir)
	if err != nil {
		return ""
	}
	abs = filepath.Clean(abs)
	specs := filepath.Dir(abs)
	docs := filepath.Dir(specs)
	if filepath.Base(specs) != "specs" || filepath.Base(docs) != "docs" {
		return ""
	}
	mindspec := filepath.Dir(docs)
	if filepath.Base(mindspec) == ".mindspec" {
		return filepath.Dir(mindspec)
	}
	return mindspec
}

// ADRFilePath returns the on-disk path for a single ADR file by ID.
// Returns an error if adrID is not a well-formed ADR identifier.
func ADRFilePath(root, adrID string) (string, error) {
	if err := idvalidate.ADRID(adrID); err != nil {
		return "", err
	}
	return filepath.Join(ADRDir(root), adrID+".md"), nil
}

// DomainDir returns the path to a specific domain's doc directory under root.
// Returns an error if domain is not a well-formed domain name.
func DomainDir(root, domain string) (string, error) {
	if err := idvalidate.DomainName(domain); err != nil {
		return "", err
	}
	return filepath.Join(DocsDir(root), "domains", domain), nil
}

// RecordingDir returns the path to a spec's recording directory.
// Returns an error if specID is not a well-formed spec identifier.
func RecordingDir(root, specID string) (string, error) {
	specDir, err := SpecDir(root, specID)
	if err != nil {
		return "", err
	}
	return filepath.Join(specDir, "recording"), nil
}

// MindspecDir returns the path to the .mindspec directory under root.
func MindspecDir(root string) string {
	return filepath.Join(root, ".mindspec")
}

// SessionPath returns the path to .mindspec/session.json under root.
func SessionPath(root string) string {
	return filepath.Join(root, ".mindspec", "session.json")
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
//
// This function intentionally does NOT honor cfg.WorktreeRoot: it scans
// path strings that may have been produced with whatever worktree-root
// name was active when the directory was created. Recognizing the
// canonical ".worktrees" token keeps detection robust across configs.
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
			if strings.HasPrefix(wtDir, SpecWorktreePrefix) {
				lastKind = WorktreeSpec
				lastSpecID = strings.TrimPrefix(wtDir, SpecWorktreePrefix)
				lastBeadID = ""
			} else if strings.HasPrefix(wtDir, BeadWorktreePrefix) {
				lastKind = WorktreeBead
				lastBeadID = strings.TrimPrefix(wtDir, BeadWorktreePrefix)
			}
		}
	}
	return lastKind, lastSpecID, lastBeadID
}

// ContextLine renders the worktree context of dir for guard-failure
// messages (spec 092 Req 8). It names the kind of worktree dir is in
// (via DetectWorktreeContext) and the path the failing check actually
// evaluated — often a different directory. Guard call sites append this
// line to their failure message so an agent that ran a command from the
// wrong directory sees both where it is and what was checked.
//
// The format is fixed:
//
//	you are in the <main|spec|bead> worktree (<dir>); this check evaluated <checkedPath>
func ContextLine(dir, checkedPath string) string {
	kind, _, _ := DetectWorktreeContext(dir)
	return fmt.Sprintf("you are in the %s worktree (%s); this check evaluated %s", kind, dir, checkedPath)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
