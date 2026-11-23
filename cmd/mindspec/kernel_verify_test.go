package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestS134PanelRefute_RetiredHistory(t *testing.T) {
	kerneltest.SetClock(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
	for _, slot := range []string{"R1", "R2"} {
		kernelPanelVerdict(t, f, gate, head, "review", slot, "REQUEST_CHANGES", 1)
	}
	v := kernelPanelVerifyRecord(t, f, gate, 0)
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: v.Step}, kernel.LeaseDeps{})
	f.Must(err)
	data, err := json.Marshal(v)
	f.Must(err)
	path := filepath.Join(t.TempDir(), "verify.json")
	f.Must(os.WriteFile(path, data, 0600))
	newSubject := ""
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: v.Step, Lease: lease.Token, Record: path, BeforeClose: func(kernel.State, kernel.Receipt) error {
		actorCmdGit(t, f, "symbolic-ref", "HEAD", "refs/heads/bead/"+f.Items[0])
		rebindWrite(t, f.Root, "later.txt", "changed subject\n")
		actorCmdGit(t, f, "add", "--", "later.txt")
		actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "change before verify applies")
		newSubject = actorCmdGit(t, f, "rev-parse", "HEAD")
		return nil
	}})
	f.Must(err)
	judge := gate + ".judge.a1"
	f.Must(f.Store.WriteReceipt(judge, kernel.Receipt{Kind: "superseded", Subject: newSubject}))
	f.Must(f.Store.SetStatusIf(judge, "open", "closed"))
	round, _, err := kernel.Decode[kernel.GateRound](kernelPanelRecord(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	round.Judged = &kernel.Judged{Judge: judge, Value: "carry-over", NewSubject: newSubject}
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRound, round)))
	kerneltest.SetClock(t, kernel.Now().Add(time.Second))
	v2 := kernelPanelVerifyRecord(t, f, gate, 1)
	if v2.Step != v.Step+".a2" {
		t.Fatalf("retirement did not create a2: %+v", v2)
	}
	kernelPanelSubmitVerify(t, f, v2)
	kerneltest.SetClock(t, kernel.Now().Add(time.Second))
	out, err := kernelPanelRun(t, panelRefuteCmd, "review", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--finding", "ccr[0]", "--by", "operator", "--ground", "disproven", "--reason", "checked on the new subject", "--evidence", "proof.txt")
	kernelPanelRequire(t, out, err)
	raw, err := os.ReadFile(filepath.Join(f.SpecDir, "reviews", "review", panel.FileName))
	f.Must(err)
	var snapshot panel.Panel
	f.Must(json.Unmarshal(raw, &snapshot))
	old := 0
	for _, ref := range snapshot.Refutations {
		if ref.VerifyStep == v.Step {
			old++
		}
	}
	if old != 2 {
		t.Fatalf("retally deleted retired verify history from panel.json: %+v", snapshot.Refutations)
	}
	if kernelPanelRecord(t, f, gate).Status != "closed" {
		t.Fatal("current refutations did not pass the gate")
	}
}

func TestS134PanelRefute_CrashTake(t *testing.T) {
	kerneltest.SetClock(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
	for _, slot := range []string{"R1", "R2"} {
		kernelPanelVerdict(t, f, gate, head, "review", slot, "REQUEST_CHANGES", 1)
	}
	v := kernelPanelVerifyRecord(t, f, gate, 0)
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: v.Step}, kernel.LeaseDeps{})
	f.Must(err)
	data, err := json.Marshal(v)
	f.Must(err)
	path := filepath.Join(t.TempDir(), "verify.json")
	f.Must(os.WriteFile(path, data, 0600))
	killed := errors.New("killed after close writes")
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: v.Step, Lease: lease.Token, Record: path, BeforeClose: func(kernel.State, kernel.Receipt) error { return killed }})
	if !errors.Is(err, killed) {
		t.Fatalf("crash seam did not run: %v", err)
	}
	lease, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: v.Step, Take: true, Reason: "replace the crashed verifier"}, kernel.LeaseDeps{})
	f.Must(err)
	v.Returns[0].Return = "reproduced"
	data, err = json.Marshal(v)
	f.Must(err)
	f.Must(os.WriteFile(path, data, 0600))
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: v.Step, Lease: lease.Token, Record: path})
	if err == nil || !strings.Contains(err.Error(), "mindspec_closing") {
		t.Fatalf("generation 2 overwrote generation 1's closing pin: %v", err)
	}
	v2 := kernelPanelVerifyRecord(t, f, gate, 1)
	kernelPanelSubmitVerify(t, f, v2)
	kerneltest.SetClock(t, kernel.Now().Add(time.Second))
	out, err := kernelPanelRun(t, panelRefuteCmd, "review", "--spec", f.SpecID, "--slot", "R2", "--round", "1", "--finding", "ccr[0]", "--by", "operator", "--ground", "disproven", "--reason", "checked", "--evidence", "proof.txt")
	kernelPanelRequire(t, out, err)
	if kernelPanelRecord(t, f, gate).Status == "closed" || kernelPanelRecord(t, f, gate+".r2.fix").Status == "closed" {
		t.Fatal("the unfinished generation-1 refutation discharged the reproduced finding")
	}
}

