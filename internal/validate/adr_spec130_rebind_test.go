package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestADRAmendments_Spec130_Rebind(t *testing.T) {
	pins := map[string][]string{
		"ADR-0037-panel-gate-enforced-contract.md": {
			"§4 — Panel rebind (R4.1–R4.5, R3.5)", "`mindspec panel rebind <slug> --reason <text>`", "`reviewed_head_sha` and `subject_digest`", "without a round bump", "reason must be non-empty", "parse, be intact, not abandoned, have no round mismatch", "pass R7 with staleness ignored", "ancestor of the tip", "Every diff path is accepted except `.mindspec/config.yaml`", "no allowlist or config key", "{round, from_sha, to_sha, paths, reason, created_at}", "R3.1 exclusions", "`rebind` ledger event", "before one snapshot write", "`already fresh` and write nothing", "`panel_rebound_entries`", "same fail-closed pre-close metadata stage as `panel_refuted`", "Only R5.3-matched `from_sha`", "exact-equality landed set",
		},
		"ADR-0041-gate-before-mutate.md": {
			"Datum (c) — Rebind evidence (R3.5, R13.3)", "each R5.3-matched `rebinds[].from_sha`", "first round's carried baseline", "Unbacked or hand-changed entries supply no evidence", "exact equality with the merge's second parent", "no prefix matching or ancestor tolerance", "`panel_rebound_entries`", "same fail-closed pre-close metadata stage as `panel_refuted`", "failed audit write refuses completion before close",
		},
	}
	for name, fragments := range pins {
		raw, err := os.ReadFile(filepath.Join("..", "..", ".mindspec", "adr", name))
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(string(raw), "## Amendment (Spec 130)")
		if len(parts) != 2 {
			t.Fatalf("%s: missing unique amendment", name)
		}
		amendment := strings.Join(strings.Fields(parts[1]), " ")
		for _, phrase := range fragments {
			if !strings.Contains(amendment, phrase) {
				t.Errorf("%s amendment missing %q", name, phrase)
			}
		}
		if strings.Contains(name, "0041") {
			body := strings.Join(strings.Fields(parts[0]), " ")
			want := "Datum (c) is the SET of all well-formed `reviewed_head_sha` values across registrations. See [Amendment (Spec 130)](#amendment-spec-130) for the R3.5 ledger-matched `rebinds[].from_sha` extension."
			if !strings.Contains(body, want) {
				t.Error("datum (c) missing rebind back-pointer")
			}
		}
	}
}
