package panel

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ptr is a tiny helper for *string panel fields.
func ptr(s string) *string { return &s }

// regn builds a Registration whose Slug() is the basename of dir.
func regn(dir string) *Registration { return &Registration{Dir: dir} }

// result builds a *Result around a Panel with the given approve count already
// tallied (verdicts synthesized so Complete()/MissingCount work).
func result(p *Panel, approves, rejects int, round int, malformed []string, hardBlocks []string) *Result {
	r := &Result{
		Dir:         "/wt/review/slug",
		Panel:       p,
		LatestRound: round,
		Approves:    approves,
		Rejects:     rejects,
		Malformed:   malformed,
		HardBlocks:  hardBlocks,
	}
	// Synthesize the verdict slice so len(Verdicts) reflects the real
	// present-verdict count (approves + rejects + neutral). For these
	// decision tests we only need the count and the file names.
	total := approves + rejects
	for i := 0; i < total; i++ {
		v := Verdict{File: slotFile(i, round), Slot: slotName(i), Round: round}
		if i < approves {
			v.Verdict = VerdictApprove
		} else {
			v.Verdict = VerdictReject
		}
		r.Verdicts = append(r.Verdicts, v)
	}
	if p != nil && round > 0 {
		r.RoundMismatch = p.Round != round
	}
	return r
}

func slotName(i int) string { return "rev" + string(rune('a'+i)) }
func slotFile(i, r int) string {
	return slotName(i) + "-round-" + string(rune('0'+r)) + ".json"
}

const fence = "Do NOT bypass with raw `git merge"

