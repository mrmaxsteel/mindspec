package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/config"
	"github.com/mindspec/mindspec/internal/gitops"
	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/specmeta"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"

	"gopkg.in/yaml.v3"
)

var (
	implRunBDCombinedFn = bead.RunBDCombined
	implRunBDFn         = bead.RunBD
	loadConfigFn        = config.Load
	mergeBranchFn       = gitops.MergeBranch
	deleteBranchFn      = gitops.DeleteBranch
	worktreeRemoveFn    = bead.WorktreeRemove
	hasRemoteFn         = gitops.HasRemote
	pushBranchFn        = gitops.PushBranch
	createPRFn          = gitops.CreatePR
	diffStatFn          = gitops.DiffStat
	commitCountFn       = gitops.CommitCount
	prStatusFn          = gitops.PRStatus
	prChecksWatchFn     = gitops.PRChecksWatch
	mergePRFn           = gitops.MergePR
	isAncestorFn        = gitops.IsAncestor
	branchExistsFn      = gitops.BranchExists
)

// ImplOpts holds options for implementation approval.
type ImplOpts struct {
	Wait bool // If true and strategy is PR, wait for CI checks then merge.
}

// ImplResult holds the result of implementation approval.
type ImplResult struct {
	SpecID        string
	Warnings      []string
	MergeStrategy string // "direct", "pr", or "" if no merge
	SpecBranch    string
	CommitCount   int
	DiffStat      string
	PRURL         string // set when strategy is "pr"
	PRMerged      bool   // true if PR was merged via --wait
}

// ApproveImpl transitions from review mode to idle, completing the spec lifecycle.
func ApproveImpl(root, specID string, opts ...ImplOpts) (*ImplResult, error) {
	var opt ImplOpts
	if len(opts) > 0 {
		opt = opts[0]
	}
	_ = opt // used below in merge flow
	result := &ImplResult{SpecID: specID}

	// Verify current state is review mode for this spec
	s, err := state.Read(root)
	if err != nil {
		return nil, fmt.Errorf("reading state: %w", err)
	}
	if s.Mode != state.ModeReview {
		return nil, fmt.Errorf("expected review mode, got %q", s.Mode)
	}
	if s.ActiveSpec != specID {
		return nil, fmt.Errorf("active spec is %q, not %q", s.ActiveSpec, specID)
	}

	// Resolve and enforce molecule binding before mutation.
	meta, err := specmeta.EnsureFullyBound(root, specID)
	if err != nil {
		return nil, fmt.Errorf("resolving molecule binding for %s: %w", specID, err)
	}

	// Close parent epic + all unique mapped steps (best-effort).
	for _, targetID := range closeoutTargets(meta) {
		status, err := readBeadStatus(targetID)
		if err == nil && status == "closed" {
			continue
		}

		if _, err := implRunBDCombinedFn("close", targetID); err != nil {
			if isAlreadyClosedErr(err) {
				continue
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not close molecule member %s: %v", targetID, err))
		}
	}

	// Preflight: verify spec branch has actual implementation content.
	if s.SpecBranch != "" {
		if err := verifyImplContent(root, s, specID); err != nil {
			return nil, fmt.Errorf("preflight check failed: %w", err)
		}
	}

	// Merge spec branch → main (ADR-0006: one PR per spec lifecycle).
	if s.SpecBranch != "" {
		cfg, cfgErr := loadConfigFn(root)
		if cfgErr != nil {
			cfg = config.DefaultConfig()
		}

		mergeErr := mergeSpecToMain(root, s, cfg, result, opt)
		if mergeErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("spec→main merge: %v", mergeErr))
		} else if result.MergeStrategy == "direct" || result.PRMerged {
			// Clean up spec worktree and branch after successful merge.
			specWtName := "worktree-spec-" + specID
			if err := worktreeRemoveFn(specWtName); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not remove spec worktree: %v", err))
			}
			if err := deleteBranchFn(s.SpecBranch); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not delete spec branch: %v", err))
			}
		}
	}

	// Stop recording (best-effort — before transitioning to idle)
	if err := recording.StopRecording(root, specID); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not stop recording: %v", err))
	}

	// Transition to idle
	idleState := &state.State{Mode: state.ModeIdle}
	if err := state.Write(root, idleState); err != nil {
		return nil, fmt.Errorf("setting state to idle: %w", err)
	}

	return result, nil
}

