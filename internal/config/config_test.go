package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.ProtectedBranches) != 2 {
		t.Fatalf("expected 2 protected branches, got %d", len(cfg.ProtectedBranches))
	}
	if cfg.ProtectedBranches[0] != "main" || cfg.ProtectedBranches[1] != "master" {
		t.Errorf("unexpected protected branches: %v", cfg.ProtectedBranches)
	}
	if cfg.MergeStrategy != "auto" {
		t.Errorf("expected merge_strategy=auto, got %q", cfg.MergeStrategy)
	}
	if cfg.WorktreeRoot != ".worktrees" {
		t.Errorf("expected worktree_root=.worktrees, got %q", cfg.WorktreeRoot)
	}
	if !cfg.Enforcement.PreCommitHook {
		t.Error("expected pre_commit_hook=true")
	}
	if !cfg.Enforcement.CLIGuards {
		t.Error("expected cli_guards=true")
	}
	if !cfg.Enforcement.AgentHooks {
		t.Error("expected agent_hooks=true")
	}
	if cfg.Recording.Enabled {
		t.Error("expected recording.enabled=false by default")
	}
	if cfg.Decomposition.MaxBeads != 6 {
		t.Errorf("expected decomposition.max_beads=6, got %d", cfg.Decomposition.MaxBeads)
	}
	if cfg.Decomposition.MaxScopeOverlap != 0.50 {
		t.Errorf("expected decomposition.max_scope_overlap=0.50, got %v", cfg.Decomposition.MaxScopeOverlap)
	}
	if cfg.Decomposition.MinScopeOverlap != 0.15 {
		t.Errorf("expected decomposition.min_scope_overlap=0.15, got %v", cfg.Decomposition.MinScopeOverlap)
	}
	if cfg.Decomposition.MaxChainDepth != 3 {
		t.Errorf("expected decomposition.max_chain_depth=3, got %d", cfg.Decomposition.MaxChainDepth)
	}
	if cfg.Decomposition.MinParallelism != 0.25 {
		t.Errorf("expected decomposition.min_parallelism=0.25, got %v", cfg.Decomposition.MinParallelism)
	}
}

func TestLoadFromFile_Decomposition(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `
decomposition:
  max_beads: 12
  max_chain_depth: 5
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Decomposition.MaxBeads != 12 {
		t.Errorf("expected max_beads=12, got %d", cfg.Decomposition.MaxBeads)
	}
	if cfg.Decomposition.MaxChainDepth != 5 {
		t.Errorf("expected max_chain_depth=5, got %d", cfg.Decomposition.MaxChainDepth)
	}
	// Untouched fields should backfill from defaults.
	if cfg.Decomposition.MaxScopeOverlap != 0.50 {
		t.Errorf("expected max_scope_overlap=0.50 (default), got %v", cfg.Decomposition.MaxScopeOverlap)
	}
	if cfg.Decomposition.MinScopeOverlap != 0.15 {
		t.Errorf("expected min_scope_overlap=0.15 (default), got %v", cfg.Decomposition.MinScopeOverlap)
	}
	if cfg.Decomposition.MinParallelism != 0.25 {
		t.Errorf("expected min_parallelism=0.25 (default), got %v", cfg.Decomposition.MinParallelism)
	}
}

func TestLoadMissing(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MergeStrategy != "auto" {
		t.Errorf("expected defaults when file missing, got merge_strategy=%q", cfg.MergeStrategy)
	}
}

func TestLoadFromFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `
protected_branches:
  - main
  - develop
merge_strategy: pr
worktree_root: .wt
enforcement:
  pre_commit_hook: true
  cli_guards: false
  agent_hooks: true
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.ProtectedBranches) != 2 || cfg.ProtectedBranches[1] != "develop" {
		t.Errorf("unexpected protected branches: %v", cfg.ProtectedBranches)
	}
	if cfg.MergeStrategy != "pr" {
		t.Errorf("expected merge_strategy=pr, got %q", cfg.MergeStrategy)
	}
	if cfg.WorktreeRoot != ".wt" {
		t.Errorf("expected worktree_root=.wt, got %q", cfg.WorktreeRoot)
	}
	if cfg.Enforcement.CLIGuards {
		t.Error("expected cli_guards=false")
	}
}

