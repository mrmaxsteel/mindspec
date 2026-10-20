package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestADRAmendments_Spec130_Deferral(t *testing.T) {
	pins := map[string][]string{
		"ADR-0037-panel-gate-enforced-contract.md": {
			"§1 — Discharge writers (R9)", "are the only writers of refutations and deferrals", "Refutations supersede matching deferrals",
			"§8 — Deferred audit carve-out (R10.1)", "carve-out extends to the durable `panel_deferred` audit", "only from Allow decisions", "pending markers", "any number of entries",
			"Spec-115 contract reach — deferrals (R10.2)", "preflight leg blocks", "MINDSPEC_SKIP_PANEL", "enforcement.panel_gate: false", "Later refutations do not alter that audit", "Integrity runs before abandonment skips",
			"Complete preflight outside the cap — deferrals (R10.2)", "before materialization and outside the gate subprocess cap", "Both panel hatches skip this leg"},
		"ADR-0043-panel-disposition-telemetry-store.md": {"Canonical writers — discharge verbs (R9.1)", "supersedes \"Writes go through ONE canonical Go verb\"", "`refute`, `defer` and `tally` also write the store", "one lock hold", "Agents do not hand-append these rows"},
	}
	for file, fragments := range pins {
		data, err := os.ReadFile(filepath.Join("..", "..", ".mindspec", "adr", file))
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(string(data), "## Amendment (Spec 130)")
		if len(parts) != 2 {
			t.Fatalf("%s amendment must be unique", file)
		}
		for _, fragment := range fragments {
			if !strings.Contains(parts[1], fragment) {
				t.Errorf("%s missing deferral amendment %q", file, fragment)
			}
		}
	}
}
