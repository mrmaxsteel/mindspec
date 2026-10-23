package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestADRAmendments_Spec133ADR0043(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", ".mindspec", "adr", "ADR-0043-panel-disposition-telemetry-store.md"))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "## Amendment (Spec 133)")
	if len(parts) != 2 {
		t.Fatal("expected exactly one amendment")
	}
	for _, want := range []string{"`deferred_to_plan`", "`plan_deferrals`", "`confirmed-deferred`", "`plan:<deferral id>`", "one ledger write", "finding hash", "no\n`bead` field", "does not read `plan_deferrals`"} {
		if !strings.Contains(parts[1], want) {
			t.Errorf("amendment missing %s", want)
		}
	}
}
