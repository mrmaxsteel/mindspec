package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Result tracks what the init operation created or skipped.
type Result struct {
	Created []string
	Skipped []string
	BeadsOK bool // true if bd/beads found in PATH
}

// FormatSummary returns a human-readable summary of the init result.
func (r *Result) FormatSummary() string {
	var sb strings.Builder

	if len(r.Created) > 0 {
		sb.WriteString("Created:\n")
		for _, p := range r.Created {
			sb.WriteString("  + ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	if len(r.Skipped) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Skipped (already exist):\n")
		for _, p := range r.Skipped {
			sb.WriteString("  - ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	if !r.BeadsOK {
		sb.WriteString("\nNote: 'bd' (Beads CLI) not found in PATH.\n")
		sb.WriteString("  Install Beads and run 'beads init' to enable task tracking.\n")
		sb.WriteString("  MindSpec works without Beads but the full workflow requires it.\n")
	}

	return sb.String()
}

// Run bootstraps a MindSpec project at root. If dryRun is true, no files are
// written — the result shows what would be created.
func Run(root string, dryRun bool) (*Result, error) {
	r := &Result{}

	// Check for Beads CLI
	r.BeadsOK = checkBeadsCLI()

	for _, item := range manifest() {
		target := filepath.Join(root, item.path)

		if item.isDir {
			if dirExists(target) {
				r.Skipped = append(r.Skipped, item.path+"/")
				continue
			}
			r.Created = append(r.Created, item.path+"/")
			if !dryRun {
				if err := os.MkdirAll(target, 0755); err != nil {
					return nil, fmt.Errorf("creating %s: %w", item.path, err)
				}
			}
		} else {
			if fileExists(target) {
				r.Skipped = append(r.Skipped, item.path)
				continue
			}
			r.Created = append(r.Created, item.path)
			if !dryRun {
				// Ensure parent dir exists
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return nil, fmt.Errorf("creating parent for %s: %w", item.path, err)
				}
				content := item.content
				if item.contentFunc != nil {
					content = item.contentFunc()
				}
				if err := os.WriteFile(target, []byte(content), 0644); err != nil {
					return nil, fmt.Errorf("writing %s: %w", item.path, err)
				}
			}
		}
	}

	return r, nil
}

type manifestItem struct {
	path        string
	isDir       bool
	content     string
	contentFunc func() string // lazy content (e.g. timestamp)
}

func manifest() []manifestItem {
	domains := []string{"core", "context-system", "workflow"}
	domainFileNames := []string{"overview.md", "architecture.md", "interfaces.md", "runbook.md"}

	items := []manifestItem{
		// Required directories
		{path: "docs/core", isDir: true},
		{path: "docs/domains", isDir: true},
		{path: "docs/specs", isDir: true},
		{path: "docs/adr", isDir: true},
		{path: "docs/templates", isDir: true},
		{path: "docs/templates/domain", isDir: true},
		{path: ".mindspec", isDir: true},

		// Root files
		{path: "GLOSSARY.md", content: starterGlossary},
		{path: "CLAUDE.md", content: starterClaudeMD},
		{path: "docs/context-map.md", content: starterContextMap},
		{path: ".mindspec/policies.yml", content: starterPolicies},
		{path: ".mindspec/state.json", contentFunc: starterState},

		// Templates
		{path: "docs/templates/spec.md", content: templateSpec},
		{path: "docs/templates/plan.md", content: templatePlan},
		{path: "docs/templates/adr.md", content: templateADR},
	}

	// Domain template files
	for _, tmplFile := range domainFileNames {
		items = append(items, manifestItem{
			path:    "docs/templates/domain/" + tmplFile,
			content: domainTemplate(tmplFile),
		})
	}

	// Domain scaffolding — one set of 4 files per domain
	for _, domain := range domains {
		domainDir := filepath.Join("docs", "domains", domain)
		items = append(items, manifestItem{path: domainDir, isDir: true})

		displayName := domainDisplayName(domain)
		for _, tmplFile := range domainFileNames {
			items = append(items, manifestItem{
				path:    filepath.Join(domainDir, tmplFile),
				content: strings.ReplaceAll(domainTemplate(tmplFile), "{{.DomainName}}", displayName),
			})
		}
	}

	return items
}

func domainDisplayName(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "-")
}

func checkBeadsCLI() bool {
	_, err := exec.LookPath("bd")
	if err == nil {
		return true
	}
	_, err = exec.LookPath("beads")
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func starterState() string {
	s := map[string]string{
		"mode":        "idle",
		"activeSpec":  "",
		"activeBead":  "",
		"lastUpdated": time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data) + "\n"
}

// --- Starter file content ---

const starterGlossary = `# Glossary

Maps key concepts to their primary documentation sections.

| Term | Target |
|:-----|:-------|
| **Spec Mode** | [docs/context-map.md](docs/context-map.md) |
| **Plan Mode** | [docs/context-map.md](docs/context-map.md) |
| **Implementation Mode** | [docs/context-map.md](docs/context-map.md) |
| **Human Gate** | [docs/context-map.md](docs/context-map.md) |
| **Domain** | [docs/context-map.md](docs/context-map.md) |
| **Context Map** | [docs/context-map.md](docs/context-map.md) |
`

const starterClaudeMD = `# CLAUDE.md

Run ` + "`mindspec instruct`" + ` for mode-appropriate operating guidance. This is emitted automatically by the SessionStart hook.

## Build & Test

` + "```bash" + `
make build    # Build binary
make test     # Run all tests
` + "```" + `

## Custom Commands

| Command | Purpose |
|:--------|:--------|
| ` + "`/spec-init`" + ` | Initialize a new specification (enters Spec Mode) |
| ` + "`/spec-approve`" + ` | Approve spec → Plan Mode |
| ` + "`/plan-approve`" + ` | Approve plan → Implementation Mode |
| ` + "`/impl-approve`" + ` | Approve implementation → Idle |
| ` + "`/spec-status`" + ` | Check current mode and active spec/bead state |
`

const starterContextMap = `# Context Map

> Declares the bounded contexts in this project, their relationships, and integration contracts.

## Bounded Contexts

### Core

**Owns**: <List core responsibilities>

**Domain docs**: [` + "`" + `docs/domains/core/` + "`" + `](domains/core/overview.md)

### Context-System

**Owns**: <List context-system responsibilities>

**Domain docs**: [` + "`" + `docs/domains/context-system/` + "`" + `](domains/context-system/overview.md)

### Workflow

**Owns**: <List workflow responsibilities>

**Domain docs**: [` + "`" + `docs/domains/workflow/` + "`" + `](domains/workflow/overview.md)

---

## Relationships

` + "```" + `
Core <--- Context-System ---> Workflow
` + "```" + `

---

## Source of Truth

| Concept | Authoritative Location |
|:--------|:----------------------|
| Project structure health | Core (` + "`mindspec doctor`" + `) |
| Glossary term mapping | ` + "`GLOSSARY.md`" + ` |
| Mode state and transitions | Workflow |
| Long-form specifications | ` + "`docs/specs/`" + ` |
| Domain architecture | ` + "`docs/domains/<domain>/`" + ` |
| ADR lifecycle | ` + "`docs/adr/`" + ` |
| Machine-checkable policies | ` + "`.mindspec/policies.yml`" + ` |
`

const starterPolicies = `# Architecture Policies

policies:
  - id: spec-mode-no-code
    description: "In Spec Mode, only markdown files may be created or modified."
    severity: error
    mode: spec

  - id: plan-mode-no-code
    description: "In Plan Mode, only plans, ADR proposals, and documentation may be modified."
    severity: error
    mode: plan

  - id: spec-required
    description: "Every functional change must refer to a spec in docs/specs/"
    severity: error

  - id: doc-sync-required
    description: "Changes to core logic must be accompanied by documentation updates."
    severity: warning

  - id: clean-tree-before-transition
    description: "Working tree must be clean before starting new work or switching modes."
    severity: error
`

// --- Template content ---

const templateSpec = `# Spec <ID>: <Title>

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

- ` + "`<command 1>`" + `: <Expected outcome>

## Open Questions

- [ ] <Question that must be resolved before planning>

## Approval

- **Status**: DRAFT
- **Approved By**: —
- **Approval Date**: —
- **Notes**: —
`

const templatePlan = `---
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

**Depends on**: nothing

---

## Dependency Graph

` + "```" + `
<NNN>-A (<short description>)
  └── <NNN>-B (<short description>)
` + "```" + `
`

const templateADR = `# ADR-NNNN: <Title>

- **Status**: Proposed
- **Date**: YYYY-MM-DD
- **Domains**: <impacted domains>
- **Supersedes**: —

## Context

<What is the issue that motivates this decision?>

## Decision

<What is the change that we're proposing and/or doing?>

## Consequences

### Positive
- <Positive consequence>

### Negative
- <Negative consequence>

### Neutral
- <Neutral consequence>
`

func domainTemplate(filename string) string {
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
| ` + "`<path>`" + ` | <purpose> |

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