// TestPanelGateDecision is the full Spec 093 Req 12 decision matrix as a
// table-driven test against the PURE decision function (no os/exec, no fs,
// no git — every input is a fully-resolved GateFacts). Relocated from
// internal/hook with the decision (Spec 099 Bead 1) so the leaf package
// self-tests every short-circuit branch, including the transient-gitErr (5b)
// row that previously lived only in the hook's wiring test.
func TestPanelGateDecision(t *testing.T) {
	t.Parallel()

	sha := "abc1234def5678abc1234def5678abc1234def56"
	otherSHA := "999000999000999000999000999000999000beef"

	defaultPanel := func() *Panel {
		return &Panel{
			BeadID: ptr("mindspec-bd01"), Spec: "093", Target: "bead",
			Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha,
		}
	}

	tests := []struct {
		name        string
		facts       GateFacts
		want        GateAction
		mustHave    []string
		mustNotHave []string
	}{
		{
			name:     "escape hatch → Allow+Warn, never names the var in a Block",
			facts:    GateFacts{BeadID: "mindspec-bd01", SkipEnv: true},
			want:     Warn,
			mustHave: []string{"panel gate skipped", "mindspec-bd01"},
		},
		{
			name:  "no panel → fail-open Allow (HC-4)",
			facts: GateFacts{BeadID: "mindspec-bd01", Reg: nil, Res: nil},
			want:  Allow,
		},
		{
			name: "malformed registration → Block (not 'no panel')",
			facts: GateFacts{
				BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"),
				Res: &Result{Dir: "/wt/review/slug", PanelErr: errFake},
			},
			want:     Block,
			mustHave: []string{"unreadable", "slug", fence},
		},
		{
			name: "abandoned → Allow+Warn naming reason, BEFORE staleness (T3-1)",
			facts: func() GateFacts {
				p := defaultPanel()
				p.Abandoned = true
				p.AbandonReason = "max: superseded by bd99 2026-06-12"
				// Stale SHA present — must NOT cause a Block.
				return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"),
					Res: result(p, 0, 0, 1, nil, nil), HeadSHA: otherSHA}
			}(),
			want:        Warn,
			mustHave:    []string{"abandoned", "superseded by bd99"},
			mustNotHave: []string{fence},
		},
		{
			name: "round mismatch → Block",
			facts: func() GateFacts {
				p := defaultPanel()
				p.Round = 1
				return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"),
					Res: result(p, 6, 0, 2, nil, nil), HeadSHA: sha}
			}(),
			want:     Block,
			mustHave: []string{"out of date", "ms-panel-run step 0", fence},
		},
		{
			name: "missing ref → Allow+Warn (rerun-after-merge)",
			facts: GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"),
				Res: result(defaultPanel(), 5, 0, 1, nil, nil), MissingRef: true},
			want:        Warn,
			mustHave:    []string{"no longer exists", "bead/mindspec-bd01"},
			mustNotHave: []string{fence},
		},
		{
			name: "transient git error (5b) → Allow+Warn, honest (NOT 'merge landed')",
			facts: GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"),
				Res:    result(defaultPanel(), 5, 0, 1, nil, nil),
				GitErr: errors.New("exit status 128: not a git repository")},
			want:        Warn,
			mustHave:    []string{"transient git error", "bead/mindspec-bd01", "NOT a confirmed merge"},
			mustNotHave: []string{fence, "already landed"},
		},
		{
			name: "stale SHA → Block (lola-f4a8 pin)",
			facts: GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"),
				Res: result(defaultPanel(), 5, 0, 1, nil, nil), HeadSHA: otherSHA},
			want:     Block,
			mustHave: []string{"reviewed", "branch now at", fence},
		},
		{
			name: "dirty user tree → Block (CommitAll bypass)",
			facts: GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"),
				Res: result(defaultPanel(), 5, 0, 1, nil, nil), HeadSHA: sha,
				WorktreePath: "/wt", UserDirt: []string{"src/main.go"}},
			want:     Block,
			mustHave: []string{"uncommitted changes in /wt", "CommitAll", "src/main.go", fence},
		},
		{
			name: "worktree absent → dirty check skipped, falls through to threshold Allow",
			facts: GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"),
				Res: result(defaultPanel(), 6, 0, 1, nil, nil), HeadSHA: sha,
				WorktreeAbsent: true, UserDirt: []string{"ignored"}},
			want: Allow,
		},
		{
			name: "incomplete (4/6) → Block naming present files",
			facts: GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"),
				Res: result(defaultPanel(), 4, 0, 1, nil, nil), HeadSHA: sha},
			want:     Block,
			mustHave: []string{"incomplete", "4/6 verdicts present", "present:", fence},
		},
		{
			name: "REJECT → Block halt path",
			facts: GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"),
				Res: result(defaultPanel(), 5, 1, 1, nil, nil), HeadSHA: sha},
			want:     Block,
			mustHave: []string{"HARD block / REJECT", "halt path", fence},
		},
		{
			name: "hard_block → Block halt path even at full APPROVE",
			facts: GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"),
				Res: result(defaultPanel(), 6, 0, 1, nil, []string{"reva"}), HeadSHA: sha},
			want:     Block,
			mustHave: []string{"HARD block / REJECT", "hard_block from reva", fence},
		},
		{
			// Spec 114 R1 (one of the FOUR intended outcome flips): an
			// unresolved REQUEST_CHANGES is no longer out-voted by the
			// approve count. Was Allow.
			name: "threshold met: 6/6 present, 5 APPROVE + 1 dissent → Block (Spec 114 R1)",
			facts: func() GateFacts {
				p := defaultPanel()
				r := result(p, 5, 0, 1, nil, nil)
				r.Verdicts = append(r.Verdicts,
					Verdict{File: "z-round-1.json", Slot: "z", Round: 1, Verdict: VerdictRequestChanges})
				return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}
			}(),
			want:        Block,
			mustHave:    []string{"z", fence},
			mustNotHave: []string{"refut", "refutations", "panel refute"},
		},
		{
			name: "sub-threshold 4/6 (complete? no) — covered above; here 6 present 4 approve via neutral",
			facts: func() GateFacts {
				// 6 present verdicts, 4 APPROVE, 2 REQUEST_CHANGES (neutral).
				p := defaultPanel()
				r := result(p, 4, 0, 1, nil, nil)
				// add two neutral verdicts to reach Complete() == true.
				r.Verdicts = append(r.Verdicts,
					Verdict{File: "x-round-1.json", Slot: "x", Round: 1, Verdict: VerdictRequestChanges},
					Verdict{File: "y-round-1.json", Slot: "y", Round: 1, Verdict: VerdictRequestChanges})
				return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}
			}(),
			want:     Block,
			mustHave: []string{"4/6 APPROVE", "threshold is 5/6", "consolidated-round-1.md", fence},
		},
		{
			// Outcome-preserving update (Spec 114 R1 step 6f): this fixture's
			// SUBJECT is "no hardcoded 6", not RC tolerance, so the incidental
			// REQUEST_CHANGES filler is replaced with a genuine third APPROVE
			// to keep the Allow outcome under the new unresolved-RC leg.
			name: "expected_reviewers:3 → 3/3 present, 3 APPROVE Allow (no hardcoded 6)",
			facts: func() GateFacts {
				p := &Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 3, ReviewedHeadSHA: sha}
				r := result(p, 3, 0, 1, nil, nil)
				return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}
			}(),
			want: Allow,
		},
		{
			name: "expected_reviewers:3 → 1/3 APPROVE Block citing 2/3",
			facts: func() GateFacts {
				p := &Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 3, ReviewedHeadSHA: sha}
				r := result(p, 1, 0, 1, nil, nil)
				r.Verdicts = append(r.Verdicts,
					Verdict{File: "x-round-1.json", Slot: "x", Round: 1, Verdict: VerdictRequestChanges},
					Verdict{File: "y-round-1.json", Slot: "y", Round: 1, Verdict: VerdictRequestChanges})
				return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}
			}(),
			want:        Block,
			mustHave:    []string{"1/3 APPROVE", "threshold is 2/3"},
			mustNotHave: []string{"5/6", "/6"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PanelGateDecision(tt.facts)
			if got.Action != tt.want {
				t.Fatalf("action = %v, want %v\nmsg: %s", got.Action, tt.want, got.Message)
			}
			for _, s := range tt.mustHave {
				if !strings.Contains(got.Message, s) {
					t.Errorf("message missing %q:\n%s", s, got.Message)
				}
			}
			for _, s := range tt.mustNotHave {
				if strings.Contains(got.Message, s) {
					t.Errorf("message must NOT contain %q:\n%s", s, got.Message)
				}
			}
			// HC-7 across the whole table: no Block (or any) message names
			// the skip variable.
			if strings.Contains(got.Message, SkipPanelEnv) && tt.want == Block {
				t.Errorf("Block message must never print %s:\n%s", SkipPanelEnv, got.Message)
			}
		})
	}
}

