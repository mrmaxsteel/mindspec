package hook

import (
	"fmt"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
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
// stateFn lazily resolves workflow state (e.g. ReadState); it is only
// invoked once a hook decides state is actually needed, so cheap
// short-circuits (non-protected branches, enforcement disabled) skip
// the beads subprocess fan-out entirely.
func Run(name string, inp *Input, stateFn func() *HookState, enforce bool) Result {
	switch name {
	case "pre-commit":
		return runPreCommit(stateFn)
	default:
		return Result{Action: Pass}
	}
}

// runPreCommit implements branch protection: blocks commits on protected
// branches regardless of mode (including idle).
// Escape hatch: MINDSPEC_ALLOW_MAIN=1 git commit
func runPreCommit(stateFn func() *HookState) Result {
	// Escape hatch
	if os.Getenv("MINDSPEC_ALLOW_MAIN") == "1" {
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
	branch, _ := gitutil.CurrentBranch()
	if branch == "" {
		return Result{Action: Pass}
	}

	protected := cfg.IsProtectedBranch(branch)

	// Short-circuit: non-protected, non-spec branches always pass — return
	// before resolving state, so the beads phase-context fan-out never runs.
	if !protected && !strings.HasPrefix(branch, "spec/") {
		return Result{Action: Pass}
	}

	// State is needed from here on: resolve it now.
	var st *HookState
	if stateFn != nil {
		st = stateFn()
	}

	// No state at all → mindspec not initialized, allow
	if st == nil {
		return Result{Action: Pass}
	}

	// Block: commits on protected branches in ANY mode (including idle).
	// The guidance says "main is protected" — the hook enforces it.
	if protected {
		mode := st.Mode
		if mode == "" {
			mode = "idle"
		}
		msg := fmt.Sprintf("mindspec: commits on '%s' are blocked (mode: %s).", termsafe.Escape(branch), mode)
		msg += "\n  For bug fixes: git checkout -b fix/<description>"
		msg += "\n  For features: mindspec spec create <slug>"
		if st.ActiveWorktree != "" {
			msg += fmt.Sprintf("\n  Or switch to your worktree: %s", containment.EmitCd(st.ActiveWorktree))
		}
		msg += "\n  Escape hatch: MINDSPEC_ALLOW_MAIN=1 git commit ..."
		return Result{Action: Block, Message: msg}
	}

	// Non-protected branch: idle mode has no further restrictions
	if st.Mode == "" || st.Mode == "idle" {
		return Result{Action: Pass}
	}

	// Fail closed on a spec/ branch when the mode is genuinely
	// indeterminate (spec 128 bead 5 round-4 panel G2/R6): ReadState
	// maps a bd/Dolt discovery failure OR an unresolved multi-active-
	// spec ambiguity to state.ModeIndeterminate, and that sentinel
	// matched NEITHER the idle fast-path above (it is not "" or "idle")
	// NOR the implement-mode check below (it is not literally
	// "implement") — so it silently fell through to the final Pass,
	// permitting a direct spec-branch commit precisely when the hook
	// cannot prove implementation mode has ended. Blocked explicitly,
	// with an HONEST message naming the real cause — never claiming the
	// mode is implement when it genuinely could not be determined. The
	// escape hatch still covers a deliberate legitimate commit while
	// discovery is unavailable.
	if st.Mode == state.ModeIndeterminate && strings.HasPrefix(branch, "spec/") {
		msg := fmt.Sprintf("mindspec: cannot determine lifecycle mode for spec branch '%s' — refusing a direct commit while the mode is indeterminate.", termsafe.Escape(branch))
		msg += "\n  This is a bd/Dolt discovery failure or an unresolved ambiguity, never assumed to be a safe post-implement state."
		msg += "\n  Run: mindspec doctor   (diagnose the underlying failure)"
		if st.ActiveWorktree != "" {
			msg += fmt.Sprintf("\n  Or switch to your bead worktree: %s", containment.EmitCd(st.ActiveWorktree))
		}
		msg += "\n  Legitimate direct spec-branch commit while discovery is unavailable may use the escape hatch:"
		msg += "\n    MINDSPEC_ALLOW_MAIN=1 git commit ..."
		return Result{Action: Block, Message: msg}
	}

	// Block: on a spec/ branch during implement mode — code belongs on bead branches.
	//
	// Spec 093 Req 1: the message carries the when-is-it-legitimate
	// context that used to live only in skill prose (ms-bead-fix /
	// ms-spec-final-review), and states the ACTUAL gate coverage per
	// C2-1: protected branches any-mode + spec/ branches implement-mode
	// only; bead/ branches always pass (extending the gate to bead
	// branches is an explicit Non-Goal — it would block the autopilot's
	// own impl subagents). The conditional "Or switch to your bead
	// worktree" line is preserved, conditionality included (G1-3).
	// Hook Block messages follow the Emit protocol (HC-5 exception),
	// not the guard.FormatFailure error-return convention, but still
	// end with actionable guidance.
	if st.Mode == "implement" && strings.HasPrefix(branch, "spec/") {
		msg := fmt.Sprintf("mindspec: commits on spec branch '%s' are blocked during implement mode.\n  Implementation code belongs on bead branches.", termsafe.Escape(branch))
		msg += "\n  Run: mindspec next   (to claim a bead and create a bead worktree)"
		if st.ActiveWorktree != "" {
			msg += fmt.Sprintf("\n  Or switch to your bead worktree: %s", containment.EmitCd(st.ActiveWorktree))
		}
		msg += "\n  Legitimate direct spec-branch commits (final-review fix-ups: PR-body precision,"
		msg += "\n  stray-file reverts, CI-unblocking test fixes) may use the escape hatch:"
		msg += "\n    MINDSPEC_ALLOW_MAIN=1 git commit ..."
		msg += "\n  Do NOT use the escape hatch to land feature code outside a bead branch."
		msg += "\n  (Gate coverage: protected branches in any mode; spec/ branches during implement"
		msg += "\n  mode only. bead/ branches always pass — commit there freely.)"
		return Result{Action: Block, Message: msg}
	}

	return Result{Action: Pass}
}
