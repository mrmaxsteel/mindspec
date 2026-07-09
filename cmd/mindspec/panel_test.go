package main

// panel_test.go: tests for `mindspec panel create | verify | tally` (spec
// 110 Bead 4).

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// --- shared fixture helpers -------------------------------------------------

// ptrStr is a tiny helper for *string panel fields.
func ptrStr(s string) *string { return &s }

// mkPanelTestRoot creates a fresh `.mindspec`-marked workspace root
// (findRoot's marker) with an optional config.yaml body.
func mkPanelTestRoot(t *testing.T, configYAML string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	if configYAML != "" {
		if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(configYAML), 0o644); err != nil {
			t.Fatalf("write config.yaml: %v", err)
		}
	}
	return root
}

// writePanelFixture writes root/review/<slug>/panel.json directly (the
// repo-root convention `panel create` and `panel.Scan` use on a non-flat
// tree), for tests that need a pre-registered panel without going through
// `panel create` itself.
func writePanelFixture(t *testing.T, root, slug string, p panel.Panel) string {
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
	return dir
}

// snapshotTree walks root and returns every relative path found, for a
// before/after "wrote nothing" comparison.
func snapshotTree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

// resetPanelCreateFlags resets panelCreateCmd's flags to their defaults
// (and clears Changed) before a subtest runs. cobra flags live on the
// package-level command and are NOT reset between Execute() calls, so a
// value set by one t.Run — e.g. a --bead containing a control byte —
// otherwise persists into the next subtest's Execute() and can produce a
// false-positive rejection attributed to the wrong flag.
func resetPanelCreateFlags(t *testing.T) {
	t.Helper()
	for _, name := range []string{"spec", "target", "bead", "round"} {
		if f := panelCreateCmd.Flags().Lookup(name); f != nil {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
	}
}

// stubWorktreeListEmpty points panelWorktreeListFn at a stub returning no
// entries, so `panel verify`/`panel tally` never spawn a real `bd`
// subprocess in tests. Restored via t.Cleanup.
func stubWorktreeListEmpty(t *testing.T) {
	t.Helper()
	orig := panelWorktreeListFn
	panelWorktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	t.Cleanup(func() { panelWorktreeListFn = orig })
}

// buildResult constructs a *panel.Result fixture: `approves` APPROVE
// verdicts, `rejects` REJECT verdicts, and enough REQUEST_CHANGES
// verdicts appended to reach `total` — so completeness can be tuned
// independently of the approve/reject counts (e.g. a sub-threshold row
// needs Approves < threshold while still being COMPLETE). Mirrors
// internal/panel/panel_decision_test.go's own `result` helper, rebuilt
// here over exported fields only (cmd/mindspec is a different package).
func buildResult(p *panel.Panel, approves, rejects, total, round int, hardBlocks []string) *panel.Result {
	r := &panel.Result{
		Dir: "/wt/review/demo", Panel: p, LatestRound: round,
		Approves: approves, Rejects: rejects, HardBlocks: hardBlocks,
	}
	idx := 0
	add := func(verdict string) {
		r.Verdicts = append(r.Verdicts, panel.Verdict{
			File: fmt.Sprintf("slot%d-round-%d.json", idx, round), Slot: fmt.Sprintf("slot%d", idx),
			Round: round, Verdict: verdict,
		})
		idx++
	}
	for i := 0; i < approves; i++ {
		add(panel.VerdictApprove)
	}
	for i := 0; i < rejects; i++ {
		add(panel.VerdictReject)
	}
	for idx < total {
		add(panel.VerdictRequestChanges)
	}
	if p != nil && round > 0 {
		r.RoundMismatch = p.Round != round
	}
	return r
}

// --- TestPanelCreate_StampsResolversAndCoBumpsRoundSHA ----------------------

func TestPanelCreate_StampsResolversAndCoBumpsRoundSHA(t *testing.T) {
	cfgYAML := "panel:\n  reviewers:\n    - family: claude\n      count: 2\n    - family: codex\n      count: 1\n  approve_threshold: \"2\"\n"
	root := mkPanelTestRoot(t, cfgYAML)
	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	origRevParse := revParseForPanelFn
	t.Cleanup(func() { revParseForPanelFn = origRevParse })

	beadID := "mindspec-x.1"
	sha1 := "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111"
	revParseForPanelFn = func(string, string) (string, error) { return sha1, nil }

	runPanel := func(args ...string) (string, error) {
		var stdout, stderr bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		rootCmd.SetArgs(append([]string{"panel"}, args...))
		err := rootCmd.Execute()
		return stdout.String() + stderr.String(), err
	}

	if out, err := runPanel("create", "demo", "--spec", "110-test", "--target", "bead/"+beadID, "--bead", beadID); err != nil {
		t.Fatalf("panel create (round 1): %v\noutput=%s", err, out)
	}

	dir := filepath.Join(root, "review", "demo")
	data, err := os.ReadFile(filepath.Join(dir, panel.FileName))
	if err != nil {
		t.Fatalf("read panel.json: %v", err)
	}
	var got panel.Panel
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal panel.json: %v", err)
	}
	if got.ExpectedReviewers != 3 {
		t.Errorf("expected_reviewers = %d, want 3 (from the config resolver)", got.ExpectedReviewers)
	}
	if got.ApproveThresholdExpr != "2" {
		t.Errorf("approve_threshold = %q, want the raw config expression %q", got.ApproveThresholdExpr, "2")
	}
	if got.ReviewedHeadSHA != sha1 {
		t.Errorf("reviewed_head_sha = %q, want %q", got.ReviewedHeadSHA, sha1)
	}
	if got.Round != 1 {
		t.Errorf("round = %d, want 1", got.Round)
	}
	if got.BeadID == nil || *got.BeadID != beadID {
		t.Errorf("bead_id = %v, want %q", got.BeadID, beadID)
	}

	brief1, err := os.ReadFile(filepath.Join(dir, "BRIEF.md"))
	if err != nil {
		t.Fatalf("read BRIEF.md: %v", err)
	}
	if !strings.Contains(string(brief1), sha1) {
		t.Fatalf("BRIEF.md missing round-1 SHA %s:\n%s", sha1, brief1)
	}

	// Seed a prior-round verdict file and remember the skill-authored body
	// (everything the header splice must never touch).
	verdictBody := `{"verdict":"APPROVE"}`
	if err := os.WriteFile(filepath.Join(dir, "R1-round-1.json"), []byte(verdictBody), 0o644); err != nil {
		t.Fatalf("seed verdict file: %v", err)
	}
	bodyMarker := "<!-- TODO(skill): one-paragraph summary"
	if !strings.Contains(string(brief1), bodyMarker) {
		t.Fatalf("fixture assumption broken: BRIEF.md has no skill-authored stub body to protect:\n%s", brief1)
	}

	sha2 := "bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222"
	revParseForPanelFn = func(string, string) (string, error) { return sha2, nil }
	if out, err := runPanel("create", "demo", "--spec", "110-test", "--target", "bead/"+beadID, "--bead", beadID, "--round", "2"); err != nil {
		t.Fatalf("panel create (round 2): %v\noutput=%s", err, out)
	}

	data2, err := os.ReadFile(filepath.Join(dir, panel.FileName))
	if err != nil {
		t.Fatalf("read panel.json (round 2): %v", err)
	}
	var got2 panel.Panel
	if err := json.Unmarshal(data2, &got2); err != nil {
		t.Fatalf("unmarshal panel.json (round 2): %v", err)
	}
	if got2.Round != 2 || got2.ReviewedHeadSHA != sha2 {
		t.Fatalf("round 2 create did not co-bump round+SHA: got round=%d sha=%q", got2.Round, got2.ReviewedHeadSHA)
	}

	brief2, err := os.ReadFile(filepath.Join(dir, "BRIEF.md"))
	if err != nil {
		t.Fatalf("read BRIEF.md (round 2): %v", err)
	}
	if !strings.Contains(string(brief2), sha2) {
		t.Fatalf("BRIEF.md header not updated to round-2 SHA %s:\n%s", sha2, brief2)
	}
	if !strings.Contains(string(brief2), bodyMarker) {
		t.Fatalf("re-panel clobbered the skill-authored body:\n%s", brief2)
	}

	verdictAfter, err := os.ReadFile(filepath.Join(dir, "R1-round-1.json"))
	if err != nil {
		t.Fatalf("re-read prior-round verdict file: %v", err)
	}
	if string(verdictAfter) != verdictBody {
		t.Fatalf("re-panel touched the prior-round verdict file: got %q, want %q", verdictAfter, verdictBody)
	}
}

