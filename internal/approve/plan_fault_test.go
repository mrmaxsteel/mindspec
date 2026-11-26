package approve

import (
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

const validTwoBeadPlan = `---
status: Draft
spec_id: "042-test"
version: "1.0"
work_chunks:
  - id: 1
    depends_on: []
  - id: 2
    depends_on: [1]
---

# Plan

## ADR Fitness

No ADRs are relevant to this work.

## Testing Strategy

Unit tests will verify the implementation.

## Bead 1: First thing

**Steps**
1. Step one
2. Step two
3. Step three

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Acceptance Criteria**
- First bead works

**Depends on**
None

## Bead 2: Second thing

**Steps**
1. Step one
2. Step two
3. Step three

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Acceptance Criteria**
- Second bead works

**Depends on**
Bead 1
`

func wirePlanEpicSeams(t *testing.T, specID, epicID string, queryFn func(args ...string) ([]byte, error)) {
	t.Helper()
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return epicJSONFor(epicID, specID), nil
			}
		}
		return []byte(`[]`), nil
	})
	t.Cleanup(restoreList)
	restoreRunBD := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "show" {
			return epicJSONFor(epicID, specID), nil
		}
		return []byte(`[]`), nil
	})
	t.Cleanup(restoreRunBD)
	restorePlanList := SetPlanListJSONForTest(queryFn)
	t.Cleanup(restorePlanList)

	origBranchExists := planBranchExistsInFn
	planBranchExistsInFn = func(workdir, branch string) (bool, error) { return false, nil }
	t.Cleanup(func() { planBranchExistsInFn = origBranchExists })
	origFindLanded := planFindLandedMergeFn
	planFindLandedMergeFn = func(workdir, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
		return nil, lifecycle.ErrLandedMergeNoCandidate
	}
	t.Cleanup(func() { planFindLandedMergeFn = origFindLanded })
}

func existingChildrenJSON(ids []string, closed map[string]bool) []byte {
	var parts []string
	for _, id := range ids {
		status := "open"
		extra := ""
		if closed[id] {
			status = "closed"
			extra = fmt.Sprintf(`,"close_reason":%q`, supersedeCloseReasonPrefix+"1")
		}
		parts = append(parts, fmt.Sprintf(`{"id":%q,"status":%q%s}`, id, status, extra))
	}
	return []byte("[" + strings.Join(parts, ",") + "]")
}

func TestFaultInjection_ApprovePlan_P0A_ForeignChildSurvivesKill(t *testing.T) {
	f := enrolFixture(t)
	foreign := kernel.Record{ID: "foreign-work", Parent: f.Epic, Type: "task", Status: "in_progress", Title: "foreign"}
	f.Fake.Records[foreign.ID] = foreign
	enrolKill(t, f, "creates")
	ids := enrolIDs(t, f)
	result, err := enrolApprove(f)
	f.Must(err)
	if !reflect.DeepEqual(result.BeadIDs, ids) || !reflect.DeepEqual(f.Fake.Records[foreign.ID], foreign) {
		t.Fatal("retry changed recorded items or foreign work")
	}
	if !f.State().Enrolled {
		t.Fatal("retry did not enrol")
	}

}

func TestFaultInjection_ApprovePlan_P0B_RecordedCreatesResume(t *testing.T) {
	f := enrolFixture(t)
	enrolKill(t, f, "creates")
	ids := enrolIDs(t, f)
	epic := f.Fake.Records[f.Epic]
	if _, ok := epic.Metadata[kernel.KeyPlanApproval]; ok {
		t.Fatal("approval preceded complete item recording")
	}
	result, err := enrolApprove(f)
	f.Must(err)
	if !reflect.DeepEqual(result.BeadIDs, ids) || !f.State().Enrolled {
		t.Fatal("retry did not reuse and enrol the recorded set")
	}

}