func kernelPanelVerifyRecord(t *testing.T, f *kerneltest.Fixture, gate string, reproduced int) kernel.VerifyRecord {
	t.Helper()
	_, err := kernel.NextAction(f.State(), true)
	f.Must(err)
	round, _, err := kernel.Decode[kernel.GateRound](kernelPanelRecord(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	r := kernelPanelRecord(t, f, round.Verify)
	findings, _, err := kernel.Decode[[]kernel.Finding](r.Metadata, kernel.KeyFindings)
	f.Must(err)
	subject, _, err := kernel.Decode[kernel.Subject](r.Metadata, kernel.KeySubject)
	f.Must(err)
	v := kernel.VerifyRecord{Step: r.Key(), Subject: subject.Commit, Returns: []kernel.VerifyReturn{}}
	for i, finding := range findings {
		ret := kernel.VerifyReturn{Slot: finding.Slot, Round: finding.Round, Finding: finding.Finding, FindingSHA256: finding.FindingSHA256, Return: "refuted", Ground: "disproven", Reason: "checked", Evidence: "https://example.invalid/proof"}
		if i < reproduced {
			ret.Return, ret.Ground, ret.Reason, ret.Evidence = "reproduced", "", "", ""
		}
		v.Returns = append(v.Returns, ret)
	}
	return v
}

func kernelPanelSubmitVerify(t *testing.T, f *kerneltest.Fixture, v kernel.VerifyRecord) kernel.Output {
	t.Helper()
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: v.Step}, kernel.LeaseDeps{})
	f.Must(err)
	data, err := json.Marshal(v)
	f.Must(err)
	path := filepath.Join(t.TempDir(), "verify.json")
	f.Must(os.WriteFile(path, data, 0600))
	out, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: v.Step, Lease: lease.Token, Record: path})
	f.Must(err)
	return out
}

func kernelPanelReproduce(t *testing.T, f *kerneltest.Fixture, gate string) kernel.Output {
	t.Helper()
	return kernelPanelSubmitVerify(t, f, kernelPanelVerifyRecord(t, f, gate, int(^uint(0)>>1)))
}

func TestS134VerifyCmd_Shape(t *testing.T) {
	for _, bad := range []string{"unknown-field", "missing", "repeated", "empty", "return", "ground", "ac", "reason", "evidence", "target", "deferral"} {
		t.Run(bad, func(t *testing.T) {
			f, gate, head := kernelPanelFixture(t)
			kernelPanelAuthorise(f, gate, head, 1, 2)
			kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
			for _, slot := range []string{"R1", "R2"} {
				kernelPanelVerdict(t, f, gate, head, "review", slot, "REQUEST_CHANGES", 1)
			}
			v := kernelPanelVerifyRecord(t, f, gate, 0)
			lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: v.Step}, kernel.LeaseDeps{})
			f.Must(err)
			want := ""
			switch bad {
			case "unknown-field":
				want = "unknown field"
			case "missing":
				v.Returns = v.Returns[:1]
				want = "missing finding"
			case "repeated":
				v.Returns = append(v.Returns, v.Returns[0])
				want = "repeated finding"
			case "empty":
				v.Returns = nil
				want = "returns"
			case "return":
				v.Returns[0].Return = "accepted"
				want = "must be reproduced, refuted or deferred"
			case "ground":
				v.Returns[0].Ground = "accepted"
				want = "valid ground"
			case "ac":
				v.Returns[0].Ground = "out_of_acceptance_scope"
				want = "needs ac"
			case "reason":
				v.Returns[0].Reason = ""
				want = "reason"
			case "evidence":
				v.Returns[0].Evidence = "absent-evidence"
				want = "resolving evidence"
			case "target":
				v.Returns[0].Return = "deferred"
				want = "target"
			case "deferral":
				v.Returns[0].Return, v.Returns[0].Target = "deferred", "follow-up"
				fake := fakeDeferralBD(t)
				rebindWrite(t, fake, "follow-up.json", `[{"id":"follow-up","status":"closed","description":"missing test"}]`)
				want = "must exist and be open"
			}
			data, err := json.Marshal(v)
			f.Must(err)
			if bad == "unknown-field" {
				data = append(data[:len(data)-1], []byte(`,"extra":true}`)...)
			}
			path := filepath.Join(t.TempDir(), "verify.json")
			f.Must(os.WriteFile(path, data, 0600))
			old := submitCommandStateFn
			submitCommandStateFn = func(string, string) (kernel.State, error) { return f.State(), nil }
			t.Cleanup(func() { submitCommandStateFn = old })
			out, err := kernelPanelRun(t, submitCmd, v.Step, "--lease", lease.Token, "--record", path)
			kernelPanelRefusal(t, out, err, want)
			r := kernelPanelRecord(t, f, v.Step)
			if r.Status == "closed" || r.Metadata[kernel.KeyClosing] != nil {
				t.Fatalf("bad record closed or pinned its step: %+v", r)
			}
			lines, err := panel.ReadLedger(filepath.Join(f.SpecDir, "reviews", "review"))
			f.Must(err)
			for _, line := range lines {
				var event struct{ Record string }
				f.Must(json.Unmarshal(line, &event))
				if event.Record == panel.RecordDeferral {
					t.Fatal("failed record wrote a deferral")
				}
			}
		})
	}
}

