package domain

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// BoundedContext represents a parsed entry from the context map's Bounded Contexts section.
type BoundedContext struct {
	Name string
	Owns string
}

// DomainEntry represents a domain for list output.
type DomainEntry struct {
	Name          string
	Owns          string
	Relationships []string
}

// ParseBoundedContexts reads the ## Bounded Contexts section from a context-map.md file.
func ParseBoundedContexts(path string) ([]BoundedContext, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var contexts []BoundedContext
	scanner := bufio.NewScanner(f)

	inSection := false
	var current *BoundedContext

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Detect ## headings
		if strings.HasPrefix(trimmed, "## ") {
			heading := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if strings.EqualFold(heading, "Bounded Contexts") {
				inSection = true
				continue
			}
			if inSection {
				// Hit next ## section, stop
				if current != nil {
					contexts = append(contexts, *current)
				}
				break
			}
			continue
		}

		if !inSection {
			continue
		}

		// --- separator ends the section
		if trimmed == "---" {
			if current != nil {
				contexts = append(contexts, *current)
			}
			break
		}

		// ### heading = new bounded context
		if strings.HasPrefix(trimmed, "### ") {
			if current != nil {
				contexts = append(contexts, *current)
			}
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			current = &BoundedContext{Name: name}
			continue
		}

		// **Owns**: line
		if current != nil && strings.HasPrefix(trimmed, "**Owns**:") {
			owns := strings.TrimPrefix(trimmed, "**Owns**:")
			current.Owns = strings.TrimSpace(owns)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return contexts, nil
}

// List returns all domains found under docs/domains/, enriched with
// ownership and relationship data from the context map.
func List(root string) ([]DomainEntry, error) {
	domainsDir := filepath.Join(workspace.DocsDir(root), "domains")
	entries, err := os.ReadDir(domainsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Parse context map for ownership
	cmPath := workspace.ContextMapPath(root)
	bcMap := make(map[string]string) // normalized name → owns
	if contexts, err := ParseBoundedContexts(cmPath); err == nil {
		for _, bc := range contexts {
			bcMap[normalizeDomain(bc.Name)] = bc.Owns
		}
	}

	// Parse relationships
	relMap := make(map[string][]string) // normalized name → relationship labels
	if rels, err := contextpack.ParseContextMap(cmPath); err == nil {
		for _, r := range rels {
			from := normalizeDomain(r.From)
			to := normalizeDomain(r.To)
			relMap[from] = appendUnique(relMap[from], fmt.Sprintf("→ %s (%s)", r.To, r.Direction))
			relMap[to] = appendUnique(relMap[to], fmt.Sprintf("← %s (%s)", r.From, r.Direction))
		}
	}

	var result []DomainEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		norm := normalizeDomain(name)
		result = append(result, DomainEntry{
			Name:          name,
			Owns:          bcMap[norm],
			Relationships: relMap[norm],
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// FormatTable formats domain entries as an aligned table.
func FormatTable(entries []DomainEntry) string {
	if len(entries) == 0 {
		return "No domains found.\n"
	}

	// Calculate column widths
	nameW, ownsW := len("Domain"), len("Owns")
	for _, e := range entries {
		if len(e.Name) > nameW {
			nameW = len(e.Name)
		}
		if len(e.Owns) > ownsW {
			ownsW = len(e.Owns)
		}
	}

	var b strings.Builder
	header := fmt.Sprintf("%-*s  %-*s  %s\n", nameW, "Domain", ownsW, "Owns", "Relationships")
	b.WriteString(header)
	b.WriteString(strings.Repeat("-", nameW))
	b.WriteString("  ")
	b.WriteString(strings.Repeat("-", ownsW))
	b.WriteString("  ")
	b.WriteString(strings.Repeat("-", 13))
	b.WriteString("\n")

	for _, e := range entries {
		rels := strings.Join(e.Relationships, ", ")
		b.WriteString(fmt.Sprintf("%-*s  %-*s  %s\n", nameW, e.Name, ownsW, e.Owns, rels))
	}

	return b.String()
}

func normalizeDomain(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
