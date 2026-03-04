package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindspec/mindspec/internal/phase"
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

	// Check lifecycle binding (ADR-0020) — warning only, not blocking
	checkLifecycleBinding(r, root, specID)
	checkSpecApprovalGateConsistency(specID)

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

// checkLifecycleBinding checks if the spec has a corresponding beads epic.
func checkLifecycleBinding(r *Result, root, specID string) {
	_, err := phase.FindEpicBySpecID(specID)
	if err != nil {
		r.AddWarning("lifecycle-binding", "spec has no beads epic; run spec approve to create one")
	}

	// Detect stale lifecycle.yaml files from pre-ADR-0023 repos.
	specDir := workspace.SpecDir(root, specID)
	lcPath := filepath.Join(specDir, "lifecycle.yaml")
	if _, statErr := os.Stat(lcPath); statErr == nil {
		r.AddWarning("stale-lifecycle", "stale lifecycle.yaml detected; lifecycle state is now derived from beads")
	}
}

func checkSpecApprovalGateConsistency(specID string) {
	// Derive phase from beads (ADR-0023).
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil {
		// No epic → can't check gate consistency
		return
	}
	derivedPhase, err := phase.DerivePhase(epicID)
	if err != nil {
		return
	}
	_ = derivedPhase // Gate consistency could be checked against spec frontmatter if needed
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
