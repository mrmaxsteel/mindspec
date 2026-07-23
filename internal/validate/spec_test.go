package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Spec 110 Bead 2: spec-approve parser parity fixtures ---

// writeSpecFixture writes a full, structurally-valid spec.md (every section
// requiredSpecSections lists is present and non-empty) at
// root/docs/specs/999-test/spec.md, so a test can isolate its R5/R6 fixture
// shape in the `## Impacted Domains` / `## ADR Touchpoints` bodies without
// tripping the unrelated section-missing/section-empty/requirements-count/
// acceptance-criteria/open-question checks.
func writeSpecFixture(t *testing.T, root, impactedDomainsBody, adrTouchpointsBody string) {
	t.Helper()
	specDir := filepath.Join(root, "docs", "specs", "999-test")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}

	spec := "# Spec 999-test\n\n" +
		"## Goal\n\nDo something useful.\n\n" +
		"## Impacted Domains\n\n" + impactedDomainsBody + "\n" +
		"## ADR Touchpoints\n\n" + adrTouchpointsBody + "\n" +
		"## Requirements\n\n1. First requirement\n2. Second requirement\n\n" +
		"## Scope\n\n### In Scope\n- something\n\n### Out of Scope\n- something else\n\n" +
		"## Acceptance Criteria\n\n- [ ] First criterion\n- [ ] Second criterion\n- [ ] Third criterion\n\n" +
		"## Open Questions\n\nNone\n\n" +
		"## Approval\n\n- **Status**: DRAFT\n"

	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
}

// findSpecIssue reports whether r.Issues contains an issue with the given
// code, and if so, whether it carries error severity. (docsync_test.go
// already declares a package-level findIssue returning *Issue; this helper
// is named distinctly to avoid the redeclaration.)
func findSpecIssue(r *Result, name string) (found bool, isError bool) {
	for _, issue := range r.Issues {
		if issue.Name == name {
			return true, issue.Severity == SevError
		}
	}
	return false, false
}

// TestValidateSpec_ImpactedDomainSeverityMatchesPlanApprove pins spec 110
// R5: ValidateSpec's Impacted-Domains resolution check rejects EXACTLY what
// plan-approve (ValidatePlan / normalizeImpactedDomains) already rejects,
// under the identical `impacted-domains-resolve` code and SevError
// severity — no stricter rule, nothing plan-approve tolerates newly fails.
func TestValidateSpec_ImpactedDomainSeverityMatchesPlanApprove(t *testing.T) {
	t.Run("path-like-zero-owner-fails", func(t *testing.T) {
		tmp := t.TempDir()
		// An unrelated domain manifest exists, but it does not claim
		// internal/nope/x.go — zero owners.
		writeManifest(t, tmp, "genevieve", "paths:\n  - internal/genevieve/**\n")
		writeSpecFixture(t, tmp,
			"- internal/nope/x.go: touched by tests\n",
			"- [ADR-0001](../../adr/ADR-0001.md): relevant\n")
		writeTestADRWithDomains(t, tmp, "ADR-0001", "Accepted", "core", "")

		r := ValidateSpec(tmp, "999-test")

		found, isErr := findSpecIssue(r, "impacted-domains-resolve")
		if !found {
			t.Fatalf("expected impacted-domains-resolve issue, got: %+v", r.Issues)
		}
		if !isErr {
			t.Error("expected impacted-domains-resolve to be SevError severity")
		}
		if !r.HasFailures() {
			t.Error("expected HasFailures() == true for a path-like zero-owner entry")
		}
	})

	t.Run("bare-name-no-manifest-passes", func(t *testing.T) {
		tmp := t.TempDir()
		writeSpecFixture(t, tmp,
			"- genevieve: touched by tests\n",
			"- [ADR-0001](../../adr/ADR-0001.md): relevant\n")
		writeTestADRWithDomains(t, tmp, "ADR-0001", "Accepted", "core", "")

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "impacted-domains-resolve"); found {
			t.Errorf("expected no impacted-domains-resolve issue for a bare-name-no-manifest entry (plan-approve parity), got: %+v", r.Issues)
		}
		if r.HasFailures() {
			t.Errorf("expected HasFailures() == false for a bare-name-no-manifest entry, got issues: %+v", r.Issues)
		}
	})

	t.Run("real-domain-dir-passes", func(t *testing.T) {
		tmp := t.TempDir()
		writeManifest(t, tmp, "core", "paths:\n  - internal/core/**\n")
		writeSpecFixture(t, tmp,
			"- core: touched by tests\n",
			"- [ADR-0001](../../adr/ADR-0001.md): relevant\n")
		// writeManifest materializes the canonical .mindspec/docs tree,
		// which flips workspace.DocsDir's preference away from the legacy
		// docs/adr writeTestADRWithDomains targets — use the canonical
		// helper so the ADR fixture stays visible to the R6 touchpoint
		// check this test doesn't otherwise care about.
		writeCanonicalADRWithDomains(t, tmp, "ADR-0001", "Accepted", "core")

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "impacted-domains-resolve"); found {
			t.Errorf("expected no impacted-domains-resolve issue for a real domain-dir entry, got: %+v", r.Issues)
		}
		if r.HasFailures() {
			t.Errorf("expected HasFailures() == false for a real domain-dir entry, got issues: %+v", r.Issues)
		}
	})
}

