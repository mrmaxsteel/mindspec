package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// RelInfo represents a relationship to another domain.
type RelInfo struct {
	Domain    string `json:"domain"`
	Direction string `json:"direction"`
}

// DomainInfo holds the detailed view of a single domain.
type DomainInfo struct {
	Name          string    `json:"name"`
	Owns          string    `json:"owns"`
	Boundaries    string    `json:"boundaries"`
	KeyFiles      string    `json:"key_files"`
	Relationships []RelInfo `json:"relationships"`
	Specs         []string  `json:"specs"`
}

// Show returns detailed information about a single domain.
func Show(root, name string) (*DomainInfo, error) {
	domainDir := workspace.DomainDir(root, name)
	if _, err := os.Stat(domainDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("domain %q does not exist", name)
	}

	info := &DomainInfo{Name: name}

	// Read overview.md and extract sections
	overviewPath := filepath.Join(domainDir, "overview.md")
	if data, err := os.ReadFile(overviewPath); err == nil {
		content := string(data)
		info.Owns = extractSection(content, "What This Domain Owns")
		info.Boundaries = extractSection(content, "Boundaries")
		info.KeyFiles = extractSection(content, "Key Files")
	}

	// Parse relationships from context map
	cmPath := workspace.ContextMapPath(root)
	if rels, err := contextpack.ParseContextMap(cmPath); err == nil {
		norm := normalizeDomain(name)
		for _, r := range rels {
			from := normalizeDomain(r.From)
			to := normalizeDomain(r.To)
			if from == norm {
				info.Relationships = append(info.Relationships, RelInfo{
					Domain:    r.To,
					Direction: fmt.Sprintf("→ %s", r.Direction),
				})
			}
			if to == norm {
				info.Relationships = append(info.Relationships, RelInfo{
					Domain:    r.From,
					Direction: fmt.Sprintf("← %s", r.Direction),
				})
			}
		}
	}

	// Scan specs for impacted domains
	specsDir := filepath.Join(workspace.DocsDir(root), "specs")
	if entries, err := os.ReadDir(specsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			specDir := filepath.Join(specsDir, e.Name())
			meta, err := contextpack.ParseSpec(specDir)
			if err != nil {
				continue
			}
			for _, d := range meta.Domains {
				cleaned := strings.Trim(d, "*")
				if normalizeDomain(cleaned) == normalizeDomain(name) {
					info.Specs = append(info.Specs, meta.SpecID)
					break
				}
			}
		}
	}

	return info, nil
}

// extractSection extracts the content under a markdown ## heading
// until the next heading of the same or higher level.
func extractSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	inSection := false
	var sectionLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			h := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if strings.EqualFold(h, heading) {
				inSection = true
				continue
			}
			if inSection {
				break
			}
			continue
		}

		// # heading also ends section
		if inSection && strings.HasPrefix(trimmed, "# ") {
			break
		}

		if inSection {
			sectionLines = append(sectionLines, line)
		}
	}

	return strings.TrimSpace(strings.Join(sectionLines, "\n"))
}

// FormatSummary formats a DomainInfo as plain text.
func FormatSummary(info *DomainInfo) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Domain: %s\n\n", info.Name))

	if info.Owns != "" {
		b.WriteString(fmt.Sprintf("Owns:\n%s\n\n", indent(info.Owns)))
	}

	if info.Boundaries != "" {
		b.WriteString(fmt.Sprintf("Boundaries:\n%s\n\n", indent(info.Boundaries)))
	}

	if len(info.Relationships) > 0 {
		b.WriteString("Relationships:\n")
		for _, r := range info.Relationships {
			b.WriteString(fmt.Sprintf("  %s %s\n", r.Direction, r.Domain))
		}
		b.WriteString("\n")
	}

	if info.KeyFiles != "" {
		b.WriteString(fmt.Sprintf("Key Files:\n%s\n\n", indent(info.KeyFiles)))
	}

	if len(info.Specs) > 0 {
		b.WriteString("Specs:\n")
		for _, s := range info.Specs {
			b.WriteString(fmt.Sprintf("  - %s\n", s))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// FormatJSON formats a DomainInfo as indented JSON.
func FormatJSON(info *DomainInfo) (string, error) {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
