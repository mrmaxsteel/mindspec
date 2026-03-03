package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/config"
	"github.com/mindspec/mindspec/internal/state"
)

func writeLifecycleSpec(t *testing.T, root, specID string) {
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
	// Write lifecycle.yaml with epic_id
	lc := &state.Lifecycle{Phase: state.ModeReview, EpicID: "epic-parent"}
	if err := state.WriteLifecycle(specDir, lc); err != nil {
		t.Fatalf("write lifecycle: %v", err)
	}
}

func TestApproveImpl_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")

	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	// Set state to review mode
	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
	})

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
		if len(args) == 2 && args[0] == "close" {
			closed = append(closed, args[1])
			return []byte("ok"), nil
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}
	commitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }
	branchExistsFn = func(name string) bool { return false }

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SpecID != "010-test" {
		t.Errorf("SpecID: got %q, want %q", result.SpecID, "010-test")
	}
	// Should close only the epic
	if len(closed) != 1 || closed[0] != "epic-parent" {
		t.Errorf("expected to close epic-parent, got: %v", closed)
	}

	// Verify focus is now idle
	mc, mcErr := state.ReadFocus(tmp)
	if mcErr != nil {
		t.Fatalf("reading focus: %v", mcErr)
	}
	if mc.Mode != state.ModeIdle {
		t.Errorf("mode: got %q, want %q", mc.Mode, state.ModeIdle)
	}

	// Verify lifecycle.yaml is now "done"
	specDir := filepath.Join(tmp, "docs", "specs", "010-test")
	lc, lcErr := state.ReadLifecycle(specDir)
	if lcErr != nil {
		t.Fatalf("reading lifecycle: %v", lcErr)
	}
	if lc.Phase != "done" {
		t.Errorf("lifecycle phase: got %q, want %q", lc.Phase, "done")
	}
}

func TestApproveImpl_WrongMode(t *testing.T) {
	tmp := t.TempDir()

	writeLifecycleSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	origFindLocalRoot := findLocalRootFn
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test") }
	t.Cleanup(func() { findLocalRootFn = origFindLocalRoot })

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeImplement,
		ActiveSpec: "010-test",
		ActiveBead: "bead-1",
	})

	_, err := ApproveImpl(tmp, "010-test")
	if err == nil {
		t.Fatal("expected error for wrong mode")
	}
	if !strings.Contains(err.Error(), "expected review mode") {
		t.Errorf("error should mention expected review mode: %v", err)
	}
}

func TestApproveImpl_WrongSpec(t *testing.T) {
	tmp := t.TempDir()

	writeLifecycleSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
	})

	origFindLocalRoot := findLocalRootFn
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test") }
	t.Cleanup(func() { findLocalRootFn = origFindLocalRoot })

	_, err := ApproveImpl(tmp, "011-other")
	if err == nil {
		t.Fatal("expected error for wrong spec")
	}
	if !strings.Contains(err.Error(), "active spec") {
		t.Errorf("error should mention active spec mismatch: %v", err)
	}
}

func TestApproveImpl_EpicCloseFailureWarns(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
	})

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
	commitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }
	branchExistsFn = func(name string) bool { return false }

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for failed epic close")
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "epic-parent") {
		t.Errorf("expected warning to mention epic: %v", result.Warnings)
	}
}

