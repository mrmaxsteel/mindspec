package complete

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
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
	// A config hatch must not skip ledger handling. The env hatch is tested too.
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("enforcement:\n  panel_gate: false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	paths := []string{".mindspec/specs/008-test/reviews/a/dispositions.jsonl", ".mindspec/specs/008-test/reviews/b/dispositions.jsonl"}
	return root, wt, paths, newMockExec()
}

func TestComplete_CommitsLedger(t *testing.T) {
	for _, hatch := range []string{"0", "1"} {
		t.Run(hatch, func(t *testing.T) {
			t.Setenv("MINDSPEC_SKIP_PANEL", hatch)
			root, wt, paths, mock := ledgerFixture(t, true)
			t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
			t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
			t.Setenv("GIT_TERMINAL_PROMPT", "0")
			t.Setenv("GIT_ASKPASS", "/usr/bin/false")
			for name, value := range map[string]string{"GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "test@mindspec.local", "GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "test@mindspec.local"} {
				t.Setenv(name, value)
			}
			git := func(args ...string) string {
				t.Helper()
				cmd := exec.Command("git", append([]string{"-C", wt}, args...)...)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v %s", args, err, out)
				}
				return string(out)
			}
			git("init", "-q")
			realExec := &executor.MindspecExecutor{}
			mock.CheckIgnoreFn = realExec.CheckIgnore

			mock.CommitPathsFn = func(gotWT, msg string, gotPaths []string) error {
				if len(mock.CallsTo("CompleteBead")) != 1 {
					t.Fatal("ledger committed before bead merge")
				}
				if gotWT != wt || msg != "chore(panel): ledger" || !reflect.DeepEqual(gotPaths, paths) {
					t.Fatalf("commit: %s %s %v", gotWT, msg, gotPaths)
				}
				return realExec.CommitPaths(gotWT, msg, gotPaths)
			}
			if _, err := Run(root, "bead-1", "", "", mock, CompleteOpts{}); err != nil {
				t.Fatal(err)
			}
			if len(mock.CallsTo("CommitPaths")) != 1 {
				t.Fatal("expected one ledger commit")
			}
			for _, file := range paths {
				if got := git("show", "HEAD:"+file); got != "{}\n" {
					t.Fatalf("ledger not committed: %s %q", file, got)
				}
			}
			if got := git("log", "-1", "--format=%s"); got != "chore(panel): ledger\n" {
				t.Fatalf("wrong ledger subject: %q", got)
			}
			if got := git("ls-files"); strings.Contains(got, "other") || strings.Contains(got, "panel.json") {
				t.Fatalf("committed unrelated paths: %s", got)
			}
			checks := mock.CallsTo("CheckIgnore")
			if len(checks) != 2 || checks[0].Args[0] != wt || checks[1].Args[0] != wt {
				t.Fatalf("wrong check-ignore checkout: %v", checks)
			}
		})
	}
}

