package main

// context_test.go: cobra-level integration tests for
// `mindspec context bead <id> --max-tokens N` (spec 088 Bead 3).
//
// The tests exercise the real rootCmd dispatch path so flag parsing,
// the --max-tokens >= 0 validation, and the BuildBead vs
// RenderBeadContext branch are all covered as the user experiences
// them. We install a beadShowFn stub via SetBeadShowForTest so the
// tests never invoke a real `bd` subprocess.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/tokenize"
)

// resetContextBeadFlags zeroes the --max-tokens flag state between
// invocations so a prior test's `--max-tokens` value does not leak
// into a later test that omits the flag.
func resetContextBeadFlags(t *testing.T) {
	t.Helper()
	if f := contextBeadCmd.Flags().Lookup("max-tokens"); f != nil {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	}
}

// runContextBead invokes `mindspec context bead <args...>` through
// rootCmd and returns combined stdout+stderr and the returned error.
func runContextBead(t *testing.T, args []string) (string, error) {
	t.Helper()
	resetContextBeadFlags(t)
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	full := append([]string{"context", "bead"}, args...)
	rootCmd.SetArgs(full)
	err := rootCmd.Execute()
	return stdout.String() + stderr.String(), err
}

// withTestChdir chdirs to dir for the duration of the test.
func withTestChdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// writeFixture writes a single file with parent-dir creation.
func writeFixture(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

// stubBeadShowEntry mirrors contextpack.beadShowEntry's JSON shape
// (the type itself is unexported so we marshal a map literal).
func stubBeadShowJSON(t *testing.T, id, title, spec string) []byte {
	t.Helper()
	entry := map[string]interface{}{
		"id":                  id,
		"title":               title,
		"description":         "Tiny description.",
		"acceptance_criteria": "- [ ] minimal AC",
		"design":              "Minimal design.",
		"metadata": map[string]interface{}{
			"spec_id": spec,
		},
	}
	b, err := json.Marshal([]interface{}{entry})
	if err != nil {
		t.Fatalf("marshal stub bead: %v", err)
	}
	return b
}

// TestContextBeadMaxTokensFlag exercises the --max-tokens flag on
// `mindspec context bead <id>` against a tiny fixture. Verifies:
//   - the new spec 088 BuildBead path is taken
//   - output is non-empty
//   - tokenize.Approx.Count(output) <= --max-tokens budget
func TestContextBeadMaxTokensFlag(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, ".mindspec/docs/specs/099-cli-test/spec.md", "---\nstatus: Approved\n---\n# Spec\n\n## Goal\n\nTiny goal.\n\n## Impacted Domains\n\n- context-system\n\n## Acceptance Criteria\n\n- [ ] tiny AC\n")
	writeFixture(t, root, ".mindspec/docs/specs/099-cli-test/plan.md", "---\nstatus: Approved\nspec_id: 099-cli-test\n---\n# Plan\n\n## Bead 1 — tiny\n\nTiny plan section.\n")
	writeFixture(t, root, ".mindspec/docs/domains/context-system/overview.md", "# overview\n\nshort.\n")
	writeFixture(t, root, ".mindspec/docs/domains/context-system/interfaces.md", "# interfaces\n\nshort.\n")
	withTestChdir(t, root)

	restore := contextpack.SetBeadShowForTest(func(args ...string) ([]byte, error) {
		return stubBeadShowJSON(t, "cli-bead", "tiny bead", "099-cli-test"), nil
	})
	defer restore()

	// 500 approx tokens is comfortably above the tiny must-tier
	// (header + ## Bead + provenance reserve for one spec/plan +
	// one bead + two domain docs) but still tight enough that the
	// tail-shave on tiers 2-6 actually fires — exercising the
	// budget machinery rather than just measuring an unbudgeted
	// render that happens to fit.
	const budget = 500
	out, err := runContextBead(t, []string{"cli-bead", "--max-tokens", "500"})
	if err != nil {
		t.Fatalf("runContextBead returned error: %v; output=%q", err, out)
	}
	if out == "" {
		t.Fatalf("expected non-empty output, got empty")
	}
	// The output must include the new layout markers (## Bead and
	// ## Provenance), proving the BuildBead branch was taken (the
	// legacy path emits ## Acceptance Criteria / ## Work Chunk
	// instead).
	if !strings.Contains(out, "## Bead\n") {
		t.Fatalf("output missing `## Bead` section (new layout marker):\n%s", out)
	}
	if !strings.Contains(out, "## Provenance\n") {
		t.Fatalf("output missing `## Provenance` block (new layout marker):\n%s", out)
	}
	count := tokenize.Approx{}.Count(out)
	if count > budget {
		t.Fatalf("tokenize.Approx.Count(output) = %d, want <= %d", count, budget)
	}
}

// TestContextPackRejectsNegativeBudget asserts that `mindspec
// context bead <id> --max-tokens -1` is rejected at flag-validation
// time with the documented error string, BEFORE any bd lookup runs.
// The beadShowFn stub returns an error to defensively prove the flag
// check fires first.
func TestContextPackRejectsNegativeBudget(t *testing.T) {
	restore := contextpack.SetBeadShowForTest(func(args ...string) ([]byte, error) {
		t.Fatalf("beadShowFn must NOT be called when --max-tokens is negative; called with %v", args)
		return nil, nil
	})
	defer restore()

	out, err := runContextBead(t, []string{"test-bead", "--max-tokens", "-1"})
	if err == nil {
		t.Fatalf("expected error for --max-tokens -1, got nil; output=%q", out)
	}
	combined := err.Error() + "\n" + out
	if !strings.Contains(combined, "--max-tokens must be >= 0") {
		t.Fatalf("expected error containing %q, got: %v / output=%q",
			"--max-tokens must be >= 0", err, out)
	}
}
