package executor

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

// --- Test fakes ---

// fakeWorktreeOps records every call and serves canned responses. It satisfies
// WorktreeOps so orchestration tests can run without `bd` on PATH.
type fakeWorktreeOps struct {
	mu          sync.Mutex
	listEntries []bead.WorktreeListEntry
	listErr     error
	createErr   error
	removeErr   error

	createCalls []createCall
	removeCalls []string
	listCalls   int

	// onRemove and onCreate fire when called (used to capture CWD or to
	// reify side effects the real `bd worktree` would have).
	onCreate func(name, branch string)
	onRemove func(name string)
}

type createCall struct {
	Name   string
	Branch string
}

func (f *fakeWorktreeOps) Create(name, branch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls = append(f.createCalls, createCall{Name: name, Branch: branch})
	if f.onCreate != nil {
		f.onCreate(name, branch)
	}
	return f.createErr
}

func (f *fakeWorktreeOps) Remove(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removeCalls = append(f.removeCalls, name)
	if f.onRemove != nil {
		f.onRemove(name)
	}
	return f.removeErr
}

func (f *fakeWorktreeOps) List() ([]bead.WorktreeListEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++
	return f.listEntries, f.listErr
}

// newTempRepo creates a fresh `git init -b main` repo with one empty commit
// and a `.beads/` directory. Returns the path. Skips the test if git isn't
// available — matches the pattern in TestCommitAll_ExportedJSONLLandsInCommit.
func newTempRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	runGitIn(t, dir, "init", "-b", "main")
	runGitIn(t, dir, "config", "user.email", "test@example.com")
	runGitIn(t, dir, "config", "user.name", "test")
	runGitIn(t, dir, "commit", "--allow-empty", "-m", "root")
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	return dir
}