// TestValidateSpec_ADRTouchpointExtractionBoundary pins spec 110 R6: an
// anchored `## ADR Touchpoints` link (bare-ID or filename-form) to a
// nonexistent ADR fails ValidateSpec with `adr-touchpoint-missing` +
// SevError; a bare-prose `ADR-####` mention with no anchor never fails; and
// none of these cases ever emits an `adr-coverage-*` or `adr-cite-irrelevant`
// diagnostic — those stay plan-approve-only (R7c).
func TestValidateSpec_ADRTouchpointExtractionBoundary(t *testing.T) {
	assertNoCoverageOrIrrelevant := func(t *testing.T, r *Result) {
		t.Helper()
		for _, issue := range r.Issues {
			if issue.Name == "adr-cite-irrelevant" {
				t.Errorf("unexpected adr-cite-irrelevant at spec-approve: %+v", issue)
			}
			if len(issue.Name) >= len("adr-coverage") && issue.Name[:len("adr-coverage")] == "adr-coverage" {
				t.Errorf("unexpected adr-coverage-* at spec-approve: %+v", issue)
			}
		}
	}

	t.Run("bare-id-anchor-missing-fails", func(t *testing.T) {
		tmp := t.TempDir()
		writeSpecFixture(t, tmp,
			"- core: touched by tests\n",
			"- [ADR-9999](../../adr/ADR-9999-x.md): relevant\n")

		r := ValidateSpec(tmp, "999-test")

		found, isErr := findSpecIssue(r, "adr-touchpoint-missing")
		if !found {
			t.Fatalf("expected adr-touchpoint-missing issue, got: %+v", r.Issues)
		}
		if !isErr {
			t.Error("expected adr-touchpoint-missing to be SevError severity")
		}
		if !r.HasFailures() {
			t.Error("expected HasFailures() == true for a missing anchored ADR touchpoint")
		}
		assertNoCoverageOrIrrelevant(t, r)
	})

	t.Run("filename-form-anchor-missing-fails", func(t *testing.T) {
		tmp := t.TempDir()
		writeSpecFixture(t, tmp,
			"- core: touched by tests\n",
			"- [ADR-9999-x.md](../../adr/ADR-9999-x.md): relevant\n")

		r := ValidateSpec(tmp, "999-test")

		found, isErr := findSpecIssue(r, "adr-touchpoint-missing")
		if !found {
			t.Fatalf("expected adr-touchpoint-missing issue for the filename-form anchor, got: %+v", r.Issues)
		}
		if !isErr {
			t.Error("expected adr-touchpoint-missing to be SevError severity")
		}
		if !r.HasFailures() {
			t.Error("expected HasFailures() == true for a missing filename-form anchored ADR touchpoint")
		}
		assertNoCoverageOrIrrelevant(t, r)
	})

	t.Run("filename-form-anchor-existing-passes", func(t *testing.T) {
		tmp := t.TempDir()
		writeTestADRWithDomains(t, tmp, "ADR-0031", "Accepted", "workflow", "")
		writeSpecFixture(t, tmp,
			"- core: touched by tests\n",
			"- [ADR-0031-doc-sync-gate.md](../../adr/ADR-0031-doc-sync-gate.md): relevant\n")

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "adr-touchpoint-missing"); found {
			t.Errorf("expected no adr-touchpoint-missing for an existing filename-form anchored ADR, got: %+v", r.Issues)
		}
		if r.HasFailures() {
			t.Errorf("expected HasFailures() == false for an existing filename-form anchored ADR, got: %+v", r.Issues)
		}
		assertNoCoverageOrIrrelevant(t, r)
	})

	t.Run("bare-id-anchor-existing-passes", func(t *testing.T) {
		tmp := t.TempDir()
		writeTestADRWithDomains(t, tmp, "ADR-0031", "Accepted", "workflow", "")
		writeSpecFixture(t, tmp,
			"- core: touched by tests\n",
			"- [ADR-0031](../../adr/ADR-0031.md): relevant\n")

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "adr-touchpoint-missing"); found {
			t.Errorf("expected no adr-touchpoint-missing for an existing bare-ID anchored ADR, got: %+v", r.Issues)
		}
		if r.HasFailures() {
			t.Errorf("expected HasFailures() == false for an existing bare-ID anchored ADR, got: %+v", r.Issues)
		}
		assertNoCoverageOrIrrelevant(t, r)
	})

	t.Run("five-digit-id-produces-no-diagnostic", func(t *testing.T) {
		// Regression for the digit-boundary fix: a 5-digit anchored id must
		// fall outside the extraction shape entirely — it must NOT truncate
		// to ADR-1234 and mis-report a missing ADR the author never wrote.
		tmp := t.TempDir()
		writeSpecFixture(t, tmp,
			"- core: touched by tests\n",
			"- [ADR-12345](../../adr/ADR-12345.md): relevant\n")

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "adr-touchpoint-missing"); found {
			t.Errorf("expected no adr-touchpoint-missing for a 5-digit anchored id (must not truncate to ADR-1234), got: %+v", r.Issues)
		}
		if r.HasFailures() {
			t.Errorf("expected HasFailures() == false for a 5-digit anchored id, got: %+v", r.Issues)
		}
		assertNoCoverageOrIrrelevant(t, r)
	})

	t.Run("five-digit-id-does-not-false-pass-via-four-digit-prefix", func(t *testing.T) {
		// The boundary defect's other direction: [ADR-00311] must not match
		// via the ADR-0031 prefix and silently pass against an existing
		// ADR-0031. It must also not be promoted to a new error class (that
		// would be stricter than plan-approve parity) — just no match.
		tmp := t.TempDir()
		writeTestADRWithDomains(t, tmp, "ADR-0031", "Accepted", "workflow", "")
		writeSpecFixture(t, tmp,
			"- core: touched by tests\n",
			"- [ADR-00311](../../adr/ADR-00311.md): relevant\n")

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "adr-touchpoint-missing"); found {
			t.Errorf("expected no adr-touchpoint-missing for ADR-00311 (must not falsely resolve via the ADR-0031 prefix), got: %+v", r.Issues)
		}
		if r.HasFailures() {
			t.Errorf("expected HasFailures() == false for a 5-digit anchored id, got: %+v", r.Issues)
		}
		assertNoCoverageOrIrrelevant(t, r)
	})

	t.Run("bare-prose-mention-passes", func(t *testing.T) {
		tmp := t.TempDir()
		writeSpecFixture(t, tmp,
			"- core: touched by tests\n",
			"This work relates to ADR-9999, which is not yet a file.\n")

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "adr-touchpoint-missing"); found {
			t.Errorf("expected no adr-touchpoint-missing for a bare-prose ADR mention, got: %+v", r.Issues)
		}
		if r.HasFailures() {
			t.Errorf("expected HasFailures() == false for a bare-prose ADR mention, got: %+v", r.Issues)
		}
		assertNoCoverageOrIrrelevant(t, r)
	})

	t.Run("110-shaped-spec-passes", func(t *testing.T) {
		tmp := t.TempDir()
		writeTestADRWithDomains(t, tmp, "ADR-0031", "Accepted", "workflow", "")
		writeTestADRWithDomains(t, tmp, "ADR-0037", "Accepted", "workflow", "")
		writeSpecFixture(t, tmp,
			"- workflow: touched by tests\n",
			"- [ADR-0031-doc-sync-gate.md](../../adr/ADR-0031-doc-sync-gate.md): governs domain resolution.\n"+
				"- [ADR-0037](../../adr/ADR-0037-panel-gate.md): governs the panel gate.\n"+
				"Also mentions ADR-9999 and ADR-0040 in prose (not yet files at authoring time).\n")

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "adr-touchpoint-missing"); found {
			t.Errorf("expected no adr-touchpoint-missing for a 110-shaped spec, got: %+v", r.Issues)
		}
		if r.HasFailures() {
			t.Errorf("expected HasFailures() == false for a 110-shaped spec, got: %+v", r.Issues)
		}
		assertNoCoverageOrIrrelevant(t, r)
	})
}

