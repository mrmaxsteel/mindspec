package approve

// Spec 097 R2 (mindspec-4axk) — the bead `--design` ADR list is built from
// the plan's structured `adr_citations` frontmatter, NOT from a regex scrape
// of the spec's `## ADR Touchpoints` prose.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeADRFixture writes a minimal, canonical-layout ADR whose title the
// adr.FileStore parses from the `# <ID>: <title>` header.
func writeADRFixture(t *testing.T, adrDir, id, title string) {
	t.Helper()
	content := fmt.Sprintf(`# %s: %s

- **Status**: Accepted
- **Domain(s)**: core

## Context

Some context.

## Decision

Some decision.

## Consequences

Some consequences.
`, id, title)
	if err := os.WriteFile(filepath.Join(adrDir, id+".md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write adr %s: %v", id, err)
	}
}

// TestCreateImplementationBeads_DesignFromADRCitationsNotProse is the spec
// 097 R2 RED-on-revert pin. A spec cites TWO ADRs in `## ADR Touchpoints`
// prose, but the plan declares only ONE of them in structured
// `adr_citations`. The created bead's `--design` field must carry the
// DECLARED ADR (by ID + title) and must NOT carry the prose-only ADR.
//
// RED-on-revert: reverting `buildDesignField` to the retired
// `adrIDRe`/`parseADRIDs` scrape of `## ADR Touchpoints` would harvest the
// prose-only ADR-0002 into `--design`, failing the second assertion.
func TestCreateImplementationBeads_DesignFromADRCitationsNotProse(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, ".mindspec", "docs", "specs", "097-test")
	adrDir := filepath.Join(tmp, ".mindspec", "docs", "adr")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec: %v", err)
	}
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("mkdir adr: %v", err)
	}

	// Both ADRs exist in the store; titles are distinct so we can tell which
	// one was harvested.
	writeADRFixture(t, adrDir, "ADR-0001", "Declared In Frontmatter")
	writeADRFixture(t, adrDir, "ADR-0002", "Prose Only")

	// spec.md cites BOTH ADRs in `## ADR Touchpoints` prose.
	specContent := "# Spec\n\n## ADR Touchpoints\n" +
		"- [ADR-0001](../../adr/ADR-0001.md): declared\n" +
		"- [ADR-0002](../../adr/ADR-0002.md): prose only\n\n" +
		"## Requirements\n1. Do the thing\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	// plan.md declares ONLY ADR-0001 in structured `adr_citations`.
	planContent := `---
status: Approved
spec_id: "097-test"
version: "1.0"
adr_citations:
  - id: ADR-0001
---

# Plan

## Bead 1: Only bead

**Steps**
1. Step one

**Verification**
- [ ] Tests pass

**Acceptance Criteria**
- [ ] It works

**Depends on**
None
`
	planPath := filepath.Join(specDir, "plan.md")
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	// Children-listing seam: no existing children.
	origList := planListJSONFn
	defer func() { planListJSONFn = origList }()
	planListJSONFn = func(args ...string) ([]byte, error) { return []byte(`[]`), nil }

	// bd-create seam: capture the `--design` payload.
	origRun := planRunBDFn
	defer func() { planRunBDFn = origRun }()
	var design string
	planRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			for i, a := range args {
				if a == "--design" && i+1 < len(args) {
					design = args[i+1]
				}
			}
			return []byte(`{"id":"bead-1"}`), nil
		}
		return nil, nil // dep add etc.
	}

	ids, _, err := createImplementationBeads(planPath, "097-test", "epic-097")
	if err != nil {
		t.Fatalf("createImplementationBeads: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 bead, got %v", ids)
	}

	// The DECLARED adr_citations ID IS harvested, by ID + title.
	if !strings.Contains(design, "see ADR-0001 — Declared In Frontmatter") {
		t.Errorf("declared adr_citations ID must be harvested into --design; got:\n%s", design)
	}
	// The prose-only ID (absent from adr_citations) is NOT harvested — the
	// RED-on-revert assertion.
	if strings.Contains(design, "ADR-0002") {
		t.Errorf("prose-only ADR ID must NOT be harvested from `## ADR Touchpoints`; got:\n%s", design)
	}
}
