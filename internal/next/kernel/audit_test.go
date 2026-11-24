package kernel_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/journal"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Audit_TypedRecordsAndLocalJournal(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	gate := item + ".review"
	reduce(t, f, true)
	// A repaired close, a receipt under a repair submit, and a late submission.
	seedClosed(t, f, gate+".r1.R1", nil)
	kerneltest.Repaired(f, gate+".r1.R1", kernel.Receipt{Kind: "passed", Retally: "repair"})
	seedClosed(t, f, gate+".r1.R2", &kernel.Receipt{Kind: "passed", Retally: "repair-submit", Via: "repair", Repaired: true, Generation: 2})
	seedClosed(t, f, gate+".r1.R3", &kernel.Receipt{Kind: "passed", Retally: "late", Generation: 1, SupersededGeneration: 2})
	// A completed retry without Repaired does not count as an unreceipted close.
	seedClosed(t, f, gate+".r1.R3.a2", &kernel.Receipt{Kind: "passed", Retally: "retry"})
	// One lens attempt per gate round.
	for _, lens := range []string{gate + ".r1.lens", gate + ".r2.lens"} {
		f.Create(lens, gate)
	}
	verify := gate + ".r1.verify"
	f.Create(verify, gate)
	f.Must(f.Store.WriteMetadata(verify, kernel.Metadata(kernel.KeyReturns, []kernel.VerifyReturn{{Slot: "R1", Return: "reproduced"}, {Slot: "R2", Return: "refuted"}, {Slot: "R3", Return: "deferred"}})))
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyDeferrals, []kernel.DeferRecord{{Finding: "verify finding", To: "follow", At: fixedTime}})))
	f.Must(f.Store.WriteMetadata(item, kernel.Metadata(kernel.KeyDefer, kernel.DeferRecord{Finding: "finding", At: fixedTime})))
	f.Must(f.Store.WriteMetadata(item, kernel.Metadata(kernel.KeyTakes, []kernel.TakeReason{{Via: "lease", Generation: 2, At: fixedTime}, {Via: "repair", Generation: 2, At: fixedTime}})))
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRePanel, kernel.RePanel{SlotCount: 3, Paths: []string{"source"}})))
	answer := kernel.DecisionAnswer{Option: "fix", At: fixedTime}
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
	lensCount, lensLines := lensRounds(t, f.State())
	if lensCount == 0 {
		t.Fatal("fixture has no lens step")
	}
	before := f.State()
	for run := 0; run < 2; run++ {
		rows, err := kernel.Audit(f.State(), events)
		f.Must(err)
		want := map[string]int{"--allow-doc-skew": 1, "--override-adr": 1, "--allow-not-ready": 1, "--allow-net-deletion": 1, "deferrals": 2, "takeovers": 2, "decision-answers": 1, "raw-claims": 1, "unreceipted-closes": 2, "carry-overs": 1, "re-panels": 1, "verify-reproduced": 1, "verify-refuted": 1, "verify-deferred": 1, "late-submissions": 1, "repair-receipts": 1, "lens-steps": lensCount}
		var out bytes.Buffer
		kernel.PrintAudit(&out, f.State(), rows)
		got := map[string]int{}
		for _, row := range rows {
			got[row.Event] = row.Count
		}
		if len(rows) != len(want) || !reflect.DeepEqual(got, want) {
			t.Fatalf("audit counts:\n got %v\nwant %v\n%s", got, want, out.String())
		}
		for _, line := range append([]string{"takeovers | 2 | lease 1, repair 1"}, lensLines...) {
			if !strings.Contains(out.String(), line) {
				t.Fatalf("missing %q:\n%s", line, out.String())
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

func TestS134Audit_RepeatedTakeovers(t *testing.T) {
	f, id, deps := actorFixture(t)
	leaseActor(t, f, id, deps)
	for _, reason := range []string{"first replacement", "second replacement"} {
		_, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: reason}, deps)
		f.Must(err)
	}
	latest, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "third replacement"}, deps)
	f.Must(err)
	_, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Renew: latest.Token}, deps)
	f.Must(err)
	_, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Release: latest.Token}, deps)
	f.Must(err)
	latest, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, deps)
	f.Must(err)
	expired := latest.Lease
	expired.Expires = fixedTime.Add(-time.Hour)
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyLease, expired)))
	_, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, deps)
	f.Must(err)

	for replay := 0; replay < 2; replay++ {
		rows, err := kernel.Audit(f.State(), nil)
		f.Must(err)
		for _, row := range rows {
			if row.Event == "takeovers" {
				t.Logf("replay %d: takeovers=%d after three successful --take operations", replay, row.Count)
				if row.Count != 3 {
					t.Errorf("takeover audit lost an event: got %d, want 3", row.Count)
				}
			}
		}
	}
}

