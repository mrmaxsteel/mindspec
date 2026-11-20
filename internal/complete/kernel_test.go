package complete

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func completeKernelFixture(t *testing.T) (*kerneltest.Fixture, string, string) {
	t.Helper()
	f := kerneltest.Enrolled(t, kerneltest.WithFake(kernel.NativeIDs))
	f.Must(os.MkdirAll(filepath.Join(f.Root, ".beads"), 0755))
	f.Must(os.WriteFile(filepath.Join(f.Root, ".beads", "config.yaml"), []byte("types.custom: gate,step\n"), 0600))
	_, err := f.Command("git", "add", ".")
	f.Must(err)
	_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fixture")
	f.Must(err)
	data, err := f.Command("git", "rev-parse", "HEAD")
	f.Must(err)
	head := strings.TrimSpace(string(data))
	for _, branch := range []string{"spec/" + f.SpecID, "bead/" + f.Items[0]} {
		_, err := f.Command("git", "branch", branch)
		f.Must(err)
	}
	head = completeKernelMove(t, f, "initial.txt")
	gate := f.Items[0] + ".review"
	kerneltest.PassGate(f, gate, head)
	return f, gate, head
}
func completeKernelMove(t *testing.T, f *kerneltest.Fixture, path string) string {
	t.Helper()
	_, err := f.Command("git", "switch", "bead/"+f.Items[0])
	f.Must(err)
	f.Must(os.MkdirAll(filepath.Dir(filepath.Join(f.Root, path)), 0755))
	f.Must(os.WriteFile(filepath.Join(f.Root, path), []byte("# change\n"), 0600))
	_, err = f.Command("git", "add", path)
	f.Must(err)
	_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "change")
	f.Must(err)
	head, err := f.Command("git", "rev-parse", "HEAD")
	f.Must(err)
	return strings.TrimSpace(string(head))
}
func TestS134Complete_AcceptedSubjectAndJudge(t *testing.T) {
	for _, carry := range []bool{false, true} {
		t.Run(fmt.Sprint(carry), func(t *testing.T) {
			f, gate, old := completeKernelFixture(t)
			moved := completeKernelMove(t, f, ".mindspec/config.yaml")
			judge := kerneltest.Judge(f, gate, moved, 1)
			_, err := prepareKernelComplete(f.State(), f.Items[0], true)
			if err == nil || !strings.Contains(err.Error(), moved) || !strings.Contains(err.Error(), old) || !strings.Contains(err.Error(), judge) {
				t.Fatalf("missing subject or judge: %v", err)
			}
			if !carry {
				return
			}
			r, _ := f.Store.Lookup(gate)
			receipt, _, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
			f.Must(err)
			receipt.CarryOvers = []kernel.CarryOver{{OldSubject: old, NewSubject: moved}}
			f.Must(f.Store.WriteReceipt(gate, receipt))
			f.Must(f.Store.WriteReceipt(judge, kernel.Receipt{Kind: "superseded", Rebind: "carry-over"}))
			f.Must(f.Store.SetStatusIf(judge, "open", "closed"))
			_, err = prepareKernelComplete(f.State(), f.Items[0], true)
			f.Must(err)
			_, err = kernel.NextAction(f.State(), true)
			f.Must(err)
			if r, err := f.Store.Lookup(gate + ".judge.a2"); err != nil || r != nil {
				t.Fatalf("new judge after carry-over: %v %v", r, err)
			}
		})
	}
}
func TestS134Complete_PanelOnlySubject(t *testing.T) {
	f, gate, head := completeKernelFixture(t)
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "passed", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
	kerneltest.PassGate(f, gate, head)
	completeKernelMove(t, f, filepath.ToSlash(filepath.Join(".mindspec/specs", f.SpecID, "reviews/passed/note.txt")))
	_, err := prepareKernelComplete(f.State(), f.Items[0], true)
	f.Must(err)
}
func TestS134Complete_ForcedRawAndIdempotent(t *testing.T) {
	f, gate, head := completeKernelFixture(t)
	item := f.Items[0]
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	if !kernel.RawClose(f.State(), item) {
		t.Fatal("no raw-close")
	}
	_, err := prepareKernelComplete(f.State(), item, true)
	f.Must(err)
	kerneltest.Judge(f, gate, head, 1)
	before := kerneltest.Snapshot(t, f)
	_, err = prepareKernelComplete(f.State(), item, true)
	if err == nil || !strings.Contains(err.Error(), "bd reopen "+item) {
		t.Fatalf("forced close: %v", err)
	}
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("forced close wrote")
	}
	f.Must(f.Store.SetStatusIf(item, "closed", "open"))
	kerneltest.PassGate(f, gate, head)
	_, err = prepareKernelComplete(f.State(), item, true)
	f.Must(err)
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	_, err = f.Command("git", "switch", "spec/"+f.SpecID)
	f.Must(err)
	_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "merge", "--no-ff", "-m", "Merge bead/"+item, "bead/"+item)
	f.Must(err)
	// The recorded work closes only after its accepted content lands.
	if !f.State().Merged[item] {
		t.Fatalf("not merged at %s", head)
	}
	if kernel.RawClose(f.State(), item) {
		t.Fatal("raw-close after merge")
	}
	flags, err := kernel.ApprovalBlockers(f.State())
	f.Must(err)
	for _, flag := range flags {
		if flag.Code == "forced-close" && flag.Step == item {
			t.Fatal("approval retained the repaired forced-close flag")
		}
	}
	before = kerneltest.Snapshot(t, f)
	_, err = prepareKernelComplete(f.State(), item, true)
	f.Must(err)
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("rerun wrote")
	}
}
func TestS134Complete_PassingGateCannotBeSkipped(t *testing.T) {
	for _, skip := range []string{"", "1"} {
		t.Run("skip="+skip, func(t *testing.T) {
			f, gate, _ := completeKernelFixture(t)
			t.Setenv("MINDSPEC_SKIP_PANEL", skip)
			f.Must(f.Store.SetStatusIf(gate, "closed", "open"))
			f.Must(f.Store.WriteMetadata(gate, map[string]any{kernel.KeyRegistration: nil}))
			_, err := prepareKernelComplete(f.State(), f.Items[0], true)
			if err == nil || !strings.Contains(err.Error(), gate) {
				t.Fatalf("missing gate refusal: %v", err)
			}
		})
	}
}
func TestS134Complete_EnablesDependentsOnlyAfterMerge(t *testing.T) {
	f, _, _ := completeKernelFixture(t)
	// A distinct work commit keeps the implementation unmerged until completion.
	head := completeKernelMove(t, f, "work.txt")
	kerneltest.PassGate(f, f.Items[0]+".review", head)
	_, err := kernel.NextAction(f.State(), true)
	f.Must(err)
	for _, item := range f.Items[1:] {
		r, err := f.Store.Lookup(item + ".impl.a1")
		f.Must(err)
		if r != nil {
			t.Fatal("dependent before completion")
		}
	}
	prior := map[string]bool{}
	for id := range f.State().Records {
		prior[id] = true
	}
	f.Must(f.Store.SetStatusIf(f.Items[0], "open", "closed"))
	_, err = f.Command("git", "switch", "spec/"+f.SpecID)
	f.Must(err)
	_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "merge", "--no-ff", "-m", "Merge bead/"+f.Items[0], "bead/"+f.Items[0])
	f.Must(err)
	f.Must(finishKernelComplete(f.State(), f.Items[0]))
	created := []string{}
	for id := range f.State().Records {
		if !prior[id] {
			created = append(created, id)
		}
	}
	expected := []string{f.Items[1] + ".impl.a1", f.Items[2] + ".impl.a1"}
	sort.Strings(created)
	sort.Strings(expected)
	if !reflect.DeepEqual(created, expected) {
		t.Fatalf("new record identities: got %v want %v", created, expected)
	}
	for _, item := range f.Items[1:] {
		r, err := f.Store.Lookup(item + ".impl.a1")
		f.Must(err)
		if r == nil {
			t.Fatal("missing dependent after completion")
		}
	}
}

