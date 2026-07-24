package executor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
)

// escapeLines applies termsafe.Escape to each line of a (possibly
// multi-line) block of agent-influenced text — porcelain output, git
// error text, conflicted-file lists — while preserving the real newlines
// that separate genuine lines (R4: per-line escaping for line-oriented
// bodies, never per-message, so a hostile line cannot forge additional
// lines while legitimate multi-line structure survives).
func escapeLines(s string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = termsafe.Escape(l)
	}
	return strings.Join(lines, "\n")
}

// checkWorktreeContainment is the shared check-at-use gate (ADR-0042 §4,
// AC-11) every composed-worktree-path create/chdir/mkdir site in this
// package calls immediately before the actual filesystem/git operation.
// root is the trusted repo root (g.Root, or the resolved spec-worktree
// anchor once ITS OWN containment was already checked); composed is the
// path about to be used. On failure it returns a guard-formatted refusal
// naming the R5 convergent lever — never the raw composed path, which may
// carry a hostile worktree_root byte-for-byte.
func checkWorktreeContainment(root, composed string) error {
	if err := containment.CheckContainment(root, composed); err != nil {
		return guard.NewFailure(
			fmt.Sprintf("refusing to use worktree path: %v", err),
			containment.RejectionLever,
		)
	}
	return nil
}

// WorktreeOps abstracts the bead worktree CLI surface so tests can run
// orchestration logic without requiring `bd` on PATH. The default
// implementation shells out to `bd worktree` via the bead package.
//
// Git, config, and exec are otherwise called directly (see ARCH-11): they
// are either trivially testable against a real temp git repo, or — in the
// case of `bead.Export` — covered by an integration-style test gated on
// `bd` being on PATH. Bug wu7t's orphaned-spec-branch finalize path
// (finalizeOrphanedSpecBranch, below) is the one exception: it needs a
// REAL, assertable export commit on a throwaway from-main worktree without
// requiring bd/Dolt in the test environment, so commitWithExport's export
// step is routed through the package-level execBeadExportFn var instead —
// the same implXxxFn seam convention internal/approve/impl.go uses.
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

// RemoveBeadWorktreeAndRestore removes the bead worktree and then chdirs the
// process to the resolved repo root (g.Root), in that order. It is the
// cwd-safety-critical unit `mindspec release` uses (Spec 101 R2): the bead
// worktree is removed FIRST (before any bd/state mutation), and the process is
// moved to the repo root IMMEDIATELY after so no subsequent bd/git subprocess
// runs from a possibly-deleted cwd (the spec-092 Req 3c / mindspec-qxsy
// cwd-deletion bug class — mirrors complete.go's os.Chdir(root) after worktree
// removal). `release` is expected to be invoked from INSIDE the very worktree
// being removed, so coupling the removal and the chdir in one method keeps the
// remove-first / chdir-immediately invariant provable.
//
// Removal routes through the WorktreeOps seam (ADR-0030: never a raw
// `git worktree remove`). The chdir target is g.Root (the resolved main repo
// root), NEVER the bead worktree path, which may now be deleted. A removal
// error is returned to the caller (the caller decides recovery); a chdir error
// surfaces as a warning but is non-fatal — the bd subprocesses that follow
// would otherwise silently degrade from a deleted cwd, so the warning is the
// honest signal rather than a hard failure.
func (g *MindspecExecutor) RemoveBeadWorktreeAndRestore(beadID string) error {
	wtName, err := workspace.BeadWorktreeName(beadID)
	if err != nil {
		return err
	}
	removeErr := g.WorktreeOps.Remove(wtName)
	// Chdir to root IMMEDIATELY after removal, regardless of removeErr: the
	// process may already be sitting in the (now partially/fully) removed
	// worktree, and every later bd subprocess must run from a live cwd.
	if chdirErr := os.Chdir(g.Root); chdirErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not chdir to repo root %s: %v\n", g.Root, chdirErr)
	}
	return removeErr
}

