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
	"github.com/mindspec/mindspec/internal/next"
	"github.com/mindspec/mindspec/internal/phase"
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
	origMerge := mergeIntoFn
	origDelete := deleteBranchFn
	origCommitAll := commitAllFn
	origResolveTarget := resolveTargetFn
	origResolveActiveBead := resolveActiveBeadFn
	origFindLocalRoot := findLocalRootFn
	origFetchBeadByID := fetchBeadByIDFn
	origFindRecentClosed := findRecentClosedFn

	t.Cleanup(func() {
		closeBeadFn = origClose
		worktreeListFn = origWtList
		worktreeRemoveFn = origWtRemove
		runBDFn = origRunBD
		execCommandFn = origExec
		mergeIntoFn = origMerge
		deleteBranchFn = origDelete
		commitAllFn = origCommitAll
		resolveTargetFn = origResolveTarget
		resolveActiveBeadFn = origResolveActiveBead
		findLocalRootFn = origFindLocalRoot
		fetchBeadByIDFn = origFetchBeadByID
		findRecentClosedFn = origFindRecentClosed
	})

	// Default stubs
	mergeIntoFn = func(targetWorkdir, sourceBranch string) error { return nil }
	deleteBranchFn = func(branch string) error { return nil }
	commitAllFn = func(workdir, msg string) error { return nil }
	resolveTargetFn = func(root, flag string) (string, error) { return "", fmt.Errorf("no active specs") }
	resolveActiveBeadFn = func(root, specID string) (string, error) { return "", fmt.Errorf("no active bead") }
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test: no local root") }
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) { return next.BeadInfo{}, fmt.Errorf("not found") }
	findRecentClosedFn = func(specID string) (string, error) { return "", nil }
}

// setupTempRoot creates a temp dir with .mindspec/.
func setupTempRoot(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)
	return tmp
}

// stubPhaseEpic stubs phase.FindEpicBySpecID to return the given epicID for specID.
func stubPhaseEpic(t *testing.T, specID, epicID string) {
	t.Helper()
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "list" && args[1] == "--type=epic" {
			epics := []phase.EpicInfo{{
				ID: epicID, Title: "[SPEC " + specID + "] Test", Status: "open",
				IssueType: "epic", Metadata: map[string]interface{}{},
			}}
			// Parse spec_num and spec_title from specID for metadata.
			var num int
			var title string
			if idx := strings.Index(specID, "-"); idx > 0 {
				fmt.Sscanf(specID[:idx], "%d", &num)
				title = specID[idx+1:]
			}
			if num > 0 && title != "" {
				epics[0].Metadata["spec_num"] = float64(num)
				epics[0].Metadata["spec_title"] = title
			}
			return json.Marshal(epics)
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restore)
}

func TestRun_HappyPath(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	resolveActiveBeadFn = func(r, specID string) (string, error) { return "bead-1", nil }

	// Create spec worktree dir so merge path is found
	specWtDir := filepath.Join(root, ".worktrees", "worktree-spec-008-test")
	os.MkdirAll(specWtDir, 0755)

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

	// Verify merge targets spec worktree, not bead worktree
	var mergedWorkdir, mergedBranch string
	mergeIntoFn = func(targetWorkdir, sourceBranch string) error {
		mergedWorkdir = targetWorkdir
		mergedBranch = sourceBranch
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

	result, err := Run(root, "", "", "")
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

	// Verify merge targeted the spec worktree (not the bead worktree)
	if mergedWorkdir != specWtDir {
		t.Errorf("merge workdir: got %q, want %q", mergedWorkdir, specWtDir)
	}
	if mergedBranch != "bead/bead-1" {
		t.Errorf("merge branch: got %q, want %q", mergedBranch, "bead/bead-1")
	}

	// ADR-0023: no focus file written — state derived from beads.
	focusPath := filepath.Join(root, ".mindspec", "focus")
	if _, statErr := os.Stat(focusPath); statErr == nil {
		t.Error("expected no focus file to be written")
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

	_, err := Run(root, "", "", "")
	if err == nil {
		t.Fatal("expected error for dirty worktree")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("error should mention uncommitted changes: %v", err)
	}
}

func TestRun_DirtyTreeWithoutWorktreeSuggestsNext(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	resolveActiveBeadFn = func(r, specID string) (string, error) { return "bead-1", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", " M hello.go")
	}

	// ADR-0023: no focus file — hint is now always shown when no worktree found.
	_, err := Run(root, "", "", "")
	if err == nil {
		t.Fatal("expected error for dirty tree")
	}
	if !strings.Contains(err.Error(), "mindspec next") {
		t.Fatalf("expected recovery hint to mention `mindspec next`, got: %v", err)
	}
}

func TestCheckCleanWorktree_IgnoresGeneratedStateFiles(t *testing.T) {
	saveAndRestore(t)

	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "printf ' M .mindspec/focus\n?? .mindspec/session.json\n'")
	}

	if err := checkCleanWorktree("/tmp/ignored"); err != nil {
		t.Fatalf("expected state-only changes to be ignored, got: %v", err)
	}
}

