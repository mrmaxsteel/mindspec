package complete

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func bp(s string) *string { return &s }

func writePanel(t *testing.T, root, slug string, p panel.Panel, verdicts map[string]string) {
	t.Helper()
	dir := filepath.Join(root, "review", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(p)
	os.WriteFile(filepath.Join(dir, "panel.json"), data, 0o644)
	for name, v := range verdicts {
		vd, _ := json.Marshal(map[string]string{"verdict": v})
		os.WriteFile(filepath.Join(dir, name), vd, 0o644)
	}
}

// TestPanelAdvisory_NoPanel_NoOutput: with no registered panel, the
// advisory prints nothing and returns nil (Spec 093 Req 13d: no cost when
// unregistered).
func TestPanelAdvisory_NoPanel_NoOutput(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	reg := panelAdvisory("mindspec-bd01", []string{root}, &buf)
	if reg != nil {
		t.Errorf("expected nil registration, got %v", reg)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no advisory output, got %q", buf.String())
	}
}

// TestPanelAdvisory_IncompletePanel_WouldBlock: a registered incomplete
// panel prints the tally + "would BLOCK" line computed by the SAME
// panel.Tally the hook uses.
func TestPanelAdvisory_IncompletePanel_WouldBlock(t *testing.T) {
	root := t.TempDir()
	writePanel(t, root, "093-bd01", panel.Panel{
		BeadID: bp("mindspec-bd01"), Spec: "093", Round: 1, ExpectedReviewers: 6,
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE",
	})
	var buf bytes.Buffer
	reg := panelAdvisory("mindspec-bd01", []string{root}, &buf)
	if reg == nil {
		t.Fatal("expected a matched registration")
	}
	out := buf.String()
	if !strings.Contains(out, "would BLOCK") || !strings.Contains(out, "incomplete") {
		t.Errorf("advisory should warn would-BLOCK incomplete: %q", out)
	}
}

// TestPanelAdvisory_Dissent_WouldBlock: a complete, over-threshold panel
// carrying an unresolved REQUEST_CHANGES prints "would BLOCK" (Spec 114 R1 —
// an unresolved dissent is no longer out-voted by the approve count; renamed
// from TestPanelAdvisory_Passing_WouldPass, whose RC-tolerated "would PASS"
// this spec's R1 removes).
func TestPanelAdvisory_Dissent_WouldBlock(t *testing.T) {
	root := t.TempDir()
	writePanel(t, root, "093-bd01", panel.Panel{
		BeadID: bp("mindspec-bd01"), Spec: "093", Round: 1, ExpectedReviewers: 3,
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "REQUEST_CHANGES",
	})
	var buf bytes.Buffer
	panelAdvisory("mindspec-bd01", []string{root}, &buf)
	if !strings.Contains(buf.String(), "would BLOCK") {
		t.Errorf("advisory should say would-BLOCK on an unresolved dissent: %q", buf.String())
	}
}

// TestPanelAdvisory_Passing_WouldPass: an all-APPROVE, over-threshold panel
// still prints "would PASS" (pinning that Spec 114 R1 only removes RC
// tolerance — a genuinely clean panel is unaffected).
func TestPanelAdvisory_Passing_WouldPass(t *testing.T) {
	root := t.TempDir()
	writePanel(t, root, "093-bd01", panel.Panel{
		BeadID: bp("mindspec-bd01"), Spec: "093", Round: 1, ExpectedReviewers: 3,
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
	})
	var buf bytes.Buffer
	panelAdvisory("mindspec-bd01", []string{root}, &buf)
	if !strings.Contains(buf.String(), "would PASS") {
		t.Errorf("advisory should say would-PASS: %q", buf.String())
	}
}

// TestWritePanelAuditMetadata_SkipEnv writes panel_gate_skipped when the env
// hatch was set for a bead with a registered panel (Spec 093 Req 13b),
// preserving unrelated keys.
func TestWritePanelAuditMetadata_SkipEnv(t *testing.T) {
	origMeta := completeMergeMetadataFn
	origSkip := panelSkipEnvFn
	defer func() { completeMergeMetadataFn = origMeta; panelSkipEnvFn = origSkip }()

	var got map[string]interface{}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		got = updates
		return nil
	}
	panelSkipEnvFn = func() bool { return true }

	reg := &panel.Registration{Dir: "/wt/review/093-bd01", Panel: panel.Panel{BeadID: bp("mindspec-bd01")}}
	writePanelAuditMetadata("mindspec-bd01", reg, nil)

	if got["panel_gate_skipped"] != true {
		t.Errorf("expected panel_gate_skipped=true, got %v", got)
	}
	if _, ok := got["panel_gate_skipped_at"]; !ok {
		t.Error("expected a timestamp")
	}
}

