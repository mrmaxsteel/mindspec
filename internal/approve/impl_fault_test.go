package approve

// Spec 119 Bead 6 (AC-26 / ADR-0041): the `impl approve` fault-injection
// matrix (i0/i1/i2/i3/i5 — the five intra-FinalizeEpic stages, i4, are
// covered separately by internal/executor/finalize_fault_test.go, since
// FinalizeEpic's own mutation chain lives in that package). Mechanism B
// throughout: a package-level seam wrapper mutates real state (a real
// on-disk ADR placeholder for i0, an in-memory phase-metadata store for
// i1) then fails; each KILL test re-invokes ApproveImpl and asserts
// convergence. Close and phase-write failures stop the run. A re-run
// resumes from the stored state. Override-metadata failures remain warnings.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// fiFakeErr is a trivial injectable error for the fault-injection tests in
// this file (mirrors internal/complete's fakeErr).
type fiFakeErr string

func (e fiFakeErr) Error() string { return string(e) }

// writeSpecWithDomain writes a spec.md declaring the given Impacted Domain
// (legacy layout, matching writeSpecDir's root/docs/specs/<id> convention
// used throughout this package's existing tests) and a plan.md citing
// adrID, with the given bead IDs.
func writeSpecWithDomain(t *testing.T, root, specID, domain, adrID string, beadIDs []string) {
	t.Helper()
	writeSpecDir(t, root, specID)
	specDir := filepath.Join(root, "docs", "specs", specID)
	spec := "# Spec " + specID + "\n\n## Impacted Domains\n\n- " + domain + "\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	var ids string
	for _, id := range beadIDs {
		ids += "  - " + id + "\n"
	}
	citation := ""
	if adrID != "" {
		citation = "adr_citations:\n  - " + adrID + "\n"
	}
	plan := "---\nstatus: Approved\nspec_id: " + quoteYAML(specID) + "\nbead_ids:\n" + ids + citation + "---\n\n# Plan\n"
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(plan), 0o644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}
	domainDir := filepath.Join(root, "docs", "domains", domain)
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir domain dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "OWNERSHIP.yaml"), []byte("paths:\n  - internal/"+domain+"/**\n"), 0o644); err != nil {
		t.Fatalf("write OWNERSHIP.yaml: %v", err)
	}
}

func quoteYAML(s string) string { return "\"" + s + "\"" }

// --- i0: supersede-ADR placeholder pre-create ------------------------------
//
// The first run writes the real placeholder and fails. The same command
// accepts only that exact generated placeholder and resumes to completion.
func TestFaultInjection_ApproveImpl_I0_SupersedeADRPlaceholder_KillThenConverge(t *testing.T) {
	tmp := t.TempDir()
	const specID, supersedeID = "010-test", "ADR-9600"
	writeSpecWithDomain(t, tmp, specID, "widget", supersedeID, []string{"bead-1"})
	if err := os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	saveAndRestore(t)

	var closeCalls int
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "close" {
			closeCalls++
		}
		return []byte("ok"), nil
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
		ChangedFilesResult: []string{"internal/widget/thing.go"},
	}
	serveRefFromDisk(mock, tmp)

	callCount := 0
	implCreateWithIDFn = func(root, id, title string, opts adr.CreateOpts) (string, error) {
		callCount++
		if callCount == 1 {
			if _, err := adr.CreateWithID(root, id, title, opts); err != nil {
				t.Fatalf("fixture: real placeholder create failed: %v", err)
			}
			return "", fiFakeErr("fault-injection: kill after placeholder write landed")
		}
		return adr.CreateWithID(root, id, title, opts)
	}

	// Run 1 (KILL): the real placeholder lands on disk, then the seam fails.
	_, err := ApproveImpl(tmp, specID, mock, ImplOpts{AllowDocSkew: "test: fixture", SupersedeADR: supersedeID})
	if err == nil {
		t.Fatal("expected i0 kill: the supersede-adr seam failure must fail ApproveImpl")
	}
	if closeCalls != 0 || len(mock.CallsTo("FinalizeEpic")) != 0 {
		t.Fatalf("nothing may mutate on the i0 kill: closeCalls=%d finalizeCalls=%d", closeCalls, len(mock.CallsTo("FinalizeEpic")))
	}
	placeholderPath, pathErr := workspace.ADRFilePath(tmp, supersedeID)
	if pathErr != nil {
		t.Fatalf("resolving placeholder path: %v", pathErr)
	}
	if _, statErr := os.Stat(placeholderPath); statErr != nil {
		t.Fatalf("expected the real placeholder to land on disk despite the kill (path=%s): %v", placeholderPath, statErr)
	}

	// The same command resumes after its exact placeholder write.
	result, err := ApproveImpl(tmp, specID, mock, ImplOpts{AllowDocSkew: "test: fixture", SupersedeADR: supersedeID})
	if err != nil {
		t.Fatalf("expected the same command to converge, got: %v", err)
	}
	if result == nil || result.SpecID != specID {
		t.Fatalf("unexpected result: %+v", result)
	}
	if closeCalls != 1 {
		t.Errorf("expected the epic close to run exactly once on convergence, got %d", closeCalls)
	}
	if len(mock.CallsTo("FinalizeEpic")) != 1 {
		t.Errorf("expected FinalizeEpic to run exactly once on convergence, got %d", len(mock.CallsTo("FinalizeEpic")))
	}
}

