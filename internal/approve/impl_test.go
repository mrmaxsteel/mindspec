package approve

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

// serveRefFromDisk wires a MockExecutor's ref-read seams
// (FileAtRefOrAbsent + TreeDirsAtRef) to resolve against the on-disk
// tree at root, simulating "the diffed ref's tree == this fixture".
// Spec 095 moved the gate's OWNERSHIP attribution (manifests + domain
// enumeration) onto the diffed ref; these mock-backed unit tests build
// their OWNERSHIP fixture on disk, so this makes the ref read resolve to
// the same files. Absent paths classify as claims-nothing (present
// false, nil error) — never an operational error — mirroring the real
// MindspecExecutor.FileAtRefOrAbsent contract.
func serveRefFromDisk(mock *executor.MockExecutor, root string) {
	mock.FileAtRefOrAbsentFn = func(_ref, rel string) ([]byte, bool, error) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, false, nil
			}
			return nil, false, err
		}
		return data, true, nil
	}
	mock.TreeDirsAtRefFn = func(_ref, dir string) ([]string, error) {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		var dirs []string
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, e.Name())
			}
		}
		return dirs, nil
	}
}

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
	// Spec 115 Bead 2: the plan-bead gate + the obligation-backstop leg
	// both now require a readable plan.md — a mechanical fixture
	// addition (AC2(b)); the assertions below are unchanged.
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})

	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			payload := []map[string]string{{"status": "closed"}}
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
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "closed"}}
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
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
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
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "closed"}}
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
	// Spec 115 Bead 2: the obligation-backstop leg now requires a
	// readable plan.md — a mechanical fixture addition (AC2(b)); the
	// assertions below are unchanged.
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "closed"}}
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
	origPhaseMeta := implPhaseMetadataFn
	origGetwd := implGetwdFn
	// Spec 115 Bead 2: the pre-terminal orphan/obligation gate's seams.
	origScanOrphans := implScanOrphansFn
	origClosedEpicBeadIDs := implClosedEpicBeadIDsFn
	origWorktreeList := implWorktreeListFn
	origIsAncestor := implIsAncestorFn
	origBranchExists := implBranchExistsFn
	origGetMetadata := implGetMetadataFn
	origCheckObligations := implCheckObligationsFn
	t.Cleanup(func() {
		implRunBDFn = origRunBD
		implRunBDCombinedFn = origRunBDCombined
		implMergeMetadataFn = origMergeMeta
		implGitUserEmailFn = origGitEmail
		implPhaseMetadataFn = origPhaseMeta
		implGetwdFn = origGetwd
		implScanOrphansFn = origScanOrphans
		implClosedEpicBeadIDsFn = origClosedEpicBeadIDs
		implWorktreeListFn = origWorktreeList
		implIsAncestorFn = origIsAncestor
		implBranchExistsFn = origBranchExists
		implGetMetadataFn = origGetMetadata
		implCheckObligationsFn = origCheckObligations
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
		payload := []map[string]string{{"status": "closed"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	// Spec 086 Bead 3: keep override metadata + git-identity reads
	// inert by default. Tests that observe the write swap these in.
	implMergeMetadataFn = func(id string, updates map[string]interface{}) error { return nil }
	implGitUserEmailFn = func() string { return "test@example.invalid" }
	// Spec 092 Bead 3: phase reconcile/done writes inert by default.
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error { return nil }
	// Spec 092 Req 8: pin the context-line cwd so the asserted worktree
	// kind does not depend on where `go test` runs (the repo checkout
	// itself may be a bead worktree).
	implGetwdFn = func() (string, error) { return "/testcwd", nil }
	// Spec 115 Bead 2: the pre-terminal orphan/obligation gate's seams
	// default to inert no-ops — no orphans, no worktrees, no recorded
	// obligations — so every test that doesn't care about this gate
	// (the overwhelming majority) reaches its own assertions exactly as
	// before. Tests exercising the gate itself override these.
	implScanOrphansFn = func(specID, workdir, excludeBeadID string) ([]lifecycle.Orphan, error) {
		return nil, nil
	}
	implClosedEpicBeadIDsFn = func(specID string) ([]string, error) { return nil, nil }
	implWorktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	implIsAncestorFn = func(workdir, ancestor, descendant string) (bool, error) { return true, nil }
	implBranchExistsFn = func(name string) bool { return false }
	implGetMetadataFn = func(id string) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}
	implCheckObligationsFn = func(beadID string, getMeta func(string) (map[string]interface{}, error)) error {
		return nil
	}
}

// TestApproveImpl_NoCommitsNoBeads: with NO plan.md AND zero commits
// beyond main, ApproveImpl still errors — but Spec 115 Bead 2's R3
// obligation-backstop leg (fail-closed on an unreadable plan-bead
// enumeration) now intercepts this degenerate state BEFORE the
// CommitCount preflight ever runs (the new gate sits earlier in the
// call order — AC4). This is a deliberate, spec-mandated tightening: a
// missing plan.md must never again be silently tolerated by a
// downstream check (R3's whole point). This test pins that Leg 3
// intercepts the missing-plan state before the CommitCount preflight is
// ever reached, so FinalizeEpic must still never run. (The preflight's
// own degenerate-plan disjunction is now fully subsumed by Leg 3 and
// unreachable in normal flow — see the comment at the preflight call
// site in impl.go — so this test does not exercise that message at
// all.)
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
		t.Fatal("expected an error for a spec with no readable plan.md")
	}
	if !strings.Contains(err.Error(), "plan bead list could not be read") {
		t.Errorf("error should mention the unreadable plan-bead enumeration: %v", err)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("refusal must end with a recovery line: %v", err)
	}
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
		t.Errorf("FinalizeEpic must not run when the plan is unreadable: %d calls", len(calls))
	}
}

