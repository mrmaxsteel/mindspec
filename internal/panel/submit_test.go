package panel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// The in-package tests cannot import paneltest because that package imports panel.
func submitFixture(t *testing.T) (SubmitTarget, []byte, SubmitResolver) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "p")
	p := Panel{Spec: "test", Target: "main", Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: strings.Repeat("a", 40), SubjectDigest: "sha256:test", Telemetry: "required", Gate: "adhoc", Roster: []RosterSlot{{Slot: "R1", Family: "harness", Model: "m", Lens: "a"}, {Slot: "R2", Family: "harness", Model: "m", Lens: "b"}}}
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	seedTestRecords(t, dir, RoundRecord{Record: RecordRound, VerdictSchema: 2, Spec: p.Spec, Panel: "p", Round: 1, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target, Gate: p.Gate, Roster: p.Roster, ExpectedReviewers: 2, Telemetry: "required", CreatedAt: ledgerTime})
	v := VerdictV2{Schema: 2, Slot: "R1", Round: 1, ReviewedSHA: p.ReviewedHeadSHA, Status: "final", Verdict: "APPROVE", Model: "m", Rationale: "reviewed", ConcreteChangesRequired: []ChangeRequired{}, CommandsRun: []CommandRun{}}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	target := SubmitTarget{Dir: dir, Spec: "test", Panel: "p", Slot: "R1", Round: 1, SlotDir: filepath.Join(root, "scratch", "test", "p", "r1", "R1")}
	resolve := func(p Panel, s RosterSlot) (SubmitResolution, error) {
		return SubmitResolution{Family: s.Family, Model: s.Model, RunnerKind: "harness", RunnerSource: "default"}, nil
	}
	return target, b, resolve
}
func seedTestRecords(t *testing.T, dir string, records ...any) {
	t.Helper()
	for _, r := range records {
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if err := AppendRecords(dir, raw); err != nil {
			t.Fatal(err)
		}
	}
}

