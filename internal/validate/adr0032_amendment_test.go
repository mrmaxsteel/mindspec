package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestADR0032Amendment_ThirdAmendmentAnchorsPresent pins spec 122 AC-13:
// ADR-0032's new (third) `## Amendment` section — landing in THIS bead,
// alongside the first Requirement-1/2 code (R6's falsifier: "lands in a
// different bead than the implementing code") — must carry every
// assertable anchor the spec names: the forward-only in-use predicate and
// BOTH carve-outs, symmetric deterministic name-resolution with
// directory-shape completeness, the ADR-side no-new-error doctrine, the
// path-overlap/new-ADR trigger sentence, and the evidenced-supersession
// paragraph naming bead `mindspec-6ou2`'s 6/6 2026-06-26 panel decision. It
// also asserts the plan-time PRE-DRAFT marker comment has been removed —
// the amendment is FINALIZED, not merely staged.
func TestADR0032Amendment_ThirdAmendmentAnchorsPresent(t *testing.T) {
	root := findProjectRoot(t)
	adrPath := filepath.Join(root, ".mindspec", "adr", "ADR-0032-adr-semantic-gates.md")

	data, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatalf("reading %s: %v", adrPath, err)
	}
	content := string(data)

	if strings.Contains(content, "PRE-DRAFT") {
		t.Error("ADR-0032 still carries a PRE-DRAFT marker — the third amendment must be FINALIZED in this bead")
	}

	if !strings.Contains(content, "## Amendment") {
		t.Fatal("ADR-0032 has no ## Amendment section at all")
	}
	// The third amendment section is the LAST one in the file (spec 122,
	// after tri-state coverage and spec-100 normalization) — isolate its
	// body so anchor assertions are scoped to it, not the earlier two.
	sections := strings.Split(content, "\n## Amendment")
	if len(sections) < 4 {
		// preamble + 3 amendments = 4 pieces after the split.
		t.Fatalf("expected ADR-0032 to carry 3 '## Amendment' sections (tri-state, spec-100, spec-122), found %d", len(sections)-1)
	}
	thirdAmendment := sections[len(sections)-1]

	if !strings.Contains(thirdAmendment, "spec 122") {
		t.Error("the third amendment section header does not cite spec 122")
	}

	anchors := []string{
		"authoring-time",
		"symmetric",
		"carve-out",
		"mindspec-6ou2",
		"2026-06-26",
	}
	for _, a := range anchors {
		if !strings.Contains(thirdAmendment, a) {
			t.Errorf("third amendment section missing required anchor %q", a)
		}
	}

	// The forward-only in-use predicate.
	if !strings.Contains(thirdAmendment, "IN USE") {
		t.Error("third amendment missing the ownership-model-in-use predicate anchor")
	}
	// Both carve-outs, by name.
	if !strings.Contains(thirdAmendment, "grandfathering") {
		t.Error("third amendment missing the Approved/status-less grandfathering carve-out anchor")
	}
	if !strings.Contains(thirdAmendment, "manifest-less") {
		t.Error("third amendment missing the manifest-less carve-out anchor")
	}
	// Directory-shape completeness.
	if !strings.Contains(thirdAmendment, "directory-shape completeness") && !strings.Contains(thirdAmendment, "trailing slash") {
		t.Error("third amendment missing the directory-shape completeness anchor")
	}
	// ADR-side no-new-error doctrine.
	if !strings.Contains(thirdAmendment, "new error class") {
		t.Error("third amendment missing the ADR-side no-new-error doctrine anchor")
	}
	// New-ADR trigger sentence.
	if !strings.Contains(thirdAmendment, "New-ADR trigger") {
		t.Error("third amendment missing the new-ADR trigger sentence anchor")
	}
	// Evidenced-supersession three-objection refutation.
	if !strings.Contains(thirdAmendment, "Evidenced supersession") && !strings.Contains(thirdAmendment, "supersession") {
		t.Error("third amendment missing the evidenced-supersession paragraph anchor")
	}
	if !strings.Contains(thirdAmendment, "6/6") {
		t.Error("third amendment missing the 6/6 panel-decision anchor")
	}
}
