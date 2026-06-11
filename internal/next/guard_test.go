package next

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// --- classifyDirty ---

func TestClassifyDirty_OnlyArtifact(t *testing.T) {
	artifact, user := classifyDirty([]string{".beads/issues.jsonl"})
	if !reflect.DeepEqual(artifact, []string{".beads/issues.jsonl"}) {
		t.Errorf("artifactDirt = %v, want [.beads/issues.jsonl]", artifact)
	}
	if len(user) != 0 {
		t.Errorf("userDirt = %v, want empty", user)
	}
}

func TestClassifyDirty_OnlyUser(t *testing.T) {
	artifact, user := classifyDirty([]string{"foo.txt", "internal/next/guard.go"})
	if len(artifact) != 0 {
		t.Errorf("artifactDirt = %v, want empty", artifact)
	}
	if !reflect.DeepEqual(user, []string{"foo.txt", "internal/next/guard.go"}) {
		t.Errorf("userDirt = %v", user)
	}
}

func TestClassifyDirty_Mixed(t *testing.T) {
	artifact, user := classifyDirty([]string{".beads/issues.jsonl", "foo.txt"})
	if !reflect.DeepEqual(artifact, []string{".beads/issues.jsonl"}) {
		t.Errorf("artifactDirt = %v", artifact)
	}
	if !reflect.DeepEqual(user, []string{"foo.txt"}) {
		t.Errorf("userDirt = %v", user)
	}
}

func TestClassifyDirty_SkipsEmpty(t *testing.T) {
	artifact, user := classifyDirty([]string{"", "  ", ".beads/issues.jsonl"})
	if !reflect.DeepEqual(artifact, []string{".beads/issues.jsonl"}) {
		t.Errorf("artifactDirt = %v", artifact)
	}
	if len(user) != 0 {
		t.Errorf("userDirt = %v, want empty", user)
	}
}

func TestClassifyDirty_NearMissIsUserDirt(t *testing.T) {
	// Whole-path equality: a similarly-named file is NOT treated as an
	// artifact. Prevents a casual rename (e.g. `beads/issues.jsonl` without
	// the dot prefix, or a `.beads/issues.jsonl.bak`) from silently
	// bypassing the guard.
	artifact, user := classifyDirty([]string{
		"beads/issues.jsonl",
		".beads/issues.jsonl.bak",
		"other/.beads/issues.jsonl",
	})
	if len(artifact) != 0 {
		t.Errorf("artifactDirt = %v, want empty (only exact-path matches count)", artifact)
	}
	if len(user) != 3 {
		t.Errorf("userDirt count = %d, want 3", len(user))
	}
}

// --- parsePorcelain ---

func TestParsePorcelain_SingleFile(t *testing.T) {
	got := parsePorcelain(" M .beads/issues.jsonl\n")
	if !reflect.DeepEqual(got, []string{".beads/issues.jsonl"}) {
		t.Errorf("got %v", got)
	}
}

