package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
)

// --- Helpers ---

func newTestExecutor(t *testing.T) *GitExecutor {
	t.Helper()
	root := t.TempDir()

	g := &GitExecutor{
		Root: root,
		// Defaults: all no-op / success.
		CreateBranchFn:    func(name, from string) error { return nil },
		BranchExistsFn:    func(name string) bool { return false },
		DeleteBranchFn:    func(name string) error { return nil },
		MergeBranchFn:     func(workdir, source, target string) error { return nil },
		MergeIntoFn:       func(targetWorkdir, sourceBranch string) error { return nil },
		CommitAllFn:       func(workdir, message string) error { return nil },
		DiffStatFn:        func(workdir, base, head string) (string, error) { return "", nil },
		CommitCountFn:     func(workdir, base, head string) (int, error) { return 0, nil },
		IsAncestorFn:      func(workdir, ancestor, descendant string) (bool, error) { return true, nil },
		HasRemoteFn:       func() bool { return false },
		PushBranchFn:      func(branch string) error { return nil },
		EnsureGitignoreFn: func(root, entry string) error { return nil },
		WorktreeCreateFn:  func(name, branch string) error { return nil },
		WorktreeRemoveFn:  func(name string) error { return nil },
		WorktreeListFn:    func() ([]bead.WorktreeListEntry, error) { return nil, nil },
		LoadConfigFn:      func(root string) (*config.Config, error) { return config.DefaultConfig(), nil },
		ExecCommandFn:     exec.Command,
	}
	return g
}

// --- Interface compliance ---

func TestGitExecutorImplementsExecutor(t *testing.T) {
	var _ Executor = (*GitExecutor)(nil)
}

func TestMockExecutorImplementsExecutor(t *testing.T) {
	var _ Executor = (*MockExecutor)(nil)
}

// --- InitSpecWorkspace ---

func TestInitSpecWorkspace_CreatesBranchAndWorktree(t *testing.T) {
	g := newTestExecutor(t)

	var createdBranch, createdFrom string
	g.CreateBranchFn = func(name, from string) error {
		createdBranch = name
		createdFrom = from
		return nil
	}

	var wtRelPath, wtBranch string
	g.WorktreeCreateFn = func(name, branch string) error {
		wtRelPath = name
		wtBranch = branch
		return nil
	}

	info, err := g.InitSpecWorkspace("077-my-feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if createdBranch != "spec/077-my-feature" {
		t.Errorf("branch = %q, want %q", createdBranch, "spec/077-my-feature")
	}
	if createdFrom != "HEAD" {
		t.Errorf("from = %q, want %q", createdFrom, "HEAD")
	}
	if wtBranch != "spec/077-my-feature" {
		t.Errorf("worktree branch = %q, want %q", wtBranch, "spec/077-my-feature")
	}
	if wtRelPath != ".worktrees/worktree-spec-077-my-feature" {
		t.Errorf("worktree path = %q, want %q", wtRelPath, ".worktrees/worktree-spec-077-my-feature")
	}
	if info.Branch != "spec/077-my-feature" {
		t.Errorf("info.Branch = %q, want %q", info.Branch, "spec/077-my-feature")
	}
}

func TestInitSpecWorkspace_SkipsBranchIfExists(t *testing.T) {
	g := newTestExecutor(t)

	g.BranchExistsFn = func(name string) bool { return true }

	branchCreated := false
	g.CreateBranchFn = func(name, from string) error {
		branchCreated = true
		return nil
	}

	_, err := g.InitSpecWorkspace("077-my-feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branchCreated {
		t.Error("should not create branch when it already exists")
	}
}

func TestInitSpecWorkspace_ConfigError(t *testing.T) {
	g := newTestExecutor(t)
	g.LoadConfigFn = func(root string) (*config.Config, error) {
		return nil, fmt.Errorf("config broken")
	}

	_, err := g.InitSpecWorkspace("077-my-feature")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); !contains(got, "loading config") {
		t.Errorf("error = %q, want to contain %q", got, "loading config")
	}
}

// --- HandoffEpic ---

