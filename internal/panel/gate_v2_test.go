package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func gateV2Fixture(t *testing.T) (string, SubmitTarget, VerdictV2) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "p")
	in := CreateInput{Spec: "test", Target: "main", Gate: "adhoc", Round: 1, ReviewedHeadSHA: strings.Repeat("a", 40), SubjectDigest: "sha256:test", Roster: []RosterSlot{{Slot: "R1", Family: "harness", Model: "m", Lens: "one"}, {Slot: "R2", Family: "harness", Model: "m", Lens: "two"}, {Slot: "R3", Family: "harness", Model: "m", Lens: "three"}, {Slot: "R4", Family: "harness", Model: "m", Lens: "four"}}}
	if err := Create(dir, in); err != nil {
		t.Fatal(err)
	}
	v := VerdictV2{Schema: 2, Slot: "R1", Round: 1, ReviewedSHA: in.ReviewedHeadSHA, Status: "final", Verdict: "APPROVE", Model: "m", Rationale: "reviewed", ConcreteChangesRequired: []ChangeRequired{}, CommandsRun: []CommandRun{}}
	target := SubmitTarget{Dir: dir, Panel: "p", Spec: "test", Slot: "R1", Round: 1}
	other := v
	for _, slot := range in.Roster[1:] {
		other.Slot = slot.Slot
		gateV2Submit(t, target, other)
	}
	return dir, target, v
}
func gateV2Resolve(_ Panel, slot RosterSlot) (SubmitResolution, error) {
	return SubmitResolution{Family: slot.Family, Model: slot.Model, RunnerKind: "harness", RunnerSource: "default"}, nil
}
func gateV2Submit(t *testing.T, target SubmitTarget, v VerdictV2) SubmissionRecord {
	t.Helper()
	target.Slot = v.Slot
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	sub, err := Submit(target, raw, gateV2Resolve)
	if err != nil {
		t.Fatal(err)
	}
	return sub
}
func gateV2Read(t *testing.T, dir string) *Result {
	t.Helper()
	r, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func gateV2File(t *testing.T, dir string, v VerdictV2) {
	t.Helper()
	b, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-round-%d.json", v.Slot, v.Round)), b, 0600); err != nil {
		t.Fatal(err)
	}
}
func gateV2AssertOut(t *testing.T, dir string, unsubmitted bool) {
	t.Helper()
	r := gateV2Read(t, dir)
	if r.Complete() || r.PassRule(true) || len(r.Verdicts) != 3 || r.Approves != 3 || r.Rejects != 0 || len(r.HardBlocks) != 0 {
		t.Fatalf("excluded slot counted: %+v", r)
	}
	if unsubmitted && !reflect.DeepEqual(r.Unsubmitted, []string{"R1-round-1.json"}) {
		t.Fatalf("unsubmitted: %+v", r)
	}
	if !unsubmitted && (len(r.NotIn) != 1 || r.NotIn[0].File != "R1-round-1.json" || r.NotIn[0].Reason == "") {
		t.Fatalf("not-in: %+v", r)
	}
	before, err := ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCoverageManifest(r); err != nil {
		t.Fatal(err)
	}
	after, err := ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("incomplete tally wrote a manifest")
	}
}
func TestTally_DirectCallerCountsOnlyInSlots(t *testing.T) {
	for _, state := range []string{"unsubmitted", "dispatched", "draft", "void", "failed"} {
		t.Run(state, func(t *testing.T) {
			dir, target, v := gateV2Fixture(t)
			switch state {
			case "unsubmitted":
				gateV2File(t, dir, v)
			case "draft":
				v.Status = "draft"
				gateV2Submit(t, target, v)
			case "void":
				v.Verdict = "VOID"
				reason := "cannot review"
				v.VoidReason = &reason
				gateV2Submit(t, target, v)
			default:
				gateV2Submit(t, target, v)
				seedTestRecords(t, dir, submitLaunch(1, 1, "dispatched"))
				if state == "failed" {
					seedTestRecords(t, dir, submitLaunch(1, 1, "launch_failed"))
				}
			}
			gateV2AssertOut(t, dir, state == "unsubmitted")
		})
	}
}
func TestGateV2_SubmittedOnly(t *testing.T) {
	dir, target, v := gateV2Fixture(t)
	gateV2File(t, dir, v)
	gateV2AssertOut(t, dir, true)
	gateV2Submit(t, target, v)
	if !gateV2Read(t, dir).PassRule(true) {
		t.Fatal("submitted approvals did not pass")
	}
	for _, kind := range []string{"REJECT", "hard_block", "INVALID"} {
		t.Run(kind, func(t *testing.T) {
			d, tar, v := gateV2Fixture(t)
			gateV2Submit(t, tar, v)
			off := map[string]any{"verdict": kind}
			if kind == "hard_block" {
				off["verdict"] = "APPROVE"
				off["hard_block"] = true
			}
			b, _ := json.Marshal(off)
			if err := os.WriteFile(filepath.Join(d, "outsider-round-1.json"), b, 0600); err != nil {
				t.Fatal(err)
			}
			r := gateV2Read(t, d)
			vote, msg := r.VoteDecision()
			if r.PassRule(true) || vote != VoteBlock || !strings.Contains(msg, "outsider-round-1.json") || len(r.Unsubmitted) != 0 || len(r.Unexpected) != 1 {
				t.Fatalf("off-roster veto lost: %s %+v", msg, r)
			}
		})
	}
}
func TestGateV2_DigestBinding(t *testing.T) {
	dir, target, v := gateV2Fixture(t)
	gateV2Submit(t, target, v)
	v.Rationale = "edited"
	gateV2File(t, dir, v)
	gateV2AssertOut(t, dir, false)
}
func TestGateV2_DraftAndVoidMissing(t *testing.T) {
	for _, state := range []string{"draft", "VOID"} {
		t.Run(state, func(t *testing.T) {
			dir, target, v := gateV2Fixture(t)
			if state == "draft" {
				v.Status = "draft"
			} else {
				v.Verdict = "VOID"
				reason := "no review"
				v.VoidReason = &reason
			}
			gateV2Submit(t, target, v)
			gateV2AssertOut(t, dir, false)
		})
	}
}
func gateV2Refute(t *testing.T, dir string, v Verdict, finding Finding) {
	t.Helper()
	p := load(dir).Panel
	ref := Refutation{Slot: v.Slot, Round: v.Round, Finding: finding.ID, FindingSHA256: finding.SHA256, FindingText: finding.Text, Ground: "disproven", Reason: "verified", Evidence: "https://example.com/proof", By: "operator", CreatedAt: ledgerTime}
	seedTestRecords(t, dir, RefutationRecord{Record: RecordRefutation, Spec: "test", Panel: "p", Refutation: ref})
	p.Refutations = append(p.Refutations, ref)
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
}
func TestGateV2_ReplacementAndDischarge(t *testing.T) {
	dir, target, v := gateV2Fixture(t)
	v.Verdict = "REQUEST_CHANGES"
	v.ConcreteChangesRequired = []ChangeRequired{{Text: "a", Basis: "traced"}}
	gateV2Submit(t, target, v)
	r := gateV2Read(t, dir)
	gateV2Refute(t, dir, r.Verdicts[0], r.Verdicts[0].Findings[0])
	if !gateV2Read(t, dir).PassRule(true) {
		t.Fatal("refutation did not discharge")
	}
	v.Rationale = "replacement"
	gateV2Submit(t, target, v)
	if !gateV2Read(t, dir).PassRule(true) {
		t.Fatal("same item text lost its refutation after replacement")
	}
	v.ConcreteChangesRequired[0].Text = "b"
	gateV2Submit(t, target, v)
	if gateV2Read(t, dir).PassRule(true) {
		t.Fatal("changed finding kept stale discharge")
	}
	v.Verdict = "APPROVE"
	v.ConcreteChangesRequired = []ChangeRequired{}
	gateV2Submit(t, target, v)
	if !gateV2Read(t, dir).PassRule(true) {
		t.Fatal("four submitted approvals did not pass after replacement")
	}
	v.Verdict = "REQUEST_CHANGES"
	v.ConcreteChangesRequired = []ChangeRequired{{Text: "b", Basis: "traced"}}
	gateV2Submit(t, target, v)
	if gateV2Read(t, dir).PassRule(true) {
		t.Fatal("an APPROVE replaced by a REQUEST_CHANGES must block")
	}
	v.Verdict = "REJECT"
	v.ConcreteChangesRequired = []ChangeRequired{}
	gateV2Submit(t, target, v)
	if gateV2Read(t, dir).PassRule(true) {
		t.Fatal("latest reject did not veto")
	}
}

