package instruct

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

func makeInstructSpecDir(t *testing.T, root, specID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "specs", specID), 0o755); err != nil {
		t.Fatal(err)
	}
}

func stubLifecycleFindingSeams(t *testing.T,
	staleOpen func(specID, workdir string) ([]lifecycle.StaleOpenBead, error),
	finalizeBranches func(workdir string) ([]lifecycle.FinalizeOrphan, error),
	findEpic func(specID string) (string, error),
	epicStatus func(epicID string) (string, error),
	staleTracker func(workdir, specID, epicID string, liveClosed bool) (*lifecycle.FinalizeOrphan, error),
) {
	t.Helper()
	origStaleOpen := instructFindStaleOpenBeadsFn
	origFinalize := instructFindOutstandingFinalizeBranches
	origFindEpic := instructFindEpicBySpecIDFn
	origEpicStatus := instructFindEpicStatusFn
	origStaleTracker := instructStaleTrackerOnMainFn
	t.Cleanup(func() {
		instructFindStaleOpenBeadsFn = origStaleOpen
		instructFindOutstandingFinalizeBranches = origFinalize
		instructFindEpicBySpecIDFn = origFindEpic
		instructFindEpicStatusFn = origEpicStatus
		instructStaleTrackerOnMainFn = origStaleTracker
	})
	if staleOpen != nil {
		instructFindStaleOpenBeadsFn = staleOpen
	}
	if finalizeBranches != nil {
		instructFindOutstandingFinalizeBranches = finalizeBranches
	}
	if findEpic != nil {
		instructFindEpicBySpecIDFn = findEpic
	}
	if epicStatus != nil {
		instructFindEpicStatusFn = epicStatus
	}
	if staleTracker != nil {
		instructStaleTrackerOnMainFn = staleTracker
	}
}

// TestCollectLifecycleFindings_StaleOpenSurfaced proves the rendered line
// is EXACTLY StaleOpenBead.Message() — no re-derivation.
func TestCollectLifecycleFindings_StaleOpenSurfaced(t *testing.T) {
	root := t.TempDir()
	makeInstructSpecDir(t, root, "119-test")

	s := lifecycle.StaleOpenBead{BeadID: "one", SpecBranch: "spec/119-test", LandedSHA: "deadbeef"}
	stubLifecycleFindingSeams(t,
		func(specID, workdir string) ([]lifecycle.StaleOpenBead, error) {
			if specID == "119-test" {
				return []lifecycle.StaleOpenBead{s}, nil
			}
			return nil, nil
		},
		func(workdir string) ([]lifecycle.FinalizeOrphan, error) { return nil, nil },
		func(specID string) (string, error) { return "", nil },
		nil, nil,
	)

	got := collectLifecycleFindings(root)
	if len(got) != 1 || got[0] != s.Message() {
		t.Fatalf("collectLifecycleFindings = %v, want [%q]", got, s.Message())
	}
}

// TestCollectLifecycleFindings_FinalizeBranchSurfaced proves the rendered
// line is EXACTLY FinalizeOrphan.FullMessage().
func TestCollectLifecycleFindings_FinalizeBranchSurfaced(t *testing.T) {
	root := t.TempDir()

	o := lifecycle.FinalizeOrphan{Kind: "finalize_branch", SpecID: "119-test", Branch: "chore/finalize-119-test", Message: "stranded"}
	stubLifecycleFindingSeams(t,
		func(specID, workdir string) ([]lifecycle.StaleOpenBead, error) { return nil, nil },
		func(workdir string) ([]lifecycle.FinalizeOrphan, error) { return []lifecycle.FinalizeOrphan{o}, nil },
		func(specID string) (string, error) { return "", nil },
		nil, nil,
	)

	got := collectLifecycleFindings(root)
	if len(got) != 1 || got[0] != o.FullMessage() {
		t.Fatalf("collectLifecycleFindings = %v, want [%q]", got, o.FullMessage())
	}
}

// TestCollectLifecycleFindings_StaleTrackerSurfaced proves the per-spec
// stale-tracker finding renders via FullMessage() and that liveClosed is
// derived from the epic's live status.
func TestCollectLifecycleFindings_StaleTrackerSurfaced(t *testing.T) {
	root := t.TempDir()
	makeInstructSpecDir(t, root, "119-test")

	o := lifecycle.FinalizeOrphan{Kind: "stale_tracker", SpecID: "119-test", Message: "epic closed but main disagrees"}
	stubLifecycleFindingSeams(t,
		func(specID, workdir string) ([]lifecycle.StaleOpenBead, error) { return nil, nil },
		func(workdir string) ([]lifecycle.FinalizeOrphan, error) { return nil, nil },
		func(specID string) (string, error) { return "epic-1", nil },
		func(epicID string) (string, error) { return "closed", nil },
		func(workdir, specID, epicID string, liveClosed bool) (*lifecycle.FinalizeOrphan, error) {
			if !liveClosed {
				t.Fatalf("expected liveClosed=true")
			}
			return &o, nil
		},
	)

	got := collectLifecycleFindings(root)
	if len(got) != 1 || got[0] != o.FullMessage() {
		t.Fatalf("collectLifecycleFindings = %v, want [%q]", got, o.FullMessage())
	}
}