// InitSpecWorkspace creates a workspace for spec authoring.
// Mirrors the logic in internal/spec/create.go (Phase 1).
func (g *MindspecExecutor) InitSpecWorkspace(specID string) (WorkspaceInfo, error) {
	cfg, err := config.Load(g.Root)
	if err != nil {
		return WorkspaceInfo{}, fmt.Errorf("loading config: %w", err)
	}

	specBranch, err := workspace.SpecBranch(specID)
	if err != nil {
		return WorkspaceInfo{}, err
	}
	wtName, err := workspace.SpecWorktreeName(specID)
	if err != nil {
		return WorkspaceInfo{}, err
	}
	wtPath := cfg.WorktreePath(g.Root, wtName)
	wtRootPath := filepath.Join(g.Root, cfg.WorktreeRoot)

	// R5 check-at-use (ADR-0042 §4, AC-11): re-validate containment of the
	// composed worktree-root directory immediately before creating it.
	if err := checkWorktreeContainment(g.Root, wtRootPath); err != nil {
		return WorkspaceInfo{}, err
	}
	// Ensure .worktrees/ directory exists and is gitignored.
	if err := os.MkdirAll(wtRootPath, 0o755); err != nil {
		return WorkspaceInfo{}, fmt.Errorf("creating %s directory: %w", cfg.WorktreeRoot, err)
	}
	if err := gitutil.EnsureGitignoreEntry(g.Root, cfg.WorktreeRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	// Create spec branch if it doesn't exist. Prefer branching from
	// origin/<detected-default> after a fetch so specs never start from a
	// stale local base (Spec 101 R4); fall back to local HEAD with a WARN on
	// any offline/auth/no-remote/detect failure — never a hard failure.
	if !gitutil.BranchExists(specBranch) {
		base := specBranchBase()
		if err := gitutil.CreateBranch(specBranch, base); err != nil {
			return WorkspaceInfo{}, fmt.Errorf("creating branch %s: %w", specBranch, err)
		}
	}

	// R5 check-at-use (ADR-0042 §4, AC-11): re-validate containment of the
	// composed spec-worktree path (G3's site — the PRIMARY spec-worktree
	// create) immediately before WorktreeOps.Create.
	if err := checkWorktreeContainment(g.Root, wtPath); err != nil {
		return WorkspaceInfo{}, err
	}
	// Create worktree via beads CLI.
	relWtPath := filepath.Join(cfg.WorktreeRoot, wtName)
	if err := g.WorktreeOps.Create(relWtPath, specBranch); err != nil {
		return WorkspaceInfo{}, fmt.Errorf("creating worktree: %w", err)
	}

	return WorkspaceInfo{Path: wtPath, Branch: specBranch}, nil
}

// specBranchBase resolves the base ref a new spec branch should be created
// from (Spec 101 R4). When a remote exists, it fetches and returns
// `origin/<detected-default-branch>` so the spec starts from the up-to-date
// upstream tip. On ANY failure (no remote, offline, auth failure, or a
// default-branch detection miss) it falls back to local "HEAD" and emits a
// WARN naming the stale-base risk — a fetch/detect error is NEVER a hard
// `spec create` failure. All git I/O routes through the gitutil seam
// (ADR-0030); command code never shells out raw.
func specBranchBase() string {
	const remote = "origin"
	if !gitutil.HasRemote() {
		warnStaleBase("no git remote configured")
		return "HEAD"
	}
	if err := gitutil.FetchRemote(remote); err != nil {
		warnStaleBase(fmt.Sprintf("could not fetch %s (%v)", remote, err))
		return "HEAD"
	}
	def, err := gitutil.DetectDefaultBranch(remote)
	if err != nil {
		warnStaleBase(fmt.Sprintf("could not detect default branch of %s (%v)", remote, err))
		return "HEAD"
	}
	return remote + "/" + def
}

// warnStaleBase emits the stale-base WARN shared by every specBranchBase
// fallback path.
func warnStaleBase(reason string) {
	fmt.Fprintf(os.Stderr,
		"WARN: %s; creating the spec branch from local HEAD, which may be a stale base (push/pull the default branch to avoid branching from out-of-date work)\n",
		reason)
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

	branchName, err := workspace.BeadBranch(beadID)
	if err != nil {
		return WorkspaceInfo{}, err
	}
	wtName, err := workspace.BeadWorktreeName(beadID)
	if err != nil {
		return WorkspaceInfo{}, err
	}
	baseBranch := "HEAD"
	if specID != "" {
		baseBranch, err = workspace.SpecBranch(specID)
		if err != nil {
			return WorkspaceInfo{}, err
		}
	}

	// Check for existing worktree.
	entries, listErr := g.WorktreeOps.List()
	if listErr == nil {
		for _, e := range entries {
			if e.Name == wtName || e.Branch == branchName {
				return WorkspaceInfo{Path: e.Path, Branch: branchName}, nil
			}
		}
	}

	// Resolve anchor root: prefer spec worktree if it exists.
	anchorRoot := g.resolveAnchorRoot(specID)

	// R5 check-at-use (ADR-0042 §4, AC-11): anchorRoot may be a composed
	// spec-worktree path (resolveAnchorRoot returns one when it exists on
	// disk) — re-validate its containment before chdir'ing into it below.
	// A bare g.Root anchor is the trusted root and skips this (root-only).
	if anchorRoot != g.Root {
		if err := checkWorktreeContainment(g.Root, anchorRoot); err != nil {
			return WorkspaceInfo{}, err
		}
	}

	// Create bead branch from spec branch (or HEAD).
	if !gitutil.BranchExists(branchName) {
		if err := gitutil.CreateBranch(branchName, baseBranch); err != nil {
			return WorkspaceInfo{}, fmt.Errorf("creating branch %s from %s: %w", branchName, baseBranch, err)
		}
	}

	// R5 check-at-use (ADR-0042 §4, AC-11): re-validate containment of the
	// composed worktree-root directory immediately before creating it.
	anchorWtRootPath := filepath.Join(anchorRoot, cfg.WorktreeRoot)
	if err := checkWorktreeContainment(g.Root, anchorWtRootPath); err != nil {
		return WorkspaceInfo{}, err
	}
	// Ensure .worktrees/ directory and gitignore at the anchor root.
	if err := os.MkdirAll(anchorWtRootPath, 0o755); err != nil {
		return WorkspaceInfo{}, fmt.Errorf("creating %s directory: %w", cfg.WorktreeRoot, err)
	}
	if err := gitutil.EnsureGitignoreEntry(anchorRoot, cfg.WorktreeRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	// Create worktree under the anchor root (wtName validated above).
	relWtPath := filepath.Join(cfg.WorktreeRoot, wtName)
	// R5 check-at-use (ADR-0042 §4, AC-11): re-validate containment of the
	// composed bead-worktree path immediately before WorktreeOps.Create.
	if err := checkWorktreeContainment(g.Root, filepath.Join(anchorRoot, relWtPath)); err != nil {
		return WorkspaceInfo{}, err
	}
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
	beadBranch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return err
	}
	wtName, err := workspace.BeadWorktreeName(beadID)
	if err != nil {
		return err
	}

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
	//
	// Reverse-derivation gate (ADR-0042 §1 reverse, AC-23): specBranch is
	// itself an explicit CompleteBead argument — an agent-writable string,
	// not necessarily waist-composed. CompleteBead is an EXPLICIT verb, so
	// a trimmed suffix failing idvalidate.SpecID REFUSES before any
	// merge/worktree operation rather than silently composing a hostile
	// spec-worktree path.
	specID := strings.TrimPrefix(specBranch, workspace.SpecBranchPrefix)
	if err := idvalidate.SpecID(specID); err != nil {
		return fmt.Errorf("refusing to complete bead %s: spec branch %s does not carry a valid spec id: %w", beadID, specBranch, err)
	}
	cfg, cfgErr := config.Load(g.Root)
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}
	specWtPath, err := workspace.SpecWorktreePath(g.Root, cfg, specID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(specWtPath); err == nil {
		// Spec 106 Bead 4 (Req 9): DIRECTIONAL layout-fingerprint guard in front
		// of the bead→spec merge. Blocks ONLY the regression direction (a
		// canonical/legacy bead branch onto a flat spec target); mutates nothing.
		if guardErr := guardMergeLayout(beadBranch, specBranch, g.layoutAtRef, workspace.MigrationRecoveryActive(g.Root)); guardErr != nil {
			return guardErr
		}
		if mergeErr := gitutil.MergeInto(specWtPath, beadBranch); mergeErr != nil {
			// Spec 092 Req 14(a) incident amendment (2026-06-11
			// merge-driver incident, panel j3-recurrence): a failed
			// bead→spec merge must NEVER be downgraded to a warning.
			// The old warn-and-continue let the caller proceed past a
			// conflicted merge, leaving a closed-but-unmerged bead with
			// the spec worktree stuck mid-merge. New behavior: abort
			// the in-progress merge (spec worktree back to pre-merge
			// state), preserve the bead branch + bead worktree (no
			// cleanup below runs), and return non-zero with the
			// conflicted files and resolve-in-spec-worktree recovery.
			return beadToSpecConflictFailure(beadBranch, specBranch, specWtPath,
				fmt.Sprintf("mindspec complete %s", beadID), mergeErr)
		}
	}

	// Safety check: verify bead branch is merged into spec branch before cleanup.
	// This prevents data loss if the merge above failed silently.
	if gitutil.BranchExists(beadBranch) {
		isAnc, ancErr := gitutil.IsAncestor(g.Root, beadBranch, specBranch)
		if ancErr != nil || !isAnc {
			return fmt.Errorf("bead branch %s is NOT merged into %s — aborting cleanup to prevent data loss", beadBranch, specBranch)
		}

		// Spec 121 R5(b), ADR-0041 §2(ii): the merge-time landed-binding is
		// recorded FAIL-CLOSED, BEFORE any branch/worktree cleanup below —
		// the third durable datum internal/lifecycle.FindLandedMerge
		// consults once this bead's branch and worktree are gone. Gated on
		// beadBranch
		// actually existing and being confirmed merged (this scope): a
		// beadBranch that never existed at all has nothing to bind (no
		// merge relationship to record). A failure here SUPPRESSES cleanup
		// and refuses recoverably (ADR-0035): the branch survives as the
		// corroborating datum (the surviving-branch leg), a re-attempted
		// MergeInto of an already-ancestor branch is a no-op, and the
		// re-run's ensureLandedBinding locates the SAME merge by identity —
		// never a silent warn-and-continue past an unrecorded binding.
		if bindErr := g.ensureLandedBinding(beadID, specBranch, beadBranch); bindErr != nil {
			return guard.NewFailure(
				fmt.Sprintf("bead %s's branch %s merged into %s, but the merge-time landed-merge binding could not be recorded (%v) — refusing to clean up its branch/worktree before this binding is durable.", beadID, beadBranch, specBranch, bindErr),
				fmt.Sprintf("mindspec complete %s", beadID),
			)
		}
	}

	// Remove worktree and delete branch from repo root (not from inside the
	// worktree being removed). Matches the pattern in FinalizeEpic().
	if err := withWorkingDir(g.Root, func() error {
		if wtName != "" {
			if err := g.WorktreeOps.Remove(wtName); err != nil {
				// Final-review G3-2 (spec 120): wtName may be the
				// agent-writable bd-list e.Name (reassigned on the
				// OR-match above) — escape it so a control-bearing name
				// cannot forge extra terminal lines.
				fmt.Fprintf(os.Stderr, "warning: could not remove worktree %s: %v\n", termsafe.Escape(wtName), err)
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

// finalizeStepHookFn is a Spec 119 Bead 6 (AC-26 i4) TEST-ONLY package-var
// hook: FinalizeEpic invokes it at FIVE significant post-mutation stages
// (labeled below, at each call site) so fault-injection tests can inject a
// terminal error IMMEDIATELY AFTER each stage's real mutations have
// already landed — there is no existing seam that separates these internal
// steps of the mutation chain. nil in production (a pure no-op, verified by
// TestFinalizeStepHookFn_DefaultsToNil); tests set it to a stage-labeled
// closure and MUST restore it to nil (t.Cleanup) so it never leaks into
// another test.
//
// Stages (see finalize_fault_test.go for the kill-test at each):
//
//	"auto_merge"     — after the bead-branch auto-merge leg (unmerged
//	                    closed-bead branches merged into the spec branch)
//	"push"           — after the unconditional spec-branch push (a real
//	                    push failure already terminates finalize; this
//	                    hook's error faithfully models the same kill)
//	"orphan_finalize" — after finalizeOrphanedSpecBranch returns (the wu7t
//	                    path; its error already terminates)
//	"pre_cleanup"    — between the merge/push legs and the cleanup leg
//	"post_cleanup"   — after the cleanup leg's mutations complete (worktree/
//	                    branch removals, the direct spec→main merge, spec-
//	                    branch deletion)
var finalizeStepHookFn func(stage string) error

// finalizeStepHook invokes finalizeStepHookFn for stage when set,
// translating a non-nil return into the terminal error FinalizeEpic's
// caller sees — a no-op when no hook is installed (production default).
func finalizeStepHook(stage string) error {
	if finalizeStepHookFn == nil {
		return nil
	}
	return finalizeStepHookFn(stage)
}

// FinalizeEpic merges the spec branch to main, cleans up workspaces and
// branches. Handles bead branch auto-merge into the spec branch before
// cleanup, ensuring all bead work is integrated.
//
// Spec 119 Bead 3 (R6/P6): BOTH bead-branch enumerations below — the
// auto-merge leg and the worktree/branch cleanup leg — are scoped to
// lifecycleAllowSet, the finalizing spec's plan-declared, lifecycle-
// classified bead IDs (computed by the enforcement layer and passed in;
// see the Executor interface doc). A candidate bead/<id> is admitted iff
// its id is a member. lifecycleAllowSet == nil is the "not computed"
// sentinel: encountering ANY bead/<id> candidate while it is nil aborts
// the finalize fail-closed (AC-14) rather than silently skipping the leg
// (a stale re-implementation of today's swallowed-List-error bug) or
// silently admitting the candidate (a scope bypass).
//
// Spec 119 Bead 6 (AC-26 i4): this is a single COMMIT-phase mutation chain
// (ADR-0041 §1) with no existing seam separating its internal steps;
// finalizeStepHook is invoked at five stages so each can be individually
// fault-injected (finalize_fault_test.go).
func (g *MindspecExecutor) FinalizeEpic(epicID, specID, specBranch string, lifecycleAllowSet []string) (FinalizeResult, error) {
	result := FinalizeResult{}

	if !gitutil.BranchExists(specBranch) {
		return result, fmt.Errorf("spec branch %s does not exist", specBranch)
	}

	// Composition-waist gate (ADR-0042 §1): FinalizeEpic is an explicit
	// lifecycle verb — a malformed specID must refuse before any composed
	// worktree path is used, not silently degrade. Every SpecWorktreePath/
	// SpecWorktreeName/FinalizeBranch/FinalizeWorktreePath call below with
	// this same specID is therefore guaranteed to succeed.
	if err := idvalidate.SpecID(specID); err != nil {
		return result, fmt.Errorf("finalize epic %s: invalid spec id %s: %w", epicID, idrender.Spec(specID), err)
	}

	allow := make(map[string]bool, len(lifecycleAllowSet))
	for _, id := range lifecycleAllowSet {
		allow[id] = true
	}

	// Bug wu7t: capture the spec branch tip BEFORE any finalize-time
	// auto-commits land on it (the "chore: commit remaining spec
	// artifacts" commit right below, and any bead-branch auto-merges
	// further down). This is the checkpoint the remote/pr path re-checks
	// against origin/main (see the "Push to remote" block) to tell a live
	// spec branch — still carrying an unmerged implementation PR — from a
	// dead one whose PR already merged. Best-effort: a resolve failure
	// here just disables the protected-main detection and falls through
	// to today's behavior (push the spec branch, nothing else).
	preFinalizeTip, tipErr := gitutil.RevParseRef(g.Root, specBranch)
	if tipErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not resolve pre-finalize tip of %s: %v\n", specBranch, tipErr)
		preFinalizeTip = ""
	}

	// Auto-commit any remaining spec artifacts.
	cfg, cfgErr := config.Load(g.Root)
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}
	// specID already validated above; err is impossible here.
	specWtPath, _ := workspace.SpecWorktreePath(g.Root, cfg, specID)
	if err := g.commitWithExport(specWtPath, "chore: commit remaining spec artifacts"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: auto-commit in spec worktree: %v\n", err)
	}

	// Auto-merge unmerged bead branches into spec branch before cleanup.
	// This handles beads that were closed via `bd close` without `mindspec complete`.
	//
	// Spec 119 Bead 3: a WorktreeOps.List() error is now FAIL-CLOSED —
	// today's `if listErr == nil` silently skipped the whole leg on any
	// enumeration failure, which could leave real merge candidates
	// entirely unmerged with no signal. Abort with a named error instead
	// (AC-14).
	entries, listErr := g.WorktreeOps.List()
	if listErr != nil {
		return result, fmt.Errorf("finalize epic %s: enumerating worktrees for bead auto-merge: %w", epicID, listErr)
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Branch, workspace.BeadBranchPrefix) {
			continue
		}
		// Reverse-derivation gate (ADR-0042 §1 reverse, AC-23): beadID is
		// parsed back OUT of an agent-creatable local git branch name. A
		// malformed candidate is skipped entirely — never auto-merged,
		// never embedded in an ID role — rather than trusted by its
		// bead/-prefix shape alone.
		beadID := strings.TrimPrefix(e.Branch, workspace.BeadBranchPrefix)
		if idvalidate.BeadID(beadID) != nil {
			// ADR-0042 degrade policy: name the skipped branch (escaped) so a
			// malformed reverse-derivation candidate is not silently dropped.
			fmt.Fprintf(os.Stderr, "warning: skipping worktree branch %s: not a well-formed bead branch (reverse-derivation gate)\n", termsafe.Escape(e.Branch))
			continue
		}
		if lifecycleAllowSet == nil {
			// AC-14: a nil allow-set alongside a real bead/<id> candidate
			// means the caller never computed a scope — abort rather
			// than silently skip (today's bug) or silently admit
			// (a scope bypass).
			return result, fmt.Errorf("finalize epic %s: bead branch %s present with no lifecycle allow-set computed — refusing to merge without an explicit scope", epicID, e.Branch)
		}
		if !allow[beadID] {
			// R6: the exclusion boundary is lifecycle identity — a
			// foreign-epic bead or a same-epic NON-lifecycle follow-up
			// (open or closed) is left exactly alone.
			continue
		}
		// Auto-commit any remaining bead artifacts.
		if e.Path != "" {
			_ = g.commitWithExport(e.Path, "chore: commit remaining bead artifacts")
		}
		// Merge bead branch into spec branch if not already an ancestor.
		//
		// Spec 121 final-review G1-1: the ancestry check, both R5(b)
		// binding legs, and the F1-1 fail-closed abort below all operate
		// on g.Root — NOT the spec worktree — so they run REGARDLESS of
		// whether specWtPath is stat-able. The prior shape gated this
		// entire block on os.Stat(specWtPath) while the cleanup leg
		// further down deletes allow-set bead branches unconditionally:
		// with the worktree missing (a prior partial run removed it), a
		// merged-but-unbound branch would be destroyed with neither the
		// surviving-branch datum nor a binding ever recorded — the exact
		// no-evidence attested-restore state R5 exists to prevent (same
		// harm as F1-1, different trigger). Only the actual MergeInto (a
		// genuinely unmerged bead) needs the worktree; when it is missing
		// there, refuse recoverably rather than fall through to cleanup.
		isAnc, ancErr := gitutil.IsAncestor(g.Root, e.Branch, specBranch)
		switch {
		case ancErr != nil:
			// Spec 121 R5(b), ADR-0041 §2(ii) fail-closed (panel F1-1):
			// an IsAncestor infra failure here must NOT silently fall
			// through — the downstream cleanup leg further below
			// force-deletes this bead's branch (`git branch -D`, keyed
			// ONLY off allow-set membership, with no re-check of its
			// own). Falling through here would let a possibly-
			// already-merged-but-UNBOUND branch be destroyed with
			// neither the surviving-branch datum nor a binding ever
			// recorded — landing exactly in the no-evidence
			// attested-restore state R5 exists to prevent. Mirror
			// CompleteBead's identical ancestry-check-error handling
			// (its own `ancErr != nil || !isAnc` safety-check abort):
			// refuse recoverably, mutate nothing, preserve the branch.
			return result, guard.NewFailure(
				fmt.Sprintf("bead %s's branch %s: could not determine whether it is already merged into %s (%v) — refusing to merge or clean it up.", beadID, e.Branch, specBranch, ancErr),
				fmt.Sprintf("mindspec impl approve %s", specID),
			)
		case isAnc:
			// Spec 121 R5(b), ADR-0041 §2(ii) re-run convergence (plan Bead 2 panel G2):
			// this bead's branch is ALREADY an ancestor of specBranch —
			// either it was never actually merged here (a trivially-
			// ancestor branch that never diverged; ensureLandedBinding's
			// locate-by-identity finds nothing and no-ops), or it WAS
			// merged (by a prior run of this very loop, or by
			// CompleteBead) but that prior run's binding write failed
			// and suppressed cleanup — in which case the binding is
			// still ABSENT and must be recorded now, BEFORE the cleanup
			// leg further down runs, so the fail-closed refusal
			// converges here too, not just at CompleteBead's leg.
			if bindErr := g.ensureLandedBinding(beadID, specBranch, e.Branch); bindErr != nil {
				return result, guard.NewFailure(
					fmt.Sprintf("bead %s's branch %s is already merged into %s but its landed-merge binding is missing and could not be recorded (%v) — refusing to clean it up before this binding is durable.", beadID, e.Branch, specBranch, bindErr),
					fmt.Sprintf("mindspec impl approve %s", specID),
				)
			}
		default:
			// Genuinely unmerged — the only leg that truly needs the spec
			// worktree (MergeInto runs inside it). G1-1: when it is
			// missing, refuse recoverably — proceeding would skip the
			// merge AND the binding while cleanup destroys the branch.
			if _, statErr := os.Stat(specWtPath); statErr != nil {
				return result, guard.NewFailure(
					fmt.Sprintf("bead %s's branch %s is not yet merged into %s and the spec worktree (%s) is missing (%v) — refusing to merge or clean it up without the worktree.", beadID, e.Branch, specBranch, specWtPath, statErr),
					fmt.Sprintf("mindspec impl approve %s", specID),
				)
			}
			// Spec 106 Bead 4 (Req 9): directional guard in front of
			// the FinalizeEpic bead→spec auto-merge. guardMergeLayout
			// checks ONLY the directional layout-regression invariant
			// (a flat spec branch must not receive a canonical/legacy-
			// layout bead merge) — it is NOT panel-gate enforcement,
			// NOT the Spec 114/115 obligation-reconciliation backstop,
			// and NOT the bd_close orphan check (Spec 115 mindspec-o4fd
			// OQ1/OQ4): none of those three fire on this
			// executor-owned merge path (ADR-0030: enforcement lives in
			// internal/approve and internal/complete, not here).
			// Mutates nothing on a block.
			if guardErr := guardMergeLayout(e.Branch, specBranch, g.layoutAtRef, workspace.MigrationRecoveryActive(g.Root)); guardErr != nil {
				return result, guardErr
			}
			if mergeErr := gitutil.MergeInto(specWtPath, e.Branch); mergeErr != nil {
				// Spec 092 Req 14(a) — SEMANTIC abort, not a
				// warning: a bead→spec conflict here used to
				// warn-and-continue, removing the spec worktree,
				// direct-merging spec→main WITHOUT the conflicted
				// bead's commits, deleting the spec branch, and
				// exiting 0. New behavior: abort the in-progress
				// merge, perform NO worktree removal, NO direct
				// merge to main, NO branch deletion, and return
				// non-zero (HC-4: the bead→spec merge is part of
				// the terminal mutation). The recovery matches the
				// post-abort reality: the spec worktree still
				// exists because the abort preserved it.
				return result, beadToSpecConflictFailure(e.Branch, specBranch, specWtPath,
					fmt.Sprintf("mindspec impl approve %s", specID), mergeErr)
			}
			fmt.Printf("Merged bead branch %s → %s\n", e.Branch, specBranch)

			// Spec 121 R5(b), ADR-0041 §2(ii): record the merge-time
			// landed-binding FAIL-CLOSED, BEFORE the cleanup leg
			// further down removes this bead's worktree/branch. See
			// CompleteBead's identical discipline above for the full
			// rationale.
			if bindErr := g.ensureLandedBinding(beadID, specBranch, e.Branch); bindErr != nil {
				return result, guard.NewFailure(
					fmt.Sprintf("bead %s's branch %s merged into %s, but the merge-time landed-merge binding could not be recorded (%v) — refusing to clean it up before this binding is durable.", beadID, e.Branch, specBranch, bindErr),
					fmt.Sprintf("mindspec impl approve %s", specID),
				)
			}
		}
	}

	// Spec 119 Bead 6 (AC-26 i4, stage "auto_merge"): every bead-branch
	// auto-merge above has already landed for real by this point.
	if err := finalizeStepHook("auto_merge"); err != nil {
		return result, err
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

		// Push the spec branch FIRST, unconditionally — the baseline
		// contract every "pr" finalize has always had. Bug wu7t panel
		// round 1 (Group 3): the wu7t chore-branch flow below runs AFTER
		// this push, so a failure anywhere in the orphan path still
		// surfaces as an error but can never cost the operator the
		// baseline spec-branch push.
		if err := gitutil.PushBranch(specBranch); err != nil {
			return result, fmt.Errorf("pushing %s: %w", specBranch, err)
		}
		// Spec 119 Bead 6 (AC-26 i4, stage "push"): the spec branch is now
		// genuinely pushed. A real push failure above already terminates
		// finalize; this hook's error faithfully models the same kill
		// immediately after the push landed.
		if err := finalizeStepHook("push"); err != nil {
			return result, err
		}

		// Bug wu7t: on a protected main, the epic-close JSONL export
		// commit above cannot land directly on main — normally it rides
		// the spec branch to a PR. But if the IMPLEMENTATION PR already
		// merged the spec branch's bead work into main (the common case:
		// a spec branch is a one-shot PR carrier, spec 101), the spec
		// branch is now a DEAD carrier — nobody reviews or merges a
		// second PR off an already-merged branch — so this finalize
		// commit never reaches main. main's committed
		// .beads/issues.jsonl then stays stale (epic open / last bead
		// in_progress), and the bd post-merge hook silently reverts the
		// closes in Dolt on every subsequent merge/FF (observed live on
		// spec 106).
		//
		// Detect the already-merged case by checking whether
		// preFinalizeTip (the spec branch's tip captured BEFORE this
		// run's own auto-commits, above) is already an ancestor of
		// origin/main. `git fetch origin main` is best-effort: any
		// failure (offline, auth, no remote reachable) falls back to
		// today's behavior with a warning — mirroring
		// specBranchBase()'s fallback discipline, a fetch/detect error
		// here is never a hard `impl approve` failure.
		//
		// Per-consumer contract (spec 121 R4, ADR-0041 §2(iii)): SHA
		// ancestry remains SUFFICIENT as-is for this probe — it decides
		// carrier ROUTING, and an ancestor spec branch is a spent PR
		// carrier regardless of later history (nothing pushed to it can
		// reach main again; a later revert of its work on main is a
		// main-side state the doctor/stale-tracker surface detects, not a
		// routing input here). When ancestry is FALSE, fall back to the
		// net-effect predicate over the spec branch's full diff — this is
		// what closes the SQUASH blind spot (mindspec-3xqm item 1): a
		// squash-merged impl PR discards the spec branch's SHAs, so
		// ancestry alone would miss it and push the epic-close commit onto
		// the now-dead spec-branch carrier (the pre-121 bug). On a
		// content-fallback INFRA failure, this probe WARNS naming itself
		// undetermined and proceeds on the ancestry answer alone (false) —
		// a DELIBERATE fail-open (spec 121 R4): the stranded outcome stays
		// fully detected by the shipped doctor finalize-orphan finding, the
		// same absorb already covering this probe's own fetch-failure leg.
		orphaned := false
		if preFinalizeTip != "" {
			if fetchErr := withWorkingDir(g.Root, func() error {
				return gitutil.FetchRemoteBranch("origin", "main")
			}); fetchErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not fetch origin/main to check protected-main finalize state: %v\n", fetchErr)
			} else if isAnc, ancErr := gitutil.IsAncestor(g.Root, preFinalizeTip, "origin/main"); ancErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not determine whether %s was already merged into origin/main: %v\n", specBranch, ancErr)
			} else if isAnc {
				orphaned = true
			} else if landed, neErr := netEffectLandedFn(g.Root, preFinalizeTip, "origin/main"); neErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not determine whether %s's content already landed on origin/main (net-effect probe undetermined) — proceeding on ancestry alone: %v\n", specBranch, neErr)
			} else {
				orphaned = landed
			}
		}

		if orphaned {
			branchName, finErr := g.finalizeOrphanedSpecBranch(cfg, epicID, specID)
			if finErr != nil {
				return result, finErr
			}
			result.FinalizeBranch = branchName
			// Spec 119 Bead 6 (AC-26 i4, stage "orphan_finalize"): the wu7t
			// chore/finalize-<specID> branch has already landed on the
			// remote by this point. A real finalizeOrphanedSpecBranch error
			// above already terminates; this hook's error faithfully
			// models the same kill immediately after that branch landed.
			if err := finalizeStepHook("orphan_finalize"); err != nil {
				return result, err
			}
		}
	} else {
		result.MergeStrategy = "direct"
	}

	// Spec 106 Bead 4 (Req 9): DIRECTIONAL layout-fingerprint guard for the
	// no-remote DIRECT spec→main merge. Evaluated HERE — before the cleanup
	// block below removes any worktree — so a blocked regression (a
	// canonical/legacy spec branch onto a flat main) mutates NOTHING: no
	// worktree removal, no branch deletion, no merge commit. The remote-PR path
	// above pushed the branch for a PR and does NOT local-merge, so this guard
	// covers only the local direct-merge seam (the PR path relies on the Bead-3
	// precondition + PR review).
	if result.MergeStrategy == "direct" {
		if guardErr := guardMergeLayout(specBranch, "main", g.layoutAtRef, workspace.MigrationRecoveryActive(g.Root)); guardErr != nil {
			return result, guardErr
		}
	}

	// Spec 119 Bead 6 (AC-26 i4, stage "pre_cleanup"): every merge/push leg
	// above (auto-merge, push, the orphan-finalize branch) has already run;
	// the cleanup leg (worktree/branch removals, the no-remote direct
	// merge, spec-branch deletion) has not started yet.
	if err := finalizeStepHook("pre_cleanup"); err != nil {
		return result, err
	}

	// Run from repo root for cleanup operations.
	if err := withWorkingDir(g.Root, func() error {
		// Clean up lingering bead worktrees/branches. Spec 119 Bead 3:
		// the SAME lifecycleAllowSet scoping and fail-closed enumeration
		// as the auto-merge leg above — a foreign-epic bead or a
		// same-epic non-lifecycle follow-up (open or closed) must
		// survive this leg too (R6), and a WorktreeOps.List() failure
		// here aborts rather than silently skipping the cleanup (AC-14).
		entries, listErr := g.WorktreeOps.List()
		if listErr != nil {
			return fmt.Errorf("finalize epic %s: enumerating worktrees for cleanup: %w", epicID, listErr)
		}
		for _, e := range entries {
			if !strings.HasPrefix(e.Branch, workspace.BeadBranchPrefix) {
				continue
			}
			// Reverse-derivation gate (ADR-0042 §1 reverse, AC-23): as in
			// the auto-merge leg above, a malformed candidate is skipped
			// entirely — never cleaned up or embedded in an ID role.
			beadID := strings.TrimPrefix(e.Branch, workspace.BeadBranchPrefix)
			if idvalidate.BeadID(beadID) != nil {
				// ADR-0042 degrade policy: name the skipped branch (escaped)
				// so a malformed reverse-derivation candidate is not silently
				// dropped from cleanup.
				fmt.Fprintf(os.Stderr, "warning: skipping worktree branch %s: not a well-formed bead branch (reverse-derivation gate)\n", termsafe.Escape(e.Branch))
				continue
			}
			if lifecycleAllowSet == nil {
				return fmt.Errorf("finalize epic %s: bead branch %s present with no lifecycle allow-set computed — refusing to clean up without an explicit scope", epicID, e.Branch)
			}
			if !allow[beadID] {
				continue
			}
			_ = g.WorktreeOps.Remove(e.Name)
			_ = gitutil.DeleteBranch(e.Branch)
		}

		// Remove spec worktree (specID already validated in FinalizeEpic above).
		specWtName, _ := workspace.SpecWorktreeName(specID)
		if err := g.WorktreeOps.Remove(specWtName); err != nil {
			if !isAlreadyRemovedErr(err) {
				return fmt.Errorf("removing spec worktree: %w", err)
			}
		}

		// Direct merge for local (no-remote) workflows. The Spec 106 Bead 4
		// layout-regression guard for this seam already ran ABOVE (before any
		// cleanup), so a cross-layout regression never reaches this merge.
		if result.MergeStrategy == "direct" {
			if err := gitutil.MergeBranch(g.Root, specBranch, "main"); err != nil {
				// Spec 092 Req 14(b) + Req 18: a direct spec→main
				// conflict used to warn-and-continue into
				// DeleteBranch(specBranch) — destroying the only
				// recovery source moments after the merge failed. New
				// behavior: abort the in-progress merge (main left
				// clean), SKIP branch deletion (the early return below
				// never reaches it), and return non-zero (HC-4: for
				// no-remote workflows the direct merge is part of the
				// terminal mutation). This site runs at g.Root on main,
				// AFTER the spec worktree was removed above — the
				// recovery is root-anchored and references no worktree
				// path.
				return directMergeConflictFailure(g.Root, specBranch, err)
			}
		}

		// Delete spec branch.
		if err := gitutil.DeleteBranch(specBranch); err != nil {
			if !isAlreadyRemovedErr(err) {
				return fmt.Errorf("deleting spec branch: %w", err)
			}
		}

		// Spec 119 Bead 6 (AC-26 i4, stage "post_cleanup"): every cleanup
		// mutation above (bead worktree/branch removals, spec worktree
		// removal, the no-remote direct spec→main merge, spec-branch
		// deletion) has already landed for real by this point.
		return finalizeStepHook("post_cleanup")
	}); err != nil {
		return result, err
	}

	return result, nil
}

