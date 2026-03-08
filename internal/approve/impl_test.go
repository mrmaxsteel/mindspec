package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
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
	commitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }

	result, err := ApproveImpl(tmp, "010-test")
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

	// Per ADR-0023: no focus file is written (beads is single state authority).
	// No lifecycle.yaml is updated.
}

func TestApproveImpl_WrongMode(t *testing.T) {
	tmp := t.TempDir()

	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	origFindLocalRoot := findLocalRootFn
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test") }
	t.Cleanup(func() { findLocalRootFn = origFindLocalRoot })

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

	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	origFindLocalRoot := findLocalRootFn
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test") }
	t.Cleanup(func() { findLocalRootFn = origFindLocalRoot })

	// Phase stub returns review mode for spec 010-test (not 011-other)
	stubPhaseForReview(t)

	_, err := ApproveImpl(tmp, "011-other")
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
	commitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }

	result, err := ApproveImpl(tmp, "010-test")
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
	deleteBranchFn = func(name string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }
	diffStatFn = func(workdir, base, head string) (string, error) {
		return " 3 files changed, 50 insertions(+), 10 deletions(-)", nil
	}
	commitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }

	var pushed bool
	hasRemoteFn = func() bool { return true }
	pushBranchFn = func(branch string) error {
		pushed = true
		if branch != "spec/010-test" {
			return fmt.Errorf("unexpected branch: %s", branch)
		}
		return nil
	}

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !pushed {
		t.Error("expected branch to be pushed")
	}
	if !result.Pushed {
		t.Error("result.Pushed should be true")
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
	commitCountFn = func(workdir, base, head string) (int, error) { return 3, nil }
	hasRemoteFn = func() bool { return false }

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pushed {
		t.Error("expected Pushed to be false when no remote")
	}
}

