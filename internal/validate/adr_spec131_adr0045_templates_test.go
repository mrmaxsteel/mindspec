package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADRAmendments_Spec131ADR0045Templates(t *testing.T) {
	raw, err := os.ReadFile("../../.mindspec/adr/ADR-0045-binary-owns-sequence-and-rules.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "## Amendment (Spec 131)") != 1 {
		t.Fatal("expected one amendment heading")
	}
	_, text, _ := strings.Cut(string(raw), "## Amendment (Spec 131)")
	for _, pin := range []string{
		"Implementation order step 1 is implemented", "Implementation order step 5 starts",
		"<!-- staged-rule: id=<rule-id>; remove-by=YYYY-MM-DD -->",
		"Implementation order step 3 ships the BRIEF and slot prompt templates.",
		"acceptance, quoted spec acceptance criteria, conduct and verdict contract",
		"gate question and not-blocking list from config", "spec at the reviewed SHA",
		"It escapes control bytes.", "binary does not check those paths",
		"Authored guidance names no documentation path or internal reference.",
		"Each repository path in that guidance must exist at the reviewed SHA.",
		"`mindspec panel prompt` prints the BRIEF", "effective family and model, lens and focus",
		"slot directory and tree", "It writes no file.", "nonce markers or the attempt verdict path",
		"`mindspec panel submit` and the latest dispatched attempt", "only that slot's prior-round items",
		"latest submission of the prior round", "parsed verdict file for a legacy round",
		"ADDRESSED, PARTIAL, MISSED or NEW_ISSUE", "Scratch stays in the slot directory.",
		"Repository template overrides remain outside this amendment.",
	} {
		if !strings.Contains(text, pin) {
			t.Errorf("missing template amendment item: %s", pin)
		}
	}
}
