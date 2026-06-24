package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// Spec 106 Bead 2: the diff-string layout classifiers and the ref-anchored
// ownership pair recognize ALL THREE layout prefixes — flat (.mindspec/<name>),
// canonical (.mindspec/docs/<name>), and legacy (docs/<name>) — identically.
// This posture is PERMANENT (historical refs / forks emit the legacy/canonical
// paths forever) and is purely ADDITIVE: canonical/legacy classification is
// byte-identical to pre-spec, only the flat prefix is newly recognized.

// layoutTriple returns the (flat, canonical, legacy) path for a docs-relative
// artifact rel (e.g. "specs/106/spec.md").
func layoutTriple(rel string) (flat, canonical, legacy string) {
	return ".mindspec/" + rel, ".mindspec/docs/" + rel, "docs/" + rel
}

// TestLayoutClassifier_ThreePrefixEquivalence: a flat-, canonical-, and
// legacy-prefix diff classify IDENTICALLY across isDocFile / isSourceFile /
// isProcessArtifact for every lifecycle artifact class (AC11).
func TestLayoutClassifier_ThreePrefixEquivalence(t *testing.T) {
	rels := []string{
		"specs/106-layout-flatten/spec.md",
		"specs/106-layout-flatten/plan.md",
		"adr/ADR-0039-layout.md",
		"domains/workflow/architecture.md",
		"domains/workflow/OWNERSHIP.yaml",
		"core/USAGE.md",
		"context-map.md",
	}
	for _, rel := range rels {
		flat, canonical, legacy := layoutTriple(rel)
		// isDocFile: identically TRUE in every layout.
		if !(isDocFile(flat) && isDocFile(canonical) && isDocFile(legacy)) {
			t.Errorf("isDocFile not identically true for %q: flat=%v canonical=%v legacy=%v",
				rel, isDocFile(flat), isDocFile(canonical), isDocFile(legacy))
		}
		// isSourceFile: identically FALSE — docs are never source, any layout.
		if isSourceFile(flat) || isSourceFile(canonical) || isSourceFile(legacy) {
			t.Errorf("isSourceFile must be false for doc %q in every layout", rel)
		}
		// isProcessArtifact: identically TRUE (the divergence skip set).
		if !(isProcessArtifact(flat) && isProcessArtifact(canonical) && isProcessArtifact(legacy)) {
			t.Errorf("isProcessArtifact not identically true for %q", rel)
		}
	}
}

// TestSpecMDID_ThreePrefixIdentical: specMDID extracts the same id from a
// flat, canonical, and legacy spec.md path, and rejects non-spec.md / nested
// paths in every layout (AC11).
func TestSpecMDID_ThreePrefixIdentical(t *testing.T) {
	flat, canonical, legacy := layoutTriple("specs/106-layout-flatten/spec.md")
	for _, p := range []string{flat, canonical, legacy} {
		if got := specMDID(p); got != "106-layout-flatten" {
			t.Errorf("specMDID(%q) = %q, want 106-layout-flatten", p, got)
		}
	}
	for _, p := range []string{
		".mindspec/specs/106/plan.md",                    // not spec.md
		".mindspec/docs/specs/106/recording/foo/spec.md", // nested id segment
		"internal/specs/106/spec.md",                     // not a specs root
		"docs/adr/106/spec.md",                           // wrong subtree
	} {
		if got := specMDID(p); got != "" {
			t.Errorf("specMDID(%q) = %q, want empty", p, got)
		}
	}
}

// TestDomainPrefix_ThreePrefixIdentical: the domain-doc prefix matcher
// (hasArtifactPrefix) recognizes a domain's doc dir identically across layouts
// and scopes to the named domain (AC11).
func TestDomainPrefix_ThreePrefixIdentical(t *testing.T) {
	for _, p := range []string{
		".mindspec/domains/workflow/architecture.md",
		".mindspec/docs/domains/workflow/architecture.md",
		"docs/domains/workflow/architecture.md",
	} {
		if !hasArtifactPrefix(p, "domains") {
			t.Errorf("hasArtifactPrefix(%q, domains) = false, want true", p)
		}
		if !hasArtifactPrefix(p, "domains/workflow") {
			t.Errorf("hasArtifactPrefix(%q, domains/workflow) = false, want true", p)
		}
	}
	if hasArtifactPrefix(".mindspec/domains/core/x.md", "domains/workflow") {
		t.Error("domains/workflow prefix must not match a core path")
	}
}