func TestS134Complete_ClosedRetypedWitness(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(kernel.NativeIDs))
	f.Must(os.MkdirAll(filepath.Join(f.Root, ".beads"), 0755))
	f.Must(os.WriteFile(filepath.Join(f.Root, ".beads/config.yaml"), []byte("types.custom: gate,step\n"), 0600))
	for key, r := range f.Fake.Records {
		if _, ok, _ := r.Tuple(); ok {
			r.Status = "closed"
			r.Type = "chore"
			f.Fake.Records[key] = r
		}
	}
	epic := f.Fake.Records[f.Epic]
	delete(epic.Metadata, kernel.KeyRun)
	f.Fake.Records[f.Epic] = epic
	s := f.State()
	if !s.Enrolled || !s.MarkerMissing {
		t.Fatalf("closed witness lost: %+v", s)
	}
	_, err := prepareKernelComplete(s, f.Items[0], true)
	if err == nil || !strings.Contains(err.Error(), f.Items[0]+".review") {
		t.Fatalf("closed witness bypassed gate: %v", err)
	}
}
func TestS134Complete_HaltWritesNothing(t *testing.T) {
	f, gate, _ := completeKernelFixture(t)
	// Enrolment creates the runtime lock before this snapshot.
	f.Must(phase.WithPhaseLock(f.Root, func() error { return nil }))
	stale := f.State()
	d := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
	kerneltest.Answer(f, d, "halt")
	before := kerneltest.Snapshot(t, f)
	_, err := prepareKernelComplete(stale, f.Items[0], true)
	if err == nil || !strings.Contains(err.Error(), "halt") {
		t.Fatalf("halt not refused: %v", err)
	}
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("halted completion wrote")
	}
}

