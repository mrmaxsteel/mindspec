package kernel_test

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Output_DoneAfterRawGateRepair(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	head := beadCommit(t, f, "bead/"+item)
	passingPanel(t, f)
	gate := item + ".review"
	r, _ := f.Store.Lookup(gate)
	if r.Status != "open" || r.Metadata[kernel.KeyClose] != nil {
		t.Fatal("raw close must precede gate receipt")
	}
	_, err := f.Store.Run("close", f.ID(gate))
	f.Must(err)
	reduce(t, f, true)
	reduce(t, f, true)
	if !kernel.RepairedClose(f.State(), gate) {
		t.Fatal("not repaired")
	}
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	mergeItem(t, f, item, head)
	f.Must(f.Store.WriteMetadata(item, kernel.Metadata(kernel.KeyCompletion, head)))
	for _, other := range f.Items[1:] {
		doneItem(t, f, other)
	}
	final := f.Epic + ".final"
	tip := reducerGit(t, f, "rev-parse", "spec/"+f.SpecID)
	kerneltest.PassGate(f, final, tip)
	s := f.State()
	for _, item := range f.Items {
		if !kernel.ItemDone(s, item) {
			t.Fatalf("item not done: %s", item)
		}
	}
	flags, err := kernel.ApprovalBlockers(s)
	f.Must(err)
	if len(flags) != 0 {
		t.Fatalf("approval blocked: %+v", flags)
	}
	f.Must(kernel.CheckPassedGate(s, final))
	f.Must(f.Store.SetStatusIf(f.Epic, "open", "closed"))
	for _, apply := range []bool{false, true} {
		out := reduce(t, f, apply)
		if out.Status != "done" || kernel.OutputExitCode(out) != 5 || len(out.Flags)+len(out.Actions)+len(out.Run)+len(out.Decisions)+len(out.Waiting) != 0 {
			t.Errorf("repaired gate prevents terminal done: apply=%t status=%s exit=%d flags=%+v actions=%+v run=%+v", apply, out.Status, kernel.OutputExitCode(out), out.Flags, out.Actions, out.Run)
		}
	}
	// Terminal status must still reject invalid active evidence and unrepaired history.
	for _, test := range []struct {
		name   string
		mutate func(*kernel.State)
	}{
		{"open-retry", func(s *kernel.State) { r := s.Records[gate+".a2"]; r.Status = "open"; s.Records[r.Key()] = r }},
		{"invalid-retry-receipt", func(s *kernel.State) {
			r := s.Records[gate+".a2"]
			delete(r.Metadata, kernel.KeyClose)
			s.Records[r.Key()] = r
		}},
		{"missing-repair-lineage", func(s *kernel.State) {
			r := s.Records[gate+".a2"]
			delete(r.Metadata, kernel.KeyFrom)
			s.Records[r.Key()] = r
		}},
		{"invalid-final-receipt", func(s *kernel.State) {
			r := s.Records[final]
			delete(r.Metadata, kernel.KeyClose)
			s.Records[r.Key()] = r
		}},
		{"open-final-child", func(s *kernel.State) {
			r := s.Records[final]
			r.Status = "open"
			r.ID = "foreign-open"
			s.Records[r.ID] = r
			s.Foreign = append(s.Foreign, r.ID)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := f.State().Clone()
			test.mutate(&state)
			out := kernel.OutputForPass(kernel.PassResult{State: state}, state, false)
			if out.Status == "done" {
				t.Fatalf("invalid state accepted as done: %s", test.name)
			}
		})
	}
	t.Run("final-subject-moved", func(t *testing.T) {
		state := f.State()
		out := kernel.OutputForPass(kernel.PassResult{State: state}, state, false)
		if out.Status != "done" {
			t.Fatalf("valid state is not done: %s", out.Status)
		}
		moved := beadCommit(t, f, "spec/"+f.SpecID)
		if moved == tip || reducerGit(t, f, "rev-parse", "spec/"+f.SpecID) != moved {
			t.Fatal("spec head did not move")
		}
		state = f.State()
		out = kernel.OutputForPass(kernel.PassResult{State: state}, state, false)
		if out.Status == "done" {
			t.Fatal("moved final subject accepted as done")
		}
	})
}
