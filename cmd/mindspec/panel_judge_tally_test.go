package main

import (
	"encoding/json"
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

func judgePanelFixture(t *testing.T, stray bool) (*kerneltest.Fixture, string, string, string) {
	t.Helper()
	kerneltest.SetClock(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	f, gate, _ := kernelPanelFixture(t)
	actorCmdGit(t, f, "symbolic-ref", "HEAD", "refs/heads/bead/"+f.Items[0])
	head := judgeCmdCommit(t, f, ".mindspec/config.yaml", "panel:\n  reviewers:\n    - family: harness\n      model: test-model\n      count: 5\n")
	kernelPanelAuthorise(f, gate, head, 1, 5)
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: []string{"R1", "R2", "R3", "R4", "R5"}})
	dir := filepath.Join(f.SpecDir, "reviews/review")
	if stray {
		f.Must(os.WriteFile(filepath.Join(dir, "R9-round-1.json"), []byte(`{"verdict":"INVALID"}`), 0600))
	}
	for i := 1; i <= 5; i++ {
		word := "APPROVE"
		if i == 5 {
			word = "REQUEST_CHANGES"
		}
		kernelPanelVerdict(t, f, gate, head, "review", fmt.Sprintf("R%d", i), word, 1)
	}
	return f, gate, head, dir
}

func panelJudgeSubmit(t *testing.T, f *kerneltest.Fixture, gate, old, newest, value string, n int, paths []string) {
	t.Helper()
	_, err := kernel.NextAction(f.State(), true)
	f.Must(err)
	judge := ""
	for _, id := range f.State().Steps {
		r := f.State().Records[id]
		tuple, _, _ := r.Tuple()
		if tuple.Kind == "change-judge" && r.Status != "closed" {
			judge = id
		}
	}
	if judge == "" {
		t.Fatal("no current change judge")
	}
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: judge}, kernel.LeaseDeps{})
	f.Must(err)
	j := kernel.Judgement{Value: value, OldSubject: old, NewSubject: newest, Reason: "same behavior", SlotCount: n, Paths: paths}
	if value == "re-panel" {
		j.Reason = ""
	}
	data, err := json.Marshal(j)
	f.Must(err)
	file := filepath.Join(t.TempDir(), "judgement.json")
	f.Must(os.WriteFile(file, data, 0600))
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: judge, Lease: lease.Token, Record: file})
	f.Must(err)
}

func panelRefuteF(t *testing.T, f *kerneltest.Fixture) {
	t.Helper()
	kerneltest.SetClock(t, kernel.Now().Add(time.Second))
	out, err := kernelPanelRun(t, panelRefuteCmd, "review", "--spec", f.SpecID, "--slot", "R5", "--round", "1", "--finding", "ccr[0]", "--by", "operator", "--ground", "disproven", "--reason", "checked", "--evidence", "proof.txt")
	kernelPanelRequire(t, out, err)
}