// TestApproveImpl_MissingPlanRefusesBeforeAnyMutation is a qb03
// regression test. mindspec-qb03 removed the CommitCount preflight's
// refusal DISJUNCTION (`count == 0 && (planErr != nil || len(beadIDs)
// == 0)`) from impl.go as unreachable dead code: Leg 3 of
// runOrphanObligationGate already refuses whenever the plan is
// unreadable (planErr != nil), and it runs BEFORE every mutation this
// function performs (see the ordering contract atop this file — steps
// 6/7/9). TestApproveImpl_NoCommitsNoBeads above already pins that
// FinalizeEpic (MUTATION 3/3, terminal) never runs in this state; this
// test goes one step further and proves the OTHER two mutations in the
// contract — MUTATION (1/3) epic close via bd, and MUTATION (2/3) the
// phase-metadata write (both the done write AND the deferred Req-1
// reconcile) — also never fire, i.e. the refusal is genuinely
// PRE-MUTATION, not merely pre-FinalizeEpic. If Leg 3's `planErr != nil`
// check were bypassed, ApproveImpl would fall through to the reconcile
// write, the epic-close call, and (with a zero-commit cleanup path)
// FinalizeEpic — so bdCloseCalls/phaseWrites go nonzero and this test
// fails, which is the exact regression qb03's removal must never
// reintroduce.
func TestApproveImpl_MissingPlanRefusesBeforeAnyMutation(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	// Deliberately no plan.md.
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "closed"}}
		return json.Marshal(payload)
	}
	var bdCloseCalls []string
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "close" {
			bdCloseCalls = append(bdCloseCalls, args[1])
		}
		return []byte("ok"), nil
	}
	var phaseWrites []map[string]interface{}
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		phaseWrites = append(phaseWrites, updates)
		return nil
	}

	mock := &executor.MockExecutor{
		CommitCountResult: 0,
	}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected a refusal for a spec with no readable plan.md")
	}
	if !strings.Contains(err.Error(), "plan bead list could not be read") {
		t.Errorf("refusal must name the unreadable plan-bead enumeration (Leg 3): %v", err)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("refusal must end with a recovery line: %v", err)
	}
	if len(bdCloseCalls) != 0 {
		t.Errorf("MUTATION (1/3) epic close must not run before Leg 3's refusal; bd close calls: %v", bdCloseCalls)
	}
	if len(phaseWrites) != 0 {
		t.Errorf("MUTATION (2/3) phase-metadata write (done write or Req-1 reconcile) must not run before Leg 3's refusal; writes: %v", phaseWrites)
	}
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
		t.Errorf("MUTATION (3/3, terminal) FinalizeEpic must not run before Leg 3's refusal: %d calls", len(calls))
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
	// Spec 092 Reqs 8/12 (mindspec-tjat): the plan-bead gate failure
	// carries the worktree-context line and ends with a copy-pastable
	// recovery line naming the open bead. This is the per-site
	// recovery-convention test for this gate (Req 21 mirror — see
	// internal/guard/recovery_convention_test.go).
	msg := err.Error()
	wantCtx := "you are in the main worktree (/testcwd); this check evaluated " + tmp
	if !strings.Contains(msg, wantCtx) {
		t.Errorf("plan-bead gate failure missing context line %q: %v", wantCtx, msg)
	}
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("plan-bead gate failure must end with a recovery line (Req 12/21): %v", msg)
	}
	lines := strings.Split(msg, "\n")
	if got, want := lines[len(lines)-1], "recovery: mindspec complete bead-bbb"; got != want {
		t.Errorf("final recovery line = %q, want %q", got, want)
	}
	if !strings.HasPrefix(lines[len(lines)-2], "you are in the ") {
		t.Errorf("context line must immediately precede the final recovery line: %v", msg)
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
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
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
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
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
// the EIGHT anchored call expressions inside ApproveImpl appear in
// strict source order. Per Spec 086 panel CONSENSUS revision 9 plus
// the Spec 092 Req 1 reconcile placement, the contract is:
//
//  1. readBeadStatus            (bead-status loop)
//  2. validate.ValidateDocs     (doc-sync gate)
//  3. validate.CheckADRDivergence (ADR-divergence gate — LAST
//     pre-terminal gate)
//  4. implPhaseMetadataFn with "mindspec_phase" but WITHOUT
//     "mindspec_done" (Spec 092 Req 1 deferred stale-phase
//     reconcile: after the last pre-terminal gate, before the epic
//     close, and NEVER after the done write — a placement after the
//     done write would clobber `done` with the derived `review`)
//  5. implRunBDCombinedFn("close", ...)  (EPIC CLOSE)
//  6. implPhaseMetadataFn with "mindspec_done" literal (phase=done
//     write — supersedes the reconcile)
//  7. exec.CommitCount          (pre-flight; NOT a pre-terminal gate
//     for the reconcile — placement pinned by Spec 086 rev 9)
//  8. exec.FinalizeEpic         (terminal mutation)
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
		{label: "validate.ValidateDocsRange", match: func(c *ast.CallExpr) bool {
			// Spec 095: impl-approve's doc-sync is now the explicit
			// base..specBranch RANGE form (was ValidateDocs working-tree).
			return isSelectorCall(c.Fun, "validate", "ValidateDocsRange")
		}},
		{label: "validate.CheckADRDivergence", match: func(c *ast.CallExpr) bool {
			return isSelectorCall(c.Fun, "validate", "CheckADRDivergence")
		}},
		// Spec 115 Bead 2: the pre-terminal orphan/obligation refusal
		// gate — runOrphanObligationGate wraps implScanOrphansFn, which
		// defaults to lifecycle.ScanOrphanedClosedBeads (the R1 error-
		// preserving core). It must sit after the last read-only gate
		// (CheckADRDivergence above) and BEFORE the deferred phase-
		// reconcile write, MUTATION (1/3), and FinalizeEpic below.
		{label: "runOrphanObligationGate (Spec 115 Bead 2 — wraps lifecycle.ScanOrphanedClosedBeads)", match: func(c *ast.CallExpr) bool {
			id, ok := c.Fun.(*ast.Ident)
			return ok && id.Name == "runOrphanObligationGate"
		}},
		{label: "implPhaseMetadataFn(reconcile, mindspec_phase only)", match: func(c *ast.CallExpr) bool {
			id, ok := c.Fun.(*ast.Ident)
			if !ok || id.Name != "implPhaseMetadataFn" || len(c.Args) < 2 {
				return false
			}
			return callMapHasKey(c.Args[1], "mindspec_phase") && !callMapHasKey(c.Args[1], "mindspec_done")
		}},
		{label: "implRunBDCombinedFn(\"close\")", match: func(c *ast.CallExpr) bool {
			id, ok := c.Fun.(*ast.Ident)
			if !ok || id.Name != "implRunBDCombinedFn" || len(c.Args) == 0 {
				return false
			}
			return firstArgStringLit(c) == "close"
		}},
		{label: "implPhaseMetadataFn(mindspec_phase=done)", match: func(c *ast.CallExpr) bool {
			id, ok := c.Fun.(*ast.Ident)
			if !ok || id.Name != "implPhaseMetadataFn" || len(c.Args) < 2 {
				return false
			}
			return callMapHasKey(c.Args[1], "mindspec_done")
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

// TestApproveImplCallOrder_OrphanGatePrecedesSupersedePlaceholder pins
// Spec 119 Bead 3's Step-3 preflight restructure (R1): the Spec 115
// orphan/obligation gate (runOrphanObligationGate) must run BEFORE the
// --supersede-adr placeholder-ADR file write (implCreateWithIDFn) — today
// that file write used to precede a derivable refusal, violating "facts
// before mutation". A derivable refusal must never leave a placeholder ADR
// file on disk.
func TestApproveImplCallOrder_OrphanGatePrecedesSupersedePlaceholder(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "impl.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse impl.go: %v", err)
	}
	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if ok && fd.Name.Name == "ApproveImpl" {
			fn = fd
			break
		}
	}
	if fn == nil {
		t.Fatal("ApproveImpl FuncDecl not found")
	}

	var orphanGatePos, supersedePos token.Pos
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok {
			switch id.Name {
			case "runOrphanObligationGate":
				if orphanGatePos == 0 {
					orphanGatePos = call.Pos()
				}
			case "implCreateWithIDFn":
				if supersedePos == 0 {
					supersedePos = call.Pos()
				}
			}
		}
		return true
	})

	if orphanGatePos == 0 {
		t.Fatal("runOrphanObligationGate call not found in ApproveImpl body")
	}
	if supersedePos == 0 {
		t.Fatal("implCreateWithIDFn call not found in ApproveImpl body")
	}
	if !(orphanGatePos < supersedePos) {
		t.Errorf("runOrphanObligationGate (pos %d) must precede the --supersede-adr placeholder write implCreateWithIDFn (pos %d)", orphanGatePos, supersedePos)
	}
}

