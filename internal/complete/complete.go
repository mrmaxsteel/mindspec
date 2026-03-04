package complete

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/gitops"
	"github.com/mindspec/mindspec/internal/next"
	"github.com/mindspec/mindspec/internal/phase"
	"github.com/mindspec/mindspec/internal/recording"
	"github.com/mindspec/mindspec/internal/resolve"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/validate"
	"github.com/mindspec/mindspec/internal/workspace"
)

// Package-level function variables for testability.
var (
	closeBeadFn         = bead.Close
	worktreeListFn      = bead.WorktreeList
	worktreeRemoveFn    = bead.WorktreeRemove
	runBDFn             = bead.RunBD
	execCommandFn       = exec.Command
	mergeIntoFn         = gitops.MergeInto
	deleteBranchFn      = gitops.DeleteBranch
	commitAllFn         = gitops.CommitAll
	resolveTargetFn     = resolve.ResolveTarget
	resolveActiveBeadFn = next.ResolveActiveBead
	findLocalRootFn     = defaultFindLocalRoot
	fetchBeadByIDFn     = next.FetchBeadByID
	findRecentClosedFn  = findRecentClosed
)

// Result summarizes what mindspec complete did.
type Result struct {
	BeadID          string
	BeadClosed      bool
	WorktreeRemoved bool
	NextMode        string
	NextBead        string
	NextSpec        string
}

func defaultFindLocalRoot() (string, error) {
	return workspace.FindLocalRoot(".")
}

// Run orchestrates bead completion: close bead, remove worktree, advance state.
// root is the main repo root (for spec dirs, lifecycle, merges).
// Focus is read from localRoot (per-worktree focus).
// specIDHint is optional and typically comes from --spec for disambiguation.
func Run(root, beadID, specIDHint, commitMsg string) (*Result, error) {
	// Backward-compatible UX: `mindspec complete <spec-id>` should behave like
	// `mindspec complete --spec=<spec-id>`, not treat the spec ID as a bead ID.
	if beadID != "" && specIDHint == "" && validate.SpecID(beadID) == nil {
		specIDHint = beadID
		beadID = ""
	}

	// Determine local root for per-worktree focus reads.
	localRoot := root
	if lr, err := findLocalRootFn(); err == nil {
		localRoot = lr
	}

	// 1. Derive activeSpec from resolver, activeBead from arg or Beads query
	// Try localRoot first (per-worktree focus) then fall back to root.
	specID, err := resolveTargetFn(localRoot, specIDHint)
	if err != nil && localRoot != root {
		specID, err = resolveTargetFn(root, specIDHint)
	}
	if err != nil {
		return nil, fmt.Errorf("resolving active spec: %w", err)
	}
	if beadID == "" {
		beadID, err = resolveActiveBeadFn(root, specID)
		if err != nil {
			return nil, fmt.Errorf("resolving active bead: %w", err)
		}
	}
	// If no in-progress bead found, check for recently closed beads that
	// may need cleanup (e.g., agent ran `bd close` directly).
	if beadID == "" {
		beadID, err = findRecentClosedFn(specID)
		if err != nil {
			return nil, fmt.Errorf("resolving active bead: %w", err)
		}
	}
	if beadID == "" {
		return nil, fmt.Errorf("no bead ID provided and no in-progress bead found for spec %s", specID)
	}

	// Derive spec branch and worktree paths from conventions
	specBranch := state.SpecBranch(specID)

	// 2. Find worktree matching bead
	var wtName, wtPath string
	entries, err := worktreeListFn()
	if err == nil {
		expectedName := "worktree-" + beadID
		expectedBranch := "bead/" + beadID
		for _, e := range entries {
			if e.Name == expectedName || e.Branch == expectedBranch {
				wtName = e.Name
				wtPath = e.Path
				break
			}
		}
	}

	// 2.5. Auto-commit if commit message provided
	commitPath := wtPath
	if commitPath == "" {
		commitPath = root
	}
	if commitMsg != "" {
		msg := fmt.Sprintf("impl(%s): %s", beadID, commitMsg)
		if err := commitAllFn(commitPath, msg); err != nil {
			return nil, fmt.Errorf("auto-commit failed: %w", err)
		}
	}

	// 3. Check clean tree
	checkPath := wtPath
	if checkPath == "" {
		checkPath = root // No worktree — check main tree
	}
	if err := checkCleanWorktree(checkPath); err != nil {
		if wtPath == "" {
			return nil, fmt.Errorf("%w\nhint: no active bead worktree is set. Run `mindspec next`, `cd` into the printed worktree path, then commit and rerun `mindspec complete`", err)
		}
		return nil, fmt.Errorf("%w\nhint: use `mindspec complete \"describe what you did\"` to auto-commit", err)
	}

	// 4. Close bead (idempotent: tolerate already-closed beads)
	if err := closeBeadFn(beadID); err != nil {
		// Check if the bead is already closed — if so, warn and continue cleanup.
		info, fetchErr := fetchBeadByIDFn(beadID)
		if fetchErr == nil && strings.EqualFold(strings.TrimSpace(info.Status), "closed") {
			fmt.Printf("Warning: bead %s already closed — performing merge and cleanup.\n", beadID)
		} else {
			return nil, fmt.Errorf("closing bead: %w", err)
		}
	}

	// 4.5. Emit recording bead marker (best-effort)
	if specID != "" {
		_ = recording.EmitBeadMarker(root, specID, "complete", beadID)
	}

	result := &Result{
		BeadID:     beadID,
		BeadClosed: true,
	}

	// 4.7. Merge bead branch back to spec branch (ADR-0006).
	// Use MergeInto targeting the spec worktree (which already has specBranch
	// checked out) instead of MergeBranch (which tries to checkout specBranch
	// and fails when the bead worktree is nested inside the spec worktree).
	beadBranch := "bead/" + beadID
	specWtPath := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
	if _, err := os.Stat(specWtPath); err == nil {
		if err := mergeIntoFn(specWtPath, beadBranch); err != nil {
			fmt.Printf("Warning: could not merge %s → %s: %v\n", beadBranch, specBranch, err)
		}
	}

	// 5. Remove worktree (if found)
	if wtName != "" {
		if err := worktreeRemoveFn(wtName); err != nil {
			fmt.Printf("Warning: could not remove worktree %s: %v\n", wtName, err)
		} else {
			result.WorktreeRemoved = true
		}
	}

	// 5.5. Delete the bead branch after merge + worktree removal (best-effort).
	if err := deleteBranchFn(beadBranch); err != nil {
		fmt.Printf("Warning: could not delete branch %s: %v\n", beadBranch, err)
	}

	// 6. Advance state
	nextMode, nextBead := advanceState(specID)
	result.NextMode = nextMode
	result.NextBead = nextBead
	result.NextSpec = specID

	// ADR-0023: no focus write — state is derived from beads.
	if nextMode == state.ModeIdle {
		result.NextSpec = ""
	}

	return result, nil
}

