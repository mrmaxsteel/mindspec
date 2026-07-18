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

	"gopkg.in/yaml.v3"
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

// ciWorkflowStep is the subset of a GitHub Actions step's fields relevant to
// the AC-11 pin. Parsed via yaml.v3 rather than naive line scanning — GitHub
// Actions puts `continue-on-error: true` on ITS OWN line within a step's
// block, not necessarily on the same line as `run:`, so a per-line substring
// scan for "mindspec doctor" + "continue-on-error" can miss a bypass that
// lives a few lines below the run command inside the SAME step.
type ciWorkflowStep struct {
	Name            string `yaml:"name"`
	Run             string `yaml:"run"`
	ContinueOnError bool   `yaml:"continue-on-error"`
}

type ciWorkflowJob struct {
	Steps []ciWorkflowStep `yaml:"steps"`
}

type ciWorkflowFile struct {
	Jobs map[string]ciWorkflowJob `yaml:"jobs"`
}

// findDoctorStep parses the workflow YAML and returns the step (from any
// job) whose `run:` invokes `mindspec doctor`, and whether one was found.
func findDoctorStep(t *testing.T, content string) (ciWorkflowStep, bool) {
	t.Helper()
	var wf ciWorkflowFile
	if err := yaml.Unmarshal([]byte(content), &wf); err != nil {
		t.Fatalf("parsing ci.yml as YAML: %v", err)
	}
	for _, job := range wf.Jobs {
		for _, step := range job.Steps {
			if strings.Contains(step.Run, "mindspec doctor") {
				return step, true
			}
		}
	}
	return ciWorkflowStep{}, false
}

// TestCIWorkflow_RunsMindspecDoctor is the AC-11 pin: the workflow file
// must invoke `mindspec doctor` as a permitted step (the compiled binary,
// not `go run`), so a doctor Error/Missing finding fails the CI build via
// its non-zero exit — mirroring `rg -n 'mindspec doctor' .github/workflows/ci.yml`.
//
// The bypass check parses the step's OWN YAML block (via yaml.v3) rather
// than scanning for `continue-on-error` on the same text line as the `run:`
// command — GitHub Actions allows (and commonly formats) `continue-on-
// error: true` on a separate line of the step, which a per-line substring
// scan would silently miss.
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

	step, found := findDoctorStep(t, content)
	if !found {
		t.Fatalf("expected a workflow step whose `run:` invokes `mindspec doctor`; got:\n%s", content)
	}
	if step.ContinueOnError {
		t.Fatalf("mindspec doctor step must not set continue-on-error: true (defeats AC-11's non-zero-exit-fails-build contract); step: %+v", step)
	}
	if strings.Contains(step.Run, "|| true") {
		t.Fatalf("mindspec doctor step must not swallow its exit code via `|| true`; run: %q", step.Run)
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
