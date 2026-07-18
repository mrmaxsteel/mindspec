package approve

// Spec 119 Bead 6 (AC-26 / ADR-0041): the `impl approve` fault-injection
// matrix (i0/i1/i2/i3/i5 — the five intra-FinalizeEpic stages, i4, are
// covered separately by internal/executor/finalize_fault_test.go, since
// FinalizeEpic's own mutation chain lives in that package). Mechanism B
// throughout: a package-level seam wrapper mutates real state (a real
// on-disk ADR placeholder for i0, an in-memory phase-metadata store for
// i1) then fails; each KILL test re-invokes ApproveImpl and asserts
// convergence. i2/i3/i5 are DOCUMENTED-FORWARD-SAFE (ADR-0041 §3): their
// errors are appended to result.Warnings and the run continues regardless
// — pinned end-to-end below rather than merely asserted by code cite.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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
	specDir := filepath.Join(root, "docs", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
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
// implCreateWithIDFn (impl.go, the supersede flow's placeholder pre-create)
// TERMINATES via `return nil, fmt.Errorf("--supersede-adr: %w", err)` when
// it fails — the FIRST post-preflight mutation on this lane (a file-system
// write after the orphan/obligation gate, before the ADR-divergence
// decision). Mechanism B: the wrapper performs the REAL adr.CreateWithID
// write, then fails. Re-invocation WITH the same flag hits the real
// exact-path collision refusal. The impl-approve lane hard-requires
// Accepted coverage (Proposed-only is an ERROR here, unlike complete's
// advisory tolerance) — the documented recovery is the operator flipping
// the placeholder to Accepted, then a flag-less re-run converges.
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

	// Run 2: re-invocation WITH the same flag hits the real collision.
	_, err = ApproveImpl(tmp, specID, mock, ImplOpts{AllowDocSkew: "test: fixture", SupersedeADR: supersedeID})
	if err == nil {
		t.Fatal("expected i0 re-invocation (same flag) to hit the collision refusal")
	}
	if !strings.Contains(err.Error(), supersedeID) || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected a NAMED collision refusal for %s, got: %v", supersedeID, err)
	}
	if closeCalls != 0 || len(mock.CallsTo("FinalizeEpic")) != 0 {
		t.Fatalf("nothing may mutate on the i0 collision refusal: closeCalls=%d finalizeCalls=%d", closeCalls, len(mock.CallsTo("FinalizeEpic")))
	}

	// Operator recovery: flip the placeholder from Proposed to Accepted —
	// the impl-approve lane hard-requires Accepted coverage.
	data, err := os.ReadFile(placeholderPath)
	if err != nil {
		t.Fatalf("reading placeholder: %v", err)
	}
	data = bytes.Replace(data, []byte("Status**: Proposed"), []byte("Status**: Accepted"), 1)
	if err := os.WriteFile(placeholderPath, data, 0o644); err != nil {
		t.Fatalf("flipping placeholder to Accepted: %v", err)
	}

	// Run 3: flag-less re-run converges to done (doc-sync's own gate is
	// unrelated to this ADR-divergence supersede mechanism, so AllowDocSkew
	// stays set across every run here — same fixture, same doc-sync facts).
	result, err := ApproveImpl(tmp, specID, mock, ImplOpts{AllowDocSkew: "test: fixture"})
	if err != nil {
		t.Fatalf("expected the flag-less re-run to converge to done, got: %v", err)
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

// --- i2/i3/i5: DOCUMENTED-FORWARD-SAFE post-preflight writes ---------------
//
// Three points swallow their own error and let ApproveImpl continue
// regardless (ADR-0041 §3), each appending a message to result.Warnings
// rather than failing the run:
//
//   - i2 — the epic-close mutation (`implRunBDCombinedFn("close", epicID)`,
//     impl.go): a non-"already closed" failure appends
//     "could not close lifecycle epic ...".
//   - i3 — the phase=done write (`implPhaseMetadataFn`, the done branch):
//     a failure appends "could not set done marker on epic ...".
//   - i5 — the post-FinalizeEpic override-metadata writes
//     (`implMergeMetadataFn`, the AllowDocSkew/OverrideADR/SupersedeADR
//     branches): a failure appends "could not record ... metadata on ...".
//
// This test fails ALL THREE simultaneously and pins that ApproveImpl still
// succeeds, with a Warning recorded for each — proving the swallow is real
// forward-safety, not merely documented.
func TestFaultInjection_ApproveImpl_I2I3I5_PostPreflightWritesSwallowed(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	writePlanWithBeads(t, tmp, "010-test", []string{"bead-1"})
	if err := os.MkdirAll(filepath.Join(tmp, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	saveAndRestore(t)
	// Stored phase already satisfies the review/done gate (implGateOK), so
	// needsPhaseReconcile is FALSE and implPhaseMetadataFn is invoked ONLY
	// for the (forward-safe) done-write below — never the mandatory,
	// hard-failing Req-1 reconcile write (that is i1's own kill point,
	// covered separately).
	stubPhaseStoredChildren(t, "review", []phase.ChildInfo{
		{ID: "bead-1", Status: "closed", IssueType: "task"},
	})

	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "close" {
			return nil, fiFakeErr("fault-injection: simulated epic-close failure")
		}
		return []byte("ok"), nil
	}
	implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
		return fiFakeErr("fault-injection: simulated phase-metadata failure")
	}
	implMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		return fiFakeErr("fault-injection: simulated override-metadata failure")
	}

	mock := &executor.MockExecutor{
		CommitCountResult:  5,
		FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5},
	}

	result, err := ApproveImpl(tmp, "010-test", mock, ImplOpts{AllowDocSkew: "test: i2i3i5 forward-safety"})
	if err != nil {
		t.Fatalf("i2/i3/i5 failures must all be swallowed (forward-safe), got: %v", err)
	}
	if result == nil {
		t.Fatal("expected a non-nil result despite every post-preflight write failing")
	}
	if len(mock.CallsTo("FinalizeEpic")) != 1 {
		t.Errorf("expected FinalizeEpic to have run exactly once, got %d", len(mock.CallsTo("FinalizeEpic")))
	}
	joined := strings.Join(result.Warnings, " | ")
	for _, want := range []string{"could not close lifecycle epic", "could not set done marker", "could not record impl-skew override metadata"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected a warning containing %q, got warnings: %v", want, result.Warnings)
		}
	}
}
