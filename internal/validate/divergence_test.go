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

	r, findings := ValidateDivergence(mock, root, specDir, "mindspec-zy4u.2", "BASE", "HEAD", "", false)
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

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
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

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
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

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
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

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
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

// proposedCoverageFixture builds the shared mindspec-53qx scenario: a
// cited Proposed ADR covering the changed file's domain.
func proposedCoverageFixture(t *testing.T) (root, specDir string, mock *executor.MockExecutor) {
	t.Helper()
	root = t.TempDir()
	specDir = filepath.Join(root, ".mindspec", "docs", "specs", "106-proposed")
	writeSpecAndPlan(t, root, specDir, "106-proposed",
		[]string{"payments"},
		[]string{"ADR-0099"},
	)
	writeADR(t, root, "ADR-0099", "Proposed", []string{"payments"})
	writeManifest(t, root, "payments", "paths:\n  - internal/payments/**\n")
	mock = &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/payments/charge.go"},
	}
	return root, specDir, mock
}

// TestValidateDivergenceProposedCitedWarnsAtBeadComplete — mindspec-53qx
// + panel condition C1 at the bead-complete lane: a cited Proposed ADR
// covering the changed file's domain satisfies the divergence check (no
// failure — the bead being completed is precisely the implementation
// that validates the Proposed ADR) but surfaces the advisory
// adr-divergence-proposed WARNING so the pending status flip is
// visible, not silent.
func TestValidateDivergenceProposedCitedWarnsAtBeadComplete(t *testing.T) {
	root, specDir, mock := proposedCoverageFixture(t)

	r, findings := ValidateDivergence(mock, root, specDir, "mindspec-bead.1", "BASE", "HEAD", "", false)
	if r == nil {
		t.Fatal("nil result")
	}
	if r.HasFailures() {
		t.Errorf("expected no failures with cited Proposed covering ADR at bead-complete, got %+v", r.Issues)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %+v", findings)
	}
	var warned bool
	for _, i := range r.Issues {
		if i.Name == "adr-divergence-proposed" {
			warned = true
			if i.Severity != SevWarning {
				t.Errorf("adr-divergence-proposed severity = %v at bead-complete, want warning", i.Severity)
			}
			for _, want := range []string{"payments", "ADR-0099", "flip it to Accepted"} {
				if !strings.Contains(i.Message, want) {
					t.Errorf("warning message missing %q: %s", want, i.Message)
				}
			}
		}
	}
	if !warned {
		t.Errorf("expected adr-divergence-proposed warning at bead-complete, got %+v", r.Issues)
	}
}

// TestValidateDivergenceProposedCitedErrorsAtImplApprove — panel
// condition C1's other half: the same Proposed-only coverage that is
// tolerated mid-implementation is an ERROR at the impl-approve
// backstop, closing the lifecycle loop. The message names the ADR and
// points at the existing --override-adr escape.
func TestValidateDivergenceProposedCitedErrorsAtImplApprove(t *testing.T) {
	root, specDir, mock := proposedCoverageFixture(t)

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", true)
	if r == nil {
		t.Fatal("nil result")
	}
	if !r.HasFailures() {
		t.Fatalf("expected failure for Proposed-only coverage at impl-approve, got %+v", r.Issues)
	}
	var errored bool
	for _, i := range r.Issues {
		if i.Name == "adr-divergence-proposed" && i.Severity == SevError {
			errored = true
			for _, want := range []string{"payments", "ADR-0099", "now that the implementation ships", "--override-adr"} {
				if !strings.Contains(i.Message, want) {
					t.Errorf("error message missing %q: %s", want, i.Message)
				}
			}
		}
	}
	if !errored {
		t.Errorf("expected adr-divergence-proposed error at impl-approve, got %+v", r.Issues)
	}
	// Proposed-covered yields NO finding — the supersede-placeholder
	// seed contract is for genuinely uncovered domains.
	if len(findings) != 0 {
		t.Errorf("expected no findings for Proposed-covered domain, got %+v", findings)
	}
}

