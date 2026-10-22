package main

// next_phase_advance_test.go — spec 128 (lifecycle-verb-state-honesty)
// Bead 2, plan step 2: unit tests for advancePhaseOnClaim (next.go's
// Step 5.2), extracted to its own named function for exactly this
// purpose (matching claimFailureError/warnWorktreeSetupFailure's
// established pattern). Round-1 panel G1 fix-round additions: the
// fresh-evidence-inside-the-lock guarantee, real-lock interleave proofs
// against release/complete, and convergent-recovery lock-timeout/
// merge-failure tests over the real advancePhaseOnClaim path.

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// wireAdvancePhaseFixture stubs phase's bd seams to resolve beadID's
// epic lineage (a `bd show <beadID>` dependency-parent lookup) to
// epicID, and epicID's own metadata to epicMeta. children defaults the
// claimed bead itself as the sole in_progress lifecycle child — the
// REAL state `next` leaves behind by the time Step 5.2 runs (ClaimBead,
// Step 5, already made it in_progress in bd) — so the fresh-derive gate
// advancePhaseOnClaim now applies sees evidence that actually supports
// advancing to implement. Use wireAdvancePhaseFixtureWithChildren
// directly for scenarios needing a different child set.
func wireAdvancePhaseFixture(t *testing.T, beadID, epicID string, epicMeta map[string]interface{}) {
	t.Helper()
	wireAdvancePhaseFixtureWithChildren(t, beadID, epicID, epicMeta, []phase.ChildInfo{
		{ID: beadID, Status: "in_progress", IssueType: "task"},
	})
}

func wireAdvancePhaseFixtureWithChildren(t *testing.T, beadID, epicID string, epicMeta map[string]interface{}, children []phase.ChildInfo) {
	t.Helper()
	beadShowJSON, _ := json.Marshal([]map[string]interface{}{{
		"title": "[010-test] a bead",
		"dependencies": []map[string]interface{}{
			{"id": epicID, "issue_type": "epic", "dependency_type": "parent-child"},
		},
	}})
	epicJSON, _ := json.Marshal([]phase.EpicInfo{{
		ID: epicID, Title: "[SPEC 010-test] Test", Status: "open", IssueType: "epic", Metadata: epicMeta,
	}})
	childrenJSON, _ := json.Marshal(children)
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == beadID {
			return beadShowJSON, nil
		}
		if len(args) >= 2 && args[0] == "show" && args[1] == epicID {
			return epicJSON, nil
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return epicJSON, nil
			}
			if strings.HasPrefix(a, "--parent") {
				return childrenJSON, nil
			}
		}
		return []byte("[]"), nil
	}))
}

// TestAdvancePhaseOnClaim_LifecycleClaimAdvancesToImplement: a
// task-typed (LIFECYCLE) claim on an epic stored at `plan` advances
// stored mindspec_phase to `implement` — the claimed bead is genuinely
// in_progress, so the fresh-derive gate agrees.
func TestAdvancePhaseOnClaim_LifecycleClaimAdvancesToImplement(t *testing.T) {
	root := t.TempDir()
	wireAdvancePhaseFixture(t, "mindspec-bead1", "epic-1", map[string]interface{}{
		"mindspec_phase":           state.ModePlan,
		"mindspec_phase_semantics": "v2",
	})
	// EnsurePhaseSemanticsV2's own self-heal write (marker already
	// present here, so it no-ops — but stub it anyway so a change to
	// that no-op contract fails loudly here instead of hitting real bd).
	t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
	var gotUpdates map[string]interface{}
	origMerge := nextPhaseMergeMetadataFn
	nextPhaseMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		gotUpdates = updates
		return nil
	}
	t.Cleanup(func() { nextPhaseMergeMetadataFn = origMerge })

	advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: "task"})

	if gotUpdates["mindspec_phase"] != state.ModeImplement {
		t.Errorf("mindspec_phase = %v, want %q", gotUpdates["mindspec_phase"], state.ModeImplement)
	}
}

// TestAdvancePhaseOnClaim_UntypedClaimAdvancesToImplement: an untyped
// (empty issue_type) bead is ALSO lifecycle per the ADR-0023
// discriminator.
func TestAdvancePhaseOnClaim_UntypedClaimAdvancesToImplement(t *testing.T) {
	root := t.TempDir()
	wireAdvancePhaseFixture(t, "mindspec-bead1", "epic-1", map[string]interface{}{
		"mindspec_phase":           state.ModePlan,
		"mindspec_phase_semantics": "v2",
	})
	t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
	var gotUpdates map[string]interface{}
	origMerge := nextPhaseMergeMetadataFn
	nextPhaseMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		gotUpdates = updates
		return nil
	}
	t.Cleanup(func() { nextPhaseMergeMetadataFn = origMerge })

	advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: ""})

	if gotUpdates["mindspec_phase"] != state.ModeImplement {
		t.Errorf("mindspec_phase = %v, want %q for an untyped (lifecycle-default) claim", gotUpdates["mindspec_phase"], state.ModeImplement)
	}
}