// TestApproveImpl_LifecycleClassificationErrorRefusesPreMutation pins
// AC-14: a phase.LifecycleChildIDsForEpic query failure (Step 1/2, the
// FinalizeEpic allow-set's classification leg) must refuse in ApproveImpl's
// preflight — BEFORE any mutation — with a named error. The third `bd list
// --parent` call is the ONE this bead adds (DerivePhaseDetail's own
// children query is #1, the Spec 095 advisory
// OpenNonLifecycleChildrenForEpic guard hint is #2 and swallows errors to
// nil); only failing call #3 isolates LifecycleChildIDsForEpic specifically.
func TestApproveImpl_LifecycleClassificationErrorRefusesPreMutation(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "closed"}}
		return json.Marshal(payload)
	}
	var combinedCalls []string
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 {
			combinedCalls = append(combinedCalls, args[0])
		}
		return []byte("ok"), nil
	}
	var createCalls int
	implCreateWithIDFn = func(root, id, title string, opts adr.CreateOpts) (string, error) {
		createCalls++
		return "", nil
	}
	var phaseMetaCalls int
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		phaseMetaCalls++
		return nil
	}

	parentCalls := 0
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
			parentCalls++
			if parentCalls <= 2 {
				children := []phase.ChildInfo{{ID: "bead-1", Status: "closed", IssueType: "task"}}
				return json.Marshal(children)
			}
			return nil, fmt.Errorf("simulated bd list --parent failure")
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)

	mock := &executor.MockExecutor{CommitCountResult: 5}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected an error when LifecycleChildIDsForEpic fails, got nil")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("refusal must end with a recovery line: %v", err)
	}

	// Zero mutating calls anywhere: no epic close, no phase-metadata
	// write, no supersede-placeholder ADR write, no executor mutation.
	if len(combinedCalls) != 0 {
		t.Errorf("implRunBDCombinedFn must not be called on a preflight refusal; got %v", combinedCalls)
	}
	if createCalls != 0 {
		t.Errorf("implCreateWithIDFn must not be called on a preflight refusal; got %d calls", createCalls)
	}
	if phaseMetaCalls != 0 {
		t.Errorf("implPhaseMetadataFn must not be called on a preflight refusal; got %d calls", phaseMetaCalls)
	}
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
		t.Errorf("FinalizeEpic must not be called on a preflight refusal; got %d calls", len(calls))
	}
}

// TestApproveImpl_FinalizeEpicReceivesIntersectedAllowSet pins AC-13's
// approve-side half end-to-end: the lifecycleAllowSet handed to
// exec.FinalizeEpic must be EXACTLY planDeclared(specID) ∩
// lifecycleChildren(epicID) — never parent-membership alone. The epic
// here has THREE children: bead-1 (task, plan-declared → IN), bead-2
// (bug, plan-declared but non-lifecycle → OUT despite being plan-
// declared), and bead-3 (task, lifecycle but NOT plan-declared → OUT
// despite being lifecycle-classified). Only bead-1 must appear in the
// passed set.
func TestApproveImpl_FinalizeEpicReceivesIntersectedAllowSet(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1", "bead-2"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		payload := []map[string]string{{"status": "closed"}}
		return json.Marshal(payload)
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

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
			children := []phase.ChildInfo{
				{ID: "bead-1", Status: "closed", IssueType: "task"},
				{ID: "bead-2", Status: "open", IssueType: "bug"},
				{ID: "bead-3", Status: "closed", IssueType: "task"},
			}
			return json.Marshal(children)
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}

	if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := mock.CallsTo("FinalizeEpic")
	if len(calls) != 1 {
		t.Fatalf("expected 1 FinalizeEpic call, got %d", len(calls))
	}
	if len(calls[0].Args) != 4 {
		t.Fatalf("FinalizeEpic call recorded %d args, want 4 (epicID, specID, specBranch, lifecycleAllowSet)", len(calls[0].Args))
	}
	allowSet, ok := calls[0].Args[3].([]string)
	if !ok {
		t.Fatalf("FinalizeEpic 4th arg = %T, want []string", calls[0].Args[3])
	}
	if len(allowSet) != 1 || allowSet[0] != "bead-1" {
		t.Errorf("lifecycleAllowSet = %v, want exactly [bead-1] (bead-2 is non-lifecycle, bead-3 is not plan-declared)", allowSet)
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

// writeWidgetAcceptedFixture builds an approve-side fixture under the
// canonical .mindspec/docs tree: spec.md declares "widget" impacted,
// plan.md cites Accepted ADR-0195, and ADR-0195 is on disk Accepted for
// widget. The widget OWNERSHIP manifest is deliberately NOT written —
// callers decide whether it lives at the diffed ref (served via the
// mock) or on disk, to exercise the spec-095 ref-anchored read.
func writeWidgetAcceptedFixture(t *testing.T, root, specID string) {
	t.Helper()
	docsRoot := filepath.Join(root, ".mindspec", "docs")
	specDir := filepath.Join(docsRoot, "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Spec "+specID+"\n\n## Impacted Domains\n\n- widget\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"),
		[]byte("---\nspec_id: "+specID+"\nstatus: Approved\nbead_ids:\n  - bead-1\nadr_citations:\n  - id: ADR-0195\n---\n\n# Plan\n"), 0o644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}
	adrDir := filepath.Join(docsRoot, "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("mkdir adr dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-0195.md"),
		[]byte("# ADR-0195: Widget\n\n- **Date**: 2026-01-01\n- **Status**: Accepted\n- **Domain(s)**: widget\n- **Supersedes**: n/a\n- **Superseded-by**: n/a\n\n## Decision\nTest fixture.\n"), 0o644); err != nil {
		t.Fatalf("write ADR-0195.md: %v", err)
	}
}

