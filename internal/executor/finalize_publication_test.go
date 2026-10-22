package executor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalizeEpic_PushHookPendingWorkPreservedAndRetry(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal(err)
	}
	for _, site := range []string{"bead", "spec"} {
		t.Run(site, func(t *testing.T) {
			g, fake, root := finalizePreflightFixture(t)
			fake.listEntries = fake.listEntries[:1]
			specWT := filepath.Join(root, ".worktrees", "worktree-spec-077-test")
			beadWT := fake.listEntries[0].Path
			wt := specWT
			if site == "bead" {
				wt = beadWT
			}
			pending := filepath.Join(wt, "hook-output.txt")
			marker := filepath.Join(t.TempDir(), "hook-fired")
			origin := t.TempDir()
			runGitIn(t, origin, "init", "--bare", "-b", "main")
			runGitIn(t, root, "remote", "add", "origin", "file://"+origin)
			runGitIn(t, root, "push", "origin", "main")
			t.Setenv("FINALIZE_HOOK_PENDING", pending)
			t.Setenv("FINALIZE_HOOK_MARKER", marker)
			// The real push must run this hook. A retry must not create
			// more work after the first output has been preserved.
			hook := "#!/bin/sh\nif [ ! -f \"$FINALIZE_HOOK_MARKER\" ]; then\n  printf 'hook output\\n' > \"$FINALIZE_HOOK_PENDING\" || exit 1\n  printf fired > \"$FINALIZE_HOOK_MARKER\" || exit 1\nfi\n"
			if err := os.WriteFile(filepath.Join(root, ".git", "hooks", "pre-push"), []byte(hook), 0755); err != nil {
				t.Fatal(err)
			}
			fake.onRemove = func(name string) {
				path := specWT
				if name == "worktree-bead-1" {
					path = beadWT
				}
				runGitIn(t, root, "worktree", "remove", "--force", path)
			}
			_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "", false, true)
			fired, readErr := os.ReadFile(marker)
			if readErr != nil || string(fired) != "fired" {
				t.Fatalf("real pre-push hook did not fire: %q %v", fired, readErr)
			}
			var stage *FinalizeStageError
			if !errors.As(err, &stage) || stage.Stage != "cleanup" || !strings.Contains(err.Error(), "pending changes") || !strings.HasSuffix(err.Error(), "recovery: mindspec impl approve 077-test") {
				t.Errorf("want cleanup-stage pending-work recovery, got %v", err)
			}
			if len(fake.removeCalls) != 0 {
				t.Errorf("pending work reached forced removal: %v", fake.removeCalls)
			}
			for _, path := range []string{specWT, beadWT} {
				if _, err := os.Stat(path); err != nil {
					t.Errorf("worktree removed: %s: %v", path, err)
				}
			}
			for _, branch := range []string{"spec/077-test", "bead/bead-1"} {
				if !branchExistsIn(t, root, branch) {
					t.Errorf("branch removed: %s", branch)
				}
			}
			content, readErr := os.ReadFile(pending)
			if readErr != nil || string(content) != "hook output\n" {
				t.Errorf("hook output lost: %q %v", content, readErr)
			}
			if !branchExistsIn(t, origin, "spec/077-test") {
				t.Error("fixture did not publish before the cleanup check")
			}
			t.Logf("hook_fired=true cleanup_calls=%v pending_work_exists=%v finalize_error=%v", fake.removeCalls, readErr == nil, err)
			if t.Failed() {
				return
			}
			preserved := filepath.Join(t.TempDir(), "hook-output.txt")
			if site == "spec" {
				runGitIn(t, wt, "add", "hook-output.txt")
				runGitIn(t, wt, "commit", "-m", "preserve hook output")
			} else if err := os.Rename(pending, preserved); err != nil {
				t.Fatal(err)
			}
			if _, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "", false, true); err != nil {
				t.Fatalf("retry after preserving output: %v", err)
			}
			if site == "spec" {
				content, err = exec.Command("git", "-C", origin, "show", "spec/077-test:hook-output.txt").CombinedOutput()
			} else {
				content, err = os.ReadFile(preserved)
			}
			if err != nil || string(content) != "hook output\n" {
				t.Fatalf("retry lost preserved output: %q %v", content, err)
			}
			if len(fake.removeCalls) != 2 || branchExistsIn(t, root, "spec/077-test") || branchExistsIn(t, root, "bead/bead-1") {
				t.Fatalf("retry did not finish cleanup: %v", fake.removeCalls)
			}
		})
	}
}