func TestApproveImpl_DirectMergeSummary(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

	origFindLocalRoot := findLocalRootFn
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test") }
	origRunBD := implRunBDFn
	origRunBDCombined := implRunBDCombinedFn
	origLoadConfig := loadConfigFn
	origMergeBranch := mergeBranchFn
	origDeleteBranch := deleteBranchFn
	origWorktreeRemove := worktreeRemoveFn
	origDiffStat := diffStatFn
	origCommitCount := commitCountFn
	defer func() {
		implRunBDFn = origRunBD
		implRunBDCombinedFn = origRunBDCombined
		loadConfigFn = origLoadConfig
		mergeBranchFn = origMergeBranch
		deleteBranchFn = origDeleteBranch
		worktreeRemoveFn = origWorktreeRemove
		diffStatFn = origDiffStat
		commitCountFn = origCommitCount
		findLocalRootFn = origFindLocalRoot
	}()

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	loadConfigFn = func(root string) (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.MergeStrategy = "direct"
		return cfg, nil
	}
	mergeBranchFn = func(workdir, source, target string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }
	diffStatFn = func(workdir, base, head string) (string, error) {
		return " 3 files changed, 50 insertions(+), 10 deletions(-)", nil
	}
	commitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MergeStrategy != "direct" {
		t.Errorf("MergeStrategy: got %q, want %q", result.MergeStrategy, "direct")
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

func TestApproveImpl_CleanupRunsFromRoot(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

	specWorktreePath := filepath.Join(tmp, ".worktrees", "worktree-spec-010-test")
	if err := os.MkdirAll(specWorktreePath, 0755); err != nil {
		t.Fatalf("mkdir spec worktree: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(specWorktreePath); err != nil {
		t.Fatalf("chdir spec worktree: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	loadConfigFn = func(root string) (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.MergeStrategy = "direct"
		return cfg, nil
	}
	mergeBranchFn = func(workdir, source, target string) error { return nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 1, nil }
	branchExistsFn = func(name string) bool { return false }
	diffStatFn = func(workdir, base, head string) (string, error) { return "", nil }

	var worktreeRemoved bool
	worktreeRemoveFn = func(name string) error {
		worktreeRemoved = true
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cwdInfo, err := os.Stat(cwd)
		if err != nil {
			return err
		}
		rootInfo, err := os.Stat(tmp)
		if err != nil {
			return err
		}
		if !os.SameFile(cwdInfo, rootInfo) {
			return fmt.Errorf("expected cleanup cwd %s, got %s", tmp, cwd)
		}
		if name != "worktree-spec-010-test" {
			return fmt.Errorf("unexpected worktree name: %s", name)
		}
		return nil
	}
	deleteBranchFn = func(name string) error { return nil }

	if _, err := ApproveImpl(tmp, "010-test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !worktreeRemoved {
		t.Fatal("expected spec worktree cleanup to run")
	}
}

func TestApproveImpl_WritesIdleFocusToRootAndLocalWorktree(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	specWorktreePath := filepath.Join(tmp, ".worktrees", "worktree-spec-010-test")
	if err := os.MkdirAll(filepath.Join(specWorktreePath, ".mindspec"), 0755); err != nil {
		t.Fatalf("mkdir spec worktree .mindspec: %v", err)
	}

	if err := state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	}); err != nil {
		t.Fatalf("write root focus: %v", err)
	}
	if err := state.WriteFocus(specWorktreePath, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	}); err != nil {
		t.Fatalf("write local focus: %v", err)
	}

	saveAndRestore(t)
	findLocalRootFn = func() (string, error) { return specWorktreePath, nil }
	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	loadConfigFn = func(root string) (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.MergeStrategy = "direct"
		return cfg, nil
	}
	mergeBranchFn = func(workdir, source, target string) error { return nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 1, nil }
	branchExistsFn = func(name string) bool { return false }
	diffStatFn = func(workdir, base, head string) (string, error) { return "", nil }
	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }

	if _, err := ApproveImpl(tmp, "010-test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rootFocus, err := state.ReadFocus(tmp)
	if err != nil {
		t.Fatalf("reading root focus: %v", err)
	}
	if rootFocus == nil || rootFocus.Mode != state.ModeIdle {
		t.Fatalf("root focus mode = %v, want %q", rootFocus, state.ModeIdle)
	}

	localFocus, err := state.ReadFocus(specWorktreePath)
	if err != nil {
		t.Fatalf("reading local focus: %v", err)
	}
	if localFocus == nil || localFocus.Mode != state.ModeIdle {
		t.Fatalf("local focus mode = %v, want %q", localFocus, state.ModeIdle)
	}
}

func TestApproveImpl_UsesRootFocusWhenLocalFocusMissing(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	specWorktreePath := filepath.Join(tmp, ".worktrees", "worktree-spec-010-test")
	if err := os.MkdirAll(specWorktreePath, 0755); err != nil {
		t.Fatalf("mkdir spec worktree: %v", err)
	}

	if err := state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	}); err != nil {
		t.Fatalf("write root focus: %v", err)
	}

	saveAndRestore(t)
	findLocalRootFn = func() (string, error) { return specWorktreePath, nil }
	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	loadConfigFn = func(root string) (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.MergeStrategy = "direct"
		return cfg, nil
	}
	mergeBranchFn = func(workdir, source, target string) error { return nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 1, nil }
	branchExistsFn = func(name string) bool { return false }
	diffStatFn = func(workdir, base, head string) (string, error) { return "", nil }
	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }

	if _, err := ApproveImpl(tmp, "010-test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rootFocus, err := state.ReadFocus(tmp)
	if err != nil {
		t.Fatalf("reading root focus: %v", err)
	}
	if rootFocus == nil || rootFocus.Mode != state.ModeIdle {
		t.Fatalf("root focus mode = %v, want %q", rootFocus, state.ModeIdle)
	}
}

