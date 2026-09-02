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
	"errors"
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

// TestReadState_BeadsDirPresent_DerivationFailure_ReturnsNonNil is
// round-3 panel G1 (hard block — the MIRROR case of round-2's fix,
// reproduced and bisected against 3f35e4e3): once .beads/ exists, a
// genuine bd/Dolt derivation failure (phase.Context.DerivationFailed)
// must NEVER read as "no state" — the old unconditional
// `hs.Mode == "" && hs.SessionStartedAt == ""` check fired for this case
// too (WorktreeMain's error arm leaves ctx.Phase empty), letting
// runPreCommit treat a genuine outage as "not initialized, allow" — a
// protected-branch commit could slip through during exactly the failure
// this hook exists to guard against. No session file exists in this
// fixture either, so the OLD code's fallthrough condition was fully
// satisfied before this fix.
func TestReadState_BeadsDirPresent_DerivationFailure_ReturnsNonNil(t *testing.T) {
	root := t.TempDir()
	chdirForReadState(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	wantErr := errors.New("bd: dolt daemon unreachable")
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return nil, wantErr
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))

	got := ReadState()
	if got == nil {
		t.Fatal("ReadState() on a .beads-present workspace with a genuine derivation failure = nil, want non-nil (a branch-protection bypass: runPreCommit would allow the commit)")
	}
	if got.Mode != state.ModeIndeterminate {
		t.Errorf("Mode = %q, want %q (the honest indeterminate sentinel, never idle or empty)", got.Mode, state.ModeIndeterminate)
	}
}

// TestPreCommit_BeadsDirPresent_DerivationFailure_BlocksOnProtectedBranch
// drives the REAL Run("pre-commit", ..., ReadState, true) — production
// wiring end to end, not an injected staticState — on a real git repo,
// protected branch, with .beads/ present and bd genuinely failing:
// asserts Block. This is round-3 panel G1's second ask: the direct
// ReadState unit test above proves the return value; this proves the
// hook DECISION built on it.
func TestPreCommit_BeadsDirPresent_DerivationFailure_BlocksOnProtectedBranch(t *testing.T) {
	root := t.TempDir()
	mustGitInit(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	mindspecDir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(mindspecDir, 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(`
protected_branches: [main]
enforcement:
  pre_commit_hook: true
`), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir %s: %v", root, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	wantErr := errors.New("bd: dolt daemon unreachable")
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return nil, wantErr
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))

	r := Run("pre-commit", &Input{}, ReadState, true)
	if r.Action != Block {
		t.Errorf("expected Block for a protected-branch commit during a genuine bd outage on an initialized workspace, got %v (message: %q)", r.Action, r.Message)
	}
}
