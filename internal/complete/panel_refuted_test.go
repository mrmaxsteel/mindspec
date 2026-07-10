package complete

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// Spec 114 R2 (Bead 2): the DURABLE-OBLIGATION protocol e2e suite. Named
// under the TestPanelRefuted… prefix so `-run 'PanelRefuted|PanelGate'`
// selects it alongside the pre-existing TestPanelGate_* suite (round-3 F1).

// fakeMetadataStore is an in-memory bd-metadata double: completeGetMetadataFn
// and completeMergeMetadataFn both round-trip through the SAME map, so a
// write earlier in a run (or in a PRIOR simulated run) is visible to a read
// later — exactly like a real `bd show` reading back a prior `bd update`.
// failGet / failMerge let a test simulate a read/write failure on a
// SPECIFIC call without touching the underlying store (mirroring the
// fail-closed contract: a failed call never mutates state).
type fakeMetadataStore struct {
	data map[string]map[string]interface{}

	failGet   func(id string) bool
	failMerge func(id string, updates map[string]interface{}) bool
}

func newFakeMetadataStore() *fakeMetadataStore {
	return &fakeMetadataStore{data: map[string]map[string]interface{}{}}
}

func (s *fakeMetadataStore) Get(id string) (map[string]interface{}, error) {
	if s.failGet != nil && s.failGet(id) {
		return nil, errors.New("simulated bd show read failure")
	}
	out := map[string]interface{}{}
	for k, v := range s.data[id] {
		out[k] = v
	}
	return out, nil
}

func (s *fakeMetadataStore) Merge(id string, updates map[string]interface{}) error {
	if s.failMerge != nil && s.failMerge(id, updates) {
		return errors.New("simulated bd metadata write failure")
	}
	if s.data[id] == nil {
		s.data[id] = map[string]interface{}{}
	}
	for k, v := range updates {
		s.data[id][k] = v
	}
	return nil
}

// wire installs this store as BOTH metadata seams and registers cleanup.
// Call saveAndRestore(t) FIRST (as every Run-calling test does) so its
// t.Cleanup (LIFO) restores the true production seams last.
func (s *fakeMetadataStore) wire(t *testing.T) {
	t.Helper()
	completeGetMetadataFn = s.Get
	completeMergeMetadataFn = s.Merge
}

// failOnKey returns a failMerge predicate that fails a write iff updates
// carries the named key — the "only on maps containing X" stubbing pattern
// several tests below need.
func failOnKey(key string) func(string, map[string]interface{}) bool {
	return func(_ string, updates map[string]interface{}) bool {
		_, ok := updates[key]
		return ok
	}
}

// --- (i) AC2 audit-half ------------------------------------------------

// TestPanelRefuted_WriteMetadata is writePanelRefutedMetadata's unit suite,
// beside TestWritePanelAuditMetadata_Abandoned: captures the merged map
// (panel_refuted, an RFC3339 panel_refuted_at, the slot/round/reason/
// evidence entries) and asserts the error return propagates (non-swallowing,
// unlike writePanelAuditMetadata).
func TestPanelRefuted_WriteMetadata(t *testing.T) {
	t.Run("writes panel_refuted with entries", func(t *testing.T) {
		origGet, origMerge := completeGetMetadataFn, completeMergeMetadataFn
		defer func() { completeGetMetadataFn = origGet; completeMergeMetadataFn = origMerge }()

		var got map[string]interface{}
		completeGetMetadataFn = func(string) (map[string]interface{}, error) { return map[string]interface{}{}, nil }
		completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
			got = updates
			return nil
		}

		entries := []panel.Refutation{{Slot: "z", Round: 1, Reason: "max: dismissed", Evidence: "commit abc123"}}
		if err := writePanelRefutedMetadata("mindspec-bd01", entries); err != nil {
			t.Fatalf("writePanelRefutedMetadata: %v", err)
		}
		if got["panel_refuted"] != true {
			t.Errorf("expected panel_refuted=true, got %v", got)
		}
		at, _ := got["panel_refuted_at"].(string)
		if _, err := time.Parse(time.RFC3339, at); err != nil {
			t.Errorf("panel_refuted_at not RFC3339: %q (%v)", at, err)
		}
		gotEntries, ok := got["panel_refuted_entries"].([]panel.Refutation)
		if !ok || len(gotEntries) != 1 || gotEntries[0] != entries[0] {
			t.Errorf("panel_refuted_entries = %+v, want %+v", got["panel_refuted_entries"], entries)
		}
	})

	t.Run("error return propagates (non-swallowing)", func(t *testing.T) {
		origGet, origMerge := completeGetMetadataFn, completeMergeMetadataFn
		defer func() { completeGetMetadataFn = origGet; completeMergeMetadataFn = origMerge }()
		completeGetMetadataFn = func(string) (map[string]interface{}, error) { return map[string]interface{}{}, nil }
		completeMergeMetadataFn = func(string, map[string]interface{}) error { return errors.New("write failed") }

		err := writePanelRefutedMetadata("mindspec-bd01", []panel.Refutation{{Slot: "z", Round: 1}})
		if err == nil {
			t.Fatal("expected the merge error to propagate (non-swallowing)")
		}
	})
}

