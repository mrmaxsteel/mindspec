package bead

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mindspec/mindspec/internal/trace"
)

// execCommand is a package-level variable for testability.
// Tests override this to capture arguments or return canned output.
var execCommand = exec.Command

// BeadInfo represents a work item from the Beads CLI.
type BeadInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    int    `json:"priority"`
	IssueType   string `json:"issue_type"`
	Owner       string `json:"owner"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Preflight checks prerequisites for bead commands:
// git repo, .beads/ directory, bd on PATH.
func Preflight(root string) error {
	// Check git repo
	cmd := execCommand("git", "rev-parse", "--git-dir")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("not a git repository (run 'git init'): %s", string(out))
	}

	// Check .beads/ exists
	beadsDir := filepath.Join(root, ".beads")
	if _, err := os.Stat(beadsDir); os.IsNotExist(err) {
		return fmt.Errorf(".beads/ directory not found (run 'beads init' to initialize)")
	}

	// Check bd on PATH
	if _, err := exec.LookPath("bd"); err != nil {
		return fmt.Errorf("bd not found on PATH (install Beads: https://github.com/beads-project/beads)")
	}

	return nil
}

// Create creates a new bead via `bd create` and returns the created bead info.
func Create(title, desc, issueType string, priority int, parent string) (*BeadInfo, error) {
	args := []string{"create", title,
		"--description=" + desc,
		"--type=" + issueType,
		fmt.Sprintf("--priority=%d", priority),
		"--json",
	}
	if parent != "" {
		args = append(args, "--parent="+parent)
	}

	out, err := tracedOutput("create", args)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd create failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bd create failed: %w", err)
	}

	var info BeadInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("parsing bd create output: %w", err)
	}
	return &info, nil
}

// Search searches for beads matching query, returning only open beads.
func Search(query string) ([]BeadInfo, error) {
	args := []string{"search", query, "--json", "--status=open"}
	out, err := tracedOutput("search", args)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd search failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bd search failed: %w", err)
	}

	var items []BeadInfo
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parsing bd search output: %w", err)
	}
	return items, nil
}

// SearchAny searches for beads matching query, returning beads of any status.
func SearchAny(query string) ([]BeadInfo, error) {
	args := []string{"search", query, "--json"}
	out, err := tracedOutput("search-any", args)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd search failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bd search failed: %w", err)
	}

	var items []BeadInfo
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parsing bd search output: %w", err)
	}
	return items, nil
}

// Show returns details for a single bead by ID.
func Show(id string) (*BeadInfo, error) {
	args := []string{"show", id, "--json"}
	out, err := tracedOutput("show", args)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd show failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bd show failed: %w", err)
	}

	var info BeadInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("parsing bd show output: %w", err)
	}
	return &info, nil
}

// ListOpen returns all open beads.
func ListOpen() ([]BeadInfo, error) {
	args := []string{"list", "--status=open", "--json"}
	out, err := tracedOutput("list-open", args)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd list failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bd list failed: %w", err)
	}

	var items []BeadInfo
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parsing bd list output: %w", err)
	}
	return items, nil
}

// DepAdd adds a dependency: blocked depends on blocker.
func DepAdd(blocked, blocker string) error {
	args := []string{"dep", "add", blocked, blocker}
	out, err := tracedCombined("dep-add", args)
	if err != nil {
		return fmt.Errorf("bd dep add failed: %s", string(out))
	}
	return nil
}

// Update changes a bead's status.
func Update(id, status string) error {
	args := []string{"update", id, "--status=" + status}
	out, err := tracedCombined("update", args)
	if err != nil {
		return fmt.Errorf("bd update failed: %s", string(out))
	}
	return nil
}

// Close closes one or more beads via `bd close`.
func Close(ids ...string) error {
	if len(ids) == 0 {
		return fmt.Errorf("Close requires at least one bead ID")
	}
	args := append([]string{"close"}, ids...)
	out, err := tracedCombined("close", args)
	if err != nil {
		return fmt.Errorf("bd close failed: %s", string(out))
	}
	return nil
}

// --- Worktree wrappers (delegate to bd worktree) ---

// WorktreeListEntry represents a worktree from `bd worktree list --json`.
type WorktreeListEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	IsMain     bool   `json:"is_main"`
	BeadsState string `json:"beads_state"`
}

// WorktreeInfoResult represents `bd worktree info --json` output.
type WorktreeInfoResult struct {
	IsWorktree bool   `json:"is_worktree"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	BeadsState string `json:"beads_state"`
	MainRepo   string `json:"main_repo"`
}

