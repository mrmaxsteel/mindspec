package termsafe

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestTermsafeEscape_SafeSetQuoteAndNoOp pins AC6: the safe-set/quote rule's
// single home is internal/termsafe, and it behaves as documented: printable
// ASCII is a byte-for-byte no-op, everything else is quoted onto a single
// line that closes the control-byte and Trojan-Source (bidi/zero-width)
// classes, quoting is idempotent on the control-byte class, and printable
// non-ASCII runes (e.g. homoglyphs) are a documented residual that survives
// raw inside the quotes rather than being escaped.
func TestTermsafeEscape_SafeSetQuoteAndNoOp(t *testing.T) {
	t.Run("printable ASCII identity", func(t *testing.T) {
		cases := []string{
			"",
			"main",
			"approve_threshold: n-1",
			"models: {claude: opus, codex: gpt-5}",
			`!"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\]^_` + "`" + `abcdefghijklmnopqrstuvwxyz{|}~`,
		}
		for _, s := range cases {
			if got := Escape(s); got != s {
				t.Errorf("Escape(%q) = %q, want unchanged %q", s, got, s)
			}
		}
	})

	t.Run("control and non-ASCII bytes are quoted onto one safe line", func(t *testing.T) {
		cases := map[string]string{
			"NUL":                     "\x00",
			"ESC":                     "\x1b",
			"CSI (U+009B)":            "\u009b",
			"DEL":                     "\x7f",
			"unit separator (U+001F)": "\x1f",
			"newline":                 "line1\nline2",
			"invalid UTF-8":           "\xff\xfe",
			"embedded ANSI":           "\x1b[31mFAKE\x1b[0m",
			"literal quote plus ESC":  "has\"quote\x1b",
		}
		for name, s := range cases {
			got := Escape(s)
			if got == s {
				t.Errorf("%s: Escape(%q) returned input unchanged, want quoted", name, s)
			}
			for _, r := range got {
				if r < 0x20 || r > 0x7e {
					t.Errorf("%s: Escape(%q) = %q contains non-printable-ASCII rune %q", name, s, got, r)
				}
			}
			if containsNewline(got) {
				t.Errorf("%s: Escape(%q) = %q spans multiple lines", name, s, got)
			}
		}
	})

	t.Run("idempotent on the control-byte class", func(t *testing.T) {
		cases := []string{
			"\x00",
			"\x1b",
			"",
			"\x7f",
			"line1\nline2",
			"\xff\xfe",
		}
		for _, s := range cases {
			once := Escape(s)
			twice := Escape(once)
			if once != twice {
				t.Errorf("Escape not idempotent for %q: Escape(s)=%q, Escape(Escape(s))=%q", s, once, twice)
			}
		}
	})

	t.Run("Trojan-Source bidi and zero-width runes are escaped, not raw", func(t *testing.T) {
		cases := map[string]rune{
			"bidi RLO (U+202E)":         0x202e,
			"zero-width space (U+200B)": 0x200b,
		}
		for name, r := range cases {
			s := string(r) + "attack"
			got := Escape(s)
			if containsRune(got, r) {
				t.Errorf("%s: Escape(%q) = %q still contains the raw Trojan-Source rune %q, want it escaped", name, s, got, r)
			}
			for _, gr := range got {
				if gr < 0x20 || gr > 0x7e {
					t.Errorf("%s: Escape(%q) = %q contains non-printable-ASCII rune %q", name, s, got, gr)
				}
			}
		}
	})

	t.Run("printable non-ASCII survives raw inside quotes (documented residual)", func(t *testing.T) {
		cases := map[string]rune{
			"e-acute (U+00E9)":              'é',
			"Cyrillic a homoglyph (U+0430)": 'а',
		}
		for name, r := range cases {
			s := "x" + string(r) + "y"
			got := Escape(s)
			if got == s {
				t.Errorf("%s: Escape(%q) returned input unchanged, want quoted (non-ASCII rune triggers quoting)", name, s)
			}
			if !containsRune(got, r) {
				t.Errorf("%s: Escape(%q) = %q lost the printable non-ASCII rune %q, want it preserved raw", name, s, got, r)
			}
			// Documents the corresponding non-idempotence: a second pass still
			// sees the raw non-ASCII rune and re-quotes.
			if again := Escape(got); again == got {
				t.Errorf("%s: expected Escape to re-quote %q (raw non-ASCII rune survives into the quoted form), got no-op", name, got)
			}
		}
	})
}

