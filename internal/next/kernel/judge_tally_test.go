package kernel_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"crypto/sha256"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"os"
)

func tallyJudgeFixture(t *testing.T, refused bool) (*kerneltest.Fixture, string, string) {
	t.Helper()
	f := reducerFixture(t)
	reducerGit(t, f, "switch", "bead/"+f.Items[0])
	old := judgeCommit(t, f, ".mindspec/config.yaml", "panel:\n  reviewers:\n    - family: harness\n      count: 3\n")
	gate := f.Items[0] + ".review"
	if refused {
		refusedPanel(t, f, gate, "review", 1, panel.VerdictRequestChanges)
	} else {
		passingPanel(t, f)
	}
	return f, gate, old
}

func judgeAttemptSubmit(t *testing.T, f *kerneltest.Fixture, gate string, attempt int, j kernel.Judgement) kernel.Output {
	t.Helper()
	id := fmt.Sprintf("%s.judge.a%d", gate, attempt)
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	out, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: judgeArtifact(t, f, j)})
	f.Must(err)
	return out
}

func tallyRound(t *testing.T, f *kerneltest.Fixture, gate string) kernel.GateRound {
	t.Helper()
	round, has, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	if !has {
		t.Fatal("missing round")
	}
	return round
}

func TestS134Judge_TallyCarryAndPanelExemption(t *testing.T) {
	for _, panelOnly := range []bool{false, true} {
		t.Run(fmt.Sprint(panelOnly), func(t *testing.T) {
			f, gate, old := tallyJudgeFixture(t, false)
			name := "changed.txt"
			if panelOnly {
				name = filepath.ToSlash(filepath.Join(".mindspec/specs", f.SpecID, "reviews/panel/note.txt"))
			}
			newest := judgeCommit(t, f, name, "change")
			out := reduce(t, f, true)
			if panelOnly {
				if record(t, f, gate).Status != "closed" || action(out, gate+".judge.a1") {
					t.Fatalf("panel directory was judged: %+v", out)
				}
				return
			}
			if record(t, f, gate).Status == "closed" || !action(out, gate+".judge.a1") {
				t.Fatalf("tally did not wait for judge: %+v", out)
			}
			if v, _ := f.Store.Lookup(gate + ".r1.verify"); v != nil {
				t.Fatal("held tally created verify")
			}
			before := reducerGit(t, f, "status", "--porcelain")
			judgeSubmit(t, f, gate, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same behavior"})
			if before != reducerGit(t, f, "status", "--porcelain") {
				t.Fatal("judge submit changed git status")
			}
			receipt := actorReceipt(t, f, gate)
			if receipt.Subject != newest || receipt.Kind != "passed" {
				t.Fatalf("review subject missing in receipt: %+v", receipt)
			}
			f.Must(kernel.CheckPassedGate(f.State(), gate))
			if action(reduce(t, f, true), gate+".judge.a2") {
				t.Fatal("judged subject was judged again")
			}
		})
	}
}

func TestS134Judge_OpenRePanelOutcomes(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(fmt.Sprint(refused), func(t *testing.T) {
			f, gate, old := tallyJudgeFixture(t, refused)
			newest := judgeCommit(t, f, "changed.txt", "change")
			reduce(t, f, true)
			judgeSubmit(t, f, gate, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 2, Paths: []string{"changed.txt"}})
			if refused {
				id, v := verifyRecord(t, f, gate, 0)
				subject, _, _ := kernel.Decode[kernel.Subject](record(t, f, id).Metadata, kernel.KeySubject)
				if subject.Commit != newest {
					t.Fatal("verify did not use review subject")
				}
				lease := leaseActor(t, f, id, kernel.LeaseDeps{})
				_, err := submitVerify(t, f, v, lease, nil)
				f.Must(err)
			}
			r := tallyRound(t, f, gate)
			if record(t, f, gate).Status == "closed" || r.Round != 2 || r.Subject != newest || r.SlotCount != 2 || !reflect.DeepEqual(r.Paths, []string{"changed.txt"}) || r.Prior == nil || r.Prior.Round != 1 {
				t.Fatalf("re-panel did not authorize a scoped round on open gate: %+v", r)
			}
			if !refused {
				name := filepath.Join(f.SpecDir, "reviews/panel", panel.FileName)
				data, err := os.ReadFile(name)
				f.Must(err)
				rel, err := filepath.Rel(f.Root, name)
				f.Must(err)
				if got, ok := r.Prior.Read[filepath.ToSlash(rel)]; !ok || got != fmt.Sprintf("%x", sha256.Sum256(data)) {
					t.Fatalf("tally prior panel.json digest = %q (present %v), want digest of %s", got, ok, rel)
				}
			}
			if refused && r.Prior.Verify == "" {
				t.Fatal("verify authorization lost prior.verify")
			}
			if action(reduce(t, f, true), gate+".judge.a2") {
				t.Fatal("scoped authorization created judge")
			}
			kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "round-two", Round: 2, Subject: r.Subject, Slots: []string{"R1", "R2"}})
			kerneltest.PassGate(f, gate, r.Subject)
			if action(reduce(t, f, true), gate+".judge.a2") {
				t.Fatal("scoped tally created judge")
			}
			judgeCommit(t, f, "later.txt", "later")
			if !action(reduce(t, f, true), gate+".judge.a2") {
				t.Fatal("fresh commit created no judge")
			}
		})
	}
}

