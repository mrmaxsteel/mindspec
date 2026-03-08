package cleanup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func setupCleanupTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".mindspec"), 0755)
	return root
}

func mockCleanupFns(t *testing.T) {
	t.Helper()
	origFindLocalRoot := findLocalRootFn
	t.Cleanup(func() {
		findLocalRootFn = origFindLocalRoot
	})
	// Default: fall back to root arg (no local root resolution in tests).
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test: no local root") }
}

func TestCleanup_ForceRemovesWorktreeAndBranch(t *testing.T) {
	root := setupCleanupTest(t)
	mockCleanupFns(t)
	mock := &executor.MockExecutor{}

	result, err := Run(root, "010-test", true, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.WorktreeRemoved {
		t.Error("result.WorktreeRemoved should be true")
	}
	if !result.BranchDeleted {
		t.Error("result.BranchDeleted should be true")
	}

	// Verify executor.Cleanup was called with force=true
	calls := mock.CallsTo("Cleanup")
	if len(calls) != 1 {
		t.Fatalf("expected 1 Cleanup call, got %d", len(calls))
	}
	if calls[0].Args[0] != "010-test" {
		t.Errorf("Cleanup specID: got %q, want %q", calls[0].Args[0], "010-test")
	}
	if calls[0].Args[1] != true {
		t.Errorf("Cleanup force: got %v, want true", calls[0].Args[1])
	}
}

func TestCleanup_RefusesActiveSpec(t *testing.T) {
	root := setupCleanupTest(t)
	mockCleanupFns(t)
	mock := &executor.MockExecutor{}

	// Stub phase to return implement mode for 010-test
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				epics := []phase.EpicInfo{{
					ID: "epic-1", Title: "[SPEC 010-test] Test", Status: "open",
					IssueType: "epic", Metadata: map[string]interface{}{"spec_num": float64(10), "spec_title": "test"},
				}}
				return json.Marshal(epics)
			}
		}
		// queryChildren: one in_progress child
		children := []phase.ChildInfo{{ID: "bead-1", Status: "in_progress", IssueType: "task"}}
		return json.Marshal(children)
	})
	t.Cleanup(restoreList)
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restore)

	_, err := Run(root, "010-test", false, mock)
	if err == nil {
		t.Fatal("expected error for active spec")
	}
}

func TestCleanup_ForceBypassesActiveCheck(t *testing.T) {
	root := setupCleanupTest(t)
	mockCleanupFns(t)
	mock := &executor.MockExecutor{}

	result, err := Run(root, "010-test", true, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.WorktreeRemoved {
		t.Error("expected worktree to be removed")
	}
}

func TestCleanup_DifferentSpecAllowed(t *testing.T) {
	root := setupCleanupTest(t)
	mockCleanupFns(t)
	mock := &executor.MockExecutor{}

	// Phase returns a different active spec
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				epics := []phase.EpicInfo{{
					ID: "epic-other", Title: "[SPEC 999-other-spec] Other", Status: "open",
					IssueType: "epic", Metadata: map[string]interface{}{"spec_num": float64(999), "spec_title": "other-spec"},
				}}
				return json.Marshal(epics)
			}
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restore)

	_, err := Run(root, "010-test", false, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