// TestCollectLifecycleFindings_Clean: no findings on a healthy repo.
func TestCollectLifecycleFindings_Clean(t *testing.T) {
	root := t.TempDir()
	makeInstructSpecDir(t, root, "119-test")

	stubLifecycleFindingSeams(t,
		func(specID, workdir string) ([]lifecycle.StaleOpenBead, error) { return nil, nil },
		func(workdir string) ([]lifecycle.FinalizeOrphan, error) { return nil, nil },
		func(specID string) (string, error) { return "epic-1", nil },
		func(epicID string) (string, error) { return "open", nil },
		func(workdir, specID, epicID string, liveClosed bool) (*lifecycle.FinalizeOrphan, error) {
			return nil, nil
		},
	)

	if got := collectLifecycleFindings(root); len(got) != 0 {
		t.Fatalf("expected no findings on a healthy repo, got %v", got)
	}
}

// TestBuildContext_IdleModePopulatesLifecycleFindings proves the Context
// field is populated ONLY in idle mode (the template it feeds,
// templates/idle.md, is idle-only).
func TestBuildContext_IdleModePopulatesLifecycleFindings(t *testing.T) {
	root := t.TempDir()
	makeInstructSpecDir(t, root, "119-test")

	s := lifecycle.StaleOpenBead{BeadID: "one", SpecBranch: "spec/119-test", LandedSHA: "deadbeef"}
	stubLifecycleFindingSeams(t,
		func(specID, workdir string) ([]lifecycle.StaleOpenBead, error) {
			return []lifecycle.StaleOpenBead{s}, nil
		},
		func(workdir string) ([]lifecycle.FinalizeOrphan, error) { return nil, nil },
		func(specID string) (string, error) { return "", nil },
		nil, nil,
	)

	ctx := BuildContext(root, &state.Focus{Mode: state.ModeIdle})
	if len(ctx.LifecycleFindings) != 1 || ctx.LifecycleFindings[0] != s.Message() {
		t.Fatalf("idle mode LifecycleFindings = %v, want [%q]", ctx.LifecycleFindings, s.Message())
	}
}

// TestBuildContext_NonIdleModeSkipsLifecycleFindings proves other modes
// never invoke the lifecycle scan (avoids paying per-spec bd/git scan cost
// on every instruct call in active development modes).
func TestBuildContext_NonIdleModeSkipsLifecycleFindings(t *testing.T) {
	root := t.TempDir()
	makeInstructSpecDir(t, root, "119-test")

	called := false
	stubLifecycleFindingSeams(t,
		func(specID, workdir string) ([]lifecycle.StaleOpenBead, error) {
			called = true
			return nil, nil
		},
		func(workdir string) ([]lifecycle.FinalizeOrphan, error) {
			called = true
			return nil, nil
		},
		func(specID string) (string, error) { return "", nil },
		nil, nil,
	)

	ctx := BuildContext(root, &state.Focus{Mode: state.ModeImplement, ActiveSpec: "119-test"})
	if called {
		t.Error("non-idle mode must not invoke the lifecycle-findings scan")
	}
	if ctx.LifecycleFindings != nil {
		t.Errorf("non-idle mode LifecycleFindings must stay nil, got %v", ctx.LifecycleFindings)
	}
}

// TestRender_IdleMode_RendersLifecycleFindings proves templates/idle.md
// renders the field verbatim, and that it is absent when empty (zero-cost
// contract, mirroring PanelState).
func TestRender_IdleMode_RendersLifecycleFindings(t *testing.T) {
	ctx := &Context{Mode: state.ModeIdle, LifecycleFindings: []string{"bead one is stale. Run `mindspec complete one`."}}
	out, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "bead one is stale. Run `mindspec complete one`.") {
		t.Errorf("expected the lifecycle finding line verbatim in output:\n%s", out)
	}
	if !strings.Contains(out, "Lifecycle Findings") {
		t.Errorf("expected a Lifecycle Findings section heading in output:\n%s", out)
	}

	empty := &Context{Mode: state.ModeIdle}
	out2, err := Render(empty)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out2, "Lifecycle Findings") {
		t.Errorf("empty LifecycleFindings must render nothing (zero-cost contract), got:\n%s", out2)
	}
}

// TestLifecycleFindingSeams_InvokeExportedPredicates is the AC-12
// anti-drift pin's instruct half: collectLifecycleFindings must invoke the
// SAME exported internal/lifecycle predicate symbols internal/doctor's
// checkStaleOpenBeads/checkFinalizeOrphans invoke, never a private
// reimplementation.
func TestLifecycleFindingSeams_InvokeExportedPredicates(t *testing.T) {
	if reflect.ValueOf(instructFindStaleOpenBeadsFn).Pointer() != reflect.ValueOf(lifecycle.FindStaleOpenBeads).Pointer() {
		t.Error("instructFindStaleOpenBeadsFn must be lifecycle.FindStaleOpenBeads (AC-12 anti-drift)")
	}
	if reflect.ValueOf(instructFindOutstandingFinalizeBranches).Pointer() != reflect.ValueOf(lifecycle.FindOutstandingFinalizeBranches).Pointer() {
		t.Error("instructFindOutstandingFinalizeBranches must be lifecycle.FindOutstandingFinalizeBranches (AC-12 anti-drift)")
	}
	if reflect.ValueOf(instructStaleTrackerOnMainFn).Pointer() != reflect.ValueOf(lifecycle.StaleTrackerOnMain).Pointer() {
		t.Error("instructStaleTrackerOnMainFn must be lifecycle.StaleTrackerOnMain (AC-12 anti-drift)")
	}
}