func TestRecordingEnabled(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `
recording:
  enabled: true
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Recording.Enabled {
		t.Error("expected recording.enabled=true")
	}
}

// TestSourceGlobs_RoundTrip covers the populated state of the spec
// 091 Req 11 `source_globs:` field: declared globs round-trip through
// Load unchanged.
func TestSourceGlobs_RoundTrip(t *testing.T) {
	ResetCache()
	defer ResetCache()

	root := t.TempDir()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `
source_globs:
  - cmd/**
  - internal/**
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.SourceGlobs) != 2 {
		t.Fatalf("expected 2 source_globs, got %d: %v", len(cfg.SourceGlobs), cfg.SourceGlobs)
	}
	if cfg.SourceGlobs[0] != "cmd/**" || cfg.SourceGlobs[1] != "internal/**" {
		t.Errorf("unexpected source_globs: %v", cfg.SourceGlobs)
	}
}

// TestSourceGlobs_DefaultEmptyWhenFieldAbsent covers the field-absent
// state: a config.yaml without `source_globs:` yields an empty slice
// (the documented empty default — no backfill, the framework never
// guesses globs).
func TestSourceGlobs_DefaultEmptyWhenFieldAbsent(t *testing.T) {
	ResetCache()
	defer ResetCache()

	root := t.TempDir()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("merge_strategy: pr\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.SourceGlobs) != 0 {
		t.Errorf("expected empty source_globs when field absent, got %v", cfg.SourceGlobs)
	}
}

// TestSourceGlobs_DefaultEmptyWhenFileAbsent covers the file-absent
// state (the common brownfield case — `mindspec init` does not create
// config.yaml): Load returns defaults with empty SourceGlobs.
func TestSourceGlobs_DefaultEmptyWhenFileAbsent(t *testing.T) {
	ResetCache()
	defer ResetCache()

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.SourceGlobs) != 0 {
		t.Errorf("expected empty source_globs when config file absent, got %v", cfg.SourceGlobs)
	}
}

func TestIsProtectedBranch(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.IsProtectedBranch("main") {
		t.Error("main should be protected")
	}
	if !cfg.IsProtectedBranch("master") {
		t.Error("master should be protected")
	}
	if cfg.IsProtectedBranch("feature/foo") {
		t.Error("feature/foo should not be protected")
	}
}

func TestWorktreePath(t *testing.T) {
	cfg := DefaultConfig()
	got := cfg.WorktreePath("/repo", "worktree-spec-046")
	want := filepath.Join("/repo", ".worktrees", "worktree-spec-046")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLoadCaches_SamePointer(t *testing.T) {
	ResetCache()
	defer ResetCache()

	dir := t.TempDir()
	a, err := Load(dir)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if a != b {
		t.Fatalf("expected identical *Config pointer from cache, got %p vs %p", a, b)
	}
}

func TestLoadCaches_DiskMutationIgnored(t *testing.T) {
	ResetCache()
	defer ResetCache()

	dir := t.TempDir()
	mindspecDir := filepath.Join(dir, ".mindspec")
	if err := os.MkdirAll(mindspecDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mindspecDir, "config.yaml"),
		[]byte("merge_strategy: rebase\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first.MergeStrategy != "rebase" {
		t.Fatalf("first load: want rebase, got %q", first.MergeStrategy)
	}

	// Mutate disk; cached Load must NOT pick this up.
	if err := os.WriteFile(filepath.Join(mindspecDir, "config.yaml"),
		[]byte("merge_strategy: squash\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	second, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if second.MergeStrategy != "rebase" {
		t.Fatalf("cache busted: want rebase (cached), got %q", second.MergeStrategy)
	}

	// ResetCache should force a re-read.
	ResetCache()
	third, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if third.MergeStrategy != "squash" {
		t.Fatalf("after reset: want squash, got %q", third.MergeStrategy)
	}
}

func TestLoadCaches_KeyedByAbsolutePath(t *testing.T) {
	ResetCache()
	defer ResetCache()

	dirA := t.TempDir()
	dirB := t.TempDir()
	a, err := Load(dirA)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load(dirB)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("distinct roots should yield distinct cache entries")
	}
}

// TestLoad_ZeroConfigPanelModelsLoopDefaults covers spec 109 AC1: an absent
// config.yaml yields the panel/models/loop/runner defaults documented in R2
// (panel), R3 (models), R4 (loop), R10 (runner) — and every pre-existing
// field is unchanged.
func TestLoad_ZeroConfigPanelModelsLoopDefaults(t *testing.T) {
	ResetCache()
	defer ResetCache()

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// panel:
	wantReviewers := []Reviewer{{Family: "claude", Count: intp(3)}, {Family: "codex", Count: intp(3)}}
	if !reflect.DeepEqual(cfg.Panel.Reviewers, wantReviewers) {
		t.Errorf("panel.reviewers: got %+v, want %+v", cfg.Panel.Reviewers, wantReviewers)
	}
	if cfg.Panel.ApproveThreshold != "n-1" {
		t.Errorf("panel.approve_threshold: got %q, want \"n-1\"", cfg.Panel.ApproveThreshold)
	}
	if !cfg.Panel.Substitution.ClaudeSubOnQuota {
		t.Error("panel.substitution.claude_sub_on_quota: want true by default")
	}

	// models: (empty map, each phase inherits the ambient model)
	if len(cfg.Models) != 0 {
		t.Errorf("models: want empty map, got %v", cfg.Models)
	}

	// loop:
	if cfg.Loop.Enabled {
		t.Error("loop.enabled: want false by default")
	}
	wantGateAuthority := map[string]string{
		"spec_approve": "human",
		"plan_approve": "human",
		"bead_merge":   "human",
		"impl_approve": "human",
	}
	if !reflect.DeepEqual(cfg.Loop.GateAuthority, wantGateAuthority) {
		t.Errorf("loop.gate_authority: got %v, want %v", cfg.Loop.GateAuthority, wantGateAuthority)
	}
	if cfg.Loop.Halt.MaxRoundsPerBead != 3 {
		t.Errorf("loop.halt.max_rounds_per_bead: got %d, want 3", cfg.Loop.Halt.MaxRoundsPerBead)
	}
	if cfg.Loop.Halt.PanelDeadlockRounds != 2 {
		t.Errorf("loop.halt.panel_deadlock_rounds: got %d, want 2", cfg.Loop.Halt.PanelDeadlockRounds)
	}
	if cfg.Loop.Halt.MaxConsecutiveImplFailures != 2 {
		t.Errorf("loop.halt.max_consecutive_impl_failures: got %d, want 2", cfg.Loop.Halt.MaxConsecutiveImplFailures)
	}
	if cfg.Loop.Halt.OnReject != "halt" {
		t.Errorf("loop.halt.on_reject: got %q, want \"halt\"", cfg.Loop.Halt.OnReject)
	}
	if cfg.Loop.Budget.MaxBeadsPerWake != 0 {
		t.Errorf("loop.budget.max_beads_per_wake: got %d, want 0 (unlimited)", cfg.Loop.Budget.MaxBeadsPerWake)
	}
	if cfg.Loop.Budget.TokenBudget != 0 {
		t.Errorf("loop.budget.token_budget: got %d, want 0 (unlimited)", cfg.Loop.Budget.TokenBudget)
	}
	if cfg.Loop.Context.ControllerHandoff != "per-spec" {
		t.Errorf("loop.context.controller_handoff: got %q, want \"per-spec\"", cfg.Loop.Context.ControllerHandoff)
	}
	if cfg.Loop.HandoffLog != "AUTOPILOT-LOG.md" {
		t.Errorf("loop.handoff_log: got %q, want \"AUTOPILOT-LOG.md\"", cfg.Loop.HandoffLog)
	}

	// runner:
	if cfg.Runner != "claude-code-skills" {
		t.Errorf("runner: got %q, want \"claude-code-skills\"", cfg.Runner)
	}

	// Every pre-existing field unchanged.
	if len(cfg.ProtectedBranches) != 2 || cfg.ProtectedBranches[0] != "main" || cfg.ProtectedBranches[1] != "master" {
		t.Errorf("protected_branches regressed: %v", cfg.ProtectedBranches)
	}
	if cfg.MergeStrategy != "auto" {
		t.Errorf("merge_strategy regressed: %q", cfg.MergeStrategy)
	}
	if cfg.WorktreeRoot != ".worktrees" {
		t.Errorf("worktree_root regressed: %q", cfg.WorktreeRoot)
	}
	if cfg.AutoFinalize {
		t.Error("auto_finalize regressed: want false")
	}
	if !cfg.Enforcement.PreCommitHook || !cfg.Enforcement.CLIGuards || !cfg.Enforcement.AgentHooks || !cfg.Enforcement.PanelGate {
		t.Errorf("enforcement regressed: %+v", cfg.Enforcement)
	}
	if cfg.Recording.Enabled {
		t.Error("recording.enabled regressed: want false")
	}
	wantDecomp := Decomposition{MaxBeads: 6, MaxScopeOverlap: 0.50, MinScopeOverlap: 0.15, MaxChainDepth: 3, MinParallelism: 0.25}
	if cfg.Decomposition != wantDecomp {
		t.Errorf("decomposition regressed: got %+v, want %+v", cfg.Decomposition, wantDecomp)
	}
	if len(cfg.SourceGlobs) != 0 {
		t.Errorf("source_globs regressed: %v", cfg.SourceGlobs)
	}
}

// TestPanelExpectedReviewers_SumsReviewerCounts covers spec 109 AC2:
// PanelExpectedReviewers sums reviewers[].count (6 for the default 3+3 mix,
// and the correct sum for a custom mix); PanelApproveThresholdExpr returns
// the raw expression, never resolving it to an int.
func TestPanelExpectedReviewers_SumsReviewerCounts(t *testing.T) {
	def := DefaultConfig()
	if got := def.PanelExpectedReviewers(); got != 6 {
		t.Errorf("default PanelExpectedReviewers: got %d, want 6", got)
	}
	if got := def.PanelApproveThresholdExpr(); got != "n-1" {
		t.Errorf("default PanelApproveThresholdExpr: got %q, want \"n-1\"", got)
	}

	custom := &Config{Panel: Panel{
		Reviewers:        []Reviewer{{Family: "claude", Count: intp(2)}, {Family: "codex", Count: intp(4)}},
		ApproveThreshold: "3",
	}}
	if got := custom.PanelExpectedReviewers(); got != 6 {
		t.Errorf("custom PanelExpectedReviewers: got %d, want 6", got)
	}
	if got := custom.PanelApproveThresholdExpr(); got != "3" {
		t.Errorf("custom PanelApproveThresholdExpr: got %q, want the raw \"3\" (unresolved)", got)
	}

	// Unset Panel (zero-value Config, not routed through Load) still
	// resolves to the documented defaults.
	unset := &Config{}
	if got := unset.PanelExpectedReviewers(); got != 6 {
		t.Errorf("unset PanelExpectedReviewers: got %d, want 6", got)
	}
	if got := unset.PanelApproveThresholdExpr(); got != "n-1" {
		t.Errorf("unset PanelApproveThresholdExpr: got %q, want \"n-1\"", got)
	}
}

// TestLoad_RefusesUnweakenableKnobs covers spec 109 AC3/R5: each
// un-weakenable knob and each threshold-skip/reviewer-floor loophole makes
// Load return an error.
func TestLoad_RefusesUnweakenableKnobs(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			name:    "panel_skip under gate_authority",
			content: "loop:\n  gate_authority:\n    panel_skip: human\n",
		},
		{
			name:    "on_reject other than halt",
			content: "loop:\n  halt:\n    on_reject: auto-fix\n",
		},
		{
			name:    "gate_authority value out of enum",
			content: "loop:\n  gate_authority:\n    bead_merge: robot\n",
		},
		{
			name:    "controller_handoff out of enum",
			content: "loop:\n  context:\n    controller_handoff: never\n",
		},
		{
			name:    "unknown runner",
			content: "runner: mystery-adapter\n",
		},
		{
			name:    "approve_threshold zero",
			content: "panel:\n  approve_threshold: \"0\"\n",
		},
		{
			name:    "approve_threshold negative",
			content: "panel:\n  approve_threshold: \"-1\"\n",
		},
		{
			name:    "approve_threshold exceeds reviewer count",
			content: "panel:\n  reviewers:\n    - family: claude\n      count: 2\n    - family: codex\n      count: 2\n  approve_threshold: \"5\"\n",
		},
		{
			name:    "reviewers count below 1",
			content: "panel:\n  reviewers:\n    - family: claude\n      count: 0\n    - family: codex\n      count: 3\n",
		},
		{
			name:    "reviewers sum below 2",
			content: "panel:\n  reviewers:\n    - family: claude\n      count: 1\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ResetCache()
			defer ResetCache()

			root := t.TempDir()
			dir := filepath.Join(root, ".mindspec")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}

			if _, err := Load(root); err == nil {
				t.Errorf("expected Load to refuse %q, got nil error", tc.name)
			}
		})
	}
}

// TestLoad_PopulatedConfigRoundTrips covers spec 109 AC7: a fully populated
// panel/models/loop/runner config round-trips through Load unchanged.
func TestLoad_PopulatedConfigRoundTrips(t *testing.T) {
	ResetCache()
	defer ResetCache()

	root := t.TempDir()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `
panel:
  reviewers:
    - family: claude
      count: 2
    - family: codex
      count: 2
  approve_threshold: "3"
  substitution:
    claude_sub_on_quota: false
models:
  implement: opus
  review: fable
  authoring: opus
  grill: sonnet
  final_review: fable
loop:
  enabled: true
  gate_authority:
    spec_approve: panel
    plan_approve: panel
    bead_merge: human
    impl_approve: panel
  halt:
    max_rounds_per_bead: 5
    panel_deadlock_rounds: 3
    max_consecutive_impl_failures: 4
    on_reject: halt
  budget:
    max_beads_per_wake: 10
    token_budget: 500000
  context:
    controller_handoff: at-usage-threshold
  handoff_log: CUSTOM-LOG.md
runner: claude-code-workflow
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantReviewers := []Reviewer{{Family: "claude", Count: intp(2)}, {Family: "codex", Count: intp(2)}}
	if !reflect.DeepEqual(cfg.Panel.Reviewers, wantReviewers) {
		t.Errorf("panel.reviewers: got %+v, want %+v", cfg.Panel.Reviewers, wantReviewers)
	}
	if cfg.Panel.ApproveThreshold != "3" {
		t.Errorf("panel.approve_threshold: got %q, want \"3\"", cfg.Panel.ApproveThreshold)
	}
	if cfg.Panel.Substitution.ClaudeSubOnQuota {
		t.Error("panel.substitution.claude_sub_on_quota: want false (explicit override)")
	}

	wantModels := map[string]string{
		"implement":    "opus",
		"review":       "fable",
		"authoring":    "opus",
		"grill":        "sonnet",
		"final_review": "fable",
	}
	if !reflect.DeepEqual(cfg.Models, wantModels) {
		t.Errorf("models: got %v, want %v", cfg.Models, wantModels)
	}

	if !cfg.Loop.Enabled {
		t.Error("loop.enabled: want true (explicit override)")
	}
	wantGateAuthority := map[string]string{
		"spec_approve": "panel",
		"plan_approve": "panel",
		"bead_merge":   "human",
		"impl_approve": "panel",
	}
	if !reflect.DeepEqual(cfg.Loop.GateAuthority, wantGateAuthority) {
		t.Errorf("loop.gate_authority: got %v, want %v", cfg.Loop.GateAuthority, wantGateAuthority)
	}
	if cfg.Loop.Halt.MaxRoundsPerBead != 5 {
		t.Errorf("loop.halt.max_rounds_per_bead: got %d, want 5", cfg.Loop.Halt.MaxRoundsPerBead)
	}
	if cfg.Loop.Halt.PanelDeadlockRounds != 3 {
		t.Errorf("loop.halt.panel_deadlock_rounds: got %d, want 3", cfg.Loop.Halt.PanelDeadlockRounds)
	}
	if cfg.Loop.Halt.MaxConsecutiveImplFailures != 4 {
		t.Errorf("loop.halt.max_consecutive_impl_failures: got %d, want 4", cfg.Loop.Halt.MaxConsecutiveImplFailures)
	}
	if cfg.Loop.Halt.OnReject != "halt" {
		t.Errorf("loop.halt.on_reject: got %q, want \"halt\"", cfg.Loop.Halt.OnReject)
	}
	if cfg.Loop.Budget.MaxBeadsPerWake != 10 {
		t.Errorf("loop.budget.max_beads_per_wake: got %d, want 10", cfg.Loop.Budget.MaxBeadsPerWake)
	}
	if cfg.Loop.Budget.TokenBudget != 500000 {
		t.Errorf("loop.budget.token_budget: got %d, want 500000", cfg.Loop.Budget.TokenBudget)
	}
	if cfg.Loop.Context.ControllerHandoff != "at-usage-threshold" {
		t.Errorf("loop.context.controller_handoff: got %q, want \"at-usage-threshold\"", cfg.Loop.Context.ControllerHandoff)
	}
	if cfg.Loop.HandoffLog != "CUSTOM-LOG.md" {
		t.Errorf("loop.handoff_log: got %q, want \"CUSTOM-LOG.md\"", cfg.Loop.HandoffLog)
	}

	if cfg.Runner != "claude-code-workflow" {
		t.Errorf("runner: got %q, want \"claude-code-workflow\"", cfg.Runner)
	}

	// Resolvers reflect the populated config.
	if got := cfg.PanelExpectedReviewers(); got != 4 {
		t.Errorf("PanelExpectedReviewers: got %d, want 4", got)
	}
	if got := cfg.PanelApproveThresholdExpr(); got != "3" {
		t.Errorf("PanelApproveThresholdExpr: got %q, want the raw \"3\" (unresolved)", got)
	}
}

// TestLoad_GatesAbsentByteIdentical109 covers spec 112 AC1/R2: zero-config
// defaults equal 109's exactly; the 109 AC7 populated fixture loads with
// every field unchanged; with gates absent every gate-scoped resolver
// returns the global-derived values; `gates: {}` behaves identically to an
// absent `gates:` key everywhere; and a count-less reviewer entry — which
// 109 refuses outright — loads as count: 1 (the one deliberate monotone
// relaxation).
func TestLoad_GatesAbsentByteIdentical109(t *testing.T) {
	ResetCache()
	defer ResetCache()

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantReviewers := []Reviewer{{Family: "claude", Count: intp(3)}, {Family: "codex", Count: intp(3)}}
	if !reflect.DeepEqual(cfg.Panel.Reviewers, wantReviewers) {
		t.Errorf("panel.reviewers: got %+v, want %+v", cfg.Panel.Reviewers, wantReviewers)
	}
	if cfg.Panel.ApproveThreshold != "n-1" {
		t.Errorf("panel.approve_threshold: got %q, want \"n-1\"", cfg.Panel.ApproveThreshold)
	}
	if !cfg.Panel.Substitution.ClaudeSubOnQuota {
		t.Error("panel.substitution.claude_sub_on_quota: want true by default")
	}
	if len(cfg.Panel.Gates) != 0 {
		t.Errorf("panel.gates: want empty by default, got %v", cfg.Panel.Gates)
	}
	if len(cfg.Panel.Substitution.Substitutes) != 0 {
		t.Errorf("panel.substitution.substitutes: want empty by default, got %v", cfg.Panel.Substitution.Substitutes)
	}
	if cfg.Panel.Note != "" {
		t.Errorf("panel.note: want empty by default, got %q", cfg.Panel.Note)
	}

	// The 109 AC7 populated fixture loads with every field unchanged.
	root := t.TempDir()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `
panel:
  reviewers:
    - family: claude
      count: 2
    - family: codex
      count: 2
  approve_threshold: "3"
  substitution:
    claude_sub_on_quota: false
runner: claude-code-workflow
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	ResetCache()
	fixtureCfg, err := Load(root)
	if err != nil {
		t.Fatalf("109 fixture: unexpected error: %v", err)
	}
	wantFixtureReviewers := []Reviewer{{Family: "claude", Count: intp(2)}, {Family: "codex", Count: intp(2)}}
	if !reflect.DeepEqual(fixtureCfg.Panel.Reviewers, wantFixtureReviewers) {
		t.Errorf("109 fixture panel.reviewers: got %+v, want %+v", fixtureCfg.Panel.Reviewers, wantFixtureReviewers)
	}
	if fixtureCfg.Panel.ApproveThreshold != "3" {
		t.Errorf("109 fixture approve_threshold: got %q, want \"3\"", fixtureCfg.Panel.ApproveThreshold)
	}
	if fixtureCfg.Panel.Substitution.ClaudeSubOnQuota {
		t.Error("109 fixture claude_sub_on_quota: want false (explicit override)")
	}
	if fixtureCfg.Runner != "claude-code-workflow" {
		t.Errorf("109 fixture runner: got %q, want \"claude-code-workflow\"", fixtureCfg.Runner)
	}
	if len(fixtureCfg.Panel.Gates) != 0 || len(fixtureCfg.Panel.Substitution.Substitutes) != 0 {
		t.Errorf("109 fixture gates/substitutes must stay empty: gates=%v substitutes=%v", fixtureCfg.Panel.Gates, fixtureCfg.Panel.Substitution.Substitutes)
	}

	// Gates absent: every gate-scoped resolver returns the global-derived
	// values, for both the zero-config and the populated fixture.
	for _, c := range []*Config{cfg, fixtureCfg} {
		wantSum := c.PanelExpectedReviewers()
		wantExpr := c.PanelApproveThresholdExpr()
		for _, gate := range PanelGateKeys {
			gotSum, err := c.PanelGateExpectedReviewers(gate)
			if err != nil {
				t.Fatalf("PanelGateExpectedReviewers(%s): %v", gate, err)
			}
			if gotSum != wantSum {
				t.Errorf("PanelGateExpectedReviewers(%s) = %d, want global %d (gates absent)", gate, gotSum, wantSum)
			}
			gotExpr, err := c.PanelGateApproveThresholdExpr(gate)
			if err != nil {
				t.Fatalf("PanelGateApproveThresholdExpr(%s): %v", gate, err)
			}
			if gotExpr != wantExpr {
				t.Errorf("PanelGateApproveThresholdExpr(%s) = %q, want global %q (gates absent)", gate, gotExpr, wantExpr)
			}
		}
	}

	// `gates: {}` (present-but-empty) behaves identically to absent.
	root2 := t.TempDir()
	dir2 := filepath.Join(root2, ".mindspec")
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "config.yaml"), []byte("panel:\n  gates: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ResetCache()
	emptyGatesCfg, err := Load(root2)
	if err != nil {
		t.Fatalf("gates: {}: unexpected error: %v", err)
	}
	if len(emptyGatesCfg.Panel.Gates) != 0 {
		t.Errorf("gates: {} should parse as empty, got %v", emptyGatesCfg.Panel.Gates)
	}
	if !reflect.DeepEqual(emptyGatesCfg.Panel.Reviewers, wantReviewers) {
		t.Errorf("gates: {} must not change reviewers: got %+v", emptyGatesCfg.Panel.Reviewers)
	}
	for _, gate := range PanelGateKeys {
		got, err := emptyGatesCfg.PanelGateExpectedReviewers(gate)
		if err != nil || got != 6 {
			t.Errorf("gates:{} PanelGateExpectedReviewers(%s) = %d, err=%v, want 6", gate, got, err)
		}
	}

	// A count-less reviewer entry — which 109 refuses outright — loads as
	// count: 1.
	root3 := t.TempDir()
	dir3 := filepath.Join(root3, ".mindspec")
	if err := os.MkdirAll(dir3, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir3, "config.yaml"), []byte("panel:\n  reviewers:\n    - family: claude\n    - family: codex\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ResetCache()
	countlessCfg, err := Load(root3)
	if err != nil {
		t.Fatalf("count-less entry: unexpected error: %v", err)
	}
	if len(countlessCfg.Panel.Reviewers) != 2 {
		t.Fatalf("count-less entry: want 2 reviewers, got %d", len(countlessCfg.Panel.Reviewers))
	}
	for _, r := range countlessCfg.Panel.Reviewers {
		if r.Count != nil {
			t.Errorf("count-less entry: want nil Count (unset), got %d", *r.Count)
		}
		if r.CountValue() != 1 {
			t.Errorf("count-less entry: CountValue() = %d, want 1", r.CountValue())
		}
	}
}

// TestLoad_PerGateProtocolRoundTrips covers spec 112 AC2/R1: the Goal's
// full standing-protocol YAML (four gates, mixed compact/exploded entries,
// explicit lenses, substitutes) round-trips through Load with every field
// equal to what was written, and two loads differing only in panel.note
// yield identical validation results and identical resolver outputs
// (note-inertness).
func TestLoad_PerGateProtocolRoundTrips(t *testing.T) {
	ResetCache()
	defer ResetCache()

	protocolYAML := `
panel:
  note: "fable-window 2026-07, codex-enabled"
  reviewers:
    - {family: claude, count: 3}
    - {family: codex, count: 3}
  approve_threshold: "n-1"
  substitution:
    claude_sub_on_quota: true
    substitutes:
      gpt-5.5: claude-sonnet-5
  gates:
    spec_approve:
      reviewers:
        - {model: claude-fable-5, count: 3}
        - {model: claude-opus-4-8, count: 3}
        - {model: gpt-5.5, count: 3}
    plan_approve:
      reviewers:
        - {model: claude-fable-5, count: 3}
        - {model: claude-opus-4-8, count: 3}
        - {model: gpt-5.5, count: 3}
    bead:
      reviewers:
        - {model: claude-opus-4-8, lens: author-of-record}
        - {model: claude-opus-4-8, lens: codebase-pin}
        - {model: claude-opus-4-8, lens: contract-stability}
        - {model: claude-sonnet-5, lens: empirical-prober}
        - {model: claude-sonnet-5, lens: adversarial}
        - {model: claude-sonnet-5, lens: integration}
    final_review:
      reviewers:
        - {model: claude-fable-5, count: 3}
        - {model: claude-opus-4-8, count: 3}
        - {model: gpt-5.5, count: 3}
        - {model: claude-sonnet-5, count: 3}
      approve_threshold: "11"
`
	root := t.TempDir()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(protocolYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Panel.Note != "fable-window 2026-07, codex-enabled" {
		t.Errorf("panel.note: got %q", cfg.Panel.Note)
	}
	wantGlobal := []Reviewer{{Family: "claude", Count: intp(3)}, {Family: "codex", Count: intp(3)}}
	if !reflect.DeepEqual(cfg.Panel.Reviewers, wantGlobal) {
		t.Errorf("panel.reviewers: got %+v, want %+v", cfg.Panel.Reviewers, wantGlobal)
	}
	if cfg.Panel.ApproveThreshold != "n-1" {
		t.Errorf("panel.approve_threshold: got %q", cfg.Panel.ApproveThreshold)
	}
	if !cfg.Panel.Substitution.ClaudeSubOnQuota {
		t.Error("claude_sub_on_quota: want true")
	}
	wantSubstitutes := map[string]string{"gpt-5.5": "claude-sonnet-5"}
	if !reflect.DeepEqual(cfg.Panel.Substitution.Substitutes, wantSubstitutes) {
		t.Errorf("substitutes: got %v, want %v", cfg.Panel.Substitution.Substitutes, wantSubstitutes)
	}

	if len(cfg.Panel.Gates) != 4 {
		t.Fatalf("expected exactly 4 configured gates (no adhoc), got %d: %v", len(cfg.Panel.Gates), cfg.Panel.Gates)
	}
	if _, ok := cfg.Panel.Gates["adhoc"]; ok {
		t.Error("adhoc must stay unconfigured in this fixture")
	}

	wantNineMix := []Reviewer{
		{Model: "claude-fable-5", Count: intp(3)},
		{Model: "claude-opus-4-8", Count: intp(3)},
		{Model: "gpt-5.5", Count: intp(3)},
	}
	if !reflect.DeepEqual(cfg.Panel.Gates["spec_approve"].Reviewers, wantNineMix) {
		t.Errorf("spec_approve reviewers: got %+v, want %+v", cfg.Panel.Gates["spec_approve"].Reviewers, wantNineMix)
	}
	if !reflect.DeepEqual(cfg.Panel.Gates["plan_approve"].Reviewers, wantNineMix) {
		t.Errorf("plan_approve reviewers: got %+v, want %+v", cfg.Panel.Gates["plan_approve"].Reviewers, wantNineMix)
	}

	wantBead := []Reviewer{
		{Model: "claude-opus-4-8", Lens: "author-of-record"},
		{Model: "claude-opus-4-8", Lens: "codebase-pin"},
		{Model: "claude-opus-4-8", Lens: "contract-stability"},
		{Model: "claude-sonnet-5", Lens: "empirical-prober"},
		{Model: "claude-sonnet-5", Lens: "adversarial"},
		{Model: "claude-sonnet-5", Lens: "integration"},
	}
	if !reflect.DeepEqual(cfg.Panel.Gates["bead"].Reviewers, wantBead) {
		t.Errorf("bead reviewers: got %+v, want %+v", cfg.Panel.Gates["bead"].Reviewers, wantBead)
	}

	wantFinalReview := []Reviewer{
		{Model: "claude-fable-5", Count: intp(3)},
		{Model: "claude-opus-4-8", Count: intp(3)},
		{Model: "gpt-5.5", Count: intp(3)},
		{Model: "claude-sonnet-5", Count: intp(3)},
	}
	if !reflect.DeepEqual(cfg.Panel.Gates["final_review"].Reviewers, wantFinalReview) {
		t.Errorf("final_review reviewers: got %+v, want %+v", cfg.Panel.Gates["final_review"].Reviewers, wantFinalReview)
	}
	if cfg.Panel.Gates["final_review"].ApproveThreshold != "11" {
		t.Errorf("final_review approve_threshold: got %q, want \"11\"", cfg.Panel.Gates["final_review"].ApproveThreshold)
	}

	// Resolved sums match the Goal's documented example (9/9/6/12), plus
	// unconfigured adhoc resolving to bead's mix (6).
	wantSums := map[string]int{"spec_approve": 9, "plan_approve": 9, "bead": 6, "final_review": 12, "adhoc": 6}
	for gate, want := range wantSums {
		got, err := cfg.PanelGateExpectedReviewers(gate)
		if err != nil {
			t.Fatalf("PanelGateExpectedReviewers(%s): %v", gate, err)
		}
		if got != want {
			t.Errorf("PanelGateExpectedReviewers(%s) = %d, want %d", gate, got, want)
		}
	}
	beadSlots, err := cfg.PanelGateReviewerSlots("bead")
	if err != nil {
		t.Fatalf("bead slots: %v", err)
	}
	adhocSlots, err := cfg.PanelGateReviewerSlots("adhoc")
	if err != nil {
		t.Fatalf("adhoc slots: %v", err)
	}
	if !reflect.DeepEqual(beadSlots, adhocSlots) {
		t.Errorf("unconfigured adhoc must equal bead's resolved slots: bead=%+v adhoc=%+v", beadSlots, adhocSlots)
	}

	// Two loads differing ONLY in panel.note yield identical validation
	// results and identical resolver outputs (note-inertness, R1).
	noteChanged := strings.Replace(protocolYAML, `note: "fable-window 2026-07, codex-enabled"`, `note: "a completely different note"`, 1)
	root2 := t.TempDir()
	dir2 := filepath.Join(root2, ".mindspec")
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "config.yaml"), []byte(noteChanged), 0644); err != nil {
		t.Fatal(err)
	}
	ResetCache()
	cfg2, err := Load(root2)
	if err != nil {
		t.Fatalf("note-varied fixture: unexpected error: %v", err)
	}
	if cfg2.Panel.Note == cfg.Panel.Note {
		t.Fatal("test bug: the note-varied fixture must actually differ in panel.note")
	}
	for _, gate := range PanelGateKeys {
		s1, err1 := cfg.PanelGateExpectedReviewers(gate)
		s2, err2 := cfg2.PanelGateExpectedReviewers(gate)
		if err1 != nil || err2 != nil || s1 != s2 {
			t.Errorf("gate %s: expected-reviewers diverged across differing panel.note: (%d,%v) vs (%d,%v)", gate, s1, err1, s2, err2)
		}
		e1, err1 := cfg.PanelGateApproveThresholdExpr(gate)
		e2, err2 := cfg2.PanelGateApproveThresholdExpr(gate)
		if err1 != nil || err2 != nil || e1 != e2 {
			t.Errorf("gate %s: threshold expr diverged across differing panel.note: (%q,%v) vs (%q,%v)", gate, e1, err1, e2, err2)
		}
	}
}

// TestLoad_RefusesPerGateKnobs covers spec 112 AC3/R4: every per-gate
// refusal (a)-(h) errors with a recovery line — including the two named
// inheritance shapes — and an unknown model id alone does NOT error.
func TestLoad_RefusesPerGateKnobs(t *testing.T) {
	cases := []struct {
		name          string
		content       string
		wantErr       bool
		recoveryWants []string
	}{
		{
			name:          "unknown gate key",
			content:       "panel:\n  gates:\n    bad_gate:\n      approve_threshold: \"n-1\"\n",
			wantErr:       true,
			recoveryWants: []string{"spec_approve", "plan_approve", "bead", "final_review", "adhoc", "recovery:"},
		},
		{
			name:          "empty gate entry sets neither reviewers nor approve_threshold",
			content:       "panel:\n  gates:\n    bead: {}\n",
			wantErr:       true,
			recoveryWants: []string{"recovery:"},
		},
		{
			name:          "reviewer entry with neither model nor family",
			content:       "panel:\n  gates:\n    bead:\n      reviewers:\n        - {count: 3}\n        - {model: claude-opus-4-8, count: 3}\n",
			wantErr:       true,
			recoveryWants: []string{"recovery:"},
		},
		{
			name:          "explicit non-positive count",
			content:       "panel:\n  gates:\n    bead:\n      reviewers:\n        - {model: claude-opus-4-8, count: 0}\n        - {model: claude-sonnet-5, count: 3}\n",
			wantErr:       true,
			recoveryWants: []string{"recovery:"},
		},
		{
			name:          "per-gate reviewer sum below 2",
			content:       "panel:\n  gates:\n    bead:\n      reviewers:\n        - {model: claude-opus-4-8, count: 1}\n",
			wantErr:       true,
			recoveryWants: []string{"recovery:"},
		},
		{
			name:          "per-gate threshold zero",
			content:       "panel:\n  gates:\n    bead:\n      reviewers:\n        - {model: claude-opus-4-8, count: 3}\n        - {model: claude-sonnet-5, count: 3}\n      approve_threshold: \"0\"\n",
			wantErr:       true,
			recoveryWants: []string{"recovery:"},
		},
		{
			name:          "per-gate threshold negative",
			content:       "panel:\n  gates:\n    bead:\n      reviewers:\n        - {model: claude-opus-4-8, count: 3}\n        - {model: claude-sonnet-5, count: 3}\n      approve_threshold: \"-1\"\n",
			wantErr:       true,
			recoveryWants: []string{"recovery:"},
		},
		{
			name:          "per-gate threshold exceeds own resolved sum",
			content:       "panel:\n  gates:\n    bead:\n      reviewers:\n        - {model: claude-opus-4-8, count: 3}\n        - {model: claude-sonnet-5, count: 3}\n      approve_threshold: \"7\"\n",
			wantErr:       true,
			recoveryWants: []string{"recovery:"},
		},
		{
			name:          "global integer threshold inherited by a differently-sized per-gate reviewers-only entry",
			content:       "panel:\n  approve_threshold: \"5\"\n  gates:\n    bead:\n      reviewers:\n        - {model: claude-opus-4-8, count: 1}\n        - {model: claude-sonnet-5, count: 1}\n",
			wantErr:       true,
			recoveryWants: []string{"recovery:"},
		},
		{
			name:          "bead integer threshold inherited through adhoc->bead by a smaller reviewers-only adhoc",
			content:       "panel:\n  gates:\n    bead:\n      reviewers:\n        - {model: claude-opus-4-8, count: 3}\n        - {model: claude-sonnet-5, count: 3}\n      approve_threshold: \"5\"\n    adhoc:\n      reviewers:\n        - {model: claude-opus-4-8, count: 1}\n        - {model: claude-sonnet-5, count: 1}\n",
			wantErr:       true,
			recoveryWants: []string{"recovery:"},
		},
		{
			name:          "substitutes self-mapping",
			content:       "panel:\n  substitution:\n    substitutes:\n      claude: claude\n",
			wantErr:       true,
			recoveryWants: []string{"recovery:"},
		},
		{
			name:          "substitutes empty key",
			content:       "panel:\n  substitution:\n    substitutes:\n      \"\": claude-sonnet-5\n",
			wantErr:       true,
			recoveryWants: []string{"recovery:"},
		},
		{
			name:          "substitutes empty value",
			content:       "panel:\n  substitution:\n    substitutes:\n      gpt-5.5: \"\"\n",
			wantErr:       true,
			recoveryWants: []string{"recovery:"},
		},
		{
			name:    "unknown model id alone does not error",
			content: "panel:\n  reviewers:\n    - {model: totally-made-up-model-xyz, count: 3}\n    - {family: codex, count: 3}\n",
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ResetCache()
			defer ResetCache()

			root := t.TempDir()
			dir := filepath.Join(root, ".mindspec")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := Load(root)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected Load to refuse %q, got nil error", tc.name)
				}
				for _, want := range tc.recoveryWants {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("%s: error message missing %q: %v", tc.name, want, err)
					}
				}
			} else if err != nil {
				t.Fatalf("expected Load to accept %q, got error: %v", tc.name, err)
			}
		})
	}
}

// TestLoad_UnknownGateKeyEscapesControlBytes is the round-1 panel G1.2
// regression: a hostile `panel.gates` key carrying ESC, BEL, and an
// embedded newline must not reach the Load refusal text raw. Before the
// G1.1 fix, the recovery clause repeated the offending key via a bare %s
// (the message clause already quoted it via %q) — control bytes reached
// stderr unescaped and the embedded newline split the error into extra
// physical lines, forging a line that could itself masquerade as a
// `recovery: ` line.
func TestLoad_UnknownGateKeyEscapesControlBytes(t *testing.T) {
	ResetCache()
	defer ResetCache()

	// YAML double-quoted scalar escapes: \a = BEL (0x07), \e = ESC (0x1b),
	// \n = a literal newline byte in the decoded key — not a raw newline in
	// this Go source file.
	content := "panel:\n  gates:\n    \"bad\\a\\e\\nkey\": {}\n"

	root := t.TempDir()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(root)
	if err == nil {
		t.Fatal("expected Load to refuse the unknown gate key, got nil error")
	}
	msg := err.Error()

	if strings.ContainsRune(msg, '\x07') {
		t.Errorf("error text contains a raw BEL byte: %q", msg)
	}
	if strings.ContainsRune(msg, '\x1b') {
		t.Errorf("error text contains a raw ESC byte: %q", msg)
	}

	// The embedded newline in the offending key must appear escaped
	// (literal backslash-n via strconv.Quote), not as a raw newline byte —
	// otherwise it forges an extra physical line. The only legitimate raw
	// newline in the whole message is the single message/recovery
	// separator, so there must be exactly one.
	if n := strings.Count(msg, "\n"); n != 1 {
		t.Errorf("error text has %d newlines, want exactly 1 (message/recovery separator only) — an embedded raw newline forged an extra physical line: %q", n, msg)
	}

	lines := strings.Split(msg, "\n")
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "recovery: ") {
		t.Errorf("final line does not start with %q, a forged/split recovery line: %q", "recovery: ", last)
	}
}

// TestPanelGateResolvers_FallbackAndAdhoc covers spec 112 AC4/R3: a
// configured gate resolves to its own values; threshold-only and
// reviewers-only gate entries inherit the missing half per field
// (including through adhoc's chain in both directions); an unconfigured
// gate resolves to the global default; unconfigured adhoc resolves to
// bead's resolved mix; an unknown gate name errors; and the threshold
// resolver always returns the raw expression, never a resolved integer.
func TestPanelGateResolvers_FallbackAndAdhoc(t *testing.T) {
	cfg := &Config{Panel: Panel{
		Reviewers:        []Reviewer{{Family: "claude", Count: intp(3)}, {Family: "codex", Count: intp(3)}},
		ApproveThreshold: "n-1",
		Gates: map[string]GatePanel{
			"bead": {
				Reviewers:        []Reviewer{{Model: "claude-opus-4-8", Count: intp(2)}, {Model: "claude-sonnet-5", Count: intp(2)}},
				ApproveThreshold: "3",
			},
			// threshold-only: inherits the global reviewers (6).
			"final_review": {ApproveThreshold: "5"},
			// reviewers-only: inherits bead's resolved threshold ("3").
			"adhoc": {Reviewers: []Reviewer{{Model: "gpt-5.5", Count: intp(2)}}},
		},
	}}

	if got, err := cfg.PanelGateExpectedReviewers("bead"); err != nil || got != 4 {
		t.Errorf("bead reviewers (own): got %d, err=%v, want 4", got, err)
	}
	if got, err := cfg.PanelGateApproveThresholdExpr("bead"); err != nil || got != "3" {
		t.Errorf("bead threshold (own): got %q, err=%v, want \"3\"", got, err)
	}

	if got, err := cfg.PanelGateExpectedReviewers("final_review"); err != nil || got != 6 {
		t.Errorf("final_review reviewers (inherited global): got %d, err=%v, want 6", got, err)
	}
	if got, err := cfg.PanelGateApproveThresholdExpr("final_review"); err != nil || got != "5" {
		t.Errorf("final_review threshold (own): got %q, err=%v, want \"5\"", got, err)
	}

	if got, err := cfg.PanelGateExpectedReviewers("adhoc"); err != nil || got != 2 {
		t.Errorf("adhoc reviewers (own): got %d, err=%v, want 2", got, err)
	}
	if got, err := cfg.PanelGateApproveThresholdExpr("adhoc"); err != nil || got != "3" {
		t.Errorf("adhoc threshold (inherited from bead): got %q, err=%v, want \"3\"", got, err)
	}

	if got, err := cfg.PanelGateExpectedReviewers("spec_approve"); err != nil || got != 6 {
		t.Errorf("spec_approve (unconfigured) reviewers: got %d, err=%v, want global 6", got, err)
	}
	if got, err := cfg.PanelGateApproveThresholdExpr("spec_approve"); err != nil || got != "n-1" {
		t.Errorf("spec_approve (unconfigured) threshold: got %q, err=%v, want global \"n-1\"", got, err)
	}

	if _, err := cfg.PanelGateExpectedReviewers("bogus"); err == nil {
		t.Error("expected error for unknown gate name (PanelGateExpectedReviewers)")
	}
	if _, err := cfg.PanelGateApproveThresholdExpr("bogus"); err == nil {
		t.Error("expected error for unknown gate name (PanelGateApproveThresholdExpr)")
	}
	if _, err := cfg.PanelGateReviewerSlots("bogus"); err == nil {
		t.Error("expected error for unknown gate name (PanelGateReviewerSlots)")
	}

	// Unconfigured adhoc resolves to bead's resolved mix (a separate config
	// with no adhoc entry at all).
	cfg2 := &Config{Panel: Panel{
		Reviewers: []Reviewer{{Family: "claude", Count: intp(3)}, {Family: "codex", Count: intp(3)}},
		Gates: map[string]GatePanel{
			"bead": {
				Reviewers:        []Reviewer{{Model: "claude-opus-4-8", Count: intp(3)}, {Model: "claude-sonnet-5", Count: intp(3)}},
				ApproveThreshold: "5",
			},
		},
	}}
	beadSlots, err := cfg2.PanelGateReviewerSlots("bead")
	if err != nil {
		t.Fatalf("bead slots: %v", err)
	}
	adhocSlots, err := cfg2.PanelGateReviewerSlots("adhoc")
	if err != nil {
		t.Fatalf("adhoc slots: %v", err)
	}
	if !reflect.DeepEqual(beadSlots, adhocSlots) {
		t.Errorf("unconfigured adhoc should equal bead's resolved slots: bead=%+v adhoc=%+v", beadSlots, adhocSlots)
	}
	if gotAdhoc, err := cfg2.PanelGateApproveThresholdExpr("adhoc"); err != nil || gotAdhoc != "5" {
		t.Errorf("unconfigured adhoc threshold should equal bead's resolved: got %q, err=%v, want \"5\"", gotAdhoc, err)
	}
}

// TestPanelGateSlots_DeterministicExpansion covers spec 112 AC5/R3:
// deterministic slot expansion, the cursor starting at index 0, and the
// normative 9-reviewer worked example.
func TestPanelGateSlots_DeterministicExpansion(t *testing.T) {
	structural := map[string]bool{"author-of-record": true, "codebase-pin": true, "contract-stability": true}
	sharp := map[string]bool{"empirical-prober": true, "adversarial": true, "integration": true}

	// Falsifiability fixture (per the plan): an explicitly-lensed entry
	// PRECEDES lens-less entries at a position where slot-index mod 6
	// diverges from the cursor value — R3 is slot-index 2 (0-based) but the
	// FIRST lens-less slot, so a correct cursor-only implementation gives
	// it author-of-record while a wrong lens[slot-index % 6] impl would
	// give lens[2] = codebase-pin instead.
	fixture := &Config{Panel: Panel{Gates: map[string]GatePanel{
		"bead": {Reviewers: []Reviewer{
			{Model: "claude-opus-4-8", Lens: "adversarial", Count: intp(2)},
			{Model: "claude-sonnet-5", Count: intp(3)},
		}},
	}}}
	slots, err := fixture.PanelGateReviewerSlots("bead")
	if err != nil {
		t.Fatalf("PanelGateReviewerSlots: %v", err)
	}
	if len(slots) != 5 {
		t.Fatalf("expected 5 slots, got %d: %+v", len(slots), slots)
	}
	for i, want := range []string{"R1", "R2", "R3", "R4", "R5"} {
		if slots[i].Slot != want {
			t.Errorf("slot[%d].Slot = %q, want %q", i, slots[i].Slot, want)
		}
	}
	if slots[0].Lens != "adversarial" || slots[1].Lens != "adversarial" {
		t.Errorf("explicit lens must be preserved and not cursor-consuming: got %q, %q", slots[0].Lens, slots[1].Lens)
	}
	if slots[2].Lens != "author-of-record" {
		t.Errorf("first lens-less slot (R3, slot-index 2) = %q, want author-of-record (cursor starts at 0 and skips explicitly-lensed slots)", slots[2].Lens)
	}
	if slots[3].Lens != "empirical-prober" {
		t.Errorf("R4 = %q, want empirical-prober", slots[3].Lens)
	}
	if slots[4].Lens != "codebase-pin" {
		t.Errorf("R5 = %q, want codebase-pin", slots[4].Lens)
	}

	// The 9-reviewer, all-lens-less, 3-entries-of-count-3 worked example
	// (spec 112 R3) — normative, not illustrative.
	worked := &Config{Panel: Panel{Gates: map[string]GatePanel{
		"spec_approve": {Reviewers: []Reviewer{
			{Model: "claude-fable-5", Count: intp(3)},
			{Model: "claude-opus-4-8", Count: intp(3)},
			{Model: "gpt-5.5", Count: intp(3)},
		}},
	}}}
	workedSlots, err := worked.PanelGateReviewerSlots("spec_approve")
	if err != nil {
		t.Fatalf("PanelGateReviewerSlots(worked): %v", err)
	}
	wantLenses := []string{
		"author-of-record", "empirical-prober", "codebase-pin",
		"adversarial", "contract-stability", "integration",
		"author-of-record", "empirical-prober", "codebase-pin",
	}
	if len(workedSlots) != 9 {
		t.Fatalf("worked example: expected 9 slots, got %d: %+v", len(workedSlots), workedSlots)
	}
	for i, s := range workedSlots {
		wantSlot := fmt.Sprintf("R%d", i+1)
		if s.Slot != wantSlot {
			t.Errorf("worked[%d].Slot = %q, want %q", i, s.Slot, wantSlot)
		}
		if s.Lens != wantLenses[i] {
			t.Errorf("worked[%d].Lens = %q, want %q", i, s.Lens, wantLenses[i])
		}
	}

	// Coverage: a gate with >= 6 lens-less reviewers exercises all six
	// default lenses.
	seen := map[string]bool{}
	for _, s := range workedSlots {
		seen[s.Lens] = true
	}
	for _, l := range defaultLenses {
		if !seen[l] {
			t.Errorf("worked example missing default lens %q: %+v", l, workedSlots)
		}
	}

	// Every lens-less entry of count >= 2 spans a structural AND a sharp lens.
	for _, b := range [][2]int{{0, 3}, {3, 6}, {6, 9}} {
		gotStructural, gotSharp := false, false
		for _, s := range workedSlots[b[0]:b[1]] {
			if structural[s.Lens] {
				gotStructural = true
			}
			if sharp[s.Lens] {
				gotSharp = true
			}
		}
		if !gotStructural || !gotSharp {
			t.Errorf("entry slots %+v must span a structural and a sharp lens", workedSlots[b[0]:b[1]])
		}
	}

	// Two loads (resolutions) of identical config yield identical slot lists.
	workedSlots2, err := worked.PanelGateReviewerSlots("spec_approve")
	if err != nil {
		t.Fatalf("PanelGateReviewerSlots(worked, second call): %v", err)
	}
	if !reflect.DeepEqual(workedSlots, workedSlots2) {
		t.Errorf("two resolutions of identical config diverged: %+v vs %+v", workedSlots, workedSlots2)
	}

	// Cursor-reset-on-explicit fixture (round-1 panel F1-1): lens-less,
	// explicit, lens-less. A correct impl leaves the cursor at 1 across the
	// explicit slot, so the second lens-less slot (R3) gets
	// defaultLenses[1] = empirical-prober; a wrong "reset the cursor to 0
	// whenever a slot is explicitly lensed" variant would instead give R3
	// = defaultLenses[0] = author-of-record. The middle entry also sets
	// both Family and Model (round-1 panel S2) to exercise the "Model
	// wins" branch of the unexported model() accessor.
	cursorReset := &Config{Panel: Panel{Gates: map[string]GatePanel{
		"final_review": {Reviewers: []Reviewer{
			{Model: "claude-fable-5", Count: intp(1)},
			{Family: "legacy-family-should-lose", Model: "claude-opus-4-8", Lens: "adversarial", Count: intp(1)},
			{Model: "claude-sonnet-5", Count: intp(1)},
		}},
	}}}
	crSlots, err := cursorReset.PanelGateReviewerSlots("final_review")
	if err != nil {
		t.Fatalf("PanelGateReviewerSlots(cursorReset): %v", err)
	}
	if len(crSlots) != 3 {
		t.Fatalf("cursorReset: expected 3 slots, got %d: %+v", len(crSlots), crSlots)
	}
	if crSlots[0].Lens != "author-of-record" {
		t.Errorf("cursorReset R1 = %q, want author-of-record", crSlots[0].Lens)
	}
	if crSlots[1].Lens != "adversarial" {
		t.Errorf("cursorReset R2 (explicit) = %q, want adversarial", crSlots[1].Lens)
	}
	if crSlots[1].Model != "claude-opus-4-8" {
		t.Errorf("cursorReset R2.Model = %q, want claude-opus-4-8 (Model must win over Family)", crSlots[1].Model)
	}
	if crSlots[2].Lens != "empirical-prober" {
		t.Errorf("cursorReset R3 = %q, want empirical-prober (cursor must NOT reset across an explicitly-lensed slot); a wrong cursor-reset-on-explicit implementation would give author-of-record instead", crSlots[2].Lens)
	}
}

// TestPanelGateAdvisoryDefault_SelectionRule covers the config half of spec
// 112 R7 (the step-4 helper, ahead of Bead 3's caller-side test): known-gate
// / bead-fallback / skip / gates-absent-global rows.
func TestPanelGateAdvisoryDefault_SelectionRule(t *testing.T) {
	// Gates absent: always the global default, regardless of recordedGate/isBead.
	globalCfg := DefaultConfig()
	if got, ok := globalCfg.PanelGateAdvisoryDefault("", false); !ok || got != 6 {
		t.Errorf("gates-absent, non-bead, no recorded gate: got (%d,%v), want (6,true)", got, ok)
	}
	if got, ok := globalCfg.PanelGateAdvisoryDefault("spec_approve", false); !ok || got != 6 {
		t.Errorf("gates-absent, recorded gate ignored: got (%d,%v), want (6,true)", got, ok)
	}
	if got, ok := globalCfg.PanelGateAdvisoryDefault("", true); !ok || got != 6 {
		t.Errorf("gates-absent, bead panel: got (%d,%v), want (6,true)", got, ok)
	}

	cfg := &Config{Panel: Panel{
		Reviewers: []Reviewer{{Family: "claude", Count: intp(3)}, {Family: "codex", Count: intp(3)}},
		Gates: map[string]GatePanel{
			"bead": {Reviewers: []Reviewer{{Model: "claude-opus-4-8", Count: intp(3)}, {Model: "claude-sonnet-5", Count: intp(3)}}},
			"final_review": {Reviewers: []Reviewer{
				{Model: "claude-fable-5", Count: intp(3)}, {Model: "claude-opus-4-8", Count: intp(3)},
				{Model: "gpt-5.5", Count: intp(3)}, {Model: "claude-sonnet-5", Count: intp(3)},
			}},
		},
	}}

	if got, ok := cfg.PanelGateAdvisoryDefault("final_review", false); !ok || got != 12 {
		t.Errorf("known gate final_review: got (%d,%v), want (12,true)", got, ok)
	}
	if got, ok := cfg.PanelGateAdvisoryDefault("", true); !ok || got != 6 {
		t.Errorf("bead fallback (empty recorded gate, isBead): got (%d,%v), want (6,true)", got, ok)
	}
	if got, ok := cfg.PanelGateAdvisoryDefault("", false); ok {
		t.Errorf("non-bead panel with no recorded gate should skip: got (%d,%v), want ok=false", got, ok)
	}
	if got, ok := cfg.PanelGateAdvisoryDefault("weird-value", true); ok {
		t.Errorf("unknown recorded gate should skip: got (%d,%v), want ok=false", got, ok)
	}
	if got, ok := cfg.PanelGateAdvisoryDefault("weird-value", false); ok {
		t.Errorf("unknown recorded gate (non-bead) should skip: got (%d,%v), want ok=false", got, ok)
	}
}