// TestWritePanelAuditMetadata_Abandoned writes panel_abandoned + reason when
// the matched panel.json is abandoned (Spec 093 Req 13e).
func TestWritePanelAuditMetadata_Abandoned(t *testing.T) {
	origMeta := completeMergeMetadataFn
	origSkip := panelSkipEnvFn
	defer func() { completeMergeMetadataFn = origMeta; panelSkipEnvFn = origSkip }()

	var got map[string]interface{}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		got = updates
		return nil
	}
	panelSkipEnvFn = func() bool { return false }

	reg := &panel.Registration{Dir: "/wt/review/093-bd01", Panel: panel.Panel{
		BeadID: bp("mindspec-bd01"), Abandoned: true, AbandonReason: "max: superseded",
	}}
	writePanelAuditMetadata("mindspec-bd01", reg, nil)

	if got["panel_abandoned"] != true {
		t.Errorf("expected panel_abandoned=true, got %v", got)
	}
	if got["panel_abandoned_reason"] != "max: superseded" {
		t.Errorf("expected reason recorded, got %v", got["panel_abandoned_reason"])
	}
}

// TestPanelAdvisory_ReviewerCountNote: a panel whose recorded
// expected_reviewers differs from the config default produces a non-empty
// advisory on the writer (spec 109 R8), and the panelGate Allow/Block
// decision is unaffected by whether the note is computed at all — the two
// calls are independent (reviewerCountAdvisory takes no GateFacts and
// cannot influence panel.PanelGateDecision). panelGateEnabled is passed
// false so the decision itself needs no git-dependent fact resolution,
// isolating this test to the note wiring.
func TestPanelAdvisory_ReviewerCountNote(t *testing.T) {
	root := t.TempDir()
	writePanel(t, root, "109-bd04", panel.Panel{
		BeadID: bp("mindspec-bd04"), Spec: "109", Round: 1, ExpectedReviewers: 6,
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
	})

	reg, err := panelGate("mindspec-bd04", []string{root}, "", false, nil)
	if err != nil {
		t.Fatalf("panelGate returned an error; decision must be unaffected by the advisory: %v", err)
	}
	if reg == nil {
		t.Fatal("expected a matched registration")
	}

	var noteBuf bytes.Buffer
	reviewerCountAdvisory(reg, 4, &noteBuf) // configDefault=4, recorded=6
	out := noteBuf.String()
	if !strings.Contains(out, "recorded 6") || !strings.Contains(out, "config default is 4") {
		t.Errorf("expected a reviewer-count note (recorded 6 vs default 4), got %q", out)
	}

	// Matching config default -> no note (the common case, including every
	// no-panel and unchanged-config-default call site).
	var quietBuf bytes.Buffer
	reviewerCountAdvisory(reg, 6, &quietBuf)
	if quietBuf.Len() != 0 {
		t.Errorf("expected no note when counts match, got %q", quietBuf.String())
	}

	// nil registration (no matched panel) -> no note, no panic.
	var nilBuf bytes.Buffer
	reviewerCountAdvisory(nil, 4, &nilBuf)
	if nilBuf.Len() != 0 {
		t.Errorf("expected no note for a nil registration, got %q", nilBuf.String())
	}
}

