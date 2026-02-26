package hook

import "github.com/mindspec/mindspec/internal/state"

// Run dispatches to the named hook and returns its result.
func Run(name string, inp *Input, st *state.State, enforce bool) Result {
	switch name {
	case "plan-gate-exit":
		return PlanGateExit(inp, st)
	case "plan-gate-enter":
		return PlanGateEnter(inp, st)
	case "worktree-file":
		return WorktreeFile(inp, st, enforce)
	case "worktree-bash":
		return WorktreeBash(inp, st, enforce)
	case "needs-clear":
		return NeedsClear(inp, st)
	case "workflow-guard":
		return WorkflowGuard(inp, st, enforce)
	default:
		return Result{Action: Pass}
	}
}

// Stub implementations — replaced in Beads 2 and 3.

// PlanGateExit blocks ExitPlanMode when in plan mode.
func PlanGateExit(_ *Input, st *state.State) Result {
	if st == nil {
		return Result{Action: Pass}
	}
	if st.Mode == state.ModePlan {
		return Result{
			Action:  Block,
			Message: "MindSpec plan mode is active. Do NOT exit plan mode directly. Run /ms-plan-approve to validate the plan, create beads, and transition to implementation.",
		}
	}
	return Result{Action: Pass}
}

// PlanGateEnter injects additionalContext when entering plan mode.
func PlanGateEnter(_ *Input, st *state.State) Result {
	if st == nil {
		return Result{Action: Pass}
	}
	if st.Mode == state.ModePlan {
		return Result{
			Action:  Warn,
			Message: "MindSpec plan mode is active. Write your plan to docs/specs/*/plan.md. When complete, use /ms-plan-approve — do NOT use ExitPlanMode directly.",
		}
	}
	return Result{Action: Pass}
}

// WorktreeFile blocks file writes outside the active worktree.
func WorktreeFile(inp *Input, st *state.State, enforce bool) Result {
	if st == nil || st.ActiveWorktree == "" || !enforce {
		return Result{Action: Pass}
	}
	if inp.FilePath == "" {
		return Result{Action: Pass}
	}
	// Allow files within the worktree or the main repo root
	wt := st.ActiveWorktree
	if hasPathPrefix(inp.FilePath, wt) {
		return Result{Action: Pass}
	}
	// Also allow main repo path (shared git content)
	cwd, _ := getCwd()
	if cwd != "" && hasPathPrefix(inp.FilePath, cwd) {
		return Result{Action: Pass}
	}
	return Result{
		Action:  Block,
		Message: "mindspec: blocked — file " + inp.FilePath + " is outside active worktree " + wt + ". Switch to: cd " + wt,
	}
}

// WorktreeBash blocks bash commands outside the active worktree.
func WorktreeBash(inp *Input, st *state.State, enforce bool) Result {
	if st == nil || st.ActiveWorktree == "" || !enforce {
		return Result{Action: Pass}
	}
	cmd := stripEnvPrefixes(inp.Command)
	if isAllowedCommand(cmd) {
		return Result{Action: Pass}
	}
	cwd, _ := getCwd()
	wt := st.ActiveWorktree
	if cwd != "" && hasPathPrefix(cwd, wt) {
		return Result{Action: Pass}
	}
	return Result{
		Action:  Block,
		Message: "mindspec: blocked — your working directory is the main worktree. Run: cd " + wt,
	}
}

// NeedsClear blocks `mindspec next` when needs_clear is set.
func NeedsClear(inp *Input, st *state.State) Result {
	if st == nil || !st.NeedsClear {
		return Result{Action: Pass}
	}
	cmd := inp.Command
	if !containsWord(cmd, "mindspec next") {
		return Result{Action: Pass}
	}
	if containsWord(cmd, "--force") {
		return Result{Action: Pass}
	}
	return Result{
		Action:  Block,
		Message: "needs_clear is set. Run /clear to reset your context, then retry mindspec next. Use --force to bypass.",
	}
}

// WorkflowGuard is the universal state-aware guard (Bead 3).
func WorkflowGuard(_ *Input, _ *state.State, _ bool) Result {
	// Stub — implemented in Bead 3
	return Result{Action: Pass}
}