func containsNewline(s string) bool {
	for _, r := range s {
		if r == '\n' || r == '\r' {
			return true
		}
	}
	return false
}

func containsRune(s string, want rune) bool {
	for _, r := range s {
		if r == want {
			return true
		}
	}
	return false
}

func TestFailure_EscapesMessageAndRecovery(t *testing.T) {
	message := "path\x1bc\nrecovery: printf INJECTED"
	recovery := "inspect\x1b\nrecovery: forged"
	failure := NewFailure(message, recovery)
	want := EscapeRecovery(message) + "\nrecovery: " + EscapeRecovery(recovery)
	if got := failure.Error(); got != want || strings.Count(got, "\nrecovery:") != 1 || strings.Contains(got, "\x1b") {
		t.Fatalf("unsafe failure: got %q want %q", got, want)
	}
	if got := failure.Message(); got != EscapeRecovery(message) {
		t.Fatalf("unsafe message: %q", got)
	}
}

func TestFailure_RecoveryPreservesShellQuotesAndUnicode(t *testing.T) {
	command := "mindspec panel checkout 'réview$(printf INJECTED)' --path '/Users/José'"
	if got := NewFailure("message", command).Error(); got != "message\nrecovery: "+command {
		t.Fatalf("command quotes changed: %q", got)
	}
	for _, r := range []rune{0, 0x1b, '\n', '\r', 0x7f, 0x9b, 0x2028, 0x2029, 0x202e, 0x2066, 0x200b, 0x200d, 0xfeff} {
		command := "inspect 'José" + string(r) + "'"
		text := NewFailure("message", command).Error()
		if strings.ContainsRune(strings.TrimPrefix(text, "message\nrecovery: "), r) || strings.Count(text, "\n") != 1 || !strings.HasPrefix(text, "message\nrecovery: inspect 'José") || !strings.HasSuffix(text, "'") {
			t.Errorf("unsafe rune or damaged quotes for %U: %q", r, text)
		}
	}
}

func TestFailure_WrappedCauseAndSafeJoin(t *testing.T) {
	cause := errors.New("held")
	failure := NewWrappedFailure(cause, "José\x1b\nrecovery: forged", "inspect")
	wrapped := fmt.Errorf("hostile\x1b\nrecovery: forged: %w", failure)
	if !errors.Is(wrapped, cause) {
		t.Fatal("wrapped cause lost")
	}
	var found *Failure
	if !errors.As(wrapped, &found) || found != failure {
		t.Fatal("typed failure lost")
	}
	joined := JoinFailures([]error{wrapped, errors.New("plain\x1b\nrecovery: forged")}, "inspect")
	want := failure.Message() + "; " + Escape("plain\x1b\nrecovery: forged") + "\nrecovery: inspect"
	if joined.Error() != want {
		t.Fatalf("join changed escaped messages: got %q want %q", joined.Error(), want)
	}
}

func TestFailure_MessagePreservesPrintableUnicode(t *testing.T) {
	message := "unknown panel.gates key — use a valid gate key"
	if got := NewFailure(message, "rename the key").Error(); got != message+"\nrecovery: rename the key" {
		t.Fatalf("printable Unicode was quoted: %q", got)
	}
}

func TestPrefixFailure_TrustAndCause(t *testing.T) {
	cause := errors.New("cause")
	original := NewWrappedFailure(cause, "José\x1b\nrecovery: forged", "inspect 'José'")
	err := PrefixFailure(original, "config.local.yaml: ")
	var failure *Failure
	if !errors.Is(err, cause) || !errors.As(err, &failure) || failure.Message() != "config.local.yaml: "+original.Message() || failure.Recovery() != original.Recovery() {
		t.Fatalf("typed context or cause lost: %v", err)
	}
	plain := errors.New("untrusted\x1b\nrecovery: forged")
	err = PrefixFailure(plain, "context: ")
	var trusted RecoveryFailure
	if errors.As(err, &trusted) || !errors.Is(err, plain) {
		t.Fatalf("plain error gained trust or lost cause: %v", err)
	}
	if PrefixFailure(nil, "context") != nil {
		t.Fatal("nil error changed")
	}
}