func TestGateV2_RoundSchemaDowngrade(t *testing.T) {
	dir, target, v := gateV2Fixture(t)
	gateV2Submit(t, target, v)
	round, err := RoundSchema(dir)
	if err != nil || round.VerdictSchema != 2 {
		t.Fatalf("want v2 round, got %+v: %v", round, err)
	}
	round.Round++
	round.VerdictSchema = 0
	seedTestRecords(t, dir, round)
	if _, err := RoundSchema(dir); err == nil || !strings.Contains(err.Error(), "removed the verdict schema") {
		t.Fatalf("want schema downgrade error, got %v", err)
	}
	r := gateV2Read(t, dir)
	if r.IntegrityErr == nil || r.PassRule(true) {
		t.Fatal("schema downgrade did not block the gate")
	}
	if err := Create(dir, CreateInput{Spec: round.Spec, Target: round.Target, Gate: round.Gate, Round: 3, ReviewedHeadSHA: round.ReviewedHeadSHA, SubjectDigest: round.SubjectDigest, Roster: round.Roster}); err != nil {
		t.Fatal(err)
	}
	round, err = RoundSchema(dir)
	if err != nil || round.Round != 3 || round.VerdictSchema != 2 {
		t.Fatalf("want recovered v2 round 3, got %+v: %v", round, err)
	}
	target.Round, v.Round = 3, 3
	for _, slot := range round.Roster {
		v.Slot = slot.Slot
		gateV2Submit(t, target, v)
	}
	r = gateV2Read(t, dir)
	statuses, err := ReadSlotStatuses(r, nil, time.Now())
	if err != nil || len(statuses) != len(round.Roster) {
		t.Fatalf("recovered slot statuses: %+v: %v", statuses, err)
	}
	for _, status := range statuses {
		if status.Status != "final" {
			t.Fatalf("recovered slot is not final: %+v", status)
		}
	}
	if r.IntegrityErr != nil || !r.PassRule(true) {
		t.Fatalf("recovered round did not pass: %+v", r)
	}
}

