package lifecycle

import (
	"testing"

	"github.com/mindspec/mindspec/internal/hook"
	"github.com/mindspec/mindspec/internal/state"
)

// ---------------------------------------------------------------------------
// WorkflowGuard enforcement matrix
// ---------------------------------------------------------------------------

func TestHookMatrix_WorkflowGuard(t *testing.T) {
	// Mock getCwd to a non-worktree path so outsideActiveWorktree doesn't
	// interfere with spec/plan code-edit tests.
	origGetCwd := hook.ExportGetCwd()
	t.Cleanup(func() { hook.SetGetCwd(origGetCwd) })
	hook.SetGetCwd(func() (string, error) { return "/repo", nil })

	tests := []struct {
		name     string
		mode     string
		filePath string
		want     hook.Action
	}{
		// idle — always blocks file edits
		{"idle/code", state.ModeIdle, "internal/foo.go", hook.Block},
		{"idle/doc", state.ModeIdle, ".mindspec/docs/spec.md", hook.Block},
		{"idle/plan", state.ModeIdle, "docs/specs/001/plan.md", hook.Block},
		{"idle/config", state.ModeIdle, ".mindspec/config.yaml", hook.Block},

		// spec — blocks code, allows docs
		{"spec/go_file", state.ModeSpec, "internal/foo.go", hook.Block},
		{"spec/test_file", state.ModeSpec, "internal/foo_test.go", hook.Block},
		{"spec/cmd_file", state.ModeSpec, "cmd/main.go", hook.Block},
		{"spec/spec_doc", state.ModeSpec, ".mindspec/docs/specs/001/spec.md", hook.Pass},
		{"spec/adr_doc", state.ModeSpec, ".mindspec/docs/adr/ADR-0001.md", hook.Pass},
		{"spec/config", state.ModeSpec, ".mindspec/config.yaml", hook.Pass},
		{"spec/claude_md", state.ModeSpec, "CLAUDE.md", hook.Pass},
		{"spec/readme", state.ModeSpec, "README.md", hook.Pass},
		{"spec/docs_dir", state.ModeSpec, "docs/guide.md", hook.Pass},
		{"spec/random_md", state.ModeSpec, "notes/ideas.md", hook.Pass},

		// plan — blocks code, allows docs
		{"plan/go_file", state.ModePlan, "cmd/main.go", hook.Block},
		{"plan/ts_file", state.ModePlan, "src/index.ts", hook.Block},
		{"plan/plan_doc", state.ModePlan, "docs/specs/001/plan.md", hook.Pass},
		{"plan/spec_doc", state.ModePlan, ".mindspec/docs/specs/001/spec.md", hook.Pass},
		{"plan/claude_settings", state.ModePlan, ".claude/settings.json", hook.Pass},
		{"plan/github", state.ModePlan, ".github/workflows/ci.yml", hook.Pass},

		// implement — always passes (worktree-file handles scope)
		{"implement/code", state.ModeImplement, "internal/foo.go", hook.Pass},
		{"implement/test", state.ModeImplement, "internal/foo_test.go", hook.Pass},
		{"implement/doc", state.ModeImplement, ".mindspec/docs/spec.md", hook.Pass},
		{"implement/any", state.ModeImplement, "Makefile", hook.Pass},

		// review — always warns
		{"review/code", state.ModeReview, "internal/foo.go", hook.Warn},
		{"review/doc", state.ModeReview, ".mindspec/docs/spec.md", hook.Warn},
		{"review/test", state.ModeReview, "internal/foo_test.go", hook.Warn},

		// empty mode — treated as idle, blocks
		{"empty/code", "", "internal/foo.go", hook.Block},

		// empty file path — no code file, so Pass in spec/plan
		{"spec/empty_path", state.ModeSpec, "", hook.Pass},
		{"plan/empty_path", state.ModePlan, "", hook.Pass},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &hook.HookState{Mode: tt.mode}
			inp := &hook.Input{FilePath: tt.filePath}
			got := hook.Run("workflow-guard", inp, st, true)
			if got.Action != tt.want {
				t.Errorf("WorkflowGuard(%s, %q) = %v, want %v",
					tt.mode, tt.filePath, got.Action, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WorkflowGuard edge cases
// ---------------------------------------------------------------------------

func TestHookMatrix_WorkflowGuard_NilState(t *testing.T) {
	inp := &hook.Input{FilePath: "internal/foo.go"}
	got := hook.Run("workflow-guard", inp, nil, true)
	if got.Action != hook.Pass {
		t.Errorf("nil state: got %v, want Pass", got.Action)
	}
}

func TestHookMatrix_WorkflowGuard_EnforcementDisabled(t *testing.T) {
	st := &hook.HookState{Mode: state.ModeSpec}
	inp := &hook.Input{FilePath: "internal/foo.go"}
	got := hook.Run("workflow-guard", inp, st, false)
	if got.Action != hook.Pass {
		t.Errorf("enforcement disabled: got %v, want Pass", got.Action)
	}
}

func TestHookMatrix_WorkflowGuard_PlanOutsideWorktree(t *testing.T) {
	origGetCwd := hook.ExportGetCwd()
	t.Cleanup(func() { hook.SetGetCwd(origGetCwd) })
	hook.SetGetCwd(func() (string, error) { return "/repo", nil })

	st := &hook.HookState{
		Mode:           state.ModePlan,
		ActiveSpec:     "044",
		ActiveWorktree: "/repo/.worktrees/worktree-spec-044",
	}
	inp := &hook.Input{FilePath: "/repo/internal/harness/agent.go"}
	got := hook.Run("workflow-guard", inp, st, true)
	if got.Action != hook.Warn {
		t.Errorf("plan mode code edit outside worktree: got %v, want Warn", got.Action)
	}
}

func TestHookMatrix_WorkflowGuard_UnknownMode(t *testing.T) {
	st := &hook.HookState{Mode: "unknown-mode"}
	inp := &hook.Input{FilePath: "internal/foo.go"}
	got := hook.Run("workflow-guard", inp, st, true)
	if got.Action != hook.Pass {
		t.Errorf("unknown mode: got %v, want Pass", got.Action)
	}
}

// ---------------------------------------------------------------------------
// PlanGate hooks
// ---------------------------------------------------------------------------

func TestHookMatrix_PlanGateExit(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want hook.Action
	}{
		{"plan_mode", state.ModePlan, hook.Block},
		{"spec_mode", state.ModeSpec, hook.Pass},
		{"implement", state.ModeImplement, hook.Pass},
		{"idle", state.ModeIdle, hook.Pass},
		{"review", state.ModeReview, hook.Pass},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &hook.HookState{Mode: tt.mode}
			got := hook.Run("plan-gate-exit", &hook.Input{}, st, true)
			if got.Action != tt.want {
				t.Errorf("PlanGateExit(%s) = %v, want %v", tt.mode, got.Action, tt.want)
			}
		})
	}
}

func TestHookMatrix_PlanGateExit_NilState(t *testing.T) {
	got := hook.Run("plan-gate-exit", &hook.Input{}, nil, true)
	if got.Action != hook.Pass {
		t.Errorf("nil state: got %v, want Pass", got.Action)
	}
}

func TestHookMatrix_PlanGateEnter(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want hook.Action
	}{
		{"plan_mode", state.ModePlan, hook.Warn},
		{"spec_mode", state.ModeSpec, hook.Pass},
		{"implement", state.ModeImplement, hook.Pass},
		{"idle", state.ModeIdle, hook.Pass},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &hook.HookState{Mode: tt.mode}
			got := hook.Run("plan-gate-enter", &hook.Input{}, st, true)
			if got.Action != tt.want {
				t.Errorf("PlanGateEnter(%s) = %v, want %v", tt.mode, got.Action, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WorktreeFile enforcement
// ---------------------------------------------------------------------------

func TestHookMatrix_WorktreeFile(t *testing.T) {
	wt := "/repo/.worktrees/worktree-bead-001"

	tests := []struct {
		name     string
		filePath string
		want     hook.Action
	}{
		{"inside_worktree", wt + "/internal/foo.go", hook.Pass},
		{"worktree_root", wt, hook.Pass},
		{"outside_worktree", "/other/repo/foo.go", hook.Block},
		{"empty_path", "", hook.Pass},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &hook.HookState{
				Mode:           state.ModeImplement,
				ActiveWorktree: wt,
			}
			inp := &hook.Input{FilePath: tt.filePath}
			got := hook.Run("worktree-file", inp, st, true)
			if got.Action != tt.want {
				t.Errorf("WorktreeFile(%q) = %v, want %v", tt.filePath, got.Action, tt.want)
			}
		})
	}
}

func TestHookMatrix_WorktreeFile_NoWorktree(t *testing.T) {
	// No active worktree — always pass
	st := &hook.HookState{Mode: state.ModeImplement}
	inp := &hook.Input{FilePath: "/some/random/file.go"}
	got := hook.Run("worktree-file", inp, st, true)
	if got.Action != hook.Pass {
		t.Errorf("no worktree: got %v, want Pass", got.Action)
	}
}

func TestHookMatrix_WorktreeFile_EnforcementDisabled(t *testing.T) {
	st := &hook.HookState{
		Mode:           state.ModeImplement,
		ActiveWorktree: "/repo/.worktrees/wt",
	}
	inp := &hook.Input{FilePath: "/other/file.go"}
	got := hook.Run("worktree-file", inp, st, false)
	if got.Action != hook.Pass {
		t.Errorf("enforcement disabled: got %v, want Pass", got.Action)
	}
}

// ---------------------------------------------------------------------------
// WorktreeBash enforcement
// ---------------------------------------------------------------------------

func TestHookMatrix_WorktreeBash(t *testing.T) {
	wt := "/repo/.worktrees/worktree-bead-001"

	tests := []struct {
		name    string
		command string
		want    hook.Action
	}{
		// Allowed command prefixes always pass
		{"cd_allowed", "cd /somewhere", hook.Pass},
		{"mindspec_allowed", "mindspec next", hook.Pass},
		{"bin_mindspec_allowed", "./bin/mindspec validate spec 001", hook.Pass},
		{"bd_allowed", "bd ready", hook.Pass},
		{"make_allowed", "make build", hook.Pass},
		{"git_allowed", "git status", hook.Pass},
		{"go_allowed", "go test ./...", hook.Pass},

		// Env prefix stripping
		{"env_prefix_allowed", "MINDSPEC_ALLOW_MAIN=1 git commit -m test", hook.Pass},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &hook.HookState{
				Mode:           state.ModeImplement,
				ActiveWorktree: wt,
			}
			inp := &hook.Input{Command: tt.command}
			got := hook.Run("worktree-bash", inp, st, true)
			if got.Action != tt.want {
				t.Errorf("WorktreeBash(%q) = %v, want %v", tt.command, got.Action, tt.want)
			}
		})
	}
}

