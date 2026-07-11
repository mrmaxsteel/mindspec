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
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

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
// pattern claims this package: the canonical `internal/lifecycle/**`
// glob, the bare directory, or any pattern scoped under it. A pattern
// like `internal/lifecycleX/**` does NOT match.
func claimsLifecyclePackage(pattern string) bool {
	return pattern == "internal/lifecycle/**" ||
		pattern == "internal/lifecycle" ||
		strings.HasPrefix(pattern, "internal/lifecycle/")
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

	var claimants []string
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
		for _, pattern := range doc.Paths {
			if claimsLifecyclePackage(pattern) {
				claimants = append(claimants, domain)
				break
			}
		}
	}

	sort.Strings(claimants)
	if len(claimants) != 1 {
		t.Fatalf("internal/lifecycle/** must be claimed by EXACTLY ONE domain, got %d claimant(s) %v across %d manifests", len(claimants), claimants, len(manifests))
	}
	if claimants[0] != "workflow" {
		t.Fatalf("internal/lifecycle/** must be claimed by the workflow domain, got %q", claimants[0])
	}
}
