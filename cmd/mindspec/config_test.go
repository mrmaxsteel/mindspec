package main

// config_test.go: tests for `mindspec config show` / renderConfig (spec 109
// Bead 4, R9).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// bp returns a pointer to s, for building panel.Panel.BeadID fixtures.
func bp(s string) *string { return &s }

// resetConfigShowGateFlags resets configShowCmd's --gate/--json flags to
// their zero values, both immediately and via t.Cleanup. rootCmd is a
// package-level singleton these tests share via Execute(), and cobra does
// NOT reset a flag back to its default when a later Execute() call omits
// it — a test that sets --gate/--json would otherwise leak that state into
// every subsequent `config show` invocation in this package, regardless of
// test order.
func resetConfigShowGateFlags(t *testing.T) {
	t.Helper()
	reset := func() {
		configShowCmd.Flags().Set("gate", "")
		configShowCmd.Flags().Set("json", "false")
	}
	reset()
	t.Cleanup(reset)
}

// TestConfigShow_EmitsPanelModelsLoop asserts that renderConfig(DefaultConfig())
// (a pure function — no fs, no process) surfaces the panel reviewers, the raw
// approve_threshold expression, the empty models block, loop.enabled=false,
// runner: claude-code-skills, and the "declared, not yet enforced" annotation
// on each of the three inert blocks (spec 109 AC6).
func TestConfigShow_EmitsPanelModelsLoop(t *testing.T) {
	out, err := renderConfig(config.DefaultConfig())
	if err != nil {
		t.Fatalf("renderConfig: %v", err)
	}

	// panel reviewers (3+3 default mix) and the raw threshold expression.
	if !strings.Contains(out, "family: claude") || !strings.Contains(out, "count: 3") {
		t.Errorf("expected the claude reviewer entry, got:\n%s", out)
	}
	if !strings.Contains(out, "family: codex") {
		t.Errorf("expected the codex reviewer entry, got:\n%s", out)
	}
	if !strings.Contains(out, "approve_threshold: n-1") {
		t.Errorf("expected the raw approve_threshold expression, got:\n%s", out)
	}

	// models: empty by default, INERT.
	if !strings.Contains(out, "models: {}") {
		t.Errorf("expected an empty models block, got:\n%s", out)
	}

	// loop: enabled=false by default, INERT.
	if !strings.Contains(out, "loop:") || !strings.Contains(out, "enabled: false") {
		t.Errorf("expected loop with enabled: false, got:\n%s", out)
	}

	// runner: default, INERT.
	if !strings.Contains(out, "runner: claude-code-skills") {
		t.Errorf("expected runner: claude-code-skills, got:\n%s", out)
	}

	// The three inert blocks (models/loop/runner) are each annotated; panel
	// (which DOES drive behavior today) is not one of them.
	if got := strings.Count(out, "declared, not yet enforced"); got < 3 {
		t.Errorf("expected the \"declared, not yet enforced\" annotation on all three inert blocks (models/loop/runner), got %d occurrences:\n%s", got, out)
	}
}

