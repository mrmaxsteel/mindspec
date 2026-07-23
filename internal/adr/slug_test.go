package adr

import (
	"strings"
	"testing"
)

// TestDeriveSlug pins the AC-8 kebab-derivation rules (spec 123 R5(a)):
// lowercase, non-alphanumeric runs collapse to a single hyphen,
// leading/trailing hyphens trim, and a punctuation-only title derives
// empty (the bare-filename fallback trigger).
func TestDeriveSlug(t *testing.T) {
	cases := []struct {
		name  string
		title string
		want  string
	}{
		{"simple multi-word", "Integrate at contracts, not tools", "integrate-at-contracts-not-tools"},
		{"punctuation only", "!!! ??? ---", ""},
		{"mixed case and digits", "Use Go 1.22 Toolchain", "use-go-1-22-toolchain"},
		{"already hyphenated collapses", "multi--dash---title", "multi-dash-title"},
		{"leading/trailing punctuation trims", "  --Hello World--  ", "hello-world"},
		{"empty title", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveSlug(tc.title); got != tc.want {
				t.Errorf("deriveSlug(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}

// TestDeriveSlug_CapAtHyphenBoundary pins the 48-char cap, backed off to
// the nearest hyphen so a truncation mid-word never leaves a partial
// token dangling.
func TestDeriveSlug_CapAtHyphenBoundary(t *testing.T) {
	title := strings.Repeat("word ", 20) // derives to ~99 chars, well over the cap
	got := deriveSlug(title)

	if len(got) > maxSlugLen {
		t.Fatalf("slug exceeds cap: len=%d slug=%q", len(got), got)
	}
	if strings.HasSuffix(got, "-") {
		t.Fatalf("slug must not end with a dangling hyphen: %q", got)
	}
	for _, tok := range strings.Split(got, "-") {
		if tok != "word" {
			t.Fatalf("truncation left a partial token: %q (full slug %q)", tok, got)
		}
	}
}

// TestDeriveSlug_LongSingleTokenHardTruncates pins the no-hyphen-boundary
// edge case: a single token longer than the cap has no hyphen to back
// off to, so it hard-truncates at maxSlugLen rather than emptying out.
func TestDeriveSlug_LongSingleTokenHardTruncates(t *testing.T) {
	title := strings.Repeat("a", 60)
	got := deriveSlug(title)
	if len(got) != maxSlugLen {
		t.Fatalf("got len %d, want hard truncation to %d", len(got), maxSlugLen)
	}
}

// TestResolveSlug pins the --slug override contract (AC-8): nil override
// derives from the title; a present-but-empty override opts out of
// slugging (same as an empty derived slug); a valid override is used
// verbatim; a malformed override is refused with a recovery line.
func TestResolveSlug(t *testing.T) {
	t.Run("nil override derives from title", func(t *testing.T) {
		got, err := resolveSlug("Integrate at contracts, not tools", nil)
		if err != nil {
			t.Fatalf("resolveSlug: %v", err)
		}
		if got != "integrate-at-contracts-not-tools" {
			t.Errorf("got %q, want derived slug", got)
		}
	})

	t.Run("empty override opts out", func(t *testing.T) {
		empty := ""
		got, err := resolveSlug("Some Title", &empty)
		if err != nil {
			t.Fatalf("resolveSlug: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty (bare-filename fallback)", got)
		}
	})

	t.Run("valid override used verbatim", func(t *testing.T) {
		v := "my-custom-slug"
		got, err := resolveSlug("Ignored Title", &v)
		if err != nil {
			t.Fatalf("resolveSlug: %v", err)
		}
		if got != v {
			t.Errorf("got %q, want override %q", got, v)
		}
	})

	t.Run("malformed override refused with recovery line", func(t *testing.T) {
		bad := "Not A Valid Slug!"
		_, err := resolveSlug("Ignored Title", &bad)
		if err == nil {
			t.Fatal("expected error for malformed --slug, got nil")
		}
		if !strings.Contains(err.Error(), "recovery:") {
			t.Errorf("expected a recovery line, got: %v", err)
		}
	})
}
