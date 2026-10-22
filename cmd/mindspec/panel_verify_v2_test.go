package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func verifyV2(t *testing.T, f cliV2Fixture, asJSON bool) (string, int) {
	t.Helper()
	before := panelTreeBytes(t, f.root)
	for _, name := range []string{"spec", "json"} {
		flag := panelVerifyCmd.Flags().Lookup(name)
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	}
	if asJSON {
		_ = panelVerifyCmd.Flags().Set("json", "true")
	}
	var out bytes.Buffer
	panelVerifyCmd.SetOut(&out)
	t.Cleanup(func() {
		panelVerifyCmd.SetOut(nil)
		_ = panelVerifyCmd.Flags().Set("json", "false")
		panelVerifyCmd.Flags().Lookup("json").Changed = false
	})
	err := panelVerifyCmd.RunE(panelVerifyCmd, []string{filepath.Base(f.dir)})
	if after := panelTreeBytes(t, f.root); !reflect.DeepEqual(before, after) {
		t.Fatal("verify changed bytes or file paths")
	}
	return out.String(), otelExitCode(err)
}
func verifySlotV2(t *testing.T, f cliV2Fixture, want string, code int) {
	t.Helper()
	out, got := verifyV2(t, f, false)
	if got != code || !strings.Contains(out, "R1 "+want+" ") || !strings.Contains(out, "rule: PASS iff") || !strings.Contains(out, "PASS") && !strings.Contains(out, "BLOCK") {
		t.Fatalf("want R1 %s exit %d, got exit %d:\n%s", want, code, got, out)
	}
}
func completeOtherV2(t *testing.T, f cliV2Fixture) {
	t.Helper()
	v := f.verdict()
	v.Slot = "R2"
	f.submit(t, v)
}
func seedVerifyStateV2(t *testing.T, f cliV2Fixture, state string) {
	t.Helper()
	v := f.verdict()
	switch state {
	case "missing":
	case "final":
		f.submit(t, v)
	case "damaged":
		f.submit(t, v)
		rebindWrite(t, f.dir, "R1-round-1.json", `{"verdict":"REJECT"}`)
	case "void":
		v.Verdict = "VOID"
		reason := "cannot review"
		v.VoidReason = &reason
		f.submit(t, v)
	case "draft":
		v.Status = "draft"
		f.submit(t, v)
	case "running":
		f.launch(t, "dispatched", 1)
	case "unsubmitted":
		rebindWrite(t, f.dir, "R1-round-1.json", `{"verdict":"APPROVE"}`)
	case "quota_wall", "launch_failed", "malformed":
		f.launch(t, "dispatched", 1)
		f.launch(t, state, 1)
	default:
		t.Fatalf("unknown test status %s", state)
	}
}
func TestPanelVerify_SlotStatus(t *testing.T) {
	for _, state := range []string{"final", "damaged", "void", "draft", "running", "quota_wall", "launch_failed", "malformed", "unsubmitted", "missing"} {
		t.Run(state, func(t *testing.T) {
			f := newCLIV2(t)
			completeOtherV2(t, f)
			seedVerifyStateV2(t, f, state)
			code := 2
			if state == "final" {
				code = 0
			}
			if state == "running" || state == "missing" || state == "draft" {
				code = 1
			}
			verifySlotV2(t, f, state, code)
		})
	}
}
func TestPanelVerify_DispatchedUnsubmitted(t *testing.T) {
	f := newCLIV2(t)
	completeOtherV2(t, f)
	f.launch(t, "dispatched", 1)
	rebindWrite(t, f.dir, "R1-round-1.json", `{"verdict":"APPROVE"}`)
	verifySlotV2(t, f, "unsubmitted", 2)
}
func TestPanelVerify_HarnessTimeout(t *testing.T) {
	f := newCLIV2(t)
	completeOtherV2(t, f)
	rebindWrite(t, f.root, ".mindspec/config.local.yaml", "panel:\n  runners: [{family: harness, kind: harness, timeout_minutes: 1}]\n")
	r := panel.LaunchRecord{Record: panel.RecordLaunch, Spec: f.p.Spec, Panel: "p", Slot: "R1", Round: 1, Attempt: 1, Family: "harness", Model: "m", Status: "dispatched", RunnerKind: "harness", RunnerSource: "default", CreatedAt: time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano)}
	paneltest.Seed(t, f.dir, r)
	verifySlotV2(t, f, "launch_failed", 2)
}
func TestPanelVerify_ExitCodes(t *testing.T) {
	for _, state := range []string{"final", "running", "launch_failed"} {
		t.Run(state, func(t *testing.T) {
			f := newCLIV2(t)
			completeOtherV2(t, f)
			seedVerifyStateV2(t, f, state)
			code := map[string]int{"final": 0, "running": 1, "launch_failed": 2}[state]
			_, got := verifyV2(t, f, false)
			if got != code {
				t.Fatalf("exit %d want %d", got, code)
			}
		})
	}
	t.Run("pending_precedes_failure", func(t *testing.T) {
		f := newCLIV2(t)
		seedVerifyStateV2(t, f, "launch_failed")
		_, code := verifyV2(t, f, false)
		if code != 1 {
			t.Fatalf("missing slot should take precedence: %d", code)
		}
	})
}

