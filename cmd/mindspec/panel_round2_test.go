package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestPanelDefer_ApproveResubmittedRequestChanges(t *testing.T) {
	for _, finding := range []string{"ccr[0]", ""} {
		t.Run("finding="+finding, func(t *testing.T) {
			f := newCLIV2(t)
			fake := fakeDeferralBD(t)
			rebindWrite(t, fake, "epics.json", `[{"id":"epic-x","metadata":{"spec_num":900,"spec_title":"test"}}]`)
			rebindWrite(t, fake, "follow-up.json", `[{"id":"follow-up","status":"open","description":"minor nit","dependencies":[{"id":"epic-x","dependency_type":"discovered-from"}]}]`)
			v := f.verdict()
			v.Findings = []string{"minor nit"}
			requirePanelVerb(t, f.args(f.file(t, v))...)
			// Supply the other approval so the resubmitted finding alone blocks.
			other := f.verdict()
			other.Slot = "R2"
			f.submit(t, other)
			args := append(deferArgs(), "--spec", "900-test")
			if finding != "" {
				args = append(args, "--finding", finding)
			}
			requirePanelVerb(t, args...)
			v.Verdict = panel.VerdictRequestChanges
			v.Findings = nil
			v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "minor nit", Basis: "traced"}}
			requirePanelVerb(t, f.args(f.file(t, v))...)
			out, tallyErr := runPanelVerbCmd("tally", "p", "--spec", "900-test")
			res, err := panel.Tally(f.dir)
			if err != nil {
				t.Fatal(err)
			}
			if tallyErr == nil || len(res.EffectiveDeferrals()) != 0 || res.ResolvedCount() != 0 || res.PassRule(true) || !strings.Contains(out, "1 unresolved (R1 (ccr[0]))") {
				t.Fatalf("APPROVE deferral cleared a resubmitted blocking finding: %s", out)
			}
			if len(res.Panel.Deferrals) != 1 || len(res.DeferralTexts(res.Panel.Deferrals[0])) != 0 {
				t.Fatal("audit-only deferral became a completion obligation")
			}
			manifest, err := json.Marshal(panel.DeriveCoverageManifest(res))
			if err != nil {
				t.Fatal(err)
			}
			if err := panel.AppendRecords(f.dir, manifest); err != nil {
				t.Fatal(err)
			}
			if err := panel.CheckCompletenessAt(f.dir, "p"); err == nil {
				t.Fatal("minor deferral covered the resubmitted blocking verdict")
			}
			lines, err := panel.ReadLedger(f.dir)
			if err != nil {
				t.Fatal(err)
			}
			sawMinor := false
			for _, line := range lines {
				var row panel.DispositionRow
				if err := json.Unmarshal(line, &row); err != nil {
					t.Fatal(err)
				}
				if row.Record == panel.RecordDisposition && row.Disposition == "confirmed-deferred" {
					if row.Severity != "minor" {
						t.Fatalf("audit row changed: %+v", row)
					}
					sawMinor = true
				}
			}
			if !sawMinor {
				t.Fatal("missing minor audit row")
			}
		})
	}
}

func TestPanelDefer_RequestChangesSubmissionBinding(t *testing.T) {
	f := newCLIV2(t)
	fake := fakeDeferralBD(t)
	rebindWrite(t, fake, "epics.json", `[{"id":"epic-x","metadata":{"spec_num":900,"spec_title":"test"}}]`)
	rebindWrite(t, fake, "follow-up.json", `[{"id":"follow-up","status":"open","description":"blocking finding","dependencies":[{"id":"epic-x","dependency_type":"discovered-from"}]}]`)
	v := f.verdict()
	v.Verdict = panel.VerdictRequestChanges
	v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "blocking finding", Basis: "traced"}}
	submitted := f.submit(t, v)
	other := f.verdict()
	other.Slot = "R2"
	f.submit(t, other)
	requirePanelVerb(t, append(deferArgs(), "--spec", "900-test", "--finding", "ccr[0]")...)
	res, err := panel.Tally(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.EffectiveDeferrals()) != 1 {
		t.Fatal("current REQUEST_CHANGES deferral did not apply")
	}
	d := res.EffectiveDeferrals()[0]
	if d.SourceVerdict != panel.VerdictRequestChanges || d.SourceFileSHA256 != submitted.FileSHA256 {
		t.Fatalf("missing source binding: %+v", d)
	}
	manifest, err := json.Marshal(panel.DeriveCoverageManifest(res))
	if err != nil {
		t.Fatal(err)
	}
	if err := panel.AppendRecords(f.dir, manifest); err != nil {
		t.Fatal(err)
	}
	if err := panel.CheckCompletenessAt(f.dir, "p"); err != nil {
		t.Fatalf("current deferral must cover its slot: %v", err)
	}
	v.Rationale += " new submission"
	f.submit(t, v)
	res, err = panel.Tally(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.EffectiveDeferrals()) != 0 || res.ResolvedCount() != 0 || res.UnresolvedNames() != "R1 (ccr[0])" {
		t.Fatalf("superseded submission still deferred: %s", panel.RuleLine(res))
	}
	// Currency belongs to tally/gate; the durable floor does not read verdicts.
	if err := panel.CheckCompletenessAt(f.dir, "p"); err != nil {
		t.Fatalf("ledger-only floor must retain recorded coverage: %v", err)
	}
	if err := panel.CheckCore3(panel.Registration{Dir: f.dir, Panel: *res.Panel}); err == nil || !strings.Contains(err.Error(), "no covering disposition row") {
		t.Fatalf("lifecycle coverage check must reject superseded deferral: %v", err)
	}
}
