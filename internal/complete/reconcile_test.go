package complete

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// Spec 119 R4 (Bead 1): the merged-unclosed / branch-less forward-reconcile
// matrix (AC-5..AC-9). These tests pin the reconcile-mode branch complete.Run
// takes when a bead has no matching worktree AND its canonical bead/<id> ref
// genuinely does not exist: it must skip exec.MergeBase and
// exec.CompleteBead's merge/branch-cleanup legs (both would operate on the
// now-absent ref) while still evaluating every gate against the landed merge
// commit's own M^1..M diff, then record durable evidence and close.
//
// Most cases here stub mergedUnclosedFn directly (an ordering/facts test via
// MockExecutor, per the plan's "unit tests via executor seams" strategy);
// TestRun_Reconcile_RealPanel_MissingRefWarnCloses below is the one REAL-git
// + REAL-panel e2e case, since the panel gate's own staleness rev-parse is
// NOT routed through the passed-in executor (panel_advisory.go's gateExecutor
// is a separate, hardcoded MindspecExecutor) and so cannot be driven by a
// mock.

// notFoundRevParseExec wraps a MockExecutor so RevParseRef reports the
// absent-ref signal (IsRefNotFound == true) for a specific ref, driving the
// reconcile-detection trigger in Run without a real repo.
func notFoundRevParseExec(mock *executor.MockExecutor, absentRef string) {
	mock.RevParseRefFn = func(workdir, ref string) (string, error) {
		if ref == absentRef {
			return "", errRefNotFoundStub
		}
		return "", nil
	}
	mock.IsRefNotFoundFn = func(err error) bool { return err == errRefNotFoundStub }
}

var errRefNotFoundStub = fakeErr("ref not found (stub)")

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

func stubbedLanded() *lifecycle.LandedMerge {
	return &lifecycle.LandedMerge{
		SHA:          "merge0000000000000000000000000000000000",
		FirstParent:  "firstparent000000000000000000000000000",
		SecondParent: "secondparent00000000000000000000000000",
	}
}

// TestRun_Reconcile_MergedUnclosed_SkipsMergeBaseAndMerge pins the core
// AC-5 ordering facts: no MergeBase call, no CompleteBead call, the doc-sync/
// ADR-divergence gates receive the landed M^1..M range (via ChangedFiles),
// durable evidence is recorded, and the bead closes.
func TestRun_Reconcile_MergedUnclosed_SkipsMergeBaseAndMerge(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	notFoundRevParseExec(mock, "bead/bead-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	landed := stubbedLanded()
	mergedUnclosedFn = func(root, specBranch, beadID string) (*lifecycle.LandedMerge, bool, error) {
		if specBranch != "spec/008-test" || beadID != "bead-1" {
			t.Fatalf("unexpected mergedUnclosedFn args: specBranch=%q beadID=%q", specBranch, beadID)
		}
		return landed, true, nil
	}

	var closed bool
	closeBeadFn = func(ids ...string) error { closed = true; return nil }

	var gotMetaKey string
	var gotMetaVal interface{}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if id == "bead-1" {
			if v, ok := updates["mindspec_reconcile_landed_merge_sha"]; ok {
				gotMetaKey = "mindspec_reconcile_landed_merge_sha"
				gotMetaVal = v
			}
		}
		return nil
	}

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("reconcile must succeed, got: %v", err)
	}
	if !closed {
		t.Error("expected the bead to be closed")
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}
	if len(mock.CallsTo("MergeBase")) != 0 {
		t.Errorf("expected ZERO MergeBase calls in reconcile mode, got %d", len(mock.CallsTo("MergeBase")))
	}
	if len(mock.CallsTo("CompleteBead")) != 0 {
		t.Errorf("expected ZERO CompleteBead calls in reconcile mode, got %d", len(mock.CallsTo("CompleteBead")))
	}
	if gotMetaKey != "mindspec_reconcile_landed_merge_sha" || gotMetaVal != landed.SHA {
		t.Errorf("expected durable evidence naming the landed merge SHA, got key=%q val=%v", gotMetaKey, gotMetaVal)
	}

	changedCalls := mock.CallsTo("ChangedFiles")
	if len(changedCalls) == 0 {
		t.Fatal("expected at least one ChangedFiles call for the per-bead gates")
	}
	for _, c := range changedCalls {
		base := c.Args[0].(string)
		head := c.Args[1].(string)
		if base != landed.FirstParent || head != landed.SHA {
			t.Errorf("ChangedFiles range = (%q, %q), want (%q, %q) [M^1..M]", base, head, landed.FirstParent, landed.SHA)
		}
	}
}

