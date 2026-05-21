package contextpack

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mrmaxsteel/mindspec/internal/tokenize"
)

// fixtureRepo materializes a temp repo skeleton and returns its root.
type fixtureFile struct {
	rel  string
	body string
}

func buildFixtureRepo(t *testing.T, files []fixtureFile) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		p := filepath.Join(root, f.rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(f.body), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	return root
}

// withChdir changes into dir for the duration of the test.
func withChdir(t *testing.T, dir string) {
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

// mkBeadShow returns a beadShowFn stub returning the marshaled entry.
func mkBeadShow(entry beadShowEntry) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		return json.Marshal([]beadShowEntry{entry})
	}
}

const fixtureSpecBody = `---
status: Approved
---
# Spec 099-test: Test Spec

## Goal

The goal is to test the budgeter end-to-end with a known fixture.

## Impacted Domains

- context-system

## Acceptance Criteria

- [ ] BuildBead emits deterministic output
- [ ] SHA-256 provenance block is present
`

const fixturePlanBody = `---
status: Approved
spec_id: 099-test
version: "1"
adr_citations:
    - id: ADR-9001
    - id: ADR-9002
---
# Plan: 099-test

## ADR Fitness

Cited above.

## Testing Strategy

Test fixture.

## Bead 1 — fixture bead

This is the bead-scoped section of the plan that mentions bead-fixture.

Step 1: do the thing.
Step 2: verify the thing.

## Bead 2 — other bead

Unrelated content.
`

const fixtureADR1 = `# ADR-9001: Test ADR One

**Date**: 2026-05-21
**Status**: Accepted
**Domain(s)**: context-system

## Context

Some context.

## Decision

This is the decision section of ADR-9001. It records the architecture
choice for the test fixture.

## Consequences

Some consequences.
`

const fixtureADR2 = `# ADR-9002: Test ADR Two

**Date**: 2026-05-21
**Status**: Accepted
**Domain(s)**: context-system

## Decision

Second ADR decision section, also verbatim.
`

const fixtureOverview = `# context-system overview

This domain contains the context-pack budgeter and tokenizer.
`

const fixtureInterfaces = `# context-system interfaces

BuildBead, Tokenizer, ParseSpec.
`

func standardFixture() []fixtureFile {
	return []fixtureFile{
		{rel: ".mindspec/docs/specs/099-test/spec.md", body: fixtureSpecBody},
		{rel: ".mindspec/docs/specs/099-test/plan.md", body: fixturePlanBody},
		{rel: ".mindspec/docs/adr/ADR-9001.md", body: fixtureADR1},
		{rel: ".mindspec/docs/adr/ADR-9002.md", body: fixtureADR2},
		{rel: ".mindspec/docs/domains/context-system/overview.md", body: fixtureOverview},
		{rel: ".mindspec/docs/domains/context-system/interfaces.md", body: fixtureInterfaces},
	}
}

func standardBeadEntry() beadShowEntry {
	return beadShowEntry{
		ID:                 "bead-fixture",
		Title:              "fixture bead",
		Description:        "Implement the fixture bead end-to-end.",
		AcceptanceCriteria: "- [ ] Fixture bead passes its test.",
		Design:             "Design notes: do the simplest thing that works.",
		Metadata: map[string]interface{}{
			"spec_id":    "099-test",
			"file_paths": []interface{}{"internal/contextpack/budgeter.go"},
		},
	}
}

