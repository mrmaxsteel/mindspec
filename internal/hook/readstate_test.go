package hook

// readstate_test.go — spec 128 (lifecycle-verb-state-honesty) bead 5,
// round-2 panel G1 (hard block): ReadState's own "no state at all ->
// mindspec not initialized, allow" contract (runPreCommit's `st == nil`
// check) must still hold for a workspace with no .beads/ directory —
// bead 5's honest-idle fix elsewhere (phase's BeadsDirMissing-aware
// WorktreeMain arm) changed that case's derived Phase from "" to "idle",
// which would otherwise flip every bootstrap-time commit on a protected
// branch from allowed to blocked. No test in this package exercised the
// real ReadState() before this bead (dispatch_test.go's runPreCommit
// tests all inject a synthetic stateFn).
//
// These tests are NOT t.Parallel(): ReadState() resolves relative to the
// process's actual working directory (workspace.FindLocalRoot(".")), so
// each test chdirs the whole process for its duration.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// chdirForReadState creates root/.mindspec (so workspace.FindLocalRoot
// succeeds) and chdirs the process into root for the test's duration,
// restoring the original cwd via t.Cleanup.
func chdirForReadState(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir %s: %v", root, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
}

// TestReadState_BeadsDirMissing_ReturnsNil is round-2 panel G1's direct
// unit pin: a workspace with no .beads/ directory at all reads as "no
// state" (nil) — the SAME "mindspec not initialized, allow" contract
// runPreCommit's own `st == nil` check documents — restoring the
// historical behavior bead 5's honest-idle derive.go fix silently
// changed (Phase went from "" to "idle" for this exact case, and
// ReadState's OLD nil-check only fired on an empty Mode).
func TestReadState_BeadsDirMissing_ReturnsNil(t *testing.T) {
	root := t.TempDir()
	chdirForReadState(t, root)

	if got := ReadState(); got != nil {
		t.Errorf("ReadState() on a workspace with no .beads/ = %+v, want nil (mindspec not initialized, allow)", got)
	}
}

// TestReadState_BeadsDirPresent_HealthyIdle_ReturnsNonNilIdle is the
// other half of the same contract: a GENUINELY initialized workspace
// (`.beads/` present, bd healthy, zero active specs) must still read as
// a real, non-nil idle state — runPreCommit's own comment is explicit
// ("Block: commits on protected branches in ANY mode (including idle)")
// — so this fix must not also swallow a genuine idle back into nil.
func TestReadState_BeadsDirPresent_HealthyIdle_ReturnsNonNilIdle(t *testing.T) {
	root := t.TempDir()
	chdirForReadState(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))

	got := ReadState()
	if got == nil {
		t.Fatal("ReadState() on a genuinely-initialized, healthy-idle workspace = nil, want a non-nil idle state (runPreCommit must still block idle commits on protected branches)")
	}
	if got.Mode != state.ModeIdle {
		t.Errorf("Mode = %q, want %q", got.Mode, state.ModeIdle)
	}
}