// TestCheckADRDivergenceLaneSelection pins the wiring: beadID == ""
// (the approve.ApproveImpl backstop) selects impl-approve severity for
// Proposed-only coverage; a non-empty beadID (complete.Run) selects the
// advisory warning.
func TestCheckADRDivergenceLaneSelection(t *testing.T) {
	root, specDir, mock := proposedCoverageFixture(t)

	rImpl, _ := CheckADRDivergence(root, "BASE", mock, specDir, "", "", "")
	if !rImpl.HasFailures() {
		t.Errorf("beadID==\"\" (impl backstop) should fail on Proposed-only coverage, got %+v", rImpl.Issues)
	}

	rBead, _ := CheckADRDivergence(root, "BASE", mock, specDir, "mindspec-bead.1", "", "")
	if rBead.HasFailures() {
		t.Errorf("non-empty beadID (bead-complete) should not fail on Proposed-only coverage, got %+v", rBead.Issues)
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

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
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

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
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

// TestRootOperatorDocsNotUnowned — spec 106 Bead 5 R3: the irreversible
// flatten's move commit repairs links in the repo-root operator docs
// README.md (move commit) and BENCH-MOVED.md (R2 fix). Those are root docs
// (consistent with internal/layout DefaultRootDocs and the internal/doctor
// repo-root *.md link scan, repoRootMarkdown), so isDocFile must classify them
// as documentation and the
// ADR-divergence lane must never emit adr-divergence-unowned for them. A real
// source file in the same diff still surfaces, proving the doc classification
// did not silence the lane wholesale.
func TestRootOperatorDocsNotUnowned(t *testing.T) {
	// Unit guard: the root operator docs classify as docs/process-artifacts,
	// while a real source-adjacent file stays source (negative guard — the
	// fix must NOT be a broad "any top-level .md / anything" rule).
	for _, d := range []string{"README.md", "BENCH-MOVED.md", "CLAUDE.md", "AGENTS.md"} {
		if !isDocFile(d) {
			t.Errorf("isDocFile(%q) = false, want true (repo-root operator doc)", d)
		}
		if !isProcessArtifact(d) {
			t.Errorf("isProcessArtifact(%q) = false, want true", d)
		}
	}
	if isDocFile("internal/foo.go") || isProcessArtifact("internal/foo.go") {
		t.Error("internal/foo.go must NOT classify as doc/process-artifact")
	}
	if !isSourceFile("internal/foo.go") {
		t.Error("internal/foo.go must still classify as source")
	}

	// End-to-end guard: README.md + BENCH-MOVED.md in a divergence diff are
	// skipped before attribution; the control source file is the only finding.
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "106-flatten")
	writeSpecAndPlan(t, root, specDir, "106-flatten",
		[]string{"core"},
		[]string{},
	)
	writeManifest(t, root, "core", "paths:\n  - internal/core/**\n")

	rootDocs := []string{"README.md", "BENCH-MOVED.md"}
	mock := &executor.MockExecutor{
		ChangedFilesResult: append(append([]string{}, rootDocs...),
			"internal/foo.go"), // control: unowned source still surfaces
	}

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
	if r == nil {
		t.Fatal("nil result")
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding (the control source file), got %+v", findings)
	}
	if findings[0].Path != "internal/foo.go" || findings[0].Kind != "unowned" {
		t.Errorf("control finding = %+v, want unowned internal/foo.go", findings[0])
	}
	for _, i := range r.Issues {
		for _, d := range rootDocs {
			if strings.Contains(i.Message, d) {
				t.Errorf("root operator doc %q leaked into issue %+v", d, i)
			}
		}
	}
}

// TestValidateDivergenceFilePathImpactedDomainResolves — spec 100 R1 AC1:
// a spec whose `## Impacted Domains` entry is a FILE PATH (not the
// domain dir name) is normalized to its owning domain, so the changed
// file resolves to that domain and the gate reports ZERO
// adr-divergence-unowned (coveredAccepted → silent pass).
func TestValidateDivergenceFilePathImpactedDomainResolves(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "107-filepath")
	writeSpecAndPlan(t, root, specDir, "107-filepath",
		[]string{"internal/genevieve/review.py"},
		[]string{"ADR-0099"},
	)
	writeManifest(t, root, "genevieve", "paths:\n  - internal/genevieve/**\n")
	writeADR(t, root, "ADR-0099", "Accepted", []string{"genevieve"})

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/genevieve/review.py"},
	}

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
	if r == nil {
		t.Fatal("nil result")
	}
	if r.HasFailures() {
		t.Fatalf("expected no failures (file-path entry should resolve to genevieve), got %+v", r.Issues)
	}
	for _, i := range r.Issues {
		if i.Name == "adr-divergence-unowned" {
			t.Errorf("unexpected adr-divergence-unowned: %+v", i)
		}
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %+v", findings)
	}
}

