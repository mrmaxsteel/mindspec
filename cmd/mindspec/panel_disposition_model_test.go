package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestPanelDispositionAppendUsesSubmittedModel(t *testing.T) {
	for _, supplied := range []string{"old-model", "", "actual-model", "omitted"} {
		t.Run(supplied, func(t *testing.T) {
			_, dir := setupPlanDeferral(t, "warn", "bead", true)
			res, err := panel.Tally(dir)
			if err != nil {
				t.Fatal(err)
			}
			oldModel := res.Panel.Roster[0].Model
			paneltest.Seed(t, dir, panel.SubstitutionRecord{Record: panel.RecordSubstitution, Spec: "900-demo", Panel: "p", Slot: "R1", Round: 1, FromFamily: "harness", ToFamily: "replacement", Model: "actual-model", CreatedAt: paneltest.CreatedAt})
			v := paneltest.Verdict(panel.VerdictApprove)
			v.Model = "actual-model"
			resolve := func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
				return panel.SubmitResolution{Family: "replacement", Model: "actual-model", RunnerKind: "harness", RunnerSource: "default"}, nil
			}
			if _, err := panel.Submit(panel.SubmitTarget{Dir: dir, Slot: "R1", Round: 1}, paneltest.Bytes(t, v), resolve); err != nil {
				t.Fatal(err)
			}
			row := map[string]any{"reviewer": "R1", "severity": "minor", "summary": "test finding", "disposition": "confirmed-fixed"}
			if supplied != "omitted" {
				row["model"] = supplied
			}
			if supplied == "old-model" {
				row["model"] = oldModel
			}
			raw, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "dispositions.jsonl")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			err = runDispositionAppend(t, "900-demo", "p", string(raw))
			if supplied == "old-model" {
				if err == nil || !strings.Contains(err.Error(), "disposition model differs") || !strings.Contains(err.Error(), "set model to actual-model") {
					t.Fatalf("old roster model must be refused with submitted model recovery: %v", err)
				}
				after, readErr := os.ReadFile(path)
				if readErr != nil || !bytes.Equal(before, after) {
					t.Fatalf("refused model changed ledger: %v", readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			lines, err := panel.ReadLedger(dir)
			if err != nil {
				t.Fatal(err)
			}
			var saved panel.DispositionRow
			if err := json.Unmarshal(lines[len(lines)-1], &saved); err != nil {
				t.Fatal(err)
			}
			if saved.Record != panel.RecordDisposition || saved.Model != "actual-model" {
				t.Fatalf("append must record submitted model: %+v", saved)
			}
		})
	}
}
