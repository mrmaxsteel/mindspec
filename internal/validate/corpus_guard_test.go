package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// TestCorpusGuard_ImpactedDomainsForwardOnlyGrandfather pins spec 122
// AC-1b(ii): the real "bare-label specs" in THIS repo's `.mindspec/specs/*`
// corpus — specs whose `## Impacted Domains` carries at least one Rule-2
// bare token (bareUnresolvedImpactedDomains returns non-empty for it) —
// split by explicit spec.md frontmatter status, must stay green under
// Requirement 1's forward-only gate: neither the Approved bare-label
// cohort nor the status-less legacy cohort (no YAML frontmatter, or
// frontmatter with no `status:` key — both of which validate.SpecStatus
// returns as the empty string) may pick up a NEW `impacted-domains-resolve`
// finding attributable to the forward-only check. The single
// `status: Draft` scaffold template (067-harness-adr023-compat) is
// EXCLUDED from this guard set — it is dispositioned by AC-14 (Bead 4)
// instead, where R1 CORRECTLY fires on its Draft placeholder label,
// crossing no green->red boundary.
//
// Deliberately calls checkImpactedDomainsResolutionParity directly (the
// exact function the spec-approve authoring gate invokes) rather than the
// full ValidateSpec: ValidateSpec also runs checkLifecycleBinding, which
// shells out to `bd` per spec via a freshly-constructed phase.Cache with no
// cross-call memoization — spawning ~100 uncached `bd` subprocesses in a
// tight loop is slow and, in this shared-dolt-store development
// environment (concurrent agents), badly contended. This guard's actual
// claim is scoped to the impacted-domains-resolve LANE (per AC-1's own
// scoping note), so it exercises exactly that lane's real function with
// zero `bd` dependency.
//
// The per-spec assertion compares the ISSUE COUNT `normalizeImpactedDomains`
// alone would already emit (the OLD, unconditional Rule-1/2/3 pass) against
// the full count checkImpactedDomainsResolutionParity emits (old pass PLUS
// the new forward-only pass): equal counts prove the forward-only addition
// contributed NOTHING NEW for that spec — the precise "did my new code
// newly redden this spec" question, immune to unrelated PRE-EXISTING
// Rule-3 zero/multi-owner errors a path-shaped entry may already carry
// (which are not this bead's concern and are not newly introduced by it).
//
// Both cohorts are asserted NON-EMPTY so the guard cannot go vacuous. This
// test is RED against a status-blind implementation that keys on
// `!= "Approved"` (which would flip every status-less legacy bare-label
// spec red) or that hard-fails Approved specs outright — either of which is
// exactly the corpus-collision regression Requirement 1's forward-only,
// explicit-status gating exists to avoid.
func TestCorpusGuard_ImpactedDomainsForwardOnlyGrandfather(t *testing.T) {
	root := findProjectRoot(t)
	specsDir := filepath.Join(root, ".mindspec", "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		t.Fatalf("reading %s: %v", specsDir, err)
	}

	// The spec's chosen disposition (see spec.md AC-1b / AC-14): excluded
	// from THIS grandfather guard because it is a not-yet-authored Draft
	// scaffold whose placeholder label R1 is SUPPOSED to catch.
	const excludedDraftScaffold = "067-harness-adr023-compat"

	type candidate struct {
		specID  string
		specDir string
	}
	var approvedCohort []candidate
	var statusLessCohort []candidate

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		specID := e.Name()
		if specID == excludedDraftScaffold {
			continue
		}
		specDir, dirErr := workspace.SpecDir(root, specID)
		if dirErr != nil {
			continue // not a well-formed spec ID; not this guard's concern
		}
		specMD := filepath.Join(specDir, "spec.md")
		if _, statErr := os.Stat(specMD); statErr != nil {
			continue // no spec.md at all (an incomplete/stray dir)
		}

		impacted, impErr := loadImpactedDomains(specDir)
		if impErr != nil {
			continue // malformed spec.md; unrelated to this guard
		}
		bare := bareUnresolvedImpactedDomains(nil, root, "", impacted)
		if len(bare) == 0 {
			continue // not a "bare-label spec" — Rule 2 does not apply here
		}

		status := SpecStatusAt(specDir)
		c := candidate{specID: specID, specDir: specDir}
		switch {
		case strings.EqualFold(status, "Approved"):
			approvedCohort = append(approvedCohort, c)
		case status == "":
			statusLessCohort = append(statusLessCohort, c)
		}
	}

	if len(approvedCohort) == 0 {
		t.Fatal("guard is vacuous: zero Approved bare-label specs found in the real corpus — the guard proves nothing")
	}
	if len(statusLessCohort) == 0 {
		t.Fatal("guard is vacuous: zero status-less legacy bare-label specs found in the real corpus — the guard proves nothing")
	}

	checkCohort := func(cohortName string, cohort []candidate) {
		t.Helper()
		for _, c := range cohort {
			impacted, impErr := loadImpactedDomains(c.specDir)
			if impErr != nil {
				t.Errorf("%s spec %q: unexpected loadImpactedDomains error: %v", cohortName, c.specID, impErr)
				continue
			}
			_, normErrs := normalizeImpactedDomains(nil, root, "", impacted)

			r := &Result{SubCommand: "spec", TargetID: c.specID}
			checkImpactedDomainsResolutionParity(r, root, c.specDir)
			var got []string
			for _, issue := range r.Issues {
				if issue.Name == "impacted-domains-resolve" {
					got = append(got, issue.Message)
				}
			}

			if len(got) != len(normErrs) {
				t.Errorf("%s spec %q: forward-only added a NEW impacted-domains-resolve finding (pre-existing count %d, got %d): %v", cohortName, c.specID, len(normErrs), len(got), got)
			}
		}
	}
	checkCohort("Approved", approvedCohort)
	checkCohort("status-less", statusLessCohort)
}
