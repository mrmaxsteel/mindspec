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
// (consistent with internal/layout DefaultRootDocs and internal/doctor
// movedTreeRootDocs), so isDocFile must classify them as documentation and the
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