// TestConfigShow_ReviewerCountNoteWhenPanelDiffers exercises the full `config
// show` command (through rootCmd) against a repo with a registered panel
// whose recorded expected_reviewers differs from the config default: the
// output contains the caller-side panel.ReviewerCountNote advisory (spec 109
// R8), and the command still exits 0 and mutates no file (read-only, R9).
func TestConfigShow_ReviewerCountNoteWhenPanelDiffers(t *testing.T) {
	resetConfigShowGateFlags(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	writeConfigShowPanel(t, root, "109-bd04", panel.Panel{
		Spec: "109", Round: 1, ExpectedReviewers: 4,
	})
	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"config", "show"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("mindspec config show: %v\nstderr=%s", err, stderr.String())
	}
	out := stdout.String() + stderr.String()

	// Default config is 3+3=6 expected reviewers; the panel recorded 4.
	if !strings.Contains(out, "recorded 4") || !strings.Contains(out, "config default is 6") {
		t.Errorf("expected a reviewer-count note (recorded 4 vs default 6), got:\n%s", out)
	}

	// Read-only: mutates no file under root.
	entries, err := os.ReadDir(filepath.Join(root, "review", "109-bd04"))
	if err != nil {
		t.Fatalf("read panel dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != panel.FileName {
		t.Errorf("config show must write nothing beyond the fixture panel.json, got: %v", entries)
	}
}

// TestConfigShow_EscapesHostileControlBytes covers the final-review G2 fix:
// a hostile .mindspec/config.yaml whose panel.reviewers[].family and a
// models map key/value all carry ESC (\x1b), BEL (\x07), and an embedded
// newline followed by text shaped like a forged config key
// ("injected_key: forged_value", G2's own reproduction) must never reach
// `mindspec config show`'s stdout as raw control bytes, and must never
// forge a new display line that looks like a legitimate config entry. The
// hostile value is round-tripped through real YAML (double-quoted hex
// escapes, verified to decode to the exact raw bytes below) and through the
// full `mindspec config show` command — not just the pure renderConfig
// helper — so this exercises the same path G2 drove.
func TestConfigShow_EscapesHostileControlBytes(t *testing.T) {
	resetConfigShowGateFlags(t)
	hostileFamily := "claude\x1b[31mESC\x07BEL\ninjected_key: forged_value"
	hostileModelsKey := "phase\x1bkey\x07\ninjected_key: forged_map_key"
	hostileModelsValue := "modelname\x1bwith\x07control\ninjected_key: forged_value"

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	hostileYAML := "panel:\n" +
		"  reviewers:\n" +
		"    - family: \"claude\\x1B[31mESC\\x07BEL\\ninjected_key: forged_value\"\n" +
		"      count: 3\n" +
		"models:\n" +
		"  \"phase\\x1Bkey\\x07\\ninjected_key: forged_map_key\": \"modelname\\x1Bwith\\x07control\\ninjected_key: forged_value\"\n"
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(hostileYAML), 0o644); err != nil {
		t.Fatalf("write hostile config.yaml: %v", err)
	}

	// Sanity: confirm the YAML source actually decodes to the exact raw
	// bytes this test targets, so a future change to the fixture text can't
	// silently stop exercising the hostile path.
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load(hostile fixture): %v", err)
	}
	if got := cfg.Panel.Reviewers[0].Family; got != hostileFamily {
		t.Fatalf("fixture family did not decode to the expected raw bytes: got %q, want %q", got, hostileFamily)
	}
	if got, ok := cfg.Models[hostileModelsKey]; !ok || got != hostileModelsValue {
		t.Fatalf("fixture models entry did not decode to the expected raw bytes: got %q (ok=%v), want key %q value %q", got, ok, hostileModelsKey, hostileModelsValue)
	}

	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"config", "show"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("mindspec config show: %v\nstderr=%s", err, stderr.String())
	}
	outBytes := stdout.Bytes()
	out := stdout.String()

	// Zero raw control bytes on stdout: ESC and BEL must never appear as
	// literal bytes, only inside an escaped (\x1b / \a) textual form.
	if bytes.IndexByte(outBytes, 0x1b) != -1 {
		t.Errorf("raw ESC byte (0x1b) reached stdout:\n%q", out)
	}
	if bytes.IndexByte(outBytes, 0x07) != -1 {
		t.Errorf("raw BEL byte (0x07) reached stdout:\n%q", out)
	}

	// No forged line: the embedded newline must never produce a physical
	// output line that starts with the attacker's fake key (G2's
	// "injected_key" shape). If the newline were rendered raw, the output
	// would contain a line reading exactly "injected_key: forged_value" (or
	// "...forged_map_key", indented like a real config line).
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "injected_key:") {
			t.Errorf("hostile newline forged a display line starting a fake key: %q\nfull output:\n%s", line, out)
		}
	}

	// The hostile values must still be RENDERED (quoted/escaped), not
	// silently dropped or truncated — assert the exact escaped literal
	// (computed independently via strconv.Quote on the raw bytes) appears
	// verbatim, on one line, in stdout.
	wantFamily := strconv.Quote(hostileFamily)
	if !strings.Contains(out, wantFamily) {
		t.Errorf("expected the escaped family literal %s in stdout, got:\n%s", wantFamily, out)
	}
	wantModelsKey := strconv.Quote(hostileModelsKey)
	wantModelsValue := strconv.Quote(hostileModelsValue)
	if !strings.Contains(out, wantModelsKey) {
		t.Errorf("expected the escaped models key literal %s in stdout, got:\n%s", wantModelsKey, out)
	}
	if !strings.Contains(out, wantModelsValue) {
		t.Errorf("expected the escaped models value literal %s in stdout, got:\n%s", wantModelsValue, out)
	}
}