// TestADRMarkdown_ThreePrefixIdentical: isADRMarkdown recognizes ADR .md files
// across layouts, and rejects non-.md / non-adr paths (AC11).
func TestADRMarkdown_ThreePrefixIdentical(t *testing.T) {
	for _, p := range []string{
		".mindspec/adr/ADR-0039.md",
		".mindspec/docs/adr/ADR-0039.md",
		"docs/adr/ADR-0039.md",
	} {
		if !isADRMarkdown(p) {
			t.Errorf("isADRMarkdown(%q) = false, want true", p)
		}
	}
	if isADRMarkdown(".mindspec/adr/ADR-0039.txt") {
		t.Error("isADRMarkdown must require a .md suffix")
	}
	if isADRMarkdown(".mindspec/domains/workflow/x.md") {
		t.Error("isADRMarkdown must not match a non-adr path")
	}
}

// TestProjectDocs_NonSourceNonUnowned: project-docs/** (the dogfood-eviction
// tree) is classified non-source docs so it trips neither the doc-sync source
// lane nor adr-divergence-unowned (AC11 / AC21, Req 14).
func TestProjectDocs_NonSourceNonUnowned(t *testing.T) {
	for _, p := range []string{"project-docs/foo.md", "project-docs/user/guide.md"} {
		if isSourceFile(p) {
			t.Errorf("project-docs path %q must not classify as source", p)
		}
		if !isDocFile(p) {
			t.Errorf("project-docs path %q must classify as a doc", p)
		}
		if !isProcessArtifact(p) {
			t.Errorf("project-docs path %q must be a process artifact (skipped before attribution)", p)
		}
	}
}

// TestReviewMatchers_RootAndCoLocated: BOTH the permanent root review/<slug>/
// tree AND the co-located <spec-dir>/reviews/<slug>/ tree classify non-source,
// via two INDEPENDENT matchers (AC11).
func TestReviewMatchers_RootAndCoLocated(t *testing.T) {
	// Root review/ tree — the permanent historical-ref matcher.
	if !isProcessArtifact("review/panel-abc/panel.json") {
		t.Error("root review/<slug>/ must classify non-source")
	}
	// Co-located reviews under a spec dir, in every layout.
	for _, p := range []string{
		".mindspec/specs/106/reviews/panel-abc/panel.json",      // flat
		".mindspec/docs/specs/106/reviews/panel-abc/panel.json", // canonical
		"docs/specs/106/reviews/panel-abc/panel.json",           // legacy
	} {
		if !isProcessArtifact(p) {
			t.Errorf("co-located review %q must classify non-source", p)
		}
	}
	// The two matchers are INDEPENDENT: the literal root "review/" prefix does
	// not substring-match "reviews/", and the /reviews/ segment matcher does
	// not fire on the root review/ tree.
	if isCoLocatedReview("review/panel-abc/panel.json") {
		t.Error("root review/ must NOT match the /reviews/ segment matcher")
	}
	if !isCoLocatedReview(".mindspec/specs/106/reviews/panel-abc/panel.json") {
		t.Error("co-located reviews must match the /reviews/ segment matcher")
	}
	// A /reviews/ path NOT caught by isDocFile still classifies non-source —
	// proving the segment matcher is additive, not redundant with isDocFile.
	if !isProcessArtifact("internal/foo/reviews/panel-abc/notes.md") {
		t.Error("/reviews/ segment must classify non-source independent of isDocFile")
	}
}

// writeFlatManifest writes an OWNERSHIP.yaml under the FLAT layout root
// (.mindspec/domains/<domain>/OWNERSHIP.yaml).
func writeFlatManifest(t *testing.T, root, domain, body string) {
	t.Helper()
	dir := filepath.Join(root, ".mindspec", "domains", domain)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "OWNERSHIP.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write flat OWNERSHIP.yaml: %v", err)
	}
}

// TestLoadOwnership_FlatTree: the on-disk loader resolves a domain manifest
// from the FLAT .mindspec/domains/<d> root (Req 3/6).
func TestLoadOwnership_FlatTree(t *testing.T) {
	root := t.TempDir()
	writeFlatManifest(t, root, "workflow", "paths:\n  - internal/validate/**\n")
	o, err := LoadOwnership(root, "workflow")
	if err != nil {
		t.Fatalf("LoadOwnership: %v", err)
	}
	if o.Source() != "manifest" {
		t.Fatalf("Source() = %q, want manifest (flat manifest must resolve, not read 'missing')", o.Source())
	}
	if len(o.Paths) != 1 || o.Paths[0] != "internal/validate/**" {
		t.Errorf("Paths = %v, want [internal/validate/**]", o.Paths)
	}
}

