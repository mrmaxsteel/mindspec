package resolve

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
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// stubActiveEpicsFailingFirstCall is stubActiveEpics (resolve_test.go) with
// one twist: the very FIRST bd list/show call, of any kind, fails
// transiently before dispatching by args as usual. Models the round-1
// panel's G2/F2a repro: phase.Cache does not memoize errors, so a first-
// call flake must never be silently read as "no epic exists" once bd
// recovers on retry.
func stubActiveEpicsFailingFirstCall(t *testing.T, epics []phase.EpicInfo, childrenByEpic map[string][]phase.ChildInfo) {
	t.Helper()
	epicByID := map[string]phase.EpicInfo{}
	for _, e := range epics {
		epicByID[e.ID] = e
	}
	var calls int32
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return nil, errors.New("bd: transient flake")
		}
		for _, a := range args {
			if a == "--type=epic" {
				return json.Marshal(epics)
			}
		}
		for i, a := range args {
			if a == "--parent" && i+1 < len(args) {
				epicID := args[i+1]
				if children, ok := childrenByEpic[epicID]; ok {
					return json.Marshal(children)
				}
				return []byte("[]"), nil
			}
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			epicID := args[1]
			if e, ok := epicByID[epicID]; ok {
				return json.Marshal([]phase.EpicInfo{e})
			}
			return []byte("[]"), nil
		}
		return []byte("[]"), nil
	})
	t.Cleanup(restoreRun)
}

// runGitCmd runs git with args in dir, with a pinned test identity so
// commits work without host config (matching cmd/mindspec/next_dirty_test.go's
// runGitInDir convention).
func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-c", "user.email=test@example.invalid",
		"-c", "user.name=mindspec-test",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

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
	runGitCmd(t, root, "init", "-b", "main")
	specDir := filepath.Join(root, ".mindspec", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# draft\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "-m", "draft spec")
	runGitCmd(t, root, "branch", "spec/"+specID)
	return root
}

// TestResolveModeWithCache_DraftSpec_ReturnsModeSpec is OQ-2/AC-1's core
// unit pin: a spec with a directory + branch but no epic derives `spec`,
// never `idle` (the #220 counterexample).
func TestResolveModeWithCache_DraftSpec_ReturnsModeSpec(t *testing.T) {
	specID := "042-draft-fixture"
	root := newDraftFixtureRoot(t, specID)
	stubActiveEpics(t, nil, nil) // bd healthy, zero epics

	c := phase.NewCache()
	mode, err := ResolveModeWithCache(c, root, specID)
	if err != nil {
		t.Fatalf("ResolveModeWithCache() error: %v", err)
	}
	if mode != state.ModeSpec {
		t.Errorf("mode = %q, want %q", mode, state.ModeSpec)
	}
}

// TestResolveModeWithCache_NoEpicNoDraft_ReturnsIdle pins the genuinely-idle
// case is unchanged: no epic, no on-disk draft artifacts at all. A real
// (if minimal) git repo is required — draftSpecStatusWithCache's G3 tree
// probe needs a confirmed branch-exists answer, and BranchExistsIn
// correctly refuses to answer at all outside a real repository (a
// genuine structural failure, never folded into "absent").
func TestResolveModeWithCache_NoEpicNoDraft_ReturnsIdle(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitCmd(t, root, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("baseline\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "-m", "baseline")
	stubActiveEpics(t, nil, nil)

	c := phase.NewCache()
	mode, err := ResolveModeWithCache(c, root, "999-nonexistent-spec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != state.ModeIdle {
		t.Errorf("mode = %q, want %q", mode, state.ModeIdle)
	}
}

// TestResolveModeWithCache_DiscoveryFailure_Propagates is the R1.4 named-
// hazard pin: a genuine bd/discovery failure (injected via the phase
// package's bd seam) must never be read as "no epic, therefore idle" — it
// propagates as an error, and the mode return is never fabricated (empty).
func TestResolveModeWithCache_DiscoveryFailure_Propagates(t *testing.T) {
	root := t.TempDir()
	wantErr := errors.New(`exec: "bd": executable file not found in $PATH`)
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return nil, wantErr
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return nil, wantErr
	}))

	c := phase.NewCache()
	mode, err := ResolveModeWithCache(c, root, "042-my-spec")
	if err == nil {
		t.Fatalf("expected a discovery-failure error, got mode=%q", mode)
	}
	if mode != "" {
		t.Errorf("mode on a discovery failure = %q, want empty — never fabricate a mode from a failed read", mode)
	}
	if !strings.Contains(err.Error(), "querying epics") {
		t.Errorf("error should name the failed query (querying epics), got: %v", err)
	}
}

