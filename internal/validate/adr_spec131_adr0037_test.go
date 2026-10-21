package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADRAmendments_Spec131ADR0037(t *testing.T) {
	raw, err := os.ReadFile("../../.mindspec/adr/ADR-0037-panel-gate-enforced-contract.md")
	if err != nil {
		t.Fatal(err)
	}
	_, text, ok := strings.Cut(string(raw), "## Amendment (Spec 131)")
	if !ok {
		t.Fatal("missing amendment")
	}
	text = strings.Join(strings.Fields(text), " ")
	for _, pin := range []string{"round event pins `verdict_schema`", "round without this field keeps the previous file-based rules", "latest submission for each slot and round", "must be final and must not be `VOID`", "match its recorded SHA-256 digest", "highest later attempt", "names the latest submission's `seq`", "new submission replaces the previous submission", "position and text hash", "hash of the ordered item texts", "§8 also applies to `panel submit`", "does not authenticate a reviewer"} {
		if !strings.Contains(text, pin) {
			t.Errorf("missing %q", pin)
		}
	}
}
