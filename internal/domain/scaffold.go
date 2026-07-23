package domain

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/ownership"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// populatePromptWriter receives the OWNERSHIP.yaml populate prompt
// that Add prints after scaffolding (spec 091 Req 9). Defaults to
// stdout; tests swap it to capture the output. Add keeps its
// (root, name) signature so existing call sites are untouched.
var populatePromptWriter io.Writer = os.Stdout

// scaffoldMappedCheck is the "is this domain already mapped in
// context-map.md" predicate Add's convergence check consumes. It defaults
// to the exported HasEntry — the SAME helper doctor's unmapped-domain
// detection consumes (as its own mirrored seam var) — never a private
// reimplementation. An identity-pinning test in this package (and one in
// internal/doctor) asserts both seam vars still point at HasEntry, so a
// future refactor cannot quietly let the two sides disagree about what
// "mapped" means (spec 123 R3/AC-4).
var scaffoldMappedCheck = HasEntry

// Add scaffolds a new domain directory with 4 template files plus an
// empty-stub OWNERSHIP.yaml (spec 091 Req 9), appends a bounded
// context entry to the context map, and prints the ownership populate
// prompt for the new domain.
//
// Spec 123 R2 turned the former one-shot "refuse if the dir exists" guard
// into a convergence check: Add now succeeds from ANY partial state —
// domain dir present with some/all standard files missing, and/or the
// context-map.md entry missing (including context-map.md itself being
// entirely absent, the exact #207 aftermath). It refuses with "already
// exists" ONLY when the domain is fully scaffolded (all four templates +
// OWNERSHIP.yaml present) AND mapped (a context-map entry already exists).
// Every write below is create-if-missing / idempotent, and write order is
// domain files first, context-map entry last (R2c), so a failure partway
// through always leaves a state a bare re-run repairs.
//
// The previously local `nameRe` regex was promoted into idvalidate.DomainName
// (re-exported as validate.DomainName) so the validator is shared across all
// CLI entrypoints (SEC-1, bead mindspec-x1qr). Do not reintroduce a local
// regex check here — duplication invites drift.
func Add(root, name string) error {
	domainDir, err := workspace.DomainDir(root, name)
	if err != nil {
		return err
	}
	// Defense in depth: DomainDir already validates, but call again explicitly
	// so future refactors that bypass DomainDir still get validation.
	if err := validate.DomainName(name); err != nil {
		return err
	}

	title := titleCase(name)

	templates := map[string]string{
		"overview.md":     overviewTemplate(title),
		"architecture.md": architectureTemplate(title),
		"interfaces.md":   interfacesTemplate(title),
		"runbook.md":      runbookTemplate(title),
	}
	ownerPath := filepath.Join(domainDir, "OWNERSHIP.yaml")

	// Determine completeness BEFORE writing anything, so the refusal
	// decision reflects the state Add was called against, not the state
	// this call is about to create.
	dirPreexisted := dirExists(domainDir)
	missingTemplate := false
	for filename := range templates {
		if !fileExists(filepath.Join(domainDir, filename)) {
			missingTemplate = true
			break
		}
	}
	missingOwner := !fileExists(ownerPath)
	mapped := false
	if cmData, err := os.ReadFile(workspace.ContextMapPath(root)); err == nil {
		mapped = scaffoldMappedCheck(string(cmData), name)
	}

	if dirPreexisted && !missingTemplate && !missingOwner && mapped {
		return fmt.Errorf("domain %q already exists", name)
	}

	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return fmt.Errorf("creating domain directory: %w", err)
	}

	// R2(b): create-if-missing — each standard file is written only if
	// absent; an existing, possibly operator-edited file is never
	// overwritten.
	for filename, content := range templates {
		path := filepath.Join(domainDir, filename)
		if fileExists(path) {
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	// Spec 091 Req 9: after the four standard files, scaffold the
	// empty-stub OWNERSHIP.yaml (Req 8 stub, `domain add` comment
	// variant) if it is missing. No flag, no opt-out — the framework
	// writes the stub mechanically; the resident agent does the
	// cognitive work of choosing paths (ZFC).
	if !fileExists(ownerPath) {
		stub := ownership.RenderStub("mindspec domain add " + name)
		if err := os.WriteFile(ownerPath, stub, 0644); err != nil {
			return fmt.Errorf("writing OWNERSHIP.yaml: %w", err)
		}
	}

	// Context-map entry last (R2c): domain files/manifest are the
	// dependency-free half of scaffolding, so writing them first means a
	// crash before the context-map write still leaves a re-runnable state.
	if err := appendContextMap(root, name, title); err != nil {
		return fmt.Errorf("updating context map: %w", err)
	}

	fmt.Fprintln(populatePromptWriter, ownership.BuildPopulatePrompt(name))

	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// titleCase converts "my-domain" → "My-Domain".
func titleCase(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "-")
}

// appendContextMap adds a new bounded context entry to context-map.md,
// converging from every partial state (spec 123 R2):
//   - context-map.md absent (R2a) — the R1 skeleton (domain.ContextMapSkeleton)
//     is created in memory first, so the insertion scan below always finds a
//     "## Bounded Contexts" section to insert into, never falling back to a
//     tail-append.
//   - the domain is already mapped (scaffoldMappedCheck/HasEntry) — a no-op,
//     so a re-run never duplicates an existing entry.
func appendContextMap(root, name, title string) error {
	cmPath := workspace.ContextMapPath(root)
	data, err := os.ReadFile(cmPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading context map: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(cmPath), 0755); err != nil {
			return fmt.Errorf("creating context map directory: %w", err)
		}
		data = []byte(ContextMapSkeleton())
	}

	content := string(data)

	if scaffoldMappedCheck(content, name) {
		// Already mapped — nothing to backfill; leave the file untouched
		// (byte-identical re-run, no duplicate entry).
		return nil
	}

	docsRoot := docsRootLabel(root)
	entry := fmt.Sprintf("\n### %s\n\n**Owns**: _(fill in)_\n\n**Domain docs**: [`%s/domains/%s/`](domains/%s/overview.md)\n",
		title, docsRoot, name, name)

	// Find the --- separator after Bounded Contexts section.
	// Insert the new entry before it.
	lines := strings.Split(content, "\n")

	inBoundedContexts := false
	insertIdx := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			heading := strings.TrimPrefix(trimmed, "## ")
			if strings.EqualFold(strings.TrimSpace(heading), "Bounded Contexts") {
				inBoundedContexts = true
				continue
			}
			if inBoundedContexts {
				// Hit next ## section — insert before this
				insertIdx = i
				break
			}
		}

		if inBoundedContexts && trimmed == "---" {
			insertIdx = i
			break
		}
	}

	if insertIdx >= 0 {
		// Insert entry before the separator/next section
		entryLines := strings.Split(entry, "\n")
		newLines := make([]string, 0, len(lines)+len(entryLines))
		newLines = append(newLines, lines[:insertIdx]...)
		newLines = append(newLines, entryLines...)
		newLines = append(newLines, lines[insertIdx:]...)
		content = strings.Join(newLines, "\n")
	} else {
		// No separator found — append at end
		content += entry
	}

	if err := os.WriteFile(cmPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing context map: %w", err)
	}

	return nil
}

