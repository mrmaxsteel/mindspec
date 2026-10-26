package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func setupPlanDeferral(t *testing.T, policy, gate string, v2 ...bool) (string, string) {
	t.Helper()
	root := setupCreateIntegrity(t)
	resetDischargeFlags(t)
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("enforcement:\n  panel_presence: "+policy+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config.ResetCache()
	dir := filepath.Join(root, ".mindspec", "specs", "900-demo", "reviews", "p")
	target := "spec/900-demo"
	if gate == "bead" {
		target = "bead/probe-x.1"
	}
	roster := []panel.RosterSlot{{Slot: "R1", Model: "claude", Lens: "correctness"}, {Slot: "R2", Model: "codex", Lens: "tests"}}
	in := panel.CreateInput{Spec: "900-demo", Target: target, Gate: gate, Round: 1, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:abc", Roster: roster, ExpectedReviewers: 2}
	if len(v2) > 0 && v2[0] {
		in.ReviewedHeadSHA = paneltest.ReviewedSHA
		if err := panel.Create(dir, in); err != nil {
			t.Fatal(err)
		}
		for _, s := range roster {
			v := paneltest.Verdict(panel.VerdictApprove)
			v.Slot, v.Model = s.Slot, s.Model
			if s.Slot == "R1" {
				v.Verdict = panel.VerdictRequestChanges
				v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "missing test", Basis: "traced"}, {Text: "second ask", Basis: "traced"}}
			}
			if _, err := panel.Submit(panel.SubmitTarget{Dir: dir, Slot: s.Slot, Round: 1}, paneltest.Bytes(t, v), paneltest.Resolve); err != nil {
				t.Fatal(err)
			}
		}
		return root, dir
	}
	if err := paneltest.CreateV1(dir, in); err != nil {
		t.Fatal(err)
	}
	writeDischargeVerdict(t, dir, "R1", panel.FindingVerdict{Verdict: panel.VerdictRequestChanges, ConcreteChangesRequired: []string{"missing test", "second ask"}})
	writeDischargeVerdict(t, dir, "R2", panel.FindingVerdict{Verdict: panel.VerdictApprove})
	return root, dir
}

func toPlanArgs() []string {
	return []string{"defer", "p", "--slot", "R1", "--round", "1", "--to-plan", "--finding", "", "--by", "operator"}
}

func TestS133DeferToPlan_WritesAndCoverage(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, one := range []bool{false, true} {
			t.Run(policy+"/one="+map[bool]string{false: "false", true: "true"}[one], func(t *testing.T) {
				root, dir := setupPlanDeferral(t, policy, "spec_approve")
				args := toPlanArgs()
				want := 2
				if one {
					args = append(args, "--finding", "ccr[1]")
					want = 1
				}
				requirePanelVerb(t, args...)
				lines, err := panel.ReadLedger(dir)
				if err != nil {
					t.Fatal(err)
				}
				if len(lines) != 1+2*want {
					t.Fatalf("want paired records per finding: %s", lines)
				}
				seen := map[string]bool{}
				for i := 1; i < len(lines); i += 2 {
					var event panel.PlanDeferralRecord
					var row panel.DispositionRow
					var object map[string]any
					if err := json.Unmarshal(lines[i], &event); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(lines[i], &object); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(lines[i+1], &row); err != nil {
						t.Fatal(err)
					}
					if _, exists := object["bead"]; exists {
						t.Fatal("plan record has bead field")
					}
					if event.Record != "deferred_to_plan" || event.ID == "" || event.Slot != "R1" || event.Round != 1 || event.By != "operator" || event.CreatedAt == "" || event.FindingText == "" || event.FindingSHA256 == "" {
						t.Fatalf("bad event: %+v", event)
					}
					if row.Disposition != "confirmed-deferred" || row.EvidenceRef != "plan:"+event.ID || row.Reviewer != "R1" || row.Round == nil || *row.Round != 1 || row.Severity != "blocking" || row.Model != "claude" {
						t.Fatalf("bad row: %+v", row)
					}
					seen[event.Finding] = true
				}
				if len(seen) != want || !seen["ccr[1]"] {
					t.Fatalf("wrong covered findings: %v", seen)
				}
				before := panelTreeBytes(t, root)
				requirePanelVerb(t, args...)
				if !reflect.DeepEqual(before, panelTreeBytes(t, root)) {
					t.Fatal("retry changed records")
				}
				res, err := panel.Tally(dir)
				if err != nil {
					t.Fatal(err)
				}
				if len(res.Panel.Deferrals) != 0 || len(res.Panel.PlanDeferrals) != want || res.IntegrityErr != nil {
					t.Fatalf("wrong snapshot: %+v", res)
				}
				if one {
					if res.PassRule(true) {
						t.Fatal("one deferral covered another finding")
					}
					requirePanelVerb(t, append(toPlanArgs(), "--finding", "ccr[0]")...)
					res, err = panel.Tally(dir)
					if err != nil {
						t.Fatal(err)
					}
				}
				decision := panel.PanelGateDecision(panel.GateFacts{Reg: &panel.Registration{Dir: dir, Panel: *res.Panel}, Res: res, HeadSHA: "abc123", WorktreeAbsent: true})
				if decision.Action != panel.Allow {
					t.Fatalf("plan deferral did not pass: %+v", decision)
				}
				if err := panel.WriteCoverageManifest(res); err != nil {
					t.Fatal(err)
				}
				if out, err := runPanelVerbCmd("disposition", "check", "--spec", "900-demo", "--panel", "p"); err != nil {
					t.Fatalf("coverage check: %v\n%s", err, out)
				}
				// Repair a snapshot lost after the durable append.
				res.Panel.PlanDeferrals = nil
				if err := panel.WriteSnapshot(dir, *res.Panel); err != nil {
					t.Fatal(err)
				}
				requirePanelVerb(t, toPlanArgs()...)
				res, err = panel.Tally(dir)
				if err != nil || !res.PassRule(true) || len(res.Panel.PlanDeferrals) != 2 {
					t.Fatalf("retry did not repair snapshot: %+v %v", res, err)
				}
			})
		}
	}
}

