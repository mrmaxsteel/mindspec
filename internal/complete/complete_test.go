package complete

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/state"
)

// saveAndRestore saves all function variables and returns a restore function.
func saveAndRestore(t *testing.T) {
	t.Helper()
	origClose := closeBeadFn
	origWtList := worktreeListFn
	origWtRemove := worktreeRemoveFn
	origRunBD := runBDFn
	origExec := execCommandFn
	origMerge := mergeBranchFn
	origDelete := deleteBranchFn
	origResolveTarget := resolveTargetFn
	origResolveActiveBead := resolveActiveBeadFn
	origFindLocalRoot := findLocalRootFn

	t.Cleanup(func() {
		closeBeadFn = origClose
		worktreeListFn = origWtList
		worktreeRemoveFn = origWtRemove
		runBDFn = origRunBD
		execCommandFn = origExec
		mergeBranchFn = origMerge
		deleteBranchFn = origDelete
		resolveTargetFn = origResolveTarget
		resolveActiveBeadFn = origResolveActiveBead
		findLocalRootFn = origFindLocalRoot
	})

	// Default stubs
	mergeBranchFn = func(repoPath, src, dst string) error { return nil }
	deleteBranchFn = func(branch string) error { return nil }
	resolveTargetFn = func(root, flag string) (string, error) { return "", fmt.Errorf("no active specs") }
	resolveActiveBeadFn = func(root, specID string) (string, error) { return "", fmt.Errorf("no active bead") }
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test: no local root") }
}

// setupTempRoot creates a temp dir with .mindspec/.
func setupTempRoot(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
	return tmp
}

// writeLifecycleFixture creates a lifecycle.yaml for a spec in the temp root.
func writeLifecycleFixture(t *testing.T, root, specID, epicID string) {
	t.Helper()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", specID)
	os.MkdirAll(specDir, 0755)
	lc := &state.Lifecycle{Phase: state.ModeImplement, EpicID: epicID}
	if err := state.WriteLifecycle(specDir, lc); err != nil {
		t.Fatalf("writing lifecycle fixture: %v", err)
	}
}

func TestRun_HappyPath(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeLifecycleFixture(t, root, "008-test", "mol-parent-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	resolveActiveBeadFn = func(r, specID string) (string, error) { return "bead-1", nil }

	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}

	// Clean worktree
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "") // empty = clean
	}

	var closedID string
	closeBeadFn = func(ids ...string) error {
		closedID = ids[0]
		return nil
	}

	var removedName string
	worktreeRemoveFn = func(name string) error {
		removedName = name
		return nil
	}

	// Next ready bead exists
	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			items := []bead.BeadInfo{
				{ID: "bead-2", Title: "[IMPL 008-test.2] Next chunk"},
			}
			return json.Marshal(items)
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}

	result, err := Run(root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if closedID != "bead-1" {
		t.Errorf("closed ID: got %q, want %q", closedID, "bead-1")
	}
	if removedName != "worktree-bead-1" {
		t.Errorf("removed worktree: got %q, want %q", removedName, "worktree-bead-1")
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}
	if !result.WorktreeRemoved {
		t.Error("expected WorktreeRemoved=true")
	}
	if result.NextMode != state.ModeImplement {
		t.Errorf("NextMode: got %q, want %q", result.NextMode, state.ModeImplement)
	}
	if result.NextBead != "bead-2" {
		t.Errorf("NextBead: got %q, want %q", result.NextBead, "bead-2")
	}

	// Verify focus was written
	mc, err := state.ReadFocus(root)
	if err != nil {
		t.Fatalf("reading focus: %v", err)
	}
	if mc.Mode != state.ModeImplement {
		t.Errorf("focus mode: got %q, want %q", mc.Mode, state.ModeImplement)
	}
	if mc.ActiveBead != "bead-2" {
		t.Errorf("focus activeBead: got %q, want %q", mc.ActiveBead, "bead-2")
	}
}

func TestRun_DirtyTreeRefuses(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	resolveActiveBeadFn = func(r, specID string) (string, error) { return "bead-1", nil }

	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}

	// Dirty worktree
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "M modified-file.go")
	}

	_, err := Run(root, "")
	if err == nil {
		t.Fatal("expected error for dirty worktree")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("error should mention uncommitted changes: %v", err)
	}
}

func TestRun_DefaultsToActiveBead(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeLifecycleFixture(t, root, "008-test", "mol-parent-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	resolveActiveBeadFn = func(r, specID string) (string, error) { return "bead-from-resolver", nil }

	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return nil, nil // no worktrees
	}

	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "")
	}

	var closedID string
	closeBeadFn = func(ids ...string) error {
		closedID = ids[0]
		return nil
	}
	worktreeRemoveFn = func(name string) error { return nil }
	runBDFn = func(args ...string) ([]byte, error) { return nil, fmt.Errorf("no results") }

	_, err := Run(root, "") // no explicit bead ID
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closedID != "bead-from-resolver" {
		t.Errorf("closed ID: got %q, want %q", closedID, "bead-from-resolver")
	}
}

func TestRun_NoWorktree(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeLifecycleFixture(t, root, "008-test", "mol-parent-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }

	// No worktrees at all
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return nil, nil
	}

	// Clean main tree
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "")
	}

	closeBeadFn = func(ids ...string) error { return nil }
	worktreeRemoveFn = func(name string) error {
		t.Error("should not try to remove worktree when none exists")
		return nil
	}
	runBDFn = func(args ...string) ([]byte, error) { return nil, fmt.Errorf("no results") }

	result, err := Run(root, "bead-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WorktreeRemoved {
		t.Error("should not have removed worktree")
	}
	if !result.BeadClosed {
		t.Error("bead should be closed")
	}
}

