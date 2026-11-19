package kernel_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Output_RevisionInputs(t *testing.T) {
	f := reducerFixture(t)
	reduce(t, f, true)
	id := f.Items[0] + ".impl.a1"
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyLease, kernel.LeaseRecord{Expires: fixedTime.Add(time.Hour), Generation: 1})))
	for _, input := range []string{"bd", "ref", "ledger", "verdict", "expiry", "display", "free-text"} {
		t.Run(input, func(t *testing.T) {
			state := f.State()
			base, err := kernel.NextAction(state, false)
			f.Must(err)
			switch input {
			case "bd":
				r := state.Records[id]
				r.Labels = append(r.Labels, "changed")
				state.Records[id] = r
			case "ref":
				state.Refs["refs/heads/bead/"+f.Items[0]] = "changed"
			case "ledger":
				state.Ledgers["directory"] = [][]byte{[]byte(`{"record":"round"}`)}
			case "verdict":
				state.Verdicts["path"] = json.RawMessage(`{"verdict":"APPROVE"}`)
			case "expiry":
				state.At = state.At.Add(2 * time.Hour)
			case "display":
				state.At = state.At.Add(time.Minute)
			case "free-text":
				r := state.Records[id]
				r.Title = "changed title"
				r.Description = "changed display text"
				state.Records[id] = r
			}
			out, err := kernel.NextAction(state, false)
			f.Must(err)
			if (base.Revision == out.Revision) != (input == "display" || input == "free-text") {
				t.Fatalf("revision ignored %s: %s %s", input, base.Revision, out.Revision)
			}
		})
	}
	// File inputs must reach the reader, not only a synthetic State field.
	registerRound(t, f, 1)
	a := reduce(t, f, false)
	path := filepath.Join(f.SpecDir, "reviews", "panel", "R1-round-1.json")
	put(t, path, `{"schema":2,"verdict":"APPROVE"}`)
	b := reduce(t, f, false)
	if a.Revision == b.Revision {
		t.Fatal("verdict file was not an input")
	}
}
func TestS134Output_StatusAndOrdering(t *testing.T) {
	f := reducerFixture(t)
	s := f.State()
	empty := kernel.PassResult{Output: kernel.Output{Schema: "mindspec.next-action/v1"}, State: s}
	out := kernel.OutputForPass(empty, s, false)
	if out.Status != "indeterminate" || kernel.OutputExitCode(out) != 1 {
		t.Fatalf("empty result=%+v", out)
	}
	for status, code := range map[string]int{"work": 0, "wait": 3, "halted": 4, "done": 5, "indeterminate": 1, "unknown": 1} {
		if got := kernel.OutputExitCode(kernel.Output{Status: status}); got != code {
			t.Errorf("%s exit %d", status, got)
		}
	}
	result := empty
	result.Output.Actions = []kernel.Action{{Step: f.Items[2] + ".impl.a1"}, {Step: f.Items[0] + ".review.r1.R10"}, {Step: f.Items[0] + ".review.r1.R2"}, {Step: f.Items[0] + ".impl.a1"}}
	out = kernel.OutputForPass(result, s, false)
	got := []string{}
	for _, a := range out.Actions {
		got = append(got, a.Step)
	}
	want := []string{f.Items[0] + ".impl.a1", f.Items[0] + ".review.r1.R2", f.Items[0] + ".review.r1.R10", f.Items[2] + ".impl.a1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan-kind-numeric-slot order: %v", got)
	}
}
func TestS134Output_Done(t *testing.T) {
	f := reducerFixture(t)
	for _, item := range f.Items {
		kerneltest.PassGate(f, item+".review", reducerGit(t, f, "rev-parse", "bead/"+item))
		f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	}
	kerneltest.PassGate(f, f.Epic+".final", reducerGit(t, f, "rev-parse", "spec/"+f.SpecID))
	f.Must(f.Store.SetStatusIf(f.Epic, "open", "closed"))
	out := reduce(t, f, false)
	if out.Status != "done" || kernel.OutputExitCode(out) != 5 {
		t.Fatalf("all done exit: %+v", out)
	}
}

func TestS134Output_EarliestExpiryAndSafeText(t *testing.T) {
	f := reducerFixture(t)
	s := f.State()
	earlier := fixedTime.Add(time.Hour)
	later := earlier.Add(time.Millisecond)
	result := kernel.PassResult{State: s, Output: kernel.Output{Waiting: []kernel.Waiting{
		{Step: f.Items[0] + ".impl.a1", Lease: kernel.LeaseView{Generation: 1, Expires: later}},
		{Step: f.Items[1] + ".impl.a1", Lease: kernel.LeaseView{Generation: 2, Expires: earlier}},
	}, Flags: []kernel.Flag{{Code: "foreign-child-open", Step: "bad\n\x1bchild", Repair: []string{"bd", "close", "bad\n\x1bchild"}}}}}
	out := kernel.OutputForPass(result, s, false)
	if out.RetryAt != earlier.Format(time.RFC3339Nano) {
		t.Fatalf("earliest expiry = %s", out.RetryAt)
	}
	for _, r := range out.Flags[0].Step {
		if r < 32 {
			t.Fatalf("unsafe terminal text: %q", out.Flags[0].Step)
		}
	}
}
