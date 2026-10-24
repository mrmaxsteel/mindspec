package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADRAmendments_Spec133ADR0041(t *testing.T) {
	raw, err := os.ReadFile("../../.mindspec/adr/ADR-0041-gate-before-mutate.md")
	if err != nil {
		t.Fatal(err)
	}
	_, amendment, ok := strings.Cut(string(raw), "## Amendment (Spec 133)")
	if !ok {
		t.Fatal("missing amendment")
	}
	text := strings.Join(strings.Fields(amendment), " ")
	for _, pin := range []string{
		"before the first write", "supersede close", "Existing approvals remain valid",
		"default is `warn`", "under `warn` and `enforce`", "same target tip",
		"own write set of `spec approve`", "`status`, `approved_at` and `approved_by`",
		"standard `## Approval` section", "generated scaffold byte for byte",
		"`Status`, `Approved By`, `Approval Date` and `Notes`", "Any extra line remains subject content",
		"own write set of `plan approve`", "`bead_ids`", "exclude both verbs' own write sets",
		"parsed frontmatter keys and values", "Uncommitted subject changes",
		"panel_gate_skipped", "panel_abandoned", "panel_absent", "each due record",
		"plan_deferral_ids", "every tracked spec approval ledger", "`## Resolutions`",
	} {
		if !strings.Contains(text, pin) {
			t.Errorf("missing %q", pin)
		}
	}
}
