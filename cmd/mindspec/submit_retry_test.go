package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

// A hand edit of a counted verdict file retires the passed gate, and one
// applied pass lists the slot retry under the gate retry. The printed submit
// of that retry publishes to the panel that is registered on the first gate
// attempt, and the next pass runs complete.
func TestS134SubmitCmd_GateRetrySlotPublishes(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, err := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, err)
	old := submitCommandStateFn
	submitCommandStateFn = func(string, string) (kernel.State, error) { return f.State(), nil }
	t.Cleanup(func() { submitCommandStateFn = old })
	dir := filepath.Join(f.SpecDir, "reviews", "1")
	// verdict writes an APPROVE verdict of slot as name in the panel
	// directory, where the slot submit takes its artifact.
	verdict := func(slot, name string) string {
		v := paneltest.Verdict("APPROVE")
		v.Slot, v.Round, v.ReviewedSHA, v.Model = slot, 1, head, "test-model"
		file := filepath.Join(dir, name)
		f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0600))
		return file
	}
	for _, slot := range []string{"R1", "R2"} {
		id := gate + ".r1." + slot
		lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
		f.Must(err)
		out, err = kernelPanelRun(t, submitCmd, id, "--lease", lease.Token, "--artifact", verdict(slot, "verdict-"+slot+".json"))
		kernelPanelRequire(t, out, err)
	}
	if _, err := kernel.NextAction(f.State(), true); err != nil || kernelPanelRecord(t, f, gate).Status != "closed" {
		t.Fatalf("the round did not pass: %v", err)
	}
	file := filepath.Join(dir, "R1-round-1.json")
	data, err := os.ReadFile(file)
	f.Must(err)
	f.Must(os.WriteFile(file, []byte(strings.Replace(string(data), `"test-model"`, `"edited-model"`, 1)), 0644))
	retry := gate + ".r1.R1.a2"
	o, err := kernel.NextAction(f.State(), true)
	f.Must(err)
	var printed []string
	for _, a := range o.Actions {
		if a.Step == retry {
			printed = a.Submit
		}
	}
	want := []string{"mindspec", "submit", retry, "--lease", "<token>", "--artifact", "<path>"}
	if !reflect.DeepEqual(printed, want) {
		t.Fatalf("one applied pass prints submit %q for the retry, want %q: %+v", printed, want, o.Actions)
	}
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: retry}, kernel.LeaseDeps{})
	f.Must(err)
	args := append([]string{}, printed[2:]...)
	args[2], args[4] = lease.Token, verdict("R1", "verdict-retry.json")
	out, err = kernelPanelRun(t, submitCmd, args...)
	kernelPanelRequire(t, out, err)
	if kernelPanelRecord(t, f, retry).Status != "closed" {
		t.Fatal("the printed retry submit did not close the retry")
	}
	o, err = kernel.NextAction(f.State(), true)
	f.Must(err)
	found := false
	for _, run := range o.Run {
		found = found || reflect.DeepEqual(run.Argv, []string{"mindspec", "complete", f.Items[0]})
	}
	if !found {
		t.Fatalf("after the retry submit the next pass does not run complete: %+v", o)
	}
}
