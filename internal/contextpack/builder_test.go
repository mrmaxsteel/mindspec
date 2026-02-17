package contextpack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestProject creates a minimal project structure for integration testing.
func setupTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Spec
	specDir := filepath.Join(root, "docs", "specs", "001-test")
	os.MkdirAll(specDir, 0o755)
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(`# Spec 001: Test

## Goal

Build a test feature.

## Impacted Domains

- core: CLI and workspace
- context-system: context delivery

## Requirements

1. Test requirement
`), 0o644)

	// Domain docs - core
	coreDir := filepath.Join(root, "docs", "domains", "core")
	os.MkdirAll(coreDir, 0o755)
	os.WriteFile(filepath.Join(coreDir, "overview.md"), []byte("# Core Overview\nCore owns the CLI."), 0o644)
	os.WriteFile(filepath.Join(coreDir, "architecture.md"), []byte("# Core Architecture\nCLI patterns."), 0o644)
	os.WriteFile(filepath.Join(coreDir, "interfaces.md"), []byte("# Core Interfaces\nFindRoot() etc."), 0o644)
	os.WriteFile(filepath.Join(coreDir, "runbook.md"), []byte("# Core Runbook\nBuild with make."), 0o644)

	// Domain docs - context-system
	csDir := filepath.Join(root, "docs", "domains", "context-system")
	os.MkdirAll(csDir, 0o755)
	os.WriteFile(filepath.Join(csDir, "overview.md"), []byte("# Context-System Overview\nOwns context packs."), 0o644)
	os.WriteFile(filepath.Join(csDir, "architecture.md"), []byte("# Context-System Architecture\nDDD routing."), 0o644)
	os.WriteFile(filepath.Join(csDir, "interfaces.md"), []byte("# Context-System Interfaces\nBuild() etc."), 0o644)
	os.WriteFile(filepath.Join(csDir, "runbook.md"), []byte("# Context-System Runbook\nGenerate packs."), 0o644)

	// Neighbor domain - workflow
	wfDir := filepath.Join(root, "docs", "domains", "workflow")
	os.MkdirAll(wfDir, 0o755)
	os.WriteFile(filepath.Join(wfDir, "overview.md"), []byte("# Workflow Overview\nModes and lifecycle."), 0o644)
	os.WriteFile(filepath.Join(wfDir, "interfaces.md"), []byte("# Workflow Interfaces\nSpec metadata."), 0o644)

	// Context map
	os.WriteFile(filepath.Join(root, "docs", "context-map.md"), []byte(`# Context Map

## Relationships

### Core → Context-System (upstream)

Core provides workspace resolution.

**Contract**: [interfaces](domains/core/interfaces.md)

### Core → Workflow (upstream)

Core provides CLI shell.

**Contract**: [interfaces](domains/core/interfaces.md)

### Workflow → Context-System (upstream)

Workflow provides spec metadata.

**Contract**: [interfaces](domains/workflow/interfaces.md)

### Context-System → Workflow (downstream)

Context-system delivers context packs.
`), 0o644)

	// ADRs
	adrDir := filepath.Join(root, "docs", "adr")
	os.MkdirAll(adrDir, 0o755)
	os.WriteFile(filepath.Join(adrDir, "ADR-0001.md"), []byte(`# ADR-0001: DDD

- **Status**: Accepted
- **Domain(s)**: core, context-system

## Decision
Use DDD.
`), 0o644)
	os.WriteFile(filepath.Join(adrDir, "ADR-0002.md"), []byte(`# ADR-0002: Beads

- **Status**: Accepted
- **Domain(s)**: workflow

## Decision
Beads as substrate.
`), 0o644)

	// Policies (canonical path)
	msDir := filepath.Join(root, ".mindspec")
	os.MkdirAll(msDir, 0o755)
	os.WriteFile(filepath.Join(msDir, "policies.yml"), []byte(`policies:
  - id: spec-no-code
    description: "No code in spec mode"
    severity: error
    mode: spec

  - id: doc-sync
    description: "Doc sync required"
    severity: warning

  - id: impl-scope
    description: "Scope discipline"
    severity: error
    mode: implementation
`), 0o644)

	return root
}