// TestValidateDivergenceNamedDomainUnchanged — spec 100 R1 AC3: the
// named-domain (099-style) path is unchanged — declaring the owning
// domain BY NAME with the same manifest and citation still resolves and
// still passes, no new false positives.
func TestValidateDivergenceNamedDomainUnchanged(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "108-named")
	writeSpecAndPlan(t, root, specDir, "108-named",
		[]string{"genevieve"},
		[]string{"ADR-0099"},
	)
	writeManifest(t, root, "genevieve", "paths:\n  - internal/genevieve/**\n")
	writeADR(t, root, "ADR-0099", "Accepted", []string{"genevieve"})

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/genevieve/review.py"},
	}

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
	if r == nil {
		t.Fatal("nil result")
	}
	if r.HasFailures() {
		t.Fatalf("expected no failures for named-domain path, got %+v", r.Issues)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %+v", findings)
	}
}

// TestValidateDivergenceGenuinelyUnownedStillFails — spec 100 R1 AC4: a
// genuinely-unowned changed file (no domain manifest paths: matches it)
// still reports adr-divergence-unowned — the gate is correctly scoped,
// not globally disabled. The Impacted-Domains entry here is a valid
// owning path so normalization succeeds; the CHANGED file is the one
// that no manifest claims.
func TestValidateDivergenceGenuinelyUnownedStillFails(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "109-genuine")
	writeSpecAndPlan(t, root, specDir, "109-genuine",
		[]string{"internal/genevieve/review.py"},
		[]string{"ADR-0099"},
	)
	writeManifest(t, root, "genevieve", "paths:\n  - internal/genevieve/**\n")
	writeADR(t, root, "ADR-0099", "Accepted", []string{"genevieve"})

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/genevieve/review.py", "internal/payments/charge.go"},
	}

	r, _ := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
	if r == nil {
		t.Fatal("nil result")
	}
	var hasUnowned bool
	for _, i := range r.Issues {
		if i.Name == "adr-divergence-unowned" &&
			strings.Contains(i.Message, "internal/payments/charge.go") {
			hasUnowned = true
		}
	}
	if !hasUnowned {
		t.Errorf("expected adr-divergence-unowned for the unowned file, got %+v", r.Issues)
	}
}

// TestValidateDivergenceZeroOwnerEntryErrors — spec 100 R1 AC5: an
// Impacted-Domains FILE-PATH entry owned by NO domain surfaces the
// clear normalization ERROR naming the entry.
func TestValidateDivergenceZeroOwnerEntryErrors(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "110-zero")
	writeSpecAndPlan(t, root, specDir, "110-zero",
		[]string{"internal/nope/x.go"},
		[]string{},
	)
	writeManifest(t, root, "genevieve", "paths:\n  - internal/genevieve/**\n")

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/genevieve/review.py"},
	}

	r, _ := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
	if r == nil {
		t.Fatal("nil result")
	}
	var found bool
	for _, i := range r.Issues {
		if i.Name == "impacted-domains-resolve" && strings.Contains(i.Message, "internal/nope/x.go") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected impacted-domains-resolve error naming the zero-owner entry, got %+v", r.Issues)
	}
}

