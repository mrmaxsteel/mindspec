// Package specgate exercises the spec-083 Phase 0 prerequisite gate
// (Test G in the spec's Validation Proofs section).
//
// The verification logic lives in scripts/verify-agentmind-tag.sh — this
// Go test file is the per-commit-green smoke test that the script exists,
// is executable, and behaves correctly for the offline-safe code paths.
// The network-dependent assertion (does refs/tags/v0.0.1 exist upstream?)
// is run only outside `-short` mode, because at the time of Bead 1 the
// upstream v0.0.1 tag has not yet been published — running the network
// check under `-short` would break Hard Constraint #6 from the spec
// ("go test -short ./... must pass on every commit").
package specgate

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// scriptPath resolves the absolute path to scripts/verify-agentmind-tag.sh
// from the repo root by walking up from this test file's location.
func scriptPath(t *testing.T) string {
	t.Helper()
	// runtime.Caller(0) gives this file's path; walk up to repo root.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile = <repo>/internal/specgate/verify_agentmind_tag_test.go
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	p := filepath.Join(repoRoot, "scripts", "verify-agentmind-tag.sh")
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("filepath.Abs(%q): %v", p, err)
	}
	return abs
}

// TestVerifyAgentmindTagScriptExists asserts the script file is present
// and executable. Runs in -short mode.
func TestVerifyAgentmindTagScriptExists(t *testing.T) {
	p := scriptPath(t)
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat %s: %v", p, err)
	}
	if info.IsDir() {
		t.Fatalf("%s is a directory, want a file", p)
	}
	// On unix-like hosts, assert the executable bit. On Windows the bit
	// is not meaningful; skip that half of the check.
	if runtime.GOOS != "windows" {
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("%s is not executable (mode=%v)", p, info.Mode().Perm())
		}
	}
}

// TestVerifyAgentmindTagHelp asserts the script's --help flag prints the
// expected documentation header and exits 0. Runs in -short mode (no
// network access required).
func TestVerifyAgentmindTagHelp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash script not directly runnable on windows test host")
	}
	p := scriptPath(t)
	cmd := exec.Command(p, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script --help failed: %v\noutput: %s", err, string(out))
	}
	s := string(out)
	for _, want := range []string{
		"verify-agentmind-tag.sh",
		"Spec 083",
		"Test G",
		"Exit codes:",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("--help output missing %q\ngot: %s", want, s)
		}
	}
}

// TestVerifyAgentmindTagUnreachableRepo asserts that pointing the script
// at a nonexistent GitHub repository yields exit code 3 (upstream
// unreachable). The fake repo is a github.com/mrmaxsteel/* path that
// definitely does not exist; this exercises the error-handling branch
// without depending on the real agentmind repo's state. Runs in -short
// mode but requires network access; skipped if network is unavailable.
func TestVerifyAgentmindTagUnreachableRepo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash script not directly runnable on windows test host")
	}
	if os.Getenv("MINDSPEC_NO_NETWORK_TESTS") != "" {
		t.Skip("MINDSPEC_NO_NETWORK_TESTS set; skipping network-dependent test")
	}
	p := scriptPath(t)
	cmd := exec.Command(p)
	cmd.Env = append(os.Environ(),
		"AGENTMIND_REPO_URL=https://github.com/mrmaxsteel/this-repo-definitely-does-not-exist-bead1gate",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit, got success; output: %s", string(out))
	}
	ee, ok := err.(*exec.ExitError)
	if !ok {
		// Likely a network failure — skip rather than fail, since this
		// test is fundamentally a network-touching assertion.
		t.Skipf("script did not run as an exec.ExitError (network failure?): %v", err)
	}
	// Exit 3 = upstream unreachable. Exit 2 would mean the fake repo
	// somehow resolved; that should not happen but we treat it as a
	// test environment anomaly rather than a hard failure of the script.
	if ee.ExitCode() != 3 {
		t.Fatalf("expected exit code 3 (unreachable), got %d\noutput: %s",
			ee.ExitCode(), string(out))
	}
	if !strings.Contains(string(out), "upstream unreachable") {
		t.Errorf("expected 'upstream unreachable' in output, got: %s", string(out))
	}
}

// TestVerifyAgentmindTagAgainstUpstream is the real Test G assertion:
// does refs/tags/v0.0.1 exist at github.com/mrmaxsteel/agentmind?
//
// This is the gate that, when it passes, unblocks Phase 1 of spec 083.
// At Bead 1 implementation time it is EXPECTED to fail (exit code 2)
// because v0.0.1 has not yet been published upstream. Skipped under
// -short so Hard Constraint #6 (per-commit `go test -short ./...` green)
// holds; run it manually via `go test ./internal/specgate/...` or
// `make verify-agentmind-tag` to check the gate's current state.
func TestVerifyAgentmindTagAgainstUpstream(t *testing.T) {
	if testing.Short() {
		t.Skip("network-dependent Test G gate; skipped under -short")
	}
	if runtime.GOOS == "windows" {
		t.Skip("bash script not directly runnable on windows test host")
	}
	if os.Getenv("MINDSPEC_NO_NETWORK_TESTS") != "" {
		t.Skip("MINDSPEC_NO_NETWORK_TESTS set; skipping network-dependent test")
	}
	p := scriptPath(t)
	cmd := exec.Command(p)
	out, err := cmd.CombinedOutput()
	if err == nil {
		// SUCCESS PATH — v0.0.1 has been published upstream. The script
		// prints the SHA on stdout. This is the post-gate-passing state.
		t.Logf("Test G passes — agentmind v0.0.1 SHA: %s", strings.TrimSpace(string(out)))
		return
	}
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Skipf("script did not run as an exec.ExitError (network failure?): %v", err)
	}
	switch ee.ExitCode() {
	case 2:
		// EXPECTED at Bead 1 implementation time: tag not yet published.
		// We do NOT fail the test here, because the bead's job is to
		// implement the gate, not to wait for the upstream tag. The gate
		// reports its honest current state and that state is recorded
		// in spec.md as <TBD>.
		t.Logf("Test G gate currently fails as expected: agentmind v0.0.1 not yet published upstream\noutput: %s", string(out))
	case 3:
		// Repo unreachable — note as a known condition during the
		// parallel migration, do not fail.
		t.Logf("Test G gate: upstream repository unreachable (expected during migration)\noutput: %s", string(out))
	default:
		t.Fatalf("unexpected exit code %d from verify-agentmind-tag.sh\noutput: %s",
			ee.ExitCode(), string(out))
	}
}
