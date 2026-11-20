package complete

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func rawLandCompletion(t *testing.T, f *kerneltest.Fixture) {
	t.Helper()
	_, err := f.Command("git", "switch", "spec/"+f.SpecID)
	f.Must(err)
	_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "merge", "--no-ff", "-m", "raw", "bead/"+f.Items[0])
	f.Must(err)
	if !f.State().Merged[f.Items[0]] {
		t.Fatal("fixture did not land")
	}
}

func TestS134Complete_MergedRechecksBeforeClose(t *testing.T) {
	for _, mode := range []string{"halt", "gate", "blocker"} {
		t.Run(mode, func(t *testing.T) {
			saveAndRestore(t)
			f, gate, _ := completeKernelFixture(t)
			rawLandCompletion(t, f)
			installCompleteKernelRun(t, f)
			old := closingStepHook
			t.Cleanup(func() { closingStepHook = old })
			called := false
			closingStepHook = func(step int) error {
				if step == 1 {
					called = true
					lateCompletionBlock(t, f, gate, mode)
				}
				return nil
			}
			prepared, err := prepareKernelComplete(f.State(), f.Items[0], true)
			f.Must(err)
			_, err = closeMergedBead(f.Root, f.Items[0], f.SpecID, f.Epic, "spec/"+f.SpecID, f.Root, f.Root, executor.MergeIdentity{Parents: []string{"base", "head"}}, nil, nil, false, &executor.MockExecutor{}, CompleteOpts{kernelState: &prepared})
			if !called || err == nil {
				t.Fatalf("late check did not refuse: called=%t err=%v", called, err)
			}
			if f.State().Records[f.Items[0]].Status != "open" {
				t.Fatal("late refusal closed merged work")
			}
			if completionReleased(t, f, f.Store.Lookup, f.Items[1:]) {
				t.Fatal("late refusal released dependents")
			}
		})
	}
}

func lateCompletionBlock(t *testing.T, f *kerneltest.Fixture, gate, mode string) {
	t.Helper()
	switch mode {
	case "halt":
		d := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
		kerneltest.Answer(f, d, "halt")
	case "gate":
		f.Must(f.Store.WriteMetadata(gate, map[string]any{kernel.KeyClose: nil}))
	case "blocker":
		bug := kernel.Record{ID: "foreign-bug", Type: "bug", Status: "open"}
		f.Fake.Records[bug.ID] = bug
		row := f.Fake.Records[f.Items[0]]
		row.Dependencies = append(row.Dependencies, kernel.Dependency{ID: bug.ID, Type: "blocks"})
		f.Fake.Records[row.ID] = row
	}
}

func TestS134Complete_FinishRechecksBeforeRecord(t *testing.T) {
	for _, mode := range []string{"halt", "gate", "blocker"} {
		t.Run(mode, func(t *testing.T) {
			f, gate, _ := completeKernelFixture(t)
			rawLandCompletion(t, f)
			f.Must(f.Store.SetStatusIf(f.Items[0], "open", "closed"))
			prepared, err := prepareKernelComplete(f.State(), f.Items[0], true)
			f.Must(err)
			lateCompletionBlock(t, f, gate, mode)
			before := kerneltest.Snapshot(t, f)
			err = finishKernelComplete(prepared, f.Items[0])
			if err == nil {
				t.Fatal("late check wrote completion")
			}
			if mode == "halt" && !strings.Contains(err.Error(), "halt") {
				t.Fatalf("missing halt: %v", err)
			}
			if f.State().Records[f.Items[0]].Metadata[kernel.KeyCompletion] != nil {
				t.Fatal("late check wrote completion record")
			}
			if completionReleased(t, f, f.Store.Lookup, f.Items[1:]) {
				t.Fatal("late check released dependents")
			}
			if kerneltest.Snapshot(t, f) != before {
				t.Fatal("refused finish wrote")
			}
		})
	}
}

func TestS134Complete_RecordMustMatchAcceptedSubject(t *testing.T) {
	saveAndRestore(t)
	f, gate, old := completeKernelFixture(t)
	item := f.Items[0]
	f.Must(f.Store.WriteMetadata(item, map[string]any{kernel.KeyCompletion: old}))
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	f.Must(f.Store.SetStatusIf(item, "closed", "open"))
	head := completeKernelMove(t, f, "new.txt")
	kerneltest.PassGate(f, gate, head)
	rawLandCompletion(t, f)
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	_, err := kernel.NextAction(f.State(), true)
	f.Must(err)
	if completionReleased(t, f, f.Store.Lookup, f.Items[1:]) {
		t.Fatal("stale completion record released dependents")
	}
	installCompleteKernelRun(t, f)
	_, err = Run(f.Root, item, f.SpecID, "", &s134CompletionExecutor{MockExecutor: &executor.MockExecutor{}, fixture: f}, CompleteOpts{})
	f.Must(err)
	if f.State().Records[item].Metadata[kernel.KeyCompletion] != head || !completionReleased(t, f, f.Store.Lookup, f.Items[1:]) {
		t.Fatal("new accepted completion did not release dependents")
	}
}

