package approve

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/validate"
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

	// Spec 089: stub phase.EnsureMigrated's bd-shelling seam so the
	// migration write doesn't fail before the mode check runs.
	restorePhaseMerge := phase.SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		return nil
	})
	t.Cleanup(restorePhaseMerge)

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
	origMergeMeta := implMergeMetadataFn
	origGitEmail := implGitUserEmailFn
	t.Cleanup(func() {
		implRunBDFn = origRunBD
		implRunBDCombinedFn = origRunBDCombined
		implMergeMetadataFn = origMergeMeta
		implGitUserEmailFn = origGitEmail
	})

	// Spec 089: phase.EnsureMigrated (wired into approve-impl) shells to
	// `bd` via bead.MergeMetadata when the epic lacks mindspec_phase. CI
	// has no `bd` on PATH, so stub the seam to a no-op for the duration
	// of the test.
	restorePhaseMerge := phase.SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		return nil
	})
	t.Cleanup(restorePhaseMerge)

	// Stub phase package for review mode by default
	stubPhaseForReview(t)

	// Deterministic defaults for tests that don't care about specifics.
	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "open"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	// Spec 086 Bead 3: keep override metadata + git-identity reads
	// inert by default. Tests that observe the write swap these in.
	implMergeMetadataFn = func(id string, updates map[string]interface{}) error { return nil }
	implGitUserEmailFn = func() string { return "test@example.invalid" }
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

// --- Spec 086 Bead 3: doc-sync gate + override + call-order tests ---

// TestApproveImplBlocksOnSpecDocSkew exercises the doc-sync gate from
// the spec-branch perspective: a diff that modifies spec.md alone
// (no plan/ADR/sibling) trips spec-artifact-sync and ApproveImpl must
// return an error when no override is supplied.
func TestApproveImplBlocksOnSpecDocSkew(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
		MergeBaseResult:    "merge-base-sha",
		// spec.md-only change → spec-artifact-sync emits SevError.
		ChangedFilesResult: []string{".mindspec/docs/specs/010-test/spec.md"},
	}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected doc-sync gate to reject spec.md-only diff")
	}
	if !strings.Contains(err.Error(), "doc-sync") {
		t.Errorf("error should mention doc-sync: %v", err)
	}
	// FinalizeEpic MUST NOT have been called — the gate runs first.
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
		t.Errorf("FinalizeEpic must not be called when gate fails: got %d calls", len(calls))
	}
}

