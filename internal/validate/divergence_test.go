package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// writeSpecAndPlan writes a minimal spec.md + plan.md pair under the
// given specDir. impactedDomains populates the spec's `## Impacted
// Domains` bulleted list; citationIDs populates the plan frontmatter's
// `adr_citations:` scalar list. Both files are written with the
// frontmatter shape ValidateDivergence's loaders expect.
func writeSpecAndPlan(t *testing.T, root, specDir, specID string, impactedDomains, citationIDs []string) {
	t.Helper()
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", specDir, err)
	}

	var domBullets strings.Builder
	for _, d := range impactedDomains {
		fmt.Fprintf(&domBullets, "- %s\n", d)
	}
	specBody := fmt.Sprintf(`# Spec %s
## Goal
test fixture
## Impacted Domains
%s`, specID, domBullets.String())
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specBody), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	var citeLines strings.Builder
	for _, id := range citationIDs {
		fmt.Fprintf(&citeLines, "  - %s\n", id)
	}
	planBody := fmt.Sprintf(`---
status: Draft
spec_id: %s
version: 1
adr_citations:
%s---

## Bead 1
**Steps**
1. do
2. do
3. do
**Verification**
- [ ] check
**Acceptance Criteria**
done
`, specID, citeLines.String())
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(planBody), 0o644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}
}

// writeADR writes a minimal ADR-NNNN.md under .mindspec/docs/adr/ with
// the given status and domains. The body is parseable by
// internal/adr.ParseADR.
func writeADR(t *testing.T, root, id, status string, domains []string) {
	t.Helper()
	adrDir := filepath.Join(root, ".mindspec", "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("mkdir adr dir: %v", err)
	}
	body := fmt.Sprintf(`# %s: Test

**Date**: 2026-01-01
**Status**: %s
**Domain(s)**: %s
**Supersedes**: n/a
**Superseded-by**: n/a

## Context

test fixture
`, id, status, strings.Join(domains, ", "))
	path := filepath.Join(adrDir, id+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write adr: %v", err)
	}
}

// TestCompleteRejectsUndeclaredDomainTouch — the canonical Spec 087 Bead
// 2 acceptance scenario. The bead's diff touches
// internal/payments/charge.go, the plan cites an ADR scoped only to
// "search", and ValidateDivergence must surface an
// `adr-divergence-uncovered` failure on the payments domain. Per the
// outer-instruction note this test lives in divergence_test.go (the
// alternative location the instructions explicitly permit).
func TestCompleteRejectsUndeclaredDomainTouch(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "100-undeclared")
	writeSpecAndPlan(t, root, specDir, "100-undeclared",
		[]string{"payments", "search"},
		[]string{"ADR-0099"},
	)
	writeADR(t, root, "ADR-0099", "Accepted", []string{"search"})
	writeManifest(t, root, "payments", "paths:\n  - internal/payments/**\n")
	writeManifest(t, root, "search", "paths:\n  - internal/search/**\n")

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/payments/charge.go"},
	}

	r, findings := ValidateDivergence(mock, root, specDir, "mindspec-zy4u.2", "BASE", "HEAD")
	if r == nil {
		t.Fatal("nil result")
	}
	if !r.HasFailures() {
		t.Fatalf("expected uncovered failure, got %+v", r.Issues)
	}
	var hasUncovered bool
	for _, i := range r.Issues {
		if i.Name == "adr-divergence-uncovered" &&
			strings.Contains(i.Message, "internal/payments/charge.go") &&
			strings.Contains(i.Message, "payments") {
			hasUncovered = true
			break
		}
	}
	if !hasUncovered {
		t.Errorf("expected adr-divergence-uncovered issue naming file+domain, got %+v", r.Issues)
	}
	if len(findings) != 1 || findings[0].Kind != "uncovered" {
		t.Errorf("expected 1 uncovered finding, got %+v", findings)
	}
	if findings[0].Domain != "payments" {
		t.Errorf("finding.Domain = %q, want payments", findings[0].Domain)
	}
}

// TestVizAgentmindBenchFiltered confirms HC-4 layer 2: files whose
// first path segment is viz/, agentmind/, or bench/ are dropped before
// attribution. No Issues, no findings — the diff is effectively empty
// from the gate's perspective.
func TestVizAgentmindBenchFiltered(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "101-filter")
	writeSpecAndPlan(t, root, specDir, "101-filter",
		[]string{"core"},
		[]string{},
	)
	writeManifest(t, root, "core", "paths:\n  - internal/core/**\n")

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{
			"viz/foo.go",
			"agentmind/bar.go",
			"bench/v2/baz.go",
		},
	}

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD")
	if r == nil {
		t.Fatal("nil result")
	}
	if r.HasFailures() {
		t.Errorf("expected no failures, got %+v", r.Issues)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %+v", findings)
	}
}

// TestUnownedFileRejected exercises the no-owning-domain branch: the
// diff touches a path that no OWNERSHIP.yaml for any
// spec-impacted-domain claims. Surfaces as `adr-divergence-unowned`
// plus a structured DivergenceFinding with Kind="unowned" and empty
// Domain/ManifestPath.
func TestUnownedFileRejected(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "102-unowned")
	writeSpecAndPlan(t, root, specDir, "102-unowned",
		[]string{"core"},
		[]string{},
	)
	// Only `core` is impacted, and its manifest does not cover
	// payments. The diff touches a payments file → no domain claim →
	// unowned.
	writeManifest(t, root, "core", "paths:\n  - internal/core/**\n")

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/payments/charge.go"},
	}

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD")
	if r == nil {
		t.Fatal("nil result")
	}
	if !r.HasFailures() {
		t.Fatalf("expected unowned failure, got %+v", r.Issues)
	}
	var hasUnowned bool
	for _, i := range r.Issues {
		if i.Name == "adr-divergence-unowned" &&
			strings.Contains(i.Message, "internal/payments/charge.go") {
			hasUnowned = true
			break
		}
	}
	if !hasUnowned {
		t.Errorf("expected adr-divergence-unowned issue naming file, got %+v", r.Issues)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %+v", findings)
	}
	if findings[0].Kind != "unowned" {
		t.Errorf("Kind = %q, want unowned", findings[0].Kind)
	}
	if findings[0].Domain != "" || findings[0].ManifestPath != "" {
		t.Errorf("expected empty Domain/ManifestPath, got %+v", findings[0])
	}
}

