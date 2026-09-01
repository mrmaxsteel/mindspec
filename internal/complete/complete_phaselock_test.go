package complete

// complete_phaselock_test.go — round-1 panel G3: complete's best-effort
// step-6.5 mindspec_phase cache sync (completeSyncPhase, extracted from
// Run for direct unit testing) must SURFACE a WithPhaseWriteLock
// failure, never silently discard it (the pre-round-1 `_ =
// phase.WithPhaseWriteLock(...)` did). The isolated ADR-0035-shape unit
// proof for the rendering itself lives in
// internal/phase/phaselock_test.go's TestDescribeLockFailure; these
// tests exercise the real production wiring end to end.

import (
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// TestCompleteSyncPhase_LockTimeoutSurfacedNotDiscarded: a real held
// lock forces a genuine timeout (the wait is shrunk via
// phase.SetLockWaitForTest so the test stays fast), and the surfaced
// text carries the full ADR-0035 treatment.
func TestCompleteSyncPhase_LockTimeoutSurfacedNotDiscarded(t *testing.T) {
	t.Cleanup(phase.SetLockWaitForTest(150*time.Millisecond, 10*time.Millisecond))
	// Both WithPhaseWriteLock calls below (the holder's and
	// completeSyncPhase's) read the epic fresh under the lock — without
	// stubbing these seams they would shell out to a REAL bd binary
	// against a non-bd t.TempDir() (round-1 panel G2's class of bug).
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) { return []byte("[]"), nil }))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) { return []byte("[]"), nil }))
	root := t.TempDir()

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

	out := captureStderr(t, func() {
		completeSyncPhase(root, "epic-1", "plan")
	})

	if !strings.Contains(out, phase.PhaseLockRelPath) {
		t.Errorf("surfaced lock-timeout output must name the lock path: %q", out)
	}
	if !strings.Contains(out, "recovery: ") {
		t.Errorf("surfaced lock-timeout output must end with a recovery line: %q", out)
	}
	// The recovery line itself SAYS "never delete it" — a bare substring
	// check for "delete" would false-positive on that very safety
	// sentence, so assert the POSITIVE safety phrasing instead.
	if !strings.Contains(strings.ToLower(out), "never delete") {
		t.Errorf("must explicitly advise against deleting/removing the lock file: %q", out)
	}
}

// TestCompleteSyncPhase_WriteFailureSurfacedNoRecoveryLine: a genuine
// merge-write failure (completeMergeMetadataFn stubbed to fail — not a
// lock timeout) is surfaced as-is, with no manufactured ADR-0035
// recovery line.
func TestCompleteSyncPhase_WriteFailureSurfacedNoRecoveryLine(t *testing.T) {
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) { return []byte("[]"), nil }))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) { return []byte("[]"), nil }))
	origMerge := completeMergeMetadataFn
	completeMergeMetadataFn = func(string, map[string]interface{}) error {
		return errMockLockSync
	}
	t.Cleanup(func() { completeMergeMetadataFn = origMerge })
	root := t.TempDir()

	out := captureStderr(t, func() {
		completeSyncPhase(root, "epic-1", "plan")
	})
	if !strings.Contains(out, errMockLockSync.Error()) {
		t.Errorf("must surface the underlying write failure: %q", out)
	}
	if strings.Contains(out, "recovery: ") {
		t.Errorf("a plain (non-timeout) write failure must not get a manufactured recovery line: %q", out)
	}
}

// TestCompleteSyncPhase_SuccessPrintsNothing is the ordinary-path smoke
// test: a successful sync prints nothing to stderr.
func TestCompleteSyncPhase_SuccessPrintsNothing(t *testing.T) {
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) { return []byte("[]"), nil }))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) { return []byte("[]"), nil }))
	origMerge := completeMergeMetadataFn
	completeMergeMetadataFn = func(string, map[string]interface{}) error { return nil }
	t.Cleanup(func() { completeMergeMetadataFn = origMerge })
	root := t.TempDir()

	out := captureStderr(t, func() {
		completeSyncPhase(root, "epic-1", "plan")
	})
	if out != "" {
		t.Errorf("completeSyncPhase must print nothing to stderr on success, got %q", out)
	}
}

type mockLockSyncErr struct{}

func (*mockLockSyncErr) Error() string { return "bd metadata merge-write failed: connection refused" }

var errMockLockSync = &mockLockSyncErr{}