// gateAwareAdvise reproduces complete.go's actual gate-aware call site
// (complete.go step 2.25, spec 112 R7) exactly: guarded on reg != nil, since
// PanelGateAdvisoryDefault's arguments deref reg.Panel and panelGate returns
// a nil registration on its fail-open paths (empty bead ID, no registered
// panel — the common panel-less `mindspec complete`); reviewerCountAdvisory
// only runs when PanelGateAdvisoryDefault reports ok (the R7 skip
// carve-outs print nothing).
func gateAwareAdvise(cfg *config.Config, reg *panel.Registration) string {
	var buf bytes.Buffer
	if reg != nil {
		if def, ok := cfg.PanelGateAdvisoryDefault(reg.Panel.Gate, reg.Panel.IsBead()); ok {
			reviewerCountAdvisory(reg, def, &buf)
		}
	}
	return buf.String()
}

// TestPanelAdvisory_GateAwareCompare covers spec 112 R7/AC7: both
// caller-side ReviewerCountNote surfaces compare a recorded
// expected_reviewers against the GATE-APPROPRIATE config default, never the
// flat global one, once gates: is configured — killing the spurious-note
// regression a per-gate config would otherwise create. This test drives
// the complete-side surface (cmd/mindspec's config-show surface gets its
// own TestConfigShow_ReviewerCountNoteGateAware).
func TestPanelAdvisory_GateAwareCompare(t *testing.T) {
	// gatesCfg configures "bead" (resolved sum 6 — incidentally equal to
	// the built-in global default, so a match here proves comparison
	// against bead's OWN default, not a coincidental global match) and
	// "final_review" (resolved sum 12, clearly differing from both) —
	// enough to distinguish "this gate's default" from "another gate's
	// default" and from "the global default".
	six, twelve := 6, 12
	gatesCfg := config.DefaultConfig()
	gatesCfg.Panel.Gates = map[string]config.GatePanel{
		"bead":         {Reviewers: []config.Reviewer{{Model: "claude-opus-4-8", Count: &six}}},
		"final_review": {Reviewers: []config.Reviewer{{Model: "claude-fable-5", Count: &twelve}}},
	}

	t.Run("bead panel matching its own gate default, despite differing from another gate", func(t *testing.T) {
		root := t.TempDir()
		writePanel(t, root, "112-bead-match", panel.Panel{
			BeadID: bp("mindspec-bead-match"), Spec: "112", Round: 1, ExpectedReviewers: 6,
		}, map[string]string{"a-round-1.json": "APPROVE"})
		reg, err := panelGate("mindspec-bead-match", []string{root}, "", false, nil)
		if err != nil || reg == nil {
			t.Fatalf("panelGate: reg=%v err=%v", reg, err)
		}
		if out := gateAwareAdvise(gatesCfg, reg); out != "" {
			t.Errorf("recorded count matches bead's own gate default (6): expected no note, got %q", out)
		}
	})

	t.Run("bead panel mismatching its own gate default", func(t *testing.T) {
		root := t.TempDir()
		writePanel(t, root, "112-bead-mismatch", panel.Panel{
			BeadID: bp("mindspec-bead-mismatch"), Spec: "112", Round: 1, ExpectedReviewers: 4,
		}, map[string]string{"a-round-1.json": "APPROVE"})
		reg, err := panelGate("mindspec-bead-mismatch", []string{root}, "", false, nil)
		if err != nil || reg == nil {
			t.Fatalf("panelGate: reg=%v err=%v", reg, err)
		}
		out := gateAwareAdvise(gatesCfg, reg)
		if !strings.Contains(out, "recorded 4") || !strings.Contains(out, "config default is 6") {
			t.Errorf("expected a mismatch note (recorded 4 vs bead's own default 6), got %q", out)
		}
	})

	t.Run("known non-bead gate matching its own default", func(t *testing.T) {
		reg := &panel.Registration{Panel: panel.Panel{
			Spec: "112", Round: 1, ExpectedReviewers: 12, Gate: "final_review",
		}}
		if out := gateAwareAdvise(gatesCfg, reg); out != "" {
			t.Errorf("recorded count matches final_review's default (12): expected no note, got %q", out)
		}
	})

	t.Run("known non-bead gate mismatching its own default", func(t *testing.T) {
		reg := &panel.Registration{Panel: panel.Panel{
			Spec: "112", Round: 1, ExpectedReviewers: 9, Gate: "final_review",
		}}
		out := gateAwareAdvise(gatesCfg, reg)
		if !strings.Contains(out, "recorded 9") || !strings.Contains(out, "config default is 12") {
			t.Errorf("expected a mismatch note (recorded 9 vs final_review's default 12), got %q", out)
		}
	})

	t.Run("non-bead panel with no recorded gate is skipped while gates are configured", func(t *testing.T) {
		reg := &panel.Registration{Panel: panel.Panel{
			Spec: "112", Round: 1, ExpectedReviewers: 999,
		}}
		if out := gateAwareAdvise(gatesCfg, reg); out != "" {
			t.Errorf("a non-bead, gate-less panel must skip the note while gates: is configured, got %q", out)
		}
	})

	t.Run("unknown recorded gate value is skipped and surfaces no resolver error", func(t *testing.T) {
		reg := &panel.Registration{Panel: panel.Panel{
			BeadID: bp("mindspec-x"), Spec: "112", Round: 1, ExpectedReviewers: 999, Gate: "not-a-real-gate",
		}}
		if out := gateAwareAdvise(gatesCfg, reg); out != "" {
			t.Errorf("an unrecognized recorded gate value must skip the note (never call a resolver with it), got %q", out)
		}
	})

	t.Run("panel-less complete stays advisory-silent and panic-free, gates configured", func(t *testing.T) {
		root := t.TempDir()
		reg, err := panelGate("", []string{root}, "", false, nil) // empty bead ID -> fail-open nil
		if err != nil || reg != nil {
			t.Fatalf("expected nil, nil for an empty bead ID (fail-open), got reg=%v err=%v", reg, err)
		}
		if out := gateAwareAdvise(gatesCfg, reg); out != "" {
			t.Errorf("a nil registration must stay silent, got %q", out)
		}
	})

	t.Run("panel-less complete stays advisory-silent and panic-free, gates absent", func(t *testing.T) {
		root := t.TempDir()
		reg, err := panelGate("", []string{root}, "", false, nil)
		if err != nil || reg != nil {
			t.Fatalf("expected nil, nil for an empty bead ID (fail-open), got reg=%v err=%v", reg, err)
		}
		if out := gateAwareAdvise(config.DefaultConfig(), reg); out != "" {
			t.Errorf("a nil registration must stay silent, got %q", out)
		}
	})

	t.Run("gates absent: every panel compares against the global default exactly as 109", func(t *testing.T) {
		root := t.TempDir()
		writePanel(t, root, "112-global", panel.Panel{
			BeadID: bp("mindspec-global"), Spec: "112", Round: 1, ExpectedReviewers: 4, Gate: "final_review",
		}, map[string]string{"a-round-1.json": "APPROVE"})
		reg, err := panelGate("mindspec-global", []string{root}, "", false, nil)
		if err != nil || reg == nil {
			t.Fatalf("panelGate: reg=%v err=%v", reg, err)
		}
		out := gateAwareAdvise(config.DefaultConfig(), reg)
		if !strings.Contains(out, "recorded 4") || !strings.Contains(out, "config default is 6") {
			t.Errorf("gates-absent must compare against the global default (6) regardless of any recorded gate, got %q", out)
		}
	})
}

