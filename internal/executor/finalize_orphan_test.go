package executor

// Bug mindspec-wu7t (P1) — protected-main finalize regression fixtures.
//
// On a branch-protected main, `mindspec impl approve <spec>` cannot commit
// the epic-close/finalize JSONL directly to main: FinalizeEpic's "pr" path
// auto-commits the refreshed .beads/issues.jsonl export onto the SPEC
// branch and pushes it for a PR. If the implementation PR was ALREADY
// merged (the common case in practice) and the spec branch is later
// deleted as post-merge debris, that finalize commit never reaches main —
// main's committed .beads/issues.jsonl stays stale, and the bd post-merge
// hook silently reverts the epic-close/bead-done state in Dolt on every
// subsequent merge/FF (observed live on spec 106).
//
// The fix (pinned by the tests here): FinalizeEpic pushes the spec branch
// first (baseline contract), then detects the already-merged case (the
// spec branch's pre-finalize tip is already an ancestor of origin/main)
// and reroutes the finalize commit onto a fresh chore/finalize-<specID>
// branch created from origin/main. The chore flow is retry-idempotent
// (panel round 1): leftover temp worktrees are pruned, the stale local
// branch is recreated, and an already-pushed remote branch is overwritten
// via a lease pinned to its observed tip.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// stubBeadExport swaps commitWithExport's bead-export step (the bug wu7t
// execBeadExportFn seam) for a deterministic file write, so finalize
// commits require neither `bd` on PATH nor a live Dolt store. Returns a
// pointer to the content slice so retry-style tests can change what the
// "export" produces between runs. The MkdirAll matters: the from-main
// temporary worktree in the orphaned case is checked out from origin/main,
// whose tree never tracked .beads/ in this synthetic fixture.
func stubBeadExport(t *testing.T) *[]byte {
	t.Helper()
	content := []byte(`{"id":"epic-1","status":"closed"}` + "\n")
	orig := execBeadExportFn
	execBeadExportFn = func(workdir string) error {
		if err := os.MkdirAll(filepath.Join(workdir, ".beads"), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(workdir, ".beads", "issues.jsonl"), content, 0o644)
	}
	t.Cleanup(func() { execBeadExportFn = orig })
	return &content
}

// setupOrphanOrigin wires a bare "origin" remote onto the repo at dir with
// main pushed, and creates spec/077-test carrying one commit. When merged
// is true, the spec branch is additionally merged into main locally and on
// origin BEFORE any FinalizeEpic run — the git-fixture equivalent of "the
// implementation PR already merged; the spec branch is now dead debris".
// Ends checked out on main. Returns the bare origin's path.
func setupOrphanOrigin(t *testing.T, dir string, merged bool) string {
	t.Helper()
	origin := t.TempDir()
	runGitIn(t, origin, "init", "--bare", "-b", "main")
	runGitIn(t, dir, "remote", "add", "origin", origin)
	runGitIn(t, dir, "push", "-u", "origin", "main")

	runGitIn(t, dir, "checkout", "-b", "spec/077-test")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "spec change")
	runGitIn(t, dir, "checkout", "main")

	if merged {
		runGitIn(t, dir, "merge", "--no-ff", "-m", "Merge spec/077-test", "spec/077-test")
		runGitIn(t, dir, "push", "origin", "main")
	}
	return origin
}

