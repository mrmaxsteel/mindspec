package domain

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Add scaffolds a new domain directory with 4 template files and appends
// a bounded context entry to the context map.
func Add(root, name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid domain name %q: must match [a-z][a-z0-9-]*", name)
	}

	domainDir := workspace.DomainDir(root, name)
	if _, err := os.Stat(domainDir); err == nil {
		return fmt.Errorf("domain %q already exists", name)
	}

	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return fmt.Errorf("creating domain directory: %w", err)
	}

	title := titleCase(name)

	templates := map[string]string{
		"overview.md":     overviewTemplate(title),
		"architecture.md": architectureTemplate(title),
		"interfaces.md":   interfacesTemplate(title),
		"runbook.md":      runbookTemplate(title),
	}

	for filename, content := range templates {
		path := filepath.Join(domainDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	if err := appendContextMap(root, name, title); err != nil {
		return fmt.Errorf("updating context map: %w", err)
	}

	return nil
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

// appendContextMap adds a new bounded context entry to context-map.md.
func appendContextMap(root, name, title string) error {
	cmPath := workspace.ContextMapPath(root)
	data, err := os.ReadFile(cmPath)
	if err != nil {
		return fmt.Errorf("reading context map: %w", err)
	}

	docsRoot := docsRootLabel(root)
	entry := fmt.Sprintf("\n### %s\n\n**Owns**: _(fill in)_\n\n**Domain docs**: [`%s/domains/%s/`](domains/%s/overview.md)\n",
		title, docsRoot, name, name)

	content := string(data)

	// Find the --- separator after Bounded Contexts section.
	// Insert the new entry before it.
	lines := strings.Split(content, "\n")
	scanner := bufio.NewScanner(strings.NewReader(content))
	_ = scanner // we'll use line-based approach

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
