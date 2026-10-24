package complete

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func ledgerFixture(t *testing.T, ledger bool) (string, string, []string, *executor.MockExecutor) {
	t.Helper()
	saveAndRestore(t)
	root := setupTempRoot(t)
	wt := filepath.Join(root, ".worktrees", "worktree-spec-008-test")
	reviews := filepath.Join(wt, ".mindspec", "specs", "008-test", "reviews")
	for _, slug := range []string{"a", "b", "other"} {
		dir := filepath.Join(reviews, slug)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		id := "bead-1"
		if slug == "other" {
			id = "bead-2"
		}
		if err := os.WriteFile(filepath.Join(dir, "panel.json"), []byte(`{"bead_id":"`+id+`"}`), 0644); err != nil {
			t.Fatal(err)
		}
		if ledger {
			if err := os.WriteFile(filepath.Join(dir, "dispositions.jsonl"), []byte("{}\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	stubPhaseEpic(t, "008-test", "mol-parent-1")
	resolveTargetFn = func(string, string) (string, error) { return "008-test", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	closeBeadFn = func(...string) error { return nil }
	runBDFn = func(...string) ([]byte, error) { return []byte("[]"), nil }
	// Keep panel decision checks separate from ledger git state.
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("enforcement:\n  panel_gate: false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	paths := []string{".mindspec/specs/008-test/reviews/a/dispositions.jsonl", ".mindspec/specs/008-test/reviews/b/dispositions.jsonl"}
	return root, wt, paths, newMockExec()
}

func TestMaterialize_LedgerStaysLocal(t *testing.T) {
	root, wt, _, mock := ledgerFixture(t, false)
	ledger := ".mindspec/specs/008-test/reviews/a/dispositions.jsonl"
	// Verify the snapshot pattern with real git in a temp repo.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", wt}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(wt, ledger), []byte("before\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git("add", "--", ledger)
	git("-c", "user.name=Test", "-c", "user.email=test@mindspec.local", "commit", "-qm", "base")
	if err := os.WriteFile(filepath.Join(wt, ledger), []byte("after\n"), 0644); err != nil {
		t.Fatal(err)
	}
	realExec := &executor.MindspecExecutor{}
	tree, err := realExec.SnapshotWorktreeTree(wt, panel.ToolArtifactPaths(filepath.Join(wt, ".mindspec", "specs")))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", wt, "show", tree+":"+ledger)
	out, err := cmd.Output()
	if err != nil || string(out) != "before\n" {
		t.Fatalf("M-stage did not preserve ledger at tip: %s %v", out, err)
	}
	if got := nonArtifactPaths([]string{ledger}, filepath.Join(wt, ".mindspec", "specs")); len(got) != 0 {
		t.Fatalf("ledger classified as user dirt: %v", got)
	}
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
		return []bead.WorktreeListEntry{{Path: wt, Branch: "bead/bead-1"}}, nil
	}
	checkDirtyTreeFn = func(string, string) ([]string, []string, error) { return []string{ledger}, nil, nil }
	mock.SnapshotWorktreeTreeFn = func(_ string, excludes []string) (string, error) {
		if len(excludes) == 2 {
			want := []string{".beads/issues.jsonl", ".mindspec/specs/*/reviews/*/dispositions.jsonl"}
			if !reflect.DeepEqual(excludes, want) {
				t.Fatalf("wrong M-stage exclusion paths: %v", excludes)
			}
			return "without-ledger", nil
		}
		if !reflect.DeepEqual(excludes, panel.LedgerArtifactPaths(filepath.Join(wt, ".mindspec", "specs"))) {
			t.Fatalf("S-stage must exclude local ledgers: %v", excludes)
		}
		return "without-ledger", nil
	}
	mock.ChangedFilesTreeFn = func(_, _, tree string) ([]string, error) {
		if tree == "with-ledger" {
			return []string{ledger}, nil
		}
		return nil, nil
	}
	if _, err := Run(root, "bead-1", "", "", mock, CompleteOpts{}); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range mock.CallsTo("MaterializeStageCommit") {
		if c.Args[3] == "artifact-sync" {
			found = true
		}
	}
	if found {
		t.Fatal("dirty ledger was committed by S-stage")
	}

}

func TestComplete_LocalLedgers(t *testing.T) {
	for _, state := range []string{"untracked", "ignored", "dirty-tracked"} {
		t.Run(state, func(t *testing.T) {
			root, wt, paths, mock := ledgerFixture(t, true)
			git := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", wt, "-c", "core.hooksPath=/dev/null", "-c", "user.name=Test", "-c", "user.email=test@example.invalid"}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("git: %v %s", err, out)
				}
				return string(out)
			}
			git("init", "-q")
			if state == "ignored" {
				if err := os.WriteFile(filepath.Join(wt, ".gitignore"), []byte(".mindspec/specs/*/reviews/**\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if state == "dirty-tracked" {
				git("add", "--", paths[0])
				git("commit", "-qm", "fixture")
				if err := os.WriteFile(filepath.Join(wt, paths[0]), []byte("dirty\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			real := &executor.MindspecExecutor{}
			mock.CheckIgnoreFn = real.CheckIgnore
			mock.StatusFn = real.Status
			if _, err := Run(root, "bead-1", "", "", mock, CompleteOpts{}); err != nil {
				t.Fatal(err)
			}
			if len(mock.CallsTo("CommitPaths")) != 0 || len(mock.CallsTo("CheckIgnore")) != 0 {
				t.Fatal("completion performed ledger git operations")
			}
			if state == "dirty-tracked" && git("show", "HEAD:"+paths[0]) != "{}\n" {
				t.Fatal("completion committed dirty ledger")
			}
		})
	}
}
