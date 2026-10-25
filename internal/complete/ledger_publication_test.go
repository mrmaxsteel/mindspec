package complete

import (
	"errors"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComplete_TrackedLedgerClosing(t *testing.T) {
	for _, staged := range []bool{false, true} {
		t.Run(map[bool]string{false: "unstaged", true: "staged"}[staged], func(t *testing.T) {
			f := newWindowFixture(t)
			ledger := f.registerLedger()
			gitRun(t, f.specWT, "add", "-f", "--", ledger)
			gitRun(t, f.specWT, "commit", "-qm", "tracked ledger baseline")
			f.mergedOnly()
			tracked, _ := f.ex.FileAtRef("spec/"+closingSpec, ledger)
			t.Logf("trackedBeforeEdit=%t", len(tracked) > 0)
			file := filepath.Join(f.specWT, ledger)
			before, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			after := string(before) + "\n"
			if err := os.WriteFile(file, []byte(after), 0644); err != nil {
				t.Fatal(err)
			}
			if staged {
				gitRun(t, f.specWT, "add", "-f", "--", ledger)
			}
			result, err := f.run(CompleteOpts{})
			if err != nil {
				t.Fatal(err)
			}
			committed, err := f.ex.FileAtRef("spec/"+closingSpec, ledger)
			status, _ := f.ex.Status(f.specWT)
			t.Logf("closed=%t removed=%t status=%q committedCurrent=%t readErr=%v", result.BeadClosed, result.WorktreeRemoved, status, string(committed) == after, err)
			if err != nil || string(committed) != after || status != "" {
				t.Fatal("tracked ledger edit not published by closing")
			}
		})
	}
}
func TestComplete_TrackedLedgerPublishedRetry(t *testing.T) {
	f := newWindowFixture(t)
	ledger := f.registerLedger()
	gitRun(t, f.specWT, "add", "-f", "--", ledger)
	gitRun(t, f.specWT, "commit", "-qm", "tracked ledger fixture")
	f.mergedOnly()
	before, err := os.ReadFile(filepath.Join(f.specWT, ledger))
	if err != nil {
		t.Fatal(err)
	}
	after := append(append([]byte{}, before...), '\n')
	writeFile(t, f.specWT, ledger, string(after))
	gitRun(t, f.specWT, "add", "-f", "--", ledger)
	f.ex.cleanupErr = errors.New("cleanup interrupted")
	_, err = f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "closing step 6")
	published, err := f.ex.FileAtRef("spec/"+closingSpec, ledger)
	if err != nil || string(published) != string(after) {
		t.Fatalf("first publication missing ledger: %q %v", published, err)
	}
	// A changed ledger invalidates the already-published close on retry.
	after = append(after, '\n')
	writeFile(t, f.specWT, ledger, string(after))
	_, err = f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "closing step 6")
	committed, err := f.ex.FileAtRef("spec/"+closingSpec, ledger)
	if err != nil {
		t.Fatal(err)
	}
	status, err := f.ex.Status(f.specWT)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("committed_matches_working=%v status=%q", string(committed) == string(after), status)
	if string(committed) != string(after) {
		t.Fatal("tracked ledger edit was not published at close")
	}
	// With identical ledger bytes, another retry must skip publication.
	f.ex.publishErr = errors.New("publication should already be done")
	f.ex.cleanupErr = nil
	result, err := f.run(CompleteOpts{})
	if err != nil || result == nil || !result.WorktreeRemoved {
		t.Fatalf("published retry did not finish cleanup: %+v %v", result, err)
	}
}

func TestComplete_UnignoredLedgerPublication(t *testing.T) {
	for _, mergeOnly := range []bool{false, true} {
		for _, tracked := range []bool{false, true} {
			name := map[bool]string{false: "ordinary", true: "merged"}[mergeOnly] + "/" + map[bool]string{false: "untracked", true: "tracked"}[tracked]
			t.Run(name, func(t *testing.T) {
				f := newWindowFixture(t)
				ledger := f.registerLedgerWithExclude(false)
				if tracked {
					gitRun(t, f.specWT, "add", "--", ledger)
					gitRun(t, f.specWT, "commit", "-qm", "tracked ledger baseline")
				}
				if mergeOnly {
					f.mergedOnly()
				}
				before, err := os.ReadFile(filepath.Join(f.specWT, ledger))
				if err != nil {
					t.Fatal(err)
				}
				after := append(before, '\n')
				writeFile(t, f.specWT, ledger, string(after))
				result, err := f.run(CompleteOpts{})
				if err != nil || result == nil || !result.BeadClosed {
					t.Fatalf("completion failed: %+v %v", result, err)
				}
				committed, err := f.ex.FileAtRef("spec/"+closingSpec, ledger)
				if err != nil || string(committed) != string(after) {
					t.Fatalf("ledger edit not committed: %q %v", committed, err)
				}
			})
		}
	}
}

func TestUnignoredLedgersFailure(t *testing.T) {
	for _, failure := range []error{errors.New("git unavailable"), ledgerExitError(128)} {
		ex := &executor.MockExecutor{CheckIgnoreErr: failure}
		if _, err := unignoredLedgers("spec-worktree", []string{"ledger"}, ex); !errors.Is(err, failure) {
			t.Fatalf("lost check-ignore failure: %v", err)
		}
	}
}

type ledgerExitError int

func (e ledgerExitError) Error() string { return "check-ignore failed" }
func (e ledgerExitError) ExitCode() int { return int(e) }

func TestCommitPanelLedgersFailureRecovery(t *testing.T) {
	ex := &executor.MockExecutor{CommitPathsErr: errors.New("commit failed")}
	err := commitPanelLedgers("/spec tree", []string{"review/one's ledger"}, ex)
	if err == nil || !strings.Contains(err.Error(), "commit failed") || !strings.Contains(err.Error(), "git -C '/spec tree' add -- 'review/one'\"'\"'s ledger'") {
		t.Fatalf("missing scoped recovery: %v", err)
	}
}
