package approve

import (
	"encoding/json"
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Spec 119 Bead 4 (R1 / P9 / P10): ApprovePlan's preflight resolves every
// plan-content and epic/child-set fact — and refuses FAIL-CLOSED on every
// derivable violation — BEFORE the first mutation. These tests pin that
// contract: each refusal case below leaves plan.md byte-identical to what it
// was before ApprovePlan ran, and records ZERO bd mutation calls (no
// `create`, no `dep add`, no `close`) and zero executor CommitAll calls.

// validSingleBeadPlan is a minimal, valid (passes ValidatePlan), single-bead,
// Draft-status plan with one aligned work_chunk. Individual preflight tests
// mutate the frontmatter or child-set fixtures around this base shape.
const validSingleBeadPlan = `---
status: Draft
spec_id: "042-test"
version: "1.0"
work_chunks:
  - id: 1
    depends_on: []
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
`

// setupPreflightPlan writes planContent as specID's plan.md (plus an empty
// spec.md) under a fresh flat-layout root, returning the paths and the plan's
// original bytes so callers can assert byte-identity after a refusal.
func setupPreflightPlan(t *testing.T, specID, planContent string) (root, planPath string, original []byte) {
	t.Helper()
	// Spec 128 Bead 2: phase.EnsurePhaseSemanticsV2 now runs beside
	// EnsureMigrated at ApprovePlan's ahead-of-preflight position and
	// writes via phase's OWN mergeMetadataFn seam (never
	// planMergeMetadataFn — see semantics_v2.go's doc comment on why it
	// is deliberately NOT a caller-injected seam). None of these fixture
	// epics carry the v2 marker (they predate it), so without this stub
	// every preflight test below would attempt a REAL bd write. A no-op
	// stub here is inert for every OTHER assertion these tests make
	// (planRunBDFn/planRunBDCombinedFn/planMergeMetadataFn are separate
	// seams), exactly like EnsureMigrated's own tests already stub this
	// same seam for its (rarer) migration write.
	noopStubPhaseMergeMetadataForTest(t)
	root = planFixtureRoot(t)
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		t.Fatalf("SpecDir: %v", err)
	}
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("mkdir specDir: %v", err)
	}
	planPath = filepath.Join(specDir, "plan.md")
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# Spec\n"), 0644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	original, err = os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read plan.md: %v", err)
	}
	return root, planPath, original
}

// forbidBDCalls stubs planRunBDFn / planRunBDCombinedFn to fail loudly if
// invoked, so a preflight refusal test proves no bd mutation was attempted.
func forbidBDCalls(t *testing.T) {
	t.Helper()
	origBD := planRunBDFn
	t.Cleanup(func() { planRunBDFn = origBD })
	planRunBDFn = func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("bd must not be called on a preflight refusal: %v", args)
	}
	origCombined := planRunBDCombinedFn
	t.Cleanup(func() { planRunBDCombinedFn = origCombined })
	planRunBDCombinedFn = func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("bd must not be called on a preflight refusal: %v", args)
	}
}

// assertPlanUnchangedAndNoMutation asserts plan.md is byte-identical to
// original and the mock executor recorded zero CommitAll calls — the R1
// "refusal leaves state byte-identical modulo migration" contract.
func assertPlanUnchangedAndNoMutation(t *testing.T, planPath string, original []byte, mockExec *executor.MockExecutor) {
	t.Helper()
	got, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("re-reading plan.md: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("plan.md mutated on a preflight refusal:\n--- before ---\n%s\n--- after ---\n%s", original, got)
	}
	if calls := mockExec.CallsTo("CommitAll"); len(calls) != 0 {
		t.Errorf("expected zero CommitAll calls on a preflight refusal, got %d", len(calls))
	}
}

// noopStubPhaseMergeMetadataForTest stubs phase.mergeMetadataFn (the seam
// phase.EnsureMigrated, phase.SanctionedDerivedSync, and
// phase.EnsurePhaseSemanticsV2 all write through) to a harmless no-op for
// the duration of one test. Spec 128 Bead 2: every ApprovePlan preflight
// fixture predates the v2 marker, so EnsurePhaseSemanticsV2's ahead-of-
// preflight marker-write would otherwise hit a REAL, unstubbed
// bead.MergeMetadata call. Shared across this file's fixtures so each
// test doesn't repeat the stub inline.
func noopStubPhaseMergeMetadataForTest(t *testing.T) {
	t.Helper()
	stubDocumentGateAudit(t)
	t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
}