func TestApproveImpl_CleanupRunsFromRoot(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

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
	commitCountFn = func(workdir, base, head string) (int, error) { return 1, nil }
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
	// queryEpics and queryChildren now use listJSONFn, not runBDFn
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
	// runBDFn still used for bd show, bd dolt pull, etc.
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

// saveAndRestore saves the current values of all impl function variables and
// restores them via t.Cleanup.
func saveAndRestore(t *testing.T) {
	t.Helper()
	origRunBD := implRunBDFn
	origRunBDCombined := implRunBDCombinedFn
	origMergeBranch := mergeBranchFn
	origDeleteBranch := deleteBranchFn
	origWorktreeRemove := worktreeRemoveFn
	origDiffStat := diffStatFn
	origCommitCount := commitCountFn
	origIsAncestor := isAncestorFn
	origBranchExists := branchExistsFn
	origHasRemote := hasRemoteFn
	origPushBranch := pushBranchFn
	origFindLocalRoot := findLocalRootFn
	origCommitAll := commitAllFn
	origWorktreeList := worktreeListFn
	t.Cleanup(func() {
		implRunBDFn = origRunBD
		implRunBDCombinedFn = origRunBDCombined
		mergeBranchFn = origMergeBranch
		deleteBranchFn = origDeleteBranch
		worktreeRemoveFn = origWorktreeRemove
		worktreeListFn = origWorktreeList
		diffStatFn = origDiffStat
		commitCountFn = origCommitCount
		isAncestorFn = origIsAncestor
		branchExistsFn = origBranchExists
		hasRemoteFn = origHasRemote
		pushBranchFn = origPushBranch
		findLocalRootFn = origFindLocalRoot
		commitAllFn = origCommitAll
	})

	// Default: findLocalRoot falls back to the root arg passed to ApproveImpl.
	findLocalRootFn = func() (string, error) { return "", fmt.Errorf("test: no local root") }

	// Stub phase package for review mode by default
	stubPhaseForReview(t)

	// Deterministic defaults for tests that don't care about specifics.
	mergeBranchFn = func(workdir, source, target string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }
	diffStatFn = func(workdir, base, head string) (string, error) { return "", nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 1, nil }
	isAncestorFn = func(workdir, ancestor, descendant string) (bool, error) { return true, nil }
	branchExistsFn = func(name string) bool { return true }
	hasRemoteFn = func() bool { return false }
	pushBranchFn = func(branch string) error { return nil }
	commitAllFn = func(workdir, message string) error { return nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
}

func TestVerifyImplContent_NoCommits(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

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
	commitCountFn = func(workdir, base, head string) (int, error) { return 0, nil }
	branchExistsFn = func(name string) bool { return true }
	worktreeRemoveFn = func(name string) error { return nil }
	deleteBranchFn = func(name string) error { return nil }
	diffStatFn = func(workdir, base, head string) (string, error) { return "", nil }

	if _, err := ApproveImpl(tmp, "010-test"); err != nil {
		t.Fatalf("expected approval to continue as cleanup path, got: %v", err)
	}
}

func TestVerifyImplContent_OpenBeads(t *testing.T) {
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
	commitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }

	err := verifyImplContent(tmp, "spec/010-test", "010-test")
	if err == nil {
		t.Fatal("expected error when beads are still open")
	}
	if !strings.Contains(err.Error(), "bead-bbb") || !strings.Contains(err.Error(), "in_progress") {
		t.Errorf("error should mention open bead: %v", err)
	}
}

func TestVerifyImplContent_BeadBranchAutoMerged(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

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

	err := verifyImplContent(tmp, "spec/010-test", "010-test")
	if err != nil {
		t.Fatalf("expected auto-merge to succeed, got error: %v", err)
	}

	// Merge should be bead->spec (auto-merge during verifyImplContent)
	if len(merges) < 1 {
		t.Fatal("expected at least one merge call")
	}
	if merges[0].source != "bead/bead-aaa" {
		t.Errorf("expected first merge source bead/bead-aaa, got %q", merges[0].source)
	}
	if merges[0].target != "spec/010-test" {
		t.Errorf("expected first merge target spec/010-test, got %q", merges[0].target)
	}
}

func TestVerifyImplContent_BeadBranchMergeFails(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

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

	err := verifyImplContent(tmp, "spec/010-test", "010-test")
	if err == nil {
		t.Fatal("expected error when auto-merge fails")
	}
	if !strings.Contains(err.Error(), "merging bead branch") {
		t.Errorf("error should mention merge failure: %v", err)
	}
}

func TestApproveImpl_AutoCommitsSpecWorktree(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 3, nil }
	diffStatFn = func(workdir, base, head string) (string, error) { return "", nil }
	deleteBranchFn = func(name string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }

	var commitCalled bool
	var commitWorkdir, commitMsg string
	commitAllFn = func(workdir, message string) error {
		commitCalled = true
		commitWorkdir = workdir
		commitMsg = message
		return nil
	}

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !commitCalled {
		t.Fatal("expected auto-commit to be called before cleanup")
	}
	expectedWt := filepath.Join(tmp, ".worktrees", "worktree-spec-010-test")
	if commitWorkdir != expectedWt {
		t.Errorf("auto-commit workdir: got %q, want %q", commitWorkdir, expectedWt)
	}
	if !strings.Contains(commitMsg, "remaining spec artifacts") {
		t.Errorf("auto-commit message should mention spec artifacts, got: %q", commitMsg)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings on successful auto-commit, got: %v", result.Warnings)
	}
}

func TestApproveImpl_AutoCommitFailureWarns(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	commitCountFn = func(workdir, base, head string) (int, error) { return 3, nil }
	diffStatFn = func(workdir, base, head string) (string, error) { return "", nil }
	deleteBranchFn = func(name string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }

	commitAllFn = func(workdir, message string) error {
		return fmt.Errorf("commit failed: lock held")
	}

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("auto-commit failure should warn, not error: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning when auto-commit fails")
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "auto-commit") && strings.Contains(w, "lock held") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected auto-commit warning, got: %v", result.Warnings)
	}
}

func TestVerifyImplContent_AllGood(t *testing.T) {
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
	commitCountFn = func(workdir, base, head string) (int, error) { return 5, nil }
	branchExistsFn = func(name string) bool { return true }
	isAncestorFn = func(workdir, ancestor, descendant string) (bool, error) { return true, nil }
	deleteBranchFn = func(name string) error { return nil }
	worktreeRemoveFn = func(name string) error { return nil }
	diffStatFn = func(workdir, base, head string) (string, error) { return "2 files changed", nil }

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SpecBranch != "spec/010-test" {
		t.Errorf("SpecBranch: got %q, want %q", result.SpecBranch, "spec/010-test")
	}
}