// TestListDomainDirs_FlatTree: the on-disk enumerator lists domains from the
// FLAT .mindspec/domains root (Req 3/6).
func TestListDomainDirs_FlatTree(t *testing.T) {
	root := t.TempDir()
	writeFlatManifest(t, root, "workflow", "paths: []\n")
	writeFlatManifest(t, root, "core", "paths: []\n")
	dirs, err := listDomainDirs(root)
	if err != nil {
		t.Fatalf("listDomainDirs: %v", err)
	}
	if len(dirs) != 2 || dirs[0] != "core" || dirs[1] != "workflow" {
		t.Fatalf("flat enumeration = %v, want [core workflow]", dirs)
	}
}

// TestDomainManifestRelPaths_FlatFirst: the ref-addressed candidate list leads
// with the flat path, then canonical, then legacy (Req 6).
func TestDomainManifestRelPaths_FlatFirst(t *testing.T) {
	got := domainManifestRelPaths("workflow")
	want := []string{
		".mindspec/domains/workflow/OWNERSHIP.yaml",
		".mindspec/docs/domains/workflow/OWNERSHIP.yaml",
		"docs/domains/workflow/OWNERSHIP.yaml",
	}
	if len(got) != len(want) {
		t.Fatalf("candidate count = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidate[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestLoadOwnershipAtRef_FlatRef: the CRITICAL ref-anchored loader resolves a
// domain manifest committed under the FLAT layout at a ref — returning the
// domain's claims (NOT "missing"), so a flat-tree bead does not hard-block on
// adr-divergence-unowned (AC11). RED-on-revert: a canonical-hardcoded
// domainManifestRelPath would read the flat manifest as absent → "missing".
func TestLoadOwnershipAtRef_FlatRef(t *testing.T) {
	const flatRel = ".mindspec/domains/workflow/OWNERSHIP.yaml"
	mock := &executor.MockExecutor{
		FileAtRefOrAbsentFn: func(ref, p string) ([]byte, bool, error) {
			if p == flatRel {
				return []byte("paths:\n  - internal/validate/**\n"), true, nil
			}
			return nil, false, nil // canonical + legacy candidates absent at this ref
		},
	}
	o, err := LoadOwnershipAtRef(mock, "bead/flat", "workflow")
	if err != nil {
		t.Fatalf("LoadOwnershipAtRef: %v", err)
	}
	if o.Source() != "manifest" {
		t.Fatalf("Source() = %q, want manifest (flat ref manifest must resolve)", o.Source())
	}
	if len(o.Paths) != 1 || o.Paths[0] != "internal/validate/**" {
		t.Errorf("Paths = %v, want [internal/validate/**]", o.Paths)
	}
	if want := "bead/flat:" + flatRel; o.ManifestPath != want {
		t.Errorf("ManifestPath = %q, want %q (flat ref-qualified)", o.ManifestPath, want)
	}
}

// TestListDomainDirsAtRef_FlatRef: the ref-anchored enumerator discovers a
// domain dir that exists only under the FLAT layout root at a ref (Req 6).
func TestListDomainDirsAtRef_FlatRef(t *testing.T) {
	mock := &executor.MockExecutor{
		TreeDirsAtRefFn: func(_ref, dir string) ([]string, error) {
			if dir == ".mindspec/domains" {
				return []string{"workflow"}, nil // exists ONLY under the flat root
			}
			return nil, nil
		},
	}
	dirs, err := listDomainDirsAtRef(mock, "bead/flat")
	if err != nil {
		t.Fatalf("listDomainDirsAtRef: %v", err)
	}
	if len(dirs) != 1 || dirs[0] != "workflow" {
		t.Fatalf("flat ref enumeration = %v, want [workflow]", dirs)
	}
}

// TestValidateDivergence_ProjectDocsNotUnowned: a bead whose diff touches only
// project-docs/** (dogfood eviction) does NOT surface adr-divergence-unowned
// (AC11 / AC21, Req 14) — the project-docs tree is skipped before attribution.
func TestValidateDivergence_ProjectDocsNotUnowned(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "106-layout-flatten")
	writeSpecAndPlan(t, root, specDir, "106-layout-flatten", []string{"workflow"}, []string{"ADR-0106"})
	writeADR(t, root, "ADR-0106", "Accepted", []string{"workflow"})
	writeManifest(t, root, "workflow", "paths:\n  - internal/validate/**\n")

	mock := &executor.MockExecutor{
		ChangedFilesResult: []string{"project-docs/foo.md", "project-docs/user/guide.md"},
	}
	r, findings := ValidateDivergence(mock, root, specDir, "mindspec-106.1", "BASE", "HEAD", "", false)
	if r.HasFailures() {
		t.Fatalf("project-docs/** must not trip adr-divergence; got %+v", r.Issues)
	}
	for _, f := range findings {
		if f.Kind == "unowned" {
			t.Errorf("project-docs path surfaced as unowned: %+v", f)
		}
	}
}
