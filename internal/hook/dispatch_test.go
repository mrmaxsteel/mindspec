package hook

import (
	"testing"

	"github.com/mindspec/mindspec/internal/state"
)

// --- Plan Gate Exit ---

func TestPlanGateExit_NilState(t *testing.T) {
	t.Parallel()
	r := PlanGateExit(&Input{}, nil)
	if r.Action != Pass {
		t.Error("nil state should pass")
	}
}

func TestPlanGateExit_PlanMode(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModePlan}
	r := PlanGateExit(&Input{}, st)
	if r.Action != Block {
		t.Error("plan mode should block ExitPlanMode")
	}
	if r.Message == "" {
		t.Error("block should have a message")
	}
}

func TestPlanGateExit_OtherMode(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{state.ModeIdle, state.ModeSpec, state.ModeImplement, state.ModeReview} {
		st := &state.State{Mode: mode}
		r := PlanGateExit(&Input{}, st)
		if r.Action != Pass {
			t.Errorf("mode %q should pass, got action %d", mode, r.Action)
		}
	}
}

// --- Plan Gate Enter ---

func TestPlanGateEnter_NilState(t *testing.T) {
	t.Parallel()
	r := PlanGateEnter(&Input{}, nil)
	if r.Action != Pass {
		t.Error("nil state should pass")
	}
}

func TestPlanGateEnter_PlanMode(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModePlan}
	r := PlanGateEnter(&Input{}, st)
	if r.Action != Warn {
		t.Error("plan mode should warn on EnterPlanMode")
	}
}

func TestPlanGateEnter_OtherMode(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeIdle}
	r := PlanGateEnter(&Input{}, st)
	if r.Action != Pass {
		t.Error("idle mode should pass")
	}
}

// --- Worktree File ---

func TestWorktreeFile_NilState(t *testing.T) {
	t.Parallel()
	r := WorktreeFile(&Input{FilePath: "/some/file.go"}, nil, true)
	if r.Action != Pass {
		t.Error("nil state should pass")
	}
}

func TestWorktreeFile_NoWorktree(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeImplement}
	r := WorktreeFile(&Input{FilePath: "/some/file.go"}, st, true)
	if r.Action != Pass {
		t.Error("no active worktree should pass")
	}
}

func TestWorktreeFile_EnforcementDisabled(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeImplement, ActiveWorktree: "/wt"}
	r := WorktreeFile(&Input{FilePath: "/outside/file.go"}, st, false)
	if r.Action != Pass {
		t.Error("enforcement disabled should pass")
	}
}

func TestWorktreeFile_InsideWorktree(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeImplement, ActiveWorktree: "/project/.worktrees/wt1"}
	r := WorktreeFile(&Input{FilePath: "/project/.worktrees/wt1/internal/foo.go"}, st, true)
	if r.Action != Pass {
		t.Error("file inside worktree should pass")
	}
}

func TestWorktreeFile_OutsideWorktree(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeImplement, ActiveWorktree: "/project/.worktrees/wt1"}
	r := WorktreeFile(&Input{FilePath: "/totally/different/path.go"}, st, true)
	if r.Action != Block {
		t.Error("file outside worktree should block")
	}
}

func TestWorktreeFile_EmptyFilePath(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeImplement, ActiveWorktree: "/wt"}
	r := WorktreeFile(&Input{}, st, true)
	if r.Action != Pass {
		t.Error("empty file path should pass")
	}
}

// --- Worktree Bash ---

func TestWorktreeBash_NilState(t *testing.T) {
	t.Parallel()
	r := WorktreeBash(&Input{Command: "rm -rf /"}, nil, true)
	if r.Action != Pass {
		t.Error("nil state should pass")
	}
}

func TestWorktreeBash_AllowedCommand(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeImplement, ActiveWorktree: "/wt"}
	for _, cmd := range []string{"cd /wt", "mindspec instruct", "git status", "make build", "go test ./...", "bd ready"} {
		r := WorktreeBash(&Input{Command: cmd}, st, true)
		if r.Action != Pass {
			t.Errorf("allowed command %q should pass", cmd)
		}
	}
}

func TestWorktreeBash_EnvPrefixStrip(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeImplement, ActiveWorktree: "/wt"}
	r := WorktreeBash(&Input{Command: "MINDSPEC_ALLOW_MAIN=1 git commit"}, st, true)
	if r.Action != Pass {
		t.Error("env-prefixed allowed command should pass after stripping")
	}
}

