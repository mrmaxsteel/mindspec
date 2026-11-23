package kernel_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestS134Verify_CloseRows(t *testing.T) {
	for _, outcome := range []string{"refuted", "deferred"} {
		t.Run(outcome, func(t *testing.T) {
			f := reducerFixture(t)
			gate := f.Items[0] + ".review"
			registerGate(t, f, gate, "review", 1)
			dir := filepath.Join(f.SpecDir, "reviews", "review")
			text := "check \x1b[31mred\r\nrecovery: forged \u202eend"
			for i, word := range []string{panel.VerdictReject, panel.VerdictRequestChanges, panel.VerdictApprove} {
				v := paneltest.Verdict(word)
				v.Slot, v.ReviewedSHA, v.Model = fmt.Sprintf("R%d", i+1), gateHead(t, f, gate), "test-model"
				if i == 1 {
					v.ConcreteChangesRequired[0].Text = text
				}
				_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "review", Slot: v.Slot, Round: 1}, paneltest.Bytes(t, v), verifyResolution)
				f.Must(err)
				closePass(t, f, gate+".r1."+v.Slot)
			}
			reduce(t, f, true)
			id, v := verifyRecord(t, f, gate, 0)
			if len(v.Returns) != 2 || v.Returns[0].Finding != "veto" || v.Returns[1].Finding != "ccr[0]" {
				t.Fatalf("mixed fixture lost its findings: %+v", v)
			}
			v.Returns[1].Return = outcome
			if outcome == "deferred" {
				v.Returns[1].Target = "follow-up"
				// Only the follow-up lookup uses the fake CLI. The kernel
				// keeps its own isolated store and runs all submit checks.
				fake := t.TempDir()
				data, err := json.Marshal([]any{map[string]any{
					"id": "follow-up", "status": "open", "description": text,
					"dependencies": []map[string]string{{"id": f.Epic, "dependency_type": "discovered-from"}},
				}})
				f.Must(err)
				path := filepath.Join(fake, "follow-up.json")
				f.Must(os.WriteFile(path, data, 0600))
				t.Setenv("S134_VERIFY_FOLLOW_UP", path)
				f.Must(os.WriteFile(filepath.Join(fake, "bd"), []byte("#!/bin/sh\nif [ \"$1\" != show ] || [ \"$2\" != follow-up ]; then exit 1; fi\ncat \"$S134_VERIFY_FOLLOW_UP\"\n"), 0700))
				t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
			f.Must(err)
			_, err = submitVerify(t, f, v, lease, nil)
			f.Must(err)
			returns, _, err := kernel.Decode[[]kernel.VerifyReturn](record(t, f, id).Metadata, kernel.KeyReturns)
			f.Must(err)
			if !reflect.DeepEqual(returns, v.Returns) {
				t.Fatalf("veto return was lost: %+v", returns)
			}
			res, err := panel.Tally(dir)
			f.Must(err)
			refs, defs := res.Panel.Refutations, res.Panel.Deferrals
			if outcome == "refuted" {
				if len(refs) != 1 || len(defs) != 0 || refs[0].Finding != "ccr[0]" || refs[0].Slot != "R2" || refs[0].FindingText != text {
					t.Fatalf("mixed close stored a veto disposition or lost the ccr: refs=%+v defs=%+v", refs, defs)
				}
			} else if len(refs) != 0 || len(defs) != 1 || defs[0].Finding != "ccr[0]" || defs[0].FindingText != text {
				t.Fatalf("deferred close stored wrong dispositions: refs=%+v defs=%+v", refs, defs)
			}
			lines, err := panel.ReadLedger(dir)
			f.Must(err)
			events, rows := 0, 0
			for _, line := range lines {
				var event struct {
					Record  string `json:"record"`
					Finding string `json:"finding"`
				}
				f.Must(json.Unmarshal(line, &event))
				if event.Record == panel.RecordRefutation || event.Record == panel.RecordDeferral {
					events++
					if event.Finding != "ccr[0]" {
						t.Fatalf("veto became a ledger disposition: %s", line)
					}
				}
				if event.Record == panel.RecordDisposition {
					rows++
					var row panel.DispositionRow
					f.Must(json.Unmarshal(line, &row))
					if row.Summary != termsafe.Escape(text) {
						t.Fatalf("audit summary is not escaped: got %q want %q", row.Summary, termsafe.Escape(text))
					}
				}
			}
			if events != 1 || rows != 1 {
				t.Fatalf("mixed close must write exactly one disposition and row: events=%d rows=%d", events, rows)
			}
		})
	}
}