// TestFinalizeEpic_OrphanedSpecBranch is the core bug wu7t regression test.
//
// Table case 1 ("not yet merged") pins that the normal flow is completely
// unchanged: only the spec branch is pushed, FinalizeResult.FinalizeBranch
// stays empty.
//
// Table case 2 ("already merged") asserts the epic-close JSONL export
// commit is NOT stranded on the dead spec branch: a chore/finalize-<specID>
// branch must exist on the REMOTE, its tip must be a descendant of
// origin/main, and it must carry the export change.
func TestFinalizeEpic_OrphanedSpecBranch(t *testing.T) {
	tests := []struct {
		name               string
		alreadyMerged      bool
		wantFinalizeBranch bool
	}{
		{
			name:               "not yet merged: behaves exactly as today",
			alreadyMerged:      false,
			wantFinalizeBranch: false,
		},
		{
			name:               "already merged: finalize export lands on a from-main branch",
			alreadyMerged:      true,
			wantFinalizeBranch: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, fake, dir := newRepoExecutor(t)
			exportContent := stubBeadExport(t)
			origin := setupOrphanOrigin(t, dir, tc.alreadyMerged)

			fake.listEntries = nil // no bead worktrees

			// FinalizeEpic's cleanup unconditionally DELETES the local
			// spec branch after pushing (existing, unchanged behavior —
			// only the remote copy matters once a PR/finalize-branch is in
			// flight), so capture its pre-finalize tip now for the
			// still-pushed assertion below.
			specTipBefore := refHash(t, dir, "spec/077-test")

			result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.MergeStrategy != "pr" {
				t.Fatalf("MergeStrategy = %q, want %q", result.MergeStrategy, "pr")
			}

			if tc.wantFinalizeBranch {
				wantBranch := "chore/finalize-077-test"
				if result.FinalizeBranch != wantBranch {
					t.Fatalf("FinalizeBranch = %q, want %q", result.FinalizeBranch, wantBranch)
				}

				// The branch must exist on the REMOTE — the whole point is
				// that the epic-close commit reaches a carrier a reviewer
				// can actually open a PR against and merge.
				if !branchExistsIn(t, origin, wantBranch) {
					t.Fatalf("%s must exist on the remote", wantBranch)
				}
				remoteTip := refHash(t, origin, wantBranch)

				// Its tip must be a DESCENDANT of origin/main (created
				// from origin/main, not carried over from the dead spec
				// branch).
				if !isAncestorIn(t, origin, "main", wantBranch) {
					t.Errorf("%s must be a descendant of origin/main", wantBranch)
				}

				// It must contain the export change.
				out, showErr := exec.Command("git", "-C", origin, "show", remoteTip+":.beads/issues.jsonl").Output()
				if showErr != nil {
					t.Fatalf("reading .beads/issues.jsonl from %s: %v", wantBranch, showErr)
				}
				if string(out) != string(*exportContent) {
					t.Errorf(".beads/issues.jsonl on %s = %q, want %q", wantBranch, out, *exportContent)
				}

				// The temporary worktree used to build the commit must
				// have been cleaned up — it must not leak into `mindspec
				// doctor` or `bd worktree list`.
				wtPath := filepath.Join(dir, ".worktrees", "worktree-finalize-077-test")
				if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
					t.Errorf("temporary finalize worktree %s should have been removed", wtPath)
				}
			} else {
				if result.FinalizeBranch != "" {
					t.Errorf("FinalizeBranch = %q, want empty (spec branch not yet merged)", result.FinalizeBranch)
				}
				if branchExistsIn(t, origin, "chore/finalize-077-test") {
					t.Errorf("no chore/finalize branch should exist on origin when the spec branch was not yet merged")
				}
			}

			// The spec branch itself is ALWAYS still pushed — unchanged
			// contract in both cases. (The local branch is deleted by
			// FinalizeEpic's existing cleanup, so compare the remote tip
			// against the pre-finalize snapshot rather than the now-gone
			// local ref.)
			if !branchExistsIn(t, origin, "spec/077-test") {
				t.Fatalf("spec branch must still be pushed to origin")
			}
			if specTipRemote := refHash(t, origin, "spec/077-test"); specTipRemote != specTipBefore {
				t.Errorf("spec branch must still be pushed to origin: pre-finalize=%s remote=%s", specTipBefore, specTipRemote)
			}
		})
	}
}

