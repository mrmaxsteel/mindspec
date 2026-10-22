package pluginmindspec

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func workflowGit(t *testing.T, root, input string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null", "-c", "user.name=Workflow Test", "-c", "user.email=workflow@example.invalid", "-C", root}, args...)...)
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "GIT_") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false")
	cmd.Stdin = strings.NewReader(input)
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("fixture git %v: %v %s", args, e, out)
	}
	return strings.TrimSpace(string(out))
}
func workflowFixture(t *testing.T, silent, quota bool, source string) {
	t.Helper()
	if _, e := exec.LookPath("node"); e != nil {
		t.Fatal("node is required for the workflow fixture")
	}
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	workflowGit(t, root, "", "init", "-q", "-b", "main")
	ignore := ".mindspec-scratch\n"
	input := fmt.Sprintf("commit refs/heads/main\ncommitter Workflow Test <workflow@example.invalid> 1760000000 +0000\ndata 7\ninitial\nM 100644 inline README.md\ndata 6\nhello\nM 100644 inline .gitignore\ndata %d\n%s\n\ndone\n", len(ignore), ignore)
	workflowGit(t, root, input, "fast-import", "--quiet")
	workflowGit(t, root, "", "checkout", "-f", "main")
	sha := workflowGit(t, root, "", "rev-parse", "HEAD")
	if e := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o700); e != nil {
		t.Fatal(e)
	}
	family, model := "harness", "m"
	cfg := "panel: {}\n"
	if quota {
		family = "a"
		fake := filepath.Join(root, "quota.sh")
		if e := os.WriteFile(fake, []byte("#!/bin/sh\ncat > /dev/null\nprintf LIMIT >&2\nexit 2\n"), 0o700); e != nil {
			t.Fatal(e)
		}
		cfg = fmt.Sprintf("panel:\n  runners:\n    - {family: a, kind: process, argv: [%q], prompt: stdin, output: stdout, quota_signals: [LIMIT], on_quota: b}\n    - {family: b, kind: harness}\n", fake)
	}
	if e := os.WriteFile(filepath.Join(root, ".mindspec/config.yaml"), []byte(cfg), 0o600); e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join(root, ".mindspec", "reviews", "p")
	if e := panel.Create(dir, panel.CreateInput{Gate: "adhoc", Target: "main", Round: 1, ReviewedHeadSHA: sha, SubjectDigest: "sha256:workflow", Roster: []panel.RosterSlot{{Slot: "R1", Family: family, Model: model, Lens: "adversarial"}, {Slot: "R2", Family: "harness", Model: "m", Lens: "adversarial"}}}); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(map[string]interface{}{"schema": 2, "slot": "R2", "round": 1, "reviewed_sha": sha, "status": "final", "verdict": "APPROVE", "model": "m", "rationale": "fixture", "concrete_changes_required": []interface{}{}, "commands_run": []interface{}{}})
	if _, e := panel.Submit(panel.SubmitTarget{Dir: dir, Panel: "p", Slot: "R2", Round: 1}, raw, func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
		return panel.SubmitResolution{Family: "harness", Model: "m", RunnerKind: "harness", RunnerSource: "default"}, nil
	}); e != nil {
		t.Fatal(e)
	}
	bin := filepath.Join(t.TempDir(), "mindspec")
	build := exec.Command("go", "build", "-o", bin, "./cmd/mindspec")
	build.Dir = filepath.Join("..", "..")
	if out, e := build.CombinedOutput(); e != nil {
		t.Fatalf("fixture binary: %v %s", e, out)
	}
	workflowPath := filepath.Join(t.TempDir(), "workflow.js")
	if e := os.WriteFile(workflowPath, []byte(source), 0o600); e != nil {
		t.Fatal(e)
	}
	js := `const fs=require('fs'), cp=require('child_process');
const [bin, root, source, sha, silentArg] = process.argv.slice(1);
const silent=silentArg==='true'; let agents=0, creates=0, tallies=0;
const commands=[];
function execute(command) {
  commands.push(command);
  if(command.startsWith('mindspec panel create ')){creates++;return {stdout:'registered',exit:0};}
  if(command.startsWith('mindspec panel tally ')) tallies++;
  try {return {stdout:cp.execSync(command.replace(/^mindspec /,"'"+bin+"' "),{cwd:root,encoding:'utf8',stdio:['pipe','pipe','pipe']}),exit:0};}
  catch(e){return {stdout:String(e.stdout||''),exit:e.status||1};}
}
async function agent(prompt,options){
  if(options.label==='panel-command')return execute(prompt.split('\n').at(-1));
  agents++;
  if(!prompt.includes('Do not use the Write tool on a verdict path.')||prompt.includes('using your Write tool')||prompt.includes('using the Write tool'))throw Error('reviewer requests Write tool');
  const match=prompt.match(/mindspec panel submit --file '([^']+)'/g);
  if(!match||!match.length)throw Error('reviewer has no submit command');
  if(silent)return {};
  const command=match.at(-1), file=command.match(/--file '([^']+)'/)[1];
  const model=prompt.match(/^Model \(the verdict model value\): (.+)$/m)[1];
  if(model==='b' && options.model!==undefined)throw Error('family label must use the default harness model');
  const verdict={schema:2,slot:options.label,round:1,reviewed_sha:sha,status:'final',verdict:'APPROVE',model,rationale:'fixture review',concrete_changes_required:[],commands_run:[]};
  // Simulate a harness Bash step. No Write tool participates.
  fs.writeFileSync(file,JSON.stringify(verdict));
  const submitted=execute(command);if(submitted.exit!==0)throw Error('submit failed: '+submitted.stdout);
  return {};
}
const AsyncFunction=Object.getPrototypeOf(async function(){}).constructor;
(async()=>{const result=await new AsyncFunction('args','agent','pipeline',fs.readFileSync(source,'utf8'))({slug:'p',target:'main',round:1},agent,async(items,fn)=>Promise.all(items.map(fn)));process.stdout.write(JSON.stringify({result,agents,creates,tallies,commands}));})().catch(e=>{console.error(e);process.exit(1)});`
	cmd := exec.Command("node", "-e", js, bin, root, workflowPath, sha, fmt.Sprint(silent))
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("workflow: %v %s", e, out)
	}
	var result struct {
		Result struct {
			SlotResults  []panel.SlotStatus `json:"slotResults"`
			VerifyOutput string             `json:"verifyOutput"`
		}
		Agents, Creates, Tallies int
		Commands                 []string
	}
	if e := json.Unmarshal(out, &result); e != nil {
		t.Fatalf("workflow JSON: %v %s", e, out)
	}
	if result.Creates != 1 {
		t.Fatal("workflow did not register once")
	}
	events, e := panel.SlotAttempts(dir, 1)
	if e != nil {
		t.Fatal(e)
	}
	dispatches := 0
	for _, event := range events["R1"] {
		if event.Status == "dispatched" {
			dispatches++
		}
	}
	var slots []panel.SlotStatus
	if e := json.Unmarshal([]byte(result.Result.VerifyOutput), &slots); e != nil {
		t.Fatal(e)
	}
	if len(slots) != 2 || len(result.Result.SlotResults) != 2 || slots[1].Status != "final" {
		t.Fatalf("slot not reported: %s", out)
	}
	if silent {
		if result.Agents != 2 || dispatches != 2 || slots[0].Status != "running" || result.Tallies != 0 {
			t.Fatalf("silent agent redispatch count or report differs: %s", out)
		}
	} else {
		if result.Agents != 1 || dispatches != 1 || slots[0].Status != "final" || result.Tallies != 1 {
			t.Fatalf("harness did not submit final: %s", out)
		}
		sub, e := panel.LatestSubmissions(dir, 1)
		if e != nil || sub["R1"].Via != "direct" || sub["R1"].Seq != 1 {
			t.Fatalf("submission not registered: %+v %v", sub, e)
		}
	}
	if quota && (slots[0].Family != "b" || slots[0].Model != "b" || slots[0].HandoffPending) {
		t.Fatalf("quota handoff did not finish: %s", out)
	}
}

func TestMsPanelWorkflow_HarnessSubmits(t *testing.T) {
	for _, quota := range []bool{false, true} {
		t.Run(fmt.Sprint(quota), func(t *testing.T) { workflowFixture(t, false, quota, WorkflowFiles()["ms-panel.js"]) })
	}
}
func TestPanelRun_SilentHarnessRedispatchOnce(t *testing.T) {
	workflowFixture(t, true, false, WorkflowFiles()["ms-panel.js"])
}