// TestRun_Reconcile_SecondInvocationNoOp: a second `mindspec complete` after
// a successful reconcile (bead now closed) converges to a no-op success —
// same detection path, same evidence write, closeBeadFn tolerates
// already-closed.
func TestRun_Reconcile_SecondInvocationNoOp(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	notFoundRevParseExec(mock, "bead/bead-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	landed := stubbedLanded()
	mergedUnclosedFn = func(root, specBranch, beadID string) (*lifecycle.LandedMerge, bool, error) {
		return landed, true, nil
	}
	// Already closed: closeBeadFn errors, fetchBeadByIDFn (default stub)
	// confirms "closed" — the existing already-closed tolerance path.
	closeBeadFn = func(ids ...string) error { return fakeErr("already closed") }

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("second reconcile invocation must be a no-op success, got: %v", err)
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}
	if len(mock.CallsTo("CompleteBead")) != 0 {
		t.Errorf("expected ZERO CompleteBead calls, got %d", len(mock.CallsTo("CompleteBead")))
	}
}

// TestRun_Reconcile_PanelFree_NoWarning (AC-7): a panel-free bead reconciles
// and closes with no panel output at all (§6 fail-open parity).
//
// RED-on-revert (mindspec-lc12.1 fix-up, panel finding #2): the outward
// success/BeadClosed/no-panel-output assertions alone are satisfied even
// when the reconcile-detection branch (`if wtPath == ""` in complete.go) is
// disabled, because on the ordinary (non-reconcile) path a MockExecutor's
// exec.MergeBase and exec.CompleteBead both default to succeeding with no
// error regardless of whether the bead/<id> ref genuinely exists — the
// normal path "succeeds" against a mock exactly like the reconcile path
// does. The assertions below pin the RECONCILE-SPECIFIC outcome the AC
// actually requires: zero exec.MergeBase calls (base comes from the landed
// merge's first parent, never a merge-base computation against the absent
// ref), zero exec.CompleteBead calls (no branch-cleanup/merge legs — the
// branch is already gone), and the durable
// `mindspec_reconcile_landed_merge_sha` evidence write naming the landed
// SHA. Verified by disabling the branch (`if wtPath == ""` -> `if false`)
// in internal/complete/complete.go: this test FAILS (MergeBase/CompleteBead
// each called once, no reconcile evidence written); restoring the branch
// makes it PASS again.
func TestRun_Reconcile_PanelFree_NoWarning(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	notFoundRevParseExec(mock, "bead/bead-1")

	var buf strings.Builder
	origOut := panelAdvisoryOut
	panelAdvisoryOut = &buf
	t.Cleanup(func() { panelAdvisoryOut = origOut })

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	landed := stubbedLanded()
	mergedUnclosedFn = func(root, specBranch, beadID string) (*lifecycle.LandedMerge, bool, error) {
		return landed, true, nil
	}
	closeBeadFn = func(ids ...string) error { return nil }

	var gotMetaKey string
	var gotMetaVal interface{}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if id == "bead-1" {
			if v, ok := updates["mindspec_reconcile_landed_merge_sha"]; ok {
				gotMetaKey = "mindspec_reconcile_landed_merge_sha"
				gotMetaVal = v
			}
		}
		return nil
	}

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("panel-free reconcile must succeed, got: %v", err)
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}
	if buf.Len() != 0 {
		t.Errorf("expected NO panel output for a panel-free bead, got: %q", buf.String())
	}
	// Reconcile-specific: the skip-legs path was actually taken, not just a
	// mock-tolerant normal path that happens to also succeed.
	if len(mock.CallsTo("MergeBase")) != 0 {
		t.Errorf("expected ZERO MergeBase calls on the panel-free reconcile path, got %d", len(mock.CallsTo("MergeBase")))
	}
	if len(mock.CallsTo("CompleteBead")) != 0 {
		t.Errorf("expected ZERO CompleteBead calls on the panel-free reconcile path (branch already gone), got %d", len(mock.CallsTo("CompleteBead")))
	}
	if gotMetaKey != "mindspec_reconcile_landed_merge_sha" || gotMetaVal != landed.SHA {
		t.Errorf("expected durable evidence naming the landed merge SHA, got key=%q val=%v", gotMetaKey, gotMetaVal)
	}
}

