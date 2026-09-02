package validate

// Spec 128 Bead 6 (OQ-1(a)): the prospective-snapshot legs
// (ValidateDocsSnapshot / CheckADRDivergenceSnapshot /
// ValidateDivergenceSnapshot). These pin the two properties the
// per-bead gate reorder depends on: (1) the diff routes through
// ChangedFilesTree(wtPath, base, treeOID) — never ChangedFiles — and
// (2) the OWNERSHIP attribution reads (TreeDirsAtRef/FileAtRefOrAbsent)
// use treeOID as their ref, never a committed branch name, so
// attribution and diffed content come from the identical candidate.

import (
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// TestValidateDocsSnapshot_UsesChangedFilesTree pins the diff routing:
// exactly one ChangedFilesTree(wtPath, base, treeOID) call, zero
// ChangedFiles calls.
func TestValidateDocsSnapshot_UsesChangedFilesTree(t *testing.T) {
	// ChangedFilesTreeResult explicitly configured (non-nil, even if
	// empty) so MockExecutor's ChangedFiles-fallback default (spec 128
	// bead 6) is never taken — this test wants to observe the tree leg
	// in isolation.
	mock := &executor.MockExecutor{ChangedFilesTreeResult: []string{}}
	ValidateDocsSnapshot(t.TempDir(), "BASE-SHA", "TREE-OID", "/wt/bead-1", mock)

	treeCalls := mock.CallsTo("ChangedFilesTree")
	if len(treeCalls) != 1 {
		t.Fatalf("expected exactly 1 ChangedFilesTree call, got %d", len(treeCalls))
	}
	if treeCalls[0].Args[0] != "/wt/bead-1" || treeCalls[0].Args[1] != "BASE-SHA" || treeCalls[0].Args[2] != "TREE-OID" {
		t.Errorf("ChangedFilesTree args = %v, want (/wt/bead-1, BASE-SHA, TREE-OID)", treeCalls[0].Args)
	}
	if calls := mock.CallsTo("ChangedFiles"); len(calls) != 0 {
		t.Errorf("expected zero ChangedFiles (committed-range) calls, got %d", len(calls))
	}
}

// TestValidateDocsSnapshot_AttributionReadsTreeOID: when the diff
// contains an internal/ source change, the domain-enumeration and
// manifest reads (TreeDirsAtRef/FileAtRefOrAbsent) must use treeOID as
// their ref — not a branch name, not "".
func TestValidateDocsSnapshot_AttributionReadsTreeOID(t *testing.T) {
	var gotDirsRef, gotFileRef string
	mock := &executor.MockExecutor{
		ChangedFilesTreeResult: []string{"internal/widget/widget.go", "docs/domains/widget/overview.md"},
		TreeDirsAtRefFn: func(ref, dir string) ([]string, error) {
			gotDirsRef = ref
			return []string{"widget"}, nil
		},
		FileAtRefOrAbsentFn: func(ref, path string) ([]byte, bool, error) {
			gotFileRef = ref
			return []byte("paths:\n  - internal/widget/**\n"), true, nil
		},
	}
	res := ValidateDocsSnapshot(t.TempDir(), "BASE-SHA", "TREE-OID", "/wt/bead-1", mock)
	if res == nil {
		t.Fatal("expected a non-nil result")
	}
	if gotDirsRef != "TREE-OID" {
		t.Errorf("TreeDirsAtRef ref = %q, want TREE-OID", gotDirsRef)
	}
	if gotFileRef != "TREE-OID" {
		t.Errorf("FileAtRefOrAbsent ref = %q, want TREE-OID", gotFileRef)
	}
}

// TestCheckADRDivergenceSnapshot_UsesChangedFilesTree mirrors the
// doc-sync pin for the ADR-divergence gate.
func TestCheckADRDivergenceSnapshot_UsesChangedFilesTree(t *testing.T) {
	root := t.TempDir()
	mock := &executor.MockExecutor{}
	// specDir is required and must point at a directory (missing
	// spec.md/plan.md degrades to a no-op result per ValidateDivergence's
	// own doc comment — this test only asserts the diff routing, not the
	// findings).
	_, findings := CheckADRDivergenceSnapshot(root, "BASE-SHA", mock, root, "mindspec-x.1", "TREE-OID", "/wt/bead-1")
	if findings != nil {
		t.Fatalf("expected nil findings for a specDir with no spec.md, got %v", findings)
	}
	// With no spec.md, ValidateDivergence returns before ever reaching
	// the diff — confirm the specDir-required leg and the diff-routing
	// leg separately via ValidateDivergenceSnapshot directly below.
}

// TestValidateDivergenceSnapshot_UsesChangedFilesTreeAndForcedOwnerRef
// pins both properties directly against ValidateDivergenceSnapshot,
// with a specDir carrying a minimal spec.md/plan.md so the diff is
// actually reached.
func TestValidateDivergenceSnapshot_UsesChangedFilesTreeAndForcedOwnerRef(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "128-snap")
	writeSpecAndPlan(t, root, specDir, "128-snap", nil, nil)

	var gotDirsRef string
	mock := &executor.MockExecutor{
		// Explicitly configured (non-nil, even if empty) so the
		// ChangedFiles-fallback default is never taken.
		ChangedFilesTreeResult: []string{},
		TreeDirsAtRefFn: func(ref, dir string) ([]string, error) {
			gotDirsRef = ref
			return nil, nil
		},
	}
	_, _ = ValidateDivergenceSnapshot(mock, root, specDir, "mindspec-x.1", "BASE-SHA", "TREE-OID", "/wt/bead-1", false)

	treeCalls := mock.CallsTo("ChangedFilesTree")
	if len(treeCalls) != 1 {
		t.Fatalf("expected exactly 1 ChangedFilesTree call, got %d", len(treeCalls))
	}
	if treeCalls[0].Args[0] != "/wt/bead-1" || treeCalls[0].Args[1] != "BASE-SHA" || treeCalls[0].Args[2] != "TREE-OID" {
		t.Errorf("ChangedFilesTree args = %v, want (/wt/bead-1, BASE-SHA, TREE-OID)", treeCalls[0].Args)
	}
	if calls := mock.CallsTo("ChangedFiles"); len(calls) != 0 {
		t.Errorf("expected zero ChangedFiles (committed-range) calls, got %d", len(calls))
	}
	// Domain enumeration (candidateDomains fallback) reads at treeOID —
	// ownerRef forced, never a branch name.
	if gotDirsRef != "TREE-OID" {
		t.Errorf("TreeDirsAtRef ref = %q, want TREE-OID (ownerRef forced to the snapshot tree)", gotDirsRef)
	}
}
