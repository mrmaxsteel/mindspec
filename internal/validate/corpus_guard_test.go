package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/adr"
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

// alreadyRedDraftScaffold is the spec 122 AC-14 067 disposition: the
// `status: Draft` scaffold-template spec whose Impacted-Domains is still
// the unedited placeholder label. It is ALREADY red today (section-empty /
// criteria-count / open-question, asserted below) and R1 CORRECTLY fires
// its NEW `impacted-domains-resolve` finding on that placeholder label —
// crossing no green->red boundary because 067 was never green. This is the
// ONE spec this guard permits to gain a new domains-lane finding.
const alreadyRedDraftScaffold067 = "067-harness-adr023-compat"

// TestCorpusGuard_CeremonyPolarity pins spec 122 AC-14(b): across THIS
// repo's real corpus (every `.mindspec/specs/*` with a spec.md, against
// the real `.mindspec/domains` + `.mindspec/adr`), no spec that validated
// GREEN on the domains/ADR lanes Requirements 1 and 2 can affect turns RED
// after this spec's beads land — R7's green->red polarity rule, with the
// 067 disposition asserted exactly as AC-14 chose it.
//
// Scope, stated honestly: this walks every lane `ValidateSpec`/
// `ValidatePlan` COULD affect via spec 122 — R1's forward-only domains
// reject (`impacted-domains-resolve`, checked once via
// checkImpactedDomainsResolutionParity: `ValidatePlan`'s own forward-only
// block reads the IDENTICAL inputs, SpecStatusAt(specDir) +
// loadImpactedDomains(specDir), not plan.md content — see plan.go's own
// comment beside its call — so one spec-side check is dispositive for
// BOTH authoring verbs) and R2's ADR-side resolution
// (`adr-cite-irrelevant` / `adr-coverage-missing`). It deliberately does
// NOT invoke the full `ValidateSpec`/`ValidatePlan` (which additionally
// shell out to `bd` per spec via `checkLifecycleBinding` /
// `checkBeadIDs` / `checkPlanApprovalGateConsistency`): those bd-backed
// lanes are WARNING-only (`checkLifecycleBinding`) or entirely untouched
// by spec 122's code, so by construction they cannot flip green->red
// here, and — per the SAME "~100 uncached bd subprocesses is slow and
// badly contended" rationale documented on
// TestCorpusGuard_ImpactedDomainsForwardOnlyGrandfather above — running
// them across the whole corpus in this guard would make the package's
// test suite impractically slow (confirmed empirically: the full
// ValidateSpec/ValidatePlan pair took >7s/spec against this repo's shared
// dolt store, i.e. minutes for a ~120-spec corpus). R3/R4 need no lane
// here at all: the spec's own words pin them hint-text-only ("the gate's
// PASS/FAIL boundary is unchanged").
//
// R1's polarity is checked directly (a spec is newly red only if the
// forward-only pass contributes findings beyond the pre-existing
// normalizeImpactedDomains pass — the exact technique
// TestCorpusGuard_ImpactedDomainsForwardOnlyGrandfather already
// established, applied here to the WHOLE corpus rather than just the
// bare-label cohort).
//
// R2's polarity is checked by MONOTONICITY rather than a historical diff:
// the ADR-side resolver can only ever map a previously-literal `Domain(s)`
// entry to a real domain name, which can only make an
// intersection/coverage check MORE likely to pass, never less (the spec's
// own words: "ADR-side resolution introduces NO new error class") — so
// this test constructs the SAME plan-lane check twice per spec, once
// through the domain-resolving store (today's real code path) and once
// through the plain literal store (the pre-Bead-2 code path), and asserts
// the resolved-store error count never EXCEEDS the literal-store count.
//
// The monotonicity assertion here catches a CORRUPTED resolver (one that
// ADDS errors) but is deliberately paired with — not a substitute for —
// the POSITIVE strict-inequality WITNESS in
// TestCorpusGuard_R2ResolutionWitness (spec 122 Bead 4, FX-1 / codex G-2):
// monotonicity alone is TAUTOLOGICAL against a REVERTED (no-op) resolver,
// because if newDomainResolvingStore degrades to `return inner` then
// resolvedStore == literalStore, the two counts go EQUAL, and
// `withResolution > withoutResolution` stays false — so the single most
// important regression (R2 resolution lost) would slip through green here.
// The witness test constructs a synthetic 6ou2/#147-shaped fixture where
// the resolving store STRICTLY removes errors the literal store emits and
// asserts `withResolution < withoutResolution`, going red the instant the
// resolver is reverted to a no-op.
//
// Both techniques run entirely in-process against the CURRENT tree, so no
// historical git checkout is needed to prove "no new redness relative to
// before this spec."
func TestCorpusGuard_CeremonyPolarity(t *testing.T) {
	root := findProjectRoot(t)
	specsDir := filepath.Join(root, ".mindspec", "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		t.Fatalf("reading %s: %v", specsDir, err)
	}

	var sawExpectedNewFinding bool
	var checkedR1, checkedR2 int

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		specID := e.Name()
		specDir, dirErr := workspace.SpecDir(root, specID)
		if dirErr != nil {
			continue // not a well-formed spec ID; not this guard's concern
		}
		specMD := filepath.Join(specDir, "spec.md")
		if _, statErr := os.Stat(specMD); statErr != nil {
			continue // no spec.md at all
		}

		impacted, impErr := loadImpactedDomains(specDir)
		if impErr != nil {
			continue // malformed spec.md; unrelated to this guard
		}

		// ---- R1 polarity: the domains-lane forward-only pass adds
		// nothing new, except on the one pinned disposition. ----
		_, preExistingErrs := normalizeImpactedDomains(nil, root, "", impacted)
		r := &Result{SubCommand: "spec", TargetID: specID}
		checkImpactedDomainsResolutionParity(r, root, specDir)
		var domainsLaneErrs []string
		for _, issue := range r.Issues {
			if issue.Name == "impacted-domains-resolve" {
				domainsLaneErrs = append(domainsLaneErrs, issue.Message)
			}
		}
		checkedR1++
		newFromForwardOnly := len(domainsLaneErrs) - len(preExistingErrs)
		switch {
		case newFromForwardOnly < 0:
			t.Fatalf("spec %q: forward-only pass produced FEWER impacted-domains-resolve findings (%d) than the pre-existing pass (%d) — impossible unless the forward-only branch also SUPPRESSES a pre-existing finding, which R1 must never do", specID, len(domainsLaneErrs), len(preExistingErrs))
		case newFromForwardOnly > 0 && specID != alreadyRedDraftScaffold067:
			t.Errorf("spec %q: R1's forward-only pass newly reddened this spec (outside the pinned %s disposition) with %d new impacted-domains-resolve finding(s): %v", specID, alreadyRedDraftScaffold067, newFromForwardOnly, domainsLaneErrs)
		case newFromForwardOnly > 0 && specID == alreadyRedDraftScaffold067:
			sawExpectedNewFinding = true
		}

		// ---- R2 monotonicity: ADR-side resolution never INCREASES the
		// adr-cite-irrelevant/adr-coverage-missing error count. ----
		planPath := filepath.Join(specDir, "plan.md")
		planData, planErr := os.ReadFile(planPath)
		if planErr != nil {
			continue // no plan.md yet (spec-only stage); not this lane's concern
		}
		fm, fmErr := parsePlanFrontmatter(string(planData))
		if fmErr != nil {
			continue // malformed plan frontmatter; unrelated to this guard
		}
		normalizedImpacted, _ := normalizeImpactedDomains(nil, root, "", impacted)
		if len(normalizedImpacted) == 0 && len(fm.ADRCitations) == 0 {
			continue // both checks below no-op on empty input; nothing to compare
		}
		checkedR2++

		resolvedStore := newDomainResolvingStore(newMemoStore(adrStoreForSpecFn(root, specDir)), nil, root, "")
		literalStore := newMemoStore(adrStoreForSpecFn(root, specDir))

		countLaneErrs := func(store adr.Store) int {
			rr := &Result{SubCommand: "plan", TargetID: specID}
			checkADRCitations(rr, store, fm.ADRCitations, normalizedImpacted)
			checkADRCoverage(rr, store, fm.ADRCitations, normalizedImpacted)
			n := 0
			for _, issue := range rr.Issues {
				if issue.Severity == SevError && (issue.Name == "adr-cite-irrelevant" || issue.Name == "adr-coverage-missing") {
					n++
				}
			}
			return n
		}
		withResolution := countLaneErrs(resolvedStore)
		withoutResolution := countLaneErrs(literalStore)
		if withResolution > withoutResolution {
			t.Errorf("spec %q: ADR-side resolution (R2) INCREASED the adr-cite-irrelevant/adr-coverage-missing count (%d > %d) — R2 must only ever remove false positives, never add", specID, withResolution, withoutResolution)
		}
	}

	if checkedR1 == 0 {
		t.Fatal("guard is vacuous: zero specs walked for the R1 polarity check")
	}
	if checkedR2 == 0 {
		t.Fatal("guard is vacuous: zero specs walked for the R2 monotonicity check")
	}
	if !sawExpectedNewFinding {
		t.Fatalf("guard did not observe the expected NEW impacted-domains-resolve finding on %s (R1 correctly firing on its Draft placeholder label) — the 067 disposition this AC pins did not fire; re-verify the fixture still matches AC-14's premise", alreadyRedDraftScaffold067)
	}

	// AC-14's own framing requires 067 to have been ALREADY red (for
	// reasons UNRELATED to spec 122) before this new finding, so it
	// crosses no green->red boundary. Verified via the full (bd-inclusive)
	// ValidateSpec for this ONE spec only — not the whole corpus, per the
	// bd-contention note above.
	r067 := ValidateSpec(root, alreadyRedDraftScaffold067)
	var preExisting122UnrelatedErrs []string
	for _, issue := range r067.Issues {
		if issue.Severity == SevError && issue.Name != "impacted-domains-resolve" {
			preExisting122UnrelatedErrs = append(preExisting122UnrelatedErrs, issue.Name)
		}
	}
	if len(preExisting122UnrelatedErrs) == 0 {
		t.Fatalf("067 disposition premise violated: expected %s to already be RED for reasons UNRELATED to spec 122 (so its new impacted-domains-resolve finding crosses no green->red boundary), but found none; issues=%v", alreadyRedDraftScaffold067, r067.Issues)
	}
}

