package kernel_test

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Reducer_FinalApprovalEntry(t *testing.T) {
	for _, hold := range []string{"none", "foreign-child", "foreign-blocker", "judge", "item-incomplete", "final-open", "epic-closed"} {
		t.Run(hold, func(t *testing.T) {
			f := reducerFixture(t)
			for _, item := range f.Items {
				head := reducerGit(t, f, "rev-parse", "bead/"+item)
				kerneltest.PassGate(f, item+".review", head)
				f.Must(f.Store.WriteMetadata(item, map[string]any{kernel.KeyCompletion: head}))
				f.Must(f.Store.SetStatusIf(item, "open", "closed"))
			}
			final := f.Epic + ".final"
			if hold != "final-open" {
				kerneltest.PassGate(f, final, reducerGit(t, f, "rev-parse", "spec/"+f.SpecID))
			}
			switch hold {
			case "foreign-child":
				f.Fake.Records["foreign-note"] = kernel.Record{ID: "foreign-note", Parent: f.Epic, Type: "task", Status: "open"}
			case "foreign-blocker":
				f.Fake.Records["foreign-bug"] = kernel.Record{ID: "foreign-bug", Type: "bug", Status: "open"}
				r := f.Fake.Records[f.ID(final)]
				r.Dependencies = append(r.Dependencies, kernel.Dependency{ID: "foreign-bug", Type: "blocks"})
				f.Fake.Records[r.ID] = r
			case "judge":
				kerneltest.Judge(f, final, newCommit(t, f, "spec/"+f.SpecID), 1)
			case "item-incomplete":
				f.Must(f.Store.WriteMetadata(f.Items[0], map[string]any{kernel.KeyCompletion: ""}))
			case "epic-closed":
				f.Must(f.Store.SetStatusIf(f.Epic, "open", "closed"))
			}
			before := kerneltest.Snapshot(t, f)
			out := reduce(t, f, false)
			if got := hasRun(out, "mindspec", "impl", "approve", f.SpecID); got != (hold == "none") {
				t.Fatalf("final approval entry with %s: %+v", hold, out)
			}
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("read-only final approval selection wrote state")
			}
			if hold == "foreign-child" {
				r := f.Fake.Records["foreign-note"]
				r.Status = "closed"
				f.Fake.Records[r.ID] = r
				before = kerneltest.Snapshot(t, f)
				out = reduce(t, f, true)
				if !hasRun(out, "mindspec", "impl", "approve", f.SpecID) || out.Status != "work" || before != kerneltest.Snapshot(t, f) {
					t.Fatalf("closing foreign child did not release a read-only approval entry: %+v", out)
				}
			}
		})
	}
}
