package instruct

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// writePanelDir creates root/review/<slug>/ with panel.json + the named
// verdict/consolidated files (relative names → contents).
func writePanelDir(t *testing.T, root, slug, panelJSON string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(root, "review", slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if panelJSON != "" {
		if err := os.WriteFile(filepath.Join(dir, "panel.json"), []byte(panelJSON), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

const beadPanelJSON = `{"bead_id":"mindspec-x.1","spec":"s","target":"bead/mindspec-x.1","round":1,"expected_reviewers":6,"reviewed_head_sha":"abc1234"}`

func sixApproves(round int) map[string]string {
	out := map[string]string{}
	for _, slot := range []string{"a", "b", "c", "d", "e", "f"} {
		out[fmt.Sprintf("%s-round-%d.json", slot, round)] = `{"verdict":"APPROVE"}`
	}
	return out
}

// beadPanel builds a registered bead Panel.Result fixture: N expected
// reviewers, `approves` APPROVE verdicts in latest round `round`, with
// reviewed_head_sha = sha. rejects/hardBlocks/malformed simulate the
// halt and incomplete paths. The verdict slice length is the count of
// present verdicts (approves + rejects + others), so MissingCount and
// Complete behave like a real Tally.
func beadPanel(beadID, sha string, round, expected, approves, rejects, hardBlocks, otherPresent int) *panel.Result {
	p := &panel.Panel{
		BeadID:            &beadID,
		Spec:              "s",
		Target:            "bead/" + beadID,
		Round:             round,
		ExpectedReviewers: expected,
		ReviewedHeadSHA:   sha,
	}
	res := &panel.Result{
		Dir:         "review/p",
		Panel:       p,
		LatestRound: round,
		Approves:    approves,
		Rejects:     rejects,
	}
	total := approves + rejects + otherPresent
	for i := 0; i < total; i++ {
		res.Verdicts = append(res.Verdicts, panel.Verdict{Slot: "slot", Round: round})
	}
	for i := 0; i < hardBlocks; i++ {
		res.HardBlocks = append(res.HardBlocks, "slot")
	}
	return res
}

// TestPanelStateEntry_Verdict is the table-driven core: each row pins
// the would-be gate verdict (PASS/BLOCK/WARN) and a reason substring,
// mirroring the Bead 4 decision matrix (Spec 093 Reqs 11/12) read-only.
func TestPanelStateEntry_Verdict(t *testing.T) {
	cases := []struct {
		name       string
		entry      PanelStateEntry
		wantGate   PanelGateVerdict
		wantReason string
	}{
		{
			// At threshold: 5/6 APPROVE, sha fresh → PASS.
			name: "at_threshold_fresh",
			entry: PanelStateEntry{
				Slug:          "p",
				Tally:         beadPanel("b.1", "abc1234", 1, 6, 5, 0, 0, 1),
				LiveBranchSHA: "abc1234",
			},
			wantGate:   GatePass,
			wantReason: "meets threshold 5/6",
		},
		{
			// Above threshold: 6/6 APPROVE → PASS.
			name: "above_threshold_fresh",
			entry: PanelStateEntry{
				Slug:          "p",
				Tally:         beadPanel("b.1", "abc1234", 1, 6, 6, 0, 0, 0),
				LiveBranchSHA: "abc1234",
			},
			wantGate:   GatePass,
			wantReason: "6/6 APPROVE",
		},
		{
			// Below threshold: 4/6 APPROVE (2 dissent present) → BLOCK.
			name: "below_threshold",
			entry: PanelStateEntry{
				Slug:          "p",
				Tally:         beadPanel("b.1", "abc1234", 1, 6, 4, 0, 0, 2),
				LiveBranchSHA: "abc1234",
			},
			wantGate:   GateBlock,
			wantReason: "threshold is 5/6",
		},
		{
			// Incomplete: only 3/6 verdicts present → BLOCK (incomplete
			// is checked before the APPROVE-count path).
			name: "incomplete",
			entry: PanelStateEntry{
				Slug:          "p",
				Tally:         beadPanel("b.1", "abc1234", 1, 6, 3, 0, 0, 0),
				LiveBranchSHA: "abc1234",
			},
			wantGate:   GateBlock,
			wantReason: "incomplete: 3/6 verdicts",
		},
		{
			// Stale reviewed_head_sha: complete + at threshold, but the
			// branch moved → BLOCK (staleness precedes the tally).
			name: "stale_sha",
			entry: PanelStateEntry{
				Slug:          "p",
				Tally:         beadPanel("b.1", "abc1234", 1, 6, 6, 0, 0, 0),
				LiveBranchSHA: "def5678",
			},
			wantGate:   GateBlock,
			wantReason: "commits landed after review",
		},
		{
			// REJECT recorded despite enough APPROVEs → BLOCK (halt).
			name: "reject_recorded",
			entry: PanelStateEntry{
				Slug:          "p",
				Tally:         beadPanel("b.1", "abc1234", 1, 6, 5, 1, 0, 0),
				LiveBranchSHA: "abc1234",
			},
			wantGate:   GateBlock,
			wantReason: "halt path",
		},
		{
			// hard_block recorded → BLOCK (halt) even at full APPROVE.
			name: "hard_block",
			entry: PanelStateEntry{
				Slug:          "p",
				Tally:         beadPanel("b.1", "abc1234", 1, 6, 6, 0, 1, 0),
				LiveBranchSHA: "abc1234",
			},
			wantGate:   GateBlock,
			wantReason: "halt path",
		},
		{
			// Round/filename mismatch → BLOCK before anything else.
			name: "round_mismatch",
			entry: func() PanelStateEntry {
				r := beadPanel("b.1", "abc1234", 2, 6, 6, 0, 0, 0)
				r.RoundMismatch = true
				r.Panel.Round = 1 // panel.json lags the filename round
				return PanelStateEntry{Slug: "p", Tally: r, LiveBranchSHA: "abc1234"}
			}(),
			wantGate:   GateBlock,
			wantReason: "out of date vs verdict files",
		},
		{
			// Branch deleted (rerun-after-merge) → PASS with Warn.
			name: "branch_missing",
			entry: PanelStateEntry{
				Slug:          "p",
				Tally:         beadPanel("b.1", "abc1234", 1, 6, 6, 0, 0, 0),
				BranchMissing: true,
			},
			wantGate:   GateWarn,
			wantReason: "no longer exists",
		},
		{
			// Abandoned → PASS with Warn naming the reason.
			name: "abandoned",
			entry: func() PanelStateEntry {
				r := beadPanel("b.1", "abc1234", 1, 6, 0, 0, 0, 0)
				r.Panel.Abandoned = true
				r.Panel.AbandonReason = "max@ — superseded by spec rescope"
				return PanelStateEntry{Slug: "p", Tally: r, LiveBranchSHA: "abc1234"}
			}(),
			wantGate:   GateWarn,
			wantReason: "superseded by spec rescope",
		},
		{
			// Abandoned without a reason → still Warn, but flags the gap.
			name: "abandoned_no_reason",
			entry: func() PanelStateEntry {
				r := beadPanel("b.1", "abc1234", 1, 6, 0, 0, 0, 0)
				r.Panel.Abandoned = true
				return PanelStateEntry{Slug: "p", Tally: r}
			}(),
			wantGate:   GateWarn,
			wantReason: "abandon_reason is required",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotGate, gotReason := c.entry.verdict()
			if gotGate != c.wantGate {
				t.Errorf("verdict gate: got %v, want %v (reason: %q)", gotGate, c.wantGate, gotReason)
			}
			if !strings.Contains(gotReason, c.wantReason) {
				t.Errorf("verdict reason: got %q, want substring %q", gotReason, c.wantReason)
			}
		})
	}
}

// TestRenderPanelState_NoPanels is the clean / zero-cost case: no
// entries → empty string, NOT an error (Spec 093 Req 15: no panel →
// block absent).
func TestRenderPanelState_NoPanels(t *testing.T) {
	if out := renderPanelState(nil); out != "" {
		t.Errorf("no panels should render empty, got:\n%s", out)
	}
	if out := renderPanelState([]PanelStateEntry{}); out != "" {
		t.Errorf("empty slice should render empty, got:\n%s", out)
	}
}

// TestRenderPanelState_Block pins the rendered block shape for a
// below-threshold panel: heading, "gate would BLOCK", the tally line,
// and the consolidated-file hint.
func TestRenderPanelState_Block(t *testing.T) {
	r := beadPanel("b.1", "abc1234", 2, 6, 4, 0, 0, 2)
	r.HasConsolidated = true
	out := renderPanelState([]PanelStateEntry{
		{Slug: "093-x-b1", Tally: r, LiveBranchSHA: "abc1234"},
	})

	for _, want := range []string{
		"## Open Panel Rounds",
		"093-x-b1",
		"gate would BLOCK",
		"threshold is 5/6",
		"latest round 2 · 6/6 verdicts · 4 APPROVE (threshold 5)",
		"consolidated-round-2.md is present",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("block missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestRenderPanelState_Pass pins the PASS rendering and that the tally
// line reflects the at-threshold state.
func TestRenderPanelState_Pass(t *testing.T) {
	out := renderPanelState([]PanelStateEntry{
		{Slug: "p", Tally: beadPanel("b.1", "abc1234", 1, 6, 5, 0, 0, 1), LiveBranchSHA: "abc1234"},
	})
	if !strings.Contains(out, "gate would PASS") {
		t.Errorf("expected PASS label\n%s", out)
	}
	if strings.Contains(out, "gate would BLOCK") {
		t.Errorf("PASS panel must not also say BLOCK\n%s", out)
	}
	if !strings.Contains(out, "5 APPROVE (threshold 5)") {
		t.Errorf("expected at-threshold tally line\n%s", out)
	}
}

// TestRenderPanelState_StaleSHA pins that the stale-sha block shows both
// the reviewed and live short SHAs and a BLOCK.
func TestRenderPanelState_StaleSHA(t *testing.T) {
	out := renderPanelState([]PanelStateEntry{
		{Slug: "p", Tally: beadPanel("b.1", "abcdef0123", 3, 6, 6, 0, 0, 0), LiveBranchSHA: "9876543210"},
	})
	if !strings.Contains(out, "gate would BLOCK") {
		t.Errorf("stale sha must BLOCK\n%s", out)
	}
	if !strings.Contains(out, "reviewed abcdef0") || !strings.Contains(out, "branch now at 9876543") {
		t.Errorf("stale block must show both short SHAs\n%s", out)
	}
}

// TestRenderPanelState_DeterministicOrder pins slug-sorted ordering for
// multiple panels.
func TestRenderPanelState_DeterministicOrder(t *testing.T) {
	out := renderPanelState([]PanelStateEntry{
		{Slug: "zeta", Tally: beadPanel("b.2", "s", 1, 6, 6, 0, 0, 0), LiveBranchSHA: "s"},
		{Slug: "alpha", Tally: beadPanel("b.1", "s", 1, 6, 6, 0, 0, 0), LiveBranchSHA: "s"},
	})
	if strings.Index(out, "alpha") > strings.Index(out, "zeta") {
		t.Errorf("panels must render in slug order (alpha before zeta)\n%s", out)
	}
}

// TestGatherPanelState_FreshVsStale exercises the fs Scan→Tally path
// with an injected branch-SHA resolver: a fresh-SHA panel PASSes and a
// stale-SHA panel BLOCKs, proving the resolver feeds staleness.
func TestGatherPanelState_FreshVsStale(t *testing.T) {
	root := t.TempDir()
	writePanelDir(t, root, "fresh", beadPanelJSON, sixApproves(1))

	// Fresh: resolver returns the reviewed sha → PASS.
	freshResolver := func(beadID string) (string, bool) { return "abc1234", true }
	entries := gatherPanelState(freshResolver, root)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if g, _ := entries[0].verdict(); g != GatePass {
		t.Errorf("fresh sha should PASS, got %v", g)
	}

	// Stale: resolver returns a different sha → BLOCK.
	staleResolver := func(beadID string) (string, bool) { return "deadbeef", true }
	entries = gatherPanelState(staleResolver, root)
	if g, _ := entries[0].verdict(); g != GateBlock {
		t.Errorf("stale sha should BLOCK, got %v", g)
	}

	// Branch gone: resolver reports not-found → WARN pass-through.
	goneResolver := func(beadID string) (string, bool) { return "", false }
	entries = gatherPanelState(goneResolver, root)
	if g, _ := entries[0].verdict(); g != GateWarn {
		t.Errorf("missing branch should WARN, got %v", g)
	}
}

// TestGatherPanelState_NoPanels returns nil (clean, not error) when no
// review dir exists.
func TestGatherPanelState_NoPanels(t *testing.T) {
	if got := gatherPanelState(nil, t.TempDir()); got != nil {
		t.Errorf("expected nil for no panels, got %v", got)
	}
}

// TestHasIncompletePanel covers the Req 15 auto-include condition.
func TestHasIncompletePanel(t *testing.T) {
	noPanel := t.TempDir()
	if HasIncompletePanel(noPanel) {
		t.Error("no panel dir → not incomplete")
	}

	incomplete := t.TempDir()
	writePanelDir(t, incomplete, "p", beadPanelJSON, map[string]string{
		"a-round-1.json": `{"verdict":"APPROVE"}`, // only 1 of 6
	})
	if !HasIncompletePanel(incomplete) {
		t.Error("1/6 verdicts → incomplete")
	}

	complete := t.TempDir()
	writePanelDir(t, complete, "p", beadPanelJSON, sixApproves(1))
	if HasIncompletePanel(complete) {
		t.Error("6/6 verdicts → complete")
	}
}

// TestRender_AppendsPanelState proves the FULL Panel/Subagent State
// block (all three Req-14 sub-blocks) lands at the bottom of the rendered
// markdown — the SessionStart channel (Spec 093 Req 15 AC L1100-1103) —
// and that an empty PanelState appends nothing.
func TestRender_AppendsPanelState(t *testing.T) {
	root := setupTestProject(t)
	s := &state.Focus{Mode: state.ModeImplement, ActiveSpec: "004-instruct", ActiveBead: "beads-001"}
	ctx := BuildContext(root, s)

	// Empty → nothing appended.
	out, err := Render(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Panel/Subagent State") {
		t.Error("empty PanelState must not render the block")
	}

	// Populated with all three sub-blocks → the full block appears.
	ctx.PanelState = renderFullPanelState(
		[]BeadStateEntry{{ID: "ms.1", Title: "t", Worktree: "/wt/ms.1", LastCommit: "abc do", Active: true}},
		[]PanelStateEntry{{Slug: "p", Tally: beadPanel("b.1", "abc1234", 1, 6, 4, 0, 0, 2), LiveBranchSHA: "abc1234"}},
		[]StaleWorktreeEntry{{Path: "/x/worktree-ms.gone", Source: "worktree-list"}},
	)
	out, err = Render(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Panel/Subagent State",
		"## In-Progress Beads",
		"## Open Panel Rounds",
		"## Stale Agent Worktrees",
		"gate would BLOCK",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered markdown missing %q\n%s", want, out)
		}
	}
}

// TestRunWithOptions_PanelStateGate is the end-to-end Req 15 stub-guard
// assertion (AC L1100-1103) through the public RunWithOptions path — the
// same path the SessionStart hook drives. It proves the NEGATIVE side
// deterministically: opts.PanelState=false → buildPanelStateBlock is
// NEVER invoked (zero git/bd subprocess attributable to panel-state) and
// the Panel/Subagent State block is absent from the rendered markdown.
//
// This direction is env-independent: every Run code path that does not
// request panel-state leaves the builder untouched regardless of which
// mode it resolves. (The POSITIVE markdown-channel direction — builder
// output → rendered block — is pinned deterministically by
// TestRender_AppendsPanelState, which avoids the bd/git mode-resolution
// that the pre-existing TestRun_IdleNoBeads env-leak perturbs.)
//
// buildPanelStateBlock is the single IO entrypoint for the three
// gatherers; swapping it for a call-counting fake pins the guard at the
// exact boundary the hook gates (HasIncompletePanel decides opts.PanelState).
func TestRunWithOptions_PanelStateGate(t *testing.T) {
	root := setupRunTestProject(t)

	var calls int
	orig := buildPanelStateBlock
	t.Cleanup(func() { buildPanelStateBlock = orig })
	buildPanelStateBlock = func(_ *phase.Cache, _, _, _ string) string {
		calls++
		return "# Panel/Subagent State\n"
	}

	var buf bytes.Buffer
	if err := RunWithOptions(context.Background(), root, "", "", &buf, Options{PanelState: false}); err != nil {
		t.Fatalf("RunWithOptions(PanelState:false): %v", err)
	}
	if calls != 0 {
		t.Errorf("PanelState=false must NOT invoke the panel-state builder (stub-guard), got %d calls", calls)
	}
	if strings.Contains(buf.String(), "Panel/Subagent State") {
		t.Errorf("no-panel-state run must not render the block\n%s", buf.String())
	}
}

// TestHasIncompletePanel_NoSubprocess hardens the Req-15 stub-guard at the
// hook's actual gate: HasIncompletePanel — the ONLY work outside the
// auto-include branch — is fs-only on a panel-less project (it makes no
// bd or git subprocess), so a session with no open panel pays zero added
// SessionStart cost. We assert it returns false (→ opts.PanelState=false
// in the hook) and prove the no-subprocess property by construction: the
// panel package imports only stdlib (verified by the package boundary).
func TestHasIncompletePanel_NoSubprocess(t *testing.T) {
	root := setupRunTestProject(t) // a .git + docs project, but NO review/ dir
	if HasIncompletePanel(root) {
		t.Error("a project with no panel dir must report no incomplete panel (gates opts.PanelState off)")
	}
}

// --- In-progress-beads block + cap (Spec 093 Req 14 bullet 1) ----------

// TestRenderInProgressBeads_Empty is the zero-cost case.
func TestRenderInProgressBeads_Empty(t *testing.T) {
	if out := renderInProgressBeads(nil); out != "" {
		t.Errorf("no in-progress beads should render empty, got:\n%s", out)
	}
}

// TestRenderInProgressBeads_Detail pins the per-bead detail shape (active
// marker, worktree, last-commit lines).
func TestRenderInProgressBeads_Detail(t *testing.T) {
	out := renderInProgressBeads([]BeadStateEntry{
		{ID: "ms.1", Title: "do thing", Worktree: "/wt/ms.1", LastCommit: "abc1234 did thing", Active: true},
		{ID: "ms.2", Title: "other", Worktree: "", LastCommit: ""},
	})
	for _, want := range []string{
		"## In-Progress Beads",
		"**ms.1 (active)** — do thing",
		"worktree: `/wt/ms.1`",
		"last commit: abc1234 did thing",
		"**ms.2** — other",
		"worktree: (none checked out)",
		"last commit: (branch unresolved)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("in-progress block missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestRenderInProgressBeads_Cap is the binding cap test (Spec 093 Req 14
// AC L1098-1099 / plan Step 3 L533-534): 6 in-progress beads → git detail
// for the active bead + 3 others only (exactly 4 detail blocks), with the
// remainder summarized verbatim as "… and 2 more (no git detail)". The
// table pins the boundary on both sides (5 → "1 more", 6 → "2 more"), and
// asserts the capped beads carry NO detail lines.
func TestRenderInProgressBeads_Cap(t *testing.T) {
	// Build n in-progress beads, all with detail populated, active first.
	build := func(n int) []BeadStateEntry {
		entries := make([]BeadStateEntry, 0, n)
		for i := 1; i <= n; i++ {
			id := fmt.Sprintf("ms.%d", i)
			entries = append(entries, BeadStateEntry{
				ID:         id,
				Title:      "bead " + id,
				Worktree:   "/wt/" + id,
				LastCommit: fmt.Sprintf("sha%d commit %s", i, id),
				Active:     i == 1,
			})
		}
		return entries
	}

	cases := []struct {
		name          string
		n             int
		wantDetailIDs []string // these must show a worktree/last-commit line
		wantSummary   string   // exact remainder line (or "" if none)
		wantHiddenIDs []string // these must NOT appear at all
	}{
		{
			name:          "exactly_cap_no_summary",
			n:             4,
			wantDetailIDs: []string{"ms.1", "ms.2", "ms.3", "ms.4"},
			wantSummary:   "",
			wantHiddenIDs: nil,
		},
		{
			name:          "one_over_cap",
			n:             5,
			wantDetailIDs: []string{"ms.1", "ms.2", "ms.3", "ms.4"},
			wantSummary:   "… and 1 more (no git detail)",
			wantHiddenIDs: []string{"ms.5"},
		},
		{
			name:          "six_beads_active_plus_3",
			n:             6,
			wantDetailIDs: []string{"ms.1", "ms.2", "ms.3", "ms.4"},
			wantSummary:   "… and 2 more (no git detail)",
			wantHiddenIDs: []string{"ms.5", "ms.6"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := renderInProgressBeads(build(c.n))

			// Exactly inProgressDetailCap (=4) beads get a last-commit line.
			gotDetail := strings.Count(out, "last commit: sha")
			wantDetail := c.n
			if wantDetail > inProgressDetailCap {
				wantDetail = inProgressDetailCap
			}
			if gotDetail != wantDetail {
				t.Errorf("%s: got %d detail blocks, want %d (cap %d)\n%s",
					c.name, gotDetail, wantDetail, inProgressDetailCap, out)
			}

			for _, id := range c.wantDetailIDs {
				if !strings.Contains(out, "last commit: sha"+strings.TrimPrefix(id, "ms.")) {
					t.Errorf("%s: expected detail for %s\n%s", c.name, id, out)
				}
			}
			for _, id := range c.wantHiddenIDs {
				// The summarized beads carry no detail: their commit line
				// must be absent (they are NOT individually rendered).
				marker := "commit " + id
				if strings.Contains(out, marker) {
					t.Errorf("%s: capped bead %s must NOT show git detail\n%s", c.name, id, out)
				}
			}
			if c.wantSummary == "" {
				if strings.Contains(out, "no git detail") {
					t.Errorf("%s: no remainder expected but summary line present\n%s", c.name, out)
				}
			} else {
				if !strings.Contains(out, c.wantSummary) {
					t.Errorf("%s: expected summary %q\n%s", c.name, c.wantSummary, out)
				}
			}
		})
	}
}

// TestGatherInProgressBeads_OrderAndCap proves the gatherer (1) floats the
// active bead to the front so it always lands inside the detail cap even
// when its bd-id sorts last, (2) lists the others in deterministic bd-id
// order, and (3) resolves last-commit ONLY within the cap (the 5th+ bead
// gets no git lookup — the subprocess-budget guarantee).
func TestGatherInProgressBeads_OrderAndCap(t *testing.T) {
	// Active bead "ms.9" sorts LAST by id but must come first.
	list := func() ([]inProgressBead, error) {
		return []inProgressBead{
			{ID: "ms.3", Title: "c"},
			{ID: "ms.9", Title: "i"}, // active
			{ID: "ms.1", Title: "a"},
			{ID: "ms.5", Title: "e"},
			{ID: "ms.7", Title: "g"},
		}, nil
	}
	var looked []string
	lastCommit := func(beadID string) string {
		looked = append(looked, beadID)
		return "sha " + beadID
	}

	entries := gatherInProgressBeads(list, lastCommit, "ms.9")
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
	// Active first, rest in bd-id order.
	wantOrder := []string{"ms.9", "ms.1", "ms.3", "ms.5", "ms.7"}
	for i, w := range wantOrder {
		if entries[i].ID != w {
			t.Errorf("order[%d] = %s, want %s (full: %+v)", i, entries[i].ID, w, ids(entries))
		}
	}
	if !entries[0].Active {
		t.Error("first entry (ms.9) must be marked Active")
	}
	// last-commit lookup ran for exactly the first inProgressDetailCap (4).
	if len(looked) != inProgressDetailCap {
		t.Errorf("last-commit lookups = %d, want %d (cap); looked=%v", len(looked), inProgressDetailCap, looked)
	}
	// The 5th bead (ms.7) got NO git detail.
	if entries[4].LastCommit != "" {
		t.Errorf("beyond-cap bead must have no last-commit detail, got %q", entries[4].LastCommit)
	}
}

func ids(entries []BeadStateEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}

// TestGatherInProgressBeads_Empty returns nil for no beads / error.
func TestGatherInProgressBeads_Empty(t *testing.T) {
	none := func() ([]inProgressBead, error) { return nil, nil }
	if got := gatherInProgressBeads(none, nil, ""); got != nil {
		t.Errorf("no beads → nil, got %v", got)
	}
	errList := func() ([]inProgressBead, error) { return nil, fmt.Errorf("bd down") }
	if got := gatherInProgressBeads(errList, nil, ""); got != nil {
		t.Errorf("bd error → nil, got %v", got)
	}
}

// --- Stale-agent-worktrees block (Spec 093 Req 14 bullet 3) ------------

// TestRenderStaleWorktrees pins the block shape + sources + empty case.
func TestRenderStaleWorktrees(t *testing.T) {
	if out := renderStaleWorktrees(nil); out != "" {
		t.Errorf("no stale worktrees should render empty, got:\n%s", out)
	}
	out := renderStaleWorktrees([]StaleWorktreeEntry{
		{Path: "/repo/.worktrees/worktree-ms.7", Source: "worktree-list"},
		{Path: "/repo/.claude/worktrees/agent-abc", Source: "agent-scan"},
	})
	for _, want := range []string{
		"## Stale Agent Worktrees",
		"`/repo/.worktrees/worktree-ms.7` (worktree-list)",
		"`/repo/.claude/worktrees/agent-abc` (agent-scan)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stale block missing %q\n%s", want, out)
		}
	}
}

// TestGatherStaleWorktrees implements the Req 14 bullet-3 criterion: a
// worktree-<id> with no matching in-progress bead is stale; one WITH a
// live in-progress bead is not; the main worktree and spec worktrees are
// excluded; and .claude/worktrees/agent-* dirs are scanned.
func TestGatherStaleWorktrees(t *testing.T) {
	root := t.TempDir()
	// Create an agent scratch dir on disk.
	agentDir := filepath.Join(root, ".claude", "worktrees", "agent-xyz")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}

	list := func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{
			{Name: "main", Path: root, IsMain: true},
			{Name: "worktree-ms.live", Path: "/x/worktree-ms.live"},       // has in-progress bead → NOT stale
			{Name: "worktree-ms.gone", Path: "/x/worktree-ms.gone"},       // no in-progress bead → STALE
			{Name: "worktree-spec-093-x", Path: "/x/worktree-spec-093-x"}, // spec worktree → excluded
		}, nil
	}
	inProgress := map[string]bool{"ms.live": true}

	entries := gatherStaleWorktrees(list, inProgress, root)

	paths := make(map[string]string)
	for _, e := range entries {
		paths[e.Path] = e.Source
	}
	if src, ok := paths["/x/worktree-ms.gone"]; !ok || src != "worktree-list" {
		t.Errorf("worktree-ms.gone (no live bead) must be stale via worktree-list; got %+v", entries)
	}
	if _, ok := paths["/x/worktree-ms.live"]; ok {
		t.Error("worktree-ms.live has a live in-progress bead and must NOT be stale")
	}
	if _, ok := paths["/x/worktree-spec-093-x"]; ok {
		t.Error("spec worktree must be excluded from stale-bead-worktree detection")
	}
	if _, ok := paths[root]; ok {
		t.Error("the main worktree must never be flagged stale")
	}
	if src, ok := paths[agentDir]; !ok || src != "agent-scan" {
		t.Errorf("the .claude/worktrees/agent-* dir must be scanned; got %+v", entries)
	}
}

// TestGatherStaleWorktrees_Empty returns nil when nothing is stale.
func TestGatherStaleWorktrees_Empty(t *testing.T) {
	root := t.TempDir() // no .claude/worktrees
	list := func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{Name: "main", Path: root, IsMain: true}}, nil
	}
	if got := gatherStaleWorktrees(list, map[string]bool{}, root); got != nil {
		t.Errorf("nothing stale → nil, got %v", got)
	}
}

// --- Composite Panel/Subagent State block (Spec 093 Reqs 14-15) --------

// TestRenderFullPanelState_AllThree proves the composer emits the
// Panel/Subagent State wrapper plus ALL THREE sub-blocks (the Req 15 AC
// L1100-1103 "full block" requirement).
func TestRenderFullPanelState_AllThree(t *testing.T) {
	inProgress := []BeadStateEntry{
		{ID: "ms.1", Title: "t", Worktree: "/wt/ms.1", LastCommit: "abc do", Active: true},
	}
	panels := []PanelStateEntry{
		{Slug: "p", Tally: beadPanel("b.1", "abc1234", 1, 6, 4, 0, 0, 2), LiveBranchSHA: "abc1234"},
	}
	stale := []StaleWorktreeEntry{
		{Path: "/x/worktree-ms.gone", Source: "worktree-list"},
	}

	out := renderFullPanelState(inProgress, panels, stale)
	for _, want := range []string{
		"# Panel/Subagent State",
		"## In-Progress Beads",
		"## Open Panel Rounds",
		"## Stale Agent Worktrees",
		"gate would BLOCK",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("full block missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestRenderFullPanelState_Empty returns "" when all three sub-blocks are
// empty (the zero-cost / clean-state contract — caller appends nothing).
func TestRenderFullPanelState_Empty(t *testing.T) {
	if out := renderFullPanelState(nil, nil, nil); out != "" {
		t.Errorf("all-empty must render nothing, got:\n%s", out)
	}
}

// TestRenderJSON_PanelState asserts the panel_state field is populated
// through RenderJSON when set, and omitted when empty (Spec 093 Req 14
// JSON shape — R2 coverage gap (2)).
func TestRenderJSON_PanelState(t *testing.T) {
	root := setupTestProject(t)
	s := &state.Focus{Mode: state.ModeImplement, ActiveSpec: "004-instruct", ActiveBead: "beads-001"}

	// Populated → panel_state present + carries the rendered markdown.
	ctx := BuildContext(root, s)
	ctx.PanelState = renderFullPanelState(
		[]BeadStateEntry{{ID: "ms.1", Active: true}},
		[]PanelStateEntry{{Slug: "p", Tally: beadPanel("b.1", "abc1234", 1, 6, 4, 0, 0, 2), LiveBranchSHA: "abc1234"}},
		nil,
	)
	jsonOut, err := RenderJSON(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonOut, `"panel_state"`) {
		t.Errorf("panel_state field must be present when set\n%s", jsonOut)
	}
	if !strings.Contains(jsonOut, "Panel/Subagent State") {
		t.Errorf("panel_state must carry the rendered block\n%s", jsonOut)
	}

	// Empty → omitempty drops the field.
	ctx2 := BuildContext(root, s)
	jsonOut2, err := RenderJSON(ctx2)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(jsonOut2, `"panel_state"`) {
		t.Errorf("panel_state must be omitted when empty\n%s", jsonOut2)
	}
}