func TestWorktreeBash_AllowsSpecWorktree(t *testing.T) {
	origGetCwd := getCwd
	t.Cleanup(func() { getCwd = origGetCwd })

	// CWD is the spec worktree, ActiveWorktree is the bead worktree
	getCwd = func() (string, error) {
		return "/repo/.worktrees/worktree-spec-051-test", nil
	}
	st := &state.State{
		Mode:           state.ModeImplement,
		ActiveSpec:     "051-test",
		ActiveWorktree: "/repo/.worktrees/worktree-spec-051-test/.worktrees/worktree-bead-abc",
	}
	r := WorktreeBash(&Input{Command: "cat foo.txt"}, st, true)
	if r.Action != Pass {
		t.Errorf("spec worktree CWD should pass, got %v: %s", r.Action, r.Message)
	}
}

// --- Session Freshness Gate ---

func TestSessionFreshnessGate_NilState(t *testing.T) {
	t.Parallel()
	r := SessionFreshnessGate(&Input{Command: "mindspec next"}, nil)
	if r.Action != Pass {
		t.Error("nil state should pass")
	}
}

func TestSessionFreshnessGate_NoSessionData(t *testing.T) {
	t.Parallel()
	st := &state.State{}
	r := SessionFreshnessGate(&Input{Command: "mindspec next"}, st)
	if r.Action != Pass {
		t.Error("missing sessionStartedAt should skip gate")
	}
}

func TestSessionFreshnessGate_FreshStartup(t *testing.T) {
	t.Parallel()
	st := &state.State{
		SessionSource:    "startup",
		SessionStartedAt: "2026-02-27T00:00:00Z",
	}
	r := SessionFreshnessGate(&Input{Command: "mindspec next"}, st)
	if r.Action != Pass {
		t.Error("fresh startup with no prior bead should pass")
	}
}

func TestSessionFreshnessGate_FreshClear(t *testing.T) {
	t.Parallel()
	st := &state.State{
		SessionSource:    "clear",
		SessionStartedAt: "2026-02-27T00:05:00Z",
		BeadClaimedAt:    "2026-02-27T00:01:00Z", // claimed before clear
	}
	r := SessionFreshnessGate(&Input{Command: "mindspec next"}, st)
	if r.Action != Pass {
		t.Error("clear after bead claim should pass (sessionStartedAt > beadClaimedAt)")
	}
}

func TestSessionFreshnessGate_StaleSession(t *testing.T) {
	t.Parallel()
	st := &state.State{
		SessionSource:    "startup",
		SessionStartedAt: "2026-02-27T00:00:00Z",
		BeadClaimedAt:    "2026-02-27T00:01:00Z", // claimed after session start
	}
	r := SessionFreshnessGate(&Input{Command: "mindspec next"}, st)
	if r.Action != Block {
		t.Error("bead claimed after session start without /clear should block")
	}
}

func TestSessionFreshnessGate_ResumedSession(t *testing.T) {
	t.Parallel()
	st := &state.State{
		SessionSource:    "resume",
		SessionStartedAt: "2026-02-27T00:00:00Z",
	}
	r := SessionFreshnessGate(&Input{Command: "mindspec next"}, st)
	if r.Action != Block {
		t.Error("resumed session should block")
	}
}

func TestSessionFreshnessGate_ForceBypass(t *testing.T) {
	t.Parallel()
	st := &state.State{
		SessionSource:    "resume",
		SessionStartedAt: "2026-02-27T00:00:00Z",
	}
	r := SessionFreshnessGate(&Input{Command: "mindspec next --force"}, st)
	if r.Action != Pass {
		t.Error("--force should bypass session freshness gate")
	}
}

func TestSessionFreshnessGate_DifferentCommand(t *testing.T) {
	t.Parallel()
	st := &state.State{
		SessionSource:    "resume",
		SessionStartedAt: "2026-02-27T00:00:00Z",
	}
	r := SessionFreshnessGate(&Input{Command: "mindspec instruct"}, st)
	if r.Action != Pass {
		t.Error("non-next command should pass regardless of session state")
	}
}

// --- Workflow Guard ---

func TestWorkflowGuard_NilState(t *testing.T) {
	t.Parallel()
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, nil, true)
	if r.Action != Pass {
		t.Error("nil state should pass")
	}
}

func TestWorkflowGuard_EnforcementDisabled(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeIdle}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, false)
	if r.Action != Pass {
		t.Error("enforcement disabled should pass")
	}
}

func TestWorkflowGuard_Idle_CodeFile(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeIdle}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if r.Action != Warn {
		t.Errorf("idle mode code edit should warn, got %d", r.Action)
	}
}

func TestWorkflowGuard_Idle_DocFile(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeIdle}
	r := WorkflowGuard(&Input{FilePath: ".mindspec/docs/specs/001/spec.md"}, st, true)
	if r.Action != Warn {
		t.Errorf("idle mode doc edit should still warn, got %d", r.Action)
	}
}

func TestWorkflowGuard_Idle_EmptyMode(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: ""}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if r.Action != Warn {
		t.Errorf("empty mode should warn like idle, got %d", r.Action)
	}
}

