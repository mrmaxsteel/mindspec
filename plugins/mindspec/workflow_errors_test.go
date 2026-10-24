package pluginmindspec

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func workflowUnit(t *testing.T, script string) {
	t.Helper()
	source, _, ok := strings.Cut(WorkflowFiles()["ms-panel.js"], "if (!args ||")
	if !ok {
		t.Fatal("workflow entry missing")
	}
	out, err := exec.Command("node", "-e", source+"\n(async()=>{\n"+script+"\n})().catch(e=>{console.error(e);process.exit(1)});").CombinedOutput()
	if err != nil {
		t.Fatalf("workflow: %v\n%s", err, out)
	}
}

func TestWorkflowVerifyErrors(t *testing.T) {
	for _, script := range []string{
		`cli=async()=>({stdout:'{"error":"review registry unavailable"}',exit:2});
 try { await roster(); throw Error('accepted error'); } catch(e) { if(e.constructor!==Error || !e.message.includes('review registry unavailable')) throw e; }`,
		`cli=async()=>({stdout:'relay\n\x1b[31m'+ 'x'.repeat(500),exit:2});
 try { await roster(); throw Error('accepted non-JSON'); } catch(e) { if(e.constructor!==Error || !e.message.includes('verify') || !e.message.includes('relay\\n\\u001b') || e.message.length>300) throw e; }`,
		`cli=async()=>({stdout:'{}',exit:2});
 try { await roster(); throw Error('accepted no roster'); } catch(e) { if(!e.message.includes('verify') || !e.message.includes('2')) throw e; }`,
	} {
		t.Run(script[:20], func(t *testing.T) { workflowUnit(t, script) })
	}
}

func TestWorkflowDispatchFailure(t *testing.T) {
	for _, verb := range []string{"CHECKOUT", "PROMPT"} {
		t.Run(verb, func(t *testing.T) {
			workflowUnit(t, `
 args={slug:'p',target:'main'};
 const slots=[{slot:'R1',kind:'harness',status:'missing'},{slot:'R2',kind:'harness',status:'final'}];
 let checkouts=0, reviewers=0;
 cli=async(verb)=>{if(verb===CHECKOUT)checkouts++; if(verb===`+verb+`)return {exit:2,stdout:'',stderr:'disk unavailable\n\x1b[31m'+ 'x'.repeat(500)}; return {exit:0,stdout:'/slot'};};
 agent=async()=>{reviewers++};
 roster=async()=>({slots});
 const results=await Promise.all(slots.map(runSlot));
 if(checkouts!==1 || reviewers!==0) throw Error('dispatch failure consumed retry: '+checkouts);
 if(!results[0].error?.includes('disk unavailable') || !results[0].error.includes('`+strings.ToLower(verb)+`') || results[1].status!=='final') throw Error('missing slot failure: '+JSON.stringify(results));
 `)
		})
	}
}

func TestWorkflowBriefRequired(t *testing.T) {
	for _, value := range []string{"undefined", "'relative.md'", "'/absolute\\nfile'"} {
		workflowUnit(t, `args={slug:'p',target:'main',round:1,brief_file:`+value+`};
  try {buildCommand(CREATE);throw Error('accepted bad brief');}catch(e){if(e.message==='accepted bad brief')throw e;}`)
	}
	workflowUnit(t, `args={slug:'p',target:'main',round:1,brief_file:"/absolute/it's filled.md"};
 if(!buildCommand(CREATE).includes(' --brief-file '+quote(args.brief_file)))throw Error('brief file not passed');`)
}