func TestS134PanelRefute_VerifyOpenAndOldVerb(t *testing.T) {
	for _, reproduced := range []bool{false, true} {
		t.Run(fmt.Sprint(reproduced), func(t *testing.T) {
			kerneltest.SetClock(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
			f, gate, head := kernelPanelFixture(t)
			kernelPanelAuthorise(f, gate, head, 1, 2)
			kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
			for _, slot := range []string{"R1", "R2"} {
				kernelPanelVerdict(t, f, gate, head, "review", slot, "REQUEST_CHANGES", 1)
			}
			v := kernelPanelVerifyRecord(t, f, gate, 0)
			refute := func(slot, evidence string) {
				t.Helper()
				out, err := kernelPanelRun(t, panelRefuteCmd, "review", "--spec", f.SpecID, "--slot", slot, "--round", "1", "--finding", "ccr[0]", "--by", "operator", "--ground", "disproven", "--reason", "checked", "--evidence", evidence)
				kernelPanelRequire(t, out, err)
			}
			refute("R2", "proof.txt")
			if !reproduced {
				refute("R1", "proof.txt")
				r := kernelPanelRecord(t, f, v.Step)
				receipt, _, _ := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
				if receipt.Kind != "superseded" || kernelPanelRecord(t, f, gate).Status != "closed" {
					t.Fatalf("passing refute did not supersede open verify: %+v", r)
				}
				return
			}
			v.Returns[1].Return, v.Returns[1].Ground, v.Returns[1].Reason, v.Returns[1].Evidence = "reproduced", "", "", ""
			kerneltest.SetClock(t, kernel.Now().Add(500*time.Millisecond))
			kernelPanelSubmitVerify(t, f, v)
			fix := kernelPanelRecord(t, f, gate+".r2.fix")
			findings, _, err := kernel.Decode[[]kernel.Finding](fix.Metadata, kernel.KeyReproduced)
			f.Must(err)
			if kernelPanelRecord(t, f, gate).Status == "closed" || len(findings) != 1 || findings[0].Slot != "R2" {
				t.Fatalf("old verb refutation discharged the reproduced finding: %+v", findings)
			}
			kerneltest.SetClock(t, kernel.Now().Add(time.Second))
			refute("R2", "https://example.invalid/second-evidence")
			if kernelPanelRecord(t, f, gate).Status != "closed" || kernelPanelRecord(t, f, fix.Key()).Status != "closed" {
				t.Fatal("second verb refutation did not pass and supersede the fix")
			}
			res, err := panel.Tally(filepath.Join(f.SpecDir, "reviews", "review"))
			f.Must(err)
			var refs []panel.Refutation
			for _, r := range res.Panel.Refutations {
				if r.Slot == "R2" {
					refs = append(refs, r)
				}
			}
			if len(refs) != 2 || reflect.DeepEqual(refs[0], refs[1]) {
				t.Fatalf("second enrolled verb did not append history: %+v", refs)
			}
			lines, err := panel.ReadLedger(res.Dir)
			f.Must(err)
			rows := map[string]bool{}
			for _, line := range lines {
				var row panel.DispositionRow
				f.Must(json.Unmarshal(line, &row))
				if row.Record == panel.RecordDisposition {
					rows[row.ID] = true
				}
			}
			for _, ref := range refs {
				if !rows[panel.RefutationRowID("review", ref)] || !strings.HasSuffix(panel.RefutationRowID("review", ref), ":"+ref.CreatedAt) {
					t.Fatalf("verb row has no created_at identity: %+v", ref)
				}
			}
		})
	}
}

func TestS134PanelDefer_VerifyCloseBinding(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
	for _, slot := range []string{"R1", "R2"} {
		kernelPanelVerdict(t, f, gate, head, "review", slot, "REQUEST_CHANGES", 1)
	}
	v := kernelPanelVerifyRecord(t, f, gate, 0)
	v.Returns[0].Return, v.Returns[0].Reason, v.Returns[0].Target = "deferred", "follow up", "follow-up"
	fake := fakeDeferralBD(t)
	rebindWrite(t, fake, "follow-up.json", fmt.Sprintf(`[{"id":"follow-up","status":"open","description":"missing test","dependencies":[{"id":%q,"dependency_type":"discovered-from"}]}]`, f.Epic))
	kernelPanelSubmitVerify(t, f, v)
	res, err := panel.Tally(filepath.Join(f.SpecDir, "reviews", "review"))
	f.Must(err)
	if len(res.Panel.Deferrals) != 1 || len(res.Panel.Refutations) != 1 {
		t.Fatalf("verify close omitted a disposition: %+v", res.Panel)
	}
	d, ref := res.Panel.Deferrals[0], res.Panel.Refutations[0]
	if d.VerifyStep != v.Step || ref.VerifyStep != v.Step || !d.Enrolled || !ref.Enrolled || d.SourceVerdict != panel.VerdictRequestChanges || d.SourceFileSHA256 == "" {
		t.Fatalf("verify dispositions lost their attempt or source: %+v %+v", d, ref)
	}
	history, _, err := kernel.Decode[[]kernel.DeferRecord](kernelPanelRecord(t, f, gate).Metadata, kernel.KeyDeferrals)
	f.Must(err)
	if len(history) != 1 || history[0].VerifyStep != v.Step || history[0].FindingSHA256 != d.FindingSHA256 || history[0].SourceFileSHA256 != d.SourceFileSHA256 || history[0].SourceVerdict != d.SourceVerdict {
		t.Fatalf("verify deferral history omitted its binding: %+v", history)
	}
	if err := kernel.CheckPassedGate(f.State(), gate); err != nil {
		t.Fatal(err)
	}
}

func TestS134VerifyCmd_FrozenRoundCreate(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, err := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, err)
	kernelPanelVerdict(t, f, gate, head, "1", "R1", "REQUEST_CHANGES", 1)
	kernelPanelVerdict(t, f, gate, head, "1", "R2", "APPROVE", 1)
	v := kernelPanelVerifyRecord(t, f, gate, 1)
	kernelPanelSubmitVerify(t, f, v)
	fix := gate + ".r2.fix"
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: fix}, kernel.LeaseDeps{})
	f.Must(err)
	pinned := kernelPanelRecord(t, f, fix).Metadata[kernel.KeyReproduced]
	actorCmdGit(t, f, "symbolic-ref", "HEAD", "refs/heads/bead/"+f.Items[0])
	base := filepath.Join(".mindspec", "specs", f.SpecID, "reviews", "1")
	for _, name := range []string{"R1", "R9"} {
		word := "APPROVE"
		if name == "R9" {
			word = "REJECT"
		}
		path := filepath.Join(base, name+"-round-1.json")
		rebindWrite(t, f.Root, path, fmt.Sprintf(`{"verdict":%q}`, word))
		actorCmdGit(t, f, "add", "-f", "--", path)
		actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "panel evidence edit")
	}
	_, err = kernel.NextAction(f.State(), true)
	f.Must(err)
	for _, id := range []string{gate + ".r1.verify.a2", gate + ".r1.R1.a2"} {
		if r, err := f.Store.Lookup(id); err != nil || r != nil {
			t.Fatalf("frozen round created %s: %v", id, err)
		}
	}
	if !reflect.DeepEqual(pinned, kernelPanelRecord(t, f, fix).Metadata[kernel.KeyReproduced]) {
		t.Fatal("changed verdict files changed the fix scope")
	}
	rebindWrite(t, f.Root, "fixed.txt", "fixed\n")
	actorCmdGit(t, f, "add", "--", "fixed.txt")
	actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "fix finding")
	commit := actorCmdGit(t, f, "rev-parse", "HEAD")
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: fix, Lease: lease.Token, Commit: commit})
	f.Must(err)
	out, err = kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, err)
	round, _, err := kernel.Decode[kernel.GateRound](kernelPanelRecord(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	if round.Round != 2 || round.Prior == nil || round.Prior.Verify != v.Step {
		t.Fatalf("next registration lost the frozen verify: %+v", round)
	}
}