// TestSixOUTwoCompleteShapedCoverageProbePasses pins spec 122 AC-5's
// bead-time half — bead mindspec-6ou2's ACTUAL filed scenario (items
// 3/4): domain dir `orders` claims `src/orders/**`; the spec's
// `## Impacted Domains` is the file-path form `src/orders/api.py`
// (already resolved fine spec-side by spec 100 — this is NOT the
// spec-side failure); the plan cites an Accepted ADR whose `Domain(s)`
// line declares the SAME territory as a directory path, in BOTH slash
// forms across sibling subtests. Before spec 122 R2 the ADR-side
// literal `src/orders/` / `src/orders` never resolved to the
// spec-resolved name `orders`, so the changed file `src/orders/api.py`
// failed the coverage probe (RED today — see the plan-lane sibling
// TestSixOUTwoADRDomainResolvesBothSlashForms in plan_test.go for the
// validate-plan half of the same repro). After R2 both forms resolve
// and the bead-time coverage probe passes with NO --override-adr.
func TestSixOUTwoCompleteShapedCoverageProbePasses(t *testing.T) {
	cases := []struct {
		name  string
		label string
	}{
		{"with trailing slash", "src/orders/"},
		{"without trailing slash", "src/orders"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			specDir := filepath.Join(root, ".mindspec", "docs", "specs", "999-sixou2")
			writeSpecAndPlan(t, root, specDir, "999-sixou2",
				[]string{"src/orders/api.py"},
				[]string{"ADR-0090"},
			)
			writeManifest(t, root, "orders", "paths:\n  - src/orders/**\n")
			writeADR(t, root, "ADR-0090", "Accepted", []string{tc.label})

			mock := &executor.MockExecutor{
				ChangedFilesResult: []string{"src/orders/api.py"},
			}

			r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
			if r == nil {
				t.Fatal("nil result")
			}
			if r.HasFailures() {
				t.Fatalf("label %q: expected zero-override coverage pass, got %+v", tc.label, r.Issues)
			}
			if len(findings) != 0 {
				t.Errorf("label %q: expected no findings, got %+v", tc.label, findings)
			}
		})
	}
}

// Test147EndToEndZeroErrors is the genuinely-new R5a pin (spec 122
// AC-7): the FULL #147 shape through the complete-shaped divergence
// lane. Spec Impacted Domains are file paths (`genevieve/review.py`,
// `genevieve/summarizer.py`); domain dir `genevieve` claims
// `genevieve/**/*.py` AND `.github/workflows/code-review.yaml`; the
// cited Accepted ADR's `Domain(s)` line lists those SAME file-path
// strings (not the domain name); the bead's diff touches
// `genevieve/summarizer.py` and `.github/workflows/code-review.yaml`.
//
// Before spec 122 R2: the spec side resolves fine (spec 100 — pinned
// standalone, cited not re-authored, by
// TestValidateDivergenceFilePathImpactedDomainResolves above), but the
// ADR's literal `Domain(s)` entries never equal the resolved domain
// name `genevieve`, so the coverage probe fails for both changed files
// (RED today at the coverage step — the #147 coverage tail). After R2,
// the ADR-side entries resolve to `genevieve` (a bare file path claimed
// by exactly one domain's OWNERSHIP paths:) and the lane returns ZERO
// errors: no unowned finding, no coverage failure, no override needed.
func Test147EndToEndZeroErrors(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "999-genevieve147")
	writeSpecAndPlan(t, root, specDir, "999-genevieve147",
		[]string{"genevieve/review.py", "genevieve/summarizer.py"},
		[]string{"ADR-0147"},
	)
	writeManifest(t, root, "genevieve",
		"paths:\n  - genevieve/**/*.py\n  - .github/workflows/code-review.yaml\n")
	writeADR(t, root, "ADR-0147", "Accepted",
		[]string{"genevieve/review.py", "genevieve/summarizer.py"})

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"genevieve/summarizer.py", ".github/workflows/code-review.yaml"},
	}

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
	if r == nil {
		t.Fatal("nil result")
	}
	if r.HasFailures() {
		t.Fatalf("expected zero errors for the full #147 shape, got %+v", r.Issues)
	}
	for _, i := range r.Issues {
		if i.Name == "adr-divergence-unowned" {
			t.Errorf("unexpected adr-divergence-unowned: %+v", i)
		}
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %+v", findings)
	}
}

