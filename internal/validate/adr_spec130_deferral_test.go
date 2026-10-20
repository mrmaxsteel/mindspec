package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestADRAmendments_Spec130_Deferral(t *testing.T) {
	pins := map[string][][]string{
		"ADR-0037-panel-gate-enforced-contract.md": {
			{"§1 — Discharge writers (R9)", "are the only writers of refutations and deferrals", "Refutations supersede matching deferrals"},
			{"§8 — Deferred audit carve-out (R10.1)", "carve-out extends to the durable `panel_deferred` audit", "only from Allow decisions", "pending markers", "any number of entries"},
			{"Spec-115 contract reach — deferrals (R10.2)", "preflight leg blocks on a finding that no valid discharge covers", "A failed re-check voids that deferral", "It honours `MINDSPEC_SKIP_PANEL` and `enforcement.panel_gate: false`", "Later refutations do not alter that audit", "Integrity runs before abandonment skips"},
			{"Complete preflight outside the cap — deferrals (R10.2)", "before materialization and outside the gate subprocess cap", "A failed read voids that deferral", "Both panel hatches skip this leg"}},
		"ADR-0043-panel-disposition-telemetry-store.md": {{"Canonical writers — discharge verbs (R9.1)", "supersedes \"Writes go through ONE canonical Go verb\"", "`refute`, `defer` and `tally` also write the store", "one lock hold", "Agents do not hand-append these rows"}},
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
		if file == "ADR-0037-panel-gate-enforced-contract.md" && strings.Contains(string(data), "Deferral discharge is inactive") {
			t.Error("deferral discharge must be active")
		}
		for _, bullet := range fragments {
			var lines []string
			for _, line := range strings.Split(parts[1], "\n") {
				if strings.HasPrefix(line, "- **"+bullet[0]) {
					lines = append(lines, line)
				}
			}
			if len(lines) != 1 {
				t.Errorf("%s needs one deferral bullet %q, got %d", file, bullet[0], len(lines))
				continue
			}
			for _, fragment := range bullet[1:] {
				if !strings.Contains(lines[0], fragment) {
					t.Errorf("%s bullet %q missing %q", file, bullet[0], fragment)
				}
			}
		}
	}
}
