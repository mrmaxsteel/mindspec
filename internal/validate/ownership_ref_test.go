package validate

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// Spec 095 / mindspec-vvs9: the doc-sync + ADR-divergence gates resolve
// their OWNERSHIP attribution input (per-domain manifests AND domain
// enumeration) from the SAME git ref they diff, via the executor, not
// os.ReadFile on the ambient working tree. These tests pin the loader
// classification and prove the read follows the ref in BOTH directions.
// Each FAILS if the read reverts to os.ReadFile(root) / listDomainDirs(root).

const coreManifestRel = ".mindspec/docs/domains/core/OWNERSHIP.yaml"

// TestLoadOwnershipAtRef_Present: a manifest present at the ref parses
// into ref-anchored claims with a ref-qualified ManifestPath.
func TestLoadOwnershipAtRef_Present(t *testing.T) {
	mock := &executor.MockExecutor{
		FileAtRefOrAbsentFn: func(ref, p string) ([]byte, bool, error) {
			if ref == "bead/x" && p == coreManifestRel {
				return []byte("paths:\n  - internal/core/**\nexclude:\n  - internal/core/legacy/**\n"), true, nil
			}
			return nil, false, nil
		},
	}
	o, err := LoadOwnershipAtRef(mock, "bead/x", "core")
	if err != nil {
		t.Fatalf("LoadOwnershipAtRef err: %v", err)
	}
	if len(o.Paths) != 1 || o.Paths[0] != "internal/core/**" {
		t.Errorf("Paths = %v, want [internal/core/**]", o.Paths)
	}
	if len(o.Exclude) != 1 || o.Exclude[0] != "internal/core/legacy/**" {
		t.Errorf("Exclude = %v, want [internal/core/legacy/**]", o.Exclude)
	}
	if want := "bead/x:" + coreManifestRel; o.ManifestPath != want {
		t.Errorf("ManifestPath = %q, want %q (ref-qualified)", o.ManifestPath, want)
	}
	if o.Source() != "manifest" {
		t.Errorf("Source() = %q, want manifest", o.Source())
	}
}

// TestLoadOwnershipAtRef_AbsentClaimsNothing: a manifest absent at a
// VALID ref claims nothing (ADR-0036), with NO error — identical to
// absent-on-disk.
func TestLoadOwnershipAtRef_AbsentClaimsNothing(t *testing.T) {
	// Default mock: FileAtRefOrAbsentPresent == false, err == nil.
	mock := &executor.MockExecutor{}
	o, err := LoadOwnershipAtRef(mock, "bead/x", "core")
	if err != nil {
		t.Fatalf("absent-at-ref must NOT error, got: %v", err)
	}
	if o.ManifestPath != "" {
		t.Errorf("ManifestPath = %q, want empty for absent manifest", o.ManifestPath)
	}
	if len(o.Paths) != 0 {
		t.Errorf("Paths = %v, want empty (claims nothing)", o.Paths)
	}
	if o.Source() != "missing" {
		t.Errorf("Source() = %q, want missing", o.Source())
	}
}

// TestLoadOwnershipAtRef_OperationalErrorIsHardError: an operational
// git/executor failure on the ref read is propagated as a HARD error —
// NEVER silently collapsed to claims-nothing (which would un-gate
// doc-drift on a git glitch; ADR-0036 amend). This is the load-bearing
// distinction the executor's FileAtRefOrAbsent boundary exists to make.
func TestLoadOwnershipAtRef_OperationalErrorIsHardError(t *testing.T) {
	mock := &executor.MockExecutor{
		FileAtRefOrAbsentErr: fmt.Errorf("git ls-tree no-such-ref: fatal: Not a valid object name"),
	}
	o, err := LoadOwnershipAtRef(mock, "no-such-ref", "core")
	if err == nil {
		t.Fatalf("operational error must be a HARD error; got nil err and o=%+v", o)
	}
	if o != nil {
		t.Errorf("operational error must return nil Ownership, got %+v", o)
	}
	if !strings.Contains(err.Error(), "core") || !strings.Contains(err.Error(), "no-such-ref") {
		t.Errorf("error should name domain + ref, got: %v", err)
	}
}