// epicJSONFor builds the `bd list --type=epic` fixture response identifying
// specID's lifecycle epic (matching the ExtractSpecMetadata/SpecIDFromMetadata
// resolution resolveTargetEpic performs).
func epicJSONFor(epicID, specID string) []byte {
	num, title := splitSpecIDForTest(specID)
	return []byte(fmt.Sprintf(`[{"id":%q,"title":"[SPEC %s] Test","status":"open","issue_type":"epic","metadata":{"spec_num":%d,"spec_title":%q,"mindspec_phase":"plan"}}]`, epicID, specID, num, title))
}

// splitSpecIDForTest extracts the numeric prefix and slug title (lowercase,
// dash-joined) from a spec ID like "042-test", matching how
// phase.SpecIDFromMetadata reconstructs it from spec_num/spec_title.
func splitSpecIDForTest(specID string) (int, string) {
	idx := strings.Index(specID, "-")
	if idx < 0 {
		return 0, specID
	}
	var num int
	fmt.Sscanf(specID[:idx], "%d", &num)
	return num, specID[idx+1:]
}

func TestApprovePlan_Preflight_MisalignedWorkChunks_RefusesPreMutation(t *testing.T) {
	specID := "042-test"
	planContent := strings.Replace(validSingleBeadPlan,
		"work_chunks:\n  - id: 1\n    depends_on: []\n",
		"work_chunks:\n  - id: 1\n    depends_on: []\n  - id: 2\n    depends_on: [1]\n", 1)
	root, planPath, original := setupPreflightPlan(t, specID, planContent)
	forbidBDCalls(t)
	// Hermetic: no epic matches, so EnsureMigrated (ahead of preflight)
	// no-ops without reaching the real `bd` CLI.
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer restoreList()

	mockExec := &executor.MockExecutor{}
	_, err := ApprovePlan(root, specID, "tester", mockExec)
	if err == nil {
		t.Fatal("expected a preflight refusal for misaligned work_chunks")
	}
	if !strings.Contains(err.Error(), "misaligned") {
		t.Errorf("expected a 'misaligned' error, got: %v", err)
	}
	assertPlanUnchangedAndNoMutation(t, planPath, original, mockExec)
}

func TestApprovePlan_Preflight_AllEpicsQueryFailure_RefusesPreMutation(t *testing.T) {
	specID := "042-test"
	root, planPath, original := setupPreflightPlan(t, specID, validSingleBeadPlan)
	forbidBDCalls(t)

	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return nil, fmt.Errorf("bd list: connection refused")
			}
		}
		return []byte(`[]`), nil
	})
	defer restoreList()

	mockExec := &executor.MockExecutor{}
	_, err := ApprovePlan(root, specID, "tester", mockExec)
	if err == nil {
		t.Fatal("expected a preflight refusal on an AllEpics query failure")
	}
	if !strings.Contains(err.Error(), "querying epics failed") {
		t.Errorf("expected a distinct 'querying epics failed' message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "recovery: mindspec plan approve "+specID) {
		t.Errorf("expected the query-failure recovery command, got: %v", err)
	}
	assertPlanUnchangedAndNoMutation(t, planPath, original, mockExec)
}

