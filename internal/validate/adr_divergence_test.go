package validate

import (
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// TestCheckADRDivergenceEmptySpecDir confirms the new Spec 087 signature
// degrades gracefully when no spec dir is supplied: a single
// `adr-divergence-load` failure and a nil findings slice. The
// SubCommand label is preserved across the transition from the spec-086
// stub (HC-3 traceability).
func TestCheckADRDivergenceEmptySpecDir(t *testing.T) {
	r, findings := CheckADRDivergence("/tmp/root", "HEAD~1", &executor.MockExecutor{}, "", "", "", "")
	if r == nil {
		t.Fatal("CheckADRDivergence returned nil *Result")
	}
	if r.SubCommand != "adr-divergence" {
		t.Errorf("SubCommand = %q, want %q", r.SubCommand, "adr-divergence")
	}
	if findings != nil {
		t.Errorf("expected nil findings, got %+v", findings)
	}
	if !r.HasFailures() {
		t.Fatal("expected failure for empty specDir")
	}
	gotName := r.Issues[0].Name
	if gotName != "adr-divergence-load" {
		t.Errorf("issue Name = %q, want %q", gotName, "adr-divergence-load")
	}
}

// TestCheckADRDivergenceReturnsPopulated replaces the spec-086 stub-only
// test (TestCheckADRDivergenceReturnsEmpty). It exercises the real body
// against a fixture spec where the diff touches a file in an impacted
// domain that the plan does NOT cite an Accepted ADR for. The previous
// stub asserted an empty Result; this replacement asserts a non-empty
// Result plus a structured DivergenceFinding — same symbol, deeper
// contract (Spec 087 plan Bead 2 step 6 revision 10).
func TestCheckADRDivergenceReturnsPopulated(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "099-test")
	writeSpecAndPlan(t, root, specDir, "099-test",
		[]string{"payments"},
		[]string{}, // no ADR citations
	)
	writeManifest(t, root, "payments", "paths:\n  - internal/payments/**\n")

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/payments/charge.go"},
	}

	r, findings := CheckADRDivergence(root, "BASE", mock, specDir, "mindspec-zy4u.2", "", "")
	if r == nil {
		t.Fatal("nil result")
	}
	if r.SubCommand != "adr-divergence" {
		t.Errorf("SubCommand = %q, want adr-divergence", r.SubCommand)
	}
	if !r.HasFailures() {
		t.Fatalf("expected uncovered failure, got %+v", r.Issues)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Kind != "uncovered" {
		t.Errorf("Kind = %q, want uncovered", findings[0].Kind)
	}
	if findings[0].Domain != "payments" {
		t.Errorf("Domain = %q, want payments", findings[0].Domain)
	}
	if findings[0].Path != "internal/payments/charge.go" {
		t.Errorf("Path = %q, want internal/payments/charge.go", findings[0].Path)
	}
}

// TestCheckADRDivergenceHeadRefResolution pins the headRef resolution
// table (PR #132 panel C2 medium / mutation M4): the recorded
// ChangedFiles call args distinguish every head value, so a severed or
// re-pointed default cannot survive.
//
//   - per-bead default (beadID set, headRef ""): the canonical
//     workspace.BeadBranch — bead/<id> — never the ambient HEAD
//   - impl-approve default (beadID "", headRef ""): the spec branch
//     derived from specDir
//   - explicit headRef: passed through verbatim (complete.Run's
//     resolved bead worktree branch)
func TestCheckADRDivergenceHeadRefResolution(t *testing.T) {
	newFixture := func(t *testing.T) (string, string) {
		t.Helper()
		root := t.TempDir()
		specDir := filepath.Join(root, ".mindspec", "docs", "specs", "107-headref")
		writeSpecAndPlan(t, root, specDir, "107-headref",
			[]string{"payments"},
			[]string{},
		)
		writeManifest(t, root, "payments", "paths:\n  - internal/payments/**\n")
		return root, specDir
	}

	assertDiffRange := func(t *testing.T, mock *executor.MockExecutor, wantBase, wantHead string) {
		t.Helper()
		calls := mock.CallsTo("ChangedFiles")
		if len(calls) != 1 {
			t.Fatalf("expected exactly 1 ChangedFiles call, got %d", len(calls))
		}
		if calls[0].Args[0] != wantBase || calls[0].Args[1] != wantHead {
			t.Errorf("ChangedFiles(%v, %v), want (%q, %q)", calls[0].Args[0], calls[0].Args[1], wantBase, wantHead)
		}
	}

	t.Run("per-bead default is the canonical bead branch", func(t *testing.T) {
		root, specDir := newFixture(t)
		mock := &executor.MockExecutor{}
		CheckADRDivergence(root, "BASE", mock, specDir, "mindspec-bead.1", "", "")
		assertDiffRange(t, mock, "BASE", "bead/mindspec-bead.1")
	})

	t.Run("impl-approve default derives the spec branch", func(t *testing.T) {
		root, specDir := newFixture(t)
		mock := &executor.MockExecutor{}
		CheckADRDivergence(root, "BASE", mock, specDir, "", "", "")
		assertDiffRange(t, mock, "BASE", "spec/107-headref")
	})

	t.Run("explicit headRef wins over both defaults", func(t *testing.T) {
		root, specDir := newFixture(t)
		mock := &executor.MockExecutor{}
		CheckADRDivergence(root, "BASE", mock, specDir, "mindspec-bead.1", "bead/explicit-tip", "")
		assertDiffRange(t, mock, "BASE", "bead/explicit-tip")
	})
}