// finalizeOrphanedSpecBranch is bug wu7t's protected-main recovery: when
// FinalizeEpic's caller (the "Push to remote" block above) determines the
// spec branch is a dead PR carrier — its pre-finalize tip is already an
// ancestor of origin/main — the epic-close JSONL export commit needs a
// FRESH from-main carrier instead. It creates workspace.FinalizeBranch(specID)
// (e.g. "chore/finalize-<specID>") from origin/main in a TEMPORARY worktree,
// refreshes .beads/issues.jsonl via the same commitWithExport helper every
// other finalize commit uses, commits it there, and pushes the branch.
//
// The whole flow is RETRY-IDEMPOTENT (panel round 1, Group 1): a crashed or
// failed prior run may leave behind a temporary worktree (pruned first,
// before any branch operation — the leftover may still have choreBranch
// checked out, which would block the branch delete), a stale local branch
// (deleted and recreated fresh from origin/main), and an already-pushed
// remote branch (reconciled with a force-with-lease push pinned to the
// observed remote tip — a plain push would be rejected non-fast-forward,
// R4's empirical repro). The temporary worktree is always removed before
// returning, success or failure, so it never leaks into `mindspec doctor`
// or `bd worktree list`. Returns the branch name on success; the local
// chore branch is intentionally left behind (harmless — a retry deletes and
// recreates it; note this differs from the spec branch, which FinalizeEpic's
// cleanup DELETES locally after pushing).
func (g *MindspecExecutor) finalizeOrphanedSpecBranch(cfg *config.Config, epicID, specID string) (string, error) {
	choreBranch, err := workspace.FinalizeBranch(specID)
	if err != nil {
		return "", err
	}
	wtPath, err := workspace.FinalizeWorktreePath(g.Root, cfg, specID)
	if err != nil {
		return "", err
	}

	// Final-review S1 (spec 120): containment gate BEFORE the destructive
	// self-heal below. wtPath is composed from the agent-writable
	// cfg.WorktreeRoot, which is only LEXICALLY validated at config
	// ingress (charset/relative/no-"..", explicitly not symlink-aware) —
	// a symlinked ancestor can resolve the composed path outside the
	// project root. The self-heal force-removes whatever exists at that
	// path, so the symlink-aware check-at-use gate must run first; on
	// failure, refuse outright (never force-remove an uncontained path).
	// The second gate further down (before MkdirAll/WorktreeAdd) still
	// runs at ITS use sites per the ADR-0042 check-at-use discipline.
	if err := checkWorktreeContainment(g.Root, wtPath); err != nil {
		return "", err
	}

	// Self-heal leftovers from a crashed prior run BEFORE touching the
	// branch: a leftover temp worktree fails WorktreeAdd below, and one
	// with choreBranch still checked out also blocks DeleteBranch. All
	// best-effort — force-remove a registered worktree, fall back to
	// clearing an unregistered directory, and prune any dangling
	// registration whose directory is already gone.
	if _, statErr := os.Stat(wtPath); statErr == nil {
		if remErr := gitutil.WorktreeRemoveForce(g.Root, wtPath); remErr != nil {
			_ = os.RemoveAll(wtPath)
		}
	}
	_ = gitutil.WorktreePrune(g.Root)

	if err := withWorkingDir(g.Root, func() error {
		if gitutil.BranchExists(choreBranch) {
			if err := gitutil.DeleteBranch(choreBranch); err != nil {
				return fmt.Errorf("clearing stale local branch %s: %w", choreBranch, err)
			}
		}
		return gitutil.CreateBranch(choreBranch, "origin/main")
	}); err != nil {
		return "", fmt.Errorf("creating %s from origin/main: %w", choreBranch, err)
	}

	// R5 check-at-use (ADR-0042 §4, AC-11): re-validate containment of the
	// composed finalize-worktree path immediately before creating its
	// parent directory and before the git worktree add below.
	if err := checkWorktreeContainment(g.Root, wtPath); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return "", fmt.Errorf("creating worktrees directory for %s: %w", choreBranch, err)
	}
	if err := gitutil.WorktreeAdd(g.Root, wtPath, choreBranch); err != nil {
		return "", fmt.Errorf("creating temporary worktree for %s: %w", choreBranch, err)
	}
	defer func() {
		if err := gitutil.WorktreeRemoveForce(g.Root, wtPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove temporary finalize worktree %s: %v\n", wtPath, err)
		}
	}()

	commitMsg := fmt.Sprintf("chore(beads): finalize epic %s for spec %s", epicID, specID)
	if err := g.commitWithExport(wtPath, commitMsg); err != nil {
		return "", fmt.Errorf("committing finalize export on %s: %w", choreBranch, err)
	}

	// Retry-idempotent push: ask the REMOTE (not a possibly-stale local
	// remote-tracking ref) whether the branch already exists there. Absent
	// → plain push (create). Present (a prior run's push) → overwrite it
	// with a lease pinned to the exact tip just observed: the branch is
	// machine-owned and regenerated from live Dolt each run, so replacing
	// the old machine tip is correct, while the lease still fails loudly
	// if some OTHER writer moved the tip in between.
	if err := withWorkingDir(g.Root, func() error {
		remoteSHA, lsErr := gitutil.RemoteHeadSHA("origin", choreBranch)
		if lsErr != nil {
			return fmt.Errorf("checking remote tip of %s: %w", choreBranch, lsErr)
		}
		if remoteSHA == "" {
			return gitutil.PushBranch(choreBranch)
		}
		return gitutil.PushBranchForceWithLease(choreBranch, remoteSHA)
	}); err != nil {
		return "", fmt.Errorf("pushing %s: %w", choreBranch, err)
	}

	return choreBranch, nil
}

