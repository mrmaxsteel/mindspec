package bead

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SpecBeadResult holds the result of spec bead creation.
type SpecBeadResult struct {
	Bead   *BeadInfo
	GateID string // empty if gate creation was skipped or failed
}

// CreateSpecBead creates a spec bead from an approved specification.
// It is idempotent: if a bead with the [SPEC <specID>] prefix already exists, it returns that.
// Also creates a human gate bead [GATE spec-approve <specID>] as a child of the spec bead.
func CreateSpecBead(root, specID string) (*SpecBeadResult, error) {
	specPath := filepath.Join(root, "docs", "specs", specID, "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read spec: %v", err)
	}
	content := string(data)

	// Validate approval
	if !isSpecApproved(content) {
		return nil, fmt.Errorf("spec %s is not approved (expected 'Status: APPROVED' in Approval section)", specID)
	}

	// Extract metadata
	title := extractSpecTitle(content)
	goal := extractGoalSummary(content)
	domains := extractDomains(content)

	// Build structured description (capped at 400 chars)
	desc := buildSpecDescription(goal, specID, domains)

	// Idempotent lookup for spec bead
	prefix := fmt.Sprintf("[SPEC %s]", specID)
	var specBead *BeadInfo
	existing, err := Search(prefix)
	if err == nil && len(existing) > 0 {
		specBead = &existing[0]
	} else {
		// Create new bead
		beadTitle := fmt.Sprintf("%s %s", prefix, title)
		specBead, err = Create(beadTitle, desc, "feature", 2, "")
		if err != nil {
			return nil, err
		}
	}

	// Create or find the spec approval gate
	gateTitle := SpecGateTitle(specID)
	gate, err := FindOrCreateGate(gateTitle, specBead.ID)
	var gateID string
	if err != nil {
		// Gate creation is best-effort — warn but don't fail
		fmt.Fprintf(os.Stderr, "warning: could not create spec gate: %v\n", err)
	} else {
		gateID = gate.ID
	}

	return &SpecBeadResult{
		Bead:   specBead,
		GateID: gateID,
	}, nil
}

// isSpecApproved checks for approval status in the spec content.
// Handles both `Status: APPROVED` and `**Status**: APPROVED` formats.
func isSpecApproved(content string) bool {
	// Look in the Approval section
	approvalIdx := strings.Index(content, "## Approval")
	if approvalIdx == -1 {
		return false
	}
	approvalSection := content[approvalIdx:]

	// Check for next section boundary
	if nextIdx := strings.Index(approvalSection[len("## Approval"):], "\n## "); nextIdx != -1 {
		approvalSection = approvalSection[:len("## Approval")+nextIdx]
	}

	// Check various formats
	lower := strings.ToLower(approvalSection)
	return strings.Contains(lower, "status: approved") ||
		strings.Contains(lower, "status**: approved")
}

// extractSpecTitle extracts the title from the `# Spec NNN: <title>` heading.
func extractSpecTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "# Spec ") || strings.HasPrefix(line, "# ") {
			// Try "# Spec NNN: Title" format
			if idx := strings.Index(line, ": "); idx != -1 {
				return strings.TrimSpace(line[idx+2:])
			}
			// Fallback: use the heading as-is minus the #
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return "Untitled spec"
}

// extractGoalSummary extracts a summary from the ## Goal section.
// Returns first sentence or up to 120 chars.
func extractGoalSummary(content string) string {
	goalIdx := strings.Index(content, "## Goal")
	if goalIdx == -1 {
		return ""
	}
	section := content[goalIdx+len("## Goal"):]

	// Find next section or end
	if nextIdx := strings.Index(section, "\n## "); nextIdx != -1 {
		section = section[:nextIdx]
	}

	// Get the first non-empty line as the summary
	var summary string
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			summary = trimmed
			break
		}
	}

	// Truncate at first sentence or 120 chars
	if idx := strings.Index(summary, ". "); idx != -1 && idx < 120 {
		summary = summary[:idx+1]
	}
	if len(summary) > 120 {
		summary = summary[:117] + "..."
	}

	return summary
}

// extractDomains extracts domain names from the ## Impacted Domains section.
func extractDomains(content string) string {
	domainIdx := strings.Index(content, "## Impacted Domains")
	if domainIdx == -1 {
		return ""
	}
	section := content[domainIdx+len("## Impacted Domains"):]
	if nextIdx := strings.Index(section, "\n## "); nextIdx != -1 {
		section = section[:nextIdx]
	}

	var domains []string
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			// Extract domain name (could be `- **domain**: desc` or `- domain: desc`)
			entry := strings.TrimPrefix(trimmed, "- ")
			entry = strings.TrimPrefix(entry, "**")
			if idx := strings.Index(entry, "**"); idx != -1 {
				entry = entry[:idx]
			} else if idx := strings.Index(entry, ":"); idx != -1 {
				entry = entry[:idx]
			}
			entry = strings.TrimSpace(entry)
			if entry != "" {
				domains = append(domains, entry)
			}
		}
	}

	return strings.Join(domains, ", ")
}

// buildSpecDescription creates a structured description capped at 400 chars.
func buildSpecDescription(goal, specID, domains string) string {
	desc := fmt.Sprintf("Summary: %s\nSpec: docs/specs/%s/spec.md", goal, specID)
	if domains != "" {
		desc += fmt.Sprintf("\nDomains: %s", domains)
	}

	if len(desc) > 400 {
		desc = desc[:397] + "..."
	}
	return desc
}