// TestPanelRefuted_E2E_SatisfiesAndRecordsBothEntries is the (i) e2e half: a
// fresh 5A+1RC panel with a matching refutations entry lets `Run` succeed,
// and the captured metadata carries BOTH refutation_pending and
// panel_refuted (the same-run Satisfy path, O2).
func TestPanelRefuted_E2E_SatisfiesAndRecordsBothEntries(t *testing.T) {
	const specID, beadID = "114-pr01", "mindspec-114pr.1"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore()
	store.wire(t)

	closeCalled := false
	closeBeadFn = func(...string) error { closeCalled = true; return nil }

	writePanel(t, root, specID+"-bd01", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
		Refutations:     []panel.Refutation{{Slot: "X", Round: 1, Reason: "max: dismissed", Evidence: "commit abc"}},
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
		"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "REQUEST_CHANGES",
	})

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"})
	if err != nil {
		t.Fatalf("a refuted RC panel must complete; got: %v", err)
	}
	if !ex.completeCalled || !closeCalled {
		t.Fatalf("expected the terminal close+merge to run: close=%v merge=%v", closeCalled, ex.completeCalled)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed, got %+v", res)
	}

	meta := store.data[beadID]
	if meta["refutation_pending_entries"] == nil {
		t.Error("expected refutation_pending_entries to be durably recorded")
	}
	if meta["panel_refuted"] != true {
		t.Errorf("expected panel_refuted=true, got %v", meta["panel_refuted"])
	}
}

// --- (ii) AC11 panel_refuted-write-fails → not-closed → merge never runs --

func TestPanelRefuted_SatisfyWriteFailure_FailsCompletion(t *testing.T) {
	const specID, beadID = "114-pr02", "mindspec-114pr.2"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore()
	store.failMerge = failOnKey("panel_refuted")
	store.wire(t)

	closeCalled := false
	closeBeadFn = func(...string) error { closeCalled = true; return nil }

	writePanel(t, root, specID+"-bd02", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
		Refutations:     []panel.Refutation{{Slot: "X", Round: 1, Reason: "dismissed"}},
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
		"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "REQUEST_CHANGES",
	})

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	_, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"})
	if err == nil {
		t.Fatal("expected Run to fail when the satisfying panel_refuted write fails")
	}
	if closeCalled {
		t.Error("closeBeadFn must NOT run when reconciliation fails pre-close")
	}
	if ex.completeCalled {
		t.Error("the terminal merge must NOT run when reconciliation fails pre-close")
	}
	if store.data[beadID]["panel_refuted"] != nil {
		t.Errorf("panel_refuted must not be recorded on a failed write, got %v", store.data[beadID])
	}
	// The pending marker itself (a DIFFERENT metadata key) still succeeded.
	if store.data[beadID]["refutation_pending_entries"] == nil {
		t.Error("expected the marker write (a different key) to have succeeded")
	}
}

// --- (iii) AC11 applied≡persisted: marker-write / union-read fail ⟹ Block --

