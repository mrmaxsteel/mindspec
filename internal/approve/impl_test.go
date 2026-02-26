package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mindspec/mindspec/internal/config"
	"github.com/mindspec/mindspec/internal/state"
)

func writeBoundSpec(t *testing.T, root, specID string) {
	t.Helper()
	specDir := filepath.Join(root, "docs", "specs", specID)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	spec := `---
molecule_id: mol-parent
step_mapping:
  spec: step-spec
  spec-approve: step-spec-approve
  plan: step-plan
  plan-approve: step-plan-approve
  implement: step-impl
  review: step-review
  spec-lifecycle: mol-parent
---
# Spec ` + specID + `
`
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
}

func TestApproveImpl_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	writeBoundSpec(t, tmp, "010-test")

	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	// Set state to review mode
	state.Write(tmp, &state.State{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
	})

	origRunBD := implRunBDFn
	origRunBDCombined := implRunBDCombinedFn
	defer func() {
		implRunBDFn = origRunBD
		implRunBDCombinedFn = origRunBDCombined
	}()

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

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SpecID != "010-test" {
		t.Errorf("SpecID: got %q, want %q", result.SpecID, "010-test")
	}
	got := append([]string(nil), closed...)
	sort.Strings(got)
	want := []string{"mol-parent", "step-impl", "step-plan", "step-plan-approve", "step-review", "step-spec", "step-spec-approve"}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("closed IDs mismatch\ngot:  %v\nwant: %v", got, want)
	}

	// Verify state is now idle
	s, err := state.Read(tmp)
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	if s.Mode != state.ModeIdle {
		t.Errorf("mode: got %q, want %q", s.Mode, state.ModeIdle)
	}
}

func TestApproveImpl_WrongMode(t *testing.T) {
	tmp := t.TempDir()

	writeBoundSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
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

	writeBoundSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
	})

	_, err := ApproveImpl(tmp, "011-other")
	if err == nil {
		t.Fatal("expected error for wrong spec")
	}
	if !strings.Contains(err.Error(), "active spec") {
		t.Errorf("error should mention active spec mismatch: %v", err)
	}
}

func TestApproveImpl_PartialCloseFailureWarnsAndContinues(t *testing.T) {
	tmp := t.TempDir()
	writeBoundSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
	})

	origRunBD := implRunBDFn
	origRunBDCombined := implRunBDCombinedFn
	defer func() {
		implRunBDFn = origRunBD
		implRunBDCombinedFn = origRunBDCombined
	}()

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}

	var closed []string
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("unexpected args: %v", args)
		}
		if args[1] == "step-plan" {
			return nil, fmt.Errorf("boom")
		}
		closed = append(closed, args[1])
		return []byte("ok"), nil
	}

	result, err := ApproveImpl(tmp, "010-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for failed close")
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "step-plan") {
		t.Errorf("expected warning to mention failed member: %v", result.Warnings)
	}
	if len(closed) == 0 {
		t.Fatal("expected other members to still be closed")
	}
}

func TestApproveImpl_DirectMergeSummary(t *testing.T) {
	tmp := t.TempDir()
	writeBoundSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

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

func TestApproveImpl_PRWaitFlow(t *testing.T) {
	tmp := t.TempDir()
	writeBoundSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

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
	writeBoundSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
		Mode:       state.ModeReview,
		ActiveSpec: "010-test",
		SpecBranch: "spec/010-test",
	})

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
	})
}

func TestVerifyImplContent_NoCommits(t *testing.T) {
	tmp := t.TempDir()
	writeBoundSpec(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
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

func TestVerifyImplContent_OpenBeads(t *testing.T) {
	tmp := t.TempDir()
	writeBoundSpec(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa", "bead-bbb"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
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

func TestVerifyImplContent_BeadBranchNotMerged(t *testing.T) {
	tmp := t.TempDir()
	writeBoundSpec(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
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

	_, err := ApproveImpl(tmp, "010-test")
	if err == nil {
		t.Fatal("expected error when bead branch is not merged into spec branch")
	}
	if !strings.Contains(err.Error(), "bead/bead-aaa") || !strings.Contains(err.Error(), "not merged") {
		t.Errorf("error should mention unmerged bead branch: %v", err)
	}
}

func TestVerifyImplContent_AllGood(t *testing.T) {
	tmp := t.TempDir()
	writeBoundSpec(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-aaa", "bead-bbb"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	state.Write(tmp, &state.State{
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
