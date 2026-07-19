package idvalidate

import (
	"os"
	"strings"
	"testing"
)

// readTestdataLines reads a newline-delimited fixture from testdata,
// skipping blank lines. Fails the test on any I/O error.
func readTestdataLines(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading testdata/%s: %v", name, err)
	}
	var lines []string
	for _, l := range strings.Split(string(data), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		lines = append(lines, l)
	}
	return lines
}

// TestIDValidateAcceptsLiveInventory pins the AC-1 live-inventory fixture:
// every spec ID currently on disk under .mindspec/specs/ (testdata/
// live_spec_ids.txt) and every bead ID currently minted by bd (testdata/
// live_bead_ids.txt) must pass the corrected validators. This test is RED
// against the pre-correction patterns (494 bead IDs + 2 spec dirs failed
// in the committed fixture; the exact count tracks the live inventory)
// and green after the R1 grammar correction — a regression here means a
// future change narrowed the grammar and would brick real IDs.
func TestIDValidateAcceptsLiveInventory(t *testing.T) {
	specIDs := readTestdataLines(t, "live_spec_ids.txt")
	if len(specIDs) == 0 {
		t.Fatal("live_spec_ids.txt fixture is empty")
	}
	var specFailures int
	for _, id := range specIDs {
		if err := SpecID(id); err != nil {
			specFailures++
			t.Errorf("SpecID(%q) unexpected error: %v", id, err)
		}
	}

	beadIDs := readTestdataLines(t, "live_bead_ids.txt")
	if len(beadIDs) == 0 {
		t.Fatal("live_bead_ids.txt fixture is empty")
	}
	var beadFailures int
	for _, id := range beadIDs {
		if err := BeadID(id); err != nil {
			beadFailures++
			t.Errorf("BeadID(%q) unexpected error: %v", id, err)
		}
	}

	t.Logf("live inventory: %d spec IDs (%d failed), %d bead IDs (%d failed)",
		len(specIDs), specFailures, len(beadIDs), beadFailures)
}

// TestIDValidateWideningPreservesRejections proves the R1 grammar
// correction is a strict WIDENING: every hostile input the pre-correction
// patterns rejected is still rejected, and a sample of IDs the old
// patterns accepted still passes under the new patterns.
func TestIDValidateWideningPreservesRejections(t *testing.T) {
	// Metacharacter/whitespace/control/glob/uppercase fragments are
	// never members of either pattern's charset, so splicing them
	// anywhere into an otherwise plausible ID is always hostile.
	metaFragments := []string{
		";", "|", "$", "&", "#", "`", "'", "\"",
		"/", "\\",
		" ", "\t", "\n",
		"\x1b", // control byte (ESC)
		"*", "?", "[", "]", "{", "}",
		"UPPER",
	}
	var hostileIDs []string
	for _, frag := range metaFragments {
		hostileIDs = append(hostileIDs,
			"mindspec-x1qr"+frag,
			"033-slug"+frag,
			frag+"mindspec-x1qr",
			frag+"033-slug",
		)
	}

	// Structural hostiles called out explicitly in the spec (AC-1):
	// bare/leading/trailing "." or "-", "..", and non-numeric dotted
	// children. These are listed as standalone literals (not spliced)
	// because "-" and "." are meaningful separators under the widened
	// grammar — naive splicing could accidentally produce a legitimate
	// extra segment instead of a hostile one.
	hostileIDs = append(hostileIDs,
		"",
		".",
		"..",
		"a..1",             // double dot
		"a.b",              // non-numeric child, no base segment
		"mindspec-x1qr.a",  // non-numeric child, realistic base
		"mindspec-x1qr..1", // double dot, realistic base
		"mindspec-x1qr.",   // trailing dot, no digits
		".mindspec-x1qr",   // leading dot
		"-mindspec-x1qr",   // leading hyphen
		"mindspec-x1qr-",   // trailing hyphen (empty final segment)
		"033-slug.",        // trailing dot on spec ID
		".033-slug",        // leading dot on spec ID
		"033-slug-",        // trailing hyphen on spec ID
		"-033-slug",        // leading hyphen on spec ID
	)

	for _, id := range hostileIDs {
		if err := SpecID(id); err == nil {
			t.Errorf("SpecID(%q) expected error (hostile), got nil", id)
		}
		if err := BeadID(id); err == nil {
			t.Errorf("BeadID(%q) expected error (hostile), got nil", id)
		}
	}

	// Widening direction: a sample of IDs the OLD (stricter) patterns
	// accepted must still pass under the corrected patterns.
	oldAcceptedSpecIDs := []string{
		"001-init",
		"033-security-hardening",
		"033-security-hardening-sast-findings",
		"999-foo",
		"0001-four-digits",
	}
	for _, id := range oldAcceptedSpecIDs {
		if err := SpecID(id); err != nil {
			t.Errorf("SpecID(%q) (old-accepted) unexpected error after widening: %v", id, err)
		}
	}
	oldAcceptedBeadIDs := []string{
		"mindspec-x1qr",
		"mindspec-abcd",
		"mindspec-0kce",
		"proj-abc123",
		"a-b-c-d-1234",
	}
	for _, id := range oldAcceptedBeadIDs {
		if err := BeadID(id); err != nil {
			t.Errorf("BeadID(%q) (old-accepted) unexpected error after widening: %v", id, err)
		}
	}
}

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
		// Corrected grammar (spec 120 R1): bd has no minimum suffix
		// length, and dotted numeric epic-children are bd's real,
		// hierarchical minting scheme — not a flat slug.
		"mindspec-xy",      // short suffix, no {4,} floor (was falsely rejected)
		"mindspec-0ke",     // 3-char suffix (live bd ID)
		"mindspec-mol-015", // multi-segment slug (live bd ID)
		"mindspec-9cyu.1",  // dotted epic-child
		"mindspec-69y.2.2", // nested epic-child
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
		"MINDSPEC-x1qr",      // uppercase
		"mindspec-x1qr/../x", // traversal
		"mindspec x1qr",      // space
		"-mindspec-x1qr",     // leading hyphen
		"mindspec--x1qr",     // double hyphen → empty segment
		"mindspec-x1qr*",     // glob
		"mindspec-x1qr?",     // glob
		"mindspec-x1qr..1",   // double dot in child position
		"mindspec-x1qr.a",    // non-numeric child
		"mindspec-x1qr.",     // trailing dot, no digits
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
