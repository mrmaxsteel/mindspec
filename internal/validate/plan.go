package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PlanFrontmatter represents the YAML frontmatter of a plan.md file.
type PlanFrontmatter struct {
	Status       string        `yaml:"status"`
	SpecID       string        `yaml:"spec_id"`
	Version      string        `yaml:"version"`
	LastUpdated  string        `yaml:"last_updated"`
	ApprovedAt   string        `yaml:"approved_at"`
	ApprovedBy   string        `yaml:"approved_by"`
	BeadIDs      []string      `yaml:"bead_ids"`
	ADRCitations []ADRCitation `yaml:"adr_citations"`
}

// ADRCitation represents an ADR citation in plan frontmatter.
type ADRCitation struct {
	ID       string   `yaml:"id"`
	Sections []string `yaml:"sections"`
}

// ValidatePlan checks the structural quality of a plan.md file.
func ValidatePlan(root, specID string) *Result {
	r := &Result{SubCommand: "plan", TargetID: specID}

	planPath := filepath.Join(root, "docs", "specs", specID, "plan.md")
	data, err := os.ReadFile(planPath)
	if err != nil {
		r.AddError("plan-file", fmt.Sprintf("cannot read plan: %v", err))
		return r
	}

	content := string(data)

	// Parse frontmatter
	fm, err := parsePlanFrontmatter(content)
	if err != nil {
		r.AddError("frontmatter-parse", fmt.Sprintf("cannot parse YAML frontmatter: %v", err))
		return r
	}

	// Check required frontmatter fields
	checkFrontmatterFields(r, fm)

	// Check ADR citations
	if len(fm.ADRCitations) == 0 {
		r.AddWarning("adr-citations", "no ADR citations in frontmatter")
	}

	// Check bead IDs exist in Beads
	checkBeadIDs(r, fm.BeadIDs)

	// Parse and check bead sections
	beadSections := parseBeadSections(content)
	if len(beadSections) == 0 {
		r.AddError("bead-sections", "no bead sections found (expected ## Bead ... headings)")
		return r
	}

	for _, bs := range beadSections {
		checkBeadSection(r, bs)
	}

	return r
}

// parsePlanFrontmatter extracts and parses YAML frontmatter from plan content.
func parsePlanFrontmatter(content string) (*PlanFrontmatter, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("no frontmatter found (expected leading ---)")
	}

	var fmLines []string
	found := false
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			found = true
			break
		}
		fmLines = append(fmLines, line)
	}

	if !found {
		return nil, fmt.Errorf("unclosed frontmatter (missing closing ---)")
	}

	// Filter out commented lines (# prefix)
	var activeFmLines []string
	for _, line := range fmLines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			activeFmLines = append(activeFmLines, line)
		}
	}

	fmContent := strings.Join(activeFmLines, "\n")

	var fm PlanFrontmatter
	if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
		return nil, err
	}

	return &fm, nil
}

// checkFrontmatterFields verifies required fields are present.
func checkFrontmatterFields(r *Result, fm *PlanFrontmatter) {
	if fm.Status == "" {
		r.AddError("frontmatter-status", "missing required field: status")
	}
	if fm.SpecID == "" {
		r.AddError("frontmatter-spec-id", "missing required field: spec_id")
	}
	if fm.Version == "" {
		r.AddError("frontmatter-version", "missing required field: version")
	}
}

// beadSection represents a parsed bead section from a plan.
type beadSection struct {
	heading      string
	stepsCount   int
	hasVerify    bool
	verifyCount  int
	hasDependsOn bool
}

// parseBeadSections finds and parses ## Bead ... sections.
func parseBeadSections(content string) []beadSection {
	var sections []beadSection
	lines := strings.Split(content, "\n")

	var current *beadSection
	inSteps := false
	inVerify := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(line, "## Bead ") {
			if current != nil {
				sections = append(sections, *current)
			}
			current = &beadSection{heading: strings.TrimPrefix(line, "## ")}
			inSteps = false
			inVerify = false
			continue
		}

		if current == nil {
			continue
		}

		// Detect section markers within a bead
		if strings.HasPrefix(trimmed, "**Steps**") {
			inSteps = true
			inVerify = false
			continue
		}
		if strings.HasPrefix(trimmed, "**Verification**") {
			inVerify = true
			inSteps = false
			current.hasVerify = true
			continue
		}
		if strings.HasPrefix(trimmed, "**Depends on**") {
			current.hasDependsOn = true
			inSteps = false
			inVerify = false
			continue
		}
		if strings.HasPrefix(trimmed, "**Scope**") {
			inSteps = false
			inVerify = false
			continue
		}

		// New top-level section ends bead
		if strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "## Bead ") {
			if current != nil {
				sections = append(sections, *current)
				current = nil
			}
			inSteps = false
			inVerify = false
			continue
		}

		// Count items
		if inSteps && len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && trimmed[1] == '.' {
			current.stepsCount++
		}
		if inVerify && (strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]")) {
			current.verifyCount++
		}
	}

	if current != nil {
		sections = append(sections, *current)
	}

	return sections
}

// checkBeadSection validates a single bead section.
func checkBeadSection(r *Result, bs beadSection) {
	if bs.stepsCount < 3 {
		r.AddError("bead-steps", fmt.Sprintf("%s: expected 3-7 steps, found %d", bs.heading, bs.stepsCount))
	} else if bs.stepsCount > 7 {
		r.AddWarning("bead-steps", fmt.Sprintf("%s: has %d steps (recommended 3-7)", bs.heading, bs.stepsCount))
	}

	if !bs.hasVerify || bs.verifyCount == 0 {
		r.AddError("bead-verification", fmt.Sprintf("%s: missing verification steps", bs.heading))
	}

	if !bs.hasDependsOn {
		r.AddWarning("bead-depends", fmt.Sprintf("%s: no 'Depends on' declaration", bs.heading))
	}
}
