package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// fixStepMarkers are the fix steps of the unresolved route. No other route
// prints one.
var fixStepMarkers = []string{"--fixer", "mindspec rules --scope fixer", "mindspec panel verify"}

// TestPanelTally_NextBlockRoutes drives the real gate decision for each leg
// and checks the Next block that a non-kernel tally prints for it.
func TestPanelTally_NextBlockRoutes(t *testing.T) {
	const sha = "1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa"
	const otherSHA = "2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb"
	const beadID = "mindspec-nb.1"
	reg := &panel.Registration{Dir: "/wt/review/demo"}
	newPanel := func(gate string) *panel.Panel {
		p := &panel.Panel{Spec: "900-demo", Gate: gate, Round: 1, ExpectedReviewers: 3, ReviewedHeadSHA: sha, Target: "spec/900-demo"}
		if gate == "bead" {
			p.BeadID = ptrStr(beadID)
			p.Target = "bead/" + beadID
		}
		if gate == "adhoc" {
			p.Spec = ""
			p.Target = "main"
		}
		return p
	}
	type row struct {
		name      string
		res       func(p *panel.Panel) *panel.Result
		head      string
		artifact  string
		leg       panel.DecisionLeg
		action    panel.GateAction
		want      []string
		forbidden []string
		fix       bool
	}
	operatorWant := []string{panelOperatorActionLine, panelHaltLine}
	rows := []row{
		{name: "incomplete", res: func(p *panel.Panel) *panel.Result { return buildResult(p, 2, 0, 2, 1, nil) }, head: sha,
			leg: panel.LegIncomplete, action: panel.Block,
			want:      []string{"mindspec panel checkout 'demo' --slot 'R3' --dispatch", "mindspec panel prompt 'demo' --slot 'R3'", "then: mindspec panel tally 'demo'"},
			forbidden: []string{"--slot 'R1'", panelOperatorActionLine}},
		{name: "reject", res: func(p *panel.Panel) *panel.Result { return buildResult(p, 2, 1, 3, 1, nil) }, head: sha,
			leg: panel.LegOperator, action: panel.Block, want: operatorWant, forbidden: []string{"mindspec panel refute"}},
		{name: "hard_block", res: func(p *panel.Panel) *panel.Result { return buildResult(p, 3, 0, 3, 1, []string{"slot0"}) }, head: sha,
			leg: panel.LegOperator, action: panel.Block, want: operatorWant},
		{name: "missing artifact", res: func(p *panel.Panel) *panel.Result { return buildResult(p, 2, 0, 3, 1, nil) }, head: sha,
			artifact: " (artifact bench/out.json absent)", leg: panel.LegUnresolved, action: panel.Block,
			want: append([]string{"mindspec panel refute 'demo' --slot <slot> --round 1"}, operatorWant...)},
		{name: "unresolved", res: func(p *panel.Panel) *panel.Result { return buildResult(p, 2, 0, 3, 1, nil) }, head: sha,
			leg: panel.LegUnresolved, action: panel.Block, fix: true,
			want:      []string{"mindspec panel refute 'demo' --slot <slot> --round 1", "--round 2 --brief-file <absolute path>"},
			forbidden: []string{panelOperatorActionLine, panelHaltLine}},
		{name: "stale", res: func(p *panel.Panel) *panel.Result { return buildResult(p, 3, 0, 3, 1, nil) }, head: otherSHA,
			leg: panel.LegStale, action: panel.Block,
			want:      []string{"when the subject needs a new round: mindspec panel create 'demo'", "for a passing panel whose changed paths need no new review: mindspec panel rebind 'demo'", "by the operator only: mindspec panel abandon 'demo'"},
			forbidden: []string{panelHaltLine, panelOperatorActionLine}},
		{name: "round mismatch", res: func(p *panel.Panel) *panel.Result { return buildResult(p, 3, 0, 3, 2, nil) }, head: sha,
			leg: panel.LegRoundMismatch, action: panel.Block, want: []string{panelHaltLine, "mindspec panel abandon 'demo'"}, forbidden: []string{panelOperatorActionLine}},
		{name: "abandoned warn", res: func(p *panel.Panel) *panel.Result {
			p.Abandoned = true
			p.AbandonReason = "operator: superseded"
			return buildResult(p, 3, 0, 3, 1, nil)
		}, head: sha, leg: panel.LegAbandoned, action: panel.Warn, want: []string{panelHaltLine, "mindspec panel rebind 'demo'"}},
	}
	for _, gate := range []string{"bead", "spec_approve", "plan_approve", "final_review", "adhoc"} {
		for _, tc := range rows {
			t.Run(gate+"/"+tc.name, func(t *testing.T) {
				p := newPanel(gate)
				facts := panel.GateFacts{Reg: reg, Res: tc.res(p), HeadSHA: tc.head, ArtifactDetails: tc.artifact}
				if gate == "bead" {
					facts.BeadID = beadID
				}
				d := panel.PanelGateDecision(facts)
				if d.Action != tc.action || d.Leg != tc.leg {
					t.Fatalf("decision = %v/%q, want %v/%q (%s)", d.Action, d.Leg, tc.action, tc.leg, d.Message)
				}
				var warn, out bytes.Buffer
				origWarn := tallyWarnOut
				tallyWarnOut = &warn
				t.Cleanup(func() { tallyWarnOut = origWarn })
				f := tallyRecoveryFacts{Slug: "demo", SpecID: p.Spec, Target: p.Target, Gate: gate, NextRound: 2,
					MissingSlots: []string{"R3"}, ArtifactMissing: tc.artifact != "", Out: &out}
				var err error
				if gate == "bead" {
					f.BeadID = beadID
					err = tallyExitAction(d, f)
				} else {
					err = tallyExitActionNonBead(d, f)
				}
				var text string
				if tc.action == panel.Block {
					if err == nil {
						t.Fatal("blocked decision exited 0")
					}
					text = err.Error()
					lines := strings.Split(text, "\n")
					if strings.Count(text, "\nrecovery:") != 1 || !strings.HasPrefix(lines[len(lines)-1], "recovery: mindspec panel tally 'demo'") {
						t.Fatalf("want one final tally recovery line: %s", text)
					}
				} else {
					if err != nil {
						t.Fatalf("warn exited non-zero: %v", err)
					}
					text = warn.String()
				}
				if !strings.Contains(text, "Next:") {
					t.Fatalf("no Next block: %s", text)
				}
				for _, w := range tc.want {
					if !strings.Contains(text, w) {
						t.Errorf("missing %q:\n%s", w, text)
					}
				}
				for _, w := range tc.forbidden {
					if strings.Contains(text, w) {
						t.Errorf("unexpected %q:\n%s", w, text)
					}
				}
				for _, m := range fixStepMarkers {
					if strings.Contains(text, m) != tc.fix {
						t.Errorf("fix step %q present=%v, want %v:\n%s", m, !tc.fix, tc.fix, text)
					}
				}
				if tc.fix {
					checkStep := "mindspec complete " + beadID + " --check"
					if strings.Contains(text, checkStep) != (gate == "bead") {
						t.Errorf("complete --check on gate %s:\n%s", gate, text)
					}
				}
				// Gate flags of the next round: no --spec on adhoc, the bead binding on a bead gate.
				// (disposition append takes --spec on every gate, so check only the create line.)
				if i := strings.Index(text, "mindspec panel create"); i >= 0 {
					create := text[i:]
					create = create[:strings.Index(create, "\n")]
					if (gate == "adhoc") == strings.Contains(create, "--spec") {
						t.Errorf("--spec on gate %s: %s", gate, create)
					}
					if (gate == "bead") != strings.Contains(create, "--bead "+beadID) {
						t.Errorf("bead binding on gate %s: %s", gate, create)
					}
				}
				if strings.Contains(text, "mindspec panel defer") {
					switch gate {
					case "adhoc":
						t.Errorf("defer on adhoc:\n%s", text)
					case "spec_approve":
						if !strings.Contains(text, "--to-plan") {
							t.Errorf("spec_approve defer without --to-plan:\n%s", text)
						}
					}
				} else if tc.leg == panel.LegUnresolved && gate != "adhoc" {
					t.Errorf("no defer line on gate %s:\n%s", gate, text)
				}
				if out.Len() != 0 {
					t.Errorf("a non-passing tally wrote to stdout: %s", out.String())
				}
			})
		}
	}
}