func TestS134Judge_VerifyAcrossJudgement(t *testing.T) {
	for _, receipted := range []bool{false, true} {
		t.Run(fmt.Sprint(receipted), func(t *testing.T) {
			f, gate, old := tallyJudgeFixture(t, true)
			reduce(t, f, true)
			id, v := verifyRecord(t, f, gate, 0)
			lease := leaseActor(t, f, id, kernel.LeaseDeps{})
			if receipted {
				restore := kernel.ReplaceRulesForTest(kernel.NewRegistry())
				_, err := submitVerify(t, f, v, lease, nil)
				restore()
				f.Must(err)
			}
			newest := judgeCommit(t, f, "changed.txt", "change")
			reduce(t, f, true)
			judgeSubmit(t, f, gate, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same behavior"})
			r := tallyRound(t, f, gate)
			if r.Verify != id+".a2" || record(t, f, gate).Status == "closed" {
				t.Fatalf("verify not retired: %+v", r)
			}
			subject, _, err := kernel.Decode[kernel.Subject](record(t, f, id+".a2").Metadata, kernel.KeySubject)
			f.Must(err)
			if subject.Commit != newest {
				t.Fatalf("replacement verify subject = %s, want %s", subject.Commit, newest)
			}
			receipt := actorReceipt(t, f, id)
			if receipted && receipt.Kind != "passed" || !receipted && receipt.Kind != "superseded" {
				t.Fatalf("wrong retirement receipt: %+v", receipt)
			}
			if _, err := submitVerify(t, f, v, lease, nil); !receipted && err == nil {
				t.Fatal("retired verify accepted a new receipt")
			}
			if record(t, f, gate).Status == "closed" {
				t.Fatal("old verify passed the gate")
			}
			if fix, _ := f.Store.Lookup(gate + ".r2.fix"); fix != nil {
				t.Fatal("old verify created fix")
			}
		})
	}
}

func TestS134Judge_WithdrawAuthorisedRound(t *testing.T) {
	f, gate, old := tallyJudgeFixture(t, true)
	j := judgeCommit(t, f, "changed.txt", "J")
	reduce(t, f, true)
	judgeSubmit(t, f, gate, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: j, SlotCount: 2, Paths: []string{"changed.txt"}})
	id, v := verifyRecord(t, f, gate, 0)
	_, err := submitVerify(t, f, v, leaseActor(t, f, id, kernel.LeaseDeps{}), nil)
	f.Must(err)
	if tallyRound(t, f, gate).Round != 2 {
		t.Fatal("verify did not authorize round two")
	}
	// The prior is authoritative even when an authorized subject differs. This
	// isolates the accepted-subject reader from the author's round record.
	state := f.State()
	pending := tallyRound(t, f, gate)
	pending.Subject = old
	g := state.Records[gate]
	g.Metadata[kernel.KeyRound] = pending
	state.Records[gate] = g
	acceptedPrior, err := kernel.AcceptedSubject(state, gate)
	f.Must(err)
	if acceptedPrior != j {
		t.Fatalf("accepted pending subject came from authorized round, want prior review subject: %s != %s", acceptedPrior, j)
	}
	newest := judgeCommit(t, f, "later.txt", "S")
	out := reduce(t, f, true)
	if !action(out, gate+".judge.a2") {
		t.Fatalf("pending scoped round was not rejudged: %+v", out)
	}
	accepted, err := kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	if accepted != j {
		t.Fatalf("old subject = %s, want accepted %s", accepted, j)
	}
	judgeAttemptSubmit(t, f, gate, 2, kernel.Judgement{Value: "re-panel", OldSubject: j, NewSubject: newest, SlotCount: 3, Paths: []string{"later.txt"}})
	r := tallyRound(t, f, gate)
	if r.Round != 1 || r.Verify != id+".a2" {
		t.Fatalf("withdraw did not restore prior and create one verify: %+v", r)
	}
	subject, _, err := kernel.Decode[kernel.Subject](record(t, f, id+".a2").Metadata, kernel.KeySubject)
	f.Must(err)
	if subject.Commit != newest {
		t.Fatalf("withdrawn round verify subject = %s, want %s", subject.Commit, newest)
	}
	next, v := verifyRecord(t, f, gate, 0)
	_, err = submitVerify(t, f, v, leaseActor(t, f, next, kernel.LeaseDeps{}), nil)
	f.Must(err)
	r = tallyRound(t, f, gate)
	if r.Round != 2 || r.Subject != newest || r.SlotCount != 3 || !reflect.DeepEqual(r.Paths, []string{"later.txt"}) || r.Prior.Verify != next || record(t, f, gate).Status == "closed" {
		t.Fatalf("reverify closed gate or lost latest scope: %+v", r)
	}
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "round-two", Round: 2, Subject: newest, Slots: []string{"R1", "R2", "R3"}})
	for i, id := range r.Slots {
		v := paneltest.Verdict("APPROVE")
		v.Slot, v.Round, v.ReviewedSHA, v.Model = fmt.Sprintf("R%d", i+1), 2, newest, "test-model"
		_, err := panel.Submit(panel.SubmitTarget{Dir: filepath.Join(f.SpecDir, "reviews/round-two"), Spec: f.SpecID, Panel: "round-two", Slot: v.Slot, Round: 2}, paneltest.Bytes(t, v), func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
			return panel.SubmitResolution{Family: "harness", Model: "test-model", RunnerKind: "harness", RunnerSource: "default"}, nil
		})
		f.Must(err)
		f.Create(id, gate)
		closePass(t, f, id)
	}
	reduce(t, f, true)
	if record(t, f, gate).Status != "closed" {
		t.Fatal("withdrawn round did not pass after round two")
	}
	if receipt := actorReceipt(t, f, gate); receipt.Kind != "passed" || receipt.Subject != newest {
		t.Fatalf("withdrawn round did not pass after round two: %+v", receipt)
	}
}