// Cleanup removes stale workspaces and branches for a spec.
// Mirrors the logic in internal/cleanup/cleanup.go.
func (g *MindspecExecutor) Cleanup(specID string, force bool) error {
	specBranch, err := workspace.SpecBranch(specID)
	if err != nil {
		return err
	}
	specWtName, err := workspace.SpecWorktreeName(specID)
	if err != nil {
		return err
	}

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
		// R4: each porcelain line names an agent-writable file path —
		// escape per-line so a hostile filename cannot forge extra lines
		// or control bytes into the terminal-facing message.
		return fmt.Errorf("workspace has uncommitted changes:\n%s", escapeLines(strings.TrimSpace(out)))
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
	// SEC-5 (spec 097 R1, executor gap): guard both range operands before
	// they reach git argv — a `-`-prefixed ref would otherwise be reparsed
	// as an option at this direct exec site that does not route through
	// internal/gitutil.
	if err := gitutil.RejectOptionLike(base); err != nil {
		return nil, err
	}
	if err := gitutil.RejectOptionLike(head); err != nil {
		return nil, err
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
	// SEC-5: guard the ref operand before it reaches git argv.
	if err := gitutil.RejectOptionLike(ref); err != nil {
		return nil, err
	}
	cmd := exec.Command("git", "-C", g.Root, "show", ref+":"+path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w", ref, path, err)
	}
	return out, nil
}

// FileAtRefOrAbsent returns the bytes of path at ref, distinguishing a
// path absent from ref's (valid) tree from an operational git failure.
// It first probes existence with `git ls-tree <ref> -- <path>`: that
// command exits 0 with EMPTY output when ref is a valid tree-ish that
// does not contain path (→ present == false, nil error), exits 0 with a
// non-empty line when the path IS present, and FAILS only on an invalid
// ref / git error (→ non-nil error). `git show <ref>:<path>` alone
// cannot make this distinction — it returns a generic error for BOTH
// missing-path and bad-ref — which is exactly why the ref-anchored
// OWNERSHIP loader must not treat all show failures as "absent" (that
// would silently un-gate doc-drift on a git glitch; spec 095 / vvs9 +
// ADR-0036 amend).
func (g *MindspecExecutor) FileAtRefOrAbsent(ref, path string) ([]byte, bool, error) {
	present, err := g.pathExistsAtRef(ref, path)
	if err != nil {
		return nil, false, err
	}
	if !present {
		return nil, false, nil
	}
	data, err := g.FileAtRef(ref, path)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// pathExistsAtRef reports whether path is a tracked entry in ref's tree.
// `git ls-tree <ref> -- <path>` emits one line when the path exists,
// empty output (exit 0) when ref is valid but the path is absent, and
// fails on an invalid ref — the signal that separates an absent path
// (claims-nothing) from an operational error.
func (g *MindspecExecutor) pathExistsAtRef(ref, path string) (bool, error) {
	// SEC-5: guard the ref operand before it reaches git argv.
	if err := gitutil.RejectOptionLike(ref); err != nil {
		return false, err
	}
	cmd := exec.Command("git", "-C", g.Root, "ls-tree", ref, "--", path)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git ls-tree %s -- %s: %w", ref, path, err)
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// BlobExistsAtRef reports whether path is a REGULAR FILE (a git "blob") in
// ref's tree — NOT a directory ("tree"). `git ls-tree <ref> -- <path>` alone
// is NOT a type test: it exits 0 with a matching entry line for a directory
// committed at that exact path just as readily as for a file (verified: a
// tree entry renders `<mode> tree <sha>\t<path>`, a file `<mode> blob
// <sha>\t<path>`), which is exactly why `FileAtRef`/`git show <ref>:<path>`
// (which also succeeds against a tree) cannot be used as a file-type probe
// either. This parses the emitted entry's type field and requires it to be
// exactly "blob". An absent path yields empty output (exit 0) → (false,
// nil); only an invalid ref / git failure returns a non-nil error. Bead 2
// (spec 118 / AC-16, AC-23): the layout git-ref resolver uses this for every
// context-map.md tier so a same-named directory committed at that path is
// never mistaken for the marker file.
func (g *MindspecExecutor) BlobExistsAtRef(ref, path string) (bool, error) {
	// SEC-5: guard the ref operand before it reaches git argv.
	if err := gitutil.RejectOptionLike(ref); err != nil {
		return false, err
	}
	cmd := exec.Command("git", "-C", g.Root, "ls-tree", ref, "--", path)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git ls-tree %s -- %s: %w", ref, path, err)
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return false, nil
	}
	tab := strings.IndexByte(line, '\t')
	if tab < 0 {
		return false, nil
	}
	meta := strings.Fields(line[:tab])
	if len(meta) < 2 {
		return false, nil
	}
	return meta[1] == "blob", nil
}

// TreeDirsAtRef returns the basenames of sub-directory (tree) entries
// directly under dirPath in ref's tree, via `git ls-tree <ref>
// <dirPath>/`. An absent dirPath at a valid ref yields an empty slice
// (no error — like listDomainDirs over a missing directory); an invalid
// ref / git failure returns a non-nil error. Mirrors listDomainDirs
// over a git ref so a branch-only domain directory is enumerable from
// the diffed ref (spec 095 / vvs9). The output is NOT sorted here; the
// caller (listDomainDirsAtRef) sorts to match listDomainDirs.
func (g *MindspecExecutor) TreeDirsAtRef(ref, dirPath string) ([]string, error) {
	// SEC-5: guard the ref operand before it reaches git argv.
	if err := gitutil.RejectOptionLike(ref); err != nil {
		return nil, err
	}
	cmd := exec.Command("git", "-C", g.Root, "ls-tree", ref, dirPath+"/")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-tree %s %s/: %w", ref, dirPath, err)
	}
	var dirs []string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		// Format: "<mode> <type> <object>\t<path>"; keep only trees.
		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			continue
		}
		meta := strings.Fields(line[:tab])
		if len(meta) < 2 || meta[1] != "tree" {
			continue
		}
		dirs = append(dirs, path.Base(line[tab+1:]))
	}
	return dirs, nil
}