// TestApproveImpl_WholeBranchOwnershipFromRef pins spec 095 for the
// impl-approve whole-branch backstop: its OWNERSHIP attribution is read
// from the spec-branch tip, not the ambient working tree. A claim
// committed on the branch satisfies the gate; a claim that exists ONLY
// on disk (absent at the ref) does NOT spuriously satisfy it.
func TestApproveImpl_WholeBranchOwnershipFromRef(t *testing.T) {
	const specID = "010-test"
	changed := []string{
		"internal/widget/foo.go",
		".mindspec/docs/domains/widget/OWNERSHIP.yaml",
	}

	t.Run("branch-tip claim passes", func(t *testing.T) {
		tmp := t.TempDir()
		writeWidgetAcceptedFixture(t, tmp, specID)
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755)
		saveAndRestore(t)
		implRunBDFn = func(args ...string) ([]byte, error) {
			return json.Marshal([]map[string]string{{"status": "closed"}})
		}
		mock := &executor.MockExecutor{
			CommitCountResult:  5,
			FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
			MergeBaseResult:    "merge-base-sha",
			ChangedFilesResult: changed,
			// Claim lives only at the diffed ref (spec-branch tip).
			FileAtRefOrAbsentFn: func(_ref, p string) ([]byte, bool, error) {
				if p == ".mindspec/docs/domains/widget/OWNERSHIP.yaml" {
					return []byte("paths:\n  - internal/widget/**\n"), true, nil
				}
				return nil, false, nil
			},
			TreeDirsAtRefFn: func(_ref, dir string) ([]string, error) {
				if dir == ".mindspec/docs/domains" {
					return []string{"widget"}, nil
				}
				return nil, nil
			},
		}
		if _, err := ApproveImpl(tmp, specID, mock); err != nil {
			t.Fatalf("a branch-tip OWNERSHIP claim must satisfy the whole-branch gate with no override; got: %v", err)
		}
		if len(mock.CallsTo("FinalizeEpic")) != 1 {
			t.Error("expected FinalizeEpic to run when the gates pass")
		}
	})

	t.Run("root-only claim absent at ref does NOT spuriously pass", func(t *testing.T) {
		tmp := t.TempDir()
		writeWidgetAcceptedFixture(t, tmp, specID)
		// Claim present ON DISK only — absent at the ref (mock returns
		// absent by default).
		if err := os.MkdirAll(filepath.Join(tmp, ".mindspec", "docs", "domains", "widget"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, ".mindspec", "docs", "domains", "widget", "OWNERSHIP.yaml"),
			[]byte("paths:\n  - internal/widget/**\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		saveAndRestore(t)
		implRunBDFn = func(args ...string) ([]byte, error) {
			return json.Marshal([]map[string]string{{"status": "closed"}})
		}
		mock := &executor.MockExecutor{
			CommitCountResult:  5,
			FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
			MergeBaseResult:    "merge-base-sha",
			ChangedFilesResult: changed,
			// ref reads: absent everywhere (default zero values).
		}
		_, err := ApproveImpl(tmp, specID, mock, ImplOpts{AllowDocSkew: "isolate the ADR lane"})
		if err == nil {
			t.Fatal("a claim absent at the diffed ref must NOT satisfy the gate (read follows the ref)")
		}
		if !strings.Contains(err.Error(), "adr-divergence-unowned") {
			t.Errorf("expected adr-divergence-unowned block, got: %v", err)
		}
		if len(mock.CallsTo("FinalizeEpic")) != 0 {
			t.Error("FinalizeEpic must not run when the gate blocks")
		}
	})
}

// TestApproveImpl_DocSyncUsesBaseToSpecBranchRange pins the spec 095
// correction: impl-approve's doc-sync diffs the explicit base..specBranch
// RANGE, not working-tree-vs-base. Every ChangedFiles call the gates
// make must be (merge-base, spec/<id>); none may use the working-tree
// idiom ChangedFiles("", base). RED-on-revert: restoring
// ValidateDocs(root, base, exec) reintroduces a ChangedFiles("", base)
// call and trips the assertion.
func TestApproveImpl_DocSyncUsesBaseToSpecBranchRange(t *testing.T) {
	const specID = "010-test"
	tmp := t.TempDir()
	writeWidgetAcceptedFixture(t, tmp, specID)
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755)
	saveAndRestore(t)
	implRunBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]map[string]string{{"status": "closed"}})
	}
	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
		MergeBaseResult:    "merge-base-sha",
		// Empty diff → both gate lanes no-op; we only assert the RANGE.
		ChangedFilesFn: func(base, head string) ([]string, error) { return nil, nil },
	}
	if _, err := ApproveImpl(tmp, specID, mock); err != nil {
		t.Fatalf("empty-diff approve should succeed, got: %v", err)
	}
	calls := mock.CallsTo("ChangedFiles")
	if len(calls) < 1 {
		t.Fatal("expected at least one ChangedFiles call from the gates")
	}
	for _, c := range calls {
		if c.Args[0] != "merge-base-sha" || c.Args[1] != "spec/"+specID {
			t.Errorf("ChangedFiles(%v, %v): impl-approve gates must diff merge-base..spec/%s, never the working-tree idiom", c.Args[0], c.Args[1], specID)
		}
	}
}

// writeProposedADRFixtureImpl builds an approve-side fixture where the
// only coverage for the touched domain rests on a cited PROPOSED ADR:
// spec.md declares "core" impacted, plan.md cites ADR-9002, and
// ADR-9002 is on disk with Status Proposed and Domain core.
func writeProposedADRFixtureImpl(t *testing.T, root, specID string) {
	t.Helper()

	// Spec 091 Req 13 removed the silent "internal/<domain>/**" loader
	// fallback, so the touched source file internal/core/foo.go is now
	// "unowned" unless a domain manifest claims it — which would
	// short-circuit the ADR-divergence lane at adr-divergence-unowned
	// BEFORE the Proposed-coverage check this fixture exists to
	// exercise. The manifest loader is canonical-only
	// (.mindspec/docs/domains/<domain>/OWNERSHIP.yaml), and creating it
	// makes CanonicalDocsDir exist, which flips DocsDir/ADRDir/SpecDir
	// off the legacy `docs/` tree onto `.mindspec/docs/`. So the WHOLE
	// fixture lives under the canonical tree for consistency, otherwise
	// the ADR/spec/plan written under legacy `docs/` would no longer be
	// found and the lane would report uncovered instead of Proposed.
	docsRoot := filepath.Join(root, ".mindspec", "docs")

	specDir := filepath.Join(docsRoot, "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Spec "+specID+"\n\n## Impacted Domains\n\n- core\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	planMD := "---\nspec_id: " + specID + "\nstatus: Approved\nbead_ids:\n  - bead-1\nadr_citations:\n  - id: ADR-9002\n---\n\n# Plan\n"
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(planMD), 0o644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}

	adrDir := filepath.Join(docsRoot, "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("mkdir adr dir: %v", err)
	}
	adrMD := "# ADR-9002: Proposed core decision\n\n" +
		"- **Date**: 2026-01-01\n" +
		"- **Status**: Proposed\n" +
		"- **Domain(s)**: core\n" +
		"- **Supersedes**: n/a\n" +
		"- **Superseded-by**: n/a\n\n" +
		"## Decision\nTest fixture.\n"
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-9002.md"), []byte(adrMD), 0o644); err != nil {
		t.Fatalf("write ADR-9002.md: %v", err)
	}

	coreDomainDir := filepath.Join(docsRoot, "domains", "core")
	if err := os.MkdirAll(coreDomainDir, 0o755); err != nil {
		t.Fatalf("mkdir core domain dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(coreDomainDir, "OWNERSHIP.yaml"),
		[]byte("paths:\n  - internal/core/**\n"), 0o644); err != nil {
		t.Fatalf("write core OWNERSHIP.yaml: %v", err)
	}
}