// --- TestPanelCreate_RejectsUnsafeSlugAndControlBytes -----------------------

func TestPanelCreate_RejectsUnsafeSlugAndControlBytes(t *testing.T) {
	tests := []struct {
		name   string
		slug   string
		bead   string
		target string
	}{
		{"empty slug", "", "", "bead/x"},
		{"dot slug", ".", "", "bead/x"},
		{"dotdot slug", "..", "", "bead/x"},
		{"traversal slug", "../../etc", "", "bead/x"},
		{"newline in slug", "demo\nEVIL", "", "bead/x"},
		{"newline in --bead", "demo", "mindspec-x.1\nEVIL", "bead/x"},
		{"newline in --target", "demo", "", "bead/x\nEVIL"},
		// C1 control range (U+0080-U+009F), valid-UTF8-encoded: the CSI
		// U+009B terminal-injection vector report.go's stripControl
		// already handles (the 'codex-render-leak #2' incident). These
		// bytes are NOT C0/DEL, so a predicate that only checks
		// r < 0x20 || r == 0x7f misses them.
		{"C1 CSI in slug", "demoslug", "", "bead/x"},
		{"C1 CSI in --bead", "demo", "mindspec-x.1EVIL", "bead/x"},
		{"C1 CSI in --target", "demo", "", "bead/xEVIL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetPanelCreateFlags(t)
			root := mkPanelTestRoot(t, "")
			withTestChdir(t, root)
			config.ResetCache()
			t.Cleanup(config.ResetCache)

			origRevParse := revParseForPanelFn
			t.Cleanup(func() { revParseForPanelFn = origRevParse })
			revParseForPanelFn = func(string, string) (string, error) { return "deadbeef", nil }

			before := snapshotTree(t, root)

			args := []string{"panel", "create", tc.slug, "--spec", "110-test", "--target", tc.target}
			if tc.bead != "" {
				args = append(args, "--bead", tc.bead)
			}
			var stdout, stderr bytes.Buffer
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stderr)
			rootCmd.SetArgs(args)
			err := rootCmd.Execute()
			if err == nil {
				t.Fatalf("expected a non-nil error, got nil (stdout=%s)", stdout.String())
			}

			after := snapshotTree(t, root)
			if !reflect.DeepEqual(before, after) {
				t.Errorf("panel create wrote a file for an unsafe input:\nbefore=%v\nafter=%v", before, after)
			}
		})
	}
}