func TestParsePorcelain_MultipleFiles(t *testing.T) {
	input := " M .beads/issues.jsonl\n?? newfile.txt\n M internal/next/guard.go\n"
	got := parsePorcelain(input)
	want := []string{".beads/issues.jsonl", "newfile.txt", "internal/next/guard.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParsePorcelain_RenameTakesNewPath(t *testing.T) {
	got := parsePorcelain("R  old/name.go -> new/name.go\n")
	if !reflect.DeepEqual(got, []string{"new/name.go"}) {
		t.Errorf("got %v, want [new/name.go]", got)
	}
}

func TestParsePorcelain_EmptyAndShortLinesIgnored(t *testing.T) {
	got := parsePorcelain("\n \n M foo.txt\n")
	if !reflect.DeepEqual(got, []string{"foo.txt"}) {
		t.Errorf("got %v", got)
	}
}

func TestParsePorcelain_StagedAndUnstaged(t *testing.T) {
	// XY format: X=index status, Y=worktree status. Both should yield the path.
	got := parsePorcelain("MM .beads/issues.jsonl\nAM new.txt\n")
	want := []string{".beads/issues.jsonl", "new.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// --- CheckDirtyTree ---

// fakeGuard captures the calls made to the guard's injected helpers so tests
// can assert both the decision and the side effects (bd export invocation,
// re-check count).
type fakeGuard struct {
	porcelainResponses []string // popped front-first per call
	porcelainErr       error
	porcelainCalls     int
	exportCalled       bool
	exportErr          error
	exportRoot         string
}

func (f *fakeGuard) install(t *testing.T) {
	t.Helper()
	origStatus := statusPorcelainFn
	origExport := exportBeadsFn
	t.Cleanup(func() {
		statusPorcelainFn = origStatus
		exportBeadsFn = origExport
	})
	statusPorcelainFn = func(cwd string) (string, error) {
		if f.porcelainErr != nil {
			return "", f.porcelainErr
		}
		if f.porcelainCalls >= len(f.porcelainResponses) {
			f.porcelainCalls++
			return "", nil
		}
		resp := f.porcelainResponses[f.porcelainCalls]
		f.porcelainCalls++
		return resp, nil
	}
	exportBeadsFn = func(workdir string) error {
		f.exportCalled = true
		f.exportRoot = workdir
		return f.exportErr
	}
}

func TestCheckDirtyTree_Clean(t *testing.T) {
	g := &fakeGuard{porcelainResponses: []string{""}}
	g.install(t)

	userDirt, err := CheckDirtyTree("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(userDirt) != 0 {
		t.Errorf("userDirt = %v, want empty", userDirt)
	}
	if g.exportCalled {
		t.Error("export should not be called when tree is clean")
	}
	if g.porcelainCalls != 1 {
		t.Errorf("porcelain calls = %d, want 1 (no re-check on clean)", g.porcelainCalls)
	}
}

func TestCheckDirtyTree_OnlyArtifact_Proceeds(t *testing.T) {
	// Artifact dirty → run bd export → re-check returns clean → proceed.
	g := &fakeGuard{porcelainResponses: []string{
		" M .beads/issues.jsonl\n",
		"",
	}}
	g.install(t)

	userDirt, err := CheckDirtyTree("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(userDirt) != 0 {
		t.Errorf("userDirt = %v, want empty (proceed)", userDirt)
	}
	if !g.exportCalled {
		t.Error("export should run when artifact is dirty")
	}
	if g.exportRoot != "/repo" {
		t.Errorf("export called with root %q, want /repo", g.exportRoot)
	}
	if g.porcelainCalls != 2 {
		t.Errorf("porcelain calls = %d, want 2 (pre- and post-export)", g.porcelainCalls)
	}
}

func TestCheckDirtyTree_ArtifactPlusUserDirt_Aborts(t *testing.T) {
	// Mixed dirt — the export may clean up the artifact but foo.txt remains.
	// Guard must surface foo.txt so the caller can abort.
	g := &fakeGuard{porcelainResponses: []string{
		" M .beads/issues.jsonl\n M foo.txt\n",
		" M foo.txt\n",
	}}
	g.install(t)

	userDirt, err := CheckDirtyTree("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(userDirt, []string{"foo.txt"}) {
		t.Errorf("userDirt = %v, want [foo.txt]", userDirt)
	}
	if !g.exportCalled {
		t.Error("export should still run when artifact is among dirty paths")
	}
}

func TestCheckDirtyTree_OnlyUserDirt_SkipsExport(t *testing.T) {
	// No artifact in dirty set → no reason to run bd export (avoids an extra
	// subprocess when user dirt is going to block anyway).
	g := &fakeGuard{porcelainResponses: []string{" M foo.txt\n"}}
	g.install(t)

	userDirt, err := CheckDirtyTree("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(userDirt, []string{"foo.txt"}) {
		t.Errorf("userDirt = %v", userDirt)
	}
	if g.exportCalled {
		t.Error("export should NOT run when only user dirt is present")
	}
	if g.porcelainCalls != 1 {
		t.Errorf("porcelain calls = %d, want 1 (no re-check needed)", g.porcelainCalls)
	}
}

func TestCheckDirtyTree_ArtifactDirtSurvivesExport(t *testing.T) {
	// Legitimate Dolt changes: bd export doesn't clean the diff (the JSONL
	// really has changed). The guard still proceeds — the artifact is the
	// bead's own first-commit payload (ADR-0025).
	g := &fakeGuard{porcelainResponses: []string{
		" M .beads/issues.jsonl\n",
		" M .beads/issues.jsonl\n",
	}}
	g.install(t)

	userDirt, err := CheckDirtyTree("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(userDirt) != 0 {
		t.Errorf("userDirt = %v, want empty (still proceed on legitimate artifact diff)", userDirt)
	}
}

func TestCheckDirtyTree_ExportFailurePropagates(t *testing.T) {
	g := &fakeGuard{
		porcelainResponses: []string{" M .beads/issues.jsonl\n"},
		exportErr:          fmt.Errorf("bd export boom"),
	}
	g.install(t)

	_, err := CheckDirtyTree("/repo", "/repo")
	if err == nil {
		t.Fatal("expected error when bd export fails")
	}
}

func TestCheckDirtyTree_PorcelainFailurePropagates(t *testing.T) {
	g := &fakeGuard{porcelainErr: fmt.Errorf("git status boom")}
	g.install(t)

	_, err := CheckDirtyTree("/repo", "/repo")
	if err == nil {
		t.Fatal("expected error when git status fails")
	}
}

// --- CheckDirtyTreeDetail (spec 092 Reqs 6/7, mindspec-i4ad) ---

func TestCheckDirtyTreeDetail_ResidualArtifactDirtExposed(t *testing.T) {
	// The artifact diff survives bd export (legitimate Dolt change, or a
	// pre-commit hook re-export). Detail exposes the residual so
	// `mindspec complete` can fold it into a follow-up commit.
	g := &fakeGuard{porcelainResponses: []string{
		" M .beads/issues.jsonl\n",
		" M .beads/issues.jsonl\n",
	}}
	g.install(t)

	artifactDirt, userDirt, err := CheckDirtyTreeDetail("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(artifactDirt, []string{".beads/issues.jsonl"}) {
		t.Errorf("artifactDirt = %v, want [.beads/issues.jsonl]", artifactDirt)
	}
	if len(userDirt) != 0 {
		t.Errorf("userDirt = %v, want empty", userDirt)
	}
}

func TestCheckDirtyTreeDetail_NormalizedArtifactDirtIsEmpty(t *testing.T) {
	// Stale throttled export: bd export normalizes the diff away. No
	// residual artifact dirt — the caller has nothing to commit.
	g := &fakeGuard{porcelainResponses: []string{
		" M .beads/issues.jsonl\n",
		"",
	}}
	g.install(t)

	artifactDirt, userDirt, err := CheckDirtyTreeDetail("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(artifactDirt) != 0 {
		t.Errorf("artifactDirt = %v, want empty after normalization", artifactDirt)
	}
	if len(userDirt) != 0 {
		t.Errorf("userDirt = %v, want empty", userDirt)
	}
}

func TestCheckDirtyTreeDetail_MixedDirtReturnsBoth(t *testing.T) {
	g := &fakeGuard{porcelainResponses: []string{
		" M .beads/issues.jsonl\n M foo.txt\n",
		" M .beads/issues.jsonl\n M foo.txt\n",
	}}
	g.install(t)

	artifactDirt, userDirt, err := CheckDirtyTreeDetail("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(artifactDirt, []string{".beads/issues.jsonl"}) {
		t.Errorf("artifactDirt = %v", artifactDirt)
	}
	if !reflect.DeepEqual(userDirt, []string{"foo.txt"}) {
		t.Errorf("userDirt = %v", userDirt)
	}
}

func TestCheckDirtyTreeDetail_UserDirtOnly_NoArtifactNoExport(t *testing.T) {
	g := &fakeGuard{porcelainResponses: []string{" M foo.txt\n"}}
	g.install(t)

	artifactDirt, userDirt, err := CheckDirtyTreeDetail("/repo", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(artifactDirt) != 0 {
		t.Errorf("artifactDirt = %v, want empty", artifactDirt)
	}
	if !reflect.DeepEqual(userDirt, []string{"foo.txt"}) {
		t.Errorf("userDirt = %v", userDirt)
	}
	if g.exportCalled {
		t.Error("export should NOT run when no artifact path is dirty")
	}
}

// TestCheckDirtyTreeDetail_SecondPorcelainFailurePropagates covers the
// post-export re-snapshot error branch (the SECOND statusPorcelainFn
// call, after `bd export` normalized the artifact) — Bead 9 punch-list
// B11. The first snapshot succeeds with artifact dirt; the re-snapshot
// fails and must propagate, after the export ran.
func TestCheckDirtyTreeDetail_SecondPorcelainFailurePropagates(t *testing.T) {
	origStatus := statusPorcelainFn
	origExport := exportBeadsFn
	t.Cleanup(func() {
		statusPorcelainFn = origStatus
		exportBeadsFn = origExport
	})

	calls := 0
	statusPorcelainFn = func(cwd string) (string, error) {
		calls++
		if calls == 1 {
			return " M .beads/issues.jsonl\n", nil
		}
		return "", fmt.Errorf("git status boom after export")
	}
	exported := false
	exportBeadsFn = func(workdir string) error { exported = true; return nil }

	_, _, err := CheckDirtyTreeDetail("/repo", "/repo")
	if err == nil {
		t.Fatal("expected the post-export status failure to propagate")
	}
	if !strings.Contains(err.Error(), "git status boom after export") {
		t.Errorf("error should carry the second status failure, got: %v", err)
	}
	if !exported {
		t.Error("bd export must have run before the failing re-snapshot")
	}
	if calls != 2 {
		t.Errorf("statusPorcelainFn calls = %d, want 2", calls)
	}
}

// --- DirtyTreeFailure (spec 092 Reqs 8/12, mindspec-tjat) ---
//
// Per-site recovery-convention tests (Req 21 mirror — see
// internal/guard/recovery_convention_test.go): every produced failure
// satisfies guard.HasFinalRecoveryLine, carries the Req 8
// worktree-context line as the last body line, and never advises
// stash/restore/checkout — main's pre-seeded dirt must survive the
// wrong_directory_guard_recovery scenario untouched.

func assertDirtyTreeFailureInvariants(t *testing.T, msg string) {
	t.Helper()
	if !guard.HasFinalRecoveryLine(msg) {
		t.Errorf("dirty-tree failure must end with a recovery line (Req 12/21): %q", msg)
	}
	if strings.Contains(msg, "Recovery steps:") {
		t.Errorf("dirty-tree failure still contains the pre-092 \"Recovery steps:\" block: %q", msg)
	}
	for _, banned := range []string{"git stash", "git restore", "git checkout"} {
		if strings.Contains(msg, banned) {
			t.Errorf("dirty-tree failure advises %q — destructive over dirt the agent did not author: %q", banned, msg)
		}
	}
	// Req 12 ordering: the context line is the last body line,
	// immediately preceding the (single) final recovery line.
	lines := strings.Split(msg, "\n")
	if len(lines) < 2 {
		t.Fatalf("dirty-tree failure unexpectedly short: %q", msg)
	}
	if !strings.HasPrefix(lines[len(lines)-2], "you are in the ") {
		t.Errorf("context line must immediately precede the final recovery line, got %q", lines[len(lines)-2])
	}
}

func TestDirtyTreeFailure_SteersToActiveWorktree(t *testing.T) {
	t.Parallel()
	cwd := "/repo"
	wt := "/repo/.worktrees/worktree-spec-001-wrongdir"
	err := DirtyTreeFailure(cwd, []string{"notes.txt"}, wt)
	if err == nil {
		t.Fatal("DirtyTreeFailure returned nil")
	}
	msg := err.Error()
	assertDirtyTreeFailureInvariants(t, msg)

	if !strings.Contains(msg, "notes.txt") {
		t.Errorf("failure must name the dirty path: %q", msg)
	}
	// tjat AC: `you are in the <kind> worktree` naming the evaluated path.
	wantCtx := "you are in the main worktree (/repo); this check evaluated /repo"
	if !strings.Contains(msg, wantCtx) {
		t.Errorf("failure missing context line %q: %q", wantCtx, msg)
	}
	wantRecovery := "recovery: cd " + wt + " && mindspec next"
	lines := strings.Split(msg, "\n")
	if got := lines[len(lines)-1]; got != wantRecovery {
		t.Errorf("final recovery line = %q, want %q", got, wantRecovery)
	}
}

func TestDirtyTreeFailure_NoActiveWorktree_CommitAdvice(t *testing.T) {
	t.Parallel()
	err := DirtyTreeFailure("/repo", []string{"a.go", "b.go"}, "")
	if err == nil {
		t.Fatal("DirtyTreeFailure returned nil")
	}
	msg := err.Error()
	assertDirtyTreeFailureInvariants(t, msg)
	if !strings.Contains(msg, "a.go") || !strings.Contains(msg, "b.go") {
		t.Errorf("failure must name every dirty path: %q", msg)
	}
	last := msg[strings.LastIndex(msg, "\n")+1:]
	if !strings.HasPrefix(last, guard.RecoveryPrefix) || !strings.Contains(last, "mindspec next") {
		t.Errorf("commit-advice recovery must end with a re-run of mindspec next: %q", last)
	}
	if !strings.Contains(last, "git add -A && git commit") {
		t.Errorf("commit-advice recovery must offer a non-destructive commit: %q", last)
	}
}

func TestDirtyTreeFailure_InsideActiveWorktree_CommitAdvice(t *testing.T) {
	t.Parallel()
	wt := "/repo/.worktrees/worktree-mindspec-abc1"
	// cwd inside the active worktree: the dirt is the agent's own —
	// no steer, commit advice instead.
	err := DirtyTreeFailure(wt+"/internal", []string{"x.go"}, wt)
	if err == nil {
		t.Fatal("DirtyTreeFailure returned nil")
	}
	msg := err.Error()
	assertDirtyTreeFailureInvariants(t, msg)
	if strings.Contains(msg, "recovery: cd ") {
		t.Errorf("must not steer when already inside the active worktree: %q", msg)
	}
	wantCtx := "you are in the bead worktree (" + wt + "/internal); this check evaluated " + wt + "/internal"
	if !strings.Contains(msg, wantCtx) {
		t.Errorf("failure missing bead-kind context line %q: %q", wantCtx, msg)
	}
}

func TestPathWithin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		dir, root string
		want      bool
	}{
		{"/repo", "/repo", true},
		{"/repo/sub", "/repo", true},
		{"/repo", "/repo/sub", false},
		{"/repo-sibling", "/repo", false},
		{"/elsewhere", "/repo", false},
	}
	for _, tc := range cases {
		if got := pathWithin(tc.dir, tc.root); got != tc.want {
			t.Errorf("pathWithin(%q, %q) = %v, want %v", tc.dir, tc.root, got, tc.want)
		}
	}
}
