package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// GitExecutor implements Executor using local git operations and beads
// worktree CLI. It preserves all current behavior: worktree-first creation,
// --no-ff merges, .gitignore management.
//
// Function fields are exposed for testability via injection.
type GitExecutor struct {
	Root string // Main repo root (absolute path)

	// Git operations (default to gitutil package functions).
	CreateBranchFn    func(name, from string) error
	BranchExistsFn    func(name string) bool
	DeleteBranchFn    func(name string) error
	MergeBranchFn     func(workdir, source, target string) error
	MergeIntoFn       func(targetWorkdir, sourceBranch string) error
	CommitAllFn       func(workdir, message string) error
	DiffStatFn        func(workdir, base, head string) (string, error)
	CommitCountFn     func(workdir, base, head string) (int, error)
	IsAncestorFn      func(workdir, ancestor, descendant string) (bool, error)
	HasRemoteFn       func() bool
	PushBranchFn      func(branch string) error
	EnsureGitignoreFn func(root, entry string) error

	// Worktree operations (default to bead package functions).
	WorktreeCreateFn func(name, branch string) error
	WorktreeRemoveFn func(name string) error
	WorktreeListFn   func() ([]bead.WorktreeListEntry, error)

	// Config loader (default to config.Load).
	LoadConfigFn func(root string) (*config.Config, error)

	// Command execution (for git status checks).
	ExecCommandFn func(name string, arg ...string) *exec.Cmd
}

// NewGitExecutor creates a GitExecutor with default function bindings.
func NewGitExecutor(root string) *GitExecutor {
	return &GitExecutor{
		Root:              root,
		CreateBranchFn:    gitutil.CreateBranch,
		BranchExistsFn:    gitutil.BranchExists,
		DeleteBranchFn:    gitutil.DeleteBranch,
		MergeBranchFn:     gitutil.MergeBranch,
		MergeIntoFn:       gitutil.MergeInto,
		CommitAllFn:       gitutil.CommitAll,
		DiffStatFn:        gitutil.DiffStat,
		CommitCountFn:     gitutil.CommitCount,
		IsAncestorFn:      gitutil.IsAncestor,
		HasRemoteFn:       gitutil.HasRemote,
		PushBranchFn:      gitutil.PushBranch,
		EnsureGitignoreFn: gitutil.EnsureGitignoreEntry,
		WorktreeCreateFn:  bead.WorktreeCreate,
		WorktreeRemoveFn:  bead.WorktreeRemove,
		WorktreeListFn:    bead.WorktreeList,
		LoadConfigFn:      config.Load,
		ExecCommandFn:     exec.Command,
	}
}