// TestApproveImplOverrideRecordsToEpic: same gated diff, but the
// AllowDocSkew override allows the approval to complete AND the
// override metadata is recorded on the spec EPIC after FinalizeEpic.
func TestApproveImplOverrideRecordsToEpic(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	var metaEpicID string
	var metaWrites []map[string]interface{}
	implMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		metaEpicID = id
		metaWrites = append(metaWrites, updates)
		return nil
	}
	implGitUserEmailFn = func() string { return "approver@example.invalid" }

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
		MergeBaseResult:    "merge-base-sha",
		ChangedFilesResult: []string{".mindspec/docs/specs/010-test/spec.md"},
	}

	_, err := ApproveImpl(tmp, "010-test", mock, ImplOpts{AllowDocSkew: "spec doc PR in flight"})
	if err != nil {
		t.Fatalf("override should allow approval, got: %v", err)
	}

	if metaEpicID != "epic-parent" {
		t.Errorf("override metadata should target epic-parent, got %q", metaEpicID)
	}
	found := false
	for _, m := range metaWrites {
		if reason, ok := m["mindspec_impl_skew_reason"].(string); ok && reason == "spec doc PR in flight" {
			if by, _ := m["mindspec_impl_skew_by"].(string); by != "approver@example.invalid" {
				t.Errorf("mindspec_impl_skew_by: got %q, want approver@example.invalid", by)
			}
			if at, _ := m["mindspec_impl_skew_at"].(string); at == "" {
				t.Error("mindspec_impl_skew_at should not be empty")
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mindspec_impl_skew_reason write; got %v", metaWrites)
	}

	// FinalizeEpic must have been called before the metadata write.
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 1 {
		t.Errorf("expected 1 FinalizeEpic call, got %d", len(calls))
	}
}

// TestApproveImplCallOrder parses internal/approve/impl.go and asserts
// the SEVEN anchored call expressions inside ApproveImpl appear in
// strict source order. Per panel CONSENSUS revision 9 the contract is:
//
//  1. readBeadStatus            (bead-status loop)
//  2. validate.ValidateDocs     (doc-sync gate)
//  3. validate.CheckADRDivergence (ADR-divergence gate)
//  4. implRunBDCombinedFn("close", ...)  (EPIC CLOSE)
//  5. bead.MergeMetadata with "mindspec_phase" literal (phase write)
//  6. exec.CommitCount          (pre-flight)
//  7. exec.FinalizeEpic         (terminal mutation)
//
// Additionally the override metadata write (implMergeMetadataFn with
// "mindspec_impl_skew_reason") must appear AFTER FinalizeEpic per
// panel CONSENSUS revision 4 (write-order rule).
func TestApproveImplCallOrder(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "impl.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse impl.go: %v", err)
	}

	type anchor struct {
		label string
		match func(call *ast.CallExpr) bool
		pos   token.Pos
	}

	anchors := []*anchor{
		{label: "readBeadStatus", match: func(c *ast.CallExpr) bool {
			id, ok := c.Fun.(*ast.Ident)
			return ok && id.Name == "readBeadStatus"
		}},
		{label: "validate.ValidateDocs", match: func(c *ast.CallExpr) bool {
			return isSelectorCall(c.Fun, "validate", "ValidateDocs")
		}},
		{label: "validate.CheckADRDivergence", match: func(c *ast.CallExpr) bool {
			return isSelectorCall(c.Fun, "validate", "CheckADRDivergence")
		}},
		{label: "implRunBDCombinedFn(\"close\")", match: func(c *ast.CallExpr) bool {
			id, ok := c.Fun.(*ast.Ident)
			if !ok || id.Name != "implRunBDCombinedFn" || len(c.Args) == 0 {
				return false
			}
			return firstArgStringLit(c) == "close"
		}},
		{label: "bead.MergeMetadata(mindspec_phase)", match: func(c *ast.CallExpr) bool {
			if !isSelectorCall(c.Fun, "bead", "MergeMetadata") || len(c.Args) < 2 {
				return false
			}
			return callMapHasKey(c.Args[1], "mindspec_phase")
		}},
		{label: "exec.CommitCount", match: func(c *ast.CallExpr) bool {
			return isSelectorCall(c.Fun, "exec", "CommitCount")
		}},
		{label: "exec.FinalizeEpic", match: func(c *ast.CallExpr) bool {
			return isSelectorCall(c.Fun, "exec", "FinalizeEpic")
		}},
	}

	// Override-skew metadata write — asserted to be AFTER FinalizeEpic.
	var overridePos token.Pos
	var finalizePos token.Pos

	// Find the ApproveImpl FuncDecl and walk its body.
	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fd.Name.Name == "ApproveImpl" {
			fn = fd
			break
		}
	}
	if fn == nil {
		t.Fatal("ApproveImpl FuncDecl not found")
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, a := range anchors {
			if a.pos == 0 && a.match(call) {
				a.pos = call.Pos()
			}
		}
		if isSelectorCall(call.Fun, "exec", "FinalizeEpic") && finalizePos == 0 {
			finalizePos = call.Pos()
		}
		// Override metadata: any call to implMergeMetadataFn inside
		// ApproveImpl is the override-skew write (there is only one).
		// The reason-key literal lives inside the buildImplSkewMetadata
		// helper, which is statically resolvable but not via a single
		// CallExpr walk — keeping the anchor on the function-var name
		// pins the source-position contract.
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "implMergeMetadataFn" {
			overridePos = call.Pos()
		}
		return true
	})

	for _, a := range anchors {
		if a.pos == 0 {
			t.Errorf("anchor %s not found in ApproveImpl body", a.label)
		}
	}
	if t.Failed() {
		t.FailNow()
	}

	// Strict source-position ordering.
	for i := 1; i < len(anchors); i++ {
		if !(anchors[i-1].pos < anchors[i].pos) {
			t.Errorf("call order violation: %s (pos %d) must precede %s (pos %d)",
				anchors[i-1].label, anchors[i-1].pos, anchors[i].label, anchors[i].pos)
		}
	}

	// Override metadata write must be AFTER FinalizeEpic.
	if overridePos == 0 {
		t.Error("override metadata write (implMergeMetadataFn call inside ApproveImpl) not found")
	} else if !(finalizePos < overridePos) {
		t.Errorf("override metadata write (pos %d) must appear AFTER FinalizeEpic (pos %d)", overridePos, finalizePos)
	}

	// Cross-check: the helper that supplies the metadata must carry
	// the "mindspec_impl_skew_reason" key literal so the override
	// write is recording the right field. This pins the impl-side
	// contract that the bead description enumerates.
	src, err := os.ReadFile("impl.go")
	if err != nil {
		t.Fatalf("read impl.go: %v", err)
	}
	if !strings.Contains(string(src), "\"mindspec_impl_skew_reason\"") {
		t.Error("impl.go must contain the literal \"mindspec_impl_skew_reason\" (the override metadata key)")
	}
}