// TestPanelTally_NextBlockPassAndHalted covers the PASS verb of each gate and
// the halted route of a Block with an unset leg.
func TestPanelTally_NextBlockPassAndHalted(t *testing.T) {
	for gate, verb := range map[string]string{
		"bead":         "mindspec complete mindspec-nb.1",
		"spec_approve": "mindspec spec approve '900-demo'",
		"plan_approve": "mindspec plan approve '900-demo'",
		"final_review": "mindspec impl approve '900-demo'",
		"adhoc":        "",
	} {
		t.Run(gate, func(t *testing.T) {
			var out bytes.Buffer
			f := tallyRecoveryFacts{Slug: "demo", SpecID: "900-demo", Target: "main", Gate: gate, NextRound: 2, Out: &out}
			pass := panel.Decision{Action: panel.Allow, Leg: panel.LegPass}
			var err error
			if gate == "bead" {
				f.BeadID = "mindspec-nb.1"
				err = tallyExitAction(pass, f)
			} else {
				err = tallyExitActionNonBead(pass, f)
			}
			if err != nil {
				t.Fatalf("pass exited non-zero: %v", err)
			}
			if verb == "" {
				if out.Len() != 0 {
					t.Fatalf("adhoc pass printed a Next block: %s", out.String())
				}
			} else if got := out.String(); got != "Next:\n  "+verb+"\n" {
				t.Fatalf("pass Next block = %q, want verb %q", got, verb)
			}
			halted := panel.Decision{Action: panel.Block, Message: "blocked"}
			f.Out = nil
			if gate == "bead" {
				err = tallyExitAction(halted, f)
			} else {
				err = tallyExitActionNonBead(halted, f)
			}
			if err == nil || !strings.Contains(err.Error(), panelHaltLine) || !guard.HasFinalRecoveryLine(err.Error()) {
				t.Fatalf("unset leg did not halt: %v", err)
			}
			for _, m := range fixStepMarkers {
				if strings.Contains(err.Error(), m) {
					t.Fatalf("halted route printed fix step %q: %v", m, err)
				}
			}
		})
	}
}

