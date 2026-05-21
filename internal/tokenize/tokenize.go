// Package tokenize defines the Tokenizer interface used by
// internal/contextpack to budget bead context bundles. The
// default Approx implementation uses runes/3.7 (rounded down)
// and is documented as accurate to +/-3% of a reference BPE
// tokenizer on English+code text in the 500-2000 token range.
// Callers MUST NOT depend on the precise rune-ratio constant;
// a future BPE-backed Tokenizer may drop in as long as it
// satisfies the same +/-3% contract on the reference corpus.
//
// This package has zero external dependencies beyond the
// standard library (unicode/utf8 for Approx). It MUST NOT
// import os/exec, internal/gitutil, or internal/executor —
// the spec 085 boundary lint (TestEnforcementHasNoGitLeaks)
// enforces this constraint and TestTokenizeNoForbiddenImports
// in this package provides a local guard for the same bans.
package tokenize

import "unicode/utf8"

// Tokenizer counts approximate tokens in a string per a
// documented contract (see package doc). Pluggable; the
// default implementation is Approx.
type Tokenizer interface {
	Count(s string) int
	Name() string
}

// Approx is the default Tokenizer: runes/3.7 rounded down.
// Accurate to +/-3% on English+code in the 500-2000 token
// range. Name returns "approx".
type Approx struct{}

// Count returns int(float64(utf8.RuneCountInString(s)) / 3.7),
// the documented runes/3.7 truncation. Truncation (not rounding)
// is intentional: it makes Count monotone in input length and
// matches the rune-aligned tail-shave loop used by the budgeter
// (Bead 2) without off-by-one drift at the cap.
func (Approx) Count(s string) int {
	return int(float64(utf8.RuneCountInString(s)) / 3.7)
}

// Name returns "approx".
func (Approx) Name() string { return "approx" }