// --- i1: the deferred stale-phase reconcile write --------------------------
//
// implPhaseMetadataFn's Req-1 reconcile write (impl.go, `needsPhaseReconcile`
// branch — runs AFTER the last pre-terminal gate and BEFORE MUTATION 1/3)
// TERMINATES via guard.NewFailure when it fails. Mechanism B: the SAME
// seam wrapper c1/c8-style — an in-memory store the reconcile write and the
// later done-write share, so the write genuinely attempts to land before
// failing. Re-invocation re-derives the identical stored/derived phase pair
// and the SAME reconcile write succeeds this time (deterministic
// re-derivation) — converging through the exact success shape
// TestApproveImpl_StalePhaseReconcilesForward pins.
func TestFaultInjection_ApproveImpl_I1_PhaseReconcileWrite_KillThenConverge(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	if err := os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}

	saveAndRestore(t)
	stubPhaseStoredChildren(t, "implement", []phase.ChildInfo{
		{ID: "bead-1", Status: "closed", IssueType: "task"},
	})

	store := map[string]interface{}{
		"mindspec_phase":       "implement",
		"mindspec_migrated_at": "2026-01-01T00:00:00Z",
	}
	reconcileAttempts := 0
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		if _, done := updates["mindspec_done"]; !done {
			reconcileAttempts++
			if reconcileAttempts == 1 {
				// KILL: the reconcile write fails on its first attempt —
				// nothing lands in the store.
				return fiFakeErr("fault-injection: simulated reconcile write failure")
			}
		}
		for k, v := range updates {
			store[k] = v
		}
		return nil
	}
	var closeCalls int
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "close" {
			closeCalls++
		}
		return []byte("ok"), nil
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}

	// Run 1 (KILL): the reconcile write fails; ApproveImpl must fail BEFORE
	// the epic close or FinalizeEpic.
	_, err := ApproveImpl(tmp, "010-test", mock)
	if err == nil {
		t.Fatal("expected i1 kill: the reconcile-write failure must fail ApproveImpl")
	}
	if closeCalls != 0 || len(mock.CallsTo("FinalizeEpic")) != 0 {
		t.Fatalf("nothing may mutate on the i1 kill: closeCalls=%d finalizeCalls=%d", closeCalls, len(mock.CallsTo("FinalizeEpic")))
	}
	if store["mindspec_phase"] != "implement" {
		t.Errorf("expected the stored phase unchanged after the i1 kill, got %v", store["mindspec_phase"])
	}

	// Re-invoke: the SAME stored/derived phase pair is re-derived
	// deterministically, and this time the reconcile write succeeds.
	result, err := ApproveImpl(tmp, "010-test", mock)
	if err != nil {
		t.Fatalf("expected i1 re-invocation to converge, got: %v", err)
	}
	if result == nil || result.SpecID != "010-test" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if closeCalls != 1 {
		t.Errorf("expected the epic close to run exactly once on convergence, got %d", closeCalls)
	}
	if got := store["mindspec_phase"]; got != "done" {
		t.Errorf("end-state mindspec_phase = %v, want done", got)
	}
}

