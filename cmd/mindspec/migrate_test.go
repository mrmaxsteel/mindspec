package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigrateLayoutSubcommandRegistered asserts the `migrate layout` mover
// subcommand is wired under `migrate` with its precondition flags (spec 106
// Bead 3, AC7/AC16).
func TestMigrateLayoutSubcommandRegistered(t *testing.T) {
	t.Parallel()

	found := false
	for _, c := range migrateCmd.Commands() {
		if c.Name() == "layout" {
			found = true
			for _, flag := range []string{"abort", "run-id", "target"} {
				if c.Flags().Lookup(flag) == nil {
					t.Errorf("migrate layout missing --%s flag", flag)
				}
			}
		}
	}
	if !found {
		t.Fatal("migrate layout subcommand not registered under migrate")
	}
}

// TestLayoutPackageOwned is the AC21 ownership half: the net-new mover package
// internal/layout/** is claimed in the workflow OWNERSHIP.yaml, so a complete
// gate does not trip adr-divergence-unowned. The manifest is resolved relative
// to this package (cmd/mindspec → repo root is ../..).
func TestLayoutPackageOwned(t *testing.T) {
	t.Parallel()

	manifest := filepath.Join("..", "..", ".mindspec", "docs", "domains", "workflow", "OWNERSHIP.yaml")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("read workflow OWNERSHIP.yaml: %v", err)
	}
	if !strings.Contains(string(data), "internal/layout/**") {
		t.Errorf("workflow OWNERSHIP.yaml does not claim internal/layout/**:\n%s", data)
	}
}