func TestHandoffEpic_IsNoOp(t *testing.T) {
	g := newTestExecutor(t)
	err := g.HandoffEpic("epic-1", "077-test", []string{"bead-a", "bead-b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- DispatchBead ---

func TestDispatchBead_CreatesBeadBranch(t *testing.T) {
	g := newTestExecutor(t)

	var createdBranch, createdFrom string
	g.CreateBranchFn = func(name, from string) error {
		createdBranch = name
		createdFrom = from
		return nil
	}

	info, err := g.DispatchBead("mindspec-abc.1", "077-my-feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if createdBranch != "bead/mindspec-abc.1" {
		t.Errorf("branch = %q, want %q", createdBranch, "bead/mindspec-abc.1")
	}
	if createdFrom != "spec/077-my-feature" {
		t.Errorf("from = %q, want %q", createdFrom, "spec/077-my-feature")
	}
	if info.Branch != "bead/mindspec-abc.1" {
		t.Errorf("info.Branch = %q, want %q", info.Branch, "bead/mindspec-abc.1")
	}
}

func TestDispatchBead_ReusesExistingWorktree(t *testing.T) {
	g := newTestExecutor(t)

	g.WorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{
			Name:   "worktree-mindspec-abc.1",
			Path:   "/repo/.worktrees/worktree-mindspec-abc.1",
			Branch: "bead/mindspec-abc.1",
		}}, nil
	}

	branchCreated := false
	g.CreateBranchFn = func(name, from string) error {
		branchCreated = true
		return nil
	}

	info, err := g.DispatchBead("mindspec-abc.1", "077-my-feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branchCreated {
		t.Error("should not create branch when worktree already exists")
	}
	if info.Path != "/repo/.worktrees/worktree-mindspec-abc.1" {
		t.Errorf("info.Path = %q, want existing worktree path", info.Path)
	}
}

func TestDispatchBead_FallsBackToHEAD(t *testing.T) {
	g := newTestExecutor(t)

	var createdFrom string
	g.CreateBranchFn = func(name, from string) error {
		createdFrom = from
		return nil
	}

	_, err := g.DispatchBead("mindspec-abc.1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createdFrom != "HEAD" {
		t.Errorf("from = %q, want %q", createdFrom, "HEAD")
	}
}

// --- CompleteBead ---

func TestCompleteBead_MergesAndCleans(t *testing.T) {
	g := newTestExecutor(t)

	// Create spec worktree directory so merge path is found.
	specWt := filepath.Join(g.Root, ".worktrees", "worktree-spec-077-test")
	os.MkdirAll(specWt, 0o755)

	g.WorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{
			Name:   "worktree-mindspec-x.1",
			Path:   filepath.Join(g.Root, ".worktrees/worktree-mindspec-x.1"),
			Branch: "bead/mindspec-x.1",
		}}, nil
	}

	// Stub IsTreeClean via ExecCommandFn — return empty porcelain.
	g.ExecCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "")
	}

	var merged bool
	g.MergeIntoFn = func(targetWorkdir, sourceBranch string) error {
		merged = true
		return nil
	}

	var removedWt string
	g.WorktreeRemoveFn = func(name string) error {
		removedWt = name
		return nil
	}

	var deletedBranch string
	g.DeleteBranchFn = func(name string) error {
		deletedBranch = name
		return nil
	}

	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !merged {
		t.Error("expected merge into spec branch")
	}
	if removedWt != "worktree-mindspec-x.1" {
		t.Errorf("removed worktree = %q, want %q", removedWt, "worktree-mindspec-x.1")
	}
	if deletedBranch != "bead/mindspec-x.1" {
		t.Errorf("deleted branch = %q, want %q", deletedBranch, "bead/mindspec-x.1")
	}
}

func TestCompleteBead_AutoCommits(t *testing.T) {
	g := newTestExecutor(t)

	g.ExecCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "")
	}

	var commitMsg string
	g.CommitAllFn = func(workdir, message string) error {
		commitMsg = message
		return nil
	}

	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "add feature X")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commitMsg != "impl(mindspec-x.1): add feature X" {
		t.Errorf("commit msg = %q, want %q", commitMsg, "impl(mindspec-x.1): add feature X")
	}
}

// --- FinalizeEpic ---

func TestFinalizeEpic_DirectMerge(t *testing.T) {
	g := newTestExecutor(t)

	g.BranchExistsFn = func(name string) bool { return true }
	g.CommitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }
	g.DiffStatFn = func(workdir, base, head string) (string, error) { return "3 files changed", nil }

	var mergedSource, mergedTarget string
	g.MergeBranchFn = func(workdir, source, target string) error {
		mergedSource = source
		mergedTarget = target
		return nil
	}

	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeStrategy != "direct" {
		t.Errorf("strategy = %q, want %q", result.MergeStrategy, "direct")
	}
	if result.CommitCount != 5 {
		t.Errorf("commits = %d, want 5", result.CommitCount)
	}
	if result.DiffStat != "3 files changed" {
		t.Errorf("diffstat = %q", result.DiffStat)
	}
	if mergedSource != "spec/077-test" || mergedTarget != "main" {
		t.Errorf("merge = %s → %s, want spec/077-test → main", mergedSource, mergedTarget)
	}
}

func TestFinalizeEpic_PushesWhenRemote(t *testing.T) {
	g := newTestExecutor(t)

	g.BranchExistsFn = func(name string) bool { return true }
	g.HasRemoteFn = func() bool { return true }

	var pushedBranch string
	g.PushBranchFn = func(branch string) error {
		pushedBranch = branch
		return nil
	}

	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeStrategy != "pr" {
		t.Errorf("strategy = %q, want %q", result.MergeStrategy, "pr")
	}
	if pushedBranch != "spec/077-test" {
		t.Errorf("pushed = %q, want %q", pushedBranch, "spec/077-test")
	}
}

