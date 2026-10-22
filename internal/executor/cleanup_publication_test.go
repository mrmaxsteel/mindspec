package executor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

func installCleanupHook(t *testing.T, root, kind, pending string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "cleanup-hook-fired")
	t.Setenv("CLEANUP_HOOK_OUTPUT", pending)
	t.Setenv("CLEANUP_HOOK_MARKER", marker)
	hook := "#!/bin/sh\nif [ ! -f \"$CLEANUP_HOOK_MARKER\" ]; then\n printf 'cleanup hook output\\n' > \"$CLEANUP_HOOK_OUTPUT\" || exit 1\n printf fired > \"$CLEANUP_HOOK_MARKER\" || exit 1\nfi\n"
	if err := os.WriteFile(filepath.Join(root, ".git/hooks", kind), []byte(hook), 0755); err != nil {
		t.Fatal(err)
	}
	return marker
}

func TestCompleteBead_MergeHookPreservesWorkAndRetry(t *testing.T) {
	g, fake, root := finalizePreflightFixture(t)
	fake.listEntries = fake.listEntries[:1]
	wt := fake.listEntries[0].Path
	pending := filepath.Join(wt, "merge-hook-output.txt")
	marker := installCleanupHook(t, root, "post-merge", pending)
	fake.onRemove = func(string) { runGitIn(t, root, "worktree", "remove", "--force", wt) }
	err := g.CompleteBead("bead-1", "spec/077-test", "", "", false)
	if data, readErr := os.ReadFile(marker); readErr != nil || string(data) != "fired" {
		t.Fatalf("merge hook did not fire: %q %v", data, readErr)
	}
	if err == nil || !strings.Contains(err.Error(), "recovery: mindspec complete bead-1") {
		t.Errorf("want cleanup refusal with retry, got %v", err)
	}
	if len(fake.removeCalls) != 0 || !branchExistsIn(t, root, "bead/bead-1") {
		t.Errorf("cleanup removed work: calls=%v", fake.removeCalls)
	}
	if data, readErr := os.ReadFile(pending); readErr != nil || string(data) != "cleanup hook output\n" {
		t.Errorf("hook output lost: %q %v", data, readErr)
	}
	t.Logf("merge_hook_fired=true removal_calls=%v error=%v", fake.removeCalls, err)
	if t.Failed() {
		return
	}
	saved := filepath.Join(t.TempDir(), "saved-output.txt")
	if err := os.Rename(pending, saved); err != nil {
		t.Fatal(err)
	}
	if err := g.CompleteBead("bead-1", "spec/077-test", "", "", false); err != nil {
		t.Fatal(err)
	}
	if len(fake.removeCalls) != 1 || branchExistsIn(t, root, "bead/bead-1") {
		t.Fatalf("retry did not clean up: %v", fake.removeCalls)
	}
	if data, err := os.ReadFile(saved); err != nil || string(data) != "cleanup hook output\n" {
		t.Fatalf("saved output lost: %q %v", data, err)
	}
	t.Log("retry_converged=true saved_output_preserved=true")
}

func TestFinalizeOrphanedSpecBranch_HookWorkPreservedAndRetry(t *testing.T) {
	for _, kind := range []string{"post-commit", "pre-push"} {
		t.Run(kind, func(t *testing.T) {
			g, _, root := newRepoExecutor(t)
			stubBeadExport(t)
			origin := setupOrphanOrigin(t, root, true)
			wt := filepath.Join(root, ".worktrees/worktree-finalize-077-test")
			pending := filepath.Join(wt, "carrier-hook-output.txt")
			marker := installCleanupHook(t, root, kind, pending)
			_, err := g.finalizeOrphanedSpecBranch(config.DefaultConfig(), "epic-1", "077-test")
			if data, readErr := os.ReadFile(marker); readErr != nil || string(data) != "fired" {
				t.Fatalf("publication hook did not fire: %q %v", data, readErr)
			}
			if err == nil || !strings.Contains(err.Error(), "recovery: mindspec impl approve 077-test") {
				t.Errorf("want carrier cleanup refusal with retry: %v", err)
			}
			if data, readErr := os.ReadFile(pending); readErr != nil || string(data) != "cleanup hook output\n" {
				t.Errorf("carrier output lost: %q %v", data, readErr)
			}
			if _, statErr := os.Stat(wt); statErr != nil {
				t.Errorf("carrier worktree removed: %v", statErr)
			}
			if !branchExistsIn(t, root, "chore/finalize-077-test") {
				t.Error("carrier branch removed")
			}
			if !branchExistsIn(t, origin, "chore/finalize-077-test") {
				t.Error("fixture did not publish carrier")
			}
			t.Logf("hook=%s hook_fired=true error=%v", kind, err)
			if t.Failed() {
				return
			}
			// A retry must inspect the leftover before it attempts to recreate it.
			_, retryErr := g.finalizeOrphanedSpecBranch(config.DefaultConfig(), "epic-1", "077-test")
			if retryErr == nil || !strings.Contains(retryErr.Error(), "recovery: mindspec impl approve 077-test") {
				t.Fatalf("dirty leftover must refuse: %v", retryErr)
			}
			if data, err := os.ReadFile(pending); err != nil || string(data) != "cleanup hook output\n" {
				t.Fatalf("leftover retry lost output: %q %v", data, err)
			}
			saved := filepath.Join(t.TempDir(), "saved-carrier-output.txt")
			if err := os.Rename(pending, saved); err != nil {
				t.Fatal(err)
			}
			if _, err := g.finalizeOrphanedSpecBranch(config.DefaultConfig(), "epic-1", "077-test"); err != nil {
				t.Fatalf("clean retry failed: %v", err)
			}
			if data, err := os.ReadFile(saved); err != nil || string(data) != "cleanup hook output\n" {
				t.Fatalf("saved output lost: %q %v", data, err)
			}
			if _, err := os.Stat(wt); !os.IsNotExist(err) {
				t.Fatalf("clean retry left worktree: %v", err)
			}
			t.Log("dirty_leftover_preserved=true retry_converged=true saved_output_preserved=true")
		})
	}
}

