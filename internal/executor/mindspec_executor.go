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
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// WorktreeOps abstracts the bead worktree CLI surface so tests can run
// orchestration logic without requiring `bd` on PATH. The default
// implementation shells out to `bd worktree` via the bead package.
//
// This is the only DI seam on MindspecExecutor. Git, config, and exec are
// called directly (see ARCH-11): they are either trivially testable against a
// real temp git repo, or — in the case of `bead.Export` — covered by an
// integration-style test gated on `bd` being on PATH.
type WorktreeOps interface {
	Create(name, branch string) error
	Remove(name string) error
	List() ([]bead.WorktreeListEntry, error)
}

// defaultWorktreeOps is the production implementation; it delegates to the
// `bd worktree` CLI via the bead package.
type defaultWorktreeOps struct{}

func (defaultWorktreeOps) Create(name, branch string) error { return bead.WorktreeCreate(name, branch) }
func (defaultWorktreeOps) Remove(name string) error         { return bead.WorktreeRemove(name) }
func (defaultWorktreeOps) List() ([]bead.WorktreeListEntry, error) {
	return bead.WorktreeList()
}

// MindspecExecutor implements Executor using local git operations and beads
// worktree CLI. It preserves all current behavior: worktree-first creation,
// --no-ff merges, .gitignore management.
type MindspecExecutor struct {
	Root string // Main repo root (absolute path)

	// WorktreeOps is the worktree CLI surface. Defaults to the bead package's
	// `bd worktree` wrappers; tests may inject a fake to avoid requiring `bd`
	// on PATH.
	WorktreeOps WorktreeOps
}

// NewMindspecExecutor creates a MindspecExecutor wired to the production
// git/bead/config helpers.
func NewMindspecExecutor(root string) *MindspecExecutor {
	return &MindspecExecutor{
		Root:        root,
		WorktreeOps: defaultWorktreeOps{},
	}
}