func TestFinalizeEpic_BranchNotFound(t *testing.T) {
	g := newTestExecutor(t)
	g.BranchExistsFn = func(name string) bool { return false }

	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test")
	if err == nil {
		t.Fatal("expected error")
	}
	if !contains(err.Error(), "does not exist") {
		t.Errorf("error = %q, want 'does not exist'", err.Error())
	}
}

// --- Cleanup ---

func TestCleanup_RemovesWorktreeAndBranch(t *testing.T) {
	g := newTestExecutor(t)

	g.BranchExistsFn = func(name string) bool { return true }

	var removedWt, deletedBranch string
	g.WorktreeRemoveFn = func(name string) error {
		removedWt = name
		return nil
	}
	g.DeleteBranchFn = func(name string) error {
		deletedBranch = name
		return nil
	}

	err := g.Cleanup("077-test", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removedWt != "worktree-spec-077-test" {
		t.Errorf("removed worktree = %q", removedWt)
	}
	if deletedBranch != "spec/077-test" {
		t.Errorf("deleted branch = %q", deletedBranch)
	}
}

// --- IsTreeClean ---

func TestIsTreeClean_Clean(t *testing.T) {
	g := newTestExecutor(t)
	g.ExecCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "")
	}

	err := g.IsTreeClean("/some/path")
	if err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestIsTreeClean_Dirty(t *testing.T) {
	g := newTestExecutor(t)
	g.ExecCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", " M file.go")
	}

	err := g.IsTreeClean("/some/path")
	if err == nil {
		t.Fatal("expected error for dirty tree")
	}
	if !contains(err.Error(), "uncommitted changes") {
		t.Errorf("error = %q", err.Error())
	}
}

// --- DiffStat / CommitCount ---

func TestDiffStat_DelegatesToGitutil(t *testing.T) {
	g := newTestExecutor(t)

	g.DiffStatFn = func(workdir, base, head string) (string, error) {
		if workdir != g.Root {
			t.Errorf("workdir = %q, want %q", workdir, g.Root)
		}
		return "5 files changed", nil
	}

	stat, err := g.DiffStat("main", "spec/077")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stat != "5 files changed" {
		t.Errorf("stat = %q", stat)
	}
}

func TestCommitCount_DelegatesToGitutil(t *testing.T) {
	g := newTestExecutor(t)

	g.CommitCountFn = func(workdir, base, head string) (int, error) {
		if workdir != g.Root {
			t.Errorf("workdir = %q, want %q", workdir, g.Root)
		}
		return 42, nil
	}

	count, err := g.CommitCount("main", "spec/077")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 42 {
		t.Errorf("count = %d, want 42", count)
	}
}

// --- MockExecutor ---

func TestMockExecutor_RecordsCalls(t *testing.T) {
	m := &MockExecutor{
		InitSpecWorkspaceResult: WorkspaceInfo{Path: "/ws", Branch: "spec/001"},
		DispatchBeadResult:      WorkspaceInfo{Path: "/bead", Branch: "bead/x.1"},
		FinalizeEpicResult:      FinalizeResult{MergeStrategy: "direct", CommitCount: 3},
	}

	info, _ := m.InitSpecWorkspace("001-test")
	if info.Path != "/ws" {
		t.Errorf("path = %q", info.Path)
	}

	_ = m.HandoffEpic("e1", "001-test", []string{"b1", "b2"})
	bInfo, _ := m.DispatchBead("x.1", "001-test")
	if bInfo.Branch != "bead/x.1" {
		t.Errorf("branch = %q", bInfo.Branch)
	}

	_ = m.CompleteBead("x.1", "spec/001-test", "done")
	fr, _ := m.FinalizeEpic("e1", "001-test", "spec/001-test")
	if fr.CommitCount != 3 {
		t.Errorf("commits = %d", fr.CommitCount)
	}

	_ = m.Cleanup("001-test", false)
	_ = m.IsTreeClean("/path")
	_, _ = m.DiffStat("main", "spec/001")
	_, _ = m.CommitCount("main", "spec/001")
	_ = m.CommitAll("/path", "msg")

	// Verify call recording.
	if calls := m.CallsTo("InitSpecWorkspace"); len(calls) != 1 {
		t.Errorf("InitSpecWorkspace calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("HandoffEpic"); len(calls) != 1 {
		t.Errorf("HandoffEpic calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("DispatchBead"); len(calls) != 1 {
		t.Errorf("DispatchBead calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("CompleteBead"); len(calls) != 1 {
		t.Errorf("CompleteBead calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("FinalizeEpic"); len(calls) != 1 {
		t.Errorf("FinalizeEpic calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("Cleanup"); len(calls) != 1 {
		t.Errorf("Cleanup calls = %d, want 1", len(calls))
	}
	if total := len(m.Calls); total != 10 {
		t.Errorf("total calls = %d, want 10", total)
	}
}

// --- Helpers ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
