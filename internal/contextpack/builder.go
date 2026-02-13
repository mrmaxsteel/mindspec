package contextpack

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Mode constants for context pack generation.
const (
	ModeSpec      = "spec"
	ModePlan      = "plan"
	ModeImplement = "implement"
)

// ProvenanceEntry records why a piece of content was included.
type ProvenanceEntry struct {
	Source  string
	Section string
	Reason  string
}

// ContextPack is the assembled context for a spec.
type ContextPack struct {
	SpecID     string
	Mode       string
	CommitSHA  string
	GeneratedAt string
	Goal       string
	Domains    []string
	Neighbors  []string
	Sections   []PackSection
	Provenance []ProvenanceEntry
}

// PackSection represents a titled section within a context pack.
type PackSection struct {
	Heading string
	Content string
}

// Build assembles a context pack for the given spec and mode.
func Build(root, specID, mode string) (*ContextPack, error) {
	specDir := filepath.Join(root, "docs", "specs", specID)

	// Parse spec
	meta, err := ParseSpec(specDir)
	if err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", specID, err)
	}

	pack := &ContextPack{
		SpecID:      specID,
		Mode:        mode,
		CommitSHA:   gitCommitSHA(root),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Goal:        meta.Goal,
		Domains:     meta.Domains,
	}

	// Read domain docs for impacted domains
	for _, domain := range meta.Domains {
		doc, err := ReadDomainDocs(root, domain)
		if err != nil {
			return nil, fmt.Errorf("reading domain docs for %q: %w", domain, err)
		}
		addDomainSections(pack, doc, mode, false)
	}

	// Parse context map and resolve neighbors
	cmPath := filepath.Join(root, "docs", "context-map.md")
	rels, err := ParseContextMap(cmPath)
	if err != nil {
		// Context map is optional; log but don't fail
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("parsing context map: %w", err)
		}
	} else {
		pack.Neighbors = ResolveNeighbors(rels, meta.Domains)

		// For plan and implement modes, include neighbor interfaces
		if mode == ModePlan || mode == ModeImplement {
			for _, neighbor := range pack.Neighbors {
				doc, err := ReadDomainDocs(root, neighbor)
				if err != nil {
					continue
				}
				if doc.Interfaces != "" {
					pack.Sections = append(pack.Sections, PackSection{
						Heading: fmt.Sprintf("Neighbor Domain: %s — Interfaces", neighbor),
						Content: doc.Interfaces,
					})
					pack.Provenance = append(pack.Provenance, ProvenanceEntry{
						Source:  fmt.Sprintf("docs/domains/%s/interfaces.md", neighbor),
						Section: "Neighbor Interfaces",
						Reason:  "1-hop neighbor via Context Map",
					})
				}
			}
		}
	}

	// Scan and filter ADRs
	adrs, err := ScanADRs(root)
	if err != nil {
		return nil, fmt.Errorf("scanning ADRs: %w", err)
	}

	// Collect all relevant domains (impacted + neighbors for broader ADR coverage)
	relevantDomains := make([]string, len(meta.Domains))
	copy(relevantDomains, meta.Domains)

	filtered := FilterADRs(adrs, relevantDomains)
	for _, adr := range filtered {
		pack.Sections = append(pack.Sections, PackSection{
			Heading: fmt.Sprintf("ADR: %s", adr.ID),
			Content: adr.Content,
		})
		pack.Provenance = append(pack.Provenance, ProvenanceEntry{
			Source:  fmt.Sprintf("docs/adr/%s.md", adr.ID),
			Section: adr.ID,
			Reason:  fmt.Sprintf("Accepted ADR for domains: %s", strings.Join(adr.Domains, ", ")),
		})
	}

	// Parse and filter policies
	polPath := filepath.Join(root, "architecture", "policies.yml")
	policies, err := ParsePolicies(polPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("parsing policies: %w", err)
		}
	} else {
		filteredPolicies := FilterPolicies(policies, mode)
		if len(filteredPolicies) > 0 {
			pack.Sections = append(pack.Sections, PackSection{
				Heading: "Applicable Policies",
				Content: renderPoliciesTable(filteredPolicies),
			})
			pack.Provenance = append(pack.Provenance, ProvenanceEntry{
				Source:  "architecture/policies.yml",
				Section: "Policies",
				Reason:  fmt.Sprintf("Policies applicable to mode %q", mode),
			})
		}
	}

	return pack, nil
}

