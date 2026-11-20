package complete

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Complete_MergedSubject(t *testing.T) {
	for _, mode := range []string{"open", "closed", "reset", "panel-only", "missing-gate"} {
		t.Run(mode, func(t *testing.T) {
			saveAndRestore(t)
			f, gate, accepted := completeKernelFixture(t)
			item := f.Items[0]
			head := accepted
			switch mode {
			case "reset":
				_, err := f.Command("git", "switch", "spec/"+f.SpecID)
				f.Must(err)
				_, err = f.Command("git", "branch", "-f", "bead/"+item, "spec/"+f.SpecID)
				f.Must(err)
				raw, err := f.Command("git", "rev-parse", "bead/"+item)
				f.Must(err)
				head = strings.TrimSpace(string(raw))
			case "panel-only":
				kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "passed", Round: 1, Subject: accepted, Slots: []string{"R1", "R2"}})
				kerneltest.PassGate(f, gate, accepted)
				head = completeKernelMove(t, f, filepath.ToSlash(filepath.Join(".mindspec/specs", f.SpecID, "reviews/passed/note.txt")))
			case "missing-gate":
				f.Must(f.Store.WriteMetadata(gate, map[string]any{kernel.KeyClose: nil}))
			default:
				head = completeKernelMove(t, f, "unreviewed.go")
			}
			if mode != "reset" {
				_, err := f.Command("git", "switch", "spec/"+f.SpecID)
				f.Must(err)
				_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "merge", "--no-ff", "-m", "raw", "bead/"+item)
				f.Must(err)
			}
			if mode == "closed" || mode == "missing-gate" {
				f.Must(f.Store.SetStatusIf(item, "open", "closed"))
			}
			if !f.State().Merged[item] {
				t.Fatal("fixture did not land")
			}
			if mode == "missing-gate" {
				_, err := prepareKernelComplete(f.State(), item, true)
				if err == nil {
					t.Fatal("merged replay skipped the gate")
				}
			}
			installCompleteKernelRun(t, f)
			ex := &s134CompletionExecutor{MockExecutor: &executor.MockExecutor{}, fixture: f}
			result, err := Run(f.Root, item, f.SpecID, "", ex, CompleteOpts{})
			if mode == "panel-only" {
				f.Must(err)
				return
			}
			if err == nil || !strings.Contains(err.Error(), gate) {
				t.Fatalf("merged item bypassed gate: %+v %v", result, err)
			}
			if mode != "missing-gate" && (!strings.Contains(err.Error(), "restore branch bead/"+item+" to accepted subject "+accepted+" with git") || strings.Contains(err.Error(), "next-action")) {
				t.Fatalf("merged subject has no usable recovery: %v", err)
			}
			if mode != "missing-gate" && (!strings.Contains(err.Error(), head) || !strings.Contains(err.Error(), accepted)) {
				t.Fatalf("missing head and accepted subject: %v", err)
			}
			for _, dependent := range f.Items[1:] {
				row, err := f.Store.Lookup(dependent + ".impl.a1")
				f.Must(err)
				if row != nil {
					t.Fatal("refused completion released dependent")
				}
			}
		})
	}
}

func TestS134Complete_RawMergeDoesNotReleaseDependents(t *testing.T) {
	saveAndRestore(t)
	f, _, _ := completeKernelFixture(t)
	item := f.Items[0]
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	_, err := f.Command("git", "switch", "spec/"+f.SpecID)
	f.Must(err)
	_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "merge", "--no-ff", "-m", "raw", "bead/"+item)
	f.Must(err)
	_, err = kernel.NextAction(f.State(), true)
	f.Must(err)
	for _, dependent := range f.Items[1:] {
		row, err := f.Store.Lookup(dependent + ".impl.a1")
		f.Must(err)
		if row != nil {
			t.Fatal("raw merge and close released dependent without completion")
		}
	}
	installCompleteKernelRun(t, f)
	result, err := Run(f.Root, item, f.SpecID, "", &s134CompletionExecutor{MockExecutor: &executor.MockExecutor{}, fixture: f}, CompleteOpts{})
	f.Must(err)
	if result == nil || !result.AlreadyMerged {
		t.Fatalf("replay did not report landed work: %+v", result)
	}
	for _, dependent := range f.Items[1:] {
		row, err := f.Store.Lookup(dependent + ".impl.a1")
		f.Must(err)
		if row == nil {
			t.Fatal("recorded completion did not release dependent")
		}
	}
}