// TestPanelGateDecision_UnresolvedRequestChangesBlocks (Spec 114 R1, AC1 +
// AC8 block-half + AC10 predicate): the new leg (9.5) — any unresolved
// REQUEST_CHANGES or unrecognized verdict Blocks the gate exactly like a
// REJECT, regardless of the approve count, since a mechanical gate cannot
// out-vote a reviewer's dissent by arithmetic. Table-driven over three
// shapes: a single unresolved RC (AC1), an unrecognized/non-standard verdict
// string (AC8 block-half — "not an APPROVE"), and two unresolved RC slots
// (naming both, per R3's multiplicity requirement). Every row re-asserts the
// AC10 no-advertise predicate: the message never contains a paste-able
// refutation incantation (Bead 1 has no refutation escape yet — ANY
// unresolved verdict blocks unconditionally) nor the skip variable (HC-7),
// and always carries the raw-merge fence.
func TestPanelGateDecision_UnresolvedRequestChangesBlocks(t *testing.T) {
	t.Parallel()
	sha := "abc1234def5678abc1234def5678abc1234def56"

	defaultPanel := func() *Panel {
		return &Panel{
			BeadID: ptr("mindspec-bd01"), Spec: "093", Target: "bead",
			Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha,
		}
	}

	tests := []struct {
		name     string
		facts    GateFacts
		mustHave []string
	}{
		{
			// AC1: 6 expected reviewers, complete latest-round verdicts,
			// fresh SHA, clean tree, 5 APPROVE + 1 REQUEST_CHANGES, no
			// refutation → Block naming the RC slot.
			name: "5 APPROVE + 1 REQUEST_CHANGES → Block naming the RC slot",
			facts: func() GateFacts {
				p := defaultPanel()
				r := result(p, 5, 0, 1, nil, nil)
				r.Verdicts = append(r.Verdicts,
					Verdict{File: "z-round-1.json", Slot: "z", Round: 1, Verdict: VerdictRequestChanges})
				return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}
			}(),
			mustHave: []string{"z", "5/6 APPROVE", "threshold is 5/6", fence},
		},
		{
			// AC8 (block-half): an unrecognized/non-standard verdict string
			// is "not an APPROVE" (tally.go's Approves/Rejects doc) and
			// Blocks identically to a REQUEST_CHANGES.
			name: `5 APPROVE + 1 unrecognized verdict ("MAYBE") → Block`,
			facts: func() GateFacts {
				p := defaultPanel()
				r := result(p, 5, 0, 1, nil, nil)
				r.Verdicts = append(r.Verdicts,
					Verdict{File: "z-round-1.json", Slot: "z", Round: 1, Verdict: "MAYBE"})
				return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}
			}(),
			mustHave: []string{"z", "5/6 APPROVE", fence},
		},
		{
			// Two unresolved RC slots → Block naming BOTH (R3a's
			// multiplicity requirement: each must be addressed individually).
			name: "two unresolved RC slots → Block naming both",
			facts: func() GateFacts {
				p := defaultPanel()
				r := result(p, 4, 0, 1, nil, nil)
				r.Verdicts = append(r.Verdicts,
					// revA/revB (not "x"/"y"): those single letters are
					// substrings of fixed text in every Block message (e.g.
					// "/ms-bead-fix" contains "x"; "bypass"/"only" contain
					// "y"), which made the mustHave assertion below pass even
					// with leg 9.5's multi-slot naming fully disabled (round-1
					// panel finding S2). revA/revB appear ONLY via the %s
					// slot-list substitution, so this row genuinely
					// discriminates leg 9.5.
					Verdict{File: "revA-round-1.json", Slot: "revA", Round: 1, Verdict: VerdictRequestChanges},
					Verdict{File: "revB-round-1.json", Slot: "revB", Round: 1, Verdict: VerdictRequestChanges})
				return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}
			}(),
			mustHave: []string{"revA", "revB", "4/6 APPROVE", fence},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PanelGateDecision(tt.facts)
			if got.Action != Block {
				t.Fatalf("action = %v, want Block\nmsg: %s", got.Action, got.Message)
			}
			for _, s := range tt.mustHave {
				if !strings.Contains(got.Message, s) {
					t.Errorf("message missing %q:\n%s", s, got.Message)
				}
			}
			// AC10 (no-advertise predicate): the message names the
			// unresolved slot(s) and ends with a recovery-shaped fence, but
			// never prints a paste-able refutation incantation (Bead 1 has
			// no refutation escape) nor the skip variable (HC-7).
			for _, s := range []string{"refute", "refutations", "panel refute", SkipPanelEnv} {
				if strings.Contains(got.Message, s) {
					t.Errorf("message must NOT contain %q (AC10/HC-7):\n%s", s, got.Message)
				}
			}
			if !strings.Contains(got.Message, fence) {
				t.Errorf("message must carry the raw-merge fence:\n%s", got.Message)
			}
		})
	}
}