// lensRounds lists the lens attempts of each gate round from the tuples, independent of the audit.
func lensRounds(t *testing.T, s kernel.State) (int, []string) {
	t.Helper()
	rounds := map[string]int{}
	total := 0
	for _, r := range s.Records {
		tuple, ok, err := r.Tuple()
		if err != nil || !ok || tuple.Kind != "lens" {
			continue
		}
		rounds[fmt.Sprintf("%s r%d", tuple.Gate, tuple.Round)]++
		total++
	}
	var lines []string
	for key, n := range rounds {
		lines = append(lines, fmt.Sprintf("%s: %d", key, n))
	}
	sort.Strings(lines)
	return total, lines
}

// auditCounts reads the producer rows that the late and repair tests check.
func auditCounts(t *testing.T, f *kerneltest.Fixture) (map[string]int, string) {
	t.Helper()
	rows, err := kernel.Audit(f.State(), nil)
	f.Must(err)
	var out bytes.Buffer
	kernel.PrintAudit(&out, f.State(), rows)
	got := map[string]int{}
	for _, row := range rows {
		switch row.Event {
		case "takeovers", "unreceipted-closes", "late-submissions", "repair-receipts":
			got[row.Event] = row.Count
		}
	}
	return got, out.String()
}

func TestS134Audit_LateSubmissionProducer(t *testing.T) {
	f, id, deps := actorFixture(t)
	first := leaseActor(t, f, id, deps)
	commit := actorCommit(t, f, "implementation.txt", "finished before the takeover", "bead/"+f.Items[0])
	_, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, deps)
	f.Must(err)
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: first.Token, Commit: commit})
	f.Must(err)
	for replay := 0; replay < 2; replay++ {
		got, out := auditCounts(t, f)
		want := map[string]int{"takeovers": 1, "unreceipted-closes": 0, "late-submissions": 1, "repair-receipts": 0}
		if !reflect.DeepEqual(got, want) || !strings.Contains(out, "takeovers | 1 | lease 1, repair 0") {
			t.Fatalf("replay %d: got %v, want %v\n%s", replay, got, want, out)
		}
	}
}

func TestS134Audit_RepairReceiptProducer(t *testing.T) {
	f, id, deps := actorFixture(t)
	put(t, filepath.Join(f.Root, ".beads", "config.yaml"), "types.custom: gate,step\n")
	leaseActor(t, f, id, deps)
	commit := actorCommit(t, f, "code.txt", "implementation", "bead/"+f.Items[0])
	changeActorMetadata(t, f, id, "closed")
	useDefaultRules(t)
	reduce(t, f, true)
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Repair: true, Reason: "raw close", Commit: commit})
	f.Must(err)
	for replay := 0; replay < 2; replay++ {
		got, out := auditCounts(t, f)
		want := map[string]int{"takeovers": 1, "unreceipted-closes": 1, "late-submissions": 0, "repair-receipts": 1}
		if !reflect.DeepEqual(got, want) || !strings.Contains(out, "takeovers | 1 | lease 0, repair 1") {
			t.Fatalf("replay %d: got %v, want %v\n%s", replay, got, want, out)
		}
	}
}

