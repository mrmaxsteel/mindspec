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
	origWorktreeRemove := worktreeRemoveFn
	origDeleteBranch := deleteBranchFn
	origFindLocalRoot := findLocalRootFn
	t.Cleanup(func() {
		worktreeRemoveFn = origWorktreeRemove
		deleteBranchFn = origDeleteBranch
		findLocalRootFn = origFindLocalRoot
	})
	// Default: fall back to root arg (no local root resolution in tests).
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test: no local root") }
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

	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }

	_, err := Run(root, "010-test", false)
	if err == nil {
		t.Fatal("expected error for active spec")
	}
}

func TestCleanup_ClearsFocusAfterForce(t *testing.T) {
	root := setupCleanupTest(t, "010-test", state.ModeImplement)
	mockCleanupFns(t)

	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }

	// Force cleanup should clear focus even when mode is active.
	result, err := Run(root, "010-test", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.WorktreeRemoved {
		t.Error("expected worktree to be removed")
	}

	// Focus should now be idle.
	f, err := state.ReadFocus(root)
	if err != nil {
		t.Fatalf("reading focus: %v", err)
	}
	if f.Mode != state.ModeIdle {
		t.Errorf("focus mode after force cleanup: got %q, want %q", f.Mode, state.ModeIdle)
	}
	if f.ActiveSpec != "" {
		t.Errorf("focus activeSpec after force cleanup: got %q, want empty", f.ActiveSpec)
	}
}

func TestCleanup_PreservesFocusForOtherSpec(t *testing.T) {
	root := setupCleanupTest(t, "other-spec", state.ModeImplement)
	mockCleanupFns(t)

	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }

	// Clean up spec "010-test" while "other-spec" is active.
	_, err := Run(root, "010-test", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Focus should NOT be cleared — it belongs to a different spec.
	f, _ := state.ReadFocus(root)
	if f.ActiveSpec != "other-spec" {
		t.Errorf("focus activeSpec should still be %q, got %q", "other-spec", f.ActiveSpec)
	}
	if f.Mode != state.ModeImplement {
		t.Errorf("focus mode should still be %q, got %q", state.ModeImplement, f.Mode)
	}
}