func TestPanelRefuted_MarkerWriteFailure_Blocks(t *testing.T) {
	t.Run("marker write itself fails", func(t *testing.T) {
		const specID, beadID = "114-pr03", "mindspec-114pr.3"
		root, beadSHA := setupPanelGateRepo(t, specID, beadID)
		store := newFakeMetadataStore()
		store.failMerge = failOnKey("refutation_pending_entries")
		store.wire(t)

		closeCalled := false
		closeBeadFn = func(...string) error { closeCalled = true; return nil }

		writePanel(t, root, specID+"-bd03", panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
			ReviewedHeadSHA: beadSHA,
			Refutations:     []panel.Refutation{{Slot: "X", Round: 1, Reason: "dismissed"}},
		}, map[string]string{
			"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
			"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "REQUEST_CHANGES",
		})

		ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
		_, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"})
		if err == nil {
			t.Fatal("expected a Block when the durable marker write fails")
		}
		if closeCalled || ex.completeCalled {
			t.Errorf("nothing may mutate on a marker-write failure: close=%v merge=%v", closeCalled, ex.completeCalled)
		}
		if store.data[beadID]["panel_refuted"] != nil {
			t.Errorf("panel_refuted must never be written when the RC was never durably applied, got %v", store.data[beadID])
		}
		msg := err.Error()
		if !strings.Contains(msg, "X") || !strings.Contains(msg, "remains unresolved") {
			t.Errorf("expected the RC-unresolved Block naming X, got: %s", msg)
		}
		if strings.Contains(msg, "aborted with refutation applied") {
			t.Errorf("must NOT read as an abort-with-applied, got: %s", msg)
		}
	})

	t.Run("the union read fails", func(t *testing.T) {
		const specID, beadID = "114-pr03b", "mindspec-114pr.3b"
		root, beadSHA := setupPanelGateRepo(t, specID, beadID)
		store := newFakeMetadataStore()
		store.failGet = func(string) bool { return true }
		store.wire(t)

		closeCalled := false
		closeBeadFn = func(...string) error { closeCalled = true; return nil }

		writePanel(t, root, specID+"-bd03b", panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
			ReviewedHeadSHA: beadSHA,
			Refutations:     []panel.Refutation{{Slot: "X", Round: 1, Reason: "dismissed"}},
		}, map[string]string{
			"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
			"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "REQUEST_CHANGES",
		})

		ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
		_, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"})
		if err == nil {
			t.Fatal("expected a Block when the union read fails")
		}
		if closeCalled || ex.completeCalled {
			t.Errorf("nothing may mutate on a union-read failure: close=%v merge=%v", closeCalled, ex.completeCalled)
		}
		if !strings.Contains(err.Error(), "X") {
			t.Errorf("expected the RC-unresolved Block naming X, got: %s", err.Error())
		}
	})
}

// --- (iv) AC11 CROSS-RUN panel-removed retry (G2) -----------------------

func TestPanelRefuted_CrossRun_PanelRemoved_Refuses(t *testing.T) {
	const specID, beadID = "114-pr04", "mindspec-114pr.4"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore()
	store.wire(t)

	p := panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
		Refutations:     []panel.Refutation{{Slot: "X", Round: 1, Reason: "dismissed"}},
	}
	verdicts := map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
		"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "REQUEST_CHANGES",
	}
	slug := specID + "-bd04"
	writePanel(t, root, slug, p, verdicts)

	// Run 1: marker persists, but the SATISFYING panel_refuted write fails
	// — the bead stays in_progress with a durable, unsatisfied pending.
	store.failMerge = failOnKey("panel_refuted")
	closeCalled := false
	closeBeadFn = func(...string) error { closeCalled = true; return nil }
	ex1 := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadID, specID, "", ex1, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err == nil {
		t.Fatal("run 1 precondition: the panel_refuted write must fail")
	}
	if closeCalled || ex1.completeCalled {
		t.Fatal("run 1 precondition: nothing may have mutated")
	}
	if store.data[beadID]["refutation_pending_entries"] == nil {
		t.Fatal("run 1 precondition: the pending marker must be durably recorded")
	}

	// Run 2: REMOVE panel.json entirely — the every-path reconciliation
	// (no-panel + an unsatisfied pending) must REFUSE, not silently pass
	// through the fail-open §6 no-panel path.
	store.failMerge = nil
	panelDir := filepath.Join(root, "review", slug)
	if err := os.Remove(filepath.Join(panelDir, "panel.json")); err != nil {
		t.Fatalf("removing panel.json: %v", err)
	}
	ex2 := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadID, specID, "", ex2, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err == nil {
		t.Fatal("run 2 must REFUSE: an unsatisfied pending obligation cannot be silently dropped by removing the panel")
	}
	if closeCalled || ex2.completeCalled {
		t.Error("run 2 must not mutate anything (Refuse, not silent no-panel pass-through)")
	}

	// Positive control: RESTORE the panel (RC still present, refutation
	// still on file) — run 3 satisfies and completes.
	writePanel(t, root, slug, p, verdicts)
	ex3 := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadID, specID, "", ex3, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
		t.Fatalf("run 3 (panel restored) must satisfy and complete; got: %v", err)
	}
	if !closeCalled || !ex3.completeCalled {
		t.Error("run 3 must close and merge")
	}
	if store.data[beadID]["panel_refuted"] != true {
		t.Errorf("run 3 must record panel_refuted, got %v", store.data[beadID])
	}
}