// rawAppendSubmitTest deliberately takes no lock. Only the synchronized hook test
// uses it to alter the ledger before Submit reads attempt state.
func rawAppendSubmitTest(t *testing.T, dir string, r any) {
	t.Helper()
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, dispositionsFileName), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(append(raw, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
func submitLaunch(round, attempt int, status string) LaunchRecord {
	return LaunchRecord{Record: RecordLaunch, Spec: "test", Panel: "p", Slot: "R1", Round: round, Attempt: attempt, Family: "harness", Model: "m", Status: status, RunnerKind: "harness", RunnerSource: "default", CreatedAt: ledgerTime}
}
func verdictRecords(t *testing.T, dir string) []SubmissionRecord {
	t.Helper()
	lines, err := ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := []SubmissionRecord{}
	for _, line := range lines {
		var r SubmissionRecord
		_ = json.Unmarshal(line, &r)
		if r.Record == RecordVerdict {
			out = append(out, r)
		}
	}
	return out
}
func assertStoredBytes(t *testing.T, target SubmitTarget, b []byte) {
	t.Helper()
	stored, err := os.ReadFile(filepath.Join(target.Dir, fmt.Sprintf("%s-round-%d.json", target.Slot, target.Round)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, b) {
		t.Fatalf("stored bytes changed: %s", stored)
	}
}
func TestPanelSubmit_Writes(t *testing.T) {
	target, b, resolve := submitFixture(t)
	seedTestRecords(t, target.Dir, submitLaunch(1, 1, "dispatched"))
	r, err := Submit(target, b, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if r.Seq != 1 || r.Via != "direct" || r.Attempt != 1 || r.FileSHA256 != findingHash(b) {
		t.Fatalf("wrong record: %+v", r)
	}
	assertStoredBytes(t, target, b)
	r, err = Submit(target, b, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if r.Seq != 1 || len(verdictRecords(t, target.Dir)) != 1 {
		t.Fatal("same bytes added a submission")
	}
	events, err := SlotAttempts(target.Dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events["R1"]) != 2 || events["R1"][1].Status != "submitted" || events["R1"][1].Seq != 1 {
		t.Fatalf("missing outcome: %+v", events)
	}
}
func TestPanelSubmit_Replace(t *testing.T) {
	target, a, resolve := submitFixture(t)
	if _, err := Submit(target, a, resolve); err != nil {
		t.Fatal(err)
	}
	var v VerdictV2
	_ = json.Unmarshal(a, &v)
	v.Verdict = "REQUEST_CHANGES"
	v.ConcreteChangesRequired = []ChangeRequired{{Text: "fix", Basis: "traced"}}
	b, _ := json.Marshal(v)
	if _, err := Submit(target, b, resolve); err != nil {
		t.Fatal(err)
	}
	records := verdictRecords(t, target.Dir)
	if len(records) != 2 || records[0].Verdict != "APPROVE" || records[1].Verdict != "REQUEST_CHANGES" || records[1].Seq != 2 {
		t.Fatalf("lost replacement history: %+v", records)
	}
	assertStoredBytes(t, target, b)
	// ABA compares only the latest record, so A must create seq 3.
	r, err := Submit(target, a, resolve)
	if err != nil || r.Seq != 3 {
		t.Fatalf("ABA: %+v %v", r, err)
	}
}
func TestPanelSubmit_CrashRepair(t *testing.T) {
	target, b, resolve := submitFixture(t)
	crash := errors.New("crash after append")
	target.AfterAppend = func() error { return crash }
	if _, err := Submit(target, b, resolve); !errors.Is(err, crash) {
		t.Fatalf("hook: %v", err)
	}
	if len(verdictRecords(t, target.Dir)) != 1 {
		t.Fatal("append was not durable")
	}
	if _, err := os.Stat(filepath.Join(target.Dir, "R1-round-1.json")); !os.IsNotExist(err) {
		t.Fatalf("file exists before rename: %v", err)
	}
	// A same-bytes repair also works after reviewed_head_sha has moved.
	p := load(target.Dir).Panel
	p.ReviewedHeadSHA = strings.Repeat("b", 40)
	if err := WriteSnapshot(target.Dir, p); err != nil {
		t.Fatal(err)
	}
	target.AfterAppend = nil
	r, err := Submit(target, b, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if r.Seq != 1 || len(verdictRecords(t, target.Dir)) != 1 {
		t.Fatal("repair added a submission")
	}
	assertStoredBytes(t, target, b)
}
func TestPanelSubmit_Race(t *testing.T) {
	target, a, resolve := submitFixture(t)
	var v VerdictV2
	_ = json.Unmarshal(a, &v)
	v.Rationale = "different bytes"
	b, _ := json.Marshal(v)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, data := range [][]byte{a, b} {
		wg.Add(1)
		go func(data []byte) { defer wg.Done(); <-start; _, err := Submit(target, data, resolve); errs <- err }(data)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	records := verdictRecords(t, target.Dir)
	if len(records) != 2 || records[0].Seq != 1 || records[1].Seq != 2 {
		t.Fatalf("bad sequences: %+v", records)
	}
	stored, err := os.ReadFile(filepath.Join(target.Dir, "R1-round-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if findingHash(stored) != records[1].FileSHA256 {
		t.Fatal("file differs from latest record")
	}
}
func TestPanelSubmit_AttemptBindingUnderLock(t *testing.T) {
	target, b, resolve := submitFixture(t)
	seedTestRecords(t, target.Dir, submitLaunch(1, 1, "dispatched"))
	target.File = filepath.Join(target.SlotDir, "attempt-1", "verdict.json")
	entered := make(chan struct{})
	release := make(chan struct{})
	old := submitLockHeldHook
	submitLockHeldHook = func() { close(entered); <-release }
	t.Cleanup(func() { submitLockHeldHook = old })
	done := make(chan error, 1)
	go func() { _, err := Submit(target, b, resolve); done <- err }()
	<-entered
	rawAppendSubmitTest(t, target.Dir, submitLaunch(1, 1, "launch_failed"))
	rawAppendSubmitTest(t, target.Dir, submitLaunch(1, 2, "dispatched"))
	close(release)
	err := <-done
	if err == nil || !strings.Contains(err.Error(), "attempt 1 was superseded by 2") {
		t.Fatalf("accepted superseded attempt: %v", err)
	}
	if len(verdictRecords(t, target.Dir)) != 0 {
		t.Fatal("superseded attempt appended verdict")
	}
	events, err := SlotAttempts(target.Dir, 1)
	if err != nil || len(events["R1"]) != 3 {
		t.Fatalf("submit appended an outcome: %+v %v", events, err)
	}
}
func TestLatestSubmissions(t *testing.T) {
	target, x, resolve := submitFixture(t)
	if _, err := Submit(target, x, resolve); err != nil {
		t.Fatal(err)
	}
	p := load(target.Dir).Panel
	p.Round = 2
	if err := WriteSnapshot(target.Dir, p); err != nil {
		t.Fatal(err)
	}
	seedTestRecords(t, target.Dir, RoundRecord{Record: RecordRound, VerdictSchema: 2, Spec: p.Spec, Panel: "p", Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target, Gate: p.Gate, Roster: p.Roster, ExpectedReviewers: 2, Telemetry: p.Telemetry, CreatedAt: ledgerTime})
	var v VerdictV2
	_ = json.Unmarshal(x, &v)
	v.Round = 2
	v.Rationale = "Y"
	v.Status = "draft"
	target.Round = 2
	y, _ := json.Marshal(v)
	if _, err := Submit(target, y, resolve); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"draft", "final"} {
		if status == "final" {
			v.Status = status
			y, _ = json.Marshal(v)
			if _, err := Submit(target, y, resolve); err != nil {
				t.Fatal(err)
			}
		}
		one, err := LatestSubmissions(target.Dir, 1)
		if err != nil {
			t.Fatal(err)
		}
		two, err := LatestSubmissions(target.Dir, 2)
		if err != nil {
			t.Fatal(err)
		}
		three, err := LatestSubmissions(target.Dir, 3)
		if err != nil {
			t.Fatal(err)
		}
		if one["R1"].FileSHA256 != findingHash(x) || two["R1"].FileSHA256 != findingHash(y) || two["R1"].Status != status || len(three) != 0 || len(two) != 1 {
			t.Fatalf("round isolation: %+v %+v %+v", one, two, three)
		}
	}
}
func TestSlotAttempts(t *testing.T) {
	target, b, resolve := submitFixture(t)
	seedTestRecords(t, target.Dir, submitLaunch(1, 1, "dispatched"), submitLaunch(1, 1, "launch_failed"), submitLaunch(1, 2, "dispatched"))
	outcome := submitLaunch(1, 2, "submitted")
	outcome.Seq = 1
	seedTestRecords(t, target.Dir, outcome)
	p := load(target.Dir).Panel
	p.Round = 2
	if err := WriteSnapshot(target.Dir, p); err != nil {
		t.Fatal(err)
	}
	seedTestRecords(t, target.Dir, RoundRecord{Record: RecordRound, VerdictSchema: 2, Spec: p.Spec, Panel: "p", Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target, Gate: p.Gate, Roster: p.Roster, ExpectedReviewers: 2, Telemetry: p.Telemetry, CreatedAt: ledgerTime}, submitLaunch(2, 1, "dispatched"))
	for _, dir := range []string{filepath.Join(target.SlotDir, "attempt-1"), filepath.Join(target.SlotDir, "attempt-2")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	target.SlotDir = filepath.Join(filepath.Dir(filepath.Dir(target.SlotDir)), "r2", "R1")
	if err := os.MkdirAll(filepath.Join(target.SlotDir, "attempt-1"), 0o700); err != nil {
		t.Fatal(err)
	}
	events, err := SlotAttempts(target.Dir, 2)
	if err != nil || len(events["R1"]) != 1 || events["R1"][0].Attempt != 1 {
		t.Fatalf("round isolation: %+v %v", events, err)
	}
	k, err := NextAttemptNumber(target.Dir, 2, "R1", target.SlotDir)
	if err != nil || k != 2 {
		t.Fatalf("next attempt %d %v", k, err)
	}
	var v VerdictV2
	_ = json.Unmarshal(b, &v)
	v.Round = 2
	b, _ = json.Marshal(v)
	target.Round = 2
	target.File = filepath.Join(target.SlotDir, "attempt-1", "verdict.json")
	if _, err := Submit(target, b, resolve); err != nil {
		t.Fatal(err)
	}
	events, err = SlotAttempts(target.Dir, 2)
	if err != nil || events["R1"][0].AfterLatestSubmission || !events["R1"][1].AfterLatestSubmission || events["R1"][1].Seq != 1 {
		t.Fatalf("relative ledger positions: %+v %v", events, err)
	}
}
func TestPanelSubmit_CoreRefusals(t *testing.T) {
	for _, name := range []string{"process", "landed", "sha", "model", "round", "slot", "not dispatched", "ended", "abandoned"} {
		t.Run(name, func(t *testing.T) {
			target, b, resolve := submitFixture(t)
			var v VerdictV2
			_ = json.Unmarshal(b, &v)
			switch name {
			case "process":
				resolve = func(p Panel, s RosterSlot) (SubmitResolution, error) {
					return SubmitResolution{Family: s.Family, Model: s.Model, RunnerKind: "process", RunnerSource: "default"}, nil
				}
			case "landed":
				resolve = func(p Panel, s RosterSlot) (SubmitResolution, error) {
					return SubmitResolution{Family: s.Family, Model: s.Model, RunnerKind: "harness", RunnerSource: "default", Landed: true}, nil
				}
			case "sha":
				v.ReviewedSHA = strings.Repeat("b", 40)
			case "model":
				v.Model = "other"
			case "round":
				target.Round = 3
			case "slot":
				target.Slot = "R9"
			case "not dispatched":
				target.File = filepath.Join(target.SlotDir, "attempt-1", "v.json")
			case "ended":
				target.File = filepath.Join(target.SlotDir, "attempt-1", "v.json")
				seedTestRecords(t, target.Dir, submitLaunch(1, 1, "dispatched"), submitLaunch(1, 1, "launch_failed"))
			case "abandoned":
				p := load(target.Dir).Panel
				p.Abandoned = true
				if err := WriteSnapshot(target.Dir, p); err != nil {
					t.Fatal(err)
				}
			}
			b, _ = json.Marshal(v)
			before, err := os.ReadFile(filepath.Join(target.Dir, dispositionsFileName))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Submit(target, b, resolve); err == nil {
				t.Fatal("accepted forbidden submit")
			}
			after, _ := os.ReadFile(filepath.Join(target.Dir, dispositionsFileName))
			if !bytes.Equal(before, after) {
				t.Fatal("refusal mutated ledger")
			}
		})
	}
}
