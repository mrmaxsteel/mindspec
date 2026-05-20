package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/contextpack"
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
// Accepts the mapping form (`- id: ADR-0001, sections: [...]`) and the
// shorthand scalar form (`- ADR-0001`) for plans that only need IDs.
type ADRCitation struct {
	ID       string   `yaml:"id"`
	Sections []string `yaml:"sections"`
}

// UnmarshalYAML accepts both scalar (`- ADR-0001`) and mapping (`- id: ADR-0001`) forms.
func (c *ADRCitation) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		c.ID = strings.TrimSpace(node.Value)
		return nil
	}
	type raw ADRCitation
	var r raw
	if err := node.Decode(&r); err != nil {
		return err
	}
	*c = ADRCitation(r)
	return nil
}

// ValidatePlan checks the structural quality of a plan.md file.
func ValidatePlan(root, specID string) *Result {
	r := &Result{SubCommand: "plan", TargetID: specID}

	if err := SpecID(specID); err != nil {
		r.AddError("spec-id", err.Error())
		return r
	}

	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		r.AddError("spec-id", err.Error())
		return r
	}
	planPath := filepath.Join(specDir, "plan.md")
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
		// Spec 087 Bead 1: the new cite-relevant + coverage gates are
		// additive; per the Spec 039 backwards-compatibility pattern they
		// are suppressed for already-approved plans so historical plans
		// don't re-fail under the new check.
		var impacted []string
		if !isApproved {
			impacted, _ = loadImpactedDomains(specDir)
		}
		checkADRCitations(r, store, fm.ADRCitations, impacted)
		checkADRCoverage(r, store, fm.ADRCitations, impacted)
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
		checkBeadSection(r, bs)
	}

	// Spec 076: cross-bead decomposition quality checks
	if !isApproved {
		// Load project config for decomposition thresholds. Non-fatal: a
		// malformed config file should not block plan validation, so we
		// fall back to baked-in defaults and proceed.
		cfg, err := config.Load(root)
		if err != nil {
			cfg = config.DefaultConfig()
		}
		checkDecompositionQuality(r, beadSections, cfg.Decomposition)
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
//
// Spec 087 Bead 1: additionally checks that each cited ADR's declared
// domains intersect the spec's impacted-domains list. When the
// intersection is empty (and impactedDomains is known), the citation is
// architecturally irrelevant — emits "adr-cite-irrelevant". The check is
// suppressed when impactedDomains is empty (e.g. spec.md has no
// `## Impacted Domains` section yet) so the existing behaviour is
// preserved verbatim for plans that pre-date the semantic gate.
func checkADRCitations(r *Result, store adr.Store, citations []ADRCitation, impactedDomains []string) {
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

		// Spec 087 Bead 1: cite-relevant check. Only run when we have a
		// concrete impacted-domains list to compare against — without one,
		// "relevance" is undefined and the check would false-positive every
		// pre-087 plan.
		if len(impactedDomains) > 0 {
			overlap := intersectFold(a.Domains, impactedDomains)
			if len(overlap) == 0 {
				r.AddError("adr-cite-irrelevant", fmt.Sprintf("cited ADR %s declares domains %v which do not intersect spec impacted domains %v", cite.ID, a.Domains, impactedDomains))
			}
		}
	}
}

// checkADRCoverage ensures every spec impacted-domain is covered by at
// least one cited Accepted ADR. Coverage is the predicate "there exists a
// cited ADR `a` such that a.Status == Accepted AND a.Domains contains
// domain (case-folded)". A cited Superseded ADR satisfies coverage only
// when the chain head (resolved by walkSupersededChain) is ALSO cited and
// itself Accepted+covering — see IsDomainCovered for the canonical
// predicate (revision 5 single source of truth).
//
// Suppressed when impactedDomains is empty: a spec without a declared
// impacted-domains list cannot be coverage-checked.
func checkADRCoverage(r *Result, store adr.Store, citations []ADRCitation, impactedDomains []string) {
	if len(impactedDomains) == 0 {
		return
	}
	for _, d := range impactedDomains {
		if !IsDomainCovered(store, citations, d) {
			r.AddError("adr-coverage-missing", fmt.Sprintf("impacted domain %q has no cited Accepted ADR; run: mindspec adr create --domain %s", d, d))
		}
	}
}

