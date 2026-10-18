package main

// Spec 128 final-review round 1 (R4/G5): `spec approve`'s success tail
// used to instruct "Commit approval artifacts before continuing" AFTER
// ApproveSpec had already auto-committed them and verified the spec
// worktree clean — a false post-state instruction on the exact lifecycle
// surface this spec makes truthful. This command-level regression pins
// the tail's honesty: the false instruction is gone, the actual result
// is stated, and the verb itself performed the approval commit. The
// clean-worktree/commit-present half runs against a REAL executor in
// internal/approve (TestApproveSpec_RealExecutorCommitsAndLeavesClean).

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

func TestSpecApprove_TailStatesCommittedResultNotFalseInstruction(t *testing.T) {
	root := verbOutputTestRepo(t)
	withTestChdir(t, root)
	specID := "996-tail-honesty"
	writeVerbFixtureFile(t, root, filepath.Join(".mindspec/specs", specID, "spec.md"), validVerbFixtureSpecMD(specID))
	runGitInDir(t, root, "add", "-A")
	runGitInDir(t, root, "commit", "-m", "spec-init "+specID)

	stubApproveBeadsForVerbTest(t)
	mock := &executor.MockExecutor{}
	withMockNewExecutor(t, mock)

	out, err := captureStdout(t, func() error {
		return specApproveCmd.RunE(specApproveCmd, []string{specID})
	})
	if err != nil {
		t.Fatalf("spec approve: %v\noutput:\n%s", err, out)
	}

	if strings.Contains(out, "Commit approval artifacts before continuing") {
		t.Errorf("the success tail still instructs committing artifacts the verb already committed:\n%s", out)
	}
	if !strings.Contains(out, "Approval artifacts committed") {
		t.Errorf("the success tail must state the actual result (artifacts committed), got:\n%s", out)
	}

	// The verb itself performed the approval commit — the tail's claim is
	// backed by a real CommitAll with the approval subject, not asserted
	// into thin air.
	sawApprovalCommit := false
	for _, c := range mock.CallsTo("CommitAll") {
		if len(c.Args) == 2 {
			if msg, ok := c.Args[1].(string); ok && strings.Contains(msg, "chore: approve spec "+specID) {
				sawApprovalCommit = true
			}
		}
	}
	if !sawApprovalCommit {
		t.Errorf("expected ApproveSpec to commit the approval artifacts itself (CommitAll with the approval subject), calls: %v", mock.CallsTo("CommitAll"))
	}
}