func TestS133DeferToPlan_Refusals(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, name := range []string{"bead", "plan_approve", "final_review", "adhoc", "with bead", "without to-plan", "old round", "unknown finding", "empty actor"} {
			t.Run(policy+"/"+name, func(t *testing.T) {
				gate := "spec_approve"
				if name == "bead" || name == "plan_approve" || name == "final_review" || name == "adhoc" {
					gate = name
				}
				root, _ := setupPlanDeferral(t, policy, gate)
				args := toPlanArgs()
				want := "--to-plan requires a spec_approve panel"
				switch name {
				case "with bead":
					args = append(args, "--bead", "follow-up")
					want = "mutually exclusive"
				case "without to-plan":
					args = deferArgs()
					want = "this panel has no owning epic for a deferral\nrecovery: fix the finding or use mindspec panel refute"
				case "old round":
					args = replaceRefuteArg(args, "--round", "2")
					want = "latest verdict round"
				case "unknown finding":
					args = append(args, "--finding", "ccr[9]")
					want = "unknown finding"
				case "empty actor":
					args = replaceRefuteArg(args, "--by", " ")
					want = "--by must not be empty"
				}
				msg := assertPanelRefusal(t, root, args...)
				if !strings.Contains(msg, want) {
					t.Fatalf("wrong refusal: %s", msg)
				}
			})
		}
	}
}

func TestS133DeferToPlan_ProductSurface(t *testing.T) {
	forbidden := regexp.MustCompile(`(?i)\bADR[- ]?[0-9]+|\bspec[- #]+[0-9]+|\bR[0-9]+(?:\.[0-9]+)?\b|\bAC[- ]?[0-9]+|\bbead[- #]+[0-9]+|\bissue[- #]+[0-9]+|mindspec-[a-z0-9]+\.[0-9]+|#[0-9]+`)
	for _, canary := range []string{"ADR-0043", "spec 133", "R6.3", "AC21", "Bead 5", "issue 212", "mindspec-qjy8.5", "#212"} {
		if !forbidden.MatchString(canary) {
			t.Fatalf("surface detector missed %s", canary)
		}
	}
	for _, name := range []string{"panel_defer.go", "panel_defer_to_plan.go", "../../internal/panel/plan_deferral.go", "../../internal/panel/ledger.go"} {
		tree, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(tree, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			if forbidden.MatchString(value) {
				t.Errorf("internal reference in %s: %s", name, value)
			}
			return true
		})
	}
}

func TestS133DeferToPlan_CountedSubmission(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			_, dir := setupPlanDeferral(t, policy, "spec_approve", true)
			requirePanelVerb(t, toPlanArgs()...)
			res, err := panel.Tally(dir)
			if err != nil {
				t.Fatal(err)
			}
			if res.VerdictSchema != 2 || len(res.Panel.PlanDeferrals) != 2 || !res.PassRule(true) {
				t.Fatalf("counted submission not discharged: %+v", res)
			}
			if err := panel.WriteCoverageManifest(res); err != nil {
				t.Fatal(err)
			}
			if out, err := runPanelVerbCmd("disposition", "check", "--spec", "900-demo", "--panel", "p"); err != nil {
				t.Fatalf("coverage check: %v\n%s", err, out)
			}
		})
	}
}

func TestS133DeferToPlan_AtomicBatch(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			root, dir := setupPlanDeferral(t, policy, "spec_approve")
			v, err := panel.ReadSubmittedFindingVerdict(dir, "R1", 1)
			if err != nil {
				t.Fatal(err)
			}
			fs, err := panel.Findings(v)
			if err != nil {
				t.Fatal(err)
			}
			// A conflicting second finding must leave the first finding unwritten.
			f := fs[1]
			d := panel.PlanDeferral{ID: panel.PlanDeferralID("900-demo", "p", "R1", 1, f), Slot: "R1", Round: 1,
				Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, By: "earlier", CreatedAt: paneltest.CreatedAt}
			raw, err := json.Marshal(panel.PlanDeferralRecord{Record: panel.RecordDeferredToPlan, Spec: "900-demo", Panel: "p", PlanDeferral: d})
			if err != nil {
				t.Fatal(err)
			}
			if err := panel.AppendRecords(dir, raw); err != nil {
				t.Fatal(err)
			}
			msg := assertPanelRefusal(t, root, toPlanArgs()...)
			if !strings.Contains(msg, "already exists with different content") {
				t.Fatalf("wrong refusal: %s", msg)
			}
			lines, err := panel.ReadLedger(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range lines {
				var event panel.PlanDeferralRecord
				if err := json.Unmarshal(line, &event); err != nil {
					t.Fatal(err)
				}
				if event.Record == panel.RecordDeferredToPlan && event.Slot == "R1" && event.Finding == fs[0].ID {
					t.Fatalf("refused batch wrote the first finding: %s", line)
				}
			}
		})
	}
}
