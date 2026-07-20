package complete

// Spec 119 Bead 6 (AC-26 / ADR-0041): the `complete` fault-injection matrix,
// mechanism-A (real-git decorator) leg — c2, c3, c5. See
// fault_injection_test.go's package doc comment for the full matrix
// overview and the mechanism-B (c1/c4/c7/c8) tests.
//
// Each test here wraps a REAL executor.MindspecExecutor over a real temp
// git repo with a decorator that lets the REAL git mutation land (a real
// `git commit`, or the REAL merge+cleanup inside CompleteBead) and THEN
// forces a terminal error — never a mock that only pretends to mutate.
// This is the spec's own "minimum three" (after tracker auto-commit /
// after `bd close` / after the merge) for c2/c3 and c5 respectively.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// noopWorktreeOps is a no-op executor.WorktreeOps fake for the real-git
// fault-injection fixtures: List reports no entries (the fixtures manage
// their own real linked worktrees directly via `git worktree add`, outside
// bd's bookkeeping), and Remove/Create are harmless no-ops so
// CompleteBead's own cleanup leg never shells out to a real `bd worktree`
// command.
type noopWorktreeOps struct{}

func (noopWorktreeOps) Create(name, branch string) error { return nil }
func (noopWorktreeOps) Remove(name string) error         { return nil }
func (noopWorktreeOps) List() ([]bead.WorktreeListEntry, error) {
	return nil, nil
}

// gitCommitAll runs `git add -A && git commit -q -m msg` for real in dir,
// tolerating "nothing to commit" (a no-op success) so a decorator method
// invoked when there is genuinely nothing new to stage does not spuriously
// fail the fixture.
func gitCommitAll(t *testing.T, dir, msg string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "add", "-A")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add -A in %s: %v\n%s", dir, err, out)
	}
	cmd = exec.Command("git", "-C", dir, "commit", "-q", "-m", msg)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "nothing to commit") {
		t.Fatalf("git commit in %s: %v\n%s", dir, err, out)
	}
}

// gitCommitPaths runs `git add <paths...> && git commit -q -m msg` for real.
func gitCommitPaths(t *testing.T, dir, msg string, paths []string) {
	t.Helper()
	args := append([]string{"-C", dir, "add"}, paths...)
	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add %v in %s: %v\n%s", paths, dir, err, out)
	}
	cmd = exec.Command("git", "-C", dir, "commit", "-q", "-m", msg)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "nothing to commit") {
		t.Fatalf("git commit in %s: %v\n%s", dir, err, out)
	}
}

// killAfterExecutor is the mechanism-A decorator (ADR-0041 §3): it wraps a
// REAL executor.MindspecExecutor and, for each of the three mutation seams
// below, performs the REAL git mutation FIRST and only THEN — when the
// corresponding kill* flag is set — forces a terminal error, faithfully
// modeling "the process died right after this mutation landed" rather than
// a mock that never mutates anything at all.
type killAfterExecutor struct {
	executor.Executor
	t *testing.T

	killCommitAll    bool
	killCommitPaths  bool
	killCompleteBead bool

	completeBeadCalls int
}

func (e *killAfterExecutor) CommitAll(dir, msg string) error {
	gitCommitAll(e.t, dir, msg)
	if e.killCommitAll {
		return fakeErr("fault-injection: kill after CommitAll's real commit landed")
	}
	return nil
}

func (e *killAfterExecutor) CommitPaths(dir, msg string, paths []string) error {
	gitCommitPaths(e.t, dir, msg, paths)
	if e.killCommitPaths {
		return fakeErr("fault-injection: kill after CommitPaths' real commit landed")
	}
	return nil
}

func (e *killAfterExecutor) CompleteBead(beadID, specBranch, msg string) error {
	e.completeBeadCalls++
	if err := e.Executor.CompleteBead(beadID, specBranch, msg); err != nil {
		return err
	}
	if e.killCompleteBead {
		return fakeErr("fault-injection: kill after CompleteBead's real merge+cleanup landed")
	}
	return nil
}