func TestPanelVerify_DamagedRegistration(t *testing.T) {
	for _, state := range []string{"malformed", "downgraded"} {
		t.Run(state, func(t *testing.T) {
			f := newCLIV2(t)
			if state == "malformed" {
				rebindWrite(t, f.dir, "panel.json", "{")
			} else {
				paneltest.Seed(t, f.dir, panel.RoundRecord{Record: panel.RecordRound, Spec: f.p.Spec, Panel: "p", Round: 2, ReviewedHeadSHA: f.p.ReviewedHeadSHA, SubjectDigest: f.p.SubjectDigest, Target: f.p.Target, Gate: f.p.Gate, Roster: f.p.Roster, ExpectedReviewers: 2, Telemetry: "required", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
			}
			out, code := verifyV2(t, f, false)
			if code != 2 || !strings.Contains(out, "panel directory:") || !strings.Contains(out, "resolved round:") || !strings.Contains(out, "rule: PASS iff") || !strings.Contains(out, "BLOCK:") {
				t.Fatalf("want BLOCK preview and exit 2, got exit %d:\n%s", code, out)
			}
		})
	}
}

func TestPanelTally_DamagedRegistration(t *testing.T) {
	f := newCLIV2(t)
	rebindWrite(t, f.dir, "panel.json", "{")
	out, err := runPanelVerbCmd("tally", "p")
	if err == nil || !strings.Contains(out, "decision: BLOCK") || strings.Count(out, "recovery:") != 1 {
		t.Fatalf("want BLOCK decision and one recovery line, got %v:\n%s", err, out)
	}
}
func TestPanelVerify_V1Round(t *testing.T) {
	f := seedCLIV1(t, newCLIV2(t))
	for _, slot := range []string{"R1", "R2"} {
		rebindWrite(t, f.dir, slot+"-round-1.json", `{"verdict":"APPROVE"}`)
	}
	verifySlotV2(t, f, "final", 0)
	rebindWrite(t, f.dir, "R1-round-1.json", "{")
	verifySlotV2(t, f, "malformed", 2)
}
func TestPanelVerify_JSONRoster(t *testing.T) {
	f := newCLIV2(t)
	completeOtherV2(t, f)
	f.submit(t, f.verdict())
	text, code := verifyV2(t, f, true)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, text)
	}
	var slots []panel.SlotStatus
	if err := json.Unmarshal([]byte(text), &slots); err != nil {
		t.Fatal(err)
	}
	if len(slots) != 2 {
		t.Fatalf("slots %+v", slots)
	}
	for i, s := range slots {
		if s.Slot != fmt.Sprintf("R%d", i+1) || s.Family != "harness" || s.Model != "m" || s.Lens != f.p.Roster[i].Lens || s.Kind != "harness" || s.Status != "final" || s.HandoffPending || s.Annotations == nil {
			t.Fatalf("roster %+v", slots)
		}
	}
}
func TestPanelVerify_Replaced(t *testing.T) {
	f := newCLIV2(t)
	completeOtherV2(t, f)
	v := f.verdict()
	v.Verdict = "REQUEST_CHANGES"
	v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "fix", Basis: "traced"}}
	f.submit(t, v)
	v.Verdict = "APPROVE"
	v.ConcreteChangesRequired = []panel.ChangeRequired{}
	f.submit(t, v)
	for _, rationale := range []string{"reviewed", "reviewed again"} {
		v.Rationale = rationale
		f.submit(t, v)
		out, code := verifyV2(t, f, false)
		if code != 0 || strings.Count(out, "replaced REQUEST_CHANGES->APPROVE") != 1 {
			t.Fatalf("replacement lost: %d %s", code, out)
		}
	}
	out, err := runPanelVerbCmd("tally", "p")
	if err != nil || strings.Count(out, "replaced REQUEST_CHANGES->APPROVE") != 1 {
		t.Fatalf("tally replacement: %v %s", err, out)
	}
}

