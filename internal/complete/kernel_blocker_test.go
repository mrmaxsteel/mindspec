package complete

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

func completionReleased(t *testing.T, f interface {
	Must(error)
}, lookup func(string) (*kernel.Record, error), items []string) bool {
	any := false
	for _, d := range items {
		row, err := lookup(d + ".impl.a1")
		f.Must(err)
		any = any || row != nil
	}
	return any
}

// A: foreign blocker on the item; raw merge of the accepted subject; raw close; complete replay.
func TestS134Complete_ForeignBlockerMergedReplay(t *testing.T) {
	for _, mode := range []string{"open-unmerged", "closed-unmerged", "closed-merged"} {
		t.Run(mode, func(t *testing.T) {
			saveAndRestore(t)
			f, _, _ := completeKernelFixture(t)
			item := f.Items[0]
			bug := kernel.Record{ID: "foreign-bug", Type: "bug", Status: "open", Title: "foreign blocker"}
			f.Fake.Records[bug.ID] = bug
			row := f.Fake.Records[item]
			row.Dependencies = append(row.Dependencies, kernel.Dependency{ID: bug.ID, Type: "blocks"})
			f.Fake.Records[item] = row
			if mode == "closed-merged" {
				_, err := f.Command("git", "switch", "spec/"+f.SpecID)
				f.Must(err)
				_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "merge", "--no-ff", "-m", "raw", "bead/"+item)
				f.Must(err)
			}
			if mode != "open-unmerged" {
				f.Must(f.Store.SetStatusIf(item, "open", "closed"))
			}
			before := f.State().Records[item]
			installCompleteKernelRun(t, f)
			result, err := Run(f.Root, item, f.SpecID, "", &s134CompletionExecutor{MockExecutor: &executor.MockExecutor{}, fixture: f}, CompleteOpts{})
			rel := completionReleased(t, f, f.Store.Lookup, f.Items[1:])
			t.Logf("mode=%s err=%v released=%t completion=%v", mode, err, rel, f.State().Records[item].Metadata[kernel.KeyCompletion])
			if err == nil || rel || !strings.Contains(err.Error(), "step is held or blocked by foreign-bug") {
				t.Errorf("foreign blocker not enforced: result=%+v", result)
			}
			_, prepErr := prepareKernelComplete(f.State(), item, true)
			if prepErr == nil || !strings.Contains(prepErr.Error(), "step is held or blocked by foreign-bug") {
				t.Fatalf("preparation bypassed blocker: %v", prepErr)
			}
			after := f.State().Records[item]
			if !reflect.DeepEqual(before, after) || after.Metadata[kernel.KeyCompletion] != nil {
				t.Fatalf("refused replay changed item: %+v", after)
			}
		})
	}
}

// B: crash after close, before merge: rerun completes and releases.
func TestS134Complete_CrashAfterCloseBeforeMerge(t *testing.T) {
	saveAndRestore(t)
	f, _, _ := completeKernelFixture(t)
	item := f.Items[0]
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	installCompleteKernelRun(t, f)
	_, err := Run(f.Root, item, f.SpecID, "", &s134CompletionExecutor{MockExecutor: &executor.MockExecutor{}, fixture: f}, CompleteOpts{})
	rel := completionReleased(t, f, f.Store.Lookup, f.Items[1:])
	t.Logf("err=%v released=%t merged=%t completion=%v", err, rel, f.State().Merged[item], f.State().Records[item].Metadata[kernel.KeyCompletion])
	if err != nil || !rel {
		t.Errorf("rerun after close-before-merge did not finish")
	}
}
