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
	st := &HookState{Mode: state.ModePlan}
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
		st := &HookState{Mode: mode}
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
	st := &HookState{Mode: state.ModePlan}
	r := PlanGateEnter(&Input{}, st)
	if r.Action != Warn {
		t.Error("plan mode should warn on EnterPlanMode")
	}
}

func TestPlanGateEnter_OtherMode(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeIdle}
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
	st := &HookState{Mode: state.ModeImplement}
	r := WorktreeFile(&Input{FilePath: "/some/file.go"}, st, true)
	if r.Action != Pass {
		t.Error("no active worktree should pass")
	}
}

func TestWorktreeFile_EnforcementDisabled(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeImplement, ActiveWorktree: "/wt"}
	r := WorktreeFile(&Input{FilePath: "/outside/file.go"}, st, false)
	if r.Action != Pass {
		t.Error("enforcement disabled should pass")
	}
}

func TestWorktreeFile_InsideWorktree(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeImplement, ActiveWorktree: "/project/.worktrees/wt1"}
	r := WorktreeFile(&Input{FilePath: "/project/.worktrees/wt1/internal/foo.go"}, st, true)
	if r.Action != Pass {
		t.Error("file inside worktree should pass")
	}
}

func TestWorktreeFile_OutsideWorktree(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeImplement, ActiveWorktree: "/project/.worktrees/wt1"}
	r := WorktreeFile(&Input{FilePath: "/totally/different/path.go"}, st, true)
	if r.Action != Block {
		t.Error("file outside worktree should block")
	}
}

func TestWorktreeFile_EmptyFilePath(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeImplement, ActiveWorktree: "/wt"}
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
	st := &HookState{Mode: state.ModeImplement, ActiveWorktree: "/wt"}
	for _, cmd := range []string{"cd /wt", "mindspec instruct", "git status", "make build", "go test ./...", "bd ready"} {
		r := WorktreeBash(&Input{Command: cmd}, st, true)
		if r.Action != Pass {
			t.Errorf("allowed command %q should pass", cmd)
		}
	}
}

func TestWorktreeBash_EnvPrefixStrip(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeImplement, ActiveWorktree: "/wt"}
	r := WorktreeBash(&Input{Command: "MINDSPEC_ALLOW_MAIN=1 git commit"}, st, true)
	if r.Action != Pass {
		t.Error("env-prefixed allowed command should pass after stripping")
	}
}

func TestWorktreeBash_BlocksProtectedBranchCommit(t *testing.T) {
	origGetCwd := getCwd
	origGetGitBranch := getGitBranch
	t.Cleanup(func() {
		getCwd = origGetCwd
		getGitBranch = origGetGitBranch
	})

	getCwd = func() (string, error) { return "/repo", nil }
	getGitBranch = func(workdir string) (string, error) { return "main", nil }

	st := &HookState{Mode: state.ModeImplement, ActiveWorktree: "/repo/.worktrees/worktree-bead-abc"}
	r := WorktreeBash(&Input{Command: "git commit -m test"}, st, true)
	if r.Action != Block {
		t.Fatalf("expected Block for protected-branch commit, got %v", r.Action)
	}
}

func TestWorktreeBash_BlocksProtectedBranchMerge(t *testing.T) {
	origGetCwd := getCwd
	origGetGitBranch := getGitBranch
	t.Cleanup(func() {
		getCwd = origGetCwd
		getGitBranch = origGetGitBranch
	})

	getCwd = func() (string, error) { return "/repo", nil }
	getGitBranch = func(workdir string) (string, error) { return "main", nil }

	st := &HookState{Mode: state.ModeImplement, ActiveWorktree: "/repo/.worktrees/worktree-bead-abc"}
	r := WorktreeBash(&Input{Command: "git merge spec/001-greeting"}, st, true)
	if r.Action != Block {
		t.Fatalf("expected Block for protected-branch merge, got %v", r.Action)
	}
}

