package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADRAmendments_Spec130_Ledger(t *testing.T) {
	checks := map[string][]string{
		"adr/ADR-0037-panel-gate-enforced-contract.md": {
			"Spec-116 artifactPaths — single home (R4.6)", "`panel.ToolArtifactPaths`", "stdlib `path.Match`", "§8 — tracked ledger (R6)", "Spec-114 carve-out", "Spec-115 reach — third impl approve leg (R6.6)", "Neither `MINDSPEC_SKIP_PANEL` nor `enforcement.panel_gate: false` skips this leg", "Complete preflight outside the cap (R6.6)", "Before any side effect", "`check-ignore` in the spec worktree", "After the bead merge", "`exec.CommitPaths`", "`chore(panel): ledger`",
		},
		"adr/ADR-0043-panel-disposition-telemetry-store.md": {
			"`dispositions.jsonl` is tracked again", "spec 129 R8.4 option (B)", "exact-line presence, idempotently", "Adhoc ledgers stay ignored",
		},
	}
	for file, phrases := range checks {
		data, err := os.ReadFile("../../.mindspec/" + file)
		if err != nil {
			t.Fatal(err)
		}
		before, amendment, ok := strings.Cut(string(data), "## Amendment (Spec 130)")
		if !ok {
			t.Fatalf("%s missing amendment", file)
		}
		if !strings.Contains(before, "[Amendment (Spec 130)](#amendment-spec-130)") {
			t.Errorf("%s missing inline back-pointer", file)
		}
		for _, phrase := range phrases {
			if !strings.Contains(amendment, phrase) {
				t.Errorf("%s missing %q", file, phrase)
			}
		}
	}
	data, err := os.ReadFile("../../.mindspec/specs/129-readiness-tracker-honesty/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Decided 2026-10-19: option (B), shipped by spec 130 R6") {
		t.Fatal("spec 129 R8.4 decision missing")
	}
	data, err = os.ReadFile("../../.mindspec/adr/ADR-0037-panel-gate-enforced-contract.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "local copy\n> rather than importing internal/next\"; superseded by the R4.6 single home in\n> [Amendment (Spec 130)]") {
		t.Fatal("Spec-116 artifactPaths inline back-pointer missing")
	}
}
