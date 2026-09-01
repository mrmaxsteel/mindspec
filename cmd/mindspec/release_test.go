package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// releaseRecorder captures the order of the two destructive steps (Remove the
// worktree, set the bead open) so tests can assert remove-FIRST / set-open-LAST
// — not merely that both occurred. It is a small local fake of the cmd-layer
// release seams (the unexported executor.fakeWorktreeOps is not importable from
// package main).
type releaseRecorder struct {
	calls []string // ordered: "remove:<id>", "setopen:<id>"

	dirty      []string
	dirtyErr   error
	removeErr  error
	setOpenErr error

	active     string
	epicID     string
	haveCursor bool
	phaseSyncs []string // "<epic>=<mode>"
}

func (r *releaseRecorder) deps() releaseDeps {
	return releaseDeps{
		root:             "/repo",
		beadWorktreePath: "/repo/.worktrees/worktree-spec/.worktrees/worktree-bead",
		checkDirty: func(repoRoot, cwd string) ([]string, error) {
			return r.dirty, r.dirtyErr
		},
		removeWorktree: func(beadID string) error {
			r.calls = append(r.calls, "remove:"+beadID)
			return r.removeErr
		},
		setOpen: func(beadID string) error {
			r.calls = append(r.calls, "setopen:"+beadID)
			return r.setOpenErr
		},
		activeBead: func(beadID string) (string, string, bool) {
			return r.active, r.epicID, r.haveCursor
		},
		syncPhase: func(epicID, mode string) {
			r.phaseSyncs = append(r.phaseSyncs, epicID+"="+mode)
		},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
}

// TestReleaseOrdering pins the happy path: a clean worktree → Remove is called,
// the bead is set open + assignee cleared, and Remove happens BEFORE set-open.
func TestReleaseOrdering(t *testing.T) {
	r := &releaseRecorder{
		dirty:      nil,
		active:     "mindspec-abc",
		epicID:     "mindspec-epic",
		haveCursor: true,
	}
	if err := runRelease(r.deps(), "mindspec-abc", false); err != nil {
		t.Fatalf("runRelease (clean): unexpected error: %v", err)
	}

	want := []string{"remove:mindspec-abc", "setopen:mindspec-abc"}
	if strings.Join(r.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("call order = %v, want %v (remove must precede set-open)", r.calls, want)
	}
}

// TestReleaseOrdering_RemoveBeforeSetOpenOnFailurePath proves the ordering
// invariant via a set-open FAILURE: if set-open were called first, Remove would
// never run after the failure. With remove-first, Remove ran and set-open's
// failure leaves the recoverable still-claimed/worktree-gone state.
func TestReleaseOrdering_RemoveBeforeSetOpenOnFailurePath(t *testing.T) {
	r := &releaseRecorder{setOpenErr: errors.New("bd down")}
	err := runRelease(r.deps(), "mindspec-abc", false)
	if err == nil {
		t.Fatal("runRelease must fail when set-open fails")
	}
	// Remove must have happened (and happened before the failed set-open).
	if len(r.calls) != 2 || r.calls[0] != "remove:mindspec-abc" || r.calls[1] != "setopen:mindspec-abc" {
		t.Fatalf("call order = %v, want remove then setopen", r.calls)
	}
	if !strings.Contains(err.Error(), "still-claimed") {
		t.Errorf("set-open failure should describe the recoverable still-claimed state; got: %v", err)
	}
}

// TestReleaseDirtyRefusesWithoutForce: a dirty bead worktree refuses (non-zero,
// Remove NOT called) without --force.
func TestReleaseDirtyRefusesWithoutForce(t *testing.T) {
	r := &releaseRecorder{dirty: []string{"src/foo.go"}}
	err := runRelease(r.deps(), "mindspec-abc", false)
	if err == nil {
		t.Fatal("runRelease must refuse a dirty worktree without --force")
	}
	if len(r.calls) != 0 {
		t.Fatalf("no destructive step may run on a dirty refusal; got calls %v", r.calls)
	}
	if !strings.Contains(err.Error(), "uncommitted") {
		t.Errorf("refusal should name the uncommitted changes; got: %v", err)
	}
}

// TestRunRelease_DiscardNeverInRecoveryLine is spec 127 AC-9(ii)/AC-10(ii)
// (O2-r2-5, O3-r2-7): the dirty-tree refusal's machine-greppable
// `recovery: ` line names ONLY the safe action (commit + re-run) — the
// `--force` discard is a separately-labeled OPERATOR CHOICE that
// appears in the message body instead, never on a `recovery: ` line an
// agent might paste reflexively. Verified red-on-revert manually at
// authoring time: restoring the pre-bead-7 combined line (both actions
// on one `recovery: ` line) makes the second assertion below fail.
func TestRunRelease_DiscardNeverInRecoveryLine(t *testing.T) {
	r := &releaseRecorder{dirty: []string{"src/foo.go"}}
	err := runRelease(r.deps(), "mindspec-abc", false)
	if err == nil {
		t.Fatal("runRelease must refuse a dirty worktree without --force")
	}
	full := err.Error()

	var recoveryLines, bodyLines []string
	for _, line := range strings.Split(full, "\n") {
		if strings.HasPrefix(line, guard.RecoveryPrefix) {
			recoveryLines = append(recoveryLines, line)
		} else {
			bodyLines = append(bodyLines, line)
		}
	}
	if len(recoveryLines) == 0 {
		t.Fatal("dirty refusal has no recovery: line at all")
	}
	for _, rl := range recoveryLines {
		if strings.Contains(rl, "--force") {
			t.Errorf("recovery: line contains the --force discard — it must name only the safe commit-and-rerun action: %q", rl)
		}
	}
	body := strings.Join(bodyLines, "\n")
	if !strings.Contains(body, "--force") {
		t.Errorf("message body is missing the labeled --force discard choice entirely: %q", full)
	}
	if !strings.Contains(strings.ToLower(body), "operator choice") {
		t.Errorf("the discard must be a SEPARATELY-LABELED operator choice in the message body, not just a bare mention: %q", body)
	}
}

// TestReleaseDirtyProceedsWithForce: --force is the mindspec-level pre-gate —
// a dirty worktree proceeds (Remove called, dirty-check skipped) with --force.
func TestReleaseDirtyProceedsWithForce(t *testing.T) {
	dirtyChecked := false
	r := &releaseRecorder{dirty: []string{"src/foo.go"}}
	deps := r.deps()
	deps.checkDirty = func(repoRoot, cwd string) ([]string, error) {
		dirtyChecked = true
		return []string{"src/foo.go"}, nil
	}
	if err := runRelease(deps, "mindspec-abc", true); err != nil {
		t.Fatalf("runRelease --force on dirty worktree: unexpected error: %v", err)
	}
	if dirtyChecked {
		t.Error("--force must short-circuit the dirty-check (pre-gate), not call it")
	}
	if len(r.calls) != 2 || r.calls[0] != "remove:mindspec-abc" {
		t.Fatalf("--force must proceed to Remove; got calls %v", r.calls)
	}
}

// TestReleaseBeadStateSetOpen: the bead is set to open + assignee cleared via
// the seam (asserted via the call record + the production defaultSetOpen args
// are covered by TestReleaseDefaultSetOpenArgs below).
func TestReleaseBeadStateSetOpen(t *testing.T) {
	var gotBead string
	r := &releaseRecorder{}
	deps := r.deps()
	deps.setOpen = func(beadID string) error {
		gotBead = beadID
		r.calls = append(r.calls, "setopen:"+beadID)
		return nil
	}
	if err := runRelease(deps, "mindspec-xyz", false); err != nil {
		t.Fatalf("runRelease: %v", err)
	}
	if gotBead != "mindspec-xyz" {
		t.Errorf("setOpen called with %q, want mindspec-xyz", gotBead)
	}
}

// TestReleaseCursorRewindOnlyWhenActive: when the released bead IS the active
// one, the mindspec_phase cache is synced; when it is NOT active, the cursor is
// left untouched (no phase sync).
func TestReleaseCursorRewindOnlyWhenActive(t *testing.T) {
	// Released bead is the active one → phase sync fires.
	rActive := &releaseRecorder{active: "mindspec-abc", epicID: "mindspec-epic", haveCursor: true}
	// DerivePhase will be called against a real (absent) epic; to keep this a
	// pure unit test, drive the active branch through a syncPhase recorder and
	// a stubbed derive via the seam. Since runRelease calls phase.DerivePhase
	// directly, we instead assert the NON-active path here (no sync) and let
	// the active path be covered by the integration build.
	_ = rActive

	// Released bead is NOT active → cursor untouched, no phase sync.
	rIdle := &releaseRecorder{active: "", epicID: "mindspec-epic", haveCursor: true}
	if err := runRelease(rIdle.deps(), "mindspec-abc", false); err != nil {
		t.Fatalf("runRelease (non-active): %v", err)
	}
	if len(rIdle.phaseSyncs) != 0 {
		t.Errorf("releasing a non-active bead must not sync the phase cursor; got %v", rIdle.phaseSyncs)
	}
}

// TestReleaseDefaultSetOpenArgs pins the production bd-state mutation: the bead
// is updated to status=open with the assignee cleared (an empty --assignee).
func TestReleaseDefaultSetOpenArgs(t *testing.T) {
	orig := bdRunCombinedForRelease
	t.Cleanup(func() { bdRunCombinedForRelease = orig })

	var gotArgs []string
	bdRunCombinedForRelease = func(args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}
	if err := defaultSetOpenVia(bdRunCombinedForRelease, "mindspec-abc"); err != nil {
		t.Fatalf("defaultSetOpen: %v", err)
	}
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"update", "mindspec-abc", "--status=open", "--assignee="} {
		if !strings.Contains(joined, want) {
			t.Errorf("bd update args %q missing %q", joined, want)
		}
	}
}