// MergeBase returns the merge-base SHA of refs a and b. Wraps
// `git merge-base <a> <b>`.
func (g *MindspecExecutor) MergeBase(a, b string) (string, error) {
	// SEC-5: guard both ref operands before they reach git argv.
	if err := gitutil.RejectOptionLike(a); err != nil {
		return "", err
	}
	if err := gitutil.RejectOptionLike(b); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "-C", g.Root, "merge-base", a, b)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git merge-base %s %s: %w", a, b, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RevParseRef resolves ref to its commit SHA in workdir. Thin pass-through to
// gitutil.RevParseRef so the in-binary panel gate (spec 099) reaches the
// byte-identical bead/<id> staleness rev-parse the hook uses, while keeping
// internal/gitutil imported ONLY here in the executor (ADR-0030 git-I/O
// boundary) and out of the enforcement package internal/complete.
func (g *MindspecExecutor) RevParseRef(workdir, ref string) (string, error) {
	return gitutil.RevParseRef(workdir, ref)
}

// Status returns `git status --porcelain` for workdir. Thin pass-through to
// gitutil.Status — the panel gate's worktree dirty-check seam routed through
// the executor (ADR-0030).
func (g *MindspecExecutor) Status(workdir string) (string, error) {
	return gitutil.Status(workdir)
}