func TestS134PanelRefute_FrozenSubjectAndReadSet(t *testing.T) {
	for _, change := range []string{"fix", "edited", "retry-edited", "added", "removed-committed", "removed-uncommitted", "removed-never-committed"} {
		t.Run(change, func(t *testing.T) {
			stray := strings.HasPrefix(change, "removed")
			f, gate, head, dir := judgePanelFixture(t, stray)
			if change != "removed-never-committed" {
				actorCmdGit(t, f, "add", "-f", "--", dir)
			} else {
				for _, name := range []string{"R1-round-1.json", "R2-round-1.json", "R3-round-1.json", "R4-round-1.json", "R5-round-1.json", panel.FileName, "dispositions.jsonl"} {
					actorCmdGit(t, f, "add", "-f", "--", filepath.Join(dir, name))
				}
			}
			actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "panel evidence")
			v := kernelPanelVerifyRecord(t, f, gate, 99)
			if change == "retry-edited" {
				lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: v.Step}, kernel.LeaseDeps{})
				f.Must(err)
				f.Must(f.Store.SetStatusIf(v.Step, "in_progress", "closed"))
				_ = lease
				original := kernelPanelRecord(t, f, v.Step).Metadata[kernel.KeyRead]
				v = kernelPanelVerifyRecord(t, f, gate, 99)
				if !strings.HasSuffix(v.Step, ".a2") || !reflect.DeepEqual(original, kernelPanelRecord(t, f, v.Step).Metadata[kernel.KeyRead]) {
					t.Fatal("verify retry lost pinned read set")
				}
			}
			kernelPanelSubmitVerify(t, f, v)
			fix := gate + ".r2.fix"
			changedPath := ""
			switch change {
			case "fix":
				judgeCmdCommit(t, f, "fixed.txt", "fix")
			case "edited", "retry-edited":
				changedPath = filepath.Join(dir, "R2-round-1.json")
				bytes, err := os.ReadFile(changedPath)
				f.Must(err)
				f.Must(os.WriteFile(changedPath, append(bytes, '\n'), 0600))
				_, err = panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "review", Slot: "R2", Round: 1}, append(bytes, '\n'), func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
					return panel.SubmitResolution{Family: "harness", Model: "test-model", RunnerKind: "harness", RunnerSource: "default"}, nil
				})
				f.Must(err)
			case "added":
				changedPath = filepath.Join(dir, "R9-round-1.json")
				f.Must(os.WriteFile(changedPath, []byte(`{"verdict":"APPROVE"}`), 0600))
			default:
				changedPath = filepath.Join(dir, "R9-round-1.json")
				f.Must(os.Rename(changedPath, filepath.Join(t.TempDir(), "quarantined-verdict.json")))
			}
			if change == "edited" || change == "retry-edited" || change == "added" || change == "removed-committed" {
				actorCmdGit(t, f, "add", "-f", "--", changedPath)
				actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "panel evidence change")
			}
			_, err := kernel.NextAction(f.State(), true)
			f.Must(err)
			if judge, _ := f.Store.Lookup(gate + ".judge.a1"); judge != nil {
				t.Fatal("case (a) acted on a frozen round")
			}
			panelRefuteF(t, f)
			g := kernelPanelRecord(t, f, gate)
			if change == "fix" {
				receipt, _, _ := kernel.Decode[kernel.Receipt](g.Metadata, kernel.KeyClose)
				fixReceipt, _, _ := kernel.Decode[kernel.Receipt](kernelPanelRecord(t, f, fix).Metadata, kernel.KeyClose)
				if g.Status != "closed" || receipt.Subject != head || fixReceipt.Kind != "superseded" {
					t.Fatalf("frozen retally did not close on review subject: gate=%+v fix=%+v", g, fixReceipt)
				}
				// The verb's final pass judges the later fix commit only after the gate passed.
				if kernelPanelRecord(t, f, gate+".judge.a1").Status == "closed" {
					t.Fatal("later fix commit has no case-b judge")
				}
			} else {
				if g.Status == "closed" || kernelPanelRecord(t, f, fix).Status != "open" {
					t.Fatal("changed frozen verdict set passed the gate or closed the fix")
				}
				if change == "edited" || change == "retry-edited" {
					// The stored record stays the verdict. The edited file
					// differs from it, so the tally does not count it.
					result, err := kernel.TallyPanel(f.State(), panel.Registration{Dir: dir})
					f.Must(err)
					uncounted := false
					for _, v := range result.NotIn {
						uncounted = uncounted || v.File == filepath.Base(changedPath) && strings.Contains(v.Reason, "differs from the stored record")
					}
					if !uncounted {
						t.Fatalf("edited verdict file was counted: %+v", result.NotIn)
					}
				} else {
					result, err := panel.Tally(dir)
					f.Must(err)
					err = kernel.VerifyRetallyFits(f.State(), gate, result)
					if err == nil || !strings.Contains(err.Error(), filepath.Base(changedPath)) {
						t.Fatalf("read set did not name changed path: %v", err)
					}
				}
			}
			forbidden := gate + ".r1.verify.a2"
			if change == "retry-edited" {
				forbidden = gate + ".r1.verify.a3"
			}
			if r, _ := f.Store.Lookup(forbidden); r != nil {
				t.Fatal("frozen round created another verify")
			}
		})
	}
}

func TestS134PanelRefute_JudgeHoldsRetally(t *testing.T) {
	f, gate, _, _ := judgePanelFixture(t, false)
	judgeCmdCommit(t, f, "changed.txt", "J")
	_, err := kernel.NextAction(f.State(), true)
	f.Must(err)
	panelRefuteF(t, f)
	if kernelPanelRecord(t, f, gate).Status == "closed" {
		t.Fatal("passing retally closed gate while judge open")
	}
	if r, _ := f.Store.Lookup(gate + ".r1.verify"); r != nil {
		t.Fatal("held retally created verify")
	}
}