func TestS134Judge_RejudgeFrozenSuccessor(t *testing.T) {
	f, gate, old := judgeFixture(t)
	j := judgeChange(t, f)
	reduce(t, f, true)
	judgeSubmit(t, f, gate, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: j, SlotCount: 2, Paths: []string{".mindspec/config.yaml"}})
	newest := judgeCommit(t, f, "later.txt", "S")
	out := reduce(t, f, true)
	if !action(out, gate+".judge.a2") || hasRun(out, "mindspec", "panel", "create", f.Items[0]) {
		t.Fatalf("subject move did not hold successor: %+v", out)
	}
	accepted, err := kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	if accepted != old {
		t.Fatalf("successor is not accepted: %s != %s", accepted, old)
	}
	judgeAttemptSubmit(t, f, gate, 2, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same behavior"})
	if actorReceipt(t, f, gate+".a2").Kind != "superseded" || len(actorReceipt(t, f, gate).CarryOvers) != 1 {
		t.Fatal("rejudge did not withdraw successor and extend frozen receipt")
	}
	f.Must(kernel.CheckPassedGate(f.State(), gate))
}

func TestS134Judge_PathCover(t *testing.T) {
	f, gate, old := tallyJudgeFixture(t, false)
	judgeCommit(t, f, "one.txt", "one")
	newest := judgeCommit(t, f, "two.txt", "two")
	reduce(t, f, true)
	id := gate + ".judge.a1"
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: judgeArtifact(t, f, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 2, Paths: []string{"one.txt"}})})
	if err == nil || !strings.Contains(err.Error(), "two.txt") || record(t, f, id).Status == "closed" {
		t.Fatalf("incomplete path cover accepted: %v", err)
	}
	assertSubmitRefusal(t, err, "two.txt", "mindspec next-action")
}

