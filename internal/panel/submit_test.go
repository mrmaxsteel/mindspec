package panel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// The in-package tests cannot import paneltest because that package imports panel.
func submitFixture(t *testing.T) (SubmitTarget, []byte, SubmitResolver) {
	t.Helper()
	return submitFixtureSchema(t, 2)
}

func submitFixtureSchema(t *testing.T, schema int) (SubmitTarget, []byte, SubmitResolver) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "p")
	p := Panel{Spec: "test", Target: "main", Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: strings.Repeat("a", 40), SubjectDigest: "sha256:test", Telemetry: "required", Gate: "adhoc", Roster: []RosterSlot{{Slot: "R1", Family: "harness", Model: "m", Lens: "a"}, {Slot: "R2", Family: "harness", Model: "m", Lens: "b"}}}
	if err := WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	seedTestRecords(t, dir, RoundRecord{Record: RecordRound, VerdictSchema: schema, Spec: p.Spec, Panel: "p", Round: 1, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target, Gate: p.Gate, Roster: p.Roster, ExpectedReviewers: 2, Telemetry: "required", CreatedAt: ledgerTime})
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

func TestPanelSubmit_AdHocLedger(t *testing.T) {
	target, data, resolve := submitFixture(t)
	dir := filepath.Join(t.TempDir(), "adhoc")
	p := load(target.Dir).Panel
	in := CreateInput{Target: p.Target, Gate: "adhoc", Round: 1, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Roster: p.Roster}
	if err := Create(dir, in); err != nil {
		t.Fatal(err)
	}
	target.Dir, target.Spec, target.Panel = dir, "", "adhoc"
	record, err := Submit(target, data, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if record.Spec != "adhoc" || load(dir).Panel.Spec != "" {
		t.Fatalf("ad hoc identity changed: record=%+v panel=%+v", record, load(dir).Panel)
	}
	lines, err := ReadLedger(dir)
	if err != nil || len(lines) != 2 {
		t.Fatalf("ledger records: %d %v", len(lines), err)
	}
	if err := Validate(lines[1]); err != nil {
		t.Fatalf("invalid submission: %v", err)
	}
	assertStoredBytes(t, target, data)
}

func TestPanelSubmit_HardBlockRecord(t *testing.T) {
	target, data, resolve := submitFixture(t)
	var verdict VerdictV2
	if err := json.Unmarshal(data, &verdict); err != nil {
		t.Fatal(err)
	}
	verdict.HardBlock = true
	data, err := json.Marshal(verdict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Submit(target, data, resolve); err != nil {
		t.Fatal(err)
	}
	lines, err := ReadLedger(target.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &record); err != nil {
		t.Fatal(err)
	}
	if record["hard_block"] != true {
		t.Fatalf("submission lost hard_block: %s", lines[len(lines)-1])
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
	t.Run("paths", testSubmitAttemptPathBinding)
	target, b, resolve := submitFixture(t)
	seedTestRecords(t, target.Dir, submitLaunch(1, 1, "dispatched"))
	target.File = filepath.Join(target.SlotDir, "attempt-1", "verdict.json")
	if err := os.MkdirAll(filepath.Dir(target.File), 0o700); err != nil {
		t.Fatal(err)
	}
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

func testSubmitAttemptPathBinding(t *testing.T) {
	for _, name := range []string{"symlink", "outside slot", "nested outside slot", "missing directory", "no slot", "missing slot", "symlink outside slot"} {
		t.Run(name, func(t *testing.T) {
			target, b, resolve := submitFixture(t)
			seedTestRecords(t, target.Dir, submitLaunch(1, 1, "dispatched"), submitLaunch(1, 1, "launch_failed"), submitLaunch(1, 2, "dispatched"))
			if err := os.MkdirAll(filepath.Join(target.SlotDir, "attempt-1"), 0o700); err != nil {
				t.Fatal(err)
			}
			realSlot, err := filepath.EvalSymlinks(target.SlotDir)
			if err != nil {
				t.Fatal(err)
			}
			target.SlotDir = realSlot
			target.File = filepath.Join(realSlot, "attempt-1", "verdict.json")
			want := "cannot bind attempt path"
			switch name {
			case "symlink":
				alias := filepath.Join(t.TempDir(), "slot-link")
				if err := os.Symlink(realSlot, alias); err != nil {
					t.Fatal(err)
				}
				target.File = filepath.Join(alias, "attempt-1", "verdict.json")
				want = "attempt 1 was superseded by 2"
			case "outside slot":
				target.SlotDir = t.TempDir()
			case "nested outside slot":
				target.SlotDir = t.TempDir()
				target.File = filepath.Join(realSlot, "attempt-1", "nested", "verdict.json")
				if err := os.MkdirAll(filepath.Dir(target.File), 0o700); err != nil {
					t.Fatal(err)
				}
			case "missing directory":
				target.File = filepath.Join(realSlot, "attempt-9", "verdict.json")
			case "no slot":
				target.SlotDir = ""
			case "missing slot":
				target.SlotDir = filepath.Join(t.TempDir(), "missing")
			case "symlink outside slot":
				alias := filepath.Join(realSlot, "attempt-3")
				if err := os.Symlink(t.TempDir(), alias); err != nil {
					t.Fatal(err)
				}
				target.File = filepath.Join(alias, "verdict.json")
			}
			if name != "missing directory" {
				if err := os.WriteFile(target.File, b, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(filepath.Join(target.Dir, dispositionsFileName))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Submit(target, b, resolve); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("attempt path refusal: want %q, got %v", want, err)
			}
			after, err := os.ReadFile(filepath.Join(target.Dir, dispositionsFileName))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("attempt path refusal mutated ledger: %v", err)
			}
		})
	}
	t.Run("nested symlink outside slot", func(t *testing.T) {
		target, b, resolve := submitFixture(t)
		outside := t.TempDir()
		if err := os.MkdirAll(filepath.Join(outside, "sub"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(target.SlotDir, 0o700); err != nil {
			t.Fatal(err)
		}
		alias := filepath.Join(target.SlotDir, "attempt-1")
		if err := os.Symlink(outside, alias); err != nil {
			t.Skipf("cannot create symlink: %v", err)
		}
		target.File = filepath.Join(alias, "sub", "verdict.json")
		if err := os.WriteFile(target.File, b, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Submit(target, b, resolve); err == nil || !strings.Contains(err.Error(), "cannot bind attempt path") {
			t.Fatalf("attempt path refusal: want %q, got %v", "cannot bind attempt path", err)
		}
	})
}

func TestPanelSubmit_AttemptNamedAncestor(t *testing.T) {
	for _, name := range []string{"slot", "no slot", "missing slot"} {
		t.Run(name, func(t *testing.T) {
			target, b, resolve := submitFixture(t)
			target.SlotDir = filepath.Join(t.TempDir(), "attempt-1", "p", "r1", "R1")
			target.File = filepath.Join(target.SlotDir, "verdict.json")
			if err := os.MkdirAll(target.SlotDir, 0o700); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "no slot":
				target.SlotDir = ""
			case "missing slot":
				target.SlotDir = filepath.Join(t.TempDir(), "missing")
			}
			if _, err := Submit(target, b, resolve); err != nil {
				t.Fatalf("valid non-attempt submission refused: %v", err)
			}
			assertStoredBytes(t, target, b)
		})
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
	t.Run("terminal safe", func(t *testing.T) {
		want := "\"message\\x1b[31m\"\nrecovery: retry\\x1b[0m"
		if got := submitRefusal("message\x1b[31m", "retry\x1b[0m").Error(); got != want {
			t.Fatalf("unsafe refusal: want %q, got %q", want, got)
		}
	})
	for _, name := range []string{"process", "landed", "non-v2", "sha", "model", "round", "slot", "not dispatched", "ended", "abandoned"} {
		t.Run(name, func(t *testing.T) {
			target, b, resolve := submitFixture(t)
			var v VerdictV2
			_ = json.Unmarshal(b, &v)
			wantRecovery := ""
			switch name {
			case "process":
				wantRecovery = "recovery: mindspec panel launch or mindspec panel recover"
				resolve = func(p Panel, s RosterSlot) (SubmitResolution, error) {
					return SubmitResolution{Family: s.Family, Model: s.Model, RunnerKind: "process", RunnerSource: "default"}, nil
				}
			case "landed":
				wantRecovery = "recovery: mindspec panel create 'p' --round 2"
				resolve = func(p Panel, s RosterSlot) (SubmitResolution, error) {
					return SubmitResolution{Family: s.Family, Model: s.Model, RunnerKind: "harness", RunnerSource: "default", Landed: true}, nil
				}
			case "non-v2":
				p := load(target.Dir).Panel
				p.Round = 2
				target.Round = 2
				v.Round = 2
				if err := WriteSnapshot(target.Dir, p); err != nil {
					t.Fatal(err)
				}
				seedTestRecords(t, target.Dir, RoundRecord{Record: RecordRound, Spec: p.Spec, Panel: "p", Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target, Gate: p.Gate, Roster: p.Roster, ExpectedReviewers: 2, Telemetry: p.Telemetry, CreatedAt: ledgerTime})
				wantRecovery = "recovery: mindspec panel create 'p' --round 3"
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
				wantRecovery = "recovery: mindspec panel checkout --slot 'R1' --dispatch --replace"
				target.File = filepath.Join(target.SlotDir, "attempt-1", "v.json")
				seedTestRecords(t, target.Dir, submitLaunch(1, 1, "dispatched"), submitLaunch(1, 1, "launch_failed"))
			case "abandoned":
				p := load(target.Dir).Panel
				p.Abandoned = true
				if err := WriteSnapshot(target.Dir, p); err != nil {
					t.Fatal(err)
				}
			}
			if target.File != "" {
				if err := os.MkdirAll(filepath.Dir(target.File), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			b, _ = json.Marshal(v)
			before, err := os.ReadFile(filepath.Join(target.Dir, dispositionsFileName))
			if err != nil {
				t.Fatal(err)
			}
			_, err = Submit(target, b, resolve)
			if err == nil {
				t.Fatal("accepted forbidden submit")
			}
			lines := strings.Split(err.Error(), "\n")
			if strings.Count(err.Error(), "recovery:") != 1 {
				t.Fatalf("want one recovery line: %v", err)
			}
			if wantRecovery != "" && (lines[len(lines)-1] != wantRecovery || strings.HasPrefix(err.Error(), "/")) {
				t.Fatalf("want final line %q without pointer prefix, got %q", wantRecovery, err)
			}
			after, _ := os.ReadFile(filepath.Join(target.Dir, dispositionsFileName))
			if !bytes.Equal(before, after) {
				t.Fatal("refusal mutated ledger")
			}
		})
	}
}

func TestPanelSubmit_LegacyRoundBeforeVerdict(t *testing.T) {
	for _, name := range []string{"valid v2", "legacy verdict", "malformed"} {
		t.Run(name, func(t *testing.T) {
			target, b, resolve := submitFixtureSchema(t, 0)
			switch name {
			case "legacy verdict":
				b = []byte(`{"verdict":"APPROVE"}`)
			case "malformed":
				b = []byte(`{`)
			}
			path := filepath.Join(target.Dir, dispositionsFileName)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(before, []byte(`"verdict_schema"`)) {
				t.Fatal("legacy fixture has a schema pin")
			}
			_, err = Submit(target, b, resolve)
			want := "round does not accept submissions\nrecovery: mindspec panel create 'p' --round 2"
			if err == nil || err.Error() != want {
				t.Fatalf("legacy round refusal: want %q, got %v", want, err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("legacy refusal mutated ledger: %v", err)
			}
			if _, err := os.Stat(filepath.Join(target.Dir, "R1-round-1.json")); !os.IsNotExist(err) {
				t.Fatalf("legacy refusal wrote verdict: %v", err)
			}
		})
	}
}

func assertRecoveryArgs(t *testing.T, recovery string, want []string) {
	t.Helper()
	const prefix = "mindspec panel create "
	if !strings.HasPrefix(recovery, prefix) {
		t.Fatalf("missing create command: %q", recovery)
	}
	script := `capture() { printf '%s\000' "$@"; }; capture ` + strings.TrimPrefix(recovery, prefix)
	got, err := exec.Command("/bin/sh", "-c", script).CombinedOutput()
	expected := strings.Join(want, "\x00") + "\x00"
	if err != nil || string(got) != expected {
		t.Fatalf("recovery=%q got=%q want=%q shell error=%v", recovery, got, expected, err)
	}
}

func TestPanelSubmit_RecoveryPreservesSlug(t *testing.T) {
	for _, slug := range []string{"panel review", "panel's review", "p;printf INJECTED;#"} {
		t.Run(slug, func(t *testing.T) {
			target, raw, resolve := submitFixtureSchema(t, 0)
			dir := filepath.Join(filepath.Dir(target.Dir), slug)
			if err := os.Rename(target.Dir, dir); err != nil {
				t.Fatal(err)
			}
			target.Dir, target.Panel = dir, slug
			_, err := Submit(target, raw, resolve)
			if err == nil {
				t.Fatal("historical submit must refuse")
			}
			_, recovery, ok := strings.Cut(err.Error(), "\nrecovery: ")
			if !ok {
				t.Fatal(err)
			}
			assertRecoveryArgs(t, recovery, []string{slug, "--round", "2"})
		})
	}
}

func TestPanelSubmit_IntegrityRecoveryPreservesArguments(t *testing.T) {
	for _, slug := range []string{"panel review", "panel's review", "p;printf INJECTED;#"} {
		t.Run(slug, func(t *testing.T) {
			spec := "spec;printf INJECTED;#"
			r := &Result{Dir: filepath.Join(t.TempDir(), slug), Panel: &Panel{Spec: spec, Round: 1, ExpectedReviewers: 1}}
			err := r.intactError()
			if err == nil {
				t.Fatal("invalid reviewer count must refuse")
			}
			_, recovery, ok := strings.Cut(err.Error(), "recovery: ")
			if !ok {
				t.Fatal(err)
			}
			assertRecoveryArgs(t, recovery, []string{slug, "--spec", spec, "--round", "2", "--reviewers", "2"})
		})
	}
}

func TestPanelSubmit_ReplacementRecoveryPreservesSlot(t *testing.T) {
	for _, slot := range []string{"panel review", "panel's review", "p;printf INJECTED;#"} {
		t.Run(slot, func(t *testing.T) {
			target, raw, resolve := submitFixture(t)
			p := load(target.Dir).Panel
			p.Roster[0].Slot = slot
			if err := WriteSnapshot(target.Dir, p); err != nil {
				t.Fatal(err)
			}
			target.Slot = slot
			target.File = filepath.Join(target.SlotDir, "attempt-1", "verdict.json")
			if err := os.MkdirAll(filepath.Dir(target.File), 0o700); err != nil {
				t.Fatal(err)
			}
			var v VerdictV2
			if err := json.Unmarshal(raw, &v); err != nil {
				t.Fatal(err)
			}
			v.Slot = slot
			raw, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Submit(target, raw, resolve); err == nil {
				t.Fatal("undispatched attempt must refuse")
			} else {
				_, recovery, ok := strings.Cut(err.Error(), "\nrecovery: ")
				const prefix = "mindspec panel checkout "
				if !ok || !strings.HasPrefix(recovery, prefix) {
					t.Fatalf("missing replacement recovery: %v", err)
				}
				got, shellErr := exec.Command("/bin/sh", "-c", `capture() { printf '%s\000' "$@"; }; capture `+strings.TrimPrefix(recovery, prefix)).CombinedOutput()
				want := "--slot\x00" + slot + "\x00--dispatch\x00--replace\x00"
				if shellErr != nil || string(got) != want {
					t.Fatalf("recovery=%q got=%q want=%q error=%v", recovery, got, want, shellErr)
				}
			}
		})
	}
}