// TestWritePanelAuditMetadata_NoPanel_NoWrite: nil registration → no
// metadata write at all (no skip, no abandon).
func TestWritePanelAuditMetadata_NoPanel_NoWrite(t *testing.T) {
	origMeta := completeMergeMetadataFn
	origSkip := panelSkipEnvFn
	defer func() { completeMergeMetadataFn = origMeta; panelSkipEnvFn = origSkip }()

	called := false
	completeMergeMetadataFn = func(string, map[string]interface{}) error { called = true; return nil }
	panelSkipEnvFn = func() bool { return true } // even with skip set...

	writePanelAuditMetadata("mindspec-bd01", nil, nil) // ...nil reg → no write
	if called {
		t.Error("no panel → no metadata write")
	}
}

// --- Spec 115 Bead 1: CheckPendingObligations, the exported check-only ------
// obligation predicate (AC6's predicate half).

// TestPendingObligationPredicate pins CheckPendingObligations' fail-closed
// decode discipline and its exact (slot, round) coverage, independent of
// reconcilePendingRefutations (which now shares the same unexported core but
// SETTLES rather than merely checks).
func TestPendingObligationPredicate(t *testing.T) {
	t.Run("no recorded pending → nil", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{}, nil
		}
		if err := CheckPendingObligations("mindspec-x", getMeta); err != nil {
			t.Errorf("expected nil for a bead with no recorded pending, got: %v", err)
		}
	})

	t.Run("uncovered entry → error naming slot@round", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": []refutationPendingEntry{
					{Slot: "X", Round: 2, Reason: "dismissed", Evidence: "commit abc"},
				},
			}, nil
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		if err == nil {
			t.Fatal("expected an error naming the uncovered obligation")
		}
		if !strings.Contains(err.Error(), "X") || !strings.Contains(err.Error(), "round 2") {
			t.Errorf("expected the error to name slot X @ round 2, got: %v", err)
		}
	})

	t.Run("(slot, round)-exact covered → nil", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": []refutationPendingEntry{
					{Slot: "X", Round: 1, Reason: "dismissed", Evidence: "commit abc"},
				},
				"panel_refuted_entries": []panel.Refutation{
					{Slot: "X", Round: 1, Reason: "dismissed", Evidence: "commit abc"},
				},
			}, nil
		}
		if err := CheckPendingObligations("mindspec-x", getMeta); err != nil {
			t.Errorf("expected nil once the (slot, round) is exactly covered, got: %v", err)
		}
	})

	t.Run("(slot, round) mismatch is NOT covered by a different round", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": []refutationPendingEntry{
					{Slot: "X", Round: 2, Reason: "dismissed"},
				},
				// Covers round 1, not round 2 — a round-1 refutation never
				// clears a later round-2 re-RC on the same slot.
				"panel_refuted_entries": []panel.Refutation{
					{Slot: "X", Round: 1},
				},
			}, nil
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		if err == nil {
			t.Fatal("expected an error: round 1's coverage must not clear round 2's pending obligation")
		}
		if !strings.Contains(err.Error(), "round 2") {
			t.Errorf("expected the error to name round 2, got: %v", err)
		}
	})

	t.Run("metadata read error → error, never decode-as-empty", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return nil, errors.New("simulated bd show read failure")
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		if err == nil {
			t.Fatal("expected a metadata-read error to propagate")
		}
		if !strings.Contains(err.Error(), "could not be read") {
			t.Errorf("expected a read-error message, got: %v", err)
		}
	})

	t.Run("corrupt refutation_pending_entries → error, never decode-as-empty", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": "corrupt-not-an-array",
			}, nil
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		if err == nil {
			t.Fatal("expected a corrupt-entries error to propagate")
		}
		if !strings.Contains(err.Error(), "could not be decoded") {
			t.Errorf("expected a decode-error message, got: %v", err)
		}
	})

	t.Run("corrupt panel_refuted_entries → error, never read as nothing-covered", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": []refutationPendingEntry{{Slot: "X", Round: 1}},
				"panel_refuted_entries":      "corrupt-not-an-array",
			}, nil
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		if err == nil {
			t.Fatal("expected a corrupt-audit error to propagate")
		}
		if !strings.Contains(err.Error(), "could not be decoded") {
			t.Errorf("expected a decode-error message, got: %v", err)
		}
	})

	t.Run("shape-invalid entry (empty slot) → error", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": []refutationPendingEntry{{Slot: "", Round: 1}},
			}, nil
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		if err == nil {
			t.Fatal("expected a shape-invalid-entry error (empty slot)")
		}
		if !strings.Contains(err.Error(), "malformed refutation_pending entry") {
			t.Errorf("expected the malformed-entry message, got: %v", err)
		}
	})

	t.Run("shape-invalid entry (round < 1) → error", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": []refutationPendingEntry{{Slot: "X", Round: 0}},
			}, nil
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		if err == nil {
			t.Fatal("expected a shape-invalid-entry error (round < 1)")
		}
		if !strings.Contains(err.Error(), "malformed refutation_pending entry") {
			t.Errorf("expected the malformed-entry message, got: %v", err)
		}
	})

	// --- Spec 115 Bead 1 R8 fix (round 2): present-but-JSON-null must ------
	// error, never decode-as-empty like an absent key. A JSON document
	// containing `"refutation_pending_entries": null` decodes (via
	// encoding/json into map[string]interface{}) to a map that HAS the key
	// with a nil value — exactly modeled here by a map literal with an
	// explicit `nil` value, which is indistinguishable from a real
	// bd.GetMetadata JSON-null round-trip from the comma-ok idiom's
	// perspective.

	t.Run("present-but-null refutation_pending_entries → error, never decode-as-empty", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": nil,
			}, nil
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		if err == nil {
			t.Fatal("expected a present-but-null refutation_pending_entries to error (RED-on-revert: fails if the comma-ok guard is removed)")
		}
		if !strings.Contains(err.Error(), "present-but-null") {
			t.Errorf("expected a present-but-null message, got: %v", err)
		}
	})

	t.Run("absent refutation_pending_entries key → still nil (no-op preserved)", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"some_other_key": "irrelevant",
			}, nil
		}
		if err := CheckPendingObligations("mindspec-x", getMeta); err != nil {
			t.Errorf("expected nil for a genuinely absent key, got: %v", err)
		}
	})

	t.Run("present-but-null panel_refuted_entries → error, never read as nothing-covered", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": []refutationPendingEntry{{Slot: "X", Round: 1}},
				"panel_refuted_entries":      nil,
			}, nil
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		if err == nil {
			t.Fatal("expected a present-but-null panel_refuted_entries to error (RED-on-revert: fails if the comma-ok guard is removed)")
		}
		if !strings.Contains(err.Error(), "present-but-null") {
			t.Errorf("expected a present-but-null message, got: %v", err)
		}
	})

	// --- Spec 115 Bead 1 R8 round-3 fix (structural): the round-2 fix ------
	// validated panel_refuted_entries AFTER the len(pending)==0 early
	// return, so a present-but-corrupt panel_refuted_entries value passed
	// silently whenever refutation_pending_entries was absent/empty. These
	// pin the validation now running UNCONDITIONALLY before that early
	// return — RED-on-revert: moving the panel_refuted_entries validation
	// back after the early return makes these two fail.

	t.Run("empty pending + present-but-null panel_refuted_entries → error (RED-on-revert)", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": []refutationPendingEntry{},
				"panel_refuted_entries":      nil,
			}, nil
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		if err == nil {
			t.Fatal("expected a present-but-null panel_refuted_entries to error even when pending is empty (RED-on-revert: passes only if the validation is moved back after the len(pending)==0 early return)")
		}
		if !strings.Contains(err.Error(), "present-but-null") {
			t.Errorf("expected a present-but-null message, got: %v", err)
		}
	})

	t.Run("absent pending + corrupt panel_refuted_entries → error (RED-on-revert)", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"panel_refuted_entries": "corrupt-not-an-array",
			}, nil
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		if err == nil {
			t.Fatal("expected a corrupt panel_refuted_entries to error even when refutation_pending_entries is absent (RED-on-revert: passes only if the validation is moved back after the len(pending)==0 early return)")
		}
		if !strings.Contains(err.Error(), "could not be decoded") {
			t.Errorf("expected a decode-error message, got: %v", err)
		}
	})

	// --- No-false-refuse pins: valid present-empty arrays on both keys ----
	// must still no-op, so the structural move above does not turn a
	// pristine or valid-but-empty bead into a false refuse.

	t.Run("both keys absent → nil (pristine bead, no false-refuse)", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{}, nil
		}
		if err := CheckPendingObligations("mindspec-x", getMeta); err != nil {
			t.Errorf("expected nil for a bead with both keys absent, got: %v", err)
		}
	})

	t.Run("valid empty pending + valid empty refuted → nil (no false-refuse)", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": []refutationPendingEntry{},
				"panel_refuted_entries":      []panel.Refutation{},
			}, nil
		}
		if err := CheckPendingObligations("mindspec-x", getMeta); err != nil {
			t.Errorf("expected nil for valid present-but-empty arrays on both keys, got: %v", err)
		}
	})
}

