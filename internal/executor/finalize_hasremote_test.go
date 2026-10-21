package executor

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

func testFinalizeDecision(t *testing.T, configured, passed bool) {
	t.Helper()
	g, _, dir := newRepoExecutor(t)
	stubBeadExport(t)
	runGitIn(t, dir, "switch", "-c", "spec/077-test")
	runGitIn(t, dir, "commit", "--allow-empty", "-m", "spec work")
	tip := refHash(t, dir, "HEAD")
	runGitIn(t, dir, "switch", "main")
	origin := t.TempDir()
	if configured {
		runGitIn(t, origin, "init", "--bare", "-b", "main")
		runGitIn(t, dir, "remote", "add", "origin", "file://"+origin)
		runGitIn(t, dir, "push", "origin", "main")
	}
	result, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{}, "", false, passed)
	if passed && !configured {
		if err == nil || !strings.Contains(err.Error(), "pushing") {
			t.Fatalf("passed true must try push: %v", err)
		}
		if result.MergeStrategy != "pr" || refHash(t, dir, "main") == tip {
			t.Fatalf("push failure selected local merge: %+v", result)
		}
		if !branchExistsIn(t, dir, "spec/077-test") {
			t.Fatal("failed push removed spec branch")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if passed {
		if result.MergeStrategy != "pr" || refHash(t, origin, "spec/077-test") != tip {
			t.Fatalf("push result = %+v", result)
		}
	} else {
		if result.MergeStrategy != "direct" || !isAncestorIn(t, dir, tip, "main") {
			t.Fatalf("direct result = %+v", result)
		}
		if configured && branchExistsIn(t, origin, "spec/077-test") {
			t.Fatal("passed false pushed a branch")
		}
	}
}
func TestFinalizeEpic_WithRemotePushes(t *testing.T)      { testFinalizeDecision(t, true, true) }
func TestFinalizeEpic_NoRemoteMergesLocally(t *testing.T) { testFinalizeDecision(t, false, false) }
func TestFinalizeEpic_PassedFalseWithRemoteMergesLocally(t *testing.T) {
	testFinalizeDecision(t, true, false)
}
func TestFinalizeEpic_PassedTrueNoRemoteTakesPushLeg(t *testing.T) {
	testFinalizeDecision(t, false, true)
}

func finalizePreflightFixture(t *testing.T) (*MindspecExecutor, *fakeWorktreeOps, string) {
	t.Helper()
	g, fake, dir := newRepoExecutor(t)
	stubBeadExport(t)
	runGitIn(t, dir, "branch", "spec/077-test")
	specWt := filepath.Join(dir, ".worktrees", "worktree-spec-077-test")
	runGitIn(t, dir, "worktree", "add", specWt, "spec/077-test")
	wt := plantBeadWorktree(t, dir, "bead/bead-1", "feature.txt")
	fake.listEntries = []bead.WorktreeListEntry{
		{Name: "worktree-bead-1", Path: wt, Branch: "bead/bead-1"},
		{Name: "foreign", Branch: "bead/foreign-1"},
		{Name: "invalid", Branch: "bead/-invalid"},
	}
	return g, fake, dir
}

func TestPreflightFinalizeMerges_MatchesFinalizeMerges(t *testing.T) {
	for _, remote := range []bool{true, false} {
		name := "direct"
		if remote {
			name = "push"
		}
		t.Run(name, func(t *testing.T) {
			g, fake, dir := finalizePreflightFixture(t)
			var merges []string
			stubWorkDestructionFn(t, func(_ string, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
				merges = append(merges, branch+" -> "+target)
				return guard.DestructionClean, gitutil.WorkDestructionEvidence{}, nil
			})
			before := refHash(t, dir, "spec/077-test")
			if err := g.PreflightFinalizeMerges("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "reviewed", remote); err != nil {
				t.Fatal(err)
			}
			early := append([]string(nil), merges...)
			want := []string{"bead/bead-1 -> spec/077-test"}
			if !remote {
				want = append(want, "spec/077-test -> main")
			}
			if !reflect.DeepEqual(early, want) {
				t.Fatalf("early merges=%v want=%v", early, want)
			}
			if refHash(t, dir, "spec/077-test") != before || len(fake.removeCalls) != 0 {
				t.Fatal("preflight mutated state")
			}
			if remote {
				origin := t.TempDir()
				runGitIn(t, origin, "init", "--bare", "-b", "main")
				runGitIn(t, dir, "remote", "add", "origin", "file://"+origin)
				runGitIn(t, dir, "push", "origin", "main")
			}
			merges = nil
			clearFinalizeStepHook(t)
			stop := errors.New("stop before cleanup")
			finalizeStepHookFn = func(stage string) error {
				if stage == "pre_cleanup" {
					return stop
				}
				return nil
			}
			_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "reviewed", false, remote)
			if !errors.Is(err, stop) {
				t.Fatalf("finalization: %v", err)
			}
			if !reflect.DeepEqual(merges, early) {
				t.Fatalf("final guards=%v early=%v", merges, early)
			}
			if !isAncestorIn(t, dir, "bead/bead-1", "spec/077-test") {
				t.Fatal("bead did not merge")
			}
		})
	}
	t.Run("list-error", func(t *testing.T) {
		g, fake, _ := newRepoExecutor(t)
		fake.listErr = errors.New("read failed")
		if err := g.PreflightFinalizeMerges("epic-1", "077-test", "spec/077-test", []string{}, "", true); err == nil {
			t.Fatal("list error passed")
		}
	})
}

