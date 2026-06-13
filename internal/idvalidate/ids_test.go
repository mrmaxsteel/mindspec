package idvalidate

import (
	"strings"
	"testing"
)

func TestSpecID(t *testing.T) {
	valid := []string{
		"001-init",
		"033-security-hardening",
		"033-security-hardening-sast-findings",
		"999-foo",
		"0001-four-digits",
	}
	for _, id := range valid {
		if err := SpecID(id); err != nil {
			t.Errorf("SpecID(%q) unexpected error: %v", id, err)
		}
	}

	invalid := []string{
		"",
		".",
		"..",
		"../etc/passwd",
		"foo/bar",
		"foo\\bar",
		"33-too-short",   // only 2 digits
		"no-digits",      // missing leading digits
		"033",            // no slug
		"033-",           // trailing hyphen
		"033-UPPERCASE",  // uppercase
		"033-has spaces", // spaces
		"033-has_under",  // underscores
	}
	for _, id := range invalid {
		if err := SpecID(id); err == nil {
			t.Errorf("SpecID(%q) expected error, got nil", id)
		}
	}
}

func TestADRID(t *testing.T) {
	valid := []string{
		"ADR-0001",
		"ADR-0042-secure-paths",
		"ADR-9999",
		"ADR-0001-multi-word-slug",
		"ADR-12345", // 5 digits OK
	}
	for _, id := range valid {
		if err := ADRID(id); err != nil {
			t.Errorf("ADRID(%q) unexpected error: %v", id, err)
		}
	}

	invalid := []string{
		"",
		".",
		"..",
		"../etc/passwd",
		"ADR/0001",
		"ADR\\0001",
		"adr-0001",        // lowercase
		"ADR-0001*",       // glob metachar
		"ADR-0001?",       // glob metachar
		"ADR-0001[",       // glob metachar
		"ADR-0001]",       // glob metachar
		"ADR-0001{x,y}",   // brace expansion
		"ADR-",            // empty number
		"ADR-001",         // 3 digits
		"ADR-0001-UPPER",  // uppercase slug
		"ADR-0001/../bad", // traversal
		"/ADR-0001",       // leading slash
		"ADR-0001 ",       // trailing space
		"ADR-*",           // glob
	}
	for _, id := range invalid {
		if err := ADRID(id); err == nil {
			t.Errorf("ADRID(%q) expected error, got nil", id)
		}
	}
}

func TestBeadID(t *testing.T) {
	valid := []string{
		"mindspec-x1qr",
		"mindspec-abcd",
		"mindspec-0kce",
		"proj-abc123",
		"a-b-c-d-1234",
	}
	for _, id := range valid {
		if err := BeadID(id); err != nil {
			t.Errorf("BeadID(%q) unexpected error: %v", id, err)
		}
	}

	invalid := []string{
		"",
		".",
		"..",
		"mindspec/x1qr",
		"mindspec\\x1qr",
		"mindspec-",          // empty suffix
		"mindspec-xy",        // suffix too short
		"MINDSPEC-x1qr",      // uppercase
		"mindspec-x1qr/../x", // traversal
		"mindspec x1qr",      // space
		"-mindspec-x1qr",     // leading hyphen
		"mindspec--x1qr",     // double hyphen → empty segment
		"mindspec-x1qr*",     // glob
		"mindspec-x1qr?",     // glob
	}
	for _, id := range invalid {
		if err := BeadID(id); err == nil {
			t.Errorf("BeadID(%q) expected error, got nil", id)
		}
	}
}

func TestDomainName(t *testing.T) {
	valid := []string{
		"security",
		"cli-handlers",
		"context-system", // existing domain dir, must stay valid
		"a",
		"a-b-c",
		"foo123",
	}
	for _, name := range valid {
		if err := DomainName(name); err != nil {
			t.Errorf("DomainName(%q) unexpected error: %v", name, err)
		}
	}

	invalid := []string{
		"",
		".",
		"..",
		"Security",     // uppercase
		"1security",    // digit start
		"security/",    // path sep
		"security\\",   // backslash (POSIX glob escape / Windows path sep) pin
		"sec\\urity",   // embedded backslash pin
		"sec*urity",    // glob metacharacter pin
		"../etc",       // traversal
		"security_x",   // underscore
		"-leading",     // leading hyphen
		"cli-",         // R6: trailing hyphen
		"x-",           // R6: trailing hyphen
		"a--b",         // R6: consecutive hyphens
		"context--sys", // R6: consecutive hyphens
	}
	for _, name := range invalid {
		if err := DomainName(name); err == nil {
			t.Errorf("DomainName(%q) expected error, got nil", name)
		}
	}
}

// TestDomainNameErrorNoRawRegex pins R7: the unified DomainName error message
// must be plain-English and must NOT leak the raw regex (no "[a-z]" substring).
func TestDomainNameErrorNoRawRegex(t *testing.T) {
	for _, name := range []string{"Bad", "cli-", "a--b", "."} {
		err := DomainName(name)
		if err == nil {
			t.Fatalf("DomainName(%q) expected error, got nil", name)
		}
		if strings.Contains(err.Error(), "[a-z]") {
			t.Errorf("DomainName(%q) error leaks raw regex: %q", name, err.Error())
		}
	}
}