// writeConfigShowPanel writes root/review/<slug>/panel.json for the
// repo-root review/ convention panel.Scan checks.
func writeConfigShowPanel(t *testing.T, root, slug string, p panel.Panel) {
	t.Helper()
	dir := filepath.Join(root, "review", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal panel: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, panel.FileName), data, 0o644); err != nil {
		t.Fatalf("write panel.json: %v", err)
	}
}

// TestConfigShow_GatesSubstitutesAndModelAdvisory covers spec 112 AC8/R8:
// renderConfig (a pure function — no fs, no process) renders configured
// gates in enum declaration order with resolved sums and raw threshold
// expressions, substitutes in sorted-key order alongside the
// slot-id-preservation convention line, echoes a set note, warns on a
// made-up model id without affecting the exit code, and never warns on a
// seeded id or legacy family string. Two renders of one config are
// byte-identical (Go map iteration order must never leak into output).
func TestConfigShow_GatesSubstitutesAndModelAdvisory(t *testing.T) {
	three := 3
	cfg := config.DefaultConfig()
	cfg.Panel.Note = "fable-window 2026-07, codex-enabled"
	cfg.Panel.Gates = map[string]config.GatePanel{
		// bead: threshold-only override — inherits the global reviewers
		// (3+3=6), exercising the per-field fallback chain's rendering.
		"bead": {ApproveThreshold: "5"},
		// final_review: reviewers-only override, including one made-up
		// model id, rendered before "adhoc" but after "bead" in enum order.
		"final_review": {Reviewers: []config.Reviewer{
			{Model: "claude-fable-5", Count: &three},
			{Model: "claude-opus-4-8", Count: &three},
			{Model: "gpt-5.5", Count: &three},
			{Model: "made-up-model-9000", Count: &three},
		}},
	}
	cfg.Panel.Substitution.Substitutes = map[string]string{
		"gpt-5.5":        "claude-sonnet-5",
		"claude-fable-5": "claude-opus-4-8",
	}

	out1, err := renderConfig(cfg)
	if err != nil {
		t.Fatalf("renderConfig: %v", err)
	}

	// Scope the enum-order check to the gates: block itself — loop.gate_authority
	// (an unrelated, pre-existing map) also happens to use the
	// spec_approve/plan_approve key names, so a whole-output Contains would
	// false-positive against IT rather than the gates: rendering.
	gatesStart := strings.Index(out1, "  gates:")
	modelsStart := strings.Index(out1, "\nmodels:")
	if gatesStart == -1 || modelsStart == -1 || gatesStart > modelsStart {
		t.Fatalf("could not locate the gates: block in renderConfig output:\n%s", out1)
	}
	gatesSection := out1[gatesStart:modelsStart]

	// Enum declaration order: "bead:" (configured) precedes "final_review:"
	// (configured); the three unconfigured gates (spec_approve,
	// plan_approve, adhoc) render nothing under gates:.
	beadIdx := strings.Index(gatesSection, "bead:")
	finalIdx := strings.Index(gatesSection, "final_review:")
	if beadIdx == -1 || finalIdx == -1 || beadIdx > finalIdx {
		t.Errorf("expected \"bead:\" before \"final_review:\" (PanelGateKeys enum order), got gates section:\n%s", gatesSection)
	}
	if strings.Contains(gatesSection, "spec_approve:") || strings.Contains(gatesSection, "plan_approve:") || strings.Contains(gatesSection, "adhoc:") {
		t.Errorf("unconfigured gates must not render their own gates: entry, got gates section:\n%s", gatesSection)
	}

	// Resolved sums + raw threshold expressions.
	if !strings.Contains(out1, "expected_reviewers: 6") {
		t.Errorf("expected bead's inherited resolved sum (6), got:\n%s", out1)
	}
	if !strings.Contains(out1, "approve_threshold: 5") {
		t.Errorf("expected bead's own raw threshold expression (5), got:\n%s", out1)
	}
	if !strings.Contains(out1, "expected_reviewers: 12") {
		t.Errorf("expected final_review's resolved sum (12), got:\n%s", out1)
	}

	// substitutes: sorted-key order (claude-fable-5 before gpt-5.5) plus
	// the slot-id-preservation convention line.
	cfIdx := strings.Index(out1, "claude-fable-5: claude-opus-4-8")
	gptIdx := strings.Index(out1, "gpt-5.5: claude-sonnet-5")
	if cfIdx == -1 || gptIdx == -1 || cfIdx > gptIdx {
		t.Errorf("expected substitutes in sorted-key order (claude-fable-5 before gpt-5.5), got:\n%s", out1)
	}
	if !strings.Contains(out1, `reviewer_id "<slot> <substitute-model>-sub"`) {
		t.Errorf("expected the slot-id-preservation convention line, got:\n%s", out1)
	}

	// note echoed verbatim.
	if !strings.Contains(out1, "note: fable-window 2026-07, codex-enabled") {
		t.Errorf("expected the note echoed, got:\n%s", out1)
	}

	// Known-model advisory: the made-up id warns; none of the seeded ids
	// or legacy family strings (claude, codex, and the four protocol
	// model ids used above) ever warn (negative control).
	if !strings.Contains(out1, "made-up-model-9000") || !strings.Contains(out1, "not in the known-model list") {
		t.Errorf("expected a known-model warning for the made-up id, got:\n%s", out1)
	}
	for _, seeded := range []string{"claude", "codex", "claude-fable-5", "claude-opus-4-8", "claude-sonnet-5", "gpt-5.5"} {
		if idx := strings.Index(out1, seeded+" not in the known-model list"); idx != -1 {
			t.Errorf("seeded id/family %q must never warn, got:\n%s", seeded, out1)
		}
	}

	// Deterministic output: two renders of the SAME config are byte-identical.
	out2, err := renderConfig(cfg)
	if err != nil {
		t.Fatalf("renderConfig (second render): %v", err)
	}
	if out1 != out2 {
		t.Errorf("expected two renders of one config to be byte-identical:\n--- render 1 ---\n%s\n--- render 2 ---\n%s", out1, out2)
	}
}