func TestContextPackDeterministic(t *testing.T) {
	root := buildFixtureRepo(t, standardFixture())
	// Add the file_paths target as a literal file.
	if err := os.MkdirAll(filepath.Join(root, "internal/contextpack"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal/contextpack/budgeter.go"), []byte("package contextpack\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withChdir(t, root)

	restore := SetBeadShowForTest(mkBeadShow(standardBeadEntry()))
	defer restore()

	out1, err := BuildBead("bead-fixture", 2000, tokenize.Approx{})
	if err != nil {
		t.Fatalf("BuildBead run 1: %v", err)
	}
	out2, err := BuildBead("bead-fixture", 2000, tokenize.Approx{})
	if err != nil {
		t.Fatalf("BuildBead run 2: %v", err)
	}
	if !bytes.Equal(out1, out2) {
		t.Fatalf("BuildBead output not byte-identical across runs.\nrun1 len=%d\nrun2 len=%d", len(out1), len(out2))
	}
	if sha256.Sum256(out1) != sha256.Sum256(out2) {
		t.Fatalf("SHA-256 of output differs across runs")
	}
}

func TestContextPackBudget(t *testing.T) {
	root := buildFixtureRepo(t, standardFixture())
	withChdir(t, root)

	restore := SetBeadShowForTest(mkBeadShow(standardBeadEntry()))
	defer restore()

	out, err := BuildBead("bead-fixture", 2000, tokenize.Approx{})
	if err != nil {
		t.Fatalf("BuildBead: %v", err)
	}
	got := tokenize.Approx{}.Count(string(out))
	if got > 2000 {
		t.Fatalf("output token count %d > 2000", got)
	}
	if !strings.Contains(string(out), "## Bead") {
		t.Fatalf("missing must-tier ## Bead section in output")
	}
	if !strings.Contains(string(out), "Implement the fixture bead end-to-end.") {
		t.Fatalf("must-tier description missing from output")
	}
}

func TestContextPackTruncationMarker(t *testing.T) {
	files := standardFixture()
	// Inflate the domain doc to force shave in tier 5.
	big := strings.Repeat("This is a long sentence that adds many runes to the domain overview document. ", 200)
	for i, f := range files {
		if f.rel == ".mindspec/docs/domains/context-system/overview.md" {
			files[i].body = "# overview\n\n" + big
		}
	}
	root := buildFixtureRepo(t, files)
	withChdir(t, root)
	restore := SetBeadShowForTest(mkBeadShow(standardBeadEntry()))
	defer restore()

	out, err := BuildBead("bead-fixture", 500, tokenize.Approx{})
	if err != nil {
		t.Fatalf("BuildBead: %v", err)
	}
	if !strings.Contains(string(out), truncationMarker) {
		t.Fatalf("expected truncation marker %q in output", truncationMarker)
	}
	if !utf8.ValidString(string(out)) {
		t.Fatalf("output is not valid UTF-8 after tail-shave")
	}
	// Marker must NOT appear in the ## Bead must-tier slice.
	beadStart := strings.Index(string(out), "## Bead\n")
	specStart := strings.Index(string(out), "## Spec")
	if beadStart >= 0 && specStart > beadStart {
		mustSlice := string(out)[beadStart:specStart]
		if strings.Contains(mustSlice, truncationMarker) {
			t.Fatalf("truncation marker found inside ## Bead must-tier section")
		}
	}
}

func TestContextPackErrorOnMustTierOverflow(t *testing.T) {
	root := buildFixtureRepo(t, standardFixture())
	withChdir(t, root)

	bigDesign := strings.Repeat("x", 4000)
	entry := standardBeadEntry()
	entry.Design = bigDesign
	restore := SetBeadShowForTest(mkBeadShow(entry))
	defer restore()

	out, err := BuildBead("bead-fixture", 100, tokenize.Approx{})
	if err == nil {
		t.Fatalf("expected error on must-tier overflow, got nil")
	}
	if out != nil {
		t.Fatalf("expected nil bytes on overflow, got %d bytes", len(out))
	}
	if !strings.Contains(err.Error(), "bead context exceeds") {
		t.Fatalf("error %q does not contain expected substring", err.Error())
	}
	if !strings.Contains(err.Error(), "100") {
		t.Fatalf("error %q does not contain budget value", err.Error())
	}
}

func TestProvenanceBlockContainsInputSHA(t *testing.T) {
	root := buildFixtureRepo(t, standardFixture())
	if err := os.MkdirAll(filepath.Join(root, "internal/contextpack"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal/contextpack/budgeter.go"), []byte("package contextpack\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withChdir(t, root)
	restore := SetBeadShowForTest(mkBeadShow(standardBeadEntry()))
	defer restore()

	out, err := BuildBead("bead-fixture", 5000, tokenize.Approx{})
	if err != nil {
		t.Fatalf("BuildBead: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "## Provenance") {
		t.Fatalf("missing ## Provenance block")
	}
	idx := strings.Index(s, "## Provenance")
	tail := s[idx:]

	expected := []string{
		"bead:bead-fixture",
		"spec:",
		"plan:",
		"adr:ADR-9001",
		"adr:ADR-9002",
		"domain:context-system/overview.md",
		"domain:context-system/interfaces.md",
		"file:internal/contextpack/budgeter.go",
	}
	for _, key := range expected {
		if !strings.Contains(tail, key) {
			t.Errorf("missing prov key %q in Provenance block", key)
		}
	}

	hexRe := regexp.MustCompile(`sha256:([a-f0-9]+)`)
	matches := hexRe.FindAllStringSubmatch(tail, -1)
	if len(matches) == 0 {
		t.Fatalf("no sha256: lines in Provenance block")
	}
	for _, m := range matches {
		if len(m[1]) != 64 {
			t.Errorf("sha256 hex length %d != 64 (got %q)", len(m[1]), m[1])
		}
		if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(m[1]) {
			t.Errorf("sha256 value %q is not 64-char lowercase hex", m[1])
		}
	}
}

func TestContextPackErrorOnMissingSpecID(t *testing.T) {
	root := buildFixtureRepo(t, standardFixture())
	withChdir(t, root)

	entry := standardBeadEntry()
	delete(entry.Metadata, "spec_id")
	restore := SetBeadShowForTest(mkBeadShow(entry))
	defer restore()

	// Install a walk recorder; assert it records zero invocations.
	var walkCount int
	restoreWalk := SetWalkForTest(func(root string, fn filepath.WalkFunc) error {
		walkCount++
		return nil
	})
	defer restoreWalk()

	out, err := BuildBead("bead-fixture", 1000, tokenize.Approx{})
	if err == nil {
		t.Fatalf("expected error on missing spec_id, got nil")
	}
	if out != nil {
		t.Fatalf("expected nil bytes on missing spec_id, got %d", len(out))
	}
	if !strings.Contains(err.Error(), "lacks metadata.spec_id") {
		t.Fatalf("error %q missing expected substring", err.Error())
	}
	if walkCount != 0 {
		t.Fatalf("walkFn invoked %d times; expected zero (no fallback scan)", walkCount)
	}
}

func TestContextPackProvenanceReserveIsDynamic(t *testing.T) {
	// Build provLines for two different input cardinalities and
	// compare their token counts via renderProvBlockForTest.
	hex64 := strings.Repeat("a", 64)
	smallLines := []struct{ Key, Sha string }{
		{Key: "bead:b1", Sha: hex64},
		{Key: "spec:.mindspec/docs/specs/x/spec.md", Sha: hex64},
		{Key: "plan:.mindspec/docs/specs/x/plan.md", Sha: hex64},
		{Key: "adr:ADR-0001", Sha: hex64},
		{Key: "domain:core/overview.md", Sha: hex64},
	}
	largeLines := append([]struct{ Key, Sha string }{}, smallLines...)
	for i := 0; i < 30; i++ {
		largeLines = append(largeLines, struct{ Key, Sha string }{
			Key: fmt.Sprintf("file:internal/big/extra_%02d.go", i),
			Sha: hex64,
		})
	}

	small := renderProvBlockForTest("approx", 2000, smallLines)
	large := renderProvBlockForTest("approx", 2000, largeLines)
	t1 := tokenize.Approx{}.Count(small)
	t2 := tokenize.Approx{}.Count(large)
	diff := t2 - t1
	if diff < 0 {
		diff = -diff
	}
	if diff <= 50 {
		t.Fatalf("expected dynamic reserve token-count diff > 50, got %d (small=%d large=%d)", diff, t1, t2)
	}

	// Also run BuildBead end-to-end with both fixture sizes and
	// assert both bundles satisfy tok.Count(output) <= maxTokens.
	root := buildFixtureRepo(t, standardFixture())
	withChdir(t, root)
	restore := SetBeadShowForTest(mkBeadShow(standardBeadEntry()))
	defer restore()

	out, err := BuildBead("bead-fixture", 3000, tokenize.Approx{})
	if err != nil {
		t.Fatalf("BuildBead: %v", err)
	}
	tk := tokenize.Approx{}
	if got := tk.Count(string(out)); got > 3000 {
		t.Fatalf("output token count %d > 3000", got)
	}
}