// setupRealGitFaultFixture builds a real temp git repo for the mechanism-A
// fault-injection tests: a base commit on main (spec.md/plan.md/ADR
// satisfying doc-sync + adr-divergence for domain "widget" with no
// override needed), a specBranch, a beadBranch carrying an OWNERSHIP.yaml +
// widget.go commit, and a REAL spec worktree directory at the conventional
// path (root/.worktrees/worktree-spec-<id>) — required for
// exec.CompleteBead's real merge (gitutil.MergeInto) to have somewhere to
// run at all; without it the merge leg is silently skipped and the
// ancestor safety check fails instead. Returns root, specBranch, and
// beadBranch; root ends checked out on specBranch (beadBranch is not
// checked out anywhere, so it can later be deleted for real).
func setupRealGitFaultFixture(t *testing.T, specID, beadID string) (root, specBranch, beadBranch string) {
	t.Helper()
	root = t.TempDir()
	gitRun(t, root, "init", "-q", "-b", "main")
	// CI runners have no user.name/user.email in any git config tier
	// (global/system) and no usable GECOS fallback, so the REAL merge
	// commits the production code makes here (gitutil.MergeInto inside
	// CompleteBead) would otherwise fail with "Please tell me who you
	// are" / "unable to auto-detect email address". Set repo-local
	// identity once so every worktree sharing this .git config (the
	// spec and bead worktrees added below) can commit for real —
	// mirrors internal/executor/executor_test.go's runGitIn setup.
	gitRun(t, root, "config", "user.email", "test@example.invalid")
	gitRun(t, root, "config", "user.name", "test")
	gitRun(t, root, "config", "commit.gpgsign", "false")
	writeFile(t, root, ".mindspec/docs/specs/"+specID+"/spec.md", "# Spec\n\n## Impacted Domains\n\n- widget\n")
	writeFile(t, root, ".mindspec/docs/specs/"+specID+"/plan.md",
		"---\nstatus: Approved\nspec_id: \""+specID+"\"\nversion: \"1\"\nadr_citations:\n  - ADR-9500\n---\n\n# Plan\n")
	writeFile(t, root, ".mindspec/docs/adr/ADR-9500.md",
		"# ADR-9500: Widget\n\n"+
			"- **Date**: 2026-01-01\n"+
			"- **Status**: Accepted\n"+
			"- **Domain(s)**: widget\n"+
			"- **Supersedes**: n/a\n"+
			"- **Superseded-by**: n/a\n\n"+
			"## Decision\nTest fixture.\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base")

	specBranch = "spec/" + specID
	beadBranch = "bead/" + beadID
	gitRun(t, root, "checkout", "-q", "-b", specBranch)
	gitRun(t, root, "checkout", "-q", "-b", beadBranch)
	writeFile(t, root, ".mindspec/docs/domains/widget/OWNERSHIP.yaml", "paths:\n  - internal/widget/**\n")
	writeFile(t, root, "internal/widget/widget.go", "package widget\n\nfunc New() {}\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "impl: widget + ownership claim")
	// root's own working copy returns to main — neither specBranch nor
	// beadBranch may be checked out there, since both get their own REAL
	// linked worktrees below (and beadBranch must later be deletable for
	// real, which git refuses for a branch checked out anywhere).
	gitRun(t, root, "checkout", "-q", "main")

	specWtPath := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
	if err := os.MkdirAll(filepath.Dir(specWtPath), 0o755); err != nil {
		t.Fatalf("mkdir worktrees dir: %v", err)
	}
	gitRun(t, root, "worktree", "add", specWtPath, specBranch)

	return root, specBranch, beadBranch
}

// wireRealGitFaultSeams stubs the complete-package lifecycle seams (NOT the
// executor) for the real-git fault-injection tests: an implement-mode epic,
// a `bd close` that always succeeds, and no live bd on PATH.
func wireRealGitFaultSeams(t *testing.T, specID string) {
	t.Helper()
	stubPhaseEpic(t, specID, "epic-"+specID)
	resolveTargetFn = func(r, flag string) (string, error) { return specID, nil }
	findLocalRootFn = func() (string, error) { return "", fakeErr("test: no local root") }
	closeBeadFn = func(...string) error { return nil }
}

// --- c2: `--commit-msg` tracker auto-commit --------------------------------
//
// exec.CommitAll (complete.go step 2.5, invoked only when --commit-msg is
// supplied) TERMINATES the run via `return nil, fmt.Errorf("auto-commit
// failed: %w", err)` when it fails. Mechanism A: the decorator performs the
// REAL commit in the bead worktree, then dies. Re-invocation (no
// --commit-msg needed — the tree is already clean) converges to
// completion: no panel is registered in this fixture, so the accepted
// convergence outcome here is completion rather than the panel-staleness
// refusal (both are AC-26-accepted; a registered-panel variant would hit
// the staleness Warn/Block instead, per the plan's illustrative case).
func TestFaultInjection_Complete_C2_CommitMsgAutoCommit_KillThenConverge(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "921-fic2", "mindspec-119fic2.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)

	beadWtPath := filepath.Join(root, ".worktrees", "worktree-"+beadID)
	gitRun(t, root, "worktree", "add", beadWtPath, beadBranch)
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{Name: "worktree-" + beadID, Path: beadWtPath, Branch: beadBranch}}, nil
	}

	// An uncommitted change in the bead worktree for --commit-msg to pick up.
	if err := os.WriteFile(filepath.Join(beadWtPath, "internal", "widget", "widget.go"),
		[]byte("package widget\n\nfunc New() {}\n\nfunc More() {}\n"), 0o644); err != nil {
		t.Fatalf("writing uncommitted change: %v", err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &killAfterExecutor{Executor: base, t: t, killCommitAll: true}

	// KILL: the real commit lands in the bead worktree, then the decorator
	// forces a terminal error.
	_, err := Run(root, beadID, specID, "did more work", ex, CompleteOpts{})
	if err == nil {
		t.Fatal("expected c2 kill: the auto-commit failure must fail Run")
	}
	if !strings.Contains(err.Error(), "auto-commit failed") {
		t.Errorf("expected an auto-commit-failed error, got: %v", err)
	}
	if ex.completeBeadCalls != 0 {
		t.Errorf("expected ZERO CompleteBead calls before the c2 kill, got %d", ex.completeBeadCalls)
	}
	// The real commit landed: the bead worktree's tree is clean and its
	// branch tip advanced past the pre-kill commit.
	if status := gitStatusPorcelain(t, beadWtPath); status != "" {
		t.Errorf("expected a clean bead worktree after the real commit landed, got dirty status:\n%s", status)
	}

	// Re-invoke: no --commit-msg needed (already committed); the kill flag
	// is cleared so the SAME decorator now lets the run proceed.
	ex.killCommitAll = false
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected c2 re-invocation to converge to completion, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed after convergence, got %+v", res)
	}
	if ex.completeBeadCalls != 1 {
		t.Errorf("expected exactly 1 CompleteBead call on convergence, got %d", ex.completeBeadCalls)
	}

	// The real merge landed on the spec branch (via the real CompleteBead).
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be a real ancestor of the spec branch after convergence")
	}
}