// TestDiscoverDraftSpecsWithCache_FindsSingleDraft pins the main-checkout
// discovery path AC-1(iii) relies on.
func TestDiscoverDraftSpecsWithCache_FindsSingleDraft(t *testing.T) {
	specID := "042-draft-fixture"
	root := newDraftFixtureRoot(t, specID)
	stubActiveEpics(t, nil, nil)

	c := phase.NewCache()
	drafts, err := DiscoverDraftSpecsWithCache(c, root)
	if err != nil {
		t.Fatalf("DiscoverDraftSpecsWithCache() error: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("drafts = %+v, want exactly one", drafts)
	}
	if drafts[0].SpecID != specID {
		t.Errorf("SpecID = %q, want %q", drafts[0].SpecID, specID)
	}
	if drafts[0].SpecBranch != "spec/"+specID {
		t.Errorf("SpecBranch = %q, want %q", drafts[0].SpecBranch, "spec/"+specID)
	}
}

// TestDiscoverDraftSpecsWithCache_SkipsSpecsWithEpics: a spec directory +
// branch that ALSO has a beads epic is not a draft (OQ-2 is pre-epic only).
func TestDiscoverDraftSpecsWithCache_SkipsSpecsWithEpics(t *testing.T) {
	specID := "042-draft-fixture"
	root := newDraftFixtureRoot(t, specID)
	epics := []phase.EpicInfo{
		{ID: "epic-a", Title: "[SPEC 042-draft-fixture] Draft Fixture", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(42), "spec_title": "draft-fixture"}},
	}
	stubActiveEpics(t, epics, nil)

	c := phase.NewCache()
	drafts, err := DiscoverDraftSpecsWithCache(c, root)
	if err != nil {
		t.Fatalf("DiscoverDraftSpecsWithCache() error: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("drafts = %+v, want none — this spec already has an epic", drafts)
	}
}

// TestDiscoverDraftSpecsWithCache_DiscoveryFailure_Propagates: a candidate
// draft directory exists, but bd itself is unreachable — the scan must
// refuse rather than silently reporting zero drafts (which would collapse
// to a false idle in state.go's caller).
func TestDiscoverDraftSpecsWithCache_DiscoveryFailure_Propagates(t *testing.T) {
	specID := "042-draft-fixture"
	root := newDraftFixtureRoot(t, specID)
	wantErr := errors.New(`exec: "bd": executable file not found in $PATH`)
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return nil, wantErr
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return nil, wantErr
	}))

	c := phase.NewCache()
	drafts, err := DiscoverDraftSpecsWithCache(c, root)
	if err == nil {
		t.Fatalf("expected a discovery-failure error, got drafts=%+v", drafts)
	}
	if drafts != nil {
		t.Errorf("drafts on a discovery failure = %+v, want nil", drafts)
	}
}

// TestDiscoverDraftSpecsWithCache_NoSpecsDir: an empty/nonexistent specs
// directory (and no spec/* branches) is zero drafts, not an error.
func TestDiscoverDraftSpecsWithCache_NoSpecsDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitCmd(t, root, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("baseline\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "-m", "baseline")
	stubActiveEpics(t, nil, nil)

	c := phase.NewCache()
	drafts, err := DiscoverDraftSpecsWithCache(c, root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("drafts = %+v, want none", drafts)
	}
}

// TestResolveModeWithCache_TransientFirstCallFailure_FindsRecoveredEpic is
// G2/F2a's regression pin: a first-call bd flake must not discard an epic
// that a health-probe-warmed retry finds — the caller must derive the
// epic's REAL phase, never fabricate idle.
func TestResolveModeWithCache_TransientFirstCallFailure_FindsRecoveredEpic(t *testing.T) {
	specID := "131-active"
	root := t.TempDir() // the epic is found on retry, so the draft probe
	// (which would need a git repo) is never reached.
	epics := []phase.EpicInfo{
		{ID: "epic-a", Title: "[SPEC 131-active] Active", Status: "open", IssueType: "epic",
			Metadata: map[string]interface{}{"spec_num": float64(131), "spec_title": "active"}},
	}
	childrenByEpic := map[string][]phase.ChildInfo{
		"epic-a": {{ID: "bead-1", Status: "in_progress", IssueType: "task"}},
	}
	stubActiveEpicsFailingFirstCall(t, epics, childrenByEpic)

	c := phase.NewCache()
	mode, err := ResolveModeWithCache(c, root, specID)
	if err != nil {
		t.Fatalf("ResolveModeWithCache() error: %v", err)
	}
	if mode != state.ModeImplement {
		t.Errorf("mode = %q, want %q — a recovered epic lookup must derive the real phase, never idle", mode, state.ModeImplement)
	}
}

// TestResolveModeWithCache_TransientFirstCallFailure_GenuineNoEpicStillProbesDraft
// pins that the G2/F2a retry does not break the ordinary draft path when
// bd genuinely has no epic for the spec once it recovers.
func TestResolveModeWithCache_TransientFirstCallFailure_GenuineNoEpicStillProbesDraft(t *testing.T) {
	specID := "042-draft-fixture"
	root := newDraftFixtureRoot(t, specID)
	stubActiveEpicsFailingFirstCall(t, nil, nil)

	c := phase.NewCache()
	mode, err := ResolveModeWithCache(c, root, specID)
	if err != nil {
		t.Fatalf("ResolveModeWithCache() error: %v", err)
	}
	if mode != state.ModeSpec {
		t.Errorf("mode = %q, want %q", mode, state.ModeSpec)
	}
}

// TestDiscoverDraftSpecsWithCache_MultipleDrafts_SortedResult is G5's unit
// pin: two drafts, sorted by SpecID.
func TestDiscoverDraftSpecsWithCache_MultipleDrafts_SortedResult(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitCmd(t, root, "init", "-b", "main")
	for _, specID := range []string{"130-bar", "129-foo"} { // created out of order
		specDir := filepath.Join(root, ".mindspec", "specs", specID)
		if err := os.MkdirAll(specDir, 0o755); err != nil {
			t.Fatalf("mkdir spec dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# draft\n"), 0o644); err != nil {
			t.Fatalf("write spec.md: %v", err)
		}
	}
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "-m", "two drafts")
	runGitCmd(t, root, "branch", "spec/130-bar")
	runGitCmd(t, root, "branch", "spec/129-foo")
	stubActiveEpics(t, nil, nil)

	c := phase.NewCache()
	drafts, err := DiscoverDraftSpecsWithCache(c, root)
	if err != nil {
		t.Fatalf("DiscoverDraftSpecsWithCache() error: %v", err)
	}
	if len(drafts) != 2 {
		t.Fatalf("drafts = %+v, want exactly two", drafts)
	}
	if drafts[0].SpecID != "129-foo" || drafts[1].SpecID != "130-bar" {
		t.Errorf("drafts not sorted by SpecID: %+v", drafts)
	}
}

