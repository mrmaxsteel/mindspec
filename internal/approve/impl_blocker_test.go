package approve

import (
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
	_, err := ApproveImpl(f.Root, f.SpecID, ex)
	if err == nil || !strings.Contains(err.Error(), "step is held or blocked by foreign-bug") {
		t.Fatalf("approval bypassed blocker: %v", err)
	}
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("refused approval wrote")
	}
	bug.Status = "closed"
	f.Fake.Records[bug.ID] = bug
	_, err = ApproveImpl(f.Root, f.SpecID, ex)
	f.Must(err)
}