// TestRun_Reconcile_DocSyncFailure_NoEvidenceNotClosed (AC-6 leg b): a
// planted failing gate on the reconcile path exits non-zero naming it, the
// bead is NOT closed, and no reconcile evidence is written.
//
// RED-on-revert (mindspec-lc12.1 fix-up, panel finding #2): the original
// three assertions (error names "doc-sync", not closed, no evidence
// written) all hold even when the reconcile-detection branch is disabled,
// because the ordinary (non-reconcile) path reaches the SAME doc-sync gate
// over a MergeBase-derived base — a MockExecutor's exec.MergeBase defaults
// to succeeding, so the normal path plants the identical doc-sync failure
// for the identical reason. That does not pin the reconcile behavior at
// all. The added assertion below does: in genuine reconcile mode NO
// exec.MergeBase call is ever made (base comes from the landed merge's
// first parent, per complete.go's "No exec.MergeBase call is made on this
// path (AC-5)"). Verified by disabling the branch (`if wtPath == ""` ->
// `if false`) in internal/complete/complete.go: this test FAILS (one
// MergeBase call recorded); restoring the branch makes it PASS again.
func TestRun_Reconcile_DocSyncFailure_NoEvidenceNotClosed(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	notFoundRevParseExec(mock, "bead/bead-1")
	mock.ChangedFilesFn = func(base, head string) ([]string, error) {
		return []string{"internal/foo/bar.go"}, nil
	}

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	mergedUnclosedFn = func(root, specBranch, beadID string) (*lifecycle.LandedMerge, bool, error) {
		return stubbedLanded(), true, nil
	}
	var closed bool
	closeBeadFn = func(ids ...string) error { closed = true; return nil }
	var metaWritten bool
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if _, ok := updates["mindspec_reconcile_landed_merge_sha"]; ok {
			metaWritten = true
		}
		return nil
	}

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected the doc-sync gate to block the reconcile path")
	}
	if !strings.Contains(err.Error(), "doc-sync") {
		t.Errorf("error should name the doc-sync gate, got: %v", err)
	}
	if closed {
		t.Error("bead must NOT be closed when a per-bead gate blocks")
	}
	if metaWritten {
		t.Error("no reconcile evidence should be written when a gate blocks")
	}
	// Reconcile-specific: the doc-sync gate must be reached via the
	// landed-merge's first parent, never a merge-base computation — the
	// discriminator that actually distinguishes this from the ordinary path.
	if len(mock.CallsTo("MergeBase")) != 0 {
		t.Errorf("expected ZERO MergeBase calls on the reconcile path, got %d", len(mock.CallsTo("MergeBase")))
	}
}

// TestRun_Reconcile_NoLandedMergeRefuses (AC-8): no worktree, no bead/<id>
// ref, and no positively identified landed merge → refusal naming the
// missing evidence; the bead is not closed.
func TestRun_Reconcile_NoLandedMergeRefuses(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	notFoundRevParseExec(mock, "bead/bead-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	mergedUnclosedFn = func(root, specBranch, beadID string) (*lifecycle.LandedMerge, bool, error) {
		return nil, false, nil
	}
	var closed bool
	closeBeadFn = func(ids ...string) error { closed = true; return nil }

	_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err == nil {
		t.Fatal("expected a refusal naming the missing landed-merge evidence")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("refusal must carry a recovery line, got: %v", err)
	}
	if closed {
		t.Error("bead must not be closed on the AC-8 refusal")
	}
	if len(mock.CallsTo("MergeBase")) != 0 {
		t.Errorf("expected no MergeBase call before the AC-8 refusal, got %d", len(mock.CallsTo("MergeBase")))
	}
}