// TestConfigShow_ReviewerCountNoteGateAware is the R7 cmd-side
// falsification pin (spec 112 AC7): over a temp root with gates:
// configured and registered panels on disk (the full `config show` path),
// a bead panel whose recorded count matches the configured bead gate's
// default but differs from the global default emits NO note — this case
// FAILS against unwired 109 code, which compares globally. A genuine
// mismatch against the panel's own recorded gate's default emits the note,
// and a gate-less non-bead registration emits nothing.
func TestConfigShow_ReviewerCountNoteGateAware(t *testing.T) {
	resetConfigShowGateFlags(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	// bead gate resolves to 9 — differing from the 3+3=6 global default,
	// so a recorded panel of 9 proves comparison against the GATE default.
	gatesYAML := `
panel:
  gates:
    bead:
      reviewers:
        - {model: claude-opus-4-8, count: 9}
`
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(gatesYAML), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	// Case A: matches bead's own gate default (9), differs from the
	// global default (6) -> NO note.
	writeConfigShowPanel(t, root, "112-bead-match", panel.Panel{
		BeadID: bp("mindspec-bead-match"), Spec: "112", Round: 1, ExpectedReviewers: 9,
	})
	// Case B: mismatches bead's own gate default (9) -> note.
	writeConfigShowPanel(t, root, "112-bead-mismatch", panel.Panel{
		BeadID: bp("mindspec-bead-mismatch"), Spec: "112", Round: 1, ExpectedReviewers: 7,
	})
	// Case C: gate-less non-bead registration -> nothing (the R7 skip
	// carve-out, gates: configured).
	writeConfigShowPanel(t, root, "112-nonbead", panel.Panel{
		Spec: "112", Round: 1, ExpectedReviewers: 999,
	})

	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"config", "show"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("mindspec config show: %v\nstderr=%s", err, stderr.String())
	}
	out := stdout.String() + stderr.String()

	if strings.Contains(out, "112-bead-match") {
		t.Errorf("a bead panel matching its own gate's default (9) must emit no note even though it differs from the global default (6), got:\n%s", out)
	}
	if !strings.Contains(out, "112-bead-mismatch") || !strings.Contains(out, "recorded 7") || !strings.Contains(out, "config default is 9") {
		t.Errorf("expected a mismatch note for 112-bead-mismatch (recorded 7 vs bead's own default 9), got:\n%s", out)
	}
	if strings.Contains(out, "112-nonbead") {
		t.Errorf("a gate-less non-bead registration must emit no note while gates: is configured, got:\n%s", out)
	}
}

