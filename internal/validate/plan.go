package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
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

	planPath := filepath.Join(workspace.SpecDir(root, specID), "plan.md")
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

	// Backwards compatibility: skip new quality gates for already-approved plans
	isApproved := strings.EqualFold(fm.Status, "Approved")

	hasADRFitness := hasSection(content, "## ADR Fitness")

	// Check ADR citations + fitness (Spec 039)
	if len(fm.ADRCitations) == 0 {
		if isApproved {
			// skip for approved plans
		} else if hasADRFitness {
			r.AddWarning("adr-citations", "no ADR citations in frontmatter (ADR Fitness section explains why)")
		} else {
			r.AddError("adr-citations", "no ADR citations in frontmatter and no ## ADR Fitness section — plan shows no evidence of architectural evaluation")
		}
	} else {
		checkADRCitations(r, root, fm.ADRCitations)
	}

	// Check ADR Fitness section (Spec 039: promoted to error)
	if !hasADRFitness && !isApproved {
		r.AddError("adr-fitness-missing", "plan must include an ## ADR Fitness section documenting evaluation of relevant ADRs")
	}

	// Check Testing Strategy section (Spec 039: new check)
	if !hasSection(content, "## Testing Strategy") && !isApproved {
		r.AddError("testing-strategy-missing", "plan must include a ## Testing Strategy section declaring the test approach")
	}

	// Check Provenance section (Spec 039: new check, warning only)
	if !hasSection(content, "## Provenance") && !isApproved {
		r.AddWarning("provenance-missing", "plan should include a ## Provenance section mapping spec acceptance criteria to bead verification")
	}

	// Check bead IDs exist in Beads
	checkBeadIDs(r, fm.BeadIDs)
	checkPlanApprovalGateConsistency(r, specID, fm)

	// Parse and check bead sections
	beadSections := ParseBeadSections(content)
	if len(beadSections) == 0 {
		r.AddError("bead-sections", "no bead sections found (expected ## Bead ... headings)")
		return r
	}

	for _, bs := range beadSections {
		checkBeadSection(r, bs, isApproved)
	}

	return r
}

func checkPlanApprovalGateConsistency(r *Result, specID string, fm *PlanFrontmatter) {
	if !strings.EqualFold(strings.TrimSpace(fm.Status), "Approved") {
		return
	}

	// Derive lifecycle phase from beads (ADR-0023).
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil {
		return // no epic → skip check
	}

	derivedPhase, err := phase.DerivePhase(epicID)
	if err != nil {
		return
	}

	// If plan is approved but beads-derived phase is still plan or spec, warn.
	if derivedPhase == state.ModePlan || derivedPhase == state.ModeSpec {
		r.AddWarning("plan-gate-consistency", fmt.Sprintf("plan frontmatter status is Approved but derived phase is %q", derivedPhase))
	}
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

// BeadSection represents a parsed bead section from a plan.
type BeadSection struct {
	Heading      string
	StepsCount   int
	HasVerify    bool
	VerifyCount  int
	VerifyLines  []string // raw text of verification items
	HasDependsOn bool
	DependsOn    string // raw text after "Depends on" marker
}

// testArtifactPatterns are substrings that indicate a concrete test artifact reference.
var testArtifactPatterns = []string{
	"_test.go",
	".test.ts",
	".test.js",
	".spec.ts",
	"make test",
	"go test",
	"pytest",
	"npm test",
	"mindspec validate",
}

// ParseBeadSections finds and parses ## Bead ... sections from plan content.
func ParseBeadSections(content string) []BeadSection {
	var sections []BeadSection
	lines := strings.Split(content, "\n")

	var current *BeadSection
	inSteps := false
	inVerify := false
	inDependsOn := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(line, "## Bead ") {
			if current != nil {
				sections = append(sections, *current)
			}
			current = &BeadSection{Heading: strings.TrimPrefix(line, "## ")}
			inSteps = false
			inVerify = false
			inDependsOn = false
			continue
		}

		if current == nil {
			continue
		}

		// Detect section markers within a bead.
		// Accept both bold markers (**Steps**) and H3 headings (### Steps).
		if strings.HasPrefix(trimmed, "**Steps**") || trimmed == "### Steps" {
			inSteps = true
			inVerify = false
			inDependsOn = false
			continue
		}
		if strings.HasPrefix(trimmed, "**Verification**") || trimmed == "### Verification" {
			inVerify = true
			inSteps = false
			inDependsOn = false
			current.HasVerify = true
			continue
		}
		if strings.HasPrefix(trimmed, "**Depends on**") || trimmed == "### Depends on" {
			current.HasDependsOn = true
			inSteps = false
			inVerify = false
			inDependsOn = true
			// Capture inline text after colon, e.g. "**Depends on**: Bead 1"
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				after := strings.TrimSpace(trimmed[idx+1:])
				if after != "" {
					current.DependsOn = after
					inDependsOn = false
				}
			}
			continue
		}
		if strings.HasPrefix(trimmed, "**Scope**") || trimmed == "### Scope" {
			inSteps = false
			inVerify = false
			inDependsOn = false
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
			inDependsOn = false
			continue
		}

		// Count items
		if inSteps && len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && trimmed[1] == '.' {
			current.StepsCount++
		}
		if inVerify && (strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]")) {
			current.VerifyCount++
			current.VerifyLines = append(current.VerifyLines, trimmed)
		}
		if inDependsOn && trimmed != "" {
			current.DependsOn = trimmed
			inDependsOn = false
		}
	}

	if current != nil {
		sections = append(sections, *current)
	}

	return sections
}

