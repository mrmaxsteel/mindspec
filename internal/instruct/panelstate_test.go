package instruct

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
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

// TestRender_AppendsPanelState proves Context.PanelState lands at the
// bottom of the rendered markdown (the SessionStart channel), and that
// an empty PanelState appends nothing.
func TestRender_AppendsPanelState(t *testing.T) {
	root := setupTestProject(t)
	s := &state.Focus{Mode: state.ModeImplement, ActiveSpec: "004-instruct", ActiveBead: "beads-001"}
	ctx := BuildContext(root, s)

	// Empty → nothing appended.
	out, err := Render(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Open Panel Rounds") {
		t.Error("empty PanelState must not render the block")
	}

	// Populated → block appears.
	ctx.PanelState = renderPanelState([]PanelStateEntry{
		{Slug: "p", Tally: beadPanel("b.1", "abc1234", 1, 6, 4, 0, 0, 2), LiveBranchSHA: "abc1234"},
	})
	out, err = Render(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Open Panel Rounds") {
		t.Errorf("populated PanelState must render the block\n%s", out)
	}
	if !strings.Contains(out, "gate would BLOCK") {
		t.Errorf("expected BLOCK verdict in render\n%s", out)
	}
}