// InitSpecWorkspace creates a workspace for spec authoring.
// Mirrors the logic in internal/spec/create.go (Phase 1).
func (g *MindspecExecutor) InitSpecWorkspace(specID string) (WorkspaceInfo, error) {
	cfg, err := config.Load(g.Root)
	if err != nil {
		return WorkspaceInfo{}, fmt.Errorf("loading config: %w", err)
	}

	specBranch := workspace.SpecBranch(specID)
	wtName := workspace.SpecWorktreeName(specID)
	wtPath := cfg.WorktreePath(g.Root, wtName)

	// Ensure .worktrees/ directory exists and is gitignored.
	if err := os.MkdirAll(filepath.Join(g.Root, cfg.WorktreeRoot), 0o755); err != nil {
		return WorkspaceInfo{}, fmt.Errorf("creating %s directory: %w", cfg.WorktreeRoot, err)
	}
	if err := gitutil.EnsureGitignoreEntry(g.Root, cfg.WorktreeRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	// Create spec branch from HEAD if it doesn't exist.
	if !gitutil.BranchExists(specBranch) {
		if err := gitutil.CreateBranch(specBranch, "HEAD"); err != nil {
			return WorkspaceInfo{}, fmt.Errorf("creating branch %s: %w", specBranch, err)
		}
	}

	// Create worktree via beads CLI.
	relWtPath := filepath.Join(cfg.WorktreeRoot, wtName)
	if err := g.WorktreeOps.Create(relWtPath, specBranch); err != nil {
		return WorkspaceInfo{}, fmt.Errorf("creating worktree: %w", err)
	}

	return WorkspaceInfo{Path: wtPath, Branch: specBranch}, nil
}

// HandoffEpic is a no-op for MindspecExecutor. Beads are already created by the
// enforcement layer (plan approve). Other executor implementations (e.g.
// Gastown) may use this to schedule work distribution.
func (g *MindspecExecutor) HandoffEpic(epicID, specID string, beadIDs []string) error {
	return nil
}

// DispatchBead creates a workspace for a bead implementation.
// Mirrors the logic in internal/next/beads.go EnsureWorktree().
func (g *MindspecExecutor) DispatchBead(beadID, specID string) (WorkspaceInfo, error) {
	cfg, err := config.Load(g.Root)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	branchName := workspace.BeadBranch(beadID)
	baseBranch := "HEAD"
	if specID != "" {
		baseBranch = workspace.SpecBranch(specID)
	}

	// Check for existing worktree.
	entries, err := g.WorktreeOps.List()
	if err == nil {
		wtName := workspace.BeadWorktreeName(beadID)
		for _, e := range entries {
			if e.Name == wtName || e.Branch == branchName {
				return WorkspaceInfo{Path: e.Path, Branch: branchName}, nil
			}
		}
	}

	// Resolve anchor root: prefer spec worktree if it exists.
	anchorRoot := g.resolveAnchorRoot(specID)

	// Create bead branch from spec branch (or HEAD).
	if !gitutil.BranchExists(branchName) {
		if err := gitutil.CreateBranch(branchName, baseBranch); err != nil {
			return WorkspaceInfo{}, fmt.Errorf("creating branch %s from %s: %w", branchName, baseBranch, err)
		}
	}

	// Ensure .worktrees/ directory and gitignore at the anchor root.
	if err := os.MkdirAll(filepath.Join(anchorRoot, cfg.WorktreeRoot), 0o755); err != nil {
		return WorkspaceInfo{}, fmt.Errorf("creating %s directory: %w", cfg.WorktreeRoot, err)
	}
	if err := gitutil.EnsureGitignoreEntry(anchorRoot, cfg.WorktreeRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	// Create worktree under the anchor root.
	wtName := workspace.BeadWorktreeName(beadID)
	relWtPath := filepath.Join(cfg.WorktreeRoot, wtName)
	if err := withWorkingDir(anchorRoot, func() error {
		return g.WorktreeOps.Create(relWtPath, branchName)
	}); err != nil {
		return WorkspaceInfo{}, fmt.Errorf("creating worktree: %w", err)
	}

	wtPath := cfg.WorktreePath(anchorRoot, wtName)

	// Read back from worktree list to confirm actual path.
	if entries, err := g.WorktreeOps.List(); err == nil {
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
func (g *MindspecExecutor) CompleteBead(beadID, specBranch, msg string) error {
	beadBranch := workspace.BeadBranch(beadID)
	wtName := workspace.BeadWorktreeName(beadID)

	// Find bead worktree.
	var wtPath string
	if entries, err := g.WorktreeOps.List(); err == nil {
		for _, e := range entries {
			if e.Name == wtName || e.Branch == beadBranch {
				wtName = e.Name
				wtPath = e.Path
				break
			}
		}
	}

	// Auto-commit if message provided, then verify clean tree.
	// When msg is empty the caller is responsible for commit/clean-tree
	// checks (e.g. complete.Run handles recovery-mode skip).
	if msg != "" {
		commitPath := wtPath
		if commitPath == "" {
			commitPath = g.Root
		}
		commitMsg := fmt.Sprintf("impl(%s): %s", beadID, msg)
		if err := g.commitWithExport(commitPath, commitMsg); err != nil {
			return fmt.Errorf("auto-commit failed: %w", err)
		}

		checkPath := wtPath
		if checkPath == "" {
			checkPath = g.Root
		}
		if err := g.IsTreeClean(checkPath); err != nil {
			return fmt.Errorf("%w\nhint: use commit message to auto-commit", err)
		}
	}

	// Merge bead branch into spec branch via spec worktree.
	// Derive spec worktree path from specBranch (spec/<specID>).
	specID := strings.TrimPrefix(specBranch, workspace.SpecBranchPrefix)
	cfg, cfgErr := config.Load(g.Root)
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}
	specWtPath := workspace.SpecWorktreePath(g.Root, cfg, specID)
	if _, err := os.Stat(specWtPath); err == nil {
		if err := gitutil.MergeInto(specWtPath, beadBranch); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not merge %s into %s: %v\n", beadBranch, specBranch, err)
		}
	}

	// Safety check: verify bead branch is merged into spec branch before cleanup.
	// This prevents data loss if the merge above failed silently.
	if gitutil.BranchExists(beadBranch) {
		isAnc, ancErr := gitutil.IsAncestor(g.Root, beadBranch, specBranch)
		if ancErr != nil || !isAnc {
			return fmt.Errorf("bead branch %s is NOT merged into %s — aborting cleanup to prevent data loss", beadBranch, specBranch)
		}
	}

	// Remove worktree and delete branch from repo root (not from inside the
	// worktree being removed). Matches the pattern in FinalizeEpic().
	if err := withWorkingDir(g.Root, func() error {
		if wtName != "" {
			if err := g.WorktreeOps.Remove(wtName); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not remove worktree %s: %v\n", wtName, err)
			}
		}
		if err := gitutil.DeleteBranch(beadBranch); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not delete branch %s: %v\n", beadBranch, err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

// FinalizeEpic merges the spec branch to main, cleans up workspaces and
// branches. Handles bead branch auto-merge into the spec branch before
// cleanup, ensuring all bead work is integrated.
func (g *MindspecExecutor) FinalizeEpic(epicID, specID, specBranch string) (FinalizeResult, error) {
	result := FinalizeResult{}

	if !gitutil.BranchExists(specBranch) {
		return result, fmt.Errorf("spec branch %s does not exist", specBranch)
	}

	// Auto-commit any remaining spec artifacts.
	cfg, cfgErr := config.Load(g.Root)
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}
	specWtPath := workspace.SpecWorktreePath(g.Root, cfg, specID)
	if err := g.commitWithExport(specWtPath, "chore: commit remaining spec artifacts"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: auto-commit in spec worktree: %v\n", err)
	}

	// Auto-merge unmerged bead branches into spec branch before cleanup.
	// This handles beads that were closed via `bd close` without `mindspec complete`.
	if entries, listErr := g.WorktreeOps.List(); listErr == nil {
		for _, e := range entries {
			if !strings.HasPrefix(e.Branch, workspace.BeadBranchPrefix) {
				continue
			}
			// Auto-commit any remaining bead artifacts.
			if e.Path != "" {
				_ = g.commitWithExport(e.Path, "chore: commit remaining bead artifacts")
			}
			// Merge bead branch into spec branch if not already an ancestor.
			if _, statErr := os.Stat(specWtPath); statErr == nil {
				isAnc, ancErr := gitutil.IsAncestor(g.Root, e.Branch, specBranch)
				if ancErr == nil && !isAnc {
					if mergeErr := gitutil.MergeInto(specWtPath, e.Branch); mergeErr != nil {
						fmt.Fprintf(os.Stderr, "warning: could not merge %s into %s: %v\n", e.Branch, specBranch, mergeErr)
					} else {
						fmt.Printf("Merged bead branch %s → %s\n", e.Branch, specBranch)
					}
				}
			}
		}
	}

	// Gather stats (after bead merges so counts are accurate).
	if count, err := gitutil.CommitCount(g.Root, "main", specBranch); err == nil {
		result.CommitCount = count
	}
	if stat, err := gitutil.DiffStat(g.Root, "main", specBranch); err == nil {
		result.DiffStat = stat
	}

	// Push to remote if available.
	if gitutil.HasRemote() {
		result.MergeStrategy = "pr"
		if err := gitutil.PushBranch(specBranch); err != nil {
			return result, fmt.Errorf("pushing %s: %w", specBranch, err)
		}
	} else {
		result.MergeStrategy = "direct"
	}

	// Run from repo root for cleanup operations.
	if err := withWorkingDir(g.Root, func() error {
		// Clean up lingering bead worktrees/branches.
		if entries, listErr := g.WorktreeOps.List(); listErr == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Branch, workspace.BeadBranchPrefix) {
					_ = g.WorktreeOps.Remove(e.Name)
					_ = gitutil.DeleteBranch(e.Branch)
				}
			}
		}

		// Remove spec worktree.
		specWtName := workspace.SpecWorktreeName(specID)
		if err := g.WorktreeOps.Remove(specWtName); err != nil {
			if !isAlreadyRemovedErr(err) {
				return fmt.Errorf("removing spec worktree: %w", err)
			}
		}

		// Direct merge for local (no-remote) workflows.
		if result.MergeStrategy == "direct" {
			if err := gitutil.MergeBranch(g.Root, specBranch, "main"); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not merge %s into main: %v\n", specBranch, err)
			}
		}

		// Delete spec branch.
		if err := gitutil.DeleteBranch(specBranch); err != nil {
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
func (g *MindspecExecutor) Cleanup(specID string, force bool) error {
	specBranch := workspace.SpecBranch(specID)
	specWtName := workspace.SpecWorktreeName(specID)

	// Remove worktree (best-effort).
	if err := g.WorktreeOps.Remove(specWtName); err != nil {
		if !isAlreadyRemovedErr(err) {
			fmt.Fprintf(os.Stderr, "warning: could not remove worktree: %v\n", err)
		}
	}

	// Delete branch (best-effort).
	if gitutil.BranchExists(specBranch) {
		if err := gitutil.DeleteBranch(specBranch); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not delete branch: %v\n", err)
		}
	}

	return nil
}

// IsTreeClean returns nil if the workspace at path has no uncommitted changes.
func (g *MindspecExecutor) IsTreeClean(path string) error {
	out, err := gitutil.Status(path)
	if err != nil {
		return fmt.Errorf("checking worktree status: %w", err)
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("workspace has uncommitted changes:\n%s", strings.TrimSpace(out))
	}
	return nil
}

// DiffStat returns a short diffstat summary between two refs.
func (g *MindspecExecutor) DiffStat(base, head string) (string, error) {
	return gitutil.DiffStat(g.Root, base, head)
}

// CommitCount returns the number of commits between base and head.
func (g *MindspecExecutor) CommitCount(base, head string) (int, error) {
	return gitutil.CommitCount(g.Root, base, head)
}

// CommitAll stages all changes and commits with the given message.
// Refreshes .beads/issues.jsonl from Dolt before staging so the committed
// JSONL is byte-identical to Dolt at commit time (ADR-0025, spec 082).
func (g *MindspecExecutor) CommitAll(path, msg string) error {
	return g.commitWithExport(path, msg)
}

// ChangedFiles returns the list of paths changed between two git refs.
// When base == "", delegates to gitutil.DiffNameOnlyRef("", head) to preserve
// the exact semantics (working tree vs head) the docsync.go call site relies
// on. When both refs are set, shells out to `git diff --name-only base..head`
// directly and parses the newline-separated output inline (the executor must
// not import enforcement packages to reuse a parser).
func (g *MindspecExecutor) ChangedFiles(base, head string) ([]string, error) {
	if base == "" {
		return gitutil.DiffNameOnlyRef("", head)
	}
	cmd := exec.Command("git", "-C", g.Root, "diff", "--name-only", base+".."+head)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only %s..%s: %w", base, head, err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// FileAtRef returns the byte contents of path at git ref. Wraps
// `git show <ref>:<path>`.
func (g *MindspecExecutor) FileAtRef(ref, path string) ([]byte, error) {
	cmd := exec.Command("git", "-C", g.Root, "show", ref+":"+path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w", ref, path, err)
	}
	return out, nil
}

// MergeBase returns the merge-base SHA of refs a and b. Wraps
// `git merge-base <a> <b>`.
func (g *MindspecExecutor) MergeBase(a, b string) (string, error) {
	cmd := exec.Command("git", "-C", g.Root, "merge-base", a, b)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git merge-base %s %s: %w", a, b, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// commitWithExport runs the pre-stage beads export, then delegates to the
// underlying commit. Used by every executor path that ends in `git commit`
// so every mindspec-driven commit carries current beads state.
//
// bd's pre-commit hook also runs `bd export`; the two exports are
// byte-identical (deterministic on unchanged Dolt state). Do not "optimize"
// either away — this one guards against bypassed hooks (--no-verify, test
// paths) and the hook guards ad-hoc `git commit` outside mindspec.
//
// Path semantics: `exportDir` is the workdir of the pending commit (bead
// worktree, spec worktree, or main). `-o .beads/issues.jsonl` resolves
// relative to cmd.Dir, so bd writes to that worktree's tracked JSONL — the
// exact file `git add -A` will stage on this branch. The spec primer phrase
// "main repo's .beads/" describes the semantic endpoint (main becomes
// authoritative after PR merge), not the literal export target: each
// worktree has its own checked-out copy of the tracked file, so refreshing
// "main's copy" from a bead worktree would leave the staged blob stale.
func (g *MindspecExecutor) commitWithExport(path, msg string) error {
	exportDir := path
	if exportDir == "" {
		exportDir = g.Root
	}
	if err := bead.Export(exportDir); err != nil {
		return fmt.Errorf("refreshing .beads/issues.jsonl: %w", err)
	}
	return gitutil.CommitAll(path, msg)
}

// resolveAnchorRoot returns the spec worktree path if it exists, otherwise
// the main repo root. Bead worktrees are anchored under the spec worktree.
func (g *MindspecExecutor) resolveAnchorRoot(specID string) string {
	if specID == "" {
		return g.Root
	}
	cfg, cfgErr := config.Load(g.Root)
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}
	specWt := workspace.SpecWorktreePath(g.Root, cfg, specID)
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