func TestHookMatrix_WorktreeBash_NoWorktree(t *testing.T) {
	st := &hook.HookState{Mode: state.ModeImplement}
	inp := &hook.Input{Command: "ls -la"}
	got := hook.Run("worktree-bash", inp, st, true)
	if got.Action != hook.Pass {
		t.Errorf("no worktree: got %v, want Pass", got.Action)
	}
}

func TestHookMatrix_WorktreeBash_NilState(t *testing.T) {
	inp := &hook.Input{Command: "ls -la"}
	got := hook.Run("worktree-bash", inp, nil, true)
	if got.Action != hook.Pass {
		t.Errorf("nil state: got %v, want Pass", got.Action)
	}
}

// ---------------------------------------------------------------------------
// SessionFreshnessGate
// ---------------------------------------------------------------------------

func TestHookMatrix_SessionFreshness(t *testing.T) {
	tests := []struct {
		name    string
		st      *hook.HookState
		command string
		want    hook.Action
	}{
		{
			"nil_state",
			nil,
			"mindspec next",
			hook.Pass,
		},
		{
			"not_next_command",
			&hook.HookState{SessionStartedAt: "2026-02-28T10:00:00Z", SessionSource: "resume"},
			"mindspec status",
			hook.Pass,
		},
		{
			"fresh_startup",
			&hook.HookState{SessionStartedAt: "2026-02-28T10:00:00Z", SessionSource: "startup"},
			"mindspec next",
			hook.Pass,
		},
		{
			"resume_blocks",
			&hook.HookState{SessionStartedAt: "2026-02-28T10:00:00Z", SessionSource: "resume"},
			"mindspec next",
			hook.Block,
		},
		{
			"compact_blocks",
			&hook.HookState{SessionStartedAt: "2026-02-28T10:00:00Z", SessionSource: "compact"},
			"mindspec next",
			hook.Block,
		},
		{
			"force_bypasses",
			&hook.HookState{SessionStartedAt: "2026-02-28T10:00:00Z", SessionSource: "resume"},
			"mindspec next --force",
			hook.Pass,
		},
		{
			"bead_already_claimed",
			&hook.HookState{
				SessionStartedAt: "2026-02-28T10:00:00Z",
				SessionSource:    "startup",
				BeadClaimedAt:    "2026-02-28T10:01:00Z",
			},
			"mindspec next",
			hook.Block,
		},
		{
			"no_session_start",
			&hook.HookState{SessionSource: "startup"},
			"mindspec next",
			hook.Pass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inp := &hook.Input{Command: tt.command}
			got := hook.Run("needs-clear", inp, tt.st, true)
			if got.Action != tt.want {
				t.Errorf("SessionFreshness(%s) = %v, want %v", tt.name, got.Action, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Unknown hook name
// ---------------------------------------------------------------------------

func TestHookMatrix_UnknownHook(t *testing.T) {
	got := hook.Run("nonexistent-hook", &hook.Input{}, &hook.HookState{Mode: state.ModeSpec}, true)
	if got.Action != hook.Pass {
		t.Errorf("unknown hook: got %v, want Pass", got.Action)
	}
}

// ---------------------------------------------------------------------------
// Spec 056: Named enforcement scenarios for traceability
// ---------------------------------------------------------------------------

func TestHookMatrix_HookBlocksMainCommit(t *testing.T) {
	// On main (idle mode, no worktree), workflow-guard blocks code edits.
	st := &hook.HookState{Mode: state.ModeIdle}
	inp := &hook.Input{FilePath: "internal/foo.go"}
	got := hook.Run("workflow-guard", inp, st, true)
	if got.Action != hook.Block {
		t.Errorf("idle mode code edit: got %v, want Block", got.Action)
	}
	if got.Message == "" {
		t.Error("expected non-empty block message")
	}
}

func TestHookMatrix_HookBlocksStaleNext(t *testing.T) {
	// After resume (stale session), needs-clear blocks mindspec next.
	st := &hook.HookState{
		SessionStartedAt: "2026-02-28T10:00:00Z",
		SessionSource:    "resume",
	}
	inp := &hook.Input{Command: "mindspec next"}
	got := hook.Run("needs-clear", inp, st, true)
	if got.Action != hook.Block {
		t.Errorf("stale session next: got %v, want Block", got.Action)
	}
	if got.Message == "" {
		t.Error("expected non-empty block message")
	}
}
