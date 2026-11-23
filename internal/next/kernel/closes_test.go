package kernel_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func assertCloseFlag(t *testing.T, out kernel.Output, code, id string, want bool) {
	t.Helper()
	got := false
	for _, flag := range out.Flags {
		got = got || (flag.Code == code && flag.Step == id)
	}
	if got != want {
		t.Fatalf("%s for %s = %v, want %v: %+v", code, id, got, want, out)
	}
}
func assertRetry(t *testing.T, f *kerneltest.Fixture, id, retry string, want bool) {
	t.Helper()
	r, err := f.Store.Lookup(retry)
	f.Must(err)
	if (r != nil) != want {
		t.Fatalf("retry %s = %+v, want exists %v", retry, r, want)
	}
	if r != nil {
		from, ok, err := kernel.Decode[kernel.From](r.Metadata, kernel.KeyFrom)
		if err != nil || !ok || string(from) != id {
			t.Fatalf("retry lineage: %+v", r)
		}
	}
}
func TestS134Closes_UnreceiptedSlot(t *testing.T) {
	for _, mode := range []string{"no-verdict", "verdict-old-token", "bad-blob", "invalid-repair", "repaired"} {
		t.Run(mode, func(t *testing.T) {
			f := reducerFixture(t)
			id := f.Items[0] + ".review.r1.R1"
			if mode == "verdict-old-token" {
				passingPanel(t, f)
				physical := f.ID(id)
				r := f.Fake.Records[physical]
				delete(r.Metadata, kernel.KeyClose)
				f.Fake.Records[physical] = r
			} else {
				seedClosed(t, f, id, nil)
			}
			if mode == "verdict-old-token" {
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyLease, kernel.LeaseRecord{Generation: 2, TokenHash: "current-token-hash", Expires: fixedTime.Add(time.Hour)})))
			}
			if mode == "bad-blob" {
				f.Must(f.Store.WriteReceipt(id, kernel.Receipt{Kind: "passed", Path: "missing", Commit: strings.Repeat("a", 40), Blob: strings.Repeat("b", 40)}))
			}
			if mode == "invalid-repair" {
				kerneltest.Repaired(f, id, kernel.Receipt{Kind: "passed"})
			}
			repaired := mode == "repaired"
			if repaired {
				kerneltest.Repaired(f, id, kernel.Receipt{Kind: "passed", Retally: "valid-repair"})
			}
			before := kerneltest.Snapshot(t, f)
			out := reduce(t, f, false)
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("probe wrote")
			}
			assertCloseFlag(t, out, "unreceipted-close", id, !repaired)
			out = reduce(t, f, true)
			assertCloseFlag(t, out, "unreceipted-close", id, !repaired)
			assertRetry(t, f, id, id+".a2", !repaired)
			if !repaired {
				found := false
				for _, a := range out.Actions {
					found = found || a.Step == id+".a2"
				}
				if !found {
					t.Fatal("new retry has no action in same pass")
				}
			}
			snap := kerneltest.Snapshot(t, f)
			reduce(t, f, true)
			if snap != kerneltest.Snapshot(t, f) {
				t.Fatal("retry was not idempotent")
			}
		})
	}
}
func TestS134Closes_ReducerReceipts(t *testing.T) {
	for _, c := range []struct {
		suffix  string
		receipt kernel.Receipt
	}{
		{".review.r1.R1", kernel.Receipt{Kind: "superseded", Subject: "new-subject"}},
		{".review.r2.fix", kernel.Receipt{Kind: "superseded", Retally: "passing-retally"}},
		{".review.judge.a1", kernel.Receipt{Kind: "superseded", Rebind: "panel-rebind"}},
	} {
		t.Run(c.suffix, func(t *testing.T) {
			f := reducerFixture(t)
			id := f.Items[0] + c.suffix
			seedClosed(t, f, id, &c.receipt)
			out := reduce(t, f, true)
			assertCloseFlag(t, out, "unreceipted-close", id, false)
			retry := id + ".a2"
			if strings.HasSuffix(id, ".a1") {
				retry = strings.TrimSuffix(id, ".a1") + ".a2"
			}
			assertRetry(t, f, id, retry, false)
		})
	}
}
func TestS134Closes_RetryRepairsLineage(t *testing.T) {
	f := reducerFixture(t)
	id := f.Items[0] + ".review.r1.R1"
	seedClosed(t, f, id, nil)
	reduce(t, f, true)
	retry := id + ".a2"
	f.Must(f.Store.WriteReceipt(retry, kernel.Receipt{Kind: "passed", Retally: "retry-verdict"}))
	f.Must(f.Store.SetStatusIf(retry, "open", "closed"))
	out := reduce(t, f, true)
	assertCloseFlag(t, out, "unreceipted-close", id, false)
	assertRetry(t, f, retry, id+".a3", false)
	// A completed but unrelated attempt cannot repair the original raw close.
	f.Must(f.Store.WriteMetadata(retry, kernel.Metadata(kernel.KeyFrom, kernel.From("other"))))
	assertCloseFlag(t, reduce(t, f, false), "unreceipted-close", id, true)
}
func TestS134Closes_WorkItemBlocksDependent(t *testing.T) {
	for _, forced := range []bool{false, true} {
		t.Run(fmt.Sprint(forced), func(t *testing.T) {
			f := reducerFixture(t)
			item := f.Items[0]
			// Use a distinct bead tip so the work item has not merged.
			newCommit(t, f, "bead/"+item)
			if !forced {
				kerneltest.PassGate(f, item+".review", strings.Repeat("a", 40))
			}
			f.Must(f.Store.SetStatusIf(item, "open", "closed"))
			state := f.State()
			if kernel.RawClose(state, item) == forced || kernel.ForcedClose(state, item) != forced {
				t.Fatalf("close predicates: raw=%v forced=%v", kernel.RawClose(state, item), kernel.ForcedClose(state, item))
			}
			out := reduce(t, f, true)
			code := "raw-close"
			if forced {
				code = "forced-close"
			}
			assertCloseFlag(t, out, code, item, true)
			for _, dep := range f.Items[1:] {
				assertRetry(t, f, "", dep+".impl.a1", false)
			}
		})
	}
	// Even a merged item closed over an open child cannot release work.
	for _, subject := range []string{"missing", "empty", "current"} {
		t.Run(subject, func(t *testing.T) {
			f := reducerFixture(t)
			item := f.Items[0]
			kerneltest.PassGate(f, item+".review", strings.Repeat("a", 40))
			judge := item + ".review.judge.a1"
			f.Create(judge, item)
			if subject != "missing" {
				commit := ""
				if subject == "current" {
					commit = reducerGit(t, f, "rev-parse", "bead/"+item)
				}
				f.Must(f.Store.WriteMetadata(judge, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: commit})))
			}
			f.Must(f.Store.SetStatusIf(item, "open", "closed"))
			out := reduce(t, f, true)
			assertCloseFlag(t, out, "forced-close", item, true)
			if actorRecord(t, f, judge).Status != "open" {
				t.Fatal("open judge was closed without a subject change")
			}
			for _, dependent := range f.Items[1:] {
				assertRetry(t, f, "", dependent+".impl.a1", false)
			}
		})
	}
}
func TestS134Closes_RawClaimStep(t *testing.T) {
	for _, row := range []struct {
		name  string
		lease *kernel.LeaseRecord
		want  bool
	}{
		{"no-lease", nil, true},
		{"empty-token-hash", &kernel.LeaseRecord{Generation: 1, Expires: fixedTime.Add(time.Hour)}, true},
		{"valid-lease", &kernel.LeaseRecord{Generation: 1, TokenHash: "leased", Expires: fixedTime.Add(time.Hour)}, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			f := reducerFixture(t)
			id := f.Items[0] + ".impl.a1"
			reduce(t, f, true)
			f.Must(f.Store.SetStatusIf(id, "open", "in_progress"))
			if row.lease != nil {
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyLease, *row.lease)))
			}
			before := kerneltest.Snapshot(t, f)
			assertCloseFlag(t, reduce(t, f, false), "raw-claim", id, row.want)
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("flag wrote")
			}
		})
	}
}

