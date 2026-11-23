package kernel_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func verifyResolution(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
	return panel.SubmitResolution{Family: "harness", Model: "test-model", RunnerKind: "harness", RunnerSource: "default"}, nil
}

func verifyRecord(t *testing.T, f *kerneltest.Fixture, gate string, reproduced int) (string, kernel.VerifyRecord) {
	t.Helper()
	round, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	step := record(t, f, round.Verify)
	findings, _, err := kernel.Decode[[]kernel.Finding](step.Metadata, kernel.KeyFindings)
	f.Must(err)
	sub, _, err := kernel.Decode[kernel.Subject](step.Metadata, kernel.KeySubject)
	f.Must(err)
	v := kernel.VerifyRecord{Step: step.Key(), Subject: sub.Commit, Returns: []kernel.VerifyReturn{}}
	for i, finding := range findings {
		ret := kernel.VerifyReturn{Slot: finding.Slot, Round: finding.Round, Finding: finding.Finding, FindingSHA256: finding.FindingSHA256, Return: "refuted", Ground: "disproven", Reason: "local check", Evidence: "https://example.invalid/evidence"}
		if i < reproduced {
			ret.Return, ret.Ground, ret.Reason, ret.Evidence = "reproduced", "", "", ""
		}
		v.Returns = append(v.Returns, ret)
	}
	return step.Key(), v
}

func submitVerify(t *testing.T, f *kerneltest.Fixture, v kernel.VerifyRecord, lease kernel.LeaseResult, before func(kernel.State, kernel.Receipt) error) (kernel.Output, error) {
	t.Helper()
	data, err := json.Marshal(v)
	f.Must(err)
	return kernel.Submit(f.State(), kernel.SubmitOptions{Step: v.Step, Lease: lease.Token, Record: recordFile(t, string(data)), BeforeClose: before})
}

func reproduceVerify(t *testing.T, f *kerneltest.Fixture, gate string) kernel.Output {
	t.Helper()
	reduce(t, f, true)
	step, v := verifyRecord(t, f, gate, int(^uint(0)>>1))
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: step}, kernel.LeaseDeps{})
	f.Must(err)
	out, err := submitVerify(t, f, v, lease, nil)
	f.Must(err)
	return out
}

func TestS134Verify_Outcomes(t *testing.T) {
	for _, final := range []bool{false, true} {
		for _, verdict := range []string{panel.VerdictRequestChanges, panel.VerdictReject} {
			for _, reproduced := range []int{0, 1} {
				t.Run(fmt.Sprintf("final=%t/%s/reproduced=%d", final, verdict, reproduced), func(t *testing.T) {
					f := reducerFixture(t)
					gate := f.Items[0] + ".review"
					if final {
						for _, item := range f.Items {
							doneItem(t, f, item)
						}
						gate = f.Epic + ".final"
					}
					refusedPanel(t, f, gate, "review", 1, verdict)
					before := kerneltest.Snapshot(t, f)
					probe := reduce(t, f, false)
					if !action(probe, gate+".r1.verify") || action(probe, gate+".r2.fix") || len(probe.Decisions) != 0 || kerneltest.Snapshot(t, f) != before {
						t.Fatalf("refused round did not propose only a read-only verify: %+v", probe)
					}
					reduce(t, f, true)
					count, err := kernel.RoundsForGate(f.State(), gate)
					f.Must(err)
					id, v := verifyRecord(t, f, gate, reproduced)
					lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
					f.Must(err)
					out, err := submitVerify(t, f, v, lease, nil)
					f.Must(err)
					after, err := kernel.RoundsForGate(f.State(), gate)
					f.Must(err)
					if count != after {
						t.Fatalf("verify added a round: %d -> %d", count, after)
					}
					g := record(t, f, gate)
					if reproduced == 0 {
						receipt, _, _ := kernel.Decode[kernel.Receipt](g.Metadata, kernel.KeyClose)
						if g.Status != "closed" || receipt.Kind != "passed" || receipt.Retally != "verify" {
							t.Fatalf("no reproduced findings must pass by verify: %+v", g)
						}
						f.Must(kernel.CheckPassedGate(f.State(), gate))
						fix, err := f.Store.Lookup(gate + ".r2.fix")
						f.Must(err)
						if fix != nil || len(out.Decisions) != 0 {
							t.Fatalf("all-refuted verify created an outcome: %+v", out)
						}
					} else {
						fix := record(t, f, gate+".r2.fix")
						findings, _, err := kernel.Decode[[]kernel.Finding](fix.Metadata, kernel.KeyReproduced)
						f.Must(err)
						if g.Status == "closed" || len(findings) != 1 || findings[0].Slot != v.Returns[0].Slot || findings[0].FindingSHA256 != v.Returns[0].FindingSHA256 {
							t.Fatalf("fix scope is not exactly the reproduced finding: %+v", fix)
						}
						wantDecisions := 0
						if verdict == panel.VerdictReject {
							wantDecisions = 1
						}
						if len(out.Decisions) != wantDecisions {
							t.Fatalf("wrong reject decision: %+v", out)
						}
					}
				})
			}
		}
	}
}

