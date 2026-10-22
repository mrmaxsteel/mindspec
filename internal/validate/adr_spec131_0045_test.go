package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADRAmendments_Spec131ADR0045Rules(t *testing.T) {
	raw, err := os.ReadFile("../../.mindspec/adr/ADR-0045-binary-owns-sequence-and-rules.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "## Amendment (Spec 131)") != 1 {
		t.Fatal("expected one amendment heading")
	}
	_, text, _ := strings.Cut(string(raw), "## Amendment (Spec 131)")
	for _, pin := range []string{
		"Implementation order step 1 is implemented by one embedded YAML rule table.",
		"`id`, `scope`, `prohibition`, `replacement` and `enforced_by`",
		"`gates` and `target`",
		"`mindspec rules show <id>`",
		"Implementation order step 5 starts with the `## Rules` section in `mindspec instruct`.",
		"Every mode includes orchestrator rules. Implement mode also includes implementer rules.",
		"<!-- staged-rule: id=<rule-id>; remove-by=YYYY-MM-DD -->",
		"`id=sequence.bead-impl-prompt`",
		"`target: next-action`",
		"`next-action` remains outside this amendment.",
	} {
		if !strings.Contains(text, pin) {
			t.Errorf("missing rule-table amendment item: %s", pin)
		}
	}
}