// newBranchOnlyDraftFixture builds a repo shaped like the REAL `spec
// create` output as seen from a checkout that never had the draft written
// into its own working tree (round-1 panel G3): the spec/<id> branch
// carries a real committed spec.md, but root's OWN working tree (standing
// in for "the main checkout") never does — spec create writes and commits
// the spec directory only inside the spec workspace it creates
// (internal/spec/create.go), never to the invoking checkout.
//
// Mechanism: `git worktree add` creates the spec/<id> branch and a
// SEPARATE worktree; the spec dir is written and committed there; the
// worktree is then removed, leaving the branch + commit behind but never
// touching root's own working tree — byte-for-byte what spec create does,
// minus mindspec's own CLI wrapping.
func newBranchOnlyDraftFixture(t *testing.T, specID string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitCmd(t, root, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("baseline\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "-m", "baseline")

	otherWT := t.TempDir()
	runGitCmd(t, root, "worktree", "add", "-b", "spec/"+specID, otherWT)
	specDir := filepath.Join(otherWT, ".mindspec", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# draft\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	runGitCmd(t, otherWT, "add", "-A")
	runGitCmd(t, otherWT, "commit", "-m", "chore: initialize spec "+specID)
	runGitCmd(t, root, "worktree", "remove", otherWT, "--force")

	return root
}

// TestDraftSpecStatusWithCache_BranchOnlyDraft_VisibleViaTreeProbe is G3's
// (MAJOR) regression pin: a draft whose spec dir was committed only to the
// spec/<id> branch — never checked out into root's own working tree, the
// real shape `spec create` produces — is still discovered as a draft by
// reading the branch's own committed tree.
func TestDraftSpecStatusWithCache_BranchOnlyDraft_VisibleViaTreeProbe(t *testing.T) {
	specID := "142-branch-only"
	root := newBranchOnlyDraftFixture(t, specID)
	stubActiveEpics(t, nil, nil)

	// The plain on-disk check must genuinely miss — pinning the fixture's
	// premise (this is NOT the AC-1(i)/(iii) in-worktree-dir shape).
	dir, err := workspace.SpecDir(root, specID)
	if err != nil {
		t.Fatalf("SpecDir: %v", err)
	}
	if _, statErr := os.Stat(dir); statErr == nil {
		t.Fatalf("fixture premise violated: %s exists on disk", dir)
	}

	c := phase.NewCache()
	isDraft, branch, err := draftSpecStatusWithCache(c, root, specID)
	if err != nil {
		t.Fatalf("draftSpecStatusWithCache() error: %v", err)
	}
	if !isDraft {
		t.Fatalf("isDraft = false, want true — the branch-committed spec.md must be visible via the tree probe")
	}
	if branch != "spec/"+specID {
		t.Errorf("branch = %q, want %q", branch, "spec/"+specID)
	}

	mode, err := ResolveModeWithCache(c, root, specID)
	if err != nil {
		t.Fatalf("ResolveModeWithCache() error: %v", err)
	}
	if mode != state.ModeSpec {
		t.Errorf("mode = %q, want %q", mode, state.ModeSpec)
	}

	drafts, err := DiscoverDraftSpecsWithCache(phase.NewCache(), root)
	if err != nil {
		t.Fatalf("DiscoverDraftSpecsWithCache() error: %v", err)
	}
	if len(drafts) != 1 || drafts[0].SpecID != specID {
		t.Errorf("drafts = %+v, want exactly [%s]", drafts, specID)
	}
}
