package complete

import (
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"strings"
	"testing"
)

func TestComplete_ClosedBeadMovedTipRefuses(t *testing.T) {
	root := setupTempRoot(t)
	wireRealMergedUnclosedFixture(t, root, "132-closing")
	gitRun(t, root, "init", "-q", "-b", "main")
	gitRun(t, root, "config", "user.email", "test@example.invalid")
	gitRun(t, root, "config", "user.name", "Test")
	writeFile(t, root, "README.md", "base\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "base")
	gitRun(t, root, "checkout", "-qb", "spec/132-closing")
	gitRun(t, root, "checkout", "-qb", "bead/closing-1")
	writeFile(t, root, "README.md", "merged\n")
	gitRun(t, root, "commit", "-qam", "work")
	gitRun(t, root, "checkout", "-q", "spec/132-closing")
	gitRun(t, root, "merge", "--no-ff", "-qm", "Merge bead/closing-1", "bead/closing-1")
	gitRun(t, root, "checkout", "-q", "bead/closing-1")
	writeFile(t, root, "README.md", "later\n")
	gitRun(t, root, "commit", "-qam", "later fix")
	cfg, _ := config.Load(root)
	specWT, _ := workspace.SpecWorktreePath(root, cfg, "132-closing")
	gitRun(t, root, "worktree", "add", specWT, "spec/132-closing")
	called := false
	closeBeadFn = func(...string) error { called = true; return nil }
	_, err := Run(root, "closing-1", "132-closing", "", &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}, CompleteOpts{})
	if err == nil || !strings.Contains(err.Error(), "later fix") || called {
		t.Fatalf("moved tip must refuse before close and name the new commit: closed=%v err=%v", called, err)
	}
}
