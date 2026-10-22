package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADRAmendments_Spec131ADR0044(t *testing.T) {
	raw, err := os.ReadFile("../../.mindspec/adr/ADR-0044-panel-review-conduct.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "## Amendment (Spec 131)") != 1 {
		t.Fatal("expected one amendment heading")
	}
	_, text, _ := strings.Cut(string(raw), "## Amendment (Spec 131)")
	for _, row := range []string{
		"| 1. Reviewer mutation-isolation | `panel.isolated-checkout`, `review.reproduce` |",
		"| 2. Verify-never-confirm | `review.reproduce`, `review.no-vote`, `review.classifier-redispatch` |",
		"| 3. Findings-never-out-voted | `review.no-vote` |",
		"| 4. Absolute scratch + clean worktree | `panel.scratch-in-slot`, `panel.clean-before-complete` |",
		"| 5. CI-parity kernel | `review.ci-parity` |",
		"Decision 4 requires an absolute scratch path in the slot or fixer directory.",
		"`review.ci-parity` applies only to the `final_review` gate.",
	} {
		if !strings.Contains(text, row) {
			t.Errorf("missing decision mapping or scratch rule: %s", row)
		}
	}
}
