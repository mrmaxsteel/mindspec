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

// TestApproveImpl_AC5_SkippedWhenRemoteConfigured is bead-6 fix round 1's
// O1-1/O3-1 regression test: R3b/AC-5's own text scopes the §1 preflight
// to "the finalize merge" — the no-remote DIRECT spec→main merge. When a
// remote IS configured, exec.FinalizeEpic instead pushes specBranch for a
// PR and never local-merges into main, so the §1 check must not even be
// CONSULTED — proven here by stubbing implWorkDestructionPreflightFn to
// ALWAYS refuse (simulating a real DestructionSuperseded evaluation, as
// O3-1's real-git fixture reproduced for the routine "PR already merged,
// local main now reflects it" state) and asserting ApproveImpl still
// reaches FinalizeEpic and succeeds — the destructive stub answer is
// never given the chance to fire.
func TestApproveImpl_AC5_SkippedWhenRemoteConfigured(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0755)

	saveAndRestore(t)
	implRunBDCombinedFn = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	origHasRemote := implHasRemoteFn
	t.Cleanup(func() { implHasRemoteFn = origHasRemote })
	implHasRemoteFn = func() bool { return true }

	origPreflight := implWorkDestructionPreflightFn
	t.Cleanup(func() { implWorkDestructionPreflightFn = origPreflight })
	preflightCalled := false
	implWorkDestructionPreflightFn = func(workdir, branch, target, overrideReason, rerun string) error {
		preflightCalled = true
		return guard.NewFailure("this must never fire on the PR-routed leg", "unreachable")
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "pr", CommitCount: 5},
	}

	_, err := ApproveImpl(tmp, "010-test", mock)
	if err != nil {
		t.Fatalf("a PR-routed run (remote configured) must not be refused by the §1 preflight, got: %v", err)
	}
	if preflightCalled {
		t.Error("AC-5/O1-1: the §1 preflight must not be consulted at all when a remote is configured (the PR-routed leg never local-merges)")
	}
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 1 {
		t.Errorf("expected FinalizeEpic to be reached on the PR-routed leg, got %d call(s)", len(calls))
	}
}

// TestApproveImpl_AC5_StillAppliesWithNoRemoteConfigured is the paired
// regression guard for TestApproveImpl_AC5_SkippedWhenRemoteConfigured:
// narrowing the §1 preflight's applicability to the no-remote leg must
// not also disable it there — the ORIGINAL AC-5 refusal (a stale
// recreated spec branch on the direct-merge path) still fires.
func TestApproveImpl_AC5_StillAppliesWithNoRemoteConfigured(t *testing.T) {
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

	origHasRemote := implHasRemoteFn
	t.Cleanup(func() { implHasRemoteFn = origHasRemote })
	implHasRemoteFn = func() bool { return false }

	origPreflight := implWorkDestructionPreflightFn
	t.Cleanup(func() { implWorkDestructionPreflightFn = origPreflight })
	preflightCalled := false
	implWorkDestructionPreflightFn = func(workdir, branch, target, overrideReason, rerun string) error {
		preflightCalled = true
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
		t.Fatal("a stale recreated spec branch must still refuse the finalize merge when no remote is configured")
	}
	if !preflightCalled {
		t.Error("the §1 preflight must still be consulted on the no-remote direct-merge leg")
	}
	if closeCalled {
		t.Error("the §1 preflight refusal must fire BEFORE the epic close mutation")
	}
	if calls := mock.CallsTo("FinalizeEpic"); len(calls) != 0 {
		t.Errorf("FinalizeEpic must never be called on a §1 preflight refusal, got %d call(s)", len(calls))
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
