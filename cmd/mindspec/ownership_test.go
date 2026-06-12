package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/ownership"
)

// writeOwnershipDomain creates .mindspec/docs/domains/<name>/ with an
// optional OWNERSHIP.yaml (omitted when manifest is "").
func writeOwnershipDomain(t *testing.T, root, name, manifest string) {
	t.Helper()
	dir := filepath.Join(root, ".mindspec", "docs", "domains", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(dir, "OWNERSHIP.yaml"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRunOwnershipPopulate_NoArgEnumeratesWithSeparator drives the
// no-arg form against two unpopulated domains and asserts BOTH prompts
// appear AND are joined by the `---` separator. This kills mutant 8a's
// "emit only the first domain's prompt, no separator" regression.
func TestRunOwnershipPopulate_NoArgEnumeratesWithSeparator(t *testing.T) {
	root := t.TempDir()
	writeOwnershipDomain(t, root, "alpha", "") // missing manifest
	writeOwnershipDomain(t, root, "bravo", string(ownership.RenderStub("mindspec domain add bravo")))

	var buf bytes.Buffer
	if err := runOwnershipPopulate(&buf, root, nil); err != nil {
		t.Fatalf("runOwnershipPopulate: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, `.mindspec/docs/domains/alpha/OWNERSHIP.yaml`) {
		t.Errorf("missing alpha prompt:\n%s", out)
	}
	if !strings.Contains(out, `.mindspec/docs/domains/bravo/OWNERSHIP.yaml`) {
		t.Errorf("missing bravo prompt (mutant 8a drops trailing domains):\n%s", out)
	}
	if !strings.Contains(out, "\n---\n") {
		t.Errorf("multi-domain output missing `---` separator (mutant 8a):\n%s", out)
	}
	// Separator sits BETWEEN the two prompts.
	alphaIdx := strings.Index(out, "domains/alpha/")
	sepIdx := strings.Index(out, "\n---\n")
	bravoIdx := strings.Index(out, "domains/bravo/")
	if !(alphaIdx < sepIdx && sepIdx < bravoIdx) {
		t.Errorf("separator not between prompts (alpha=%d sep=%d bravo=%d):\n%s", alphaIdx, sepIdx, bravoIdx, out)
	}
}

// TestRunOwnershipPopulate_AllPopulatedMessage drives the no-arg form
// against a workspace whose only domain has a populated manifest: no
// prompt, the re-emit hint instead.
func TestRunOwnershipPopulate_AllPopulatedMessage(t *testing.T) {
	root := t.TempDir()
	writeOwnershipDomain(t, root, "alpha", "paths:\n  - internal/alpha/**\n")

	var buf bytes.Buffer
	if err := runOwnershipPopulate(&buf, root, nil); err != nil {
		t.Fatalf("runOwnershipPopulate: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "All domain OWNERSHIP.yaml manifests are populated") {
		t.Errorf("missing all-populated message:\n%s", out)
	}
	if strings.Contains(out, "Populate .mindspec/docs/domains/alpha/OWNERSHIP.yaml") {
		t.Errorf("populated domain should NOT get a prompt in no-arg form:\n%s", out)
	}
}

// TestRunOwnershipPopulate_ExplicitArgReEmitsForPopulated proves the
// explicit-arg form emits regardless of populated state (Req 10 /
// Req 16 widen-hint).
func TestRunOwnershipPopulate_ExplicitArgReEmitsForPopulated(t *testing.T) {
	root := t.TempDir()
	writeOwnershipDomain(t, root, "alpha", "paths:\n  - internal/alpha/**\n")

	var buf bytes.Buffer
	if err := runOwnershipPopulate(&buf, root, []string{"alpha"}); err != nil {
		t.Fatalf("runOwnershipPopulate: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Populate .mindspec/docs/domains/alpha/OWNERSHIP.yaml") {
		t.Errorf("explicit arg must re-emit for a populated domain:\n%s", out)
	}
}

// TestRunOwnershipPopulate_RejectsInvalidName proves the validate.DomainName
// guard is load-bearing: malformed and traversal names return a
// non-nil error (exit 1) and emit NO prompt. This kills mutant 8a's
// "drop validate.DomainName(args[0])" regression.
func TestRunOwnershipPopulate_RejectsInvalidName(t *testing.T) {
	root := t.TempDir()

	for _, bad := range []string{"../etc", "Bad-Name", "has space", "..", ""} {
		var buf bytes.Buffer
		err := runOwnershipPopulate(&buf, root, []string{bad})
		if err == nil {
			t.Errorf("expected error for invalid domain %q, got nil (output: %q)", bad, buf.String())
		}
		if buf.Len() != 0 {
			t.Errorf("invalid domain %q must emit no prompt, got: %q", bad, buf.String())
		}
	}
}

// TestRunSourcePopulate_Prints proves `mindspec source populate`
// prints the Req 12 prompt.
func TestRunSourcePopulate_Prints(t *testing.T) {
	var buf bytes.Buffer
	if err := runSourcePopulate(&buf); err != nil {
		t.Fatalf("runSourcePopulate: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"source_globs:",
		".mindspec/config.yaml",
		"missing-source-globs",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("source-populate output missing %q:\n%s", want, out)
		}
	}
}