// TestApproveImplProposedCoverageBlocks — panel condition C1 on
// mindspec-53qx: the impl-approve ADR-divergence backstop must ERROR
// when coverage of a touched domain rests on a still-Proposed cited
// ADR, naming the ADR and pointing at --override-adr. This is the gate
// that closes the lifecycle loop the plan-time Proposed tolerance
// opens.
func TestApproveImplProposedCoverageBlocks(t *testing.T) {
	tmp := t.TempDir()
	writeProposedADRFixtureImpl(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]map[string]string{{"status": "closed"}})
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
		MergeBaseResult:    "merge-base-sha",
		// Source touch attributed to "core" via the OWNERSHIP manifest.
		ChangedFilesResult: []string{"internal/core/foo.go"},
	}
	// Spec 095: impl-approve reads OWNERSHIP from the spec-branch ref;
	// serve that ref tree from the on-disk fixture.
	serveRefFromDisk(mock, tmp)

	_, err := ApproveImpl(tmp, "010-test", mock, ImplOpts{
		AllowDocSkew: "test setup",
	})
	if err == nil {
		t.Fatal("expected impl approve to fail on Proposed-only coverage")
	}
	for _, want := range []string{"adr-divergence", "ADR-9002", "now that the implementation ships", "--override-adr"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
	if len(mock.CallsTo("FinalizeEpic")) != 0 {
		t.Error("FinalizeEpic must not run when the Proposed-coverage gate fails")
	}
}

// TestApproveImplProposedCoverageOverride — the existing --override-adr
// escape bypasses the Proposed-coverage error exactly like other
// ADR-divergence failures, with the recorded-reason metadata flow
// already pinned by TestApproveImplOverrideMetadataGoesThroughSeam.
func TestApproveImplProposedCoverageOverride(t *testing.T) {
	tmp := t.TempDir()
	writeProposedADRFixtureImpl(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755)

	saveAndRestore(t)

	implRunBDFn = func(args ...string) ([]byte, error) {
		return json.Marshal([]map[string]string{{"status": "closed"}})
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
		MergeBaseResult:    "merge-base-sha",
		ChangedFilesResult: []string{"internal/core/foo.go"},
	}

	_, err := ApproveImpl(tmp, "010-test", mock, ImplOpts{
		AllowDocSkew: "test setup",
		OverrideADR:  "ADR-9002 acceptance tracked in followup bead",
	})
	if err != nil {
		t.Fatalf("--override-adr should bypass the Proposed-coverage gate, got: %v", err)
	}
	if len(mock.CallsTo("FinalizeEpic")) != 1 {
		t.Errorf("expected FinalizeEpic to run under override, got %d calls", len(mock.CallsTo("FinalizeEpic")))
	}
}

// --- Spec 092 Bead 3 (Req 1, 2): stale-phase reconcile tests ---

// stubPhaseStoredChildren sets up the phase package stubs with an epic
// "epic-parent" for spec "010-test" carrying the given stored
// mindspec_phase metadata value ("" = key absent) and the given
// children. Unrelated metadata (mindspec_migrated_at) is always
// present so preservation can be observed.
func stubPhaseStoredChildren(t *testing.T, stored string, children []phase.ChildInfo) {
	t.Helper()
	meta := map[string]interface{}{
		"spec_num":             float64(10),
		"spec_title":           "test",
		"mindspec_migrated_at": "2026-01-01T00:00:00Z",
	}
	if stored != "" {
		meta["mindspec_phase"] = stored
	}
	epics := []phase.EpicInfo{{
		ID: "epic-parent", Title: "[SPEC 010-test] Test", Status: "open",
		IssueType: "epic", Metadata: meta,
	}}
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return json.Marshal(epics)
			}
		}
		if contains(args, "--parent") {
			return json.Marshal(children)
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == "epic-parent" {
			return json.Marshal(epics)
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)
}

// captureStderr redirects os.Stderr for the duration of fn and returns
// everything written to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	defer func() {
		os.Stderr = orig
	}()
	fn()
	w.Close()
	os.Stderr = orig
	return <-done
}

// TestApproveImpl_StalePhaseReconcilesForward is the spec AC "3smk
// unit" success path: stored mindspec_phase=implement, all children
// closed (child-derived = review). ApproveImpl must succeed, write the
// phase forward via the merge seam ONLY after all pre-terminal gates
// pass (i.e. before the epic close, after the ADR gate), emit the
// lifecycle.phase_reconciled stderr event, and end with stored
// mindspec_phase exactly "done" (the :206-equivalent done write runs
// after, and supersedes, the reconcile — a placement that clobbered
// done with review would fail the end-state assertion).
func TestApproveImpl_StalePhaseReconcilesForward(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)
	stubPhaseStoredChildren(t, "implement", []phase.ChildInfo{
		{ID: "bead-1", Status: "closed", IssueType: "task"},
	})

	// Simulated epic metadata store: the seam emulates the
	// read-merge-write semantics of bead.MergeMetadata.
	store := map[string]interface{}{
		"mindspec_phase":       "implement",
		"mindspec_migrated_at": "2026-01-01T00:00:00Z",
	}
	var events []string
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		if id != "epic-parent" {
			t.Errorf("phase metadata write targeted %q, want epic-parent", id)
		}
		if _, done := updates["mindspec_done"]; done {
			events = append(events, "done-write")
		} else {
			events = append(events, fmt.Sprintf("reconcile-write:%v", updates["mindspec_phase"]))
		}
		for k, v := range updates {
			store[k] = v
		}
		return nil
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "close" {
			events = append(events, "epic-close")
		}
		return []byte("ok"), nil
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}

	var result *ImplResult
	var err error
	stderr := captureStderr(t, func() {
		result, err = ApproveImpl(tmp, "010-test", mock)
	})
	if err != nil {
		t.Fatalf("stale-phase approve should self-heal, got: %v", err)
	}
	if result == nil || result.SpecID != "010-test" {
		t.Fatalf("unexpected result: %+v", result)
	}

	// Ordering: reconcile write (review) → epic close → done write.
	want := []string{"reconcile-write:review", "epic-close", "done-write"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}

	// End-state assertion: stored phase is exactly "done" — the done
	// write superseded the reconcile.
	if got := store["mindspec_phase"]; got != "done" {
		t.Errorf("end-state mindspec_phase = %v, want done (reconcile must not clobber the done write)", got)
	}
	// Merge semantics: unrelated key survived every write.
	if got := store["mindspec_migrated_at"]; got != "2026-01-01T00:00:00Z" {
		t.Errorf("mindspec_migrated_at = %v, want preserved", got)
	}

	// Structured self-heal event (HC-3): one stderr line.
	wantEvent := "event=lifecycle.phase_reconciled spec=010-test epic=epic-parent stored=implement derived=review"
	if !strings.Contains(stderr, wantEvent) {
		t.Errorf("stderr missing %q; stderr=%q", wantEvent, stderr)
	}
	// No emitted output may contain a raw bd metadata-update command.
	if strings.Contains(stderr, "bd update --metadata") {
		t.Errorf("stderr contains banned raw metadata command: %q", stderr)
	}
}

