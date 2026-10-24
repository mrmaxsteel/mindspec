package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitAll_LeavesPanelLedgerLocal(t *testing.T) {
	for _, state := range []string{"untracked", "ignored", "dirty-tracked", "staged-tracked"} {
		t.Run(state, func(t *testing.T) {
			root := newTempRepo(t)
			stubBeadExport(t)
			file := ".mindspec/specs/010-test/reviews/local/dispositions.jsonl"
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
				if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".mindspec/specs/*/reviews/**\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(path, []byte("dirty\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if state == "staged-tracked" {
				runGitIn(t, root, "add", "--", file)
			}
			g := &MindspecExecutor{Root: root}
			if err := g.CommitAll(root, "lifecycle artifacts"); err != nil {
				t.Fatal(err)
			}
			data, err := exec.Command("git", "-C", root, "show", "HEAD:"+file).Output()
			if tracked && (err != nil || string(data) != "before\n") || !tracked && err == nil {
				t.Fatalf("lifecycle committed ledger: %s %v", data, err)
			}
			if err := checkMergedFinalizeWorktree(root, "010-test"); err != nil {
				t.Fatalf("ledger blocked cleanup: %v", err)
			}
			if err := g.CommitAll(root, "ledger-only no-op"); err != nil {
				t.Fatalf("ledger-only state blocked commit funnel: %v", err)
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
}
