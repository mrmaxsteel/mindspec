//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/panellaunch"
	"github.com/spf13/cobra"
)

type launchFixture struct {
	slotFixture
	script string
	v      panel.VerdictV2
}

func newLaunchFixture(t *testing.T) launchFixture {
	t.Helper()
	for _, name := range []string{"slot", "all", "fixer", "prune", "dispatch", "replace", "spec"} {
		flag := panelCheckoutCmd.Flags().Lookup(name)
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	}
	f := newCheckoutFixture(t, ".mindspec-scratch")
	f.dir = filepath.Join(f.root, "review", "run")
	in := panel.CreateInput{Target: "main", Gate: "adhoc", Round: 1, ReviewedHeadSHA: f.p.ReviewedHeadSHA, SubjectDigest: f.p.SubjectDigest, Roster: []panel.RosterSlot{{Slot: "R1", Family: "a", Model: "m", Lens: "adversarial"}, {Slot: "R2", Family: "harness", Model: "m", Lens: "adversarial"}}}
	if err := panel.Create(f.dir, in); err != nil {
		t.Fatal(err)
	}
	res, err := panel.Tally(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	f.p = *res.Panel
	v := paneltest.Verdict("APPROVE")
	v.ReviewedSHA = f.p.ReviewedHeadSHA
	fake := filepath.Join(f.root, "fake.sh")
	l := launchFixture{f, fake, v}
	l.configure(t, "stdout", "", "")
	l.runner(t, "", 0)
	other := v
	other.Slot = "R2"
	if _, err := panel.Submit(panel.SubmitTarget{Dir: f.dir, Panel: "run", Slot: "R2", Round: 1}, paneltest.Bytes(t, other), paneltest.Resolve); err != nil {
		t.Fatal(err)
	}
	return l
}
func (f launchFixture) layout(t *testing.T) panel.SlotLayout {
	t.Helper()
	l, e := panel.NewSlotLayout(f.root, f.isolation, "", "run", 1, "R1")
	if e != nil {
		t.Fatal(e)
	}
	return l
}
func (f launchFixture) configure(t *testing.T, output, quota, to string) {
	t.Helper()
	r := config.Runner{Family: "a", Kind: "process", Argv: []string{f.script}, Prompt: "stdin", Output: output, OnQuota: to}
	if output == "file" {
		r.Prompt = "file"
		r.Argv = append(r.Argv, "{prompt_file}")
	}
	if quota != "" {
		r.QuotaSignals = []string{quota}
	}
	runners := []config.Runner{r}
	if to != "" {
		runners = append(runners, config.Runner{Family: to, Kind: "harness"})
	}
	raw, e := json.Marshal(map[string]interface{}{"panel": map[string]interface{}{"runners": runners}})
	if e != nil {
		t.Fatal(e)
	}
	slotWrite(t, filepath.Join(f.root, ".mindspec", "config.local.yaml"), string(raw))
	config.ResetCache()
}
func (f launchFixture) scriptBody(t *testing.T, body string) {
	t.Helper()
	slotWrite(t, f.script, "#!/bin/sh\nset -eu\n"+body+"\n")
	if err := os.Chmod(f.script, 0o700); err != nil {
		t.Fatal(err)
	}
}
func (f launchFixture) runner(t *testing.T, prefix string, exit int) {
	t.Helper()
	raw := string(paneltest.Bytes(t, f.v))
	f.scriptBody(t, `if [ "$#" -gt 0 ]; then prompt=$(cat "$1"); else prompt=$(cat); fi
nonce=$(printf '%s\n' "$prompt" | sed -n 's/^MINDSPEC-VERDICT-BEGIN //p')
`+prefix+"\n"+`if [ "$#" -gt 0 ]; then
cat > "$(dirname "$1")/verdict.json" <<'VERDICT'
`+raw+`
VERDICT
else
printf 'MINDSPEC-VERDICT-BEGIN %s\n' "$nonce"
cat <<'VERDICT'
`+raw+`
VERDICT
printf 'MINDSPEC-VERDICT-END %s\n' "$nonce"
fi
exit `+fmt.Sprint(exit))
}
func launchRun(t *testing.T, command *cobra.Command, args ...string) (string, error) {
	t.Helper()
	for _, name := range []string{"spec", "replace", "reason"} {
		if flag := command.Flags().Lookup(name); flag != nil {
			_ = flag.Value.Set(flag.DefValue)
			flag.Changed = false
		}
	}
	if command == panelLaunchCmd && len(args) > 0 && args[len(args)-1] == "--replace" {
		_ = command.Flags().Set("replace", "true")
		args = args[:len(args)-1]
	}
	if command == panelRecoverCmd {
		_ = command.Flags().Set("reason", "test recovery")
	}
	var out bytes.Buffer
	command.SetOut(&out)
	defer command.SetOut(nil)
	err := command.RunE(command, args)
	return out.String(), err
}
func (f launchFixture) run(t *testing.T, replace bool) (string, error) {
	t.Helper()
	args := []string{"run", "R1"}
	if replace {
		args = append(args, "--replace")
	}
	return launchRun(t, panelLaunchCmd, args...)
}
func (f launchFixture) status(t *testing.T, want string) {
	t.Helper()
	res, e := panel.Tally(f.dir)
	if e != nil {
		t.Fatal(e)
	}
	s, e := panelSlotStatuses(f.root, res)
	if e != nil {
		t.Fatal(e)
	}
	if s[0].Status != want {
		t.Fatalf("status=%+v want %s", s, want)
	}
	if want == "final" && (!res.PassRule(true) || panel.VerifyExitCode(s) != 0) {
		t.Fatalf("final did not pass: %+v %+v", res, s)
	}
	if want != "final" && res.PassRule(true) {
		t.Fatalf("nonfinal passed: %+v", res)
	}
}
func (f launchFixture) events(t *testing.T) []panel.SlotAttempt {
	t.Helper()
	e, err := panel.SlotAttempts(f.dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	return e["R1"]
}
func (f launchFixture) latest(t *testing.T) panel.SubmissionRecord {
	t.Helper()
	s, e := panel.LatestSubmissions(f.dir, 1)
	if e != nil {
		t.Fatal(e)
	}
	return s["R1"]
}

func TestPanelLaunch_Outcomes(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		f := newLaunchFixture(t)
		f.runner(t, "sleep 0.2", 0)
		original := panelLaunchProcessFn
		panelLaunchProcessFn = func(in panellaunch.Request) error { in.Timeout = 40 * time.Millisecond; return panellaunch.Run(in) }
		defer func() { panelLaunchProcessFn = original }()
		if _, err := f.run(t, false); err == nil {
			t.Fatal("timed out reviewer succeeded")
		}
		f.status(t, "launch_failed")
	})
	for _, output := range []string{"stdout", "file"} {
		t.Run(output, func(t *testing.T) {
			f := newLaunchFixture(t)
			f.configure(t, output, "", "")
			out, e := f.run(t, false)
			if e != nil {
				t.Fatalf("%v %s", e, out)
			}
			f.status(t, "final")
			s := f.latest(t)
			if s.Via != "launch" || s.Attempt != 1 {
				t.Fatalf("submission=%+v", s)
			}
			events := f.events(t)
			if len(events) != 2 || events[1].Status != "submitted" || events[1].Seq != s.Seq {
				t.Fatalf("events=%+v", events)
			}
		})
	}
	for _, c := range []struct {
		name, prefix string
		exit         int
		want         string
	}{{"exit1", "", 1, "launch_failed"}, {"quota", "printf 'LIMIT' >&2", 2, "quota_wall"}, {"stdout_quota", "printf 'LIMIT'", 2, "launch_failed"}} {
		t.Run(c.name, func(t *testing.T) {
			f := newLaunchFixture(t)
			f.configure(t, "stdout", "LIMIT", "")
			f.runner(t, c.prefix, c.exit)
			if out, e := f.run(t, false); e == nil {
				t.Fatalf("unexpected success %s", out)
			}
			f.status(t, c.want)
			if len(f.events(t)) != 2 {
				t.Fatal("unexpected retry")
			}
		})
	}
}
func TestPanelLaunch_Channels(t *testing.T) {
	for _, mode := range []string{"stderr", "nonce", "double"} {
		t.Run(mode, func(t *testing.T) {
			f := newLaunchFixture(t)
			body := ""
			switch mode {
			case "stderr":
				body = "exec 1>&2"
			case "nonce":
				body = "nonce=wrong"
			case "double":
				body = `printf 'MINDSPEC-VERDICT-BEGIN %s\n{}\nMINDSPEC-VERDICT-END %s\n' "$nonce" "$nonce"`
			}
			f.runner(t, body, 0)
			if _, e := f.run(t, false); e == nil {
				t.Fatal("invalid channel accepted")
			}
			f.status(t, "malformed")
			if len(f.events(t)) != 4 {
				t.Fatal("must retry once")
			}
		})
	}
}
func TestPanelLaunch_Retry(t *testing.T) {
	f := newLaunchFixture(t)
	bad := strings.TrimSuffix(string(paneltest.Bytes(t, f.v)), "}") + `,"bad\u001b":1}`
	f.runner(t, `if [ ! -e retry-marker ]; then
: > retry-marker
printf 'MINDSPEC-VERDICT-BEGIN %s\n' "$nonce"
cat <<'BAD'
`+bad+`
BAD
printf 'MINDSPEC-VERDICT-END %s\n' "$nonce"
exit 0
fi
printf '%s' "$prompt" > retry-prompt`, 0)
	out, e := f.run(t, false)
	if e != nil {
		t.Fatalf("%v %s", e, out)
	}
	if strings.ContainsRune(out, '\x1b') || (!strings.Contains(out, `/bad`) || !strings.Contains(out, `\x1b`)) {
		t.Fatalf("unsafe output: %q", out)
	}
	f.status(t, "final")
	prompt := string(slotRead(t, filepath.Join(f.layout(t).Dir, "retry-prompt")))
	if !strings.Contains(prompt, "Previous output refused:") || (!strings.Contains(prompt, `/bad`) || !strings.Contains(prompt, `\x1b`)) || strings.ContainsRune(prompt, '\x1b') {
		t.Fatalf("retry prompt=%q", prompt)
	}
	if len(f.events(t)) != 4 {
		t.Fatal("not exactly one retry")
	}
}
func TestPanelLaunch_Substitution(t *testing.T) {
	t.Run("harness", func(t *testing.T) {
		f := newLaunchFixture(t)
		f.configure(t, "stdout", "LIMIT", "b")
		f.runner(t, "printf LIMIT >&2", 2)
		out, e := f.run(t, false)
		if e == nil || !strings.Contains(out, "--dispatch") {
			t.Fatalf("handoff %v %s", e, out)
		}
		f.status(t, "quota_wall")
		lines, _ := panel.ReadLedger(f.dir)
		n := 0
		for _, l := range lines {
			var s panel.SubstitutionRecord
			_ = json.Unmarshal(l, &s)
			if s.Record == panel.RecordSubstitution {
				n++
				if s.ToFamily != "b" || s.Model != "b" {
					t.Fatal(s)
				}
			}
		}
		if n != 1 {
			t.Fatalf("substitution count=%d", n)
		}
	})
	t.Run("process", func(t *testing.T) {
		f := newLaunchFixture(t)
		f.configure(t, "file", "LIMIT", "")
		f.runner(t, "printf LIMIT >&2", 2)
		second := filepath.Join(f.root, "second.sh")
		slotWrite(t, second, "#!/bin/sh\nprintf LIMIT >&2\nexit 2\n")
		_ = os.Chmod(second, 0o700)
		a := config.Runner{Family: "a", Kind: "process", Argv: []string{f.script, "{prompt_file}"}, Prompt: "file", Output: "file", QuotaSignals: []string{"LIMIT"}, OnQuota: "b"}
		b := config.Runner{Family: "b", Kind: "process", Argv: []string{second}, Prompt: "stdin", Output: "file", QuotaSignals: []string{"LIMIT"}, OnQuota: "a"}
		raw, _ := json.Marshal(map[string]interface{}{"panel": map[string]interface{}{"runners": []config.Runner{a, b}}})
		slotWrite(t, filepath.Join(f.root, ".mindspec/config.local.yaml"), string(raw))
		config.ResetCache()
		if _, e := f.run(t, false); e == nil {
			t.Fatal("quota unexpectedly succeeded")
		}
		f.status(t, "quota_wall")
		events := f.events(t)
		if len(events) != 4 || events[2].Model != "b" || events[3].VerdictSHA256 != "" || f.latest(t).Seq != 0 {
			t.Fatalf("stale output or extra retry: %+v", events)
		}
	})
}
func TestPanelLaunch_DraftAndVoid(t *testing.T) {
	for _, kind := range []string{"draft", "void"} {
		t.Run(kind, func(t *testing.T) {
			f := newLaunchFixture(t)
			if kind == "draft" {
				f.v.Status = "draft"
			} else {
				f.v.Verdict = "VOID"
				reason := "blocked"
				f.v.VoidReason = &reason
			}
			f.runner(t, "", 0)
			if _, e := f.run(t, false); e == nil {
				t.Fatal("nonfinal succeeded")
			}
			f.status(t, kind)
		})
	}
}
func TestPanelLaunch_IdenticalResubmit(t *testing.T) {
	t.Run("harness_draft", func(t *testing.T) {
		f := newLaunchFixture(t)
		f.v.Status = "draft"
		slotWrite(t, filepath.Join(f.root, ".mindspec/config.local.yaml"), "panel: {runners: [{family: a, kind: harness}]}\n")
		config.ResetCache()
		for k := 1; k <= 2; k++ {
			if out, e := runPanelCmd("checkout", "run", "--slot", "R1", "--dispatch"); e != nil {
				t.Fatalf("dispatch %v %s", e, out)
			}
			file := filepath.Join(f.layout(t).AttemptDir(k), "verdict.json")
			slotWrite(t, file, string(paneltest.Bytes(t, f.v)))
			if out, e := runPanelVerbCmd("submit", "--file", file); e != nil {
				t.Fatalf("submit %v %s", e, out)
			}
			f.status(t, "draft")
		}
		if f.latest(t).Seq != 1 || f.events(t)[3].Seq != 1 {
			t.Fatal("harness draft rerun did not close the new attempt")
		}
	})
	for _, kind := range []string{"draft", "void"} {
		t.Run(kind, func(t *testing.T) {
			f := newLaunchFixture(t)
			if kind == "draft" {
				f.v.Status = "draft"
			} else {
				f.v.Verdict = "VOID"
				r := "blocked"
				f.v.VoidReason = &r
			}
			f.runner(t, "", 0)
			for i := 0; i < 2; i++ {
				if _, e := f.run(t, false); e == nil {
					t.Fatal("nonfinal succeeded")
				}
				f.status(t, kind)
			}
			if f.latest(t).Seq != 1 || f.events(t)[3].Seq != 1 {
				t.Fatal("same bytes did not close attempt")
			}
		})
	}
}
func TestPanelLaunch_VoidRetry(t *testing.T) {
	f := newLaunchFixture(t)
	f.v.Verdict = "VOID"
	r := "blocked"
	f.v.VoidReason = &r
	f.runner(t, "", 0)
	_, _ = f.run(t, false)
	f.status(t, "void")
	f.v.Verdict = "APPROVE"
	f.v.VoidReason = nil
	f.runner(t, "", 0)
	if out, e := f.run(t, false); e != nil {
		t.Fatalf("%v %s", e, out)
	}
	f.status(t, "final")
	if f.events(t)[2].Attempt != 2 {
		t.Fatal("did not start attempt 2")
	}
	if out, e := runPanelVerbCmd("tally", "run"); e != nil {
		t.Fatalf("tally: %v %s", e, out)
	}
}
func TestPanelLaunch_Env(t *testing.T) {
	f := newLaunchFixture(t)
	t.Setenv("MINDSPEC_FAKE_SECRET", "hidden")
	t.Setenv("GIT_INDEX_FILE", "wrong")
	f.runner(t, `if env | sed -n '/^MINDSPEC_/p;/^GIT_INDEX_FILE=/p' | read line; then exit 7; fi`, 0)
	if out, e := f.run(t, false); e != nil {
		t.Fatalf("%v %s", e, out)
	}
	f.status(t, "final")
}
func TestPanelLaunch_Refusals(t *testing.T) {
	f := newLaunchFixture(t)
	if out, e := launchRun(t, panelLaunchCmd, "run", "R2"); e == nil || !strings.Contains(e.Error(), "panel prompt") {
		t.Fatalf("harness %v %s", e, out)
	}
	if _, e := f.run(t, false); e != nil {
		t.Fatal(e)
	}
	before := slotRead(t, filepath.Join(f.dir, "dispositions.jsonl"))
	if _, e := f.run(t, false); e == nil {
		t.Fatal("final relaunched without replace")
	}
	if !bytes.Equal(before, slotRead(t, filepath.Join(f.dir, "dispositions.jsonl"))) {
		t.Fatal("refusal appended")
	}
	l, err := panel.AcquireSlotLock(f.layout(t).Lock)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	if _, e := f.run(t, true); e == nil {
		t.Fatal("concurrent launch accepted")
	}
}
func recoverCase(t *testing.T, output string) {
	f := newLaunchFixture(t)
	f.configure(t, output, "", "")
	f.runner(t, "", 1)
	if _, e := f.run(t, false); e == nil {
		t.Fatal("failed exit accepted")
	}
	f.status(t, "launch_failed")
	out, e := launchRun(t, panelRecoverCmd, "run", "R1")
	if e != nil || !strings.Contains(out, "exit 1 seq 1") {
		t.Fatalf("recover %v %s", e, out)
	}
	f.status(t, "final")
	if s := f.latest(t); s.Via != "recover" || s.Reason != "test recovery" {
		t.Fatal(s)
	}
	name := "stdout.log"
	if output == "file" {
		name = "verdict.json"
	}
	path := filepath.Join(f.layout(t).AttemptDir(1), name)
	slotWrite(t, path, string(slotRead(t, path))+" ")
	if _, e := launchRun(t, panelRecoverCmd, "run", "R1"); e == nil || !strings.Contains(e.Error(), "digest") {
		t.Fatalf("changed output accepted: %v", e)
	}
}
func TestPanelRecover_Stdout(t *testing.T) { recoverCase(t, "stdout") }
func TestPanelRecover_File(t *testing.T)   { recoverCase(t, "file") }
func TestPanelLaunch_IdenticalFinalReplacement(t *testing.T) {
	f := newLaunchFixture(t)
	for i := 0; i < 2; i++ {
		if out, e := f.run(t, i > 0); e != nil {
			t.Fatalf("%v %s", e, out)
		}
		f.status(t, "final")
	}
	if f.latest(t).Seq != 1 || f.events(t)[3].Seq != 1 {
		t.Fatal("identical replacement did not close")
	}
	f.runner(t, "", 1)
	_, _ = f.run(t, true)
	f.status(t, "launch_failed")
}
func TestPanelLaunch_RecoverAfterFailedReplacement(t *testing.T) {
	for _, mode := range []string{"failed", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			f := newLaunchFixture(t)
			if _, e := f.run(t, false); e != nil {
				t.Fatal(e)
			}
			if mode == "failed" {
				f.runner(t, "", 1)
				_, _ = f.run(t, true)
				f.status(t, "launch_failed")
				f.runner(t, "", 0)
			} else {
				f.runner(t, `if [ ! -e malformed-once ]; then : > malformed-once; exit 0; fi`, 0)
			}
			if out, e := f.run(t, true); e != nil {
				t.Fatalf("%v %s", e, out)
			}
			f.status(t, "final")
			if f.latest(t).Seq != 1 {
				t.Fatal("same final appended new verdict")
			}
		})
	}
}
func TestPanelDispatch_IdenticalFinalReplacement(t *testing.T) {
	f := newLaunchFixture(t)
	f.configure(t, "stdout", "", "")
	raw := paneltest.Bytes(t, f.v)
	submit := func() {
		t.Helper()
		if _, e := panel.Submit(panel.SubmitTarget{Dir: f.dir, Panel: "run", Slot: "R1", Round: 1}, raw, paneltest.Resolve); e != nil {
			t.Fatal(e)
		}
	}
	submit()
	slotWrite(t, filepath.Join(f.root, ".mindspec/config.local.yaml"), "panel: {runners: [{family: a, kind: harness}]}\n")
	config.ResetCache()
	for i := 0; i < 2; i++ {
		for _, n := range []string{"slot", "all", "fixer", "prune", "dispatch", "replace", "spec"} {
			flag := panelCheckoutCmd.Flags().Lookup(n)
			_ = flag.Value.Set(flag.DefValue)
			flag.Changed = false
		}
		_, e := runPanelCmd("checkout", "run", "--slot", "R1", "--dispatch", "--replace")
		if e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			f.status(t, "running")
		} else {
			submit()
			f.status(t, "final")
		}
	}
	if f.latest(t).Seq != 1 || f.events(t)[3].Seq != 1 {
		t.Fatalf("events=%+v", f.events(t))
	}
}
func TestPanelLaunch_LandedRoundRefused(t *testing.T) {
	f := newLaunchFixture(t)
	id := "test-bead"
	p := f.p
	p.BeadID = &id
	// Build a second panel through its writer, with a bound bead target.
	f.dir = filepath.Join(f.root, "review", "landed")
	p.Target = "bead/" + id
	if e := panel.Create(f.dir, panel.CreateInput{BeadID: &id, Target: p.Target, Gate: "bead", Round: 1, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Roster: p.Roster}); e != nil {
		t.Fatal(e)
	}
	old := checkoutReadMetadataFn
	checkoutReadMetadataFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{"mindspec_landed_review_panel": "landed@r1"}, nil
	}
	defer func() { checkoutReadMetadataFn = old }()
	before := slotRead(t, filepath.Join(f.dir, "dispositions.jsonl"))
	_, e := launchRun(t, panelLaunchCmd, "landed", "R1", "--replace")
	if e == nil || !strings.Contains(e.Error(), "landed") {
		t.Fatalf("landed launch=%v", e)
	}
	slotWrite(t, filepath.Join(f.root, ".mindspec/config.local.yaml"), "panel: {runners: [{family: a, kind: harness}]}\n")
	config.ResetCache()
	_, e = runPanelCmd("checkout", "landed", "--slot", "R1", "--dispatch", "--replace")
	if e == nil || !strings.Contains(e.Error(), "landed") {
		t.Fatalf("landed dispatch=%v", e)
	}
	if !bytes.Equal(before, slotRead(t, filepath.Join(f.dir, "dispositions.jsonl"))) {
		t.Fatal("landed refusal appended")
	}
}
func TestPanelLaunch_QuotaHandoffE2E(t *testing.T) {
	f := newLaunchFixture(t)
	f.configure(t, "stdout", "LIMIT", "b")
	f.runner(t, "printf LIMIT >&2", 2)
	_, _ = f.run(t, false)
	res, _ := panel.Tally(f.dir)
	slots, e := panelSlotStatuses(f.root, res)
	if e != nil || slots[0].Kind != "harness" || !slots[0].HandoffPending || panel.VerifyExitCode(slots) != 1 {
		t.Fatalf("handoff %+v %v", slots, e)
	}
	_, e = runPanelCmd("checkout", "run", "--slot", "R1", "--dispatch")
	if e != nil {
		t.Fatal(e)
	}
	f.v.Model = "b"
	path := filepath.Join(f.layout(t).AttemptDir(2), "verdict.json")
	slotWrite(t, path, string(paneltest.Bytes(t, f.v)))
	if out, e := runPanelVerbCmd("submit", "--file", path); e != nil {
		t.Fatalf("submit %v %s", e, out)
	}
	f.status(t, "final")
}
func TestLedger_NoScratchPaths(t *testing.T) {
	f := newLaunchFixture(t)
	if _, e := f.run(t, false); e != nil {
		t.Fatal(e)
	}
	raw := slotRead(t, filepath.Join(f.dir, "dispositions.jsonl"))
	if bytes.Contains(raw, []byte(f.isolation)) || bytes.Contains(raw, []byte(f.root)) {
		t.Fatalf("scratch path in ledger: %s", raw)
	}
}
func TestPanelLaunch_AdapterE2E(t *testing.T) {
	f := newLaunchFixture(t)
	f.runner(t, `test "$(cat tree/README.md)" = hello
test "$(git -C tree rev-parse HEAD)" = '`+f.p.ReviewedHeadSHA+`'`, 0)
	if out, e := f.run(t, false); e != nil {
		t.Fatalf("adapter launch %v %s", e, out)
	}
	f.status(t, "final")
	if out, e := runPanelVerbCmd("tally", "run"); e != nil {
		t.Fatalf("tally %v %s", e, out)
	}
}