func TestS134Complete_DeletedBranchSubject(t *testing.T) {
	saveAndRestore(t)
	f, gate, accepted := completeKernelFixture(t)
	installCompleteKernelRun(t, f)
	_, err := Run(f.Root, f.Items[0], f.SpecID, "", &s134CompletionExecutor{MockExecutor: &executor.MockExecutor{}, fixture: f}, CompleteOpts{})
	f.Must(err)
	_, err = f.Command("git", "switch", "spec/"+f.SpecID)
	f.Must(err)
	landed, err := f.Command("git", "rev-list", "--parents", "-n", "1", "HEAD")
	f.Must(err)
	parts := strings.Fields(string(landed))
	if len(parts) != 3 {
		t.Fatal("fixture lacks a merge commit")
	}
	f.Must(f.Store.WriteMetadata(f.Items[0], map[string]any{"mindspec_landed_merge_sha": parts[0], "mindspec_landed_second_parent": parts[2]}))
	_, err = f.Command("git", "branch", "-D", "bead/"+f.Items[0])
	f.Must(err)
	s := f.State()
	if !s.Merged[f.Items[0]] {
		t.Fatal("landed binding did not survive branch deletion")
	}
	f.Must(kernel.CheckPassedGate(s, gate))
	t.Run("missing-subject", func(t *testing.T) {
		row := s.Records[f.Items[0]]
		row.Metadata = map[string]any{}
		s.Records[f.Items[0]] = row
		if kernel.CheckPassedGate(s, gate) == nil {
			t.Fatal("empty merged subject bypassed comparison")
		}
	})
	// A valid landed binding must still be compared with the accepted subject.
	kerneltest.PassGate(f, gate, strings.Repeat("a", 40))
	err = kernel.CheckPassedGate(f.State(), gate)
	if err == nil || !strings.Contains(err.Error(), accepted) {
		t.Fatalf("deleted branch bypassed landed subject comparison: %v", err)
	}
}

func TestS134Complete_DeletedRawBranchCannotComplete(t *testing.T) {
	saveAndRestore(t)
	f, _, _ := completeKernelFixture(t)
	item := f.Items[0]
	rawLandCompletion(t, f)
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	_, err := f.Command("git", "branch", "-D", "bead/"+item)
	f.Must(err)
	if f.State().Merged[item] {
		t.Fatal("raw merge without a landed binding counted as merged")
	}
	installCompleteKernelRun(t, f)
	_, err = Run(f.Root, item, f.SpecID, "", &s134CompletionExecutor{MockExecutor: &executor.MockExecutor{}, fixture: f}, CompleteOpts{})
	if err == nil {
		t.Fatal("raw merge without a branch or binding completed")
	}
	if f.State().Records[item].Metadata[kernel.KeyCompletion] != nil || completionReleased(t, f, f.Store.Lookup, f.Items[1:]) {
		t.Fatal("missing subject released dependents")
	}
}

func TestS134Complete_ClosedEpicMissingSubject(t *testing.T) {
	saveAndRestore(t)
	f, gate, accepted := completeKernelFixture(t)
	item := f.Items[0]
	rawLandCompletion(t, f)
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	f.Must(f.Store.SetStatusIf(f.Epic, "open", "closed"))
	_, err := f.Command("git", "branch", "-D", "bead/"+item)
	f.Must(err)
	installCompleteKernelRun(t, f)
	_, err = Run(f.Root, item, f.SpecID, "", &s134CompletionExecutor{MockExecutor: &executor.MockExecutor{}, fixture: f}, CompleteOpts{})
	if err == nil || !strings.Contains(err.Error(), "head  differs from accepted subject "+accepted+" for gate "+gate) {
		t.Fatalf("missing subject bypassed ended gate check: %v", err)
	}
	if f.State().Records[item].Metadata[kernel.KeyCompletion] != nil || completionReleased(t, f, f.Store.Lookup, f.Items[1:]) {
		t.Fatal("missing subject released dependents")
	}
}

func TestS134Complete_RecordedCompletionStillRespectsBlocker(t *testing.T) {
	saveAndRestore(t)
	f, _, accepted := completeKernelFixture(t)
	item := f.Items[0]
	rawLandCompletion(t, f)
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	// This is the durable state after the record write and before its releasing pass.
	f.Must(f.Store.WriteMetadata(item, map[string]any{kernel.KeyCompletion: accepted}))
	lateCompletionBlock(t, f, item+".review", "blocker")
	before := f.State().Records[item]
	_, err := kernel.NextAction(f.State(), true)
	f.Must(err)
	if completionReleased(t, f, f.Store.Lookup, f.Items[1:]) {
		t.Fatal("recorded completion bypassed foreign blocker during pass recovery")
	}
	if !reflect.DeepEqual(before, f.State().Records[item]) {
		t.Fatal("waiting pass changed completed item")
	}
	bug := f.Fake.Records["foreign-bug"]
	bug.Status = "closed"
	f.Fake.Records[bug.ID] = bug
	_, err = kernel.NextAction(f.State(), true)
	f.Must(err)
	if !completionReleased(t, f, f.Store.Lookup, f.Items[1:]) {
		t.Fatal("cleared blocker did not release dependents")
	}
}