func TestS134PanelRefute_RePanelAuthorisation(t *testing.T) {
	for _, reproduced := range []bool{false, true} {
		t.Run(fmt.Sprint(reproduced), func(t *testing.T) {
			f, gate, old, _ := judgePanelFixture(t, false)
			newest := judgeCmdCommit(t, f, "changed.txt", "J")
			panelJudgeSubmit(t, f, gate, old, newest, "re-panel", 2, []string{"changed.txt"})
			v := kernelPanelVerifyRecord(t, f, gate, 99)
			if reproduced {
				kernelPanelSubmitVerify(t, f, v)
			}
			panelRefuteF(t, f)
			r, _, err := kernel.Decode[kernel.GateRound](kernelPanelRecord(t, f, gate).Metadata, kernel.KeyRound)
			f.Must(err)
			if kernelPanelRecord(t, f, gate).Status == "closed" || r.Round != 2 || r.SlotCount != 2 || r.Subject != newest || !reflect.DeepEqual(r.Paths, []string{"changed.txt"}) || r.Prior == nil {
				t.Fatalf("passing retally did not authorize scoped round: %+v", r)
			}
			if reproduced {
				receipt, _, _ := kernel.Decode[kernel.Receipt](kernelPanelRecord(t, f, gate+".r2.fix").Metadata, kernel.KeyClose)
				if receipt.Kind != "superseded" || r.Prior.Verify != v.Step || len(r.Prior.Asks) != 1 || len(r.Prior.Asks[0].Returns) != 2 {
					t.Fatalf("fix retally lost verify and verb evidence: %+v", r.Prior)
				}
			} else {
				receipt, _, _ := kernel.Decode[kernel.Receipt](kernelPanelRecord(t, f, v.Step).Metadata, kernel.KeyClose)
				if receipt.Kind != "superseded" || r.Prior.Verify != "" || len(r.Prior.Asks) != 1 || r.Prior.Asks[0].Returns[0].Return != "refuted" {
					t.Fatalf("open verify retally lost verb evidence: %+v", r.Prior)
				}
			}
		})
	}
}

func TestS134PanelRefute_LimitReattempt(t *testing.T) {
	f, gate, old, _ := judgePanelFixture(t, false)
	policy := f.State().Policy
	policy.RoundLimit = 1
	f.Must(f.Store.WriteMetadata(f.Epic, kernel.Metadata(kernel.KeyRun, policy)))
	newest := judgeCmdCommit(t, f, "changed.txt", "J")
	panelJudgeSubmit(t, f, gate, old, newest, "re-panel", 2, []string{"changed.txt"})
	kernelPanelReproduce(t, f, gate)
	first := gate + ".decide.limit.r1"
	if kernelPanelRecord(t, f, first).Status != "open" {
		t.Fatal("missing first limit decision")
	}
	panelRefuteF(t, f)
	receipt, _, _ := kernel.Decode[kernel.Receipt](kernelPanelRecord(t, f, first).Metadata, kernel.KeyClose)
	if receipt.Kind != "superseded" || kernelPanelRecord(t, f, first+".a2").Status != "open" {
		t.Fatal("scoped retally reused superseded limit decision")
	}
	_, err := kernel.Answer(f.State(), first+".a2", "continue:1")
	f.Must(err)
	if fix, _ := f.Store.Lookup(gate + ".r2.fix"); fix != nil {
		t.Fatal("scoped continuation created a fix")
	}
	kernelPanelDraftLens(t, f, gate)
	out, err := kernelPanelCreate(t, f, "review", "--bead", f.Items[0], "--gate", "bead")
	kernelPanelRequire(t, out, err)
	r, _, err := kernel.Decode[kernel.GateRound](kernelPanelRecord(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	if r.Round != 2 || r.SlotCount != 2 {
		t.Fatalf("continued scope did not register: %+v", r)
	}
}

func TestS134JudgeCmd_OpenTallyOutcomes(t *testing.T) {
	for _, value := range []string{"carry-over", "re-panel"} {
		t.Run(value, func(t *testing.T) {
			e := finalKernelFixture(t)
			e.apply()
			e.create(e.implement(0))
			item := e.f.Items[0]
			gate := item + ".review"
			newest := e.commit(e.tree(item), "during-round.txt", "new subject")
			out := e.review("APPROVE")
			if e.row(gate).Status == "closed" {
				t.Fatal("moved-subject tally passed before judgement")
			}
			found := false
			for _, a := range out.Actions {
				found = found || a.Kind == "change-judge"
			}
			if !found {
				t.Fatalf("last verdict produced no judge: %+v", out)
			}
			n := 0
			if value == "re-panel" {
				n = 2
			}
			before := e.git(e.tree(item), "status", "--porcelain")
			e.judge(value, n)
			if before != e.git(e.tree(item), "status", "--porcelain") {
				t.Fatal("judge record changed git status")
			}
			if value == "re-panel" {
				e.create(e.apply())
				repanelReview(t, e, gate, 2, "APPROVE", []string{"during-round.txt"})
			}
			receipt, _, err := kernel.Decode[kernel.Receipt](e.row(gate).Metadata, kernel.KeyClose)
			e.must(err)
			if receipt.Subject != newest {
				t.Fatalf("passing receipt lost review subject: %+v", receipt)
			}
			e.must(kernel.CheckPassedGate(e.f.State(), gate))
			e.ok("complete", item)
			for _, a := range e.apply().Actions {
				if a.Kind == "change-judge" {
					t.Fatalf("completed judged subject created a judge: %+v", a)
				}
			}
		})
	}
}

func TestS134PanelRefute_JudgeHoldsOpenVerify(t *testing.T) {
	for _, verb := range []string{"refute", "defer"} {
		for _, open := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/open=%v", verb, open), func(t *testing.T) {
				f, gate, _, _ := judgePanelFixture(t, false)
				v := kernelPanelVerifyRecord(t, f, gate, 99)
				judgeCmdCommit(t, f, "changed.txt", "J")
				if open {
					_, err := kernel.NextAction(f.State(), true)
					f.Must(err)
					if j, _ := f.Store.Lookup(gate + ".judge.a1"); j == nil {
						t.Fatal("setup: no judge")
					}
				}
				if verb == "refute" {
					panelRefuteF(t, f)
				} else {
					fake := fakeDeferralBD(t)
					rebindWrite(t, fake, "epics.json", fmt.Sprintf(`[{"id":%q,"issue_type":"epic","metadata":{"spec_id":%q}}]`, f.Epic, f.SpecID))
					rebindWrite(t, fake, "follow-up.json", fmt.Sprintf(`[{"id":"follow-up","status":"open","description":"missing test","dependencies":[{"id":%q,"dependency_type":"discovered-from"}]}]`, f.Epic))
					out, err := kernelPanelRun(t, panelDeferCmd, "review", "--spec", f.SpecID, "--slot", "R5", "--round", "1", "--finding", "ccr[0]", "--bead", "follow-up", "--by", "operator")
					kernelPanelRequire(t, out, err)
				}
				verify := kernelPanelRecord(t, f, v.Step)
				if verify.Status == "closed" {
					t.Fatal("held retally closed open verify")
				}
				if _, has := verify.Metadata[kernel.KeyClose]; has {
					t.Fatal("held retally receipted open verify")
				}
				if open && kernelPanelRecord(t, f, gate+".judge.a1").Status != "open" {
					t.Fatal("held retally changed open judge status")
				}
				if kernelPanelRecord(t, f, gate).Status == "closed" {
					t.Fatal("held retally closed gate")
				}
			})
		}
	}
}