type s134CompletionExecutor struct {
	*executor.MockExecutor
	fixture *kerneltest.Fixture
}

func (e *s134CompletionExecutor) CompleteBead(item, branch, _, _ string, _ bool) error {
	_, err := e.fixture.Command("git", "switch", branch)
	if err != nil {
		return err
	}
	_, err = e.fixture.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "merge", "--no-ff", "-m", "Merge bead/"+item, "bead/"+item)
	return err
}
func TestS134Complete_RunAcceptsCarryOverAndPanelOnly(t *testing.T) {
	for _, mode := range []string{"carry-over", "panel-only", "raw-close", "unaccepted", "missing-gate"} {
		t.Run(mode, func(t *testing.T) {
			saveAndRestore(t)
			f, gate, old := completeKernelFixture(t)
			note := kernel.Record{ID: "foreign-note", Parent: f.Epic, Type: "task", Status: "open", Title: "foreign"}
			bug := kernel.Record{ID: "foreign-bug", Type: "bug", Status: "open", Title: "foreign blocker"}
			f.Fake.Records[note.ID] = note
			f.Fake.Records[bug.ID] = bug
			downstream := f.Fake.Records[f.Items[1]]
			downstream.Dependencies = append(downstream.Dependencies, kernel.Dependency{ID: bug.ID, Type: "blocks"})
			f.Fake.Records[downstream.ID] = downstream
			f.Must(f.Store.SetStatusIf(f.Items[0], "open", "in_progress"))
			if mode == "raw-close" {
				f.Must(f.Store.SetStatusIf(f.Items[0], "in_progress", "closed"))
				if !kernel.RawClose(f.State(), f.Items[0]) {
					t.Fatal("fixture did not leave a raw close")
				}
			}
			if mode == "missing-gate" {
				f.Must(f.Store.SetStatusIf(gate, "closed", "open"))
				f.Must(f.Store.WriteMetadata(gate, map[string]any{kernel.KeyRegistration: nil}))
				t.Setenv("MINDSPEC_SKIP_PANEL", "1")
			} else if mode == "panel-only" {
				kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "passed", Round: 1, Subject: old, Slots: []string{"R1", "R2"}})
				kerneltest.PassGate(f, gate, old)
				completeKernelMove(t, f, filepath.ToSlash(filepath.Join(".mindspec/specs", f.SpecID, "reviews/passed/note.txt")))
			} else if mode != "raw-close" {
				if mode == "carry-over" {
					kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "passed", Round: 1, Subject: old, Slots: []string{"R1", "R2"}})
					kerneltest.PassGate(f, gate, old)
					_, err := f.Command("git", "add", ".")
					f.Must(err)
					_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "panel registration")
					f.Must(err)
				}
				head := completeKernelMove(t, f, ".mindspec/config.yaml")
				if mode == "carry-over" {
					r, err := f.Store.Lookup(gate)
					f.Must(err)
					receipt, _, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
					f.Must(err)
					receipt.CarryOvers = []kernel.CarryOver{{OldSubject: old, NewSubject: head}}
					f.Must(f.Store.WriteReceipt(gate, receipt))
				}
			}
			installCompleteKernelRun(t, f)
			exec := &s134CompletionExecutor{MockExecutor: &executor.MockExecutor{}, fixture: f}
			priorRecords := map[string]kernel.Record{}
			data, err := json.Marshal(f.Fake.Records)
			f.Must(err)
			f.Must(json.Unmarshal(data, &priorRecords))
			result, err := Run(f.Root, f.Items[0], f.SpecID, "", exec, CompleteOpts{})
			if mode == "unaccepted" || mode == "missing-gate" {
				if err == nil || !strings.Contains(err.Error(), gate) {
					t.Fatalf("missing gate refusal: %+v %v", result, err)
				}
				return
			}
			if err != nil || result == nil || !result.BeadClosed {
				t.Fatalf("complete did not accept %s: %+v %v", mode, result, err)
			}
			if !f.State().Merged[f.Items[0]] || kernel.RawClose(f.State(), f.Items[0]) {
				t.Fatal("completion did not merge and clear raw-close")
			}
			for _, item := range f.Items[2:] {
				r, err := f.Store.Lookup(item + ".impl.a1")
				f.Must(err)
				if r == nil {
					t.Fatal("completion pass omitted dependent")
				}
			}
			allowed := map[string]bool{f.Epic: true}
			state := f.State()
			for _, ids := range [][]string{state.Items, state.Gates, state.Steps} {
				for _, id := range ids {
					if row, ok := state.Records[id]; ok {
						allowed[row.ID] = true
					}
				}
			}
			for id, row := range f.Fake.Records {
				if !reflect.DeepEqual(priorRecords[id], row) && !allowed[id] {
					t.Fatalf("completion changed unrelated record %s", id)
				}
			}
			for id := range priorRecords {
				if _, ok := f.Fake.Records[id]; !ok {
					t.Fatalf("completion deleted record %s", id)
				}
			}
			if !reflect.DeepEqual(f.Fake.Records[note.ID], note) || !reflect.DeepEqual(f.Fake.Records[bug.ID], bug) {
				t.Fatal("completion changed foreign records")
			}
			blocked, err := f.Store.Lookup(f.Items[1] + ".impl.a1")
			f.Must(err)
			if blocked != nil {
				t.Fatal("completion ignored the foreign blocker")
			}
			before := kerneltest.Snapshot(t, f)
			f.Fake.Hook = func(args []string) error {
				if len(args) > 1 && args[0] == "update" && args[1] == f.Items[0] {
					return errors.New("replay wrote completed work item")
				}
				return nil
			}
			replay, err := Run(f.Root, f.Items[0], f.SpecID, "", exec, CompleteOpts{})
			f.Must(err)
			if !replay.AlreadyMerged || !strings.Contains(FormatResult(replay), "already complete") || strings.Contains(FormatResult(replay), "Mode: idle") {
				t.Fatalf("dishonest replay summary: %+v %s", replay, FormatResult(replay))
			}
			if kerneltest.Snapshot(t, f) != before {
				t.Fatal("completion rerun wrote state")
			}
		})
	}
}

