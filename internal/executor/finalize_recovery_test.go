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

func TestFinalizeEpic_CleanupAncestryRefusalText(t *testing.T) {
	g, fake, root := finalizePreflightFixture(t)
	fake.listEntries = fake.listEntries[:1]
	wt := fake.listEntries[0].Path
	t.Cleanup(SetFinalizeStepHookForTest(func(stage string) error {
		if stage == "pre_cleanup" {
			// A branch that moves after publication must survive cleanup.
			runGitIn(t, wt, "commit", "--allow-empty", "-m", "late bead commit")
		}
		return nil
	}))
	_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "", false, false)
	var failure *FinalizeStageError
	if !errors.As(err, &failure) || failure.Stage != "cleanup" {
		t.Fatalf("expected cleanup refusal: %v", err)
	}
	want := fmt.Sprintf("cannot clean up branch bead/bead-1: tip is not an ancestor of %s", refHash(t, root, "spec/077-test"))
	if got := failure.Err.Error(); got != want {
		t.Fatalf("cleanup refusal = %q, want %q", got, want)
	}
	if len(fake.removeCalls) != 0 || !branchExistsIn(t, root, "bead/bead-1") {
		t.Fatal("ancestry refusal removed work")
	}
}

func TestFinalizeEpic_RefusalsHaveRunnableRecovery(t *testing.T) {
	for _, fault := range []string{"branch_probe", "absent_branch", "ancestry"} {
		t.Run(fault, func(t *testing.T) {
			g, fake, root := finalizePreflightFixture(t)
			fake.listEntries = fake.listEntries[:1]
			message := "cannot use an absent bead branch without published landing evidence"
			if fault == "absent_branch" {
				runGitIn(t, root, "worktree", "remove", "--force", fake.listEntries[0].Path)
				runGitIn(t, root, "branch", "-D", "bead/bead-1")
			} else {
				git, err := exec.LookPath("git")
				if err != nil {
					t.Fatal(err)
				}
				pattern := "rev-parse --verify --quiet refs/heads/bead/bead-1"
				message = "cannot check bead branch"
				if fault == "ancestry" {
					pattern = "merge-base --is-ancestor bead/bead-1 spec/077-test"
					message = "cannot read bead branch ancestry"
				}
				bin := t.TempDir()
				script := "#!/bin/sh\ncase \" $* \" in *\" " + pattern + " \"*) exit 128;; esac\nexec '" + git + "' \"$@\"\n"
				if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "", false, false)
			if err == nil || !strings.Contains(err.Error(), message) {
				t.Fatalf("wrong refusal: %v", err)
			}
			if !strings.HasSuffix(err.Error(), "recovery: mindspec impl approve 077-test") {
				t.Fatalf("refusal has no runnable approval command: %v", err)
			}
			if len(fake.removeCalls) != 0 {
				t.Fatal("refusal reached cleanup")
			}
		})
	}
}
