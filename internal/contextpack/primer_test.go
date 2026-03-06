package contextpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

func setupPrimerTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	specDir := filepath.Join(root, "docs", "specs", "047-test")
	os.MkdirAll(specDir, 0o755)
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(`# Spec 047: Test

## Goal

Test the primer.

## Impacted Domains

- core: CLI

## Requirements

1. First requirement
2. Second requirement

## Acceptance Criteria

- AC1: Primer works
- AC2: Primer is lean
`), 0o644)

	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(`---
status: Approved
spec_id: "047-test"
version: "1"
---

# Plan

## Bead 1: State flag

**Steps**
1. Add field to internal/state/state.go
2. Add helper
3. Add tests

**Verification**
- [ ] `+"`go test ./internal/state/...`"+` passes

**Depends on**
None
`), 0o644)

	// Domain docs
	coreDir := filepath.Join(root, "docs", "domains", "core")
	os.MkdirAll(coreDir, 0o755)
	os.WriteFile(filepath.Join(coreDir, "overview.md"), []byte("# Core\nCore owns the CLI."), 0o644)

	// ADR
	adrDir := filepath.Join(root, "docs", "adr")
	os.MkdirAll(adrDir, 0o755)
	os.WriteFile(filepath.Join(adrDir, "ADR-0001.md"), []byte(`# ADR-0001: DDD

- **Status**: Accepted
- **Domain(s)**: core

## Decision

Use DDD patterns.
`), 0o644)

	return root
}

func TestBuildBeadPrimer_Integration(t *testing.T) {
	root := setupPrimerTestProject(t)

	orig := runBDFn
	defer func() { runBDFn = orig }()
	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 3 && args[0] == "show" {
			info := []bead.BeadInfo{{
				ID:          args[1],
				Title:       "[IMPL 047-test.1] Bead 1: State flag",
				Description: "Add needs_clear flag to state",
			}}
			return json.Marshal(info)
		}
		return nil, nil
	}

	primer, err := BuildBeadPrimer(root, "047-test", "bead-1")
	if err != nil {
		t.Fatalf("BuildBeadPrimer: %v", err)
	}

	if primer.BeadTitle != "[IMPL 047-test.1] Bead 1: State flag" {
		t.Errorf("BeadTitle = %q", primer.BeadTitle)
	}
	if primer.BeadDescription != "Add needs_clear flag to state" {
		t.Errorf("BeadDescription = %q", primer.BeadDescription)
	}
	if primer.Requirements == "" {
		t.Error("expected requirements to be extracted")
	}
	if !strings.Contains(primer.Requirements, "First requirement") {
		t.Error("requirements missing expected content")
	}
	if primer.AcceptanceCriteria == "" {
		t.Error("expected acceptance criteria to be extracted")
	}
	if primer.PlanWorkChunk == "" {
		t.Error("expected plan work chunk to be extracted")
	}
	if !strings.Contains(primer.PlanWorkChunk, "internal/state/state.go") {
		t.Error("work chunk missing file reference")
	}
	if len(primer.FilePaths) == 0 {
		t.Error("expected file paths to be extracted")
	}
	if len(primer.ADRDecisions) == 0 {
		t.Error("expected ADR decisions to be extracted")
	}
	if primer.ADRDecisions[0].Decision != "Use DDD patterns." {
		t.Errorf("ADR decision = %q", primer.ADRDecisions[0].Decision)
	}
	if len(primer.DomainOverviews) == 0 {
		t.Error("expected domain overviews")
	}
	if primer.EstimatedTokens <= 0 {
		t.Error("expected positive token estimate")
	}
}

func TestBuildBeadPrimer_GracefulDegradation_NoSpec(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs", "specs", "missing"), 0o755)

	orig := runBDFn
	defer func() { runBDFn = orig }()
	runBDFn = func(args ...string) ([]byte, error) {
		info := []bead.BeadInfo{{ID: "bead-1", Title: "Test", Description: "Desc"}}
		return json.Marshal(info)
	}

	primer, err := BuildBeadPrimer(root, "missing", "bead-1")
	if err != nil {
		t.Fatalf("expected graceful degradation, got error: %v", err)
	}
	if primer.Requirements != "" {
		t.Error("expected empty requirements when spec.md missing")
	}
	// Should still render without error
	rendered := RenderBeadPrimer(primer)
	if !strings.Contains(rendered, "# Bead Context:") {
		t.Error("rendered output should still have header")
	}
}

func TestExtractBeadSection(t *testing.T) {
	content := `## Bead 1: First

Step one details.
Step two details.

## Bead 2: Second

Other details.
`
	// Match by title containing "Bead 1: First"
	got := extractBeadSection(content, "[IMPL test.1] Bead 1: First")
	if !strings.Contains(got, "Step one details") {
		t.Errorf("expected to find bead 1 content, got %q", got)
	}
	if strings.Contains(got, "Other details") {
		t.Error("should not include bead 2 content")
	}
}