func TestS134Complete_AncestorWithoutCompletionStillNeedsGate(t *testing.T) {
	f, gate, _ := completeKernelFixture(t)
	_, err := f.Command("git", "branch", "-f", "spec/"+f.SpecID, "bead/"+f.Items[0])
	f.Must(err)
	f.Must(f.Store.WriteMetadata(gate, map[string]any{kernel.KeyClose: nil}))
	if !f.State().Merged[f.Items[0]] {
		t.Fatal("fixture branch is not an ancestor")
	}
	_, err = prepareKernelComplete(f.State(), f.Items[0], true)
	if err == nil || !strings.Contains(err.Error(), gate) {
		t.Fatalf("ancestry bypassed the gate: %v", err)
	}
}

func installCompleteKernelRun(t *testing.T, f *kerneltest.Fixture) {
	t.Helper()
	stubPhaseEpic(t, f.SpecID, f.Epic)
	findEpicForBeadFn = func(string) (string, string, error) { return f.Epic, f.SpecID, nil }
	runBDFn = f.Fake.Run
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{Name: "worktree-" + f.Items[0], Path: f.Root, Branch: "bead/" + f.Items[0]}}, nil
	}
	completeGetMetadataFn = func(id string) (map[string]interface{}, error) {
		r, err := f.Store.Lookup(id)
		if r == nil {
			return nil, err
		}
		return r.Metadata, err
	}
	completeMergeMetadataFn = f.Store.WriteMetadata
	closeBeadFn = func(ids ...string) error {
		r, err := f.Store.Lookup(ids[0])
		if err != nil {
			return err
		}
		return f.Store.SetStatusIf(ids[0], r.Status, "closed")
	}
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		r, err := f.Store.Lookup(id)
		return next.BeadInfo{ID: id, Status: r.Status}, err
	}

}

