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
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
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

// setStateShowSpecFlag sets --spec on stateShowCmd for a direct RunE call
// (which bypasses cobra's own flag parsing).
func setStateShowSpecFlag(t *testing.T, value string) {
	t.Helper()
	if err := stateShowCmd.Flags().Set("spec", value); err != nil {
		t.Fatalf("set --spec=%s: %v", value, err)
	}
}

// newBareGitRepo builds a minimal git repo with a baseline commit and no
// spec dirs/branches at all — the fixture premise for tests exercising the
// explicit-flag pass-through against a spec that genuinely does not exist.
func newBareGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitInDir(t, root, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("baseline\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runGitInDir(t, root, "add", "-A")
	runGitInDir(t, root, "commit", "-m", "baseline")
	return root
}

// TestStateShow_FlagBlind_UnresolvableSpecWithOneDraft_Refuses is round-1
// panel G1's core repro (R1/R4/R5/R6 four-slot convergence): an explicit
// --spec that fails to resolve must never be answered with a DIFFERENT,
// unrelated draft's state — red-on-revert counterexample: --spec 999 with
// draft 129-foo on disk printed {"mode":"spec","activeSpec":"129-foo"}
// exit 0.
func TestStateShow_FlagBlind_UnresolvableSpecWithOneDraft_Refuses(t *testing.T) {
	root := newDraftFixtureRoot(t, "129-foo")
	stubNoEpics(t)
	resetStateShowFlags(t)
	setStateShowSpecFlag(t, "999")
	withTestChdir(t, root)

	out, err := captureStdout(t, func() error {
		return stateShowCmd.RunE(stateShowCmd, nil)
	})
	if err == nil {
		t.Fatalf("state show --spec 999 must refuse; got output:\n%s", out)
	}
	if strings.Contains(out, "129-foo") || strings.Contains(out, `"mode": "spec"`) {
		t.Fatalf("state show --spec 999 answered about the UNRELATED draft 129-foo:\n%s", out)
	}
	if out != "" {
		t.Errorf("state show must print nothing on a refusal, got:\n%s", out)
	}
}

// TestStateShow_FlagBlind_UnresolvableSpecWithNoDrafts_Refuses is G1's
// zero-draft leg: red-on-revert counterexample: --spec 999 with no drafts
// printed {"mode":"idle"} exit 0.
func TestStateShow_FlagBlind_UnresolvableSpecWithNoDrafts_Refuses(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	stubNoEpics(t)
	resetStateShowFlags(t)
	setStateShowSpecFlag(t, "999")
	withTestChdir(t, root)

	out, err := captureStdout(t, func() error {
		return stateShowCmd.RunE(stateShowCmd, nil)
	})
	if err == nil {
		t.Fatalf("state show --spec 999 must refuse; got output:\n%s", out)
	}
	if strings.Contains(out, `"mode"`) {
		t.Fatalf("state show --spec 999 printed a success shape instead of refusing:\n%s", out)
	}
}

// TestStateShow_FlagBlind_InvalidSpecValue_Refuses is G1's invalid-format
// leg.
func TestStateShow_FlagBlind_InvalidSpecValue_Refuses(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	stubNoEpics(t)
	resetStateShowFlags(t)
	setStateShowSpecFlag(t, "not_a_valid_id!!")
	withTestChdir(t, root)

	out, err := captureStdout(t, func() error {
		return stateShowCmd.RunE(stateShowCmd, nil)
	})
	if err == nil {
		t.Fatalf("state show --spec 'not_a_valid_id!!' must refuse; got output:\n%s", out)
	}
	if out != "" {
		t.Errorf("state show must print nothing on a refusal, got:\n%s", out)
	}
}

// TestStateShow_FlaggedNonexistentSpec_RefusesInsteadOfIdle is G1's
// success-path leg: --spec <valid-format-but-nonexistent-ID> passes
// ResolveSpecPrefixWithCache's hyphenated pass-through unchecked, then
// honestly derives idle (no epic, no draft) — red-on-revert
// counterexample: {"mode":"idle","activeSpec":"999-nonexistent-spec",...}
// exit 0, the R1.1-banned idle-with-non-empty-activeSpec shape.
func TestStateShow_FlaggedNonexistentSpec_RefusesInsteadOfIdle(t *testing.T) {
	root := newBareGitRepo(t)
	stubNoEpics(t)
	resetStateShowFlags(t)
	setStateShowSpecFlag(t, "999-nonexistent-spec")
	withTestChdir(t, root)

	out, err := captureStdout(t, func() error {
		return stateShowCmd.RunE(stateShowCmd, nil)
	})
	if err == nil {
		t.Fatalf("state show --spec 999-nonexistent-spec must refuse; got output:\n%s", out)
	}
	if strings.Contains(out, `"mode": "idle"`) {
		t.Fatalf("state show printed the R1.1-banned idle-with-activeSpec shape:\n%s", out)
	}
	if out != "" {
		t.Errorf("state show must print nothing on a refusal, got:\n%s", out)
	}
}

// stubBDFailFirstCallThenHealthy stubs a bd that fails its very FIRST call
// (of any kind) then reports the given epics healthily thereafter —
// round-1 panel G2/F2b's repro: phase.Cache does not memoize errors, so a
// transient flake must not be silently read as "bd healthy, nothing
// active" once the retry succeeds.
func stubBDFailFirstCallThenHealthy(t *testing.T, epicsJSON string) {
	t.Helper()
	var calls int32
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return nil, errors.New("bd: transient flake")
		}
		for _, a := range args {
			if a == "--type=epic" {
				return []byte(epicsJSON), nil
			}
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
}

// TestStateShow_TransientDiscoveryFailure_RetriesAndFindsActiveSpec is
// G2/F2b's regression pin: no-flag `state show` over a single transient bd
// flake must retry against the now-warm cache and report the REAL active
// spec, never fabricate idle — red-on-revert counterexample: {"mode":
// "idle"} exit 0 while the healthy-bd control on the identical fixture
// reports {"mode":"plan","activeSpec":"131-active"}.
func TestStateShow_TransientDiscoveryFailure_RetriesAndFindsActiveSpec(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	epicsJSON := `[{"id":"epic-a","title":"[SPEC 131-active] Active","status":"open","issue_type":"epic","metadata":{"spec_num":131,"spec_title":"active"}}]`
	stubBDFailFirstCallThenHealthy(t, epicsJSON)
	resetStateShowFlags(t)
	withTestChdir(t, root)

	out, err := captureStdout(t, func() error {
		return stateShowCmd.RunE(stateShowCmd, nil)
	})
	if err != nil {
		t.Fatalf("state show error: %v\noutput:\n%s", err, out)
	}

	var parsed map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(out), &parsed); jsonErr != nil {
		t.Fatalf("state show output not valid JSON: %v\noutput:\n%s", jsonErr, out)
	}
	if parsed["mode"] != "plan" {
		t.Errorf(`mode = %v, want "plan" — a recovered active spec must not be discarded as idle`, parsed["mode"])
	}
	if parsed["activeSpec"] != "131-active" {
		t.Errorf("activeSpec = %v, want %q", parsed["activeSpec"], "131-active")
	}
}

