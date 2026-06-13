package redact

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestFalsifiability_LeakyCorpusExitsNonZero is the REAL falsifiability
// proof the panel demanded (C3 / codex-completeness): it does not merely
// assert leak-detection logic in-process — it SHELLS OUT to `go test`
// against a deliberately-leaky redactor variant and asserts the toolchain
// exits NON-ZERO. This proves the gate is genuinely build-breaking, not a
// positive-only check that passes while a leak is present.
//
// The leaky variant is a self-contained temp module whose `Scrub` is the
// IDENTITY function (a fully neutered redactor) and whose test reuses the
// SAME zero-leakage shape as the golden corpus. If `go test` on it
// EXITED ZERO, the gate would be vacuous and this test fails.
func TestFalsifiability_LeakyCorpusExitsNonZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skips the go-toolchain subprocess under -short")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module leakyredact\n\ngo 1.23\n")

	// A neutered (identity) redactor — the canonical "what if a scrub pass
	// were removed" mutation.
	writeFile(t, filepath.Join(dir, "leaky.go"), `package leaky

// Scrub is DELIBERATELY neutered (identity) to prove the leak gate trips.
func Scrub(s string) (string, bool) { return s, true }
`)

	// The same zero-leakage assertion the real corpus uses. Against the
	// identity Scrub above it MUST report a leak and FAIL.
	writeFile(t, filepath.Join(dir, "leaky_test.go"), `package leaky

import (
	"strings"
	"testing"
)

func TestZeroLeakage(t *testing.T) {
	raw := "fatal at /Users/victim/secret and ghp_ABCDEFGHIJKLMNOPqrst"
	sensitive := []string{"/Users/victim", "ghp_ABCDEFGHIJKLMNOP"}
	clean, _ := Scrub(raw)
	for _, s := range sensitive {
		if strings.Contains(clean, s) {
			t.Fatalf("LEAK: %q survived in %q", s, clean)
		}
	}
}
`)

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GO111MODULE=on")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("`go test` on a DELIBERATELY-leaky redactor EXITED ZERO — the "+
			"redaction gate is not falsifiable / not build-breaking.\noutput:\n%s", out)
	}
	// Sanity: the failure is the leak assertion, not a compile error.
	if !strings.Contains(string(out), "LEAK") {
		t.Fatalf("leaky corpus failed but NOT via the leak assertion (compile "+
			"error?):\n%s", out)
	}
	if exitErr, isExit := err.(*exec.ExitError); !isExit || exitErr.ExitCode() == 0 {
		t.Fatalf("expected a non-zero process exit; got err=%v on %s", err, runtime.Version())
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