// --- c3: artifact-sync commit ---------------------------------------------
//
// exec.CommitPaths (complete.go step 3, the pathspec-scoped artifact-dirt
// follow-up commit) TERMINATES the run via `return nil, fmt.Errorf(
// "committing beads artifact sync: %w", err)` when it fails. Mechanism A:
// the SAME decorator, its CommitPaths method; the real commit of the
// exact artifact pathspec lands, then dies. Re-invocation finds a clean
// tree (the artifact dirt is gone) and converges to done.
func TestFaultInjection_Complete_C3_ArtifactSyncCommit_KillThenConverge(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "922-fic3", "mindspec-119fic3.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)

	beadWtPath := filepath.Join(root, ".worktrees", "worktree-"+beadID)
	gitRun(t, root, "worktree", "add", beadWtPath, beadBranch)
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{Name: "worktree-" + beadID, Path: beadWtPath, Branch: beadBranch}}, nil
	}

	// Artifact dirt: an untracked .beads/issues.jsonl the complete-package
	// seam (checkDirtyTreeFn) classifies as artifact-only, driving Run into
	// the CommitPaths follow-up leg without needing a real `bd export`.
	checkDirtyTreeFn = func(repoRoot, cwd string) (artifactDirt, userDirt []string, err error) {
		return []string{".beads/issues.jsonl"}, nil, nil
	}
	if err := os.MkdirAll(filepath.Join(beadWtPath, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadWtPath, ".beads", "issues.jsonl"), []byte(`{"id":"x"}`+"\n"), 0o644); err != nil {
		t.Fatalf("writing artifact dirt: %v", err)
	}

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &killAfterExecutor{Executor: base, t: t, killCommitPaths: true}

	// KILL: the real artifact-sync commit lands, then the decorator forces
	// a terminal error.
	_, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err == nil {
		t.Fatal("expected c3 kill: the artifact-sync commit failure must fail Run")
	}
	if !strings.Contains(err.Error(), "committing beads artifact sync") {
		t.Errorf("expected an artifact-sync-commit error, got: %v", err)
	}
	if ex.completeBeadCalls != 0 {
		t.Errorf("expected ZERO CompleteBead calls before the c3 kill, got %d", ex.completeBeadCalls)
	}
	if status := gitStatusPorcelain(t, beadWtPath); status != "" {
		t.Errorf("expected a clean bead worktree after the real artifact-sync commit landed, got dirty status:\n%s", status)
	}

	// Re-invoke: the tree is already clean (no more artifact dirt to sync);
	// convergence to completion.
	checkDirtyTreeFn = func(repoRoot, cwd string) (artifactDirt, userDirt []string, err error) {
		return nil, nil, nil
	}
	ex.killCommitPaths = false
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected c3 re-invocation to converge to completion, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed after convergence, got %+v", res)
	}
	if ex.completeBeadCalls != 1 {
		t.Errorf("expected exactly 1 CompleteBead call on convergence, got %d", ex.completeBeadCalls)
	}
	if !isAncestorIn(t, root, beadBranch, specBranch) {
		t.Error("expected the bead branch to be a real ancestor of the spec branch after convergence")
	}
}