func TestResumeFinalizedEpic_PendingSpecWorkPreservesAllWorktreesAndRetry(t *testing.T) {
	g, fake, root := finalizePreflightFixture(t)
	fake.listEntries = fake.listEntries[:1]
	specWT := filepath.Join(root, ".worktrees", "worktree-spec-077-test")
	beadWT := fake.listEntries[0].Path
	runGitIn(t, specWT, "merge", "--no-ff", "-m", "Merge bead/bead-1", "bead/bead-1")
	tip, beadTip := refHash(t, root, "spec/077-test"), refHash(t, root, "bead/bead-1")
	runGitIn(t, root, "merge", "--no-ff", "-m", "land spec", "spec/077-test")
	runGitIn(t, specWT, "switch", "--detach")
	runGitIn(t, root, "branch", "-D", "spec/077-test")
	oldRead, oldBinding := finalizeMetadataReadFn, mergeBindingReadFn
	t.Cleanup(func() { finalizeMetadataReadFn, mergeBindingReadFn = oldRead, oldBinding })
	finalizeMetadataReadFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{"mindspec_finalize_tip": tip}, nil
	}
	mergeBindingReadFn = func(string) (map[string]interface{}, error) {
		return map[string]interface{}{"mindspec_landed_merge_sha": tip, "mindspec_landed_second_parent": beadTip}, nil
	}
	fake.onRemove = func(name string) {
		path := specWT
		if name == "worktree-bead-1" {
			path = beadWT
		}
		runGitIn(t, root, "worktree", "remove", "--force", path)
	}
	pending := filepath.Join(specWT, "pending.txt")
	if err := os.WriteFile(pending, []byte("preserve resumed work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := g.ResumeFinalizedEpic("epic-1", "077-test", []string{"bead-1"}, false)
	var stage *FinalizeStageError
	if !errors.As(err, &stage) || stage.Stage != "cleanup" || !strings.Contains(err.Error(), "pending changes") || !strings.HasSuffix(err.Error(), "recovery: mindspec impl approve 077-test") {
		t.Errorf("want cleanup-stage pending-work recovery, got %v", err)
	}
	if len(fake.removeCalls) != 0 || !branchExistsIn(t, root, "bead/bead-1") {
		t.Errorf("resume removed work before checking the spec worktree: %v", fake.removeCalls)
	}
	if content, err := os.ReadFile(pending); err != nil || string(content) != "preserve resumed work\n" {
		t.Errorf("resume lost pending work: %q %v", content, err)
	}
	if t.Failed() {
		return
	}
	preserved := filepath.Join(t.TempDir(), "pending.txt")
	if err := os.Rename(pending, preserved); err != nil {
		t.Fatal(err)
	}
	if _, err := g.ResumeFinalizedEpic("epic-1", "077-test", []string{"bead-1"}, false); err != nil {
		t.Fatalf("resume retry: %v", err)
	}
	if content, err := os.ReadFile(preserved); err != nil || string(content) != "preserve resumed work\n" {
		t.Fatalf("retry lost preserved output: %q %v", content, err)
	}
	if len(fake.removeCalls) != 2 || branchExistsIn(t, root, "bead/bead-1") {
		t.Fatalf("resume retry did not finish cleanup: %v", fake.removeCalls)
	}
}

func TestFinalizeEpic_AbsentPendingBeadWorktreeStillMerges(t *testing.T) {
	g, fake, root := finalizePreflightFixture(t)
	fake.listEntries = fake.listEntries[:1]
	beadWT := fake.listEntries[0].Path
	if err := os.Rename(beadWT, filepath.Join(t.TempDir(), "saved-worktree")); err != nil {
		t.Fatal(err)
	}
	oldExport := execBeadExportFn
	var exported []string
	execBeadExportFn = func(path string) error {
		exported = append(exported, path)
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("export requires an existing directory: %w", err)
		}
		return oldExport(path)
	}
	t.Cleanup(func() { execBeadExportFn = oldExport })
	fake.onRemove = func(name string) {
		if name == "worktree-bead-1" {
			// Model removal of the stale registration, without recreating
			// its absent directory or touching the preserved checkout.
			runGitIn(t, root, "worktree", "prune")
			return
		}
		runGitIn(t, root, "worktree", "remove", "--force", filepath.Join(root, ".worktrees", name))
	}
	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "", false, false)
	if err != nil {
		t.Fatalf("absent pending worktree must not block its branch merge: %v", err)
	}
	for _, path := range exported {
		if path == beadWT {
			t.Fatal("artifact export ran in the absent bead worktree")
		}
	}
	content, err := exec.Command("git", "-C", root, "show", "main:feature.txt").CombinedOutput()
	if err != nil || string(content) != "content of feature.txt\n" {
		t.Fatalf("absent worktree's branch did not land: %q %v", content, err)
	}
	if len(fake.removeCalls) != 2 || branchExistsIn(t, root, "bead/bead-1") {
		t.Fatalf("cleanup did not converge: %v", fake.removeCalls)
	}
}
