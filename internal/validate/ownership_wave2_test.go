package validate

import (
	"os"
	"path/filepath"
	"testing"
)

// repoRootForWorkflowManifest walks up from the test's working directory
// until it finds the workflow domain's OWNERSHIP.yaml, returning the repo
// root that contains it. This anchors TestWorkflowOwnsTraceAndGolangci on
// the REAL committed manifest (the one spec 108 R1 edits) rather than a
// synthetic fixture, so removing either claim from the live manifest fails
// the test.
func repoRootForWorkflowManifest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".mindspec", "domains", "workflow", "OWNERSHIP.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate .mindspec/domains/workflow/OWNERSHIP.yaml walking up from %s", dir)
		}
		dir = parent
	}
}

// TestWorkflowOwnsTraceAndGolangci proves the two paths spec 108 R1 claims
// — internal/trace/** and .golangci.yml — attribute to the workflow domain
// through attributeDomain against the live workflow OWNERSHIP.yaml. Without
// the claims these files trip adr-divergence-unowned when edited (spec AC 2).
func TestWorkflowOwnsTraceAndGolangci(t *testing.T) {
	root := repoRootForWorkflowManifest(t)
	domains := []string{"workflow"}

	cases := []string{
		"internal/trace/event.go",
		".golangci.yml",
	}
	for _, path := range cases {
		owner, own, err := attributeDomain(nil, root, "", path, domains)
		if err != nil {
			t.Fatalf("attributeDomain(%q) err: %v", path, err)
		}
		if owner != "workflow" {
			t.Errorf("attributeDomain(%q) = %q, want %q", path, owner, "workflow")
		}
		if own == nil {
			t.Errorf("attributeDomain(%q) returned nil Ownership", path)
		}
	}
}
