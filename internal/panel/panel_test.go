package panel

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a fixture helper: writes content at root/rel, creating
// parent dirs.
func writeFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const beadPanelJSON = `{
  "bead_id": "mindspec-x.1",
  "spec": "093-skills-thin-down",
  "target": "bead/mindspec-x.1",
  "round": 1,
  "expected_reviewers": 6,
  "reviewed_head_sha": "abc1234abc1234abc1234abc1234abc1234abc12"
}`

func TestScan_FindsRegisteredPanels(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/bead-x1-panel/panel.json", beadPanelJSON)
	writeFile(t, root, "review/other-panel/panel.json", `{"bead_id": null, "spec": "s", "target": "spec/s", "round": 2, "expected_reviewers": 6, "reviewed_head_sha": "ffff"}`)

	regs := Scan(root)
	if len(regs) != 2 {
		t.Fatalf("expected 2 registrations, got %d: %+v", len(regs), regs)
	}
	// Sorted by dir: bead-x1-panel < other-panel.
	if regs[0].Slug() != "bead-x1-panel" || regs[1].Slug() != "other-panel" {
		t.Errorf("unexpected order: %q, %q", regs[0].Slug(), regs[1].Slug())
	}
	if regs[0].Err != nil {
		t.Fatalf("unexpected parse error: %v", regs[0].Err)
	}
	p := regs[0].Panel
	if !p.IsBead() || *p.BeadID != "mindspec-x.1" {
		t.Errorf("bead_id not parsed: %+v", p)
	}
	if p.ExpectedReviewers != 6 || p.Round != 1 || p.ReviewedHeadSHA == "" {
		t.Errorf("schema fields not parsed: %+v", p)
	}
	if regs[1].Panel.IsBead() {
		t.Errorf("null bead_id must parse as non-bead: %+v", regs[1].Panel)
	}
}

// TestScan_FindsCoLocatedReviewsPanels (Spec 106 Bead 4, AC13): Scan globs the
// spec-scoped `reviews/<slug>/panel.json` convention as well as the repo-root
// `review/<slug>/panel.json` one, so handing it a spec-dir root surfaces the
// co-located panel. The literal `review/` must not substring-match `reviews/`.
func TestScan_FindsCoLocatedReviewsPanels(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "106-x")
	// Co-located panel under <spec-dir>/reviews/<slug>/.
	writeFile(t, specDir, "reviews/106-bead4/panel.json", beadPanelJSON)
	// A repo-root review/ panel under the repo root.
	writeFile(t, root, "review/106-root/panel.json", beadPanelJSON)

	// Scanning the spec dir finds ONLY the co-located reviews/ panel.
	specRegs := Scan(specDir)
	if len(specRegs) != 1 || specRegs[0].Slug() != "106-bead4" {
		t.Fatalf("spec-dir scan should find the co-located reviews/ panel; got %+v", specRegs)
	}

	// Scanning the repo root finds ONLY the root review/ panel (reviews/ lives
	// under the spec dir, not the repo root).
	rootRegs := Scan(root)
	if len(rootRegs) != 1 || rootRegs[0].Slug() != "106-root" {
		t.Fatalf("repo-root scan should find the root review/ panel; got %+v", rootRegs)
	}

	// The union (both roots) dedupes to both distinct panels.
	union := Scan(root, specDir)
	if len(union) != 2 {
		t.Fatalf("union scan should find both panels, got %d: %+v", len(union), union)
	}
}

func TestScan_LegacyBriefOnlyDirIsUnregistered(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/legacy-panel/BRIEF.md", "# brief")
	writeFile(t, root, "review/legacy-panel/claude-a-round-1.json", `{"verdict":"APPROVE"}`)

	if regs := Scan(root); len(regs) != 0 {
		t.Fatalf("legacy BRIEF-only dir must not register (HC-4 fail-open), got %+v", regs)
	}
}

func TestScan_MissingRootsAndEmptyRootSkipped(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/p/panel.json", beadPanelJSON)

	regs := Scan("", filepath.Join(root, "does-not-exist"), root)
	if len(regs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(regs))
	}
}