// TestCorpusGuard_R2ResolutionWitness is the POSITIVE strict-inequality
// witness for spec 122 R2 (Bead 4, FX-1 / codex G-2). It closes the
// tautology in TestCorpusGuard_CeremonyPolarity's corpus-wide monotonicity
// assertion: monotonicity (`withResolution <= withoutResolution`) catches a
// resolver that ADDS errors but stays GREEN against a resolver REVERTED to
// a no-op (`newDomainResolvingStore` -> `return inner`), because then the
// two stores are identical and the counts are merely EQUAL. This witness
// asserts a STRICT inequality on a fixture engineered so the resolving
// store removes real errors — so the instant R2 resolution is lost, the two
// counts collapse to equal and this test goes RED.
//
// The fixture is the 6ou2/#147 shape, bd-free and hermetic (temp dir, no
// executor, exec == nil / ownerRef == "", the working-tree read the plan
// lane uses): a domain `orders` claiming `src/orders/**`, and an Accepted
// ADR-0090 whose `Domain(s)` is the path-shaped `src/orders/` (NOT the
// bare domain name). Against impacted domains `[orders]`:
//
//   - literal store: the ADR's Domains parse as ["src/orders/"], which
//     intersects `[orders]` NOWHERE, so checkADRCitations emits
//     adr-cite-irrelevant AND checkADRCoverage emits adr-coverage-missing
//     — 2 errors.
//   - resolving store: the same `src/orders/` resolves to owner dir-name
//     "orders", intersects, and covers — 0 errors.
//
// so withResolution (0) < withoutResolution (2), STRICTLY. Reverting the
// decorator to a no-op makes both 2, breaking the strict `<`.
func TestCorpusGuard_R2ResolutionWitness(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "orders", "paths:\n  - src/orders/**\n")
	// Path-shaped Domain(s) — the exact 6ou2 phenomenon: authored as a
	// directory path, not the bare domain name, so it only meets the spec
	// side once R2 resolves it.
	writeADR(t, root, "ADR-0090", "Accepted", []string{"src/orders/"})

	impacted := []string{"orders"}
	citations := []ADRCitation{{ID: "ADR-0090"}}

	countLaneErrs := func(store adr.Store) int {
		rr := &Result{SubCommand: "plan", TargetID: "witness"}
		checkADRCitations(rr, store, citations, impacted)
		checkADRCoverage(rr, store, citations, impacted)
		n := 0
		for _, issue := range rr.Issues {
			if issue.Severity == SevError && (issue.Name == "adr-cite-irrelevant" || issue.Name == "adr-coverage-missing") {
				n++
			}
		}
		return n
	}

	resolvedStore := newDomainResolvingStore(newMemoStore(adr.NewFileStore(root)), nil, root, "")
	literalStore := newMemoStore(adr.NewFileStore(root))

	withResolution := countLaneErrs(resolvedStore)
	withoutResolution := countLaneErrs(literalStore)

	// Anchor the literal-side premise so the witness cannot go hollow: if a
	// future refactor made the literal store ALSO resolve (or made the
	// fixture stop tripping the lanes), withoutResolution would drop and the
	// strict `<` below could pass for the wrong reason.
	if withoutResolution < 2 {
		t.Fatalf("witness premise broken: the literal (unresolved) store must emit BOTH adr-cite-irrelevant and adr-coverage-missing for the path-shaped Domain(s) fixture (expected >=2, got %d) — re-verify the fixture still reproduces the 6ou2 shape", withoutResolution)
	}
	if !(withResolution < withoutResolution) {
		t.Errorf("R2 resolution witness FAILED: expected the domain-resolving store to STRICTLY remove errors the literal store emits (withResolution < withoutResolution), got withResolution=%d withoutResolution=%d — R2 resolution appears reverted to a no-op (newDomainResolvingStore no longer maps path-shaped Domain(s) to owner dir-names)", withResolution, withoutResolution)
	}
}