// --- TestPanelVerify_MatchesGateAndWritesNothing ----------------------------

func TestPanelVerify_MatchesGateAndWritesNothing(t *testing.T) {
	// Part A (pure): renderPanelVerify's action equals
	// panel.PanelGateDecision(facts).Action over fabricated facts.
	sha := "abc1234def5678abc1234def5678abc1234def56"
	p := &panel.Panel{
		BeadID: ptrStr("mindspec-bd01"), Spec: "110", Target: "bead/mindspec-bd01",
		Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha,
	}
	reg := &panel.Registration{Dir: "/wt/review/demo"}
	facts := panel.GateFacts{BeadID: "mindspec-bd01", Reg: reg, Res: buildResult(p, 6, 0, 6, 1, nil), HeadSHA: sha}
	_, gotAction := renderPanelVerify(facts.Res, facts)
	if want := panel.PanelGateDecision(facts).Action; gotAction != want {
		t.Fatalf("renderPanelVerify action = %v, want panel.PanelGateDecision action %v", gotAction, want)
	}

	// Part B (real command): running `panel verify` over a registered,
	// complete, non-bead panel mutates no file.
	root := mkPanelTestRoot(t, "")
	writePanelFixture(t, root, "demo", panel.Panel{
		Spec: "110-test", Target: "bead/mindspec-bd01", Round: 1,
		ExpectedReviewers: 1, ReviewedHeadSHA: sha,
	})
	dir := filepath.Join(root, "review", "demo")
	if err := os.WriteFile(filepath.Join(dir, "R1-round-1.json"), []byte(`{"verdict":"APPROVE"}`), 0o644); err != nil {
		t.Fatalf("seed verdict file: %v", err)
	}

	withTestChdir(t, root)
	stubWorktreeListEmpty(t)

	before := snapshotTree(t, root)
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"panel", "verify", "demo"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("panel verify: %v\nstderr=%s", err, stderr.String())
	}
	after := snapshotTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("panel verify mutated the tree:\nbefore=%v\nafter=%v", before, after)
	}
}

