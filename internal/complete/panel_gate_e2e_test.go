package complete

import (
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// Spec 099 Bead 2 (R1+R2+R5): the AUTHORITATIVE in-binary panel gate in
// mindspec complete. These e2e tests stand up a REAL temp git repo (so the
// gate's staleness rev-parse runs against a real bead/<id> ref) and a
// readStubMergeExecutor (so the terminal bead→spec merge is a no-op), the
// same harness the ref-anchored gate tests use. They are RED-on-revert:
// reverting the step-2.25 gate to advisory-only lets the sub-threshold block
// through, and a fail-open complete that no-ops instead of completing fails
// the "actually completes" assertion.

// gateRevParse returns the SHA of a ref in a real repo (used to pin
// reviewed_head_sha == rev-parse(bead/<id>), the freshness HEAD source).
func gateRevParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-parse", ref)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse %s: %v\n%s", ref, err, out)
	}
	return strings.TrimSpace(string(out))
}

// setupPanelGateRepo builds a real repo with main + spec/<id> + bead/<id>
// branches and returns the root and the bead-branch SHA. The spec lifecycle
// seams are stubbed for an implement-mode epic with no live worktree (so
// beadHead is the canonical bead branch, matching the gate's rev-parse
// target). The caller writes the panel.json into <root>/review/<slug>.
func setupPanelGateRepo(t *testing.T, specID, beadID string) (root, beadSHA string) {
	t.Helper()
	saveAndRestore(t)

	specBranch := "spec/" + specID
	beadBranch := "bead/" + beadID

	root = t.TempDir()
	gitRun(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "README.md", "# fixture\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base")
	gitRun(t, root, "branch", specBranch)
	gitRun(t, root, "checkout", "-q", "-b", beadBranch, specBranch)
	writeFile(t, root, "internal/thing/thing.go", "package thing\n\nfunc New() {}\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "impl")
	beadSHA = gateRevParse(t, root, beadBranch)
	gitRun(t, root, "checkout", "-q", "main")

	stubPhaseEpic(t, specID, "epic-"+specID)
	resolveTargetFn = func(string, string) (string, error) { return specID, nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(...string) error { return nil }
	runBDFn = func(...string) ([]byte, error) { return json.Marshal([]bead.BeadInfo{}) }
	findLocalRootFn = func() (string, error) { return root, nil }
	return root, beadSHA
}

// approveVerdicts returns N COMPLETE round-1 verdicts, all APPROVE (Spec 114
// R1: an unresolved REQUEST_CHANGES now blocks the gate exactly like a
// REJECT, so a fixture whose SUBJECT is staleness/dirty-tree/fail-open — not
// RC tolerance — must feed a genuinely clean, all-APPROVE panel). Complete
// (len==N) and over-threshold, so the only remaining block clause is
// staleness / dirty tree.
func approveVerdicts(n int) map[string]string {
	m := map[string]string{}
	letters := "abcdefghij"
	for i := 0; i < n; i++ {
		m[string(letters[i])+"-round-1.json"] = "APPROVE"
	}
	return m
}

// atThresholdWithOneDissentVerdicts returns N COMPLETE round-1 verdicts:
// N-1 APPROVE (over the default N−1 threshold on its own) plus ONE
// REQUEST_CHANGES (Spec 114 R1, AC1's e2e fixture) — the exact shape the
// out-vote used to tolerate: the approve count alone meets the floor, but
// an unresolved dissent must now Block anyway.
func atThresholdWithOneDissentVerdicts(n int) map[string]string {
	m := map[string]string{}
	letters := "abcdefghij"
	for i := 0; i < n-1; i++ {
		m[string(letters[i])+"-round-1.json"] = "APPROVE"
	}
	m[string(letters[n-1])+"-round-1.json"] = "REQUEST_CHANGES"
	return m
}

// subThresholdVerdicts returns N COMPLETE round-1 verdicts that FAIL the N−1
// threshold (2 APPROVE, the rest REQUEST_CHANGES). Complete (so the block is
// the threshold clause, naming the APPROVE tally), not incomplete.
func subThresholdVerdicts() map[string]string {
	m := map[string]string{
		"a-round-1.json": "APPROVE",
		"b-round-1.json": "APPROVE",
	}
	letters := "cdef"
	for _, l := range letters {
		m[string(l)+"-round-1.json"] = "REQUEST_CHANGES"
	}
	return m
}

// TestPanelGate_FailOpen_ActuallyCompletes (R2, the dogfooding-safety pin): a
// complete for a bead with NO panel.json referencing it ACTUALLY COMPLETES —
// closed + merged + exit 0. Not merely "no error": the terminal CompleteBead
// must have run. RED if a panel-less complete short-circuited to a no-op.
func TestPanelGate_FailOpen_ActuallyCompletes(t *testing.T) {
	const specID, beadID = "099-pgopen", "mindspec-099pg.1"
	root, _ := setupPanelGateRepo(t, specID, beadID)
	// No review/ tree at all → no registration → fail open.

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	// AllowDocSkew bypasses the LATER (step-3.5) doc-sync gate so the test
	// isolates the panel gate: the fixture's source-only commit would
	// otherwise trip doc-sync, masking the panel-gate outcome.
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"})
	if err != nil {
		t.Fatalf("panel-less complete must fail open and complete; got: %v", err)
	}
	if !ex.completeCalled {
		t.Fatal("fail-open must ACTUALLY merge (CompleteBead must run), not no-op")
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("fail-open complete must close the bead; res=%+v", res)
	}
}

// TestPanelGate_SubThreshold_Blocks (R5 parity): a registered sub-threshold
// panel for THIS bead BLOCKS — exit non-zero, worktree kept (CompleteBead
// 0×), BeadClosed unset, the message PASSES guard.HasFinalRecoveryLine AND
// contains the raw-`git merge` fence. RED-on-revert: removing the gate lets
// it through.
func TestPanelGate_SubThreshold_Blocks(t *testing.T) {
	const specID, beadID = "099-pgblock", "mindspec-099pg.2"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	writePanel(t, root, specID+"-bd02", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA, // fresh, so the BLOCK is the threshold clause
	}, subThresholdVerdicts())

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err == nil {
		t.Fatal("a sub-threshold panel must BLOCK complete (RED-on-revert to advisory-only)")
	}
	if ex.completeCalled {
		t.Error("block must be PRE-merge: CompleteBead must not run")
	}
	if res != nil {
		t.Errorf("a blocked complete returns nil result; got %+v", res)
	}
	msg := err.Error()
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("block message must end with a recovery line (ADR-0035); got:\n%s", msg)
	}
	if !strings.Contains(msg, "git merge bead/"+beadID) {
		t.Errorf("block message must carry the raw-`git merge` fence (R5); got:\n%s", msg)
	}
	if !strings.Contains(msg, "APPROVE") {
		t.Errorf("block message should name the threshold tally; got:\n%s", msg)
	}
}

