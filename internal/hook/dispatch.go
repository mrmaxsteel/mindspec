package hook

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mindspec/mindspec/internal/config"
	"github.com/mindspec/mindspec/internal/workspace"
)

// HookState holds the subset of workflow state that hooks need.
// Constructed from phase context + session.json.
type HookState struct {
	Mode             string
	ActiveSpec       string
	ActiveWorktree   string
	SessionSource    string
	SessionStartedAt string
	BeadClaimedAt    string
}

// Run dispatches to the named hook and returns its result.
func Run(name string, inp *Input, st *HookState, enforce bool) Result {
	switch name {
	case "pre-commit":
		return runPreCommit(st)
	default:
		return Result{Action: Pass}
	}
}

// runPreCommit implements branch protection: blocks commits on protected
// branches when mindspec is active (mode != idle).
// Escape hatch: MINDSPEC_ALLOW_MAIN=1 git commit
func runPreCommit(st *HookState) Result {
	// Escape hatch
	if os.Getenv("MINDSPEC_ALLOW_MAIN") == "1" {
		return Result{Action: Pass}
	}

	// No state or idle mode → allow
	if st == nil || st.Mode == "" || st.Mode == "idle" {
		return Result{Action: Pass}
	}

	// Load config for protected branches and enforcement setting
	root, err := workspace.FindLocalRoot(".")
	if err != nil {
		return Result{Action: Pass}
	}
	cfg, err := config.Load(root)
	if err != nil {
		return Result{Action: Pass}
	}

	// Check enforcement config
	if !cfg.Enforcement.PreCommitHook {
		return Result{Action: Pass}
	}

	// Get current branch
	branch := getCurrentBranch()
	if branch == "" {
		return Result{Action: Pass}
	}

	// Block: on a spec/ branch during implement mode — code belongs on bead branches.
	if st.Mode == "implement" && strings.HasPrefix(branch, "spec/") {
		msg := fmt.Sprintf("mindspec: commits on spec branch '%s' are blocked during implement mode.\n  Implementation code belongs on bead branches.", branch)
		msg += "\n  Run: mindspec next   (to claim a bead and create a bead worktree)"
		if st.ActiveWorktree != "" {
			msg += fmt.Sprintf("\n  Or switch to your bead worktree: cd %s", st.ActiveWorktree)
		}
		msg += "\n  Escape hatch: MINDSPEC_ALLOW_MAIN=1 git commit ..."
		return Result{Action: Block, Message: msg}
	}

	// Check if branch is protected
	if !cfg.IsProtectedBranch(branch) {
		return Result{Action: Pass}
	}

	// Block: on a protected branch while mindspec is active
	msg := fmt.Sprintf("mindspec: commits on '%s' are blocked while mindspec is active (mode: %s).", branch, st.Mode)
	if st.ActiveWorktree != "" {
		msg += fmt.Sprintf("\n  Switch to your worktree: cd %s", st.ActiveWorktree)
	}
	msg += "\n  Escape hatch: MINDSPEC_ALLOW_MAIN=1 git commit ..."
	return Result{Action: Block, Message: msg}
}

// getCurrentBranch returns the current git branch name.
func getCurrentBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
