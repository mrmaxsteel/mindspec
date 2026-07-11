package termsafe

import "testing"

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