func TestS134Verify_VetoFiles(t *testing.T) {
	for _, unexpected := range []bool{false, true} {
		t.Run(fmt.Sprint(unexpected), func(t *testing.T) {
			f := reducerFixture(t)
			gate := f.Items[0] + ".review"
			registerGate(t, f, gate, "review", 1)
			dir := filepath.Join(f.SpecDir, "reviews", "review")
			for i := 1; i <= 3; i++ {
				v := paneltest.Verdict(panel.VerdictApprove)
				if !unexpected && i == 1 {
					v = paneltest.Verdict(panel.VerdictRequestChanges)
					v.HardBlock = true
					v.ConcreteChangesRequired[0].Text = "retain this full veto change"
				}
				v.Slot, v.ReviewedSHA, v.Model = fmt.Sprintf("R%d", i), gateHead(t, f, gate), "test-model"
				_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "review", Slot: v.Slot, Round: 1}, paneltest.Bytes(t, v), verifyResolution)
				f.Must(err)
				closePass(t, f, gate+".r1."+v.Slot)
			}
			if unexpected {
				put(t, filepath.Join(dir, "R9-round-1.json"), `{"verdict":"REJECT","rationale":"stray veto"}`)
			}
			reduce(t, f, true)
			id, v := verifyRecord(t, f, gate, 1)
			findings, _, err := kernel.Decode[[]kernel.Finding](record(t, f, id).Metadata, kernel.KeyFindings)
			f.Must(err)
			if len(findings) != 1 || findings[0].Finding != "veto" || unexpected && findings[0].Slot != "R9" {
				t.Fatalf("veto files did not give exactly one veto: %+v", findings)
			}
			if !unexpected && !strings.Contains(findings[0].Text, "retain this full veto change") {
				t.Fatalf("veto lost its fix text: %+v", findings)
			}
			lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
			f.Must(err)
			bad := v
			bad.Returns = append([]kernel.VerifyReturn{}, v.Returns...)
			bad.Returns[0].Finding = "ccr[0]"
			_, err = submitVerify(t, f, bad, lease, nil)
			if err == nil || !strings.Contains(err.Error(), "unknown") || record(t, f, id).Status == "closed" {
				t.Fatalf("a veto accepted a ccr return: %v", err)
			}
			out, err := submitVerify(t, f, v, lease, nil)
			f.Must(err)
			if unexpected && len(out.Decisions) != 1 || !unexpected && len(out.Decisions) != 0 {
				t.Fatalf("hard-block and REJECT veto decisions differ: %+v", out)
			}
		})
	}
}

