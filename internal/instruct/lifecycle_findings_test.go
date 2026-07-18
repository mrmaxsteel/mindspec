package instruct

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// stubInstructScanIntegrity swaps the shared aggregate-scan seam for one
// test (final-review F1: the per-predicate glue seams collapsed into the
// one aggregate both doctor and instruct consume).
func stubInstructScanIntegrity(t *testing.T, fn func(root string, cache *phase.Cache) lifecycle.IntegrityFindings) {
	t.Helper()
	orig := instructScanIntegrityFindingsFn
	t.Cleanup(func() { instructScanIntegrityFindingsFn = orig })
	instructScanIntegrityFindingsFn = fn
}

// TestCollectLifecycleFindings_StaleOpenSurfaced proves the rendered line
// is EXACTLY StaleOpenBead.Message() — no re-derivation.
func TestCollectLifecycleFindings_StaleOpenSurfaced(t *testing.T) {
	root := t.TempDir()

	s := lifecycle.StaleOpenBead{BeadID: "one", SpecBranch: "spec/119-test", LandedSHA: "deadbeef"}
	stubInstructScanIntegrity(t, func(r string, c *phase.Cache) lifecycle.IntegrityFindings {
		return lifecycle.IntegrityFindings{StaleOpen: []lifecycle.StaleOpenBead{s}}
	})

	got := collectLifecycleFindings(root, phase.NewCache())
	if len(got) != 1 || got[0] != s.Message() {
		t.Fatalf("collectLifecycleFindings = %v, want [%q]", got, s.Message())
	}
}

// TestCollectLifecycleFindings_FinalizeBranchSurfaced proves the rendered
// line is EXACTLY FinalizeOrphan.FullMessage().
func TestCollectLifecycleFindings_FinalizeBranchSurfaced(t *testing.T) {
	root := t.TempDir()

	o := lifecycle.FinalizeOrphan{Kind: "finalize_branch", SpecID: "119-test", Branch: "chore/finalize-119-test", Message: "stranded"}
	stubInstructScanIntegrity(t, func(r string, c *phase.Cache) lifecycle.IntegrityFindings {
		return lifecycle.IntegrityFindings{FinalizeBranches: []lifecycle.FinalizeOrphan{o}}
	})

	got := collectLifecycleFindings(root, phase.NewCache())
	if len(got) != 1 || got[0] != o.FullMessage() {
		t.Fatalf("collectLifecycleFindings = %v, want [%q]", got, o.FullMessage())
	}
}

// TestCollectLifecycleFindings_StaleTrackerSurfaced proves the
// stale-tracker finding renders via FullMessage().
func TestCollectLifecycleFindings_StaleTrackerSurfaced(t *testing.T) {
	root := t.TempDir()

	o := lifecycle.FinalizeOrphan{Kind: "stale_tracker", SpecID: "119-test", Message: "epic closed but main disagrees"}
	stubInstructScanIntegrity(t, func(r string, c *phase.Cache) lifecycle.IntegrityFindings {
		return lifecycle.IntegrityFindings{StaleTrackers: []lifecycle.FinalizeOrphan{o}}
	})

	got := collectLifecycleFindings(root, phase.NewCache())
	if len(got) != 1 || got[0] != o.FullMessage() {
		t.Fatalf("collectLifecycleFindings = %v, want [%q]", got, o.FullMessage())
	}
}

// TestCollectLifecycleFindings_Clean: no findings on a healthy repo.
func TestCollectLifecycleFindings_Clean(t *testing.T) {
	root := t.TempDir()
	stubInstructScanIntegrity(t, func(r string, c *phase.Cache) lifecycle.IntegrityFindings {
		return lifecycle.IntegrityFindings{}
	})

	if got := collectLifecycleFindings(root, phase.NewCache()); len(got) != 0 {
		t.Fatalf("expected no findings on a healthy repo, got %v", got)
	}
}

// TestBuildContext_IdleModePopulatesLifecycleFindings proves the Context
// field is populated ONLY in idle mode, and that the collector receives
// the INVOCATION's cache — never a fresh one (final-review F1).
func TestBuildContext_IdleModePopulatesLifecycleFindings(t *testing.T) {
	root := t.TempDir()

	s := lifecycle.StaleOpenBead{BeadID: "one", SpecBranch: "spec/119-test", LandedSHA: "deadbeef"}
	var gotCache *phase.Cache
	stubInstructScanIntegrity(t, func(r string, c *phase.Cache) lifecycle.IntegrityFindings {
		gotCache = c
		return lifecycle.IntegrityFindings{StaleOpen: []lifecycle.StaleOpenBead{s}}
	})

	invocationCache := phase.NewCache()
	ctx := BuildContextWithCache(invocationCache, root, &state.Focus{Mode: state.ModeIdle})
	if len(ctx.LifecycleFindings) != 1 || ctx.LifecycleFindings[0] != s.Message() {
		t.Fatalf("idle mode LifecycleFindings = %v, want [%q]", ctx.LifecycleFindings, s.Message())
	}
	if gotCache != invocationCache {
		t.Error("collectLifecycleFindings must receive the EXISTING invocation cache, not a fresh phase.NewCache() (F1)")
	}
}

// TestBuildContext_NonIdleModeSkipsLifecycleFindings proves other modes
// never invoke the lifecycle scan (avoids paying the scan cost on every
// instruct call in active development modes).
func TestBuildContext_NonIdleModeSkipsLifecycleFindings(t *testing.T) {
	root := t.TempDir()

	called := false
	stubInstructScanIntegrity(t, func(r string, c *phase.Cache) lifecycle.IntegrityFindings {
		called = true
		return lifecycle.IntegrityFindings{}
	})

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

// TestLifecycleFindingSeams_InvokeExportedAggregate is the AC-12
// anti-drift pin's instruct half, updated for the final-review F1 shape:
// collectLifecycleFindings must invoke the SAME exported
// lifecycle.ScanIntegrityFindings aggregate internal/doctor's
// checkLifecycleIntegrity invokes, never a private reimplementation or a
// per-spec-dir fan-out.
func TestLifecycleFindingSeams_InvokeExportedAggregate(t *testing.T) {
	if reflect.ValueOf(instructScanIntegrityFindingsFn).Pointer() != reflect.ValueOf(lifecycle.ScanIntegrityFindings).Pointer() {
		t.Error("instructScanIntegrityFindingsFn must be lifecycle.ScanIntegrityFindings (AC-12 anti-drift)")
	}
}