func TestAdvanceState_NextReady(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeLifecycleFixture(t, root, "test-spec", "epic-123")

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			items := []bead.BeadInfo{
				{ID: "next-bead", Title: "[IMPL test-spec.2] Next"},
			}
			return json.Marshal(items)
		}
		return nil, fmt.Errorf("unexpected")
	}

	mode, nextBead := advanceState(root, "test-spec")
	if mode != state.ModeImplement {
		t.Errorf("mode: got %q, want %q", mode, state.ModeImplement)
	}
	if nextBead != "next-bead" {
		t.Errorf("nextBead: got %q, want %q", nextBead, "next-bead")
	}
}

func TestAdvanceState_BlockedChildren(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeLifecycleFixture(t, root, "test-spec", "epic-123")

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			return json.Marshal([]bead.BeadInfo{}) // nothing ready
		}
		if len(args) > 0 && args[0] == "search" {
			if len(args) > 1 && args[1] != "[test-spec]" {
				t.Errorf("search prefix: got %q, want %q", args[1], "[test-spec]")
			}
			items := []bead.BeadInfo{
				{ID: "blocked-bead", Title: "[test-spec] Bead 3: Blocked"},
			}
			return json.Marshal(items)
		}
		return nil, fmt.Errorf("unexpected")
	}

	mode, nextBead := advanceState(root, "test-spec")
	if mode != state.ModePlan {
		t.Errorf("mode: got %q, want %q", mode, state.ModePlan)
	}
	if nextBead != "" {
		t.Errorf("nextBead should be empty, got %q", nextBead)
	}
}

func TestAdvanceState_AllDone(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeLifecycleFixture(t, root, "test-spec", "epic-123")

	runBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]bead.BeadInfo{}) // nothing ready, nothing open
	}

	mode, nextBead := advanceState(root, "test-spec")
	if mode != state.ModeReview {
		t.Errorf("mode: got %q, want %q", mode, state.ModeReview)
	}
	if nextBead != "" {
		t.Errorf("nextBead should be empty, got %q", nextBead)
	}
}

func TestAdvanceState_NoLifecycle(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	// No lifecycle.yaml → idle

	mode, nextBead := advanceState(root, "test")
	if mode != state.ModeIdle {
		t.Errorf("mode: got %q, want %q", mode, state.ModeIdle)
	}
	if nextBead != "" {
		t.Errorf("nextBead should be empty, got %q", nextBead)
	}
}

func TestRun_NoBead(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	resolveActiveBeadFn = func(r, specID string) (string, error) { return "", nil }

	_, err := Run(root, "")
	if err == nil {
		t.Fatal("expected error when no bead ID available")
	}
	if !strings.Contains(err.Error(), "no bead ID") {
		t.Errorf("error should mention no bead ID: %v", err)
	}
}

func TestRun_AdvancesToImplementWhenNextBeadReady(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeLifecycleFixture(t, root, "008-test", "mol-parent-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	resolveActiveBeadFn = func(r, specID string) (string, error) { return "bead-1", nil }

	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "")
	}
	closeBeadFn = func(ids ...string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			items := []bead.BeadInfo{
				{ID: "bead-2", Title: "[IMPL 008-test.2] Next"},
			}
			return json.Marshal(items)
		}
		return nil, fmt.Errorf("unexpected")
	}

	result, err := Run(root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextMode != state.ModeImplement {
		t.Fatalf("expected implement mode, got %s", result.NextMode)
	}
	if result.NextBead != "bead-2" {
		t.Errorf("expected next bead bead-2, got %s", result.NextBead)
	}
}

func TestRun_AdvancesToReviewWhenNoMoreBeads(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	writeLifecycleFixture(t, root, "008-test", "mol-parent-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	resolveActiveBeadFn = func(r, specID string) (string, error) { return "bead-1", nil }

	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "")
	}
	closeBeadFn = func(ids ...string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }

	// No ready beads, no open beads → review
	runBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]bead.BeadInfo{})
	}

	result, err := Run(root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextMode != state.ModeReview {
		t.Fatalf("expected review mode, got %s", result.NextMode)
	}
}

func TestFormatResult_Implement(t *testing.T) {
	r := &Result{
		BeadID:          "bead-1",
		BeadClosed:      true,
		WorktreeRemoved: true,
		NextMode:        state.ModeImplement,
		NextBead:        "bead-2",
		NextSpec:        "008-test",
	}
	out := FormatResult(r)
	if !strings.Contains(out, "bead-1") {
		t.Errorf("should mention closed bead: %s", out)
	}
	if !strings.Contains(out, "bead-2") {
		t.Errorf("should mention next bead: %s", out)
	}
	if !strings.Contains(out, "Worktree removed") {
		t.Errorf("should mention worktree removal: %s", out)
	}
}

func TestFormatResult_Review(t *testing.T) {
	r := &Result{
		BeadID:     "bead-last",
		BeadClosed: true,
		NextMode:   state.ModeReview,
		NextSpec:   "test-spec",
	}
	out := FormatResult(r)
	if !strings.Contains(out, "review") {
		t.Errorf("should mention review: %s", out)
	}
	if !strings.Contains(out, "/ms:impl-approve") {
		t.Errorf("should mention /ms:impl-approve: %s", out)
	}
}
