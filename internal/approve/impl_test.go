package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func writeSpecDir(t *testing.T, root, specID string) {
	t.Helper()
	specDir := filepath.Join(root, "docs", "specs", specID)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	// Write a minimal spec.md
	spec := "# Spec " + specID + "\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
}

func TestApproveImpl_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")

	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			payload := []map[string]string{{"status": "open"}}
			return json.Marshal(payload)
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}

	var closed []string
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "close" {
			closed = append(closed, args[1])
		}
		return []byte("ok"), nil
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}

	result, err := ApproveImpl(tmp, "010-test", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SpecID != "010-test" {
		t.Errorf("SpecID: got %q, want %q", result.SpecID, "010-test")
	}
	// Should close the epic
	if len(closed) != 1 || closed[0] != "epic-parent" {
		t.Errorf("expected to close epic-parent, got: %v", closed)
	}
	// Should call FinalizeEpic
	calls := mock.CallsTo("FinalizeEpic")
	if len(calls) != 1 {
		t.Errorf("expected 1 FinalizeEpic call, got %d", len(calls))
	}
}

func TestApproveImpl_WrongMode(t *testing.T) {
	tmp := t.TempDir()

	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	// Stub phase to return implement mode (not review)
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				epics := []phase.EpicInfo{{
					ID: "epic-parent", Title: "[SPEC 010-test] Test", Status: "open",
					IssueType: "epic", Metadata: map[string]interface{}{"spec_num": float64(10), "spec_title": "test"},
				}}
				return json.Marshal(epics)
			}
		}
		if contains(args, "--parent") {
			// One child in_progress → implement mode
			children := []phase.ChildInfo{{ID: "bead-1", Status: "in_progress", IssueType: "task"}}
			return json.Marshal(children)
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)

	mock := &executor.MockExecutor{}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected error for wrong mode")
	}
	if !strings.Contains(err.Error(), "expected review mode") {
		t.Errorf("error should mention expected review mode: %v", err)
	}
}

func TestApproveImpl_WrongSpec(t *testing.T) {
	tmp := t.TempDir()

	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	// Phase stub returns review mode for spec 010-test (not 011-other)
	stubPhaseForReview(t)

	mock := &executor.MockExecutor{}

	_, err := ApproveImpl(tmp, "011-other", mock)
	if err == nil {
		t.Fatal("expected error for wrong spec")
	}
	if !strings.Contains(err.Error(), "no epic found") {
		t.Errorf("error should mention no epic found: %v", err)
	}
}

func TestApproveImpl_EpicCloseFailureWarns(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}

	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "close" {
			return nil, fmt.Errorf("boom")
		}
		return []byte("ok"), nil
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}

	result, err := ApproveImpl(tmp, "010-test", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for failed epic close")
	}
	foundEpicWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "epic-parent") {
			foundEpicWarning = true
		}
	}
	if !foundEpicWarning {
		t.Errorf("expected warning to mention epic: %v", result.Warnings)
	}
}

func TestApproveImpl_PushAndCleanup(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	mock := &executor.MockExecutor{
		CommitCountResult: 5,
		FinalizeEpicResult: executor.FinalizeResult{
			MergeStrategy: "pr",
			CommitCount:   5,
			DiffStat:      " 3 files changed, 50 insertions(+), 10 deletions(-)",
		},
	}

	result, err := ApproveImpl(tmp, "010-test", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Pushed {
		t.Error("expected Pushed to be true for PR strategy")
	}
	if result.SpecBranch != "spec/010-test" {
		t.Errorf("SpecBranch: got %q, want %q", result.SpecBranch, "spec/010-test")
	}
	if result.CommitCount != 5 {
		t.Errorf("CommitCount: got %d, want 5", result.CommitCount)
	}
	if !strings.Contains(result.DiffStat, "3 files changed") {
		t.Errorf("DiffStat should contain file stats, got: %q", result.DiffStat)
	}
}

func TestApproveImpl_NoRemoteSkipsPush(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	mock := &executor.MockExecutor{
		CommitCountResult: 3,
		FinalizeEpicResult: executor.FinalizeResult{
			MergeStrategy: "direct",
			CommitCount:   3,
		},
	}

	result, err := ApproveImpl(tmp, "010-test", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pushed {
		t.Error("expected Pushed to be false when no remote")
	}
}

func TestApproveImpl_FinalizeEpicCalled(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	mock := &executor.MockExecutor{
		CommitCountResult: 1,
		FinalizeEpicResult: executor.FinalizeResult{
			MergeStrategy: "direct",
			CommitCount:   1,
		},
	}

	if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify FinalizeEpic was called with correct args
	calls := mock.CallsTo("FinalizeEpic")
	if len(calls) != 1 {
		t.Fatalf("expected 1 FinalizeEpic call, got %d", len(calls))
	}
	if calls[0].Args[1] != "010-test" {
		t.Errorf("FinalizeEpic specID: got %v, want 010-test", calls[0].Args[1])
	}
	if calls[0].Args[2] != "spec/010-test" {
		t.Errorf("FinalizeEpic specBranch: got %v, want spec/010-test", calls[0].Args[2])
	}
}

// writePlanWithBeads creates a plan.md with bead_ids in frontmatter.
func writePlanWithBeads(t *testing.T, root, specID string, beadIDs []string) {
	t.Helper()
	specDir := filepath.Join(root, "docs", "specs", specID)
	os.MkdirAll(specDir, 0755)
	var ids string
	for _, id := range beadIDs {
		ids += fmt.Sprintf("  - %s\n", id)
	}
	content := fmt.Sprintf("---\nstatus: Approved\nspec_id: %q\nbead_ids:\n%s---\n\n# Plan\n", specID, ids)
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
}

// stubPhaseForReview sets up the phase package to return review mode for "010-test".
func stubPhaseForReview(t *testing.T) {
	t.Helper()
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				epics := []phase.EpicInfo{{
					ID:        "epic-parent",
					Title:     "[SPEC 010-test] Test",
					Status:    "open",
					IssueType: "epic",
					Metadata:  map[string]interface{}{"spec_num": float64(10), "spec_title": "test"},
				}}
				return json.Marshal(epics)
			}
		}
		// queryChildren: --parent flag → all children closed (review)
		if contains(args, "--parent") {
			children := []phase.ChildInfo{{ID: "bead-1", Status: "closed", IssueType: "task"}}
			return json.Marshal(children)
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)
}

