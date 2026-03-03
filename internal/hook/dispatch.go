package hook

import (
	"fmt"
	"strings"

	"github.com/mindspec/mindspec/internal/state"
)

// HookState holds the subset of workflow state that hooks need.
// Constructed from focus + session.json.
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
	case "plan-gate-exit":
		return PlanGateExit(inp, st)
	case "plan-gate-enter":
		return PlanGateEnter(inp, st)
	case "worktree-file":
		return WorktreeFile(inp, st, enforce)
	case "worktree-bash":
		return WorktreeBash(inp, st, enforce)
	case "needs-clear":
		return SessionFreshnessGate(inp, st)
	case "workflow-guard":
		return WorkflowGuard(inp, st, enforce)
	default:
		return Result{Action: Pass}
	}
}

// PlanGateExit blocks ExitPlanMode when in plan mode.
func PlanGateExit(_ *Input, st *HookState) Result {
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
func PlanGateEnter(_ *Input, st *HookState) Result {
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
func WorktreeFile(inp *Input, st *HookState, enforce bool) Result {
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
func WorktreeBash(inp *Input, st *HookState, enforce bool) Result {
	if st == nil || st.ActiveWorktree == "" || !enforce {
		return Result{Action: Pass}
	}
	cwd, _ := getCwd()
	cmd := stripEnvPrefixes(inp.Command)
	if op, branch, block := protectedGitWrite(inp.Command, cmd, cwd); block {
		return Result{
			Action: Block,
			Message: fmt.Sprintf(
				"mindspec: blocked — git %s on protected branch %q bypasses lifecycle guards. Use mindspec lifecycle commands, or switch to: cd %s",
				op, branch, st.ActiveWorktree,
			),
		}
	}
	if isAllowedCommand(cmd) {
		return Result{Action: Pass}
	}
	wt := st.ActiveWorktree
	if cwd != "" && hasPathPrefix(cwd, wt) {
		return Result{Action: Pass}
	}
	// Also allow CWD inside the spec worktree — lifecycle commands
	// (complete, impl-approve) need to run there after beads are done.
	if st.ActiveSpec != "" && cwd != "" {
		specWtSuffix := "/worktree-spec-" + st.ActiveSpec
		if strings.HasSuffix(cwd, specWtSuffix) || strings.Contains(cwd, specWtSuffix+"/") {
			return Result{Action: Pass}
		}
	}
	return Result{
		Action:  Block,
		Message: "mindspec: blocked — your working directory is the main worktree. Run: cd " + wt,
	}
}

// SessionFreshnessGate blocks `mindspec next` when the session is not fresh.
// A session is fresh if SessionSource is "startup" or "clear" and no bead
// has been claimed since the last session start.
func SessionFreshnessGate(inp *Input, st *HookState) Result {
	if st == nil || st.SessionStartedAt == "" {
		// No session metadata — non-Claude-Code environment, skip gate.
		return Result{Action: Pass}
	}
	cmd := inp.Command
	if !containsWord(cmd, "mindspec next") {
		return Result{Action: Pass}
	}
	if containsWord(cmd, "--force") {
		return Result{Action: Pass}
	}

	if st.SessionSource == "resume" || st.SessionSource == "compact" {
		return Result{
			Action:  Block,
			Message: "session freshness gate: session was " + st.SessionSource + " (not fresh). Run /clear to reset your context, then retry mindspec next. Use --force to bypass.",
		}
	}
	if st.BeadClaimedAt != "" && st.BeadClaimedAt >= st.SessionStartedAt {
		return Result{
			Action:  Block,
			Message: "session freshness gate: a bead was already claimed in this session. Run /clear to reset your context, then retry mindspec next. Use --force to bypass.",
		}
	}
	return Result{Action: Pass}
}

// WorkflowGuard is the universal state-aware guard.
// It checks the current mode and the target file/command, then responds with
// graduated enforcement: hard blocks for clear violations, warnings for grey areas.
func WorkflowGuard(inp *Input, st *HookState, enforce bool) Result {
	if st == nil || !enforce {
		return Result{Action: Pass}
	}
	// Workflow guard is for file edits only; bash commands are handled by WorktreeBash.
	if inp.FilePath == "" {
		return Result{Action: Pass}
	}
	switch st.Mode {
	case state.ModeIdle, "":
		return Result{
			Action:  Block,
			Message: blockIdle,
		}

	case state.ModeSpec:
		if isCodeFile(inp.FilePath) {
			if outsideActiveWorktree(st) {
				return Result{
					Action:  Warn,
					Message: warnOutsideWorktreeCode,
				}
			}
			return Result{
				Action:  Block,
				Message: "mindspec: blocked — code edits are not allowed during Spec Mode. Only documentation files may be edited.",
			}
		}
		return Result{Action: Pass}

	case state.ModePlan:
		if isCodeFile(inp.FilePath) {
			if outsideActiveWorktree(st) {
				return Result{
					Action:  Warn,
					Message: warnOutsideWorktreeCode,
				}
			}
			return Result{
				Action:  Block,
				Message: "mindspec: blocked — code edits are not allowed during Plan Mode. Only documentation and plan files may be edited.",
			}
		}
		return Result{Action: Pass}

	case state.ModeImplement:
		// In implement mode, worktree enforcement handles scope.
		// Workflow guard passes silently.
		return Result{Action: Pass}

	case state.ModeReview:
		return Result{
			Action:  Warn,
			Message: warnReview,
		}

	default:
		return Result{Action: Pass}
	}
}

const blockIdle = "mindspec: blocked — no active work context. All edits must happen on a work branch.\n\n" +
	"Start work via one of:\n" +
	"  • mindspec next                    — claim a ready bead and create a worktree\n" +
	"  • mindspec spec create <NNN-slug>  — new feature (full spec lifecycle)"

const warnOutsideWorktreeCode = "⚠️ WORKFLOW WARNING: You are editing code outside the active spec worktree. " +
	"This is allowed for unrelated work (infrastructure, tooling, bug fixes) but make sure this is intentional. " +
	"If this edit belongs to the active spec, switch to the spec worktree."

const warnReview = "⚠️ WORKFLOW WARNING: Review Mode — implementation is complete. " +
	"Edits should only address review feedback. If you need to make substantial changes, " +
	"consider opening a new spec."