func TestWorktreeBash_AllowsSpecWorktree(t *testing.T) {
	origGetCwd := getCwd
	t.Cleanup(func() { getCwd = origGetCwd })

	// CWD is the spec worktree, ActiveWorktree is the bead worktree
	getCwd = func() (string, error) {
		return "/repo/.worktrees/worktree-spec-051-test", nil
	}
	st := &HookState{
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
	st := &HookState{}
	r := SessionFreshnessGate(&Input{Command: "mindspec next"}, st)
	if r.Action != Pass {
		t.Error("missing sessionStartedAt should skip gate")
	}
}

func TestSessionFreshnessGate_FreshStartup(t *testing.T) {
	t.Parallel()
	st := &HookState{
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
	st := &HookState{
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
	st := &HookState{
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
	st := &HookState{
		SessionSource:    "resume",
		SessionStartedAt: "2026-02-27T00:00:00Z",
	}
	r := SessionFreshnessGate(&Input{Command: "mindspec next"}, st)
	if r.Action != Block {
		t.Error("resumed session should block")
	}
}

func TestSessionFreshnessGate_CompactedSession(t *testing.T) {
	t.Parallel()
	st := &HookState{
		SessionSource:    "compact",
		SessionStartedAt: "2026-02-27T00:00:00Z",
	}
	r := SessionFreshnessGate(&Input{Command: "mindspec next"}, st)
	if r.Action != Block {
		t.Error("compacted session should block")
	}
}

func TestSessionFreshnessGate_ForceBypass(t *testing.T) {
	t.Parallel()
	st := &HookState{
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
	st := &HookState{
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
	st := &HookState{Mode: state.ModeIdle}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, false)
	if r.Action != Pass {
		t.Error("enforcement disabled should pass")
	}
}

func TestWorkflowGuard_BashPassesAllModes(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{state.ModeIdle, "", state.ModeExplore, state.ModeSpec, state.ModePlan, state.ModeReview} {
		st := &HookState{Mode: mode}
		r := WorkflowGuard(&Input{Command: "git status"}, st, true)
		if r.Action != Pass {
			t.Errorf("mode %q: bash command should pass workflow guard, got %d", mode, r.Action)
		}
	}
}

func TestWorkflowGuard_Idle_CodeFile(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeIdle}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if r.Action != Block {
		t.Errorf("idle mode code edit should block, got %d", r.Action)
	}
}

func TestWorkflowGuard_Idle_DocFile(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeIdle}
	r := WorkflowGuard(&Input{FilePath: ".mindspec/docs/specs/001/spec.md"}, st, true)
	if r.Action != Block {
		t.Errorf("idle mode doc edit should still block, got %d", r.Action)
	}
}

func TestWorkflowGuard_Idle_EmptyMode(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: ""}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if r.Action != Block {
		t.Errorf("empty mode should block like idle, got %d", r.Action)
	}
}

func TestWorkflowGuard_Explore(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeExplore}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if r.Action != Warn {
		t.Errorf("explore mode should warn, got %d", r.Action)
	}
}

func TestWorkflowGuard_Spec_CodeEdit(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeSpec}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if r.Action != Block {
		t.Errorf("spec mode code edit should block, got %d", r.Action)
	}
}

func TestWorkflowGuard_Spec_DocEdit(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeSpec}
	r := WorkflowGuard(&Input{FilePath: ".mindspec/docs/specs/049/spec.md"}, st, true)
	if r.Action != Pass {
		t.Errorf("spec mode doc edit should pass, got %d", r.Action)
	}
}

func TestWorkflowGuard_Spec_MarkdownFile(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeSpec}
	r := WorkflowGuard(&Input{FilePath: "GLOSSARY.md"}, st, true)
	if r.Action != Pass {
		t.Errorf("spec mode markdown file should pass, got %d", r.Action)
	}
}

func TestWorkflowGuard_Plan_CodeEdit(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModePlan}
	r := WorkflowGuard(&Input{FilePath: "cmd/main.go"}, st, true)
	if r.Action != Block {
		t.Errorf("plan mode code edit should block, got %d", r.Action)
	}
}

func TestWorkflowGuard_Plan_DocEdit(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModePlan}
	r := WorkflowGuard(&Input{FilePath: ".mindspec/docs/specs/049/plan.md"}, st, true)
	if r.Action != Pass {
		t.Errorf("plan mode doc edit should pass, got %d", r.Action)
	}
}

func TestWorkflowGuard_Implement(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeImplement}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if r.Action != Pass {
		t.Errorf("implement mode should pass, got %d", r.Action)
	}
}

func TestWorkflowGuard_Review(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeReview}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if r.Action != Warn {
		t.Errorf("review mode should warn, got %d", r.Action)
	}
}

// --- Workflow Guard: worktree bypass for spec/plan ---

