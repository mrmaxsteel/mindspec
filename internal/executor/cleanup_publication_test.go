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
			assertTemporaryFinalizeRecovery(t, err, wt)
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
			assertTemporaryFinalizeRecovery(t, retryErr, wt)
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

func assertTemporaryFinalizeRecovery(t *testing.T, err error, wt string) {
	t.Helper()
	if err == nil {
		t.Fatal("want temporary finalization refusal")
	}
	for _, want := range []string{"temporary finalization worktree", wt, "preserve the changes at a safe path", "leave this worktree clean", "recovery: mindspec impl approve 077-test"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("temporary checkout refusal lacks %q: %v", want, err)
		}
	}
	for _, wrong := range []string{"already merged worktree", "follow-up branch"} {
		if strings.Contains(err.Error(), wrong) {
			t.Errorf("temporary checkout refusal contains %q: %v", wrong, err)
		}
	}
}

func assertFinalizeCarrierPublished(t *testing.T, root, origin, wt, branch string, export []byte) {
	t.Helper()
	if branch != "chore/finalize-077-test" {
		t.Fatalf("wrong carrier branch: %q", branch)
	}
	if local, remote := refHash(t, root, branch), refHash(t, origin, branch); local != remote {
		t.Fatalf("carrier was not published: local=%s remote=%s", local, remote)
	}
	data, err := exec.Command("git", "-C", origin, "show", branch+":.beads/issues.jsonl").Output()
	if err != nil || string(data) != string(export) {
		t.Fatalf("published export = %q, %v; want %q", data, err, export)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("temporary checkout remains: %v", err)
	}
}

