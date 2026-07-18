package complete

// Spec 119 Bead 6 (AC-26 / ADR-0041): the `complete` fault-injection matrix.
//
// Every significant post-preflight mutation point in complete.Run is
// classified KILL-TESTED or DOCUMENTED-FORWARD-SAFE (ADR-0041 §3). A
// KILL-TESTED point's test enacts the kill through a mechanism that
// genuinely performs the real mutation AND terminates the run:
//
//   - mechanism B (this file, c1/c4/c7/c8): a package-level seam wrapper
//     (completeMergeMetadataFn/completeGetMetadataFn/closeBeadFn/
//     doltCommitFn/adrCreateWithIDFn) mutates an in-memory tracker fake (or,
//     for c7, the REAL adr.CreateWithID against a real temp ADR dir) for
//     real, then fails.
//   - mechanism A (fault_injection_realgit_test.go, c2/c3/c5): a decorator
//     wrapping a REAL git executor performs the actual git mutation, then
//     forces a terminal error.
//
// Each test re-invokes the same verb and asserts convergence to completion
// or a clean, named, recoverable refusal — never a fabricated "kill" that
// doesn't actually terminate anything (ADR-0041 §3's standing rule).
//
// DOCUMENTED-FORWARD-SAFE points (c6) are named below with their code
// cites, not fictitiously kill-tested: their errors are swallowed by
// design (a Warning print or a `_ =` discard) and the run continues
// regardless, so no seam can enact a "kill" there.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// --- c1: durable-obligation marker write (persistRefutationPending) -------
//
// panelGate's step 2.25 marker write (panel_advisory.go, persistRefutationPending)
// TERMINATES the run via guard.NewFailure when it fails — mechanism B via the
// SAME completeGetMetadataFn/completeMergeMetadataFn seams c8 uses. Builds on
// the existing TestPanelRefuted_MarkerWriteFailure_Blocks single-invocation
// Block proof (panel_refuted_test.go) by adding the AC-26 re-invocation leg:
// clearing the injected failure converges to completion via the SAME
// (idempotent) re-union the marker write always performs.
func TestFaultInjection_Complete_C1_ObligationMarkerWrite_KillThenConverge(t *testing.T) {
	const specID, beadID = "911-fic1", "mindspec-119fic1.1"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore()
	store.failMerge = failOnKey("refutation_pending_entries")
	store.wire(t)

	var closeCalled bool
	closeBeadFn = func(...string) error { closeCalled = true; return nil }

	writePanel(t, root, specID+"-bd", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
		Refutations:     []panel.Refutation{{Slot: "X", Round: 1, Reason: "dismissed"}},
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
		"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "REQUEST_CHANGES",
	})

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}

	// KILL: the durable-obligation marker write fails — Run must Block
	// BEFORE any further mutation (no close, no merge).
	_, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: fault-injection"})
	if err == nil {
		t.Fatal("expected c1 kill: the marker write must Block completion")
	}
	if closeCalled || ex.completeCalled {
		t.Fatalf("nothing may mutate on the c1 Block: close=%v merge=%v", closeCalled, ex.completeCalled)
	}

	// Re-invoke: clear the injected failure. The SAME panel.json is
	// re-scanned, the SAME refutation is re-applied, and this time the
	// (idempotent) union-then-write durably persists — Run converges.
	store.failMerge = nil
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: fault-injection"})
	if err != nil {
		t.Fatalf("expected c1 re-invocation to converge to completion, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed after convergence, got %+v", res)
	}
	if !closeCalled || !ex.completeCalled {
		t.Errorf("expected the terminal close+merge to run on convergence: close=%v merge=%v", closeCalled, ex.completeCalled)
	}
}