func TestS134PanelRefute_LimitReattemptCrashRecovery(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		t.Run(fmt.Sprint(interrupted), func(t *testing.T) {
			f, gate, old, _ := judgePanelFixture(t, false)
			policy := f.State().Policy
			policy.RoundLimit = 1
			f.Must(f.Store.WriteMetadata(f.Epic, kernel.Metadata(kernel.KeyRun, policy)))
			newest := judgeCmdCommit(t, f, "changed.txt", "J")
			panelJudgeSubmit(t, f, gate, old, newest, "re-panel", 2, []string{"changed.txt"})
			kernelPanelReproduce(t, f, gate)
			first := gate + ".decide.limit.r1"
			if kernelPanelRecord(t, f, first).Status != "open" {
				t.Fatal("missing first limit decision")
			}
			restore := kernel.ReplaceRulesForTest(kernel.NewRegistry())
			panelRefuteF(t, f)
			restore()
			probe, err := kernel.Pass(kernel.Rules(), f.State(), false)
			f.Must(err)
			cut := false
			for _, w := range probe.Writes {
				if w.Kind == "create" && w.ID == first+".a2" {
					cut = true
					break
				}
				if w.Kind != "metadata" {
					t.Fatalf("unexpected pre-cut write: %+v", w)
				}
				if interrupted {
					f.Must(f.Store.WriteMetadata(w.ID, w.Metadata))
				}
			}
			if !cut {
				t.Fatal("positive control: pass did not propose replacement limit decision")
			}
			_, err = kernel.Pass(kernel.Rules(), f.State(), true)
			f.Must(err)
			_, err = kernel.Pass(kernel.Rules(), f.State(), true)
			f.Must(err)
			r, _, err := kernel.Decode[kernel.GateRound](kernelPanelRecord(t, f, gate).Metadata, kernel.KeyRound)
			f.Must(err)
			if r.Round != 2 {
				t.Fatalf("scoped authorization missing: %+v", r)
			}
			decision, err := f.Store.Lookup(first + ".a2")
			f.Must(err)
			if decision == nil {
				t.Fatal("resume lost replacement limit decision after scoped retally authorization")
			}
		})
	}
}