func TestWorkflowGuard_Explore(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeExplore}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if r.Action != Warn {
		t.Errorf("explore mode should warn, got %d", r.Action)
	}
}

func TestWorkflowGuard_Spec_CodeEdit(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeSpec}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if r.Action != Block {
		t.Errorf("spec mode code edit should block, got %d", r.Action)
	}
}

func TestWorkflowGuard_Spec_DocEdit(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeSpec}
	r := WorkflowGuard(&Input{FilePath: ".mindspec/docs/specs/049/spec.md"}, st, true)
	if r.Action != Pass {
		t.Errorf("spec mode doc edit should pass, got %d", r.Action)
	}
}

func TestWorkflowGuard_Spec_MarkdownFile(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeSpec}
	r := WorkflowGuard(&Input{FilePath: "GLOSSARY.md"}, st, true)
	if r.Action != Pass {
		t.Errorf("spec mode markdown file should pass, got %d", r.Action)
	}
}

func TestWorkflowGuard_Plan_CodeEdit(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModePlan}
	r := WorkflowGuard(&Input{FilePath: "cmd/main.go"}, st, true)
	if r.Action != Block {
		t.Errorf("plan mode code edit should block, got %d", r.Action)
	}
}

func TestWorkflowGuard_Plan_DocEdit(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModePlan}
	r := WorkflowGuard(&Input{FilePath: ".mindspec/docs/specs/049/plan.md"}, st, true)
	if r.Action != Pass {
		t.Errorf("plan mode doc edit should pass, got %d", r.Action)
	}
}

func TestWorkflowGuard_Implement(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeImplement}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if r.Action != Pass {
		t.Errorf("implement mode should pass, got %d", r.Action)
	}
}

func TestWorkflowGuard_Review(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeReview}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if r.Action != Warn {
		t.Errorf("review mode should warn, got %d", r.Action)
	}
}

// --- Helpers ---

func TestIsCodeFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		code bool
	}{
		{"internal/foo.go", true},
		{"cmd/main.go", true},
		{"Makefile", true},
		{"go.mod", true},
		{".mindspec/docs/specs/049/spec.md", false},
		{"docs/overview.md", false},
		{".claude/settings.json", false},
		{".github/hooks/test.sh", false},
		{"GLOSSARY.md", false},
		{"AGENTS.md", false},
		{"CLAUDE.md", false},
		{"README.md", false},
		{"some-notes.md", false},
		{".mindspec/config.yaml", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isCodeFile(tt.path)
		if got != tt.code {
			t.Errorf("isCodeFile(%q) = %v, want %v", tt.path, got, tt.code)
		}
	}
}

func TestStripEnvPrefixes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, out string
	}{
		{"git status", "git status"},
		{"MINDSPEC_ALLOW_MAIN=1 git commit", "git commit"},
		{"FOO=bar BAZ=qux make build", "make build"},
		{"lowercase=val cmd", "lowercase=val cmd"}, // lowercase not an env var
	}
	for _, tt := range tests {
		got := stripEnvPrefixes(tt.in)
		if got != tt.out {
			t.Errorf("stripEnvPrefixes(%q) = %q, want %q", tt.in, got, tt.out)
		}
	}
}

func TestHasPathPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path, prefix string
		want         bool
	}{
		{"/a/b/c", "/a/b", true},
		{"/a/b/c", "/a/b/", true},
		{"/a/b", "/a/b", true},
		{"/a/bc", "/a/b", false},
		{"/x/y", "/a/b", false},
		{"", "/a", false},
		{"/a", "", false},
	}
	for _, tt := range tests {
		got := hasPathPrefix(tt.path, tt.prefix)
		if got != tt.want {
			t.Errorf("hasPathPrefix(%q, %q) = %v, want %v", tt.path, tt.prefix, got, tt.want)
		}
	}
}

// --- Warning message content ---

func TestWorkflowGuard_IdleWarning_ContainsEscapeHatch(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeIdle}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	for _, phrase := range []string{
		"/ms-spec-init",
		"/ms-explore",
		"debugging a CI failure",
		"fixing a broken build",
		"correcting a typo",
		"urgent operational fix",
	} {
		if !contains(r.Message, phrase) {
			t.Errorf("idle warning should contain %q", phrase)
		}
	}
}

func TestWorkflowGuard_ExploreWarning_ContainsPromote(t *testing.T) {
	t.Parallel()
	st := &state.State{Mode: state.ModeExplore}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if !contains(r.Message, "/ms-explore promote") {
		t.Error("explore warning should mention /ms-explore promote")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstr(s, substr)))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- Run dispatcher ---

func TestRun_UnknownHook(t *testing.T) {
	t.Parallel()
	r := Run("nonexistent", &Input{}, nil, true)
	if r.Action != Pass {
		t.Error("unknown hook should pass")
	}
}