// IsDomainCovered is the canonical predicate "domain X is covered by an
// Accepted cited ADR, transitively through one supersede-chain hop".
// Exported as the single source of truth for both plan-time (Bead 1) and
// bead-time divergence (Bead 2) coverage decisions — divergence.go MUST
// call this helper rather than duplicate the Accepted+intersect logic.
//
// Coverage rules:
//   - A cited ADR with Status Accepted whose Domains contain `domain`
//     (case-folded) → covered.
//   - A cited ADR with Status Superseded → covered ONLY IF the supersede
//     chain head is also cited AND itself Accepted AND its Domains
//     contain `domain`.
//   - Proposed (including placeholders pre-created by the supersede flow)
//     does NOT satisfy coverage (revision 11).
//
// Walker failures (cycle, too-long chain) cause the Superseded path to
// be treated as not-covering; the diagnostic for those structural
// failures is surfaced separately by walkSupersededChain callers when
// they care.
func IsDomainCovered(store adr.Store, citations []ADRCitation, domain string) bool {
	// Build a set of cited ADR IDs for O(1) "is this ADR cited" checks
	// during the Superseded-chain resolution.
	citedSet := make(map[string]struct{}, len(citations))
	for _, c := range citations {
		citedSet[strings.TrimSpace(c.ID)] = struct{}{}
	}

	wantDomain := strings.ToLower(strings.TrimSpace(domain))
	for _, c := range citations {
		a, err := store.Get(c.ID)
		if err != nil {
			continue
		}
		if strings.EqualFold(a.Status, "Accepted") {
			if domainSliceContains(a.Domains, wantDomain) {
				return true
			}
			continue
		}
		if strings.EqualFold(a.Status, "Superseded") {
			headID, err := walkSupersededChain(store, a.ID)
			if err != nil {
				continue
			}
			if _, ok := citedSet[headID]; !ok {
				continue
			}
			head, err := store.Get(headID)
			if err != nil {
				continue
			}
			if strings.EqualFold(head.Status, "Accepted") && domainSliceContains(head.Domains, wantDomain) {
				return true
			}
		}
	}
	return false
}

// walkSupersededChain follows the `SupersededBy` pointer from startID
// until it reaches a terminal ADR (one with no SupersededBy). Returns
// the head ID. Detects cycles via a visited set and caps the chain at
// 10 hops; both surface as errors so callers can attribute the
// structural failure.
func walkSupersededChain(store adr.Store, startID string) (string, error) {
	const maxLen = 10
	visited := make(map[string]struct{})
	current := startID
	for hops := 0; hops <= maxLen; hops++ {
		if _, seen := visited[current]; seen {
			return "", fmt.Errorf("adr-supersede-cycle: chain re-visits %s starting from %s", current, startID)
		}
		visited[current] = struct{}{}
		a, err := store.Get(current)
		if err != nil {
			return "", fmt.Errorf("adr-supersede-chain-broken: cannot resolve %s: %w", current, err)
		}
		next := strings.TrimSpace(a.SupersededBy)
		if next == "" {
			return current, nil
		}
		current = next
	}
	return "", fmt.Errorf("adr-supersede-chain-too-long: chain from %s exceeds %d hops", startID, maxLen)
}

// loadImpactedDomains reads spec.md from specDir and returns the parsed
// impacted-domains list. The contextpack parser already lowercases and
// trims, so the returned slice is normalisation-ready for intersectFold.
func loadImpactedDomains(specDir string) ([]string, error) {
	meta, err := contextpack.ParseSpec(specDir)
	if err != nil {
		return nil, err
	}
	return meta.Domains, nil
}

// intersectFold returns the case-folded, trim-whitespace exact set
// intersection of a and b. Build a map over normalised a then probe b
// for O(n+m). This is the canonical domain-overlap algorithm per Spec
// 087 Requirement 16.
func intersectFold(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(a))
	for _, x := range a {
		set[strings.ToLower(strings.TrimSpace(x))] = struct{}{}
	}
	seen := make(map[string]struct{}, len(b))
	var out []string
	for _, y := range b {
		norm := strings.ToLower(strings.TrimSpace(y))
		if norm == "" {
			continue
		}
		if _, ok := set[norm]; !ok {
			continue
		}
		if _, dup := seen[norm]; dup {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, norm)
	}
	return out
}