func TestCheckCleanWorktree_ReportsUserChanges(t *testing.T) {
	saveAndRestore(t)

	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "printf ' M .mindspec/focus\n M hello.go\n'")
	}

	err := checkCleanWorktree("/tmp/mixed")
	if err == nil {
		t.Fatal("expected user change to block completion")
	}
	if !strings.Contains(err.Error(), "hello.go") {
		t.Fatalf("expected error to mention blocking file, got: %v", err)
	}
	if strings.Contains(err.Error(), ".mindspec/focus") {
		t.Fatalf("expected ignorable state file to be filtered, got: %v", err)
	}
}

func TestCheckCleanWorktree_ManualWorktreeHint(t *testing.T) {
	saveAndRestore(t)

	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "printf ' M .gitignore\n?? .worktrees/worktree-bead-1/tmp.txt\n'")
	}

	err := checkCleanWorktree("/tmp/worktree")
	if err == nil {
		t.Fatal("expected dirty tree error")
	}
	if !strings.Contains(err.Error(), "mindspec next") {
		t.Fatalf("expected manual worktree recovery hint, got: %v", err)
	}
}

func TestCheckCleanWorktree_LinkedWorktreeDirHint(t *testing.T) {
	saveAndRestore(t)

	root := t.TempDir()
	wt := filepath.Join(root, "impl-002-multi")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatalf("mkdir worktree dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /tmp/fake\n"), 0644); err != nil {
		t.Fatalf("write .git marker: %v", err)
	}

	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "printf '?? impl-002-multi\n'")
	}

	err := checkCleanWorktree(root)
	if err == nil {
		t.Fatal("expected dirty tree error")
	}
	if !strings.Contains(err.Error(), "mindspec next") {
		t.Fatalf("expected linked worktree hint, got: %v", err)
	}
}

func TestRun_DefaultsToActiveBead(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")

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

	_, err := Run(root, "", "", "") // no explicit bead ID
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
	stubPhaseEpic(t, "008-test", "mol-parent-1")

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

	result, err := Run(root, "bead-1", "", "")
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

	setupTempRoot(t)
	stubPhaseEpic(t, "001-test-spec", "epic-123")

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			items := []bead.BeadInfo{
				{ID: "next-bead", Title: "[IMPL 001-test-spec.2] Next"},
			}
			return json.Marshal(items)
		}
		return nil, fmt.Errorf("unexpected")
	}

	mode, nextBead := advanceState("001-test-spec")
	if mode != state.ModeImplement {
		t.Errorf("mode: got %q, want %q", mode, state.ModeImplement)
	}
	if nextBead != "next-bead" {
		t.Errorf("nextBead: got %q, want %q", nextBead, "next-bead")
	}
}

func TestAdvanceState_BlockedChildren(t *testing.T) {
	saveAndRestore(t)

	setupTempRoot(t)
	stubPhaseEpic(t, "001-test-spec", "epic-123")

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			return json.Marshal([]bead.BeadInfo{}) // nothing ready
		}
		if len(args) > 0 && args[0] == "search" {
			if len(args) > 1 && args[1] != "[001-test-spec]" {
				t.Errorf("search prefix: got %q, want %q", args[1], "[001-test-spec]")
			}
			items := []bead.BeadInfo{
				{ID: "blocked-bead", Title: "[001-test-spec] Bead 3: Blocked"},
			}
			return json.Marshal(items)
		}
		return nil, fmt.Errorf("unexpected")
	}

	mode, nextBead := advanceState("001-test-spec")
	if mode != state.ModePlan {
		t.Errorf("mode: got %q, want %q", mode, state.ModePlan)
	}
	if nextBead != "" {
		t.Errorf("nextBead should be empty, got %q", nextBead)
	}
}