// TestPanelGate_RequestChangesBlocksComplete (Spec 114 R1, AC1 end-to-end):
// a fresh, complete, otherwise-passing panel (5 APPROVE meets the default
// N−1 threshold on its own) carrying ONE unresolved REQUEST_CHANGES BLOCKS
// complete.Run exactly like a REJECT — the approve count no longer
// out-votes the dissent. Alongside the existing TestPanelGate_* suite on
// the real-repo harness (setupPanelGateRepo): proves complete.Run returns
// the guard failure having mutated NOTHING (CompleteBead never runs), the
// message ends with an ADR-0035 recovery line, carries the raw-merge
// fence, and re-asserts the AC10 no-advertise predicate on the full error
// text (no refutation incantation, no skip variable).
func TestPanelGate_RequestChangesBlocksComplete(t *testing.T) {
	const specID, beadID = "099-pgrc", "mindspec-099pg.7"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	writePanel(t, root, specID+"-bd07", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA, // fresh, so the BLOCK is the unresolved-RC clause
	}, atThresholdWithOneDissentVerdicts(6))

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err == nil {
		t.Fatal("an unresolved REQUEST_CHANGES must BLOCK complete even when Approves alone meets the threshold (RED-on-revert to R1)")
	}
	if ex.completeCalled {
		t.Error("block must be PRE-merge: CompleteBead must not run (nothing mutated)")
	}
	if res != nil {
		t.Errorf("a blocked complete returns nil result; got %+v", res)
	}

	msg := err.Error()
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("block message must end with a recovery line (ADR-0035); got:\n%s", msg)
	}
	if !strings.Contains(msg, "git merge bead/"+beadID) {
		t.Errorf("block message must carry the raw-`git merge` fence; got:\n%s", msg)
	}
	if !strings.Contains(msg, "5/6 APPROVE") {
		t.Errorf("block message should name the genuine approve tally; got:\n%s", msg)
	}
	// Leg-9.5-distinctive substring (round-1 panel finding S2): under the
	// leg-9.5-disabled mutation this test still failed, but for the WRONG
	// reason (an unrelated doc-sync gate tripped on the fixture's
	// undocumented change, while the panel gate itself would have Allowed at
	// 5/6). Asserting this exact phrase pins that the block is genuinely the
	// unresolved-REQUEST_CHANGES leg, not incidental message overlap with a
	// different gate — RED-on-revert now catches a real leg-9.5 regression.
	if !strings.Contains(msg, "unresolved REQUEST_CHANGES") {
		t.Errorf("block message must be attributable to leg 9.5 (unresolved REQUEST_CHANGES), not an unrelated gate; got:\n%s", msg)
	}
	// AC10 (no-advertise predicate), re-asserted on the FULL e2e error text
	// (not just the pure-decision message): no paste-able refutation
	// incantation (Bead 1 has none yet), and never the skip variable (HC-7).
	for _, s := range []string{"refute", "refutations", "panel refute", panel.SkipPanelEnv} {
		if strings.Contains(msg, s) {
			t.Errorf("block message must NOT contain %q (AC10/HC-7); got:\n%s", s, msg)
		}
	}
}

