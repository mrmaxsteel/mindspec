package pluginmindspec

import (
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
 cli=async(verb)=>{if(verb===CHECKOUT)checkouts++; if(verb===`+verb+`)return {exit:2,stdout:'disk unavailable'}; return {exit:0,stdout:'/slot'};};
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