func TestS134Audit_HostileDetailsStayOnOneLine(t *testing.T) {
	var out bytes.Buffer
	kernel.PrintAudit(&out, kernel.State{}, []kernel.AuditRow{{Event: "lens-steps", Count: 1, Details: []string{"gate\nrecovery: run this r1: 1"}}})
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "recovery:") {
			t.Fatalf("detail text started a recovery line:\n%s", out.String())
		}
	}
	if strings.Count(out.String(), "\n") != 3 {
		t.Fatalf("detail text added lines:\n%q", out.String())
	}
}

// TestS134Audit_CountRules pins how each new row counts, with literal expected values.
func TestS134Audit_CountRules(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	gate := item + ".review"
	reduce(t, f, true)
	// Lens: round 1 has two attempts, rounds 2 and 10 have one each. Ids sort r10 before r2.
	for _, id := range []string{gate + ".r1.lens", gate + ".r1.lens.a2", gate + ".r2.lens", gate + ".r10.lens"} {
		f.Create(id, gate)
	}
	// Verify: two attempts; the first holds two returns of one type.
	verify := gate + ".r1.verify"
	f.Create(verify, gate)
	f.Must(f.Store.WriteMetadata(verify, kernel.Metadata(kernel.KeyReturns, []kernel.VerifyReturn{{Slot: "R1", Return: "reproduced"}, {Slot: "R2", Return: "reproduced"}, {Slot: "R3", Return: "refuted"}})))
	retry := verify + ".a2"
	f.Create(retry, gate)
	f.Must(f.Store.WriteMetadata(retry, kernel.Metadata(kernel.KeyReturns, []kernel.VerifyReturn{{Slot: "R1", Return: "reproduced"}, {Slot: "R2", Return: "deferred"}})))
	// Takes: a generation 0 event counts like any other.
	f.Must(f.Store.WriteMetadata(item, kernel.Metadata(kernel.KeyTakes, []kernel.TakeReason{{Via: "lease", Generation: 0, At: fixedTime}, {Via: "lease", Generation: 2, At: fixedTime}, {Via: "repair", Generation: 1, At: fixedTime}})))
	// One valid receipt with each property, and one receipt that fails validation with the same properties.
	valid := kernel.Receipt{Kind: "passed", Retally: "repair-submit", Via: "repair", Repaired: true, Generation: 3, SupersededGeneration: 2}
	seedClosed(t, f, gate+".r1.R1", &valid)
	invalid := kernel.Receipt{Kind: "passed", Via: "repair", Repaired: true, Generation: 3, SupersededGeneration: 2}
	seedClosed(t, f, gate+".r1.R2", &invalid)
	if kernel.ValidReceipt(f.Root, invalid, f.State().Records[gate+".r1.R2"].Metadata) == nil {
		t.Fatal("fixture receipt must fail validation")
	}
	// An invalid close whose linked retry completed without Repaired adds no count.
	seedClosed(t, f, gate+".r1.R3", nil)
	seedClosed(t, f, gate+".r1.R3.a2", &kernel.Receipt{Kind: "passed", Retally: "retry"})
	f.Must(f.Store.WriteMetadata(gate+".r1.R3.a2", kernel.Metadata(kernel.KeyFrom, kernel.From(gate+".r1.R3"))))
	if !kernel.RepairedClose(f.State(), gate+".r1.R3") {
		t.Fatal("fixture retry must repair the invalid close")
	}

	rows, err := kernel.Audit(f.State(), nil)
	f.Must(err)
	got := map[string]rowView{}
	for _, row := range rows {
		got[row.Event] = rowView{row.Count, row.Details}
	}
	want := map[string]rowView{
		"takeovers":          {3, []string{"lease 2, repair 1"}},
		"unreceipted-closes": {1, nil},
		"late-submissions":   {1, nil},
		"repair-receipts":    {1, nil},
		"verify-reproduced":  {3, nil},
		"verify-refuted":     {1, nil},
		"verify-deferred":    {1, nil},
		"lens-steps":         {4, []string{gate + " r1: 2", gate + " r2: 1", gate + " r10: 1"}},
	}
	for name, w := range want {
		if !reflect.DeepEqual(got[name], w) {
			t.Errorf("%s: got %+v, want %+v", name, got[name], w)
		}
	}
}

// rowView is the count and details of one audit row.
type rowView struct {
	Count   int
	Details []string
}
