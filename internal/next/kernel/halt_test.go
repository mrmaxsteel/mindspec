package kernel_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func foreignChild(t *testing.T, f *kerneltest.Fixture, parent string) kernel.Record {
	t.Helper()
	raw, err := f.Store.Run("create", "--title", "foreign child", "--type", "task", "--parent", f.ID(parent), "--json")
	f.Must(err)
	var r kernel.Record
	f.Must(json.Unmarshal(raw, &r))
	return r
}
func haltFixture(t *testing.T) (*kerneltest.Fixture, string) {
	t.Helper()
	f := reducerFixture(t)
	doneItem(t, f, f.Items[0])
	d := kerneltest.OpenDecision(f, f.Items[1]+".review", "reject", 1, "fix", "halt")
	return f, d
}
func seedHalt(t *testing.T, f *kerneltest.Fixture, d string) {
	t.Helper()
	answer := kernel.DecisionAnswer{Option: "halt", At: kernel.Now().UTC()}
	f.Must(f.Store.WriteMetadata(d, map[string]any{kernel.KeyAnswer: answer, kernel.KeyClose: kernel.Receipt{Kind: "passed", Answer: &answer}}))
}
func assertHaltWrites(t *testing.T, before, after kernel.State, answerID string) {
	t.Helper()
	if !reflect.DeepEqual(before.Refs, after.Refs) || len(before.Records) != len(after.Records) {
		t.Fatal("halt created records or changed refs")
	}
	for id, old := range before.Records {
		r, ok := after.Records[id]
		if !ok {
			t.Fatal("halt removed record")
		}
		if jsonEqual(old, r) {
			continue
		}
		receipt, has, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
		if err != nil || !has || receipt.Kind != "halted" || receipt.HaltAnswer != answerID || old.Status == "closed" || r.Status != "closed" || kernel.ValidReceipt(after.Root, receipt, r.Metadata) != nil {
			t.Fatalf("halt close must name answer %s: %s %+v", answerID, id, receipt)
		}
		old.Status = r.Status
		if old.Metadata == nil {
			old.Metadata = map[string]any{}
		}
		old.Metadata[kernel.KeyClose] = r.Metadata[kernel.KeyClose]
		if !jsonEqual(old, r) {
			t.Fatalf("halt changed unrelated fields: %s", id)
		}
	}
}
func assertHaltedRead(t *testing.T, f *kerneltest.Fixture) {
	t.Helper()
	before := kerneltest.Snapshot(t, f)
	for i := 0; i < 2; i++ {
		out := reduce(t, f, false)
		if out.Status != "halted" || kernel.OutputExitCode(out) != 4 || len(out.Run)+len(out.Actions)+len(out.Decisions) != 0 || kerneltest.Snapshot(t, f) != before {
			t.Fatalf("halted read advanced or returned wrong exit: %+v", out)
		}
	}
}
func TestS134Halt_ReceiptKillAndRetry(t *testing.T) {
	for _, retry := range []string{"apply", "submit"} {
		t.Run(retry, func(t *testing.T) {
			control, _ := haltFixture(t)
			if !action(reduce(t, control, false), control.Items[1]+".impl.a1") {
				t.Fatal("control did not offer the downstream implement")
			}
			f, d := haltFixture(t)
			foreign := foreignChild(t, f, f.Items[1]+".review")
			note := foreignChild(t, f, f.Epic)
			// No downstream implement has been materialised at the kill point.
			seedHalt(t, f, d)
			before := f.State()
			snapshot := kerneltest.Snapshot(t, f)
			read := reduce(t, f, false)
			if kernel.OutputExitCode(read) != 4 || kerneltest.Snapshot(t, f) != snapshot {
				t.Fatal("halt witness did not stop read-only reduction")
			}
			var out kernel.Output
			if retry == "apply" {
				out = reduce(t, f, true)
			} else {
				var err error
				out, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: d, Answer: "halt"})
				f.Must(err)
			}
			if kernel.OutputExitCode(out) != 4 {
				t.Fatalf("halt retry exited successfully: %+v", out)
			}
			assertHaltWrites(t, before, f.State(), d)
			if record(t, f, d).Status != "closed" || record(t, f, f.Items[1]+".review").Status != "open" || !hasFlag(out, "foreign-child-open") {
				t.Fatalf("halt did not close decision or preserve foreign parent: %+v", out)
			}
			if !reflect.DeepEqual(record(t, f, foreign.ID), foreign) || !reflect.DeepEqual(record(t, f, note.ID), note) {
				t.Fatal("halt changed foreign work")
			}
			for _, item := range f.Items[1:] {
				if record(t, f, item).Status != "open" {
					t.Fatal("halt closed work item")
				}
			}
			if record(t, f, f.Epic).Status != "open" {
				t.Fatal("halt closed epic")
			}
			for _, id := range []string{f.Items[1] + ".impl.a1", f.Items[2] + ".impl.a1"} {
				if r, _ := f.Store.Lookup(id); r != nil {
					t.Fatal("halt created implement")
				}
			}
			// Removing the obstruction permits the same answer to finish its halt.
			_, err := f.Store.Run("update", foreign.ID, "--status", "closed")
			f.Must(err)
			before = f.State()
			out, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: d, Answer: "halt"})
			f.Must(err)
			if kernel.OutputExitCode(out) != 4 || record(t, f, f.Items[1]+".review").Status != "closed" {
				t.Fatal("same halt answer did not finish remaining closes with exit 4")
			}
			assertHaltWrites(t, before, f.State(), d)
			before = f.State()
			out = reduce(t, f, true)
			assertHaltWrites(t, before, f.State(), d)
			if kernel.OutputExitCode(out) != 4 {
				t.Fatal("halted apply lost exit 4")
			}
			assertHaltedRead(t, f)
		})
	}
}
func TestS134Halt_LocalRejectAndForeignDescendant(t *testing.T) {
	f := reducerFixture(t)
	doneItem(t, f, f.Items[0])
	gate := f.Items[1] + ".review"
	refusedPanel(t, f, gate, "review", 1, "REJECT")
	reduce(t, f, true)
	d := gate + ".decide.reject.r1"
	// A first fix answer allows a later rejected round on the same gate.
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: d, Answer: "fix"})
	f.Must(err)
	fix := gate + ".r2.fix"
	f.Must(f.Store.WriteReceipt(fix, kernel.Receipt{Kind: "passed", Retally: "seeded-fix"}))
	f.Must(f.Store.SetStatusIf(fix, "open", "closed"))
	refusedPanel(t, f, gate, "review", 2, "REJECT")
	reduce(t, f, true)
	d = gate + ".decide.reject.r2"
	foreign := foreignChild(t, f, gate)
	// A foreign descendant keeps both its slot and gate open.
	slot := fmt.Sprintf("%s.r2.R1", gate)
	// Reopen locally to represent an in-flight slot with a foreign child.
	f.Must(f.Store.SetStatusIf(slot, "closed", "open"))
	slotChild := foreignChild(t, f, slot)
	before := f.State()
	out, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: d, Answer: "halt"})
	f.Must(err)
	if kernel.OutputExitCode(out) != 4 || !hasFlag(out, "foreign-child-open") {
		t.Fatalf("halt output: %+v", out)
	}
	for _, id := range f.State().Steps {
		if id == slot {
			continue
		}
		r := record(t, f, id)
		if r.Status != "closed" {
			t.Fatalf("closable step left open: %s", id)
		}
		if before.Records[id].Status != "closed" {
			receipt, has, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
			if err != nil || !has || receipt.Kind != "halted" || receipt.HaltAnswer != d || kernel.ValidReceipt(f.Root, receipt, r.Metadata) != nil {
				t.Fatalf("halt close must name answer %s: %s %+v", d, id, receipt)
			}
		}
	}
	if record(t, f, gate).Status != "open" || record(t, f, slot).Status != "open" || !reflect.DeepEqual(record(t, f, foreign.ID), foreign) || !reflect.DeepEqual(record(t, f, slotChild.ID), slotChild) {
		t.Fatal("halt closed foreign work or its kernel ancestors")
	}
	if r := record(t, f, f.Items[2]+".impl.a1"); r.Status != "closed" {
		t.Fatal("halt did not close independent implement")
	}
	assertHaltedRead(t, f)
}