// addDomainSections adds domain doc sections based on mode tier rules.
func addDomainSections(pack *ContextPack, doc *DomainDoc, mode string, isNeighbor bool) {
	domain := doc.Domain

	// All modes: overview
	if doc.Overview != "" {
		pack.Sections = append(pack.Sections, PackSection{
			Heading: fmt.Sprintf("Domain: %s — Overview", domain),
			Content: doc.Overview,
		})
		pack.Provenance = append(pack.Provenance, ProvenanceEntry{
			Source:  fmt.Sprintf("docs/domains/%s/overview.md", domain),
			Section: "Overview",
			Reason:  "Impacted domain overview",
		})
	}

	// Plan + Implement: architecture
	if (mode == ModePlan || mode == ModeImplement) && doc.Architecture != "" {
		pack.Sections = append(pack.Sections, PackSection{
			Heading: fmt.Sprintf("Domain: %s — Architecture", domain),
			Content: doc.Architecture,
		})
		pack.Provenance = append(pack.Provenance, ProvenanceEntry{
			Source:  fmt.Sprintf("docs/domains/%s/architecture.md", domain),
			Section: "Architecture",
			Reason:  "Impacted domain architecture (plan/implement tier)",
		})
	}

	// Implement only: interfaces + runbook
	if mode == ModeImplement {
		if doc.Interfaces != "" {
			pack.Sections = append(pack.Sections, PackSection{
				Heading: fmt.Sprintf("Domain: %s — Interfaces", domain),
				Content: doc.Interfaces,
			})
			pack.Provenance = append(pack.Provenance, ProvenanceEntry{
				Source:  fmt.Sprintf("docs/domains/%s/interfaces.md", domain),
				Section: "Interfaces",
				Reason:  "Impacted domain interfaces (implement tier)",
			})
		}
		if doc.Runbook != "" {
			pack.Sections = append(pack.Sections, PackSection{
				Heading: fmt.Sprintf("Domain: %s — Runbook", domain),
				Content: doc.Runbook,
			})
			pack.Provenance = append(pack.Provenance, ProvenanceEntry{
				Source:  fmt.Sprintf("docs/domains/%s/runbook.md", domain),
				Section: "Runbook",
				Reason:  "Impacted domain runbook (implement tier)",
			})
		}
	}
}

// Render produces the markdown content for the context pack.
func (cp *ContextPack) Render() string {
	var b strings.Builder

	b.WriteString("# Context Pack\n\n")
	b.WriteString(fmt.Sprintf("- **Spec**: %s\n", cp.SpecID))
	b.WriteString(fmt.Sprintf("- **Mode**: %s\n", cp.Mode))
	b.WriteString(fmt.Sprintf("- **Commit**: %s\n", cp.CommitSHA))
	b.WriteString(fmt.Sprintf("- **Generated**: %s\n", cp.GeneratedAt))
	b.WriteString("\n---\n\n")

	// Goal
	b.WriteString("## Goal\n\n")
	b.WriteString(cp.Goal)
	b.WriteString("\n\n")

	// Impacted domains
	b.WriteString("## Impacted Domains\n\n")
	for _, d := range cp.Domains {
		b.WriteString(fmt.Sprintf("- %s\n", d))
	}
	b.WriteString("\n")

	if len(cp.Neighbors) > 0 {
		b.WriteString("## 1-Hop Neighbors\n\n")
		for _, n := range cp.Neighbors {
			b.WriteString(fmt.Sprintf("- %s\n", n))
		}
		b.WriteString("\n")
	}

	// Content sections
	for _, s := range cp.Sections {
		b.WriteString(fmt.Sprintf("## %s\n\n", s.Heading))
		b.WriteString(s.Content)
		if !strings.HasSuffix(s.Content, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Provenance
	b.WriteString("---\n\n")
	b.WriteString("## Provenance\n\n")
	b.WriteString("| Source | Section | Reason |\n")
	b.WriteString("|:-------|:--------|:-------|\n")
	for _, p := range cp.Provenance {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", p.Source, p.Section, p.Reason))
	}

	return b.String()
}

// WriteToFile writes the rendered context pack to the spec directory.
func (cp *ContextPack) WriteToFile(root, specID string) error {
	outDir := filepath.Join(root, "docs", "specs", specID)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}
	outPath := filepath.Join(outDir, "context-pack.md")
	return os.WriteFile(outPath, []byte(cp.Render()), 0o644)
}

func renderPoliciesTable(policies []Policy) string {
	var b strings.Builder
	b.WriteString("| ID | Severity | Description | Reference |\n")
	b.WriteString("|:---|:---------|:------------|:----------|\n")
	for _, p := range policies {
		ref := p.Reference
		if ref == "" {
			ref = "—"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", p.ID, p.Severity, p.Description, ref))
	}
	return b.String()
}

func gitCommitSHA(root string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