// IsRefNotFound reports whether err is gitutil.ErrRefNotFound (the genuine
// branch-deleted case). Exposing the sentinel test through the executor keeps
// the gitutil.ErrRefNotFound errors.Is check out of internal/complete
// (ADR-0030); behavior is byte-identical to errors.Is(err, ErrRefNotFound).
func (g *MindspecExecutor) IsRefNotFound(err error) bool {
	return errors.Is(err, gitutil.ErrRefNotFound)
}

// GitMv runs a history-preserving `git mv -- <src> <dst>` in workdir. Thin
// pass-through to gitutil — the layout mover's rename primitive routed through
// the executor boundary (ADR-0030, spec 106 Bead 3).
func (g *MindspecExecutor) GitMv(workdir, src, dst string) error {
	return gitutil.GitMv(workdir, src, dst)
}

// ResetHard runs `git reset --hard <ref>` in workdir (the mover's pre-publish
// rollback). Thin pass-through to gitutil.
func (g *MindspecExecutor) ResetHard(workdir, ref string) error {
	return gitutil.ResetHard(workdir, ref)
}

// CleanForce runs `git clean -fd` in workdir (paired with ResetHard on
// rollback). Thin pass-through to gitutil.
func (g *MindspecExecutor) CleanForce(workdir string) error {
	return gitutil.CleanForce(workdir)
}

// CleanForcePaths runs `git clean -fd -- <paths...>` in workdir — the SCOPED
// rollback clean (paired with ResetHard) restricted to the mover's touched
// roots. Thin pass-through to gitutil.
func (g *MindspecExecutor) CleanForcePaths(workdir string, paths []string) error {
	return gitutil.CleanForcePaths(workdir, paths)
}

// CommitPaths stages the given paths and commits them in workdir. Thin
// pass-through to gitutil — the mover's bd-export-free commit primitive.
func (g *MindspecExecutor) CommitPaths(workdir, msg string, paths []string) error {
	return gitutil.CommitPaths(workdir, msg, paths)
}

// LocalBranchRefs returns the short names of every local branch in workdir.
// Thin pass-through to gitutil — source (1) of the migrate-layout discovery scan.
func (g *MindspecExecutor) LocalBranchRefs(workdir string) ([]string, error) {
	return gitutil.LocalBranchRefs(workdir)
}