func TestFaultInjection_ApprovePlan_P1_BeadCreateFailure_KillThenConverge(t *testing.T) {
	f := enrolFixture(t)
	creates := 0
	f.Fake.Hook = func(args []string) error {
		if len(args) > 0 && args[0] == "create" {
			creates++
			if creates == 2 {
				return fmt.Errorf("create fault")
			}
		}
		return nil
	}
	if _, err := enrolApprove(f); err == nil {
		t.Fatal("fault did not fire")
	}
	ids := enrolIDs(t, f)
	if len(ids) != 1 {
		t.Fatalf("first item was not recorded immediately: %v", ids)
	}
	f.Fake.Hook = nil
	result, err := enrolApprove(f)
	f.Must(err)
	if len(result.BeadIDs) != 3 || result.BeadIDs[0] != ids[0] || !f.State().Enrolled {
		t.Fatal("retry duplicated or lost its first item")
	}

}

func TestFaultInjection_ApprovePlan_P2_PartialDepWiring_SimulatedDeathConverges(t *testing.T) {
	f := enrolFixture(t)
	fail := true
	f.Fake.Hook = func(args []string) error {
		if fail && len(args) > 1 && args[0] == "dep" && args[1] == "add" {
			fail = false
			return fmt.Errorf("link fault")
		}
		return nil
	}
	if _, err := enrolApprove(f); err == nil {
		t.Fatal("fault did not fire")
	}
	ids := enrolIDs(t, f)
	if _, ok := f.Fake.Records[f.Epic].Metadata[kernel.KeyRun]; ok {
		t.Fatal("link refusal wrote enrolment")
	}
	f.Fake.Hook = nil
	result, err := enrolApprove(f)
	f.Must(err)
	if !reflect.DeepEqual(ids, result.BeadIDs) || !f.State().Enrolled {
		t.Fatal("link retry changed items")
	}

}

func TestFaultInjection_ApprovePlan_P3_EnrolledRerunRefusesAfterCommitFault(t *testing.T) {
	f := enrolFixture(t)
	mock := &executor.MockExecutor{CommitAllErr: fmt.Errorf("commit fault")}
	if _, err := ApprovePlan(f.Root, f.SpecID, "operator", mock); err == nil || !strings.Contains(err.Error(), "commit fault") {
		t.Fatalf("commit fault: %v", err)
	}
	if !f.State().Enrolled {
		t.Fatal("enrolment lost after artifact commit failure")
	}
	before := enrolSnap(t, f)
	_, err := enrolApprove(f)
	if err == nil || !strings.Contains(err.Error(), "enrolment") || enrolSnap(t, f) != before {
		t.Fatalf("enrolled retry did not refuse without writes: %v", err)
	}

}

func TestFaultInjection_ApprovePlan_P2BP4_MetadataFaultIsHardError(t *testing.T) {
	f := enrolFixture(t)
	f.Fake.Hook = func(args []string) error {
		if len(args) > 0 && args[0] == "update" && slices.Contains(args, "--metadata") {
			return fmt.Errorf("metadata fault")
		}
		return nil
	}
	if _, err := enrolApprove(f); err == nil {
		t.Fatal("metadata write fault was swallowed")
	}
	if _, ok := f.Fake.Records[f.Epic].Metadata[kernel.KeyRun]; ok {
		t.Fatal("failed metadata write enrolled")
	}
	f.Fake.Hook = nil
	result, err := enrolApprove(f)
	f.Must(err)
	if len(result.BeadIDs) != 3 || !f.State().Enrolled {
		t.Fatal("metadata retry did not enrol")
	}

}

func planGitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git -C %s %v: %v\n%s", dir, args, err, out)
	}
}

func planGitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "commit", "-q", "-m", msg)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "nothing to commit") {
		t.Fatalf("git commit in %s: %v\n%s", dir, err, out)
	}
}

func planGitStatus(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git status in %s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

type killAfterPlanCommitExecutor struct {
	*executor.MockExecutor
	t          *testing.T
	calls      int
	killOnCall int
}

func (e *killAfterPlanCommitExecutor) CommitAll(dir, msg string) error {
	e.calls++
	planGitRun(e.t, dir, "add", "-A")
	planGitCommit(e.t, dir, msg)
	if e.calls == e.killOnCall {
		return fmt.Errorf("fault-injection: kill after CommitAll's real commit landed (call %d)", e.calls)
	}
	return nil
}
