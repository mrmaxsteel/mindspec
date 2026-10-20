package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestADRAmendments_Spec130_Core3(t *testing.T) {
	pins := map[string]map[string][]string{
		"ADR-0037-panel-gate-enforced-contract.md": {
			"§7 — Core 3 coverage (R11.5, R11.6)":               {"only on each returned Allow", "Warn registration (abandoned, missing ref, git error)", "one advisory line", "legacy registration passes", "Abandonment stays the audited exit", "runs no gate decision", "every required registration", "final review included", "legacy-marker and R5.3 integrity checks", "hand-set `abandoned: true` blocks"},
			"Spec-115 contract reach — Core 3 (R11.5)":          {"preflight leg blocks", "MINDSPEC_SKIP_PANEL", "enforcement.panel_gate: false", "orphan/obligation refusal keeps its no-hatch rule"},
			"Complete preflight outside the cap — Core 3 (R11)": {"before materialization", "outside the gate subprocess cap", "every matched required Allow", "Both panel hatches skip"},
		},
		"ADR-0043-panel-disposition-telemetry-store.md": {
			"Coverage manifest writer (R11.2)":            {"Only the binary's `tally`", "roster slots only", "supersedes \"`slots` enumerates EVERY verdict-file slot\"", "not every verdict file", "latest-round verdict files", "except freshness", "Allow and a stale non-bead", "spec, panel, round, created_at", "appended, never rewritten", "last manifest of that round equals", "supersedes \"one per terminal panel\"", "appended ONCE at terminal state", "Every terminal panel writes its manifest regardless of finding count", "not in the Q4 denominator", "Abandonment keeps earlier manifests and their Q4 inclusion", "`refute`, `defer` and `tally` also write"},
			"Manifest readers (R11.3)":                    {"Every manifest reader, `ComputeQ4` included", "only the last manifest", "keyed by spec and panel", "match LatestRound", "validates every ledger record", "completeness floor", "older-round manifest cannot satisfy"},
			"Consequences — lifecycle completeness (R11)": {"`complete` and `impl approve` gate on completeness", "decision-inert", "No lifecycle verb reads the store", "consumes none of them"},
		},
	}
	for name, bullets := range pins {
		raw, err := os.ReadFile(filepath.Join("..", "..", ".mindspec", "adr", name))
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(string(raw), "## Amendment (Spec 130)")
		if len(parts) != 2 {
			t.Fatalf("%s must have unique amendment", name)
		}
		for heading, fragments := range bullets {
			var bullet string
			for _, line := range strings.Split(parts[1], "\n") {
				if strings.HasPrefix(line, "- **"+heading+".**") {
					bullet = line
				}
			}
			if bullet == "" {
				t.Errorf("%s missing bullet %s", name, heading)
			}
			for _, fragment := range fragments {
				if !strings.Contains(bullet, fragment) {
					t.Errorf("%s bullet %s missing %q", name, heading, fragment)
				}
			}
		}
	}
}
