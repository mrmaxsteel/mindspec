package executor

// Bug mindspec-wu7t (P1) — protected-main finalize regression fixture.
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
// TestFinalizeEpic_OrphanedSpecBranch pins the fix: FinalizeEpic detects
// the already-merged case (the spec branch's pre-finalize tip is already an
// ancestor of origin/main) and reroutes the finalize commit onto a fresh
// chore/finalize-<specID> branch created from origin/main, instead of
// relying on the dead spec branch.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestFinalizeEpic_OrphanedSpecBranch is the bug wu7t regression test.
//
// Table case 1 ("not yet merged") pins that the normal flow is completely
// unchanged: only the spec branch is pushed, FinalizeResult.FinalizeBranch
// stays empty.
//
// Table case 2 ("already merged") simulates the implementation PR having
// already landed the spec branch's bead work on origin/main BEFORE
// FinalizeEpic runs (git-fixture equivalent of "the impl PR was merged, the
// spec branch is now dead debris"). It asserts the epic-close JSONL export
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

			// Bug wu7t test seam: stub the bead-export step so the
			// finalize commit's content is deterministic and requires
			// neither `bd` on PATH nor a live Dolt store. MkdirAll first —
			// the from-main temporary worktree in the orphaned case is
			// checked out from origin/main, whose tree never tracked
			// .beads/ in this synthetic fixture (newTempRepo creates the
			// directory on disk but never commits it).
			exportContent := []byte(`{"id":"epic-1","status":"closed"}` + "\n")
			origExport := execBeadExportFn
			execBeadExportFn = func(workdir string) error {
				if err := os.MkdirAll(filepath.Join(workdir, ".beads"), 0o755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(workdir, ".beads", "issues.jsonl"), exportContent, 0o644)
			}
			t.Cleanup(func() { execBeadExportFn = origExport })

			// Bare "origin" wired to the local repo's current main tip —
			// the protected-main remote FinalizeEpic pushes/fetches
			// against.
			origin := t.TempDir()
			runGitIn(t, origin, "init", "--bare", "-b", "main")
			runGitIn(t, dir, "remote", "add", "origin", origin)
			runGitIn(t, dir, "push", "-u", "origin", "main")

			// Spec branch carrying the "bead work" commit.
			runGitIn(t, dir, "checkout", "-b", "spec/077-test")
			if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			runGitIn(t, dir, "add", ".")
			runGitIn(t, dir, "commit", "-m", "spec change")

			if tc.alreadyMerged {
				// Simulate the implementation PR having already merged:
				// the spec branch's bead work lands on main (and origin)
				// BEFORE FinalizeEpic ever runs.
				runGitIn(t, dir, "checkout", "main")
				runGitIn(t, dir, "merge", "--no-ff", "-m", "Merge spec/077-test", "spec/077-test")
				runGitIn(t, dir, "push", "origin", "main")
			} else {
				runGitIn(t, dir, "checkout", "main")
			}

			fake.listEntries = nil // no bead worktrees

			// FinalizeEpic's cleanup unconditionally DELETES the local
			// spec branch after pushing (existing, unchanged behavior —
			// only the remote copy matters once a PR/finalize-branch is in
			// flight), so capture its pre-finalize tip now for the
			// still-pushed assertion below.
			specTipBefore := refHash(t, dir, "spec/077-test")

			result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test")
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
				if string(out) != string(exportContent) {
					t.Errorf(".beads/issues.jsonl on %s = %q, want %q", wantBranch, out, exportContent)
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
