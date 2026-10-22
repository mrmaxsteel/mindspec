// Package termsafe provides the single, shared implementation of the
// safe-set/quote rule used to render untrusted strings for inclusion in
// terminal-facing output (spec 116 AC6): the rule that keeps a
// config-controlled or otherwise attacker-influenced string from forging
// extra, attacker-chosen display lines when it reaches a terminal raw.
package termsafe

import (
	"errors"
	"strconv"
	"strings"
)

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

// Failure holds a safe message and one recovery line. Only constructors set
// its fields, so callers can trust its formatting without checking error text.
type Failure struct {
	message  string
	recovery string
	cause    error
}

// EscapeRecovery escapes unsafe runes without changing shell quotes or
// printable Unicode. Callers must shell-quote each command argument first.
func EscapeRecovery(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strconv.IsPrint(r) {
			b.WriteRune(r)
		} else {
			quoted := strconv.QuoteRuneToASCII(r)
			b.WriteString(quoted[1 : len(quoted)-1])
		}
	}
	return b.String()
}

// NewFailure escapes raw message and recovery text once.
func NewFailure(message, recovery string) *Failure {
	return &Failure{message: Escape(message), recovery: EscapeRecovery(recovery)}
}

// NewWrappedFailure retains cause for errors.Is and errors.As.
func NewWrappedFailure(cause error, message, recovery string) *Failure {
	f := NewFailure(message, recovery)
	f.cause = cause
	return f
}

// JoinFailures joins safe messages under one recovery line. Plain errors are
// escaped once. Typed failures keep their already escaped messages.
func JoinFailures(failures []error, recovery string) *Failure {
	messages := make([]string, 0, len(failures))
	for _, err := range failures {
		var failure *Failure
		if errors.As(err, &failure) {
			messages = append(messages, failure.Message())
		} else {
			messages = append(messages, Escape(err.Error()))
		}
	}
	return &Failure{message: strings.Join(messages, "; "), recovery: EscapeRecovery(recovery)}
}

func (f *Failure) Error() string { return f.message + "\nrecovery: " + f.recovery }
func (f *Failure) Unwrap() error { return f.cause }

// Message returns the safe message without the recovery line.
func (f *Failure) Message() string { return f.message }
