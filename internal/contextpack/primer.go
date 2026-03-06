package contextpack

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// runBDFn is a package-level variable for testability.
var runBDFn = bead.RunBD

// BeadPrimer holds the focused context for a single implementation bead.
type BeadPrimer struct {
	BeadID             string
	BeadTitle          string
	BeadDescription    string
	SpecID             string
	Requirements       string
	AcceptanceCriteria string
	PlanWorkChunk      string
	FilePaths          []string
	ADRDecisions       []ADRDecision
	DomainOverviews    []DomainOverview
	EstimatedTokens    int
}

// ADRDecision holds the ID and decision text from an ADR.
type ADRDecision struct {
	ID       string
	Decision string
}

// DomainOverview holds a domain name and its overview text.
type DomainOverview struct {
	Domain   string
	Overview string
}

// BuildBeadPrimer assembles a focused context primer for a specific implementation bead.
func BuildBeadPrimer(root, specID, beadID string) (*BeadPrimer, error) {
	p := &BeadPrimer{
		BeadID: beadID,
		SpecID: specID,
	}

	// 1. Get bead info
	out, err := runBDFn("show", beadID, "--json")
	if err != nil {
		return nil, fmt.Errorf("fetching bead %s: %w", beadID, err)
	}
	var beadInfos []bead.BeadInfo
	if err := json.Unmarshal(out, &beadInfos); err != nil {
		return nil, fmt.Errorf("parsing bead info: %w", err)
	}
	if len(beadInfos) > 0 {
		p.BeadTitle = beadInfos[0].Title
		p.BeadDescription = beadInfos[0].Description
	}

	specDir := workspace.SpecDir(root, specID)

	// 2. Extract Requirements and Acceptance Criteria from spec.md
	specContent := readFileContent(filepath.Join(specDir, "spec.md"))
	if specContent != "" {
		p.Requirements = ExtractSection(specContent, "Requirements")
		p.AcceptanceCriteria = ExtractSection(specContent, "Acceptance Criteria")
	}

	// 3. Extract plan work chunk matching this bead
	planContent := readFileContent(filepath.Join(specDir, "plan.md"))
	if planContent != "" {
		p.PlanWorkChunk = extractBeadSection(planContent, p.BeadTitle)
	}

	// 4. Extract file paths from the plan work chunk
	if p.PlanWorkChunk != "" {
		p.FilePaths = ExtractFilePathsFromText(p.PlanWorkChunk)
	}

	// 5. Scan ADRs and extract decision sections
	specMeta, _ := ParseSpec(specDir)
	if specMeta != nil && len(specMeta.Domains) > 0 {
		allADRs, _ := adr.ScanADRs(root)
		filtered := adr.FilterADRs(allADRs, specMeta.Domains)
		for _, a := range filtered {
			decision := ExtractSection(a.Content, "Decision")
			if decision != "" {
				p.ADRDecisions = append(p.ADRDecisions, ADRDecision{
					ID:       a.ID,
					Decision: decision,
				})
			}
		}

		// 6. Read domain overviews (overview only)
		for _, domain := range specMeta.Domains {
			doc, err := ReadDomainDocs(root, domain)
			if err != nil || doc.Overview == "" {
				continue
			}
			// Extract just the first paragraph of overview for brevity
			overview := doc.Overview
			if idx := strings.Index(overview, "\n\n"); idx > 0 {
				overview = overview[:idx]
			}
			p.DomainOverviews = append(p.DomainOverviews, DomainOverview{
				Domain:   domain,
				Overview: strings.TrimSpace(overview),
			})
		}
	}

	// 7. Estimate tokens
	rendered := RenderBeadPrimer(p)
	p.EstimatedTokens = len(rendered) / 4

	return p, nil
}

// RenderBeadPrimer produces markdown output for a bead primer.
func RenderBeadPrimer(p *BeadPrimer) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Bead Context: %s\n", p.BeadTitle))
	b.WriteString(fmt.Sprintf("**Spec**: %s | **Bead**: %s", p.SpecID, p.BeadID))
	if p.EstimatedTokens > 0 {
		b.WriteString(fmt.Sprintf(" | **~%d tokens**", p.EstimatedTokens))
	}
	b.WriteString("\n\n")

	if p.BeadDescription != "" {
		b.WriteString("## Scope\n\n")
		b.WriteString(p.BeadDescription)
		b.WriteString("\n\n")
	}

	if p.Requirements != "" {
		b.WriteString("## Requirements\n\n")
		b.WriteString(p.Requirements)
		b.WriteString("\n\n")
	}

	if p.AcceptanceCriteria != "" {
		b.WriteString("## Acceptance Criteria\n\n")
		b.WriteString(p.AcceptanceCriteria)
		b.WriteString("\n\n")
	}

	if p.PlanWorkChunk != "" {
		b.WriteString("## Work Chunk\n\n")
		b.WriteString(p.PlanWorkChunk)
		b.WriteString("\n\n")
	}

	if len(p.FilePaths) > 0 {
		b.WriteString("## Key File Paths\n\n")
		for _, fp := range p.FilePaths {
			b.WriteString(fmt.Sprintf("- %s\n", fp))
		}
		b.WriteString("\n")
	}

	if len(p.ADRDecisions) > 0 {
		b.WriteString("## ADR Decisions\n\n")
		for _, d := range p.ADRDecisions {
			b.WriteString(fmt.Sprintf("- **%s**: %s\n", d.ID, d.Decision))
		}
		b.WriteString("\n")
	}

	if len(p.DomainOverviews) > 0 {
		b.WriteString("## Domain Context\n\n")
		for _, d := range p.DomainOverviews {
			b.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", d.Domain, d.Overview))
		}
	}

	return b.String()
}

// extractBeadSection finds a ## Bead ... section in plan content that matches
// the bead title. Returns the full section content including subsections.
func extractBeadSection(content, beadTitle string) string {
	lines := strings.Split(content, "\n")
	var collecting bool
	var result []string

	// Try to match by bead title substring (strip [SPEC ...] prefix if present)
	searchTitle := beadTitle
	if idx := strings.Index(beadTitle, "] "); idx >= 0 {
		searchTitle = strings.TrimSpace(beadTitle[idx+2:])
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "## Bead ") {
			if collecting {
				break
			}
			heading := strings.TrimPrefix(line, "## ")
			if strings.Contains(heading, searchTitle) || strings.Contains(searchTitle, strings.TrimPrefix(heading, "Bead ")) {
				collecting = true
				continue
			}
		}
		// Another top-level ## section ends collection
		if collecting && strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "## Bead ") {
			break
		}
		if collecting {
			result = append(result, line)
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}
