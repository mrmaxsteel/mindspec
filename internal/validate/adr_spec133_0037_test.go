package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADRAmendments_Spec133ADR0037(t *testing.T) {
	raw, err := os.ReadFile("../../.mindspec/adr/ADR-0037-panel-gate-enforced-contract.md")
	if err != nil {
		t.Fatal(err)
	}
	_, text, ok := strings.Cut(string(raw), "## Amendment (Spec 133)")
	if !ok {
		t.Fatal("missing amendment")
	}
	for _, pin := range []string{"enforcement.panel_presence", "default is `warn`", "`enforce`", "Each live panel must pass", "policy does not weaken a failed decision leg", "Create, rebind, verify and approval use one digest function", "panel_gate_skipped", "panel_abandoned", "panel_absent", "nil deferral", "no bead merge fence"} {
		if !strings.Contains(text, pin) {
			t.Errorf("missing %q", pin)
		}
	}
}