func TestS134Judge_PriorFromPinnedVerify(t *testing.T) {
	f, gate, _ := tallyJudgeFixture(t, true)
	reproduceVerify(t, f, gate)
	prior := tallyRound(t, f, gate)
	verify := record(t, f, prior.Verify)
	findings, _, _ := kernel.Decode[[]kernel.Finding](verify.Metadata, kernel.KeyFindings)
	read, _, _ := kernel.Decode[map[string]string](verify.Metadata, kernel.KeyRead)
	fix := gate + ".r2.fix"
	lease := leaseActor(t, f, fix, kernel.LeaseDeps{})
	dir := filepath.Join(f.SpecDir, "reviews/review")
	put(t, filepath.Join(dir, "R1-round-1.json"), `{"verdict":"APPROVE"}`)
	put(t, filepath.Join(dir, "R9-round-1.json"), `{"verdict":"REJECT","rationale":"stray"}`)
	commit := judgeCommit(t, f, "fix.txt", "fix")
	beforeObjects := reducerGit(t, f, "count-objects", "-v")
	restore := kernel.ReplaceRulesForTest(kernel.NewRegistry())
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: fix, Lease: lease.Token, Commit: commit})
	restore()
	f.Must(err)
	before := kerneltest.Snapshot(t, f)
	probe, err := kernel.Pass(kernel.Rules(), f.State(), false)
	f.Must(err)
	proposed, _, err := kernel.Decode[kernel.GateRound](probe.State.Records[gate].Metadata, kernel.KeyRound)
	f.Must(err)
	if proposed.Round != 2 || proposed.Prior == nil {
		t.Fatal("dry pass did not compute prior authorization")
	}
	if before != kerneltest.Snapshot(t, f) || beforeObjects != reducerGit(t, f, "count-objects", "-v") {
		t.Fatal("dry pass wrote metadata or git objects")
	}
	reduce(t, f, true)
	r := tallyRound(t, f, gate)
	if r.Round != 2 || r.Prior == nil || r.Prior.Verify != prior.Verify || !reflect.DeepEqual(r.Prior.Read, read) || len(r.Prior.Asks) != len(findings) {
		t.Fatalf("prior did not use verify: %+v", r)
	}
	for i, finding := range findings {
		if r.Prior.Asks[i].Finding != finding || r.Prior.Asks[i].Returns[0].Return != "reproduced" {
			t.Fatalf("prior ask lost pinned finding: %+v", r.Prior.Asks)
		}
	}
	if beforeObjects != reducerGit(t, f, "count-objects", "-v") {
		t.Fatal("authorisation wrote git objects")
	}
}

func TestS134Judge_RefusedCarryFixSubject(t *testing.T) {
	f, gate, old := tallyJudgeFixture(t, true)
	newest := judgeCommit(t, f, "changed.txt", "J")
	reduce(t, f, true)
	judgeSubmit(t, f, gate, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same behavior"})
	reproduceVerify(t, f, gate)
	fix := gate + ".r2.fix"
	lease := leaseActor(t, f, fix, kernel.LeaseDeps{})
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: fix, Lease: lease.Token, Commit: newest})
	if err == nil {
		t.Fatal("fix accepted its own review subject")
	}
	commit := judgeCommit(t, f, "fix.txt", "fix")
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: fix, Lease: lease.Token, Commit: commit})
	f.Must(err)
	if tallyRound(t, f, gate).Round != 2 {
		t.Fatal("fix did not authorize next round")
	}
}

func TestS134SeedMatch_JudgeOpenGate(t *testing.T) {
	a, gate, _ := tallyJudgeFixture(t, false)
	b, _, _ := tallyJudgeFixture(t, false)
	newest := judgeCommit(t, a, "changed.txt", "J")
	if twin := judgeCommit(t, b, "changed.txt", "J"); twin != newest {
		t.Fatal("fixtures are not twins")
	}
	reduce(t, a, true)
	kerneltest.Judge(b, gate, newest, 1)
	if !jsonEqual(record(t, a, gate+".judge.a1"), record(t, b, gate+".judge.a1")) {
		t.Fatal("open gate Judge differs from producer")
	}
}

