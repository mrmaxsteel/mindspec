package validate

import (
	"strconv"
	"strings"
	"testing"
)

// R4 (spec 120): the ambiguous-ownership validation error interpolates
// on-disk domain-dir basenames (listDomainDirs -> os.ReadDir e.Name(),
// agent-creatable, never idvalidate'd) via strings.Join, and that Issue
// message is FormatText'd straight to the terminal. A hostile domain-dir
// basename that (with another domain) claims the same path must be forced
// through strconv.Quote per element, never rendered raw.
func TestNormalizeImpactedDomainsAmbiguousOwner_HostileBasenameForcedQuoted(t *testing.T) {
	root := t.TempDir()
	hostileDomain := "alpha\nFORGED: this Impacted-Domains entry is fine"
	writeManifest(t, root, hostileDomain, "paths:\n  - internal/foo/**\n")
	writeManifest(t, root, "beta", "paths:\n  - internal/foo/**\n")

	_, errs := normalizeImpactedDomains(nil, root, "", []string{"internal/foo/x.go"})
	if len(errs) != 1 {
		t.Fatalf("expected exactly one ambiguity error, got %v", errs)
	}
	msg := errs[0]
	// No forged line: the hostile newline must not carry FORGED onto its own
	// terminal line.
	if strings.Contains(msg, "\nFORGED:") {
		t.Errorf("hostile domain basename newline rendered raw (forged line):\n%s", msg)
	}
	// The basename is present but forced-quoted (termsafe.Escape ->
	// strconv.Quote for control-bearing input).
	if !strings.Contains(msg, strconv.Quote(hostileDomain)) {
		t.Errorf("expected forced-quoted domain %q in ambiguity error:\n%s", strconv.Quote(hostileDomain), msg)
	}
}

// A genuine (clean) domain basename in the same ambiguity error renders
// byte-identical — termsafe.Escape is the identity on printable input.
func TestNormalizeImpactedDomainsAmbiguousOwner_CleanBasenameByteIdentical(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "alpha", "paths:\n  - internal/foo/**\n")
	writeManifest(t, root, "beta", "paths:\n  - internal/foo/**\n")

	_, errs := normalizeImpactedDomains(nil, root, "", []string{"internal/foo/x.go"})
	if len(errs) != 1 {
		t.Fatalf("expected exactly one ambiguity error, got %v", errs)
	}
	// Clean owners appear verbatim (not quoted).
	if !strings.Contains(errs[0], "alpha, beta") {
		t.Errorf("clean domain basenames not rendered byte-identical:\n%s", errs[0])
	}
}