// --- Spec 087 Bead 3: ADR-divergence override/supersede mirror tests ---

// writeADRDivergenceFixtureImpl builds an approve-side fixture that
// trips ADR-divergence on the spec branch — a spec.md declaring
// "core" as an impacted domain, a plan.md citing only an
// execution-domain ADR, and that ADR on disk.
func writeADRDivergenceFixtureImpl(t *testing.T, root, specID string) {
	t.Helper()

	specDir := filepath.Join(root, "docs", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Spec "+specID+"\n\n## Impacted Domains\n\n- core\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	planMD := "---\nspec_id: " + specID + "\nstatus: Approved\nbead_ids:\n  - bead-1\nadr_citations:\n  - id: ADR-9001\n---\n\n# Plan\n"
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(planMD), 0o644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}

	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("mkdir adr dir: %v", err)
	}
	adrMD := "# ADR-9001: Exec-only test\n\n" +
		"- **Date**: 2026-01-01\n" +
		"- **Status**: Accepted\n" +
		"- **Domain(s)**: execution\n" +
		"- **Deciders**: test\n" +
		"- **Supersedes**: n/a\n" +
		"- **Superseded-by**: n/a\n\n" +
		"## Decision\nTest fixture.\n"
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-9001.md"), []byte(adrMD), 0o644); err != nil {
		t.Fatalf("write ADR-9001.md: %v", err)
	}
}

// TestApproveImplOverrideMetadataGoesThroughSeam mirrors the complete
// package's TestOverrideMetadataGoesThroughSeam: the override write
// on the spec EPIC MUST flow through implMergeMetadataFn (the spec
// 087 Bead 3 seam) and the write must happen AFTER FinalizeEpic
// returns nil (spec 086 panel CONSENSUS revision 4 discipline).
func TestApproveImplOverrideMetadataGoesThroughSeam(t *testing.T) {
	tmp := t.TempDir()
	writeADRDivergenceFixtureImpl(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755)

	saveAndRestore(t)

	seamCalls := 0
	seenBeforeFinalize := false
	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
		MergeBaseResult:    "merge-base-sha",
		// Source touch attributed to "core" via the fallback path.
		ChangedFilesResult: []string{"internal/core/foo.go"},
	}
	implMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if _, ok := updates["mindspec_adr_override_reason"]; ok {
			seamCalls++
			if len(mock.CallsTo("FinalizeEpic")) == 0 {
				seenBeforeFinalize = true
			}
		}
		return nil
	}
	implRunBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]map[string]string{{"status": "closed"}})
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	_, err := ApproveImpl(tmp, "010-test", mock, ImplOpts{
		AllowDocSkew: "test setup",
		OverrideADR:  "wip — core ADR coming in followup",
	})
	if err != nil {
		t.Fatalf("override should allow approval, got: %v", err)
	}
	if seamCalls != 1 {
		t.Errorf("expected exactly one seam call with mindspec_adr_override_reason; got %d", seamCalls)
	}
	if seenBeforeFinalize {
		t.Error("override metadata write occurred before FinalizeEpic — panel CONSENSUS rev 4 violation")
	}
}

// --- AST helpers (kept in this file; not exported) ---

func isSelectorCall(expr ast.Expr, recv, sel string) bool {
	se, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := se.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == recv && se.Sel.Name == sel
}

func firstArgStringLit(c *ast.CallExpr) string {
	if len(c.Args) == 0 {
		return ""
	}
	lit, ok := c.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	// trim surrounding quotes
	s := lit.Value
	if len(s) >= 2 {
		s = s[1 : len(s)-1]
	}
	return s
}

// callMapHasKey returns true when expr is a composite literal of type
// map[string]interface{}{...} that contains the given string key
// literal among its elements.
func callMapHasKey(expr ast.Expr, key string) bool {
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		return false
	}
	for _, e := range cl.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		lit, ok := kv.Key.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		raw := lit.Value
		if len(raw) >= 2 {
			raw = raw[1 : len(raw)-1]
		}
		if raw == key {
			return true
		}
	}
	return false
}