// RemoteTrackingRefs returns the short names of every remote-tracking ref in
// workdir. Thin pass-through to gitutil — source (2) of the discovery scan.
func (g *MindspecExecutor) RemoteTrackingRefs(workdir string) ([]string, error) {
	return gitutil.RemoteTrackingRefs(workdir)
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
	if err := execBeadExportFn(exportDir); err != nil {
		return fmt.Errorf("refreshing .beads/issues.jsonl: %w", err)
	}
	return gitutil.CommitAll(path, msg)
}

// execBeadExportFn is the bead-export step commitWithExport calls before
// every mindspec-driven commit. Defaults to bead.Export (production
// behavior is unchanged). See the WorktreeOps doc comment above for why
// this seam exists: bug wu7t's finalizeOrphanedSpecBranch test needs a
// deterministic, bd-free export stub.
var execBeadExportFn = bead.Export

// netEffectLandedFn is the ONE exported already-merged predicate
// (gitutil.NetEffectLanded) the protected-main FinalizeEpic probe falls
// back to when SHA ancestry is false (spec 121 R4, ADR-0041 §2(iii)) — the
// squash-merge blind-spot fix. AC-17 anti-drift pins this seam's default to
// be the identical symbol internal/lifecycle's doctor merged-carrier
// suppression routes through, so neither consumer can drift into a private
// reimplementation.
var netEffectLandedFn = gitutil.NetEffectLanded

// mergeBindingFn / mergeBindingReadFn are the spec 121 R5(b) merge-time
// landed-binding seams (ADR-0041 §2(ii)): immediately after a bead->spec
// gitutil.MergeInto succeeds, and BEFORE any branch/worktree cleanup for
// that bead, both producer legs (CompleteBead and FinalizeEpic's auto-merge
// loop) locate the landed merge BY IDENTITY and record it durably through
// mergeBindingFn (bead.MergeMetadata) — the third admissible datum
// internal/lifecycle.FindLandedMerge consults. mergeBindingReadFn
// (bead.GetMetadata) lets ensureLandedBinding check whether a binding is
// already recorded before writing again (idempotent re-runs, and the
// FinalizeEpic already-ancestor "skip" leg's convergence check below).
// Seamed so tests can inject a write failure without a real bd process —
// the whole point of AC-22's kill test.
var (
	mergeBindingFn     = bead.MergeMetadata
	mergeBindingReadFn = bead.GetMetadata
)

// errNoLandedMergeIdentified is locateLandedMergeByIdentity's local
// not-found sentinel (test via errors.Is): the exact-second-parent scan
// found no first-parent merge on specBranch whose second parent equals
// beadBranch's tip. Spec 125 R2: this is NOT itself "nothing to bind" —
// it is a locate MISS, and ensureLandedBinding is the one that classifies
// a miss as either the legitimate nothing-to-bind state (a trivially-
// ancestor branch that never diverged, so no merge commit was ever
// created for it) or a genuine bind failure that must go LOUD.
var errNoLandedMergeIdentified = errors.New("no landed merge commit identified by identity")

// locateLandedMergeByIdentity is the executor-local ground-truth locate
// behind the merge-time landed-binding write (ADR-0041 §2(ii), spec 125
// R1/R2/R5). internal/executor must not import internal/lifecycle — it
// transitively pulls internal/phase, an enforcement package this
// package's own contract (see the doc comment atop executor.go) forbids
// importing — so this is the executor-local counterpart of
// internal/lifecycle.FindLandedMerge. This WRITE side consumes
// gitutil.ExactSecondParentMerges; the READ side (FindLandedMerge /
// ReattestLandedMerge) applies the same two-parent + exact-second-parent
// equality INLINE over the same gitutil.FirstParentMerges scan (it needs
// every owned merge, not one tip's) — logically equivalent filters, but
// two code paths, not one shared primitive. Nor does this side share the
// read side's ownership nomination: this function KNOWS the bead's
// identity directly (it is
// completing/finalizing bead beadBranch) and so it never parses a merge
// subject to establish ownership — this is precisely why R1 persists the
// binding REGARDLESS OF THE MERGE'S SUBJECT FORMAT (the default
// conflict-recovery subject, or the exact "Merge bead/<id>" MergeInto
// itself writes): identity here is corroborated ONLY by beadBranch's own
// tip exact-matching a real merge's second parent on specBranch — the one
// ground-truth datum available at merge/bind time, before the branch is
// ever deleted. Subject-name parsing (the ownership NOMINATOR) is a
// read-side concern only (internal/lifecycle.FindLandedMerge, and the R4
// re-attest surface) reading an unattributed HISTORICAL merge with no
// surviving write-time ground truth.
//
// Deliberately NEVER a rev-parse of the current tip/HEAD of specBranch:
// on a no-op re-run (MergeInto sees an already-ancestor branch and
// performs no new merge) HEAD is NOT the merge commit, so scanning by
// beadBranch's OWN tip (unchanged across re-runs) finds the SAME merge
// every time — one code path serves both the first run and every re-run.
func locateLandedMergeByIdentity(root, specBranch, beadBranch string) (mergeSHA, secondParent string, err error) {
	tip, tipErr := gitutil.RevParseRef(root, beadBranch)
	if tipErr != nil {
		if errors.Is(tipErr, gitutil.ErrRefNotFound) {
			return "", "", errNoLandedMergeIdentified
		}
		return "", "", fmt.Errorf("resolving %s: %w", beadBranch, tipErr)
	}
	merges, err := gitutil.ExactSecondParentMerges(root, specBranch, tip)
	if err != nil {
		return "", "", fmt.Errorf("scanning %s for a landed merge of %s: %w", specBranch, beadBranch, err)
	}
	if len(merges) == 0 {
		return "", "", errNoLandedMergeIdentified
	}
	newest := merges[0]
	return newest.SHA, newest.Parents[1], nil
}

// locateLandedMergeFn is the package seam over locateLandedMergeByIdentity
// (spec 125 R2/AC-11(i)): ensureLandedBinding calls through this variable,
// never the function directly, so a test can force a locate MISS on a
// bead that DID actually merge — the exact fixture AC-3/AC-4b need to pin
// that a genuine miss is LOUD, never re-hidden by a count/merge-base
// classifier. The production default is pinned to the real function by an
// anti-drift pointer-equality test so the gate cannot go hollow.
var locateLandedMergeFn = locateLandedMergeByIdentity