// TestApproveImpl_OpenBugChildReachesReview is the spec 095 ry73 e2e
// guarantee: stored mindspec_phase=="implement" (a bug was filed as a
// child of the spec epic AFTER the last `complete`, so the cached phase
// is stale) while every LIFECYCLE (task) child is closed. The child-
// derived phase is `review` (the open bug is non-lifecycle and ignored),
// so ApproveImpl must proceed via the spec-092 derived-branch reconcile
// with NO manual `repair phase`, AND emit the advisory guard hint naming
// the open bug — without bare-recommending the bk5t-buggy `--parent ""`.
// RED-on-revert: if DerivePhaseFromChildren counted the open bug, the
// derived phase would be `implement`, both phases would fail the gate,
// and ApproveImpl would error out (no reconcile, no hint).
func TestApproveImpl_OpenBugChildReachesReview(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)
	// Stored cache says implement; children = all task closed + open bug.
	stubPhaseStoredChildren(t, "implement", []phase.ChildInfo{
		{ID: "bead-1", Status: "closed", IssueType: "task"},
		{ID: "bug-9", Title: "follow-up crash", Status: "open", IssueType: "bug"},
	})

	reconciled := false
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		if p, ok := updates["mindspec_phase"]; ok && p == "review" {
			reconciled = true
		}
		return nil
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}

	var err error
	stderr := captureStderr(t, func() {
		_, err = ApproveImpl(tmp, "010-test", mock)
	})
	if err != nil {
		t.Fatalf("open bug child must not strand the spec short of review; got: %v", err)
	}
	if !reconciled {
		t.Errorf("expected a forward reconcile write to review (stored was stale 'implement')")
	}
	// Guard hint names the offending bug child.
	if !strings.Contains(stderr, "bug-9") {
		t.Errorf("guard hint must name the open non-lifecycle child bug-9; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "hint:") {
		t.Errorf("expected an advisory hint line; stderr=%q", stderr)
	}
	// Recovery line must NOT bare-recommend the bk5t-buggy detach.
	if strings.Contains(stderr, `--parent ""`) && !strings.Contains(stderr, "do NOT use") {
		t.Errorf("guard hint must not bare-recommend 'bd update <id> --parent \"\"'; stderr=%q", stderr)
	}
}

// TestApproveImpl_StoredFreshSkipsReconcile kills panel-R3 mutant M2a:
// when the STORED phase already satisfies the gate (review) while the
// child-derived phase disagrees (implement — e.g. a bead got reopened
// after review started), ApproveImpl proceeds WITHOUT any reconcile
// metadata write. The Req 1 reconcile fires ONLY when the stored phase
// fails the gate — an unconditional reconcile would be a spurious
// backward write (clobbering review with implement here).
func TestApproveImpl_StoredFreshSkipsReconcile(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)
	// stored=review passes the gate; one in_progress child derives
	// "implement", which disagrees with (and fails) the gate.
	stubPhaseStoredChildren(t, "review", []phase.ChildInfo{
		{ID: "bead-1", Status: "in_progress", IssueType: "task"},
	})

	var events []string
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		if _, done := updates["mindspec_done"]; done {
			events = append(events, "done-write")
		} else {
			events = append(events, fmt.Sprintf("reconcile-write:%v", updates["mindspec_phase"]))
		}
		return nil
	}
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "close" {
			events = append(events, "epic-close")
		}
		return []byte("ok"), nil
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}

	var err error
	stderr := captureStderr(t, func() {
		_, err = ApproveImpl(tmp, "010-test", mock)
	})
	if err != nil {
		t.Fatalf("fresh stored phase must approve without reconcile, got: %v", err)
	}

	// No mindspec_phase-without-done write happened anywhere — in
	// particular not before the epic close.
	for _, e := range events {
		if strings.HasPrefix(e, "reconcile-write") {
			t.Fatalf("reconcile write fired although the stored phase passed the gate: events=%v", events)
		}
	}
	want := []string{"epic-close", "done-write"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
	if strings.Contains(stderr, "event=lifecycle.phase_reconciled") {
		t.Errorf("phase_reconciled event emitted although no reconcile happened; stderr=%q", stderr)
	}
}

// TestImplPhaseMetadataFnDefaultsToBeadMergeMetadata kills panel-R3
// mutant M4a-2: the production binding of the phase-write seam MUST be
// bead.MergeMetadata (read-merge-write). A rebind to a raw replace
// path would silently wipe unrelated metadata keys (Req 19).
func TestImplPhaseMetadataFnDefaultsToBeadMergeMetadata(t *testing.T) {
	if reflect.ValueOf(implPhaseMetadataFn).Pointer() != reflect.ValueOf(bead.MergeMetadata).Pointer() {
		t.Fatal("implPhaseMetadataFn must default to bead.MergeMetadata (merge semantics, spec 092 Req 19)")
	}
}

// TestImplGetwdFnDefaultsToOsGetwd kills Bead 7 panel mutant M7: the
// Req 8 context-line seam MUST default to os.Getwd — every test swaps
// the seam in saveAndRestore, so a severed default would go undetected
// without this identity pin (recurring class; same pattern as the
// phase-write pin above).
func TestImplGetwdFnDefaultsToOsGetwd(t *testing.T) {
	if reflect.ValueOf(implGetwdFn).Pointer() != reflect.ValueOf(os.Getwd).Pointer() {
		t.Fatal("implGetwdFn must default to os.Getwd (spec 092 Req 8)")
	}
}

// TestApproveImpl_LaterGateFailureLeavesPhaseUntouched is the spec AC
// "3smk unit" deferred-write half: with a stale stored phase AND a
// failing later gate (doc-sync), the command errors and NO phase
// metadata write of any kind happens — the reconcile is deferred past
// the last pre-terminal gate, so a gate failure leaves metadata
// untouched (HC-4: exit non-zero having mutated nothing).
func TestApproveImpl_LaterGateFailureLeavesPhaseUntouched(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)
	stubPhaseStoredChildren(t, "implement", []phase.ChildInfo{
		{ID: "bead-1", Status: "closed", IssueType: "task"},
	})

	phaseWrites := 0
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		phaseWrites++
		return nil
	}
	closes := 0
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "close" {
			closes++
		}
		return []byte("ok"), nil
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
		MergeBaseResult:    "merge-base-sha",
		// spec.md-only change → doc-sync gate (a LATER pre-terminal
		// gate than the phase gate) fails.
		ChangedFilesResult: []string{".mindspec/docs/specs/010-test/spec.md"},
	}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected doc-sync gate failure")
	}
	if !strings.Contains(err.Error(), "doc-sync") {
		t.Errorf("error should mention doc-sync: %v", err)
	}
	if phaseWrites != 0 {
		t.Errorf("phase metadata writes = %d, want 0 — a later-gate failure must leave metadata untouched", phaseWrites)
	}
	if closes != 0 {
		t.Errorf("epic close calls = %d, want 0", closes)
	}
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
		t.Errorf("FinalizeEpic must not be called when a gate fails: got %d calls", len(calls))
	}
}