// TestStateShow_MultipleDrafts_NoSpecFlag_ReturnsAmbiguityListing is G5's
// state.go-level pin: two on-disk drafts, no --spec, must return the
// existing ErrAmbiguousTarget listing both at spec mode — delivered
// behavior that round 1 found untested at either layer.
func TestStateShow_MultipleDrafts_NoSpecFlag_ReturnsAmbiguityListing(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitInDir(t, root, "init", "-b", "main")
	for _, specID := range []string{"130-bar", "129-foo"} {
		specDir := filepath.Join(root, ".mindspec", "specs", specID)
		if err := os.MkdirAll(specDir, 0o755); err != nil {
			t.Fatalf("mkdir spec dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# draft\n"), 0o644); err != nil {
			t.Fatalf("write spec.md: %v", err)
		}
	}
	runGitInDir(t, root, "add", "-A")
	runGitInDir(t, root, "commit", "-m", "two drafts")
	runGitInDir(t, root, "branch", "spec/130-bar")
	runGitInDir(t, root, "branch", "spec/129-foo")
	stubNoEpics(t)
	resetStateShowFlags(t)
	withTestChdir(t, root)

	out, err := captureStdout(t, func() error {
		return stateShowCmd.RunE(stateShowCmd, nil)
	})
	if err == nil {
		t.Fatalf("expected *resolve.ErrAmbiguousTarget, got output:\n%s", out)
	}
	var ambErr *resolve.ErrAmbiguousTarget
	if !errors.As(err, &ambErr) {
		t.Fatalf("error is not *resolve.ErrAmbiguousTarget: %v", err)
	}
	if len(ambErr.Active) != 2 {
		t.Fatalf("Active = %+v, want two entries", ambErr.Active)
	}
	for _, s := range ambErr.Active {
		if s.Mode != "spec" {
			t.Errorf("entry %+v: Mode should be spec", s)
		}
	}
	if !strings.Contains(err.Error(), "129-foo") || !strings.Contains(err.Error(), "130-bar") {
		t.Errorf("error should list both drafts: %v", err)
	}
}