func contains(args []string, s string) bool {
	for _, a := range args {
		if a == s || strings.HasPrefix(a, s+"=") || strings.Contains(a, s) {
			return true
		}
	}
	return false
}

// saveAndRestore saves the current values of impl function variables and
// restores them via t.Cleanup.
func saveAndRestore(t *testing.T) {
	t.Helper()
	origRunBD := implRunBDFn
	origRunBDCombined := implRunBDCombinedFn
	t.Cleanup(func() {
		implRunBDFn = origRunBD
		implRunBDCombinedFn = origRunBDCombined
	})

	// Stub phase package for review mode by default
	stubPhaseForReview(t)

	// Deterministic defaults for tests that don't care about specifics.
	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
}

func TestApproveImpl_NoCommitsNoBeads(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "closed"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	mock := &executor.MockExecutor{
		CommitCountResult: 0,
	}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected error when spec branch has no commits beyond main")
	}
	if !strings.Contains(err.Error(), "no commits beyond main") {
		t.Errorf("error should mention no commits: %v", err)
	}
}

func TestApproveImpl_NoCommitsButClosedBeads_AllowsCleanup(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			payload := []map[string]string{{"status": "closed"}}
			return json.Marshal(payload)
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	mock := &executor.MockExecutor{
		CommitCountResult:  0,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct"},
	}

	if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
		t.Fatalf("expected approval to continue as cleanup path, got: %v", err)
	}
}

func TestApproveImpl_OpenBeads(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa", "bead-bbb"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			status := "closed"
			if args[1] == "bead-bbb" {
				status = "in_progress"
			}
			payload := []map[string]string{{"status": status}}
			return json.Marshal(payload)
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	mock := &executor.MockExecutor{CommitCountResult: 5}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected error when beads are still open")
	}
	if !strings.Contains(err.Error(), "bead-bbb") || !strings.Contains(err.Error(), "in_progress") {
		t.Errorf("error should mention open bead: %v", err)
	}
}

func TestApproveImpl_AllGood(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa", "bead-bbb"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "closed"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	mock := &executor.MockExecutor{
		CommitCountResult: 5,
		FinalizeEpicResult: executor.FinalizeResult{
			MergeStrategy: "direct",
			CommitCount:   5,
			DiffStat:      "2 files changed",
		},
	}

	result, err := ApproveImpl(tmp, "010-test", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SpecBranch != "spec/010-test" {
		t.Errorf("SpecBranch: got %q, want %q", result.SpecBranch, "spec/010-test")
	}
}

func TestApproveImpl_MockExecutorNoBD(t *testing.T) {
	// Verify that a mock executor can drive ApproveImpl without any git operations.
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	mock := &executor.MockExecutor{
		CommitCountResult:  3,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 3},
	}

	result, err := ApproveImpl(tmp, "010-test", mock)
	if err != nil {
		t.Fatalf("mock executor should work without git: %v", err)
	}
	if result.CommitCount != 3 {
		t.Errorf("CommitCount: got %d, want 3", result.CommitCount)
	}
}
