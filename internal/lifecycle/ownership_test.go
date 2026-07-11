// ownership_test.go — spec 115 AC10: the structural exactly-one-claimant
// pin for this package's domain ownership.
//
// `internal/lifecycle/**` must be claimed by EXACTLY ONE domain across
// every `.mindspec/domains/*/OWNERSHIP.yaml`, and that one claimant must
// be the WORKFLOW domain (the round-1 panel's item-8 correction: all four
// consumers — complete/next/doctor/approve — are workflow-owned, and the
// package self-describes as workflow-lifecycle detectors). RED-on-revert
// by construction: a third domain adding a claim on this package, or the
// workflow claim reverting away, fails this test.
package lifecycle

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/validate"
	"gopkg.in/yaml.v3"
)

// lifecycleProbePath is the representative real file used to decide
// whether an OWNERSHIP.yaml glob pattern "claims" internal/lifecycle:
// a pattern claims the package iff it would glob-match this file under
// the SAME matcher production ownership resolution uses
// (validate.GlobMatch, spec 086/091's `**`-aware dialect — NOT a
// hand-rolled string-prefix heuristic, which misses a broader claimant
// like `internal/**` that would also match this file in production).
const lifecycleProbePath = "internal/lifecycle/orphans.go"

// ownershipRepoRoot returns the absolute path to the repo root by
// walking up from the test's runtime CWD (the package directory, per
// `go test`) until go.mod is found — the same convention as
// internal/specgate's repoRoot. The 8-level cap guards against runaway
// loops; this package sits 2 levels below the root.
func ownershipRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "go.mod")
		info, statErr := os.Stat(candidate)
		if statErr == nil && !info.IsDir() {
			return dir
		}
		if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("stat %s: %v", candidate, statErr)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate go.mod walking up from %s (8-level cap reached)", cwd)
	return ""
}

// claimsLifecyclePackage reports whether one OWNERSHIP.yaml path
// pattern claims this package, using OVERLAP semantics: a pattern
// claims internal/lifecycle iff it glob-matches lifecycleProbePath
// under production's own matcher (validate.GlobMatch). This is
// deliberately NOT a string-prefix check: a broader pattern like
// `internal/**` from an unrelated domain would also match
// lifecycleProbePath in production (and so WOULD claim the package)
// even though it does not share the `internal/lifecycle` prefix
// textually — a prefix heuristic would silently miss that overlap and
// let the "exactly one claimant" invariant go unpinned.
func claimsLifecyclePackage(pattern string) bool {
	return validate.GlobMatch(pattern, lifecycleProbePath)
}

// lifecycleClaimants applies claimsLifecyclePackage across a
// domain-name -> path-globs map and returns the sorted list of
// claiming domains. Shared by the real-manifest test and the
// synthetic discrimination subtests below so both exercise the exact
// same counting logic the AC10 invariant relies on.
func lifecycleClaimants(domainPaths map[string][]string) []string {
	var claimants []string
	for domain, paths := range domainPaths {
		for _, pattern := range paths {
			if claimsLifecyclePackage(pattern) {
				claimants = append(claimants, domain)
				break
			}
		}
	}
	sort.Strings(claimants)
	return claimants
}

// TestLifecycleOwnershipExactlyOneWorkflowClaimant is spec 115 AC10's
// discriminating proof. It parses EVERY `.mindspec/domains/*/OWNERSHIP.yaml`
// in the repo and asserts the `internal/lifecycle/**` path pattern is
// claimed by exactly one domain, and that domain is `workflow`.
func TestLifecycleOwnershipExactlyOneWorkflowClaimant(t *testing.T) {
	root := ownershipRepoRoot(t)
	manifests, err := filepath.Glob(filepath.Join(root, ".mindspec", "domains", "*", "OWNERSHIP.yaml"))
	if err != nil {
		t.Fatalf("globbing OWNERSHIP.yaml manifests: %v", err)
	}
	if len(manifests) == 0 {
		t.Fatalf("no .mindspec/domains/*/OWNERSHIP.yaml manifests found under %s — the exactly-one-claimant invariant cannot be vacuously true", root)
	}

	domainPaths := make(map[string][]string, len(manifests))
	for _, manifest := range manifests {
		domain := filepath.Base(filepath.Dir(manifest))
		data, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatalf("reading %s: %v", manifest, err)
		}
		var doc struct {
			Paths []string `yaml:"paths"`
		}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			t.Fatalf("parsing %s: %v", manifest, err)
		}
		domainPaths[domain] = doc.Paths
	}

	claimants := lifecycleClaimants(domainPaths)
	if len(claimants) != 1 {
		t.Fatalf("internal/lifecycle/** must be claimed by EXACTLY ONE domain, got %d claimant(s) %v across %d manifests", len(claimants), claimants, len(manifests))
	}
	if claimants[0] != "workflow" {
		t.Fatalf("internal/lifecycle/** must be claimed by the workflow domain, got %q", claimants[0])
	}
}