func TestFinalizeOrphanedSpecBranch_PreCommitFailureRetry(t *testing.T) {
	g, _, root := newRepoExecutor(t)
	export := stubBeadExport(t)
	origin := setupOrphanOrigin(t, root, true)
	wt := filepath.Join(root, ".worktrees/worktree-finalize-077-test")
	// The hook writes no file. Change only its environment for the retry.
	hook := "#!/bin/sh\nif [ \"$REJECT_FINALIZE_COMMIT\" = yes ]; then\n echo 'finalize commit rejected once' >&2\n exit 1\nfi\n"
	if err := os.WriteFile(filepath.Join(root, ".git/hooks/pre-commit"), []byte(hook), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REJECT_FINALIZE_COMMIT", "yes")
	_, err := g.finalizeOrphanedSpecBranch(config.DefaultConfig(), "epic-1", "077-test")
	if err == nil || !strings.Contains(err.Error(), "finalize commit rejected once") {
		t.Fatalf("pre-commit rejection did not fire: %v", err)
	}
	if branchExistsIn(t, origin, "chore/finalize-077-test") {
		t.Fatal("failed commit published a carrier")
	}
	t.Logf("first_attempt=%v", err)
	if strings.Contains(err.Error(), "pending changes") {
		t.Errorf("tool export must not add a cleanup refusal: %v", err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("failed commit left a tracker-only checkout: %v", err)
	}
	t.Setenv("REJECT_FINALIZE_COMMIT", "no")
	branch, err := g.finalizeOrphanedSpecBranch(config.DefaultConfig(), "epic-1", "077-test")
	if err != nil {
		t.Fatalf("second attempt must publish after the hook passes: %v", err)
	}
	assertFinalizeCarrierPublished(t, root, origin, wt, branch, *export)
	t.Log("second_attempt_published=true temporary_worktree_removed=true")
}

func TestFinalizeOrphanedSpecBranch_LeftoverTrackerExportRetry(t *testing.T) {
	g, _, root := newRepoExecutor(t)
	export := stubBeadExport(t)
	origin := setupOrphanOrigin(t, root, true)
	wt := filepath.Join(root, ".worktrees/worktree-finalize-077-test")
	runGitIn(t, root, "worktree", "add", "-b", "chore/finalize-077-test", wt, "origin/main")
	if err := execBeadExportFn(wt); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, wt, "add", ".beads/issues.jsonl")
	status, err := exec.Command("git", "-C", wt, "status", "--porcelain").Output()
	if err != nil || string(status) != "A  .beads/issues.jsonl\n" {
		t.Fatalf("fixture must contain only the staged export: %q, %v", status, err)
	}
	branch, err := g.finalizeOrphanedSpecBranch(config.DefaultConfig(), "epic-1", "077-test")
	if err != nil {
		t.Fatalf("tracker-only leftover must regenerate and publish: %v", err)
	}
	assertFinalizeCarrierPublished(t, root, origin, wt, branch, *export)
	t.Log("staged_export_leftover_replaced=true carrier_published=true temporary_worktree_removed=true")
}

func TestFinalizeOrphanedSpecBranch_LeftoverReadFailurePreservedAndRetry(t *testing.T) {
	for _, failure := range []string{"registration", "status"} {
		t.Run(failure, func(t *testing.T) {
			g, _, root := newRepoExecutor(t)
			export := stubBeadExport(t)
			origin := setupOrphanOrigin(t, root, true)
			wt := filepath.Join(root, ".worktrees/worktree-finalize-077-test")
			runGitIn(t, root, "worktree", "add", "-b", "chore/finalize-077-test", wt, "origin/main")
			if err := execBeadExportFn(wt); err != nil {
				t.Fatal(err)
			}
			runGitIn(t, wt, "add", ".beads/issues.jsonl")
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			marker := filepath.Join(bin, "read-failed")
			script := "#!/bin/sh\n" +
				"if { [ \"$FINALIZE_READ_FAILURE\" = registration ] && [ \"$3\" = worktree ] && [ \"$4\" = list ]; } ||\n" +
				"   { [ \"$FINALIZE_READ_FAILURE\" = status ] && [ \"$2\" = \"$FINALIZE_READ_WORKTREE\" ] && [ \"$3\" = status ]; }; then\n" +
				" printf failed > \"$FINALIZE_READ_MARKER\"\n exit 128\nfi\n" +
				"exec \"$FINALIZE_REAL_GIT\" \"$@\"\n"
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("FINALIZE_REAL_GIT", realGit)
			t.Setenv("FINALIZE_READ_FAILURE", failure)
			t.Setenv("FINALIZE_READ_WORKTREE", wt)
			t.Setenv("FINALIZE_READ_MARKER", marker)
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			_, err = g.finalizeOrphanedSpecBranch(config.DefaultConfig(), "epic-1", "077-test")
			if err == nil || !strings.Contains(err.Error(), "cannot read temporary finalization worktree "+failure) || !strings.HasSuffix(err.Error(), "recovery: mindspec impl approve 077-test") {
				t.Errorf("want temporary checkout read refusal with retry: %v", err)
			}
			if data, err := os.ReadFile(marker); err != nil || string(data) != "failed" {
				t.Errorf("read failure did not fire: %q, %v", data, err)
			}
			if data, err := os.ReadFile(filepath.Join(wt, ".beads/issues.jsonl")); err != nil || string(data) != string(*export) {
				t.Fatalf("read failure lost checkout content: %q, %v", data, err)
			}
			if !branchExistsIn(t, root, "chore/finalize-077-test") || branchExistsIn(t, origin, "chore/finalize-077-test") {
				t.Error("read failure changed carrier branches")
			}
			t.Logf("read_failure=%s refusal=%v", failure, err)
			t.Setenv("FINALIZE_READ_FAILURE", "")
			branch, err := g.finalizeOrphanedSpecBranch(config.DefaultConfig(), "epic-1", "077-test")
			if err != nil {
				t.Fatalf("retry after read repair failed: %v", err)
			}
			assertFinalizeCarrierPublished(t, root, origin, wt, branch, *export)
			t.Log("checkout_preserved=true repaired_retry_published=true temporary_worktree_removed=true")
		})
	}
}

func TestFinalizeOrphanedSpecBranch_UnregisteredDirectoryPreservedAndRetry(t *testing.T) {
	for _, dirtyParent := range []bool{false, true} {
		name := map[bool]string{false: "clean_parent", true: "dirty_parent"}[dirtyParent]
		t.Run(name, func(t *testing.T) {
			g, _, root := newRepoExecutor(t)
			export := stubBeadExport(t)
			origin := setupOrphanOrigin(t, root, true)
			wt := filepath.Join(root, ".worktrees/worktree-finalize-077-test")
			// Hide the directory from the parent status, as in a normal repo.
			if err := os.WriteFile(filepath.Join(root, ".git/info/exclude"), []byte(".worktrees/\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if dirtyParent {
				if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("parent work\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(wt, 0755); err != nil {
				t.Fatal(err)
			}
			pending := filepath.Join(wt, "keep.txt")
			if err := os.WriteFile(pending, []byte("directory work\n"), 0644); err != nil {
				t.Fatal(err)
			}
			_, err := g.finalizeOrphanedSpecBranch(config.DefaultConfig(), "epic-1", "077-test")
			if err == nil {
				t.Fatal("unregistered directory must refuse")
			}
			for _, want := range []string{wt, "not a registered worktree", "move the directory to a safe path", "recovery: mindspec impl approve 077-test"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("unregistered directory refusal lacks %q: %v", want, err)
				}
			}
			if strings.Contains(err.Error(), "a.txt") || strings.Contains(err.Error(), "pending changes") {
				t.Errorf("refusal reports parent repository changes: %v", err)
			}
			if data, err := os.ReadFile(pending); err != nil || string(data) != "directory work\n" {
				t.Fatalf("directory content lost: %q, %v", data, err)
			}
			if branchExistsIn(t, root, "chore/finalize-077-test") || branchExistsIn(t, origin, "chore/finalize-077-test") {
				t.Error("unregistered directory refusal created a carrier")
			}
			t.Logf("unregistered_refusal=%v", err)
			saved := filepath.Join(t.TempDir(), "saved-directory")
			if err := os.Rename(wt, saved); err != nil {
				t.Fatal(err)
			}
			branch, err := g.finalizeOrphanedSpecBranch(config.DefaultConfig(), "epic-1", "077-test")
			if err != nil {
				t.Fatalf("retry after moving directory must converge: %v", err)
			}
			assertFinalizeCarrierPublished(t, root, origin, wt, branch, *export)
			if data, err := os.ReadFile(filepath.Join(saved, "keep.txt")); err != nil || string(data) != "directory work\n" {
				t.Fatalf("saved directory content lost: %q, %v", data, err)
			}
			if dirtyParent {
				if data, err := os.ReadFile(filepath.Join(root, "a.txt")); err != nil || string(data) != "parent work\n" {
					t.Fatalf("parent work lost: %q, %v", data, err)
				}
			}
			t.Log("directory_preserved=true retry_published=true temporary_worktree_removed=true")
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
	assertTemporaryFinalizeRecovery(t, err, wt)
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
