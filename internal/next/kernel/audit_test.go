package kernel_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/journal"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

func TestS134Audit_TypedRecordsAndLocalJournal(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	gate := item + ".review"
	seedClosed(t, f, gate+".r1.R1", nil)
	// A completed retry counts the original close once, even without Repaired.
	reduce(t, f, true)
	retry := gate + ".r1.R1.a2"
	f.Must(f.Store.WriteReceipt(retry, kernel.Receipt{Kind: "passed", Retally: "retry"}))
	f.Must(f.Store.SetStatusIf(retry, "open", "closed"))
	f.Must(f.Store.WriteMetadata(item, kernel.Metadata(kernel.KeyDefer, kernel.DeferRecord{Finding: "finding", At: fixedTime})))
	f.Must(f.Store.WriteMetadata(item, kernel.Metadata(kernel.KeyTake, kernel.TakeReason{Generation: 2, At: fixedTime})))
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRePanel, kernel.RePanel{SlotCount: 3, Paths: []string{"source"}})))
	answer := kernel.DecisionAnswer{Option: "fix", At: fixedTime, Actor: "operator"}
	f.Must(f.Store.WriteReceipt(gate, kernel.Receipt{Kind: "passed", Answer: &answer, CarryOvers: []kernel.CarryOver{{OldSubject: "old", NewSubject: "new"}}}))
	id := item + ".impl.a1"
	f.Must(f.Store.SetStatusIf(id, "open", "in_progress"))
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyLease, kernel.LeaseRecord{Generation: 1, TokenHash: "new-lease", Expires: fixedTime})))
	// The lease writer persists exactly one observed claim. Renewal adds none.
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyLease, kernel.LeaseRecord{Generation: 1, TokenHash: "new-lease", Expires: fixedTime})))
	claims, ok, err := kernel.Decode[[]kernel.RawClaim](f.State().Records[id].Metadata, kernel.KeyRawClaims)
	if err != nil || !ok || len(claims) != 1 || !claims[0].At.Equal(fixedTime) {
		t.Fatalf("claim history: %+v %v", claims, err)
	}
	events := []journal.Record{}
	for _, name := range []string{"allow-doc-skew", "override-adr", "allow-not-ready", "allow-net-deletion"} {
		events = append(events, journal.Record{EscapeHatch: name})
	}
	// Unrelated journal events and foreign metadata do not add counts.
	events = append(events, journal.Record{EscapeHatch: "repair-phase"})
	f.Fake.Records["foreign"] = kernel.Record{ID: "foreign", Parent: f.Epic, Status: "closed", Metadata: kernel.Metadata(kernel.KeyTake, kernel.TakeReason{Generation: 1})}
	before := f.State()
	for run := 0; run < 2; run++ {
		rows, err := kernel.Audit(f.State(), events)
		f.Must(err)
		if len(rows) != 11 {
			t.Fatalf("audit rows: %+v", rows)
		}
		var out bytes.Buffer
		kernel.PrintAudit(&out, f.State(), rows)
		for _, row := range rows {
			if row.Count != 1 {
				t.Errorf("%s count=%d, want 1", row.Event, row.Count)
			}
		}
		if !strings.Contains(out.String(), "3 slots") || !strings.Contains(out.String(), fixedTime.Format("2006-01-02T15:04:05Z07:00")) {
			t.Fatalf("missing typed details: %s", out.String())
		}
	}
	if !jsonEqual(before.Records, f.State().Records) {
		t.Fatal("audit wrote records")
	}
}
func TestS134Audit_CurrentRawClaim(t *testing.T) {
	f := reducerFixture(t)
	id := f.Items[0] + ".impl.a1"
	reduce(t, f, true)
	f.Must(f.Store.SetStatusIf(id, "open", "in_progress"))
	rows, err := kernel.Audit(f.State(), nil)
	f.Must(err)
	for _, row := range rows {
		if row.Event == "raw-claims" && row.Count != 1 {
			t.Fatalf("raw claim count=%d", row.Count)
		}
	}
}

func TestS134Audit_ApprovalBlockersOrder(t *testing.T) {
	s := kernel.State{Foreign: []string{"z", "a"}, Steps: []string{"step-z", "step-a"}, Records: map[string]kernel.Record{}}
	for _, id := range append(append([]string{}, s.Foreign...), s.Steps...) {
		s.Records[id] = kernel.Record{ID: id, Status: "open"}
	}
	want := []kernel.Flag{{Code: "foreign-child-open", Step: "a"}, {Code: "foreign-child-open", Step: "z"}, {Code: "open-step", Step: "step-a"}, {Code: "open-step", Step: "step-z"}}
	got, err := kernel.ApprovalBlockers(s)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("blocker order: %+v, %v; want %+v", got, err, want)
	}
}