// TestPanelTally_KernelGatePrintsNoNextBlock: on a kernel gate next-action
// decides, so no action prints a Next block.
func TestPanelTally_KernelGatePrintsNoNextBlock(t *testing.T) {
	for _, d := range []panel.Decision{
		{Action: panel.Allow, Leg: panel.LegPass},
		{Action: panel.Warn, Leg: panel.LegAbandoned, Message: "abandoned"},
		{Action: panel.Block, Leg: panel.LegUnresolved, Message: "unresolved"},
		{Action: panel.Block, Leg: panel.LegIncomplete, Message: "incomplete"},
	} {
		var warn, out bytes.Buffer
		origWarn := tallyWarnOut
		tallyWarnOut = &warn
		err := tallyExitAction(d, tallyRecoveryFacts{Kernel: true, SpecID: "900-demo", Slug: "demo", BeadID: "mindspec-nb.1", Gate: "bead", Out: &out})
		tallyWarnOut = origWarn
		text := warn.String() + out.String()
		if err != nil {
			text += err.Error()
			if !strings.HasSuffix(err.Error(), "recovery: mindspec next-action '900-demo' --apply") {
				t.Errorf("%s: kernel Block recovery: %v", d.Leg, err)
			}
		} else if d.Action == panel.Block {
			t.Errorf("%s: kernel Block exited 0", d.Leg)
		}
		if strings.Contains(text, "Next:") {
			t.Errorf("%s: kernel gate printed a Next block: %s", d.Leg, text)
		}
	}
}