func TestGateV2_LaterAttemptSubmissionSequence(t *testing.T) {
	for _, matching := range []bool{false, true} {
		t.Run(fmt.Sprint(matching), func(t *testing.T) {
			dir, target, v := gateV2Fixture(t)
			old := gateV2Submit(t, target, v)
			v.Rationale = "replacement"
			latest := gateV2Submit(t, target, v)
			outcome := submitLaunch(1, 1, "submitted")
			outcome.Seq = old.Seq
			if matching {
				outcome.Seq = latest.Seq
			}
			seedTestRecords(t, dir, submitLaunch(1, 1, "dispatched"), outcome)
			r := gateV2Read(t, dir)
			if r.Complete() != matching || r.PassRule(true) != matching {
				t.Fatalf("submitted seq %d, latest seq %d: complete=%v pass=%v", outcome.Seq, latest.Seq, r.Complete(), r.PassRule(true))
			}
			if !matching && (len(r.NotIn) != 1 || r.NotIn[0].Reason != "later attempt is not closed") {
				t.Fatalf("wrong exclusion: %+v", r.NotIn)
			}
		})
	}
}

func TestGateV2_SubmittedOutcomeUsesNamedSubmission(t *testing.T) {
	for _, state := range []string{"draft", "void"} {
		t.Run(state, func(t *testing.T) {
			dir, target, v := gateV2Fixture(t)
			if state == "draft" {
				v.Status = "draft"
			} else {
				v.Verdict = "VOID"
				reason := "cannot review"
				v.VoidReason = &reason
			}
			named := gateV2Submit(t, target, v)
			v.Status, v.Verdict, v.VoidReason = "final", "APPROVE", nil
			latest := gateV2Submit(t, target, v)
			if named.Seq == latest.Seq {
				t.Fatal("fixture needs distinct submissions")
			}
			// Submit cannot close a later attempt with an older sequence.
			// Seed this valid record sequence to test the reader directly.
			outcome := submitLaunch(1, 1, "submitted")
			outcome.Seq = named.Seq
			seedTestRecords(t, dir, submitLaunch(1, 1, "dispatched"), outcome)
			r := gateV2Read(t, dir)
			slots, err := ReadSlotStatuses(r, nil, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if len(slots) != 4 || slots[0].Slot != "R1" || slots[0].Status != state {
				t.Fatalf("outcome names seq %d, latest is %d: want %s, got %+v", named.Seq, latest.Seq, state, slots)
			}
			if r.PassRule(true) {
				t.Fatal("older submitted sequence closed the latest final")
			}
		})
	}
}
func TestGateV2_DischargePositional(t *testing.T) {
	dir, target, v := gateV2Fixture(t)
	v.Verdict = "REQUEST_CHANGES"
	v.ConcreteChangesRequired = []ChangeRequired{{Text: "a", Basis: "traced"}, {Text: "b", Basis: "traced"}}
	gateV2Submit(t, target, v)
	r := gateV2Read(t, dir)
	gateV2Refute(t, dir, r.Verdicts[0], r.Verdicts[0].Findings[1])
	r = gateV2Read(t, dir)
	if len(r.unresolvedFindings(r.Verdicts[0])) != 1 || r.unresolvedFindings(r.Verdicts[0])[0] != "ccr[0]" {
		t.Fatalf("wrong discharge: %+v", r.unresolvedFindings(r.Verdicts[0]))
	}
	v.ConcreteChangesRequired[0], v.ConcreteChangesRequired[1] = v.ConcreteChangesRequired[1], v.ConcreteChangesRequired[0]
	gateV2Submit(t, target, v)
	if len(gateV2Read(t, dir).unresolvedFindings(gateV2Read(t, dir).Verdicts[0])) != 2 {
		t.Fatal("reorder kept positional discharge")
	}
}
func TestCreate_AlwaysV2(t *testing.T) {
	dir, _, _ := gateV2Fixture(t)
	p := load(dir).Panel
	round, err := RoundSchema(dir)
	if err != nil || p.VerdictSchema != 2 || round.VerdictSchema != 2 {
		t.Fatalf("not v2: %+v %+v %v", p, round, err)
	}
	in := roundCreateInput()
	in.Round = 2
	in.ReviewersOverride = true
	in.ReviewedHeadSHA = strings.Repeat("a", 40)
	createMust(t, dir, in)
	round, err = RoundSchema(dir)
	if err != nil || round.VerdictSchema != 2 || load(dir).Panel.VerdictSchema != 2 {
		t.Fatal("round bump not v2")
	}
}
func TestRoster_Family(t *testing.T) {
	dir, _, _ := gateV2Fixture(t)
	for _, slot := range load(dir).Panel.Roster {
		if slot.Family != "harness" {
			t.Fatalf("missing family: %+v", slot)
		}
	}
}
func TestGate_VerdictSchemaPinned(t *testing.T) {
	t.Run("round downgrade", assertRoundSchemaDowngradeBlocked)
	dir, _, _ := gateV2Fixture(t)
	p := load(dir).Panel
	p.VerdictSchema = 0
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	r := gateV2Read(t, dir)
	if r.IntegrityErr == nil || !strings.Contains(r.IntegrityErr.Error(), "verdict_schema") || r.PassRule(true) {
		t.Fatalf("schema edit accepted: %+v", r)
	}
}
func TestGate_PreV2RoundKeepsV1Rules(t *testing.T) {
	target, _, _ := submitFixtureSchema(t, 0)
	// A snapshot flag alone never upgrades the event's v1 rules.
	p := load(target.Dir).Panel
	p.VerdictSchema = 2
	if err := WriteSnapshot(target.Dir, p); err != nil {
		t.Fatal(err)
	}
	for _, slot := range []string{"R1", "R2"} {
		createWrite(t, target.Dir, slot+"-round-1.json", `{"verdict":"APPROVE"}`)
	}
	r := gateV2Read(t, target.Dir)
	if r.VerdictSchema != 0 || len(r.Verdicts) != 2 || r.IntegrityErr == nil {
		t.Fatalf("snapshot changed event schema: %+v", r)
	}
	p.VerdictSchema = 0
	if err := WriteSnapshot(target.Dir, p); err != nil {
		t.Fatal(err)
	}
	if !gateV2Read(t, target.Dir).PassRule(true) {
		t.Fatal("v1 no-submission files no longer pass")
	}
	t.Run("round downgrade", assertRoundSchemaDowngradeBlocked)
}
func TestCreate_InvalidSlotModel(t *testing.T) {
	for _, model := range []string{"", "-model", "bad model"} {
		for name, create := range map[string]func(string, CreateInput) error{"Create": Create, "CheckCreate": CheckCreate} {
			t.Run(fmt.Sprintf("%s/%q", name, model), func(t *testing.T) {
				target, _, _ := submitFixture(t)
				p := load(target.Dir).Panel
				p.Roster[0].Model = model
				in := CreateInput{Spec: p.Spec, Target: p.Target, Gate: p.Gate, Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Roster: p.Roster, ReviewersOverride: true}
				beforePanel := createRead(t, target.Dir, FileName)
				beforeLedger := createRead(t, target.Dir, dispositionsFileName)
				if err := create(target.Dir, in); err == nil || !strings.Contains(err.Error(), "invalid slot model") {
					t.Errorf("model refusal: %v", err)
				}
				if createRead(t, target.Dir, FileName) != beforePanel {
					t.Error("model refusal changed panel bytes")
				}
				if createRead(t, target.Dir, dispositionsFileName) != beforeLedger {
					t.Error("model refusal changed ledger bytes")
				}
			})
		}
	}
}

func TestCreate_FirstV2RoundFamily(t *testing.T) {
	for _, matching := range []bool{true, false} {
		t.Run(fmt.Sprint(matching), func(t *testing.T) {
			target, _, _ := submitFixtureSchema(t, 0)
			p := load(target.Dir).Panel
			// Seed another v1 fixture with the historical family-less roster.
			dir := filepath.Join(t.TempDir(), "p")
			for i := range p.Roster {
				p.Roster[i].Family = ""
			}
			if err := WriteSnapshot(dir, p); err != nil {
				t.Fatal(err)
			}
			seedTestRecords(t, dir, RoundRecord{Record: RecordRound, Spec: p.Spec, Panel: "p", Round: 1, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target, Gate: p.Gate, Roster: p.Roster, ExpectedReviewers: 2, Telemetry: "required", CreatedAt: ledgerTime})
			before, _ := ReadLedger(dir)
			in := CreateInput{Spec: p.Spec, Target: p.Target, Gate: p.Gate, Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Roster: []RosterSlot{{Slot: "R1", Family: "harness", Model: "m", Lens: "test"}, {Slot: "R2", Family: "harness", Model: "m", Lens: "test"}}, HasRunner: func(s string) bool { return matching && s == "m" }}
			err := Create(dir, in)
			if !matching {
				if err == nil || !strings.Contains(err.Error(), "--reviewers") {
					t.Fatalf("missing carry refusal: %v", err)
				}
				after, _ := ReadLedger(dir)
				if !reflect.DeepEqual(before, after) {
					t.Fatal("refusal changed ledger")
				}
				in.ReviewersOverride = true
				if err := Create(dir, in); err != nil {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			after, _ := ReadLedger(dir)
			if !reflect.DeepEqual(before, after[:len(before)]) {
				t.Fatal("changed earlier records")
			}
			family := "m"
			if !matching {
				family = "harness"
			}
			for _, slot := range load(dir).Panel.Roster {
				if slot.Family != family {
					t.Fatalf("wrong carry: %+v", slot)
				}
			}
		})
	}
}

func TestCreate_FirstV2RoundDefaultFamily(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	in := CreateInput{Spec: "test", Target: "main", Gate: "adhoc", Round: 1, ReviewedHeadSHA: strings.Repeat("a", 40), SubjectDigest: "sha256:test", Roster: []RosterSlot{{Slot: "R1", Model: "m", Lens: "one"}, {Slot: "R2", Model: "m", Lens: "two"}}}
	if err := Create(dir, in); err != nil {
		t.Fatal(err)
	}
	for _, slot := range load(dir).Panel.Roster {
		if slot.Family != "harness" {
			t.Errorf("snapshot slot %s family = %q, want harness", slot.Slot, slot.Family)
		}
	}
	lines, err := ReadLedger(dir)
	if err != nil || len(lines) != 1 {
		t.Fatalf("round ledger: %s %v", lines, err)
	}
	var round RoundRecord
	if err := json.Unmarshal(lines[0], &round); err != nil {
		t.Fatal(err)
	}
	if round.Record != RecordRound || len(round.Roster) != len(in.Roster) {
		t.Fatalf("round event: %+v", round)
	}
	for _, slot := range round.Roster {
		if slot.Family != "harness" {
			t.Errorf("round slot %s family = %q, want harness", slot.Slot, slot.Family)
		}
	}
}

func assertRoundSchemaDowngradeBlocked(t *testing.T) {
	t.Helper()
	dir, _, _ := gateV2Fixture(t)
	round, err := RoundSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := load(dir).Panel
	p.Round, p.VerdictSchema = 2, 0
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	round.Round, round.VerdictSchema = 2, 0
	seedTestRecords(t, dir, round)
	for _, slot := range round.Roster {
		createWrite(t, dir, slot.Slot+"-round-2.json", `{"verdict":"APPROVE"}`)
	}
	r := gateV2Read(t, dir)
	if r.IntegrityErr == nil || !strings.Contains(r.IntegrityErr.Error(), "removed the verdict schema") || r.PassRule(true) {
		t.Fatalf("consistent schema downgrade accepted: integrity=%v pass=%v", r.IntegrityErr, r.PassRule(true))
	}
}

func TestGate_PreV2LargeVerdict(t *testing.T) {
	target, _, _ := submitFixtureSchema(t, 0)
	raw := `{"verdict":"APPROVE","rationale":"` + strings.Repeat("x", MaxVerdictBytes) + `","concrete_changes_required":["prior item"]}`
	for _, slot := range []string{"R1", "R2"} {
		createWrite(t, target.Dir, slot+"-round-1.json", raw)
	}
	r := gateV2Read(t, target.Dir)
	if !r.PassRule(true) || r.InvalidCount() != 0 {
		t.Errorf("large historical approvals must pass: %+v", r)
	}
	slots, err := ReadSlotStatuses(r, nil, time.Now())
	if err != nil || len(slots) != 2 {
		t.Fatalf("statuses: %+v %v", slots, err)
	}
	for _, slot := range slots {
		if slot.Status != "final" {
			t.Errorf("large historical verdict must be final: %+v", slot)
		}
	}
	for _, read := range []func(string, string, int) (FindingVerdict, error){ReadFindingVerdict, ReadSubmittedFindingVerdict} {
		v, err := read(target.Dir, "R1", 1)
		if err != nil || v.Verdict != "APPROVE" {
			t.Errorf("historical finding reader: %+v %v", v, err)
		}
	}
	// A new v2 round must not impose its read limit on historical evidence.
	p := load(target.Dir).Panel
	if err := Create(target.Dir, CreateInput{Spec: p.Spec, Target: p.Target, Gate: p.Gate, Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Roster: p.Roster}); err != nil {
		t.Fatal(err)
	}
	if v, err := ReadFindingVerdict(target.Dir, "R1", 1); err != nil || v.Verdict != "APPROVE" {
		t.Errorf("read with the file's round schema: %v", err)
	}
	prior, err := PriorRoundItems(target.Dir, "R1", 2)
	if err != nil || !reflect.DeepEqual(prior.Items, []string{"prior item"}) {
		t.Errorf("historical prompt reader: %+v %v", prior, err)
	}
}

func TestGateV2_OversizeVerdictWithoutSubmission(t *testing.T) {
	target, _, _ := submitFixture(t)
	createWrite(t, target.Dir, "R1-round-1.json", `{"verdict":"APPROVE","rationale":"`+strings.Repeat("x", MaxVerdictBytes)+`"}`)
	r := gateV2Read(t, target.Dir)
	if r.InvalidCount() != 1 || len(r.Verdicts) != 1 || r.Verdicts[0].ReadError == "" || r.PassRule(true) {
		t.Fatalf("oversize v2 verdict must be INVALID: %+v", r)
	}
	slots, err := ReadSlotStatuses(r, nil, time.Now())
	if err != nil || len(slots) != 2 || slots[0].Status != "unsubmitted" || slots[1].Status != "missing" {
		t.Fatalf("unreadable v2 verdict without submission must be unsubmitted: %+v %v", slots, err)
	}
}

func TestGateV2_OversizeVerdictAfterClosedFinal(t *testing.T) {
	target, data, resolve := submitFixture(t)
	if _, err := Submit(target, data, resolve); err != nil {
		t.Fatal(err)
	}
	createWrite(t, target.Dir, "R1-round-1.json", strings.Repeat("x", MaxVerdictBytes+1))
	slots, err := ReadSlotStatuses(gateV2Read(t, target.Dir), nil, time.Now())
	if err != nil || len(slots) != 2 || slots[0].Status != "damaged" || slots[1].Status != "missing" {
		t.Fatalf("unreadable closed final verdict must be damaged: %+v %v", slots, err)
	}
	if len(slots[0].Annotations) != 1 || !strings.HasPrefix(slots[0].Annotations[0], "verdict read failed: ") {
		t.Fatalf("read failure annotation missing: %+v", slots[0])
	}
}

func TestGateV2_UnreadableVerdictStatusOrder(t *testing.T) {
	for _, tc := range []struct{ state, want string }{
		{"draft", "draft"}, {"void", "void"}, {"replacement_dispatched", "running"}, {"replacement_failed", "launch_failed"},
	} {
		t.Run(tc.state, func(t *testing.T) {
			state := tc.state
			target, data, resolve := submitFixture(t)
			var v VerdictV2
			if err := json.Unmarshal(data, &v); err != nil {
				t.Fatal(err)
			}
			switch state {
			case "draft":
				v.Status = "draft"
			case "void":
				reason := "no review"
				v.Verdict, v.VoidReason = "VOID", &reason
			}
			data, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Submit(target, data, resolve); err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(state, "replacement_") {
				seedTestRecords(t, target.Dir, submitLaunch(1, 1, "dispatched"))
				if state == "replacement_failed" {
					seedTestRecords(t, target.Dir, submitLaunch(1, 1, "launch_failed"))
				}
			}
			createWrite(t, target.Dir, "R1-round-1.json", strings.Repeat("x", MaxVerdictBytes+1))
			slots, err := ReadSlotStatuses(gateV2Read(t, target.Dir), nil, time.Now())
			if err != nil || len(slots) != 2 || slots[0].Status != tc.want || len(slots[0].Annotations) == 0 || !strings.HasPrefix(slots[0].Annotations[0], "verdict read failed: ") {
				t.Fatalf("unreadable file must keep the ordered status %s: %+v %v", tc.want, slots, err)
			}
		})
	}
}