// TestLoadOwnershipAtRef_ExcludedFirstSegmentErrors: the HC-5
// excluded-first-segment schema rejection runs byte-identically on the
// ref bytes.
func TestLoadOwnershipAtRef_ExcludedFirstSegmentErrors(t *testing.T) {
	mock := &executor.MockExecutor{
		FileAtRefOrAbsentFn: func(_ref, _p string) ([]byte, bool, error) {
			return []byte("paths:\n  - viz/foo/**\n"), true, nil
		},
	}
	_, err := LoadOwnershipAtRef(mock, "bead/x", "naughty")
	if err == nil {
		t.Fatal("expected HC-5 error for viz/ entry at ref")
	}
	if !strings.Contains(err.Error(), "viz") || !strings.Contains(err.Error(), "viz/foo/**") {
		t.Errorf("error should name the offending segment + entry, got: %v", err)
	}
}

// TestListDomainDirsAtRef pins the ref-aware domain enumeration: present
// dirs sorted; an absent domains/ tree at a valid ref returns empty (no
// error); an operational failure is a hard error.
func TestListDomainDirsAtRef(t *testing.T) {
	t.Run("present, sorted", func(t *testing.T) {
		mock := &executor.MockExecutor{TreeDirsAtRefResult: []string{"workflow", "core"}}
		got, err := listDomainDirsAtRef(mock, "bead/x")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(got) != 2 || got[0] != "core" || got[1] != "workflow" {
			t.Errorf("got %v, want [core workflow] sorted", got)
		}
	})
	t.Run("absent domains tree → empty, no error", func(t *testing.T) {
		mock := &executor.MockExecutor{} // TreeDirsAtRefResult nil, err nil
		got, err := listDomainDirsAtRef(mock, "bead/x")
		if err != nil {
			t.Fatalf("absent tree must not error, got: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})
	t.Run("operational error → hard error", func(t *testing.T) {
		mock := &executor.MockExecutor{TreeDirsAtRefErr: fmt.Errorf("fatal: bad ref")}
		if _, err := listDomainDirsAtRef(mock, "no-such-ref"); err == nil {
			t.Fatal("operational enumeration failure must be a hard error")
		}
	})
}

// TestAttributeDomain_ReadsRefNotDisk proves attribution follows the
// ownerRef tree in BOTH directions — the core mindspec-vvs9 contract.
func TestAttributeDomain_ReadsRefNotDisk(t *testing.T) {
	// Direction 1: claim ONLY at the ref (on-disk root is empty). The
	// file is attributed/owned because the read follows the ref.
	t.Run("claim only at ref → owned", func(t *testing.T) {
		root := t.TempDir() // no domains on disk
		mock := &executor.MockExecutor{
			FileAtRefOrAbsentFn: func(ref, p string) ([]byte, bool, error) {
				if ref == "branch" && p == coreManifestRel {
					return []byte("paths:\n  - internal/core/**\n"), true, nil
				}
				return nil, false, nil
			},
		}
		owner, o, err := attributeDomain(mock, root, "branch", "internal/core/foo.go", []string{"core"})
		if err != nil {
			t.Fatalf("attributeDomain err: %v", err)
		}
		if owner != "core" {
			t.Fatalf("claim committed at the ref must own the file; got %q", owner)
		}
		if o == nil || o.ManifestPath != "branch:"+coreManifestRel {
			t.Errorf("ManifestPath should be ref-qualified, got %+v", o)
		}
	})

	// Direction 2: claim present ON DISK at root but ABSENT at the ref.
	// It must NOT satisfy attribution — the read follows the ref, not
	// the working tree. RED-on-revert: reverting attributeDomain to
	// os.ReadFile(root) would wrongly attribute this to "core".
	t.Run("claim on disk but absent at ref → not owned", func(t *testing.T) {
		root := t.TempDir()
		writeManifest(t, root, "core", "paths:\n  - internal/core/**\n") // on disk only
		mock := &executor.MockExecutor{}                                 // ref read: absent everywhere
		owner, _, err := attributeDomain(mock, root, "branch", "internal/core/foo.go", []string{"core"})
		if err != nil {
			t.Fatalf("attributeDomain err: %v", err)
		}
		if owner != "" {
			t.Fatalf("a claim absent at the diffed ref must NOT own the file (read follows the ref); got %q", owner)
		}
	})
}

// TestValidateDivergence_BranchOnlyDomainDirFromRef proves the
// empty-impacted-domains enumeration is ref-anchored: a domain directory
// + manifest that exist ONLY at the ref are discovered and evaluated,
// even when the spec declares no impacted domains and the on-disk root
// has no such domain. RED-on-revert: reverting the fallback enumeration
// to listDomainDirs(root) makes the domain invisible and the owned file
// surfaces as `adr-divergence-unowned`.
func TestValidateDivergence_BranchOnlyDomainDirFromRef(t *testing.T) {
	root := t.TempDir()
	specDir := root + "/.mindspec/docs/specs/120-branchdomain"
	// No impacted domains → forces the ref-aware fallback enumeration.
	writeSpecAndPlan(t, root, specDir, "120-branchdomain", nil, []string{"ADR-0120"})
	writeADR(t, root, "ADR-0120", "Accepted", []string{"widget"})

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"internal/widget/widget.go"},
		TreeDirsAtRefFn: func(_ref, dir string) ([]string, error) {
			if dir == ".mindspec/docs/domains" {
				return []string{"widget"}, nil // domain dir exists ONLY at the ref
			}
			return nil, nil
		},
		FileAtRefOrAbsentFn: func(_ref, p string) ([]byte, bool, error) {
			if p == ".mindspec/docs/domains/widget/OWNERSHIP.yaml" {
				return []byte("paths:\n  - internal/widget/**\n"), true, nil
			}
			return nil, false, nil
		},
	}

	r, findings := ValidateDivergence(mock, root, specDir, "mindspec-bead.1", "BASE", "branch", "branch", false)
	if r.HasFailures() {
		t.Fatalf("branch-only domain (owned + ADR-covered) must pass; got %+v", r.Issues)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings for an owned+covered file, got %+v", findings)
	}
}

// The advisory reads the landing tree only. A bead-side claim cannot
// preserve ownership when that claim is absent at the landing tree.
func TestCheckUnclaimedSource_LandingTree(t *testing.T) {
	for _, claimed := range []bool{true, false} {
		t.Run(fmt.Sprint(claimed), func(t *testing.T) {
			reads := 0
			mock := &executor.MockExecutor{
				TreeDirsAtRefFn: func(ref, dir string) ([]string, error) {
					if ref != "LANDING" {
						t.Fatalf("enumeration ref = %q", ref)
					}
					return []string{"core"}, nil
				},
				FileAtRefOrAbsentFn: func(ref, path string) ([]byte, bool, error) {
					if ref != "LANDING" {
						t.Fatalf("manifest ref = %q", ref)
					}
					reads++
					if claimed {
						return []byte("paths:\n  - internal/core/**\n"), true, nil
					}
					return nil, false, nil
				},
			}
			r := &Result{}
			checkUnclaimedSource(r, mock, t.TempDir(), "LANDING", []string{"internal/**"}, []string{"internal/core/core.go"})
			if reads == 0 {
				t.Fatal("no ownership reads")
			}
			if got := len(r.Issues); (claimed && got != 0) || (!claimed && got != 1) {
				t.Fatalf("claimed=%v: issues=%+v", claimed, r.Issues)
			}
			if r.HasFailures() {
				t.Fatal("ownership advisory must not block")
			}
		})
	}
}

func TestValidateDocsSnapshot_UnclaimedSourceUsesOwnershipRef(t *testing.T) {
	for _, claimed := range []bool{true, false} {
		t.Run(fmt.Sprint(claimed), func(t *testing.T) {
			root := t.TempDir()
			writeSourceGlobsConfig(t, root, []string{"web/**"})
			mock := &executor.MockExecutor{
				ChangedFilesTreeResult: []string{"web/app.js", ".mindspec/docs/domains/core/architecture.md"},
				TreeDirsAtRefFn: func(ref, dir string) ([]string, error) {
					if ref != "LANDING" {
						t.Errorf("domain enumeration used %q, want ownership ref LANDING", ref)
					}
					if dir == ".mindspec/docs/domains" {
						return []string{"core"}, nil
					}
					return nil, nil
				},
				FileAtRefOrAbsentFn: func(ref, path string) ([]byte, bool, error) {
					if ref != "LANDING" {
						t.Errorf("manifest read used %q, want ownership ref LANDING", ref)
					}
					if path != coreManifestRel {
						return nil, false, nil
					}
					// The candidate has the opposite claim to the landing tree.
					if (ref == "LANDING") == claimed {
						return []byte("paths:\n  - web/**\n"), true, nil
					}
					return []byte("paths:\n  - internal/old/**\n"), true, nil
				},
			}
			r := ValidateDocsSnapshot(root, "BASE", "CANDIDATE", "LANDING", "/bead", mock)
			if r.HasFailures() {
				t.Fatalf("doc-sync: %+v", r.Issues)
			}
			warned := false
			for _, issue := range r.Issues {
				if issue.Name == "unclaimed-source" {
					warned = true
					if !strings.Contains(issue.Message, "web/app.js") {
						t.Errorf("advisory omits the source path: %+v", issue)
					}
				}
			}
			if warned == claimed {
				t.Errorf("claimed=%v: unclaimed-source warning=%v, issues=%+v", claimed, warned, r.Issues)
			}
			if len(mock.CallsTo("TreeDirsAtRef")) == 0 || len(mock.CallsTo("FileAtRefOrAbsent")) == 0 {
				t.Fatal("ownership inputs were not read")
			}
			for _, call := range mock.CallsTo("ChangedFilesTree") {
				if call.Args[1] != "BASE" || call.Args[2] != "CANDIDATE" {
					t.Errorf("diff range changed: %+v", call.Args)
				}
			}
		})
	}
}

// Both gates resolve path-shaped spec and ADR domains through the same
// ref used by attribution, package doc-sync, and source advisories.
func TestLandingTree_AllOwnershipReaders(t *testing.T) {
	root := t.TempDir()
	specDir := root + "/.mindspec/docs/specs/132-landing"
	writeSpecAndPlan(t, root, specDir, "132-landing", []string{"internal/core/core.go"}, []string{"ADR-0132"})
	writeADR(t, root, "ADR-0132", "Accepted", []string{"internal/core/"})
	reads := 0
	mock := &executor.MockExecutor{
		ChangedFilesTreeResult: []string{"internal/core/core.go", ".mindspec/docs/domains/core/architecture.md"},
		TreeDirsAtRefFn: func(ref, dir string) ([]string, error) {
			if ref != "LANDING" {
				t.Fatalf("enumeration used %q", ref)
			}
			reads++
			return []string{"core"}, nil
		},
		FileAtRefOrAbsentFn: func(ref, path string) ([]byte, bool, error) {
			if ref != "LANDING" {
				t.Fatalf("ownership input used %q", ref)
			}
			reads++
			if path == coreManifestRel {
				return []byte("paths:\n  - internal/core/**\n"), true, nil
			}
			return nil, false, nil
		},
	}
	docs := ValidateDocsSnapshot(root, "BASE", "CANDIDATE", "LANDING", "/bead", mock)
	if docs.HasFailures() {
		t.Fatalf("doc-sync: %+v", docs.Issues)
	}
	adr, findings := CheckADRDivergenceSnapshot(root, "BASE", mock, specDir, "bead-1", "CANDIDATE", "LANDING", "/bead")
	if adr.HasFailures() || len(findings) != 0 {
		t.Fatalf("ADR resolution or attribution: %+v %+v", adr.Issues, findings)
	}
	advisory := &Result{}
	checkUnclaimedSource(advisory, mock, root, "LANDING", []string{"internal/**"}, []string{"internal/core/core.go"})
	if len(advisory.Issues) != 0 || reads == 0 {
		t.Fatalf("ownership inputs were not applied: %+v, reads=%d", advisory.Issues, reads)
	}
	for _, call := range mock.CallsTo("ChangedFilesTree") {
		if call.Args[1] != "BASE" || call.Args[2] != "CANDIDATE" {
			t.Fatalf("ownership changed the diff: %+v", call.Args)
		}
	}
}