func TestScanSourceMarkdown(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Create source files that should be found
	for _, rel := range []string{
		"docs/guide.md",
		"docs/adr/ADR-0001.md",
		"README.md",
		"CONTRIBUTING.md",
	} {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte("# "+rel+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Create files that should be skipped
	for _, rel := range []string{
		".mindspec/docs/core/USAGE.md",
		".mindspec/migrations/run-1/plan.md",
		".git/config.md",
		".beads/issues.md",
	} {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte("skip\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := scanSourceMarkdown(root)
	if err != nil {
		t.Fatalf("scanSourceMarkdown: %v", err)
	}

	expected := map[string]bool{
		"CONTRIBUTING.md":      false,
		"README.md":            false,
		"docs/adr/ADR-0001.md": false,
		"docs/guide.md":        false,
	}
	for _, f := range files {
		if _, ok := expected[f]; ok {
			expected[f] = true
		} else {
			t.Errorf("unexpected file: %s", f)
		}
	}
	for f, found := range expected {
		if !found {
			t.Errorf("missing expected file: %s", f)
		}
	}
}

func TestScanCanonicalDocs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	docsDir := filepath.Join(root, ".mindspec", "docs")
	for _, rel := range []string{
		"core/USAGE.md",
		"glossary.md",
	} {
		abs := filepath.Join(docsDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte("canonical\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := scanCanonicalDocs(root)
	if err != nil {
		t.Fatalf("scanCanonicalDocs: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 canonical files, got %d: %v", len(files), files)
	}
}

func TestScanCanonicalDocs_NoDocs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files, err := scanCanonicalDocs(root)
	if err != nil {
		t.Fatalf("scanCanonicalDocs: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestBuildMigratePrompt_ContainsRequiredSections(t *testing.T) {
	t.Parallel()

	prompt := buildMigratePrompt(
		[]string{"docs/guide.md", "README.md"},
		[]string{".mindspec/docs/core/USAGE.md"},
	)

	required := []string{
		"Phase 1",
		"Phase 2",
		"Phase 3",
		"Phase 4",
		"Phase 5",
		"Phase 6",
		"Phase 7",
		"Source-Globs Population",
		"Ownership-Manifest Population",
		"mindspec source populate",
		"mindspec ownership populate",
		"Canonical Structure",
		"Category Rubric",
		"adr",
		"spec",
		"domain",
		"core",
		"context-map",
		"user-docs",
		"agent",
		"mindspec domain add",
		"Source Files to Classify",
		"docs/guide.md",
		"README.md",
		"Existing Canonical Docs",
		".mindspec/docs/core/USAGE.md",
		"Instructions",
		"mindspec doctor",
	}

	for _, r := range required {
		if !strings.Contains(prompt, r) {
			t.Errorf("prompt missing required content: %q", r)
		}
	}

	// Pin the renumbered Instructions text (panel R3-2). The Bead 1
	// audit renumbers the Instructions section "Phases 1-4"→"1-6" and
	// "Phase 5"→"Phase 7"; without these assertions mutant 4 (leaving
	// the old "Phases 1-4" / "per Phase 5") survives.
	if !strings.Contains(prompt, "Complete Phases 1-6 first") {
		t.Errorf("Instructions missing renumbered \"Phases 1-6\" (mutant 4):\n%s", prompt)
	}
	if !strings.Contains(prompt, "per Phase 7") {
		t.Errorf("Instructions missing renumbered \"per Phase 7\" (mutant 4):\n%s", prompt)
	}
	if strings.Contains(prompt, "Phases 1-4 first") || strings.Contains(prompt, "per Phase 5") {
		t.Errorf("Instructions still contain stale pre-renumber text (mutant 4):\n%s", prompt)
	}
}

// TestBuildMigratePrompt_PopulatePhaseOrdering pins the spec 091
// Req 14 binding ordering constraints — and ONLY those two; the new
// phases' relative order and final phase numbers are the Bead 1
// audit's call, not this test's:
//
//  1. the `mindspec source populate` instruction appears AFTER the
//     Domain Identification phase heading (it needs the repo-layout
//     understanding Phases 1-2 build);
//  2. the `mindspec ownership populate` instruction appears AFTER the
//     `mindspec domain add` instruction (the manifests it populates
//     are the empty stubs `domain add` scaffolds).
func TestBuildMigratePrompt_PopulatePhaseOrdering(t *testing.T) {
	t.Parallel()

	prompt := buildMigratePrompt(nil, nil)

	domainIdent := strings.Index(prompt, "Domain Identification")
	domainAdd := strings.Index(prompt, "mindspec domain add")
	sourcePopulate := strings.Index(prompt, "mindspec source populate")
	ownershipPopulate := strings.Index(prompt, "mindspec ownership populate")

	for name, idx := range map[string]int{
		"Domain Identification":       domainIdent,
		"mindspec domain add":         domainAdd,
		"mindspec source populate":    sourcePopulate,
		"mindspec ownership populate": ownershipPopulate,
	} {
		if idx < 0 {
			t.Fatalf("prompt missing %q", name)
		}
	}

	if sourcePopulate < domainIdent {
		t.Errorf("`mindspec source populate` (idx %d) must appear AFTER the Domain Identification heading (idx %d)", sourcePopulate, domainIdent)
	}
	if ownershipPopulate < domainAdd {
		t.Errorf("`mindspec ownership populate` (idx %d) must appear AFTER the `mindspec domain add` instruction (idx %d)", ownershipPopulate, domainAdd)
	}
}

func TestMigrateJSON_ValidOutput(t *testing.T) {
	t.Parallel()

	inv := MigrateInventory{
		SourceFiles:    []string{"docs/guide.md"},
		CanonicalFiles: []string{".mindspec/docs/core/USAGE.md"},
	}
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed MigrateInventory
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.SourceFiles) != 1 || parsed.SourceFiles[0] != "docs/guide.md" {
		t.Errorf("unexpected source_files: %v", parsed.SourceFiles)
	}
	if len(parsed.CanonicalFiles) != 1 || parsed.CanonicalFiles[0] != ".mindspec/docs/core/USAGE.md" {
		t.Errorf("unexpected canonical_files: %v", parsed.CanonicalFiles)
	}
}