// TestPanelGateDecision_ApprovesExcludesUnresolvedVerdicts (Spec 114 R1/OQ1,
// AC9 Bead-1 precursor): a REQUEST_CHANGES verdict never increments
// Result.Approves — the tally counts only canonical APPROVE/REJECT
// (tally.go:107-111), asserted against the REAL Tally() reader (not a
// hand-built Result), so the counting switch itself is pinned, not just the
// decision layer above it. This is the Bead-1-visible half of AC9 (a
// REFUTED RC never increments Approves either, once Bead 2 adds refutation
// — the invariant this pins is what makes that possible: the counting
// switch never treated RC as APPROVE in the first place, so a refutation
// can only ever remove a slot from the BLOCKING set, never add it to the
// approve count).
func TestPanelGateDecision_ApprovesExcludesUnresolvedVerdicts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, f := range []struct{ name, verdict string }{
		{"a-round-1.json", `{"verdict":"APPROVE"}`},
		{"b-round-1.json", `{"verdict":"APPROVE"}`},
		{"c-round-1.json", `{"verdict":"APPROVE"}`},
		{"d-round-1.json", `{"verdict":"APPROVE"}`},
		{"e-round-1.json", `{"verdict":"APPROVE"}`},
		{"z-round-1.json", `{"verdict":"REQUEST_CHANGES"}`},
	} {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.verdict), 0o644); err != nil {
			t.Fatalf("seed verdict file %s: %v", f.name, err)
		}
	}

	res, err := Tally(dir)
	if err != nil {
		t.Fatalf("Tally: %v", err)
	}
	if res.Approves != 5 {
		t.Fatalf("Approves = %d, want 5 (the RC slot must never be counted as APPROVE)", res.Approves)
	}
	unresolved := res.UnresolvedVerdicts()
	if len(unresolved) != 1 || unresolved[0].Slot != "z" {
		t.Fatalf("UnresolvedVerdicts() = %+v, want exactly the \"z\" RC slot", unresolved)
	}
}