// --- Spec 091 Bead 5: warnings pipe (Req 22(a) + printing half of 22(b)) ---

// captureWarnOutput swaps the package-level warnWriter seam for a
// buffer and restores it on cleanup.
func captureWarnOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := warnWriter
	warnWriter = &buf
	t.Cleanup(func() { warnWriter = orig })
	return &buf
}

// TestApproveImplWarnStreamDefaultsToStderr pins the Req 22(a) stream
// contract: in production, WARN lines go to stderr.
func TestApproveImplWarnStreamDefaultsToStderr(t *testing.T) {
	if warnWriter != os.Stderr {
		t.Errorf("warnWriter must default to os.Stderr, got %T", warnWriter)
	}
}

// TestApproveImplPrintsDocSyncWarningAndProceeds: a diff that
// produces a warning-severity doc-sync issue but NO errors must print
// `WARN <name>: <message>` AND approve successfully (warnings never
// block). Req 22(a), including the HasFailures()==false case.
func TestApproveImplPrintsDocSyncWarningAndProceeds(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)
	buf := captureWarnOutput(t)

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
		MergeBaseResult:    "merge-base-sha",
		// cmd/ source + a non-operator doc: the cmd-docs lane emits a
		// SevWarning and no lane emits a SevError.
		ChangedFilesResult: []string{"cmd/mindspec/foo.go", "docs/notes.md"},
	}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err != nil {
		t.Fatalf("warning-only doc-sync result must not block approval, got: %v", err)
	}
	// The gate passed without override: FinalizeEpic ran.
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 1 {
		t.Errorf("expected 1 FinalizeEpic call, got %d", len(calls))
	}

	out := buf.String()
	if !strings.Contains(out, "WARN cmd-docs: cmd/ changes without operator-docs update") {
		t.Errorf("expected `WARN cmd-docs: <message>` line, got %q", out)
	}
	// Exact format: line starts with `WARN <name>: ` (no decoration).
	if !strings.HasPrefix(out, "WARN cmd-docs: ") {
		t.Errorf("WARN line must be formatted `WARN <name>: <message>`, got %q", out)
	}
	// Exactly one consumer prints — no double-print.
	if n := strings.Count(out, "WARN cmd-docs:"); n != 1 {
		t.Errorf("expected exactly 1 WARN line per issue per run, got %d in %q", n, out)
	}
}

// TestApproveImplNoWarningsPrintsNothing: zero warning-severity
// issues → no WARN line (companion case for Req 22(a)).
func TestApproveImplNoWarningsPrintsNothing(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)
	buf := captureWarnOutput(t)

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
		MergeBaseResult:    "merge-base-sha",
		ChangedFilesResult: nil, // empty diff → no issues at all
	}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err != nil {
		t.Fatalf("clean doc-sync result must approve, got: %v", err)
	}
	if strings.Contains(buf.String(), "WARN") {
		t.Errorf("no warnings in result → no WARN output, got %q", buf.String())
	}
}

// TestApproveImplPrintResultWarningsRecursStatelessly pins the HC-2
// printing half: rendering the SAME warning-carrying result twice
// prints the WARN line BOTH times (no suppression, no dedup) and
// creates no marker/state file anywhere (the rendering path does no
// persistence). It also pins severity-genericity: ANY SevWarning
// renders, error issues never do.
func TestApproveImplPrintResultWarningsRecursStatelessly(t *testing.T) {
	// Run from an empty dir so any sneaky relative-path persistence
	// would be visible.
	dir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	r := &validate.Result{}
	r.AddWarning("missing-source-globs", "source_globs not set in .mindspec/config.yaml")
	r.AddError("doc-sync", "errors are not rendered by the warnings pipe")

	var buf bytes.Buffer
	printResultWarnings(&buf, r)
	printResultWarnings(&buf, r) // recurrence: same result, second run

	want := "WARN missing-source-globs: source_globs not set in .mindspec/config.yaml\n"
	if buf.String() != want+want {
		t.Errorf("warning must print on BOTH runs, verbatim:\nwant %q\ngot  %q", want+want, buf.String())
	}
	if strings.Contains(buf.String(), "doc-sync") {
		t.Errorf("SevError issues must not render as WARN lines: %q", buf.String())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("rendering must persist NO marker/state file (HC-2); found %v", entries)
	}
}