func TestFinalizeOrphanedSpecBranch_LeftoverUserWorkPreserved(t *testing.T) {
	g, _, root := newRepoExecutor(t)
	stubBeadExport(t)
	setupOrphanOrigin(t, root, true)
	wt := filepath.Join(root, ".worktrees/worktree-finalize-077-test")
	runGitIn(t, root, "branch", "chore/finalize-077-test", "main")
	runGitIn(t, root, "worktree", "add", wt, "chore/finalize-077-test")
	pending := filepath.Join(wt, "leftover.txt")
	if err := os.WriteFile(pending, []byte("leftover work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := g.finalizeOrphanedSpecBranch(config.DefaultConfig(), "epic-1", "077-test")
	if err == nil || !strings.Contains(err.Error(), "recovery: mindspec impl approve 077-test") {
		t.Errorf("want leftover refusal: %v", err)
	}
	if data, err := os.ReadFile(pending); err != nil || string(data) != "leftover work\n" {
		t.Errorf("leftover output lost: %q %v", data, err)
	}
	t.Logf("leftover_refusal=%v", err)
}

func TestCleanupBead_ReadFailurePreservesWork(t *testing.T) {
	for _, failure := range []string{"status", "stat", "list"} {
		t.Run(failure, func(t *testing.T) {
			g, fake, root := finalizePreflightFixture(t)
			fake.listEntries = fake.listEntries[:1]
			wt := fake.listEntries[0].Path
			specWT := filepath.Join(root, ".worktrees/worktree-spec-077-test")
			runGitIn(t, specWT, "merge", "--no-ff", "-m", "Merge bead/bead-1", "bead/bead-1")
			gitFile := filepath.Join(wt, ".git")
			original, err := os.ReadFile(gitFile)
			if err != nil {
				t.Fatal(err)
			}
			saved := filepath.Join(t.TempDir(), "saved-worktree")
			if failure == "list" {
				fake.listErr = errors.New("worktree list unavailable")
			} else if failure == "status" {
				if err := os.WriteFile(gitFile, []byte("gitdir: /nonexistent/bead-cleanup-status\n"), 0644); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Rename(wt, saved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Base(wt), wt); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "list" {
				// The older combined path must not remove by name when
				// it could not resolve the checkout for a status read.
				err = g.CompleteBead("bead-1", "spec/077-test", "", "", false)
			} else {
				err = g.CleanupBead("bead-1", "spec/077-test")
			}
			if err == nil || !strings.HasSuffix(err.Error(), "recovery: mindspec complete bead-1") {
				t.Errorf("want read refusal with retry: %v", err)
			}
			if len(fake.removeCalls) != 0 {
				t.Errorf("read failure reached forced removal: %v", fake.removeCalls)
			}
			if !branchExistsIn(t, root, "bead/bead-1") {
				t.Error("read failure removed branch")
			}
			if failure == "list" {
				fake.listErr = nil
			} else if failure == "status" {
				if err := os.WriteFile(gitFile, original, 0644); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Rename(wt, filepath.Join(t.TempDir(), "saved-loop")); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(saved, wt); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := os.Stat(wt); err != nil {
				t.Errorf("read failure lost worktree: %v", err)
			}
			t.Logf("failure=%s removal_calls=%v error=%v", failure, fake.removeCalls, err)
		})
	}
}
