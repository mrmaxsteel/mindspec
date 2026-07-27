package complete

// Spec 127 bead 6: the Run-side half of AC-7(i)/(ii) — the §1 preflight
// for CompleteBead's own bead→spec merge fires before the terminal
// mutation and honors the --allow-net-deletion override.
//
// These stub completeWorkDestructionPreflightFn directly (the in-package
// seam TestMain otherwise defaults to permissive — see main_test.go),
// isolating Run's ORDERING/wiring contract from gitutil.
// EvaluateWorkDestruction's own git plumbing.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// TestRun_AC7_DestructiveOutcomeRefusesBeforeCompleteBead pins that a
// destructive §1 outcome refuses BEFORE exec.CompleteBead is ever
// called — the merge-producer call itself never runs.
func TestRun_AC7_DestructiveOutcomeRefusesBeforeCompleteBead(t *testing.T) {
	saveAndRestore(t)
	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}

	completeWorkDestructionPreflightFn = func(workdir, branch, target, overrideReason, rerun string) error {
		if overrideReason != "" {
			return nil
		}
		return guard.NewFailure(
			fmt.Sprintf("refusing to merge %s into %s: staleness detected.", branch, target),
			fmt.Sprintf("inspect the branch, then re-run with --allow-net-deletion \"<reason>\" to proceed anyway — then %s", rerun),
		)
	}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("a destructive §1 outcome must refuse before the terminal mutation")
	}
	if !strings.Contains(err.Error(), "--allow-net-deletion") {
		t.Errorf("refusal must name --allow-net-deletion, got: %v", err)
	}
	if calls := mock.CallsTo("CompleteBead"); len(calls) != 0 {
		t.Errorf("AC-7: exec.CompleteBead must never be called on a §1 preflight refusal, got %d call(s)", len(calls))
	}
}

// TestRun_AC7_AllowNetDeletionOverrideThreadsToCompleteBead pins AC-7(ii)
// at the Run layer: the override reason reaches exec.CompleteBead's own
// overrideReason parameter, and the merge proceeds.
func TestRun_AC7_AllowNetDeletionOverrideThreadsToCompleteBead(t *testing.T) {
	saveAndRestore(t)
	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}
	stubChildrenByStatus(map[string][]bead.BeadInfo{
		"closed": {{ID: "bead-1", Title: "[IMPL 008-test.1] Done"}},
	})

	var recordedMeta map[string]interface{}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if id == "bead-1" {
			recordedMeta = updates
		}
		return nil
	}

	completeWorkDestructionPreflightFn = func(workdir, branch, target, overrideReason, rerun string) error {
		if overrideReason != "" {
			return nil
		}
		return fmt.Errorf("refusing to merge %s into %s — re-run with --allow-net-deletion", branch, target)
	}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{AllowNetDeletion: "verified safe by inspection"})
	if err != nil {
		t.Fatalf("--allow-net-deletion must let the merge proceed, got: %v", err)
	}
	calls := mock.CallsTo("CompleteBead")
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 CompleteBead call, got %d", len(calls))
	}
	if len(calls[0].Args) != 5 {
		t.Fatalf("CompleteBead call recorded %d args, want 5 (beadID, specBranch, msg, overrideReason, resolveMerge)", len(calls[0].Args))
	}
	if overrideArg, ok := calls[0].Args[3].(string); !ok || overrideArg != "verified safe by inspection" {
		t.Errorf("CompleteBead's overrideReason arg = %v, want %q", calls[0].Args[3], "verified safe by inspection")
	}
	if recordedMeta == nil {
		t.Fatal("expected mindspec_net_deletion_override_* metadata to be recorded on the bead")
	}
	if recordedMeta["mindspec_net_deletion_override_reason"] != "verified safe by inspection" {
		t.Errorf("recorded override reason = %v, want %q", recordedMeta["mindspec_net_deletion_override_reason"], "verified safe by inspection")
	}
}

// TestCompleteWorkDestructionPreflightFn_DeclaredDefaultIsRealImplementation
// mirrors internal/approve's identical pin: TestMain (main_test.go)
// installs a permissive stub for every test's duration, so a runtime
// pointer-identity check would always read the stub. This reads
// complete.go's own source text to pin the DECLARED default.
func TestCompleteWorkDestructionPreflightFn_DeclaredDefaultIsRealImplementation(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(".", "complete.go"))
	if err != nil {
		t.Fatalf("reading complete.go: %v", err)
	}
	if !strings.Contains(string(src), "completeWorkDestructionPreflightFn = lifecycle.EvaluateWorkDestructionPreflight") {
		t.Fatal("complete.go's declared default for completeWorkDestructionPreflightFn must be lifecycle.EvaluateWorkDestructionPreflight")
	}
}