// WorktreeCreate creates a worktree via `bd worktree create`.
// Beads handles git worktree creation, redirect setup, and .gitignore.
func WorktreeCreate(name, branch string) error {
	args := []string{"worktree", "create", name}
	if branch != "" {
		args = append(args, "--branch="+branch)
	}
	out, err := tracedCombined("worktree-create", args)
	if err != nil {
		return fmt.Errorf("bd worktree create failed: %s", string(out))
	}
	return nil
}

// WorktreeList returns all worktrees via `bd worktree list --json`.
func WorktreeList() ([]WorktreeListEntry, error) {
	args := []string{"worktree", "list", "--json"}
	out, err := tracedOutput("worktree-list", args)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd worktree list failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bd worktree list failed: %w", err)
	}

	var entries []WorktreeListEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("parsing bd worktree list output: %w", err)
	}
	return entries, nil
}

// WorktreeRemove removes a worktree via `bd worktree remove`.
// Beads performs safety checks (uncommitted changes, unpushed commits).
// When no git remote is configured, --force is passed to skip the
// unpushed-commits check (which would always fail without a remote).
func WorktreeRemove(name string) error {
	args := []string{"worktree", "remove", name}
	if !hasGitRemote() {
		args = append(args, "--force")
	}
	out, err := tracedCombined("worktree-remove", args)
	if err != nil {
		return fmt.Errorf("bd worktree remove failed: %s", string(out))
	}
	return nil
}

// hasGitRemote returns true if at least one git remote is configured.
func hasGitRemote() bool {
	cmd := execCommand("git", "remote")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// WorktreeInfo returns info about the current worktree via `bd worktree info --json`.
func WorktreeInfo() (*WorktreeInfoResult, error) {
	args := []string{"worktree", "info", "--json"}
	out, err := tracedOutput("worktree-info", args)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd worktree info failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bd worktree info failed: %w", err)
	}

	var info WorktreeInfoResult
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("parsing bd worktree info output: %w", err)
	}
	return &info, nil
}

// --- Molecule wrappers (delegate to bd mol / bd ready) ---

// MolReady returns ready (unblocked) children within a molecule.
// Uses `bd ready --parent <parentID> --json` to find work items
// whose dependencies are satisfied within the molecule.
func MolReady(parentID string) ([]BeadInfo, error) {
	args := []string{"ready", "--parent", parentID, "--json"}
	out, err := tracedOutput("mol-ready", args)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd ready --parent failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bd ready --parent failed: %w", err)
	}

	var items []BeadInfo
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parsing bd ready --parent output: %w", err)
	}
	return items, nil
}

// MolShow returns the molecule structure as raw JSON bytes.
// Uses `bd mol show <id> --json`.
func MolShow(id string) ([]byte, error) {
	args := []string{"mol", "show", id, "--json"}
	out, err := tracedOutput("mol-show", args)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd mol show failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bd mol show failed: %w", err)
	}
	return out, nil
}

// tracedOutput runs a bd command via cmd.Output() with trace instrumentation.
func tracedOutput(op string, args []string) ([]byte, error) {
	start := time.Now()
	cmd := execCommand("bd", args...)
	out, err := cmd.Output()
	trace.Emit(trace.NewEvent("bead.cli").
		WithDuration(time.Since(start)).
		WithData(map[string]any{
			"op":   op,
			"args": args,
			"ok":   err == nil,
		}))
	return out, err
}

// tracedCombined runs a bd command via cmd.CombinedOutput() with trace instrumentation.
func tracedCombined(op string, args []string) ([]byte, error) {
	start := time.Now()
	cmd := execCommand("bd", args...)
	out, err := cmd.CombinedOutput()
	trace.Emit(trace.NewEvent("bead.cli").
		WithDuration(time.Since(start)).
		WithData(map[string]any{
			"op":   op,
			"args": args,
			"ok":   err == nil,
		}))
	return out, err
}

// parseBeadList parses JSON output containing a list of BeadInfo.
func parseBeadList(data []byte) ([]BeadInfo, error) {
	var items []BeadInfo
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parsing beads JSON: %w", err)
	}
	return items, nil
}

// parseJSON unmarshals JSON data into the given target.
func parseJSON(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}
