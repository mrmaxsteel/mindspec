package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// --- Helpers ---

func newTestExecutor(t *testing.T) *MindspecExecutor {
	t.Helper()
	root := t.TempDir()

	g := &MindspecExecutor{
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
		ExportJSONLFn:     func(workdir string) error { return nil },
		LoadConfigFn:      func(root string) (*config.Config, error) { return config.DefaultConfig(), nil },
		ExecCommandFn:     exec.Command,
	}
	return g
}

// --- Interface compliance ---

func TestMindspecExecutorImplementsExecutor(t *testing.T) {
	var _ Executor = (*MindspecExecutor)(nil)
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

func TestCompleteBead_WorktreeRemoveRunsFromRepoRoot(t *testing.T) {
	g := newTestExecutor(t)

	// Create a subdirectory to simulate being inside a bead worktree.
	beadWtPath := filepath.Join(g.Root, ".worktrees", "worktree-mindspec-x.1")
	os.MkdirAll(beadWtPath, 0o755)

	g.WorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{
			Name:   "worktree-mindspec-x.1",
			Path:   beadWtPath,
			Branch: "bead/mindspec-x.1",
		}}, nil
	}

	// Capture CWD at the moment WorktreeRemoveFn is called.
	var cwdDuringRemove string
	g.WorktreeRemoveFn = func(name string) error {
		wd, _ := os.Getwd()
		cwdDuringRemove = wd
		return nil
	}

	// Start from inside the bead worktree.
	origWd, _ := os.Getwd()
	os.Chdir(beadWtPath)
	defer os.Chdir(origWd)

	err := g.CompleteBead("mindspec-x.1", "spec/077-test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// CWD during worktree removal must be repo root, not inside the worktree.
	// Resolve symlinks to handle macOS /var -> /private/var.
	realCwd, _ := filepath.EvalSymlinks(cwdDuringRemove)
	realRoot, _ := filepath.EvalSymlinks(g.Root)
	if realCwd != realRoot {
		t.Errorf("CWD during WorktreeRemoveFn = %q, want repo root %q", cwdDuringRemove, g.Root)
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

// --- CommitAll / ExportJSONL ordering ---

func TestCommitAll_ExportsBeforeCommit(t *testing.T) {
	g := newTestExecutor(t)

	var calls []string
	g.ExportJSONLFn = func(workdir string) error {
		calls = append(calls, "export:"+workdir)
		return nil
	}
	g.CommitAllFn = func(workdir, message string) error {
		calls = append(calls, "commit:"+workdir+":"+message)
		return nil
	}

	if err := g.CommitAll("/wt", "msg"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("calls = %v, want 2 entries", calls)
	}
	if calls[0] != "export:/wt" {
		t.Errorf("calls[0] = %q, want %q", calls[0], "export:/wt")
	}
	if calls[1] != "commit:/wt:msg" {
		t.Errorf("calls[1] = %q, want %q", calls[1], "commit:/wt:msg")
	}
}

func TestCommitAll_ExportErrorAbortsCommit(t *testing.T) {
	g := newTestExecutor(t)

	g.ExportJSONLFn = func(workdir string) error {
		return fmt.Errorf("dolt unreachable")
	}
	committed := false
	g.CommitAllFn = func(workdir, message string) error {
		committed = true
		return nil
	}

	err := g.CommitAll("/wt", "msg")
	if err == nil {
		t.Fatal("expected error when export fails")
	}
	if !contains(err.Error(), "dolt unreachable") {
		t.Errorf("error = %q, want to contain export failure", err.Error())
	}
	if committed {
		t.Error("commit should not run when export fails")
	}
}

func TestCommitAll_ExportUsesCommitPath(t *testing.T) {
	g := newTestExecutor(t)

	var seenWorkdir string
	g.ExportJSONLFn = func(workdir string) error {
		seenWorkdir = workdir
		return nil
	}

	if err := g.CommitAll("/worktree-path", "msg"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenWorkdir != "/worktree-path" {
		t.Errorf("export workdir = %q, want %q", seenWorkdir, "/worktree-path")
	}
}

func TestCommitAll_ExportFallsBackToRootWhenPathEmpty(t *testing.T) {
	g := newTestExecutor(t)

	var seenWorkdir string
	g.ExportJSONLFn = func(workdir string) error {
		seenWorkdir = workdir
		return nil
	}

	if err := g.CommitAll("", "msg"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenWorkdir != g.Root {
		t.Errorf("export workdir = %q, want root %q", seenWorkdir, g.Root)
	}
}

func TestCompleteBead_ExportsBeforeCommit(t *testing.T) {
	g := newTestExecutor(t)

	var calls []string
	g.ExportJSONLFn = func(workdir string) error {
		calls = append(calls, "export")
		return nil
	}
	g.CommitAllFn = func(workdir, message string) error {
		calls = append(calls, "commit")
		return nil
	}
	g.WorktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{Name: "worktree-b.1", Path: "/wt/b.1", Branch: "bead/b.1"}}, nil
	}
	g.ExecCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("true") // IsTreeClean succeeds
	}

	if err := g.CompleteBead("b.1", "spec/077", "finish"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect export first, then commit.
	if len(calls) < 2 || calls[0] != "export" || calls[1] != "commit" {
		t.Errorf("calls = %v, want [export commit ...]", calls)
	}
}

// Integration-style: real git repo, real gitutil.CommitAll, fake bd export.
// Verifies that the pre-stage export lands in the committed tree (AC: "after
// approve/complete, `git show --stat HEAD` includes `.beads/issues.jsonl`"
// and "committed JSONL is byte-identical to a fresh `bd export` at commit time").
func TestCommitAll_ExportedJSONLLandsInCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, strings.TrimSpace(string(out)))
		}
	}
	runGit("init", "-b", "main")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "test")
	runGit("commit", "--allow-empty", "-m", "root")

	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	g := newTestExecutor(t)
	g.Root = dir
	// Real gitutil.CommitAll via imported package (tests use package-level).
	g.CommitAllFn = gitutil.CommitAll
	expectedJSONL := `{"id":"fake-1","title":"fresh from dolt"}` + "\n"
	exportCalls := 0
	g.ExportJSONLFn = func(workdir string) error {
		exportCalls++
		return os.WriteFile(filepath.Join(workdir, ".beads", "issues.jsonl"), []byte(expectedJSONL), 0o644)
	}

	if err := g.CommitAll(dir, "chore: commit with export"); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}
	if exportCalls != 1 {
		t.Errorf("export calls = %d, want 1", exportCalls)
	}

	// git show --stat HEAD must list .beads/issues.jsonl.
	statCmd := exec.Command("git", "-C", dir, "show", "--stat", "--format=", "HEAD")
	statOut, err := statCmd.Output()
	if err != nil {
		t.Fatalf("git show --stat: %v", err)
	}
	if !strings.Contains(string(statOut), ".beads/issues.jsonl") {
		t.Errorf("git show --stat did not include .beads/issues.jsonl:\n%s", string(statOut))
	}

	// git cat-file blob HEAD:.beads/issues.jsonl must equal the exported bytes.
	blobCmd := exec.Command("git", "-C", dir, "cat-file", "blob", "HEAD:.beads/issues.jsonl")
	blobOut, err := blobCmd.Output()
	if err != nil {
		t.Fatalf("git cat-file: %v", err)
	}
	if string(blobOut) != expectedJSONL {
		t.Errorf("committed blob = %q, want %q", string(blobOut), expectedJSONL)
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
