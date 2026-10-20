package validate

import (
	"os"
	"strings"
	"testing"
)

func TestADRAmendments_Spec130_Keys(t *testing.T) {
	data, err := os.ReadFile("../../.mindspec/adr/ADR-0037-panel-gate-enforced-contract.md")
	if err != nil {
		t.Fatal(err)
	}
	_, amendment, ok := strings.Cut(string(data), "## Amendment (Spec 130)")
	if !ok {
		t.Fatal("missing Spec 130 amendment")
	}
	_, keys, ok := strings.Cut(amendment, "**§8 — Durable reviewed-SHA keys (R12).**")
	if !ok {
		t.Fatal("missing §8 R12-keys amendment")
	}
	keys, _, _ = strings.Cut(keys, "\n- **")
	text := strings.Join(strings.Fields(keys), " ")
	for _, phrase := range []string{
		"The Spec-114 audit-durability carve-out extends to",
		"`mindspec_landed_reviewed_sha`", "`mindspec_landed_review_panel`",
		"first Allow panel decision in scan order", "`reviewed_head_sha` and `<slug>@r<N>`",
		"in the same `mergeBindingFn` call as the landed binding, only through `CompleteBead`",
		"A Warn, either hatch, or no panel writes neither key",
		"already-bound check compares the keys only when supplied",
		"`FinalizeEpic` writes neither key and removes neither key",
		"audit durability, not general tamper-resistance",
	} {
		if !strings.Contains(text, phrase) {
			t.Errorf("R12-keys amendment missing %q", phrase)
		}
	}
}