func TestS134SeedMatch_AuthoriseVerifyRepanel(t *testing.T) {
	a, gate, old := tallyJudgeFixture(t, true)
	b, _, _ := tallyJudgeFixture(t, true)
	for _, f := range []*kerneltest.Fixture{a, b} {
		newest := judgeCommit(t, f, "changed.txt", "J")
		reduce(t, f, true)
		judgeSubmit(t, f, gate, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 2, Paths: []string{"changed.txt"}})
	}
	id, v := verifyRecord(t, a, gate, 0)
	lease := leaseActor(t, a, id, kernel.LeaseDeps{})
	_, err := submitVerify(t, a, v, lease, nil)
	a.Must(err)
	restore := kernel.ReplaceRulesForTest(kernel.NewRegistry())
	_, err = submitVerify(t, b, v, leaseActor(t, b, id, kernel.LeaseDeps{}), nil)
	restore()
	b.Must(err)
	prior := tallyRound(t, b, gate)
	findings, _, err := kernel.Decode[[]kernel.Finding](record(t, b, id).Metadata, kernel.KeyFindings)
	b.Must(err)
	prior.Read, _, err = kernel.Decode[map[string]string](record(t, b, id).Metadata, kernel.KeyRead)
	b.Must(err)
	for i, finding := range findings {
		prior.Asks = append(prior.Asks, kernel.PriorAsk{Finding: finding, Returns: []kernel.VerifyReturn{v.Returns[i]}})
	}
	scope, _, err := kernel.Decode[kernel.RePanel](record(t, b, gate).Metadata, kernel.KeyRePanel)
	b.Must(err)
	closeDraftLens(t, a, gate)
	kerneltest.Authorise(b, gate, kernel.GateRound{Round: 2, Subject: v.Subject, SlotCount: 2, Paths: scope.Paths, Slots: []string{gate + ".r2.R1", gate + ".r2.R2"}, Prior: &prior})
	if !jsonEqual(record(t, a, gate), record(t, b, gate)) {
		t.Fatal("Authorise differs from verify re-panel producer")
	}
}

func TestS134Judge_RawCloseRetriesProceed(t *testing.T) {
	for _, kind := range []string{"verify", "fix"} {
		t.Run(kind, func(t *testing.T) {
			f, gate, _ := tallyJudgeFixture(t, true)
			reduce(t, f, true)
			id, v := verifyRecord(t, f, gate, 0)
			if kind == "fix" {
				reproduceVerify(t, f, gate)
				id = gate + ".r2.fix"
			}
			leaseActor(t, f, id, kernel.LeaseDeps{})
			changeActorMetadata(t, f, id, "closed", kernel.KeyRecord, kernel.KeyReturns, kernel.KeyClose, kernel.KeyClosing)
			reduce(t, f, true)
			retry := id + ".a2"
			lease := leaseActor(t, f, retry, kernel.LeaseDeps{})
			if kind == "verify" {
				v.Step = retry
				_, err := submitVerify(t, f, v, lease, nil)
				f.Must(err)
				receipt := actorReceipt(t, f, gate)
				if receipt.Kind != "passed" || receipt.Retally != "verify" {
					t.Fatalf("verify retry did not pass gate: %+v", receipt)
				}
				assertRetry(t, f, retry, retry+".a2", false)
			} else {
				commit := judgeCommit(t, f, "fix.txt", "fix")
				_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: retry, Lease: lease.Token, Commit: commit})
				f.Must(err)
				r := tallyRound(t, f, gate)
				if r.Round != 2 || r.Prior == nil || r.Prior.Verify != v.Step {
					t.Fatalf("fix retry did not authorize round two: %+v", r)
				}
			}
		})
	}
}

func TestS134Judge_RepairVerifyRetallies(t *testing.T) {
	f, gate, _ := tallyJudgeFixture(t, true)
	reduce(t, f, true)
	id, v := verifyRecord(t, f, gate, 0)
	leaseActor(t, f, id, kernel.LeaseDeps{})
	data, err := json.Marshal(v)
	f.Must(err)
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRecord, string(data))))
	f.Must(f.Store.SetStatusIf(id, "in_progress", "closed"))
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Repair: true, Reason: "restore interrupted close"})
	f.Must(err)
	receipt := actorReceipt(t, f, gate)
	if receipt.Kind != "passed" || receipt.Retally != "verify" {
		t.Fatalf("repaired verify did not retally: %+v", receipt)
	}
	result, err := panel.Tally(filepath.Join(f.SpecDir, "reviews/review"))
	f.Must(err)
	if len(result.Panel.Refutations) != len(v.Returns) {
		t.Fatalf("repair lost refutation records: %+v", result.Panel.Refutations)
	}
	for _, ref := range result.Panel.Refutations {
		if ref.VerifyStep != id {
			t.Fatalf("repair lost verify identity: %+v", ref)
		}
	}
	if next, _ := f.Store.Lookup(id + ".a2"); next != nil {
		t.Fatal("repair created another verify")
	}
}

