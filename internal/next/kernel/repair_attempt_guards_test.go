package kernel_test

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Lease_FirstTakeSubject(t *testing.T) {
	f, id, deps := actorFixture(t)
	result, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "first claim"}, deps)
	f.Must(err)
	events, _, err := kernel.Decode[[]kernel.TakeReason](actorRecord(t, f, id).Metadata, kernel.KeyTakes)
	f.Must(err)
	if len(events) != 1 || events[0].Subject == "" || events[0].Subject != result.Lease.ClaimBase {
		t.Fatalf("take subject must equal the claim base: %+v, base=%s", events, result.Lease.ClaimBase)
	}
}

func TestS134Submit_RepairEventAfterChecks(t *testing.T) {
	for _, kind := range []string{"lens", "verify", "slot"} {
		t.Run(kind, func(t *testing.T) {
			var f *kerneltest.Fixture
			var id string
			var o kernel.SubmitOptions
			if kind == "slot" {
				var subject string
				f, id, _, subject = slotFixture(t)
				data := verdictBytes(subject, "R1")
				actorCommit(t, f, "verdict.json", data, "")
				// A crash after the record write leaves the stored verdict.
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRecord, data)))
				o.Artifact = "verdict.json"
				o.BeforeClose = func(kernel.State, kernel.Receipt) error { return errors.New("publish failed") }
			} else {
				f, id, _ = seededRecordActor(t, kind)
				changeActorMetadata(t, f, id, "open", kernel.KeyClose, kernel.KeyClosing)
				leaseActor(t, f, id, kernel.LeaseDeps{})
			}
			changeActorMetadata(t, f, id, "closed")
			o.Step, o.Repair, o.Reason = id, true, "recover"
			before := actorRecord(t, f, id)
			restore := func() {}
			if kind != "slot" {
				restore = kernel.ReplaceCloseWriteHookForTest(kind, func(kernel.State, kernel.Record, kernel.Receipt) ([]kernel.Write, error) {
					return nil, errors.New("hook failed")
				})
			}
			_, err := kernel.Submit(f.State(), o)
			restore()
			if err == nil {
				t.Fatal("failed check accepted")
			}
			if !reflect.DeepEqual(before, actorRecord(t, f, id)) {
				t.Fatal("failed repair changed the step or takes history")
			}
			o.BeforeClose = nil
			_, err = kernel.Submit(f.State(), o)
			f.Must(err)
			assertOneRepair(t, f, id)
		})
	}
}

func assertOneRepair(t *testing.T, f *kerneltest.Fixture, id string) {
	t.Helper()
	r := actorRecord(t, f, id)
	events, _, err := kernel.Decode[[]kernel.TakeReason](r.Metadata, kernel.KeyTakes)
	f.Must(err)
	receipt := actorReceipt(t, f, id)
	if len(events) != 1 || events[0].Via != "repair" || receipt.Via != "repair" || receipt.Generation != events[0].Generation || events[0].RevokedTokenHash != "" {
		t.Fatalf("one receipt must have one repair event: events=%+v receipt=%+v", events, receipt)
	}
}

func TestS134Submit_RepairEventCASRetry(t *testing.T) {
	f, id, _ := seededRecordActor(t, "lens")
	changeActorMetadata(t, f, id, "open", kernel.KeyClose)
	leaseActor(t, f, id, kernel.LeaseDeps{})
	changeActorMetadata(t, f, id, "closed")
	calls := 0
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Repair: true, Reason: "recover", BeforeClose: func(s kernel.State, _ kernel.Receipt) error {
		calls++
		if calls == 1 {
			return s.Store.SetStatusIf(id, "open", "closed")
		}
		return nil
	}})
	f.Must(err)
	if calls != 2 {
		t.Fatalf("status conflict did not rerun the closure: %d calls", calls)
	}
	assertOneRepair(t, f, id)
}

func actorAttemptFixture(t *testing.T, kind string, repair bool) (*kerneltest.Fixture, string, string, kernel.SubmitOptions) {
	t.Helper()
	f, id, lease, subject := slotFixture(t)
	kerneltest.WithRules(t, kernel.NewRegistry())
	gate := f.Items[0] + ".review"
	o := kernel.SubmitOptions{Lease: lease.Token, Artifact: "verdict.json"}
	if kind == "fix" {
		id = gate + ".r2.fix"
		f.Create(id, gate)
		f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: subject})))
		lease = leaseActor(t, f, id, kernel.LeaseDeps{})
		o = kernel.SubmitOptions{Lease: lease.Token, Commit: actorCommit(t, f, "code.txt", "fixed", "bead/"+f.Items[0])}
	} else {
		data := verdictBytes(subject, "R1")
		actorCommit(t, f, "verdict.json", data, "")
		if repair {
			// A crash after the record write leaves the stored verdict.
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRecord, data)))
		}
	}
	o.Step = id
	if repair {
		changeActorMetadata(t, f, id, "closed")
		o.Repair, o.Reason, o.Lease = true, "hostile\nrecovery: forged\x1b", ""
	}
	retry := f.Create(id+".a2", gate)
	f.Must(f.Store.WriteMetadata(retry.Key(), map[string]any{kernel.KeyFrom: kernel.From(id), kernel.KeySubject: kernel.Subject{Commit: subject}, kernel.KeyFindings: []kernel.Finding{{Text: "hostile\nrecovery: forged\x1b"}}}))
	return f, id, retry.Key(), o
}

