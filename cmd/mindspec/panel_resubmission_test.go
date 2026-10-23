package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestPanelDefer_ResubmittedFindingCanBeDeferredAgain(t *testing.T) {
	for _, first := range []string{panel.VerdictApprove, panel.VerdictRequestChanges} {
		t.Run(first, func(t *testing.T) {
			f := newCLIV2(t)
			fake := fakeDeferralBD(t)
			rebindWrite(t, fake, "epics.json", `[{"id":"epic-x","metadata":{"spec_num":900,"spec_title":"test"}}]`)
			rebindWrite(t, fake, "follow-up.json", `[{"id":"follow-up","status":"open","description":"same finding","dependencies":[{"id":"epic-x","dependency_type":"discovered-from"}]}]`)
			v := f.verdict()
			v.Verdict = first
			if first == panel.VerdictApprove {
				v.Findings = []string{"same finding"}
			} else {
				v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "same finding", Basis: "traced"}}
			}
			f.submit(t, v)
			other := f.verdict()
			other.Slot = "R2"
			f.submit(t, other)
			args := append(deferArgs(), "--spec", "900-test", "--finding", "ccr[0]")
			requirePanelVerb(t, args...)
			v.Verdict = panel.VerdictRequestChanges
			v.Findings = nil
			v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "same finding", Basis: "traced"}}
			v.Rationale += " resubmission"
			f.submit(t, v)
			before, err := panel.Tally(f.dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(before.EffectiveDeferrals()) != 0 {
				t.Fatal("old submission deferral still effective")
			}
			out, err := runPanelVerbCmd(args...)
			if err != nil {
				t.Fatalf("fresh explicit deferral of resubmitted finding refused: %v %s", err, out)
			}
			res, err := panel.Tally(f.dir)
			if err != nil || len(res.EffectiveDeferrals()) != 1 {
				t.Fatalf("effective deferral missing: %v", err)
			}
			if len(res.Panel.Deferrals) != 2 || res.EffectiveDeferrals()[0].SourceFileSHA256 == before.Panel.Deferrals[0].SourceFileSHA256 {
				t.Fatal("fresh deferral did not retain distinct submission identity")
			}
			// The new coverage row must discharge the new event at lifecycle gates.
			requirePanelVerb(t, "tally", "p", "--spec", "900-test")
			if err := panel.CheckCore3(panel.Registration{Dir: f.dir, Panel: *res.Panel}); err != nil {
				t.Fatalf("fresh submission's coverage must pass: %v", err)
			}
			lines, err := panel.ReadLedger(f.dir)
			if err != nil {
				t.Fatal(err)
			}
			rows := map[string]bool{}
			for _, line := range lines {
				var row panel.DispositionRow
				if err := json.Unmarshal(line, &row); err != nil {
					t.Fatal(err)
				}
				if row.Record != panel.RecordDisposition {
					continue
				}
				linked := false
				for _, d := range res.Panel.Deferrals {
					if strings.Contains(row.ID, d.SourceFileSHA256) && strings.Contains(row.ID, d.SourceVerdict) && row.CreatedAt == d.CreatedAt {
						wantSeverity := "blocking"
						if d.SourceVerdict == panel.VerdictApprove {
							wantSeverity = "minor"
						}
						if row.Severity != wantSeverity {
							t.Fatalf("source row severity changed: %+v", row)
						}
						linked = true
					}
				}
				if !linked || rows[row.ID] {
					t.Fatalf("coverage row lost event identity: %+v", row)
				}
				rows[row.ID] = true
			}
			if len(rows) != 2 {
				t.Fatalf("got %d coverage rows, want one per submission", len(rows))
			}
			requirePanelVerb(t, args...)
			retry, err := panel.ReadLedger(f.dir)
			if err != nil || len(retry) != len(lines) {
				t.Fatalf("exact retry must be idempotent: before=%d after=%d err=%v", len(lines), len(retry), err)
			}
		})
	}
}