func TestApprovePlan_Preflight_GenuinelyAbsentEpic_RefusesPreMutation(t *testing.T) {
	specID := "042-test"
	root, planPath, original := setupPreflightPlan(t, specID, validSingleBeadPlan)
	forbidBDCalls(t)

	// AllEpics query SUCCEEDS but returns no epic matching specID — the
	// genuinely-absent-epic case, distinct from a query failure.
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer restoreList()

	mockExec := &executor.MockExecutor{}
	_, err := ApprovePlan(root, specID, "tester", mockExec)
	if err == nil {
		t.Fatal("expected a preflight refusal for a genuinely absent epic")
	}
	if !strings.Contains(err.Error(), "no lifecycle epic found") {
		t.Errorf("expected a distinct 'no lifecycle epic found' message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "recovery: mindspec spec approve "+specID) {
		t.Errorf("expected the absent-epic recovery command (mindspec spec approve), got: %v", err)
	}
	if strings.Contains(err.Error(), "querying epics failed") {
		t.Errorf("absent-epic refusal must NOT reuse the query-failure message: %v", err)
	}
	assertPlanUnchangedAndNoMutation(t, planPath, original, mockExec)
}

func TestApprovePlan_Preflight_ChildQueryFailure_RefusesPreMutation(t *testing.T) {
	f := enrolFixture(t)
	before, _ := json.Marshal(f.Fake.Records)
	f.Fake.Hook = func(args []string) error {
		if len(args) > 0 && args[0] == "list" {
			return fmt.Errorf("child-query failure")
		}
		return nil
	}
	_, err := enrolApprove(f)
	if err == nil || !strings.Contains(err.Error(), "child-query failure") {
		t.Fatalf("query failure swallowed: %v", err)
	}
	after, _ := json.Marshal(f.Fake.Records)
	if string(before) != string(after) {
		t.Fatal("store read failure wrote records")
	}

}

func TestApprovePlan_Preflight_ForeignInProgressChildUnchanged(t *testing.T) {
	f := enrolFixture(t)
	foreign := kernel.Record{ID: "foreign-active", Parent: f.Epic, Status: "in_progress", Type: "task", Title: "foreign"}
	f.Fake.Records[foreign.ID] = foreign
	result, err := enrolApprove(f)
	f.Must(err)
	if !reflect.DeepEqual(f.Fake.Records[foreign.ID], foreign) {
		t.Fatal("approval changed foreign active work")
	}
	if len(result.BeadIDs) != 3 || !f.State().Enrolled {
		t.Fatal("approval did not enrol recorded work")
	}

}

// stubApprovePlanEpic wires phase.SetListJSONForTest + phase.SetRunBDForTest
// so ApprovePlan resolves specID to epicID (both EnsureMigrated's `bd show`
// and resolveTargetEpic's `bd list --type=epic` see the same fixture epic,
// already carrying mindspec_phase so EnsureMigrated no-ops), and stubs
// planListJSONFn to report no existing children (first approval).
func stubApprovePlanEpic(t *testing.T, epicID, specID string) {
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

	origList := planListJSONFn
	t.Cleanup(func() { planListJSONFn = origList })
	planListJSONFn = func(args ...string) ([]byte, error) { return []byte(`[]`), nil }

	restoreMerge := SetPlanMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		return nil
	})
	t.Cleanup(restoreMerge)
	restoreCombined := SetPlanRunBDCombinedForTest(func(args ...string) ([]byte, error) {
		return nil, nil
	})
	t.Cleanup(restoreCombined)
}

// TestApprovePlan_MissingWorkChunks_WarnsAndApproves is AC-19: a legacy
// prose-only plan (no `work_chunks` frontmatter at all) still approves
// successfully — wiring ZERO edges (no prose dependency parser exists, spec
// 097 R3) — but the historical silence is replaced by a warning naming the
// absent block.
func TestApprovePlan_MissingWorkChunks_WarnsAndApproves(t *testing.T) {
	f := enrolFixture(t)
	text := kerneltest.EnrolmentPlan(f.SpecID, 1)
	text = strings.Replace(text, "work_chunks:\n  - id: 1\n    depends_on: []\n", "", 1)
	enrolWrite(t, filepath.Join(f.SpecDir, "plan.md"), text)
	result, err := enrolApprove(f)
	f.Must(err)
	found := false
	for _, warning := range result.Warnings {
		found = found || strings.Contains(warning, "work_chunks")
	}
	if !found || len(result.BeadIDs) != 1 || !f.State().Enrolled {
		t.Fatalf("missing prose-plan warning or enrolment: %+v", result)
	}
	for _, link := range f.State().Links {
		if len(link.Upstream) > 0 {
			t.Fatal("prose-only plan wired upstreams")
		}
	}

}

// TestApprovePlan_DepAddFailure_WarnsBothIDs is AC-20: a best-effort
// `bd dep add` failure for one edge warns naming BOTH bead IDs of the
// unwired edge, while approve still succeeds and the OTHER edges wire.
func TestApprovePlan_DepAddFailure_RefusesBeforeEnrolment(t *testing.T) {
	f := enrolFixture(t)
	f.Fake.Hook = func(args []string) error {
		if len(args) > 1 && args[0] == "dep" && args[1] == "add" {
			return fmt.Errorf("link fault")
		}
		return nil
	}
	_, err := enrolApprove(f)
	ids := enrolIDs(t, f)
	if err == nil || !strings.Contains(err.Error(), ids[0]) || !strings.Contains(err.Error(), ids[1]) {
		t.Fatalf("link failure must name both endpoints: %v", err)
	}
	if f.State().Enrolled {
		t.Fatal("failed plan-order link enrolled")
	}

}