// TestFinalizeEpic_OrphanedSpecBranch_RetryConverges pins panel round-1
// Group 1 (R4's empirical repro): a FIRST run that pushes
// chore/finalize-<specID> and then fails in a later FinalizeEpic step (here:
// the spec-worktree removal, injected via the fake's removeErr) must not
// poison the retry. Pre-fix, the retry recreated the branch fresh from
// origin/main and the plain push was rejected non-fast-forward, hard-failing
// `impl approve` until manual intervention. The retry must succeed and the
// REMOTE chore tip must be replaced by the retry's fresh export commit.
func TestFinalizeEpic_OrphanedSpecBranch_RetryConverges(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	exportContent := stubBeadExport(t)
	origin := setupOrphanOrigin(t, dir, true)

	fake.listEntries = nil
	// Run 1: fail AFTER the spec-branch and chore-branch pushes — the
	// spec-worktree removal in FinalizeEpic's cleanup block errors.
	fake.removeErr = errors.New("simulated crash: spec worktree removal failed")

	if _, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil); err == nil {
		t.Fatal("run 1 must fail (injected worktree-removal error), got nil")
	}
	if !branchExistsIn(t, origin, "chore/finalize-077-test") {
		t.Fatal("run 1 must have pushed chore/finalize-077-test before failing")
	}
	run1Tip := refHash(t, origin, "chore/finalize-077-test")

	// Run 2 (the retry): the injected failure is gone; the export content
	// changed in between (Dolt state moved), so the retry's commit differs
	// from run 1's — the exact shape that made a plain push non-FF.
	fake.removeErr = nil
	*exportContent = []byte(`{"id":"epic-1","status":"closed","retry":2}` + "\n")

	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err != nil {
		t.Fatalf("retry must succeed, got: %v", err)
	}
	if result.FinalizeBranch != "chore/finalize-077-test" {
		t.Fatalf("retry FinalizeBranch = %q, want chore/finalize-077-test", result.FinalizeBranch)
	}

	run2Tip := refHash(t, origin, "chore/finalize-077-test")
	if run2Tip == run1Tip {
		t.Error("the retry must replace the remote chore tip with its fresh export commit")
	}
	if !isAncestorIn(t, origin, "main", "chore/finalize-077-test") {
		t.Error("the retried chore branch must still be a descendant of origin/main")
	}
	out, showErr := exec.Command("git", "-C", origin, "show", run2Tip+":.beads/issues.jsonl").Output()
	if showErr != nil {
		t.Fatalf("reading .beads/issues.jsonl from retried tip: %v", showErr)
	}
	if string(out) != string(*exportContent) {
		t.Errorf("retried .beads/issues.jsonl = %q, want the run-2 export %q", out, *exportContent)
	}
}

// TestFinalizeEpic_OrphanedSpecBranch_LeftoverTempWorktreeSelfHeals pins the
// panel round-1 Group 1 prune: a CRASHED prior run can leave the temporary
// finalize worktree behind — still registered AND with the chore branch
// checked out in it, which pre-fix failed WorktreeAdd with a raw git error
// and additionally blocked the stale-branch delete (a checked-out branch
// cannot be deleted). The retry must self-heal: prune the leftover, rebuild
// from origin/main, and land the push.
func TestFinalizeEpic_OrphanedSpecBranch_LeftoverTempWorktreeSelfHeals(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	stubBeadExport(t)
	origin := setupOrphanOrigin(t, dir, true)
	fake.listEntries = nil

	// Simulate the crashed prior run: chore branch exists locally and is
	// checked out in a leftover temp worktree at the exact path the
	// production flow uses.
	runGitIn(t, dir, "branch", "chore/finalize-077-test", "main")
	wtPath := filepath.Join(dir, ".worktrees", "worktree-finalize-077-test")
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGitIn(t, dir, "worktree", "add", wtPath, "chore/finalize-077-test")

	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	if err != nil {
		t.Fatalf("a leftover temp finalize worktree must be self-healed, got: %v", err)
	}
	if result.FinalizeBranch != "chore/finalize-077-test" {
		t.Fatalf("FinalizeBranch = %q, want chore/finalize-077-test", result.FinalizeBranch)
	}
	if !branchExistsIn(t, origin, "chore/finalize-077-test") {
		t.Fatal("chore/finalize-077-test must reach the remote despite the leftover worktree")
	}
	if !isAncestorIn(t, origin, "main", "chore/finalize-077-test") {
		t.Error("the rebuilt chore branch must be a descendant of origin/main")
	}
	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Errorf("temporary finalize worktree %s should have been removed", wtPath)
	}
}