func TestS134Verify_OpenChildHoldsPassingGate(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "review", 1, panel.VerdictRequestChanges)
	reduce(t, f, true)
	id, v := verifyRecord(t, f, gate, 0)
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	fix := gate + ".r2.fix"
	f.Create(fix, gate)
	_, err = submitVerify(t, f, v, lease, nil)
	f.Must(err)
	if record(t, f, id).Status != "closed" || record(t, f, fix).Status != "open" {
		t.Fatal("verify did not pass with an open child")
	}
	if record(t, f, gate).Status != "open" {
		t.Fatal("passing verify closed a gate with an open child")
	}
	f.Must(f.Store.WriteReceipt(fix, kernel.Receipt{Kind: "superseded", Retally: "test-child-closed"}))
	f.Must(f.Store.SetStatusIf(fix, "open", "closed"))
	reduce(t, f, true)
	receipt, _, err := kernel.Decode[kernel.Receipt](record(t, f, gate).Metadata, kernel.KeyClose)
	f.Must(err)
	if record(t, f, gate).Status != "closed" || receipt.Kind != "passed" || receipt.Retally != "verify" {
		t.Fatalf("gate did not pass after its child closed: %+v", receipt)
	}
}

func TestS134Verify_RetiredSameSubject(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "review", 1, panel.VerdictRequestChanges)
	reduce(t, f, true)
	id, v := verifyRecord(t, f, gate, 1)
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	_, err = submitVerify(t, f, v, lease, nil)
	f.Must(err)
	res, err := panel.Tally(filepath.Join(f.SpecDir, "reviews", "review"))
	f.Must(err)
	before := kernel.FilterDispositions(f.State(), gate, res)
	if len(before.Panel.Refutations) != 2 || !kernel.KeepDisposition(f.State(), gate, 1, id, "") {
		t.Fatal("passed current verify did not discharge its refutations")
	}
	findings, _, err := kernel.Decode[[]kernel.Finding](record(t, f, id).Metadata, kernel.KeyFindings)
	f.Must(err)
	read, _, err := kernel.Decode[map[string]string](record(t, f, id).Metadata, kernel.KeyRead)
	f.Must(err)
	next := v
	next.Step = id + ".a2"
	next.Returns = append([]kernel.VerifyReturn{}, v.Returns...)
	for i := range next.Returns {
		next.Returns[i].Return = "reproduced"
	}
	kerneltest.Verify(f, gate, next, findings, read)
	if kernel.KeepDisposition(f.State(), gate, 1, id, "") {
		t.Fatal("retired passed verify with the same subject discharged a finding")
	}
	filtered := kernel.FilterDispositions(f.State(), gate, res)
	if len(filtered.Panel.Refutations) != 0 || len(res.Panel.Refutations) != 2 {
		t.Fatalf("retired refutations must remain history only: filtered=%+v original=%+v", filtered.Panel.Refutations, res.Panel.Refutations)
	}
}

func TestS134Verify_UncountedSubmitPrepass(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	registerGate(t, f, gate, "review", 1)
	dir := filepath.Join(f.SpecDir, "reviews", "review")
	submit := func(slot, word string) {
		t.Helper()
		v := paneltest.Verdict(word)
		v.Slot, v.ReviewedSHA, v.Model = slot, gateHead(t, f, gate), "test-model"
		_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "review", Slot: slot, Round: 1}, paneltest.Bytes(t, v), verifyResolution)
		f.Must(err)
	}
	for _, slot := range []string{"R2", "R3"} {
		submit(slot, panel.VerdictApprove)
		closePass(t, f, gate+".r1."+slot)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		submit("R1", "VOID")
		id := gate + ".r1.R1"
		if attempt > 1 {
			id += fmt.Sprintf(".a%d", attempt)
		}
		closePass(t, f, id)
		reduce(t, f, true)
	}
	decision := record(t, f, gate+".decide.uncounted.r1")
	// Verdicts can change while the decision is held. No listed slot is now
	// capped, so the pre-pass must still hold the newly uncounted R2.
	submit("R1", panel.VerdictApprove)
	submit("R2", "VOID")
	run := f.Store.Run
	sawAnswer := false
	f.Store.Run = func(args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "update" && args[1] == decision.ID {
			for i, arg := range args {
				if arg == "--metadata" && i+1 < len(args) {
					data, err := os.ReadFile(strings.TrimPrefix(args[i+1], "@"))
					f.Must(err)
					var values map[string]json.RawMessage
					f.Must(json.Unmarshal(data, &values))
					if _, ok := values[kernel.KeyAnswer]; ok {
						sawAnswer = true
						retry, err := f.Store.Lookup(gate + ".r1.R2.a2")
						f.Must(err)
						if retry != nil {
							t.Fatal("submit pre-pass wrote a slot attempt before the decision answer")
						}
					}
				}
			}
		}
		return run(args...)
	}
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: decision.Key(), Answer: "redispatch"})
	f.Must(err)
	if !sawAnswer {
		t.Fatal("submit did not write the decision answer")
	}
	if record(t, f, decision.Key()).Status != "closed" || record(t, f, gate+".r1.R2.a2").Status != "open" {
		t.Fatal("answered decision did not release the retry in the final pass")
	}
}