func TestScan_DedupesOverlappingRoots(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/p/panel.json", beadPanelJSON)

	// Same root twice, plus a non-clean alias of it.
	alias := filepath.Join(root, ".", "subdir", "..")
	if err := os.MkdirAll(filepath.Join(root, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	regs := Scan(root, root, alias)
	if len(regs) != 1 {
		t.Fatalf("expected deduped single registration, got %d: %+v", len(regs), regs)
	}
}

func TestScan_MalformedPanelJSONStillRegistersWithErr(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/broken/panel.json", `{not json`)

	regs := Scan(root)
	if len(regs) != 1 {
		t.Fatalf("malformed panel.json must still register, got %d", len(regs))
	}
	if regs[0].Err == nil {
		t.Fatal("expected Err on malformed registration")
	}
}

func TestForBead(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/mine/panel.json", beadPanelJSON)
	writeFile(t, root, "review/theirs/panel.json", `{"bead_id":"mindspec-y.9","spec":"s","target":"bead/mindspec-y.9","round":1,"expected_reviewers":6,"reviewed_head_sha":"a"}`)
	writeFile(t, root, "review/null-target/panel.json", `{"bead_id":null,"spec":"s","target":"spec/s","round":1,"expected_reviewers":6,"reviewed_head_sha":"a"}`)
	writeFile(t, root, "review/broken/panel.json", `{`)

	regs := Scan(root)
	got := ForBead(regs, "mindspec-x.1")
	if len(got) != 1 || got[0].Slug() != "mine" {
		t.Fatalf("ForBead mismatch: %+v", got)
	}
	if got := ForBead(regs, ""); got != nil {
		t.Fatalf("empty bead id must match nothing, got %+v", got)
	}
}

func TestApproveThreshold(t *testing.T) {
	cases := []struct {
		expected int
		want     int
	}{
		{6, 5},  // default panel: 5-of-6
		{3, 2},  // DQ5 parameterization fixture
		{1, 0},  // degenerate but well-defined
		{0, 0},  // malformed registration: never a free pass
		{-2, 0}, // malformed registration
	}
	for _, c := range cases {
		p := Panel{ExpectedReviewers: c.expected}
		if got := p.ApproveThreshold(); got != c.want {
			t.Errorf("ApproveThreshold(expected_reviewers=%d) = %d, want %d", c.expected, got, c.want)
		}
	}
}

// TestApproveThreshold_InterpretsRecordedExpr (Spec 109 AC4, ADR-0037 §3
// amendment): ApproveThreshold is the SOLE interpreter of the recorded
// ApproveThresholdExpr. Absent/empty and "n-1" (case-insensitive) both
// resolve to N−1; an in-range integer string overrides it; anything else —
// out-of-range integer or unparseable — falls back to N−1, so a recorded 0
// never yields a free-pass threshold of 0.
func TestApproveThreshold_InterpretsRecordedExpr(t *testing.T) {
	cases := []struct {
		name     string
		expected int
		expr     string
		want     int
	}{
		{"absent field → N-1", 6, "", 5},
		{"lowercase n-1 → N-1", 6, "n-1", 5},
		{"uppercase N-1 → N-1", 6, "N-1", 5},
		{"mixed-case whitespace n-1 → N-1", 6, "  N-1  ", 5},
		{"in-range integer overrides", 6, "3", 3},
		{"integer at lower bound 1", 6, "1", 1},
		{"integer at upper bound N", 6, "6", 6},
		{"recorded 0 falls back to N-1, never a free pass", 6, "0", 5},
		{"negative integer falls back to N-1", 6, "-1", 5},
		{"integer above N falls back to N-1", 6, "7", 5},
		{"unparseable value falls back to N-1", 6, "banana", 5},
		{"in-range integer, smaller panel", 3, "2", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := Panel{ExpectedReviewers: c.expected, ApproveThresholdExpr: c.expr}
			if got := p.ApproveThreshold(); got != c.want {
				t.Errorf("ApproveThreshold(expected=%d, expr=%q) = %d, want %d", c.expected, c.expr, got, c.want)
			}
		})
	}
}

// TestPanelGateDecision_ConfigDefaultDoesNotAlterDecision (Spec 109 AC5):
// (a) PanelGateDecision over fixed GateFacts returns the same Allow/Block
// regardless of any "config default" — demonstrated by varying the value fed
// only to the unrelated, config-free ReviewerCountNote helper and confirming
// PanelGateDecision, recomputed on the identical facts, never changes (its
// signature carries no config input at all).
// (b) ReviewerCountNote returns "" on a match and a non-empty advisory on a
// mismatch.
// (c) A panel whose resolved threshold is 0 (ExpectedReviewers=1, absent
// ApproveThresholdExpr) still returns Block, pinning the gate-side
// `threshold > 0` guard (gate.go) as a defense that survives independently
// of the record-side N−1 fallback.
func TestPanelGateDecision_ConfigDefaultDoesNotAlterDecision(t *testing.T) {
	sha := "abc1234def5678abc1234def5678abc1234def56"
	p := &Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha}
	// 5 APPROVE + 1 neutral dissent = 6/6 present (Complete()), 5/6 APPROVE
	// meets the N-1 threshold — mirrors the existing "threshold met" row in
	// panel_decision_test.go.
	r := result(p, 5, 0, 1, nil, nil)
	r.Verdicts = append(r.Verdicts,
		Verdict{File: "z-round-1.json", Slot: "z", Round: 1, Verdict: VerdictRequestChanges})
	facts := GateFacts{
		BeadID:  "mindspec-bd01",
		Reg:     regn("/wt/review/slug"),
		Res:     r,
		HeadSHA: sha,
	}

	want := PanelGateDecision(facts)
	if want.Action != Allow {
		t.Fatalf("precondition: expected Allow with 5/6 approves, got %+v", want)
	}

	for _, configDefault := range []int{3, 6, 10} {
		note := ReviewerCountNote(p.ExpectedReviewers, configDefault)
		if configDefault == p.ExpectedReviewers && note != "" {
			t.Errorf("ReviewerCountNote(%d, %d) = %q, want empty on match", p.ExpectedReviewers, configDefault, note)
		}
		if configDefault != p.ExpectedReviewers && note == "" {
			t.Errorf("ReviewerCountNote(%d, %d) = empty, want non-empty on mismatch", p.ExpectedReviewers, configDefault)
		}

		got := PanelGateDecision(facts)
		if got != want {
			t.Errorf("PanelGateDecision changed after ReviewerCountNote(_, %d): got %+v, want %+v", configDefault, got, want)
		}
	}

	// (c) resolved-threshold-0 pin.
	p0 := &Panel{BeadID: ptr("mindspec-y1"), Round: 1, ExpectedReviewers: 1}
	if th := p0.ApproveThreshold(); th != 0 {
		t.Fatalf("precondition: ApproveThreshold() = %d, want 0", th)
	}
	// Approves=1 so the SINGLE expected reviewer's verdict makes the round
	// Complete() and the decision reaches branch (10) — the threshold check
	// itself, not the earlier "incomplete" short-circuit — where
	// threshold=0 must still Block per gate.go's `threshold > 0` guard.
	facts0 := GateFacts{
		BeadID: "mindspec-y1",
		Reg:    regn("/wt/review/y-slug"),
		Res:    result(p0, 1, 0, 1, nil, nil),
	}
	got0 := PanelGateDecision(facts0)
	if got0.Action != Block {
		t.Fatalf("resolved-0-threshold panel must Block (pins gate.go's threshold>0 guard), got %+v", got0)
	}
}

