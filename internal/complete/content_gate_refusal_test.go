package complete

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func assertContentRefusalState(t *testing.T, f *windowFixture, tip, specTip string) {
	t.Helper()
	if got := gateRevParse(t, f.root, "bead/"+closingBead); got != tip {
		t.Errorf("refusal advanced bead tip: got %s, want %s", got, tip)
	}
	if got := gateRevParse(t, f.root, "spec/"+closingSpec); got != specTip {
		t.Errorf("refusal advanced spec tip: got %s, want %s", got, specTip)
	}
	if gitutil.MergeInProgress(f.specWT) {
		t.Error("refusal started a merge")
	}
	for _, key := range []string{mergeOnlyPendingKey, mergeOnlySHAKey, stageMarkerAttestationKey} {
		if value := f.meta[key]; value != nil {
			t.Errorf("refusal wrote metadata %s=%v", key, value)
		}
	}
	if _, _, present, err := f.ex.ReadStageMarker(closingBead); err != nil || present {
		t.Errorf("refusal wrote a stage marker: present=%v err=%v", present, err)
	}
	if f.closeCalls != 0 {
		t.Errorf("refusal closed bead: calls=%d", f.closeCalls)
	}
}

func testPreflightOwnershipRefusal(t *testing.T, path string) {
	t.Helper()
	f := ownershipWindow(t)
	opts := CompleteOpts{MergeOnly: true}
	switch path {
	case "snapshot":
		writeFile(t, f.beadWT, "internal/widget/widget.go", "package widget\n\nconst Value = 2\n")
	case "worktreeless":
		gitRun(t, f.root, "worktree", "remove", "--force", f.beadWT)
		// The root has no user work. Ignore only the generated phase lock.
		checkDirtyTreeFn = func(string, string) ([]string, []string, error) { return nil, nil, nil }
	case "closed":
		stubClosedWindowChild(t)
		f.closed = true
		opts = CompleteOpts{}
	}
	preflight := errors.New("net deletion detected")
	calls := 0
	completeWorkDestructionPreflightFn = func(string, string, string, string, string) error {
		calls++
		return preflight
	}
	tip := gateRevParse(t, f.root, "bead/"+closingBead)
	specTip := gateRevParse(t, f.root, "spec/"+closingSpec)
	var err error
	if path == "committed" {
		// Exercise the committed-range deferral directly, with a real landing conflict.
		specDir, specErr := workspace.SpecDir(f.root, closingSpec)
		if specErr != nil {
			t.Fatal(specErr)
		}
		_, conflictErr := ownershipLandingTree(f.root, specDir, "bead/"+closingBead, "bead/"+closingBead, f.ex)
		var conflict *OwnershipLandingConflict
		if !errors.As(conflictErr, &conflict) {
			t.Fatalf("fixture must have an ownership conflict: %v", conflictErr)
		}
		opts.deferOwnershipConflict = true
		opts.workDestructionErr = workDestructionReport(completeWorkDestructionPreflightFn(f.root, "bead/"+closingBead, "spec/"+closingSpec, "", ""))
		err = runCommittedRangeGate(f.root, closingBead, "bead/"+closingBead, specDir, f.parent, f.ex, opts, io.Discard)
	} else {
		message := ""
		if path == "snapshot" {
			message = "snapshot source"
		}
		_, err = Run(f.root, closingBead, closingSpec, message, f.ex, opts)
	}
	if calls != 1 {
		t.Errorf("preflight calls=%d, want 1", calls)
	}
	if err == nil || !strings.Contains(err.Error(), "work-destruction preflight:") || !strings.Contains(err.Error(), preflight.Error()) {
		t.Errorf("refusal lost preflight: %v", err)
	}
	if !errors.Is(err, preflight) {
		t.Errorf("refusal lost the preflight error chain: %v", err)
	}
	assertContentRefusalState(t, f, tip, specTip)
}

func TestContentGateRefusal_SnapshotMergeOnlyPreflight(t *testing.T) {
	testPreflightOwnershipRefusal(t, "snapshot")
}

func TestContentGateRefusal_CommittedRangeMergeOnlyPreflight(t *testing.T) {
	testPreflightOwnershipRefusal(t, "committed")
}

func TestContentGateRefusal_NoWorktreePreflight(t *testing.T) {
	testPreflightOwnershipRefusal(t, "worktreeless")
}

func TestContentGateRefusal_ClosedBeadPreflight(t *testing.T) {
	testPreflightOwnershipRefusal(t, "closed")
}

