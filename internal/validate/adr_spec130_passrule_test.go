package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestADRAmendments_Spec130_PassRule(t *testing.T) {
	pins := map[string][]string{
		"ADR-0037-panel-gate-enforced-contract.md":      {"§1 — Snapshot fields", "§3 — Pass rule", "complete, no veto, fresh, discharged and intact", "APPROVE count is not a decision input", "retires the Spec-109 `approve_threshold` extension", "the approve threshold remains a NECESSARY floor", "refutation cannot buy a sub-threshold panel past the gate", "§7 — Recovery and rule line", "`panel refute` and `panel defer`", "§8 — Hand-edit detection", "accidental desync only", "anti-footgun, not anti-adversary", "no signing"},
		"ADR-0043-panel-disposition-telemetry-store.md": {"Ledger events (R5.1)", "`round`, `refutation`, `deferral`, `rebind` and `abandon`", "Gate reads (R5.3)", "`round`, `rebind` and `abandon`", "before abandonment", "carried baseline", "ignoring `created_at`"},
		"ADR-0044-panel-review-conduct.md":              {"Finding outcomes (R7, R13.4)", "Deferral is the third outcome", "`finding_sha256`-pinned", "approval threshold floor is retired", "APPROVE count is not a decision input"},
	}
	for name, fragments := range pins {
		raw, err := os.ReadFile(filepath.Join("..", "..", ".mindspec", "adr", name))
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(string(raw), "## Amendment (Spec 130)")
		if len(parts) != 2 {
			t.Fatalf("%s missing unique amendment", name)
		}
		if strings.Contains(name, "0037") {
			for _, line := range strings.Split(parts[1], "\n") {
				if !strings.HasPrefix(line, "- **§1 — Snapshot fields") {
					continue
				}
				for _, field := range []string{"roster", "subject_digest", "telemetry", "refutations", "deferrals", "rebinds", "abandon_history", "before abandonment", "at least two slots"} {
					if !strings.Contains(line, field) {
						t.Errorf("registration bullet missing %q", field)
					}
				}
			}
		}
		for _, want := range fragments {
			if !strings.Contains(parts[1], want) {
				t.Errorf("%s amendment missing %q", name, want)
			}
		}
		if strings.Contains(name, "0043") && strings.Count(parts[0], "superseded for integrity reads by [Amendment (Spec 130)]") < 2 {
			t.Errorf("%s missing ledger-read back-pointers", name)
		}
		if strings.Contains(name, "0044") && strings.Count(parts[0], "[Amendment (Spec 130)]") < 2 {
			t.Errorf("%s missing conduct back-pointers", name)
		}
	}
}