func TestPanelVerify_ReplacedHardBlock(t *testing.T) {
	f := newCLIV2(t)
	completeOtherV2(t, f)
	v := f.verdict()
	v.HardBlock = true
	f.submit(t, v)
	v.HardBlock = false
	f.submit(t, v)
	out, code := verifyV2(t, f, false)
	if code != 0 || strings.Count(out, "replaced APPROVE->APPROVE") != 1 {
		t.Fatalf("hard block history lost: %d %s", code, out)
	}
	out, err := runPanelVerbCmd("tally", "p")
	if err != nil || strings.Count(out, "replaced APPROVE->APPROVE") != 1 {
		t.Fatalf("tally hard block history: %v %s", err, out)
	}
	v.HardBlock = true
	f.submit(t, v)
	rebindWrite(t, f.dir, "R1-round-1.json", `{"verdict":"APPROVE","hard_block":false}`)
	out, code = verifyV2(t, f, false)
	if code != 2 || !strings.Contains(out, "R1 damaged ") || strings.Contains(out, "replaced APPROVE->APPROVE") {
		t.Fatalf("replacement annotation used file instead of record: %d %s", code, out)
	}
}

func seededStartedV2(t *testing.T, f cliV2Fixture) {
	t.Helper()
	completeOtherV2(t, f)
	rebindWrite(t, f.root, ".mindspec/config.local.yaml", "panel:\n  runners: [{family: harness, kind: process, argv: [fake], prompt: stdin, output: stdout}]\n")
	paneltest.Seed(t, f.dir, panel.LaunchRecord{Record: panel.RecordLaunch, Spec: f.p.Spec, Panel: "p", Slot: "R1", Round: 1, Attempt: 1, Family: "harness", Model: "m", Status: "started", RunnerKind: "process", RunnerSource: "config.local.yaml", PGID: 12345, ArgvSHA256: strings.Repeat("a", 64), NonceSHA256: strings.Repeat("b", 64), CreatedAt: paneltest.CreatedAt})
}

func TestPanelVerify_KilledLaunchLiveSupervisor(t *testing.T) {
	f := newCLIV2(t)
	seededStartedV2(t, f)
	lock, err := panel.AcquireSlotLock(filepath.Join(f.layout.AttemptDir(1), "run.lock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lock.Release(); err != nil {
			t.Error(err)
		}
	})
	if held, err := panel.ProbeSlotLock(f.layout.Lock); err != nil || held {
		t.Fatalf("slot lock must be free: held=%v %v", held, err)
	}
	verifySlotV2(t, f, "running", 1)
}

