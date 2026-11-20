package approve

import (
	"slices"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134ImplApprove_MergedForeignBlocker(t *testing.T) {
	f, ex, _ := s134ApproveFixture(t)
	item := f.Items[0]
	bug := kernel.Record{ID: "foreign-bug", Type: "bug", Status: "open"}
	f.Fake.Records[bug.ID] = bug
	row := f.Fake.Records[item]
	row.Dependencies = append(row.Dependencies, kernel.Dependency{ID: bug.ID, Type: "blocks"})
	f.Fake.Records[item] = row
	if !f.State().Merged[item] || row.Status != "closed" {
		t.Fatal("fixture is not closed and merged")
	}
	before := kerneltest.Snapshot(t, f)
	flags, err := kernel.ApprovalBlockers(f.State())
	f.Must(err)
	i := slices.IndexFunc(flags, func(flag kernel.Flag) bool { return flag.Step == item })
	if i < 0 || flags[i].Code != "work-item-blocked" ||
		flags[i].Message != "step is held or blocked by foreign-bug" ||
		!slices.Equal(flags[i].Repair, []string{"mindspec", "next-action"}) {
		t.Fatalf("blocker diagnostic: %+v", flags)
	}
	refusal := kernel.CheckWorkItem(f.State(), item)
	if refusal == nil || !strings.Contains(refusal.Error(), "recovery: "+strings.Join(flags[i].Repair, " ")) {
		t.Fatalf("repair differs from work-item refusal: %v", refusal)
	}
	_, err = ApproveImpl(f.Root, f.SpecID, ex)
	if err == nil || !strings.Contains(err.Error(), "step is held or blocked by foreign-bug") {
		t.Fatalf("approval bypassed blocker: %v", err)
	}
	if !strings.Contains(err.Error(), "work-item-blocked: "+item) || !strings.Contains(err.Error(), "; repair: mindspec next-action") {
		t.Fatalf("approval omitted kind or repair: %v", err)
	}
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("refused approval wrote")
	}
	bug.Status = "closed"
	f.Fake.Records[bug.ID] = bug
	_, err = ApproveImpl(f.Root, f.SpecID, ex)
	f.Must(err)
}