// --- c8: pending-obligation settlement write (writePanelRefutedMetadata) --
//
// reconcilePendingRefutations' pre-close settlement write (complete.go step
// 3.75, panel_advisory.go writePanelRefutedMetadata) TERMINATES pre-close on
// failure ("an obligation may NEVER merge un-audited"). Builds on the
// existing TestPanelRefuted_SatisfyWriteFailure_FailsCompletion
// single-invocation proof by adding the re-invocation leg: the marker
// (refutation_pending_entries) already landed durably in run 1, so run 2
// recomputes the SAME uncovered set and settles it — converging to done.
func TestFaultInjection_Complete_C8_ObligationSettlementWrite_KillThenConverge(t *testing.T) {
	const specID, beadID = "918-fic8", "mindspec-119fic8.1"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore()
	store.failMerge = failOnKey("panel_refuted")
	store.wire(t)

	var closeCalled bool
	closeBeadFn = func(...string) error { closeCalled = true; return nil }

	writePanel(t, root, specID+"-bd", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
		Refutations:     []panel.Refutation{{Slot: "X", Round: 1, Reason: "dismissed"}},
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
		"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "REQUEST_CHANGES",
	})

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}

	// KILL: the marker write (refutation_pending_entries) lands durably, but
	// the settling panel_refuted write fails — completion fails pre-close.
	_, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: fault-injection"})
	if err == nil {
		t.Fatal("expected c8 kill: the settlement write failure must fail completion pre-close")
	}
	if closeCalled || ex.completeCalled {
		t.Fatalf("nothing may mutate on the c8 kill: close=%v merge=%v", closeCalled, ex.completeCalled)
	}
	if store.data[beadID]["refutation_pending_entries"] == nil {
		t.Fatal("expected the marker write to have landed durably despite the c8 kill")
	}

	// Re-invoke: clear the injected failure. reconcilePendingRefutations
	// recomputes uncoveredPendingObligations from the durable marker alone
	// and settles it — Run converges to done.
	store.failMerge = nil
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: fault-injection"})
	if err != nil {
		t.Fatalf("expected c8 re-invocation to converge to completion, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed after convergence, got %+v", res)
	}
	if store.data[beadID]["panel_refuted"] != true {
		t.Errorf("expected panel_refuted=true after convergence, got %v", store.data[beadID]["panel_refuted"])
	}
	if !closeCalled || !ex.completeCalled {
		t.Errorf("expected the terminal close+merge to run on convergence: close=%v merge=%v", closeCalled, ex.completeCalled)
	}
}

// --- c4: `bd close` + dolt durability -------------------------------------
//
// The forced `bd dolt commit` durability check (complete.go, after a
// successful closeBeadFn + a re-read affirming "closed") TERMINATES via
// guard.NewFailure when it fails. Mechanism B: an in-memory tracker fake
// that closeBeadFn/fetchBeadByIDFn share, so the close genuinely lands in
// the fake tracker before doltCommitFn is asked to make it durable and
// fails. Re-invocation hits the already-closed tolerance (closeBeadFn
// errors "already closed", the re-read affirms closed) and converges —
// skipping the dolt-commit check entirely on that path, exactly as
// production does.
func TestFaultInjection_Complete_C4_DoltDurability_KillThenConverge(t *testing.T) {
	saveAndRestore(t)
	root := setupTempRoot(t)
	stubPhaseEpic(t, "914-fic4", "epic-119fic4")

	resolveTargetFn = func(r, flag string) (string, error) { return "914-fic4", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	var closed bool
	closeBeadFn = func(ids ...string) error {
		if closed {
			return fakeErr("already closed")
		}
		closed = true
		return nil
	}
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		if closed {
			return next.BeadInfo{ID: id, Status: "closed"}, nil
		}
		return next.BeadInfo{ID: id, Status: "in_progress"}, nil
	}

	var doltShouldFail = true
	doltCommitFn = func() error {
		if doltShouldFail {
			return fakeErr("fault-injection: simulated `bd dolt commit` failure")
		}
		return nil
	}

	mock := newMockExec()

	// KILL: closeBeadFn succeeds (the fake tracker's bead IS closed), the
	// post-close re-read affirms "closed", but the forced dolt-durability
	// commit fails — Run must Block before the merge.
	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected c4 kill: the dolt-commit failure must Block completion")
	}
	if !strings.Contains(err.Error(), "dolt commit") {
		t.Errorf("expected the error to name the dolt-commit failure, got: %v", err)
	}
	if len(mock.CallsTo("CompleteBead")) != 0 {
		t.Errorf("expected ZERO CompleteBead calls on the c4 kill, got %d", len(mock.CallsTo("CompleteBead")))
	}
	if !closed {
		t.Fatal("expected the fake tracker to genuinely reflect closed (the real mutation landed)")
	}

	// Re-invoke: the dolt-commit failure is gone, but it never even runs —
	// closeBeadFn now hits the "already closed" tolerance branch (the bead
	// really is closed in the fake tracker from run 1), which skips the
	// forced-durability re-check entirely and proceeds to merge+cleanup.
	doltShouldFail = false
	res, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected c4 re-invocation to converge via the already-closed tolerance, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed after convergence, got %+v", res)
	}
	if len(mock.CallsTo("CompleteBead")) != 1 {
		t.Errorf("expected exactly 1 CompleteBead call on convergence, got %d", len(mock.CallsTo("CompleteBead")))
	}
}

