package approve

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

type finalGateFixExecutor struct {
	executor.Executor
	statusPaths   []string
	finalizeCalls int
}

func (e *finalGateFixExecutor) Status(path string) (string, error) {
	e.statusPaths = append(e.statusPaths, path)
	return e.Executor.Status(path)
}

func (e *finalGateFixExecutor) FinalizeEpic(epicID, specID, branch string, allow []string, reason string, resolve, remote bool) (executor.FinalizeResult, error) {
	e.finalizeCalls++
	return e.Executor.FinalizeEpic(epicID, specID, branch, allow, reason, resolve, remote)
}

func TestS133FinalGate_MissingWorktreeRefusesUnpublishedSubject(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, state := range []struct{ status, phase string }{{"open", "review"}, {"open", "done"}, {"closed", "review"}} {
			t.Run(policy+"/"+state.status+"/"+state.phase, func(t *testing.T) {
				f := s133FinalFixtureNew(t, policy, 1, "pass")
				s133Write(t, f.specWT, "feature.txt", "source after review\n")
				s133Commit(t, f.specWT, "unreviewed source")
				if _, err := runLifecycleGate(f.root, "010-test", "final_review", f.specWT, nil); err == nil || !strings.Contains(err.Error(), "commits landed after review") {
					t.Fatalf("fixture must have a stale final review: %v", err)
				}
				r := f.read()
				epic := r["epic-parent"]
				epic.Status, epic.Metadata["mindspec_phase"] = state.status, state.phase
				r["epic-parent"] = epic
				f.write(r)
				preflightGit(t, f.root, "worktree", "remove", f.specWT)
				beforeRefs := preflightGit(t, f.root, "show-ref")
				beforeDB, err := os.ReadFile(f.db)
				if err != nil {
					t.Fatal(err)
				}
				closeCalls := 0
				closeFn := implRunBDCombinedFn
				implRunBDCombinedFn = func(args ...string) ([]byte, error) {
					if len(args) > 0 && args[0] == "close" {
						closeCalls++
					}
					return closeFn(args...)
				}
				t.Cleanup(func() { implRunBDCombinedFn = closeFn })
				exec := &finalGateFixExecutor{Executor: f.exec}
				lifecycleCalls := 0
				lifecycleFn := lifecycleExecutorFn
				lifecycleExecutorFn = func(root string) executor.Executor {
					lifecycleCalls++
					return lifecycleFn(root)
				}
				t.Cleanup(func() { lifecycleExecutorFn = lifecycleFn })
				_, err = ApproveImpl(f.root, "010-test", exec)
				if err == nil || !strings.Contains(err.Error(), "gate final_review") || !strings.HasSuffix(err.Error(), "recovery: re-create the spec worktree at spec/010-test, then run a final-review panel") {
					t.Errorf("missing-worktree gate must refuse with its recovery: %v", err)
				}
				afterDB, err := os.ReadFile(f.db)
				if err != nil || !bytes.Equal(beforeDB, afterDB) || beforeRefs != preflightGit(t, f.root, "show-ref") {
					t.Error("missing-worktree refusal changed refs or epic state")
				}
				if closeCalls != 0 || exec.finalizeCalls != 0 {
					t.Errorf("refusal reached close or finalization: close=%d finalize=%d", closeCalls, exec.finalizeCalls)
				}
				if lifecycleCalls != 0 {
					t.Errorf("missing-worktree gate entered disk registration or dirt checks: %d executor calls", lifecycleCalls)
				}
				for _, path := range exec.statusPaths {
					if path == f.root {
						t.Error("missing-worktree gate checked primary checkout dirt")
					}
				}
			})
		}
	}
}

func TestS133FinalGate_SupersedeCollisionPreservesExistingFile(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, kind := range []string{"handwritten", "extra text", "symlink"} {
			t.Run(policy+"/"+kind, func(t *testing.T) {
				f := s133FinalFixtureNew(t, policy, 1, "absent")
				const id = "ADR-9800"
				name := filepath.Join(f.specWT, ".mindspec/adr/"+id+".md")
				var target string
				switch kind {
				case "handwritten":
					s133Write(t, f.specWT, ".mindspec/adr/"+id+".md", "# ADR-9800: Existing decision\n\nHand-written content.\n")
				case "extra text":
					if _, err := adr.CreateWithID(f.specWT, id, "Placeholder for "+id, adr.CreateOpts{}); err != nil {
						t.Fatal(err)
					}
					data, err := os.ReadFile(name)
					if err != nil {
						t.Fatal(err)
					}
					s133Write(t, f.specWT, ".mindspec/adr/"+id+".md", string(data)+"\nAdditional decision text.\n")
				case "symlink":
					var err error
					target, err = adr.CreateWithID(t.TempDir(), id, "Placeholder for "+id, adr.CreateOpts{})
					if err != nil {
						t.Fatal(err)
					}
					if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, name); err != nil {
						t.Fatal(err)
					}
				}
				s133Commit(t, f.specWT, "existing decision")
				// Review the existing file so the placeholder collision is the only failing check.
				s133SeedPanel(t, f, "final_review", "spec/010-test", "", "pass", "final")
				s133Commit(t, f.specWT, "review existing decision")
				before := f.snapshot()
				data, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				closeCalls := 0
				closeFn := implRunBDCombinedFn
				implRunBDCombinedFn = func(args ...string) ([]byte, error) {
					if len(args) > 0 && args[0] == "close" {
						closeCalls++
					}
					return closeFn(args...)
				}
				t.Cleanup(func() { implRunBDCombinedFn = closeFn })
				exec := &finalGateFixExecutor{Executor: f.exec}
				_, err = ApproveImpl(f.root, "010-test", exec, ImplOpts{SupersedeADR: id})
				if err == nil || !strings.Contains(err.Error(), id) || !strings.Contains(err.Error(), "already exists") {
					t.Errorf("existing file must refuse with its identity: %v", err)
				}
				if closeCalls != 0 || exec.finalizeCalls != 0 {
					t.Errorf("collision reached close or finalization: close=%d finalize=%d", closeCalls, exec.finalizeCalls)
				}
				if after := f.snapshot(); before != after {
					t.Error("collision changed refs, worktree, epic status or phase")
				}
				after, readErr := os.ReadFile(name)
				if readErr != nil || !bytes.Equal(data, after) {
					t.Errorf("collision changed file bytes: %v", readErr)
				}
				if target != "" {
					if got, err := os.Readlink(name); err != nil || got != target {
						t.Errorf("collision changed symlink: %q %v", got, err)
					}
				}
			})
		}
	}
}
