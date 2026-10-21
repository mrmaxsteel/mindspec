package approve

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

func approvePreflightFixture(t *testing.T) (string, *executor.MockExecutor) {
	t.Helper()
	root := t.TempDir()
	writeSpecDir(t, root, "010-test")
	writePlanWithBeads(t, root, "010-test", []string{"bead-1"})
	saveAndRestore(t)
	stubPreflightPhase(t, "review")
	orig := implHasRemoteFn
	t.Cleanup(func() { implHasRemoteFn = orig })
	implHasRemoteFn = func() (bool, error) { return false, nil }
	return root, &executor.MockExecutor{CommitCountResult: 5}
}

func TestApproveImpl_AC5_StaleSpecBranchRefusesBeforeMutation(t *testing.T) {
	root, mock := approvePreflightFixture(t)
	mock.PreflightFinalizeMergesErr = guard.NewFailure("stale deletion", "inspect the branch, then retry with --allow-net-deletion")
	assertEarlyRefusal(t, root, mock, "stale deletion")
	assertPreflightRemote(t, mock, false)
}

func TestApproveImpl_AC5_StillAppliesWithNoRemoteConfigured(t *testing.T) {
	root, mock := approvePreflightFixture(t)
	mock.PreflightFinalizeMergesErr = guard.NewFailure("stale deletion", "inspect the branch")
	assertEarlyRefusal(t, root, mock, "stale deletion")
	assertPreflightRemote(t, mock, false)
}

func TestApproveImpl_AC5_PushPathPreflightsBeadMergesOnly(t *testing.T) {
	root, mock := approvePreflightFixture(t)
	implHasRemoteFn = func() (bool, error) { return true, nil }
	if _, err := ApproveImpl(root, "010-test", mock); err != nil {
		t.Fatal(err)
	}
	assertPreflightRemote(t, mock, true)
	if len(mock.CallsTo("PushProbe")) != 1 {
		t.Fatal("push path must probe once")
	}
}

func assertPreflightRemote(t *testing.T, mock *executor.MockExecutor, want bool) {
	t.Helper()
	calls := mock.CallsTo("PreflightFinalizeMerges")
	if len(calls) != 1 {
		t.Fatalf("preflight calls = %d, want 1", len(calls))
	}
	if got := calls[0].Args[5]; got != want {
		t.Fatalf("preflight remote = %v, want %v", got, want)
	}
}

func TestApproveImpl_AC7_AllowNetDeletionOverrideCompletes(t *testing.T) {
	root, mock := approvePreflightFixture(t)
	var recorded map[string]interface{}
	implMergeMetadataFn = func(_ string, updates map[string]interface{}) error { recorded = updates; return nil }
	const reason = "verified safe by inspection"
	if _, err := ApproveImpl(root, "010-test", mock, ImplOpts{AllowNetDeletion: reason}); err != nil {
		t.Fatal(err)
	}
	assertPreflightRemote(t, mock, false)
	if got := mock.CallsTo("PreflightFinalizeMerges")[0].Args[4]; got != reason {
		t.Fatalf("preflight override = %v", got)
	}
	calls := mock.CallsTo("FinalizeEpic")
	if len(calls) != 1 || len(calls[0].Args) != 7 {
		t.Fatalf("finalize calls = %+v", calls)
	}
	if calls[0].Args[4] != reason {
		t.Fatalf("finalize override = %v", calls[0].Args[4])
	}
	if recorded["mindspec_net_deletion_override_reason"] != reason {
		t.Fatalf("override metadata = %v", recorded)
	}
}