// --- Spec 115 Bead 1: AC7(a) — complete.Run re-gates an already-closed -----
// orphan and converges after a durable refutation.

// TestCompleteRun_RegatesAlreadyClosedOrphan pins EXISTING behavior (no
// production change accompanies this test): a bead that is ALREADY CLOSED
// (e.g. via a bare `bd close`, the mindspec-4gsz lifecycle-bypass — recovered
// by re-running `mindspec complete <bead>`) still passes through the
// AUTHORITATIVE panel gate at step 2.25 (complete.go:387) BEFORE step-4's
// already-closed tolerance (complete.go:576-579) ever runs: an unresolved
// round-1 REQUEST_CHANGES Blocks with no merge, even though the bead is
// already closed. Once that RC is durably refuted (Spec 114 R2: the
// refutation_pending marker + the panel_refuted reconciliation), the SAME
// already-closed bead converges — complete.Run succeeds with the
// already-closed warning and performs the bead→spec merge.
func TestCompleteRun_RegatesAlreadyClosedOrphan(t *testing.T) {
	const specID, beadID = "115-ac7a", "mindspec-115ac7a.1"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	store := newFakeMetadataStore()
	store.wire(t)

	// The bead is ALREADY CLOSED: closeBeadFn errors (a second `bd close`
	// on an already-closed bead is not idempotent at the bd layer) and the
	// tolerance re-read reports "closed" — the exact shape complete.go's
	// step-4 already-closed tolerance exists to accept.
	closeBeadFn = func(...string) error { return fmt.Errorf("bead already closed") }
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		return next.BeadInfo{ID: id, Status: "closed"}, nil
	}

	slug := specID + "-bd01"
	verdicts := map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
		"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "X-round-1.json": "REQUEST_CHANGES",
	}
	writePanel(t, root, slug, panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
	}, verdicts)

	// Round 1: the unresolved REQUEST_CHANGES Blocks at the gate — BEFORE
	// step-4's already-closed tolerance ever runs. Nothing mutates.
	ex1 := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	if _, err := Run(root, beadID, specID, "", ex1, CompleteOpts{AllowDocSkew: "test: e2e fixture"}); err == nil {
		t.Fatal("expected the panel gate to Block on the unresolved round-1 REQUEST_CHANGES")
	}
	if ex1.completeCalled {
		t.Fatal("a Block must perform no merge, even for an already-closed bead")
	}
	if store.data[beadID]["panel_refuted"] != nil {
		t.Fatal("no reconciliation may occur before the gate Block")
	}

	// Durably refute the round-1 RC (Spec 114 R2 escape): panel.json now
	// records an audited Refutation for slot X / round 1.
	writePanel(t, root, slug, panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
		Refutations:     []panel.Refutation{{Slot: "X", Round: 1, Reason: "max: dismissed", Evidence: "commit abc123"}},
	}, verdicts)

	ex2 := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	res, err := Run(root, beadID, specID, "", ex2, CompleteOpts{AllowDocSkew: "test: e2e fixture"})
	if err != nil {
		t.Fatalf("expected success once the RC is durably refuted, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed=true (already-closed tolerance), got %+v", res)
	}
	if !ex2.completeCalled {
		t.Fatal("expected the terminal bead->spec merge to run after the settled re-gate")
	}
	if store.data[beadID]["panel_refuted"] != true {
		t.Errorf("expected the reconciliation to durably record panel_refuted, got %v", store.data[beadID])
	}
}