// newBranchOnlyDraftFixture is state_show_honesty_test.go's copy of
// internal/resolve/draft_test.go's helper of the same name: a repo shaped
// like the REAL `spec create` output as seen from a checkout that never
// had the draft written into its own working tree — the spec/<id> branch
// carries a real committed spec.md via a separate worktree that is then
// removed, leaving root's own working tree untouched.
func newBranchOnlyDraftFixture(t *testing.T, specID string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitInDir(t, root, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("baseline\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runGitInDir(t, root, "add", "-A")
	runGitInDir(t, root, "commit", "-m", "baseline")

	otherWT := t.TempDir()
	runGitInDir(t, root, "worktree", "add", "-b", "spec/"+specID, otherWT)
	specDir := filepath.Join(otherWT, ".mindspec", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# draft\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	runGitInDir(t, otherWT, "add", "-A")
	runGitInDir(t, otherWT, "commit", "-m", "chore: initialize spec "+specID)
	runGitInDir(t, root, "worktree", "remove", otherWT, "--force")

	return root
}

// TestStateShow_AmbientMainCheckout_BranchOnlyDraft_ShadowTagDoesNotHideIt
// is round-3 panel's end-to-end regression pin — the EXACT reproduction
// all three round-3 slots ran: a same-named tag on `main` (reachable in
// any clone via a normal `git fetch`) must not make ambient (no --spec)
// `mindspec state show` from the main checkout fabricate bare idle for a
// genuine branch-only draft. Round 2's qualified-ref tree-probe fix alone
// did not close this — the shadow also broke branch CANDIDATE
// ENUMERATION (gitutil.LocalBranchRefs's `%(refname:short)` disambiguates
// under the tag, dropping the branch from the candidate set before the
// probe is ever reached); round 3 fixes enumeration too
// (gitutil.LocalBranchNames).
func TestStateShow_AmbientMainCheckout_BranchOnlyDraft_ShadowTagDoesNotHideIt(t *testing.T) {
	specID := "134-ambient-shadow"
	root := newBranchOnlyDraftFixture(t, specID)
	runGitInDir(t, root, "tag", "spec/"+specID, "main")
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
		t.Fatalf("ambient state show reverted the branch-only draft to bare idle under a same-named shadow tag (round-3 panel repro):\n%s", out)
	}

	var parsed map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(out), &parsed); jsonErr != nil {
		t.Fatalf("state show output not valid JSON: %v\noutput:\n%s", jsonErr, out)
	}
	if parsed["mode"] != "spec" {
		t.Errorf(`mode = %v, want "spec"`, parsed["mode"])
	}
	if parsed["activeSpec"] != specID {
		t.Errorf("activeSpec = %v, want %q", parsed["activeSpec"], specID)
	}
}