func TestWorkflowGuard_Plan_CodeEdit_OutsideWorktree(t *testing.T) {
	origGetCwd := getCwd
	t.Cleanup(func() { getCwd = origGetCwd })
	getCwd = func() (string, error) {
		return "/repo", nil // CWD is main repo, not the spec worktree
	}

	st := &HookState{
		Mode:           state.ModePlan,
		ActiveSpec:     "044",
		ActiveWorktree: "/repo/.worktrees/worktree-spec-044",
	}
	r := WorkflowGuard(&Input{FilePath: "/repo/internal/harness/agent.go"}, st, true)
	if r.Action != Warn {
		t.Errorf("plan mode code edit OUTSIDE worktree should warn, got %v: %s", r.Action, r.Message)
	}
}

func TestWorkflowGuard_Plan_CodeEdit_InsideWorktree(t *testing.T) {
	origGetCwd := getCwd
	t.Cleanup(func() { getCwd = origGetCwd })
	getCwd = func() (string, error) {
		return "/repo/.worktrees/worktree-spec-044", nil
	}

	st := &HookState{
		Mode:           state.ModePlan,
		ActiveSpec:     "044",
		ActiveWorktree: "/repo/.worktrees/worktree-spec-044",
	}
	r := WorkflowGuard(&Input{FilePath: "/repo/.worktrees/worktree-spec-044/cmd/main.go"}, st, true)
	if r.Action != Block {
		t.Errorf("plan mode code edit INSIDE worktree should block, got %v", r.Action)
	}
}

func TestWorkflowGuard_Spec_CodeEdit_OutsideWorktree(t *testing.T) {
	origGetCwd := getCwd
	t.Cleanup(func() { getCwd = origGetCwd })
	getCwd = func() (string, error) {
		return "/repo", nil
	}

	st := &HookState{
		Mode:           state.ModeSpec,
		ActiveSpec:     "044",
		ActiveWorktree: "/repo/.worktrees/worktree-spec-044",
	}
	r := WorkflowGuard(&Input{FilePath: "/repo/Makefile"}, st, true)
	if r.Action != Warn {
		t.Errorf("spec mode code edit OUTSIDE worktree should warn, got %v: %s", r.Action, r.Message)
	}
}

func TestWorkflowGuard_Plan_NoWorktree_StillBlocks(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModePlan}
	r := WorkflowGuard(&Input{FilePath: "cmd/main.go"}, st, true)
	if r.Action != Block {
		t.Errorf("plan mode with no worktree should still block code edits, got %v", r.Action)
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
		// Absolute paths (Claude Code sends these via tool_input.file_path)
		{"/Users/max/project/.mindspec/focus", false},
		{"/Users/max/project/.mindspec/docs/specs/001/spec.md", false},
		{"/Users/max/project/.claude/settings.json", false},
		{"/Users/max/project/internal/foo.go", true},
		{"/Users/max/project/cmd/main.go", true},
		// Worktree absolute paths
		{"/Users/max/project/.worktrees/wt-spec-001/.mindspec/focus", false},
		{"/Users/max/project/.worktrees/wt-spec-001/internal/foo.go", true},
	}
	for _, tt := range tests {
		got := isCodeFile(tt.path)
		if got != tt.code {
			t.Errorf("isCodeFile(%q) = %v, want %v", tt.path, got, tt.code)
		}
	}
}

func TestIsAllowedCommandAbsolutePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cmd     string
		allowed bool
	}{
		{"mindspec state set --mode idle", true},
		{"./bin/mindspec complete", true},
		{"/usr/local/bin/mindspec state show", true},
		{"/Users/max/project/bin/mindspec complete --spec 001", true},
		{"git status", true},
		{"/usr/bin/git log", true},
		{"bd list", true},
		{"/opt/homebrew/bin/bd ready", true},
		{"rm -rf /", false},
		{"/usr/bin/rm -rf /", false},
		{"curl http://example.com", false},
	}
	for _, tt := range tests {
		got := isAllowedCommand(tt.cmd)
		if got != tt.allowed {
			t.Errorf("isAllowedCommand(%q) = %v, want %v", tt.cmd, got, tt.allowed)
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

func TestWorkflowGuard_IdleBlock_ContainsEscapePaths(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeIdle}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	for _, phrase := range []string{
		"spec-init",
		"mindspec next",
		"git checkout -b",
	} {
		if !contains(r.Message, phrase) {
			t.Errorf("idle block message should contain %q", phrase)
		}
	}
}

func TestWorkflowGuard_ExploreWarning_ContainsPromote(t *testing.T) {
	t.Parallel()
	st := &HookState{Mode: state.ModeExplore}
	r := WorkflowGuard(&Input{FilePath: "internal/foo.go"}, st, true)
	if !contains(r.Message, "/ms:explore promote") {
		t.Error("explore warning should mention /ms:explore promote")
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

// --- outsideActiveWorktree ---

func TestOutsideActiveWorktree_NilState(t *testing.T) {
	t.Parallel()
	if outsideActiveWorktree(nil) {
		t.Error("nil state should return false (conservative)")
	}
}

func TestOutsideActiveWorktree_NoWorktree(t *testing.T) {
	t.Parallel()
	if outsideActiveWorktree(&HookState{Mode: state.ModePlan}) {
		t.Error("no active worktree should return false")
	}
}

func TestOutsideActiveWorktree_Inside(t *testing.T) {
	origGetCwd := getCwd
	t.Cleanup(func() { getCwd = origGetCwd })
	getCwd = func() (string, error) {
		return "/repo/.worktrees/worktree-spec-044/subdir", nil
	}
	st := &HookState{ActiveWorktree: "/repo/.worktrees/worktree-spec-044"}
	if outsideActiveWorktree(st) {
		t.Error("CWD inside worktree should return false")
	}
}

func TestOutsideActiveWorktree_Outside(t *testing.T) {
	origGetCwd := getCwd
	t.Cleanup(func() { getCwd = origGetCwd })
	getCwd = func() (string, error) {
		return "/repo", nil
	}
	st := &HookState{ActiveWorktree: "/repo/.worktrees/worktree-spec-044"}
	if !outsideActiveWorktree(st) {
		t.Error("CWD outside worktree should return true")
	}
}

func TestOutsideActiveWorktree_CwdInDifferentWorktree(t *testing.T) {
	origGetCwd := getCwd
	t.Cleanup(func() { getCwd = origGetCwd })
	getCwd = func() (string, error) {
		return "/repo/.worktrees/worktree-spec-055", nil
	}
	// ActiveWorktree is a different spec's worktree
	st := &HookState{ActiveWorktree: "/repo/.worktrees/worktree-spec-044"}
	if !outsideActiveWorktree(st) {
		t.Error("CWD in different worktree should return true")
	}
}

func TestOutsideActiveWorktree_CwdInWorktreeNoActiveSet(t *testing.T) {
	origGetCwd := getCwd
	t.Cleanup(func() { getCwd = origGetCwd })
	getCwd = func() (string, error) {
		return "/repo/.worktrees/worktree-spec-055/subdir", nil
	}
	// No ActiveWorktree set at all
	st := &HookState{Mode: "plan"}
	if !outsideActiveWorktree(st) {
		t.Error("CWD in any worktree with no ActiveWorktree should return true")
	}
}

func TestOutsideActiveWorktree_CwdInNestedWorktree(t *testing.T) {
	origGetCwd := getCwd
	t.Cleanup(func() { getCwd = origGetCwd })
	getCwd = func() (string, error) {
		return "/repo/.worktrees/worktree-spec-044/.worktrees/worktree-bead-abc", nil
	}
	st := &HookState{ActiveWorktree: "/repo/.worktrees/worktree-spec-044"}
	if !outsideActiveWorktree(st) {
		t.Error("CWD in nested worktree (bead inside spec) should return true")
	}
}

func TestOutsideActiveWorktree_CwdInActiveWorktree(t *testing.T) {
	origGetCwd := getCwd
	t.Cleanup(func() { getCwd = origGetCwd })
	getCwd = func() (string, error) {
		return "/repo/.worktrees/worktree-spec-044/internal", nil
	}
	st := &HookState{ActiveWorktree: "/repo/.worktrees/worktree-spec-044"}
	if outsideActiveWorktree(st) {
		t.Error("CWD inside active worktree should return false")
	}
}

// --- dirExists guard ---

func TestDirExists_DeletedPath(t *testing.T) {
	t.Parallel()
	if dirExists("/nonexistent/path/that/does/not/exist") {
		t.Error("dirExists should return false for nonexistent path")
	}
}

func TestDirExists_RealDir(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	if !dirExists(d) {
		t.Error("dirExists should return true for existing dir")
	}
}

// --- Run dispatcher ---

func TestRun_UnknownHook(t *testing.T) {
	t.Parallel()
	r := Run("nonexistent", &Input{}, nil, true)
	if r.Action != Pass {
		t.Error("unknown hook should pass")
	}
}
