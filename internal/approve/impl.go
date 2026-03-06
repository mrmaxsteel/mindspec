package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitops"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"

	"gopkg.in/yaml.v3"
)

var (
	implRunBDCombinedFn = bead.RunBDCombined
	implRunBDFn         = bead.RunBD
	mergeBranchFn       = gitops.MergeBranch
	deleteBranchFn      = gitops.DeleteBranch
	worktreeRemoveFn    = bead.WorktreeRemove
	worktreeListFn      = bead.WorktreeList
	hasRemoteFn         = gitops.HasRemote
	pushBranchFn        = gitops.PushBranch
	diffStatFn          = gitops.DiffStat
	commitCountFn       = gitops.CommitCount
	isAncestorFn        = gitops.IsAncestor
	branchExistsFn      = gitops.BranchExists
	findLocalRootFn     = defaultFindLocalRoot
	commitAllFn         = gitops.CommitAll
)

func defaultFindLocalRoot() (string, error) {
	return workspace.FindLocalRoot(".")
}

// ImplOpts holds options for implementation approval.
type ImplOpts struct{}

// ImplResult holds the result of implementation approval.
type ImplResult struct {
	SpecID      string
	Warnings    []string
	SpecBranch  string
	CommitCount int
	DiffStat    string
	Pushed      bool // true if branch was pushed to remote
}

