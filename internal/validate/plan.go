package validate

import (
	"errors"
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
	WorkChunks   []WorkChunk   `yaml:"work_chunks"`
}

// WorkChunk is one structured implementation chunk declared in plan
// frontmatter. The chunk `id` is 1-based and matches the Nth `## Bead N`
// section in declaration order, so chunk `id N` maps positionally to
// `bead_ids[N-1]`. `depends_on` lists the chunk ids this chunk depends on;
// a `depends_on: [M]` entry makes `bead_ids[N-1]` depend on `bead_ids[M-1]`
// (spec 097 R3). Non-strict yaml.Unmarshal harmlessly ignores any extra
// human-readable keys (title/scope/verify) the templates also carry.
type WorkChunk struct {
	ID        int   `yaml:"id"`
	DependsOn []int `yaml:"depends_on"`
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

	// Backwards compatibility: the historic Spec 039 quality gates
	// (ADR Fitness, Testing Strategy, Provenance) skip for
	// already-approved plans. The new Spec 087 semantic gates (Rev 4
	// fixup) do NOT skip — Spec 087 contains no Approved-plan
	// carve-out and silently bypassing them would let historic plans
	// citing irrelevant or insufficient ADRs pass forever.
	isApproved := strings.EqualFold(fm.Status, "Approved")

	hasADRFitness := hasSection(content, "## ADR Fitness")

	// Spec 087 Bead 1 (Rev 3 fixup): load impacted-domains and surface
	// PARSE errors (file present but malformed) as
	// `impacted-domains-load`. A missing spec.md is intentionally NOT
	// promoted here — that's a `mindspec validate spec` concern, and
	// many internal/validate plan-only test fixtures omit spec.md by
	// design. The empty-domains case (file present but no `##
	// Impacted Domains` bullets) returns nil error + empty slice;
	// downstream the new 087 checks no-op on empty domains, which the
	// spec-author surfaces via `mindspec validate spec`.
	impacted, impErr := loadImpactedDomains(specDir)
	if impErr != nil && !os.IsNotExist(errors.Unwrap(impErr)) {
		r.AddError("impacted-domains-load", impErr.Error())
	}

	// Build the ADR store ONCE for both the citation and coverage
	// lanes. mindspec-ew79: the store overlays the tree SpecDir
	// actually resolved into (which may be a spec worktree carrying
	// branch-only ADRs) over the primary checkout, instead of always
	// reading the primary checkout's ADR dir.
	store := adrStoreForSpec(root, specDir)

	// Check ADR citations + fitness (Spec 039)
	if len(fm.ADRCitations) == 0 {
		if isApproved {
			// skip Spec 039 carve-out for approved plans
		} else if hasADRFitness {
			r.AddWarning("adr-citations", "no ADR citations in frontmatter (ADR Fitness section explains why)")
		} else {
			r.AddError("adr-citations", "no ADR citations in frontmatter and no ## ADR Fitness section — plan shows no evidence of architectural evaluation")
		}
	} else {
		checkADRCitations(r, store, fm.ADRCitations, impacted)
	}

	// Spec 087 Bead 1 (Rev 2 fixup): coverage check runs even when
	// citations are empty, so a plan with non-empty impacted-domains
	// and NO citations emits an `adr-coverage-missing` error for every
	// impacted domain — the canonical "you need to cite an ADR" hint
	// the user expects, not silence. The check itself short-circuits
	// on empty impactedDomains (no domains, nothing to cover).
	if len(impacted) > 0 {
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
		checkDecompositionQuality(r, beadSections, fm.WorkChunks, cfg.Decomposition)
	}

	return r
}

