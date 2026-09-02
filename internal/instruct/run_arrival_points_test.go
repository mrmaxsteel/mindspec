package instruct

// run_arrival_points_test.go — spec 128 (lifecycle-verb-state-honesty)
// bead 5, round-1 panel G5: three instruct arrival-point gaps.
//
//  1. The protected-checkout fast path must prove beads health before
//     rendering idle even with ZERO draft candidates (handleIdleOrDraft's
//     own DiscoverDraftSpecsWithCache call never probes bd at all when
//     there are no on-disk/branch candidates to check).
//  2. A beads-derived main-context Phase==idle (the WorktreeMain
//     zero-active-specs case) is ALSO a draft-discovery arrival point,
//     not only an empty Phase.
//  3. An explicit numeric --spec prefix targeting a pre-epic draft must
//     be honored or explicitly refused — never silently fall through to
//     an unrelated ambient state or idle.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// newCleanInstructFixture builds a hermetic (t.TempDir()-rooted) git repo
// on the given branch, with NO spec directories or branches — the clean
// baseline every test in this file starts from (unlike setupRunTestProject,
// whose setupTestProject fixture carries incidental legacy spec dirs that
// would otherwise register as spurious draft candidates). Mirrors
// setupRunTestProject's cwd/GIT_CEILING_DIRECTORIES discipline (spec 119
// R12 / mindspec-z4ps): phase.ResolveContext(WithCache) and bead.RunBD
// both resolve relative to the PROCESS cwd, not an explicit root
// parameter, so the sandbox must be the process's actual cwd for the
// test's duration.
func newCleanInstructFixture(t *testing.T, branch string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", "-b", branch, root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	runGit(t, root, "-c", "user.email=test@example.invalid", "-c", "user.name=mindspec-test", "commit", "--allow-empty", "-q", "-m", "baseline")

	origCeiling, hadCeiling := os.LookupEnv("GIT_CEILING_DIRECTORIES")
	if err := os.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root)); err != nil {
		t.Fatalf("setenv GIT_CEILING_DIRECTORIES: %v", err)
	}
	t.Cleanup(func() {
		if hadCeiling {
			_ = os.Setenv("GIT_CEILING_DIRECTORIES", origCeiling)
		} else {
			_ = os.Unsetenv("GIT_CEILING_DIRECTORIES")
		}
	})

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir %s: %v", root, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	return root
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// addBeadsDir creates root/.beads so phase.BeadsDirMissing(root) is
// false — the precondition every test below needs to distinguish a
// genuine bd health failure from an honestly-uninitialized workspace.
func addBeadsDir(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
}

// addDraftSpec commits a pre-epic draft into root: a
// .mindspec/specs/<specID>/spec.md directory plus a `spec/<specID>`
// branch — the OQ-2 draft shape (matching cmd/mindspec's
// newDraftFixtureRoot / internal/resolve's draftFixtureRoot convention).
func addDraftSpec(t *testing.T, root, specID string) {
	t.Helper()
	specDir := filepath.Join(root, ".mindspec", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# draft\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "-c", "user.email=test@example.invalid", "-c", "user.name=mindspec-test", "commit", "-q", "-m", "draft "+specID)
	runGit(t, root, "branch", "spec/"+specID)
}

// stubHealthyNoEpics stubs bd as reachable with zero epics — the
// baseline for every G5.2/G5.3 test (health confirmed, nothing active).
func stubHealthyNoEpics(t *testing.T) {
	t.Helper()
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
}

// stubBDDown stubs every bd invocation failing — the G5.1 fixture's
// "beads present but unreachable" precondition.
func stubBDDown(t *testing.T) {
	t.Helper()
	wantErr := errors.New("bd: dolt daemon unreachable")
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return nil, wantErr
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return nil, wantErr
	}))
}