// --- c5: bead→spec merge ---------------------------------------------------
//
// exec.CompleteBead (complete.go step 5, the terminal bead→spec merge) is a
// hard failure per spec 092 — its error propagates as the "CLOSED in Dolt
// but completion did NOT finish" recoverable refusal. Mechanism A: the
// decorator lets the REAL embedded CompleteBead run to completion (the
// real --no-ff merge, the real branch deletion) and then overrides its nil
// return with a terminal error, faithfully modeling "the process died right
// after the merge+cleanup landed". Re-invocation converges through Bead 1's
// merged-unclosed reconcile — this is also AC-5's end-to-end kill proof.
//
// Spec 121 R5(a)/(b): this fixture's real merge-time landed-binding write
// never actually fires (gitutil.BranchExists — the safety check gating it
// — reads the CALLING PROCESS's cwd, not root, a pre-existing property
// unrelated to this test's fixture directory), so the re-invocation's
// identification needs SOME other admissible datum once the branch is
// really gone. A registered panel (scanned from the filesystem, never
// via bd) stands in for that datum here, mirroring
// TestRun_Reconcile_RealPanel_MissingRefWarnCloses's convention — its
// reviewed_head_sha is pinned to the bead's OWN pre-merge tip so it is
// fresh (not stale) for the FIRST Run call's ordinary panel gate too.
func TestFaultInjection_Complete_C5_BeadToSpecMerge_KillThenConverge(t *testing.T) {
	saveAndRestore(t)
	const specID, beadID = "925-fic5", "mindspec-119fic5.1"
	root, specBranch, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)

	beadTipSHA := gateRevParse(t, root, beadBranch)
	writePanel(t, root, specID+"-"+beadID, panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 2,
		ReviewedHeadSHA: beadTipSHA,
	}, map[string]string{
		"a-round-1.json": "APPROVE",
		"b-round-1.json": "APPROVE",
	})

	// No bead worktree registered — complete.Run resolves the canonical
	// bead/<id> ref directly (matching the shipped no-worktree-yet shape a
	// real merge candidate can be in).
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	base := &executor.MindspecExecutor{Root: root, WorktreeOps: noopWorktreeOps{}}
	ex := &killAfterExecutor{Executor: base, t: t, killCompleteBead: true}

	// KILL: the real --no-ff merge + branch/worktree cleanup lands inside
	// the embedded CompleteBead, then the decorator overrides its nil
	// return with a terminal error.
	_, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err == nil {
		t.Fatal("expected c5 kill: the post-merge failure must fail Run")
	}
	if !strings.Contains(err.Error(), "CLOSED in Dolt") {
		t.Errorf("expected the closed-but-unmerged recoverable message, got: %v", err)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Errorf("expected a final recovery line, got: %v", err)
	}
	if ex.completeBeadCalls != 1 {
		t.Fatalf("expected exactly 1 CompleteBead call, got %d", ex.completeBeadCalls)
	}
	// The real merge landed for real: the spec branch now contains the
	// bead's file, and the bead branch is genuinely gone.
	if !fileExistsAtRef(t, root, specBranch, "internal/widget/widget.go") {
		t.Error("expected the real merge to have landed the bead's file on the spec branch")
	}
	if branchExistsIn(t, root, beadBranch) {
		t.Error("expected the real cleanup to have deleted the bead branch")
	}

	// Re-invoke: no worktree, no bead/<id> ref (really gone) — Bead 1's
	// merged-unclosed reconcile detects the REAL landed merge commit via
	// lifecycle.MergedUnclosed and converges without any further merge.
	mergedUnclosedFn = lifecycle.MergedUnclosed
	var reconcileSHA string
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		if v, ok := updates["mindspec_reconcile_landed_merge_sha"]; ok {
			reconcileSHA, _ = v.(string)
		}
		return nil
	}

	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err != nil {
		t.Fatalf("expected c5 re-invocation to converge via the merged-unclosed reconcile, got: %v", err)
	}
	if res == nil || !res.BeadClosed {
		t.Fatalf("expected BeadClosed after convergence, got %+v", res)
	}
	if ex.completeBeadCalls != 1 {
		t.Errorf("expected NO additional CompleteBead call on the reconcile path, got %d total", ex.completeBeadCalls)
	}
	if reconcileSHA == "" {
		t.Error("expected the reconcile evidence to name the landed merge commit SHA")
	}
}

// --- test helpers -----------------------------------------------------------

func gitStatusPorcelain(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status in %s: %v\n%s", dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func branchExistsIn(t *testing.T, dir, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--verify", "refs/heads/"+branch)
	return cmd.Run() == nil
}

func isAncestorIn(t *testing.T, dir, ancestor, descendant string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "merge-base", "--is-ancestor", ancestor, descendant)
	return cmd.Run() == nil
}

func fileExistsAtRef(t *testing.T, dir, ref, path string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "cat-file", "-e", ref+":"+path)
	return cmd.Run() == nil
}