// TestClaimsLifecyclePackageDiscrimination is R8's round-1 finding,
// fixed: claimsLifecyclePackage must use OVERLAP matching (production's
// validate.GlobMatch against a real probe file), not a string-prefix
// heuristic — a prefix check would miss a broader glob like
// `internal/**` that overlaps internal/lifecycle in production without
// sharing its literal prefix. Each subtest below injects a SYNTHETIC
// second-domain (or third-domain) claim on top of the real `workflow`
// claim (baseline: exactly one claimant, "workflow") and asserts the
// exactly-one-claimant invariant is VIOLATED (RED) — proving the
// matcher actually discriminates rather than rubber-stamping any input.
func TestClaimsLifecyclePackageDiscrimination(t *testing.T) {
	baseline := map[string][]string{
		"workflow": {"internal/lifecycle/**"},
	}
	if got := lifecycleClaimants(baseline); len(got) != 1 || got[0] != "workflow" {
		t.Fatalf("baseline sanity check failed: got claimants %v, want exactly [workflow]", got)
	}

	t.Run("broader_internal_star_star_glob_from_another_domain_is_caught", func(t *testing.T) {
		manifests := map[string][]string{
			"workflow": {"internal/lifecycle/**"},
			"core":     {"internal/**"}, // broader glob that also matches the probe file
		}
		claimants := lifecycleClaimants(manifests)
		if len(claimants) == 1 {
			t.Fatalf("a broader internal/** claim from another domain must be detected as a 2nd claimant (invariant violated), but got only %v — the matcher is not discriminating overlap correctly", claimants)
		}
		if len(claimants) != 2 {
			t.Fatalf("expected exactly 2 claimants (workflow + core), got %d: %v", len(claimants), claimants)
		}
	})

	t.Run("exact_duplicate_glob_in_second_domain_is_caught", func(t *testing.T) {
		manifests := map[string][]string{
			"workflow": {"internal/lifecycle/**"},
			"core":     {"internal/lifecycle/**"}, // exact duplicate claim
		}
		claimants := lifecycleClaimants(manifests)
		if len(claimants) == 1 {
			t.Fatalf("an exact-duplicate internal/lifecycle/** claim in a 2nd domain must be detected (invariant violated), but got only %v", claimants)
		}
		if len(claimants) != 2 {
			t.Fatalf("expected exactly 2 claimants (workflow + core), got %d: %v", len(claimants), claimants)
		}
	})

	t.Run("workflow_claim_removed_is_caught", func(t *testing.T) {
		manifests := map[string][]string{
			"core": {"internal/core/**"}, // unrelated glob, no overlap
		}
		claimants := lifecycleClaimants(manifests)
		if len(claimants) == 1 {
			t.Fatalf("removing the workflow claim (with no replacement) must leave ZERO claimants (invariant violated), but got %v", claimants)
		}
		if len(claimants) != 0 {
			t.Fatalf("expected exactly 0 claimants, got %d: %v", len(claimants), claimants)
		}
	})

	t.Run("narrower_sibling_glob_does_not_claim", func(t *testing.T) {
		// Regression guard for the ORIGINAL (round-1) matcher's own
		// correct case: a sibling package name that merely shares the
		// internal/lifecycle string as a prefix of a DIFFERENT
		// directory (e.g. internal/lifecycleX) must NOT claim.
		manifests := map[string][]string{
			"workflow": {"internal/lifecycle/**"},
			"core":     {"internal/lifecycleX/**"},
		}
		claimants := lifecycleClaimants(manifests)
		if len(claimants) != 1 || claimants[0] != "workflow" {
			t.Fatalf("a sibling internal/lifecycleX/** glob must NOT claim internal/lifecycle, got claimants %v", claimants)
		}
	})
}