// --- c7: supersede-ADR placeholder pre-create -----------------------------
//
// The --supersede-adr placeholder pre-create (complete.go, adrCreateWithIDFn)
// TERMINATES the run via `return nil, fmt.Errorf("--supersede-adr: %w", err)`
// when it fails, and runs BEFORE the gate-failure decision — a file-system
// write after the ADR-divergence facts are computed but before any close.
// Mechanism B: the wrapper performs the REAL adr.CreateWithID placeholder
// write on the FIRST call, then fails; the SECOND (re-invocation, same
// --supersede-adr flag) call reaches the REAL adr.CreateWithID again and
// hits its genuine exact-path collision check, naming the existing file —
// the clean NAMED refusal (c2 precedent). A THIRD, flag-less re-run
// converges to done: the plan already cites the new ADR ID (a forward
// reference — the realistic --supersede-adr usage shape), so once the
// placeholder exists on disk the per-bead lane's Proposed-only tolerance
// (adr_divergence.go's advisory WARNING, not a gate failure) clears the
// gate with no override needed.
func TestFaultInjection_Complete_C7_SupersedeADRPlaceholder_KillThenConverge(t *testing.T) {
	saveAndRestore(t)
	root := setupTempRoot(t)
	const specID, beadID = "917-fic7", "mindspec-119fic7.1"
	const supersedeID = "ADR-9101"
	stubPhaseEpic(t, specID, "epic-119fic7")

	specDir := filepath.Join(".mindspec", "docs", "specs", specID)
	writeFile(t, root, filepath.Join(specDir, "spec.md"), "# Spec\n\n## Impacted Domains\n\n- widget\n")
	writeFile(t, root, filepath.Join(specDir, "plan.md"),
		"---\nstatus: Draft\nspec_id: "+specID+"\nversion: \"1\"\nadr_citations:\n  - "+supersedeID+"\n---\n\n# Plan\n")
	writeFile(t, root, ".mindspec/docs/domains/widget/OWNERSHIP.yaml", "paths:\n  - internal/widget/**\n")

	resolveTargetFn = func(r, flag string) (string, error) { return specID, nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	var closeCalled bool
	closeBeadFn = func(...string) error { closeCalled = true; return nil }

	mock := newMockExec()
	mock.ChangedFilesFn = func(base, head string) ([]string, error) {
		return []string{"internal/widget/thing.go"}, nil
	}
	serveRefFromDisk(mock, root)

	callCount := 0
	adrCreateWithIDFn = func(r, id, title string, opts adr.CreateOpts) (string, error) {
		callCount++
		if callCount == 1 {
			if _, err := adr.CreateWithID(r, id, title, opts); err != nil {
				t.Fatalf("fixture: real placeholder create failed: %v", err)
			}
			return "", fakeErr("fault-injection: kill after placeholder write landed")
		}
		return adr.CreateWithID(r, id, title, opts)
	}

	// Run 1 (KILL): the real placeholder write lands on disk, then the seam
	// forces a terminal error — no close, no merge.
	_, err := Run(root, beadID, specID, "", mock, CompleteOpts{AllowDocSkew: "test: fixture", SupersedeADR: supersedeID})
	if err == nil {
		t.Fatal("expected c7 kill: the supersede-adr seam failure must fail completion")
	}
	if closeCalled {
		t.Error("nothing may mutate on the c7 kill")
	}
	placeholderPath, pathErr := workspace.ADRFilePath(root, supersedeID)
	if pathErr != nil {
		t.Fatalf("resolving placeholder path: %v", pathErr)
	}
	if _, statErr := os.Stat(placeholderPath); statErr != nil {
		t.Fatalf("expected the real placeholder to land on disk despite the kill (path=%s), stat err=%v", placeholderPath, statErr)
	}

	// Run 2: re-invocation WITH the same flag converges to the clean NAMED
	// collision refusal (adr.CreateWithID's own exact-path check).
	_, err = Run(root, beadID, specID, "", mock, CompleteOpts{AllowDocSkew: "test: fixture", SupersedeADR: supersedeID})
	if err == nil {
		t.Fatal("expected c7 re-invocation (same flag) to hit the collision refusal")
	}
	if !strings.Contains(err.Error(), supersedeID) || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected a NAMED collision refusal for %s, got: %v", supersedeID, err)
	}
	if closeCalled {
		t.Error("nothing may mutate on the c7 collision refusal")
	}

	// Run 3: the flag-less re-run converges to done — the already-written
	// Proposed placeholder (cited by the plan) clears the per-bead lane's
	// advisory tolerance.
	res, err := Run(root, beadID, specID, "", mock, CompleteOpts{AllowDocSkew: "test: fixture"})
	if err != nil {
		t.Fatalf("expected the flag-less re-run to converge to done, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed after convergence, got %+v", res)
	}
	if !closeCalled {
		t.Error("expected the terminal close+merge to run on the flag-less convergence")
	}
}

