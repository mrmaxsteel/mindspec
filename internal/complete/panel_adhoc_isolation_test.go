package complete

// panel_adhoc_isolation_test.go — spec 123 Bead 4 (R8d / AC-16): an
// ad-hoc panel.json planted under .mindspec/reviews/ must NEVER
// influence mindspec complete's panel gate (ADR-0037 scope guard).
// Bead 4's tally-reach change appends the workspace dir (.mindspec) to
// configShowReviewRoots (cmd/mindspec/config.go) — a DIFFERENT,
// complete-agnostic reader `panel tally`/`config show` use — but
// panelGateRoots (this package) is deliberately left UNTOUCHED, so this
// test pins that the two readers never converge and an ad-hoc panel can
// never gate a merge.
//
// RED-on-revert (FX-2): the planted ad-hoc panel carries the SAME
// bead_id the complete-gate evaluates, so panel.ForBead cannot silently
// discard it — the ONLY thing keeping it out of the decision is
// panelGateRoots NOT including .mindspec/reviews. If that root list is
// ever widened to include the workspace dir (the tally-reach change
// wrongly copied into the gate), ForBead WOULD match this REJECT panel
// and, since "a Block from any matched panel wins" (R5), the "must
// Allow" assertion below goes RED. This makes the test a genuine
// isolation proof rather than a vacuous one masked by nil-bead
// filtering. Confirmed red-on-that-revert during development by
// temporarily appending workspace.MindspecDir(root) to panelGateRoots.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// writeAdHocRejectPanel plants a REJECT-verdict ad-hoc panel.json at
// <root>/.mindspec/reviews/<slug>/ — the location Bead 4's `panel create
// --gate adhoc` (no --spec) now produces on a flat tree, and the
// location configShowReviewRoots now makes talliable via `panel
// tally`/`config show`. The panel carries beadID (the gate's evaluated
// bead) and a fresh reviewedHeadSHA so that IF the gate ever scanned it,
// ForBead would match it and the REJECT verdict (not staleness) would be
// the blocking clause — the exact regression AC-16 must catch.
func writeAdHocRejectPanel(t *testing.T, root, slug, beadID, reviewedHeadSHA string) {
	t.Helper()
	dir := filepath.Join(root, ".mindspec", "reviews", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := panel.Panel{
		BeadID:            bp(beadID),
		Round:             1,
		ExpectedReviewers: 1,
		Gate:              "adhoc",
		ReviewedHeadSHA:   reviewedHeadSHA,
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, panel.FileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	vd, err := json.Marshal(map[string]string{"verdict": "REJECT"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "r-round-1.json"), vd, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPanelGate_AdHocPanelNeverGates (AC-16) proves gate isolation two
// ways: (1) panelGateRoots is byte-identical (reflect.DeepEqual) whether
// or not an ad-hoc panel.json exists under .mindspec/reviews/ — the
// planted directory never enters the scanned root set; (2) an
// otherwise-Allow bead-panel gate decision is UNCHANGED after planting a
// REJECT-verdict ad-hoc panel carrying the SAME bead_id — the ad-hoc
// panel has no effect on the Allow/Block outcome BECAUSE the gate never
// scans its directory, not because ForBead filters it out.
func TestPanelGate_AdHocPanelNeverGates(t *testing.T) {
	const specID, beadID = "123-adhoc-iso", "mindspec-123ai.4"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)

	// A genuine bead-scoped panel that PASSES (all-approve, over
	// threshold) so the baseline decision is a clean Allow — any
	// regression toward Block from the ad-hoc plant below is loud.
	writePanel(t, root, specID+"-bd04", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 3,
		ReviewedHeadSHA: beadSHA,
	}, approveVerdicts(3))

	rootsBefore := panelGateRoots(root, "", specID)
	if _, err := panelGate(beadID, rootsBefore, "", true, nil); err != nil {
		t.Fatalf("baseline bead panel must Allow before planting the ad-hoc panel: %v", err)
	}

	// Plant an ad-hoc REJECT-verdict panel under .mindspec/reviews/,
	// carrying the SAME bead_id + a fresh SHA so ForBead cannot mask it
	// and the REJECT is the operative clause if the gate ever scanned it.
	writeAdHocRejectPanel(t, root, "adr-review", beadID, beadSHA)

	rootsAfter := panelGateRoots(root, "", specID)
	if !reflect.DeepEqual(rootsBefore, rootsAfter) {
		t.Fatalf("panelGateRoots must be byte-identical whether or not an ad-hoc panel exists under .mindspec/reviews/:\nbefore=%v\nafter=%v", rootsBefore, rootsAfter)
	}

	// Sanity-check the plant is genuinely gating-capable: pointed at the
	// workspace-dir root DIRECTLY (the revert this test guards against),
	// the same REJECT panel MUST Block — proving the Allow below is root
	// exclusion, not a ForBead no-op or a toothless fixture.
	adHocRoot := filepath.Join(root, ".mindspec")
	if _, err := panelGate(beadID, []string{adHocRoot}, "", true, nil); err == nil {
		t.Fatal("fixture check: the planted ad-hoc REJECT panel must Block when its root IS scanned — otherwise the isolation assertion below is vacuous")
	}

	if _, err := panelGate(beadID, rootsAfter, "", true, nil); err != nil {
		t.Fatalf("an ad-hoc panel under .mindspec/reviews/ must never gate mindspec complete (ADR-0037 scope); got: %v", err)
	}
}
