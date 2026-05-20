package specgate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCheckoutAgentmindSiblingPresent exercises
// scripts/checkout-agentmind.sh against the local ../agentmind sibling
// repo (spec 083 Bead 2 step 4 + "sibling-checkout helper test"
// deliverable). The helper is idempotent: with a sibling already in
// place, it must exit 0 and print the sibling's absolute path on
// stdout.
//
// This test is the mindspec-side guardrail that the local-development
// path remains green even when upstream `github.com/mrmaxsteel/agentmind`
// is unreachable (Phase 0/1 deferral mode).
func TestCheckoutAgentmindSiblingPresent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script test; not run on windows")
	}
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts", "checkout-agentmind.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("script not found at %s: %v", scriptPath, err)
	}

	siblingPath := filepath.Join(repoRoot, "..", "agentmind")
	absSibling, err := filepath.Abs(siblingPath)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(absSibling, "go.mod")); err != nil {
		// Panel-mandated CI semantics: when MINDSPEC_REQUIRE_SIBLING=1
		// is set in the environment (the mode CI runs under), a
		// missing sibling is a hard failure rather than a silent skip.
		// Default behavior remains skip for local-dev ergonomics so
		// devs can `go test ./...` from a fresh clone without first
		// running `make checkout-agentmind`.
		msg := fmt.Sprintf("sibling agentmind module not present at %s "+
			"(run `make checkout-agentmind` to materialize it)", absSibling)
		if os.Getenv("MINDSPEC_REQUIRE_SIBLING") == "1" {
			t.Fatalf("MINDSPEC_REQUIRE_SIBLING=1 but %s", msg)
		}
		t.Skipf("%s — skipping local-sibling test (this is expected "+
			"during the parallel mindspec-side migration when run "+
			"outside the Bead 2 worktree)", msg)
	}

	// Force an unreachable upstream URL so the test never depends on
	// network state; the helper MUST detect the local sibling without
	// any upstream interaction.
	cmd := exec.Command(scriptPath)
	cmd.Env = append(os.Environ(),
		"AGENTMIND_REPO_URL=https://invalid.example.test/agentmind.git",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("checkout-agentmind.sh failed: %v\noutput:\n%s", err, string(out))
	}

	// Stdout's first line should be the resolved sibling path.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		t.Fatalf("expected at least one line of output, got none")
	}
	gotPath := strings.TrimSpace(lines[len(lines)-1])
	// Use evalsymlinks so /private/var vs /var (macOS) doesn't fail
	// the equality check.
	wantPath, _ := filepath.EvalSymlinks(absSibling)
	resolvedGot, _ := filepath.EvalSymlinks(gotPath)
	if wantPath == "" {
		wantPath = absSibling
	}
	if resolvedGot == "" {
		resolvedGot = gotPath
	}
	if resolvedGot != wantPath {
		t.Errorf("sibling path mismatch:\n got=%s\nwant=%s\nfull output:\n%s",
			resolvedGot, wantPath, string(out))
	}
}

// TestCheckoutAgentmindHelp confirms the helper's --help flag works
// (UX guardrail; mirrors verify-agentmind-tag.sh which has the same).
func TestCheckoutAgentmindHelp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script test; not run on windows")
	}
	t.Parallel()
	repoRoot := mustRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts", "checkout-agentmind.sh")

	cmd := exec.Command(scriptPath, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--help exit non-zero: %v\noutput:\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "checkout-agentmind.sh") {
		t.Errorf("--help output missing script name; got:\n%s", string(out))
	}
	if !strings.Contains(string(out), "Exit codes:") {
		t.Errorf("--help output missing exit-codes block; got:\n%s", string(out))
	}
}

// mustRepoRoot returns the absolute path to the mindspec repo root.
func mustRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// internal/specgate -> ../..
	return filepath.Join(wd, "..", "..")
}
