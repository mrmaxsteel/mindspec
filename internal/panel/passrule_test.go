package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func passFixture(t *testing.T, n int, verdict string) (string, Panel) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "p")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	p := Panel{Spec: "130", Target: "bead/mindspec-test.1", BeadID: ptr("mindspec-test.1"), Gate: "bead", Round: 1, ExpectedReviewers: n, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:baseline"}
	for i := 1; i <= n; i++ {
		slot := fmt.Sprintf("R%d", i)
		p.Roster = append(p.Roster, RosterSlot{slot, "model", "correctness"})
		writePassJSON(t, dir, slot+"-round-1.json", FindingVerdict{Verdict: verdict, ConcreteChangesRequired: []string{"first", "second", "third"}})
	}
	writePassJSON(t, dir, FileName, p)
	return dir, p
}
func writePassJSON(t *testing.T, dir, name string, v interface{}) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
		t.Fatal(err)
	}
}
func passTally(t *testing.T, dir string) *Result {
	t.Helper()
	r, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func passDecision(t *testing.T, dir string) Decision {
	t.Helper()
	r := passTally(t, dir)
	return PanelGateDecision(GateFacts{Reg: &Registration{Dir: dir, Panel: *r.Panel}, Res: r, HeadSHA: "abc123", WorktreeAbsent: true})
}
func pinSlot(t *testing.T, dir string, p *Panel, slot string) {
	t.Helper()
	v, err := ReadFindingVerdict(dir, slot, p.Round)
	if err != nil {
		t.Fatal(err)
	}
	f, err := SlotFinding(v)
	if err != nil {
		t.Fatal(err)
	}
	p.Refutations = append(p.Refutations, Refutation{Slot: slot, Round: p.Round, Finding: f.ID, FindingSHA256: f.SHA256})
	writePassJSON(t, dir, FileName, p)
}
func TestPassRule_ZeroApprovePasses(t *testing.T) {
	dir, p := passFixture(t, 6, VerdictRequestChanges)
	for i := 1; i <= 6; i++ {
		pinSlot(t, dir, &p, fmt.Sprintf("R%d", i))
	}
	makeNonLegacy(t, dir, &p)
	r := passTally(t, dir)
	if d := passDecision(t, dir); d.Action != Allow {
		t.Fatalf("all discharged must pass: %+v", d)
	}
	if !strings.Contains(RuleLine(r), "0 APPROVE, 6 resolved") {
		t.Fatal(RuleLine(r))
	}
	if v, _ := r.VoteDecision(); v != VotePass {
		t.Fatal("vote rule disagrees")
	}
}
func TestPassRule_EachTermBlocks(t *testing.T) {
	for _, term := range []string{"complete", "veto", "fresh", "discharged", "intact"} {
		t.Run(term, func(t *testing.T) {
			dir, p := passFixture(t, 2, VerdictApprove)
			r := passTally(t, dir)
			facts := GateFacts{Reg: &Registration{Dir: dir, Panel: p}, Res: r, HeadSHA: p.ReviewedHeadSHA, WorktreeAbsent: true}
			switch term {
			case "complete":
				r.Verdicts = r.Verdicts[:1]
			case "veto":
				r.Rejects = 1
				r.Verdicts[0].Verdict = VerdictReject
			case "fresh":
				facts.HeadSHA = "def456"
				facts.SubjectDigest = "different"
			case "discharged":
				r.Verdicts[0].Verdict = VerdictRequestChanges
				r.Approves--
			case "intact":
				r.IntegrityErr = fmt.Errorf("roster")
			}
			if d := PanelGateDecision(facts); d.Action != Block {
				t.Fatalf("%s alone must block: %+v", term, d)
			}
			if r.PassRule(term != "fresh") {
				t.Fatalf("%s pass rule accepted", term)
			}
		})
	}
}
func TestPassRule_NeverDischarged(t *testing.T) {
	for _, kind := range []string{VerdictReject, "hard_block", "INVALID"} {
		t.Run(kind, func(t *testing.T) {
			dir, p := passFixture(t, 2, VerdictRequestChanges)
			pinSlot(t, dir, &p, "R1")
			pinSlot(t, dir, &p, "R2")
			v := FindingVerdict{Verdict: kind, ConcreteChangesRequired: []string{"first", "second", "third"}}
			if kind == "hard_block" {
				v.Verdict = VerdictRequestChanges
				v.HardBlock = true
			}
			writePassJSON(t, dir, "R1-round-1.json", v)
			if d := passDecision(t, dir); d.Action != Block {
				t.Fatalf("veto discharged: %+v", d)
			}
			for _, ref := range passTally(t, dir).AppliedRefutations() {
				if ref.Slot == "R1" {
					t.Fatal("veto refutation reported effective")
				}
			}
		})
	}
}
func TestPassRule_EmptyRCInvalid(t *testing.T) {
	dir, _ := passFixture(t, 2, VerdictApprove)
	writePassJSON(t, dir, "R1-round-1.json", FindingVerdict{Verdict: VerdictRequestChanges})
	r := passTally(t, dir)
	if r.InvalidCount() != 1 || passDecision(t, dir).Action != Block {
		t.Fatalf("empty RC: %+v", r)
	}
}
func TestPassRule_RoundPinned(t *testing.T) {
	dir, p := passFixture(t, 2, VerdictRequestChanges)
	pinSlot(t, dir, &p, "R1")
	pinSlot(t, dir, &p, "R2")
	p.Round = 2
	writePassJSON(t, dir, FileName, p)
	for _, slot := range []string{"R1", "R2"} {
		writePassJSON(t, dir, slot+"-round-2.json", FindingVerdict{Verdict: VerdictRequestChanges, ConcreteChangesRequired: []string{"first", "second", "third"}})
	}
	if d := passDecision(t, dir); d.Action != Block {
		t.Fatalf("old round discharged: %+v", d)
	}
}
func TestPassRule_FindingTextPinned(t *testing.T) {
	dir, p := passFixture(t, 2, VerdictApprove)
	v := FindingVerdict{Verdict: VerdictRequestChanges, ConcreteChangesRequired: []string{"first"}}
	writePassJSON(t, dir, "R1-round-1.json", v)
	fs, _ := Findings(v)
	p.Refutations = []Refutation{{Slot: "R1", Round: 1, Finding: fs[0].ID, FindingSHA256: fs[0].SHA256}}
	writePassJSON(t, dir, FileName, p)
	if d := passDecision(t, dir); d.Action != Allow {
		t.Fatalf("matching item: %+v", d)
	}
	v.ConcreteChangesRequired[0] = "edited"
	writePassJSON(t, dir, "R1-round-1.json", v)
	r := passTally(t, dir)
	if d := passDecision(t, dir); d.Action != Block || !strings.Contains(d.Message, "ccr[0]") {
		t.Fatalf("changed finding discharged: %+v", d)
	}
	if r.ResolvedCount() != 0 {
		t.Fatal("changed finding reported resolved")
	}
}
func TestPassRule_ItemLevelDischarge(t *testing.T) {
	dir, p := passFixture(t, 2, VerdictApprove)
	v := FindingVerdict{Verdict: VerdictRequestChanges, ConcreteChangesRequired: []string{"first", "second", "third"}}
	writePassJSON(t, dir, "R1-round-1.json", v)
	fs, _ := Findings(v)
	p.Refutations = []Refutation{{Slot: "R1", Round: 1, Finding: fs[0].ID, FindingSHA256: fs[0].SHA256}}
	writePassJSON(t, dir, FileName, p)
	r := passTally(t, dir)
	if d := passDecision(t, dir); d.Action != Block {
		t.Fatalf("partial item resolved slot: %+v", d)
	}
	if got := r.UnresolvedNames(); strings.Contains(got, "ccr[0]") || !strings.Contains(got, "ccr[1]") || !strings.Contains(got, "ccr[2]") {
		t.Fatal(got)
	}
	if len(r.AppliedRefutations()) != 1 {
		t.Fatal("partial effective refutation lost")
	}
	pinSlot(t, dir, &p, "R1")
	if d := passDecision(t, dir); d.Action != Allow {
		t.Fatalf("slot refutation: %+v", d)
	}
}
func TestPassRule_SlotLevelVoidedByChange(t *testing.T) {
	for _, rationale := range []bool{false, true} {
		t.Run(fmt.Sprint(rationale), func(t *testing.T) {
			dir, p := passFixture(t, 2, VerdictApprove)
			v := FindingVerdict{Verdict: VerdictRequestChanges, ConcreteChangesRequired: []string{"first", "second", "third"}}
			if rationale {
				v.ConcreteChangesRequired = nil
				v.Rationale = "first"
			}
			writePassJSON(t, dir, "R1-round-1.json", v)
			pinSlot(t, dir, &p, "R1")
			if passDecision(t, dir).Action != Allow {
				t.Fatal("slot discharge did not apply")
			}
			if rationale {
				v.Rationale = "edited"
			} else {
				v.ConcreteChangesRequired = append(v.ConcreteChangesRequired, "fourth")
			}
			writePassJSON(t, dir, "R1-round-1.json", v)
			if d := passDecision(t, dir); d.Action != Block {
				t.Fatalf("changed slot discharged: %+v", d)
			}
		})
	}
}
func TestPassRule_DeferralInactive(t *testing.T) {
	dir, p := passFixture(t, 2, VerdictApprove)
	v := FindingVerdict{Verdict: VerdictRequestChanges, ConcreteChangesRequired: []string{"first"}}
	writePassJSON(t, dir, "R1-round-1.json", v)
	fs, _ := Findings(v)
	p.Deferrals = []Deferral{{Slot: "R1", Round: 1, Finding: fs[0].ID, FindingSHA256: fs[0].SHA256, FindingText: fs[0].Text, Bead: "mindspec-followup", By: "operator", CreatedAt: ledgerTime}}
	writePassJSON(t, dir, FileName, p)
	if d := passDecision(t, dir); d.Action != Block {
		t.Fatalf("deferral discharged legacy panel: %+v", d)
	}
	makeNonLegacy(t, dir, &p)
	if d := passDecision(t, dir); d.Action != Block || !strings.Contains(d.Message, "unresolved") {
		t.Fatalf("carried deferral discharged: %+v", d)
	}
}
func TestApproveThreshold_LegacyNotice(t *testing.T) {
	for _, value := range []string{"unanimous", "7", "0"} {
		dir, p := passFixture(t, 2, VerdictRequestChanges)
		p.ApproveThresholdExpr = value
		pinSlot(t, dir, &p, "R1")
		pinSlot(t, dir, &p, "R2")
		if d := passDecision(t, dir); d.Action != Allow {
			t.Fatalf("legacy threshold %s used: %+v", value, d)
		}
		want := "approve_threshold " + value + " ignored (legacy; not decision-bearing)"
		if got := LegacyThresholdNotice(&p); got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}

func makeNonLegacy(t *testing.T, dir string, p *Panel) {
	t.Helper()
	p.Telemetry = "required"
	event := RoundRecord{Record: RecordRound, Spec: p.Spec, Panel: "p", Round: p.Round, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target, Gate: p.Gate, BeadID: p.BeadID, Roster: p.Roster, ExpectedReviewers: p.ExpectedReviewers, Telemetry: p.Telemetry, Carried: CarriedEntries{Refutations: p.Refutations, Deferrals: p.Deferrals, Rebinds: p.Rebinds}, CreatedAt: ledgerTime}
	if err := AppendRecords(dir, mustMarshal(t, event)); err != nil {
		t.Fatal(err)
	}
	writePassJSON(t, dir, FileName, p)
	if r := passTally(t, dir); r.IntegrityErr != nil {
		t.Fatalf("baseline integrity: %v", r.IntegrityErr)
	}
}