// TestPanelGate_DifferentBead_DoesNotBlock (panel.ForBead isolation): a
// sub-threshold panel registered for bead X does NOT block complete of bead Y.
func TestPanelGate_DifferentBead_DoesNotBlock(t *testing.T) {
	const specID = "099-pgiso"
	const beadX = "mindspec-099pgx"
	const beadY = "mindspec-099pgy"
	root, _ := setupPanelGateRepo(t, specID, beadY)
	// Panel registers bead X (not the bead we complete).
	writePanel(t, root, specID+"-bdX", panel.Panel{
		BeadID: bp(beadX), Spec: specID, Round: 1, ExpectedReviewers: 6,
	}, subThresholdVerdicts())

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadY, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
		t.Fatalf("a sub-threshold panel for bead X must not block complete of bead Y; got: %v", err)
	}
	if !ex.completeCalled {
		t.Error("different-bead complete must proceed to merge")
	}
}

// TestPanelGate_Freshness_PassAndStaleBlock pins the in-binary HEAD source:
// reviewed_head_sha == rev-parse(bead/<id>) PASSES (threshold met + fresh +
// clean), while a stale reviewed_head_sha BLOCKS. The fresh case asserts the
// recorded SHA equals the live bead-branch SHA — so a future beadHead
// divergence from the hook's bead/<id> source is caught RED-on-divergence.
func TestPanelGate_Freshness_PassAndStaleBlock(t *testing.T) {
	t.Run("fresh_passes", func(t *testing.T) {
		const specID, beadID = "099-pgfresh", "mindspec-099pg.3"
		root, beadSHA := setupPanelGateRepo(t, specID, beadID)
		// Pin: the recorded SHA must be the live bead/<id> ref the gate
		// rev-parses (the same source the hook uses).
		if beadSHA != gateRevParse(t, root, "bead/"+beadID) {
			t.Fatalf("precondition: beadSHA must equal rev-parse(bead/%s)", beadID)
		}
		writePanel(t, root, specID+"-bd03", panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
			ReviewedHeadSHA: beadSHA,
		}, approveVerdicts(6))

		ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
		if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
			t.Fatalf("a fresh, threshold-met, clean panel must pass; got: %v", err)
		}
		if !ex.completeCalled {
			t.Error("fresh+passing complete must merge")
		}
	})

	t.Run("stale_blocks", func(t *testing.T) {
		const specID, beadID = "099-pgstale", "mindspec-099pg.4"
		root, _ := setupPanelGateRepo(t, specID, beadID)
		writePanel(t, root, specID+"-bd04", panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
			// Reviewed a DIFFERENT commit than the live bead tip → stale.
			ReviewedHeadSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		}, approveVerdicts(6)) // threshold MET, so only staleness can block

		ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
		_, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
		if err == nil {
			t.Fatal("a stale reviewed_head_sha must BLOCK (measured on the pre-CommitAll tip)")
		}
		if ex.completeCalled {
			t.Error("stale block must be pre-merge")
		}
		if !strings.Contains(err.Error(), "commits landed after review") {
			t.Errorf("stale block message expected; got:\n%s", err.Error())
		}
	})
}

// TestPanelGate_Hatch_SkipEnv: with MINDSPEC_SKIP_PANEL set, a sub-threshold
// panel does NOT block — and the (passing) flow never names the skip variable.
func TestPanelGate_Hatch_SkipEnv(t *testing.T) {
	const specID, beadID = "099-pgskip", "mindspec-099pg.5"
	root, _ := setupPanelGateRepo(t, specID, beadID)
	origSkip := panelSkipEnvFn
	t.Cleanup(func() { panelSkipEnvFn = origSkip })
	panelSkipEnvFn = func() bool { return true } // hatch set

	writePanel(t, root, specID+"-bd05", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
	}, subThresholdVerdicts())

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err != nil {
		t.Fatalf("MINDSPEC_SKIP_PANEL must skip the gate; got: %v", err)
	}
	if !ex.completeCalled {
		t.Error("skipped gate must let complete proceed to merge")
	}
}

