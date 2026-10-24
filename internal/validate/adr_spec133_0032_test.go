package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestADRAmendments_Spec133ADR0032(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(findProjectRoot(t), ".mindspec/adr/ADR-0032-adr-semantic-gates.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, body, ok := strings.Cut(string(data), "## Amendment (Spec 133)")
	if !ok {
		t.Fatal("missing amendment")
	}
	for _, anchor := range []string{"adr-divergence-unowned", "Warn", "domains root", "findings slice", "unowned", "Impacted Domains", "Error", "adr-divergence-attribute", "adr-divergence-uncovered", "complete --check", "each failing content gate"} {
		if !strings.Contains(body, anchor) {
			t.Errorf("missing amendment anchor %q", anchor)
		}
	}
}
