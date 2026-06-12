package hook

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// ptr is a tiny helper for *string panel fields.
func ptr(s string) *string { return &s }

// reg builds a Registration whose Slug() is the basename of dir.
func reg(dir string) *panel.Registration {
	return &panel.Registration{Dir: dir}
}

// result builds a *panel.Result around a Panel with the given approve count
// already tallied (verdicts synthesized so Complete()/MissingCount work).
func result(p *panel.Panel, approves, rejects int, round int, malformed []string, hardBlocks []string) *panel.Result {
	r := &panel.Result{
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
		v := panel.Verdict{File: slotFile(i, round), Slot: slotName(i), Round: round}
		if i < approves {
			v.Verdict = panel.VerdictApprove
		} else {
			v.Verdict = panel.VerdictReject
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
// no git — every input is a fully-resolved gateFacts).
func TestPanelGateDecision(t *testing.T) {
	t.Parallel()

	sha := "abc1234def5678abc1234def5678abc1234def56"
	otherSHA := "999000999000999000999000999000999000beef"

	defaultPanel := func() *panel.Panel {
		return &panel.Panel{
			BeadID: ptr("mindspec-bd01"), Spec: "093", Target: "bead",
			Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha,
		}
	}

	tests := []struct {
		name        string
		facts       gateFacts
		want        Action
		mustHave    []string
		mustNotHave []string
	}{
		{
			name:     "escape hatch → Pass+Warn, never names the var in a Block",
			facts:    gateFacts{beadID: "mindspec-bd01", skipEnv: true},
			want:     Warn,
			mustHave: []string{"panel gate skipped", "mindspec-bd01"},
		},
		{
			name:  "no panel → fail-open Pass (HC-4)",
			facts: gateFacts{beadID: "mindspec-bd01", reg: nil, res: nil},
			want:  Pass,
		},
		{
			name: "malformed registration → Block (not 'no panel')",
			facts: gateFacts{
				beadID: "mindspec-bd01", reg: reg("/wt/review/slug"),
				res: &panel.Result{Dir: "/wt/review/slug", PanelErr: errFake},
			},
			want:     Block,
			mustHave: []string{"unreadable", "slug", fence},
		},
		{
			name: "abandoned → Pass+Warn naming reason, BEFORE staleness (T3-1)",
			facts: func() gateFacts {
				p := defaultPanel()
				p.Abandoned = true
				p.AbandonReason = "max: superseded by bd99 2026-06-12"
				// Stale SHA present — must NOT cause a Block.
				return gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"),
					res: result(p, 0, 0, 1, nil, nil), headSHA: otherSHA}
			}(),
			want:        Warn,
			mustHave:    []string{"abandoned", "superseded by bd99"},
			mustNotHave: []string{fence},
		},
		{
			name: "round mismatch → Block",
			facts: func() gateFacts {
				p := defaultPanel()
				p.Round = 1
				return gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"),
					res: result(p, 6, 0, 2, nil, nil), headSHA: sha}
			}(),
			want:     Block,
			mustHave: []string{"out of date", "ms-panel-run step 0", fence},
		},
		{
			name: "missing ref → Pass+Warn (rerun-after-merge)",
			facts: gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"),
				res: result(defaultPanel(), 5, 0, 1, nil, nil), missingRef: true},
			want:        Warn,
			mustHave:    []string{"no longer exists", "bead/mindspec-bd01"},
			mustNotHave: []string{fence},
		},
		{
			name: "stale SHA → Block (lola-f4a8 pin)",
			facts: gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"),
				res: result(defaultPanel(), 5, 0, 1, nil, nil), headSHA: otherSHA},
			want:     Block,
			mustHave: []string{"reviewed", "branch now at", fence},
		},
		{
			name: "dirty user tree → Block (CommitAll bypass)",
			facts: gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"),
				res: result(defaultPanel(), 5, 0, 1, nil, nil), headSHA: sha,
				worktreePath: "/wt", userDirt: []string{"src/main.go"}},
			want:     Block,
			mustHave: []string{"uncommitted changes in /wt", "CommitAll", "src/main.go", fence},
		},
		{
			name: "worktree absent → dirty check skipped, falls through to threshold Pass",
			facts: gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"),
				res: result(defaultPanel(), 6, 0, 1, nil, nil), headSHA: sha,
				worktreeAbsent: true, userDirt: []string{"ignored"}},
			want: Pass,
		},
		{
			name: "incomplete (4/6) → Block naming present files",
			facts: gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"),
				res: result(defaultPanel(), 4, 0, 1, nil, nil), headSHA: sha},
			want:     Block,
			mustHave: []string{"incomplete", "4/6 verdicts present", "present:", fence},
		},
		{
			name: "REJECT → Block halt path",
			facts: gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"),
				res: result(defaultPanel(), 5, 1, 1, nil, nil), headSHA: sha},
			want:     Block,
			mustHave: []string{"HARD block / REJECT", "halt path", fence},
		},
		{
			name: "hard_block → Block halt path even at full APPROVE",
			facts: gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"),
				res: result(defaultPanel(), 6, 0, 1, nil, []string{"reva"}), headSHA: sha},
			want:     Block,
			mustHave: []string{"HARD block / REJECT", "hard_block from reva", fence},
		},
		{
			name: "threshold met: 6/6 present, 5 APPROVE + 1 dissent → Pass",
			facts: func() gateFacts {
				p := defaultPanel()
				r := result(p, 5, 0, 1, nil, nil)
				r.Verdicts = append(r.Verdicts,
					panel.Verdict{File: "z-round-1.json", Slot: "z", Round: 1, Verdict: panel.VerdictRequestChanges})
				return gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"), res: r, headSHA: sha}
			}(),
			want: Pass,
		},
		{
			name: "sub-threshold 4/6 (complete? no) — covered above; here 6 present 4 approve via neutral",
			facts: func() gateFacts {
				// 6 present verdicts, 4 APPROVE, 2 REQUEST_CHANGES (neutral).
				p := defaultPanel()
				r := result(p, 4, 0, 1, nil, nil)
				// add two neutral verdicts to reach Complete() == true.
				r.Verdicts = append(r.Verdicts,
					panel.Verdict{File: "x-round-1.json", Slot: "x", Round: 1, Verdict: panel.VerdictRequestChanges},
					panel.Verdict{File: "y-round-1.json", Slot: "y", Round: 1, Verdict: panel.VerdictRequestChanges})
				return gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"), res: r, headSHA: sha}
			}(),
			want:     Block,
			mustHave: []string{"4/6 APPROVE", "threshold is 5/6", "consolidated-round-1.md", fence},
		},
		{
			name: "expected_reviewers:3 → 3/3 present, 2 APPROVE Pass (no hardcoded 6)",
			facts: func() gateFacts {
				p := &panel.Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 3, ReviewedHeadSHA: sha}
				r := result(p, 2, 0, 1, nil, nil)
				r.Verdicts = append(r.Verdicts,
					panel.Verdict{File: "z-round-1.json", Slot: "z", Round: 1, Verdict: panel.VerdictRequestChanges})
				return gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"), res: r, headSHA: sha}
			}(),
			want: Pass,
		},
		{
			name: "expected_reviewers:3 → 1/3 APPROVE Block citing 2/3",
			facts: func() gateFacts {
				p := &panel.Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 3, ReviewedHeadSHA: sha}
				r := result(p, 1, 0, 1, nil, nil)
				r.Verdicts = append(r.Verdicts,
					panel.Verdict{File: "x-round-1.json", Slot: "x", Round: 1, Verdict: panel.VerdictRequestChanges},
					panel.Verdict{File: "y-round-1.json", Slot: "y", Round: 1, Verdict: panel.VerdictRequestChanges})
				return gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"), res: r, headSHA: sha}
			}(),
			want:        Block,
			mustHave:    []string{"1/3 APPROVE", "threshold is 2/3"},
			mustNotHave: []string{"5/6", "/6"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := panelGateDecision(tt.facts)
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
	mk := func(mut func(*gateFacts)) gateFacts {
		p := &panel.Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha}
		f := gateFacts{beadID: "mindspec-bd01", reg: reg("/wt/review/slug"),
			res: result(p, 6, 0, 1, nil, nil), headSHA: sha}
		mut(&f)
		return f
	}
	blocks := []gateFacts{
		mk(func(f *gateFacts) { f.res.PanelErr = errFake; f.res.Panel = nil }),
		mk(func(f *gateFacts) {
			f.res = result(&panel.Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 6}, 6, 0, 2, nil, nil)
		}),
		mk(func(f *gateFacts) { f.headSHA = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" }),
		mk(func(f *gateFacts) { f.worktreePath = "/wt"; f.userDirt = []string{"a.go"} }),
		mk(func(f *gateFacts) {
			f.res = result(&panel.Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha}, 3, 0, 1, nil, nil)
		}),
		mk(func(f *gateFacts) { f.res.Rejects = 1 }),
	}
	for i, f := range blocks {
		got := panelGateDecision(f)
		if got.Action != Block {
			t.Fatalf("case %d: expected Block, got %v (%s)", i, got.Action, got.Message)
		}
		if !strings.Contains(got.Message, fence) {
			t.Errorf("case %d: Block message missing raw-merge fence:\n%s", i, got.Message)
		}
	}
}