func TestS134Verify_CrashAndConflict(t *testing.T) {
	for _, conflict := range []string{"none", "bytes", "take"} {
		t.Run(conflict, func(t *testing.T) {
			t.Cleanup(kernel.ReplaceCloseWriteHookForTest("verify", func(s kernel.State, r kernel.Record, receipt kernel.Receipt) ([]kernel.Write, error) {
				writes, err := kernel.VerifyCloseWritesForTest(s, r, receipt)
				if err != nil {
					return nil, err
				}
				for _, w := range writes {
					if w.Panel == nil {
						continue
					}
					for _, line := range w.Panel.Records {
						var event struct {
							CreatedAt string `json:"created_at"`
						}
						if err := json.Unmarshal(line, &event); err != nil {
							t.Fatal(err)
						}
						if event.CreatedAt != receipt.At {
							t.Fatalf("prepared record or row created_at %s differs from closing at %s", event.CreatedAt, receipt.At)
						}
					}
				}
				return writes, nil
			}))
			f := reducerFixture(t)
			gate := f.Items[0] + ".review"
			refusedPanel(t, f, gate, "review", 1, panel.VerdictRequestChanges)
			approval := paneltest.Verdict(panel.VerdictApprove)
			approval.Slot, approval.ReviewedSHA, approval.Model = "R3", gateHead(t, f, gate), "test-model"
			_, err := panel.Submit(panel.SubmitTarget{Dir: filepath.Join(f.SpecDir, "reviews", "review"), Spec: f.SpecID, Panel: "review", Slot: "R3", Round: 1}, paneltest.Bytes(t, approval), verifyResolution)
			f.Must(err)
			reduce(t, f, true)
			id, v := verifyRecord(t, f, gate, 0)
			lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
			f.Must(err)
			killed := errors.New("killed after close writes")
			_, err = submitVerify(t, f, v, lease, func(kernel.State, kernel.Receipt) error { return killed })
			if !errors.Is(err, killed) {
				t.Fatalf("kill seam did not run: %v", err)
			}
			r := record(t, f, id)
			pin, _, err := kernel.Decode[kernel.Closing](r.Metadata, kernel.KeyClosing)
			f.Must(err)
			if r.Status == "closed" || r.Metadata[kernel.KeyClose] != nil {
				t.Fatalf("crash fixture has a receipt: %+v", r)
			}
			kerneltest.SetClock(t, fixedTime.Add(time.Second))
			if conflict != "none" {
				v.Returns[1].Return = "reproduced"
				v.Returns[0].Evidence = "https://example.invalid/new-evidence"
				if conflict == "bytes" {
					// The pin remains binding at a later commit. A panel-only
					// commit does not replace this round's review subject.
					reducerGit(t, f, "symbolic-ref", "HEAD", "refs/heads/bead/"+f.Items[0])
					reducerGit(t, f, "add", "-f", "--", filepath.Join(f.SpecDir, "reviews", "review"))
					reducerGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "persist crash-era panel evidence")
				}
				if conflict == "take" {
					lease, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "retry after crash"}, kernel.LeaseDeps{})
					f.Must(err)
				}
			}
			_, err = submitVerify(t, f, v, lease, nil)
			if conflict == "none" {
				f.Must(err)
				returns, _, err := kernel.Decode[[]kernel.VerifyReturn](record(t, f, id).Metadata, kernel.KeyReturns)
				f.Must(err)
				if !reflect.DeepEqual(returns, v.Returns) {
					t.Fatal("matching retry did not restore mindspec_returns")
				}
				receipt, _, _ := kernel.Decode[kernel.Receipt](record(t, f, id).Metadata, kernel.KeyClose)
				if receipt.At != pin.At.UTC().Format(time.RFC3339Nano) {
					t.Fatalf("rerun changed closing time: %+v %+v", pin, receipt)
				}
				res, err := panel.Tally(filepath.Join(f.SpecDir, "reviews", "review"))
				f.Must(err)
				if len(res.Panel.Refutations) != len(v.Returns) {
					t.Fatal("matching retry duplicated panel refutations")
				}
				for _, ref := range res.Panel.Refutations {
					if ref.CreatedAt != receipt.At {
						t.Fatal("matching retry changed a panel refutation's created_at")
					}
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "mindspec_closing") {
					t.Fatalf("conflicting submission accepted: %v", err)
				}
				reduce(t, f, true)
				next := record(t, f, id+".a2")
				for _, key := range []string{kernel.KeyFindings, kernel.KeyRead} {
					if !reflect.DeepEqual(r.Metadata[key], next.Metadata[key]) {
						t.Fatalf("conflict retry lost %s", key)
					}
				}
				res, err := panel.Tally(filepath.Join(f.SpecDir, "reviews", "review"))
				f.Must(err)
				if kernel.FilterDispositions(f.State(), gate, res).PassRule(true) {
					t.Fatal("unfinished retired verify discharged findings")
				}
				lease, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: next.Key()}, kernel.LeaseDeps{})
				f.Must(err)
				v.Step = next.Key()
				_, err = submitVerify(t, f, v, lease, nil)
				f.Must(err)
				fix := record(t, f, gate+".r2.fix")
				reproduced, _, _ := kernel.Decode[[]kernel.Finding](fix.Metadata, kernel.KeyReproduced)
				if len(reproduced) != 1 || reproduced[0].Slot != v.Returns[1].Slot {
					t.Fatalf("retired records changed fix scope: %+v", reproduced)
				}
			}
			lines, err := panel.ReadLedger(filepath.Join(f.SpecDir, "reviews", "review"))
			f.Must(err)
			counts := map[string]int{}
			for _, line := range lines {
				var event panel.RefutationRecord
				f.Must(json.Unmarshal(line, &event))
				if event.Record == panel.RecordRefutation {
					counts[event.VerifyStep+"/"+event.Slot]++
					if event.VerifyStep == id && event.CreatedAt != pin.At.UTC().Format(time.RFC3339Nano) {
						t.Fatal("rerun changed a record's created_at")
					}
				}
				var row panel.DispositionRow
				f.Must(json.Unmarshal(line, &row))
				if row.Record == panel.RecordDisposition && strings.HasSuffix(row.ID, ":"+id) && row.CreatedAt != pin.At.UTC().Format(time.RFC3339Nano) {
					t.Fatal("rerun changed a row's created_at")
				}
			}
			for _, ret := range v.Returns {
				if counts[id+"/"+ret.Slot] != 1 {
					t.Fatalf("crash retry duplicated or lost records: %+v", counts)
				}
			}
		})
	}
}