func TestS134Judge_IncompleteRetirementCreatesNoVerify(t *testing.T) {
	f, gate, old := tallyJudgeFixture(t, true)
	reduce(t, f, true)
	prior := tallyRound(t, f, gate).Verify
	newest := judgeCommit(t, f, "changed.txt", "J")
	reduce(t, f, true)
	put(t, filepath.Join(f.SpecDir, "reviews/review/R2-round-1.json"), "{broken")
	judgeSubmit(t, f, gate, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same behavior"})
	if next, _ := f.Store.Lookup(prior + ".a2"); next != nil {
		t.Fatal("incomplete tally created a transient verify")
	}
	if tallyRound(t, f, gate).Verify != "" || actorReceipt(t, f, prior).Kind != "superseded" {
		t.Fatal("incomplete tally did not retire and clear current verify")
	}
	if next, _ := f.Store.Lookup(gate + ".r1.R2.a2"); next == nil {
		t.Fatal("incomplete tally created no slot retry")
	}
}

// P1: the path cover excludes the gate's panel directories (plan Bead 16 step 5).
func TestS134Judge_CoverExcludesPanelDir(t *testing.T) {
	f, gate, old := tallyJudgeFixture(t, false)
	put(t, filepath.Join(f.SpecDir, "reviews/panel/note.txt"), "evidence")
	reducerGit(t, f, "add", "--", filepath.ToSlash(filepath.Join(".mindspec/specs", f.SpecID, "reviews/panel/note.txt")))
	newest := judgeCommit(t, f, "changed.txt", "change")
	reduce(t, f, true)
	id := gate + ".judge.a1"
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: judgeArtifact(t, f, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 2, Paths: []string{"changed.txt"}})})
	if err != nil {
		t.Fatalf("re-panel covering every non-panel path refused: %v", err)
	}
}

// P2: a re-panel after a superseded successor gets a fresh attempt id.
func TestS134Judge_RepanelAfterSupersededSuccessor(t *testing.T) {
	f, gate, old := judgeFixture(t)
	j := judgeChange(t, f)
	reduce(t, f, true)
	judgeSubmit(t, f, gate, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: j, SlotCount: 2, Paths: []string{".mindspec/config.yaml"}})
	s1 := judgeCommit(t, f, "later.txt", "S")
	reduce(t, f, true)
	judgeAttemptSubmit(t, f, gate, 2, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: s1, Reason: "same behavior"})
	if actorReceipt(t, f, gate+".a2").Kind != "superseded" {
		t.Fatal("setup: a2 not superseded")
	}
	s2 := judgeCommit(t, f, "third.txt", "T")
	out := reduce(t, f, true)
	if !action(out, gate+".judge.a3") {
		t.Fatalf("no third judge: %+v", out)
	}
	accepted, err := kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	judgeAttemptSubmit(t, f, gate, 3, kernel.Judgement{Value: "re-panel", OldSubject: accepted, NewSubject: s2, SlotCount: 2, Paths: []string{"third.txt"}})
	a3 := record(t, f, gate+".a3")
	if a3.Status == "closed" {
		t.Fatalf("a3 closed: %+v", a3)
	}
	round, _, err := kernel.Decode[kernel.GateRound](a3.Metadata, kernel.KeyRound)
	f.Must(err)
	if round.Subject != s2 || fmt.Sprint(round.Paths) != "[third.txt]" {
		t.Fatalf("a3 round wrong: %+v", round)
	}
	if actorReceipt(t, f, gate+".a2").Kind != "superseded" {
		t.Fatal("a2 receipt changed")
	}
}

