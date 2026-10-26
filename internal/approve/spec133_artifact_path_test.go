package approve

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestS133ArtifactPath_LifecycleRefusal(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"spec_approve", "plan_approve", "final_review"} {
			t.Run(policy+"/"+gate, func(t *testing.T) {
				root, dir := s133GateFixture(t, policy, gate, "unresolved")
				r, err := panel.Tally(dir)
				if err != nil {
					t.Fatal(err)
				}
				v := paneltest.Verdict(panel.VerdictRequestChanges)
				v.ReviewedSHA = r.Panel.ReviewedHeadSHA
				v.ConcreteChangesRequired[0].ArtifactPath = "evidence/result.txt"
				if _, err := panel.Submit(panel.SubmitTarget{Dir: dir, Slot: "R1", Round: 1}, paneltest.Bytes(t, v), paneltest.Resolve); err != nil {
					t.Fatal(err)
				}
				o, err := runLifecycleGate(root, "900-example", gate, root, nil)
				if err == nil || o.Action != panel.Block {
					t.Fatalf("artifact finding allowed: %+v %v", o, err)
				}
				for _, want := range []string{gate, "R1", "ccr[0]", "artifact paths absent from round tree", "evidence/result.txt"} {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("missing %q in refusal: %v", want, err)
					}
				}
			})
		}
	}
}
