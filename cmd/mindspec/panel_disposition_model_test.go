package main

import (
	"encoding/json"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestPanelDispositionUsesSubmittedModel(t *testing.T) {
	for _, verb := range []string{"refute", "defer", "defer-to-plan"} {
		t.Run(verb, func(t *testing.T) {
			gate := "bead"
			if verb == "defer-to-plan" {
				gate = "spec_approve"
			}
			_, dir := setupPlanDeferral(t, "warn", gate, true)
			paneltest.Seed(t, dir, panel.SubstitutionRecord{Record: panel.RecordSubstitution, Spec: "900-demo", Panel: "p", Slot: "R1", Round: 1, FromFamily: "harness", ToFamily: "replacement", Model: "actual-model", CreatedAt: paneltest.CreatedAt})
			v := paneltest.Verdict(panel.VerdictRequestChanges)
			v.Model = "actual-model"
			v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "missing test", Basis: "traced"}, {Text: "second ask", Basis: "traced"}}
			resolve := func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
				return panel.SubmitResolution{Family: "replacement", Model: "actual-model", RunnerKind: "harness", RunnerSource: "default"}, nil
			}
			if _, err := panel.Submit(panel.SubmitTarget{Dir: dir, Slot: "R1", Round: 1}, paneltest.Bytes(t, v), resolve); err != nil {
				t.Fatal(err)
			}
			var args []string
			switch verb {
			case "refute":
				args = refuteArgs()
			case "defer":
				fakeDeferralBD(t)
				args = deferArgs()
			default:
				args = toPlanArgs()
			}
			if out, err := runPanelVerbCmd(args...); err != nil {
				t.Fatalf("%s: %v %s", verb, err, out)
			}
			lines, err := panel.ReadLedger(dir)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, line := range lines {
				var row panel.DispositionRow
				if err := json.Unmarshal(line, &row); err != nil {
					t.Fatal(err)
				}
				if row.Record == panel.RecordDisposition && row.Reviewer == "R1" {
					found = true
					if row.Model != "actual-model" {
						t.Fatalf("disposition model=%s, want submitted model", row.Model)
					}
				}
			}
			if !found {
				t.Fatal("no disposition row written")
			}
		})
	}
}