// InitSpecWorkspace creates a workspace for spec authoring.
// Mirrors the logic in internal/specinit/specinit.go (Phase 1).
func (g *GitExecutor) InitSpecWorkspace(specID string) (WorkspaceInfo, error) {
	cfg, err := g.LoadConfigFn(g.Root)
	if err != nil {
		return WorkspaceInfo{}, fmt.Errorf("loading config: %w", err)
	}

	specBranch := "spec/" + specID
	wtName := "worktree-spec-" + specID
	wtPath := cfg.WorktreePath(g.Root, wtName)

	// Ensure .worktrees/ directory exists and is gitignored.
	if err := os.MkdirAll(filepath.Join(g.Root, cfg.WorktreeRoot), 0o755); err != nil {
		return WorkspaceInfo{}, fmt.Errorf("creating %s directory: %w", cfg.WorktreeRoot, err)
	}
	if err := g.EnsureGitignoreFn(g.Root, cfg.WorktreeRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	// Create spec branch from HEAD if it doesn't exist.
	if !g.BranchExistsFn(specBranch) {
		if err := g.CreateBranchFn(specBranch, "HEAD"); err != nil {
			return WorkspaceInfo{}, fmt.Errorf("creating branch %s: %w", specBranch, err)
		}
	}

	// Create worktree via beads CLI.
	relWtPath := filepath.Join(cfg.WorktreeRoot, wtName)
	if err := g.WorktreeCreateFn(relWtPath, specBranch); err != nil {
		return WorkspaceInfo{}, fmt.Errorf("creating worktree: %w", err)
	}

	return WorkspaceInfo{Path: wtPath, Branch: specBranch}, nil
}

// HandoffEpic is a no-op for GitExecutor. Beads are already created by the
// enforcement layer (plan approve). Other executor implementations (e.g.
// Gastown) may use this to schedule work distribution.
func (g *GitExecutor) HandoffEpic(epicID, specID string, beadIDs []string) error {
	return nil
}

// DispatchBead creates a workspace for a bead implementation.
// Mirrors the logic in internal/next/beads.go EnsureWorktree().
func (g *GitExecutor) DispatchBead(beadID, specID string) (WorkspaceInfo, error) {
	cfg, err := g.LoadConfigFn(g.Root)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	branchName := "bead/" + beadID
	baseBranch := "HEAD"
	if specID != "" {
		baseBranch = "spec/" + specID
	}

	// Check for existing worktree.
	entries, err := g.WorktreeListFn()
	if err == nil {
		wtName := "worktree-" + beadID
		for _, e := range entries {
			if e.Name == wtName || e.Branch == branchName {
				return WorkspaceInfo{Path: e.Path, Branch: branchName}, nil
			}
		}
	}

	// Resolve anchor root: prefer spec worktree if it exists.
	anchorRoot := g.resolveAnchorRoot(specID)

	// Create bead branch from spec branch (or HEAD).
	if !g.BranchExistsFn(branchName) {
		if err := g.CreateBranchFn(branchName, baseBranch); err != nil {
			return WorkspaceInfo{}, fmt.Errorf("creating branch %s from %s: %w", branchName, baseBranch, err)
		}
	}

	// Ensure .worktrees/ directory and gitignore at the anchor root.
	if err := os.MkdirAll(filepath.Join(anchorRoot, cfg.WorktreeRoot), 0o755); err != nil {
		return WorkspaceInfo{}, fmt.Errorf("creating %s directory: %w", cfg.WorktreeRoot, err)
	}
	if err := g.EnsureGitignoreFn(anchorRoot, cfg.WorktreeRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	// Create worktree under the anchor root.
	wtName := "worktree-" + beadID
	relWtPath := filepath.Join(cfg.WorktreeRoot, wtName)
	if err := withWorkingDir(anchorRoot, func() error {
		return g.WorktreeCreateFn(relWtPath, branchName)
	}); err != nil {
		return WorkspaceInfo{}, fmt.Errorf("creating worktree: %w", err)
	}

	wtPath := cfg.WorktreePath(anchorRoot, wtName)

	// Read back from worktree list to confirm actual path.
	if entries, err := g.WorktreeListFn(); err == nil {
		for _, e := range entries {
			if e.Name == wtName || strings.HasSuffix(e.Path, wtName) {
				wtPath = e.Path
				break
			}
		}
	}

	return WorkspaceInfo{Path: wtPath, Branch: branchName}, nil
}

// CompleteBead closes out a bead: commits, merges into spec, removes worktree,
// deletes branch. Mirrors the logic in internal/complete/complete.go.
func (g *GitExecutor) CompleteBead(beadID, specBranch, msg string) error {
	beadBranch := "bead/" + beadID
	wtName := "worktree-" + beadID

	// Find bead worktree.
	var wtPath string
	if entries, err := g.WorktreeListFn(); err == nil {
		for _, e := range entries {
			if e.Name == wtName || e.Branch == beadBranch {
				wtName = e.Name
				wtPath = e.Path
				break
			}
		}
	}

	// Auto-commit if message provided.
	if msg != "" {
		commitPath := wtPath
		if commitPath == "" {
			commitPath = g.Root
		}
		commitMsg := fmt.Sprintf("impl(%s): %s", beadID, msg)
		if err := g.CommitAllFn(commitPath, commitMsg); err != nil {
			return fmt.Errorf("auto-commit failed: %w", err)
		}
	}

	// Check clean tree.
	checkPath := wtPath
	if checkPath == "" {
		checkPath = g.Root
	}
	if err := g.IsTreeClean(checkPath); err != nil {
		return fmt.Errorf("%w\nhint: use commit message to auto-commit", err)
	}

	// Merge bead branch into spec branch via spec worktree.
	// Derive spec worktree path from specBranch (spec/<specID>).
	specID := strings.TrimPrefix(specBranch, "spec/")
	specWtPath := filepath.Join(g.Root, ".worktrees", "worktree-spec-"+specID)
	if _, err := os.Stat(specWtPath); err == nil {
		if err := g.MergeIntoFn(specWtPath, beadBranch); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not merge %s into %s: %v\n", beadBranch, specBranch, err)
		}
	}

	// Remove worktree.
	if wtName != "" {
		if err := g.WorktreeRemoveFn(wtName); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove worktree %s: %v\n", wtName, err)
		}
	}

	// Delete bead branch.
	if err := g.DeleteBranchFn(beadBranch); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not delete branch %s: %v\n", beadBranch, err)
	}

	return nil
}