// TestApproveImpl_PhaseGateFailureNamesBothPhasesWithRecovery is the
// spec AC "3smk unit" fallback half (Req 2): when neither the stored
// nor the child-derived phase satisfies the gate, the error names both
// phases and ends with the exact spec-mandated recovery line. This is
// also the per-site recovery-convention test for the approve package
// (Req 21 mirror — see internal/guard/recovery_convention_test.go).
func TestApproveImpl_PhaseGateFailureNamesBothPhasesWithRecovery(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)
	// stored=plan, one child in_progress → derived=implement: both fail.
	stubPhaseStoredChildren(t, "plan", []phase.ChildInfo{
		{ID: "bead-1", Status: "in_progress", IssueType: "task"},
	})

	phaseWrites := 0
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		phaseWrites++
		return nil
	}

	mock := &executor.MockExecutor{}
	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected phase-gate failure when neither phase satisfies the gate")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"plan"`) || !strings.Contains(msg, `"implement"`) {
		t.Errorf("error must name both stored and derived phases: %v", msg)
	}
	wantRecovery := "recovery: close remaining beads with 'mindspec complete <bead-id>', or if bead states are already correct run: mindspec repair phase 010-test"
	if !strings.Contains(msg, wantRecovery) {
		t.Errorf("error missing the spec-mandated recovery line %q: %v", wantRecovery, msg)
	}
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("phase-gate failure must end with a recovery line (Req 12/21): %v", msg)
	}
	// Spec 092 Req 8 (mindspec-tjat): the failure carries the worktree-
	// context line — where the command ran (pinned /testcwd → main kind)
	// and the repo whose state the gate evaluated — preceding the final
	// recovery line (Req 12 ordering).
	wantCtx := "you are in the main worktree (/testcwd); this check evaluated " + tmp
	if !strings.Contains(msg, wantCtx) {
		t.Errorf("phase-gate failure missing context line %q: %v", wantCtx, msg)
	}
	lines := strings.Split(msg, "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[len(lines)-2], "you are in the ") {
		t.Errorf("context line must immediately precede the final recovery line: %v", msg)
	}
	if strings.Contains(msg, "bd update --metadata") {
		t.Errorf("emitted message contains banned raw metadata command (Req 19): %v", msg)
	}
	if phaseWrites != 0 {
		t.Errorf("gate failure must not write phase metadata; writes = %d", phaseWrites)
	}
}

// TestApproveImpl_ReconcileWriteFailureHasRecoveryLine: when the
// deferred reconcile write itself fails, the command exits non-zero
// BEFORE any mutation (no epic close, no FinalizeEpic) and the error
// ends with a recovery line (guard convention).
func TestApproveImpl_ReconcileWriteFailureHasRecoveryLine(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)
	stubPhaseStoredChildren(t, "implement", []phase.ChildInfo{
		{ID: "bead-1", Status: "closed", IssueType: "task"},
	})

	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		return fmt.Errorf("dolt offline")
	}
	closes := 0
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "close" {
			closes++
		}
		return []byte("ok"), nil
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected error when the reconcile write fails")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("reconcile-write failure must end with a recovery line: %v", err)
	}
	if !strings.Contains(err.Error(), "mindspec repair phase 010-test") {
		t.Errorf("recovery should name mindspec repair phase: %v", err)
	}
	if strings.Contains(err.Error(), "bd update --metadata") {
		t.Errorf("emitted message contains banned raw metadata command (Req 19): %v", err)
	}
	if closes != 0 {
		t.Errorf("epic close calls = %d, want 0 (HC-4: nothing mutated)", closes)
	}
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
		t.Errorf("FinalizeEpic must not run after a reconcile-write failure: %d calls", len(calls))
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
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
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

	// Spec 115 Bead 2: the plan.md this fixture now carries (for the
	// obligation-backstop leg) also switches ON the previously-dormant
	// ADR-divergence gate (it no-ops on a MISSING plan.md, pre-115); this
	// fixture declares no domain/ownership coverage for cmd/, so
	// --override-adr keeps that unrelated gate out of THIS test's way —
	// the doc-sync WARN rendering + FinalizeEpic-call assertions below
	// are what this test actually pins, unchanged.
	_, err := ApproveImpl(tmp, "010-test", mock, ImplOpts{OverrideADR: "adr-divergence coverage is out of scope for this doc-sync-warning fixture"})
	if err != nil {
		t.Fatalf("warning-only doc-sync result must not block approval, got: %v", err)
	}
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
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
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

// writeScenarioImplApproveFixture builds the on-disk shape of the harness
// ScenarioImplApprove fixture (internal/harness/scenario_spec_lifecycle.go):
// a committed `done.go` source at the repo root plus a spec/plan, WITHOUT
// the ADR-divergence coverage triple. The plan cites no ADR. The OWNERSHIP
// + ADR-0001 + plan adr_citations triple is layered on top per arm via the
// ref-read seams / plan rewrite so the test can flip CheckADRDivergence
// between block (unowned) and pass.
func writeScenarioImplApproveFixture(t *testing.T, root, specID string, withADRCitation bool) {
	t.Helper()
	docsRoot := filepath.Join(root, ".mindspec", "docs")
	specDir := filepath.Join(docsRoot, "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Spec "+specID+"\n\n## Impacted Domains\n\n- sandbox\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	citation := ""
	if withADRCitation {
		citation = "adr_citations:\n  - id: ADR-0001\n"
	}
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"),
		[]byte("---\nspec_id: "+specID+"\nstatus: Approved\nbead_ids:\n  - bead-1\n"+citation+"---\n\n# Plan\n## Bead 1: Implement feature\nCreate done.go.\n"), 0o644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}
}

