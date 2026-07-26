package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adr0035_amendment_test.go — spec 127 (lifecycle-verb-trustworthiness)
// Bead 2, R5(f)/AC-9(iv): the pre-drafted ADR-0035 "Guidance non-
// destructiveness" amendment is FINALIZED in this bead (the marker
// removed, per the spec-124/adr0041_amendment_test.go precedent this
// file follows). The match is over a WHITESPACE-NORMALIZED read of the
// shipped ADR (collapsing runs of whitespace/newlines to single spaces
// before matching), the same discipline adr0041_amendment_test.go
// uses, so a future reflow of the amendment's prose can never split a
// discriminating anchor across a line wrap and silently fail this
// test.

func adr0035NormalizedText(t *testing.T) string {
	t.Helper()
	repoRoot := repoRootFromTestDir(t)
	adrPath := filepath.Join(repoRoot, ".mindspec", "adr", "ADR-0035-agent-error-contract.md")
	data, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatalf("reading ADR-0035: %v", err)
	}
	return normalizeWhitespace(string(data))
}

// TestADR0035Amendment_PreDraftMarkerGone pins the "FINALIZED, not
// merely staged" half: the plan-time PRE-DRAFT marker comment must be
// removed from the shipped ADR.
func TestADR0035Amendment_PreDraftMarkerGone(t *testing.T) {
	text := adr0035NormalizedText(t)
	if strings.Contains(text, "PRE-DRAFT") {
		t.Error("ADR-0035 still carries a PRE-DRAFT marker — the spec 127 guidance non-destructiveness amendment must be FINALIZED in this bead")
	}
}

// TestADR0035Amendment_NamesItsFourEnforcementMechanisms is AC-9(iv)'s
// core assertion: the amendment names the constructor, classifier,
// convention scan, and allowlist as its enforcement — a clause with
// no red mechanism is a slogan (spec 127 F3-2).
func TestADR0035Amendment_NamesItsFourEnforcementMechanisms(t *testing.T) {
	text := adr0035NormalizedText(t)
	amendmentIdx := strings.Index(text, "Amendment (Spec 127)")
	if amendmentIdx < 0 {
		t.Fatal("ADR-0035 does not contain the spec 127 amendment section at all")
	}
	amendment := text[amendmentIdx:]

	required := []string{
		"constructor",     // internal/guard's evidence-carrying constructor
		"classifier",      // internal/guard's destructive-family floor
		"convention scan", // internal/lint's repo-wide scan
		"allowlist",       // the exactly-pinned allowlist/registries
	}
	for _, anchor := range required {
		if !strings.Contains(strings.ToLower(amendment), anchor) {
			t.Errorf("ADR-0035's spec 127 amendment does not name %q as an enforcement mechanism", anchor)
		}
	}
	// internal/lint is the scan's stated HOME, not merely "a scan
	// somewhere" — pin the package name too (S3-r2-1/O3-r2-6).
	if !strings.Contains(amendment, "internal/lint") {
		t.Error("ADR-0035's spec 127 amendment does not name internal/lint as the convention scan's home")
	}
	if !strings.Contains(amendment, "internal/guard") {
		t.Error("ADR-0035's spec 127 amendment does not name internal/guard as the classifier/constructor's home")
	}
}

// TestADR0035Amendment_StatesInDiffExtensionObligation pins the
// obligation half of AC-9(iv): a change introducing a destructive
// family not on the floor MUST extend the floor in the same change —
// stated as a review-time convention, not a mechanized gate.
func TestADR0035Amendment_StatesInDiffExtensionObligation(t *testing.T) {
	text := adr0035NormalizedText(t)
	const anchor = "extend the floor in the same change"
	if !strings.Contains(text, anchor) {
		t.Errorf("ADR-0035's spec 127 amendment does not state the in-diff extension obligation (expected an anchor like %q)", anchor)
	}
}

// TestADR0035Amendment_StatesFiniteFloorLimitation pins the honest-
// limitation half of AC-9(iv): the floor is a reviewed finite set, not
// closure — whether text is dangerous is not decidable from the text
// (spec 127's Non-Goals, restated where the ADR reader will see it).
func TestADR0035Amendment_StatesFiniteFloorLimitation(t *testing.T) {
	text := adr0035NormalizedText(t)
	const anchor = "not decidable from the text"
	if !strings.Contains(text, anchor) {
		t.Errorf("ADR-0035's spec 127 amendment does not state the finite-floor limitation (expected an anchor like %q)", anchor)
	}
	// The exemption list's own claim boundary must ALSO be stated —
	// known-and-reviewed, never safe-because-non-prescriptive (the
	// round-6 ruling's central correction).
	const exemptionAnchor = "never that they are safe because non-prescriptive"
	if !strings.Contains(text, exemptionAnchor) {
		t.Errorf("ADR-0035's spec 127 amendment does not state the exemption list's no-polarity-claim boundary (expected an anchor like %q)", exemptionAnchor)
	}
}