func TestBuild_SpecMode(t *testing.T) {
	root := setupTestProject(t)

	pack, err := Build(root, "001-test", ModeSpec)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if pack.SpecID != "001-test" {
		t.Errorf("SpecID = %q", pack.SpecID)
	}
	if pack.Mode != ModeSpec {
		t.Errorf("Mode = %q", pack.Mode)
	}
	if pack.Goal != "Build a test feature." {
		t.Errorf("Goal = %q", pack.Goal)
	}

	rendered := pack.Render()

	// Spec mode should include overview but NOT architecture
	if !strings.Contains(rendered, "Core Overview") {
		t.Error("missing core overview in spec mode")
	}
	if strings.Contains(rendered, "Core Architecture") {
		t.Error("spec mode should not include architecture")
	}

	// Should include ADR-0001 (domains: core, context-system) but not ADR-0002 (workflow)
	if !strings.Contains(rendered, "ADR-0001") {
		t.Error("missing ADR-0001")
	}
	if strings.Contains(rendered, "ADR-0002") {
		t.Error("should not include ADR-0002 (workflow domain only)")
	}

	// Should include policies
	if !strings.Contains(rendered, "spec-no-code") {
		t.Error("missing spec-mode policy")
	}
	if !strings.Contains(rendered, "doc-sync") {
		t.Error("missing global policy")
	}
	// impl-scope should be excluded in spec mode
	if strings.Contains(rendered, "impl-scope") {
		t.Error("should not include implementation-only policy in spec mode")
	}

	// Provenance
	if !strings.Contains(rendered, "## Provenance") {
		t.Error("missing provenance section")
	}
}

func TestBuild_PlanMode(t *testing.T) {
	root := setupTestProject(t)

	pack, err := Build(root, "001-test", ModePlan)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	rendered := pack.Render()

	// Plan mode should include architecture
	if !strings.Contains(rendered, "Core Architecture") {
		t.Error("plan mode should include architecture")
	}

	// Plan mode should include neighbor interfaces
	if !strings.Contains(rendered, "Workflow Interfaces") {
		t.Error("plan mode should include neighbor interfaces")
	}

	// Plan mode should NOT include impacted domain interfaces (that's implement tier)
	if strings.Contains(rendered, "Domain: core — Interfaces") {
		t.Error("plan mode should not include impacted domain interfaces")
	}
}

func TestBuild_ImplementMode(t *testing.T) {
	root := setupTestProject(t)

	pack, err := Build(root, "001-test", ModeImplement)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	rendered := pack.Render()

	// Implement mode should include everything
	if !strings.Contains(rendered, "Core Architecture") {
		t.Error("implement mode should include architecture")
	}
	if !strings.Contains(rendered, "Domain: core — Interfaces") {
		t.Error("implement mode should include impacted domain interfaces")
	}
	if !strings.Contains(rendered, "Core Runbook") {
		t.Error("implement mode should include runbook")
	}
	if !strings.Contains(rendered, "Workflow Interfaces") {
		t.Error("implement mode should include neighbor interfaces")
	}
}

func TestBuild_PoliciesLegacyFallback(t *testing.T) {
	root := setupTestProject(t)

	// Move policies to legacy location to verify fallback behavior.
	if err := os.MkdirAll(filepath.Join(root, "architecture"), 0o755); err != nil {
		t.Fatalf("mkdir legacy architecture dir: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".mindspec", "policies.yml"))
	if err != nil {
		t.Fatalf("read canonical policies: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "architecture", "policies.yml"), data, 0o644); err != nil {
		t.Fatalf("write legacy policies: %v", err)
	}
	if err := os.Remove(filepath.Join(root, ".mindspec", "policies.yml")); err != nil {
		t.Fatalf("remove canonical policies: %v", err)
	}

	pack, err := Build(root, "001-test", ModeSpec)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	rendered := pack.Render()
	if !strings.Contains(rendered, "spec-no-code") {
		t.Fatal("expected policies section via legacy fallback")
	}
	if !strings.Contains(rendered, "architecture/policies.yml") {
		t.Fatal("expected legacy policy path in provenance when fallback is used")
	}
}

func TestBuild_NonexistentSpec(t *testing.T) {
	root := setupTestProject(t)
	_, err := Build(root, "nonexistent", ModeSpec)
	if err == nil {
		t.Fatal("expected error for nonexistent spec")
	}
}

func TestWriteToFile(t *testing.T) {
	root := setupTestProject(t)

	pack, err := Build(root, "001-test", ModeSpec)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if err := pack.WriteToFile(root, "001-test"); err != nil {
		t.Fatalf("WriteToFile: %v", err)
	}

	outPath := filepath.Join(root, "docs", "specs", "001-test", "context-pack.md")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "# Context Pack") {
		t.Error("output missing header")
	}
	if !strings.Contains(content, "## Provenance") {
		t.Error("output missing provenance")
	}
}

func TestProvenance_HasEntryPerSection(t *testing.T) {
	root := setupTestProject(t)

	pack, err := Build(root, "001-test", ModeSpec)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// In spec mode with 2 impacted domains: 2 overviews + 1 ADR + 1 policies = 4 provenance entries
	if len(pack.Provenance) < 3 {
		t.Errorf("expected at least 3 provenance entries, got %d", len(pack.Provenance))
	}

	// Each section should have a corresponding provenance entry
	for _, p := range pack.Provenance {
		if p.Source == "" || p.Reason == "" {
			t.Errorf("provenance entry missing source or reason: %+v", p)
		}
	}
}