func TestApproveImpl_PRWaitFlow(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

	origFindLocalRoot := findLocalRootFn
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test") }
	origRunBD := implRunBDFn
	origRunBDCombined := implRunBDCombinedFn
	origLoadConfig := loadConfigFn
	origHasRemote := hasRemoteFn
	origPushBranch := pushBranchFn
	origCreatePR := createPRFn
	origDeleteBranch := deleteBranchFn
	origWorktreeRemove := worktreeRemoveFn
	origDiffStat := diffStatFn
	origCommitCount := commitCountFn
	origPRChecksWatch := prChecksWatchFn
	origMergePR := mergePRFn
	defer func() {
		implRunBDFn = origRunBD
		implRunBDCombinedFn = origRunBDCombined
		loadConfigFn = origLoadConfig
		hasRemoteFn = origHasRemote
		pushBranchFn = origPushBranch
		createPRFn = origCreatePR
		deleteBranchFn = origDeleteBranch
		worktreeRemoveFn = origWorktreeRemove
		diffStatFn = origDiffStat
		commitCountFn = origCommitCount
		prChecksWatchFn = origPRChecksWatch
		mergePRFn = origMergePR
		findLocalRootFn = origFindLocalRoot
	}()

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	loadConfigFn = func(root string) (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.MergeStrategy = "pr"
		return cfg, nil
	}
	hasRemoteFn = func() bool { return true }
	pushBranchFn = func(branch string) error { return nil }
	createPRFn = func(branch, base, title, body string) (string, error) {
		return "https://github.com/test/repo/pull/42", nil
	}
	deleteBranchFn = func(name string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }
	diffStatFn = func(workdir, base, head string) (string, error) { return "1 file changed", nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 3, nil }
	prChecksWatchFn = func(url string) error { return nil }
	mergePRFn = func(url string) error { return nil }

	result, err := ApproveImpl(tmp, "010-test", ImplOpts{Wait: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MergeStrategy != "pr" {
		t.Errorf("MergeStrategy: got %q, want %q", result.MergeStrategy, "pr")
	}
	if result.PRURL != "https://github.com/test/repo/pull/42" {
		t.Errorf("PRURL: got %q", result.PRURL)
	}
	if !result.PRMerged {
		t.Error("expected PRMerged to be true with --wait")
	}
}

func TestApproveImpl_PRNoWaitFlow(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

	origFindLocalRoot := findLocalRootFn
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test") }
	origRunBD := implRunBDFn
	origRunBDCombined := implRunBDCombinedFn
	origLoadConfig := loadConfigFn
	origHasRemote := hasRemoteFn
	origPushBranch := pushBranchFn
	origCreatePR := createPRFn
	origDeleteBranch := deleteBranchFn
	origWorktreeRemove := worktreeRemoveFn
	origDiffStat := diffStatFn
	origCommitCount := commitCountFn
	defer func() {
		implRunBDFn = origRunBD
		implRunBDCombinedFn = origRunBDCombined
		loadConfigFn = origLoadConfig
		hasRemoteFn = origHasRemote
		pushBranchFn = origPushBranch
		createPRFn = origCreatePR
		deleteBranchFn = origDeleteBranch
		worktreeRemoveFn = origWorktreeRemove
		diffStatFn = origDiffStat
		commitCountFn = origCommitCount
		findLocalRootFn = origFindLocalRoot
	}()

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	loadConfigFn = func(root string) (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.MergeStrategy = "pr"
		return cfg, nil
	}
	hasRemoteFn = func() bool { return true }
	pushBranchFn = func(branch string) error { return nil }
	createPRFn = func(branch, base, title, body string) (string, error) {
		return "https://github.com/test/repo/pull/42", nil
	}
	// These should NOT be called without --wait
	var worktreeRemoved, branchDeleted bool
	deleteBranchFn = func(name string) error { branchDeleted = true; return nil }
	worktreeRemoveFn = func(name string) error { worktreeRemoved = true; return nil }
	diffStatFn = func(workdir, base, head string) (string, error) { return "", nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 1, nil }

	result, err := ApproveImpl(tmp, "010-test") // no opts = no wait
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.PRMerged {
		t.Error("expected PRMerged to be false without --wait")
	}
	if worktreeRemoved {
		t.Error("worktree should not be removed without --wait PR merge")
	}
	if branchDeleted {
		t.Error("branch should not be deleted without --wait PR merge")
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

// saveAndRestore saves the current values of all impl function variables and
// returns a cleanup function that restores them.
func saveAndRestore(t *testing.T) {
	t.Helper()
	origRunBD := implRunBDFn
	origRunBDCombined := implRunBDCombinedFn
	origLoadConfig := loadConfigFn
	origMergeBranch := mergeBranchFn
	origDeleteBranch := deleteBranchFn
	origWorktreeRemove := worktreeRemoveFn
	origDiffStat := diffStatFn
	origCommitCount := commitCountFn
	origIsAncestor := isAncestorFn
	origBranchExists := branchExistsFn
	origHasRemote := hasRemoteFn
	origPushBranch := pushBranchFn
	origCreatePR := createPRFn
	origPRChecksWatch := prChecksWatchFn
	origMergePR := mergePRFn
	origFindLocalRoot := findLocalRootFn
	t.Cleanup(func() {
		implRunBDFn = origRunBD
		implRunBDCombinedFn = origRunBDCombined
		loadConfigFn = origLoadConfig
		mergeBranchFn = origMergeBranch
		deleteBranchFn = origDeleteBranch
		worktreeRemoveFn = origWorktreeRemove
		diffStatFn = origDiffStat
		commitCountFn = origCommitCount
		isAncestorFn = origIsAncestor
		branchExistsFn = origBranchExists
		hasRemoteFn = origHasRemote
		pushBranchFn = origPushBranch
		createPRFn = origCreatePR
		prChecksWatchFn = origPRChecksWatch
		mergePRFn = origMergePR
		findLocalRootFn = origFindLocalRoot
	})

	// Default: findLocalRoot falls back to the root arg passed to ApproveImpl.
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test: no local root") }

	// Deterministic defaults for tests that don't care about merge transport.
	// Individual tests can override any of these as needed.
	loadConfigFn = func(root string) (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.MergeStrategy = "direct"
		return cfg, nil
	}
	mergeBranchFn = func(workdir, source, target string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }
	diffStatFn = func(workdir, base, head string) (string, error) { return "", nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 1, nil }
	isAncestorFn = func(workdir, ancestor, descendant string) (bool, error) { return true, nil }
	branchExistsFn = func(name string) bool { return false }
	hasRemoteFn = func() bool { return false }
	pushBranchFn = func(branch string) error { return nil }
	createPRFn = func(branch, base, title, body string) (string, error) {
		return "https://github.com/test/repo/pull/1", nil
	}
	prChecksWatchFn = func(url string) error { return nil }
	mergePRFn = func(url string) error { return nil }
}

func TestVerifyImplContent_NoCommits(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "closed"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 0, nil }

	_, err := ApproveImpl(tmp, "010-test")
	if err == nil {
		t.Fatal("expected error when spec branch has no commits beyond main")
	}
	if !strings.Contains(err.Error(), "no commits beyond main") {
		t.Errorf("error should mention no commits: %v", err)
	}
}

func TestVerifyImplContent_NoCommitsButClosedBeads_AllowsCleanup(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			payload := []map[string]string{{"status": "closed"}}
			return json.Marshal(payload)
		}
		return nil, fmt.Errorf("unexpected args: %v", args)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 0, nil }
	branchExistsFn = func(name string) bool { return true }
	loadConfigFn = func(root string) (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.MergeStrategy = "direct"
		return cfg, nil
	}
	mergeBranchFn = func(workdir, source, target string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }
	diffStatFn = func(workdir, base, head string) (string, error) { return "", nil }

	if _, err := ApproveImpl(tmp, "010-test"); err != nil {
		t.Fatalf("expected approval to continue as cleanup path, got: %v", err)
	}
}

func TestVerifyImplContent_OpenBeads(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa", "bead-bbb"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

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
	commitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }
	branchExistsFn = func(name string) bool { return false }

	_, err := ApproveImpl(tmp, "010-test")
	if err == nil {
		t.Fatal("expected error when beads are still open")
	}
	if !strings.Contains(err.Error(), "bead-bbb") || !strings.Contains(err.Error(), "in_progress") {
		t.Errorf("error should mention open bead: %v", err)
	}
}