// --- (v) AC11 G3 verified-vote-state discharge --------------------------

func TestPanelRefuted_CrossRun_NaturalResolution_Discharges(t *testing.T) {
	const specID, beadID = "114-pr05", "mindspec-114pr.5"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore()
	store.wire(t)

	slug := specID + "-bd05"
	writePanel(t, root, slug, panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
		Refutations:     []panel.Refutation{{Slot: "X", Round: 1, Reason: "dismissed"}},
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
		"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "REQUEST_CHANGES",
	})

	// Run 1: marker persists (X@1), the satisfying write fails.
	store.failMerge = failOnKey("panel_refuted")
	ex1 := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadID, specID, "", ex1, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err == nil {
		t.Fatal("run 1 precondition: the panel_refuted write must fail")
	}
	if store.data[beadID]["refutation_pending_entries"] == nil {
		t.Fatal("run 1 precondition: pending X@1 must be durably recorded")
	}

	// Run 2: the reviewer flips at the SAME round (naturally resolved) — X
	// is now APPROVE at round 1. No new refutation is needed or present.
	store.failMerge = nil
	panelDir := filepath.Join(root, "review", slug)
	if err := os.WriteFile(filepath.Join(panelDir, "X-round-1.json"), []byte(`{"verdict":"APPROVE"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	closeCalled := false
	closeBeadFn = func(...string) error { closeCalled = true; return nil }
	ex2 := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadID, specID, "", ex2, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
		t.Fatalf("run 2 must reconcile via verified DISCHARGE and complete; got: %v", err)
	}
	if !closeCalled || !ex2.completeCalled {
		t.Fatal("run 2 must close and merge")
	}
	if store.data[beadID]["refutation_discharged"] != true {
		t.Errorf("expected refutation_discharged=true, got %v", store.data[beadID])
	}
	entries, _ := store.data[beadID]["refutation_discharged_entries"].([]dischargedEntry)
	if len(entries) != 1 || entries[0].Slot != "X" || entries[0].Round != 1 {
		t.Errorf("expected the discharge entry to name X@1, got %+v", entries)
	}
	if store.data[beadID]["panel_refuted"] != nil {
		t.Errorf("this run applied NO new refutation, so panel_refuted must not be (re)written, got %v", store.data[beadID]["panel_refuted"])
	}

	// Negative twin: the discharge write itself stubbed to fail leaves the
	// bead in_progress.
	store.data[beadID] = map[string]interface{}{
		"refutation_pending_entries": []refutationPendingEntry{{Slot: "X", Round: 1}},
	}
	store.failMerge = failOnKey("refutation_discharged")
	closeCalled = false
	ex3 := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadID, specID, "", ex3, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err == nil {
		t.Fatal("expected Run to fail when the discharge write fails")
	}
	if closeCalled || ex3.completeCalled {
		t.Error("nothing may mutate when the discharge write fails")
	}
}

// --- (va) round-5 item 2: Warn paths must NOT falsely discharge ---------

// TestPanelRefuted_WarnPathDoesNotDischarge drives reconcilePendingRefutations
// directly against an ABANDONED panel (a live Warn-classified panel, per
// gate.go leg 3) whose verdict files STILL show the pending slot as a
// latest-round REQUEST_CHANGES: reconciliation must REFUSE, never discharge,
// proving discharge keys on the re-tally, not on a bare Allow/Warn gate
// action. reconcilePendingRefutations does not even receive the gate's
// action — it independently re-tallies panelReg.Dir via panel.Tally — so
// this ONE assertion covers every Warn-producing shape uniformly (abandoned,
// missing-ref, transient-gitErr all reach PanelGateDecision's Warn branch
// and are pinned individually at the pure-decision layer,
// internal/panel/panel_decision_test.go's TestPanelGateDecision table); a
// full e2e reproduction of missing-ref/transient-gitErr through complete.Run
// is not attempted here because deleting the live bead/<id> ref (the only
// way to produce a genuine MissingRef fact) also breaks the UNRELATED
// doc-sync/ADR merge-base gates that run on the same ref, which would test
// an orthogonal failure rather than this one.
func TestPanelRefuted_WarnPathDoesNotDischarge(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "review", "warn-slug")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePanel(t, root, "warn-slug", panel.Panel{
		BeadID: bp("mindspec-warn"), Round: 1, ExpectedReviewers: 6,
		Abandoned: true, AbandonReason: "max: dropped mid-review",
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
		"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE",
		// X is STILL a live, unresolved RC in the actual verdict files.
		"X-round-1.json": "REQUEST_CHANGES",
	})

	store := newFakeMetadataStore()
	store.data["mindspec-warn"] = map[string]interface{}{
		"refutation_pending_entries": []refutationPendingEntry{{Slot: "X", Round: 1}},
	}
	origGet, origMerge := completeGetMetadataFn, completeMergeMetadataFn
	defer func() { completeGetMetadataFn = origGet; completeMergeMetadataFn = origMerge }()
	completeGetMetadataFn = store.Get
	completeMergeMetadataFn = store.Merge

	panelReg := &panel.Registration{Dir: dir}
	err := reconcilePendingRefutations("mindspec-warn", panelReg, nil)
	if err == nil {
		t.Fatal("expected reconciliation to REFUSE: X is still a live RC in the re-tally, never affirmatively resolved")
	}
	if store.data["mindspec-warn"]["refutation_discharged"] != nil {
		t.Errorf("must NOT falsely discharge a still-live RC merely because the panel's gate action would be Warn: %v", store.data["mindspec-warn"])
	}
}

// --- (vb) round-5 item 1: UNION multi-entry reconciliation ---------------

func TestPanelRefuted_CrossRun_UnionReconcilesAll(t *testing.T) {
	const specID, beadID = "114-pr05b", "mindspec-114pr.5b"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore()
	store.wire(t)

	slug := specID + "-bd05b"
	writePanel(t, root, slug, panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
		Refutations:     []panel.Refutation{{Slot: "X", Round: 1, Reason: "dismissed X"}},
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
		"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "REQUEST_CHANGES",
	})

	// Run 1: marker (X,1) persists, panel_refuted fails.
	store.failMerge = failOnKey("panel_refuted")
	ex1 := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadID, specID, "", ex1, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err == nil {
		t.Fatal("run 1 precondition: the panel_refuted write must fail")
	}

	// Run 2: a round-2 re-panel where X is now APPROVE and a NEW slot B is
	// refuted.
	store.failMerge = nil
	writePanel(t, root, slug, panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 2, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
		Refutations:     []panel.Refutation{{Slot: "B", Round: 2, Reason: "dismissed B"}},
	}, map[string]string{
		// NOTE: avoid pairing "b"/"B" (or any same-letter case pair) as
		// REAL on-disk filenames in the SAME directory — macOS's default
		// case-insensitive-but-case-preserving APFS collides them, unlike
		// the synthetic-Result fixtures in internal/panel's AC12 tests.
		"a-round-2.json": "APPROVE", "c-round-2.json": "APPROVE", "d-round-2.json": "APPROVE",
		"e-round-2.json": "APPROVE", "X-round-2.json": "APPROVE", "B-round-2.json": "REQUEST_CHANGES",
	})

	closeCalled := false
	closeBeadFn = func(...string) error { closeCalled = true; return nil }
	ex2 := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadID, specID, "", ex2, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
		t.Fatalf("run 2 must UNION-reconcile both entries and complete; got: %v", err)
	}
	if !closeCalled || !ex2.completeCalled {
		t.Fatal("run 2 must close and merge")
	}

	meta := store.data[beadID]
	// The run-2 marker write must UNION (not clobber) — both (X,1) and
	// (B,2) must be present in refutation_pending_entries at some point in
	// the run (asserted indirectly via both audits below, since pending is
	// consumed by reconciliation, not read back raw here).
	if meta["panel_refuted"] != true {
		t.Errorf("expected B@2 to be satisfied (panel_refuted=true), got %v", meta["panel_refuted"])
	}
	satisfied, _ := meta["panel_refuted_entries"].([]panel.Refutation)
	foundB := false
	for _, e := range satisfied {
		if e.Slot == "B" && e.Round == 2 {
			foundB = true
		}
	}
	if !foundB {
		t.Errorf("expected panel_refuted_entries to name B@2, got %+v", satisfied)
	}

	if meta["refutation_discharged"] != true {
		t.Errorf("expected X@1 to be verified-discharged, got %v", meta["refutation_discharged"])
	}
	discharged, _ := meta["refutation_discharged_entries"].([]dischargedEntry)
	foundX := false
	for _, e := range discharged {
		if e.Slot == "X" && e.Round == 1 {
			foundX = true
		}
	}
	if !foundX {
		t.Errorf("expected refutation_discharged_entries to name X@1, got %+v", discharged)
	}
}

// --- (vi) AC11 O1#1: GetMetadata read-error ⟹ REFUSE ---------------------

func TestPanelRefuted_GetMetadataError_Refuses(t *testing.T) {
	const specID, beadID = "114-pr06", "mindspec-114pr.6"
	root, _ := setupPanelGateRepo(t, specID, beadID)
	// No panel at all — the panel-gate itself fail-opens; the reconciliation
	// read is what must Refuse.
	store := newFakeMetadataStore()
	store.failGet = func(string) bool { return true }
	store.wire(t)

	closeCalled := false
	closeBeadFn = func(...string) error { closeCalled = true; return nil }

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	_, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"})
	if err == nil {
		t.Fatal("expected Run to REFUSE when the metadata store is unreadable")
	}
	if closeCalled || ex.completeCalled {
		t.Error("an unreadable metadata store cannot prove the bead is obligation-free — nothing may mutate")
	}
}

// --- (vii) Pristine-panel-removed = §6 boundary --------------------------

func TestPanelRefuted_PristineNoPanel_FailsOpen(t *testing.T) {
	const specID, beadID = "114-pr07", "mindspec-114pr.7"
	root, _ := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore() // never carried a refutation_pending
	store.wire(t)

	closeCalled := false
	closeBeadFn = func(...string) error { closeCalled = true; return nil }

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"})
	if err != nil {
		t.Fatalf("a genuinely pristine bead (no panel, no pending) must complete via §6 fail-open; got: %v", err)
	}
	if !closeCalled || !ex.completeCalled || res == nil || !res.BeadClosed {
		t.Fatalf("expected a normal completion, got closeCalled=%v mergeCalled=%v res=%+v", closeCalled, ex.completeCalled, res)
	}
}

// --- (viii) Post-write erasure survival ----------------------------------

// TestPanelRefuted_AuditSurvivesLaterReadError models the fail-closed
// MergeMetadata contract at the complete.go INTEGRATION level: the step-3.75
// reconciliation's panel_refuted write happens BEFORE the step-5.5 doc-skew
// write; a LATER metadata write that (simulating a transient bd-show read
// failure) fails-closed must NOT touch the store at all — so the earlier
// panel_refuted entry survives untouched.
func TestPanelRefuted_AuditSurvivesLaterReadError(t *testing.T) {
	const specID, beadID = "114-pr08", "mindspec-114pr.8"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore()
	// Simulate a fail-closed read failure ONLY on the step-5.5 doc-skew
	// write (never on panel_refuted/refutation_pending_entries): a
	// fail-CLOSED implementation returns an error and does not touch the
	// store — modeled directly since fakeMetadataStore.Merge already never
	// mutates on a failMerge hit.
	store.failMerge = failOnKey("mindspec_doc_skew_reason")
	store.wire(t)

	closeCalled := false
	closeBeadFn = func(...string) error { closeCalled = true; return nil }

	writePanel(t, root, specID+"-bd08", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
		Refutations:     []panel.Refutation{{Slot: "X", Round: 1, Reason: "dismissed"}},
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
		"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "REQUEST_CHANGES",
	})

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	// AllowDocSkew triggers the step-5.5 best-effort write, which this stub
	// fails; that must be a WARNING only, never fatal.
	if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
		t.Fatalf("a best-effort doc-skew write failure must not fail completion; got: %v", err)
	}
	if !closeCalled || !ex.completeCalled {
		t.Fatal("expected the completion to proceed despite the later best-effort write failure")
	}
	if store.data[beadID]["panel_refuted"] != true {
		t.Errorf("panel_refuted must SURVIVE the later failed write, got %v", store.data[beadID])
	}
	if _, ok := store.data[beadID]["mindspec_doc_skew_reason"]; ok {
		t.Error("the doc-skew key must NOT be present — its write was simulated to fail-closed (never touch the store)")
	}
}

// --- (x) Already-closed recovery-path audit -------------------------------

func TestPanelRefuted_RecoveryPath_AuditPresent(t *testing.T) {
	const specID, beadID = "114-pr10", "mindspec-114pr.10"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore()
	store.wire(t)

	// The recovery branch: closeBeadFn errors, but a re-read affirms the
	// bead is ALREADY closed — complete.go's tolerate-and-continue path
	// (complete.go:547-554), which does NOT run the doltCommit/verify
	// durability re-checks the normal close path does.
	closeBeadFn = func(...string) error { return errors.New("bd close: already closed") }
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) { return next.BeadInfo{ID: id, Status: "closed"}, nil }

	writePanel(t, root, specID+"-bd10", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
		Refutations:     []panel.Refutation{{Slot: "X", Round: 1, Reason: "dismissed"}},
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
		"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "REQUEST_CHANGES",
	})

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"})
	if err != nil {
		t.Fatalf("the already-closed recovery path must still complete (reconciliation runs BEFORE it); got: %v", err)
	}
	if res == nil || !res.BeadClosed || !ex.completeCalled {
		t.Fatalf("expected the recovery path to proceed to merge, got res=%+v merge=%v", res, ex.completeCalled)
	}
	if store.data[beadID]["panel_refuted"] != true {
		t.Errorf("expected panel_refuted to be present after the recovery-path completion, got %v", store.data[beadID])
	}
}

// --- (xi) Asymmetry control ------------------------------------------------

// TestPanelRefuted_AbandonAddsNoNewObligation_UnaffectedByPanelRefutedStub:
// an ABANDONED panel (Warn→Allow→merge, but zero AppliedRefutations) still
// completes successfully with only a warning even when the panel_refuted
// write is stubbed to fail — because abandon adds NO new obligation, that
// write is never attempted.
func TestPanelRefuted_AbandonAddsNoNewObligation_UnaffectedByPanelRefutedStub(t *testing.T) {
	const specID, beadID = "114-pr11", "mindspec-114pr.11"
	root, _ := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore()
	store.failMerge = failOnKey("panel_refuted")
	store.wire(t)

	closeCalled := false
	closeBeadFn = func(...string) error { closeCalled = true; return nil }

	writePanel(t, root, specID+"-bd11", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		Abandoned: true, AbandonReason: "max: superseded",
	}, map[string]string{"a-round-1.json": "APPROVE", "b-round-1.json": "REQUEST_CHANGES"})

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
		t.Fatalf("an abandoned panel must complete despite the panel_refuted stub (it adds no new obligation); got: %v", err)
	}
	if !closeCalled || !ex.completeCalled {
		t.Fatal("expected the abandon Warn->Allow->merge path to proceed")
	}
	if store.data[beadID]["panel_refuted"] != nil {
		t.Errorf("abandon must never attempt a panel_refuted write, got %v", store.data[beadID])
	}
	if store.data[beadID]["panel_abandoned"] != true {
		t.Errorf("expected the pre-existing panel_abandoned audit, got %v", store.data[beadID])
	}
}

// --- (xii) HATCH-reconciliation ------------------------------------------

// TestPanelRefuted_HatchStillReconcilesPendingObligation (round-5 item 3 /
// G3): the env-skip, config-disabled, and abandoned hatches all bypass the
// GATE decision but NOT a pre-existing durable refutation_pending
// obligation — reconciliation still runs and discharges it (the fixture
// panel is a clean all-APPROVE round, so the natural-resolution disjunct
// fires). A companion control proves the SAME hatch over a PRISTINE bead
// (no pending) completes normally, writing only panel_gate_skipped — the
// hatch excepts the GATE, not the obligation.
func TestPanelRefuted_HatchStillReconcilesPendingObligation(t *testing.T) {
	seedPending := func(store *fakeMetadataStore, beadID string) {
		store.data[beadID] = map[string]interface{}{
			"refutation_pending_entries": []refutationPendingEntry{{Slot: "X", Round: 1}},
		}
	}

	t.Run("env-skip hatch reconciles a pre-existing pending via discharge", func(t *testing.T) {
		const specID, beadID = "114-pr12a", "mindspec-114pr.12a"
		root, _ := setupPanelGateRepo(t, specID, beadID)
		origSkip := panelSkipEnvFn
		t.Cleanup(func() { panelSkipEnvFn = origSkip })
		panelSkipEnvFn = func() bool { return true }

		store := newFakeMetadataStore()
		seedPending(store, beadID)
		store.wire(t)

		// A clean, fully-resolved panel (X now APPROVE) — the hatch means
		// the GATE never evaluates this, but reconciliation's re-tally does.
		writePanel(t, root, specID+"-bd12a", panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		}, map[string]string{
			"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
			"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "APPROVE",
		})

		ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
		if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
			t.Fatalf("expected the hatch to reconcile (discharge) and complete; got: %v", err)
		}
		if store.data[beadID]["refutation_discharged"] != true {
			t.Errorf("expected the pre-existing pending to be discharged, got %v", store.data[beadID])
		}
	})

	t.Run("env-skip hatch with an unreadable metadata store still REFUSES", func(t *testing.T) {
		const specID, beadID = "114-pr12a2", "mindspec-114pr.12a2"
		root, _ := setupPanelGateRepo(t, specID, beadID)
		origSkip := panelSkipEnvFn
		t.Cleanup(func() { panelSkipEnvFn = origSkip })
		panelSkipEnvFn = func() bool { return true }

		store := newFakeMetadataStore()
		store.failGet = func(string) bool { return true }
		store.wire(t)

		closeCalled := false
		closeBeadFn = func(...string) error { closeCalled = true; return nil }

		ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
		if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err == nil {
			t.Fatal("audit integrity must override the hatch: an unreadable store must REFUSE")
		}
		if closeCalled || ex.completeCalled {
			t.Error("nothing may mutate when the metadata store is unreadable, even on a hatch path")
		}
	})

	t.Run("config-disabled hatch reconciles a pre-existing pending via discharge", func(t *testing.T) {
		const specID, beadID = "114-pr12b", "mindspec-114pr.12b"
		root, _ := setupPanelGateRepo(t, specID, beadID)
		if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"),
			[]byte("enforcement:\n  panel_gate: false\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		store := newFakeMetadataStore()
		seedPending(store, beadID)
		store.wire(t)

		writePanel(t, root, specID+"-bd12b", panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		}, map[string]string{
			"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
			"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "APPROVE",
		})

		ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
		if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
			t.Fatalf("expected the config-disabled hatch to reconcile and complete; got: %v", err)
		}
		if store.data[beadID]["refutation_discharged"] != true {
			t.Errorf("expected the pre-existing pending to be discharged, got %v", store.data[beadID])
		}
	})

	t.Run("abandoned hatch reconciles a pre-existing pending via discharge", func(t *testing.T) {
		const specID, beadID = "114-pr12c", "mindspec-114pr.12c"
		root, _ := setupPanelGateRepo(t, specID, beadID)

		store := newFakeMetadataStore()
		seedPending(store, beadID)
		store.wire(t)

		writePanel(t, root, specID+"-bd12c", panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
			Abandoned: true, AbandonReason: "max: dropped",
		}, map[string]string{
			"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
			"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "APPROVE",
		})

		ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
		if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
			t.Fatalf("expected the abandoned-panel Warn path to reconcile and complete; got: %v", err)
		}
		if store.data[beadID]["refutation_discharged"] != true {
			t.Errorf("expected the pre-existing pending to be discharged, got %v", store.data[beadID])
		}
	})

	// Companion no-obligation control: the SAME env-skip hatch over a
	// PRISTINE bead (no pending) completes and writes ONLY panel_gate_skipped
	// (round-2 item 10 — the hatch excepts the gate, not the obligation).
	t.Run("no-obligation control: pristine bead under the same hatch writes only panel_gate_skipped", func(t *testing.T) {
		const specID, beadID = "114-pr12d", "mindspec-114pr.12d"
		root, _ := setupPanelGateRepo(t, specID, beadID)
		origSkip := panelSkipEnvFn
		t.Cleanup(func() { panelSkipEnvFn = origSkip })
		panelSkipEnvFn = func() bool { return true }

		store := newFakeMetadataStore() // no pre-existing pending
		store.wire(t)

		writePanel(t, root, specID+"-bd12d", panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		}, map[string]string{"a-round-1.json": "REQUEST_CHANGES"}) // would BLOCK without the hatch

		ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
		if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
			t.Fatalf("the hatch must let a pristine bead complete; got: %v", err)
		}
		meta := store.data[beadID]
		if meta["panel_gate_skipped"] != true {
			t.Errorf("expected panel_gate_skipped=true, got %v", meta)
		}
		if meta["refutation_pending_entries"] != nil || meta["panel_refuted"] != nil || meta["refutation_discharged"] != nil {
			t.Errorf("a pristine bead must write NO refutation audits, got %v", meta)
		}
	})
}

// --- (xiii) applied-empty completion writes neither audit -----------------

func TestPanelRefuted_AppliedEmpty_NoAuditWritten(t *testing.T) {
	const specID, beadID = "114-pr13", "mindspec-114pr.13"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore()
	store.wire(t)

	writePanel(t, root, specID+"-bd13", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
	}, approveVerdicts(6))

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
		t.Fatalf("a plain all-APPROVE pass must complete; got: %v", err)
	}
	meta := store.data[beadID]
	if meta["refutation_pending_entries"] != nil || meta["panel_refuted"] != nil || meta["refutation_discharged"] != nil {
		t.Errorf("a genuine clear (no refutation involved) must write NO refutation audits, got %v", meta)
	}
}
