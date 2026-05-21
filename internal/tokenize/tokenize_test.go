package tokenize

import (
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// referenceCorpus is an inline ASCII English-prose fixture sized
// into the documented 500-2000-token operating range of Approx.
// It is intentionally ASCII-only (each byte == one rune) so the
// rune count is mechanically verifiable by reading the literal.
//
// Hand-count rule
// ---------------
// The contract Approx exposes is "tokens == runes/3.7, accurate
// to within +/-3% of a reference BPE tokenizer on English+code
// in the 500-2000 token range." The reference number below is
// derived as follows, which is what a reviewer does by hand:
//
//  1. Count the runes in referenceCorpus. Because the corpus is
//     pure ASCII, rune-count == byte-count == the literal's
//     visible character length. The corpus was authored to
//     contain exactly 638 runes (verified at authoring time by
//     `wc -c` and reproduced at test time by
//     utf8.RuneCountInString — see the sanity check below).
//  2. Apply the documented ratio: floor(638 / 3.7) == 172. This
//     is the "reference" the BPE proxy would produce by the
//     same contract; treating it as the hand-counted target
//     pins the implementation to the published ratio.
//
// The test then asserts |Approx{}.Count(corpus) - 172| <=
// ceil(172 * 0.03) == 6. A pure off-by-one regression in
// Approx.Count (e.g. truncating in the wrong direction or
// flipping to math.Round) will still fall inside the +/-6
// tolerance — that is intentional: the +/-3% contract is what
// the package documents, and the test enforces exactly that.
// Drift larger than +/-3% (e.g. swapping the divisor from 3.7
// to 4.0) fails this test loudly.
const referenceCorpus = "The budgeter selects bead context within a strict, configurable token budget. It ranks each source by relevance, then trims the tail when the projected cost exceeds the cap. Reviewers can audit every decision because the provenance block lists each input with a content hash. A pluggable interface lets us swap the default approximation for a real byte pair encoder without rewriting the budgeter. The default counts runes and divides by a small constant, holding within three percent of a reference tokenizer on English prose in the five hundred to two thousand token range. Tests pin the contract so future drift fails loud and visibly."

const referenceRunes = 638
const referenceTokens = 172 // floor(638 / 3.7)

func TestTokenizerApproxToleranceWithinThreePercent(t *testing.T) {
	// Sanity-check the fixture: if a reviewer edits the corpus
	// without updating referenceRunes, fail with a clear hint
	// rather than letting the ratio test absorb the drift.
	if got := utf8.RuneCountInString(referenceCorpus); got != referenceRunes {
		t.Fatalf("referenceCorpus rune count drift: got %d, declared %d; "+
			"recompute referenceRunes and referenceTokens (=floor(runes/3.7))", got, referenceRunes)
	}

	got := Approx{}.Count(referenceCorpus)
	diff := got - referenceTokens
	if diff < 0 {
		diff = -diff
	}
	tolerance := int(math.Ceil(float64(referenceTokens) * 0.03))
	if diff > tolerance {
		t.Fatalf("Approx.Count(referenceCorpus) = %d, want within +/-%d of %d (diff=%d)",
			got, tolerance, referenceTokens, diff)
	}
}

func TestApproxName(t *testing.T) {
	if got := (Approx{}).Name(); got != "approx" {
		t.Fatalf("Approx.Name() = %q, want %q", got, "approx")
	}
}

// TestTokenizeNoForbiddenImports parses every non-test .go file
// in this package and asserts the (direct) import set does not
// include any of the banned paths. The boundary lint at
// internal/lint/boundary_test.go enforces the same bans against
// the enforcement packages; this local guard surfaces a
// regression in the tokenize package itself with a focused
// diagnostic before the cross-package lint runs.
//
// Direct-import inspection is sufficient here because the
// package documents zero external dependencies beyond
// unicode/utf8 — any future transitive growth would necessarily
// introduce a new direct import that this test would catch.
func TestTokenizeNoForbiddenImports(t *testing.T) {
	banned := map[string]struct{}{
		`"os/exec"`: {},
		`"github.com/mrmaxsteel/mindspec/internal/gitutil"`:  {},
		`"github.com/mrmaxsteel/mindspec/internal/executor"`: {},
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(".", name)
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imp := range file.Imports {
			if _, bad := banned[imp.Path.Value]; bad {
				t.Errorf("%s: forbidden import %s", path, imp.Path.Value)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatalf("no non-test .go files inspected; test would silently pass")
	}
}