// --- Spec 122 Bead 1: forward-only Rule-2 authoring reject fixtures ---

// writeSpecFixtureWithFrontmatter is writeSpecFixture extended with a
// caller-declared spec.md YAML frontmatter block (spec 122 R1's
// forward-only signal). frontmatter == "" omits the `---`-fenced block
// entirely — the no-frontmatter legacy shape (SpecStatus == ""); pass a
// block with no `status:` key (e.g. "approved_at: \"\"\n") to exercise the
// other status-less shape, or "status: Draft\n" / "status: Approved\n" for
// the explicit-status shapes.
func writeSpecFixtureWithFrontmatter(t *testing.T, root, frontmatter, impactedDomainsBody, adrTouchpointsBody string) {
	t.Helper()
	specDir := filepath.Join(root, "docs", "specs", "999-test")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}

	var b strings.Builder
	if frontmatter != "" {
		b.WriteString("---\n")
		b.WriteString(frontmatter)
		b.WriteString("---\n\n")
	}
	b.WriteString("# Spec 999-test\n\n" +
		"## Goal\n\nDo something useful.\n\n" +
		"## Impacted Domains\n\n" + impactedDomainsBody + "\n" +
		"## ADR Touchpoints\n\n" + adrTouchpointsBody + "\n" +
		"## Requirements\n\n1. First requirement\n2. Second requirement\n\n" +
		"## Scope\n\n### In Scope\n- something\n\n### Out of Scope\n- something else\n\n" +
		"## Acceptance Criteria\n\n- [ ] First criterion\n- [ ] Second criterion\n- [ ] Third criterion\n\n" +
		"## Open Questions\n\nNone\n\n" +
		"## Approval\n\n- **Status**: DRAFT\n")

	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
}