// TestValidateDivergenceUnownedSplitNamesRealOwner — spec 122 AC-11
// (R4b, #178 option 2): when a changed file IS claimed by some
// domain's OWNERSHIP but that domain is NOT in the spec's resolved
// DECLARED Impacted Domains, the adr-divergence-unowned message must
// name the REAL owning domain and the add-to-Impacted-Domains remedy —
// not the unqualified "not claimed by any OWNERSHIP.yaml" text. A
// sibling file claimed by NO manifest at all still gets the
// genuinely-unowned message. Both remain ERRORS under the same finding
// code and the same --override-adr/--supersede-adr escapes — the
// PASS/FAIL boundary is unchanged, only the message truth changes.
func TestValidateDivergenceUnownedSplitNamesRealOwner(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "999-scope-drift")
	writeSpecAndPlan(t, root, specDir, "999-scope-drift",
		[]string{"workflow"},
		[]string{},
	)
	writeManifest(t, root, "workflow", "paths:\n  - internal/validate/**\n")
	writeManifest(t, root, "execution", "paths:\n  - internal/gitutil/**\n")

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/gitutil/x.go", "internal/nope/y.go"},
	}

	r, findings := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
	if r == nil {
		t.Fatal("nil result")
	}
	if !r.HasFailures() {
		t.Fatalf("expected failures, got %+v", r.Issues)
	}

	var ownedMsg, unownedMsg string
	for _, i := range r.Issues {
		if i.Name != "adr-divergence-unowned" {
			continue
		}
		if strings.Contains(i.Message, "internal/gitutil/x.go") {
			ownedMsg = i.Message
		}
		if strings.Contains(i.Message, "internal/nope/y.go") {
			unownedMsg = i.Message
		}
	}
	if ownedMsg == "" {
		t.Fatalf("expected an adr-divergence-unowned finding for internal/gitutil/x.go, got %+v", r.Issues)
	}
	if !strings.Contains(ownedMsg, "execution") {
		t.Errorf("expected the real owner 'execution' named, got: %q", ownedMsg)
	}
	if !strings.Contains(strings.ToLower(ownedMsg), "impacted domains") {
		t.Errorf("expected the add-to-Impacted-Domains remedy, got: %q", ownedMsg)
	}
	if strings.Contains(ownedMsg, "not claimed by any OWNERSHIP.yaml") {
		t.Errorf("owned-but-undeclared file must NOT get the unqualified genuinely-unowned text, got: %q", ownedMsg)
	}

	if unownedMsg == "" {
		t.Fatalf("expected a genuinely-unowned finding for internal/nope/y.go, got %+v", r.Issues)
	}
	if !strings.Contains(unownedMsg, "not claimed by any OWNERSHIP.yaml") {
		t.Errorf("expected the genuinely-unowned message for a file no manifest claims, got: %q", unownedMsg)
	}

	if len(findings) != 2 {
		t.Fatalf("expected 2 unowned findings, got %+v", findings)
	}
	for _, f := range findings {
		if f.Kind != "unowned" {
			t.Errorf("expected Kind=unowned for both findings, got %+v", f)
		}
		if f.Domain != "" || f.ManifestPath != "" {
			t.Errorf("expected empty Domain/ManifestPath on unowned findings, got %+v", f)
		}
	}
}

