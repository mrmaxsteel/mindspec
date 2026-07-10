package complete

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
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

	reg, _, err := panelGate("mindspec-bd04", []string{root}, "", false, nil)
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
		reg, _, err := panelGate("mindspec-bead-match", []string{root}, "", false, nil)
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
		reg, _, err := panelGate("mindspec-bead-mismatch", []string{root}, "", false, nil)
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
		reg, _, err := panelGate("", []string{root}, "", false, nil) // empty bead ID -> fail-open nil
		if err != nil || reg != nil {
			t.Fatalf("expected nil, nil for an empty bead ID (fail-open), got reg=%v err=%v", reg, err)
		}
		if out := gateAwareAdvise(gatesCfg, reg); out != "" {
			t.Errorf("a nil registration must stay silent, got %q", out)
		}
	})

	t.Run("panel-less complete stays advisory-silent and panic-free, gates absent", func(t *testing.T) {
		root := t.TempDir()
		reg, _, err := panelGate("", []string{root}, "", false, nil)
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
		reg, _, err := panelGate("mindspec-global", []string{root}, "", false, nil)
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