// TestAdvancePhaseOnClaim_NonLifecycleClaimAdvancesNothing: a `bug`-typed
// claim must never advance stored mindspec_phase.
func TestAdvancePhaseOnClaim_NonLifecycleClaimAdvancesNothing(t *testing.T) {
	root := t.TempDir()
	wireAdvancePhaseFixture(t, "mindspec-bead1", "epic-1", map[string]interface{}{
		"mindspec_phase":           state.ModePlan,
		"mindspec_phase_semantics": "v2",
	})
	t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
	calls := 0
	origMerge := nextPhaseMergeMetadataFn
	nextPhaseMergeMetadataFn = func(string, map[string]interface{}) error {
		calls++
		return nil
	}
	t.Cleanup(func() { nextPhaseMergeMetadataFn = origMerge })

	advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: "bug"})

	if calls != 0 {
		t.Errorf("a non-lifecycle claim must perform ZERO phase writes, got %d", calls)
	}
}

// TestAdvancePhaseOnClaim_AlreadyImplementIsNoOp: forward-only — no write
// when stored is already `implement`.
func TestAdvancePhaseOnClaim_AlreadyImplementIsNoOp(t *testing.T) {
	root := t.TempDir()
	wireAdvancePhaseFixture(t, "mindspec-bead1", "epic-1", map[string]interface{}{
		"mindspec_phase":           state.ModeImplement,
		"mindspec_phase_semantics": "v2",
	})
	t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
	calls := 0
	origMerge := nextPhaseMergeMetadataFn
	nextPhaseMergeMetadataFn = func(string, map[string]interface{}) error {
		calls++
		return nil
	}
	t.Cleanup(func() { nextPhaseMergeMetadataFn = origMerge })

	advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: "task"})

	if calls != 0 {
		t.Errorf("an already-implement epic must be a ZERO-write no-op, got %d calls", calls)
	}
}

// TestAdvancePhaseOnClaim_NoEpicLineageIsSilentNoOp: a bead with no
// discoverable epic lineage (phase.ErrNoEpicLineage) must never warn or
// write — this is the ordinary "standalone task, not part of a spec"
// case, not an infra failure.
func TestAdvancePhaseOnClaim_NoEpicLineageIsSilentNoOp(t *testing.T) {
	root := t.TempDir()
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil // `bd show` returns no items -> ErrNoEpicLineage
	}))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
	calls := 0
	t.Cleanup(phase.SetMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
		calls++
		return nil
	}))

	stderr := captureStderr(t, func() {
		advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-standalone", IssueType: "task"})
	})

	if calls != 0 {
		t.Errorf("no epic lineage must perform ZERO phase writes, got %d", calls)
	}
	if strings.Contains(stderr, "warning:") {
		t.Errorf("a genuinely epic-less bead must not warn (that is the ordinary case, not an infra failure): %q", stderr)
	}
}

// TestAdvancePhaseOnClaim_StaleEvidenceNeverOverwritesFresherPlan is
// round-1 panel G1's hard-block core: even WITHOUT any lock contention,
// if the fresh child-derive at write time no longer says "implement"
// (some other writer already changed the children), the write must
// no-op — proven directly (single-threaded) before the real-lock
// interleave tests below prove it end-to-end.
func TestAdvancePhaseOnClaim_StaleEvidenceNeverOverwritesFresherPlan(t *testing.T) {
	root := t.TempDir()
	// The bead itself is back to OPEN (as if release already ran) by the
	// time the delayed write's fn executes — fresh derivation says plan,
	// not implement.
	wireAdvancePhaseFixtureWithChildren(t, "mindspec-bead1", "epic-1", map[string]interface{}{
		"mindspec_phase":           state.ModeImplement,
		"mindspec_phase_semantics": "v2",
	}, []phase.ChildInfo{{ID: "mindspec-bead1", Status: "open", IssueType: "task"}})
	t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
	calls := 0
	origMerge := nextPhaseMergeMetadataFn
	nextPhaseMergeMetadataFn = func(string, map[string]interface{}) error {
		calls++
		return nil
	}
	t.Cleanup(func() { nextPhaseMergeMetadataFn = origMerge })

	advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: "task"})

	if calls != 0 {
		t.Errorf("stale evidence (fresh derive != implement) must be a ZERO-write no-op, got %d calls", calls)
	}
}

