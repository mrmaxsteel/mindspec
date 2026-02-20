package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindspec/mindspec/internal/specmeta"
	"github.com/mindspec/mindspec/internal/workspace"
)

// requiredSpecSections lists sections that must exist and have content.
var requiredSpecSections = []string{
	"Goal",
	"Impacted Domains",
	"ADR Touchpoints",
	"Requirements",
	"Scope",
	"Acceptance Criteria",
	"Approval",
}

// ValidateSpec checks the structural quality of a spec.md file.
func ValidateSpec(root, specID string) *Result {
	r := &Result{SubCommand: "spec", TargetID: specID}

	specPath := filepath.Join(workspace.SpecDir(root, specID), "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		r.AddError("spec-file", fmt.Sprintf("cannot read spec: %v", err))
		return r
	}

	content := string(data)
	sections := parseSections(content)

	// Check required sections exist and have content
	for _, name := range requiredSpecSections {
		body, exists := sections[name]
		if !exists {
			r.AddError("section-missing", fmt.Sprintf("required section missing: ## %s", name))
			continue
		}
		trimmed := strings.TrimSpace(body)
		if trimmed == "" || isPlaceholder(trimmed) {
			r.AddError("section-empty", fmt.Sprintf("section has no content or only placeholder text: ## %s", name))
		}
	}

	// Check In Scope and Out of Scope subsections
	checkScopeSubsections(r, sections)

	// Check requirements count
	checkRequirementsCount(r, sections)

	// Check acceptance criteria
	checkAcceptanceCriteria(r, sections)

	// Check open questions resolved
	checkOpenQuestions(r, sections)

	// Check molecule binding (ADR-0015) — warning only, not blocking
	checkMoleculeBinding(r, root, specID)
	checkSpecApprovalGateConsistency(r, root, specID)

	return r
}

// parseSections extracts markdown sections keyed by heading text.
// Only handles ## level headings.
func parseSections(content string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentHeading string
	var currentBody []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Save previous section
			if currentHeading != "" {
				sections[currentHeading] = strings.Join(currentBody, "\n")
			}
			currentHeading = strings.TrimPrefix(line, "## ")
			currentHeading = strings.TrimSpace(currentHeading)
			currentBody = nil
		} else if currentHeading != "" {
			currentBody = append(currentBody, line)
		}
	}
	// Save last section
	if currentHeading != "" {
		sections[currentHeading] = strings.Join(currentBody, "\n")
	}

	return sections
}

// isPlaceholder returns true if text looks like an unfilled template.
func isPlaceholder(text string) bool {
	placeholders := []string{
		"<Brief description",
		"<Context, motivation",
		"<domain-1>",
		"<Requirement 1>",
		"<File or component 1>",
		"<Explicitly excluded",
		"<Specific, measurable criterion",
		"<Question that must",
		"<command 1>",
	}
	for _, p := range placeholders {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

// checkScopeSubsections verifies In Scope and Out of Scope are present.
func checkScopeSubsections(r *Result, sections map[string]string) {
	scope, exists := sections["Scope"]
	if !exists {
		return // already flagged by required section check
	}

	if !strings.Contains(scope, "### In Scope") {
		r.AddError("scope-in-missing", "Scope section missing '### In Scope' subsection")
	}
	if !strings.Contains(scope, "### Out of Scope") {
		r.AddError("scope-out-missing", "Scope section missing '### Out of Scope' subsection")
	}
}

// checkRequirementsCount verifies at least 2 requirements are listed.
func checkRequirementsCount(r *Result, sections map[string]string) {
	reqs, exists := sections["Requirements"]
	if !exists {
		return
	}

	count := 0
	for _, line := range strings.Split(reqs, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && trimmed[1] == '.' {
			count++
		}
	}

	if count < 2 {
		r.AddError("requirements-count", fmt.Sprintf("expected at least 2 requirements, found %d", count))
	}
}

// checkAcceptanceCriteria validates criteria count and quality.
func checkAcceptanceCriteria(r *Result, sections map[string]string) {
	criteria, exists := sections["Acceptance Criteria"]
	if !exists {
		return
	}

	var criteriaLines []string
	for _, line := range strings.Split(criteria, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") {
			criteriaLines = append(criteriaLines, trimmed)
		}
	}

	if len(criteriaLines) < 3 {
		r.AddError("criteria-count", fmt.Sprintf("expected at least 3 acceptance criteria, found %d", len(criteriaLines)))
	}

	for _, line := range criteriaLines {
		if IsVagueCriterion(line) {
			r.AddWarning("criteria-vague", fmt.Sprintf("possibly vague criterion: %s", strings.TrimSpace(line)))
		}
	}
}

// checkMoleculeBinding warns if the spec lacks a molecule_id in its frontmatter.
func checkMoleculeBinding(r *Result, root, specID string) {
	m, err := specmeta.ReadForSpec(root, specID)
	if err != nil {
		return // can't read → skip this check
	}
	if m.MoleculeID == "" {
		r.AddWarning("molecule-binding", "spec has no molecule_id in frontmatter; run backfill or re-init")
	}
}

func checkSpecApprovalGateConsistency(r *Result, root, specID string) {
	m, err := specmeta.ReadForSpec(root, specID)
	if err != nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(m.Status), "Approved") {
		return
	}

	gateID := strings.TrimSpace(m.StepMapping["spec-approve"])
	if gateID == "" {
		r.AddWarning("spec-gate-consistency", "spec frontmatter status is Approved but step_mapping.spec-approve is missing")
		return
	}

	status, err := readGateStatus(gateID)
	if err != nil {
		r.AddWarning("spec-gate-consistency", fmt.Sprintf("spec frontmatter status is Approved but gate %s could not be verified: %v", gateID, err))
		return
	}
	if status != "closed" {
		r.AddWarning("spec-gate-consistency", fmt.Sprintf("spec frontmatter status is Approved but gate %s is %s", gateID, status))
	}
}

// checkOpenQuestions verifies all open questions are resolved.
func checkOpenQuestions(r *Result, sections map[string]string) {
	oq, exists := sections["Open Questions"]
	if !exists {
		return
	}

	trimmed := strings.TrimSpace(oq)

	// "None" variants mean resolved
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "none") {
		return
	}

	// Check for unchecked checkboxes
	for _, line := range strings.Split(oq, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "- [ ]") {
			r.AddError("open-question", fmt.Sprintf("unresolved open question: %s", t))
		}
	}
}
