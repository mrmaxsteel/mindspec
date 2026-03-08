package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
		store := adr.NewFileStore(root)
		checkADRCitations(r, store, fm.ADRCitations)
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

	// Spec 076: cross-bead decomposition quality checks
	if !isApproved {
		checkDecompositionQuality(r, beadSections)
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
	Heading            string
	StepsCount         int
	StepLines          []string // raw text of numbered step lines
	HasVerify          bool
	VerifyCount        int
	VerifyLines        []string // raw text of verification items
	HasDependsOn       bool
	DependsOn          string // raw text after "Depends on" marker
	AcceptanceCriteria string // per-bead acceptance criteria text
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
	inAC := false
	var acLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(line, "## Bead ") {
			if current != nil {
				if len(acLines) > 0 {
					current.AcceptanceCriteria = strings.TrimSpace(strings.Join(acLines, "\n"))
				}
				sections = append(sections, *current)
			}
			current = &BeadSection{Heading: strings.TrimPrefix(line, "## ")}
			inSteps = false
			inVerify = false
			inDependsOn = false
			inAC = false
			acLines = nil
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
			inAC = false
			continue
		}
		if strings.HasPrefix(trimmed, "**Verification**") || trimmed == "### Verification" {
			inVerify = true
			inSteps = false
			inDependsOn = false
			inAC = false
			current.HasVerify = true
			continue
		}
		if strings.HasPrefix(trimmed, "**Acceptance Criteria**") || trimmed == "### Acceptance Criteria" {
			inAC = true
			inSteps = false
			inVerify = false
			inDependsOn = false
			continue
		}
		if strings.HasPrefix(trimmed, "**Depends on**") || trimmed == "### Depends on" {
			current.HasDependsOn = true
			inSteps = false
			inVerify = false
			inDependsOn = true
			inAC = false
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
			inAC = false
			continue
		}

		// New top-level section ends bead
		if strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "## Bead ") {
			if current != nil {
				if len(acLines) > 0 {
					current.AcceptanceCriteria = strings.TrimSpace(strings.Join(acLines, "\n"))
				}
				sections = append(sections, *current)
				current = nil
			}
			inSteps = false
			inVerify = false
			inDependsOn = false
			inAC = false
			continue
		}

		// Count items
		if inSteps && len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && trimmed[1] == '.' {
			current.StepsCount++
			current.StepLines = append(current.StepLines, trimmed)
		}
		if inVerify && (strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]")) {
			current.VerifyCount++
			current.VerifyLines = append(current.VerifyLines, trimmed)
		}
		if inAC && trimmed != "" {
			acLines = append(acLines, trimmed)
		}
		if inDependsOn && trimmed != "" {
			current.DependsOn = trimmed
			inDependsOn = false
		}
	}

	if current != nil {
		if len(acLines) > 0 {
			current.AcceptanceCriteria = strings.TrimSpace(strings.Join(acLines, "\n"))
		}
		sections = append(sections, *current)
	}

	return sections
}