// --- i2/i3/i5: post-preflight write failures and recovery ------------------
//
// Close and phase-write failures return the actual epic state, failed stage,
// and re-run command. A re-run completes with one successful close. Override
// metadata is written after FinalizeEpic; its failure remains a warning.
func TestFaultInjection_ApproveImpl_I2I3I5_PostPreflightWritesRecover(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		stage                 string
		state                 string
		fault                 fiFakeErr
		firstCloseAttempts    int
		firstPhaseAttempts    int
		firstOverrideAttempts int
	}{
		{"i2_close", "close", "epic still open", "fault-injection: simulated epic-close failure", 1, 0, 0},
		{"i3_phase", "phase", "epic closed", "fault-injection: simulated phase-metadata failure", 1, 1, 0},
		{"i5_override", "override", "epic closed", "fault-injection: simulated override-metadata failure", 1, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			const specID = "010-test"
			writeSpecDir(t, tmp, specID)
			writePlanWithBeads(t, tmp, specID, []string{"bead-1"})
			if err := os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755); err != nil {
				t.Fatal(err)
			}
			// Finalization is mocked here, so retain a spec-worktree directory
			// through retries to model a live checkout before cleanup.
			specWorktree, err := workspace.SpecWorktreePath(tmp, nil, specID)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(specWorktree, 0o755); err != nil {
				t.Fatal(err)
			}
			saveAndRestore(t)

			// Both status readers share the state changed by a successful close.
			// A failed close leaves the epic open. Closed child beads stay closed.
			epics := []phase.EpicInfo{{
				ID: "epic-parent", Title: "[SPEC 010-test] Test", Status: "open",
				IssueType: "epic", Metadata: map[string]interface{}{
					"spec_num": float64(10), "spec_title": "test",
					"mindspec_phase": "review", "mindspec_migrated_at": "2026-01-01T00:00:00Z",
				},
			}}
			children := []phase.ChildInfo{{ID: "bead-1", Status: "closed", IssueType: "task"}}
			t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
				if contains(args, "--type=epic") {
					return json.Marshal(epics)
				}
				if contains(args, "--parent") {
					return json.Marshal(children)
				}
				return []byte("[]"), nil
			}))
			t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
				if len(args) >= 2 && args[0] == "show" && args[1] == "epic-parent" {
					return json.Marshal(epics)
				}
				return []byte("[]"), nil
			}))
			epicStatusReads := 0
			implRunBDFn = func(args ...string) ([]byte, error) {
				if len(args) >= 2 && args[0] == "show" {
					switch args[1] {
					case "epic-parent":
						epicStatusReads++
						return json.Marshal(epics)
					case "bead-1":
						return json.Marshal(children)
					}
				}
				t.Fatalf("unexpected status read: %v", args)
				return nil, nil
			}

			faultEnabled := true
			closeAttempts, successfulCloses, phaseAttempts, overrideAttempts := 0, 0, 0, 0
			implRunBDCombinedFn = func(args ...string) ([]byte, error) {
				if len(args) != 2 || args[0] != "close" || args[1] != "epic-parent" {
					t.Fatalf("unexpected close: %v", args)
				}
				closeAttempts++
				if faultEnabled && tc.stage == "close" {
					return nil, tc.fault
				}
				if epics[0].Status == "closed" {
					t.Fatal("close called again after the epic closed")
				}
				epics[0].Status = "closed"
				successfulCloses++
				return []byte("ok"), nil
			}
			implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
				phaseAttempts++
				if id != "epic-parent" || updates["mindspec_phase"] != "done" || updates["mindspec_done"] != true {
					t.Fatalf("unexpected phase write: %s %v", id, updates)
				}
				if faultEnabled && tc.stage == "phase" {
					return tc.fault
				}
				for key, value := range updates {
					epics[0].Metadata[key] = value
				}
				return nil
			}
			mock := &executor.MockExecutor{
				CommitCountResult:  5,
				FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
			}
			implMergeMetadataFn = func(id string, updates map[string]interface{}) error {
				overrideAttempts++
				if id != "epic-parent" || updates["mindspec_impl_skew_reason"] != "test: write recovery" {
					t.Fatalf("unexpected override write: %s %v", id, updates)
				}
				if len(mock.CallsTo("FinalizeEpic")) == 0 {
					t.Fatal("override metadata written before FinalizeEpic")
				}
				if faultEnabled && tc.stage == "override" {
					return tc.fault
				}
				return nil
			}

			opts := ImplOpts{AllowDocSkew: "test: write recovery"}
			result, err := ApproveImpl(tmp, specID, mock, opts)
			if tc.stage == "override" {
				if err != nil || result == nil {
					t.Fatalf("override failure must remain a warning: result=%+v err=%v", result, err)
				}
				want := "could not record impl-skew override metadata on epic-parent: " + tc.fault.Error()
				if len(result.Warnings) != 1 || result.Warnings[0] != want {
					t.Fatalf("warnings = %v, want %q", result.Warnings, want)
				}
			} else {
				inner := tc.fault.Error()
				if tc.stage == "phase" {
					inner = "could not sync phase metadata: " + inner
				}
				want := tc.state + "; finalize stage " + tc.stage + "\n" + inner + "\nrecovery: mindspec impl approve " + specID
				if err == nil || err.Error() != want {
					t.Fatalf("failure = %v, want exactly:\n%s", err, want)
				}
				if result != nil || epicStatusReads == 0 || len(mock.CallsTo("FinalizeEpic")) != 0 {
					t.Fatalf("failure must read epic status and stop before finalize: result=%+v reads=%d calls=%v", result, epicStatusReads, mock.CallsTo("FinalizeEpic"))
				}
			}
			if closeAttempts != tc.firstCloseAttempts || phaseAttempts != tc.firstPhaseAttempts || overrideAttempts != tc.firstOverrideAttempts {
				t.Fatalf("write attempts = close:%d phase:%d override:%d, want %d/%d/%d", closeAttempts, phaseAttempts, overrideAttempts, tc.firstCloseAttempts, tc.firstPhaseAttempts, tc.firstOverrideAttempts)
			}
			finalizeCalls := len(mock.CallsTo("FinalizeEpic"))

			// Remove the fault and re-run against the same stored state.
			faultEnabled = false
			result, err = ApproveImpl(tmp, specID, mock, opts)
			if err != nil || result == nil || result.SpecID != specID {
				t.Fatalf("re-run must converge: result=%+v err=%v", result, err)
			}
			if epics[0].Status != "closed" || epics[0].Metadata["mindspec_phase"] != "done" || epics[0].Metadata["mindspec_done"] != true || successfulCloses != 1 {
				t.Fatalf("re-run state = %+v, successful closes = %d, want closed/done and one close", epics[0], successfulCloses)
			}
			if got := len(mock.CallsTo("FinalizeEpic")); got != finalizeCalls+1 {
				t.Fatalf("FinalizeEpic calls = %d, want %d", got, finalizeCalls+1)
			}
			if phaseAttempts != tc.firstPhaseAttempts+1 || overrideAttempts != tc.firstOverrideAttempts+1 || len(result.Warnings) != 0 {
				t.Fatalf("re-run writes = phase:%d override:%d, warnings=%v", phaseAttempts, overrideAttempts, result.Warnings)
			}
		})
	}
}
