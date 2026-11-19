package approve

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/journal"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func s134ApproveFixture(t *testing.T) (*kerneltest.Fixture, *executor.MockExecutor, *bytes.Buffer) {
	t.Helper()
	saveAndRestore(t)
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	t.Setenv(journal.StateDirEnv, t.TempDir())
	for _, gate := range f.State().Gates {
		kerneltest.PassGate(f, gate, "subject")
	}
	for _, item := range f.Items {
		f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	}
	head, err := f.Command("git", "rev-parse", "HEAD")
	f.Must(err)
	for _, branch := range append([]string{"spec/" + f.SpecID}, "bead/"+f.Items[0], "bead/"+f.Items[1], "bead/"+f.Items[2]) {
		_, err = f.Command("git", "branch", branch, strings.TrimSpace(string(head)))
		f.Must(err)
	}
	f.Must(f.Store.WriteMetadata(f.Epic, map[string]any{"mindspec_phase": "review", "mindspec_phase_semantics": "v2"}))
	cfgPath := filepath.Join(f.Root, ".mindspec", "config.yaml")
	f.Must(os.WriteFile(cfgPath, []byte("enforcement:\n  panel_gate: false\n"), 0644))
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	cfg, err := config.Load(f.Root)
	f.Must(err)
	wt, err := workspace.SpecWorktreePath(f.Root, cfg, f.SpecID)
	f.Must(err)
	f.Must(os.MkdirAll(wt, 0755))
	oldState := implKernelStateFn
	implKernelStateFn = func(root, id string) (*kernel.State, error) { return implKernelState(f.Store, id, f.SpecDir) }
	t.Cleanup(func() { implKernelStateFn = oldState })
	oldWriter := warnWriter
	out := &bytes.Buffer{}
	warnWriter = out
	t.Cleanup(func() { warnWriter = oldWriter })
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		if contains(args, "--type=epic") {
			return json.Marshal([]kernel.Record{f.Fake.Records[f.Epic]})
		}
		rows := []kernel.Record{}
		for _, item := range f.Items {
			rows = append(rows, f.Fake.Records[item])
		}
		return json.Marshal(rows)
	}))
	t.Cleanup(phase.SetRunBDForTest(f.Fake.Run))
	implRunBDFn = f.Fake.Run
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		if args[0] == "close" {
			r := f.Fake.Records[args[1]]
			r.Status = "closed"
			f.Fake.Records[r.ID] = r
			return []byte("{}"), nil
		}
		return f.Fake.Run(args...)
	}
	implPhaseMetadataFn = f.Store.WriteMetadata
	mock := &executor.MockExecutor{CommitCountResult: 1, ResumeFinalizedEpicResult: executor.FinalizeResult{MergeStrategy: "direct"}}
	return f, mock, out
}
func s134Close(t *testing.T, f *kerneltest.Fixture, id string, receipt *kernel.Receipt) {
	t.Helper()
	if receipt != nil {
		f.Must(f.Store.WriteReceipt(id, *receipt))
	}
	f.Must(f.Store.SetStatusIf(id, "open", "closed"))
}
func s134Slot(f *kerneltest.Fixture) string {
	id := f.Items[0] + ".review.r1.R1"
	f.Create(id, f.Items[0]+".review")
	return id
}
func TestS134ImplApprove_RefusalTable(t *testing.T) {
	for _, code := range []string{"open-step", "open-decision", "unreceipted-close", "raw-close", "forced-close", "foreign-child-open"} {
		t.Run(code, func(t *testing.T) {
			f, mock, _ := s134ApproveFixture(t)
			id := ""
			switch code {
			case "open-step":
				id = s134Slot(f)
			case "open-decision":
				id = kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
			case "unreceipted-close":
				id = s134Slot(f)
				s134Close(t, f, id, nil)
			case "raw-close":
				id = f.Items[0]
				// Remove the bead branch and its landed binding so the item is unmerged.
				_, err := f.Command("git", "branch", "-D", "bead/"+id)
				f.Must(err)
			case "forced-close":
				id = f.Items[0]
				s134Slot(f)
				// A direct child of the item holds the close even though its gate passed.
				f.Create(id+".impl.a1", id)
			case "foreign-child-open":
				id = "foreign-note"
				f.Fake.Records[id] = kernel.Record{ID: id, Parent: f.Epic, Type: "task", Status: "open", Title: "note"}
			}
			before := kerneltest.Snapshot(t, f)
			_, err := ApproveImpl(f.Root, f.SpecID, mock)
			if err == nil || !strings.Contains(err.Error(), code+": "+id) {
				t.Fatalf("missing %s %s: %v", code, id, err)
			}
			if kerneltest.Snapshot(t, f) != before {
				t.Fatal("refusal changed snapshot")
			}
			if len(mock.CallsTo("FinalizeEpic")) != 0 {
				t.Fatal("refusal finalized")
			}
		})
	}
	t.Run("completed-retry-passes", func(t *testing.T) {
		f, mock, _ := s134ApproveFixture(t)
		id := s134Slot(f)
		s134Close(t, f, id, nil)
		retry := id + ".a2"
		f.Create(retry, f.Items[0]+".review")
		f.Must(f.Store.WriteMetadata(retry, kernel.Metadata(kernel.KeyFrom, kernel.From(id))))
		s134Close(t, f, retry, &kernel.Receipt{Kind: "passed", Retally: "retry-verdict"})
		if _, err := ApproveImpl(f.Root, f.SpecID, mock); err != nil {
			t.Fatal(err)
		}
		calls := mock.CallsTo("FinalizeEpic")
		if len(calls) != 1 || !reflect.DeepEqual(calls[0].Args[3], f.Items) {
			t.Fatalf("finalize allow set: %+v", calls)
		}
	})
}
func TestS134ImplApprove_AuditAndRerun(t *testing.T) {
	f, mock, out := s134ApproveFixture(t)
	for _, hatch := range []string{"allow-doc-skew", "override-adr", "allow-not-ready", "allow-net-deletion"} {
		f.Must(journal.AppendSuccessEvent(journal.Event{Argv0: "mindspec", Command: "next", EscapeHatch: hatch, Version: "0.1.0", OS: "linux"}))
	}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	item := f.Items[0]
	gate := item + ".review"
	f.Must(f.Store.WriteMetadata(item, kernel.Metadata(kernel.KeyDefer, kernel.DeferRecord{Finding: "finding", To: "other", Reason: "defer", At: now})))
	f.Must(f.Store.WriteMetadata(item, kernel.Metadata(kernel.KeyTake, kernel.TakeReason{Reason: "take", Generation: 2, At: now})))
	decision := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
	kerneltest.Answer(f, decision, "fix")
	slot := s134Slot(f)
	s134Close(t, f, slot, nil)
	kerneltest.Repaired(f, slot, kernel.Receipt{Kind: "passed", Retally: "repair"})
	receipt := kernel.Receipt{Kind: "passed", Retally: "passing-tally", CarryOvers: []kernel.CarryOver{{OldSubject: "old", NewSubject: "new", Judge: "judge", Receipt: &kernel.Receipt{Kind: "passed", Retally: "judgement"}}}}
	f.Must(f.Store.WriteReceipt(gate, receipt))
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRePanel, kernel.RePanel{SlotCount: 3, Paths: []string{"source.go"}})))
	f.Must(f.Store.WriteMetadata(item, kernel.Metadata(kernel.KeyRawClaims, []kernel.RawClaim{{At: now}})))
	for run := 0; run < 2; run++ {
		out.Reset()
		if _, err := ApproveImpl(f.Root, f.SpecID, mock); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		for _, name := range []string{"--allow-doc-skew", "--override-adr", "--allow-not-ready", "--allow-net-deletion", "deferrals", "takeovers", "decision-answers", "raw-claims", "unreceipted-closes", "carry-overs", "re-panels"} {
			if !strings.Contains(out.String(), name+" | 1") {
				t.Errorf("run %d missing count 1 for %s:\n%s", run, name, out.String())
			}
		}
		if !strings.Contains(out.String(), "3 slots") || !strings.Contains(out.String(), decision+" fix ") {
			t.Fatalf("audit details missing:\n%s", out.String())
		}
		if run == 0 {
			head, err := f.Command("git", "rev-parse", "spec/"+f.SpecID)
			f.Must(err)
			f.Must(f.Store.WriteMetadata(f.Epic, map[string]any{"mindspec_finalize_tip": strings.TrimSpace(string(head))}))
			_, err = f.Command("git", "branch", "-D", "spec/"+f.SpecID)
			f.Must(err)
			implSpecBranchExistsFn = func(string, string) (bool, error) { return false, nil }
		}
	}
	calls := mock.CallsTo("ResumeFinalizedEpic")
	if len(calls) != 1 || !reflect.DeepEqual(calls[0].Args[2], f.Items) {
		t.Fatalf("resume allow set: %+v", calls)
	}
}
func TestS134ImplApprove_ReadEnrolmentWitnesses(t *testing.T) {
	for _, witness := range []string{"marker", "kernel-key", "panel", "legacy"} {
		t.Run(witness, func(t *testing.T) {
			f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
			if witness != "marker" {
				r := f.Fake.Records[f.Epic]
				delete(r.Metadata, kernel.KeyRun)
				f.Fake.Records[f.Epic] = r
			}
			if witness == "panel" || witness == "legacy" {
				for _, gate := range f.State().Gates {
					delete(f.Fake.Records, f.ID(gate))
				}
			}
			if witness == "panel" {
				dir := filepath.Join(f.SpecDir, "reviews", "witness")
				f.Must(os.MkdirAll(dir, 0755))
				f.Must(os.WriteFile(filepath.Join(dir, "panel.json"), []byte(`{"orchestrated":true}`), 0644))
			}
			s, err := implKernelState(f.Store, f.SpecID, f.SpecDir)
			f.Must(err)
			if (s != nil) != (witness != "legacy") {
				t.Fatalf("witness %s: %+v", witness, s)
			}
		})
	}
}

func TestS134ImplApprove_AuditReadErrorIsAdvisory(t *testing.T) {
	f, mock, out := s134ApproveFixture(t)
	f.Must(f.Store.WriteMetadata(f.Items[0], kernel.Metadata(kernel.KeyTake, "damaged audit")))
	if _, err := ApproveImpl(f.Root, f.SpecID, mock); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "warning: cannot read kernel audit") {
		t.Fatalf("audit read error was hidden: %s", out.String())
	}
}