// TestPanelGateDecision_ThresholdFloorIsLayeredNotReplaced (Spec 114
// R1/OQ1, AC7 Bead-1 precursor — layered model): the approve_threshold/N−1
// floor (leg 10) and the new unresolved-verdict leg (9.5) are TWO
// INDEPENDENT necessary conditions, not a replacement of one by the other.
// Bead 1 has no refutation yet, so AC7's full falsifier (a sub-threshold
// panel where every RC has been refuted still Blocks on the floor) is
// Bead 2's — refuting an RC is what could otherwise "buy past" the floor,
// and there is no refutation mechanism to attempt that with here. What
// Bead 1 DOES pin, as the necessary precondition for that later guarantee
// to mean anything: (a) leg (10)'s threshold floor remains independently
// reachable and blocking when NO unresolved verdict is present (proven
// by the resolved-threshold-0 case in
// TestPanelGateDecision_ConfigDefaultDoesNotAlterDecision) — leg 9.5 is
// additive, not a replacement; and (b) a panel that is BOTH sub-threshold
// AND carries an unresolved RC still Blocks with a message that names the
// dissent AND still reports the genuine sub-threshold tally (the leg-9.5
// message is a substring-set superset of leg-10's), so neither condition masks the
// other.
func TestPanelGateDecision_ThresholdFloorIsLayeredNotReplaced(t *testing.T) {
	t.Parallel()
	sha := "abc1234def5678abc1234def5678abc1234def56"
	p := &Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha}

	// 3 APPROVE + 3 REQUEST_CHANGES: sub-threshold (3 < 5) AND carrying
	// unresolved dissent — both conditions independently would Block; the
	// gate must Block via leg 9.5 (it precedes leg 10) while the message
	// still reflects the genuine 3/6 approve tally, proving the floor
	// information is not lost under the new leg.
	r := result(p, 3, 0, 1, nil, nil)
	r.Verdicts = append(r.Verdicts,
		Verdict{File: "x-round-1.json", Slot: "x", Round: 1, Verdict: VerdictRequestChanges},
		Verdict{File: "y-round-1.json", Slot: "y", Round: 1, Verdict: VerdictRequestChanges},
		Verdict{File: "z-round-1.json", Slot: "z", Round: 1, Verdict: VerdictRequestChanges})
	facts := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}

	got := PanelGateDecision(facts)
	if got.Action != Block {
		t.Fatalf("action = %v, want Block\nmsg: %s", got.Action, got.Message)
	}
	for _, want := range []string{"x", "y", "z", "3/6 APPROVE", "threshold is 5/6", fence} {
		if !strings.Contains(got.Message, want) {
			t.Errorf("message missing %q (both the dissent AND the genuine sub-threshold tally must survive):\n%s", want, got.Message)
		}
	}
}

// errFake is a sentinel error for the malformed-registration case.
var errFake = fakeErr("boom")

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

// TestPanelGateDecision_EveryBlockEndsWithFence asserts the raw-merge fence
// is the LAST line of every Block variant (single assertion across the
// table, Spec 093 AC "Block-message fence").
func TestPanelGateDecision_EveryBlockEndsWithFence(t *testing.T) {
	t.Parallel()
	sha := "abc1234def5678abc1234def5678abc1234def56"
	mk := func(mut func(*GateFacts)) GateFacts {
		p := &Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha}
		f := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"),
			Res: result(p, 6, 0, 1, nil, nil), HeadSHA: sha}
		mut(&f)
		return f
	}
	blocks := []GateFacts{
		mk(func(f *GateFacts) { f.Res.PanelErr = errFake; f.Res.Panel = nil }),
		mk(func(f *GateFacts) {
			f.Res = result(&Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 6}, 6, 0, 2, nil, nil)
		}),
		mk(func(f *GateFacts) { f.HeadSHA = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" }),
		mk(func(f *GateFacts) { f.WorktreePath = "/wt"; f.UserDirt = []string{"a.go"} }),
		mk(func(f *GateFacts) {
			f.Res = result(&Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha}, 3, 0, 1, nil, nil)
		}),
		mk(func(f *GateFacts) { f.Res.Rejects = 1 }),
	}
	for i, f := range blocks {
		got := PanelGateDecision(f)
		if got.Action != Block {
			t.Fatalf("case %d: expected Block, got %v (%s)", i, got.Action, got.Message)
		}
		if !strings.Contains(got.Message, fence) {
			t.Errorf("case %d: Block message missing raw-merge fence:\n%s", i, got.Message)
		}
	}
}
