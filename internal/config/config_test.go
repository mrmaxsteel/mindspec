package config

import (
	"os"
	"path/filepath"
	"reflect"
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
	wantReviewers := []Reviewer{{Family: "claude", Count: 3}, {Family: "codex", Count: 3}}
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
		Reviewers:        []Reviewer{{Family: "claude", Count: 2}, {Family: "codex", Count: 4}},
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

	wantReviewers := []Reviewer{{Family: "claude", Count: 2}, {Family: "codex", Count: 2}}
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
