package executor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

func TestFinalizeEpic_ExportFailurePreservesWorkAndRetries(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal(err)
	}
	for _, site := range []string{"spec", "bead"} {
		for _, remote := range []bool{false, true} {
			name := "direct"
			if remote {
				name = "push"
			}
			t.Run(site+"/"+name, func(t *testing.T) {
				g, fake, root := finalizePreflightFixture(t)
				fake.listEntries = fake.listEntries[:1]
				specWT := filepath.Join(root, ".worktrees", "worktree-spec-077-test")
				faultWT, stage := specWT, "export"
				if site == "bead" {
					faultWT, stage = fake.listEntries[0].Path, "auto_merge"
				}
				pending := filepath.Join(faultWT, "pending-user-work.txt")
				const content = "pending work must survive the export fault\n"
				if err := os.WriteFile(pending, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				origin := t.TempDir()
				if remote {
					runGitIn(t, origin, "init", "--bare", "-b", "main")
					runGitIn(t, root, "remote", "add", "origin", "file://"+origin)
					runGitIn(t, root, "push", "origin", "main")
				}
				mainTip, specTip := refHash(t, root, "main"), refHash(t, root, "spec/077-test")
				beadTip := refHash(t, root, "bead/bead-1")
				fake.onRemove = func(name string) {
					path := specWT
					if name == "worktree-bead-1" {
						path = fake.listEntries[0].Path
					}
					runGitIn(t, root, "worktree", "remove", "--force", path)
				}
				oldExport := execBeadExportFn
				fired := false
				execBeadExportFn = func(path string) error {
					if path == faultWT {
						fired = true
						return errors.New("injected tracker export failure")
					}
					return nil
				}
				t.Cleanup(func() { execBeadExportFn = oldExport })
				_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "", false, remote)
				var failure *FinalizeStageError
				if !fired {
					t.Fatal("export fault did not fire")
				}
				if !errors.As(err, &failure) || failure.Stage != stage || !strings.Contains(err.Error(), "injected tracker export failure") || !strings.HasSuffix(err.Error(), "recovery: mindspec impl approve 077-test") {
					t.Errorf("want stage %s and runnable recovery, got %v", stage, err)
				}
				if got, readErr := os.ReadFile(pending); readErr != nil || string(got) != content {
					t.Errorf("pending work lost: %q %v", got, readErr)
				}
				if _, statErr := os.Stat(specWT); statErr != nil {
					t.Errorf("spec worktree lost: %v", statErr)
				}
				if !branchExistsIn(t, root, "spec/077-test") || !branchExistsIn(t, root, "bead/bead-1") {
					t.Error("export failure removed a branch")
				}
				if len(fake.removeCalls) != 0 {
					t.Errorf("export failure reached cleanup: %v", fake.removeCalls)
				}
				if refHash(t, root, "main") != mainTip {
					t.Error("export failure published to main")
				}
				if branchExistsIn(t, root, "spec/077-test") && refHash(t, root, "spec/077-test") != specTip {
					t.Error("export failure merged or committed spec work")
				}
				if branchExistsIn(t, root, "bead/bead-1") && refHash(t, root, "bead/bead-1") != beadTip {
					t.Error("export failure committed bead work")
				}
				if remote && branchExistsIn(t, origin, "spec/077-test") {
					t.Error("export failure pushed spec branch")
				}
				if t.Failed() {
					return
				}
				execBeadExportFn = func(string) error { return nil }
				if _, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "", false, remote); err != nil {
					t.Fatalf("retry: %v", err)
				}
				targetRoot, target := root, "main"
				if remote {
					targetRoot, target = origin, "spec/077-test"
				}
				got, err := exec.Command("git", "-C", targetRoot, "show", target+":pending-user-work.txt").CombinedOutput()
				if err != nil || string(got) != content {
					t.Fatalf("retry did not publish pending work: %q %v", got, err)
				}
				if branchExistsIn(t, root, "spec/077-test") {
					t.Fatal("retry did not finish cleanup")
				}
			})
		}
	}
}

func TestFinalizeMerges_MovedBeadTipRefusesBeforeMutation(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"preflight", "finalize"} {
		t.Run(entry, func(t *testing.T) {
			g, fake, root := finalizePreflightFixture(t)
			fake.listEntries = fake.listEntries[:1]
			specWT, beadWT := filepath.Join(root, ".worktrees", "worktree-spec-077-test"), fake.listEntries[0].Path
			runGitIn(t, specWT, "merge", "--no-ff", "-m", "Merge bead/bead-1", "bead/bead-1")
			parent := refHash(t, root, "bead/bead-1")
			if err := os.WriteFile(filepath.Join(beadWT, "late.txt"), []byte("later content\n"), 0644); err != nil {
				t.Fatal(err)
			}
			runGitIn(t, beadWT, "add", ".")
			runGitIn(t, beadWT, "commit", "-m", "later fix")
			tip := refHash(t, root, "bead/bead-1")
			pending := filepath.Join(specWT, "pending.txt")
			if err := os.WriteFile(pending, []byte("pending\n"), 0644); err != nil {
				t.Fatal(err)
			}
			before, mainBefore := refHash(t, root, "spec/077-test"), refHash(t, root, "main")
			writes, exports := 0, 0
			oldWrite, oldExport := finalizeMetadataWriteFn, execBeadExportFn
			t.Cleanup(func() { finalizeMetadataWriteFn, execBeadExportFn = oldWrite, oldExport })
			finalizeMetadataWriteFn = func(string, map[string]interface{}) error { writes++; return nil }
			execBeadExportFn = func(string) error { exports++; return nil }
			clearFinalizeStepHook(t)
			finalizeStepHookFn = func(stage string) error {
				if stage == "pre_cleanup" {
					return errors.New("stop baseline before cleanup")
				}
				return nil
			}
			var err error
			if entry == "preflight" {
				err = g.PreflightFinalizeMerges("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "reviewed", true)
			} else {
				_, err = g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "reviewed", false, false)
			}
			for _, want := range []string{"bead tip moved from merge parent " + parent + " to " + tip, "Put later fixes in a follow-up bead.", "mindspec complete bead-1", "bd create \"<title>\" --parent epic-1 --deps bead-1", "git branch bead-1-followup " + tip, "reset --keep " + parent, "cherry-pick " + parent + "..bead-1-followup"} {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("missing moved-tip recovery %q: %v", want, err)
				}
			}
			if err != nil {
				previous := -1
				for _, command := range []string{"recovery: git branch bead-1-followup ", " reset --keep ", "recovery: mindspec complete bead-1", "recovery: bd create ", "recovery: edit plan.md", "recovery: mindspec next <new bead>", " cherry-pick ", "recovery: git branch -D bead-1-followup"} {
					index := strings.Index(err.Error(), command)
					if index <= previous {
						t.Errorf("unsafe recovery order at %q: %v", command, err)
					}
					previous = index
				}
			}
			merges, readErr := gitutil.FirstParentMerges(root, "spec/077-test")
			if readErr != nil {
				t.Fatal(readErr)
			}
			count := 0
			for _, m := range merges {
				if len(m.Parents) == 2 && m.Subject == "Merge bead/bead-1" {
					count++
				}
			}
			if count != 1 {
				t.Errorf("naming merges = %d, want exactly one", count)
			}
			if writes != 0 || exports != 0 || len(fake.removeCalls) != 0 || refHash(t, root, "spec/077-test") != before || refHash(t, root, "main") != mainBefore {
				t.Errorf("refusal mutated state: metadata=%d exports=%d cleanup=%v", writes, exports, fake.removeCalls)
			}
		})
	}
}
