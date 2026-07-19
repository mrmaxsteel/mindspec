package main

// Spec 121 Bead 3 (AC-16 §4 half). Mirrors internal/gitutil's Bead 1
// AC-16 §2 anchor test (adr_anchor_test.go): a manual `rg` grep is not a
// committed automated test, so a future edit that strips or paraphrases
// away the §4 amendment's substance would not go red in CI. This test
// reads ADR-0041's actual committed text and asserts the §4 anchor and
// its concrete pinned terms are present — living beside
// runFinalizePRAutomation (finalize_pr.go), the FIRST and only code to
// cite §4, per R8's "amendment lands with the first citing code" rule.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestADR0041_Section4FinalizeCarrierAnchors is the AC-16 §4 half's
// automated anchor pin. RED if the §4 amendment text is ever stripped,
// renumbered, or paraphrased away from the concrete terms it names.
func TestADR0041_Section4FinalizeCarrierAnchors(t *testing.T) {
	root := repoRootFromTestDir(t)
	path := filepath.Join(root, ".mindspec", "adr", "ADR-0041-gate-before-mutate.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	text := string(data)
	lower := strings.ToLower(text)

	// The §4 anchor label itself — finalize_pr.go's doc comment cites it
	// by this exact label.
	if !strings.Contains(text, "§4") {
		t.Error("ADR-0041 must contain the §4 clause anchor (the finalize-PR automation cites it by this exact label)")
	}
	if !strings.Contains(lower, "finalize carrier") {
		t.Error("ADR-0041's §4 must name the \"finalize carrier\" anchor term (the rg 'finalize carrier' proof)")
	}

	// The concrete substance §4 pins, independent of numbering: losing
	// these while keeping the bare "§4" label would still gut the
	// amendment.
	terms := []string{
		"tracker-only",
		"auto_merge_finalize_pr",
		"default",
		"green checks",
		"merge commit",
		"documented-forward-safe",
		"reconcil",
		"uxl4",
	}
	for _, term := range terms {
		if !strings.Contains(lower, strings.ToLower(term)) {
			t.Errorf("ADR-0041's §4 clause must name %q — an edit that drops this term breaks the finalize-PR automation's contract", term)
		}
	}

	if !strings.Contains(lower, "convergence") {
		t.Error("ADR-0041 must still contain the §2 convergence-completeness anchor text (AC-16 §2 half)")
	}
	if !strings.Contains(lower, "attested") {
		t.Error("ADR-0041 must still contain the §2 attested-restore anchor text (AC-16 §2 half)")
	}
}