// domainSliceContains is a tiny case-folded membership test on a list of
// already-normalised domain strings. Used by IsDomainCovered; factored
// out to keep that function readable.
func domainSliceContains(domains []string, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	for _, d := range domains {
		if strings.ToLower(strings.TrimSpace(d)) == want {
			return true
		}
	}
	return false
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

// checkBeadSection validates a single bead section. Note: the previous
// `isApproved` parameter gated the deleted verification-testability check
// (removed as a ZFC violation); the remaining checks are structural and
// apply uniformly regardless of plan status.
func checkBeadSection(r *Result, bs BeadSection) {
	// StepsCount == 0 is a newly-added structural floor: a bead section with a
	// **Steps** heading but zero numbered items is malformed. The lower bound
	// `< 3` (previously an error) is demoted to a warning to match the existing
	// `> 7` upper-bound warning — symmetric advisory signal, not a hard gate.
	if bs.StepsCount == 0 {
		r.AddError("bead-steps", fmt.Sprintf("%s: missing steps (no numbered items under **Steps**)", bs.Heading))
	} else if bs.StepsCount < 3 {
		r.AddWarning("bead-steps", fmt.Sprintf("%s: has %d steps (recommended 3-7)", bs.Heading, bs.StepsCount))
	} else if bs.StepsCount > 7 {
		r.AddWarning("bead-steps", fmt.Sprintf("%s: has %d steps (recommended 3-7)", bs.Heading, bs.StepsCount))
	}

	if !bs.HasVerify || bs.VerifyCount == 0 {
		r.AddError("bead-verification", fmt.Sprintf("%s: missing verification steps", bs.Heading))
	}

	// Verification *quality* (is it concrete? is it testable?) is delegated to
	// the AI reviewer at plan-approve time and to the author via the instruct
	// template. The validator's job is structural: the section exists with at
	// least one checkbox item. Hardcoding keyword allowlists ("go test",
	// "pytest", backtick counts, etc.) is a ZFC violation — it bakes
	// framework/style assumptions into the deterministic shell and fails
	// silently for every toolchain the author didn't anticipate.

	// Spec 080: per-bead acceptance criteria is a structural requirement (error, not warning).
	// Applies regardless of approval status — plans must always have per-bead AC.
	if bs.AcceptanceCriteria == "" {
		r.AddError("bead-acceptance-criteria", fmt.Sprintf("%s: missing per-bead acceptance criteria — each bead must have an **Acceptance Criteria** section", bs.Heading))
	}

	if !bs.HasDependsOn {
		r.AddWarning("bead-depends", fmt.Sprintf("%s: no 'Depends on' declaration", bs.Heading))
	}
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
// when the plan structure correlates with known degradation patterns. All
// emitted issues are advisory (AddWarning, never AddError) and never gate
// approval. Thresholds are configurable via `.mindspec/config.yaml` under
// the top-level `decomposition:` block (see config.Decomposition).
func checkDecompositionQuality(r *Result, sections []BeadSection, cfg config.Decomposition) {
	if len(sections) == 0 {
		return
	}

	// 1. Bead count check
	if len(sections) > cfg.MaxBeads {
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
		if rScope > cfg.MaxScopeOverlap {
			r.AddWarning("decomposition-scope-redundancy",
				fmt.Sprintf("scope redundancy R=%.2f exceeds threshold %.2f — high bead overlap, consider merging beads that share most files", rScope, cfg.MaxScopeOverlap))
		}
		if rScope < cfg.MinScopeOverlap && len(sections) > 2 {
			r.AddWarning("decomposition-scope-redundancy",
				fmt.Sprintf("scope redundancy R=%.2f below threshold %.2f with %d beads — beads may lack shared context", rScope, cfg.MinScopeOverlap, len(sections)))
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
	if chainDepth > cfg.MaxChainDepth {
		r.AddWarning("decomposition-chain-depth",
			fmt.Sprintf("dependency chain depth %d exceeds threshold %d — deep serial chain, coordination overhead grows super-linearly", chainDepth, cfg.MaxChainDepth))
	}

	// Parallelism ratio: beads with zero inbound deps / total
	zeroInbound := 0
	for i := 0; i < len(sections); i++ {
		if inDegree[i] == 0 {
			zeroInbound++
		}
	}
	parallelism := float64(zeroInbound) / float64(len(sections))
	if parallelism < cfg.MinParallelism {
		r.AddWarning("decomposition-parallelism",
			fmt.Sprintf("parallelism ratio %.2f below threshold %.2f — most beads are serial, check for false dependencies", parallelism, cfg.MinParallelism))
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
