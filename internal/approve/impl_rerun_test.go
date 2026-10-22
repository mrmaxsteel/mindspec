package approve

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func TestApproveImpl_PostCloseConflictKeepsInnerRecovery(t *testing.T) {
	root, mock := approvePreflightFixture(t)
	inner := guard.NewFailure("merge conflict", "mindspec impl approve 010-test --resolve-merge")
	mock.FinalizeEpicErr = &executor.FinalizeStageError{Stage: "direct_merge", Err: inner}
	_, err := ApproveImpl(root, "010-test", mock)
	if err == nil || !strings.Contains(err.Error(), "epic closed; finalize stage direct_merge") {
		t.Fatalf("missing state line: %v", err)
	}
	if !strings.HasSuffix(err.Error(), inner.Error()) || strings.Count(err.Error(), "recovery:") != 1 {
		t.Fatalf("inner recovery changed: %v", err)
	}
}

type rerunWorktrees struct {
	t                   *testing.T
	root                string
	entries             []bead.WorktreeListEntry
	removeCalls         []string
	failRemove          bool
	failAfterSpecRemove bool
}

func (w *rerunWorktrees) List() ([]bead.WorktreeListEntry, error) { return w.entries, nil }
func (w *rerunWorktrees) Create(string, string) error             { return errors.New("unexpected create") }
func (w *rerunWorktrees) Remove(name string) error {
	w.removeCalls = append(w.removeCalls, name)
	if w.failRemove {
		return errors.New("cleanup fault")
	}
	path := filepath.Join(w.root, ".worktrees", name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	cmd := exec.Command("git", "-C", w.root, "worktree", "remove", "--force", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(string(out))
	}
	if w.failAfterSpecRemove && name == "worktree-spec-010-test" {
		return errors.New("fault after spec worktree removal")
	}
	return nil
}

type rerunFixture struct {
	t                                            *testing.T
	root, specWT, db, remote, pushLog, pushCalls string
	exec                                         *executor.MindspecExecutor
	worktrees                                    *rerunWorktrees
	probes                                       int
}

func (f *rerunFixture) read() map[string]phase.EpicInfo {
	f.t.Helper()
	data, err := os.ReadFile(f.db)
	if err != nil {
		f.t.Fatal(err)
	}
	var records map[string]phase.EpicInfo
	if err := json.Unmarshal(data, &records); err != nil {
		f.t.Fatal(err)
	}
	return records
}

func (f *rerunFixture) write(records map[string]phase.EpicInfo) {
	f.t.Helper()
	data, err := json.Marshal(records)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(f.db, data, 0600); err != nil {
		f.t.Fatal(err)
	}
}

func (f *rerunFixture) metadata(id string, updates map[string]interface{}) error {
	records := f.read()
	r := records[id]
	if r.Metadata == nil {
		r.Metadata = map[string]interface{}{}
	}
	for k, v := range updates {
		r.Metadata[k] = v
	}
	records[id] = r
	f.write(records)
	return nil
}

func newRerunFixture(t *testing.T, remote, orphan bool) *rerunFixture {
	t.Helper()
	root, _ := approvePreflightFixture(t)
	f := &rerunFixture{t: t, root: root, db: filepath.Join(t.TempDir(), "records.json")}
	f.write(map[string]phase.EpicInfo{
		"epic-parent": {ID: "epic-parent", Title: "[SPEC 010-test] Test", Status: "open", IssueType: "epic", Metadata: map[string]interface{}{"spec_num": 10, "spec_title": "test", "mindspec_phase": "review", "mindspec_phase_semantics": "v2"}},
		"bead-1":      {ID: "bead-1", Status: "closed", IssueType: "task", Metadata: map[string]interface{}{}},
	})
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!" + python + "\n" + `import json, os, sys
path = os.environ['FINALIZE_TEST_DB']
with open(path) as f: records = json.load(f)
args = sys.argv[1:]
if args[0] == 'show':
    print(json.dumps([records[args[1]]]))
elif args[0] == 'update':
    records[args[1]]['metadata'] = json.loads(args[args.index('--metadata')+1])
    with open(path, 'w') as f: json.dump(records, f)
elif args[0] == 'export':
    os.makedirs('.beads', exist_ok=True)
    with open('.beads/issues.jsonl', 'w') as f:
        for key in sorted(records): f.write(json.dumps(records[key], sort_keys=True)+'\n')
else:
    sys.exit('unexpected fake bd command: '+repr(args))
`
	if err := os.WriteFile(filepath.Join(bin, "bd"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FINALIZE_TEST_DB", f.db)
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "docs", "specs"), filepath.Join(root, ".mindspec", "specs")); err != nil {
		t.Fatal(err)
	}
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	domain := filepath.Join(root, ".mindspec", "domains", "workflow")
	if err := os.MkdirAll(domain, 0755); err != nil {
		t.Fatal(err)
	}
	write(".mindspec/domains/workflow/OWNERSHIP.yaml", "paths:\n  - spec.txt\n  - feature.txt\n  - internal/widget/**\n")
	write(".mindspec/domains/workflow/overview.md", "Workflow rules.\n")
	write(".mindspec/specs/010-test/spec.md", "# Spec\n\n## Impacted Domains\n\n- workflow\n")
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "adr"), 0755); err != nil {
		t.Fatal(err)
	}
	write(".mindspec/adr/ADR-0195.md", "# ADR-0195: Workflow\n\n- **Date**: 2026-01-01\n- **Status**: Accepted\n- **Domain(s)**: workflow\n- **Supersedes**: n/a\n- **Superseded-by**: n/a\n\n## Decision\nKeep workflow behavior.\n")
	write(".mindspec/specs/010-test/plan.md", "---\nspec_id: 010-test\nstatus: Approved\nbead_ids:\n  - bead-1\nadr_citations:\n  - id: ADR-0195\n---\n\n# Plan\n")

	write(".gitignore", ".worktrees/\n.mindspec/phase.lock\n")
	write(".mindspec/config.yaml", "enforcement:\n  panel_gate: false\n")
	preflightGit(t, root, "init", "-b", "main")
	preflightGit(t, root, "config", "user.name", "test")
	preflightGit(t, root, "config", "user.email", "test@example.invalid")
	preflightGit(t, root, "add", ".")
	preflightGit(t, root, "commit", "-m", "flat base")
	preflightGit(t, root, "switch", "-c", "spec/010-test")
	write("spec.txt", "spec work\n")
	preflightGit(t, root, "add", ".")
	preflightGit(t, root, "commit", "-m", "spec work")
	preflightGit(t, root, "branch", "bead/bead-1")
	preflightGit(t, root, "switch", "bead/bead-1")
	write("feature.txt", "bead work\n")
	preflightGit(t, root, "add", ".")
	preflightGit(t, root, "commit", "-m", "bead work")
	preflightGit(t, root, "switch", "main")
	f.specWT = filepath.Join(root, ".worktrees", "worktree-spec-010-test")
	preflightGit(t, root, "worktree", "add", f.specWT, "spec/010-test")
	f.worktrees = &rerunWorktrees{t: t, root: root, entries: []bead.WorktreeListEntry{{Name: "worktree-bead-1", Branch: "bead/bead-1"}}}
	f.exec = executor.NewMindspecExecutor(root)
	f.exec.WorktreeOps = f.worktrees
	if remote {
		f.remote = t.TempDir()
		preflightGit(t, f.remote, "init", "--bare", "-b", "main")
		preflightGit(t, root, "remote", "add", "origin", "file://"+f.remote)
		if orphan {
			preflightGit(t, f.specWT, "merge", "--no-ff", "-m", "Merge bead/bead-1", "bead/bead-1")
			merge := preflightGit(t, root, "rev-parse", "spec/010-test")
			second := preflightGit(t, root, "rev-parse", "bead/bead-1")
			_ = f.metadata("bead-1", map[string]interface{}{"mindspec_landed_merge_sha": merge, "mindspec_landed_second_parent": second})
			preflightGit(t, root, "merge", "--no-ff", "-m", "implementation landed", "spec/010-test")
		}
		preflightGit(t, root, "push", "origin", "main")
		f.pushLog = filepath.Join(t.TempDir(), "pushes")
		f.pushCalls = filepath.Join(t.TempDir(), "calls")
		prePush := "#!/bin/sh\nprintf 'CALL\\n' >> '" + f.pushCalls + "'\nwhile IFS= read -r line; do\nprintf '%s\\n' \"$line\" >> '" + f.pushCalls + "'\ncase \"$line\" in *chore/finalize-010-test*) if [ \"$FINALIZE_FAIL_CARRIER\" = 1 ]; then exit 1; fi;; esac\ndone\n"
		if err := os.WriteFile(filepath.Join(root, ".git", "hooks", "pre-push"), []byte(prePush), 0755); err != nil {
			t.Fatal(err)
		}
		// One log record per actual ref update. Dry-run probes do not invoke it.
		hook := "#!/bin/sh\ncat >> '" + f.pushLog + "'\n"
		if err := os.WriteFile(filepath.Join(f.remote, "hooks", "post-receive"), []byte(hook), 0755); err != nil {
			t.Fatal(err)
		}
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Error(err)
		}
	})
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, arg := range args {
			if arg == "--type=epic" {
				return json.Marshal([]phase.EpicInfo{f.read()["epic-parent"]})
			}
		}
		return json.Marshal([]phase.ChildInfo{{ID: "bead-1", Status: "closed", IssueType: "task"}})
	}))
	show := func(args ...string) ([]byte, error) { return json.Marshal([]phase.EpicInfo{f.read()[args[1]]}) }
	t.Cleanup(phase.SetRunBDForTest(show))
	implRunBDFn = show
	implPhaseMetadataFn = f.metadata
	implMergeMetadataFn = f.metadata
	implGetMetadataFn = func(id string) (map[string]interface{}, error) { return f.read()[id].Metadata, nil }
	implRunBDCombinedFn = func(args ...string) ([]byte, error) {
		records := f.read()
		r := records[args[1]]
		count, _ := r.Metadata["test_closes"].(float64)
		r.Metadata["test_closes"] = count + 1
		r.Status = "closed"
		records[args[1]] = r
		f.write(records)
		return nil, nil
	}
	implSpecBranchExistsFn = lifecycle.BranchExistsIn
	implHasRemoteFn = func() (bool, error) { f.probes++; return remote, nil }
	return f
}

func (f *rerunFixture) approve() (*ImplResult, error) {
	f.probes = 0
	return ApproveImpl(f.root, "010-test", f.exec)
}

func (f *rerunFixture) assertConverged() {
	f.t.Helper()
	records := f.read()
	if records["epic-parent"].Metadata["test_closes"] != float64(1) {
		f.t.Fatalf("close count: %+v", records["epic-parent"])
	}
	if records["epic-parent"].Status != "closed" || records["epic-parent"].Metadata["mindspec_phase"] != "done" {
		f.t.Fatal("lifecycle state did not converge")
	}
	if exists, err := lifecycle.BranchExistsIn(f.root, "spec/010-test"); err != nil || exists {
		f.t.Fatalf("spec branch survived: %v %v", exists, err)
	}
	targetRoot, target := f.root, "main"
	if f.remote != "" {
		targetRoot, target = f.remote, "spec/010-test"
		if exists, _ := lifecycle.BranchExistsIn(f.remote, target); !exists {
			target = "main"
		}
	}
	for _, value := range []interface{}{
		records["epic-parent"].Metadata["mindspec_finalize_tip"],
		records["bead-1"].Metadata["mindspec_landed_merge_sha"],
		records["bead-1"].Metadata["mindspec_landed_second_parent"],
	} {
		sha, _ := value.(string)
		if sha == "" {
			f.t.Fatal("publication or binding evidence is missing")
		}
		preflightGit(f.t, targetRoot, "merge-base", "--is-ancestor", sha, target)
	}
	if content := preflightGit(f.t, targetRoot, "show", target+":feature.txt"); content != "bead work" {
		f.t.Fatalf("bead content did not land: %q", content)
	}
	if f.remote == "" {
		return
	}
	data, err := os.ReadFile(f.pushLog)
	if err != nil {
		f.t.Fatal(err)
	}
	states := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			f.t.Fatalf("invalid push record: %q", line)
		}
		key := fields[2] + " " + fields[1]
		if states[key] {
			f.t.Fatalf("duplicate publication of %s", key)
		}
		states[key] = true
	}
	calls, err := os.ReadFile(f.pushCalls)
	if err != nil {
		f.t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
	for i := 0; i < len(lines); i += 2 {
		if lines[i] != "CALL" || i+1 >= len(lines) {
			f.t.Fatalf("push had no ref update: %q", calls)
		}
		fields := strings.Fields(lines[i+1])
		if len(fields) != 4 {
			f.t.Fatalf("push had no ref update: %q", calls)
		}
		if fields[0] != "refs/heads/spec/010-test" && fields[0] != "refs/heads/chore/finalize-010-test" {
			f.t.Fatalf("unexpected push ref: %q", fields[0])
		}
		if !states[fields[2]+" "+fields[1]] {
			f.t.Fatalf("push was not received: %q", lines[i+1])
		}
	}
}

func TestApproveImpl_RerunConvergesFromEachStage(t *testing.T) {
	source, err := parser.ParseFile(token.NewFileSet(), "../executor/mindspec_executor.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	hookStages := map[string]bool{}
	ast.Inspect(source, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name, ok := call.Fun.(*ast.Ident)
		if !ok || name.Name != "finalizeStepHook" {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			t.Fatal("finalize hook stage must be a literal")
		}
		stage, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Fatal(err)
		}
		hookStages[stage] = true
		return true
	})
	if !hookStages["orphan_finalize"] {
		t.Fatal("carrier hook is absent")
	}
	covered := map[string]map[string]bool{}
	// Stage names mirror every finalizeStepHookFn invocation. The direct
	// path has no push or carrier stage. It still runs all shared stages.
	for _, path := range []string{"direct", "push", "orphan"} {
		covered[path] = map[string]bool{}
		stages := []string{"close", "phase", "auto_merge", "pre_cleanup", "cleanup", "partial_cleanup", "post_cleanup"}
		if path != "direct" {
			stages = append(stages, "push")
		}
		if path == "orphan" {
			stages = append(stages, "orphan_before_push", "orphan_finalize")
		}
		for _, stage := range stages {
			t.Run(path+"/"+stage, func(t *testing.T) {
				f := newRerunFixture(t, path != "direct", path == "orphan")
				closeFn, phaseFn := implRunBDCombinedFn, implPhaseMetadataFn
				fired := false
				if stage == "close" {
					implRunBDCombinedFn = func(args ...string) ([]byte, error) {
						out, err := closeFn(args...)
						fired = true
						if err != nil {
							return out, err
						}
						return nil, errors.New("close fault after write")
					}
				} else if stage == "phase" {
					implPhaseMetadataFn = func(id string, updates map[string]interface{}) error {
						if err := phaseFn(id, updates); err != nil {
							return err
						}
						fired = true
						return errors.New("phase fault after write")
					}
				} else if stage == "cleanup" {
					f.worktrees.failRemove = true
				} else if stage == "partial_cleanup" {
					f.worktrees.failAfterSpecRemove = true
				} else if stage == "orphan_before_push" {
					t.Setenv("FINALIZE_FAIL_CARRIER", "1")
				} else {
					t.Cleanup(executor.SetFinalizeStepHookForTest(func(s string) error {
						if s == stage {
							fired = true
							covered[path][s] = true
							return errors.New("stage fault")
						}
						return nil
					}))
				}
				_, err := f.approve()
				if err == nil || !strings.Contains(err.Error(), "epic closed") {
					t.Fatalf("missing injected failure: %v", err)
				}
				if stage != "cleanup" && stage != "partial_cleanup" && stage != "orphan_before_push" && !fired {
					t.Fatal("fault stage was not reached")
				}
				if stage == "orphan_before_push" {
					if !strings.Contains(err.Error(), "stage orphan_finalize") {
						t.Fatalf("wrong carrier failure: %v", err)
					}
					if exists, _ := lifecycle.BranchExistsIn(f.remote, "chore/finalize-010-test"); exists {
						t.Fatal("carrier push did not fail")
					}
					t.Setenv("FINALIZE_FAIL_CARRIER", "0")
				}
				implRunBDCombinedFn, implPhaseMetadataFn = closeFn, phaseFn
				executor.SetFinalizeStepHookForTest(nil)
				f.worktrees.failRemove = false
				f.worktrees.failAfterSpecRemove = false
				if _, err := f.approve(); err != nil {
					t.Fatalf("retry: %v", err)
				}
				if path == "orphan" {
					// The local spec branch is gone. A further retry must use
					// the same carrier without another commit or push.
					before := preflightGit(t, f.remote, "rev-parse", "chore/finalize-010-test")
					result, err := f.approve()
					if err != nil || result.FinalizeBranch != "chore/finalize-010-test" {
						t.Fatalf("carrier resume after deletion: %+v %v", result, err)
					}
					if after := preflightGit(t, f.remote, "rev-parse", "chore/finalize-010-test"); after != before {
						t.Fatalf("carrier tip changed: %s -> %s", before, after)
					}
				}
				f.assertConverged()
			})
		}
	}
	for path, stages := range covered {
		for stage := range hookStages {
			if path == "direct" && (stage == "push" || stage == "orphan_finalize") {
				continue
			}
			if path == "push" && stage == "orphan_finalize" {
				continue
			}
			if !stages[stage] {
				t.Errorf("%s did not exercise hook stage %s", path, stage)
			}
		}
	}
}

func TestApproveImpl_RerunReRunsGates(t *testing.T) {
	f := newRerunFixture(t, false, false)
	restore := executor.SetFinalizeStepHookForTest(func(stage string) error {
		if stage == "auto_merge" {
			return errors.New("stop after merge")
		}
		return nil
	})
	t.Cleanup(restore)
	if _, err := f.approve(); err == nil || !strings.Contains(err.Error(), "stage auto_merge") {
		t.Fatalf("expected stage failure: %v", err)
	}
	executor.SetFinalizeStepHookForTest(nil)
	if err := os.MkdirAll(filepath.Join(f.specWT, "internal", "widget"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.specWT, "internal", "widget", "widget.go"), []byte("package widget\n"), 0644); err != nil {
		t.Fatal(err)
	}
	preflightGit(t, f.specWT, "add", "internal")
	preflightGit(t, f.specWT, "commit", "-m", "source without docs")
	_, err := f.approve()
	if err == nil || !strings.Contains(err.Error(), "doc-sync:") {
		t.Fatalf("re-run skipped doc-sync: %v", err)
	}
	if exists, _ := lifecycle.BranchExistsIn(f.root, "bead/bead-1"); !exists {
		t.Fatal("refusal removed bead branch")
	}
}

func TestApproveImpl_RerunKeepsDirtyMergedBead(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "push"}[remote], func(t *testing.T) {
			f := newRerunFixture(t, remote, false)
			wt := filepath.Join(f.root, ".worktrees", "worktree-bead-1")
			preflightGit(t, f.root, "worktree", "add", wt, "bead/bead-1")
			f.worktrees.entries[0].Path = wt
			t.Cleanup(executor.SetFinalizeStepHookForTest(func(stage string) error {
				if stage == "auto_merge" {
					return errors.New("stop after auto-merge")
				}
				return nil
			}))
			if _, err := f.approve(); err == nil || !strings.Contains(err.Error(), "stage auto_merge") {
				t.Fatalf("expected auto-merge failure: %v", err)
			}
			preflightGit(t, f.root, "merge-base", "--is-ancestor", "bead/bead-1", "spec/010-test")
			executor.SetFinalizeStepHookForTest(nil)
			path := filepath.Join(wt, "feature.txt")
			const correction = "pending correction\n"
			if err := os.WriteFile(path, []byte(correction), 0644); err != nil {
				t.Fatal(err)
			}
			_, err := f.approve()
			if err == nil || !strings.Contains(err.Error(), "already merged worktree has pending changes") {
				t.Errorf("retry must refuse dirty merged worktree: %v", err)
			}
			if err != nil && !strings.HasSuffix(err.Error(), "recovery: mindspec impl approve 010-test") {
				t.Errorf("missing runnable recovery: %v", err)
			}
			if content, readErr := os.ReadFile(path); readErr != nil || string(content) != correction {
				t.Fatalf("retry lost pending correction: content=%q error=%v", content, readErr)
			}
			if exists, _ := lifecycle.BranchExistsIn(f.root, "bead/bead-1"); !exists {
				t.Fatal("refusal removed bead branch")
			}
		})
	}
}

func TestApproveImpl_BranchMissingKeepsDirtyMergedBead(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "push"}[remote], func(t *testing.T) {
			f := newRerunFixture(t, remote, false)
			wt := filepath.Join(f.root, ".worktrees", "worktree-bead-1")
			preflightGit(t, f.root, "worktree", "add", wt, "bead/bead-1")
			f.worktrees.entries[0].Path = wt
			t.Cleanup(executor.SetFinalizeStepHookForTest(func(stage string) error {
				if stage == "pre_cleanup" {
					return errors.New("stop before cleanup")
				}
				return nil
			}))
			if _, err := f.approve(); err == nil || !strings.Contains(err.Error(), "stop before cleanup") {
				t.Fatalf("expected publication before cleanup: %v", err)
			}
			executor.SetFinalizeStepHookForTest(nil)
			// Enter the supported branch-missing recovery state with a
			// surviving bead worktree and valid publication and child evidence.
			preflightGit(t, f.root, "worktree", "remove", f.specWT)
			preflightGit(t, f.root, "branch", "-D", "spec/010-test")
			f.assertConverged()
			path := filepath.Join(wt, "feature.txt")
			const correction = "pending correction after publication\n"
			if err := os.WriteFile(path, []byte(correction), 0644); err != nil {
				t.Fatal(err)
			}
			result, err := f.approve()
			if err == nil || !strings.Contains(err.Error(), "already merged worktree has pending changes") {
				t.Errorf("branch-missing retry must refuse dirty merged worktree: %v", err)
			}
			if err != nil && !strings.Contains(err.Error(), "recovery: mindspec impl approve 010-test") {
				t.Errorf("missing runnable recovery: %v", err)
			}
			if result != nil && strings.Contains(strings.Join(result.Warnings, "\n"), "already landed") {
				t.Error("dirty surviving worktree reported already landed")
			}
			if len(f.worktrees.removeCalls) != 0 {
				t.Errorf("refusal reached forced cleanup: %v", f.worktrees.removeCalls)
			}
			if content, readErr := os.ReadFile(path); readErr != nil || string(content) != correction {
				t.Errorf("retry lost pending correction: content=%q error=%v", content, readErr)
			}
			if exists, err := lifecycle.BranchExistsIn(f.root, "bead/bead-1"); err != nil || !exists {
				t.Errorf("refusal removed bead branch: exists=%v error=%v", exists, err)
			}
		})
	}
}

func TestApproveImpl_AfterDeletionAlreadyLanded(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "push"}[remote], func(t *testing.T) {
			f := newRerunFixture(t, remote, false)
			if _, err := f.approve(); err != nil {
				t.Fatal(err)
			}
			result, err := f.approve()
			if err != nil || result == nil || !strings.Contains(strings.Join(result.Warnings, "\n"), "already landed") {
				t.Fatalf("re-run: %+v %v", result, err)
			}
			if f.probes != 1 {
				t.Fatalf("has-remote calls = %d, want 1", f.probes)
			}
			f.assertConverged()
		})
	}
}

func TestApproveImpl_RemoteSpecBranchDeletedUsesMain(t *testing.T) {
	f := newRerunFixture(t, true, false)
	if _, err := f.approve(); err != nil {
		t.Fatal(err)
	}
	// The host merges the PR and deletes the spec branch.
	preflightGit(t, f.remote, "update-ref", "refs/heads/main", preflightGit(t, f.remote, "rev-parse", "refs/heads/spec/010-test"))
	preflightGit(t, f.remote, "update-ref", "-d", "refs/heads/spec/010-test")
	result, err := f.approve()
	if err != nil || result == nil || !strings.Contains(strings.Join(result.Warnings, "\n"), "already landed") {
		t.Fatalf("re-run: %+v %v", result, err)
	}
	if f.probes != 1 {
		t.Fatalf("has-remote calls = %d, want 1", f.probes)
	}
	if exists, _ := lifecycle.BranchExistsIn(f.remote, "spec/010-test"); exists {
		t.Fatal("re-run recreated deleted remote branch")
	}
	f.assertConverged()
}

func TestApproveImpl_TargetFetchFailureRefuses(t *testing.T) {
	f := newRerunFixture(t, true, false)
	if _, err := f.approve(); err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = fetch ]; then exit 1; fi\nexec '" + git + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err = f.approve()
	if err == nil || !strings.Contains(err.Error(), "fetching finalize target") || !strings.HasSuffix(err.Error(), implBranchMissingRefusal("010-test", "spec/010-test").Error()) {
		t.Fatalf("fetch refusal: %v", err)
	}
}

func TestApproveImpl_ForgedFinalizeTipRefuses(t *testing.T) {
	for _, tipRef := range []string{"main", "main~1"} {
		t.Run(tipRef, func(t *testing.T) {
			f := newRerunFixture(t, false, false)
			preflightGit(t, f.root, "commit", "--allow-empty", "-m", "main advances")
			records := f.read()
			r := records["epic-parent"]
			r.Status = "closed"
			r.Metadata["mindspec_phase"] = "done"
			r.Metadata["mindspec_finalize_tip"] = preflightGit(t, f.root, "rev-parse", tipRef)
			records["epic-parent"] = r
			// Even a well-formed binding cannot pass when its commits are unlanded.
			bead := records["bead-1"]
			bead.Metadata = map[string]interface{}{}
			sha := preflightGit(t, f.root, "rev-parse", "bead/bead-1")
			bead.Metadata["mindspec_landed_merge_sha"], bead.Metadata["mindspec_landed_second_parent"] = sha, sha
			records["bead-1"] = bead
			f.write(records)
			preflightGit(t, f.root, "worktree", "remove", f.specWT)
			preflightGit(t, f.root, "branch", "-D", "spec/010-test")
			_, err := f.approve()
			if err == nil || !strings.HasSuffix(err.Error(), implBranchMissingRefusal("010-test", "spec/010-test").Error()) {
				t.Fatalf("forged tip passed: %v", err)
			}
			if exists, _ := lifecycle.BranchExistsIn(f.root, "bead/bead-1"); !exists {
				t.Fatal("forged tip removed bead branch")
			}
		})
	}
}

func TestApproveImpl_UnlandedChildBindingRefusesAfterBeadDeletion(t *testing.T) {
	for _, key := range []string{"mindspec_landed_merge_sha", "mindspec_landed_second_parent"} {
		t.Run(key, func(t *testing.T) {
			f := newRerunFixture(t, false, false)
			landed := preflightGit(t, f.root, "rev-parse", "main")
			unlanded := preflightGit(t, f.root, "rev-parse", "bead/bead-1")
			f.setResumeEvidence(landed, landed)
			if err := f.metadata("bead-1", map[string]interface{}{key: unlanded}); err != nil {
				t.Fatal(err)
			}
			preflightGit(t, f.root, "worktree", "remove", f.specWT)
			preflightGit(t, f.root, "branch", "-D", "spec/010-test", "bead/bead-1")
			f.assertBranchMissingRefusal()
		})
	}
}

func TestApproveImpl_UnlandedFinalizeTipRefusesWithLandedBindings(t *testing.T) {
	f := newRerunFixture(t, false, false)
	// Every binding and the surviving bead branch are landed. Only the
	// publication tip points outside main, so cleanup cannot cause the refusal.
	preflightGit(t, f.root, "merge", "--no-ff", "-m", "land bead", "bead/bead-1")
	landed := preflightGit(t, f.root, "rev-parse", "main")
	preflightGit(t, f.specWT, "commit", "--allow-empty", "-m", "unpublished export")
	unlanded := preflightGit(t, f.root, "rev-parse", "spec/010-test")
	f.setResumeEvidence(unlanded, landed)
	preflightGit(t, f.root, "worktree", "remove", f.specWT)
	preflightGit(t, f.root, "branch", "-D", "spec/010-test")
	f.assertBranchMissingRefusal()
	if exists, _ := lifecycle.BranchExistsIn(f.root, "bead/bead-1"); !exists {
		t.Fatal("unlanded publication tip removed bead branch")
	}
}

func (f *rerunFixture) setResumeEvidence(tip, binding string) {
	f.t.Helper()
	records := f.read()
	r := records["epic-parent"]
	r.Status = "closed"
	r.Metadata["mindspec_phase"] = "done"
	r.Metadata["mindspec_finalize_tip"] = tip
	records["epic-parent"] = r
	r = records["bead-1"]
	if r.Metadata == nil {
		r.Metadata = map[string]interface{}{}
	}
	r.Metadata["mindspec_landed_merge_sha"] = binding
	r.Metadata["mindspec_landed_second_parent"] = binding
	records["bead-1"] = r
	f.write(records)
}

func (f *rerunFixture) assertBranchMissingRefusal() {
	f.t.Helper()
	result, err := f.approve()
	if err == nil || !strings.HasSuffix(err.Error(), implBranchMissingRefusal("010-test", "spec/010-test").Error()) {
		f.t.Errorf("unlanded evidence must give branch-missing refusal: %v", err)
	}
	if result != nil && strings.Contains(strings.Join(result.Warnings, "\n"), "already landed") {
		f.t.Error("unlanded evidence printed already landed")
	}
}

func TestApproveImpl_StateLineReadsEpicStatus(t *testing.T) {
	root, mock := approvePreflightFixture(t)
	implRunBDCombinedFn = func(...string) ([]byte, error) { return nil, errors.New("close failed") }
	implRunBDFn = func(args ...string) ([]byte, error) {
		if args[1] == "epic-parent" {
			return []byte(`[{"status":"open"}]`), nil
		}
		return []byte(`[{"status":"closed"}]`), nil
	}
	_, err := ApproveImpl(root, "010-test", mock)
	if err == nil || !strings.Contains(err.Error(), "epic still open") || !strings.Contains(err.Error(), "stage close") {
		t.Fatalf("missing actual state: %v", err)
	}
	if len(mock.CallsTo("FinalizeEpic")) != 0 {
		t.Fatal("close failure reached finalize")
	}
}

func TestApproveImpl_PostCloseBareErrorGetsRerun(t *testing.T) {
	root, mock := approvePreflightFixture(t)
	mock.FinalizeEpicErr = &executor.FinalizeStageError{Stage: "push", Err: errors.New("push failed\x1b[31m")}
	_, err := ApproveImpl(root, "010-test", mock)
	if err == nil || !strings.Contains(err.Error(), "epic closed") || !strings.HasSuffix(err.Error(), "recovery: mindspec impl approve 010-test") || strings.ContainsRune(err.Error(), '\x1b') {
		t.Fatalf("missing safe re-run recovery: %v", err)
	}
}