func TestS134Closes_MergedPassedClosedItem(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	kerneltest.PassGate(f, item+".review", strings.Repeat("a", 40))
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	s := f.State()
	if !s.Merged[item] {
		t.Fatal("fixture item is not merged")
	}
	if kernel.RawClose(s, item) {
		t.Fatal("merged item counted as raw close")
	}
	assertCloseFlag(t, reduce(t, f, false), "raw-close", item, false)
}

func TestS134Closes_RawClosedJudgeBlocksSamePass(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	gate := item + ".review"
	kerneltest.PassGate(f, gate, strings.Repeat("a", 40))
	id := kerneltest.Judge(f, gate, "new-subject", 1)
	f.Must(f.Store.SetStatusIf(id, "open", "closed"))
	out := reduce(t, f, true)
	assertCloseFlag(t, out, "unreceipted-close", id, true)
	assertRetry(t, f, id, gate+".judge.a2", true)
	if hasRun(out, "mindspec", "complete", item) {
		t.Fatal("raw-closed judge offered completion in same pass")
	}
	retry, _ := f.Store.Lookup(gate + ".judge.a2")
	sub, _, _ := kernel.Decode[kernel.Subject](retry.Metadata, kernel.KeySubject)
	if sub.Commit != "new-subject" || retry.Parent != item {
		t.Fatalf("retry lost subject or parent: %+v", retry)
	}
}
func TestS134Closes_RawSlotsCannotPassTally(t *testing.T) {
	f := reducerFixture(t)
	passingPanel(t, f)
	gate := f.Items[0] + ".review"
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("%s.r1.R%d", gate, i)
		physical := f.ID(id)
		r := f.Fake.Records[physical]
		delete(r.Metadata, kernel.KeyClose)
		f.Fake.Records[physical] = r
	}
	out := reduce(t, f, true)
	if f.State().Records[gate].Status == "closed" || hasRun(out, "mindspec", "complete", f.Items[0]) {
		t.Fatal("verdict files authorised raw slot closes")
	}
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("%s.r1.R%d", gate, i)
		assertCloseFlag(t, out, "unreceipted-close", id, true)
		assertRetry(t, f, id, id+".a2", true)
		f.Must(f.Store.WriteReceipt(id+".a2", kernel.Receipt{Kind: "passed", Retally: "repaired-verdict"}))
		f.Must(f.Store.SetStatusIf(id+".a2", "open", "closed"))
	}
	out = reduce(t, f, true)
	if f.State().Records[gate].Status != "closed" {
		t.Fatal("valid retries did not permit tally")
	}
	if hasFlag(out, "unreceipted-close") {
		t.Fatal("repaired slots still flagged")
	}
}

