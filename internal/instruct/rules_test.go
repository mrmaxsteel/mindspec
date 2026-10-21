package instruct

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/rules"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

func TestInstruct_RulesSection(t *testing.T) {
	for _, mode := range []string{state.ModeIdle, state.ModeSpec, state.ModePlan, state.ModeImplement, state.ModeReview, state.ModeIndeterminate, "ambiguous"} {
		t.Run(mode, func(t *testing.T) {
			ctx := &Context{Mode: mode, IndeterminateReason: "fixture"}
			out, err := Render(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(out, "## Rules\n") != 1 {
				t.Fatal("expected exactly one Rules section")
			}
			_, section, _ := strings.Cut(out, "## Rules\n\n")
			var ids []string
			for _, line := range strings.Split(strings.TrimSpace(section), "\n") {
				if !strings.HasPrefix(line, "- ") {
					continue
				}
				id, _, ok := strings.Cut(strings.TrimPrefix(line, "- "), ": ")
				if !ok {
					t.Fatalf("bad rule line %q", line)
				}
				ids = append(ids, id)
			}
			want := []string{"panel.verbs-only", "panel.fix-after-final", "panel.one-fixer", "panel.artifact-gate", "panel.clean-before-complete", "review.reproduce", "review.no-vote", "review.classifier-redispatch", "lifecycle.no-probe-complete", "sequence.bead-impl-prompt"}
			if mode == state.ModeImplement {
				want = []string{"panel.verbs-only", "panel.fix-after-final", "panel.one-fixer", "panel.artifact-gate", "panel.clean-before-complete", "review.reproduce", "review.no-vote", "review.classifier-redispatch", "review.derive", "lifecycle.no-probe-complete", "lifecycle.complete-by-orchestrator", "sequence.bead-impl-prompt"}
			}
			if !reflect.DeepEqual(ids, want) {
				t.Fatalf("%s rule ids = %v, want %v", mode, ids, want)
			}
			for _, id := range want {
				r, err := rules.Lookup(id)
				if err != nil || !strings.Contains(section, id+": "+r.Prohibition+" — "+r.Replacement) {
					t.Errorf("%s not rendered from its table row: %v", id, err)
				}
			}
			structured, err := RenderJSON(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var doc JSONOutput
			if err := json.Unmarshal([]byte(structured), &doc); err != nil || doc.Guidance != out {
				t.Fatalf("JSON guidance differs from markdown: %v", err)
			}
		})
	}
}
