package main

// state_show_honesty_test.go: AC-1's three fixture legs (spec 128 Bead 3,
// OQ-2/R1.1/R1.2/R1.4) — `mindspec state show` never reports "idle"
// alongside a non-empty activeSpec, never exits 0 with an empty mode on a
// discovery failure, and renders the honest OQ-2 draft representation for
// a main-checkout draft-only state. Each test is red-on-revert against one
// of the three 2026-09-01 captures quoted in spec.md's settled fact 6 /
// live-evidence section.
//
// Hermetic: every fixture root lives under t.TempDir(); bd is stubbed via
// the phase package's seam (never a real bd subprocess). The live-worktree
// probe ("does THIS repo's own state show say spec, not idle") is the
// orchestrator's verification step, not a test.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// newDraftFixtureRoot builds a hermetic (t.TempDir()-rooted) git repo whose
// main-checkout-shaped root carries an on-disk pre-epic draft: a
// .mindspec/specs/<specID>/spec.md directory plus a committed spec/<specID>
// branch — the OQ-2 draft shape — and nothing else.
func newDraftFixtureRoot(t *testing.T, specID string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitInDir(t, root, "init", "-b", "main")
	specDir := filepath.Join(root, ".mindspec", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# draft\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	runGitInDir(t, root, "add", "-A")
	runGitInDir(t, root, "commit", "-m", "draft spec")
	runGitInDir(t, root, "branch", "spec/"+specID)
	return root
}

// newDraftSpecWorktreeFixture is newDraftFixtureRoot's spec-worktree-shaped
// counterpart: the git repo root itself lives at
// .worktrees/worktree-spec-<specID>/ so workspace.DetectWorktreeContext
// recognizes it as a spec worktree (AC-1(i) — the "from a fresh no-epic
// draft spec's worktree" fixture). DetectWorktreeContext matches on the
// PATH STRING alone, so a hand-built repo at that path (rather than a real
// `git worktree add` linked checkout) is sufficient and stays hermetic.
func newDraftSpecWorktreeFixture(t *testing.T, specID string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	base := t.TempDir()
	root := filepath.Join(base, ".worktrees", "worktree-spec-"+specID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir spec worktree root: %v", err)
	}
	runGitInDir(t, root, "init", "-b", "main")
	specDir := filepath.Join(root, ".mindspec", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# draft\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	runGitInDir(t, root, "add", "-A")
	runGitInDir(t, root, "commit", "-m", "draft spec")
	runGitInDir(t, root, "branch", "spec/"+specID)
	return root
}

// resetStateShowFlags clears the --spec flag between invocations so a
// prior test's value never leaks into a later one.
func resetStateShowFlags(t *testing.T) {
	t.Helper()
	if f := stateShowCmd.Flags().Lookup("spec"); f != nil {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	}
}

// stubNoEpics stubs a healthy bd reporting zero epics.
func stubNoEpics(t *testing.T) {
	t.Helper()
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
}

// stubBDUnreachable stubs bd shelling out and failing every call — the
// AC-1(ii) fixture's "discovery failing" injection, via the phase package's
// seam rather than real PATH manipulation (that live reproduction is the
// orchestrator's job — spec.md settled fact 6 / AC-1(ii)).
func stubBDUnreachable(t *testing.T) error {
	t.Helper()
	wantErr := &exec.Error{Name: "bd", Err: exec.ErrNotFound}
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return nil, wantErr
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return nil, wantErr
	}))
	return wantErr
}

// TestStateShow_DraftSpecWorktree_HonestSpecMode is AC-1(i): from a fresh
// no-epic draft spec's worktree, state show never reports idle alongside a
// non-empty activeSpec — red-on-revert counterexample:
// {"mode":"idle","activeSpec":"128-…"}.
func TestStateShow_DraftSpecWorktree_HonestSpecMode(t *testing.T) {
	specID := "042-draft-fixture"
	specWorktree := newDraftSpecWorktreeFixture(t, specID)
	stubNoEpics(t)
	resetStateShowFlags(t)
	withTestChdir(t, specWorktree)

	out, err := captureStdout(t, func() error {
		return stateShowCmd.RunE(stateShowCmd, nil)
	})
	if err != nil {
		t.Fatalf("state show error: %v\noutput:\n%s", err, out)
	}

	if strings.Contains(out, `"mode": "idle"`) {
		t.Fatalf("state show reported idle from a draft spec worktree (the #220 counterexample):\n%s", out)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("state show output not valid JSON: %v\noutput:\n%s", err, out)
	}
	if parsed["mode"] != "spec" {
		t.Errorf(`mode = %v, want "spec"`, parsed["mode"])
	}
	if parsed["activeSpec"] != specID {
		t.Errorf("activeSpec = %v, want %q", parsed["activeSpec"], specID)
	}
}

// TestStateShow_MainCheckout_DiscoveryFailure_RefusesLoudly is AC-1(ii):
// from the main checkout with discovery failing, state show does not exit
// 0 with "mode": "" — red-on-revert counterexample: {"mode": ""} exit 0.
func TestStateShow_MainCheckout_DiscoveryFailure_RefusesLoudly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	stubBDUnreachable(t)
	resetStateShowFlags(t)
	withTestChdir(t, root)

	out, err := captureStdout(t, func() error {
		return stateShowCmd.RunE(stateShowCmd, nil)
	})
	if err == nil {
		t.Fatalf("state show must refuse (non-zero) on a discovery failure; got output:\n%s", out)
	}
	if strings.Contains(out, `"mode": ""`) {
		t.Fatalf("state show printed the dishonest empty-mode success shape:\n%s", out)
	}
	if out != "" {
		t.Errorf("state show must print nothing on a refusal, got:\n%s", out)
	}
	if !strings.Contains(err.Error(), "recovery: ") {
		t.Errorf("refusal missing the ADR-0035 final recovery line: %v", err)
	}
}

// TestStateShow_MainCheckout_SingleDraft_HonestSpecMode is AC-1(iii): from
// the main checkout with discovery succeeding on a draft-only state, the
// output renders the honest draft representation, not bare idle.
func TestStateShow_MainCheckout_SingleDraft_HonestSpecMode(t *testing.T) {
	specID := "077-main-draft"
	root := newDraftFixtureRoot(t, specID)
	stubNoEpics(t)
	resetStateShowFlags(t)
	withTestChdir(t, root)

	out, err := captureStdout(t, func() error {
		return stateShowCmd.RunE(stateShowCmd, nil)
	})
	if err != nil {
		t.Fatalf("state show error: %v\noutput:\n%s", err, out)
	}

	if strings.Contains(out, `"mode": "idle"`) {
		t.Fatalf("state show reported bare idle for a main-checkout draft-only state:\n%s", out)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("state show output not valid JSON: %v\noutput:\n%s", err, out)
	}
	if parsed["mode"] != "spec" {
		t.Errorf(`mode = %v, want "spec"`, parsed["mode"])
	}
	if parsed["activeSpec"] != specID {
		t.Errorf("activeSpec = %v, want %q", parsed["activeSpec"], specID)
	}
	if parsed["specBranch"] != "spec/"+specID {
		t.Errorf("specBranch = %v, want %q", parsed["specBranch"], "spec/"+specID)
	}
}