// ApproveImpl transitions from review mode to idle, completing the spec lifecycle.
func ApproveImpl(root, specID string, opts ...ImplOpts) (*ImplResult, error) {
	if err := validate.SpecID(specID); err != nil {
		return nil, err
	}
	result := &ImplResult{SpecID: specID}

	// Find the epic for this spec directly (ADR-0023).
	// We use FindEpicBySpecID instead of ResolveContextFromDir because the
	// epic may have been auto-closed by beads (molecule completion), and
	// context resolution might not surface it correctly in all cases.
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil {
		return nil, fmt.Errorf("no epic found for spec %s: %w", specID, err)
	}

	// Verify state is review mode: all children closed, pending final merge.
	// Use DerivePhaseForImplApprove which treats closed epics without the
	// done marker as review (beads auto-closed them on molecule completion).
	epicPhase, err := phase.DerivePhase(epicID)
	if err != nil {
		return nil, fmt.Errorf("deriving phase for spec %s: %w", specID, err)
	}
	if epicPhase != state.ModeReview && epicPhase != state.ModeDone {
		return nil, fmt.Errorf("expected review mode, got %q", epicPhase)
	}
	if epicID != "" {
		if _, err := implRunBDCombinedFn("close", epicID); err != nil {
			if !isAlreadyClosedErr(err) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not close lifecycle epic %s: %v", epicID, err))
			}
		}
		// Mark epic as explicitly done so phase derivation distinguishes
		// impl-approved closure from beads auto-close (molecule completion).
		// Read existing metadata first and merge to avoid clobbering spec_num/spec_title.
		doneMetadata := `{"mindspec_done":true}`
		if out, err := implRunBDFn("show", epicID, "--json"); err == nil {
			var items []struct {
				Metadata map[string]interface{} `json:"metadata"`
			}
			if json.Unmarshal(out, &items) == nil && len(items) > 0 && items[0].Metadata != nil {
				merged := items[0].Metadata
				merged["mindspec_done"] = true
				if b, err := json.Marshal(merged); err == nil {
					doneMetadata = string(b)
				}
			}
		}
		if _, err := implRunBDCombinedFn("update", epicID, "--metadata", doneMetadata); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not set done marker on epic %s: %v", epicID, err))
		}
	}

	// Derive spec branch from convention.
	specBranch := state.SpecBranch(specID)
	specBranchExists := branchExistsFn(specBranch)

	// Preflight: verify spec branch has actual implementation content.
	if specBranchExists {
		if err := verifyImplContent(root, specBranch, specID); err != nil {
			return nil, fmt.Errorf("preflight check failed: %w", err)
		}
	}

	// Push spec branch to remote if available, then clean up locally.
	if specBranchExists {
		result.SpecBranch = specBranch

		// Gather pre-push stats (best-effort).
		if count, err := commitCountFn(root, "main", specBranch); err == nil {
			result.CommitCount = count
		}
		if stat, err := diffStatFn(root, "main", specBranch); err == nil {
			result.DiffStat = stat
		}

		// Push to remote if one exists.
		if hasRemoteFn() {
			if err := pushBranchFn(specBranch); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not push branch: %v", err))
			} else {
				result.Pushed = true
			}
		}

		// Auto-commit any remaining changes (e.g. recording artifacts) in the
		// spec worktree before cleanup, so worktree removal doesn't fail on
		// a dirty tree.
		specWtPath := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
		if err := commitAllFn(specWtPath, "chore: commit remaining spec artifacts"); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("auto-commit in spec worktree: %v", err))
		}

		// Cleanup must run from repo root. If command is invoked from inside the
		// target spec worktree, bd refuses to remove that worktree.
		if err := withWorkingDir(root, func() error {
			// Clean up lingering bead worktrees/branches from this spec first.
			if err := cleanupBeadBranchesAndWorktrees(root, specID); err != nil {
				return fmt.Errorf("cleaning bead artifacts: %w", err)
			}

			// Clean up spec worktree before merge (worktree locks the branch).
			// Tolerate already-removed worktrees (agent may have cleaned up manually).
			specWtName := "worktree-spec-" + specID
			if err := worktreeRemoveFn(specWtName); err != nil {
				if !isAlreadyRemovedErr(err) {
					return fmt.Errorf("removing spec worktree %s: %w", specWtName, err)
				}
			}

			// Merge spec branch into main for local (no-remote) workflows.
			// For remote workflows, the merge happens via PR.
			if !result.Pushed {
				if err := mergeBranchFn(root, specBranch, "main"); err != nil {
					// Non-fatal: branch may have already been merged or may have no commits
					result.Warnings = append(result.Warnings, fmt.Sprintf("could not merge %s into main: %v", specBranch, err))
				}
			}

			// Delete spec branch after merge.
			// Tolerate already-removed branches.
			if err := deleteBranchFn(specBranch); err != nil {
				if !isAlreadyRemovedErr(err) {
					return fmt.Errorf("deleting spec branch %s: %w", specBranch, err)
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	// Stop recording (best-effort — before transitioning to idle)
	if err := recording.StopRecording(root, specID); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not stop recording: %v", err))
	}

	// No focus file written per ADR-0023 — beads is the single state authority.

	return result, nil
}

// cleanupBeadBranchesAndWorktrees removes lingering bead worktrees/branches for
// the spec's implementation beads (derived from plan frontmatter bead_ids).
func cleanupBeadBranchesAndWorktrees(root, specID string) error {
	planPath := filepath.Join(workspace.SpecDir(root, specID), "plan.md")
	beadIDs, err := readPlanBeadIDs(planPath)
	if err != nil {
		return nil // nothing to clean if bead_ids are unavailable
	}

	entries, _ := worktreeListFn() // best-effort; branch deletion still runs
	type wtInfo struct {
		Name string
		Path string
	}
	branchToWorktree := map[string]wtInfo{} // bead branch -> worktree info
	for _, e := range entries {
		branchToWorktree[e.Branch] = wtInfo{Name: e.Name, Path: e.Path}
	}

	var errs []string
	for _, beadID := range beadIDs {
		beadBranch := "bead/" + beadID
		if wt := branchToWorktree[beadBranch]; wt.Name != "" {
			// Auto-commit any remaining changes before removing worktree.
			if wt.Path != "" {
				if err := commitAllFn(wt.Path, "chore: commit remaining bead artifacts"); err != nil {
					// Best-effort: log but don't block cleanup.
					errs = append(errs, fmt.Sprintf("auto-commit in bead worktree %s: %v", wt.Name, err))
				}
			}
			if err := worktreeRemoveFn(wt.Name); err != nil {
				errs = append(errs, fmt.Sprintf("remove worktree %s: %v", wt.Name, err))
			}
		}
		if err := deleteBranchFn(beadBranch); err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
			errs = append(errs, fmt.Sprintf("delete branch %s: %v", beadBranch, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
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

func readBeadStatus(id string) (string, error) {
	out, err := implRunBDFn("show", id, "--json")
	if err != nil {
		return "", err
	}

	var payload []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parsing bd show output for %s: %w", id, err)
	}
	if len(payload) == 0 {
		return "", fmt.Errorf("no bead returned for %s", id)
	}
	return strings.ToLower(strings.TrimSpace(payload[0].Status)), nil
}

// verifyImplContent checks that the spec branch has real implementation content
// before allowing the merge to main. It verifies:
// 1. The spec branch has commits beyond main.
// 2. All plan beads are closed.
// 3. Any local bead branches are ancestors of the spec branch.
func verifyImplContent(root, specBranch, specID string) error {
	// Check 1: spec branch has commits beyond main (or was already integrated).
	count, err := commitCountFn(root, "main", specBranch)
	if err != nil {
		return fmt.Errorf("checking commit count: %w", err)
	}

	// Read bead_ids from plan.md frontmatter.
	planPath := filepath.Join(workspace.SpecDir(root, specID), "plan.md")
	beadIDs, err := readPlanBeadIDs(planPath)
	if err != nil {
		// If plan.md doesn't exist or has no bead_ids, skip bead checks.
		if count == 0 {
			return fmt.Errorf("spec branch %s has no commits beyond main — nothing to merge", specBranch)
		}
		return nil
	}
	specAlreadyIntegrated := count == 0
	if specAlreadyIntegrated && len(beadIDs) == 0 {
		return fmt.Errorf("spec branch %s has no commits beyond main — nothing to merge", specBranch)
	}

	// Check 2: all plan beads are closed.
	for _, bid := range beadIDs {
		status, err := readBeadStatus(bid)
		if err != nil {
			return fmt.Errorf("checking bead %s status: %w", bid, err)
		}
		if status != "closed" {
			return fmt.Errorf("bead %s is still %q — close all beads before approving implementation", bid, status)
		}
	}

	// Check 3: bead branches are ancestors of spec branch.
	// If not, auto-merge them into the spec branch via the spec worktree.
	// Skip this if spec is already integrated in main: this approval is acting
	// as a cleanup/state-finalization step.
	if specAlreadyIntegrated {
		return nil
	}

	specWtPath := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
	for _, bid := range beadIDs {
		beadBranch := "bead/" + bid
		if !branchExistsFn(beadBranch) {
			continue
		}
		isAnc, err := isAncestorFn(root, beadBranch, specBranch)
		if err != nil {
			return fmt.Errorf("checking ancestry of %s: %w", beadBranch, err)
		}
		if !isAnc {
			// Auto-merge the bead branch into the spec branch.
			if err := mergeBranchFn(specWtPath, beadBranch, specBranch); err != nil {
				return fmt.Errorf("merging bead branch %s into spec branch %s: %w", beadBranch, specBranch, err)
			}
			fmt.Printf("Merged bead branch %s → %s\n", beadBranch, specBranch)
		}
	}

	return nil
}

// readPlanBeadIDs reads bead_ids from the plan.md YAML frontmatter.
func readPlanBeadIDs(planPath string) ([]string, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("no frontmatter found")
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return nil, fmt.Errorf("no frontmatter end marker")
	}
	fmContent := content[4 : 4+end]

	var fm struct {
		BeadIDs []string `yaml:"bead_ids"`
	}
	if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
		return nil, fmt.Errorf("parsing plan frontmatter: %w", err)
	}
	if len(fm.BeadIDs) == 0 {
		return nil, fmt.Errorf("no bead_ids in plan frontmatter")
	}
	return fm.BeadIDs, nil
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

func isAlreadyClosedErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already closed")
}
