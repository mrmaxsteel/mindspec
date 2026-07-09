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
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
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

// --- Spec 113 R1: truthful non-bead staleness -------------------------------

// nonBeadPanelFixture writes a non-bead (BeadID nil) panel.json fixture plus
// one seeded verdict file, and returns the panel directory.
func nonBeadPanelFixture(t *testing.T, root, slug, target, sha, verdict string, expectedReviewers int) string {
	t.Helper()
	writePanelFixture(t, root, slug, panel.Panel{
		Spec: "113-nb", Target: target, Round: 1,
		ExpectedReviewers: expectedReviewers, ReviewedHeadSHA: sha,
	})
	dir := filepath.Join(root, "review", slug)
	body := fmt.Sprintf(`{"verdict":%q}`, verdict)
	if err := os.WriteFile(filepath.Join(dir, "R1-round-1.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("seed verdict file: %v", err)
	}
	return dir
}

// stubNonBeadRevParse points revParseForPanelFn at fn, restoring the
// original via t.Cleanup, and stubs the bd-worktree list so no test spawns a
// real subprocess.
func stubNonBeadRevParse(t *testing.T, fn func(root, ref string) (string, error)) {
	t.Helper()
	orig := revParseForPanelFn
	revParseForPanelFn = fn
	t.Cleanup(func() { revParseForPanelFn = orig })
	stubWorktreeListEmpty(t)
}

// runPanelVerbCmd runs `mindspec panel <args...>` against rootCmd and
// returns combined stdout+stderr and the error.
func runPanelVerbCmd(args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(append([]string{"panel"}, args...))
	err := rootCmd.Execute()
	return stdout.String() + stderr.String(), err
}

// TestPanelVerify_NonBeadStaleness is the AC1 pin for `panel verify` over a
// non-bead panel (spec 113 R1): the target advancing past reviewed_head_sha
// must Block (never PASS), the CLI must rev-parse the RECORDED target — not
// the literal, always-absent "bead/" — and no rendering may leak the
// malformed empty-interpolation `references branch bead/` fragment. A
// REJECT verdict at an UN-advanced target must also never PASS.
func TestPanelVerify_NonBeadStaleness(t *testing.T) {
	shaA := "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111"
	shaB := "bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222"
	target := "spec/113-x"

	t.Run("target advanced past reviewed_head_sha blocks", func(t *testing.T) {
		root := mkPanelTestRoot(t, "")
		nonBeadPanelFixture(t, root, "demo", target, shaA, panel.VerdictApprove, 1)
		withTestChdir(t, root)

		var gotRefs []string
		stubNonBeadRevParse(t, func(_, ref string) (string, error) {
			gotRefs = append(gotRefs, ref)
			return shaB, nil
		})

		out, err := runPanelVerbCmd("verify", "demo")
		if err != nil {
			t.Fatalf("panel verify: %v\noutput=%s", err, out)
		}

		if len(gotRefs) != 1 || gotRefs[0] != target {
			t.Fatalf("revParseForPanelFn rev-parsed refs %v, want exactly [%q] (never a bead/-derived ref)", gotRefs, target)
		}
		if strings.Contains(out, "PASS") {
			t.Errorf("output contains PASS for an advanced non-bead target:\n%s", out)
		}
		if !strings.Contains(out, "BLOCK") {
			t.Errorf("output missing BLOCK for an advanced non-bead target:\n%s", out)
		}
		if strings.Contains(out, "references branch bead/") {
			t.Errorf("output leaked the malformed bead/ fragment:\n%s", out)
		}
		if strings.Contains(out, "git merge bead/") {
			t.Errorf("output leaked the empty-interpolation merge fence:\n%s", out)
		}
		if strings.Contains(out, "mindspec complete") {
			t.Errorf("output emitted a mindspec complete instruction for a non-bead panel:\n%s", out)
		}
		if !strings.Contains(out, target) {
			t.Errorf("output does not name the recorded target %q:\n%s", target, out)
		}
	})

	t.Run("REJECT at an un-advanced target still does not PASS", func(t *testing.T) {
		root := mkPanelTestRoot(t, "")
		nonBeadPanelFixture(t, root, "demo", target, shaA, panel.VerdictReject, 1)
		withTestChdir(t, root)

		stubNonBeadRevParse(t, func(_, _ string) (string, error) { return shaA, nil }) // target NOT advanced

		out, err := runPanelVerbCmd("verify", "demo")
		if err != nil {
			t.Fatalf("panel verify: %v\noutput=%s", err, out)
		}
		if strings.Contains(out, "PASS") {
			t.Errorf("output contains PASS despite a REJECT verdict at an un-advanced target:\n%s", out)
		}
		if !strings.Contains(out, "BLOCK") {
			t.Errorf("output missing BLOCK for a REJECT verdict:\n%s", out)
		}
	})
}

// TestPanelTally_NonBeadRejectBlocks is the AC1 pin for `panel tally` over a
// non-bead panel with a REJECT verdict at an UN-advanced target: tally must
// exit non-zero with a final recovery line, never a `mindspec complete
// <bead>` instruction (a non-bead panel is not complete-gated), and never a
// bead/-empty-interpolation fragment.
func TestPanelTally_NonBeadRejectBlocks(t *testing.T) {
	sha := "cccc3333cccc3333cccc3333cccc3333cccc3333"
	target := "spec/113-y"
	root := mkPanelTestRoot(t, "")
	nonBeadPanelFixture(t, root, "demo", target, sha, panel.VerdictReject, 1)
	withTestChdir(t, root)

	stubNonBeadRevParse(t, func(_, _ string) (string, error) { return sha, nil }) // target NOT advanced

	out, err := runPanelVerbCmd("tally", "demo")
	if err == nil {
		t.Fatalf("expected a non-nil error (non-zero exit) for a REJECT verdict:\noutput=%s", out)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("expected a final recovery line, got: %v", err)
	}
	if strings.Contains(err.Error(), "mindspec complete") {
		t.Errorf("non-bead recovery must not emit `mindspec complete`: %v", err)
	}
	if strings.Contains(err.Error(), "bead/") {
		t.Errorf("non-bead recovery must not leak a bead/ fragment: %v", err)
	}
	if !strings.Contains(err.Error(), target) {
		t.Errorf("non-bead recovery does not name the recorded target %q: %v", target, err)
	}
}

// TestPanelTally_NonBeadStaleBlocks is the AC1 pin for `panel tally` over a
// non-bead panel whose target has advanced past reviewed_head_sha: exit
// non-zero with a final recovery line.
func TestPanelTally_NonBeadStaleBlocks(t *testing.T) {
	shaA := "dddd4444dddd4444dddd4444dddd4444dddd4444"
	shaB := "eeee5555eeee5555eeee5555eeee5555eeee5555"
	target := "spec/113-z"
	root := mkPanelTestRoot(t, "")
	nonBeadPanelFixture(t, root, "demo", target, shaA, panel.VerdictApprove, 1)
	withTestChdir(t, root)

	stubNonBeadRevParse(t, func(_, _ string) (string, error) { return shaB, nil }) // target ADVANCED

	out, err := runPanelVerbCmd("tally", "demo")
	if err == nil {
		t.Fatalf("expected a non-nil error (non-zero exit) for a stale non-bead target:\noutput=%s", out)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("expected a final recovery line, got: %v", err)
	}
	if strings.Contains(err.Error(), "mindspec complete") {
		t.Errorf("non-bead recovery must not emit `mindspec complete`: %v", err)
	}
}

// TestPanelVerify_NonBeadIncompleteBlocks is the AC1 E2E pin for leg (8)
// on a non-bead panel (spec 113 R1 explicitly names the incomplete case):
// at an UN-advanced target with FEWER seeded verdict files than
// expected_reviewers, the un-shadowed incomplete leg must Block — `panel
// verify` never PASSes and `panel tally` exits non-zero — with no
// `references branch bead/` or `mindspec complete <bead>` leak.
func TestPanelVerify_NonBeadIncompleteBlocks(t *testing.T) {
	sha := "7777aaaa7777aaaa7777aaaa7777aaaa7777aaaa"
	target := "spec/113-incomplete"

	t.Run("verify does not PASS", func(t *testing.T) {
		root := mkPanelTestRoot(t, "")
		// expected_reviewers 2, but nonBeadPanelFixture seeds exactly ONE
		// verdict file → 1/2 present → incomplete (leg 8).
		nonBeadPanelFixture(t, root, "demo", target, sha, panel.VerdictApprove, 2)
		withTestChdir(t, root)
		stubNonBeadRevParse(t, func(_, _ string) (string, error) { return sha, nil }) // target NOT advanced

		out, err := runPanelVerbCmd("verify", "demo")
		if err != nil {
			t.Fatalf("panel verify: %v\noutput=%s", err, out)
		}
		if strings.Contains(out, "PASS") {
			t.Errorf("output contains PASS for an incomplete non-bead panel:\n%s", out)
		}
		if !strings.Contains(out, "BLOCK") {
			t.Errorf("output missing BLOCK for an incomplete non-bead panel:\n%s", out)
		}
		if strings.Contains(out, "references branch bead/") {
			t.Errorf("output leaked the malformed bead/ fragment:\n%s", out)
		}
		if strings.Contains(out, "mindspec complete") {
			t.Errorf("output emitted a mindspec complete instruction for a non-bead panel:\n%s", out)
		}
	})

	t.Run("tally exits non-zero", func(t *testing.T) {
		root := mkPanelTestRoot(t, "")
		nonBeadPanelFixture(t, root, "demo", target, sha, panel.VerdictApprove, 2)
		withTestChdir(t, root)
		stubNonBeadRevParse(t, func(_, _ string) (string, error) { return sha, nil }) // target NOT advanced

		out, err := runPanelVerbCmd("tally", "demo")
		if err == nil {
			t.Fatalf("expected a non-nil error (non-zero exit) for an incomplete non-bead panel:\noutput=%s", out)
		}
		if !guard.HasFinalRecoveryLine(err.Error()) {
			t.Errorf("expected a final recovery line, got: %v", err)
		}
		if strings.Contains(err.Error(), "mindspec complete") {
			t.Errorf("non-bead recovery must not emit `mindspec complete`: %v", err)
		}
		if strings.Contains(err.Error(), "bead/") {
			t.Errorf("non-bead recovery must not leak a bead/ fragment: %v", err)
		}
	})
}

// TestPanelVerify_NonBeadMissingTargetRef pins the honest missing-ref path
// (leg 5) for a non-bead panel: when the rev-parse fails wrapping the REAL
// gitutil.ErrRefNotFound (not a fake sentinel — exec.IsRefNotFound's real
// errors.Is classification is exercised end to end), `panel verify` emits a
// Warn advisory naming the recorded target and never the malformed
// `references branch bead/,` fragment. verify is read-only, so it always
// exits 0 regardless.
func TestPanelVerify_NonBeadMissingTargetRef(t *testing.T) {
	sha := "ffff6666ffff6666ffff6666ffff6666ffff6666"
	target := "spec/113-deleted"
	root := mkPanelTestRoot(t, "")
	nonBeadPanelFixture(t, root, "demo", target, sha, panel.VerdictApprove, 1)
	withTestChdir(t, root)

	stubNonBeadRevParse(t, func(_, ref string) (string, error) {
		return "", fmt.Errorf("rev-parse %s: %w", ref, gitutil.ErrRefNotFound)
	})

	out, err := runPanelVerbCmd("verify", "demo")
	if err != nil {
		t.Fatalf("panel verify: %v\noutput=%s", err, out)
	}
	if !strings.Contains(out, target) {
		t.Errorf("missing-ref advisory does not name the target %q:\n%s", target, out)
	}
	// Finding-1 pin (spec 113 R1): assert the DISTINCTIVE leg-5 missing-ref
	// phrase "no longer exists" — a marker the transient leg-5b GitErr
	// rendering ("could not verify target ...") never emits. Without this
	// row the other assertions (names the target, no bead/ fragment, no
	// `mindspec complete`) are ALSO satisfied by the leg-5b rendering, so
	// the real exec.IsRefNotFound classification is executed but not pinned
	// — swapping GateIO.IsRefNotFound to `func(error) bool { return false }`
	// (routing this ErrRefNotFound-wrapping error to the transient leg)
	// leaves the suite green. Verified: this assertion reds under that
	// mutation and passes on pristine code, so it pins the missing-ref path.
	if !strings.Contains(out, "no longer exists") {
		t.Errorf("missing-ref advisory does not carry the distinctive leg-5 phrase %q (the transient leg-5b GitErr path would not) — the IsRefNotFound classification is unpinned:\n%s", "no longer exists", out)
	}
}

// TestSanitizeNonBeadDecision builds messages from the REAL
// panel.PanelGateDecision over beadID=="" fact rows spanning legs (2), (5),
// (5b), (6), (8), (9), (10), then asserts sanitizeNonBeadDecision's output
// bans every malformed bead/-empty-interpolation pattern and any
// `mindspec complete` instruction, preserves Action byte-for-byte, and
// names the recorded target on the (5)/(5b) paths — so this pin cannot
// drift from the real gate templates.
func TestSanitizeNonBeadDecision(t *testing.T) {
	slug := "demo"
	target := "spec/113-nb"
	sha := "1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa"
	otherSHA := "2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb"
	reg := &panel.Registration{Dir: "/wt/review/" + slug}
	basePanel := func() *panel.Panel {
		return &panel.Panel{Spec: "113-nb", Target: target, Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha}
	}

	rows := []struct {
		name        string
		facts       panel.GateFacts
		namesTarget bool
	}{
		{
			name:  "leg (2) malformed registration",
			facts: panel.GateFacts{BeadID: "", Reg: reg, Res: &panel.Result{Dir: reg.Dir, PanelErr: errors.New("boom")}},
		},
		{
			name: "leg (3) abandoned",
			facts: func() panel.GateFacts {
				p := basePanel()
				p.Abandoned = true
				p.AbandonReason = "max: superseded by bd99"
				return panel.GateFacts{BeadID: "", Reg: reg, Res: buildResult(p, 6, 0, 6, 1, nil), HeadSHA: otherSHA}
			}(),
		},
		{
			// Round==2 vs panel.Round==1 fires buildResult's RoundMismatch,
			// which the leg-4 template appends RawMergeFence("") to.
			name:  "leg (4) round mismatch",
			facts: panel.GateFacts{BeadID: "", Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 2, nil), HeadSHA: sha},
		},
		{
			name:        "leg (5) missing ref",
			facts:       panel.GateFacts{BeadID: "", Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 1, nil), MissingRef: true},
			namesTarget: true,
		},
		{
			name:        "leg (5b) transient git error",
			facts:       panel.GateFacts{BeadID: "", Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 1, nil), GitErr: errors.New("git: lock contention")},
			namesTarget: true,
		},
		{
			name:  "leg (6) stale SHA",
			facts: panel.GateFacts{BeadID: "", Reg: reg, Res: buildResult(basePanel(), 6, 0, 6, 1, nil), HeadSHA: otherSHA},
		},
		{
			name:  "leg (8) incomplete",
			facts: panel.GateFacts{BeadID: "", Reg: reg, Res: buildResult(basePanel(), 2, 0, 2, 1, nil), HeadSHA: sha},
		},
		{
			name:  "leg (9) REJECT",
			facts: panel.GateFacts{BeadID: "", Reg: reg, Res: buildResult(basePanel(), 5, 1, 6, 1, nil), HeadSHA: sha},
		},
		{
			name:  "leg (10) sub-threshold",
			facts: panel.GateFacts{BeadID: "", Reg: reg, Res: buildResult(basePanel(), 4, 0, 6, 1, nil), HeadSHA: sha},
		},
	}

	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			want := panel.PanelGateDecision(tc.facts)
			got := sanitizeNonBeadDecision(want, slug, target)

			if got.Action != want.Action {
				t.Fatalf("sanitizeNonBeadDecision changed Action: got %v, want %v", got.Action, want.Action)
			}
			if strings.Contains(got.Message, "references branch bead/,") {
				t.Errorf("sanitized message still leaks the missing-ref bead/ fragment: %q", got.Message)
			}
			if strings.Contains(got.Message, "could not verify branch bead/") {
				t.Errorf("sanitized message still leaks the transient-error bead/ fragment: %q", got.Message)
			}
			if strings.Contains(got.Message, panel.RawMergeFence("")) {
				t.Errorf("sanitized message still carries the empty-interpolation merge fence: %q", got.Message)
			}
			if strings.Contains(got.Message, "mindspec complete") {
				t.Errorf("sanitized message must never introduce a mindspec complete instruction: %q", got.Message)
			}
			if tc.namesTarget && !strings.Contains(got.Message, target) {
				t.Errorf("sanitized message does not name the recorded target %q: %q", target, got.Message)
			}
		})
	}
}