// TestCheckADRDivergence_ADR0041PresentVsAbsent is Spec 119 Bead 6's
// AC-25/Verification unit fixture: it drives CheckADRDivergence over a
// synthetic fixture citing (or not citing) "ADR-0041" — the exact
// mechanism the real ADR-divergence gate applies to the repo's own
// ADR-0041-gate-before-mutate.md — to pin that an Accepted, domain-covering
// citation clears the gate and its ABSENCE reproduces the uncovered
// failure. This is the mechanism-level proof backing the plan's "ADR cited
// from all three verbs' preflight code; the ADR-divergence gate passes"
// Verification bullet: the gate's coverage rule is generic over the ADR ID,
// so exercising it with the literal ID "ADR-0041" demonstrates the exact
// present/absent behavior the real gate applies when ADR-0041 (and its
// workflow/execution/core domains) is or is not an effective citation.
func TestCheckADRDivergence_ADR0041PresentVsAbsent(t *testing.T) {
	const specID = "119-gate-before-mutate"

	newFixture := func(t *testing.T, cited bool) (string, string) {
		t.Helper()
		root := t.TempDir()
		specDir := filepath.Join(root, ".mindspec", "docs", "specs", specID)
		var citations []string
		if cited {
			citations = []string{"ADR-0041"}
		}
		writeSpecAndPlan(t, root, specDir, specID, []string{"workflow"}, citations)
		writeManifest(t, root, "workflow", "paths:\n  - internal/approve/**\n")
		writeADR(t, root, "ADR-0041", "Accepted", []string{"workflow", "execution", "core"})
		return root, specDir
	}

	t.Run("present: an Accepted ADR-0041 citation clears the gate", func(t *testing.T) {
		root, specDir := newFixture(t, true)
		mock := &executor.MockExecutor{
			ChangedFilesResult: []string{"internal/approve/plan.go"},
		}
		r, findings := CheckADRDivergence(root, "BASE", mock, specDir, "mindspec-lc12.6", "", "")
		if r.HasFailures() {
			t.Errorf("expected no failures with ADR-0041 cited and Accepted, got %+v", r.Issues)
		}
		if len(findings) != 0 {
			t.Errorf("expected no uncovered findings, got %+v", findings)
		}
	})

	t.Run("absent: no ADR-0041 citation reproduces the uncovered failure", func(t *testing.T) {
		root, specDir := newFixture(t, false)
		mock := &executor.MockExecutor{
			ChangedFilesResult: []string{"internal/approve/plan.go"},
		}
		r, findings := CheckADRDivergence(root, "BASE", mock, specDir, "mindspec-lc12.6", "", "")
		if !r.HasFailures() {
			t.Fatal("expected an uncovered failure with no ADR-0041 citation")
		}
		if len(findings) != 1 || findings[0].Kind != "uncovered" || findings[0].Domain != "workflow" {
			t.Errorf("expected 1 uncovered finding on workflow, got %+v", findings)
		}
	})
}
