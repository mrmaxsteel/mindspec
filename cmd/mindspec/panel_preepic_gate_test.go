package main

// panel_preepic_gate_test.go: GH #222 — a spec_approve/plan_approve
// panel created BEFORE the epic exists (pre `plan approve`, spec dir
// present ONLY inside the spec branch's worktree) must be resolvable by
// `panel verify`/`panel tally`. `panel create` resolves the panel dir
// through worktree-aware workspace.SpecDir (tier 1:
// .worktrees/worktree-spec-<id>/.mindspec/specs/<id>), so pre-epic it
// writes <worktree-spec-dir>/reviews/<slug>/panel.json;
// findPanelRegistration scans configShowReviewRoots, which before the
// fix enumerated spec dirs only in the MAIN checkout — where the spec
// dir does not exist yet — yielding `no registered panel found` for the
// panel `create` had just registered.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// TestPanelGate_PreEpicWorktreeSpec_VerifyTallyResolve pins the #222
// repro end to end at the CLI seam: in a FLAT-layout root whose spec dir
// exists ONLY under .worktrees/worktree-spec-<id>/ (the pre-epic state
// `spec create` leaves on disk), `panel create --gate spec_approve`
// writes panel.json under the WORKTREE spec dir's reviews/<slug>, and
// `panel verify`/`panel tally` must then RESOLVE that registration —
// never "no registered panel found".
func TestPanelGate_PreEpicWorktreeSpec_VerifyTallyResolve(t *testing.T) {
	resetPanelCreateFlags(t)
	root := mkFlatPanelTestRoot(t)
	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	stubWorktreeListEmpty(t)

	origRevParse := revParseForPanelFn
	t.Cleanup(func() { revParseForPanelFn = origRevParse })
	revParseForPanelFn = stubBareRefRevParse

	// Pre-epic on-disk shape: the spec dir exists ONLY inside the spec
	// worktree (flat tier), NOT under the main checkout's
	// .mindspec/specs/ — exactly what `spec create` produces before
	// `plan approve` lands the spec on main.
	specID := "002-preepic"
	wtSpecDir := filepath.Join(root, ".worktrees", "worktree-spec-"+specID,
		".mindspec", "specs", specID)
	if err := os.MkdirAll(wtSpecDir, 0o755); err != nil {
		t.Fatalf("mkdir worktree spec dir: %v", err)
	}

	slug := "preepic-gate"
	out, err := runPanelCmd("create", slug,
		"--spec", specID,
		"--target", "spec/"+specID,
		"--gate", "spec_approve")
	if err != nil {
		t.Fatalf("pre-epic spec_approve panel create must succeed: %v\noutput=%s", err, out)
	}

	// `create` resolves through worktree-aware workspace.SpecDir, so the
	// registration lands inside the worktree spec dir.
	wantDir := filepath.Join(wtSpecDir, "reviews", slug)
	if _, statErr := os.Stat(filepath.Join(wantDir, panel.FileName)); statErr != nil {
		t.Fatalf("expected panel.json under the WORKTREE spec dir %s: %v", wantDir, statErr)
	}

	// The sharp pin: the SAME lookup verify/tally use must resolve the
	// registration create just wrote (the #222 failure was exactly here).
	reg, err := findPanelRegistration(root, slug)
	if err != nil {
		t.Fatalf("findPanelRegistration must resolve the pre-epic gate panel create just registered (GH #222): %v", err)
	}
	if reg.Dir != wantDir {
		t.Errorf("resolved panel dir = %q, want the worktree spec dir registration %q", reg.Dir, wantDir)
	}
	if reg.Err != nil {
		t.Errorf("resolved registration must parse cleanly: %v", reg.Err)
	}
	if reg.Panel.Gate != "spec_approve" {
		t.Errorf("gate = %q, want %q", reg.Panel.Gate, "spec_approve")
	}

	// CLI-level reach for both subcommands: neither may fail with the
	// #222 symptom. (Either may report BLOCK/incomplete — 0 verdicts are
	// present — which is the CORRECT post-resolution behavior.)
	resetPanelCreateFlags(t)
	verifyOut, verifyErr := runPanelCmd("verify", slug)
	if verifyErr != nil && strings.Contains(verifyErr.Error(), "no registered panel found") {
		t.Fatalf("panel verify must reach the pre-epic gate panel (GH #222): %v\noutput=%s", verifyErr, verifyOut)
	}
	tallyOut, tallyErr := runPanelCmd("tally", slug)
	if tallyErr != nil && strings.Contains(tallyErr.Error(), "no registered panel found") {
		t.Fatalf("panel tally must reach the pre-epic gate panel (GH #222): %v\noutput=%s", tallyErr, tallyOut)
	}
}