// TestRun_ProtectedCheckout_ZeroDraftCandidates_BdDown_RendersIndeterminate
// is G5.1's red-on-revert pin: protected main, .beads present, zero
// on-disk/branch draft candidates, bd unreachable — the output must be
// indeterminate, never idle. Before the fix, DiscoverDraftSpecsWithCache
// never issues a SINGLE bd call when there are no candidates to check, so
// bd's outage went completely unnoticed and idle rendered anyway.
func TestRun_ProtectedCheckout_ZeroDraftCandidates_BdDown_RendersIndeterminate(t *testing.T) {
	root := newCleanInstructFixture(t, "main")
	addBeadsDir(t, root)
	stubBDDown(t)

	var buf bytes.Buffer
	if err := RunWithOptions(context.Background(), root, "", "", &buf, Options{}); err != nil {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "No Active Work") {
		t.Errorf("rendered idle despite bd being unreachable (zero-candidate health-probe gap); got:\n%s", out)
	}
	if !strings.Contains(out, "cannot determine mode") {
		t.Errorf("expected the indeterminate marker text, got:\n%s", out)
	}
}

// TestRun_UnprotectedCheckout_BeadsIdleArrivalPoint_NoDrafts_RendersIdle is
// G5.2's "none" leg: an unprotected checkout with bd healthy but zero
// active specs and zero on-disk drafts renders plain idle.
func TestRun_UnprotectedCheckout_BeadsIdleArrivalPoint_NoDrafts_RendersIdle(t *testing.T) {
	root := newCleanInstructFixture(t, "feature-x")
	addBeadsDir(t, root)
	stubHealthyNoEpics(t)

	var buf bytes.Buffer
	if err := RunWithOptions(context.Background(), root, "", "", &buf, Options{}); err != nil {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No Active Work") {
		t.Errorf("expected idle template heading, got:\n%s", out)
	}
}

// TestRun_UnprotectedCheckout_BeadsIdleArrivalPoint_OneDraft_RendersSpecMode
// is G5.2's "one draft" leg: bd honestly derives idle (0 active specs),
// but a pre-epic draft exists on disk/branch — the tail must render spec
// guidance for that draft, never idle.
func TestRun_UnprotectedCheckout_BeadsIdleArrivalPoint_OneDraft_RendersSpecMode(t *testing.T) {
	root := newCleanInstructFixture(t, "feature-x")
	addBeadsDir(t, root)
	stubHealthyNoEpics(t)
	specID := "042-g52-single-draft"
	addDraftSpec(t, root, specID)

	var buf bytes.Buffer
	if err := RunWithOptions(context.Background(), root, "", "", &buf, Options{}); err != nil {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "No Active Work") {
		t.Errorf("rendered idle despite an on-disk draft; got:\n%s", out)
	}
	if !strings.Contains(out, "MindSpec — Spec Mode") {
		t.Errorf("expected the spec-mode heading, got:\n%s", out)
	}
	if !strings.Contains(out, specID) {
		t.Errorf("expected the draft's spec ID %q in the output, got:\n%s", specID, out)
	}
}

// TestRun_UnprotectedCheckout_BeadsIdleArrivalPoint_MultipleDrafts_RendersAmbiguity
// is G5.2's "multiple drafts" leg: two pre-epic drafts with bd-derived
// idle must render the ambiguity listing, never an arbitrary pick or
// idle.
func TestRun_UnprotectedCheckout_BeadsIdleArrivalPoint_MultipleDrafts_RendersAmbiguity(t *testing.T) {
	root := newCleanInstructFixture(t, "feature-x")
	addBeadsDir(t, root)
	stubHealthyNoEpics(t)
	addDraftSpec(t, root, "042-g52-multi-a")
	addDraftSpec(t, root, "043-g52-multi-b")

	var buf bytes.Buffer
	if err := RunWithOptions(context.Background(), root, "", "", &buf, Options{}); err != nil {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "No Active Work") {
		t.Errorf("rendered idle despite two on-disk drafts; got:\n%s", out)
	}
	if !strings.Contains(out, "Multiple Active Specs") {
		t.Errorf("expected the ambiguity listing, got:\n%s", out)
	}
	if !strings.Contains(out, "042-g52-multi-a") || !strings.Contains(out, "043-g52-multi-b") {
		t.Errorf("expected both draft spec IDs listed, got:\n%s", out)
	}
}

