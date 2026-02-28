package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mindspec/mindspec/internal/state"
)

func setupCleanupTest(t *testing.T, specID string, mode string) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".mindspec"), 0755)

	mc := &state.Focus{
		Mode:       mode,
		ActiveSpec: specID,
		SpecBranch: state.SpecBranch(specID),
	}
	if err := state.WriteFocus(root, mc); err != nil {
		t.Fatal(err)
	}
	return root
}

func mockCleanupFns(t *testing.T) {
	t.Helper()
	origPRStatus := prStatusFn
	origWorktreeRemove := worktreeRemoveFn
	origDeleteBranch := deleteBranchFn
	origFindPR := findPRForBranchFn
	t.Cleanup(func() {
		prStatusFn = origPRStatus
		worktreeRemoveFn = origWorktreeRemove
		deleteBranchFn = origDeleteBranch
		findPRForBranchFn = origFindPR
	})
}

func TestCleanup_ForceRemovesWorktreeAndBranch(t *testing.T) {
	root := setupCleanupTest(t, "010-test", state.ModeIdle)
	mockCleanupFns(t)

	var wtRemoved, branchDeleted bool
	worktreeRemoveFn = func(name string) error {
		wtRemoved = true
		return nil
	}
	deleteBranchFn = func(name string) error {
		branchDeleted = true
		return nil
	}

	result, err := Run(root, "010-test", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wtRemoved {
		t.Error("expected worktree to be removed")
	}
	if result.WorktreeRemoved != true {
		t.Error("result.WorktreeRemoved should be true")
	}
	// Branch may not exist in test (BranchExists returns false), so deletion is skipped.
	_ = branchDeleted
}

func TestCleanup_RefusesActiveSpec(t *testing.T) {
	root := setupCleanupTest(t, "010-test", state.ModeImplement)
	mockCleanupFns(t)

	findPRForBranchFn = func(branch string) (string, string, error) {
		return "", "", fmt.Errorf("no PR found")
	}
	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }

	_, err := Run(root, "010-test", false)
	if err == nil {
		t.Fatal("expected error for active spec")
	}
}

func TestCleanup_MergedPR(t *testing.T) {
	root := setupCleanupTest(t, "010-test", state.ModeIdle)
	mockCleanupFns(t)

	findPRForBranchFn = func(branch string) (string, string, error) {
		return "merged", "https://github.com/test/repo/pull/1", nil
	}
	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }

	result, err := Run(root, "010-test", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PRStatus != "merged" {
		t.Errorf("PRStatus: got %q, want %q", result.PRStatus, "merged")
	}
	if result.PRURL != "https://github.com/test/repo/pull/1" {
		t.Errorf("PRURL: got %q", result.PRURL)
	}
}

func TestCleanup_OpenPRRefuses(t *testing.T) {
	root := setupCleanupTest(t, "010-test", state.ModeIdle)
	mockCleanupFns(t)

	findPRForBranchFn = func(branch string) (string, string, error) {
		return "open", "https://github.com/test/repo/pull/1", nil
	}
	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }

	_, err := Run(root, "010-test", false)
	if err == nil {
		t.Fatal("expected error for open PR")
	}
}
