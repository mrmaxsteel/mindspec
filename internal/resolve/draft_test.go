package resolve

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
// case is unchanged: no epic, no on-disk draft artifacts at all.
func TestResolveModeWithCache_NoEpicNoDraft_ReturnsIdle(t *testing.T) {
	root := t.TempDir() // no git repo, no spec dir — SpecDir/branch checks
	// resolve to nonexistent paths without ever needing a git binary.
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
// directory is zero drafts, not an error.
func TestDiscoverDraftSpecsWithCache_NoSpecsDir(t *testing.T) {
	root := t.TempDir()
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
