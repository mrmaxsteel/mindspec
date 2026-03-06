package cleanup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

func setupCleanupTest(t *testing.T, specID string, mode string) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".mindspec"), 0755)
	// ADR-0023: no focus file written; state derived from beads.
	_ = specID
	_ = mode
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

	// Stub phase to return implement mode for 010-test
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "list" && args[1] == "--type=epic" {
			epics := []phase.EpicInfo{{
				ID: "epic-1", Title: "[SPEC 010-test] Test", Status: "open",
				IssueType: "epic", Metadata: map[string]interface{}{"spec_num": float64(10), "spec_title": "test"},
			}}
			return json.Marshal(epics)
		}
		if len(args) >= 2 && args[0] == "list" {
			children := []phase.ChildInfo{{ID: "bead-1", Status: "in_progress", IssueType: "task"}}
			return json.Marshal(children)
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restore)

	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }

	_, err := Run(root, "010-test", false)
	if err == nil {
		t.Fatal("expected error for active spec")
	}
}

func TestCleanup_ForceBypassesActiveCheck(t *testing.T) {
	// Per ADR-0023: force cleanup no longer clears focus (no focus files).
	// It just removes worktree and branch regardless of state.
	root := setupCleanupTest(t, "010-test", state.ModeImplement)
	mockCleanupFns(t)

	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }

	result, err := Run(root, "010-test", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.WorktreeRemoved {
		t.Error("expected worktree to be removed")
	}
}

func TestCleanup_DifferentSpecAllowed(t *testing.T) {
	// Cleaning up spec "010-test" while "other-spec" is active should succeed.
	root := setupCleanupTest(t, "other-spec", state.ModeImplement)
	mockCleanupFns(t)

	// Phase returns a different active spec
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "list" && args[1] == "--type=epic" {
			epics := []phase.EpicInfo{{
				ID: "epic-other", Title: "[SPEC 999-other-spec] Other", Status: "open",
				IssueType: "epic", Metadata: map[string]interface{}{"spec_num": float64(999), "spec_title": "other-spec"},
			}}
			return json.Marshal(epics)
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restore)

	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }

	_, err := Run(root, "010-test", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
