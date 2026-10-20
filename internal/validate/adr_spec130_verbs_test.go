package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADRAmendments_Spec130_Verbs(t *testing.T) {
	for file, pins := range map[string][]string{
		"adr/ADR-0037-panel-gate-enforced-contract.md":   {"§7 — Abandon verb (R9.4)", "`mindspec panel abandon` requires a reason and a name", "appends an `abandon` event", "sets `abandoned` and `abandon_reason`", "writes no manifest and removes none", "Abandonment stays the audited exit", "`panel_abandoned` audit at complete is unchanged"},
		"adr/ADR-0045-binary-owns-sequence-and-rules.md": {"Implementation status (R13.5)", "Implementation order step 1 (the rule table) and the step-3 `panel brief` template move to spec 131", "staged-rule marker names the spec-131 rule table", "Refute and abandon procedures are binary verbs"},
	} {
		raw, err := os.ReadFile("../../.mindspec/" + file)
		if err != nil {
			t.Fatal(err)
		}
		_, amendment, ok := strings.Cut(string(raw), "## Amendment (Spec 130)")
		if !ok {
			t.Fatalf("%s missing amendment", file)
		}
		text := strings.Join(strings.Fields(amendment), " ")
		for _, pin := range pins {
			if !strings.Contains(text, pin) {
				t.Errorf("%s missing amendment bullet text %q", file, pin)
			}
		}
	}
	raw, err := os.ReadFile("../../.mindspec/specs/129-readiness-tracker-honesty/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	_, r7, ok := strings.Cut(string(raw), "### R7 — Governance rider: ADR-0045")
	if !ok {
		t.Fatal("R7 missing")
	}
	r7, _, _ = strings.Cut(r7, "### R8")
	if !strings.Contains(r7, "Re-pointed 2026-10-19: the rule table moves to spec 131; #233's reviewed-tree check and #251 move to the spec-129 follow-on") {
		t.Error("R7.2 status line missing")
	}
}