// checkADRCitations validates that each cited ADR exists and has appropriate status.
func checkADRCitations(r *Result, root string, citations []ADRCitation) {
	for _, cite := range citations {
		path := filepath.Join(workspace.ADRDir(root), cite.ID+".md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			r.AddError("adr-cite-missing", fmt.Sprintf("cited ADR %s does not exist", cite.ID))
			continue
		}

		a, err := adr.ParseADR(path)
		if err != nil {
			r.AddWarning("adr-cite-parse", fmt.Sprintf("cannot parse cited ADR %s: %v", cite.ID, err))
			continue
		}

		if strings.EqualFold(a.Status, "Superseded") {
			msg := fmt.Sprintf("cited ADR %s is Superseded", cite.ID)
			if a.SupersededBy != "" {
				msg += fmt.Sprintf(" (see %s)", a.SupersededBy)
			}
			r.AddWarning("adr-cite-superseded", msg)
		}
		if strings.EqualFold(a.Status, "Proposed") {
			r.AddWarning("adr-cite-proposed", fmt.Sprintf("cited ADR %s has status Proposed — consider accepting it first", cite.ID))
		}
	}
}

// hasSection checks whether a given ## heading exists in the content.
func hasSection(content, heading string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == heading {
			return true
		}
	}
	return false
}

// checkBeadSection validates a single bead section.
func checkBeadSection(r *Result, bs BeadSection, isApproved bool) {
	if bs.StepsCount < 3 {
		r.AddError("bead-steps", fmt.Sprintf("%s: expected 3-7 steps, found %d", bs.Heading, bs.StepsCount))
	} else if bs.StepsCount > 7 {
		r.AddWarning("bead-steps", fmt.Sprintf("%s: has %d steps (recommended 3-7)", bs.Heading, bs.StepsCount))
	}

	if !bs.HasVerify || bs.VerifyCount == 0 {
		r.AddError("bead-verification", fmt.Sprintf("%s: missing verification steps", bs.Heading))
	}

	// Spec 039: check verification testability
	if bs.HasVerify && bs.VerifyCount > 0 && !isApproved {
		checkVerificationTestability(r, bs)
	}

	if !bs.HasDependsOn {
		r.AddWarning("bead-depends", fmt.Sprintf("%s: no 'Depends on' declaration", bs.Heading))
	}
}

// checkVerificationTestability ensures at least one verification item references a test artifact.
func checkVerificationTestability(r *Result, bs BeadSection) {
	for _, line := range bs.VerifyLines {
		lower := strings.ToLower(line)
		for _, pattern := range testArtifactPatterns {
			if strings.Contains(lower, strings.ToLower(pattern)) {
				return
			}
		}
	}
	r.AddError("bead-verification-testability",
		fmt.Sprintf("%s: verification steps must reference at least one test artifact (e.g., _test.go, make test, go test, pytest)", bs.Heading))
}
