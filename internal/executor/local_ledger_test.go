package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitAll_LeavesPanelLedgerLocal(t *testing.T) {
	for _, file := range []string{".mindspec/specs/010-test/reviews/local/dispositions.jsonl", ".mindspec/docs/specs/010-test/reviews/local/dispositions.jsonl", "docs/specs/010-test/reviews/local/dispositions.jsonl", ".mindspec/reviews/adhoc/dispositions.jsonl", "review/legacy/dispositions.jsonl", "reviews/legacy/dispositions.jsonl", ".mindspec/review/legacy/dispositions.jsonl", ".mindspec/specs/010-test/review/legacy/dispositions.jsonl", ".mindspec/docs/specs/010-test/review/legacy/dispositions.jsonl", "docs/specs/010-test/review/legacy/dispositions.jsonl"} {
		t.Run(file, func(t *testing.T) {
			for _, state := range []string{"untracked", "ignored", "dirty-tracked", "staged-tracked"} {
				t.Run(state, func(t *testing.T) {
					root := newTempRepo(t)
					stubBeadExport(t)
					path := filepath.Join(root, file)
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("before\n"), 0644); err != nil {
						t.Fatal(err)
					}
					tracked := state == "dirty-tracked" || state == "staged-tracked"
					if tracked {
						runGitIn(t, root, "add", "--", file)
						runGitIn(t, root, "commit", "-qm", "fixture")
					}
					if state == "ignored" {
						if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(file+"\n"), 0644); err != nil {
							t.Fatal(err)
						}
					}
					if err := os.WriteFile(path, []byte("dirty\n"), 0644); err != nil {
						t.Fatal(err)
					}
					if state == "staged-tracked" {
						runGitIn(t, root, "add", "--", file)
					}
					if err := os.WriteFile(filepath.Join(root, "ordinary.txt"), []byte("ordinary\n"), 0644); err != nil {
						t.Fatal(err)
					}
					g := &MindspecExecutor{Root: root}
					if err := g.CommitAll(root, "lifecycle artifacts"); err != nil {
						t.Fatal(err)
					}
					if got, err := exec.Command("git", "-C", root, "show", "HEAD:ordinary.txt").Output(); err != nil || string(got) != "ordinary\n" {
						t.Fatalf("ordinary change not committed: %q", got)
					}
					data, err := exec.Command("git", "-C", root, "show", "HEAD:"+file).Output()
					if tracked && (err != nil || string(data) != "before\n") || !tracked && err == nil {
						t.Fatalf("lifecycle committed ledger: %s %v", data, err)
					}
					if err := checkMergedFinalizeWorktree(root, "010-test"); err != nil {
						t.Fatalf("ledger blocked cleanup: %v", err)
					}
					before, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
					if err != nil {
						t.Fatal(err)
					}
					if err := g.IsTreeClean(root); err != nil {
						t.Fatal(err)
					}
					if err := g.CommitAll(root, "ledger-only no-op"); err != nil {
						t.Fatalf("ledger-only state blocked commit funnel: %v", err)
					}
					after, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
					if err != nil || string(before) != string(after) {
						t.Fatalf("ledger-only call created a commit: %s %s %v", before, after, err)
					}
					if state == "staged-tracked" {
						data, err := exec.Command("git", "-C", root, "diff", "--cached", "--name-only", "--", file).Output()
						if err != nil || strings.TrimSpace(string(data)) != file {
							t.Fatalf("ledger staging lost: %s %v", data, err)
						}
					}
					if err := os.WriteFile(filepath.Join(root, "user.txt"), []byte("pending\n"), 0644); err != nil {
						t.Fatal(err)
					}
					if err := checkMergedFinalizeWorktree(root, "010-test"); err == nil {
						t.Fatal("pending source changes passed cleanup")
					}
				})
			}
		})
	}
}

func TestFinalizeLedgerLookalikes(t *testing.T) {
	for _, suffix := range []string{"\u00a0", "\u2003", " ", "\n"} {
		for _, staged := range []bool{false, true} {
			t.Run(fmt.Sprintf("%q/staged=%t", suffix, staged), func(t *testing.T) {
				root := newTempRepo(t)
				runGitIn(t, root, "config", "core.quotePath", "false")
				name := ".mindspec/specs/010-test/reviews/local/dispositions.jsonl" + suffix
				file := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte("user data"), 0644); err != nil {
					t.Fatal(err)
				}
				if staged {
					runGitIn(t, root, "add", "--", name)
				}
				if err := checkMergedFinalizeWorktree(root, "010-test"); err == nil {
					t.Fatal("lookalike exempted from finalize")
				}
				if err := (&MindspecExecutor{Root: root}).IsTreeClean(root); err == nil {
					t.Fatal("lookalike exempted from clean check")
				}
				state, err := (&MindspecExecutor{Root: root}).CompletionState(root, "", "")
				if err != nil {
					t.Fatal(err)
				}
				if staged && (len(state.StagedPaths) != 1 || state.StagedPaths[0] != name) {
					t.Fatalf("staged bytes changed: %#v", state.StagedPaths)
				}
			})
		}
	}
}