// TestRun_ExplicitSpecFlag_NumericPrefixMatchesDraft_RendersSpecMode is
// G5.3's numeric-prefix leg: an explicit --spec numeric prefix that fails
// to resolve against ACTIVE (bd) specs, but matches exactly one on-disk
// draft's 3-digit prefix, must render that draft's spec guidance.
func TestRun_ExplicitSpecFlag_NumericPrefixMatchesDraft_RendersSpecMode(t *testing.T) {
	root := newCleanInstructFixture(t, "main")
	addBeadsDir(t, root)
	stubHealthyNoEpics(t)
	specID := "044-g53-numeric"
	addDraftSpec(t, root, specID)

	var buf bytes.Buffer
	if err := RunWithOptions(context.Background(), root, "", "044", &buf, Options{}); err != nil {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "MindSpec — Spec Mode") {
		t.Errorf("expected the spec-mode heading for the numeric-prefix-matched draft, got:\n%s", out)
	}
	if !strings.Contains(out, specID) {
		t.Errorf("expected the resolved spec ID %q, got:\n%s", specID, out)
	}
}

// TestRun_ExplicitSpecFlag_FullIDMatchesDraft_RendersSpecMode is G5.3's
// full-ID leg.
func TestRun_ExplicitSpecFlag_FullIDMatchesDraft_RendersSpecMode(t *testing.T) {
	root := newCleanInstructFixture(t, "main")
	addBeadsDir(t, root)
	stubHealthyNoEpics(t)
	specID := "045-g53-full-id"
	addDraftSpec(t, root, specID)

	var buf bytes.Buffer
	if err := RunWithOptions(context.Background(), root, "", specID, &buf, Options{}); err != nil {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "MindSpec — Spec Mode") {
		t.Errorf("expected the spec-mode heading for the full-ID-matched draft, got:\n%s", out)
	}
}

// TestRun_ExplicitSpecFlag_NonexistentTarget_Refuses is G5.3's
// nonexistent-target leg: an explicit --spec that matches neither an
// active spec nor any draft must REFUSE — never silently fall through to
// an unrelated ambient state or idle.
func TestRun_ExplicitSpecFlag_NonexistentTarget_Refuses(t *testing.T) {
	root := newCleanInstructFixture(t, "main")
	addBeadsDir(t, root)
	stubHealthyNoEpics(t)
	addDraftSpec(t, root, "046-g53-unrelated")

	var buf bytes.Buffer
	err := RunWithOptions(context.Background(), root, "", "999", &buf, Options{})
	if err == nil {
		t.Fatalf("expected a refusal for a --spec target matching neither an active spec nor any draft; got output:\n%s", buf.String())
	}
	out := buf.String()
	if strings.Contains(out, "No Active Work") || strings.Contains(out, "046-g53-unrelated") {
		t.Errorf("refusal must not silently render idle or an unrelated draft's guidance; got:\n%s", out)
	}
}

// TestRun_ExplicitSpecFlag_AmbiguousPrefixMatchesTwoDrafts_RendersAmbiguity
// is G5.3's ambiguous-prefix leg: a numeric prefix matching MORE THAN ONE
// on-disk draft must render the ambiguity listing, never an arbitrary
// pick.
func TestRun_ExplicitSpecFlag_AmbiguousPrefixMatchesTwoDrafts_RendersAmbiguity(t *testing.T) {
	root := newCleanInstructFixture(t, "main")
	addBeadsDir(t, root)
	stubHealthyNoEpics(t)
	addDraftSpec(t, root, "047-g53-collide-a")
	addDraftSpec(t, root, "0470-g53-collide-b")

	var buf bytes.Buffer
	if err := RunWithOptions(context.Background(), root, "", "047", &buf, Options{}); err != nil {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Multiple Active Specs") {
		t.Errorf("expected the ambiguity listing for a colliding numeric prefix, got:\n%s", out)
	}
}