// TestValidateDivergenceCoveredDomainPasses confirms the happy path:
// the diff touches a file in a domain that IS covered by a cited
// Accepted ADR — no failures, no findings.
func TestValidateDivergenceCoveredDomainPasses(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "103-ok")
	writeSpecAndPlan(t, root, specDir, "103-ok",
		[]string{"payments"},
		[]string{"ADR-0099"},
	)
	writeADR(t, root, "ADR-0099", "Accepted", []string{"payments"})
	writeManifest(t, root, "payments", "paths:\n  - internal/payments/**\n")

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/payments/charge.go"},
	}

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD")
	if r == nil {
		t.Fatal("nil result")
	}
	if r.HasFailures() {
		t.Errorf("expected no failures, got %+v", r.Issues)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %+v", findings)
	}
}

// TestValidateDivergenceSpecBranchADRVisible — mindspec-ew79 mirror of
// the plan-lane test: an ADR that exists only inside the spec worktree
// (the spec branch) must satisfy the bead-complete divergence check run
// from the primary checkout. Before the overlay store the FileStore was
// rooted at the primary tree only, so the cited spec-branch ADR was
// invisible and the changed file surfaced as adr-divergence-uncovered.
func TestValidateDivergenceSpecBranchADRVisible(t *testing.T) {
	root := t.TempDir()
	wtTree := filepath.Join(root, ".worktrees", "worktree-spec-105-wt")
	specDir := filepath.Join(wtTree, ".mindspec", "docs", "specs", "105-wt")
	writeSpecAndPlan(t, root, specDir, "105-wt",
		[]string{"payments"},
		[]string{"ADR-0099"},
	)
	// ADR lives ONLY in the spec worktree tree (writeADR joins
	// <arg>/.mindspec/docs/adr).
	writeADR(t, wtTree, "ADR-0099", "Accepted", []string{"payments"})
	writeManifest(t, root, "payments", "paths:\n  - internal/payments/**\n")

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/payments/charge.go"},
	}

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD")
	if r == nil {
		t.Fatal("nil result")
	}
	if r.HasFailures() {
		t.Errorf("expected no failures with spec-branch ADR visible, got %+v", r.Issues)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %+v", findings)
	}
}

// TestValidateDivergenceDiffErrorSurfaces confirms a ChangedFiles
// failure surfaces as `adr-divergence-diff` and short-circuits with
// nil findings.
func TestValidateDivergenceDiffErrorSurfaces(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "104-differr")
	writeSpecAndPlan(t, root, specDir, "104-differr",
		[]string{"core"},
		[]string{},
	)

	mock := &executor.MockExecutor{
		ChangedFilesErr: fmt.Errorf("boom"),
	}

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD")
	if r == nil {
		t.Fatal("nil result")
	}
	if !r.HasFailures() {
		t.Fatal("expected failure")
	}
	if r.Issues[0].Name != "adr-divergence-diff" {
		t.Errorf("Name = %q, want adr-divergence-diff", r.Issues[0].Name)
	}
	if findings != nil {
		t.Errorf("expected nil findings, got %+v", findings)
	}
}

// TestProcessArtifactsFiltered confirms the spec-092 fix-up: non-source
// process artifacts (.beads/, .mindspec/docs/** and the rest of the
// doc-sync isDocFile set, review/) are dropped before attribution and
// can never surface as adr-divergence-unowned. A control source file in
// the same diff still surfaces, proving the filter does not silence the
// lane wholesale.
func TestProcessArtifactsFiltered(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "105-artifacts")
	writeSpecAndPlan(t, root, specDir, "105-artifacts",
		[]string{"core"},
		[]string{},
	)
	writeManifest(t, root, "core", "paths:\n  - internal/core/**\n")

	artifacts := []string{
		".beads/issues.jsonl",
		".mindspec/docs/adr/ADR-0099-test.md",
		".mindspec/docs/specs/105-artifacts/spec.md",
		".mindspec/docs/specs/105-artifacts/plan.md",
		".mindspec/docs/core/WORKFLOW-STATE-MACHINE.md",
		".mindspec/docs/domains/core/OWNERSHIP.yaml",
		"review/prep/bead2_baseline_evidence.md",
		"docs/extra.md",
		"CLAUDE.md",
		"AGENTS.md",
	}
	mock := &executor.MockExecutor{
		ChangedFilesResult: append(append([]string{}, artifacts...),
			"internal/payments/charge.go"), // control: still unowned
	}

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD")
	if r == nil {
		t.Fatal("nil result")
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding (the control file), got %+v", findings)
	}
	if findings[0].Path != "internal/payments/charge.go" || findings[0].Kind != "unowned" {
		t.Errorf("control finding = %+v, want unowned internal/payments/charge.go", findings[0])
	}
	for _, i := range r.Issues {
		for _, a := range artifacts {
			if strings.Contains(i.Message, a) {
				t.Errorf("process artifact %q leaked into issue %+v", a, i)
			}
		}
	}
}
