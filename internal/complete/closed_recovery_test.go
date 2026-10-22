package complete

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

func stubClosedWindowChild(t *testing.T) {
	t.Helper()
	list := phaseEpicListJSONStub(closingSpec, "epic-window", state.ModeImplement)
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, arg := range args {
			if arg == "--parent" {
				return json.Marshal([]phase.ChildInfo{{ID: closingBead, Status: "closed"}})
			}
		}
		return list(args...)
	}))
}

func requireClosedConflictRecovery(t *testing.T, err error, path string) {
	t.Helper()
	if err == nil {
		t.Fatal("closed recovery must report the preserved conflict")
	}
	text := err.Error()
	want := "recovery: mindspec complete " + closingBead + " --resolve-merge"
	lines := strings.Split(text, "\n")
	var recoveries []string
	for _, line := range lines {
		if strings.HasPrefix(line, "recovery: ") {
			recoveries = append(recoveries, line)
		}
	}
	if len(recoveries) != 1 || recoveries[0] != want || lines[len(lines)-1] != want || strings.Contains(text, "--merge-only") {
		t.Fatalf("want one final closed-bead recovery %q, got:\n%s", want, text)
	}
	if !strings.Contains(text, "\nconflicted files:\n  "+path+"\n") || strings.Contains(text, `\n`) {
		t.Fatalf("conflict lines must stay unquoted:\n%s", text)
	}
}

func TestComplete_ClosedSourceConflictRecovery(t *testing.T) {
	f := sourceConflictWindow(t)
	stubClosedWindowChild(t)
	ledger := f.registerLedger()
	f.closed = true
	_, err := f.run(CompleteOpts{})
	requireClosedConflictRecovery(t, err, "README.md")
	if !gitutil.MergeInProgress(f.specWT) || f.closeCalls != 0 {
		t.Fatalf("closed recovery lost conflict or called close: closes=%d", f.closeCalls)
	}
	if _, err := os.Stat(f.beadWT); err != nil {
		t.Fatalf("conflict removed bead worktree: %v", err)
	}
	writeFile(t, f.specWT, "README.md", "resolved\n")
	gitRun(t, f.specWT, "add", "README.md")
	r, err := f.run(CompleteOpts{ResolveMerge: true})
	if err != nil || !r.BeadClosed || !r.WorktreeRemoved || f.closeCalls != 0 {
		t.Fatalf("closed source re-entry: %+v err=%v closes=%d", r, err, f.closeCalls)
	}
	f.assertPublished(ledger)
	if f.meta["mindspec_landed_merge_sha"] == nil {
		t.Fatalf("closed recovery did not write the binding: %v", f.meta)
	}
}

type closedMergeFailureExecutor struct {
	*windowExecutor
	failure error
	calls   int
}

func (e *closedMergeFailureExecutor) MergeBead(string, string, string, bool) error {
	e.calls++
	return e.failure
}

func TestComplete_ClosedNonConflictRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, message, recovery string
	}{
		{
			name:     "layout",
			message:  "layout regression: refusing to merge a legacy branch into a flat target",
			recovery: "git rebase spec/" + closingSpec + " bead/" + closingBead,
		},
		{
			name:     "destruction",
			message:  "refusing to merge: the stale snapshot would delete landed work",
			recovery: "inspect the branch, then re-run with --allow-net-deletion \"<reason>\" to proceed anyway — then mindspec complete " + closingBead,
		},
		{
			name:     "evidence",
			message:  "could not determine whether merging would destroy work",
			recovery: "retry once the underlying failure is resolved, or re-run with --allow-net-deletion \"<reason>\" to proceed without verified evidence — then mindspec complete " + closingBead,
		},
		{
			name:     "other-bead",
			message:  "another bead has a preserved merge",
			recovery: "mindspec complete other-1 --merge-only --resolve-merge",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newWindowFixture(t)
			stubClosedWindowChild(t)
			f.closed = true
			before := gateRevParse(t, f.root, "spec/"+closingSpec)
			failure := guard.NewFailure(tc.message, tc.recovery)
			ex := &closedMergeFailureExecutor{windowExecutor: f.ex, failure: failure}
			_, err := Run(f.root, closingBead, closingSpec, "", ex, CompleteOpts{})
			requireWindowRefusal(t, err, tc.message)
			lines := strings.Split(err.Error(), "\n")
			want := guard.RecoveryPrefix + tc.recovery
			if lines[len(lines)-1] != want || strings.HasSuffix(err.Error(), "recovery: mindspec complete "+closingBead+" --resolve-merge") {
				t.Fatalf("want final guard recovery %q, got:\n%s", want, err)
			}
			if err != failure {
				t.Fatalf("non-conflict failure must return intact: %v", err)
			}
			if ex.calls != 1 || f.closeCalls != 0 || len(f.meta) != 0 || gateRevParse(t, f.root, "spec/"+closingSpec) != before {
				t.Fatalf("guard failure changed completion state: merge calls=%d closes=%d metadata=%v", ex.calls, f.closeCalls, f.meta)
			}
			if _, err := os.Stat(f.beadWT); err != nil {
				t.Fatalf("guard failure removed bead worktree: %v", err)
			}
		})
	}
}