func TestComplete_NoLedgerNoCommit(t *testing.T) {
	root, _, _, mock := ledgerFixture(t, false)
	var runErr error
	var stdout string
	stderr := captureStderr(t, func() {
		stdout = captureStdout(t, func() {
			_, runErr = Run(root, "bead-1", "", "", mock, CompleteOpts{})
		})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if strings.Contains(stderr+stdout, "recovery:") || strings.Contains(stderr+stdout, "chore(panel): ledger") {
		t.Fatalf("no-ledger completion printed a recovery: stdout=%q stderr=%q", stdout, stderr)
	}
	if len(mock.CallsTo("CommitPaths")) != 0 || len(mock.CallsTo("CheckIgnore")) != 0 {
		t.Fatal("legacy panel caused ledger I/O")
	}
}

func TestLedger_IgnoredPathRefuses(t *testing.T) {
	for _, hatch := range []string{"0", "1"} {
		t.Run(hatch, func(t *testing.T) {
			t.Setenv("MINDSPEC_SKIP_PANEL", hatch)
			root, wt, _, mock := ledgerFixture(t, true)
			mock.CheckIgnoreFn = func(string, string) error { return nil }
			closeBeadFn = func(...string) error { t.Fatal("closed before ignore refusal"); return nil }
			_, err := Run(root, "bead-1", "", "", mock, CompleteOpts{})
			if err == nil || !strings.Contains(err.Error(), "mindspec init") || !strings.Contains(err.Error(), "git -C '"+wt+"'") {
				t.Fatalf("missing spec-worktree recovery: %v", err)
			}
			if len(mock.CallsTo("CompleteBead")) != 0 || len(mock.CallsTo("CommitPaths")) != 0 || len(mock.CallsTo("ReadStageMarker")) != 0 {
				t.Fatal("mutated after ignored ledger")
			}
		})
	}
}

func TestComplete_CommitsLedgerFailureRecovery(t *testing.T) {
	t.Setenv("MINDSPEC_SKIP_PANEL", "1")
	root, wt, paths, mock := ledgerFixture(t, true)
	mock.CommitPathsErr = errors.New("commit failed")
	mock.ReadStageMarkerFn = func(string) (string, string, bool, error) {
		// The marker becomes present only after the mocked merge succeeds.
		return "terminal-marker", "", len(mock.CallsTo("CompleteBead")) == 1, nil
	}
	metadata := map[string]map[string]interface{}{}
	completeMergeMetadataFn = func(id string, values map[string]interface{}) error {
		if metadata[id] == nil {
			metadata[id] = map[string]interface{}{}
		}
		for k, v := range values {
			metadata[id][k] = v
		}
		return nil
	}
	var runErr error
	stderr := captureStderr(t, func() {
		_, runErr = Run(root, "bead-1", "", "", mock, CompleteOpts{})
	})
	if runErr == nil || !strings.Contains(runErr.Error(), LedgerCommitRecovery(wt, paths)) {
		t.Errorf("missing commit recovery: %v", runErr)
	}
	if strings.Count(stderr, "recovery: "+LedgerCommitRecovery(wt, paths)) != 1 || strings.Count(stderr, "bead is closed and merged; only the panel ledger commit remains") != 1 {
		t.Errorf("missing printed post-merge recovery: %q", stderr)
	}
	var reported *ReportedError
	if !errors.As(runErr, &reported) {
		t.Errorf("printed failure is not marked as reported: %v", runErr)
	}
	if len(mock.CallsTo("ClearStageMarker")) != 1 {
		t.Errorf("terminal marker was not cleared: %v", mock.CallsTo("ClearStageMarker"))
	}
	if metadata["bead-1"]["panel_gate_skipped"] != true {
		t.Errorf("missing panel hatch audit: %v", metadata)
	}
	if metadata["mol-parent-1"]["mindspec_phase"] != "implement" {
		t.Errorf("missing post-close phase sync: %v", metadata)
	}
}

func TestComplete_CommitsLedgerFailureWithStateFailure(t *testing.T) {
	for _, failure := range []string{"advance", "phase sync"} {
		t.Run(failure, func(t *testing.T) {
			root, wt, paths, mock := ledgerFixture(t, true)
			mock.CommitPathsErr = errors.New("commit failed")
			stateErr := errors.New(failure + " failed")
			if failure == "advance" {
				completeFetchChildrenFn = func(string) ([]phase.ChildInfo, error) {
					return nil, stateErr
				}
			} else {
				completeMergeMetadataFn = func(_ string, values map[string]interface{}) error {
					if _, ok := values["mindspec_phase"]; ok {
						return stateErr
					}
					return nil
				}
			}
			var result *Result
			var runErr error
			stderr := captureStderr(t, func() {
				result, runErr = Run(root, "bead-1", "", "", mock, CompleteOpts{})
			})
			if runErr == nil || !strings.Contains(runErr.Error(), "commit failed") || !strings.Contains(runErr.Error(), stateErr.Error()) {
				t.Fatalf("ledger and state failures must both survive: %v", runErr)
			}
			var reported *ReportedError
			if !errors.As(runErr, &reported) {
				t.Errorf("printed failure is not marked as reported: %v", runErr)
			}
			if strings.Count(stderr, "recovery: "+LedgerCommitRecovery(wt, paths)) != 1 {
				t.Errorf("ledger recovery must print once: %q", stderr)
			}
			if result == nil || !result.BeadClosed || len(mock.CallsTo("CompleteBead")) != 1 {
				t.Fatalf("failure must follow close and merge: result=%+v", result)
			}
			if result.NextModeIndeterminate != (failure == "advance") {
				t.Errorf("wrong next-mode certainty: %+v", result)
			}
		})
	}
}

func TestComplete_CanonicalLedgerNoIO(t *testing.T) {
	root, wt, _, mock := ledgerFixture(t, true)
	canonical := filepath.Join(wt, ".mindspec", "docs", "specs")
	if err := os.MkdirAll(filepath.Dir(canonical), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(wt, ".mindspec", "specs"), canonical); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(root, "bead-1", "", "", mock, CompleteOpts{}); err != nil {
		t.Fatal(err)
	}
	if len(mock.CallsTo("CompleteBead")) != 1 {
		t.Fatal("canonical-layout completion never reached the merge")
	}
	if len(mock.CallsTo("CheckIgnore")) != 0 || len(mock.CallsTo("CommitPaths")) != 0 {
		t.Fatal("canonical-layout ledger caused flat-layout ledger I/O")
	}
}

func TestMaterialize_LedgerIsArtifact(t *testing.T) {
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
		if len(excludes) > 0 {
			want := []string{".beads/issues.jsonl", ".mindspec/specs/*/reviews/*/dispositions.jsonl"}
			if !reflect.DeepEqual(excludes, want) {
				t.Fatalf("wrong M-stage exclusion paths: %v", excludes)
			}
			return "without-ledger", nil
		}
		return "with-ledger", nil
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
	if !found {
		t.Fatal("dirty ledger never reached S-stage")
	}

}

func TestLedger_RecoveryOperands(t *testing.T) {
	for _, operand := range []string{"/checkout/spec", "/checkout/space and ' quote; $(false) `false`", "/checkout/line\nforge"} {
		quoted := ledgerShellArg(operand)
		cmd := exec.Command("/bin/sh", "-c", "printf '%s' "+quoted)
		out, err := cmd.Output()
		want := operand
		if strings.Contains(operand, "\n") {
			want = `"/checkout/line\nforge"`
		}
		if err != nil || string(out) != want {
			t.Fatalf("shell operand changed: %q => %q (%v)", operand, out, err)
		}
		mock := &executor.MockExecutor{CheckIgnoreFn: func(string, string) error { return nil }}
		refusal := CheckPanelLedgers(operand, []string{"ledger"}, mock)
		if refusal == nil {
			t.Fatal("missing refusal")
		}
		for _, recovery := range []string{LedgerCommitRecovery(operand, []string{operand}), finalRecoveryCommand(t, refusal.Error())} {
			if matches := guard.FindFloorMatches(recovery); len(matches) != 0 {
				t.Fatalf("recovery matches destructive floor: %v %s", matches, recovery)
			}
			if strings.ContainsAny(recovery, "\n\r\x1b") {
				t.Fatalf("terminal controls in recovery: %q", recovery)
			}
		}
	}
}
