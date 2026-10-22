package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/doctor"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
)

const mergeLoopSpec = "132-window"
const mergeLoopEpic = "mindspec-loop"
const mergeLoopBead = "mindspec-loop.1"

type mergeLoopFixture struct {
	t                           *testing.T
	root, specWT, beadWT, state string
	ex                          *executor.MindspecExecutor
	db                          map[string]interface{}
}

func loopGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func loopWrite(t *testing.T, root, path, body string) {
	t.Helper()
	file := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}
func loopBead(id string, deps ...string) map[string]interface{} {
	dependencies := []map[string]interface{}{{"id": mergeLoopEpic, "dependency_type": "parent-child"}}
	for _, dep := range deps {
		dependencies = append(dependencies, map[string]interface{}{"id": dep, "dependency_type": "blocks"})
	}
	return map[string]interface{}{"id": id, "title": "[132] implement verification", "issue_type": "task", "status": "open", "description": "Implement the accepted behavior.", "dependencies": dependencies, "parent": mergeLoopEpic, "metadata": map[string]interface{}{"spec_id": mergeLoopSpec}}
}
func loopPlan(ids []string) string {
	text := "---\nstatus: Approved\nspec_id: 132-window\nversion: \"1\"\nbead_ids:\n"
	for _, id := range ids {
		text += "  - " + id + "\n"
	}
	text += "work_chunks:\n"
	for i := range ids {
		text += fmt.Sprintf("  - id: %d\n    title: Verification\n    key_file_paths:\n      - README.md\n", i+1)
	}
	text += "---\n\n# Plan\n"
	for i := range ids {
		text += fmt.Sprintf("\n## Bead %d: Verification\n\n**Acceptance Criteria**\n- The verification file has the expected content.\n\n**Key File Paths**\n- README.md\n", i+1)
	}
	return text
}
func newMergeLoopFixture(t *testing.T) *mergeLoopFixture {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd); config.ResetCache() })
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")
	f := &mergeLoopFixture{t: t, root: t.TempDir()}
	f.specWT = filepath.Join(f.root, ".worktrees", "worktree-spec-"+mergeLoopSpec)
	f.beadWT = filepath.Join(f.specWT, ".worktrees", "worktree-"+mergeLoopBead)
	f.state = filepath.Join(t.TempDir(), "state.json")
	f.db = map[string]interface{}{"issues": map[string]interface{}{
		mergeLoopEpic: map[string]interface{}{"id": mergeLoopEpic, "title": "[SPEC 132-window] Verification", "issue_type": "epic", "status": "open", "metadata": map[string]interface{}{"spec_num": 132, "spec_title": "window", "mindspec_phase": "implement", "mindspec_phase_semantics": "v2"}},
		mergeLoopBead: loopBead(mergeLoopBead), "mindspec-loop.2": loopBead("mindspec-loop.2", mergeLoopBead)},
		"worktrees": []map[string]interface{}{{"name": "worktree-spec-" + mergeLoopSpec, "branch": "spec/" + mergeLoopSpec, "path": f.specWT}, {"name": "worktree-" + mergeLoopBead, "branch": "bead/" + mergeLoopBead, "path": f.beadWT}},
	}
	f.db["issues"].(map[string]interface{})[mergeLoopBead].(map[string]interface{})["status"] = "in_progress"
	f.save()
	bin := t.TempDir()
	script := `#!/usr/bin/env python3
import json,os,sys,subprocess
path=os.environ["MERGE_LOOP_STATE_FILE"]
db=json.load(open(path)); issues=db["issues"]; args=sys.argv[1:]
def save():
 with open(path,"w") as f: json.dump(db,f)
def value(flag):
 for i,a in enumerate(args):
  if a.startswith(flag+"="): return a.split("=",1)[1]
  if a==flag and i+1<len(args): return args[i+1]
 return None
def expanded(issue):
 row=dict(issue); row["dependencies"]=[]
 for dep in issue.get("dependencies",[]):
  other=dict(issues.get(dep["id"],{})); other.update(dep)
  row["dependencies"].append(other)
 return row
cmd=args[0]
if cmd=="show": print(json.dumps([expanded(issues[args[1]])]))
elif cmd in ("list","ready"):
 rows=list(issues.values())
 typ=value("--type"); parent=value("--parent"); status=value("--status")
 if typ: rows=[r for r in rows if r.get("issue_type")==typ]
 if parent: rows=[r for r in rows if r.get("parent")==parent]
 if status and status!="all": rows=[r for r in rows if r.get("status") in status.split(",")]
 if cmd=="ready": rows=[r for r in rows if r["status"]=="open" and all(issues[d["id"]]["status"]=="closed" for d in r.get("dependencies",[]) if d["dependency_type"]=="blocks")]
 print(json.dumps([expanded(r) for r in rows]))
elif cmd=="update":
 row=issues[args[1]]
 if value("--metadata"): row.setdefault("metadata",{}).update(json.loads(value("--metadata")))
 for i,a in enumerate(args):
  if a=="--unset-metadata": row.setdefault("metadata",{}).pop(args[i+1],None)
 if value("--status"): row["status"]=value("--status")
 if "--claim" in args: row["status"]="in_progress"
 save()
elif cmd=="close":
 for id in args[1:]:
  if not id.startswith("-"): issues[id]["status"]="closed"
 save()
elif cmd=="dolt": pass
elif cmd=="export":
 os.makedirs(".beads",exist_ok=True)
 with open(value("-o") or ".beads/issues.jsonl","w") as f:
  for id in sorted(issues): f.write(json.dumps(issues[id],sort_keys=True)+"\n")
elif cmd=="worktree" and args[1]=="list": print(json.dumps(db["worktrees"]))
elif cmd=="worktree" and args[1]=="remove":
 rows=[r for r in db["worktrees"] if r["name"]==args[2]]
 for row in rows: subprocess.run(["git","-C",os.environ["MERGE_LOOP_ROOT"],"worktree","remove","--force",row["path"]],check=True)
 db["worktrees"]=[r for r in db["worktrees"] if r not in rows]; save()
else:
 print("unsupported fake bd argv: "+repr(args),file=sys.stderr);sys.exit(1)
`
	if err := os.WriteFile(filepath.Join(bin, "bd"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MERGE_LOOP_STATE_FILE", f.state)
	t.Setenv("MERGE_LOOP_ROOT", f.root)
	loopWrite(t, f.root, ".mindspec/config.yaml", "source_globs:\n  - internal/**/*.go\n")
	loopWrite(t, f.root, ".mindspec/specs/"+mergeLoopSpec+"/spec.md", "# Spec\n\n## Goal\nVerify a merge before close.\n\n## Impacted Domains\n\n- workflow\n")
	loopWrite(t, f.root, ".mindspec/specs/"+mergeLoopSpec+"/plan.md", loopPlan([]string{mergeLoopBead, "mindspec-loop.2"}))
	loopWrite(t, f.root, ".gitignore", ".worktrees/\n")
	loopWrite(t, f.root, "README.md", "base\n")
	loopGit(t, f.root, "init", "-q", "-b", "main")
	loopGit(t, f.root, "add", "-A")
	loopGit(t, f.root, "commit", "-qm", "base")
	loopGit(t, f.root, "branch", "spec/"+mergeLoopSpec)
	loopGit(t, f.root, "checkout", "-qb", "bead/"+mergeLoopBead)
	loopWrite(t, f.root, "README.md", "implemented\n")
	loopGit(t, f.root, "commit", "-qam", "implement")
	loopGit(t, f.root, "checkout", "-q", "main")
	loopGit(t, f.root, "worktree", "add", f.specWT, "spec/"+mergeLoopSpec)
	loopGit(t, f.root, "worktree", "add", f.beadWT, "bead/"+mergeLoopBead)
	if err := os.Chdir(f.root); err != nil {
		t.Fatal(err)
	}
	config.ResetCache()
	f.ex = executor.NewMindspecExecutor(f.root)
	return f
}
func (f *mergeLoopFixture) save() {
	f.t.Helper()
	data, err := json.Marshal(f.db)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(f.state, data, 0644); err != nil {
		f.t.Fatal(err)
	}
}
func (f *mergeLoopFixture) load() {
	f.t.Helper()
	data, err := os.ReadFile(f.state)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := json.Unmarshal(data, &f.db); err != nil {
		f.t.Fatal(err)
	}
}
func (f *mergeLoopFixture) complete(id string, opts complete.CompleteOpts) *complete.Result {
	f.t.Helper()
	r, err := complete.Run(f.root, id, mergeLoopSpec, "", f.ex, opts)
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func (f *mergeLoopFixture) assertOpenMergeDetected() {
	f.t.Helper()
	report := doctor.RunLifecycleIntegrityChecksForTest(f.root)
	found := false
	for _, check := range report.Checks {
		if strings.Contains(check.Name, mergeLoopBead) && check.Status == doctor.Info {
			found = true
		}
	}
	if !found {
		f.t.Fatalf("doctor did not detect the open valid merge: %+v", report.Checks)
	}
	readinessReport, err := readiness.EvaluateReadiness(f.root, "mindspec-loop.2")
	if err != nil {
		f.t.Fatal(err)
	}
	blocked := false
	for _, signal := range readinessReport.Signals {
		if signal.ID == readiness.SignalDependencies && !signal.Pass {
			blocked = true
		}
	}
	if !blocked {
		f.t.Fatal("MF-3 must fail while the predecessor stays open")
	}
}

func (f *mergeLoopFixture) assertCleanAndReady() {
	f.t.Helper()
	report := doctor.RunLifecycleIntegrityChecksForTest(f.root)
	if report.HasFailures() {
		f.t.Fatalf("doctor lifecycle failed: %+v", report.Checks)
	}
	for _, check := range report.Checks {
		if strings.Contains(check.Name, mergeLoopBead) {
			f.t.Fatalf("doctor still reports the closed bead: %+v", check)
		}
	}
	gate, err := next.GateReadiness(f.root, "mindspec-loop.2", next.GateOptions{})
	if err != nil {
		f.t.Fatalf("dependent readiness failed: %v (%+v)", err, gate)
	}
	reportReady, readyErr := readiness.EvaluateReadiness(f.root, "mindspec-loop.2")
	if readyErr != nil {
		f.t.Fatal(readyErr)
	}
	if gate == nil || reportReady == nil {
		f.t.Fatal("missing readiness report")
	}
	found := false
	for _, signal := range reportReady.Signals {
		if signal.ID == readiness.SignalDependencies {
			found = true
			if !signal.Pass {
				f.t.Fatalf("MF-3 failed: %+v", signal)
			}
		}
	}
	if !found {
		f.t.Fatal("MF-3 was not evaluated")
	}
}
func (f *mergeLoopFixture) finishDependent() {
	f.t.Helper()
	f.load()
	rows := f.db["worktrees"].([]interface{})
	path := filepath.Join(f.specWT, ".worktrees", "worktree-mindspec-loop.2")
	loopGit(f.t, f.root, "worktree", "add", "-b", "bead/mindspec-loop.2", path, "spec/"+mergeLoopSpec)
	loopWrite(f.t, path, "README.md", "verified\n")
	loopGit(f.t, path, "add", "-A")
	loopGit(f.t, path, "commit", "-qm", "dependent")
	f.db["worktrees"] = append(rows, map[string]interface{}{"name": "worktree-mindspec-loop.2", "branch": "bead/mindspec-loop.2", "path": path})
	f.save()
	if _, err := next.GateReadiness(f.root, "mindspec-loop.2", next.GateOptions{}); err != nil {
		f.t.Fatal(err)
	}
	if err := next.ClaimBead("mindspec-loop.2"); err != nil {
		f.t.Fatal(err)
	}
	f.complete("mindspec-loop.2", complete.CompleteOpts{})
	if _, err := approve.ApproveImpl(f.root, mergeLoopSpec, f.ex); err != nil {
		f.t.Fatalf("impl approve failed: %v", err)
	}
	f.load()
	epic := f.db["issues"].(map[string]interface{})[mergeLoopEpic].(map[string]interface{})
	if epic["status"] != "closed" || epic["metadata"].(map[string]interface{})["mindspec_phase"] != "done" {
		f.t.Fatalf("approval did not finish: %v", epic)
	}
	if report := doctor.RunLifecycleIntegrityChecksForTest(f.root); report.HasFailures() {
		f.t.Fatalf("doctor after approval: %+v", report.Checks)
	}
}
func TestComplete_MergeOnlyRestoreLoop(t *testing.T) {
	f := newMergeLoopFixture(t)
	r := f.complete(mergeLoopBead, complete.CompleteOpts{MergeOnly: true})
	f.assertOpenMergeDetected()
	loopGit(t, f.specWT, "revert", "-m", "1", "--no-edit", r.MergeSHA)
	if _, err := complete.Run(f.root, mergeLoopBead, mergeLoopSpec, "", f.ex, complete.CompleteOpts{}); err == nil {
		t.Fatal("reverted merge closed")
	}
	loopGit(t, f.specWT, "checkout", r.MergeSHA, "--", "README.md")
	loopGit(t, f.specWT, "commit", "-qm", "restore merged paths")
	f.complete(mergeLoopBead, complete.CompleteOpts{})
	f.assertCleanAndReady()
	f.finishDependent()
}
func TestComplete_MergeOnlyFollowUpLoop(t *testing.T) {
	f := newMergeLoopFixture(t)
	r := f.complete(mergeLoopBead, complete.CompleteOpts{MergeOnly: true})
	f.assertOpenMergeDetected()
	parent := r.MergeParents[1]
	loopWrite(t, f.beadWT, "README.md", "fixed\n")
	loopGit(t, f.beadWT, "commit", "-qam", "fix after verification")
	tip := loopGit(t, f.root, "rev-parse", "bead/"+mergeLoopBead)
	if _, err := complete.Run(f.root, mergeLoopBead, mergeLoopSpec, "", f.ex, complete.CompleteOpts{}); err == nil {
		t.Fatal("advanced tip closed")
	}
	loopGit(t, f.root, "branch", mergeLoopBead+"-followup", tip)
	loopGit(t, f.beadWT, "reset", "--keep", parent)
	f.complete(mergeLoopBead, complete.CompleteOpts{})
	f.assertCleanAndReady()
	f.load()
	f.db["issues"].(map[string]interface{})["mindspec-loop.3"] = loopBead("mindspec-loop.3", mergeLoopBead)
	loopWrite(t, f.specWT, ".mindspec/specs/"+mergeLoopSpec+"/plan.md", loopPlan([]string{mergeLoopBead, "mindspec-loop.2", "mindspec-loop.3"}))
	loopGit(t, f.specWT, "add", ".mindspec/specs/"+mergeLoopSpec+"/plan.md")
	loopGit(t, f.specWT, "commit", "-qm", "plan follow-up")
	path := filepath.Join(f.specWT, ".worktrees", "worktree-mindspec-loop.3")
	loopGit(t, f.root, "worktree", "add", "-b", "bead/mindspec-loop.3", path, "spec/"+mergeLoopSpec)
	loopGit(t, path, "cherry-pick", parent+".."+mergeLoopBead+"-followup")
	loopGit(t, f.root, "branch", "-D", mergeLoopBead+"-followup")
	f.db["worktrees"] = append(f.db["worktrees"].([]interface{}), map[string]interface{}{"name": "worktree-mindspec-loop.3", "branch": "bead/mindspec-loop.3", "path": path})
	f.save()
	if _, err := next.GateReadiness(f.root, "mindspec-loop.3", next.GateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := next.ClaimBead("mindspec-loop.3"); err != nil {
		t.Fatal(err)
	}
	f.complete("mindspec-loop.3", complete.CompleteOpts{})
	f.finishDependent()
}
