package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestADRAmendments_Spec131ADR0043(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", ".mindspec", "adr", "ADR-0043-panel-disposition-telemetry-store.md"))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "## Amendment (Spec 131)")
	if len(parts) != 2 {
		t.Fatal("expected exactly one amendment")
	}
	for _, want := range []string{"`verdict`", "`launch`", "`substitution`", "`slot, round, seq`", "`slot, round, attempt, status`", "`slot, round`", "`seq: 1`", "same-bytes retry", "Only the panel verbs"} {
		if !strings.Contains(parts[1], want) {
			t.Errorf("amendment missing %s", want)
		}
	}
}