func TestFinalizeEpic_ExecutorGuardAfterAutoMerge(t *testing.T) {
	for _, kind := range []string{"layout", "destruction"} {
		t.Run(kind, func(t *testing.T) {
			g, fake, dir := newRepoExecutor(t)
			stubBeadExport(t)
			runGitIn(t, dir, "branch", "spec/077-test")
			mainFile := filepath.Join(dir, ".mindspec", "specs", "flat", "spec.md")
			if err := os.MkdirAll(filepath.Dir(mainFile), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(mainFile, []byte("flat main\n"), 0644); err != nil {
				t.Fatal(err)
			}
			runGitIn(t, dir, "add", ".")
			runGitIn(t, dir, "commit", "-m", "flat main")
			mainBefore := refHash(t, dir, "main")
			specWt := filepath.Join(dir, ".worktrees", "worktree-spec-077-test")
			runGitIn(t, dir, "worktree", "add", specWt, "spec/077-test")
			runGitIn(t, dir, "branch", "bead/bead-1", "spec/077-test")
			wt := filepath.Join(dir, ".bead-work")
			runGitIn(t, dir, "worktree", "add", wt, "bead/bead-1")
			file := "feature.txt"
			if kind == "layout" {
				file = ".mindspec/docs/specs/old/spec.md"
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(wt, file)), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(wt, file), []byte("bead work\n"), 0644); err != nil {
				t.Fatal(err)
			}
			runGitIn(t, wt, "add", ".")
			runGitIn(t, wt, "commit", "-m", "bead work")
			beadTip := refHash(t, dir, "bead/bead-1")
			fake.listEntries = []bead.WorktreeListEntry{{Name: "worktree-bead-1", Path: wt, Branch: "bead/bead-1"}}
			checkedAfter := false
			stubWorkDestructionFn(t, func(_ string, branch, target string) (guard.DestructionOutcome, gitutil.WorkDestructionEvidence, error) {
				if branch == "spec/077-test" && isAncestorIn(t, dir, beadTip, branch) {
					checkedAfter = true
					if kind == "destruction" {
						return guard.DestructionSuperseded, gitutil.WorkDestructionEvidence{SupersededVia: target}, nil
					}
				}
				return guard.DestructionClean, gitutil.WorkDestructionEvidence{}, nil
			})
			if err := g.PreflightFinalizeMerges("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "", false); err != nil {
				t.Fatalf("early guard: %v", err)
			}
			_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "", false, false)
			want := "layout regression"
			if kind == "destruction" {
				want = "content already landed"
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("want live %s refusal, got %v", kind, err)
			}
			if !isAncestorIn(t, dir, beadTip, "spec/077-test") {
				t.Fatal("auto-merge did not run")
			}
			if kind == "destruction" && !checkedAfter {
				t.Fatal("guard did not read post-merge tip")
			}
			if refHash(t, dir, "main") != mainBefore || len(fake.removeCalls) != 0 {
				t.Fatal("guard allowed main merge or cleanup")
			}
		})
	}
}

func TestPushProbe_ArgvMatchesPushBranch(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "argv")
	script := "#!/bin/sh\nprintf '%s\\n' \"$GIT_TERMINAL_PROMPT\" \"$@\" >> \"$ARGV_LOG\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("ARGV_LOG", log)
	g := &MindspecExecutor{Root: dir}
	if err := g.PushProbe("spec/077-test"); err != nil {
		t.Fatal(err)
	}
	if err := gitutil.PushBranch("spec/077-test"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := "0\npush\n--dry-run\n--no-verify\n-u\norigin\nspec/077-test\n0\npush\n-u\norigin\nspec/077-test\n"
	if string(data) != want {
		t.Fatalf("argv and environment mismatch:\n%s", data)
	}
}