// TestPanelGate_Hatch_ConfigToggle: enforcement.panel_gate=false (passed as
// the panelGateEnabled=false arg) skips the gate even for a sub-threshold
// panel. Exercised directly on panelGate to pin the toggle without writing a
// config file.
func TestPanelGate_Hatch_ConfigToggle(t *testing.T) {
	const specID, beadID = "099-pgcfg", "mindspec-099pg.6"
	root, _ := setupPanelGateRepo(t, specID, beadID)
	origSkip := panelSkipEnvFn
	t.Cleanup(func() { panelSkipEnvFn = origSkip })
	panelSkipEnvFn = func() bool { return false }

	writePanel(t, root, specID+"-bd06", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
	}, subThresholdVerdicts())

	// panelGateEnabled=false → no block.
	reg, err := panelGate(beadID, []string{root}, "", false, nil)
	if err != nil {
		t.Fatalf("enforcement.panel_gate=false must skip the gate; got: %v", err)
	}
	if reg == nil {
		t.Error("the matched registration must still flow for the audit writes")
	}

	// Sanity: with the toggle ON, the same panel BLOCKS (proves the toggle is
	// what suppressed the block, not a mis-wired fixture).
	if _, blockErr := panelGate(beadID, []string{root}, "", true, nil); blockErr == nil {
		t.Error("with the gate enabled the same sub-threshold panel must block")
	}
}

// TestPanelGate_BlockNeverNamesSkipVar (HC-7): the block message must NOT
// name MINDSPEC_SKIP_PANEL — a blocked agent's highest-probability next move
// is pasting a suggested prefix.
func TestPanelGate_BlockNeverNamesSkipVar(t *testing.T) {
	const specID, beadID = "099-pghc7", "mindspec-099pg.7"
	root, _ := setupPanelGateRepo(t, specID, beadID)
	writePanel(t, root, specID+"-bd07", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
	}, subThresholdVerdicts())

	_, err := panelGate(beadID, []string{root}, "", true, nil)
	if err == nil {
		t.Fatal("expected a block")
	}
	if strings.Contains(err.Error(), panel.SkipPanelEnv) {
		t.Errorf("block message must never name %s (HC-7); got:\n%s", panel.SkipPanelEnv, err.Error())
	}
}

// TestPanelGate_SharedDecisionPin proves the in-binary gate and the hook
// reach the IDENTICAL panel.PanelGateDecision over the IDENTICAL
// panel.GateFacts: panelGate resolves facts via panel.ResolveGateFacts with
// the same scanRoot (panel.PanelDirScanRoot) and the same bead/<id> rev-parse
// target the hook uses, so a sub-threshold panel produces the SAME decision
// message body the pure decision yields.
func TestPanelGate_SharedDecisionPin(t *testing.T) {
	const specID, beadID = "099-pgpin", "mindspec-099pg.8"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	writePanel(t, root, specID+"-bd08", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
	}, subThresholdVerdicts())

	_, err := panelGate(beadID, []string{root}, "", true, nil)
	if err == nil {
		t.Fatal("expected a block")
	}
	// Reconstruct the facts exactly as panelGate does and assert the pure
	// decision body is a prefix of the gate's error (the gate only appends a
	// recovery line). This pins that the in-binary path neither rewords nor
	// re-derives the decision.
	regs := panel.ForBead(panel.Scan(root), beadID)
	if len(regs) != 1 {
		t.Fatalf("expected exactly one registration, got %d", len(regs))
	}
	scanRoot := panel.PanelDirScanRoot(regs[0].Dir)
	facts := panel.ResolveGateFacts(regs[0], beadID, scanRoot, panel.GateIO{
		RevParse:      gateRevParseFn,
		Status:        gateStatusFn,
		IsRefNotFound: func(e error) bool { return errors.Is(e, gitutil.ErrRefNotFound) },
		Worktree:      func() string { return "" },
	})
	want := panel.PanelGateDecision(facts)
	if want.Action != panel.Block {
		t.Fatalf("expected a Block decision over the shared facts, got %v", want.Action)
	}
	if !strings.HasPrefix(err.Error(), strings.TrimRight(want.Message, "\n")) {
		t.Errorf("in-binary block must carry the shared decision body verbatim;\nwant prefix:\n%s\ngot:\n%s",
			want.Message, err.Error())
	}
}
