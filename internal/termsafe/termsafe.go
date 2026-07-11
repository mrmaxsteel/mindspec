// Package termsafe provides the single, shared implementation of the
// safe-set/quote rule used to render untrusted strings for inclusion in
// terminal-facing output (spec 116 AC6): the rule that keeps a
// config-controlled or otherwise attacker-influenced string from forging
// extra, attacker-chosen display lines when it reaches a terminal raw.
package termsafe

import "strconv"

// Escape renders s safely for terminal-facing output.
//
// Safe-set rule: s renders UNCHANGED iff every rune in it is printable
// ASCII in [0x20, 0x7e] (space through `~`) — this covers letters, digits,
// and all ASCII punctuation, so every existing plain value (including an
// expression like `n-1`) is byte-for-byte identical to before.
//
// Anything else — any rune outside that range — causes the WHOLE string to
// be rendered as a single-line, double-quoted Go string literal via
// strconv.Quote. Quote cannot itself emit a raw control byte or a literal
// newline, so a hostile value can never span or forge additional output
// lines. This closes C0/C1 control bytes (including ESC/BEL and \n/\r),
// DEL, and invalid UTF-8, and — because strconv.IsPrint treats them as
// non-printable — also Trojan-Source bidi-override runes (e.g. U+202E) and
// zero-width runes (e.g. U+200B), even though those fall outside the
// C0/C1/DEL control-byte class.
//
// Residual, by design: a rune that is both non-ASCII and printable (an
// accented letter, or a Cyrillic/Greek homoglyph) is enough to trigger
// quoting on its own, but strconv.Quote leaves that rune RAW inside the
// surrounding quotes — it is not escaped. So the output is not guaranteed
// to be printable-ASCII-only, and Escape is not idempotent on a string
// that mixes such a rune with a control byte: the second pass still sees
// the raw non-ASCII rune and re-quotes. Suppressing printable, single-line,
// non-ASCII content is out of scope for this rule; it defends against
// control-byte/line-forging injection, not homoglyph confusability of
// otherwise legitimate text (see spec 116 Background).
func Escape(s string) string {
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return strconv.Quote(s)
		}
	}
	return s
}