func TestS134Complete_EnrolledSkipsLegacyPanelGate(t *testing.T) {
	saveAndRestore(t)
	f, _, head := completeKernelFixture(t)
	f.Must(os.WriteFile(filepath.Join(f.Root, ".mindspec/config.yaml"), []byte("enforcement:\n  panel_gate: true\n  panel_presence: enforce\n"), 0600))
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	cfg, err := config.Load(f.Root)
	f.Must(err)
	if !cfg.Enforcement.PanelGate {
		t.Fatal("fixture did not enable the legacy gate")
	}
	installCompleteKernelRun(t, f)
	fake := deferralBD(t, f.Epic)
	setDeferredIssue(t, fake, "closed")
	oldSkip, oldScan := panelSkipEnvFn, panelScanFn
	t.Cleanup(func() { panelSkipEnvFn = oldSkip; panelScanFn = oldScan })
	panelSkipEnvFn = func() bool { return false }
	scans := 0
	panelScanFn = func(...string) []panel.Registration { scans++; return nil }
	f.Must(f.Store.WriteMetadata(f.Items[0], map[string]any{stageMarkerAttestationKey: head, "panel_deferred_entries": []PanelDeferredEntry{{Spec: f.SpecID, Panel: "legacy", Deferral: panel.Deferral{Slot: "R1", Round: 1, Finding: "slot", FindingSHA256: strings.Repeat("a", 64), FindingText: "follow-up finding", Bead: "follow-up", By: "operator", CreatedAt: "2026-01-02T03:04:05Z"}, FindingTexts: []string{"follow-up finding"}}}}))
	ex := &s134CompletionExecutor{MockExecutor: &executor.MockExecutor{}, fixture: f}
	ex.ReadStageMarkerFn = func(string) (string, string, bool, error) { return head, stageArtifactSync, true, nil }
	ex.RevParseRefFn = func(string, string) (string, error) { return head, nil }
	result, err := Run(f.Root, f.Items[0], f.SpecID, "", ex, CompleteOpts{})
	if err != nil || result == nil || !result.BeadClosed {
		t.Fatalf("enrolled resume used the legacy panel gate: %+v %v", result, err)
	}
	if scans != 0 {
		t.Fatal("enrolled resume scanned legacy panel state")
	}
	if len(ex.CallsTo("ReadStageMarker")) == 0 || len(ex.CallsTo("SnapshotWorktreeTree")) != 0 {
		t.Fatal("fixture did not resume the attested stage")
	}
}