// TestValidateDivergenceUnownedHintLayoutAware — spec 122 AC-9 (R4a),
// the divergence half: a genuinely-unowned finding's claim-it remedy
// must print the domains root that ACTUALLY resolves in the operator's
// workspace. In a FLATTENED workspace (.mindspec/domains/ present,
// .mindspec/docs/domains/ absent) it prints `.mindspec/domains/...` and
// never the substring `.mindspec/docs/domains`; in a PRE-flatten
// (canonical) workspace it prints `.mindspec/docs/domains/...`.
func TestValidateDivergenceUnownedHintLayoutAware(t *testing.T) {
	t.Run("flattened", func(t *testing.T) {
		root := t.TempDir()
		specDir := filepath.Join(root, ".mindspec", "docs", "specs", "999-flat")
		writeSpecAndPlan(t, root, specDir, "999-flat",
			[]string{"core"},
			[]string{},
		)
		writeFlatManifest(t, root, "core", "paths:\n  - internal/core/**\n")

		mock := &executor.MockExecutor{
			ChangedFilesResult: []string{"internal/payments/charge.go"},
		}

		r, _ := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
		if r == nil {
			t.Fatal("nil result")
		}
		var msg string
		for _, i := range r.Issues {
			if i.Name == "adr-divergence-unowned" {
				msg = i.Message
			}
		}
		if msg == "" {
			t.Fatalf("expected adr-divergence-unowned, got %+v", r.Issues)
		}
		if !strings.Contains(msg, ".mindspec/domains/") {
			t.Errorf("expected the flat domains root in the hint, got: %q", msg)
		}
		if strings.Contains(msg, ".mindspec/docs/domains") {
			t.Errorf("flat workspace hint must NOT contain the canonical literal, got: %q", msg)
		}
	})

	t.Run("pre-flatten (canonical)", func(t *testing.T) {
		root := t.TempDir()
		specDir := filepath.Join(root, ".mindspec", "docs", "specs", "999-canonical")
		writeSpecAndPlan(t, root, specDir, "999-canonical",
			[]string{"core"},
			[]string{},
		)
		writeManifest(t, root, "core", "paths:\n  - internal/core/**\n")

		mock := &executor.MockExecutor{
			ChangedFilesResult: []string{"internal/payments/charge.go"},
		}

		r, _ := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
		if r == nil {
			t.Fatal("nil result")
		}
		var msg string
		for _, i := range r.Issues {
			if i.Name == "adr-divergence-unowned" {
				msg = i.Message
			}
		}
		if msg == "" {
			t.Fatalf("expected adr-divergence-unowned, got %+v", r.Issues)
		}
		if !strings.Contains(msg, ".mindspec/docs/domains/") {
			t.Errorf("expected the canonical domains root in the hint, got: %q", msg)
		}
	})
}