// TestRun_Reconcile_ClosedBranchless_SettlesObligations (AC-9): a closed,
// branch-less bead with an unsettled refutation_pending_entries obligation
// settles it (panel_refuted written) WITHOUT branch restoration, and a
// subsequent CheckPendingObligations read (the impl-approve Leg-3 gate
// predicate) returns nil afterward.
func TestRun_Reconcile_ClosedBranchless_SettlesObligations(t *testing.T) {
	saveAndRestore(t)

	root := setupTempRoot(t)
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	mock := newMockExec()
	notFoundRevParseExec(mock, "bead/bead-1")

	resolveTargetFn = func(r, flag string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	runBDFn = func(args ...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }

	mergedUnclosedFn = func(root, specBranch, beadID string) (*lifecycle.LandedMerge, bool, error) {
		return stubbedLanded(), true, nil
	}

	// Already closed (bare `bd close` case tolerated below).
	closeBeadFn = func(ids ...string) error { return fakeErr("already closed") }

	// A small in-memory metadata store so the settle-then-recheck sequence
	// is genuinely observable, mirroring the durable-obligation tests
	// elsewhere in this package.
	store := map[string]interface{}{
		"refutation_pending_entries": []map[string]interface{}{
			{"slot": "a", "round": 1, "reason": "needs fix", "evidence": "see finding"},
		},
	}
	completeGetMetadataFn = func(id string) (map[string]interface{}, error) {
		out := make(map[string]interface{}, len(store))
		for k, v := range store {
			out[k] = v
		}
		return out, nil
	}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		for k, v := range updates {
			store[k] = v
		}
		return nil
	}

	result, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
	if err != nil {
		t.Fatalf("branch-less reconcile with a settleable obligation must succeed, got: %v", err)
	}
	if !result.BeadClosed {
		t.Error("expected BeadClosed=true")
	}
	if _, ok := store["panel_refuted_entries"]; !ok {
		t.Fatal("expected the pending obligation to be settled into panel_refuted_entries")
	}
	if len(mock.CallsTo("CompleteBead")) != 0 {
		t.Errorf("expected ZERO CompleteBead calls (no branch restoration), got %d", len(mock.CallsTo("CompleteBead")))
	}

	if err := CheckPendingObligations("bead-1", completeGetMetadataFn); err != nil {
		t.Errorf("CheckPendingObligations must return nil after settlement, got: %v", err)
	}
}

// TestRun_Reconcile_RealPanel_MissingRefWarnCloses is the one REAL-git +
// REAL-panel e2e in this matrix (AC-5/AC-6 leg a): a bead is merged into its
// spec branch (--no-ff, "Merge bead/<id>") and its branch then deleted — the
// genuine merged-unclosed state. A REAL registered panel targets the bead;
// the panel gate's own staleness rev-parse (routed through a hardcoded
// MindspecExecutor in panel_advisory.go, NOT the passed-in exec) genuinely
// hits the absent ref and must render decision (5) MissingRef -> Warn, never
// a Block, and complete must actually close the bead.
func TestRun_Reconcile_RealPanel_MissingRefWarnCloses(t *testing.T) {
	const specID, beadID = "119-recon", "mindspec-119recon.1"
	specBranch := "spec/" + specID
	beadBranch := "bead/" + beadID

	saveAndRestore(t)
	root := t.TempDir()
	gitRun(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "README.md", "# fixture\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base")
	gitRun(t, root, "checkout", "-q", "-b", specBranch)
	gitRun(t, root, "checkout", "-q", "-b", beadBranch)
	writeFile(t, root, "internal/thing/thing.go", "package thing\n\nfunc New() {}\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "impl")
	beadSHA := gateRevParse(t, root, beadBranch)
	gitRun(t, root, "checkout", "-q", specBranch)
	gitRun(t, root, "merge", "-q", "--no-ff", "-m", "Merge "+beadBranch, beadBranch)
	gitRun(t, root, "branch", "-D", beadBranch)
	gitRun(t, root, "checkout", "-q", "main")

	stubPhaseEpic(t, specID, "epic-"+specID)
	resolveTargetFn = func(string, string) (string, error) { return specID, nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(...string) error { return nil }
	runBDFn = func(...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }
	findLocalRootFn = func() (string, error) { return root, nil }

	// The REAL landed-merge predicate (not the saveAndRestore default stub).
	mergedUnclosedFn = lifecycle.MergedUnclosed

	// ReviewedHeadSHA == the bead's own pre-merge tip: it CORROBORATES
	// (rather than contradicts) the landed-merge-commit-identity
	// predicate's second-parent check. A bogus/stale value here would
	// make lifecycle.FindLandedMerge itself report not-found — a
	// different failure mode than the one under test (the PANEL GATE's
	// OWN decision-5 MissingRef, which fires regardless of staleness).
	writePanel(t, root, specID+"-"+beadID, panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 2,
		ReviewedHeadSHA: beadSHA,
	}, map[string]string{
		"a-round-1.json": "APPROVE",
		"b-round-1.json": "APPROVE",
	})

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"})
	if err != nil {
		t.Fatalf("merged-unclosed reconcile with a registered panel (MissingRef -> Warn) must succeed, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected the bead to close, res=%+v", res)
	}
	if ex.completeCalled {
		t.Error("reconcile mode must NOT call exec.CompleteBead's merge (branch already gone)")
	}
}
