package kernel_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func recoveryRound(t *testing.T, f *kerneltest.Fixture, gate string) kernel.GateRound {
	t.Helper()
	r, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	return r
}

func recoveryVerdict(t *testing.T, f *kerneltest.Fixture, slot string, round int, subject, name string) string {
	t.Helper()
	root, _ := filepath.EvalSymlinks(f.Root)
	spec, _ := filepath.EvalSymlinks(f.SpecDir)
	rel, err := filepath.Rel(root, spec)
	f.Must(err)
	path := filepath.Join(rel, "reviews", "review", name)
	data := strings.Replace(verdictBytes(subject, slot), `"round":1`, fmt.Sprintf(`"round":%d`, round), 1)
	actorCommit(t, f, path, data, "")
	return path
}

// recoveryMovedHead registers a round, leases a slot, and moves the head.
func recoveryMovedHead(t *testing.T) (*kerneltest.Fixture, string, string, kernel.LeaseResult, string, string) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	registerGate(t, f, gate, "review", 1)
	reduce(t, f, true)
	old := recoveryRound(t, f, gate)
	if old.Round != 1 || len(old.Slots) != 3 {
		t.Fatalf("setup: %+v", old)
	}
	r2 := gate + ".r1.R2"
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: r2}, kernel.LeaseDeps{})
	f.Must(err)
	artifact := recoveryVerdict(t, f, "R2", 1, old.Subject, "verdict-1-R2.json")
	head := beadCommit(t, f, "bead/"+f.Items[0])
	return f, gate, r2, lease, artifact, head
}

func recoverySuperseded(t *testing.T, f *kerneltest.Fixture, id string) {
	t.Helper()
	r := record(t, f, id)
	receipt, has, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
	f.Must(err)
	if r.Status != "closed" || !has || receipt.Kind != "superseded" || kernel.ValidReceipt(f.Root, receipt) != nil {
		t.Fatalf("stale slot has no superseded close: %s %+v receipt=%+v", id, r, receipt)
	}
	next := id + ".a2"
	if strings.HasSuffix(id, ".a2") {
		next = strings.TrimSuffix(id, ".a2") + ".a3"
	}
	assertRetry(t, f, id, next, false)
}

func TestS134Supersede_RefusedSubmitCoveredNextPass(t *testing.T) {
	for _, bogus := range []bool{false, true} {
		t.Run(fmt.Sprint(bogus), func(t *testing.T) {
			f, gate, slot, lease, artifact, head := recoveryMovedHead(t)
			token := lease.Token
			if bogus {
				token = "bogus"
			}
			_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: slot, Lease: token, Artifact: artifact})
			if err == nil {
				t.Fatal("stale submit accepted")
			}
			// The pre-pass cannot close the named step or advance past an open slot.
			if record(t, f, slot).Status != "in_progress" || recoveryRound(t, f, gate).Round != 1 {
				t.Fatal("pre-pass advanced past its excluded open slot")
			}
			if _, has, _ := kernel.Decode[kernel.Receipt](record(t, f, slot).Metadata, kernel.KeyClose); has {
				t.Fatal("pre-pass receipted the excluded slot")
			}
			out := reduce(t, f, true)
			for i := 1; i <= 3; i++ {
				recoverySuperseded(t, f, fmt.Sprintf("%s.r1.R%d", gate, i))
			}
			r := recoveryRound(t, f, gate)
			if r.Round != 2 || r.Subject != head || !hasRun(out, "mindspec", "panel", "create", f.Items[0]) {
				t.Fatalf("next pass did not release the next round: %+v %+v", r, out)
			}
			before := kerneltest.Snapshot(t, f)
			reduce(t, f, true)
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("replay wrote")
			}
		})
	}
}