// chdirToRepo chdirs into dir for the duration of the test. Required because
// most gitutil functions (BranchExists, CreateBranch, DeleteBranch,
// CommitCount, DiffStat, HasRemote, PushBranch) operate on the current
// working directory rather than taking a workdir argument. The executor's
// production callers run from the repo root, which is the invariant these
// tests need to recreate.
func chdirToRepo(t *testing.T, dir string) {
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

// newRepoExecutor wires a MindspecExecutor against a real temp git repo, with
// a recording fake WorktreeOps. This is the standard scaffold for tests that
// exercise the executor end-to-end without requiring `bd` on PATH.
//
// chdirs into the repo because gitutil's BranchExists/CreateBranch/DeleteBranch
// operate on $PWD. Production callers invoke the executor from the main repo
// root; the test must reproduce that invariant.
func newRepoExecutor(t *testing.T) (*MindspecExecutor, *fakeWorktreeOps, string) {
	t.Helper()
	dir := newTempRepo(t)
	chdirToRepo(t, dir)
	fake := &fakeWorktreeOps{}
	g := &MindspecExecutor{
		Root:        dir,
		WorktreeOps: fake,
	}
	return g, fake, dir
}

// --- Interface compliance ---

func TestMindspecExecutorImplementsExecutor(t *testing.T) {
	var _ Executor = (*MindspecExecutor)(nil)
}

func TestMockExecutorImplementsExecutor(t *testing.T) {
	var _ Executor = (*MockExecutor)(nil)
}

func TestNewMindspecExecutor_DefaultsWorktreeOps(t *testing.T) {
	g := NewMindspecExecutor("/tmp")
	if g.WorktreeOps == nil {
		t.Fatal("WorktreeOps should default to defaultWorktreeOps, not nil")
	}
	if _, ok := g.WorktreeOps.(defaultWorktreeOps); !ok {
		t.Errorf("WorktreeOps default type = %T, want defaultWorktreeOps", g.WorktreeOps)
	}
}

// --- InitSpecWorkspace ---

func TestInitSpecWorkspace_CreatesBranchAndWorktree(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	info, err := g.InitSpecWorkspace("077-my-feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !branchExistsIn(t, dir, "spec/077-my-feature") {
		t.Errorf("branch spec/077-my-feature should exist in repo")
	}

	if len(fake.createCalls) != 1 {
		t.Fatalf("WorktreeOps.Create calls = %d, want 1", len(fake.createCalls))
	}
	c := fake.createCalls[0]
	if c.Branch != "spec/077-my-feature" {
		t.Errorf("worktree branch = %q, want %q", c.Branch, "spec/077-my-feature")
	}
	if c.Name != ".worktrees/worktree-spec-077-my-feature" {
		t.Errorf("worktree path = %q, want %q", c.Name, ".worktrees/worktree-spec-077-my-feature")
	}
	if info.Branch != "spec/077-my-feature" {
		t.Errorf("info.Branch = %q, want %q", info.Branch, "spec/077-my-feature")
	}
}

func TestInitSpecWorkspace_SkipsBranchIfExists(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	// Pre-create the branch; record its hash so we can verify it's not
	// recreated.
	runGitIn(t, dir, "branch", "spec/077-my-feature")
	headBefore := refHash(t, dir, "spec/077-my-feature")

	if _, err := g.InitSpecWorkspace("077-my-feature"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	headAfter := refHash(t, dir, "spec/077-my-feature")
	if headBefore != headAfter {
		t.Errorf("branch should not be recreated; hash changed %s → %s", headBefore, headAfter)
	}
	if len(fake.createCalls) != 1 {
		t.Errorf("WorktreeOps.Create should still be called once, got %d", len(fake.createCalls))
	}
}

// --- InitSpecWorkspace branch base (Spec 101 Bead 4, R4 k9a8/#76) ---

// TestInitSpecWorkspace_BranchesFromOriginDetectedDefault proves that when a
// remote exists the spec branch is created from `origin/<detected-default>`
// AFTER a fetch — and that the default branch is DETECTED, not hardcoded
// `main`: the origin's default branch is `develop`, and a commit that lives
// ONLY on origin/develop (never on the local HEAD) must be the spec branch's
// base. This is the structural proof of "from origin, not from local HEAD".
func TestInitSpecWorkspace_BranchesFromOriginDetectedDefault(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	// Build a bare "origin" whose default branch is develop, with a commit
	// that the local clone does NOT yet have.
	origin := t.TempDir()
	runGitIn(t, origin, "init", "--bare", "-b", "develop")
	// Seed origin/develop via a scratch working clone.
	seed := t.TempDir()
	runGitIn(t, seed, "clone", origin, ".")
	runGitIn(t, seed, "config", "user.email", "test@example.com")
	runGitIn(t, seed, "config", "user.name", "test")
	runGitIn(t, seed, "commit", "--allow-empty", "-m", "origin develop tip")
	runGitIn(t, seed, "push", "origin", "develop")
	originDevelopSHA := refHash(t, seed, "develop")

	// Wire origin onto the local repo. Do NOT fetch yet — InitSpecWorkspace
	// must perform the fetch itself.
	runGitIn(t, dir, "remote", "add", "origin", origin)

	if _, err := g.InitSpecWorkspace("077-my-feature"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !branchExistsIn(t, dir, "spec/077-my-feature") {
		t.Fatalf("spec branch should exist")
	}
	got := refHash(t, dir, "spec/077-my-feature")
	if got != originDevelopSHA {
		t.Errorf("spec branch base = %s, want origin/develop tip %s\n"+
			"(branch must be created from origin/<detected-default> after a fetch, not local HEAD)",
			got, originDevelopSHA)
	}
	// Defense against a hardcoded-main coincidence: local HEAD (main) differs
	// from origin/develop, so equality above can only hold via real detection.
	if got == refHash(t, dir, "main") {
		t.Errorf("spec branch base equals local main HEAD — detection/fetch did not take effect")
	}
}

// TestInitSpecWorkspace_NoRemoteFallsBackToHEADWithWarn proves that with NO
// remote configured (HasRemote == false) the branch is created from local
// HEAD and a WARN naming the stale-base risk is emitted — never a hard
// failure.
func TestInitSpecWorkspace_NoRemoteFallsBackToHEADWithWarn(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	var stderr string
	captured := captureStderrAround(t, func() {
		if _, err := g.InitSpecWorkspace("077-my-feature"); err != nil {
			t.Fatalf("offline/no-remote must NOT hard-fail: %v", err)
		}
	})
	stderr = captured

	if !branchExistsIn(t, dir, "spec/077-my-feature") {
		t.Fatalf("spec branch should exist")
	}
	if refHash(t, dir, "spec/077-my-feature") != refHash(t, dir, "main") {
		t.Errorf("with no remote the spec branch must fall back to local HEAD (main)")
	}
	if !strings.Contains(strings.ToUpper(stderr), "WARN") {
		t.Errorf("expected a WARN on the no-remote fallback, stderr = %q", stderr)
	}
}

// TestInitSpecWorkspace_FetchErrorFallsBackToHEADWithWarn proves a remote that
// exists but is unreachable (bad URL → fetch fails) funnels to the local-HEAD
// + WARN fallback, never a hard `spec create` failure.
func TestInitSpecWorkspace_FetchErrorFallsBackToHEADWithWarn(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	// A remote pointing at a non-existent path makes `git fetch` fail.
	runGitIn(t, dir, "remote", "add", "origin", filepath.Join(t.TempDir(), "does-not-exist"))

	stderr := captureStderrAround(t, func() {
		if _, err := g.InitSpecWorkspace("077-my-feature"); err != nil {
			t.Fatalf("unreachable remote must NOT hard-fail: %v", err)
		}
	})

	if !branchExistsIn(t, dir, "spec/077-my-feature") {
		t.Fatalf("spec branch should exist")
	}
	if refHash(t, dir, "spec/077-my-feature") != refHash(t, dir, "main") {
		t.Errorf("on fetch failure the spec branch must fall back to local HEAD (main)")
	}
	if !strings.Contains(strings.ToUpper(stderr), "WARN") {
		t.Errorf("expected a WARN on the fetch-failure fallback, stderr = %q", stderr)
	}
}

// --- HandoffEpic ---

func TestHandoffEpic_IsNoOp(t *testing.T) {
	g, _, _ := newRepoExecutor(t)
	if err := g.HandoffEpic("epic-1", "077-test", []string{"bead-a", "bead-b"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- DispatchBead ---

func TestDispatchBead_CreatesBeadBranch(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	// Prereq: spec branch exists so baseBranch resolves.
	runGitIn(t, dir, "branch", "spec/077-my-feature")

	info, err := g.DispatchBead("mindspec-abc.1", "077-my-feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !branchExistsIn(t, dir, "bead/mindspec-abc.1") {
		t.Errorf("bead branch should exist in repo")
	}
	// The bead branch must point at the same commit as the spec branch (its base).
	if refHash(t, dir, "bead/mindspec-abc.1") != refHash(t, dir, "spec/077-my-feature") {
		t.Errorf("bead branch should be created from spec branch")
	}
	if info.Branch != "bead/mindspec-abc.1" {
		t.Errorf("info.Branch = %q, want %q", info.Branch, "bead/mindspec-abc.1")
	}
	if len(fake.createCalls) != 1 {
		t.Errorf("WorktreeOps.Create calls = %d, want 1", len(fake.createCalls))
	}
}

func TestDispatchBead_ReusesExistingWorktree(t *testing.T) {
	g, fake, _ := newRepoExecutor(t)

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-abc.1",
		Path:   "/repo/.worktrees/worktree-mindspec-abc.1",
		Branch: "bead/mindspec-abc.1",
	}}

	info, err := g.DispatchBead("mindspec-abc.1", "077-my-feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.createCalls) != 0 {
		t.Errorf("should not create worktree when one already exists, got %d Create calls", len(fake.createCalls))
	}
	if info.Path != "/repo/.worktrees/worktree-mindspec-abc.1" {
		t.Errorf("info.Path = %q, want existing worktree path", info.Path)
	}
}

func TestDispatchBead_FallsBackToHEAD(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	if _, err := g.DispatchBead("mindspec-abc.1", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Without a specID, bead branch is created from HEAD (== main, the only commit).
	if refHash(t, dir, "bead/mindspec-abc.1") != refHash(t, dir, "main") {
		t.Errorf("bead branch should be created from HEAD when specID is empty")
	}
}

// --- CompleteBead ---

func TestCompleteBead_MergesAndCleans(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	// Spec branch must exist; create it pointing at main, then build a bead
	// branch off it with one commit so the merge is non-trivial.
	runGitIn(t, dir, "branch", "spec/077-test")
	runGitIn(t, dir, "branch", "bead/mindspec-x.1")

	// Add a commit on bead branch via worktree.
	beadWtDir := filepath.Join(dir, ".wt-bead-x1")
	runGitIn(t, dir, "worktree", "add", beadWtDir, "bead/mindspec-x.1")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run() })
	if err := os.WriteFile(filepath.Join(beadWtDir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitIn(t, beadWtDir, "add", ".")
	runGitIn(t, beadWtDir, "commit", "-m", "bead work")

	// Create spec worktree so MergeInto can actually merge.
	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-077-test")
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", specWtPath, "spec/077-test")
	t.Cleanup(func() { _ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", specWtPath).Run() })

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtDir,
		Branch: "bead/mindspec-x.1",
	}}
	// When the fake's Remove is called, actually remove the git worktree so
	// the subsequent `git branch -D` (called by the executor's
	// gitutil.DeleteBranch) doesn't fail with "branch in use by worktree".
	// This is what `bd worktree remove` does in production.
	fake.onRemove = func(name string) {
		_ = exec.Command("git", "-C", dir, "worktree", "remove", "--force", beadWtDir).Run()
	}

	// Snapshot bead's hash before Complete; CompleteBead deletes the branch.
	beadHashBefore := refHash(t, dir, "bead/mindspec-x.1")
	mainHash := refHash(t, dir, "main")

	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify spec branch was advanced past main (merge succeeded).
	specHashAfter := refHash(t, dir, "spec/077-test")
	if specHashAfter == mainHash {
		t.Errorf("spec/077-test should have advanced past main (got merge); still at %s", specHashAfter)
	}
	// And the bead commit must be reachable from the new spec tip.
	if !isAncestorIn(t, dir, beadHashBefore, "spec/077-test") {
		t.Errorf("bead commit %s should be an ancestor of spec/077-test (%s) after merge",
			beadHashBefore, specHashAfter)
	}
	// Verify cleanup: WorktreeOps.Remove called with the bead's worktree name.
	if len(fake.removeCalls) != 1 || fake.removeCalls[0] != "worktree-mindspec-x.1" {
		t.Errorf("removeCalls = %v, want [worktree-mindspec-x.1]", fake.removeCalls)
	}
	// Bead branch should be deleted by the executor's gitutil.DeleteBranch call.
	if branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Errorf("bead branch should be deleted")
	}
}

// TestCompleteBeadRejectsMalformedSpecBranch is spec 120 AC-23
// (internal/executor): CompleteBead with a spec/-prefixed branch whose
// suffix fails idvalidate.SpecID refuses before any merge/worktree
// operation — the branch is an explicit CompleteBead argument (an
// agent-writable string, not necessarily waist-composed), and this is an
// EXPLICIT verb, so it must refuse rather than silently compose a hostile
// spec-worktree path.
func TestCompleteBeadRejectsMalformedSpecBranch(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	runGitIn(t, dir, "branch", "bead/mindspec-x.1")

	err := g.CompleteBead("mindspec-x.1", "spec/x;evil", "")
	if err == nil {
		t.Fatal("expected a refusal for a spec branch whose suffix fails idvalidate.SpecID")
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("expected NO worktree removal, got %v", fake.removeCalls)
	}
	if !branchExistsIn(t, dir, "bead/mindspec-x.1") {
		t.Error("bead branch must be preserved on the refusal (no cleanup attempted)")
	}
}

func TestCompleteBead_WorktreeRemoveRunsFromRepoRoot(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	// Spec + bead branches.
	runGitIn(t, dir, "branch", "spec/077-test")
	runGitIn(t, dir, "branch", "bead/mindspec-x.1")

	beadWtPath := filepath.Join(dir, ".worktrees", "worktree-mindspec-x.1")
	if err := os.MkdirAll(beadWtPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	fake.listEntries = []bead.WorktreeListEntry{{
		Name:   "worktree-mindspec-x.1",
		Path:   beadWtPath,
		Branch: "bead/mindspec-x.1",
	}}

	// Capture CWD at the moment Remove is called.
	var cwdDuringRemove string
	fake.onRemove = func(name string) {
		wd, _ := os.Getwd()
		cwdDuringRemove = wd
	}

	// Start from inside the bead worktree directory.
	origWd, _ := os.Getwd()
	if err := os.Chdir(beadWtPath); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origWd)

	// No msg → no auto-commit/IsTreeClean path; merge path will warn (spec wt
	// missing) but the cleanup path still runs.
	if err := g.CompleteBead("mindspec-x.1", "spec/077-test", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// CWD during Remove must be repo root, not inside the worktree.
	realCwd, _ := filepath.EvalSymlinks(cwdDuringRemove)
	realRoot, _ := filepath.EvalSymlinks(dir)
	if realCwd != realRoot {
		t.Errorf("CWD during Remove = %q, want repo root %q", cwdDuringRemove, dir)
	}
}

// --- FinalizeEpic ---

func TestFinalizeEpic_DirectMerge(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	// Set up spec branch with one commit ahead of main.
	runGitIn(t, dir, "checkout", "-b", "spec/077-test")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "spec change")
	runGitIn(t, dir, "checkout", "main")

	fake.listEntries = nil // no bead worktrees

	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeStrategy != "direct" {
		t.Errorf("strategy = %q, want %q", result.MergeStrategy, "direct")
	}
	if result.CommitCount != 1 {
		t.Errorf("commits = %d, want 1", result.CommitCount)
	}
	if !strings.Contains(result.DiffStat, "a.txt") {
		t.Errorf("diffstat = %q, want to include a.txt", result.DiffStat)
	}
}

func TestFinalizeEpic_BranchNotFound(t *testing.T) {
	g, _, _ := newRepoExecutor(t)

	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %q, want 'does not exist'", err.Error())
	}
}

// --- Cleanup ---

func TestCleanup_RemovesWorktreeAndBranch(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	runGitIn(t, dir, "branch", "spec/077-test")

	if err := g.Cleanup("077-test", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fake.removeCalls) != 1 || fake.removeCalls[0] != "worktree-spec-077-test" {
		t.Errorf("removeCalls = %v, want [worktree-spec-077-test]", fake.removeCalls)
	}
	if branchExistsIn(t, dir, "spec/077-test") {
		t.Errorf("spec branch should be deleted")
	}
}

// --- IsTreeClean ---

func TestIsTreeClean_Clean(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	if err := g.IsTreeClean(dir); err != nil {
		t.Errorf("expected clean tree, got: %v", err)
	}
}

func TestIsTreeClean_Dirty(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := g.IsTreeClean(dir)
	if err == nil {
		t.Fatal("expected error for dirty tree")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("error = %q", err.Error())
	}
}

// --- DiffStat / CommitCount ---

func TestDiffStat_AgainstRealRepo(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	runGitIn(t, dir, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "add x")

	stat, err := g.DiffStat("main", "feature")
	if err != nil {
		t.Fatalf("DiffStat: %v", err)
	}
	if !strings.Contains(stat, "x.txt") {
		t.Errorf("DiffStat = %q, want to include x.txt", stat)
	}
}

func TestCommitCount_AgainstRealRepo(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	runGitIn(t, dir, "checkout", "-b", "feature")
	for _, name := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(name), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		runGitIn(t, dir, "add", ".")
		runGitIn(t, dir, "commit", "-m", name)
	}

	count, err := g.CommitCount("main", "feature")
	if err != nil {
		t.Fatalf("CommitCount: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

// --- CommitAll / Export ordering ---
//
// TestCommitAll_ExportedJSONLLandsInCommit verifies the AC (ADR-0025, spec
// 082): every mindspec-driven commit refreshes .beads/issues.jsonl from Dolt
// before staging, so the committed blob matches Dolt at commit time. After the
// ARCH-11 refactor `bead.Export` is called directly (no DI), so this test
// gates on `bd` being on PATH and asserts that what `bd export` actually
// wrote ends up in the committed tree.
func TestCommitAll_ExportedJSONLLandsInCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not available; export step shells out to bd")
	}

	dir := newTempRepo(t)
	g := &MindspecExecutor{Root: dir, WorktreeOps: defaultWorktreeOps{}}

	// Seed .beads/issues.jsonl so the staged blob has known content if bd has
	// nothing to export from a Dolt store. The AC we assert here is that
	// .beads/issues.jsonl lands in the commit — its exact contents depend on
	// whether bd reaches a Dolt server.
	sentinel := `{"id":"sentinel","title":"pre-commit marker"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte(sentinel), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	err := g.CommitAll(dir, "chore: commit with export")
	if err != nil {
		// bd may fail without a Dolt-initialized .beads store; the
		// ordering-invariant test below covers the failure path. Skip when
		// bd can't export from a virgin temp repo.
		t.Skipf("CommitAll requires a bd-initialized .beads store: %v", err)
	}

	statCmd := exec.Command("git", "-C", dir, "show", "--stat", "--format=", "HEAD")
	statOut, err := statCmd.Output()
	if err != nil {
		t.Fatalf("git show --stat: %v", err)
	}
	if !strings.Contains(string(statOut), ".beads/issues.jsonl") {
		t.Errorf("git show --stat did not include .beads/issues.jsonl:\n%s", string(statOut))
	}
}

// TestCommitAll_ExportFailureAbortsCommit verifies the ordering invariant
// (export runs before git add/commit). If `bd` is missing from PATH,
// `bead.Export` returns an error and CommitAll must abort *without* creating
// a commit. This catches regressions where a future refactor reorders the
// two steps.
func TestCommitAll_ExportFailureAbortsCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if _, err := exec.LookPath("bd"); err == nil {
		t.Skip("bd is on PATH; this test requires bd to be missing so Export fails")
	}

	dir := newTempRepo(t)
	g := &MindspecExecutor{Root: dir, WorktreeOps: defaultWorktreeOps{}}

	// Make tree dirty so a commit would otherwise be created.
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	headBefore := refHash(t, dir, "HEAD")
	err := g.CommitAll(dir, "msg")
	if err == nil {
		t.Fatal("expected error when bd is missing (Export should fail)")
	}
	if !strings.Contains(err.Error(), "refreshing .beads/issues.jsonl") {
		t.Errorf("error = %q, want export-related wrap", err.Error())
	}
	headAfter := refHash(t, dir, "HEAD")
	if headBefore != headAfter {
		t.Errorf("HEAD should not advance when export fails; %s → %s", headBefore, headAfter)
	}
}

// --- MockExecutor ---

func TestMockExecutor_RecordsCalls(t *testing.T) {
	m := &MockExecutor{
		InitSpecWorkspaceResult: WorkspaceInfo{Path: "/ws", Branch: "spec/001"},
		DispatchBeadResult:      WorkspaceInfo{Path: "/bead", Branch: "bead/x.1"},
		FinalizeEpicResult:      FinalizeResult{MergeStrategy: "direct", CommitCount: 3},
		ChangedFilesResult:      []string{"a.txt", "b.txt"},
		FileAtRefResult:         []byte("file contents"),
		MergeBaseResult:         "abc123",
	}

	info, _ := m.InitSpecWorkspace("001-test")
	if info.Path != "/ws" {
		t.Errorf("path = %q", info.Path)
	}

	_ = m.HandoffEpic("e1", "001-test", []string{"b1", "b2"})
	bInfo, _ := m.DispatchBead("x.1", "001-test")
	if bInfo.Branch != "bead/x.1" {
		t.Errorf("branch = %q", bInfo.Branch)
	}

	_ = m.CompleteBead("x.1", "spec/001-test", "done")
	fr, _ := m.FinalizeEpic("e1", "001-test", "spec/001-test", nil)
	if fr.CommitCount != 3 {
		t.Errorf("commits = %d", fr.CommitCount)
	}

	_ = m.Cleanup("001-test", false)
	_ = m.IsTreeClean("/path")
	_, _ = m.DiffStat("main", "spec/001")
	_, _ = m.CommitCount("main", "spec/001")
	_ = m.CommitAll("/path", "msg")

	files, _ := m.ChangedFiles("main", "feature")
	if len(files) != 2 || files[0] != "a.txt" {
		t.Errorf("ChangedFiles = %v, want [a.txt b.txt]", files)
	}
	content, _ := m.FileAtRef("HEAD", "a.txt")
	if string(content) != "file contents" {
		t.Errorf("FileAtRef = %q, want %q", string(content), "file contents")
	}
	mb, _ := m.MergeBase("main", "feature")
	if mb != "abc123" {
		t.Errorf("MergeBase = %q, want %q", mb, "abc123")
	}

	if calls := m.CallsTo("InitSpecWorkspace"); len(calls) != 1 {
		t.Errorf("InitSpecWorkspace calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("HandoffEpic"); len(calls) != 1 {
		t.Errorf("HandoffEpic calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("DispatchBead"); len(calls) != 1 {
		t.Errorf("DispatchBead calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("CompleteBead"); len(calls) != 1 {
		t.Errorf("CompleteBead calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("FinalizeEpic"); len(calls) != 1 {
		t.Errorf("FinalizeEpic calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("Cleanup"); len(calls) != 1 {
		t.Errorf("Cleanup calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("ChangedFiles"); len(calls) != 1 {
		t.Errorf("ChangedFiles calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("FileAtRef"); len(calls) != 1 {
		t.Errorf("FileAtRef calls = %d, want 1", len(calls))
	}
	if calls := m.CallsTo("MergeBase"); len(calls) != 1 {
		t.Errorf("MergeBase calls = %d, want 1", len(calls))
	}
	if total := len(m.Calls); total != 13 {
		t.Errorf("total calls = %d, want 13", total)
	}
}

// --- ChangedFiles / FileAtRef / MergeBase (production impl) ---

func TestChangedFiles_TwoRefs(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	runGitIn(t, dir, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "beta.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "add alpha/beta")

	files, err := g.ChangedFiles("main", "feature")
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %v, want 2 entries", files)
	}
	seen := map[string]bool{}
	for _, f := range files {
		seen[f] = true
	}
	if !seen["alpha.txt"] || !seen["beta.txt"] {
		t.Errorf("files = %v, want both alpha.txt and beta.txt", files)
	}
}

func TestChangedFiles_EmptyBase_WorkingTreeVsHead(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	// Make a committed change on a side branch.
	runGitIn(t, dir, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "in-feature.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "feature commit")
	// Back to main: the working tree differs from "feature" by the feature commit.
	runGitIn(t, dir, "checkout", "main")

	files, err := g.ChangedFiles("", "feature")
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}
	// The working tree (main) lacks in-feature.txt; diff against feature lists it.
	found := false
	for _, f := range files {
		if f == "in-feature.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("files = %v, want to include in-feature.txt", files)
	}
}

func TestFileAtRef_ReturnsBlobBytes(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	want := []byte("hello from ref\n")
	if err := os.WriteFile(filepath.Join(dir, "greet.txt"), want, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "add greet")

	got, err := g.FileAtRef("HEAD", "greet.txt")
	if err != nil {
		t.Fatalf("FileAtRef: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("FileAtRef = %q, want %q", string(got), string(want))
	}
}

func TestFileAtRef_MissingPath_Errors(t *testing.T) {
	g, _, _ := newRepoExecutor(t)

	if _, err := g.FileAtRef("HEAD", "does-not-exist.txt"); err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestMergeBase_ReturnsCommonAncestor(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	// main has the root commit. Branch off and add a commit on each side.
	mainRoot := refHash(t, dir, "main")

	runGitIn(t, dir, "checkout", "-b", "left")
	if err := os.WriteFile(filepath.Join(dir, "l.txt"), []byte("l"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "left commit")

	runGitIn(t, dir, "checkout", "main")
	runGitIn(t, dir, "checkout", "-b", "right")
	if err := os.WriteFile(filepath.Join(dir, "r.txt"), []byte("r"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "right commit")

	got, err := g.MergeBase("left", "right")
	if err != nil {
		t.Fatalf("MergeBase: %v", err)
	}
	if got != mainRoot {
		t.Errorf("MergeBase = %q, want root %q", got, mainRoot)
	}
}

func TestMergeBase_TrimsWhitespace(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	got, err := g.MergeBase("HEAD", "HEAD")
	if err != nil {
		t.Fatalf("MergeBase: %v", err)
	}
	if strings.TrimSpace(got) != got {
		t.Errorf("MergeBase returned untrimmed output: %q", got)
	}
	if got != refHash(t, dir, "HEAD") {
		t.Errorf("MergeBase(HEAD,HEAD) = %q, want HEAD %q", got, refHash(t, dir, "HEAD"))
	}
}

// --- SEC-5 option-injection guard at the executor's direct git-exec sites ---
//
// internal/complete overwrites beadHead with the branch reported by
// `bd worktree list` (a name it trusts) and that ref flows into the
// executor's OWN direct git exec — MergeBase / FileAtRef / pathExistsAtRef /
// TreeDirsAtRef / ChangedFiles / BlobExistsAtRef — which bypass
// internal/gitutil's boundary guard. A ref literally named `-x` / `--upload-pack=…` would be reparsed by
// git as an option (verified: `git update-ref refs/heads/-x HEAD` succeeds,
// `git merge-base main -x` reparses `-x`). These tests pin that each site
// REJECTS a `-`-prefixed operand with the SEC-5 guard error BEFORE git is
// invoked. They are RED-on-revert: removing a gitutil.RejectOptionLike call
// hands `-x` to git, which fails with a *git* error (no "SEC-5" sentinel) —
// or, for ls-tree against a valid ref, may not fail at all.
//
// The guard error carries the distinctive "SEC-5" / "looks like an option"
// sentinel that no git failure produces, so asserting on it proves the guard
// (not git) produced the rejection — i.e. the operand never reached exec.

// assertSEC5 fails unless err is the boundary guard's option-injection
// rejection (and therefore was produced before any git exec).
func assertSEC5(t *testing.T, err error, site string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected SEC-5 rejection for a '-'-prefixed ref, got nil error", site)
	}
	if !strings.Contains(err.Error(), "SEC-5") || !strings.Contains(err.Error(), "looks like an option") {
		t.Fatalf("%s: error = %q, want the SEC-5 guard rejection (proves git was never invoked)", site, err.Error())
	}
}

func TestExecutorRefSites_RejectOptionLikeRef(t *testing.T) {
	g, _, _ := newRepoExecutor(t)

	const hostile = "-x"

	t.Run("MergeBase/firstRef", func(t *testing.T) {
		_, err := g.MergeBase(hostile, "main")
		assertSEC5(t, err, "MergeBase(a)")
	})
	t.Run("MergeBase/secondRef", func(t *testing.T) {
		_, err := g.MergeBase("main", hostile)
		assertSEC5(t, err, "MergeBase(b)")
	})
	t.Run("FileAtRef", func(t *testing.T) {
		_, err := g.FileAtRef(hostile, "README.md")
		assertSEC5(t, err, "FileAtRef")
	})
	t.Run("FileAtRefOrAbsent", func(t *testing.T) {
		// Flows through pathExistsAtRef first.
		_, _, err := g.FileAtRefOrAbsent(hostile, "README.md")
		assertSEC5(t, err, "FileAtRefOrAbsent->pathExistsAtRef")
	})
	t.Run("pathExistsAtRef", func(t *testing.T) {
		_, err := g.pathExistsAtRef(hostile, "README.md")
		assertSEC5(t, err, "pathExistsAtRef")
	})
	t.Run("TreeDirsAtRef", func(t *testing.T) {
		_, err := g.TreeDirsAtRef(hostile, "docs")
		assertSEC5(t, err, "TreeDirsAtRef")
	})
	t.Run("ChangedFiles/base", func(t *testing.T) {
		_, err := g.ChangedFiles(hostile, "main")
		assertSEC5(t, err, "ChangedFiles(base)")
	})
	t.Run("ChangedFiles/head", func(t *testing.T) {
		_, err := g.ChangedFiles("main", hostile)
		assertSEC5(t, err, "ChangedFiles(head)")
	})
	t.Run("BlobExistsAtRef", func(t *testing.T) {
		_, err := g.BlobExistsAtRef(hostile, "README.md")
		assertSEC5(t, err, "BlobExistsAtRef")
	})
	t.Run("UploadPackVector", func(t *testing.T) {
		// The canonical RCE-shaped operand from the finding.
		_, err := g.MergeBase("--upload-pack=touch /tmp/pwned", "main")
		assertSEC5(t, err, "MergeBase(--upload-pack)")
	})
}

// TestExecutorRefSites_ControlledRefsStillWork confirms the guard does not
// regress legitimate refs (main, spec/<id>, bead/<id>, HEAD) at the five
// ls-tree/show/merge-base sites.
func TestExecutorRefSites_ControlledRefsStillWork(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	// Seed a tracked file and a sub-directory so the ls-tree sites have
	// something to enumerate, plus branches named like real mindspec refs.
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "x.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "top.txt"), []byte("t"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "seed docs and top")
	runGitIn(t, dir, "branch", "spec/077-x")
	runGitIn(t, dir, "branch", "bead/mindspec-x.1")

	for _, ref := range []string{"main", "HEAD", "spec/077-x", "bead/mindspec-x.1"} {
		if _, err := g.FileAtRef(ref, "top.txt"); err != nil {
			t.Errorf("FileAtRef(%q): %v", ref, err)
		}
		present, err := g.pathExistsAtRef(ref, "top.txt")
		if err != nil || !present {
			t.Errorf("pathExistsAtRef(%q) = (%v, %v), want (true, nil)", ref, present, err)
		}
		dirs, err := g.TreeDirsAtRef(ref, ".")
		if err != nil {
			t.Errorf("TreeDirsAtRef(%q): %v", ref, err)
		}
		foundDocs := false
		for _, d := range dirs {
			if d == "docs" {
				foundDocs = true
			}
		}
		if !foundDocs {
			t.Errorf("TreeDirsAtRef(%q) = %v, want to include docs", ref, dirs)
		}
		if _, err := g.MergeBase(ref, "main"); err != nil {
			t.Errorf("MergeBase(%q, main): %v", ref, err)
		}
		if blobOK, err := g.BlobExistsAtRef(ref, "top.txt"); err != nil || !blobOK {
			t.Errorf("BlobExistsAtRef(%q, top.txt) = (%v, %v), want (true, nil)", ref, blobOK, err)
		}
	}
	// ChangedFiles between two controlled refs.
	if _, err := g.ChangedFiles("main", "spec/077-x"); err != nil {
		t.Errorf("ChangedFiles(main, spec/077-x): %v", err)
	}
}

// --- Real-git helpers ---

func runGitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git -C %s %v: %s", dir, args, strings.TrimSpace(string(out)))
	}
}

func branchExistsIn(t *testing.T, dir, name string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--verify", "refs/heads/"+name)
	return cmd.Run() == nil
}

func refHash(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

func isAncestorIn(t *testing.T, dir, anc, desc string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "merge-base", "--is-ancestor", anc, desc)
	return cmd.Run() == nil
}

// --- Spec 092 Bead 4: withWorkingDir cwd safety (Req 3a, mindspec-qxsy) ---

// captureStderrAround swaps os.Stderr for a pipe while fn runs and
// returns whatever fn wrote to it.
func captureStderrAround(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = origStderr
	stderrBytes, _ := io.ReadAll(r)
	return string(stderrBytes)
}

// TestWithWorkingDir_RemovedCwdRemainsAtDirSilently covers the hardened
// restore path for the EXPECTED case (panel R2-2): fn deletes the
// directory the process was invoked from — e.g. FinalizeEpic removing
// the spec worktree — so the deferred restore cannot chdir back. The
// process must remain at dir (never an undefined cwd) and stay SILENT:
// the removal was the operation's own doing, not a failure.
func TestWithWorkingDir_RemovedCwdRemainsAtDirSilently(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "target")
	doomed := filepath.Join(base, "doomed")
	for _, d := range []string{dir, doomed} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	origWd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origWd) })
	if err := os.Chdir(doomed); err != nil {
		t.Fatalf("chdir doomed: %v", err)
	}

	var wwdErr error
	stderr := captureStderrAround(t, func() {
		wwdErr = withWorkingDir(dir, func() error {
			// fn removes the invocation directory — the deferred restore
			// will fail in the EXPECTED way.
			return os.RemoveAll(doomed)
		})
	})

	if wwdErr != nil {
		t.Fatalf("withWorkingDir returned error: %v", wwdErr)
	}

	// The process remains at dir — never an undefined cwd.
	wd, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatalf("process left in unresolvable cwd: %v", wdErr)
	}
	realWd, _ := filepath.EvalSymlinks(wd)
	realDir, _ := filepath.EvalSymlinks(dir)
	if realWd != realDir {
		t.Errorf("cwd after restore failure: got %q, want dir %q", wd, dir)
	}

	// Expected removal → no cwd_restore_failed noise (it would fire on
	// every successful impl approve run from the spec worktree).
	if strings.Contains(stderr, "event=executor.cwd_restore_failed") {
		t.Errorf("cwd_restore_failed emitted for an expected removal; stderr: %q", stderr)
	}
}

// TestWithWorkingDir_GenuineRestoreFailureWarns covers the other half of
// panel R2-2: the original cwd still EXISTS but cannot be re-entered
// (execute permission revoked while fn ran). That is a genuine restore
// failure — the process remains at dir and the structured
// `event=executor.cwd_restore_failed dir=<dir>` warning is emitted.
func TestWithWorkingDir_GenuineRestoreFailureWarns(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod-based chdir denial does not apply")
	}

	base := t.TempDir()
	dir := filepath.Join(base, "target")
	locked := filepath.Join(base, "locked")
	for _, d := range []string{dir, locked} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	// Restore permissions so TempDir cleanup succeeds.
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	origWd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origWd) })
	if err := os.Chdir(locked); err != nil {
		t.Fatalf("chdir locked: %v", err)
	}

	var wwdErr error
	stderr := captureStderrAround(t, func() {
		wwdErr = withWorkingDir(dir, func() error {
			// Revoke search permission on the original cwd: it still
			// exists (stat succeeds) but chdir back fails with EACCES.
			return os.Chmod(locked, 0o000)
		})
	})

	if wwdErr != nil {
		t.Fatalf("withWorkingDir returned error: %v", wwdErr)
	}

	// The process remains at dir — never an undefined cwd.
	wd, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatalf("process left in unresolvable cwd: %v", wdErr)
	}
	realWd, _ := filepath.EvalSymlinks(wd)
	realDir, _ := filepath.EvalSymlinks(dir)
	if realWd != realDir {
		t.Errorf("cwd after restore failure: got %q, want dir %q", wd, dir)
	}

	want := "event=executor.cwd_restore_failed dir=" + dir
	if !strings.Contains(stderr, want) {
		t.Errorf("stderr should contain %q; got: %q", want, stderr)
	}
}

// TestWithWorkingDir_RestoresOriginalCwd pins the unchanged happy path:
// when the original cwd survives fn, it is restored and no
// cwd_restore_failed warning is emitted.
func TestWithWorkingDir_RestoresOriginalCwd(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "target")
	home := filepath.Join(base, "home")
	for _, d := range []string{dir, home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	origWd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origWd) })
	if err := os.Chdir(home); err != nil {
		t.Fatalf("chdir home: %v", err)
	}

	var cwdInFn string
	if err := withWorkingDir(dir, func() error {
		cwdInFn, _ = os.Getwd()
		return nil
	}); err != nil {
		t.Fatalf("withWorkingDir: %v", err)
	}

	realInFn, _ := filepath.EvalSymlinks(cwdInFn)
	realDir, _ := filepath.EvalSymlinks(dir)
	if realInFn != realDir {
		t.Errorf("cwd inside fn: got %q, want %q", cwdInFn, dir)
	}

	wd, _ := os.Getwd()
	realWd, _ := filepath.EvalSymlinks(wd)
	realHome, _ := filepath.EvalSymlinks(home)
	if realWd != realHome {
		t.Errorf("cwd after return: got %q, want restored %q", wd, home)
	}
}
