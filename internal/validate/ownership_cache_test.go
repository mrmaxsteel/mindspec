package validate

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// installOwnershipLoadCounter swaps the loadOwnershipForRefFn seam for a
// counting wrapper and returns a LIVE per-domain load-count map. Read it
// after running the code under test; clear it (clearCounts) to reuse
// across runs. The seam is restored at test end. These tests mutate a
// package-level var and so must NOT run in parallel.
func installOwnershipLoadCounter(t *testing.T) map[string]int {
	t.Helper()
	orig := loadOwnershipForRefFn
	counts := map[string]int{}
	loadOwnershipForRefFn = func(exec executor.Executor, root, ownerRef, domain string) (*Ownership, error) {
		counts[domain]++
		return orig(exec, root, ownerRef, domain)
	}
	t.Cleanup(func() { loadOwnershipForRefFn = orig })
	return counts
}

func clearCounts(counts map[string]int) {
	for k := range counts {
		delete(counts, k)
	}
}

func totalCount(counts map[string]int) int {
	n := 0
	for _, c := range counts {
		n += c
	}
	return n
}

// assertMaxOnePerDomain is the RED-on-revert core: a per-(file × domain)
// re-load would load a domain's manifest once per file that reaches it,
// so any count > 1 across a multi-file/entry diff means the hoist was
// reverted.
func assertMaxOnePerDomain(t *testing.T, counts map[string]int, label string) {
	t.Helper()
	for d, n := range counts {
		if n > 1 {
			t.Errorf("%s: domain %q OWNERSHIP.yaml loaded %d times; want at most 1 (per-domain-not-per-file)", label, d, n)
		}
	}
}

// TestDivergenceOwnershipLoadedPerDomainNotPerFile proves spec 108 R7 for
// the ValidateDivergence call site: over a multi-file diff each candidate
// domain's OWNERSHIP.yaml is loaded at most once, and the total load count
// is independent of the number of changed files.
func TestDivergenceOwnershipLoadedPerDomainNotPerFile(t *testing.T) {
	root, specDir, _ := diagCachingFixture(t) // domains: billing, payments, search
	counts := installOwnershipLoadCounter(t)

	run := func(files []string) int {
		clearCounts(counts)
		mock := &executor.MockExecutor{ChangedFilesResult: files}
		ValidateDivergence(mock, root, specDir, "mindspec-r7.1", "BASE", "HEAD", "", false)
		assertMaxOnePerDomain(t, counts, "divergence")
		return totalCount(counts)
	}

	// The unowned file forces a full scan of all three domains, so every
	// candidate manifest is loaded exactly once per run.
	small := run([]string{
		"internal/payments/charge.go",
		"internal/unclaimed/x.go",
	})
	big := run([]string{
		"internal/payments/charge.go",
		"internal/payments/refund.go",
		"internal/search/query.go",
		"internal/search/index.go",
		"internal/billing/invoice.go",
		"internal/billing/dunning.go",
		"internal/unclaimed/x.go",
		"internal/unclaimed/y.go",
	})

	if small != big {
		t.Errorf("load count depends on changed-file count: small=%d big=%d; want equal (domain-count function only)", small, big)
	}
	if big == 0 {
		t.Fatal("expected at least one manifest load; counter never fired")
	}
}

// TestCheckInternalPackagesOwnershipLoadedPerDomainNotPerFile proves spec
// 108 R7 for the doc-sync internal-packages call site.
func TestCheckInternalPackagesOwnershipLoadedPerDomainNotPerFile(t *testing.T) {
	root, _, _ := diagCachingFixture(t) // domains: billing, payments, search on disk
	counts := installOwnershipLoadCounter(t)

	run := func(source []string) int {
		clearCounts(counts)
		r := &Result{SubCommand: "docs"}
		// No exec needed on the on-disk (ownerRef "") path; docs empty so
		// the lane still attributes every source file.
		checkInternalPackages(r, nil, root, "", source, nil)
		assertMaxOnePerDomain(t, counts, "checkInternalPackages")
		return totalCount(counts)
	}

	small := run([]string{
		"internal/payments/a.go",
		"internal/unclaimed/z.go", // unowned → full scan of all domains
	})
	big := run([]string{
		"internal/payments/a.go",
		"internal/payments/b.go",
		"internal/search/c.go",
		"internal/search/d.go",
		"internal/billing/e.go",
		"internal/billing/f.go",
		"internal/unclaimed/z.go",
	})

	if small != big {
		t.Errorf("load count depends on source-file count: small=%d big=%d; want equal", small, big)
	}
	if big == 0 {
		t.Fatal("expected at least one manifest load; counter never fired")
	}
}

// TestNormalizeImpactedDomainsOwnershipLoadedPerDomain proves spec 108 R7
// for the Impacted-Domains normalization call site: each path-like entry
// is glob-matched against every domain, but the manifests are loaded once
// for the whole entry set, not once per (entry × domain).
func TestNormalizeImpactedDomainsOwnershipLoadedPerDomain(t *testing.T) {
	root, _, _ := diagCachingFixture(t) // domains: billing, payments, search on disk
	counts := installOwnershipLoadCounter(t)

	run := func(entries []string) int {
		clearCounts(counts)
		// Path-like entries each trigger the per-domain glob loop.
		_, errs := normalizeImpactedDomains(nil, root, "", entries)
		if len(errs) != 0 {
			t.Fatalf("unexpected normalization errors: %v", errs)
		}
		assertMaxOnePerDomain(t, counts, "normalizeImpactedDomains")
		return totalCount(counts)
	}

	small := run([]string{
		"internal/payments/x.go",
		"internal/search/y.go",
	})
	big := run([]string{
		"internal/payments/x.go",
		"internal/payments/x2.go",
		"internal/search/y.go",
		"internal/search/y2.go",
		"internal/billing/z.go",
		"internal/billing/z2.go",
	})

	if small != big {
		t.Errorf("load count depends on entry count: small=%d big=%d; want equal (domain-count function only)", small, big)
	}
	if big == 0 {
		t.Fatal("expected at least one manifest load; counter never fired")
	}
}