// FinalizeEpic merges the spec branch to main, cleans up workspaces and
// branches. Mirrors the logic in internal/approve/impl.go.
func (g *GitExecutor) FinalizeEpic(epicID, specID, specBranch string) (FinalizeResult, error) {
	result := FinalizeResult{}

	if !g.BranchExistsFn(specBranch) {
		return result, fmt.Errorf("spec branch %s does not exist", specBranch)
	}

	// Gather stats.
	if count, err := g.CommitCountFn(g.Root, "main", specBranch); err == nil {
		result.CommitCount = count
	}
	if stat, err := g.DiffStatFn(g.Root, "main", specBranch); err == nil {
		result.DiffStat = stat
	}

	// Push to remote if available.
	if g.HasRemoteFn() {
		result.MergeStrategy = "pr"
		if err := g.PushBranchFn(specBranch); err != nil {
			return result, fmt.Errorf("pushing %s: %w", specBranch, err)
		}
	} else {
		result.MergeStrategy = "direct"
	}

	// Auto-commit any remaining spec artifacts.
	specWtPath := filepath.Join(g.Root, ".worktrees", "worktree-spec-"+specID)
	if err := g.CommitAllFn(specWtPath, "chore: commit remaining spec artifacts"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: auto-commit in spec worktree: %v\n", err)
	}

	// Run from repo root for cleanup operations.
	if err := withWorkingDir(g.Root, func() error {
		// Clean up lingering bead worktrees/branches.
		if entries, listErr := g.WorktreeListFn(); listErr == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Branch, "bead/") {
					if e.Path != "" {
						_ = g.CommitAllFn(e.Path, "chore: commit remaining bead artifacts")
					}
					_ = g.WorktreeRemoveFn(e.Name)
					_ = g.DeleteBranchFn(e.Branch)
				}
			}
		}

		// Remove spec worktree.
		specWtName := "worktree-spec-" + specID
		if err := g.WorktreeRemoveFn(specWtName); err != nil {
			if !isAlreadyRemovedErr(err) {
				return fmt.Errorf("removing spec worktree: %w", err)
			}
		}

		// Direct merge for local (no-remote) workflows.
		if result.MergeStrategy == "direct" {
			if err := g.MergeBranchFn(g.Root, specBranch, "main"); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not merge %s into main: %v\n", specBranch, err)
			}
		}

		// Delete spec branch.
		if err := g.DeleteBranchFn(specBranch); err != nil {
			if !isAlreadyRemovedErr(err) {
				return fmt.Errorf("deleting spec branch: %w", err)
			}
		}

		return nil
	}); err != nil {
		return result, err
	}

	return result, nil
}

// Cleanup removes stale workspaces and branches for a spec.
// Mirrors the logic in internal/cleanup/cleanup.go.
func (g *GitExecutor) Cleanup(specID string, force bool) error {
	specBranch := "spec/" + specID
	specWtName := "worktree-spec-" + specID

	// Remove worktree (best-effort).
	if err := g.WorktreeRemoveFn(specWtName); err != nil {
		if !isAlreadyRemovedErr(err) {
			fmt.Fprintf(os.Stderr, "warning: could not remove worktree: %v\n", err)
		}
	}

	// Delete branch (best-effort).
	if g.BranchExistsFn(specBranch) {
		if err := g.DeleteBranchFn(specBranch); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not delete branch: %v\n", err)
		}
	}

	return nil
}

// IsTreeClean returns nil if the workspace at path has no uncommitted changes.
func (g *GitExecutor) IsTreeClean(path string) error {
	cmd := g.ExecCommandFn("git", "-C", path, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("checking worktree status: %w", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("workspace has uncommitted changes:\n%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// DiffStat returns a short diffstat summary between two refs.
func (g *GitExecutor) DiffStat(base, head string) (string, error) {
	return g.DiffStatFn(g.Root, base, head)
}

// CommitCount returns the number of commits between base and head.
func (g *GitExecutor) CommitCount(base, head string) (int, error) {
	return g.CommitCountFn(g.Root, base, head)
}

// CommitAll stages all changes and commits with the given message.
func (g *GitExecutor) CommitAll(path, msg string) error {
	return g.CommitAllFn(path, msg)
}

// resolveAnchorRoot returns the spec worktree path if it exists, otherwise
// the main repo root. Bead worktrees are anchored under the spec worktree.
func (g *GitExecutor) resolveAnchorRoot(specID string) string {
	if specID == "" {
		return g.Root
	}
	specWt := filepath.Join(g.Root, ".worktrees", "worktree-spec-"+specID)
	if fi, err := os.Stat(specWt); err == nil && fi.IsDir() {
		return specWt
	}
	return g.Root
}

func isAlreadyRemovedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "no such") ||
		strings.Contains(msg, "not a valid") ||
		strings.Contains(msg, "is not a worktree")
}

func withWorkingDir(dir string, fn func() error) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting cwd: %w", err)
	}
	if filepath.Clean(wd) == filepath.Clean(dir) {
		return fn()
	}
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("chdir %s: %w", dir, err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()
	return fn()
}