// Persist every prefix of the replacement transition, then resume with the
// normal reducer. A new attempt must never remain open without a round pointer.
func TestS134Judge_VerifyReplacementCrashPrefixes(t *testing.T) {
	const maxCuts = 100
	for cut := 0; cut < maxCuts; cut++ {
		finished := false
		passed := t.Run(fmt.Sprint(cut), func(t *testing.T) {
			f, gate, old := tallyJudgeFixture(t, true)
			reduce(t, f, true)
			original := tallyRound(t, f, gate).Verify
			newest := judgeCommit(t, f, "changed.txt", "J")
			reduce(t, f, true)
			restore := kernel.ReplaceRulesForTest(kernel.NewRegistry())
			judgeAttemptSubmit(t, f, gate, 1, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same behavior"})
			restore()
			probe, err := kernel.Pass(kernel.Rules(), f.State(), false)
			f.Must(err)
			finished = cut == len(probe.Writes)
			for _, w := range probe.Writes[:cut] {
				switch w.Kind {
				case "metadata":
					f.Must(f.Store.WriteMetadata(w.ID, w.Metadata))
				case "create":
					_, err := f.Store.Create(*w.Create)
					f.Must(err)
				case "close":
					row := record(t, f, w.ID)
					f.Must(f.Store.WriteReceipt(w.ID, *w.Receipt))
					f.Must(f.Store.SetStatusIf(w.ID, row.Status, "closed"))
				default:
					t.Fatalf("unexpected replacement write: %+v", w)
				}
			}
			reduce(t, f, true)
			reduce(t, f, true)
			r := tallyRound(t, f, gate)
			if r.Verify != original+".a2" {
				t.Fatalf("crash left an orphan verify or advanced its attempt: %s", r.Verify)
			}
			subject, _, err := kernel.Decode[kernel.Subject](record(t, f, r.Verify).Metadata, kernel.KeySubject)
			f.Must(err)
			if subject.Commit != newest {
				t.Fatalf("replacement subject = %s, want %s", subject.Commit, newest)
			}
			state := f.State()
			for _, id := range state.Steps {
				row := state.Records[id]
				tuple, _, _ := row.Tuple()
				if tuple.Kind == "verify" && tuple.Gate == gate && row.Status != "closed" && id != r.Verify {
					t.Fatalf("orphan open verify: %s", id)
				}
			}
			before := kerneltest.Snapshot(t, f)
			reduce(t, f, true)
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("replacement recovery did not converge")
			}
		})
		if !passed || finished {
			return
		}
	}
	t.Fatalf("crash-prefix probe exceeded %d cuts", maxCuts)
}

func TestS134Verify_ReproducedRejectCrashPrefixes(t *testing.T) {
	const maxCuts = 100
	for cut := 0; cut < maxCuts; cut++ {
		finished := false
		passed := t.Run(fmt.Sprint(cut), func(t *testing.T) {
			f := reducerFixture(t)
			gate := f.Items[0] + ".review"
			refusedPanel(t, f, gate, "review", 1, panel.VerdictReject)
			reduce(t, f, true)
			id, v := verifyRecord(t, f, gate, 99)
			lease := leaseActor(t, f, id, kernel.LeaseDeps{})
			restore := kernel.ReplaceRulesForTest(kernel.NewRegistry())
			_, err := submitVerify(t, f, v, lease, nil)
			restore()
			f.Must(err)
			probe, err := kernel.Pass(kernel.Rules(), f.State(), false)
			f.Must(err)
			finished = cut == len(probe.Writes)
			for _, w := range probe.Writes[:cut] {
				if w.Kind != "create" {
					t.Fatalf("unexpected reject outcome write: %+v", w)
				}
				_, err := f.Store.Create(*w.Create)
				f.Must(err)
			}
			reduce(t, f, true)
			reduce(t, f, true)
			decision, err := f.Store.Lookup(gate + ".decide.reject.r1")
			f.Must(err)
			if decision == nil || decision.Status != "open" {
				t.Fatal("resume lost reject decision after fix creation")
			}
			if record(t, f, gate+".r2.fix").Status != "open" {
				t.Fatal("reject recovery lost the original fix")
			}
			before := kerneltest.Snapshot(t, f)
			reduce(t, f, true)
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("reject outcome recovery did not converge")
			}
		})
		if !passed || finished {
			return
		}
	}
	t.Fatalf("crash-prefix probe exceeded %d cuts", maxCuts)
}
