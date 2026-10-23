package panel

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func planDeferralFixture(t *testing.T) (string, Panel, PlanDeferral) {
	t.Helper()
	dir, p := passFixture(t, 2, VerdictApprove)
	p.Gate, p.BeadID, p.Target = "spec_approve", nil, "spec/130"
	writePassJSON(t, dir, "R1-round-1.json", FindingVerdict{Verdict: VerdictRequestChanges, ConcreteChangesRequired: []string{"plan the test"}})
	makeNonLegacy(t, dir, &p)
	v, err := ReadFindingVerdict(dir, "R1", 1)
	if err != nil {
		t.Fatal(err)
	}
	fs, err := Findings(v)
	if err != nil {
		t.Fatal(err)
	}
	f := fs[0]
	d := PlanDeferral{ID: PlanDeferralID(p.Spec, "p", "R1", 1, f), Slot: "R1", Round: 1, Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, By: "operator", CreatedAt: ledgerTime}
	return dir, p, d
}

func TestS133DeferToPlan_IntegrityAndHash(t *testing.T) {
	// The leaf decision has no presence-policy input. Exercise the same facts
	// for each policy; a policy cannot exempt an existing registration.
	for _, policy := range []string{"warn", "enforce"} {
		for _, mutation := range []string{"backed", "hand added", "wrong hash", "wrong round", "wrong slot", "wrong finding"} {
			t.Run(policy+"/"+mutation, func(t *testing.T) {
				dir, p, d := planDeferralFixture(t)
				switch mutation {
				case "wrong hash":
					d.FindingSHA256 = strings.Repeat("a", 64)
				case "wrong round":
					d.Round++
				case "wrong slot":
					d.Slot = "R2"
				case "wrong finding":
					d.Finding = "ccr[9]"
				}
				if mutation != "hand added" {
					createAppend(t, dir, PlanDeferralRecord{Record: RecordDeferredToPlan, Spec: p.Spec, Panel: "p", PlanDeferral: d})
				}
				p.PlanDeferrals = []PlanDeferral{d}
				writePassJSON(t, dir, FileName, p)
				got := passDecision(t, dir)
				if mutation == "backed" {
					if got.Action != Allow {
						t.Fatalf("backed finding refused: %+v", got)
					}
					return
				}
				want := "unresolved"
				if mutation == "hand added" {
					want = "plan_deferrals[0]"
				}
				if got.Action != Block || !strings.Contains(got.Message, want) {
					t.Fatalf("want %s refusal, got %+v", want, got)
				}
			})
		}
	}
}

func TestS133DeferToPlan_NewRoundCarriesOnlyBacked(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			dir, p, d := planDeferralFixture(t)
			createAppend(t, dir, PlanDeferralRecord{Record: RecordDeferredToPlan, Spec: p.Spec, Panel: "p", PlanDeferral: d})
			forged := d
			forged.ID = "manual"
			p.PlanDeferrals = []PlanDeferral{d, forged}
			writePassJSON(t, dir, FileName, p)
			in := CreateInput{Spec: p.Spec, Target: p.Target, Gate: p.Gate, Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest,
				Roster: p.Roster, HasRunner: func(string) bool { return true }}
			if err := Create(dir, in); err != nil {
				t.Fatal(err)
			}
			r := passTally(t, dir)
			if r.IntegrityErr != nil || !reflect.DeepEqual(r.Panel.PlanDeferrals, []PlanDeferral{d}) {
				t.Fatalf("carry mismatch: %+v, %v", r.Panel.PlanDeferrals, r.IntegrityErr)
			}
		})
	}
}

func TestS133DeferToPlan_LedgerAndStableID(t *testing.T) {
	dir, p, d := planDeferralFixture(t)
	raw := mustMarshal(t, PlanDeferralRecord{Record: RecordDeferredToPlan, Spec: p.Spec, Panel: filepath.Base(dir), PlanDeferral: d})
	if err := Validate(raw); err != nil {
		t.Fatal(err)
	}
	if !IsVerbRecord(RecordDeferredToPlan) {
		t.Fatal("record is not reserved to panel verbs")
	}
	for _, value := range []any{"follow-up", "", nil} {
		if err := Validate(changeLedger(t, raw, "bead", value)); err == nil {
			t.Fatalf("bead field accepted: %v", value)
		}
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	if _, exists := object["bead"]; exists {
		t.Fatal("plan record contains bead")
	}
	if d.ID != PlanDeferralID(p.Spec, "p", d.Slot, d.Round, Finding{ID: d.Finding, SHA256: d.FindingSHA256}) {
		t.Fatal("unstable id")
	}
	createAppend(t, dir, object)
	object["created_at"] = ledgerLater
	createAppend(t, dir, object)
	lines, err := ReadLedger(dir)
	if err != nil || len(lines) != 2 {
		t.Fatalf("retry appended a duplicate: %d %v", len(lines), err)
	}
}