func TestVerifyImplContent_BeadBranchAutoMerged(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "closed"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }
	branchExistsFn = func(name string) bool { return name == "bead/bead-aaa" }

	isAncestorFn = func(workdir, ancestor, descendant string) (bool, error) { return false, nil }

	type mergeCall struct{ source, target string }
	var merges []mergeCall
	mergeBranchFn = func(workdir, source, target string) error {
		merges = append(merges, mergeCall{source, target})
		return nil
	}
	deleteBranchFn = func(name string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }
	loadConfigFn = func(root string) (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.MergeStrategy = "direct"
		return cfg, nil
	}
	diffStatFn = func(workdir, base, head string) (string, error) { return "1 file changed", nil }

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("expected auto-merge to succeed, got error: %v", err)
	}

	// First merge should be bead->spec (auto-merge), second is spec->main
	if len(merges) < 1 {
		t.Fatal("expected at least one merge call")
	}
	if merges[0].source != "bead/bead-aaa" {
		t.Errorf("expected first merge source bead/bead-aaa, got %q", merges[0].source)
	}
	if merges[0].target != "spec/010-test" {
		t.Errorf("expected first merge target spec/010-test, got %q", merges[0].target)
	}
	if result.MergeStrategy != "direct" {
		t.Errorf("MergeStrategy: got %q, want %q", result.MergeStrategy, "direct")
	}
}