// --- c6: DOCUMENTED-FORWARD-SAFE post-terminal metadata writes ------------
//
// Every write below runs AFTER exec.CompleteBead already returned nil (the
// terminal mutation already landed) and swallows its own error as a
// stderr Warning print — the run CONTINUES regardless, so no seam can
// enact a "kill" here (ADR-0041 §3):
//
//   - doc-skew override metadata (complete.go, "Warning: could not record
//     doc-skew override metadata")
//   - ADR-override metadata (complete.go, "Warning: could not record
//     adr-override metadata")
//   - ADR-supersede metadata (complete.go, "Warning: could not record
//     adr-supersede metadata")
//   - the phase-mode sync write onto the epic (complete.go, a bare `_ =`
//     discard: "_ = completeMergeMetadataFn(epicID, ...)")
//   - the panel-gate audit writes (panel_advisory.go's
//     writePanelAuditMetadata, called from complete.go only when
//     completeErr == nil): panel_gate_skipped and panel_abandoned each
//     warn-print on a write failure via the SAME best-effort discipline.
//
// TestFaultInjection_Complete_C6_PostTerminalMetadataSwallowed pins one
// representative instance (the doc-skew override write) end-to-end: even
// when completeMergeMetadataFn fails on EVERY call after the terminal
// merge, Run still reports success — proving the swallow is real, not
// merely documented.
func TestFaultInjection_Complete_C6_PostTerminalMetadataSwallowed(t *testing.T) {
	saveAndRestore(t)
	root := setupTempRoot(t)
	stubPhaseEpic(t, "916-fic6", "epic-119fic6")

	resolveTargetFn = func(r, flag string) (string, error) { return "916-fic6", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }
	closeBeadFn = func(...string) error { return nil }

	// Every post-terminal metadata write fails — the doc-skew override
	// write, the phase-mode sync, and the panel audit writes alike.
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		return fakeErr("fault-injection: simulated post-terminal metadata failure")
	}

	mock := newMockExec()
	res, err := Run(root, "bead-1", "", "", mock, CompleteOpts{AllowDocSkew: "test: c6 forward-safety"})
	if err != nil {
		t.Fatalf("a post-terminal metadata failure must be swallowed (forward-safe), got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed despite every post-terminal metadata write failing, got %+v", res)
	}
	if len(mock.CallsTo("CompleteBead")) != 1 {
		t.Errorf("expected the terminal merge to have run exactly once, got %d", len(mock.CallsTo("CompleteBead")))
	}
}