func actorAttemptRefusal(t *testing.T, producer string) {
	t.Helper()
	kind := "slot"
	if strings.Contains(producer, "fix") {
		kind = "fix"
	}
	f, id, retry, o := actorAttemptFixture(t, kind, strings.HasSuffix(producer, "-repair"))
	want := "step is not the current round attempt"
	if producer == "attempt.close-decode" || producer == "SlotLineageHead.decode" {
		f.Must(f.Store.WriteMetadata(retry, kernel.Metadata(kernel.KeyClose, map[string]any{"kind": []string{"hostile\nrecovery: forged\x1b"}})))
		want = "cannot read attempt receipt:"
	}
	if producer == "SlotLineageHead.decode" {
		changeActorMetadata(t, f, id, "closed")
		before := actorRecord(t, f, id)
		_, err := kernel.SlotLineageHead(f.State(), id)
		assertSubmitRefusal(t, err, "cannot read slot receipt:", "mindspec next-action")
		if !reflect.DeepEqual(before, actorRecord(t, f, id)) {
			t.Fatal("lineage read changed step")
		}
		return
	}
	actorRefusal(t, f, func() error { _, err := kernel.Submit(f.State(), o); return err }, want)
}

func TestS134Submit_SupersededHigherAttempt(t *testing.T) {
	for _, kind := range []string{"slot", "fix"} {
		t.Run(kind, func(t *testing.T) {
			f, id, retry, o := actorAttemptFixture(t, kind, false)
			f.Must(f.Store.WriteReceipt(retry, kernel.Receipt{Kind: "superseded", Subject: rawActorSubject(t, f, id)}))
			changeActorMetadata(t, f, retry, "closed")
			_, err := kernel.Submit(f.State(), o)
			f.Must(err)
			if actorRecord(t, f, id).Status != "closed" || actorReceipt(t, f, id).Generation != 1 {
				t.Fatal("current lower attempt was not accepted")
			}
		})
	}
}
func rawActorSubject(t *testing.T, f *kerneltest.Fixture, id string) string {
	t.Helper()
	subject, _, err := kernel.Decode[kernel.Subject](actorRecord(t, f, id).Metadata, kernel.KeySubject)
	f.Must(err)
	return subject.Commit
}

func TestS134Closes_SlotHeadBeforeSuperseded(t *testing.T) {
	for _, status := range []string{"raw-close", "passed"} {
		t.Run(status, func(t *testing.T) {
			f, id, _, subject := slotFixture(t)
			changeActorMetadata(t, f, id, "closed")
			if status == "passed" {
				data := verdictBytes(subject, "R1")
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRecord, data)))
				r := kernel.Receipt{Kind: "passed", RecordSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(data)))}
				f.Must(f.Store.WriteReceipt(id, r))
			}
			retry := f.Create(id+".a2", f.Items[0]+".review")
			f.Must(f.Store.WriteMetadata(retry.Key(), map[string]any{kernel.KeyFrom: kernel.From(id), kernel.KeyUncounted: true}))
			f.Must(f.Store.WriteReceipt(retry.Key(), kernel.Receipt{Kind: "superseded", Subject: subject}))
			changeActorMetadata(t, f, retry.Key(), "closed")
			head, err := kernel.SlotLineageHead(f.State(), id)
			f.Must(err)
			if head.Key() != id {
				t.Fatalf("head must precede superseded retry: got %s want %s", head.Key(), id)
			}
		})
	}
}

func TestS134Closes_RepairPinNeedsRetry(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(fmt.Sprint(conflict), func(t *testing.T) {
			f, id, _ := seededRecordActor(t, "verify")
			changeActorMetadata(t, f, id, "open", kernel.KeyClose)
			leaseActor(t, f, id, kernel.LeaseDeps{})
			if conflict {
				pin, _, err := kernel.Decode[kernel.Closing](actorRecord(t, f, id).Metadata, kernel.KeyClosing)
				f.Must(err)
				pin.Conflict = &kernel.ClosingIdentity{Generation: 2}
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyClosing, pin)))
			} else {
				_, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, kernel.LeaseDeps{})
				f.Must(err)
			}
			changeActorMetadata(t, f, id, "closed")
			useDefaultRules(t)
			out := reduce(t, f, false)
			if hasRun(out, "mindspec", "submit", id, "--repair", "--reason", "<text>") {
				t.Fatalf("pin must get retry, not repair: %+v", out.Run)
			}
			reduce(t, f, true)
			assertRetry(t, f, id, id+".a2", true)
		})
	}
}

func TestS134Submit_LateLeaseAndReceiptTogether(t *testing.T) {
	f, id, first, subject := slotFixture(t)
	_, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, kernel.LeaseDeps{})
	f.Must(err)
	actorCommit(t, f, "verdict.json", verdictBytes(subject, "R1"), "")
	before := actorRecord(t, f, id)
	run := f.Store.Run
	f.Store.Run = func(args ...string) ([]byte, error) {
		if len(args) > 3 && args[0] == "update" && args[1] == before.ID && args[2] == "--metadata" && hasReceiptMetadata(t, args[3]) {
			return nil, errors.New("receipt store failed")
		}
		return run(args...)
	}
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: first.Token, Artifact: "verdict.json"})
	f.Store.Run = run
	if err == nil {
		t.Fatal("receipt failure accepted")
	}
	// The verdict record and the superseded lease share the failed receipt write.
	if !reflect.DeepEqual(before, actorRecord(t, f, id)) {
		t.Fatal("failed receipt left a superseded lease or a stored verdict")
	}
}

func hasReceiptMetadata(t *testing.T, arg string) bool {
	t.Helper()
	data, err := os.ReadFile(strings.TrimPrefix(arg, "@"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(data), `"mindspec_close"`)
}