// mutableInterleaveFixture is a small, mutex-protected fake bd tracker
// used by the two real-lock interleave tests below: it lets the test
// simulate a concurrent writer (release/complete) mutating children and
// stored phase BETWEEN the moment a claim happens and the moment the
// delayed advancePhaseOnClaim's fn actually runs inside the real R3.3
// lock.
type mutableInterleaveFixture struct {
	mu       sync.Mutex
	beadID   string
	epicID   string
	stored   string
	children []phase.ChildInfo
}

func (f *mutableInterleaveFixture) epicJSON() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, _ := json.Marshal([]phase.EpicInfo{{
		ID: f.epicID, Title: "[SPEC 010-test] Test", Status: "open", IssueType: "epic",
		Metadata: map[string]interface{}{
			"mindspec_phase":           f.stored,
			"mindspec_phase_semantics": "v2",
			"spec_num":                 float64(10),
			"spec_title":               "test",
		},
	}})
	return b
}

func (f *mutableInterleaveFixture) childrenJSON() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, _ := json.Marshal(f.children)
	return b
}

func (f *mutableInterleaveFixture) setStored(v string) {
	f.mu.Lock()
	f.stored = v
	f.mu.Unlock()
}

func (f *mutableInterleaveFixture) setChildren(c []phase.ChildInfo) {
	f.mu.Lock()
	f.children = c
	f.mu.Unlock()
}

func (f *mutableInterleaveFixture) getStored() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stored
}

func (f *mutableInterleaveFixture) stub(t *testing.T) {
	t.Helper()
	beadShowJSON, _ := json.Marshal([]map[string]interface{}{{
		"title":        "[010-test] a bead",
		"dependencies": []map[string]interface{}{{"id": f.epicID, "issue_type": "epic", "dependency_type": "parent-child"}},
	}})
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == f.beadID {
			return beadShowJSON, nil
		}
		if len(args) >= 2 && args[0] == "show" && args[1] == f.epicID {
			return f.epicJSON(), nil
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return f.epicJSON(), nil
			}
			if strings.HasPrefix(a, "--parent") {
				return f.childrenJSON(), nil
			}
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
		if v, ok := updates["mindspec_phase"].(string); ok {
			f.setStored(v)
		}
		return nil
	}))
}

// TestAdvancePhaseOnClaim_RealLockInterleave_DelayedVsRelease is round-1
// panel G1.3: a REAL release completes FIRST — via the actual
// phase.SanctionedDerivedSync surface, acquiring and releasing the REAL
// R3.3 lock — returning the sole claimed bead to open and writing the
// fresh derived phase (`plan`). The delayed advancePhaseOnClaim then
// runs, ALSO through the real lock: its fresh in-fn children read must
// observe release's already-landed mutation and refuse to regress
// stored back to `implement`.
func TestAdvancePhaseOnClaim_RealLockInterleave_DelayedVsRelease(t *testing.T) {
	root := t.TempDir()
	f := &mutableInterleaveFixture{
		beadID: "mindspec-bead1", epicID: "epic-1",
		stored:   state.ModeImplement,
		children: []phase.ChildInfo{{ID: "mindspec-bead1", Status: "in_progress", IssueType: "task"}},
	}
	f.stub(t)

	// Release completes FIRST: the bead goes back to open, then the
	// REAL SanctionedDerivedSync call (real lock acquire+release) writes
	// the freshly-derived phase.
	f.setChildren([]phase.ChildInfo{{ID: "mindspec-bead1", Status: "open", IssueType: "task"}})
	if err := phase.SanctionedDerivedSync(root, "epic-1"); err != nil {
		t.Fatalf("SanctionedDerivedSync: %v", err)
	}
	if got := f.getStored(); got != state.ModePlan {
		t.Fatalf("fixture sanity: release must have written plan, got %q", got)
	}

	// The DELAYED advance now runs, through the real lock.
	advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: "task"})

	if got := f.getStored(); got != state.ModePlan {
		t.Errorf("delayed advance must NEVER regress release's fresher plan back to implement; stored = %q", got)
	}
}