// Exercise the entry point, rather than only the command builder.
func TestWorkflowEntryBriefGuard(t *testing.T) {
	for _, value := range []string{"undefined", "'relative.md'"} {
		script := `let calls=0; const args={slug:'p',target:'main',round:1,brief_file:` + value + `};
 const agent=async()=>{calls++;throw Error('agent called')};
 async function workflow(){` + WorkflowFiles()["ms-panel.js"] + `}
 workflow().then(()=>{throw Error('guard accepted input')},e=>{
 if(calls!==0 || !e.message.includes('brief_file must be an absolute path'))throw e;
 }).catch(e=>{console.error(e);process.exit(1)});`
		if out, err := exec.Command("node", "-e", script).CombinedOutput(); err != nil {
			t.Fatalf("entry guard: %v %s", err, out)
		}
	}
}
func TestWorkflowCreateRefusal(t *testing.T) {
	script := `const args={slug:'p',target:'main',round:1,brief_file:'/brief.md'};let calls=0;
 const agent=async(prompt,options)=>{calls++;if(!prompt.includes('stderr') || !options.schema.required.includes('stderr'))throw Error('stderr contract absent');return {stdout:'',stderr:'brief contains TODO(skill)\n\x1b[31m'+ 'x'.repeat(500),exit:2}};
 async function workflow(){` + WorkflowFiles()["ms-panel.js"] + `}
 workflow().then(result=>{if(calls!==1 || !result.createOutput.includes('TODO(skill)') || !result.createOutput.includes('exit 2') || result.createOutput.includes('\x1b') || result.createOutput.length>260)throw Error(JSON.stringify(result))}).catch(e=>{console.error(e);process.exit(1)});`
	if out, err := exec.Command("node", "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("create refusal: %v %s", err, out)
	}
}
func TestWorkflowProcessFailures(t *testing.T) {
	for _, verb := range []string{"LAUNCH", "RECOVER"} {
		status := "missing"
		if verb == "RECOVER" {
			status = "damaged"
		}
		workflowUnit(t, fmt.Sprintf(`const slot={slot:'R1',kind:'process',status:%q};
 cli=async()=>({exit:2,stdout:'',stderr:'refused\n\x1b[31m'+ 'x'.repeat(500)});
 const result=await runSlot(slot);
 if(result.slot!=='R1' || !result.error.includes('refused') || !result.error.includes(%q) || result.error.includes('\x1b') || result.error.length>260)throw Error(JSON.stringify(result));`, status, strings.ToLower(verb)))
	}
	workflowUnit(t, `cli=async()=>({exit:0,stdout:'',stderr:''}); roster=async()=>({slots:[]});
 const result=await runSlot({slot:'R1',kind:'process',status:'damaged'});
 if(result.slot!=='R1' || !result.error?.includes('disappeared'))throw Error(JSON.stringify(result));`)
}
func TestWorkflowVerifyObjectError(t *testing.T) {
	workflowUnit(t, `cli=async()=>({stdout:JSON.stringify({error:{reason:'bad\n\x1b[31m'}}),exit:2});
 try{await roster();throw Error('accepted error')}catch(e){if(e.message.includes('[object Object]') || !e.message.includes('reason') || e.message.includes('\x1b') || e.message.length>230)throw e;}`)
}

func TestWorkflowProcessLaunchRetry(t *testing.T) {
	for _, status := range []string{"launch_failed", "draft", "unsubmitted", "void", "malformed"} {
		for _, failsAgain := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/secondFails=%t", status, failsAgain), func(t *testing.T) {
				workflowUnit(t, fmt.Sprintf(`let launches=0;
 const slot={slot:'R1',kind:'process',status:'missing'};
 cli=async()=>{launches++;return {exit:launches===1 || %t ? 1:0,stderr:'reviewer failed'}};
 roster=async()=>({slots:[{...slot,status:launches===1 || %t ? %q:'final'}]});
 const result=await runSlot(slot);
 if(launches!==2 || result.status!==( %t ? %q:'final') || (%t && !result.error?.includes('reviewer failed')))throw Error(JSON.stringify({launches,result}));`, failsAgain, failsAgain, status, failsAgain, status, failsAgain))
			})
		}
	}
}
func TestWorkflowReviewerRejectionRetry(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		t.Run(fmt.Sprint(rejected), func(t *testing.T) {
			workflowUnit(t, fmt.Sprintf(`args={slug:'p',target:'main'};
 let reviewers=0;
 // Keep the real setup/binding path.
 cli=async(verb)=>({exit:0,stdout:verb===CHECKOUT?'/slot':buildCommand(SUBMIT,{slotDir:'/slot',attempt:1})});
 agent=async()=>{reviewers++;if(%t)throw 'reviewer crash';};
 roster=async()=>({slots:[{slot:'R1',kind:'harness',status:reviewers===2?'final':'unsubmitted'}]});
 const result=await runSlot({slot:'R1',kind:'harness',status:'missing'});
 if(reviewers!==2 || result.status!=='final')throw Error(JSON.stringify({reviewers,result}));`, rejected))
		})
	}
	workflowUnit(t, `dispatch=async()=>{throw 'setup failed'};
 const result=await runSlot({slot:'R1',kind:'harness',status:'missing'});
 if(result.error!=='setup failed')throw Error(JSON.stringify(result));`)
}

func TestWorkflowSubmitBindingFailure(t *testing.T) {
	workflowUnit(t, `args={slug:'p',target:'main'};
 let commands=0,reviewers=0;
 cli=async(verb)=>{commands++;return {exit:0,stdout:verb===CHECKOUT?'/slot':'no bound command'}};
 agent=async()=>{reviewers++};
 const result=await runSlot({slot:'R1',kind:'harness',status:'missing'});
 if(commands!==2 || reviewers!==0 || !result.error?.includes('one bound submission'))throw Error(JSON.stringify({commands,reviewers,result}));`)
}