// --- TestPanelTally_ExitCodeTracksDecision ----------------------------------

func TestPanelTally_ExitCodeTracksDecision(t *testing.T) {
	sha := "abc1234def5678abc1234def5678abc1234def56"
	otherSHA := "999000999000999000999000999000999000beef"
	beadID := "mindspec-bd01"
	basePanel := func() *panel.Panel {
		return &panel.Panel{
			BeadID: ptrStr(beadID), Spec: "110", Target: "bead/" + beadID,
			Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha,
		}
	}
	reg := &panel.Registration{Dir: "/wt/review/demo"}

	rows := []struct {
		name     string
		facts    panel.GateFacts
		wantErr  bool
		wantWarn bool
	}{
		{
			name:  "passing (at-threshold) → exit 0",
			facts: panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 5, 0, 6, 1, nil), HeadSHA: sha},
		},
		{
			name:    "stale SHA despite Approves alone satisfying threshold → non-zero + recovery",
			facts:   panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 1, nil), HeadSHA: otherSHA},
			wantErr: true,
		},
		{
			name:    "hard_block despite Approves alone satisfying threshold → non-zero + recovery",
			facts:   panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 1, []string{"R1"}), HeadSHA: sha},
			wantErr: true,
		},
		{
			name:    "sub-threshold → non-zero + recovery",
			facts:   panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 4, 0, 6, 1, nil), HeadSHA: sha},
			wantErr: true,
		},
		{
			name: "abandoned → exit 0 with the advisory printed",
			facts: func() panel.GateFacts {
				p := basePanel()
				p.Abandoned = true
				p.AbandonReason = "max: superseded by bd99"
				return panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(p, 0, 0, 0, 1, nil), HeadSHA: otherSHA}
			}(),
			wantWarn: true,
		},
	}

	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			wantDecision := panel.PanelGateDecision(tc.facts)
			_, d := renderPanelTally(tc.facts.Res, tc.facts, nil)
			if d.Action != wantDecision.Action || d.Message != wantDecision.Message {
				t.Fatalf("renderPanelTally decision %+v != panel.PanelGateDecision %+v", d, wantDecision)
			}

			var stderr bytes.Buffer
			origOut := tallyWarnOut
			tallyWarnOut = &stderr
			t.Cleanup(func() { tallyWarnOut = origOut })

			err := tallyExitAction(d, "demo")
			switch {
			case tc.wantErr:
				if err == nil {
					t.Fatal("expected a non-nil error")
				}
				if !guard.HasFinalRecoveryLine(err.Error()) {
					t.Errorf("expected a final recovery line, got: %v", err)
				}
			default:
				if err != nil {
					t.Fatalf("expected a nil error, got: %v", err)
				}
				if tc.wantWarn && stderr.Len() == 0 {
					t.Error("expected the Warn advisory message printed to tallyWarnOut")
				}
			}
		})
	}
}

// --- TestPanelTally_AggregatesConcreteChangesRequired -----------------------

func TestPanelTally_AggregatesConcreteChangesRequired(t *testing.T) {
	dir := t.TempDir()
	reg := panel.Registration{Dir: dir}

	verdictFile := "R2-round-1.json"
	const wantChange = "fix the frobnicator race"
	verdictBody := fmt.Sprintf(`{"verdict":"REQUEST_CHANGES","concrete_changes_required":[%q]}`, wantChange)
	if err := os.WriteFile(filepath.Join(dir, verdictFile), []byte(verdictBody), 0o644); err != nil {
		t.Fatalf("seed verdict file: %v", err)
	}

	res := &panel.Result{
		Dir: dir, Panel: &panel.Panel{ExpectedReviewers: 1, Round: 1}, LatestRound: 1,
		Verdicts: []panel.Verdict{{File: verdictFile, Slot: "R2", Round: 1, Verdict: panel.VerdictRequestChanges}},
	}
	changes := collectSlotChanges(reg, res)

	facts := panel.GateFacts{Reg: &reg, Res: res}
	body, _ := renderPanelTally(res, facts, changes)
	if !strings.Contains(body, wantChange) {
		t.Fatalf("tally output missing the concrete_changes_required entry %q:\n%s", wantChange, body)
	}
	if !strings.Contains(body, "R2:") {
		t.Fatalf("tally output does not attribute the entry to slot R2:\n%s", body)
	}
}