// TestApproveImpl_ScenarioImplApproveCoverageTriple is the HERMETIC,
// CI-runnable RED pin for 098 Bead 1 (R1 / myn3): it proves that the
// ownership + Accepted-ADR-0001 + plan-`adr_citations` coverage triple is
// exactly what flips the impl-approve CheckADRDivergence gate from an
// `adr-divergence-unowned` block (the pre-fix RED state, where a clean
// impl-approve exits 1) to PASS (FinalizeEpic runs once). It mirrors the
// on-disk shape of the harness ScenarioImplApprove fixture (a committed
// `done.go` at the repo root + a spec/plan) but runs without any agent
// login or the skipUnlessClaudeCode gate, so it is the RED-on-revert pin
// that the login-gated LLM ScenarioImplApprove run cannot carry.
//
// RED-on-revert: removing the triple from the WITH arm (dropping the
// OWNERSHIP/ADR-0001 ref reads or the plan adr_citations) reproduces the
// WITHOUT arm's `adr-divergence-unowned` block.
func TestApproveImpl_ScenarioImplApproveCoverageTriple(t *testing.T) {
	// specID 010-test matches the hardwired phase stub in saveAndRestore
	// (stubPhaseForReview, title "[SPEC 010-test]"); the divergence gate
	// keys off the changed file (done.go) + its OWNERSHIP claim, not the ID.
	const specID = "010-test"
	// done.go lands at the repo root on main after the spec-branch merge —
	// exactly the changed-file shape the harness fixture commits.
	changed := []string{"done.go"}

	t.Run("WITHOUT the coverage triple -> adr-divergence-unowned, no FinalizeEpic", func(t *testing.T) {
		tmp := t.TempDir()
		writeScenarioImplApproveFixture(t, tmp, specID, false)
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755)
		saveAndRestore(t)
		implRunBDFn = func(args ...string) ([]byte, error) {
			return json.Marshal([]map[string]string{{"status": "closed"}})
		}
		mock := &executor.MockExecutor{
			CommitCountResult:  5,
			FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
			MergeBaseResult:    "merge-base-sha",
			ChangedFilesResult: changed,
			// No OWNERSHIP claim at the ref (default absent) -> done.go is
			// unowned -> the gate blocks.
		}
		_, err := ApproveImpl(tmp, specID, mock)
		if err == nil {
			t.Fatal("a committed root source with no ownership coverage must trip adr-divergence-unowned (the RED state)")
		}
		if !strings.Contains(err.Error(), "adr-divergence-unowned") {
			t.Errorf("expected adr-divergence-unowned block, got: %v", err)
		}
		if len(mock.CallsTo("FinalizeEpic")) != 0 {
			t.Error("FinalizeEpic must not run when the divergence gate blocks")
		}
	})

	t.Run("WITH the coverage triple -> passes, FinalizeEpic runs once", func(t *testing.T) {
		tmp := t.TempDir()
		writeScenarioImplApproveFixture(t, tmp, specID, true) // plan cites ADR-0001
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755)
		// Accepted ADR-0001 (sandbox domain) on disk — the ADR store reads
		// from the checkout (adrStoreForSpec overlays the spec dir), so the
		// cited covering ADR must be resolvable here. This is the same ADR
		// file writeSandboxDomainCoverage commits in the harness fixture.
		adrDir := filepath.Join(tmp, ".mindspec", "docs", "adr")
		if err := os.MkdirAll(adrDir, 0o755); err != nil {
			t.Fatalf("mkdir adr dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(adrDir, "ADR-0001-sandbox-domain.md"),
			[]byte("# ADR-0001: Sandbox Domain\n\n- **Date**: 2026-06-11\n- **Status**: Accepted\n- **Domain(s)**: sandbox\n\n## Decision\nFixture.\n"), 0o644); err != nil {
			t.Fatalf("write ADR-0001: %v", err)
		}
		saveAndRestore(t)
		implRunBDFn = func(args ...string) ([]byte, error) {
			return json.Marshal([]map[string]string{{"status": "closed"}})
		}
		mock := &executor.MockExecutor{
			CommitCountResult:  5,
			FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
			MergeBaseResult:    "merge-base-sha",
			ChangedFilesResult: changed,
			// The OWNERSHIP.yaml claim (claiming done.go for the sandbox
			// domain) is present at the diffed ref — the manifest half of
			// the triple committed pre-fork in the harness.
			FileAtRefOrAbsentFn: func(_ref, p string) ([]byte, bool, error) {
				if p == ".mindspec/docs/domains/sandbox/OWNERSHIP.yaml" {
					return []byte("paths:\n  - done.go\n"), true, nil
				}
				return nil, false, nil
			},
			TreeDirsAtRefFn: func(_ref, dir string) ([]string, error) {
				if dir == ".mindspec/docs/domains" {
					return []string{"sandbox"}, nil
				}
				return nil, nil
			},
		}
		if _, err := ApproveImpl(tmp, specID, mock); err != nil {
			t.Fatalf("the ownership + Accepted-ADR-0001 + plan adr_citations triple must satisfy the gate with no override; got: %v", err)
		}
		if len(mock.CallsTo("FinalizeEpic")) != 1 {
			t.Errorf("expected FinalizeEpic to run exactly once when the triple closes the gate; got %d", len(mock.CallsTo("FinalizeEpic")))
		}
	})
}

// TestReadPlanBeadIDsRejectsMalformed is spec 120 AC-25 (R2 class-2
// executable-operand consumer, round 6 G3): with a plan.md whose
// frontmatter carries a malformed bead_ids entry, the impl-approve path
// REFUSES before ANY bd invocation — asserted via the implRunBDFn seam
// recording ZERO calls (so neither readBeadStatus nor
// implCheckObligationsFn ever sees the value, closing the option-
// injection at the readBeadStatus call and the obligation-gate leg); the
// refusal names the plan-frontmatter lever, shows the hostile value
// escaped-only, and satisfies guard.HasFinalRecoveryLine; a well-formed
// dotted-child bead_ids list passes byte-identically to today.
func TestReadPlanBeadIDsRejectsMalformed(t *testing.T) {
	hostileBeadIDs := []string{"--help", "x;evil"}
	for _, hostile := range hostileBeadIDs {
		t.Run(hostile, func(t *testing.T) {
			tmp := t.TempDir()
			writeSpecDir(t, tmp, "010-test")
			writePlanWithBeads(t, tmp, "010-test", []string{hostile})
			os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

			saveAndRestore(t)

			var runBDCalls int
			implRunBDFn = func(args ...string) ([]byte, error) {
				runBDCalls++
				return nil, fmt.Errorf("unexpected bd invocation: %v", args)
			}

			mock := &executor.MockExecutor{
				CommitCountResult:  5,
				FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
			}

			_, err := ApproveImpl(tmp, "010-test", mock)
			if err == nil {
				t.Fatal("expected a refusal for a malformed plan.md bead_ids entry")
			}
			if runBDCalls != 0 {
				t.Errorf("expected ZERO bd invocations before the refusal, got %d", runBDCalls)
			}
			if !guard.HasFinalRecoveryLine(err.Error()) {
				t.Errorf("refusal must end with a recovery line, got: %v", err)
			}
			if !strings.Contains(err.Error(), "plan.md") || !strings.Contains(err.Error(), "bead_ids") {
				t.Errorf("refusal must name the plan-frontmatter lever, got: %v", err)
			}
			if strings.ContainsRune(err.Error(), 0x00) || strings.ContainsRune(err.Error(), 0x1b) {
				t.Errorf("refusal must not contain raw NUL/ESC bytes: %q", err.Error())
			}
			if len(mock.CallsTo("FinalizeEpic")) != 0 {
				t.Error("expected ZERO FinalizeEpic calls on the refusal")
			}
		})
	}

	// Well-formed dotted-child bead_ids pass byte-identically to today.
	t.Run("clean dotted-child bead_ids pass", func(t *testing.T) {
		tmp := t.TempDir()
		writeSpecDir(t, tmp, "010-test")
		writePlanWithBeads(t, tmp, "010-test", []string{"mindspec-9cyu.1"})
		os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

		saveAndRestore(t)
		implRunBDFn = func(args ...string) ([]byte, error) {
			if len(args) >= 2 && args[0] == "show" {
				payload := []map[string]string{{"status": "closed"}}
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
		if _, err := ApproveImpl(tmp, "010-test", mock); err != nil {
			t.Fatalf("clean dotted-child bead_ids must not refuse: %v", err)
		}
		if len(closed) != 1 || closed[0] != "epic-parent" {
			t.Errorf("expected to close epic-parent, got: %v", closed)
		}
	})
}