func TestContentGateRefusal_Supersede(t *testing.T) {
	for _, kind := range []string{"doc", "destruction", "all", "pass"} {
		t.Run(kind, func(t *testing.T) {
			f, _ := s133CheckFixture(t, "warn", kind)
			f.registerLedger()
			calls := 0
			adrCreateWithIDFn = func(string, string, string, adr.CreateOpts) (string, error) {
				calls++
				return "", nil
			}
			tip := gateRevParse(t, f.root, "bead/"+closingBead)
			specTip := gateRevParse(t, f.root, "spec/"+closingSpec)
			_, err := f.run(CompleteOpts{SupersedeADR: "ADR-9002"})
			if kind == "pass" {
				if err != nil || calls != 1 || !f.closed {
					t.Fatalf("passing supersede must create once and complete: err=%v calls=%d closed=%v", err, calls, f.closed)
				}
				return
			}
			if err == nil || calls != 0 {
				t.Errorf("refusal created a placeholder: err=%v calls=%d", err, calls)
			}
			want := map[string][]string{"doc": {"doc-sync:"}, "destruction": {"work-destruction preflight:"}, "all": {"doc-sync:", "adr-divergence:", "work-destruction preflight:"}}[kind]
			for _, gate := range want {
				if err == nil || !strings.Contains(err.Error(), gate) {
					t.Errorf("refusal lost %s: %v", gate, err)
				}
			}
			assertContentRefusalState(t, f, tip, specTip)
		})
	}
}

func TestContentGateRefusal_SupersedeReportsADR(t *testing.T) {
	for _, gate := range []string{"doc", "preflight"} {
		t.Run(gate, func(t *testing.T) {
			saveAndRestore(t)
			calls := 0
			adrCreateWithIDFn = func(string, string, string, adr.CreateOpts) (string, error) { calls++; return "", nil }
			docs, adrs := &validate.Result{}, &validate.Result{}
			adrs.AddError("adr-divergence-uncovered", "source has no coverage")
			opts := CompleteOpts{SupersedeADR: "ADR-9002"}
			if gate == "doc" {
				docs.AddError("internal-docs", "domain docs missing")
			} else {
				opts.workDestructionErr = workDestructionReport(errors.New("net deletion detected"))
			}
			err := reportContentGates("", closingBead, opts, docs, adrs, nil, io.Discard)
			if calls != 0 || err == nil || !strings.Contains(err.Error(), "adr-divergence-uncovered") {
				t.Fatalf("must report ADR refusal without creating: calls=%d err=%v", calls, err)
			}
		})
	}
}

type refusalSnapshotExecutor struct{ executor.Executor }

func (e refusalSnapshotExecutor) SnapshotWorktreeTree(string, []string) (string, error) {
	return "", errors.New("snapshot unavailable")
}

func TestContentGateRefusal_NoArtifactNormalization(t *testing.T) {
	for _, path := range []string{"snapshot", "snapshot-error", "no-worktree", "no-worktree-commit", "resume"} {
		t.Run(path, func(t *testing.T) {
			f, _ := s133CheckFixture(t, "warn", "all")
			preflight := errors.New("net deletion detected")
			specDir, specErr := workspace.SpecDir(f.root, closingSpec)
			if specErr != nil {
				t.Fatal(specErr)
			}
			in := materializationInput{exec: f.ex, root: f.root, beadID: closingBead, beadHead: "bead/" + closingBead, wtPath: f.beadWT, specDir: specDir, base: f.parent, warnWriter: io.Discard, opts: CompleteOpts{workDestructionErr: workDestructionReport(preflight)}}
			dirtCalls := 0
			checkDirtyTreeFn = func(string, string) ([]string, []string, error) {
				dirtCalls++
				return nil, nil, errors.New("artifact export must not run")
			}
			switch path {
			case "snapshot-error":
				in.exec = refusalSnapshotExecutor{f.ex}
			case "no-worktree", "no-worktree-commit":
				in.wtPath = ""
				if path == "no-worktree-commit" {
					in.commitMsg = "source snapshot"
				}
			case "resume":
				in.resumeStage = stageMaterialization
			}
			tip := gateRevParse(t, f.root, "bead/"+closingBead)
			specTip := gateRevParse(t, f.root, "spec/"+closingSpec)
			err := runMaterialization(in)
			if !errors.Is(err, preflight) || !strings.Contains(err.Error(), "work-destruction preflight:") {
				t.Errorf("refusal lost preflight: %v", err)
			}
			if dirtCalls != 0 {
				t.Errorf("refusal called artifact normalization: %d", dirtCalls)
			}
			if path == "snapshot" {
				for _, gate := range []string{"doc-sync:", "adr-divergence:"} {
					if err == nil || !strings.Contains(err.Error(), gate) {
						t.Errorf("refusal lost %s: %v", gate, err)
					}
				}
			}
			if path == "snapshot-error" && (err == nil || !strings.Contains(err.Error(), "snapshot unavailable")) {
				t.Errorf("refusal lost snapshot error: %v", err)
			}
			assertContentRefusalState(t, f, tip, specTip)
		})
	}
}