// TestFinalizeEpic_OrphanFetchFailureFallsBackToBaseline pins the fail-safe:
// when `git fetch origin main` fails (offline / auth / the remote has no
// main ref, as here), orphan detection is SKIPPED with a stderr warning and
// FinalizeEpic behaves exactly like the pre-wu7t baseline — spec branch
// pushed, no chore branch, FinalizeBranch empty — never a hard failure.
func TestFinalizeEpic_OrphanFetchFailureFallsBackToBaseline(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)
	stubBeadExport(t)

	// A bare origin with NO main ref: pushes of other branches succeed,
	// but `git fetch origin main` fails ("couldn't find remote ref").
	origin := t.TempDir()
	runGitIn(t, origin, "init", "--bare", "-b", "main")
	runGitIn(t, dir, "remote", "add", "origin", origin)
	runGitIn(t, dir, "checkout", "-b", "spec/077-test")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "spec change")
	runGitIn(t, dir, "checkout", "main")
	fake.listEntries = nil

	var result FinalizeResult
	var err error
	stderr := captureStderrAround(t, func() {
		result, err = g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	})
	if err != nil {
		t.Fatalf("a fetch failure must fall back to baseline behavior, got: %v", err)
	}
	if !strings.Contains(stderr, "could not fetch origin/main") {
		t.Errorf("expected the fetch-failure warning on stderr, got:\n%s", stderr)
	}
	if result.FinalizeBranch != "" {
		t.Errorf("FinalizeBranch = %q, want empty on the fetch-failure fallback", result.FinalizeBranch)
	}
	if branchExistsIn(t, origin, "chore/finalize-077-test") {
		t.Error("no chore/finalize branch may be created when detection was skipped")
	}
	if !branchExistsIn(t, origin, "spec/077-test") {
		t.Error("the baseline spec-branch push must still happen on the fallback path")
	}
}

// TestFinalizeEpic_OrphanChoreFailureStillPushesSpecBranch pins panel
// round-1 Group 3: the baseline spec-branch push runs FIRST,
// unconditionally, so a failure anywhere in the chore-branch flow (here:
// the export step inside finalizeOrphanedSpecBranch) surfaces as an error
// WITHOUT costing the operator the spec-branch push.
func TestFinalizeEpic_OrphanChoreFailureStillPushesSpecBranch(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	// An export stub that always fails: the early spec-artifacts
	// auto-commit downgrades it to a warning (existing behavior), but the
	// chore-flow commit hard-fails, erroring FinalizeEpic AFTER the
	// baseline push.
	origExport := execBeadExportFn
	execBeadExportFn = func(workdir string) error {
		return errors.New("simulated export failure")
	}
	t.Cleanup(func() { execBeadExportFn = origExport })

	origin := setupOrphanOrigin(t, dir, true)
	fake.listEntries = nil
	specTipBefore := refHash(t, dir, "spec/077-test")

	var err error
	_ = captureStderrAround(t, func() {
		_, err = g.FinalizeEpic("epic-1", "077-test", "spec/077-test", nil)
	})
	if err == nil {
		t.Fatal("a chore-flow failure must surface as an error, got nil")
	}
	if !strings.Contains(err.Error(), "chore/finalize-077-test") {
		t.Errorf("error should name the chore branch; got:\n%s", err.Error())
	}

	// The baseline push already happened — the spec branch is on origin
	// at its pre-finalize tip despite the chore-flow failure.
	if !branchExistsIn(t, origin, "spec/077-test") {
		t.Fatal("the baseline spec-branch push must precede the chore flow")
	}
	if got := refHash(t, origin, "spec/077-test"); got != specTipBefore {
		t.Errorf("origin spec tip = %s, want pre-finalize tip %s", got, specTipBefore)
	}
}
