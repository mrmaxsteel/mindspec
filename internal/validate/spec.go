package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
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

	// Defense in depth: workspace.SpecDir will also validate, but failing fast
	// here gives a clearer error message tagged with the spec subcommand.
	if err := SpecID(specID); err != nil {
		r.AddError("spec-id", err.Error())
		return r
	}

	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		r.AddError("spec-id", err.Error())
		return r
	}
	specPath := filepath.Join(specDir, "spec.md")
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

	// Spec 110 R5/R6: fold the plan-approve parser-parity checks into
	// spec-approve, at the identical code/severity the plan gate already
	// uses (ADR-0037 single-source-of-truth boundary). R7c keeps
	// Accepted-status, domain-intersection, and coverage evaluation
	// plan-approve-only — these two checks are existence/resolution only.
	checkImpactedDomainsResolutionParity(r, root, specDir)
	checkADRTouchpointsExist(r, root, specDir, sections)

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

	// Criterion quality ("is this vague?") is a semantic judgment and belongs
	// to the AI reviewer at spec-approve time, not a deterministic keyword
	// allowlist. Enforcing an English phrasebook ("works correctly", "is fast")
	// in Go code is a ZFC violation and fails silently for any synonym,
	// translation, or domain-specific term the author didn't anticipate.
}

// checkImpactedDomainsResolutionParity implements spec 110 R5: it folds the
// plan-approve Impacted-Domains resolution check into spec-approve, making
// the IDENTICAL call plan.go's ValidatePlan (plan.go:142-146) makes — same
// `loadImpactedDomains` extraction, same `normalizeImpactedDomains(nil, root,
// "", impacted)` working-tree resolution, same `impacted-domains-resolve`
// error code. A path-like zero/multi-owner entry fails here exactly as it
// would fail at plan-approve. Spec-approve does not consume the normalized
// domain set further (no coverage/citation gates run here — R7c keeps those
// plan-approve-only), so only the resolver's own errors are surfaced.
//
// Spec 122 R1 (forward-only): a Rule-2 bare-name-no-manifest entry
// (`normalizeImpactedDomains` keeps it verbatim, no error) is promoted to a
// hard `impacted-domains-resolve` ERROR here ONLY when the spec's OWN
// frontmatter status — read via SpecStatusAt(specDir), the parse-the-
// contract signal, NOT the plan's status — is an explicit case-folded
// "Draft". Every other status (Approved, any other explicit non-Draft
// value, or empty because the spec has no frontmatter / no `status:` key)
// is grandfathered: this keeps the ~35 Approved bare-label specs and the
// ~22 status-less legacy specs (007-beads-tooling era) byte-identical, and
// only catches a spec being newly authored as Draft with a label that can
// never own a file.
func checkImpactedDomainsResolutionParity(r *Result, root, specDir string) {
	impacted, impErr := loadImpactedDomains(specDir)
	if impErr != nil && !os.IsNotExist(errors.Unwrap(impErr)) {
		// Mirrors plan.go's treatment of a PARSE error (spec.md present but
		// malformed). Unreachable in practice here because ValidateSpec has
		// already confirmed spec.md is readable before this check runs.
		r.AddError("impacted-domains-load", impErr.Error())
	}
	_, normErrs := normalizeImpactedDomains(nil, root, "", impacted)
	for _, e := range normErrs {
		r.AddError("impacted-domains-resolve", e)
	}

	if strings.EqualFold(SpecStatusAt(specDir), "Draft") {
		bare := bareUnresolvedImpactedDomains(nil, root, "", impacted)
		for _, e := range impactedDomainsForwardOnlyErrors(nil, root, "", bare) {
			r.AddError("impacted-domains-resolve", e)
		}
	}
}

// adrTouchpointLinkRe matches an anchored markdown link to an ADR inside the
// `## ADR Touchpoints` section, in both the bare-ID anchor form
// (`[ADR-0031](../../adr/ADR-0031.md)`) and the filename-form anchor the
// repo's merged specs actually write
// (`[ADR-0031-doc-sync-gate.md](../../adr/ADR-0031-doc-sync-gate.md)`). After
// the 4-digit ID, group 2 (`([^0-9\]][^\]]*)?`) requires either nothing (bare
// `]` immediately follows) or a non-digit, non-`]` character before any
// further filename-slug tail — so a 5th digit can never be absorbed into the
// same match. Go RE2 has no lookahead, so this digit boundary is expressed
// structurally rather than as an assertion: `[ADR-12345](...)` and
// `[ADR-00311](...)` fall outside the pattern entirely (no match at all)
// instead of truncating to a 4-digit ID the author never wrote. Either
// genuine link shape is still captured under group 1 as the bare `ADR-NNNN`
// ID. A bare-prose `ADR-####` mention with no `[...](...)` anchor at all
// never matches.
var adrTouchpointLinkRe = regexp.MustCompile(`\[(ADR-\d{4})([^0-9\]][^\]]*)?\]\([^)]+\)`)

// checkADRTouchpointsExist implements spec 110 R6: verify that every ADR
// referenced by an anchored markdown link in the spec's `## ADR Touchpoints`
// section EXISTS, resolved against the SAME `adr.Store` the plan-time
// citation gate (checkADRCitations, plan.go:156) reads
// (`newMemoStore(adrStoreForSpecFn(root, specDir))`). This is an
// existence-only check, modeled on `adr-cite-missing`'s existence-only
// shape but under a distinct code (`adr-touchpoint-missing`) since it
// checks the spec's touchpoints prose, not the plan's frontmatter
// citations (spec 097 R2 boundary). It deliberately emits no
// `adr-coverage-*` or `adr-cite-irrelevant` diagnostic — Accepted-status,
// domain-intersection, and coverage evaluation stay at plan-approve (R7c /
// ADR-0032).
func checkADRTouchpointsExist(r *Result, root, specDir string, sections map[string]string) {
	body, exists := sections["ADR Touchpoints"]
	if !exists {
		return // already flagged by the required-section check above
	}

	matches := adrTouchpointLinkRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return
	}

	store := newMemoStore(adrStoreForSpecFn(root, specDir))
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		id := m[1]
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if _, err := store.Get(id); err != nil {
			// id is a regex-captured reference out of the agent-authored
			// spec.md "## ADR Touchpoints" prose — free text, not a
			// validated ADR ID (that's why the lookup just failed) — so
			// it is escaped, not idrender'd, before it reaches
			// Result.FormatText's terminal render (spec 120 R4).
			r.AddError("adr-touchpoint-missing", fmt.Sprintf("ADR Touchpoints references %s, which does not exist; fix the typo/reference or run `mindspec adr list` to find the correct ID", termsafe.Escape(id)))
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
	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return
	}
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
			// t is a full line of agent-authored spec.md prose (the "##
			// Open Questions" body) — free text — so it is escaped
			// before reaching Result.FormatText's terminal render.
			r.AddError("open-question", fmt.Sprintf("unresolved open question: %s", termsafe.Escape(t)))
		}
	}
}