// adrStoreForSpec builds the ADR store for validating a spec whose
// files live at specDir. When specDir resolved into a different tree
// than root (i.e. a spec worktree under .worktrees/, per the ADR-0022
// resolution order in workspace.SpecDir), the returned store overlays
// that tree's ADR dir over the primary checkout's — so ADRs that exist
// only on the spec branch are visible to citation and coverage checks
// run from the primary checkout (mindspec-ew79). When specDir lives in
// the primary tree (or is unrecognizable), this is a plain FileStore
// over root, preserving prior behavior.
func adrStoreForSpec(root, specDir string) adr.Store {
	treeRoot := workspace.TreeRootForSpecDir(specDir)
	if treeRoot == "" {
		return adr.NewFileStore(root)
	}
	absRoot, err := filepath.Abs(root)
	if err == nil && filepath.Clean(absRoot) == treeRoot {
		return adr.NewFileStore(root)
	}
	return adr.NewOverlayStore(adr.NewFileStore(treeRoot), adr.NewFileStore(root))
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

// ParsePlanFrontmatter extracts and parses the YAML frontmatter from plan
// content. It exposes the parsed PlanFrontmatter (including the validated
// ADRCitations) to other packages: the approve flow consumes the structured
// adr_citations for each bead's --design field instead of regex-scraping the
// spec's `## ADR Touchpoints` prose (spec 097 R2).
func ParsePlanFrontmatter(content string) (*PlanFrontmatter, error) {
	return parsePlanFrontmatter(content)
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
// `## Impacted Domains` section yet) so the existing behavior is
// preserved verbatim for plans that pre-date the semantic gate.
func checkADRCitations(r *Result, store adr.Store, citations []ADRCitation, impactedDomains []string) {
	for _, cite := range citations {
		a, err := store.Get(cite.ID)
		if err != nil {
			r.AddError("adr-cite-missing", fmt.Sprintf("cited ADR %s does not exist", cite.ID))
			continue
		}

		// Spec 087 Bead 1 (Rev 8 fixup): compute relevance FIRST so we
		// can suppress duplicate Superseded/Proposed warnings on the
		// same citation — when an ADR is both Superseded AND irrelevant,
		// the irrelevance error is the higher-priority signal and the
		// status warning is noise on the same root cause.
		irrelevant := false
		if len(impactedDomains) > 0 {
			overlap := intersectFold(a.Domains, impactedDomains)
			if len(overlap) == 0 {
				r.AddError("adr-cite-irrelevant", fmt.Sprintf("cited ADR %s declares domains %v which do not intersect spec impacted domains %v", cite.ID, a.Domains, impactedDomains))
				irrelevant = true
			}
		}

		if !irrelevant && strings.EqualFold(a.Status, "Superseded") {
			msg := fmt.Sprintf("cited ADR %s is Superseded", cite.ID)
			if a.SupersededBy != "" {
				msg += fmt.Sprintf(" (see %s)", a.SupersededBy)
			}
			r.AddWarning("adr-cite-superseded", msg)
		}
		if !irrelevant && strings.EqualFold(a.Status, "Proposed") {
			r.AddWarning("adr-cite-proposed", fmt.Sprintf("cited ADR %s has status Proposed — consider accepting it first", cite.ID))
		}
	}
}

// checkADRCoverage ensures every spec impacted-domain is covered by at
// least one cited ADR. Coverage is tri-state (mindspec-53qx):
//
//   - coveredAccepted: a cited Accepted ADR (directly, or transitively
//     via a cited Superseded ADR whose chain head is also cited and
//     Accepted+covering) declares the domain → silent pass.
//   - coveredProposedOnly: the ONLY covering cited ADR(s) are Proposed
//     → advisory `adr-coverage-proposed` warning, not an error. Citing
//     a Proposed ADR in adr_citations is the author's explicit
//     acknowledgement of architectural intent; spec-introduced ADRs
//     are legitimately Proposed until post-impl validation (ADR-0030
//     precedent).
//   - notCovered → `adr-coverage-missing` error, as before.
//
// Suppressed when impactedDomains is empty: a spec without a declared
// impacted-domains list cannot be coverage-checked.
func checkADRCoverage(r *Result, store adr.Store, citations []ADRCitation, impactedDomains []string) {
	if len(impactedDomains) == 0 {
		return
	}
	for _, d := range impactedDomains {
		// Spec 087 Bead 1 (Rev 1 fixup): pass r so walker errors
		// (cycle, too-long chain) surface on the Result instead of
		// being swallowed silently. The Result is deduped by
		// {Name,Message} via AddError so repeat walks of the same
		// broken chain across multiple domains don't spam diagnostics.
		cov, proposedID := coverageOf(r, store, citations, d)
		switch cov {
		case notCovered:
			r.AddError("adr-coverage-missing", fmt.Sprintf("impacted domain %q has no cited Accepted ADR; run: mindspec adr create --domain %s", d, d))
		case coveredProposedOnly:
			r.AddWarning("adr-coverage-proposed", fmt.Sprintf("impacted domain %q is covered only by Proposed ADR %s — flip it to Accepted after the implementation ships", d, proposedID))
		}
	}
}

// domainCoverage is the tri-state result of the coverage probe
// (mindspec-53qx).
type domainCoverage int

const (
	// notCovered: no cited ADR (of any tolerated status) declares the
	// domain.
	notCovered domainCoverage = iota
	// coveredProposedOnly: at least one cited Proposed ADR declares the
	// domain, and no cited Accepted ADR does. Plan-time this downgrades
	// the coverage error to the advisory adr-coverage-proposed warning;
	// bead-time divergence treats it as covered.
	coveredProposedOnly
	// coveredAccepted: a cited Accepted ADR declares the domain
	// (directly or transitively via a cited Superseded chain head).
	coveredAccepted
)

// IsDomainCovered is the canonical predicate "domain X is covered by a
// cited ADR, transitively through one supersede-chain hop". Exported as
// the single source of truth for both plan-time (Bead 1) and bead-time
// divergence (Bead 2) coverage decisions — divergence.go MUST call this
// helper rather than duplicate the status+intersect logic.
//
// Coverage rules:
//   - A cited ADR with Status Accepted whose Domains contain `domain`
//     (case-folded) → covered.
//   - A cited ADR with Status Proposed whose Domains contain `domain`
//     → covered (mindspec-53qx; plan-time additionally emits the
//     advisory adr-coverage-proposed warning via checkADRCoverage).
//     NOTE: this deliberately REVERSES spec-087 "revision 11", which
//     excluded Proposed from coverage. Revision 11 created a
//     chicken-and-egg: a spec-introduced ADR is legitimately Proposed
//     until post-impl validation (ADR-0030 precedent), so authors were
//     forced to flip ADRs to Accepted prematurely just to pass the
//     gate. Citing the Proposed ADR in adr_citations is the explicit
//     opt-in acknowledgement of intent; uncited Proposed ADRs still do
//     not count.
//   - A cited ADR with Status Superseded → covered ONLY IF the supersede
//     chain head is also cited AND itself Accepted AND its Domains
//     contain `domain`.
//
// Walker failures (cycle, too-long chain) cause the Superseded path to
// be treated as not-covering. This wrapper SWALLOWS walker errors —
// callers that need diagnostic propagation (e.g. plan-time
// checkADRCoverage) must use IsDomainCoveredCtx or coverageOf instead.
// The wrapper is retained for Bead 2 / divergence consumers that only
// need the bool predicate.
func IsDomainCovered(store adr.Store, citations []ADRCitation, domain string) bool {
	cov, _ := coverageOf(nil, store, citations, domain)
	return cov != notCovered
}

// IsDomainCoveredCtx is the diagnostic-emitting variant of
// IsDomainCovered: walker errors (adr-supersede-cycle,
// adr-supersede-chain-too-long, adr-supersede-chain-broken) are emitted
// via r.AddError before the function returns false. This is the
// variant callers use so structural ADR-graph failures surface to the
// user rather than being silently treated as "not covered".
//
// Spec 087 Bead 1 (Rev 1 fixup): added to address the unanimous
// reviewer concern that walker errors were swallowed in the original
// IsDomainCovered implementation, hiding cycle / too-long failures
// behind the generic adr-coverage-missing error.
func IsDomainCoveredCtx(r *Result, store adr.Store, citations []ADRCitation, domain string) bool {
	cov, _ := coverageOf(r, store, citations, domain)
	return cov != notCovered
}

// coverageOf is the shared tri-state probe body. When r is non-nil,
// walker errors are emitted via r.AddError; a per-call dedup map
// prevents the same broken chain from spamming the Result when multiple
// impacted domains all probe the same Superseded ADR.
//
// The second return value is the ID of the (first) covering Proposed
// ADR when the result is coveredProposedOnly, for use in the advisory
// warning message; it is "" otherwise.
func coverageOf(r *Result, store adr.Store, citations []ADRCitation, domain string) (domainCoverage, string) {
	// Build a set of cited ADR IDs for O(1) "is this ADR cited" checks
	// during the Superseded-chain resolution. Spec 087 Bead 1 (Rev 5
	// fixup): normalise to upper-case — ADR IDs are ASCII and the
	// canonical form is `ADR-NNNN`, but plans / stores may carry mixed
	// case (e.g. `adr-0030` in a hand-edited frontmatter). Without this
	// the chain-head lookup misses on legitimate citations.
	citedSet := make(map[string]struct{}, len(citations))
	for _, c := range citations {
		citedSet[strings.ToUpper(strings.TrimSpace(c.ID))] = struct{}{}
	}

	// emittedWalkerErr tracks {chainStartID} → emitted so a broken
	// chain is reported once per ValidatePlan call rather than once
	// per impacted domain.
	emittedWalkerErr := map[string]struct{}{}

	wantDomain := strings.ToLower(strings.TrimSpace(domain))
	best := notCovered
	proposedID := ""
	for _, c := range citations {
		a, err := store.Get(c.ID)
		if err != nil {
			continue
		}
		if strings.EqualFold(a.Status, "Accepted") {
			if domainSliceContains(a.Domains, wantDomain) {
				return coveredAccepted, ""
			}
			continue
		}
		// mindspec-53qx: a cited Proposed ADR declaring the domain
		// yields the intermediate state — recorded but the scan
		// continues, since a later cited Accepted ADR upgrades the
		// result to coveredAccepted. Relies on parse-time Status
		// normalization (mindspec-f115) so qualified statuses like
		// "Proposed (part of spec 091)" match.
		if strings.EqualFold(a.Status, "Proposed") {
			if domainSliceContains(a.Domains, wantDomain) && best == notCovered {
				best = coveredProposedOnly
				proposedID = a.ID
			}
			continue
		}
		if strings.EqualFold(a.Status, "Superseded") {
			headID, walkErr := walkSupersededChain(store, a.ID)
			if walkErr != nil {
				if r != nil {
					key := strings.ToUpper(strings.TrimSpace(a.ID))
					if _, dup := emittedWalkerErr[key]; !dup {
						emittedWalkerErr[key] = struct{}{}
						// Surface the structural error under a Name
						// that matches the walkSupersededChain error
						// prefix (adr-supersede-cycle /
						// adr-supersede-chain-too-long /
						// adr-supersede-chain-broken). Parse the
						// prefix from the error text since the walker
						// embeds it as `name: detail`.
						msg := walkErr.Error()
						name := "adr-supersede-chain-broken"
						if idx := strings.Index(msg, ":"); idx > 0 {
							name = strings.TrimSpace(msg[:idx])
						}
						r.AddError(name, msg)
					}
				}
				continue
			}
			headKey := strings.ToUpper(strings.TrimSpace(headID))
			if _, ok := citedSet[headKey]; !ok {
				continue
			}
			head, err := store.Get(headID)
			if err != nil {
				continue
			}
			if strings.EqualFold(head.Status, "Accepted") && domainSliceContains(head.Domains, wantDomain) {
				return coveredAccepted, ""
			}
		}
	}
	if best == coveredProposedOnly {
		return coveredProposedOnly, proposedID
	}
	return notCovered, ""
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

// ValidateWorkChunkAlignment verifies that the structured `work_chunks` ids
// align positionally with the plan's `## Bead N` sections before any
// consumer wires `bead_ids[N-1]` from them (spec 097 R3). It requires the
// chunk ids to be exactly the contiguous set 1..numSections (no gaps, no
// duplicates, no count mismatch) and every `depends_on` target to be an
// in-range, non-self chunk id. A misaligned or out-of-range id set returns a
// descriptive error so callers reject it rather than panic or mis-wire.
//
// An empty `chunks` slice aligns trivially (returns nil): a plan that
// declares no structured deps simply wires nothing.
func ValidateWorkChunkAlignment(chunks []WorkChunk, numSections int) error {
	if len(chunks) == 0 {
		return nil
	}
	if len(chunks) != numSections {
		return fmt.Errorf("work_chunks count (%d) does not match the number of ## Bead sections (%d); each chunk id must map to exactly one bead section", len(chunks), numSections)
	}
	seen := make(map[int]bool, len(chunks))
	for _, c := range chunks {
		if c.ID < 1 || c.ID > numSections {
			return fmt.Errorf("work_chunk id %d is out of range; ids must be the contiguous set 1..%d", c.ID, numSections)
		}
		if seen[c.ID] {
			return fmt.Errorf("work_chunk id %d is declared more than once; ids must be the contiguous set 1..%d", c.ID, numSections)
		}
		seen[c.ID] = true
	}
	// With the count matched and every id in 1..numSections unique, the set
	// is necessarily contiguous 1..numSections. Bounds-check depends_on.
	for _, c := range chunks {
		for _, dep := range c.DependsOn {
			if dep < 1 || dep > numSections {
				return fmt.Errorf("work_chunk id %d declares depends_on id %d which is out of range 1..%d", c.ID, dep, numSections)
			}
			if dep == c.ID {
				return fmt.Errorf("work_chunk id %d declares a self-dependency", c.ID)
			}
		}
	}
	return nil
}

// checkDecompositionQuality computes cross-bead metrics and emits warnings
// when the plan structure correlates with known degradation patterns. All
// emitted issues are advisory (AddWarning, never AddError) and never gate
// approval. Thresholds are configurable via `.mindspec/config.yaml` under
// the top-level `decomposition:` block (see config.Decomposition).
//
// The dependency graph is built from the structured `work_chunks` deps
// (spec 097 R3): chunk `id N` maps positionally to the Nth section, and a
// `depends_on: [M]` entry is an edge `section[M-1] → section[N-1]`. When the
// chunk ids do not align with the sections (advisory path — never gating) the
// out-of-range entries are skipped, mirroring the bounds guard used here.
func checkDecompositionQuality(r *Result, sections []BeadSection, chunks []WorkChunk, cfg config.Decomposition) {
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
	// Build adjacency list from the structured work_chunks deps (spec 097
	// R3): chunk `id N` is the Nth section, and `depends_on: [M]` is the
	// edge section[M-1] → section[N-1]. Every index is bounds-checked so a
	// misaligned id set degrades gracefully on this advisory path.
	inDegree := make(map[int]int, len(sections))
	adj := make(map[int][]int, len(sections))
	for i := range sections {
		inDegree[i] = 0
	}

	for _, chunk := range chunks {
		i := chunk.ID - 1 // chunk ids are 1-indexed
		if i < 0 || i >= len(sections) {
			continue
		}
		for _, dep := range chunk.DependsOn {
			depIdx := dep - 1
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