// TestImpactedDomainsForwardOnlyReject_SpecApprove pins spec 122 R1 at the
// spec-approve authoring gate: AC-1 (#178 repro, forward, RED before this
// bead), AC-1b(i) (explicit-status grandfather: Approved / no-frontmatter /
// frontmatter-no-status-key all emit nothing), AC-2 (the first remedy
// applied verbatim completes the red->green transition), and AC-4 (a
// manifest-less workspace never newly fails, the anti-overreach guard).
func TestImpactedDomainsForwardOnlyReject_SpecApprove(t *testing.T) {
	const bareLabel = "api (orders — models)"
	adrBody := "- [ADR-0001](../../adr/ADR-0001.md): relevant\n"

	t.Run("draft-bare-label-rejects", func(t *testing.T) {
		// AC-1 (RED today: zero product changes -> zero
		// impacted-domains-resolve issues here; this subtest only holds
		// green once bareUnresolvedImpactedDomains + the Draft-status
		// severity gate exist).
		tmp := t.TempDir()
		writeManifest(t, tmp, "orders", "paths:\n  - src/orders/**\n")
		writeTestADRWithDomains(t, tmp, "ADR-0001", "Accepted", "core", "")
		writeSpecFixtureWithFrontmatter(t, tmp, "status: Draft\n",
			"- "+bareLabel+": touched by tests\n", adrBody)

		r := ValidateSpec(tmp, "999-test")

		var matches []*Issue
		for i := range r.Issues {
			if r.Issues[i].Name == "impacted-domains-resolve" {
				matches = append(matches, &r.Issues[i])
			}
		}
		if len(matches) != 1 {
			t.Fatalf("expected exactly 1 impacted-domains-resolve issue, got %d: %+v", len(matches), r.Issues)
		}
		msg := matches[0].Message
		if matches[0].Severity != SevError {
			t.Errorf("expected SevError, got %v", matches[0].Severity)
		}
		if !strings.Contains(msg, "orders") {
			t.Errorf("expected message to list available domain %q, got: %s", "orders", msg)
		}
		if !strings.Contains(msg, bareLabel) {
			t.Errorf("expected message to name the offending entry verbatim, got: %s", msg)
		}
		if !strings.Contains(msg, "replacing the entry with one of those names") {
			t.Errorf("expected message to carry the replace-label remedy, got: %s", msg)
		}
		if !strings.Contains(msg, "declaring a claimed path instead") {
			t.Errorf("expected message to carry the declare-a-path remedy, got: %s", msg)
		}
	})

	t.Run("approved-bare-label-grandfathered", func(t *testing.T) {
		// AC-1b(i): an already-Approved spec with the SAME bare label
		// emits NO impacted-domains-resolve error.
		tmp := t.TempDir()
		writeManifest(t, tmp, "orders", "paths:\n  - src/orders/**\n")
		writeTestADRWithDomains(t, tmp, "ADR-0001", "Accepted", "core", "")
		writeSpecFixtureWithFrontmatter(t, tmp, "status: Approved\n",
			"- "+bareLabel+": touched by tests\n", adrBody)

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "impacted-domains-resolve"); found {
			t.Errorf("expected no impacted-domains-resolve for an Approved spec, got: %+v", r.Issues)
		}
	})

	t.Run("no-frontmatter-bare-label-grandfathered", func(t *testing.T) {
		// AC-1b(i): a spec with NO YAML frontmatter at all (SpecStatus =="")
		// emits NO impacted-domains-resolve error — the pre-frontmatter
		// legacy-spec shape.
		tmp := t.TempDir()
		writeManifest(t, tmp, "orders", "paths:\n  - src/orders/**\n")
		writeTestADRWithDomains(t, tmp, "ADR-0001", "Accepted", "core", "")
		writeSpecFixtureWithFrontmatter(t, tmp, "",
			"- "+bareLabel+": touched by tests\n", adrBody)

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "impacted-domains-resolve"); found {
			t.Errorf("expected no impacted-domains-resolve for a frontmatter-less spec, got: %+v", r.Issues)
		}
	})

	t.Run("no-status-key-bare-label-grandfathered", func(t *testing.T) {
		// AC-1b(i): frontmatter present but no `status:` key (SpecStatus
		// =="") emits NO impacted-domains-resolve error — the
		// 038/039/041-shape legacy fixture.
		tmp := t.TempDir()
		writeManifest(t, tmp, "orders", "paths:\n  - src/orders/**\n")
		writeTestADRWithDomains(t, tmp, "ADR-0001", "Accepted", "core", "")
		writeSpecFixtureWithFrontmatter(t, tmp, "spec_id: \"999-test\"\n",
			"- "+bareLabel+": touched by tests\n", adrBody)

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "impacted-domains-resolve"); found {
			t.Errorf("expected no impacted-domains-resolve for a status-key-less spec, got: %+v", r.Issues)
		}
	})

	t.Run("draft-remedy-applied-passes", func(t *testing.T) {
		// AC-2: applying the first remedy verbatim (entry -> "orders")
		// completes the red->green transition, no other edit.
		tmp := t.TempDir()
		writeManifest(t, tmp, "orders", "paths:\n  - src/orders/**\n")
		writeTestADRWithDomains(t, tmp, "ADR-0001", "Accepted", "core", "")
		writeSpecFixtureWithFrontmatter(t, tmp, "status: Draft\n",
			"- orders: touched by tests\n", adrBody)

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "impacted-domains-resolve"); found {
			t.Errorf("expected no impacted-domains-resolve after applying the remedy, got: %+v", r.Issues)
		}
	})

	t.Run("draft-manifest-less-workspace-passes", func(t *testing.T) {
		// AC-4: a workspace with NO loadable OWNERSHIP.yaml anywhere never
		// newly fails, even for a Draft spec with a bare label — the
		// anti-overreach guard (ADR-0036's manifest-less carve-out).
		tmp := t.TempDir()
		writeSpecFixtureWithFrontmatter(t, tmp, "status: Draft\n",
			"- "+bareLabel+": touched by tests\n", "")

		r := ValidateSpec(tmp, "999-test")

		if found, _ := findSpecIssue(r, "impacted-domains-resolve"); found {
			t.Errorf("expected no impacted-domains-resolve in a manifest-less workspace, got: %+v", r.Issues)
		}
	})
}