func TestVerifyImplContent_BeadBranchMergeFails(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "closed"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }
	branchExistsFn = func(name string) bool { return name == "bead/bead-aaa" }
	isAncestorFn = func(workdir, ancestor, descendant string) (bool, error) { return false, nil }
	mergeBranchFn = func(workdir, source, target string) error {
		return fmt.Errorf("merge conflict")
	}

	_, err := ApproveImpl(tmp, "010-test")
	if err == nil {
		t.Fatal("expected error when auto-merge fails")
	}
	if !strings.Contains(err.Error(), "merging bead branch") {
		t.Errorf("error should mention merge failure: %v", err)
	}
}

func TestVerifyImplContent_AllGood(t *testing.T) {
	tmp := t.TempDir()
	writeLifecycleSpec(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa", "bead-bbb"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.WriteFocus(tmp, &state.Focus{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "closed"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }
	branchExistsFn = func(name string) bool { return true }
	isAncestorFn = func(workdir, ancestor, descendant string) (bool, error) { return true, nil }
	loadConfigFn = func(root string) (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.MergeStrategy = "direct"
		return cfg, nil
	}
	mergeBranchFn = func(workdir, source, target string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }
	diffStatFn = func(workdir, base, head string) (string, error) { return "2 files changed", nil }

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeStrategy != "direct" {
		t.Errorf("MergeStrategy: got %q, want %q", result.MergeStrategy, "direct")
	}
}