// --- TestPanelVerbs_DecisionIsPanelGateDecision -----------------------------

// TestPanelVerbs_DecisionIsPanelGateDecision is the R7a contract pin: over a
// branch-complete table of panel.GateFacts rows spanning gate.go branches
// (2)-(10) plus the Warn variants (abandoned, missing-ref, transient
// GitErr), both renderPanelVerify and renderPanelTally must render the
// IDENTICAL Action panel.PanelGateDecision(facts) returns — so relocating
// any decision branch into a CLI adapter breaks this test.
func TestPanelVerbs_DecisionIsPanelGateDecision(t *testing.T) {
	sha := "abc1234def5678abc1234def5678abc1234def56"
	otherSHA := "999000999000999000999000999000999000beef"
	beadID := "mindspec-bd01"
	basePanel := func() *panel.Panel {
		return &panel.Panel{
			BeadID: ptrStr(beadID), Spec: "110", Target: "bead/" + beadID,
			Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha,
		}
	}
	reg := &panel.Registration{Dir: "/wt/review/demo"}

	rows := []struct {
		name  string
		facts panel.GateFacts
	}{
		{
			name:  "malformed registration",
			facts: panel.GateFacts{BeadID: beadID, Reg: reg, Res: &panel.Result{Dir: reg.Dir, PanelErr: errors.New("boom")}},
		},
		{
			name: "abandoned (Warn, before staleness)",
			facts: func() panel.GateFacts {
				p := basePanel()
				p.Abandoned = true
				p.AbandonReason = "max: superseded by bd99"
				return panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(p, 6, 0, 6, 1, nil), HeadSHA: otherSHA}
			}(),
		},
		{
			name:  "round mismatch",
			facts: panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 2, nil), HeadSHA: sha},
		},
		{
			name:  "missing ref (Warn)",
			facts: panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 1, nil), MissingRef: true},
		},
		{
			name:  "transient git error (Warn)",
			facts: panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 1, nil), GitErr: errors.New("git: lock contention")},
		},
		{
			name:  "stale SHA",
			facts: panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 1, nil), HeadSHA: otherSHA},
		},
		{
			name: "dirty tree",
			facts: panel.GateFacts{
				BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 1, nil), HeadSHA: sha,
				WorktreePath: "/wt", UserDirt: []string{"foo.go"},
			},
		},
		{
			name:  "incomplete",
			facts: panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 2, 0, 2, 1, nil), HeadSHA: sha},
		},
		{
			name:  "REJECT",
			facts: panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 5, 1, 6, 1, nil), HeadSHA: sha},
		},
		{
			name:  "hard_block",
			facts: panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 1, []string{"R1"}), HeadSHA: sha},
		},
		{
			name:  "sub-threshold",
			facts: panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 4, 0, 6, 1, nil), HeadSHA: sha},
		},
		{
			name:  "at-threshold",
			facts: panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 5, 0, 6, 1, nil), HeadSHA: sha},
		},
		{
			name:  "above-threshold",
			facts: panel.GateFacts{BeadID: beadID, Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 1, nil), HeadSHA: sha},
		},
	}

	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			want := panel.PanelGateDecision(tc.facts).Action

			_, verifyAction := renderPanelVerify(tc.facts.Res, tc.facts)
			if verifyAction != want {
				t.Errorf("renderPanelVerify action = %v, want %v", verifyAction, want)
			}

			_, tallyDecision := renderPanelTally(tc.facts.Res, tc.facts, nil)
			if tallyDecision.Action != want {
				t.Errorf("renderPanelTally action = %v, want %v", tallyDecision.Action, want)
			}
		})
	}
}
