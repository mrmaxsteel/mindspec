package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// adr0041_amendment_test.go — spec 124 (impl-readiness-gate) Bead 2,
// AC-16/R9: the pre-drafted ADR-0041 fourth-verb clause (§1) is FINALIZED
// in this bead, in the same bead as the `next` gate-before-mutate code
// (the spec-117/122 amendment lifecycle: pre-drafted at plan time,
// finalized by the bead that lands the code, the adr0032_amendment_test.go
// / adr0040_anchor_test.go pattern).
//
// The match is over a WHITESPACE-NORMALIZED read of the shipped ADR
// (collapsing runs of whitespace/newlines to single spaces before
// matching) so a future reflow of the amendment's prose can never split
// the discriminating anchor across a line wrap and silently fail this
// test (plan-gate F3-2). The anchor is discriminating: it is vacuously
// ABSENT from the pre-spec-124 ADR-0041 (which enumerates only the
// original three lifecycle verbs) and present only after the amendment
// lands, so this test goes green only once the amendment is real.

func adr0041NormalizedText(t *testing.T) string {
	t.Helper()
	repoRoot := repoRootFromTestDir(t)
	adrPath := filepath.Join(repoRoot, ".mindspec", "adr", "ADR-0041-gate-before-mutate.md")
	data, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatalf("reading ADR-0041: %v", err)
	}
	return normalizeWhitespace(string(data))
}

// normalizeWhitespace collapses any run of whitespace (spaces, tabs,
// newlines) to a single space, so a raw single-line anchor phrase matches
// even if a future edit reflows the prose across multiple lines.
func normalizeWhitespace(s string) string {
	return strings.TrimSpace(wsRunRe.ReplaceAllString(s, " "))
}

var wsRunRe = regexp.MustCompile(`\s+`)

// TestADR0041FourthVerbClause_PreDraftMarkerGone pins the "FINALIZED, not
// merely staged" half: the plan-time PRE-DRAFT marker comment must be
// removed from the shipped ADR.
func TestADR0041FourthVerbClause_PreDraftMarkerGone(t *testing.T) {
	text := adr0041NormalizedText(t)
	if strings.Contains(text, "PRE-DRAFT") {
		t.Error("ADR-0041 still carries a PRE-DRAFT marker — the spec 124 fourth-verb clause must be FINALIZED in this bead")
	}
}

// TestADR0041FourthVerbClause_AnchorCoLocatedWithNext pins AC-16's
// discriminating anchor: the phrase "preflight-leg-only addition" appears
// within the same clause as `mindspec next` — proximity-checked (within a
// short window of the anchor) rather than requiring the exact original
// sentence shape, so a minor future rewording of the surrounding prose
// does not spuriously break this pin as long as the anchor stays
// co-located with the verb name it names.
func TestADR0041FourthVerbClause_AnchorCoLocatedWithNext(t *testing.T) {
	text := adr0041NormalizedText(t)

	const anchor = "preflight-leg-only addition"
	idx := strings.Index(text, anchor)
	if idx < 0 {
		t.Fatalf("ADR-0041 does not contain the discriminating anchor %q", anchor)
	}

	// Proximity window: the anchor and "mindspec next" must sit within
	// ~200 normalized characters of one another (comfortably inside a
	// single clause/sentence, however it wraps on disk).
	const window = 200
	start := idx - window
	if start < 0 {
		start = 0
	}
	end := idx + len(anchor) + window
	if end > len(text) {
		end = len(text)
	}
	nearby := text[start:end]

	if !strings.Contains(nearby, "mindspec next") {
		t.Errorf("the %q anchor is not co-located with `mindspec next` (nearby text: %q)", anchor, nearby)
	}
}

// TestADR0041FourthVerbClause_ScopeDeferral pins the scope-deferral
// framing R9/AC-16 require: the amendment must NOT certify `next`'s
// existing success-path claim/branch/worktree mutation chain as
// commit/reconcile-exempt — it is explicitly a scope-deferral over the
// preflight leg only.
func TestADR0041FourthVerbClause_ScopeDeferral(t *testing.T) {
	text := adr0041NormalizedText(t)

	for _, want := range []string{
		"scope-deferral",
		"governance certification",
		"success-path",
		"mutation chain",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("ADR-0041's fourth-verb clause is missing the scope-deferral anchor %q", want)
		}
	}
}

// TestADR0041FourthVerbClause_AllowNotReadyVsForce pins the
// `--allow-not-ready`-vs-`--force` distinction the amendment must state
// (orthogonal: --force gains no readiness authority).
func TestADR0041FourthVerbClause_AllowNotReadyVsForce(t *testing.T) {
	text := adr0041NormalizedText(t)

	for _, want := range []string{
		"--allow-not-ready",
		"--force",
		"byte-identical",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("ADR-0041's fourth-verb clause is missing anchor %q", want)
		}
	}
}

// TestADR0041FourthVerbClause_CitedAtGateCallSite pins the "the code
// cites it" half of AC-16: the `next` gate-before-mutate call site in
// cmd/mindspec/next.go names ADR-0041 and the "preflight-leg-only
// addition" clause it is finalizing (ADR-divergence gate visibility).
func TestADR0041FourthVerbClause_CitedAtGateCallSite(t *testing.T) {
	repoRoot := repoRootFromTestDir(t)
	nextGoPath := filepath.Join(repoRoot, "cmd", "mindspec", "next.go")
	data, err := os.ReadFile(nextGoPath)
	if err != nil {
		t.Fatalf("reading next.go: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "ADR-0041") {
		t.Error("cmd/mindspec/next.go's gate call site does not cite ADR-0041")
	}
	if !strings.Contains(content, "preflight-leg-only addition") {
		t.Error("cmd/mindspec/next.go's gate call site does not cite the \"preflight-leg-only addition\" clause")
	}
}