// beadTipLandedOnSpec answers the R2 first-parent-MEMBERSHIP question
// DIRECTLY via gitutil — never through locateLandedMergeFn above — so a
// test that forces the seam to lie cannot also mask this structural
// check: is beadBranch's tip the second parent of ANY first-parent merge
// on specBranch? A true bd_close orphan (zero own commits, still an
// ancestor of specBranch) is not; a bead that DID merge — regardless of
// whether the seam-mediated locate above found it — is. This is the ONE
// discriminator ensureLandedBinding uses to classify a locate miss
// (Background/R2: an own-commit-count or merge-base classifier is
// PROVEN INSUFFICIENT — after the merge, a merged-then-ancestor bead is
// byte-identical to a true orphan under both metrics, because
// ensureLandedBinding runs AFTER the merge).
func beadTipLandedOnSpec(root, specBranch, beadBranch string) (bool, error) {
	tip, tipErr := gitutil.RevParseRef(root, beadBranch)
	if tipErr != nil {
		if errors.Is(tipErr, gitutil.ErrRefNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("resolving %s: %w", beadBranch, tipErr)
	}
	merges, err := gitutil.ExactSecondParentMerges(root, specBranch, tip)
	if err != nil {
		return false, fmt.Errorf("scanning %s for %s's landed merge: %w", specBranch, beadBranch, err)
	}
	return len(merges) > 0, nil
}

// ensureLandedBinding locates beadBranch's landed merge on specBranch by
// identity and, when the merge-time binding is not yet recorded (or not
// yet CONSISTENT, see the G2-1 tightening below) for beadID, writes it via
// mergeBindingFn — fail-closed and BEFORE any cleanup at either producer
// leg (ADR-0041 §2(ii)).
//
// Spec 125 R2: a locate MISS is no longer silently treated as "nothing to
// bind" (the pre-125 design assumed the only miss was a legitimate
// zero-own-commits ancestor branch; in real practice the locate missed
// for beads that DID merge with real commits — the common case, not the
// exception). A miss is now classified STRUCTURALLY, by first-parent
// MEMBERSHIP computed directly via gitutil (beadTipLandedOnSpec, never
// through the seam): the bead's tip is the second parent of NO
// first-parent merge on specBranch ⟹ true nothing-to-bind, quiet, no
// binding, no warning. The tip IS such a second parent (it did merge) but
// the locate missed, or the write below fails ⟹ this is a genuine bind
// failure and MUST go LOUD — returned as an error the caller (CompleteBead
// / FinalizeEpic) translates into a guard.NewFailure that suppresses
// cleanup and names the recovery (re-run the lifecycle command): the
// branch survives as the corroborating datum and the re-invocation
// converges.
func (g *MindspecExecutor) ensureLandedBinding(beadID, specBranch, beadBranch string) error {
	mergeSHA, secondParent, locErr := locateLandedMergeFn(g.Root, specBranch, beadBranch)
	if locErr != nil {
		if !errors.Is(locErr, errNoLandedMergeIdentified) {
			return fmt.Errorf("locating landed merge of %s on %s: %w", beadBranch, specBranch, locErr)
		}
		landed, memErr := beadTipLandedOnSpec(g.Root, specBranch, beadBranch)
		if memErr != nil {
			return fmt.Errorf("checking whether %s has landed on %s: %w", beadBranch, specBranch, memErr)
		}
		if !landed {
			return nil // true nothing-to-bind: a trivially-ancestor branch that never diverged
		}
		return fmt.Errorf("bead branch %s has landed on %s (its tip is the second parent of a first-parent merge there) but the landed-merge commit could not be located by identity — refusing to record the binding rather than silently leaving it unrecorded", beadBranch, specBranch)
	}

	// Spec 121 final-review G3-1, tightened by spec 125's G2-1: "already
	// bound" means bound to BOTH the located merge's SHA AND its second
	// parent — the prior skip keyed ONLY on the stored merge SHA matching,
	// so a binding with the correct merge SHA but an EMPTY or WRONG stored
	// second parent survived as a latent bad binding forever. A mismatch
	// on EITHER key now falls through to the write below, which OVERWRITES
	// with the located merge's SHAs — and a failed overwrite fails closed
	// via the write error, suppressing cleanup and preserving the branch.
	if existing, readErr := mergeBindingReadFn(beadID); readErr == nil {
		sha, _ := existing["mindspec_landed_merge_sha"].(string)
		sp, _ := existing["mindspec_landed_second_parent"].(string)
		if strings.TrimSpace(sha) == mergeSHA && strings.TrimSpace(sp) == secondParent {
			return nil // already bound to this located merge — convergent no-op
		}
	}
	// A read failure is NOT treated as "already bound" — fall through and
	// attempt the write; a persistent failure surfaces via the write below.

	if bindErr := mergeBindingFn(beadID, map[string]interface{}{
		"mindspec_landed_merge_sha":     mergeSHA,
		"mindspec_landed_second_parent": secondParent,
		"mindspec_landed_at":            time.Now().UTC().Format(time.RFC3339),
	}); bindErr != nil {
		return fmt.Errorf("recording the landed-merge binding for %s (merge %s): %w", beadID, mergeSHA, bindErr)
	}
	return nil
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
	// Ambient helper: an invalid specID simply falls back to g.Root
	// (ADR-0042 degrade-vs-error policy for never-block consumers).
	specWt, err := workspace.SpecWorktreePath(g.Root, cfg, specID)
	if err != nil {
		return g.Root
	}
	if fi, err := os.Stat(specWt); err == nil && fi.IsDir() {
		return specWt
	}
	return g.Root
}

// abortMergeState collects the conflicted files of the failed merge in
// workdir and aborts the in-progress merge (if any), restoring the
// pre-merge working tree. Returns the conflicted paths and a
// human-readable note describing the post-abort state. Spec 092
// Reqs 14/18: every conflict-abort site routes through this so the
// failure message names the conflicted files and describes the state
// the recovery commands will actually find.
func abortMergeState(workdir string) (conflicted []string, note string) {
	conflicted = gitutil.ConflictedFiles(workdir)
	if !gitutil.MergeInProgress(workdir) {
		return conflicted, ""
	}
	if abortErr := gitutil.AbortMerge(workdir); abortErr != nil {
		return conflicted, fmt.Sprintf("warning: could not abort the in-progress merge in %s: %v — run `git merge --abort` there before resolving", workdir, abortErr)
	}
	return conflicted, fmt.Sprintf("the in-progress merge in %s was aborted; its working tree is back to the pre-merge state", workdir)
}

// beadToSpecConflictFailure is the Req 14(a) guard failure for a failed
// bead→spec merge (CompleteBead's MergeInto and FinalizeEpic's
// auto-merge — the spec worktree still exists on both paths, so the
// recovery resolves there). rerun is the lifecycle command to re-run
// once the conflict is resolved (`mindspec complete <bead-id>` /
// `mindspec impl approve <spec-id>`); both converge after a manual
// merge because the re-attempted merge sees the bead branch as an
// ancestor.
//
// Spec 125 R5/AC-1b: the printed recovery merge now supplies
// `-m "Merge <beadBranch>"` — the diagnosed root cause (Background) is
// that an operator following the OLD `-m`-less line verbatim produced
// git's default conflict-recovery subject
// (`Merge branch '<beadBranch>' into '<specBranch>'`), which the
// (now-retired) exact-subject identity scan never matched, so the
// merge-time binding silently went unrecorded. Identity is now
// corroborated by second-parent match, not subject text (R1/R2/R5), so
// this fix is belt-and-suspenders: the recovery ALSO produces an
// identifiable exact subject for any reader that still greps by it.
func beadToSpecConflictFailure(beadBranch, specBranch, specWtPath, rerun string, mergeErr error) error {
	conflicted, note := abortMergeState(specWtPath)
	var b strings.Builder
	// R4: beadBranch/specBranch are the waist-validated branch operands —
	// stay RAW. mergeErr is git-produced error text, conflicted entries
	// are agent-writable filenames, and note may embed a git error —
	// each escaped per-line.
	fmt.Fprintf(&b, "merge conflict: could not merge %s into %s: %s", beadBranch, specBranch, escapeLines(fmt.Sprint(mergeErr)))
	if len(conflicted) > 0 {
		escaped := make([]string, len(conflicted))
		for i, f := range conflicted {
			escaped[i] = termsafe.Escape(f)
		}
		fmt.Fprintf(&b, "\nconflicted files:\n  %s", strings.Join(escaped, "\n  "))
	}
	if note != "" {
		b.WriteString("\n")
		b.WriteString(escapeLines(note))
	}
	fmt.Fprintf(&b, "\nnothing was removed: the %s branch, its worktree, and the spec worktree are preserved.", beadBranch)
	fmt.Fprintf(&b, "\nresolve in the spec worktree (%s): re-run the merge there, fix the conflicts, commit the merge, then re-run the lifecycle command", specWtPath)
	return guard.NewFailure(b.String(),
		containment.EmitCd(specWtPath),
		fmt.Sprintf(`git merge --no-ff -m "Merge %s" %s`, beadBranch, beadBranch),
		rerun,
	)
}

// directMergeConflictFailure is the Req 14(b)/Req 18 guard failure for
// a failed direct spec→main merge. It runs at root on main AFTER the
// spec worktree was removed, so the message and recovery are
// root-anchored and reference no worktree path. The merge is aborted
// (main clean) and branch deletion is skipped by the caller's early
// return — the spec branch is the only copy of the work and survives.
func directMergeConflictFailure(root, specBranch string, mergeErr error) error {
	conflicted, note := abortMergeState(root)
	var b strings.Builder
	// R4: specBranch is the waist-validated branch operand — stays RAW.
	// mergeErr, conflicted entries, and note are escaped per-line (see
	// beadToSpecConflictFailure above for the same discipline).
	fmt.Fprintf(&b, "merge conflict: could not merge %s into main: %s", specBranch, escapeLines(fmt.Sprint(mergeErr)))
	if len(conflicted) > 0 {
		escaped := make([]string, len(conflicted))
		for i, f := range conflicted {
			escaped[i] = termsafe.Escape(f)
		}
		fmt.Fprintf(&b, "\nconflicted files:\n  %s", strings.Join(escaped, "\n  "))
	}
	if note != "" {
		b.WriteString("\n")
		b.WriteString(escapeLines(note))
	}
	fmt.Fprintf(&b, "\nmain is clean and the %s branch is preserved (branch deletion was skipped).", specBranch)
	fmt.Fprintf(&b, "\nresolve at the repo root: re-run the merge there, fix the conflicts, commit the merge, then delete the branch with `git branch -d %s`", specBranch)
	return guard.NewFailure(b.String(),
		containment.EmitCd(root),
		"git merge --no-ff "+specBranch,
	)
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
		if restoreErr := os.Chdir(wd); restoreErr != nil {
			// Spec 092 Req 3a (mindspec-qxsy): the restore target is
			// unreachable. chdir is atomic, so the process is still at
			// dir — the path that was just valid. Re-assert it
			// defensively; never return with the process in an
			// undefined cwd.
			_ = os.Chdir(dir)
			// Panel R2-2: when wd no longer exists the failed restore
			// is EXPECTED — the operation itself removed the directory
			// (e.g. FinalizeEpic removing the spec worktree the process
			// was invoked from) — so stay silent. Only a genuine
			// failure (wd still exists but cannot be re-entered)
			// warrants the structured warning.
			if _, statErr := os.Stat(wd); statErr == nil {
				fmt.Fprintf(os.Stderr, "event=executor.cwd_restore_failed dir=%s\n", dir)
			}
		}
	}()
	return fn()
}
