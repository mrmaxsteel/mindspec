package complete

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type closingRemovalRecorder struct {
	windowWorktrees
	calls     int
	removeErr error
}

func (w *closingRemovalRecorder) Remove(name string) error {
	w.calls++
	if w.removeErr != nil {
		return w.removeErr
	}
	return w.windowWorktrees.Remove(name)
}

func TestComplete_ClosePublicationHookPreservesWorkAndRetry(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal(err)
	}
	f := newWindowFixture(t)
	f.mergedOnly()
	remover := &closingRemovalRecorder{windowWorktrees: windowWorktrees{f}}
	f.ex.WorktreeOps = remover
	pending := filepath.Join(f.beadWT, "close-hook-output.txt")
	marker := filepath.Join(t.TempDir(), "close-hook-fired")
	t.Setenv("CLOSE_HOOK_OUTPUT", pending)
	t.Setenv("CLOSE_HOOK_MARKER", marker)
	t.Setenv("CLOSE_HOOK_ARTIFACT", filepath.Join(f.beadWT, ".beads/issues.jsonl"))
	hook := "#!/bin/sh\nif [ ! -f \"$CLOSE_HOOK_MARKER\" ]; then\n printf 'close hook output\\n' > \"$CLOSE_HOOK_OUTPUT\" || exit 1\n printf 'tool artifact drift\\n' > \"$CLOSE_HOOK_ARTIFACT\" || exit 1\n printf fired > \"$CLOSE_HOOK_MARKER\" || exit 1\nfi\n"
	writeFile(t, f.root, ".git/hooks/post-commit", hook)
	if err := os.Chmod(filepath.Join(f.root, ".git/hooks/post-commit"), 0755); err != nil {
		t.Fatal(err)
	}
	result, err := f.run(CompleteOpts{})
	if fired, readErr := os.ReadFile(marker); readErr != nil || string(fired) != "fired" {
		t.Fatalf("close publication hook did not fire: %q %v", fired, readErr)
	}
	if err == nil || !strings.Contains(err.Error(), "recovery: mindspec complete "+closingBead) {
		t.Errorf("want a completion retry, got result=%+v error=%v", result, err)
	}
	if remover.calls != 0 {
		t.Errorf("forced removal ran %d times", remover.calls)
	}
	if data, readErr := os.ReadFile(pending); readErr != nil || string(data) != "close hook output\n" {
		t.Errorf("hook output lost: %q %v", data, readErr)
	}
	for _, branch := range []string{"bead/" + closingBead, "spec/" + closingSpec} {
		if got := gateRevParse(t, f.root, branch); got == "" {
			t.Errorf("branch lost: %s", branch)
		}
	}
	if _, statErr := os.Stat(f.beadWT); statErr != nil {
		t.Errorf("worktree lost: %v", statErr)
	}
	if !f.closed || f.closeCalls != 1 {
		t.Errorf("fixture did not reach durable close: closed=%v calls=%d", f.closed, f.closeCalls)
	}
	t.Logf("hook_fired=true removal_calls=%d error=%v", remover.calls, err)
	if t.Failed() {
		return
	}
	saved := filepath.Join(t.TempDir(), "saved-close-hook-output.txt")
	if err := os.Rename(pending, saved); err != nil {
		t.Fatal(err)
	}
	result, err = f.run(CompleteOpts{})
	if err != nil || result == nil || !result.WorktreeRemoved || remover.calls != 1 {
		t.Fatalf("retry did not converge: %+v %v removals=%d", result, err, remover.calls)
	}
	if data, err := os.ReadFile(saved); err != nil || string(data) != "close hook output\n" {
		t.Fatalf("saved output lost on retry: %q %v", data, err)
	}
	if f.closeCalls != 1 {
		t.Fatalf("retry closed twice: %d", f.closeCalls)
	}
	t.Log("retry_converged=true saved_output_preserved=true")
}

func TestComplete_CloseCleanupReadFailurePreservesWorkAndRetry(t *testing.T) {
	for _, failure := range []string{"status", "stat"} {
		t.Run(failure, func(t *testing.T) {
			f := newWindowFixture(t)
			f.mergedOnly()
			remover := &closingRemovalRecorder{windowWorktrees: windowWorktrees{f}, removeErr: errors.New("unexpected forced removal")}
			f.ex.WorktreeOps = remover
			gitFile := filepath.Join(f.beadWT, ".git")
			original, err := os.ReadFile(gitFile)
			if err != nil {
				t.Fatal(err)
			}
			saved := filepath.Join(t.TempDir(), "saved-worktree")
			oldHook := closingStepHook
			t.Cleanup(func() { closingStepHook = oldHook })
			closingStepHook = func(step int) error {
				if step != 6 {
					return nil
				}
				if failure == "status" {
					return os.WriteFile(gitFile, []byte("gitdir: /nonexistent/close-cleanup-status\n"), 0644)
				}
				if err := os.Rename(f.beadWT, saved); err != nil {
					return err
				}
				return os.Symlink(filepath.Base(f.beadWT), f.beadWT)
			}
			result, err := f.run(CompleteOpts{})
			if err == nil || !strings.Contains(err.Error(), "recovery: mindspec complete "+closingBead) {
				t.Errorf("want read failure with retry: result=%+v error=%v", result, err)
			}
			if remover.calls != 0 {
				t.Errorf("read failure reached removal: %d", remover.calls)
			}
			if failure == "status" {
				if err := os.WriteFile(gitFile, original, 0644); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Rename(f.beadWT, filepath.Join(t.TempDir(), "saved-loop")); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(saved, f.beadWT); err != nil {
					t.Fatal(err)
				}
			}
			if got := gateRevParse(t, f.root, "bead/"+closingBead); got != f.tip {
				t.Errorf("branch lost or changed: %s", got)
			}
			if _, statErr := os.Stat(f.beadWT); statErr != nil {
				t.Errorf("worktree lost: %v", statErr)
			}
			t.Logf("failure=%s removal_calls=%d error=%v", failure, remover.calls, err)
			if t.Failed() {
				return
			}
			closingStepHook = nil
			remover.removeErr = nil
			result, err = f.run(CompleteOpts{})
			if err != nil || result == nil || !result.WorktreeRemoved {
				t.Fatalf("retry failed: %+v %v", result, err)
			}
		})
	}
}