func TestAdvanceState_AllDone(t *testing.T) {
	saveAndRestore(t)

	setupTempRoot(t)
	stubPhaseEpic(t, "001-test-spec", "epic-123")

	runBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]bead.BeadInfo{}) // nothing ready, nothing open
	}

	mode, nextBead := advanceState("001-test-spec")
	if mode != state.ModeReview {
		t.Errorf("mode: got %q, want %q", mode, state.ModeReview)
	}
	if nextBead != "" {
		t.Errorf("nextBead should be empty, got %q", nextBead)
	}
}

func TestAdvanceState_NoEpic(t *testing.T) {
	saveAndRestore(t)

	setupTempRoot(t)
	// No epic found for spec → idle (ADR-0023: no lifecycle.yaml needed).
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil // no epics
	})
	t.Cleanup(restore)

	mode, nextBead := advanceState("test")
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

	_, err := Run(root, "", "", "")
	if err == nil {
		t.Fatal("expected error when no bead ID available")
	}
	if !strings.Contains(err.Error(), "no bead ID") {
		t.Errorf("error should mention no bead ID: %v", err)
	}
}

func TestRun_PositionalSpecIDTreatedAsSpecHint(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)

	var gotSpecHint string
	resolveTargetFn = func(r, flag string) (string, error) {
		gotSpecHint = flag
		return "008-test", nil
	}
	resolveActiveBeadFn = func(r, specID string) (string, error) { return "repo-abc.1", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	execCommandFn = func(name string, args ...string) *exec.Cmd { return exec.Command("echo", "") }

	var closedID string
	closeBeadFn = func(ids ...string) error {
		closedID = ids[0]
		return nil
	}

	// No lifecycle fixture -> advanceState falls back to idle.
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	if _, err := Run(root, "008-test", "", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSpecHint != "008-test" {
		t.Fatalf("resolveTarget spec hint: got %q, want %q", gotSpecHint, "008-test")
	}
	if closedID != "repo-abc.1" {
		t.Fatalf("closed bead ID: got %q, want %q", closedID, "repo-abc.1")
	}
}

func TestRun_AdvancesToImplementWhenNextBeadReady(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")

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

	result, err := Run(root, "", "", "")
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
	stubPhaseEpic(t, "008-test", "mol-parent-1")

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

	result, err := Run(root, "", "", "")
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
	if !strings.Contains(out, "/ms-impl-approve") {
		t.Errorf("should mention /ms-impl-approve: %s", out)
	}
}

func TestRun_AlreadyClosed(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	resolveActiveBeadFn = func(r, specID string) (string, error) { return "", nil }

	// findRecentClosedFn returns the closed bead (simulating bd close already ran)
	findRecentClosedFn = func(specID string) (string, error) { return "bead-1", nil }

	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}

	// Clean worktree
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "")
	}

	// closeBeadFn fails because bead is already closed
	closeBeadFn = func(ids ...string) error {
		return fmt.Errorf("bd close failed: issue already closed")
	}

	// fetchBeadByIDFn confirms bead is closed
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		return next.BeadInfo{ID: "bead-1", Status: "closed"}, nil
	}

	// Create spec worktree dir so merge path is found
	specWtDir := filepath.Join(root, ".worktrees", "worktree-spec-008-test")
	os.MkdirAll(specWtDir, 0755)

	var mergedBranch string
	mergeIntoFn = func(targetWorkdir, sourceBranch string) error {
		mergedBranch = sourceBranch
		return nil
	}

	var removedName string
	worktreeRemoveFn = func(name string) error {
		removedName = name
		return nil
	}

	runBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]bead.BeadInfo{})
	}

	result, err := Run(root, "", "", "")
	if err != nil {
		t.Fatalf("expected success for already-closed bead, got: %v", err)
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}
	if mergedBranch != "bead/bead-1" {
		t.Errorf("expected merge of bead/bead-1, got %q", mergedBranch)
	}
	if removedName != "worktree-bead-1" {
		t.Errorf("expected worktree removal, got %q", removedName)
	}
}

func TestRun_CloseFailsNonIdempotent(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	resolveActiveBeadFn = func(r, specID string) (string, error) { return "bead-1", nil }

	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "")
	}

	// closeBeadFn fails with a non-"already closed" error
	closeBeadFn = func(ids ...string) error {
		return fmt.Errorf("bd close failed: network error")
	}

	// fetchBeadByIDFn says bead is still open — not an idempotent case
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		return next.BeadInfo{ID: "bead-1", Status: "open"}, nil
	}

	_, err := Run(root, "bead-1", "", "")
	if err == nil {
		t.Fatal("expected error when close fails and bead is not closed")
	}
	if !strings.Contains(err.Error(), "closing bead") {
		t.Errorf("expected 'closing bead' error, got: %v", err)
	}
}