func TestS134Verify_FilterTimesAndCopy(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "review", 1, panel.VerdictRequestChanges)
	reduce(t, f, true)
	id, v := verifyRecord(t, f, gate, 1)
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	_, err = submitVerify(t, f, v, lease, nil)
	f.Must(err)
	for _, tc := range []struct {
		created, at string
		keep        bool
	}{{"2026-01-02T03:04:05Z", "2026-01-02T03:04:05.5Z", false}, {"2026-01-02T03:04:05.5Z", "2026-01-02T03:04:05.5Z", false}, {"2026-01-02T03:04:05.6Z", "2026-01-02T03:04:05.5Z", true}} {
		r := record(t, f, id)
		receipt, _, _ := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
		receipt.At = tc.at
		f.Must(f.Store.WriteReceipt(id, receipt))
		if got := kernel.KeepDisposition(f.State(), gate, 1, "", tc.created); got != tc.keep {
			t.Fatalf("parsed time filter(%s, %s)=%t want %t", tc.created, tc.at, got, tc.keep)
		}
	}
	res, err := panel.Tally(filepath.Join(f.SpecDir, "reviews", "review"))
	f.Must(err)
	res.Panel.Refutations = append(res.Panel.Refutations, panel.Refutation{Slot: "R1", Round: 1, VerifyStep: gate + ".r1.verify.a99", Finding: "ccr[0]"})
	before, err := json.Marshal(res.Panel)
	f.Must(err)
	filtered := kernel.FilterDispositions(f.State(), gate, res)
	filtered.Panel.Refutations = nil
	after, err := json.Marshal(res.Panel)
	f.Must(err)
	if string(before) != string(after) || filtered.Panel == res.Panel {
		t.Fatal("filter mutated the tallied panel")
	}
}