// checkADRCitations validates that each cited ADR exists and has appropriate status.
func checkADRCitations(r *Result, store adr.Store, citations []ADRCitation) {
	for _, cite := range citations {
		a, err := store.Get(cite.ID)
		if err != nil {
			r.AddError("adr-cite-missing", fmt.Sprintf("cited ADR %s does not exist", cite.ID))
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

	// Spec 078: warn on missing per-bead acceptance criteria
	if bs.AcceptanceCriteria == "" && !isApproved {
		r.AddWarning("bead-acceptance-criteria", fmt.Sprintf("%s: no per-bead acceptance criteria — each bead should have criteria scoped to its own work", bs.Heading))
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

// pathRefRe matches Go file paths (internal/foo/bar.go), package paths
// (./internal/foo/...), and dotted paths (cmd/mindspec/root.go).
var pathRefRe = regexp.MustCompile(`(?:\./)?\b(?:[a-zA-Z0-9_]+/)+[a-zA-Z0-9_]+(?:\.go|/\.\.\.)?`)

// ExtractPathRefs extracts file and package path references from text.
// It returns deduplicated path strings found via regex.
func ExtractPathRefs(text string) []string {
	matches := pathRefRe.FindAllString(text, -1)
	seen := make(map[string]bool, len(matches))
	var result []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}
	return result
}

// beadDepRe matches "Bead N" references in dependency text.
var beadDepRe = regexp.MustCompile(`(?i)bead\s+(\d+)`)

// checkDecompositionQuality computes cross-bead metrics and emits warnings
// when the plan structure correlates with known degradation patterns.
func checkDecompositionQuality(r *Result, sections []BeadSection) {
	if len(sections) == 0 {
		return
	}

	// 1. Bead count check
	if len(sections) > 6 {
		r.AddWarning("decomposition-bead-count",
			fmt.Sprintf("plan has %d beads — consider whether decomposition is too fine-grained; 3-5 is optimal", len(sections)))
	}

	// 2. Scope redundancy (R_scope)
	beadPaths := make([]map[string]bool, len(sections))
	allPaths := make(map[string]bool)
	for i, bs := range sections {
		paths := make(map[string]bool)
		// Extract from steps and verification
		for _, line := range bs.StepLines {
			for _, p := range ExtractPathRefs(line) {
				paths[p] = true
				allPaths[p] = true
			}
		}
		for _, line := range bs.VerifyLines {
			for _, p := range ExtractPathRefs(line) {
				paths[p] = true
				allPaths[p] = true
			}
		}
		beadPaths[i] = paths
	}

	if len(allPaths) > 0 {
		// Count paths referenced by more than one bead
		sharedCount := 0
		for p := range allPaths {
			refCount := 0
			for _, bp := range beadPaths {
				if bp[p] {
					refCount++
				}
			}
			if refCount > 1 {
				sharedCount++
			}
		}

		rScope := float64(sharedCount) / float64(len(allPaths))
		if rScope > 0.50 {
			r.AddWarning("decomposition-scope-redundancy",
				fmt.Sprintf("scope redundancy R=%.2f exceeds threshold 0.50 — high bead overlap, consider merging beads that share most files", rScope))
		}
		if rScope < 0.15 && len(sections) > 2 {
			r.AddWarning("decomposition-scope-redundancy",
				fmt.Sprintf("scope redundancy R=%.2f below threshold 0.15 with %d beads — beads may lack shared context", rScope, len(sections)))
		}
	}

	// 3. Dependency chain depth and parallelism ratio
	// Build adjacency list from DependsOn text
	inDegree := make(map[int]int, len(sections))
	adj := make(map[int][]int, len(sections))
	for i := range sections {
		inDegree[i] = 0
	}

	for i, bs := range sections {
		if bs.DependsOn == "" {
			continue
		}
		lower := strings.ToLower(bs.DependsOn)
		if lower == "none" || lower == "nothing" || lower == "n/a" {
			continue
		}
		matches := beadDepRe.FindAllStringSubmatch(bs.DependsOn, -1)
		for _, m := range matches {
			depNum := 0
			fmt.Sscanf(m[1], "%d", &depNum)
			depIdx := depNum - 1 // beads are 1-indexed
			if depIdx >= 0 && depIdx < len(sections) && depIdx != i {
				adj[depIdx] = append(adj[depIdx], i)
				inDegree[i]++
			}
		}
	}

	// Compute longest path (chain depth) via BFS/topological order
	chainDepth := computeChainDepth(adj, len(sections))
	if chainDepth > 3 {
		r.AddWarning("decomposition-chain-depth",
			fmt.Sprintf("dependency chain depth %d exceeds threshold 3 — deep serial chain, coordination overhead grows super-linearly", chainDepth))
	}

	// Parallelism ratio: beads with zero inbound deps / total
	zeroInbound := 0
	for i := 0; i < len(sections); i++ {
		if inDegree[i] == 0 {
			zeroInbound++
		}
	}
	parallelism := float64(zeroInbound) / float64(len(sections))
	if parallelism < 0.25 {
		r.AddWarning("decomposition-parallelism",
			fmt.Sprintf("parallelism ratio %.2f below threshold 0.25 — most beads are serial, check for false dependencies", parallelism))
	}
}

// computeChainDepth returns the longest path length in the DAG.
func computeChainDepth(adj map[int][]int, n int) int {
	// Use DFS with memoization
	memo := make(map[int]int, n)
	var dfs func(node int) int
	dfs = func(node int) int {
		if v, ok := memo[node]; ok {
			return v
		}
		maxChild := 0
		for _, next := range adj[node] {
			if d := dfs(next); d > maxChild {
				maxChild = d
			}
		}
		memo[node] = maxChild + 1
		return maxChild + 1
	}

	maxDepth := 0
	for i := 0; i < n; i++ {
		if d := dfs(i); d > maxDepth {
			maxDepth = d
		}
	}
	return maxDepth
}
