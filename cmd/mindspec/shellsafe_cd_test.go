package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestExecutableCdRendersShellSafe is AC-12 (this package's slice):
// emitCdBackNote — a ROOT-ONLY sink (cwdsafety.go) — routes its `cd`
// render through the single shell-safe emitter: a space-bearing root is
// POSIX single-quoted and round-trips a real shell; a clean root renders
// byte-identical to today (pinned already by cwdsafety_test.go's
// existing assertions); the sink never refuses regardless of root
// content — emitCdBackNote has no error return at all.
func TestExecutableCdRendersShellSafe(t *testing.T) {
	t.Run("space-bearing root is quoted and round-trips a real shell", func(t *testing.T) {
		if _, err := exec.LookPath("sh"); err != nil {
			t.Skip("sh not available for the round-trip assertion")
		}
		root := t.TempDir() + " with spaces"
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", root, err)
		}
		invocationCwd := filepath.Join(root, "gone")
		if err := os.MkdirAll(invocationCwd, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", invocationCwd, err)
		}
		if err := os.RemoveAll(invocationCwd); err != nil {
			t.Fatalf("remove %q: %v", invocationCwd, err)
		}

		var stdout bytes.Buffer
		emitCdBackNote(&stdout, invocationCwd, root)

		want := "run: cd '" + root + "'"
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("emitCdBackNote not shell-safe quoted; got %q, want substring %q", stdout.String(), want)
		}

		cdLine := "cd '" + strings.ReplaceAll(root, "'", `'\''`) + "'"
		out, err := exec.Command("sh", "-c", cdLine+" && pwd").CombinedOutput()
		if err != nil {
			t.Fatalf("sh -c %q failed: %v\noutput: %s", cdLine, err, out)
		}
		if got := strings.TrimSpace(string(out)); got != root {
			t.Errorf("shell round-trip: sh landed in %q, want %q", got, root)
		}
	})

	t.Run("never refuses regardless of root content (no error return)", func(t *testing.T) {
		var stdout bytes.Buffer
		// emitCdBackNote has no error return — this test documents that
		// invariant structurally: it always writes the NOTE line (or
		// nothing, if the invocation cwd still exists) and cannot fail.
		emitCdBackNote(&stdout, filepath.Join(t.TempDir(), "does-not-exist"), "/tmp/root; rm -rf /")
		if !strings.Contains(stdout.String(), "working directory was removed") {
			t.Errorf("expected the NOTE to be emitted unconditionally; got %q", stdout.String())
		}
	})
}