func TestS134Verify_Incomplete(t *testing.T) {
	for _, bad := range []string{"VOID", "INVALID", "edited"} {
		t.Run(bad, func(t *testing.T) {
			f := reducerFixture(t)
			gate := f.Items[0] + ".review"
			registerGate(t, f, gate, "review", 1)
			head := gateHead(t, f, gate)
			dir := filepath.Join(f.SpecDir, "reviews", "review")
			for i := 1; i <= 3; i++ {
				word := panel.VerdictApprove
				if bad == "VOID" && i == 1 {
					word = "VOID"
				}
				v := paneltest.Verdict(word)
				v.Slot, v.ReviewedSHA, v.Model = fmt.Sprintf("R%d", i), head, "test-model"
				_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "review", Slot: v.Slot, Round: 1}, paneltest.Bytes(t, v), verifyResolution)
				f.Must(err)
				closePass(t, f, gate+".r1."+v.Slot)
				if i == 1 && bad != "VOID" {
					content := `{"verdict":"INVALID"}`
					if bad == "edited" {
						content = string(paneltest.Bytes(t, v)) + " "
					}
					put(t, filepath.Join(dir, "R1-round-1.json"), content)
					if bad == "edited" {
						reducerGit(t, f, "symbolic-ref", "HEAD", "refs/heads/bead/"+f.Items[0])
						reducerGit(t, f, "add", "-f", "--", dir)
						reducerGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "edit closed verdict before final slot")
					}
				}
			}
			out := reduce(t, f, true)
			if action(out, gate+".r1.verify") || !action(out, gate+".r1.R1.a2") {
				t.Fatalf("incomplete tally gave a verify instead of slot a2: %+v", out)
			}
			v := paneltest.Verdict(panel.VerdictApprove)
			v.ReviewedSHA, v.Model = head, "test-model"
			_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "review", Slot: "R1", Round: 1}, paneltest.Bytes(t, v), verifyResolution)
			f.Must(err)
			id := gate + ".r1.R1.a2"
			lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
			f.Must(err)
			reducerGit(t, f, "symbolic-ref", "HEAD", "refs/heads/bead/"+f.Items[0])
			reducerGit(t, f, "add", "-f", "--", dir)
			reducerGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "replacement approval")
			_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: filepath.Join(dir, "R1-round-1.json")})
			f.Must(err)
			f.Must(kernel.CheckPassedGate(f.State(), gate))
		})
	}
}

func TestS134Verify_ChangedBeforeApply(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "review", 1, panel.VerdictRequestChanges)
	reduce(t, f, true)
	id, v := verifyRecord(t, f, gate, 0)
	// Commit names accepted by panel refute are accepted by verify too.
	v.Returns[0].Evidence = "HEAD"
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	_, err = submitVerify(t, f, v, lease, func(kernel.State, kernel.Receipt) error {
		for i := 1; i <= 3; i++ {
			slot := fmt.Sprintf("R%d", i)
			verdict := paneltest.Verdict(panel.VerdictApprove)
			verdict.Slot, verdict.ReviewedSHA, verdict.Model = slot, v.Subject, "test-model"
			_, err := panel.Submit(panel.SubmitTarget{Dir: filepath.Join(f.SpecDir, "reviews", "review"), Spec: f.SpecID, Panel: "review", Slot: slot, Round: 1}, paneltest.Bytes(t, verdict), verifyResolution)
			f.Must(err)
		}
		return nil
	})
	f.Must(err)
	round, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	if round.Verify != "" || record(t, f, gate).Status == "closed" {
		t.Fatal("changed findings applied the stale verify")
	}
	reduce(t, f, true)
	f.Must(kernel.CheckPassedGate(f.State(), gate))
	receipt, _, _ := kernel.Decode[kernel.Receipt](record(t, f, gate).Metadata, kernel.KeyClose)
	if receipt.Retally == "verify" {
		t.Fatal("stale returns supplied passing evidence")
	}
}