func TestPanel_AbandonedFieldsParse(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/dead/panel.json", `{
	  "bead_id": "mindspec-x.2", "spec": "s", "target": "bead/mindspec-x.2",
	  "round": 3, "expected_reviewers": 6, "reviewed_head_sha": "beef",
	  "abandoned": true,
	  "abandon_reason": "max: superseded by spec rescope, see mindspec-x.3"
	}`)
	regs := Scan(root)
	if len(regs) != 1 || regs[0].Err != nil {
		t.Fatalf("scan failed: %+v", regs)
	}
	p := regs[0].Panel
	if !p.Abandoned || p.AbandonReason == "" {
		t.Errorf("abandoned/abandon_reason not parsed: %+v", p)
	}
}

// TestPanel_GateFieldDecisionInert (Spec 112 AC6, ADR-0037 §1 amendment):
// the recorded `gate` field's presence, absence, or value changes no
// PanelGateDecision outcome and no ApproveThreshold() — it is decision-inert
// metadata in exactly the sense AbandonReason is. An unexpected value never
// sets Registration.Err (parse-lenient, like AbandonReason), and a gate-less
// legacy panel.json parses field-for-field identical to today's.
func TestPanel_GateFieldDecisionInert(t *testing.T) {
	sha := "abc1234def5678abc1234def5678abc1234def56"
	base := Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha}

	for _, gate := range []string{"", "bead", "weird"} {
		p := base
		p.Gate = gate
		// 5 APPROVE + 1 neutral dissent = 6/6 present (Complete()), 5/6
		// APPROVE meets the N-1 threshold (mirrors
		// TestPanelGateDecision_ConfigDefaultDoesNotAlterDecision).
		r := result(&p, 5, 0, 1, nil, nil)
		r.Verdicts = append(r.Verdicts,
			Verdict{File: "z-round-1.json", Slot: "z", Round: 1, Verdict: VerdictRequestChanges})
		facts := GateFacts{
			BeadID:  "mindspec-bd01",
			Reg:     regn("/wt/review/slug"),
			Res:     r,
			HeadSHA: sha,
		}
		if got := PanelGateDecision(facts); got.Action != Allow {
			t.Errorf("gate=%q: PanelGateDecision = %+v, want Allow", gate, got)
		}
		if th := p.ApproveThreshold(); th != 5 {
			t.Errorf("gate=%q: ApproveThreshold() = %d, want 5 (Gate must not affect it)", gate, th)
		}
	}

	// An unexpected gate value never sets Registration.Err.
	root := t.TempDir()
	writeFile(t, root, "review/weird-gate/panel.json", `{
	  "bead_id": "mindspec-x.9", "spec": "s", "target": "bead/mindspec-x.9",
	  "round": 1, "expected_reviewers": 6, "reviewed_head_sha": "abc",
	  "gate": "weird"
	}`)
	regs := Scan(root)
	if len(regs) != 1 || regs[0].Err != nil {
		t.Fatalf("unexpected gate value must not error: %+v", regs)
	}
	if regs[0].Panel.Gate != "weird" {
		t.Errorf("Gate = %q, want %q", regs[0].Panel.Gate, "weird")
	}

	// A gate-less legacy panel.json parses field-for-field identical to
	// today's (byte-identical code paths — no new branch, no default fill).
	legacyRoot := t.TempDir()
	writeFile(t, legacyRoot, "review/legacy/panel.json", beadPanelJSON)
	legacyRegs := Scan(legacyRoot)
	if len(legacyRegs) != 1 || legacyRegs[0].Err != nil {
		t.Fatalf("legacy scan failed: %+v", legacyRegs)
	}
	got := legacyRegs[0].Panel
	want := Panel{
		BeadID:            ptr("mindspec-x.1"),
		Spec:              "093-skills-thin-down",
		Target:            "bead/mindspec-x.1",
		Round:             1,
		ExpectedReviewers: 6,
		ReviewedHeadSHA:   "abc1234abc1234abc1234abc1234abc1234abc12",
	}
	if got.Gate != "" {
		t.Errorf("legacy Gate = %q, want empty", got.Gate)
	}
	if *got.BeadID != *want.BeadID || got.Spec != want.Spec || got.Target != want.Target ||
		got.Round != want.Round || got.ExpectedReviewers != want.ExpectedReviewers ||
		got.ReviewedHeadSHA != want.ReviewedHeadSHA || got.Abandoned != want.Abandoned ||
		got.AbandonReason != want.AbandonReason || got.ApproveThresholdExpr != want.ApproveThresholdExpr {
		t.Errorf("legacy Panel mismatch: got %+v, want %+v", got, want)
	}
}
