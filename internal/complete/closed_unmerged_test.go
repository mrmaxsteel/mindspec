package complete

// Spec 092 Bead 6 — incident amendment (2026-06-11 merge-driver
// incident, panel j3-recurrence): complete.Run must PROPAGATE a
// CompleteBead failure as a non-zero exit with an explicit
// closed-but-unmerged recovery hint, instead of the old
// `Warning: bead cleanup: ...` warn-and-continue that exited 0 and
// hid the inconsistency.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// setupCompleteBeadFailure wires the minimal happy-path stubs up to the
// CompleteBead call, with the executor's CompleteBead failing with
// completeErr. Returns the mock and a pointer to the recorded
// mindspec_phase metadata writes (which must stay empty on failure).
func setupCompleteBeadFailure(t *testing.T, completeErr error) (root string, mock *executor.MockExecutor, phaseWrites *[]string) {
	t.Helper()
	saveAndRestore(t)

	root = setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "worktree-bead-1", Path: "/tmp/worktree-bead-1", Branch: "bead/bead-1"},
		}, nil
	}
	closeBeadFn = func(ids ...string) error { return nil }

	writes := []string{}
	phaseWrites = &writes
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if v, ok := updates["mindspec_phase"]; ok {
			writes = append(writes, fmt.Sprintf("%s=%v", id, v))
		}
		return nil
	}

	mock = newMockExec()
	mock.CompleteBeadErr = completeErr
	return root, mock, phaseWrites
}

// TestRun_CompleteBeadFailurePropagatesClosedButUnmerged: a CompleteBead
// failure that already carries Req 12 recovery lines (the executor's
// conflict-abort failures) surfaces as a non-zero Run error that names
// the closed-but-unmerged state and keeps the executor's recovery lines
// final.
func TestRun_CompleteBeadFailurePropagatesClosedButUnmerged(t *testing.T) {
	execErr := guard.NewFailure(
		"merge conflict: could not merge bead/bead-1 into spec/008-test\nconflicted files:\n  .beads/issues.jsonl",
		"cd /repo/.worktrees/worktree-spec-008-test",
		"git merge --no-ff bead/bead-1",
		"mindspec complete bead-1",
	)
	root, mock, phaseWrites := setupCompleteBeadFailure(t, execErr)

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected an error when CompleteBead fails (no warn-and-continue)")
	}
	if result != nil {
		t.Errorf("expected nil result on failure, got %+v", result)
	}
	msg := err.Error()

	// The closed-but-unmerged state is stated, not hidden.
	if !strings.Contains(msg, "CLOSED in Dolt") {
		t.Errorf("error must state the bead is already CLOSED in Dolt; got:\n%s", msg)
	}
	// Convergence is explicit: re-running complete after resolution.
	if !strings.Contains(msg, "mindspec complete bead-1") {
		t.Errorf("error must name the converging re-run command; got:\n%s", msg)
	}
	if !strings.Contains(msg, "idempotent") {
		t.Errorf("error must state the close step is idempotent (reconvergence); got:\n%s", msg)
	}
	// The executor's conflict detail (conflicted files) is preserved.
	if !strings.Contains(msg, ".beads/issues.jsonl") {
		t.Errorf("error must preserve the executor's conflicted-file detail; got:\n%s", msg)
	}
	// Req 12: per-site HasFinalRecoveryLine.
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("error must end with a `recovery:` line; got:\n%s", msg)
	}

	// No post-terminal mutations ran: the mindspec_phase sync (step 6.5)
	// must not have fired.
	if len(*phaseWrites) != 0 {
		t.Errorf("mindspec_phase must not be synced after a failed completion; got writes: %v", *phaseWrites)
	}
}

// TestRun_CompleteBeadPlainFailureGainsRecoveryLine: a CompleteBead
// failure WITHOUT recovery lines (e.g. the ancestor safety check) still
// surfaces non-zero, and Run appends the converging re-run as the
// final recovery line.
func TestRun_CompleteBeadPlainFailureGainsRecoveryLine(t *testing.T) {
	execErr := errors.New("bead branch bead/bead-1 is NOT merged into spec/008-test — aborting cleanup to prevent data loss")
	root, mock, _ := setupCompleteBeadFailure(t, execErr)

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected an error when CompleteBead fails")
	}
	msg := err.Error()
	if !strings.Contains(msg, "CLOSED in Dolt") {
		t.Errorf("error must state the bead is already CLOSED in Dolt; got:\n%s", msg)
	}
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("error must end with a `recovery:` line; got:\n%s", msg)
	}
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	if got := lines[len(lines)-1]; got != "recovery: mindspec complete bead-1" {
		t.Errorf("final recovery line = %q, want %q", got, "recovery: mindspec complete bead-1")
	}
}