func TestPanelVerify_KilledLaunchBothLocksFree(t *testing.T) {
	f := newCLIV2(t)
	seededStartedV2(t, f)
	for _, path := range []string{f.layout.Lock, filepath.Join(f.layout.AttemptDir(1), "run.lock")} {
		lock, err := panel.AcquireSlotLock(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := lock.Release(); err != nil {
			t.Fatal(err)
		}
		if held, err := panel.ProbeSlotLock(path); err != nil || held {
			t.Fatalf("lock must be free: held=%v %v", held, err)
		}
	}
	verifySlotV2(t, f, "launch_failed", 2)
}
func TestPanelVerify_AnnotationsFromRecords(t *testing.T) {
	t.Run("local_source_stays_recorded", func(t *testing.T) {
		f := newCLIV2(t)
		completeOtherV2(t, f)
		rebindWrite(t, f.root, ".mindspec/config.local.yaml", "panel:\n  runners: [{family: harness, kind: harness}]\n")
		if out, err := runPanelVerbCmd(f.args(f.file(t, f.verdict()))...); err != nil {
			t.Fatalf("submit: %v %s", err, out)
		}
		rebindWrite(t, f.root, ".mindspec/config.local.yaml", "models: {}\n")
		out, code := verifyV2(t, f, false)
		if code != 0 || !strings.Contains(out, "local runner") {
			t.Fatalf("local record lost: %d %s", code, out)
		}
	})
	t.Run("no_retroactive_local_source", func(t *testing.T) {
		f := newCLIV2(t)
		completeOtherV2(t, f)
		f.submit(t, f.verdict())
		rebindWrite(t, f.root, ".mindspec/config.local.yaml", "panel:\n  runners: [{family: harness, kind: harness}]\n")
		out, code := verifyV2(t, f, false)
		if code != 0 || strings.Contains(out, "local runner") {
			t.Fatalf("source inferred from current config: %d %s", code, out)
		}
	})
	t.Run("direct_on_current_process", func(t *testing.T) {
		f := newCLIV2(t)
		completeOtherV2(t, f)
		f.submit(t, f.verdict())
		rebindWrite(t, f.root, ".mindspec/config.local.yaml", "panel:\n  runners: [{family: harness, kind: process, argv: [fake], prompt: stdin, output: stdout}]\n")
		out, code := verifyV2(t, f, false)
		if code != 0 || strings.Count(out, "direct on process family") != 2 {
			t.Fatalf("direct annotation: %d %s", code, out)
		}
	})
	t.Run("substitution_and_handoff", func(t *testing.T) {
		f := newCLIV2(t)
		completeOtherV2(t, f)
		rebindWrite(t, f.root, ".mindspec/config.local.yaml", "panel:\n  runners: [{family: other, kind: harness}]\n")
		f.launch(t, "dispatched", 1)
		f.launch(t, "quota_wall", 1)
		paneltest.Seed(t, f.dir, panel.SubstitutionRecord{Record: panel.RecordSubstitution, Spec: f.p.Spec, Panel: "p", Slot: "R1", Round: 1, FromFamily: "harness", ToFamily: "other", Model: "other-model", CreatedAt: paneltest.CreatedAt})
		out, code := verifyV2(t, f, true)
		var slots []panel.SlotStatus
		if err := json.Unmarshal([]byte(out), &slots); err != nil {
			t.Fatal(err)
		}
		if code != 1 || slots[0].Family != "other" || slots[0].Model != "other-model" || slots[0].Kind != "harness" || !slots[0].HandoffPending || !reflect.DeepEqual(slots[0].Annotations, []string{"substituted harness->other"}) {
			t.Fatalf("handoff: %d %s", code, out)
		}
		f.launch(t, "dispatched", 2)
		out, _ = verifyV2(t, f, true)
		if err := json.Unmarshal([]byte(out), &slots); err != nil {
			t.Fatal(err)
		}
		if slots[0].HandoffPending || slots[0].Status != "running" {
			t.Fatalf("handoff not dispatched: %s", out)
		}
	})
}
func TestPanelVerify_Termsafe(t *testing.T) {
	f := newCLIV2(t)
	completeOtherV2(t, f)
	f.p.Roster[0].Model = "bad\x1b[31m"
	f.p.VerdictSchema = 0
	// Historical snapshots can carry reviewer strings that predate model checks.
	f.dir = filepath.Join(f.root, "review", "unsafe")
	f.p.Telemetry = ""
	if err := panel.WriteSnapshot(f.dir, f.p); err != nil {
		t.Fatal(err)
	}
	rebindWrite(t, f.dir, "R1-round-1.json", `{"verdict":"APPROVE","model":"bad\u001b[31m"}`)
	rebindWrite(t, f.dir, "R2-round-1.json", `{"verdict":"APPROVE"}`)
	out, code := verifyV2(t, f, false)
	if code != 0 || strings.ContainsRune(out, '\x1b') || !strings.Contains(out, `\x1b`) {
		t.Fatalf("unsafe output: exit %d %q", code, out)
	}
}
func TestPanelVerify_ReplacementInFlight(t *testing.T) {
	f := newCLIV2(t)
	completeOtherV2(t, f)
	v := f.verdict()
	f.submit(t, v)
	f.launch(t, "dispatched", 1)
	verifySlotV2(t, f, "running", 1)
	res, err := panel.Tally(f.dir)
	if err != nil || res.PassRule(true) {
		t.Fatalf("replacement counted: %+v %v", res, err)
	}
	f.launch(t, "launch_failed", 1)
	verifySlotV2(t, f, "launch_failed", 2)
	f.launch(t, "dispatched", 2)
	f.submit(t, v)
	verifySlotV2(t, f, "final", 0)
	res, err = panel.Tally(f.dir)
	if err != nil || !res.PassRule(true) {
		t.Fatalf("same-final repair not closed: %+v %v", res, err)
	}
}
