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
	"os/exec"
	"path/filepath"
	"strings"
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

// twoActiveSpecsListJSON stubs phase's ListJSON with TWO healthy, distinct
// active spec epics (spec_num/spec_title + a stored mindspec_phase, so
// DiscoverActiveSpecsWithCache resolves both from metadata alone — no
// GetChildren/bd-show fan-out needed). This is round-4 panel G1's exact
// reproduction shape: phase.ResolveContext's WorktreeMain arm has no case
// for `err == nil && len(specs) > 1` (derive.go's "leave specID empty,
// caller handles ambiguity" comment) — it falls through with ctx.Phase ""
// and DerivationFailed false.
func twoActiveSpecsListJSON(args ...string) ([]byte, error) {
	return []byte(`[
		{"id":"epic-aaa","status":"open","metadata":{"spec_num":901,"spec_title":"fixture one","mindspec_phase":"plan"}},
		{"id":"epic-bbb","status":"open","metadata":{"spec_num":902,"spec_title":"fixture two","mindspec_phase":"implement"}}
	]`), nil
}

// TestReadState_MultipleActiveSpecs_ReturnsNonNilIndeterminate is round-4
// panel G1/R3+R5 (hard block — the AMBIGUITY half of round-3's fix): a
// GENUINELY initialized, bd-healthy workspace with TWO active spec epics
// leaves ctx.Phase "" with ctx.DerivationFailed false (derive.go's
// WorktreeMain switch has no arm for this shape at all — "Multiple specs:
// leave specID empty, caller handles ambiguity"). Round-3's fix keyed the
// indeterminate sentinel ONLY on ctx.DerivationFailed, so this arm still
// fell through to the narrowed-but-not-narrow-enough nil check (empty Mode,
// no session) and returned nil — the SAME class of branch-protection bypass
// round-3 closed for the derivation-failure half, reopened here for the
// ambiguity half. No session file exists in this fixture, matching the
// exact conditions that made the old check fire.
func TestReadState_MultipleActiveSpecs_ReturnsNonNilIndeterminate(t *testing.T) {
	root := t.TempDir()
	chdirForReadState(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	t.Cleanup(phase.SetListJSONForTest(twoActiveSpecsListJSON))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))

	got := ReadState()
	if got == nil {
		t.Fatal("ReadState() on a .beads-present workspace with TWO healthy active specs = nil, want non-nil (a branch-protection bypass: runPreCommit would allow the commit)")
	}
	if got.Mode != state.ModeIndeterminate {
		t.Errorf("Mode = %q, want %q (the ambiguity half of the indeterminate contract — mirroring cmd/mindspec/instruct_tail.go's ctx.Phase == \"\" predicate)", got.Mode, state.ModeIndeterminate)
	}
}

// TestPreCommit_MultipleActiveSpecs_BlocksOnProtectedBranch is
// TestReadState_MultipleActiveSpecs_ReturnsNonNilIndeterminate's
// end-to-end companion: drives the REAL Run("pre-commit", ..., ReadState,
// true) on a real git repo with a protected branch, .beads/ present, and
// two healthy active specs — asserts Block, proving the hook DECISION
// built on the fixed value, not just the value itself (round-3's own
// precedent for pairing a direct ReadState pin with a driven Run pin).
func TestPreCommit_MultipleActiveSpecs_BlocksOnProtectedBranch(t *testing.T) {
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

	t.Cleanup(phase.SetListJSONForTest(twoActiveSpecsListJSON))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))

	r := Run("pre-commit", &Input{}, ReadState, true)
	if r.Action != Block {
		t.Errorf("expected Block for a protected-branch commit with two ambiguous active specs, got %v (message: %q)", r.Action, r.Message)
	}
}

// specBranchFixture builds a WorktreeSpec-shaped fixture: root is nested
// under .worktrees/worktree-spec-<specID> (the on-disk convention
// workspace.DetectWorktreeContext keys on — purely path-based, independent
// of the git branch), checked out on a REAL git branch named "spec/<specID>"
// (so runPreCommit's strings.HasPrefix(branch, "spec/") check fires), with
// .beads/ and .mindspec/config.yaml (no protected_branches entry, so the
// fixture reaches runPreCommit's spec/* logic rather than the
// protected-branch block).
func specBranchFixture(t *testing.T, specID string) (root string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, ".worktrees", "worktree-spec-"+specID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir spec worktree dir: %v", err)
	}
	mustGitInit(t, root)
	cmd := exec.Command("git", "checkout", "-b", "spec/"+specID)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b spec/%s: %v\n%s", specID, err, out)
	}
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("enforcement:\n  pre_commit_hook: true\n"), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	return root
}