func closeoutTargets(meta *specmeta.Meta) []string {
	seen := map[string]struct{}{}
	var targets []string

	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		targets = append(targets, id)
	}

	add(meta.MoleculeID)

	var remaining []string
	for _, id := range meta.StepMapping {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		remaining = append(remaining, id)
	}
	sort.Strings(remaining)
	targets = append(targets, remaining...)

	return targets
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

// mergeSpecToMain merges the spec branch to main using the configured strategy.
// It populates result with merge metadata (strategy, stats, PR URL).
func mergeSpecToMain(root string, s *state.State, cfg *config.Config, result *ImplResult, opt ImplOpts) error {
	strategy := cfg.MergeStrategy

	// "auto" resolves to "pr" if a remote exists, "direct" otherwise.
	if strategy == "auto" {
		if hasRemoteFn() {
			strategy = "pr"
		} else {
			strategy = "direct"
		}
	}

	result.MergeStrategy = strategy
	result.SpecBranch = s.SpecBranch

	// Gather pre-merge stats (best-effort).
	if count, err := commitCountFn(root, "main", s.SpecBranch); err == nil {
		result.CommitCount = count
	}
	if stat, err := diffStatFn(root, "main", s.SpecBranch); err == nil {
		result.DiffStat = stat
	}

	switch strategy {
	case "pr":
		if err := pushBranchFn(s.SpecBranch); err != nil {
			return fmt.Errorf("pushing spec branch: %w", err)
		}
		title := fmt.Sprintf("[SPEC %s] Merge spec branch to main", s.ActiveSpec)
		body := fmt.Sprintf("Automated PR for spec %s lifecycle completion.", s.ActiveSpec)
		prURL, err := createPRFn(s.SpecBranch, "main", title, body)
		if err != nil {
			return fmt.Errorf("creating PR: %w", err)
		}
		result.PRURL = prURL

		if opt.Wait {
			fmt.Printf("Waiting for CI checks on %s...\n", prURL)
			if err := prChecksWatchFn(prURL); err != nil {
				return fmt.Errorf("CI checks failed: %w", err)
			}
			if err := mergePRFn(prURL); err != nil {
				return fmt.Errorf("merging PR: %w", err)
			}
			result.PRMerged = true
		}
		return nil

	case "direct":
		return mergeBranchFn(root, s.SpecBranch, "main")

	default:
		return fmt.Errorf("unknown merge strategy: %s", strategy)
	}
}

// verifyImplContent checks that the spec branch has real implementation content
// before allowing the merge to main. It verifies:
// 1. The spec branch has commits beyond main.
// 2. All plan beads are closed.
// 3. Any local bead branches are ancestors of the spec branch.
func verifyImplContent(root string, s *state.State, specID string) error {
	// Check 1: spec branch has commits beyond main.
	count, err := commitCountFn(root, "main", s.SpecBranch)
	if err != nil {
		return fmt.Errorf("checking commit count: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("spec branch %s has no commits beyond main — nothing to merge", s.SpecBranch)
	}

	// Read bead_ids from plan.md frontmatter.
	planPath := filepath.Join(workspace.SpecDir(root, specID), "plan.md")
	beadIDs, err := readPlanBeadIDs(planPath)
	if err != nil {
		// If plan.md doesn't exist or has no bead_ids, skip bead checks.
		return nil
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
	for _, bid := range beadIDs {
		beadBranch := "bead/" + bid
		if !branchExistsFn(beadBranch) {
			continue
		}
		isAnc, err := isAncestorFn(root, beadBranch, s.SpecBranch)
		if err != nil {
			return fmt.Errorf("checking ancestry of %s: %w", beadBranch, err)
		}
		if !isAnc {
			return fmt.Errorf("bead branch %s has commits not merged into spec branch %s — run `git merge %s` on the spec branch first", beadBranch, s.SpecBranch, beadBranch)
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

func isAlreadyClosedErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already closed")
}