// TestConfigShowGate_ResolvedJSON covers spec 112 AC9/R9: `config show
// --gate bead --json` emits EXACTLY the five documented members, agreeing
// with the R3 config resolvers over the same config; substitution.in_force
// flips per R5 (non-empty substitutes map vs the legacy bool); an unknown
// --gate exits non-zero with the five-key recovery line; the command
// writes nothing.
func TestConfigShowGate_ResolvedJSON(t *testing.T) {
	resetConfigShowGateFlags(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	protocolYAML := `
panel:
  reviewers:
    - {family: claude, count: 3}
    - {family: codex, count: 3}
  approve_threshold: "n-1"
  substitution:
    claude_sub_on_quota: true
    substitutes:
      gpt-5.5: claude-sonnet-5
  gates:
    bead:
      reviewers:
        - {model: claude-opus-4-8, lens: author-of-record}
        - {model: claude-opus-4-8, lens: codebase-pin}
        - {model: claude-opus-4-8, lens: contract-stability}
        - {model: claude-sonnet-5, lens: empirical-prober}
        - {model: claude-sonnet-5, lens: adversarial}
        - {model: claude-sonnet-5, lens: integration}
`
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(protocolYAML), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	entriesBefore := listAllFiles(t, root)

	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"config", "show", "--gate", "bead", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("mindspec config show --gate bead --json: %v\nstderr=%s", err, stderr.String())
	}

	// Exactly the five documented members — no more, no less.
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &generic); err != nil {
		t.Fatalf("unmarshal JSON output: %v\noutput: %s", err, stdout.String())
	}
	wantKeys := []string{"gate", "slots", "expected_reviewers", "approve_threshold", "substitution"}
	if len(generic) != len(wantKeys) {
		t.Errorf("expected exactly %d members, got %d: %v", len(wantKeys), len(generic), generic)
	}
	for _, k := range wantKeys {
		if _, ok := generic[k]; !ok {
			t.Errorf("missing documented member %q in JSON output: %s", k, stdout.String())
		}
	}

	var doc struct {
		Gate  string `json:"gate"`
		Slots []struct {
			Slot  string `json:"slot"`
			Model string `json:"model"`
			Lens  string `json:"lens"`
		} `json:"slots"`
		ExpectedReviewers int    `json:"expected_reviewers"`
		ApproveThreshold  string `json:"approve_threshold"`
		Substitution      struct {
			Substitutes      map[string]string `json:"substitutes"`
			ClaudeSubOnQuota bool              `json:"claude_sub_on_quota"`
			InForce          string            `json:"in_force"`
		} `json:"substitution"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal typed JSON output: %v", err)
	}

	// Agrees with the R3 resolvers over the SAME config.
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	wantSlots, err := cfg.PanelGateReviewerSlots("bead")
	if err != nil {
		t.Fatalf("PanelGateReviewerSlots: %v", err)
	}
	wantSum, err := cfg.PanelGateExpectedReviewers("bead")
	if err != nil {
		t.Fatalf("PanelGateExpectedReviewers: %v", err)
	}
	wantExpr, err := cfg.PanelGateApproveThresholdExpr("bead")
	if err != nil {
		t.Fatalf("PanelGateApproveThresholdExpr: %v", err)
	}

	if doc.Gate != "bead" {
		t.Errorf("gate: got %q, want %q", doc.Gate, "bead")
	}
	if doc.ExpectedReviewers != wantSum {
		t.Errorf("expected_reviewers: got %d, want %d (R3 resolver)", doc.ExpectedReviewers, wantSum)
	}
	if doc.ApproveThreshold != wantExpr {
		t.Errorf("approve_threshold: got %q, want %q (R3 resolver)", doc.ApproveThreshold, wantExpr)
	}
	if len(doc.Slots) != len(wantSlots) {
		t.Fatalf("slots: got %d, want %d (R3 resolver)", len(doc.Slots), len(wantSlots))
	}
	for i, s := range wantSlots {
		if doc.Slots[i].Slot != s.Slot || doc.Slots[i].Model != s.Model || doc.Slots[i].Lens != s.Lens {
			t.Errorf("slot[%d]: got %+v, want %+v (R3 resolver)", i, doc.Slots[i], s)
		}
	}

	// substitution.in_force: non-empty substitutes map -> "substitutes".
	if doc.Substitution.InForce != "substitutes" {
		t.Errorf("in_force: got %q, want %q (non-empty substitutes map)", doc.Substitution.InForce, "substitutes")
	}
	if !reflect.DeepEqual(doc.Substitution.Substitutes, map[string]string{"gpt-5.5": "claude-sonnet-5"}) {
		t.Errorf("substitutes: got %v", doc.Substitution.Substitutes)
	}
	if !doc.Substitution.ClaudeSubOnQuota {
		t.Error("claude_sub_on_quota: want true")
	}

	// in_force flips when substitutes is empty (R5 supersession) — checked
	// directly against gateResolvedJSON (pure) over a config copy with an
	// emptied substitutes map, isolating this assertion from a second CLI run.
	cfgNoSub := *cfg
	cfgNoSub.Panel.Substitution.Substitutes = nil
	data, err := gateResolvedJSON(&cfgNoSub, "bead")
	if err != nil {
		t.Fatalf("gateResolvedJSON: %v", err)
	}
	var flipped struct {
		Substitution struct {
			InForce string `json:"in_force"`
		} `json:"substitution"`
	}
	if err := json.Unmarshal(data, &flipped); err != nil {
		t.Fatalf("unmarshal flipped JSON: %v", err)
	}
	if flipped.Substitution.InForce != "claude_sub_on_quota" {
		t.Errorf("in_force with empty substitutes: got %q, want %q", flipped.Substitution.InForce, "claude_sub_on_quota")
	}

	// Unknown --gate exits non-zero with the five-key recovery line.
	var stdout2, stderr2 bytes.Buffer
	rootCmd.SetOut(&stdout2)
	rootCmd.SetErr(&stderr2)
	rootCmd.SetArgs([]string{"config", "show", "--gate", "bogus", "--json"})
	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("expected a non-zero exit (error) for an unknown --gate value")
	}
	for _, key := range config.PanelGateKeys {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("expected the recovery line to enumerate %q, got: %v", key, err)
		}
	}
	if !strings.Contains(err.Error(), "recovery:") {
		t.Errorf("expected an ADR-0035 recovery line, got: %v", err)
	}

	// Read-only: writes nothing under root.
	entriesAfter := listAllFiles(t, root)
	if !reflect.DeepEqual(entriesBefore, entriesAfter) {
		t.Errorf("config show --gate must write nothing, before=%v after=%v", entriesBefore, entriesAfter)
	}
}

// TestConfigShow_HostileStringsEscaped covers spec 112 AC10/R8: a hostile
// panel.note, gate reviewer model, gate reviewer lens, and a substitutes
// key/value — each carrying ESC/BEL and an embedded newline shaped like a
// forged config line — never reach `config show` or
// `config show --gate <name>` text output as raw control bytes or a forged
// extra line, and round-trip byte-exactly through `--gate <name> --json`
// under a real encoding/json decode.
func TestConfigShow_HostileStringsEscaped(t *testing.T) {
	resetConfigShowGateFlags(t)
	hostileNote := "note\x1b[31mESC\x07BEL\ninjected_key: forged_note"
	hostileModel := "model\x1bwith\x07control\ninjected_key: forged_model"
	hostileLens := "lens\x1bwith\x07control\ninjected_key: forged_lens"
	hostileSubKey := "subkey\x1bwith\x07control\ninjected_key: forged_subkey"
	hostileSubVal := "subval\x1bwith\x07control\ninjected_key: forged_subval"

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	hostileYAML := "panel:\n" +
		"  note: \"note\\x1B[31mESC\\x07BEL\\ninjected_key: forged_note\"\n" +
		"  gates:\n" +
		"    bead:\n" +
		"      reviewers:\n" +
		"        - model: \"model\\x1Bwith\\x07control\\ninjected_key: forged_model\"\n" +
		"          lens: \"lens\\x1Bwith\\x07control\\ninjected_key: forged_lens\"\n" +
		"          count: 1\n" +
		"        - model: claude-sonnet-5\n" +
		"          count: 1\n" +
		"  substitution:\n" +
		"    substitutes:\n" +
		"      \"subkey\\x1Bwith\\x07control\\ninjected_key: forged_subkey\": \"subval\\x1Bwith\\x07control\\ninjected_key: forged_subval\"\n"
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(hostileYAML), 0o644); err != nil {
		t.Fatalf("write hostile config.yaml: %v", err)
	}

	// Sanity: confirm the fixture decodes to the exact raw bytes targeted.
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load(hostile fixture): %v", err)
	}
	if cfg.Panel.Note != hostileNote {
		t.Fatalf("fixture note did not decode as expected: got %q", cfg.Panel.Note)
	}
	gp := cfg.Panel.Gates["bead"]
	if len(gp.Reviewers) != 2 || gp.Reviewers[0].Model != hostileModel || gp.Reviewers[0].Lens != hostileLens {
		t.Fatalf("fixture bead reviewer did not decode as expected: got %+v", gp.Reviewers)
	}
	if got, ok := cfg.Panel.Substitution.Substitutes[hostileSubKey]; !ok || got != hostileSubVal {
		t.Fatalf("fixture substitutes did not decode as expected: got %v", cfg.Panel.Substitution.Substitutes)
	}

	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	checkTextOutput := func(t *testing.T, args []string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("mindspec %s: %v\nstderr=%s", strings.Join(args, " "), err, stderr.String())
		}
		outBytes := stdout.Bytes()
		out := stdout.String()

		if bytes.IndexByte(outBytes, 0x1b) != -1 {
			t.Errorf("raw ESC byte (0x1b) reached stdout for %v:\n%q", args, out)
		}
		if bytes.IndexByte(outBytes, 0x07) != -1 {
			t.Errorf("raw BEL byte (0x07) reached stdout for %v:\n%q", args, out)
		}
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "injected_key:") {
				t.Errorf("hostile newline forged a display line for %v: %q\nfull output:\n%s", args, line, out)
			}
		}
		return out
	}

	out := checkTextOutput(t, []string{"config", "show"})
	for _, want := range []string{hostileNote, hostileModel, hostileLens, hostileSubKey, hostileSubVal} {
		if lit := strconv.Quote(want); !strings.Contains(out, lit) {
			t.Errorf("expected the escaped literal %s in `config show` stdout, got:\n%s", lit, out)
		}
	}

	outGate := checkTextOutput(t, []string{"config", "show", "--gate", "bead"})
	for _, want := range []string{hostileModel, hostileLens, hostileSubKey, hostileSubVal} {
		if lit := strconv.Quote(want); !strings.Contains(outGate, lit) {
			t.Errorf("expected the escaped literal %s in `config show --gate bead` stdout, got:\n%s", lit, outGate)
		}
	}

	// --gate --json: byte-exact round trip under a real encoding/json decode.
	var stdoutJSON, stderrJSON bytes.Buffer
	rootCmd.SetOut(&stdoutJSON)
	rootCmd.SetErr(&stderrJSON)
	rootCmd.SetArgs([]string{"config", "show", "--gate", "bead", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("mindspec config show --gate bead --json: %v\nstderr=%s", err, stderrJSON.String())
	}
	var doc struct {
		Slots []struct {
			Model string `json:"model"`
			Lens  string `json:"lens"`
		} `json:"slots"`
		Substitution struct {
			Substitutes map[string]string `json:"substitutes"`
		} `json:"substitution"`
	}
	if err := json.Unmarshal(stdoutJSON.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal --gate --json output: %v\noutput: %s", err, stdoutJSON.String())
	}
	if len(doc.Slots) != 2 || doc.Slots[0].Model != hostileModel || doc.Slots[0].Lens != hostileLens {
		t.Errorf("expected the hostile model/lens to round-trip byte-exactly through JSON, got: %+v", doc.Slots)
	}
	if got, ok := doc.Substitution.Substitutes[hostileSubKey]; !ok || got != hostileSubVal {
		t.Errorf("expected the hostile substitutes entry to round-trip byte-exactly through JSON, got: %v", doc.Substitution.Substitutes)
	}
}

// listAllFiles returns every regular file path under root (relative to
// root), sorted, for a before/after read-only comparison.
func listAllFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}
