package config

import (
	"os"
	"path/filepath"
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
