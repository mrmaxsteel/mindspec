package complete

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// TestPanelAdvisory_Passing_WouldPass: a complete, over-threshold panel
// prints "would PASS".
func TestPanelAdvisory_Passing_WouldPass(t *testing.T) {
	root := t.TempDir()
	writePanel(t, root, "093-bd01", panel.Panel{
		BeadID: bp("mindspec-bd01"), Spec: "093", Round: 1, ExpectedReviewers: 3,
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "REQUEST_CHANGES",
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