func TestS134Complete_PreKernelSpecWarns(t *testing.T) {
	saveAndRestore(t)
	f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(kernel.NativeIDs))
	f.Must(os.MkdirAll(filepath.Join(f.Root, ".beads"), 0755))
	f.Must(os.WriteFile(filepath.Join(f.Root, ".beads/config.yaml"), []byte("types.custom: gate,step\n"), 0600))
	f.Must(os.WriteFile(filepath.Join(f.Root, ".mindspec/config.yaml"), []byte("enforcement:\n  panel_gate: true\n  panel_presence: warn\n"), 0600))
	_, err := f.Command("git", "add", ".")
	f.Must(err)
	_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fixture")
	f.Must(err)
	for _, branch := range []string{"spec/" + f.SpecID, "bead/" + f.Items[0]} {
		_, err = f.Command("git", "branch", branch)
		f.Must(err)
	}
	completeKernelMove(t, f, "work.txt")
	installCompleteKernelRun(t, f)
	closeBeadFn = func(ids ...string) error {
		row := f.Fake.Records[ids[0]]
		row.Status = "closed"
		f.Fake.Records[row.ID] = row
		return nil
	}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		row := f.Fake.Records[id]
		if row.Metadata == nil {
			row.Metadata = map[string]any{}
		}
		for key, v := range updates {
			row.Metadata[key] = v
		}
		f.Fake.Records[id] = row
		return nil
	}
	oldAdvisory := panelAdvisoryOut
	t.Cleanup(func() { panelAdvisoryOut = oldAdvisory })
	var warnings bytes.Buffer
	old := warnWriter
	warnWriter = &warnings
	panelAdvisoryOut = &warnings
	t.Cleanup(func() { warnWriter = old })
	result, err := Run(f.Root, f.Items[0], f.SpecID, "", &s134CompletionExecutor{MockExecutor: &executor.MockExecutor{}, fixture: f}, CompleteOpts{})
	if err != nil || result == nil || !result.BeadClosed {
		t.Fatalf("pre-kernel completion refused: %+v %v", result, err)
	}
	if !strings.Contains(warnings.String(), "warning:") || !strings.Contains(warnings.String(), "panel") {
		t.Fatalf("missing panel warning: %s", warnings.String())
	}
}

func TestS134Complete_BlockerAndGate(t *testing.T) {
	f, gate, accepted := completeKernelFixture(t)
	head := completeKernelMove(t, f, "unreviewed.go")
	row := f.Fake.Records[f.Items[0]]
	row.Status = "blocked"
	f.Fake.Records[row.ID] = row
	_, err := prepareKernelComplete(f.State(), f.Items[0], true)
	if err == nil || !strings.Contains(err.Error(), "blocked") || !strings.Contains(err.Error(), gate) || !strings.Contains(err.Error(), head) || !strings.Contains(err.Error(), accepted) {
		t.Fatalf("blocker hid gate checks: %v", err)
	}
}

func TestS134Complete_RunHaltWritesNothing(t *testing.T) {
	saveAndRestore(t)
	f, gate, _ := completeKernelFixture(t)
	d := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
	kerneltest.Answer(f, d, "halt")
	installCompleteKernelRun(t, f)
	before := kerneltest.Snapshot(t, f)
	_, err := Run(f.Root, f.Items[0], f.SpecID, "", &executor.MockExecutor{}, CompleteOpts{})
	if err == nil || !strings.Contains(err.Error(), "halt") {
		t.Fatalf("halted completion: %v", err)
	}
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("halted completion wrote")
	}
}

func TestS134Complete_RechecksBeforeClose(t *testing.T) {
	for _, mode := range []string{"halt", "gate", "judge"} {
		t.Run(mode, func(t *testing.T) {
			saveAndRestore(t)
			f, gate, head := completeKernelFixture(t)
			installCompleteKernelRun(t, f)
			called := false
			completeWorkDestructionPreflightFn = func(string, string, string, string, string) error {
				called = true
				switch mode {
				case "halt":
					d := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
					kerneltest.Answer(f, d, "halt")
				case "gate":
					f.Must(f.Store.SetStatusIf(gate, "closed", "open"))
				case "judge":
					kerneltest.Judge(f, gate, head, 1)
				}
				return nil
			}
			_, err := Run(f.Root, f.Items[0], f.SpecID, "", &s134CompletionExecutor{MockExecutor: &executor.MockExecutor{}, fixture: f}, CompleteOpts{})
			if !called || err == nil {
				t.Fatalf("late %s was not refused: called=%t err=%v", mode, called, err)
			}
			if f.State().Records[f.Items[0]].Status == "closed" || f.State().Merged[f.Items[0]] {
				t.Fatal("late refusal closed or merged work")
			}
		})
	}
}