// FormatResult returns a human-readable summary of the completion.
func FormatResult(r *Result) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Bead %s closed.\n", r.BeadID)
	if r.WorktreeRemoved {
		sb.WriteString("Worktree removed.\n")
	}
	switch r.NextMode {
	case state.ModeImplement:
		fmt.Fprintf(&sb, "Next bead: %s (mode: implement)\n", r.NextBead)
		sb.WriteString("Run `mindspec next` to claim and start.\n")
	case state.ModePlan:
		fmt.Fprintf(&sb, "Remaining beads are blocked. Mode: plan (spec: %s)\n", r.NextSpec)
	case state.ModeReview:
		fmt.Fprintf(&sb, "All beads complete. Mode: review (spec: %s)\n", r.NextSpec)
		sb.WriteString("Review implementation against acceptance criteria, then use `/ms-impl-approve` to accept.\n")
	default:
		sb.WriteString("All beads complete. Mode: idle\n")
	}
	return sb.String()
}

// autoCommitStateFiles stages and commits any dirty .mindspec/ state files
// so they don't block the clean-worktree check. These files are written by
// `mindspec next` and are not user content.
func autoCommitStateFiles(path string) error {
	// Stage .mindspec/ files
	add := execCommandFn("git", "-C", path, "add", ".mindspec/")
	if err := add.Run(); err != nil {
		return nil // .mindspec/ may not exist — not an error
	}
	// Check if anything was staged
	diff := execCommandFn("git", "-C", path, "diff", "--cached", "--quiet")
	if diff.Run() == nil {
		return nil // nothing staged
	}
	// Commit the staged state files
	commit := execCommandFn("git", "-C", path, "commit", "-m", "mindspec: state checkpoint")
	if out, err := commit.CombinedOutput(); err != nil {
		return fmt.Errorf("committing state files: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// checkCleanWorktree verifies a worktree has no uncommitted changes.
func checkCleanWorktree(path string) error {
	cmd := execCommandFn("git", "-C", path, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("checking worktree status: %w", err)
	}
	rawOut := strings.TrimRight(string(out), "\n")
	if strings.TrimSpace(rawOut) == "" {
		return nil
	}
	lines := strings.Split(rawOut, "\n")
	var blocking []string
	for _, line := range lines {
		raw := strings.TrimRight(line, "\r")
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if !isIgnorableStateChange(raw) {
			blocking = append(blocking, strings.TrimSpace(raw))
		}
	}
	if len(blocking) > 0 {
		msg := fmt.Sprintf("worktree has uncommitted changes — commit before completing:\n%s", strings.Join(blocking, "\n"))
		if hasManualWorktreeMeta(path, blocking) {
			msg += "\nhint: worktree metadata changes detected. If you created a worktree manually, clean those changes and use `mindspec next` to claim/switch work."
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func hasManualWorktreeMeta(basePath string, blocking []string) bool {
	for _, line := range blocking {
		statusPath := line
		if len(statusPath) >= 3 {
			statusPath = strings.TrimSpace(statusPath[3:])
		}
		if strings.Contains(statusPath, " -> ") {
			parts := strings.Split(statusPath, " -> ")
			statusPath = strings.TrimSpace(parts[len(parts)-1])
		}
		if strings.Contains(statusPath, ".gitignore") ||
			strings.Contains(statusPath, ".worktrees/") ||
			strings.Contains(strings.ToLower(statusPath), "worktree") {
			return true
		}

		absPath := filepath.Join(basePath, statusPath)
		if info, err := os.Stat(absPath); err == nil && info.IsDir() {
			if _, err := os.Stat(filepath.Join(absPath, ".git")); err == nil {
				return true
			}
		}
	}
	return false
}

func isIgnorableStateChange(statusLine string) bool {
	// Porcelain format: XY<space>PATH (or XY<space>OLD -> NEW)
	path := strings.TrimRight(statusLine, "\r")
	if len(path) >= 3 {
		path = strings.TrimSpace(path[3:])
	} else {
		path = strings.TrimSpace(path)
	}
	if strings.Contains(path, " -> ") {
		parts := strings.Split(path, " -> ")
		path = strings.TrimSpace(parts[len(parts)-1])
	}

	if path == ".mindspec/focus" || path == ".mindspec/session.json" {
		return true
	}
	if strings.Contains(path, "/recording/") {
		return true
	}
	return false
}

// findRecentClosed looks for a recently closed bead under the spec's epic
// that still has a bead branch (indicating it was closed without cleanup).
func findRecentClosed(specID string) (string, error) {
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		return "", nil
	}

	out, err := runBDFn("list", "--parent", epicID, "--status=closed", "--json")
	if err != nil {
		return "", nil
	}

	var items []bead.BeadInfo
	if json.Unmarshal(out, &items) != nil {
		return "", nil
	}

	// Return the first closed bead that still has a bead branch (unmerged).
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id != "" && gitops.BranchExists("bead/"+id) {
			return id, nil
		}
	}
	return "", nil
}

// advanceState determines the next mode after completing a bead.
func advanceState(specID string) (mode, nextBead string) {
	if specID == "" {
		return state.ModeIdle, ""
	}

	// Find epic via beads metadata query (ADR-0023).
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		return state.ModeIdle, ""
	}

	// Check for ready children under the epic
	out, err := runBDFn("ready", "--parent", epicID, "--json")
	if err == nil {
		var ready []bead.BeadInfo
		if json.Unmarshal(out, &ready) == nil && len(ready) > 0 {
			return state.ModeImplement, ready[0].ID
		}
	}

	// Check for open (but blocked) children — match actual title format [specID]
	implPrefix := "[" + specID + "]"
	out, err = runBDFn("search", implPrefix, "--json", "--status=open")
	if err == nil {
		var open []bead.BeadInfo
		if json.Unmarshal(out, &open) == nil && len(open) > 0 {
			return state.ModePlan, ""
		}
	}

	// All beads done → review gate (human must approve before idle)
	return state.ModeReview, ""
}
