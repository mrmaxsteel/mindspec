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
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
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
	text := "---\nstatus: Approved\nspec_id: 132-window\nversion: \"1\"\nadr_citations:\n  - ADR-9001\nbead_ids:\n"
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
elif cmd=="create":
 id="mindspec-loop.3"
 issues[id]={"id":id,"title":args[1],"issue_type":"task","status":"open","description":"Implement the follow-up fix.","parent":value("--parent"),"dependencies":[{"id":value("--parent"),"dependency_type":"parent-child"},{"id":value("--deps"),"dependency_type":"blocks"}],"metadata":{"spec_id":"132-window"}}
 save(); print(json.dumps(issues[id]))
elif cmd=="dolt": pass
elif cmd=="export":
 os.makedirs(".beads",exist_ok=True)
 with open(value("-o") or ".beads/issues.jsonl","w") as f:
  for id in sorted(issues): f.write(json.dumps(issues[id],sort_keys=True)+"\n")
elif cmd=="worktree" and args[1]=="create":
 wt_path=os.path.abspath(args[2]); branch=value("--branch")
 subprocess.run(["git","worktree","add",wt_path,branch],check=True)
 db["worktrees"].append({"name":os.path.basename(wt_path),"branch":branch,"path":wt_path});save()
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
	loopWrite(t, f.root, ".mindspec/domains/workflow/OWNERSHIP.yaml", "paths:\n  - feature.md\n")
	loopWrite(t, f.root, ".mindspec/adr/ADR-9001.md", "# ADR-9001: Verification\n\n- **Status**: Accepted\n- **Domain(s)**: workflow\n\n## Decision\nOwn verification files.\n")
	loopWrite(t, f.root, ".gitignore", ".worktrees/\n")
	loopWrite(t, f.root, "README.md", "base\n")
	loopGit(t, f.root, "init", "-q", "-b", "main")
	loopGit(t, f.root, "add", "-A")
	loopGit(t, f.root, "commit", "-qm", "base")
	loopGit(t, f.root, "branch", "spec/"+mergeLoopSpec)
	loopGit(t, f.root, "checkout", "-qb", "bead/"+mergeLoopBead)
	loopWrite(t, f.root, "README.md", "implemented\n")
	loopWrite(t, f.root, "feature.md", "feature\n")
	loopGit(t, f.root, "add", "README.md", "feature.md")
	loopGit(t, f.root, "commit", "-qm", "implement")
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

