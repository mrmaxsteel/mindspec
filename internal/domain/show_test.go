package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupShowTestRoot(t *testing.T) string {
	t.Helper()
	root := setupTestRoot(t)

	// Create a domain with populated overview
	coreDir := filepath.Join(root, "docs", "domains", "core")
	os.MkdirAll(coreDir, 0755)

	overview := `# Core Domain — Overview

## What This Domain Owns

The core domain owns CLI entry point and workspace resolution.

## Boundaries

Core does not own glossary parsing or mode enforcement.

## Key Files

| File | Purpose |
|:-----|:--------|
| cmd/mindspec/main.go | CLI entry point |

## Current State

Implemented.
`
	os.WriteFile(filepath.Join(coreDir, "overview.md"), []byte(overview), 0644)

	// Update context map to include relationships
	cm := `# Context Map

## Bounded Contexts

### Core

**Owns**: CLI entry point, workspace resolution.

**Domain docs**: docs/domains/core/

---

## Relationships

### Core → Context-System (upstream)

Core provides workspace resolution. Context-system consumes it.

**Contract**: docs/domains/core/interfaces.md
`
	os.WriteFile(filepath.Join(root, "docs", "context-map.md"), []byte(cm), 0644)

	// Create a spec that impacts core
	specDir := filepath.Join(root, "docs", "specs", "011-domain-scaffold")
	os.MkdirAll(specDir, 0755)
	spec := `# Spec 011-domain-scaffold: Domain Scaffold

## Goal

Make domains a first-class CLI primitive.

## Impacted Domains

- core: New CLI subcommands
- context-system: Reinforces DDD routing
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0644)

	return root
}

func TestShowExistingDomain(t *testing.T) {
	root := setupShowTestRoot(t)

	info, err := Show(root, "core")
	if err != nil {
		t.Fatalf("Show() error: %v", err)
	}

	if info.Name != "core" {
		t.Errorf("expected name=core, got %q", info.Name)
	}
	if !strings.Contains(info.Owns, "CLI entry point") {
		t.Errorf("expected Owns to contain 'CLI entry point', got %q", info.Owns)
	}
	if !strings.Contains(info.Boundaries, "does not own") {
		t.Errorf("expected Boundaries content, got %q", info.Boundaries)
	}
	if !strings.Contains(info.KeyFiles, "main.go") {
		t.Errorf("expected KeyFiles to contain 'main.go', got %q", info.KeyFiles)
	}
	if len(info.Relationships) == 0 {
		t.Error("expected at least one relationship")
	}
	if len(info.Specs) == 0 {
		t.Error("expected at least one impacting spec")
	}
	if info.Specs[0] != "011-domain-scaffold" {
		t.Errorf("expected spec 011-domain-scaffold, got %q", info.Specs[0])
	}
}

func TestShowNonexistentDomain(t *testing.T) {
	root := setupShowTestRoot(t)

	_, err := Show(root, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent domain, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}
}

func TestFormatJSON(t *testing.T) {
	info := &DomainInfo{
		Name:       "core",
		Owns:       "CLI, workspace",
		Boundaries: "Not glossary",
		Relationships: []RelInfo{
			{Domain: "Context-System", Direction: "→ upstream"},
		},
		Specs: []string{"011-domain-scaffold"},
	}

	out, err := FormatJSON(info)
	if err != nil {
		t.Fatalf("FormatJSON() error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["name"] != "core" {
		t.Errorf("expected name=core in JSON, got %v", parsed["name"])
	}
	if parsed["owns"] != "CLI, workspace" {
		t.Errorf("expected owns in JSON, got %v", parsed["owns"])
	}
}

func TestFormatSummary(t *testing.T) {
	info := &DomainInfo{
		Name:       "core",
		Owns:       "CLI, workspace",
		Boundaries: "Not glossary",
		Relationships: []RelInfo{
			{Domain: "Context-System", Direction: "→ upstream"},
		},
		Specs: []string{"011-domain-scaffold"},
	}

	out := FormatSummary(info)
	if !strings.Contains(out, "Domain: core") {
		t.Error("missing domain name in summary")
	}
	if !strings.Contains(out, "Owns:") {
		t.Error("missing Owns section")
	}
	if !strings.Contains(out, "Relationships:") {
		t.Error("missing Relationships section")
	}
	if !strings.Contains(out, "Specs:") {
		t.Error("missing Specs section")
	}
}

func TestExtractSection(t *testing.T) {
	content := `# Title

## Section One

Content of section one.
More content.

## Section Two

Content of section two.
`
	got := extractSection(content, "Section One")
	if !strings.Contains(got, "Content of section one") {
		t.Errorf("expected section one content, got: %q", got)
	}
	if strings.Contains(got, "Section Two") {
		t.Error("section one should not contain section two content")
	}

	got2 := extractSection(content, "Section Two")
	if !strings.Contains(got2, "Content of section two") {
		t.Errorf("expected section two content, got: %q", got2)
	}
}

func TestExtractSectionCaseInsensitive(t *testing.T) {
	content := `## What This Domain Owns

The core domain owns stuff.

## Boundaries

Other stuff.
`
	got := extractSection(content, "what this domain owns")
	if !strings.Contains(got, "core domain owns") {
		t.Errorf("expected case-insensitive match, got: %q", got)
	}
}