func TestS134Verify_Frozen(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "review", 1, panel.VerdictRequestChanges)
	reproduceVerify(t, f, gate)
	fix := gate + ".r2.fix"
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: fix}, kernel.LeaseDeps{})
	f.Must(err)
	before := record(t, f, fix).Metadata[kernel.KeyReproduced]
	dir := filepath.Join(f.SpecDir, "reviews", "review")
	put(t, filepath.Join(dir, "R1-round-1.json"), `{"verdict":"APPROVE"}`)
	put(t, filepath.Join(dir, "R9-round-1.json"), `{"verdict":"REJECT"}`)
	out := reduce(t, f, true)
	if action(out, gate+".r1.verify.a2") || action(out, gate+".r1.R1.a2") || !reflect.DeepEqual(before, record(t, f, fix).Metadata[kernel.KeyReproduced]) {
		t.Fatalf("changed files thawed the applied verify: %+v", out)
	}
	extra := gate + ".r1.R2.a2"
	f.Create(extra, gate)
	reducerGit(t, f, "symbolic-ref", "HEAD", "refs/heads/bead/"+f.Items[0])
	commit := actorCommit(t, f, "fixed.txt", "fixed", "bead/"+f.Items[0])
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: fix, Lease: lease.Token, Commit: commit})
	f.Must(err)
	round, _, _ := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	if round.Round != 2 || round.Prior == nil || round.Prior.Verify != gate+".r1.verify" || record(t, f, extra).Status != "closed" {
		t.Fatalf("next round lost the frozen prior or open slot: %+v", round)
	}
}

func TestS134Verify_RetiredAttempt(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "review", 1, panel.VerdictRequestChanges)
	reduce(t, f, true)
	id, v := verifyRecord(t, f, gate, 0)
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	newSubject := ""
	_, err = submitVerify(t, f, v, lease, func(kernel.State, kernel.Receipt) error {
		newSubject = beadCommit(t, f, "bead/"+f.Items[0])
		return nil
	})
	f.Must(err)
	if record(t, f, gate).Status == "closed" {
		t.Fatal("moved subject applied the old verify before judgement")
	}
	judge := gate + ".judge.a1"
	round, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	// The judgement producer is separate from the verify transition. Seed its
	// carry-over result, then exercise the real retirement and close writers.
	f.Must(f.Store.WriteReceipt(judge, kernel.Receipt{Kind: "superseded", Subject: newSubject}))
	f.Must(f.Store.SetStatusIf(judge, "open", "closed"))
	round.Judged = &kernel.Judged{Judge: judge, Value: "carry-over", NewSubject: newSubject}
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRound, round)))
	reduce(t, f, true)
	next, v2 := verifyRecord(t, f, gate, 1)
	if next != id+".a2" || v2.Subject != newSubject {
		t.Fatalf("judged subject did not retire the old verify: %+v", v2)
	}
	lease, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: next}, kernel.LeaseDeps{})
	f.Must(err)
	_, err = submitVerify(t, f, v2, lease, nil)
	f.Must(err)
	if record(t, f, gate).Status == "closed" {
		t.Fatal("retired verify refutations discharged the reproduced finding")
	}
	fix := record(t, f, gate+".r2.fix")
	findings, _, err := kernel.Decode[[]kernel.Finding](fix.Metadata, kernel.KeyReproduced)
	f.Must(err)
	if len(findings) != 1 || findings[0].Slot != v2.Returns[0].Slot {
		t.Fatalf("retired verify changed the fix scope: %+v", findings)
	}
	res, err := panel.Tally(filepath.Join(f.SpecDir, "reviews", "review"))
	f.Must(err)
	old := 0
	for _, ref := range res.Panel.Refutations {
		if ref.VerifyStep == id {
			old++
		}
	}
	if old != len(v.Returns) {
		t.Fatal("retirement removed refutations from panel history")
	}
}

