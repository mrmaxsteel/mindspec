package approve

// Spec 127 bead 6: AC-5 (R3b — stale recreated spec branch refused via
// the §1 preflight) and the ApproveImpl half of AC-7(i)/(ii) (the
// direct spec→main leg's own §1 evaluation, override-aware).
//
// These stub implWorkDestructionPreflightFn directly (the in-package
// seam), isolating ApproveImpl's ORDERING/wiring contract — "the §1
// check runs before any mutation, and its refusal/override decision is
// consulted" — from gitutil.EvaluateWorkDestruction's own git plumbing
// (unit-tested directly in internal/gitutil and internal/executor).

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// TestApproveImpl_AC5_StaleSpecBranchRefusesBeforeMutation is AC-5: a
// recreated-stale spec branch (the #218 step-2 shape, simulated here by
// forcing the §1 preflight to observe DestructionStaleDeletion) refuses
// the finalize merge BEFORE any mutation — no epic close (implRunBD
// CombinedFn never called), no FinalizeEpic call.
func TestApproveImpl_AC5_StaleSpecBranchRefusesBeforeMutation(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)

	closeCalled := false
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		closeCalled = true
		return []byte("ok"), nil
	}

	origPreflight := implWorkDestructionPreflightFn
	t.Cleanup(func() { implWorkDestructionPreflightFn = origPreflight })
	implWorkDestructionPreflightFn = func(workdir, branch, target, overrideReason, rerun string) error {
		if overrideReason != "" {
			return nil
		}
		return guard.NewFailure(
			fmt.Sprintf("refusing to merge %s into %s: staleness detected.", branch, target),
			fmt.Sprintf("inspect the branch, then re-run with %s \"<reason>\" to proceed anyway — then %s", executor.AllowNetDeletionFlag, rerun),
		)
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("a stale recreated spec branch must refuse the finalize merge before mutation")
	}
	if !strings.Contains(err.Error(), executor.AllowNetDeletionFlag) {
		t.Errorf("refusal must name %s, got: %v", executor.AllowNetDeletionFlag, err)
	}
	if closeCalled {
		t.Error("AC-5: the §1 preflight refusal must fire BEFORE the epic close mutation — implRunBDCombinedFn (bd close) must never be called")
	}
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
		t.Errorf("AC-5: FinalizeEpic must never be called on a §1 preflight refusal, got %d call(s)", len(calls))
	}
}

// TestApproveImpl_AC7_AllowNetDeletionOverrideCompletes is AC-7(ii)'s
// ApproveImpl-side half: the SAME destructive §1 outcome proceeds when
// --allow-net-deletion is supplied, reaching FinalizeEpic with the
// override reason threaded through, and records the override on the
// epic's metadata after FinalizeEpic returns nil.
func TestApproveImpl_AC7_AllowNetDeletionOverrideCompletes(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	var recordedMeta map[string]interface{}
	implMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if id == "epic-parent" {
			recordedMeta = updates
		}
		return nil
	}

	origPreflight := implWorkDestructionPreflightFn
	t.Cleanup(func() { implWorkDestructionPreflightFn = origPreflight })
	implWorkDestructionPreflightFn = func(workdir, branch, target, overrideReason, rerun string) error {
		if overrideReason != "" {
			return nil
		}
		return fmt.Errorf("refusing to merge %s into %s: staleness detected — re-run with %s", branch, target, executor.AllowNetDeletionFlag)
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}

	_, err := ApproveImpl(tmp, "010-test", mock, ImplOpts{AllowNetDeletion: "verified safe by inspection"})
	if err != nil {
		t.Fatalf("--allow-net-deletion must let the merge proceed, got: %v", err)
	}
	calls := mock.CallsTo("FinalizeEpic")
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 FinalizeEpic call, got %d", len(calls))
	}
	if len(calls[0].Args) != 6 {
		t.Fatalf("FinalizeEpic call recorded %d args, want 6 (epicID, specID, specBranch, lifecycleAllowSet, overrideReason, resolveMerge)", len(calls[0].Args))
	}
	if overrideArg, ok := calls[0].Args[4].(string); !ok || overrideArg != "verified safe by inspection" {
		t.Errorf("FinalizeEpic's overrideReason arg = %v, want %q", calls[0].Args[4], "verified safe by inspection")
	}
	if recordedMeta == nil {
		t.Fatal("expected mindspec_net_deletion_override_* metadata to be recorded on the epic")
	}
	if recordedMeta["mindspec_net_deletion_override_reason"] != "verified safe by inspection" {
		t.Errorf("recorded override reason = %v, want %q", recordedMeta["mindspec_net_deletion_override_reason"], "verified safe by inspection")
	}
}

// TestImplWorkDestructionPreflightFn_DeclaredDefaultIsRealImplementation
// pins the PRODUCTION default declared in impl.go's var block: this
// package's TestMain (main_test.go) installs a permissive stub for
// every test's duration (the same convention as planListJSONFn), so a
// runtime reflect.Pointer identity check here would always read the
// stub, never the production wiring. Reading impl.go's own source text
// instead proves the DECLARED default — the line every test's
// t.Cleanup ultimately restores back to — is still
// lifecycle.EvaluateWorkDestructionPreflight, not a stub that leaked
// into the source itself.
func TestImplWorkDestructionPreflightFn_DeclaredDefaultIsRealImplementation(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	implGoPath := filepath.Join(filepath.Dir(thisFile), "impl.go")
	src, err := os.ReadFile(implGoPath)
	if err != nil {
		t.Fatalf("reading %s: %v", implGoPath, err)
	}
	if !strings.Contains(string(src), "implWorkDestructionPreflightFn = lifecycle.EvaluateWorkDestructionPreflight") {
		t.Fatal("impl.go's declared default for implWorkDestructionPreflightFn must be lifecycle.EvaluateWorkDestructionPreflight")
	}
}