// TestAdvancePhaseOnClaim_RealLockInterleave_DelayedVsComplete is the
// complete-side twin: complete closes the sole lifecycle bead FIRST —
// via a real phase.WithPhaseWriteLock write (mirroring completeSyncPhase's
// own shape: derive fresh, write unconditionally — complete's OWN value
// semantics are untouched by this bead, plan step 4) — landing `review`.
// The delayed advancePhaseOnClaim must never regress that back to
// `implement`.
func TestAdvancePhaseOnClaim_RealLockInterleave_DelayedVsComplete(t *testing.T) {
	root := t.TempDir()
	f := &mutableInterleaveFixture{
		beadID: "mindspec-bead1", epicID: "epic-1",
		stored:   state.ModeImplement,
		children: []phase.ChildInfo{{ID: "mindspec-bead1", Status: "in_progress", IssueType: "task"}},
	}
	f.stub(t)

	// complete closes the bead FIRST, then performs its own real,
	// lock-guarded, freshly-derived sync write (mirroring
	// completeSyncPhase's shape).
	f.setChildren([]phase.ChildInfo{{ID: "mindspec-bead1", Status: "closed", IssueType: "task"}})
	if err := phase.WithPhaseWriteLock(root, "epic-1", func(id string, updates map[string]interface{}) error {
		if v, ok := updates["mindspec_phase"].(string); ok {
			f.setStored(v)
		}
		return nil
	}, func(string, bool) (map[string]interface{}, error) {
		freshCache := phase.NewCache()
		children, err := freshCache.GetChildren("epic-1")
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"mindspec_phase": phase.DerivePhaseFromChildren(children)}, nil
	}); err != nil {
		t.Fatalf("simulated complete sync: %v", err)
	}
	if got := f.getStored(); got != state.ModeReview {
		t.Fatalf("fixture sanity: complete must have written review, got %q", got)
	}

	// The DELAYED advance now runs, through the real lock.
	advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: "task"})

	if got := f.getStored(); got != state.ModeReview {
		t.Errorf("delayed advance must NEVER regress complete's fresher review back to implement; stored = %q", got)
	}
}

// TestAdvancePhaseOnClaim_LockTimeoutSurfacesConvergentRecovery is
// round-1 panel G1.4/G5: a genuine lock-acquisition timeout over the
// REAL advancePhaseOnClaim path must surface a recovery command that
// ACTUALLY converges (`mindspec repair phase <spec-id>`), never the
// dead-end `mindspec next <bead-id>` (the bead is already in_progress —
// a re-run can never re-select it).
func TestAdvancePhaseOnClaim_LockTimeoutSurfacesConvergentRecovery(t *testing.T) {
	t.Cleanup(phase.SetLockWaitForTest(150*time.Millisecond, 10*time.Millisecond))
	root := t.TempDir()
	wireAdvancePhaseFixture(t, "mindspec-bead1", "epic-1", map[string]interface{}{
		"mindspec_phase":           state.ModePlan,
		"mindspec_phase_semantics": "v2",
		"spec_num":                 float64(10),
		"spec_title":               "test",
	})

	holderEntered := make(chan struct{})
	releaseHolder := make(chan struct{})
	go func() {
		_ = phase.WithPhaseWriteLock(root, "epic-1", func(string, map[string]interface{}) error { return nil }, func(string, bool) (map[string]interface{}, error) {
			close(holderEntered)
			<-releaseHolder
			return nil, nil
		})
	}()
	<-holderEntered
	defer close(releaseHolder)

	stderr := captureStderr(t, func() {
		advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: "task"})
	})

	if !strings.Contains(stderr, "recovery: mindspec repair phase 010-test") {
		t.Errorf("lock-timeout warning must name the convergent repair-phase recovery: %q", stderr)
	}
	if strings.Contains(stderr, "recovery: mindspec next") {
		t.Errorf("lock-timeout warning must NEVER emit the dead-end `mindspec next` recovery: %q", stderr)
	}
}

// TestAdvancePhaseOnClaim_MergeFailureSurfacesConvergentRecovery is
// round-1 panel G1.4/G5's non-timeout leg: a genuine merge-write
// failure over the real path must ALSO surface the convergent
// repair-phase recovery, never the dead-end next recovery.
func TestAdvancePhaseOnClaim_MergeFailureSurfacesConvergentRecovery(t *testing.T) {
	root := t.TempDir()
	wireAdvancePhaseFixture(t, "mindspec-bead1", "epic-1", map[string]interface{}{
		"mindspec_phase":           state.ModePlan,
		"mindspec_phase_semantics": "v2",
		"spec_num":                 float64(10),
		"spec_title":               "test",
	})
	t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
	origMerge := nextPhaseMergeMetadataFn
	nextPhaseMergeMetadataFn = func(string, map[string]interface{}) error {
		return errors.New("dolt offline")
	}
	t.Cleanup(func() { nextPhaseMergeMetadataFn = origMerge })

	stderr := captureStderr(t, func() {
		advancePhaseOnClaim(root, next.BeadInfo{ID: "mindspec-bead1", IssueType: "task"})
	})

	if !strings.Contains(stderr, "recovery: mindspec repair phase 010-test") {
		t.Errorf("merge-failure warning must name the convergent repair-phase recovery: %q", stderr)
	}
	if strings.Contains(stderr, "recovery: mindspec next") {
		t.Errorf("merge-failure warning must NEVER emit the dead-end `mindspec next` recovery: %q", stderr)
	}
}