func TestS134Supersede_SweepsOlderSlotsAndRetries(t *testing.T) {
	f, gate, slot, _, _, head := recoveryMovedHead(t)
	// Model a store left by an interrupted older controller, two rounds behind.
	retry := slot + ".a2"
	f.Must(f.Store.SetStatusIf(slot, "in_progress", "closed"))
	f.Create(retry, gate)
	f.Must(f.Store.WriteMetadata(retry, kernel.Metadata(kernel.KeyFrom, kernel.From(slot))))
	kerneltest.Authorise(f, gate, gateRound(t, f, gate, 3, head))
	reduce(t, f, true)
	for _, id := range []string{gate + ".r1.R1", gate + ".r1.R3", retry} {
		recoverySuperseded(t, f, id)
	}
	// The superseded retry also explains its raw-closed predecessor.
	if !kernel.RepairedClose(f.State(), slot) {
		t.Fatal("raw close has no repair evidence")
	}
	assertRetry(t, f, retry, slot+".a3", false)
}

func TestS134Closes_StaleSlotHasNoRetry(t *testing.T) {
	for _, mode := range []string{"round", "subject"} {
		t.Run(mode, func(t *testing.T) {
			f, gate, slot, lease, artifact, head := recoveryMovedHead(t)
			_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: slot, Lease: lease.Token, Artifact: artifact})
			if err == nil {
				t.Fatal("stale submit accepted")
			}
			// Raw close before the pass that would cover the excluded step.
			f.Must(f.Store.SetStatusIf(slot, "in_progress", "closed"))
			n := 2
			// A changed subject authorizes a new round, never a rewrite of the current one.
			if mode == "subject" {
				f.Must(f.Store.WriteMetadata(slot, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: "old-subject"})))
			}
			kerneltest.Authorise(f, gate, gateRound(t, f, gate, n, head))
			reduce(t, f, true)
			recoverySuperseded(t, f, slot)
			before := kerneltest.Snapshot(t, f)
			reduce(t, f, true)
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("stale close repair was not idempotent")
			}
		})
	}
}

func TestS134Halt_OpenBlockersKeepRecordsOpen(t *testing.T) {
	for _, field := range []string{"final-only", "id", "depends_on_id"} {
		t.Run(field, func(t *testing.T) {
			f, d := haltFixture(t)
			final := f.Epic + ".final"
			step := f.Items[1] + ".review"
			bug := kernel.Record{ID: "foreign-halt-blocker", Type: "bug", Status: "open"}
			f.Fake.Records[bug.ID] = bug
			r := record(t, f, step)
			dep := kernel.Dependency{Type: "blocks", ID: bug.ID}
			if field == "depends_on_id" {
				dep.ID = "ignored-id"
				dep.DependsOnID = bug.ID
			}
			if field != "final-only" {
				r.Dependencies = append(r.Dependencies, dep)
				f.Fake.Records[r.ID] = r
			}
			out, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: d, Answer: "halt"})
			f.Must(err)
			if kernel.OutputExitCode(out) != 4 {
				t.Fatalf("halt exit: %+v", out)
			}
			targets := []string{final}
			if field != "final-only" {
				targets = append(targets, step)
			}
			for _, id := range targets {
				r = record(t, f, id)
				if r.Status != "open" {
					t.Fatalf("blocked record closed: %s", id)
				}
				if _, has, _ := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose); has {
					t.Fatalf("blocked record receipted: %s", id)
				}
			}
			for _, id := range f.Items[1:] {
				if record(t, f, id).Status != "open" {
					t.Fatal("halt closed work item")
				}
			}
			if record(t, f, bug.ID).Status != "open" {
				t.Fatal("halt closed foreign blocker")
			}
			before := kerneltest.Snapshot(t, f)
			out, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: d, Answer: "halt"})
			f.Must(err)
			if kernel.OutputExitCode(out) != 4 || before != kerneltest.Snapshot(t, f) {
				t.Fatal("halt retry wrote or lost halt exit")
			}
			out, err = kernel.NextAction(f.State(), true)
			f.Must(err)
			if kernel.OutputExitCode(out) != 4 || before != kerneltest.Snapshot(t, f) {
				t.Fatal("halt apply wrote or lost halt exit")
			}
		})
	}
}
