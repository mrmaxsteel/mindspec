package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalizeMergedWorktree_ReadFailuresKeepWork(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(map[bool]string{false: "finalize", true: "resume"}[resume], func(t *testing.T) {
			for _, fault := range []string{"status", "stat"} {
				t.Run(fault, func(t *testing.T) {
					g, fake, root := finalizePreflightFixture(t)
					fake.listEntries = fake.listEntries[:1]
					wt := fake.listEntries[0].Path
					specWT := filepath.Join(root, ".worktrees", "worktree-spec-077-test")
					runGitIn(t, specWT, "merge", "--no-ff", "-m", "Merge bead/bead-1", "bead/bead-1")
					if resume {
						runGitIn(t, root, "merge", "--no-ff", "-m", "land spec", "spec/077-test")
						tip := refHash(t, root, "spec/077-test")
						oldRead := finalizeMetadataReadFn
						t.Cleanup(func() { finalizeMetadataReadFn = oldRead })
						finalizeMetadataReadFn = func(string) (map[string]interface{}, error) {
							return map[string]interface{}{"mindspec_finalize_tip": tip}, nil
						}
						mergeBindingReadFn = func(string) (map[string]interface{}, error) {
							return map[string]interface{}{
								"mindspec_landed_merge_sha":     tip,
								"mindspec_landed_second_parent": refHash(t, root, "bead/bead-1"),
							}, nil
						}
						runGitIn(t, root, "worktree", "remove", specWT)
						runGitIn(t, root, "branch", "-D", "spec/077-test")
					}
					path := filepath.Join(wt, "feature.txt")
					before, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					message := "cannot read already merged worktree status"
					marker := filepath.Join(t.TempDir(), "status-failed")
					if fault == "status" {
						git, err := exec.LookPath("git")
						if err != nil {
							t.Fatal(err)
						}
						bin := t.TempDir()
						// Fail only this worktree's status read. All other git
						// calls, including ancestry and cleanup, use real git.
						script := "#!/bin/sh\n" +
							"if [ \"$1\" = -C ] && [ \"$2\" = \"$FINALIZE_STATUS_WORKTREE\" ] && [ \"$3\" = status ]; then\n" +
							"  printf 'failed\\n' > \"$FINALIZE_STATUS_MARKER\"\n  exit 128\nfi\n" +
							"exec '" + git + "' \"$@\"\n"
						if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
							t.Fatal(err)
						}
						t.Setenv("FINALIZE_STATUS_WORKTREE", wt)
						t.Setenv("FINALIZE_STATUS_MARKER", marker)
						t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
					} else {
						// A symlink loop gives a non-NotExist stat error on all
						// users, including those that bypass permission bits.
						loop := filepath.Join(t.TempDir(), "loop")
						if err := os.Symlink(loop, loop); err != nil {
							t.Fatal(err)
						}
						if _, err := os.Stat(loop); err == nil || os.IsNotExist(err) {
							t.Fatalf("fixture needs a non-NotExist stat error: %v", err)
						}
						fake.listEntries[0].Path = loop
						message = "cannot inspect already merged worktree"
					}
					if resume {
						_, err = g.ResumeFinalizedEpic("epic-1", "077-test", []string{"bead-1"}, false)
					} else {
						_, err = g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "", false, false)
					}
					if err == nil || !strings.Contains(err.Error(), message) {
						t.Errorf("wrong refusal: %v; want %q", err, message)
					}
					if err != nil && !strings.HasSuffix(err.Error(), "recovery: mindspec impl approve 077-test") {
						t.Errorf("refusal has no runnable recovery: %v", err)
					}
					if len(fake.removeCalls) != 0 {
						t.Errorf("refusal reached cleanup: %v", fake.removeCalls)
					}
					if !branchExistsIn(t, root, "bead/bead-1") {
						t.Error("refusal removed bead branch")
					}
					if content, err := os.ReadFile(path); err != nil || string(content) != string(before) {
						t.Errorf("refusal lost bead content: content=%q error=%v", content, err)
					}
					if fault == "status" {
						if _, err := os.Stat(marker); err != nil {
							t.Errorf("status shim did not fire: %v", err)
						}
					}
				})
			}
		})
	}
}