func overviewTemplate(title string) string {
	return fmt.Sprintf(`# %s Domain — Overview

## What This Domain Owns

_(Describe what this bounded context owns.)_

## Boundaries

_(Define what this domain does NOT own — what belongs to other contexts.)_

## Key Files

| File | Purpose |
|:-----|:--------|
| | |

## Current State

_(Describe the current implementation state of this domain.)_
`, title)
}

func architectureTemplate(title string) string {
	return fmt.Sprintf(`# %s Domain — Architecture

## Key Patterns

_(Describe the architectural patterns used in this domain.)_

## Invariants

_(List the invariants this domain must maintain.)_

## Design Decisions

_(Document key design decisions and their rationale.)_
`, title)
}

func interfacesTemplate(title string) string {
	return fmt.Sprintf(`# %s Domain — Interfaces

## Contracts

_(Define the contracts this domain exposes to other contexts.)_

## Integration Points

_(Describe how other domains integrate with this one.)_
`, title)
}

func runbookTemplate(title string) string {
	return fmt.Sprintf(`# %s Domain — Runbook

## Development Workflows

_(Document common development workflows for this domain.)_

## Debugging

_(Tips for debugging issues in this domain.)_

## Common Tasks

_(Step-by-step guides for common operational tasks.)_
`, title)
}

func docsRootLabel(root string) string {
	rel, err := filepath.Rel(root, workspace.DocsDir(root))
	if err != nil {
		return "docs"
	}
	return filepath.ToSlash(rel)
}