// recoveryLines keeps the command operands from the refusal. Git commands run
// unchanged in the temp repo. Lifecycle commands call their Go entry points so
// the test needs no installed binary. The fake tracker handles tracker commands.
func recoveryLines(t *testing.T, err error) []string {
	t.Helper()
	if err == nil {
		t.Fatal("expected a refusal with recovery lines")
	}
	var lines []string
	for _, line := range strings.Split(err.Error(), "\n") {
		if strings.HasPrefix(line, "recovery: ") {
			lines = append(lines, strings.TrimPrefix(line, "recovery: "))
		}
	}
	if len(lines) == 0 {
		t.Fatalf("no recovery lines: %v", err)
	}
	return lines
}
func (f *mergeLoopFixture) shellRecovery(line string) {
	f.t.Helper()
	cmd := exec.Command("sh", "-c", line)
	cmd.Dir = f.root
	if out, err := cmd.CombinedOutput(); err != nil {
		f.t.Fatalf("printed recovery %s: %v\n%s", line, err, out)
	}
}
func (f *mergeLoopFixture) completeRecovery(line string) {
	f.t.Helper()
	args := strings.Fields(line)
	if len(args) != 3 || args[0] != "mindspec" || args[1] != "complete" {
		f.t.Fatalf("unexpected complete recovery: %s", line)
	}
	f.complete(args[2], complete.CompleteOpts{})
}
func (f *mergeLoopFixture) nextRecovery(line string) string {
	f.t.Helper()
	args := strings.Fields(line)
	if len(args) != 3 || args[0] != "mindspec" || args[1] != "next" {
		f.t.Fatalf("unexpected next recovery: %s", line)
	}
	id := args[2]
	// These are next's gate, claim and dispatch entry points. No override is set.
	if _, err := next.GateReadiness(f.root, id, next.GateOptions{}); err != nil {
		f.t.Fatal(err)
	}
	if err := next.ClaimBead(id); err != nil {
		f.t.Fatal(err)
	}
	path, err := next.EnsureWorktree(f.root, id, mergeLoopSpec, f.ex)
	if err != nil {
		f.t.Fatal(err)
	}
	return path
}
func (f *mergeLoopFixture) recordPanel(id, path string, round int) {
	f.t.Helper()
	specDir, err := workspace.SpecDir(f.root, mergeLoopSpec)
	if err != nil {
		f.t.Fatal(err)
	}
	tip := loopGit(f.t, path, "rev-parse", "HEAD")
	// A fixture panel supplies local verdicts. No model or panel CLI runs here.
	p := panel.Panel{BeadID: &id, Spec: mergeLoopSpec, Target: "bead/" + id, Gate: "bead", Round: round, ExpectedReviewers: 2, ReviewedHeadSHA: tip}
	data, err := json.Marshal(p)
	if err != nil {
		f.t.Fatal(err)
	}
	dir := filepath.Join(specDir, "reviews", id)
	loopWrite(f.t, dir, "panel.json", string(data))
	for _, slot := range []string{"R1", "R2"} {
		loopWrite(f.t, dir, fmt.Sprintf("%s-round-%d.json", slot, round), `{"verdict":"APPROVE"}`)
	}
	regs := panel.ForBead(panel.Scan(specDir), id)
	if len(regs) != 1 || regs[0].Panel.ReviewedHeadSHA != tip || regs[0].Panel.Round != round {
		f.t.Fatalf("panel not recorded at tip: %+v", regs)
	}
}
func TestComplete_MergeOnlyRestoreLoop(t *testing.T) {
	for _, kind := range []string{"full", "partial"} {
		t.Run(kind, func(t *testing.T) {
			f := newMergeLoopFixture(t)
			r := f.complete(mergeLoopBead, complete.CompleteOpts{MergeOnly: true})
			f.assertOpenMergeDetected()
			if kind == "full" {
				loopGit(t, f.specWT, "revert", "-m", "1", "--no-edit", r.MergeSHA)
			} else {
				loopWrite(t, f.specWT, "README.md", "base\n")
				loopGit(t, f.specWT, "commit", "-qam", "partial revert")
			}
			_, err := complete.Run(f.root, mergeLoopBead, mergeLoopSpec, "", f.ex, complete.CompleteOpts{})
			lines := recoveryLines(t, err)
			restored, committed, closed := false, false, false
			for _, line := range lines {
				switch {
				case strings.HasPrefix(line, "git ") && strings.Contains(line, " revert "):
					if kind == "full" {
						f.shellRecovery(line)
						restored = true
					}
				case strings.HasPrefix(line, "git ") && strings.Contains(line, " checkout "):
					if kind == "partial" {
						f.shellRecovery(line)
						restored = true
					}
				case strings.HasPrefix(line, "git ") && strings.Contains(line, " commit "):
					if kind == "partial" {
						f.shellRecovery(line)
						committed = true
					}
				case strings.HasPrefix(line, "mindspec complete "):
					if !restored || (kind == "partial" && !committed) {
						t.Fatal("complete printed before restore")
					}
					f.completeRecovery(line)
					closed = true
				}
			}
			if !restored || !closed {
				t.Fatalf("restore commands did not run: %v", lines)
			}
			merges, err := f.ex.FirstParentMerges("spec/" + mergeLoopSpec)
			if err != nil || len(merges) != 1 || merges[0].SHA != r.MergeSHA {
				t.Fatalf("restore made another naming merge: %v %v", merges, err)
			}
			f.assertCleanAndReady()
			f.finishDependent()
		})
	}
}
func TestComplete_MergeOnlyFollowUpLoop(t *testing.T) {
	f := newMergeLoopFixture(t)
	f.complete(mergeLoopBead, complete.CompleteOpts{MergeOnly: true})
	f.assertOpenMergeDetected()
	loopWrite(t, f.beadWT, "README.md", "fixed\n")
	loopGit(t, f.beadWT, "commit", "-qam", "fix after verification")
	_, err := complete.Run(f.root, mergeLoopBead, mergeLoopSpec, "", f.ex, complete.CompleteOpts{MergeOnly: true})
	recoveryLines(t, err)
	if !strings.Contains(err.Error(), "bead tip moved") {
		t.Fatalf("second merge-only did not refuse moved tip: %v", err)
	}
	f.recordPanel(mergeLoopBead, f.beadWT, 1)
	_, err = complete.Run(f.root, mergeLoopBead, mergeLoopSpec, "", f.ex, complete.CompleteOpts{})
	lines := recoveryLines(t, err)
	if !strings.Contains(err.Error(), "The new bead needs its own panel") {
		t.Fatalf("missing follow-up panel instruction: %v", err)
	}
	f.load()
	if _, exists := f.db["issues"].(map[string]interface{})["mindspec-loop.3"]; exists {
		t.Fatal("follow-up already exists")
	}
	planPath := ".mindspec/specs/" + mergeLoopSpec + "/plan.md"
	planBefore, readErr := os.ReadFile(filepath.Join(f.specWT, planPath))
	if readErr != nil || strings.Contains(string(planBefore), "## Bead 3:") || strings.Contains(string(planBefore), "id: 3") {
		t.Fatalf("follow-up already planned: %v", readErr)
	}
	path := filepath.Join(f.specWT, ".worktrees", "worktree-mindspec-loop.3")
	replace := strings.NewReplacer("<title>", "Follow-up fix", "<new bead>", "mindspec-loop.3", "<new bead worktree>", path)
	panelRecorded, planned, dispatched := false, false, false
	for _, printed := range lines {
		line := replace.Replace(printed)
		switch {
		case strings.HasPrefix(line, "git "):
			f.shellRecovery(line)
		case strings.HasPrefix(line, "run a new panel round for the bead at its tip"):
			f.recordPanel(mergeLoopBead, f.beadWT, 2)
			panelRecorded = true
		case strings.HasPrefix(line, "mindspec complete "):
			if !panelRecorded {
				t.Fatal("complete ran before the printed panel round")
			}
			f.completeRecovery(line)
			f.assertCleanAndReady()
		case strings.HasPrefix(line, "bd create "):
			f.shellRecovery(line)
		case strings.HasPrefix(line, "edit plan.md on the spec branch:"):
			// This printed line is an editing instruction, not shell syntax. Check its
			// operands, then perform the requested plan edit and commit in the fixture.
			for _, operand := range []string{"Nth ## Bead heading", "acceptance criterion", "work_chunks id N", "key_file_paths", "commit plan.md"} {
				if !strings.Contains(line, operand) {
					t.Fatalf("plan recovery lacks %q: %s", operand, line)
				}
			}
			loopWrite(t, f.specWT, planPath, loopPlan([]string{mergeLoopBead, "mindspec-loop.2", "mindspec-loop.3"}))
			loopGit(t, f.specWT, "add", planPath)
			loopGit(t, f.specWT, "commit", "-qm", "plan follow-up")
			planned = true
		case strings.HasPrefix(line, "mindspec next "):
			if !planned {
				t.Fatal("dispatch ran before the printed plan edit")
			}
			got, err := filepath.EvalSymlinks(f.nextRecovery(line))
			want, wantErr := filepath.EvalSymlinks(path)
			if err != nil || wantErr != nil || got != want {
				t.Fatalf("wrong follow-up worktree: got=%s want=%s err=%v %v", got, want, err, wantErr)
			}
			dispatched = true
		default:
			t.Fatalf("unhandled printed recovery: %s", line)
		}
	}
	if !panelRecorded || !planned || !dispatched {
		t.Fatalf("incomplete recovery: %v", lines)
	}
	if got := loopGit(t, path, "show", "HEAD:README.md"); got != "fixed" {
		t.Fatalf("printed recovery lost fix: %s", got)
	}
	f.recordPanel("mindspec-loop.3", path, 1)
	f.complete("mindspec-loop.3", complete.CompleteOpts{})
	f.finishDependent()
}
