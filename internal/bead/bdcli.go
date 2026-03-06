package bead

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/trace"
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

// RunBD executes a bd command and returns stdout. Stderr is captured for error
// messages but not mixed into the output (important for JSON parsing).
// This is the primary interface for composing with bd per ADR-0012.
func RunBD(args ...string) ([]byte, error) {
	return tracedOutput("run", args)
}

// RunBDCombined executes a bd command and returns combined stdout+stderr.
// Use for commands that don't return JSON where stderr output is useful.
func RunBDCombined(args ...string) ([]byte, error) {
	return tracedCombined("run", args)
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

// --- Worktree wrappers (genuine multi-step helpers per ADR-0012) ---

// WorktreeListEntry represents a worktree from `bd worktree list --json`.
type WorktreeListEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	IsMain     bool   `json:"is_main"`
	BeadsState string `json:"beads_state"`
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