// TestValidateDivergenceUnownedIndeterminateOnBrokenSibling — spec 122
// Bead 3 FX-1 (gvb5.3 codex): the R4(b) full-enumeration re-attribution
// must NEVER classify a file as "genuinely unowned" when ownership is
// INDETERMINATE because a domain's OWNERSHIP.yaml failed to load. A
// malformed sibling manifest (`a-broken`, sorted BEFORE the declared
// owner) is outside the spec's declared candidate set, so the
// declared-set attribution pass never touches it — only the FX-1
// full-enumeration re-attribution hits it. Before the fix the
// attribution error was swallowed (`_, _, _`) and the gate confidently
// LIED "not claimed by any OWNERSHIP.yaml"; after, the load failure is
// surfaced with its remedy and the confident-unowned message is never
// emitted. Pass/fail boundary unchanged (still a SevError, overridable
// via --override-adr exactly as today).
func TestValidateDivergenceUnownedIndeterminateOnBrokenSibling(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "999-broken-sibling")
	writeSpecAndPlan(t, root, specDir, "999-broken-sibling",
		[]string{"workflow"},
		[]string{},
	)
	// Declared owner claims internal/validate/**, NOT gitutil — so the
	// changed file reaches the genuinely-unowned branch under the
	// DECLARED set (workflow's manifest is valid, no attribution error).
	writeManifest(t, root, "workflow", "paths:\n  - internal/validate/**\n")
	// Malformed sibling manifest that sorts before workflow: unterminated
	// YAML flow sequence → LoadOwnership returns a parse error.
	writeManifest(t, root, "a-broken", "paths: [ unterminated\n")

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/gitutil/x.go"},
	}

	r, _ := ValidateDivergence(mock, root, specDir, "", "BASE", "HEAD", "", false)
	if r == nil {
		t.Fatal("nil result")
	}
	if !r.HasFailures() {
		t.Fatalf("expected a failure (indeterminate ownership stays a blocking error), got %+v", r.Issues)
	}

	var attrMsg, unownedMsg string
	for _, i := range r.Issues {
		if i.Name == "adr-divergence-attribute" && strings.Contains(i.Message, "internal/gitutil/x.go") {
			attrMsg = i.Message
		}
		if i.Name == "adr-divergence-unowned" && strings.Contains(i.Message, "internal/gitutil/x.go") {
			unownedMsg = i.Message
		}
	}
	if attrMsg == "" {
		t.Fatalf("expected an adr-divergence-attribute (indeterminate ownership) error naming the file, got %+v", r.Issues)
	}
	if !strings.Contains(attrMsg, "could not be") {
		t.Errorf("expected the indeterminate message to name the load failure, got: %q", attrMsg)
	}
	if unownedMsg != "" && strings.Contains(unownedMsg, "not claimed by any OWNERSHIP.yaml") {
		t.Errorf("indeterminate ownership must NOT be reported as genuinely unowned, got: %q", unownedMsg)
	}
}

// TestValidateDivergenceUnownedHintRefConsistentRoot — spec 122 Bead 3
// FX-2 (gvb5.3 codex): in the ref-anchored divergence lane the
// genuinely-unowned hint's domains root must be resolved from the SAME
// tree the ownership enumeration read (ownerRef), not the ambient
// checkout. Here the ambient tree is CANONICAL (writeSpecAndPlan
// materializes .mindspec/docs/, and no .mindspec/domains/ on disk → the
// ambient label would be .mindspec/docs/domains) while the inspected
// REF is FLAT (domains live at .mindspec/domains/). The hint must name
// the REF's flat root and never the ambient canonical substring.
func TestValidateDivergenceUnownedHintRefConsistentRoot(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "999-ref-flat")
	// Empty Impacted Domains → the candidate set falls back to the
	// ref enumeration (listDomainDirsAtRef), driven entirely by the mock.
	writeSpecAndPlan(t, root, specDir, "999-ref-flat", nil, []string{})

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/payments/charge.go"},
		TreeDirsAtRefFn: func(ref, dirPath string) ([]string, error) {
			if dirPath == ".mindspec/domains" {
				return []string{"core"}, nil // the ref is FLAT
			}
			return nil, nil // canonical/legacy roots are empty at the ref
		},
		// FileAtRefOrAbsent defaults to absent → `core` claims nothing →
		// the changed file is genuinely unowned, with no load error.
	}

	r, _ := ValidateDivergence(mock, root, specDir, "mindspec-x.1", "BASE", "HEAD", "REFSHA", false)
	if r == nil {
		t.Fatal("nil result")
	}
	var msg string
	for _, i := range r.Issues {
		if i.Name == "adr-divergence-unowned" {
			msg = i.Message
		}
	}
	if msg == "" {
		t.Fatalf("expected an adr-divergence-unowned hint, got %+v", r.Issues)
	}
	if !strings.Contains(msg, ".mindspec/domains/") {
		t.Errorf("expected the REF's flat domains root in the hint, got: %q", msg)
	}
	if strings.Contains(msg, ".mindspec/docs/domains") {
		t.Errorf("ref-anchored hint must NOT print the ambient canonical root, got: %q", msg)
	}
}
