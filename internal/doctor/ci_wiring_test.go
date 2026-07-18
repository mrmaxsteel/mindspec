package doctor

// Spec 119 Bead 2 (AC-11): pins that CI actually runs `mindspec doctor`
// with a non-zero exit failing the build, and that the workflow file
// itself is claimed into the workflow domain's OWNERSHIP.yaml (the
// `.golangci.yml` precedent from spec 108) so the divergence gate
// attributes the CI edit cleanly.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRootForCIWiring walks up from the test's working directory until it
// finds the repo's CI workflow file, returning the repo root that contains
// it. Anchors this test on the REAL committed .github/workflows/ci.yml
// rather than a synthetic fixture.
func repoRootForCIWiring(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".github", "workflows", "ci.yml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate .github/workflows/ci.yml walking up from %s", dir)
		}
		dir = parent
	}
}

// TestCIWorkflow_RunsMindspecDoctor is the AC-11 pin: the workflow file
// must invoke `mindspec doctor` as a permitted step (the compiled binary,
// not `go run`), so a doctor Error/Missing finding fails the CI build via
// its non-zero exit — mirroring `rg -n 'mindspec doctor' .github/workflows/ci.yml`.
func TestCIWorkflow_RunsMindspecDoctor(t *testing.T) {
	root := repoRootForCIWiring(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "mindspec doctor") {
		t.Fatalf("expected ci.yml to invoke `mindspec doctor`; got:\n%s", content)
	}
	// Must not be neutered with a continue-on-error / `|| true` escape
	// hatch that would defeat AC-11's non-zero-exit-fails-build contract.
	for _, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, "mindspec doctor") {
			continue
		}
		if strings.Contains(line, "|| true") || strings.Contains(line, "continue-on-error") {
			t.Fatalf("mindspec doctor step must not swallow its exit code; got line: %q", line)
		}
	}
}

// TestWorkflowOwnershipClaimsCIWorkflow pins that .github/workflows/ci.yml
// is claimed by the workflow domain's OWNERSHIP.yaml (the .golangci.yml
// precedent from spec 108, TestWorkflowOwnsTraceAndGolangci in
// internal/validate).
func TestWorkflowOwnershipClaimsCIWorkflow(t *testing.T) {
	root := repoRootForCIWiring(t)
	data, err := os.ReadFile(filepath.Join(root, ".mindspec", "domains", "workflow", "OWNERSHIP.yaml"))
	if err != nil {
		t.Fatalf("reading workflow OWNERSHIP.yaml: %v", err)
	}
	if !strings.Contains(string(data), ".github/workflows/ci.yml") {
		t.Fatalf("expected workflow OWNERSHIP.yaml to claim .github/workflows/ci.yml; got:\n%s", string(data))
	}
}