func TestS134Closes_StaleRawSlotAudit(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	registerGate(t, f, gate, "review", 1)
	reduce(t, f, true)
	id := gate + ".r1.R1"
	head := beadCommit(t, f, "bead/"+f.Items[0])
	f.Must(f.Store.SetStatusIf(id, "open", "closed"))
	kerneltest.Authorise(f, gate, gateRound(t, f, gate, 2, head))
	before := kerneltest.Snapshot(t, f)
	reduce(t, f, false)
	if before != kerneltest.Snapshot(t, f) {
		t.Fatal("probe wrote")
	}
	reduce(t, f, true)
	receipt := actorReceipt(t, f, id)
	if receipt.Kind != "superseded" || receipt.Subject != head || !receipt.Repaired {
		t.Fatalf("supersede lost raw close: %+v", receipt)
	}
	for i := 0; i < 2; i++ {
		rows, err := kernel.Audit(f.State(), nil)
		f.Must(err)
		found := false
		for _, row := range rows {
			if row.Event == "unreceipted-closes" {
				found = true
				if row.Count != 1 {
					t.Fatalf("raw close audit=%d want 1", row.Count)
				}
			}
		}
		if !found {
			t.Fatal("audit row absent")
		}
		snapshot := kerneltest.Snapshot(t, f)
		reduce(t, f, true)
		if snapshot != kerneltest.Snapshot(t, f) {
			t.Fatal("supersede replay wrote")
		}
	}
}
