package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADRAmendments_Spec131ADR0040(t *testing.T) {
	data, err := os.ReadFile("../../.mindspec/adr/ADR-0040-orchestration-layering-ratchet.md")
	if err != nil {
		t.Fatal(err)
	}
	_, amendment, ok := strings.Cut(string(data), "## Amendment (Spec 131)")
	if !ok {
		t.Fatal("missing amendment")
	}
	text := strings.Join(strings.Fields(amendment), " ")
	for _, pin := range []string{"`panel.runners` declares reviewer execution families", "No process runner or tool runner ships with the binary", "Runner lists merge by family", "replaces the earlier entry in its position", "A new family is appended", "`.mindspec/config.local.yaml` comes last", "only `panel.runners` and `models`", "`runner:` key still selects the orchestration adapter"} {
		if !strings.Contains(text, pin) {
			t.Errorf("missing %q", pin)
		}
	}
}