// chdirTo chdirs the process into dir for the test's duration, restoring
// the original cwd via t.Cleanup.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// TestPreCommit_SpecBranch_IndeterminateFailsClosed is round-4 panel G2/R6
// (hard block): runPreCommit's spec/* branch logic checked ONLY for the
// literal Mode == "implement" — state.ModeIndeterminate matched NEITHER
// the "" / "idle" fast-path above it NOR the literal "implement" check,
// so it silently fell through to the final `return Result{Action: Pass}`,
// permitting a direct commit to a spec integration branch precisely while
// the hook cannot prove implementation mode has ended (the exact window a
// bd/Dolt discovery failure opens). Drives the REAL ReadState -> Run
// path on a WorktreeSpec fixture and a real spec/<id> git branch:
//  1. bd healthy, epic's stored mindspec_phase == implement -> Block
//     (the pre-existing behavior this fix must not disturb).
//  2. Same fixture, bd discovery now failing (AllEpics errors) -> ReadState
//     returns non-nil ModeIndeterminate -> must STILL Block, not Pass.
//
// Run as both a no-session and a claimed-bead-session sub-test so carried
// session metadata can never mask the sentinel.
func TestPreCommit_SpecBranch_IndeterminateFailsClosed(t *testing.T) {
	const specID = "999-fixture-spec"
	const epicID = "epic-999"

	healthyEpicJSON := []byte(`[{"id":"` + epicID + `","status":"open","metadata":{"spec_num":999,"spec_title":"fixture spec","mindspec_phase":"implement"}}]`)

	listJSON := func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--parent") {
			// One in_progress lifecycle child agrees with the stored
			// mindspec_phase (implement) — an empty children list would
			// child-derive "plan" instead, triggering DerivePhaseWithCache's
			// harmless-but-noisy stored-vs-derived stderr mismatch warning.
			return []byte(`[{"id":"task-1","status":"in_progress","issue_type":"task","parent":"` + epicID + `"}]`), nil
		}
		return healthyEpicJSON, nil
	}
	runBD := func(args ...string) ([]byte, error) {
		return healthyEpicJSON, nil // `bd show <epicID> --json`
	}
	failingListJSON := func(args ...string) ([]byte, error) {
		return nil, errors.New("bd: dolt daemon unreachable")
	}

	writeSession := func(t *testing.T, root string, sess string) {
		t.Helper()
		if sess == "" {
			return
		}
		if err := os.WriteFile(filepath.Join(root, ".mindspec", "session.json"), []byte(sess), 0o644); err != nil {
			t.Fatalf("write session.json: %v", err)
		}
	}

	claimedBeadSession := `{"sessionSource":"claude-code","sessionStartedAt":"2026-01-01T00:00:00Z","beadClaimedAt":"2026-01-01T00:05:00Z"}`

	for _, sessVariant := range []struct {
		name string
		json string
	}{
		{"no_session", ""},
		{"claimed_bead_session", claimedBeadSession},
	} {
		t.Run(sessVariant.name, func(t *testing.T) {
			t.Run("healthy_implement_blocks", func(t *testing.T) {
				root := specBranchFixture(t, specID)
				writeSession(t, root, sessVariant.json)
				chdirTo(t, root)
				t.Cleanup(phase.SetListJSONForTest(listJSON))
				t.Cleanup(phase.SetRunBDForTest(runBD))

				r := Run("pre-commit", &Input{}, ReadState, true)
				if r.Action != Block {
					t.Fatalf("healthy implement on spec/%s branch: got %v, want Block (message: %q)", specID, r.Action, r.Message)
				}
			})

			t.Run("discovery_failure_still_blocks", func(t *testing.T) {
				root := specBranchFixture(t, specID)
				writeSession(t, root, sessVariant.json)
				chdirTo(t, root)
				t.Cleanup(phase.SetListJSONForTest(failingListJSON))
				t.Cleanup(phase.SetRunBDForTest(runBD))

				st := ReadState()
				if st == nil {
					t.Fatal("ReadState() on a spec worktree with a genuine bd discovery failure = nil, want non-nil ModeIndeterminate")
				}
				if st.Mode != state.ModeIndeterminate {
					t.Errorf("Mode = %q, want %q", st.Mode, state.ModeIndeterminate)
				}

				r := Run("pre-commit", &Input{}, ReadState, true)
				if r.Action != Block {
					t.Fatalf("bd discovery failure on spec/%s branch: got %v, want Block (an indeterminate mode must fail closed, never Pass), message: %q", specID, r.Action, r.Message)
				}
			})
		})
	}
}
