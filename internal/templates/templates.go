package templates

// DomainTemplateFileNames lists the domain template files used for domain scaffolding.
var DomainTemplateFileNames = []string{
	"overview.md",
	"architecture.md",
	"interfaces.md",
	"runbook.md",
}

// Spec returns the built-in spec template.
func Spec() string {
	return specTemplate
}

// Plan returns the built-in plan template.
func Plan() string {
	return planTemplate
}

// SpecLifecycleFormula returns an empty string.
// Deprecated: spec lifecycle formulas are no longer used (Spec 054).
func SpecLifecycleFormula() string { return "" }

// ADR returns the built-in ADR template.
func ADR() string {
	return adrTemplate
}

// Domain returns the built-in domain template by filename.
func Domain(filename string) string {
	switch filename {
	case "overview.md":
		return `# {{.DomainName}} Domain — Overview

## What This Domain Owns

<List of responsibilities and capabilities this domain owns.>

## Boundaries

<What this domain does NOT own.>

## Key Files

| File | Purpose |
|:-----|:--------|
| <path> | <purpose> |

## Current State

<Brief description of implementation status.>
`
	case "architecture.md":
		return `# {{.DomainName}} Domain — Architecture

## Key Patterns

<Describe the key architectural patterns and design choices in this domain.>

## Invariants

1. <Invariant 1 — a property that must always hold>
`
	case "interfaces.md":
		return `# {{.DomainName}} Domain — Interfaces

## Provided Interfaces

<APIs, contracts, or capabilities this domain exposes to other domains.>

## Consumed Interfaces

<Interfaces from other domains that this domain depends on.>

## Events

<Events this domain emits or subscribes to.>
`
	case "runbook.md":
		return `# {{.DomainName}} Domain — Runbook

## Common Operations

<Step-by-step instructions for common tasks in this domain.>

## Troubleshooting

<Common issues and how to resolve them.>
`
	default:
		return ""
	}
}

const specTemplate = `---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec <ID>: <Title>

## Goal

<Brief description of what this spec achieves and the target user outcome>

## Background

<Context, motivation, and any relevant prior decisions>

## Impacted Domains

- <domain-1>: <how it is impacted>

## ADR Touchpoints

- [ADR-NNNN](../../adr/ADR-NNNN.md): <why this ADR is relevant>

## Requirements

1. <Requirement 1>
2. <Requirement 2>

## Scope

### In Scope
- <File or component 1>

### Out of Scope
- <Explicitly excluded items>

## Non-Goals

- <What this spec intentionally does not address>

## Acceptance Criteria

- [ ] <Specific, measurable criterion 1>
- [ ] <Specific, measurable criterion 2>

## Validation Proofs

- <command 1>: <Expected outcome>

## Open Questions

- [ ] <Question that must be resolved before planning>

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
`

const planTemplate = `---
status: Draft
spec_id: <NNN-slug>
version: "0.1"
last_updated: YYYY-MM-DD
work_chunks:
  - id: 1
    title: "<Short title for first chunk>"
    scope: "<Files or components this chunk delivers>"
    verify:
      - "<Specific, testable verification step>"
    depends_on: []
  - id: 2
    title: "<Short title for second chunk>"
    scope: "<Files or components>"
    verify:
      - "<Verification step>"
    depends_on: [1]
---

# Plan: Spec <NNN> — <Title>

**Spec**: [spec.md](spec.md)

---

## Bead <NNN>-A: <Short title>

**Scope**: <What this bead delivers>

**Steps**:
1. <Step 1>
2. <Step 2>
3. <Step 3>

**Verification**:
- [ ] <Specific, testable criterion>

**Acceptance Criteria**
- [ ] <Bead-specific acceptance criterion derived from spec AC>

**Depends on**: nothing

---

## Dependency Graph

<NNN>-A (<short description>)
  -> <NNN>-B (<short description>)
`

const adrTemplate = `# ADR-NNNN: <Title>

- **Date**: <YYYY-MM-DD>
- **Status**: Proposed
- **Domain(s)**: <comma-separated list>
- **Deciders**: <who decides>
- **Supersedes**: n/a
- **Superseded-by**: n/a

## Context

<What is the issue that we're seeing that motivates this decision or change?>

## Decision

<What is the change that we're proposing and/or doing?>

## Decision Details

<Detailed breakdown of the decision. Use subsections as needed.>

## Consequences

### Positive

- <Positive consequence 1>
- <Positive consequence 2>

### Negative / Tradeoffs

- <Negative consequence or tradeoff 1>
- <Negative consequence or tradeoff 2>

## Alternatives Considered

### 1. <Alternative name>

<Description and why it was rejected.>

## Validation / Rollout

1. <Validation step 1>
2. <Validation step 2>
`
