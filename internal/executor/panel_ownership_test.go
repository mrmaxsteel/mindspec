package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestConsumerLedgerNamesAreUserData(t *testing.T) {
	for _, name := range []string{"review/customer/dispositions.jsonl", "reviews/customer/dispositions.jsonl"} {
		for _, tracked := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/tracked=%t", name, tracked), func(t *testing.T) {
				root := newTempRepo(t)
				file := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte("ordinary consumer data\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if len(panel.Scan(root)) != 0 {
					t.Fatal("fixture has registered panel")
				}
				if tracked {
					runGitIn(t, root, "add", "--", name)
					runGitIn(t, root, "commit", "-qm", "consumer fixture")
					if err := os.WriteFile(file, []byte("dirty consumer data\n"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				g := &MindspecExecutor{Root: root}
				if err := g.IsTreeClean(root); err == nil {
					t.Error("IsTreeClean accepted ordinary untracked consumer file; zero panels")
				}
				if err := checkMergedFinalizeWorktree(root, "010-test"); err == nil {
					t.Error("merged finalize accepted ordinary consumer file")
				}
				if err := checkBeadCleanupWorktree(root, "bead-1"); err == nil {
					t.Error("bead cleanup accepted ordinary consumer file")
				}
				stubBeadExport(t)
				if err := g.CommitAll(root, "should not silently omit user data"); err != nil {
					t.Fatal(err)
				}
				out, err := exec.Command("git", "-C", root, "ls-files", "--", name).Output()
				if err != nil {
					t.Fatal(err)
				}
				if got, err := exec.Command("git", "-C", root, "show", "HEAD:"+name).Output(); err != nil || string(got) != string(mustReadLedger(t, file)) {
					t.Fatalf("consumer edit not committed: %q %v", got, err)
				}
				if len(out) == 0 {
					t.Error("CommitAll silently omitted ordinary consumer file")
				}
			})
		}
	}
}
func TestAdhocMaterializationLedgerExclusion(t *testing.T) {
	root := newTempRepo(t)
	name := ".mindspec/reviews/adhoc/dispositions.jsonl"
	file := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := panel.Create(filepath.Dir(file), panel.CreateInput{Gate: "adhoc", Target: "main", Round: 1, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:fixture", Roster: []panel.RosterSlot{{Slot: "R1", Family: "harness", Model: "m", Lens: "correctness"}, {Slot: "R2", Family: "harness", Model: "m", Lens: "tests"}}}); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, root, "add", "--", name)
	runGitIn(t, root, "commit", "-qm", "tracked ledger fixture")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	dirty := append(append([]byte{}, before...), '\n')
	if err := os.WriteFile(file, dirty, 0644); err != nil {
		t.Fatal(err)
	}
	ledgers, err := panelLedgerPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := gitutil.SnapshotWorktreeTree(root, append([]string{".beads/issues.jsonl"}, ledgers...))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", root, "show", tree+":"+name).Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(before) {
		t.Errorf("M-stage snapshot includes dirty tracked adhoc ledger: before=%d bytes snapshot=%d bytes", len(before), len(out))
	}
}

func mustReadLedger(t *testing.T, file string) []byte {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestLedgerOwnershipDeletedHEAD(t *testing.T) {
	for _, staged := range []bool{false, true} {
		t.Run(fmt.Sprint(staged), func(t *testing.T) {
			root := newTempRepo(t)
			ledger, reg := "review/local/dispositions.jsonl", "review/local/panel.json"
			if err := os.MkdirAll(filepath.Join(root, "review/local"), 0755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{ledger, reg} {
				if err := os.WriteFile(filepath.Join(root, name), []byte("{}"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			runGitIn(t, root, "add", "--", ledger, reg)
			runGitIn(t, root, "commit", "-qm", "panel fixture")
			if staged {
				runGitIn(t, root, "rm", "--", ledger, reg)
			} else {
				for _, name := range []string{ledger, reg} {
					if err := os.Remove(filepath.Join(root, name)); err != nil {
						t.Fatal(err)
					}
				}
			}
			// Staged deletion plus a recreated file must still preserve the HEAD ledger.
			if staged {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, ledger)), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ledger), []byte("recreated"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			paths, err := panelLedgerPaths(root)
			if err != nil || !slices.Contains(paths, ledger) {
				t.Fatalf("HEAD registration lost deleted ledger: %v %v", paths, err)
			}
			tree, err := gitutil.SnapshotWorktreeTree(root, paths)
			if err != nil {
				t.Fatal(err)
			}
			data, err := exec.Command("git", "-C", root, "show", tree+":"+ledger).Output()
			if err != nil || string(data) != "{}" {
				t.Fatalf("deleted ledger changed in snapshot: %q %v", data, err)
			}
		})
	}
}
func TestCommitAll_LiteralRegisteredLedger(t *testing.T) {
	root := newTempRepo(t)
	stubBeadExport(t)
	for _, name := range []string{"review/[x]/panel.json", "review/[x]/dispositions.jsonl", "review/x/dispositions.jsonl"} {
		file := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := panelLedgerPaths(root)
	if err != nil || !slices.Contains(paths, "review/[x]/dispositions.jsonl") {
		t.Fatalf("literal panel not resolved: %v %v; scan=%+v", paths, err, panel.Scan(root))
	}
	if err := (&MindspecExecutor{Root: root}).CommitAll(root, "literal panel exclusion"); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.Command("git", "-C", root, "cat-file", "-e", "HEAD:review/[x]/dispositions.jsonl").Output(); err == nil {
		listing, _ := exec.Command("git", "-C", root, "ls-tree", "-r", "HEAD").Output()
		t.Fatalf("registered ledger committed: paths=%q tree=%s", paths, listing)
	}
	if got, err := exec.Command("git", "-C", root, "show", "HEAD:review/x/dispositions.jsonl").Output(); err != nil || string(got) != "data" {
		t.Fatalf("consumer wildcard neighbor omitted: %q %v", got, err)
	}
}
