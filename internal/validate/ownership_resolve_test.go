package validate

import (
	"strings"
	"testing"
)

// TestNormalizeImpactedDomainsKeepsDomainDirNameVerbatim — an entry that
// IS a domain dir name is returned unchanged, no glob-match attempted
// (spec 100 R1 rule 1).
func TestNormalizeImpactedDomainsKeepsDomainDirNameVerbatim(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "workflow", "paths:\n  - internal/validate/**\n")

	got, errs := normalizeImpactedDomains(nil, root, "", []string{"workflow"})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(got) != 1 || got[0] != "workflow" {
		t.Fatalf("expected [workflow] verbatim, got %v", got)
	}
}

// TestNormalizeImpactedDomainsGlobResolvesPathEntry — a path-like entry
// resolves to its single owning domain NAME (spec 100 R1 rule 3, one
// owner).
func TestNormalizeImpactedDomainsGlobResolvesPathEntry(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "workflow", "paths:\n  - internal/validate/**\n")

	got, errs := normalizeImpactedDomains(nil, root, "", []string{"internal/validate/plan.go"})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(got) != 1 || got[0] != "workflow" {
		t.Fatalf("expected path entry to resolve to [workflow], got %v", got)
	}
}

// TestNormalizeImpactedDomainsZeroOwnerErrors — a path-like entry no
// manifest claims produces an error naming the entry (spec 100 R1 rule
// 3, zero owners).
func TestNormalizeImpactedDomainsZeroOwnerErrors(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "workflow", "paths:\n  - internal/validate/**\n")

	_, errs := normalizeImpactedDomains(nil, root, "", []string{"internal/nope/x.go"})
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %v", errs)
	}
	if !strings.Contains(errs[0], "internal/nope/x.go") {
		t.Fatalf("error should name the entry, got %q", errs[0])
	}
}

// TestNormalizeImpactedDomainsAmbiguousOwnerErrors — reuse the
// overlapping-glob shape (alpha + beta both claim internal/foo/**); a
// path under that prefix is owned by two domains → ambiguity error
// naming both owners (spec 100 R1 rule 3, >1 owner).
func TestNormalizeImpactedDomainsAmbiguousOwnerErrors(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "alpha", "paths:\n  - internal/foo/**\n")
	writeManifest(t, root, "beta", "paths:\n  - internal/foo/**\n")

	_, errs := normalizeImpactedDomains(nil, root, "", []string{"internal/foo/x.go"})
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %v", errs)
	}
	if !strings.Contains(errs[0], "internal/foo/x.go") ||
		!strings.Contains(errs[0], "alpha") ||
		!strings.Contains(errs[0], "beta") {
		t.Fatalf("ambiguity error should name the entry and both owners, got %q", errs[0])
	}
}

// TestNormalizeImpactedDomainsKeepsBareNameWithoutManifest — a bare
// name token with no on-disk domain dir is kept verbatim (rule 2), so
// legacy named-domain specs without a manifest are unchanged.
func TestNormalizeImpactedDomainsKeepsBareNameWithoutManifest(t *testing.T) {
	root := t.TempDir()
	// no manifests at all
	got, errs := normalizeImpactedDomains(nil, root, "", []string{"payments"})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors for bare name: %v", errs)
	}
	if len(got) != 1 || got[0] != "payments" {
		t.Fatalf("expected [payments] kept verbatim, got %v", got)
	}
}