func TestComplete_ClosedStatusReadFailureRefuses(t *testing.T) {
	for _, storedPhase := range []string{state.ModeImplement, "invalid"} {
		t.Run(storedPhase, func(t *testing.T) {
			f := newWindowFixture(t)
			f.closed = true
			before := gateRevParse(t, f.root, "spec/"+closingSpec)
			childReads := 0
			list := phaseEpicListJSONStub(closingSpec, "epic-window", state.ModeImplement)
			t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg == "--parent" {
						childReads++
						return nil, errors.New("child status read failed")
					}
				}
				return list(args...)
			}))
			t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "show" {
					return json.Marshal([]phase.EpicInfo{{ID: "epic-window", Status: "open", Metadata: map[string]interface{}{
						"mindspec_phase": storedPhase, phase.PhaseSemanticsV2MarkerKey: phase.PhaseSemanticsV2Value,
					}}})
				}
				t.Fatalf("unexpected phase mutation: %v", args)
				return nil, errors.New("unexpected phase mutation")
			}))
			t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error {
				t.Fatal("status read failure must not change phase metadata")
				return nil
			}))
			_, err := f.run(CompleteOpts{})
			requireWindowRefusal(t, err, "child status read failed")
			stage := "reading bead status"
			if storedPhase == "invalid" {
				stage = "reading implementation phase"
			}
			requireWindowRefusal(t, err, stage)
			want := "recovery: mindspec complete " + closingBead
			lines := strings.Split(err.Error(), "\n")
			if len(lines) != 2 || lines[1] != want {
				t.Fatalf("status read failure must end in one runnable recovery: %v", err)
			}
			if childReads == 0 {
				t.Fatal("test did not inject a child read failure")
			}
			if f.closeCalls != 0 || len(f.meta) != 0 || gitutil.MergeInProgress(f.specWT) || gateRevParse(t, f.root, "spec/"+closingSpec) != before {
				t.Fatalf("status read failure mutated completion state: closes=%d metadata=%v", f.closeCalls, f.meta)
			}
			if _, err := os.Stat(f.beadWT); err != nil {
				t.Fatalf("status read failure removed worktree: %v", err)
			}
			if gateRevParse(t, f.root, "bead/"+closingBead) != f.tip {
				t.Fatal("status read failure changed bead branch")
			}
		})
	}
}

func TestComplete_ClosedEpicReadFailureRefuses(t *testing.T) {
	f := newWindowFixture(t)
	f.closed = true
	before := gateRevParse(t, f.root, "spec/"+closingSpec)
	reads := 0
	t.Cleanup(phase.SetListJSONForTest(func(...string) ([]byte, error) {
		reads++
		return nil, errors.New("epic status read failed")
	}))
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "epic status read failed")
	lines := strings.Split(err.Error(), "\n")
	if len(lines) != 2 || lines[1] != "recovery: mindspec complete "+closingBead {
		t.Fatalf("epic read failure must end in one runnable recovery: %v", err)
	}
	if reads == 0 || f.closeCalls != 0 || len(f.meta) != 0 || gateRevParse(t, f.root, "spec/"+closingSpec) != before {
		t.Fatalf("epic read failure mutated completion state: reads=%d closes=%d metadata=%v", reads, f.closeCalls, f.meta)
	}
	if _, err := os.Stat(f.beadWT); err != nil {
		t.Fatalf("epic read failure removed worktree: %v", err)
	}
	if gateRevParse(t, f.root, "bead/"+closingBead) != f.tip {
		t.Fatal("epic read failure changed bead branch")
	}
}