func TestS134Verify_MetadataBound(t *testing.T) {
	f := reducerFixture(t)
	put(t, filepath.Join(f.Root, ".mindspec", "config.yaml"), "panel:\n  reviewers:\n    - family: harness\n      count: 9\n")
	gate := f.Items[0] + ".review"
	slots := []string{}
	ids := []string{}
	for i := 1; i <= 9; i++ {
		slots = append(slots, fmt.Sprintf("R%d", i))
		ids = append(ids, fmt.Sprintf("%s.r1.R%d", gate, i))
	}
	head := gateHead(t, f, gate)
	kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: head, SlotCount: 9, Slots: ids})
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: slots})
	dir := filepath.Join(f.SpecDir, "reviews", "review")
	for i, slot := range slots {
		v := paneltest.Verdict(panel.VerdictRequestChanges)
		v.Slot, v.ReviewedSHA, v.Model = slot, head, "test-model"
		v.ConcreteChangesRequired = []panel.ChangeRequired{}
		for j := 0; j < 100; j++ {
			v.ConcreteChangesRequired = append(v.ConcreteChangesRequired, panel.ChangeRequired{Text: strings.Repeat("x", 4096), Basis: "traced"})
		}
		_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "review", Slot: slot, Round: 1}, paneltest.Bytes(t, v), verifyResolution)
		f.Must(err)
		closePass(t, f, ids[i])
	}
	reduce(t, f, true)
	id, v := verifyRecord(t, f, gate, 0)
	findings, _, err := kernel.Decode[[]kernel.Finding](record(t, f, id).Metadata, kernel.KeyFindings)
	f.Must(err)
	if len(findings) != 900 {
		t.Fatalf("wrong finding count: %d", len(findings))
	}
	for _, finding := range findings {
		if len(finding.Text) != 4096 {
			t.Fatalf("stored finding was shortened: %d", len(finding.Text))
		}
	}
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	_, err = submitVerify(t, f, v, lease, nil)
	f.Must(err)
	returns, _, err := kernel.Decode[[]kernel.VerifyReturn](record(t, f, id).Metadata, kernel.KeyReturns)
	f.Must(err)
	if !reflect.DeepEqual(returns, v.Returns) {
		t.Fatal("large verify close lost mindspec_returns")
	}
}

func TestS134SeedMatch_Verify(t *testing.T) {
	real, seed := reducerFixture(t), reducerFixture(t)
	gate := real.Items[0] + ".review"
	for _, f := range []*kerneltest.Fixture{real, seed} {
		refusedPanel(t, f, gate, "review", 1, panel.VerdictReject)
		reduce(t, f, true)
	}
	id, v := verifyRecord(t, real, gate, 1)
	lease, err := kernel.Lease(real.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	real.Must(err)
	_, err = submitVerify(t, real, v, lease, nil)
	real.Must(err)
	// The seed starts at the same creation boundary and receives the same lease.
	seedLease, err := kernel.Lease(seed.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	seed.Must(err)
	_ = seedLease
	r := record(t, real, id)
	findings, _, _ := kernel.Decode[[]kernel.Finding](r.Metadata, kernel.KeyFindings)
	read, _, _ := kernel.Decode[map[string]string](r.Metadata, kernel.KeyRead)
	kerneltest.Verify(seed, gate, v, findings, read)
	a, b := record(t, real, id), record(t, seed, id)
	// Tokens are the sole random fields; retain all other lease values.
	al, _, _ := kernel.Decode[kernel.LeaseRecord](a.Metadata, kernel.KeyLease)
	bl, _, _ := kernel.Decode[kernel.LeaseRecord](b.Metadata, kernel.KeyLease)
	al.TokenHash = bl.TokenHash
	a.Metadata[kernel.KeyLease] = al
	if !jsonEqual(a, b) {
		t.Fatalf("verify seeder differs from real close: real=%+v seed=%+v", a, b)
	}
}
