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
	runWorkflowFixture(t, silent, quota, source, "")
}
func workflowFixtureRejects(t *testing.T, source, want string) {
	t.Helper()
	runWorkflowFixture(t, false, false, source, want)
}
func runWorkflowFixture(t *testing.T, silent, quota bool, source, wantFailure string) {
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
	if err := os.WriteFile(filepath.Join(dir, "BRIEF.md"), []byte("## Summary\nReview fixture.\n## Files in Scope\nREADME.md\n## Prior-Round Asks\nNone.\n## Lens\nR1 and R2: correctness.\n"), 0600); err != nil {
		t.Fatal(err)
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
  if(command.startsWith('mindspec panel create ')){creates++;if(!command.includes(' --bead ')||!command.includes(' --gate bead'))throw Error('bead create needs --gate bead');return {stdout:'registered',stderr:'',exit:0};}
  if(command.startsWith('mindspec panel tally ')) tallies++;
  try {return {stdout:cp.execSync(command.replace(/^mindspec /,"'"+bin+"' "),{cwd:root,encoding:'utf8',stdio:['pipe','pipe','pipe']}),stderr:'',exit:0};}
  catch(e){return {stdout:String(e.stdout||''),stderr:String(e.stderr||''),exit:e.status||1};}
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
(async()=>{const result=await new AsyncFunction('args','agent','pipeline',fs.readFileSync(source,'utf8'))({slug:'p',target:'main',round:1,brief_file:root+'/.mindspec/reviews/p/BRIEF.md',bead_id:'test-bead'},agent,async(items,fn)=>Promise.all(items.map(fn)));process.stdout.write(JSON.stringify({result,agents,creates,tallies,commands}));})().catch(e=>{console.error(e);process.exit(1)});`
	cmd := exec.Command("node", "-e", js, bin, root, workflowPath, sha, fmt.Sprint(silent))
	out, e := cmd.CombinedOutput()
	if strings.HasPrefix(wantFailure, "slot:") {
		var got struct {
			Result struct {
				SlotResults []struct{ Status, Error string }
			}
			Agents int
		}
		if e != nil || json.Unmarshal(out, &got) != nil || len(got.Result.SlotResults) != 2 || !strings.Contains(got.Result.SlotResults[0].Error, strings.TrimPrefix(wantFailure, "slot:")) || got.Result.SlotResults[1].Status != "final" || got.Agents != 0 {
			t.Fatalf("wanted slot refusal %q: %v %s", wantFailure, e, out)
		}
		return
	}
	if wantFailure != "" {
		if e == nil || !strings.Contains(string(out), wantFailure) {
			t.Fatalf("wanted refusal %q: %v %s", wantFailure, e, out)
		}
		return
	}
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
	for _, command := range result.Commands {
		if (strings.HasPrefix(command, "mindspec panel checkout ") || strings.HasPrefix(command, "mindspec panel launch ")) && strings.Contains(command, " --replace") {
			t.Fatalf("non-final slot relaunch has --replace: %s", command)
		}
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

func TestMsPanelWorkflow_SubmitCommandBinding(t *testing.T) {
	// Change the command returned by prompt, retaining its absolute verdict path.
	// The workflow must reject this line before it starts a reviewer.
	source := WorkflowFiles()["ms-panel.js"]
	prefix := `const checkedAgent = agent;
 agent = async (prompt, options) => {
  const result = await checkedAgent(prompt, options);
  if (options.label === "panel-command" && prompt.includes(" panel prompt ")) {
   result.stdout = result.stdout.replace(/^mindspec panel submit --file /m, "unrelated-path ");
  }
  return result;
 };
 `
	workflowFixtureRejects(t, prefix+source, "slot:slot prompt needs one bound submission command")
}

func TestMsPanelWorkflow_RefusedFinalReplacement(t *testing.T) {
	for _, kind := range []string{"process", "harness"} {
		t.Run(kind, func(t *testing.T) {
			workflowRefusedFinalReplacement(t, WorkflowFiles()["ms-panel.js"], kind)
		})
	}
}

func workflowRefusedFinalReplacement(t *testing.T, source, kind string) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal("node is required for the workflow fixture")
	}
	workflowPath := filepath.Join(t.TempDir(), "workflow.js")
	if err := os.WriteFile(workflowPath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	js := `const fs = require('fs');
const [source, kind] = process.argv.slice(1);
const slot = {slot:'R1',kind,status:'launch_failed',family:'fixture',model:'fixture'};
const commands = [];
let reviewers = 0;
async function agent(prompt, options) {
  if (options.label !== 'panel-command') { reviewers++; throw Error('refused slot started a reviewer'); }
  const command = prompt.split('\n').at(-1);
  commands.push(command);
  if (commands.length > 20) throw Error('workflow did not stop after refusal');
  if (command.startsWith('mindspec panel create ')) return {stdout:'registered',stderr:'',exit:0};
  if (command.startsWith('mindspec panel verify ')) return {stdout:JSON.stringify([slot]),exit:1};
  if (command.startsWith('mindspec panel launch ') || command.startsWith('mindspec panel checkout ')) {
    return {stdout:'slot already has a final submission',exit:1};
  }
  throw Error('unexpected command after refusal: ' + command);
}
const AsyncFunction = Object.getPrototypeOf(async function(){}).constructor;
(async()=>{
  const result = await new AsyncFunction('args','agent','pipeline',fs.readFileSync(source,'utf8'))(
    {slug:'p',target:'main',round:1,brief_file:'/brief.md'},agent,async(items,fn)=>Promise.all(items.map(fn)));
  process.stdout.write(JSON.stringify({result,commands,reviewers}));
})().catch(e=>{console.error(e);process.exit(1)});`
	out, err := exec.Command("node", "-e", js, workflowPath, kind).CombinedOutput()
	if err != nil {
		t.Fatalf("refusal workflow: %v %s", err, out)
	}
	var got struct {
		Result struct {
			SlotResults  []panel.SlotStatus `json:"slotResults"`
			VerifyOutput string             `json:"verifyOutput"`
			TallyOutput  string             `json:"tallyOutput"`
		}
		Commands  []string
		Reviewers int
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("refusal workflow JSON: %v %s", err, out)
	}
	var attempts []string
	for _, command := range got.Commands {
		if strings.HasPrefix(command, "mindspec panel launch ") || strings.HasPrefix(command, "mindspec panel checkout ") {
			attempts = append(attempts, command)
		}
	}
	want := "mindspec panel launch 'p' 'R1'"
	if kind == "harness" {
		want = "mindspec panel checkout 'p' --slot 'R1' --dispatch"
	}
	if len(attempts) != 1 || attempts[0] != want {
		t.Fatalf("refused %s slot needs exactly one attempt without --replace: %q", kind, attempts)
	}
	var verified []panel.SlotStatus
	if err := json.Unmarshal([]byte(got.Result.VerifyOutput), &verified); err != nil {
		t.Fatal(err)
	}
	for _, slots := range [][]panel.SlotStatus{got.Result.SlotResults, verified} {
		if len(slots) != 1 || slots[0].Slot != "R1" || slots[0].Kind != kind || slots[0].Status != "launch_failed" {
			t.Fatalf("refused slot missing from report: %s", out)
		}
	}
	if got.Reviewers != 0 || got.Result.TallyOutput != "" {
		t.Fatalf("refused slot started a reviewer or tally: %s", out)
	}
}

func TestMsPanelWorkflow_CreateGate(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "mindspec")
	build := exec.Command("go", "build", "-o", bin, "./cmd/mindspec")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture binary: %v %s", err, out)
	}
	for _, gate := range []string{"spec_approve", "plan_approve", "final_review", "adhoc", ""} {
		t.Run(gate, func(t *testing.T) {
			root := t.TempDir()
			workflowGit(t, root, "", "init", "-q", "-b", "main")
			spec := "# Example\n\n## Acceptance Criteria\n\n- The fixture accepts the command.\n"
			specDir := filepath.Join(root, ".mindspec", "specs", "900-example")
			if err := os.MkdirAll(specDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0o600); err != nil {
				t.Fatal(err)
			}
			input := fmt.Sprintf("commit refs/heads/main\ncommitter Workflow Test <workflow@example.invalid> 1760000000 +0000\ndata 7\ninitial\nM 100644 inline .mindspec/specs/900-example/spec.md\ndata %d\n%s\ndone\n", len(spec), spec)
			workflowGit(t, root, input, "fast-import", "--quiet")
			source := WorkflowFiles()["ms-panel.js"]
			js := `const cp=require('child_process'), fs=require('fs'), assert=require('assert');
const [source,gate,bin,root]=process.argv.slice(1); let created;
const agent=async(prompt,options)=>{
 assert.equal(options.label,'panel-command');
 const command=prompt.split('\n').at(-1);
 if(command.startsWith('mindspec panel create ')){created=command;return {stdout:'registered',stderr:'',exit:0};}
 if(command.startsWith('mindspec panel verify '))return {stdout:'[]',exit:0};
 if(command.startsWith('mindspec panel tally '))return {stdout:'PASS',exit:0};
 throw Error('unexpected command: '+command);
};
const AsyncFunction=Object.getPrototypeOf(async function(){}).constructor;
(async()=>{
 const briefFile=root+'/filled-brief.md'; fs.writeFileSync(briefFile,'Review this fixture.\n');
 const args={slug:'p',target:'main',round:1,brief_file:briefFile};
 if(gate){args.gate=gate;if(gate!=='adhoc')args.spec='900-example';}
 await new AsyncFunction('args','agent','pipeline',source)(args,agent,async(items,fn)=>Promise.all(items.map(fn)));
 const expected=gate ? " --gate '"+gate+"'" : ' --gate adhoc';
 assert(created.includes(expected),'create dropped gate: '+created);
 assert.equal((created.match(/ --gate /g)||[]).length,1,'duplicate gate: '+created);
 assert(!(created.includes(" --gate 'adhoc'") && created.includes(' --spec ')),'adhoc rejects a spec: '+created);
 const accepted=cp.execSync(created.replace(/^mindspec /,"'"+bin+"' "),{cwd:root,encoding:'utf8',stdio:['pipe','pipe','pipe']});
 assert(accepted.includes('registered'),'binary did not accept create: '+accepted);
 console.log(created);
})().catch(e=>{console.error(e);process.exitCode=1;});`
			out, err := exec.Command("node", "-e", js, source, gate, bin, root).CombinedOutput()
			if err != nil {
				t.Fatalf("gate %s: %v %s", gate, err, out)
			}
		})
	}
}

func TestMsPanelWorkflow_ConflictingBeadGate(t *testing.T) {
	source := "args.gate = 'final_review';\n" + WorkflowFiles()["ms-panel.js"]
	workflowFixtureRejects(t, source, "gate conflicts with bead_id")
}

func TestMsPanelWorkflow_ConflictingSpecGate(t *testing.T) {
	source := WorkflowFiles()["ms-panel.js"]
	js := `const assert=require('assert');
const AsyncFunction=Object.getPrototypeOf(async function(){}).constructor;
let calls=0;
(async()=>{
 await assert.rejects(()=>new AsyncFunction('args','agent','pipeline',process.argv[1])(
  {slug:'p',target:'main',round:1,brief_file:'/brief.md',spec:'example',gate:'adhoc'},
  async()=>{calls++;return {stdout:'[]',exit:0};},
  async(items,fn)=>Promise.all(items.map(fn))), /gate conflicts with spec/);
 assert.equal(calls,0,'refusal must precede every command');
})().catch(e=>{console.error(e);process.exitCode=1;});`
	if out, err := exec.Command("node", "-e", js, source).CombinedOutput(); err != nil {
		t.Fatalf("conflicting spec gate: %v %s", err, out)
	}
}
