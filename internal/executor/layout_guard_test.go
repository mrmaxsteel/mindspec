package executor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Spec 106 Bead 4 (Req 9 / AC15): the DIRECTIONAL merge-time layout-fingerprint
// HARD-FAIL. block ⟺ source is canonical/legacy AND target is flat (the
// regression that resurrects pre-flatten paths). The flat→canonical migration
// direction and same-layout merges pass; a live migration run exempts the block.

// TestMergeLayoutRegression_Matrix pins the pure directional predicate across
// every layout combination.
func TestMergeLayoutRegression_Matrix(t *testing.T) {
	cases := []struct {
		source, target workspace.Layout
		block          bool
	}{
		// REGRESSION (blocked): canonical/legacy/mixed source onto a flat target.
		{workspace.LayoutCanonical, workspace.LayoutFlat, true},
		{workspace.LayoutLegacy, workspace.LayoutFlat, true},
		// Bead 2 (spec 118 / AC-10, AC-12, AC-22): a MIXED source (a flat
		// lifecycle tree coexisting with a canonical or legacy one) onto a
		// flat target is the SAME regression risk as a pure canonical/legacy
		// source, so it is blocked too.
		{workspace.LayoutMixed, workspace.LayoutFlat, true},
		// MIGRATION (allowed): flat source onto canonical/legacy target — the
		// flatten landing itself.
		{workspace.LayoutFlat, workspace.LayoutCanonical, false},
		{workspace.LayoutFlat, workspace.LayoutLegacy, false},
		// Same-layout: always allowed.
		{workspace.LayoutFlat, workspace.LayoutFlat, false},
		{workspace.LayoutCanonical, workspace.LayoutCanonical, false},
		{workspace.LayoutLegacy, workspace.LayoutLegacy, false},
		// Greenfield source onto flat: not canonical/legacy/mixed → allowed.
		{workspace.LayoutGreenfield, workspace.LayoutFlat, false},
		// Canonical source onto a non-flat target: not a regression.
		{workspace.LayoutCanonical, workspace.LayoutLegacy, false},
		{workspace.LayoutCanonical, workspace.LayoutGreenfield, false},
		// Mixed source onto a non-flat target: not a regression either — the
		// directional rule keys on the TARGET being flat.
		{workspace.LayoutMixed, workspace.LayoutCanonical, false},
		{workspace.LayoutMixed, workspace.LayoutGreenfield, false},
	}
	for _, c := range cases {
		if got := mergeLayoutRegression(c.source, c.target); got != c.block {
			t.Errorf("mergeLayoutRegression(%s, %s) = %v, want %v", c.source, c.target, got, c.block)
		}
	}
}

// TestGuardMergeLayout_Directional exercises the full guard with an injected
// layout reader: regression blocked (with the rebase recovery line), migration
// allowed, same-layout allowed, the run-state exemption, and the read-error
// fail-open.
func TestGuardMergeLayout_Directional(t *testing.T) {
	layoutFor := func(m map[string]workspace.Layout) func(string) (workspace.Layout, error) {
		return func(ref string) (workspace.Layout, error) {
			if l, ok := m[ref]; ok {
				return l, nil
			}
			return "", errors.New("unknown ref")
		}
	}

	t.Run("regression canonical→flat is BLOCKED with a recovery line", func(t *testing.T) {
		at := layoutFor(map[string]workspace.Layout{
			"bead/x": workspace.LayoutCanonical,
			"spec/y": workspace.LayoutFlat,
		})
		err := guardMergeLayout("bead/x", "spec/y", at, false)
		if err == nil {
			t.Fatal("canonical→flat (regression) must be blocked")
		}
		msg := err.Error()
		if !strings.Contains(msg, "layout regression") {
			t.Errorf("block must name the layout regression; got:\n%s", msg)
		}
		if !guard.HasFinalRecoveryLine(msg) {
			t.Errorf("block must end with a recovery line (ADR-0035); got:\n%s", msg)
		}
		if !strings.Contains(msg, "rebase") {
			t.Errorf("recovery must be a rebase onto the post-flatten target; got:\n%s", msg)
		}
	})

	t.Run("migration flat→canonical is ALLOWED", func(t *testing.T) {
		at := layoutFor(map[string]workspace.Layout{
			"bead/x": workspace.LayoutFlat,
			"spec/y": workspace.LayoutCanonical,
		})
		if err := guardMergeLayout("bead/x", "spec/y", at, false); err != nil {
			t.Errorf("flat→canonical (migration) must be allowed; got: %v", err)
		}
	})

	t.Run("same-layout flat→flat is ALLOWED", func(t *testing.T) {
		at := layoutFor(map[string]workspace.Layout{
			"bead/x": workspace.LayoutFlat,
			"spec/y": workspace.LayoutFlat,
		})
		if err := guardMergeLayout("bead/x", "spec/y", at, false); err != nil {
			t.Errorf("flat→flat must be allowed; got: %v", err)
		}
	})

	t.Run("regression is EXEMPT under a live migration run-state", func(t *testing.T) {
		at := layoutFor(map[string]workspace.Layout{
			"bead/x": workspace.LayoutCanonical,
			"spec/y": workspace.LayoutFlat,
		})
		if err := guardMergeLayout("bead/x", "spec/y", at, true); err != nil {
			t.Errorf("a live migration run must EXEMPT the regression block; got: %v", err)
		}
	})

	t.Run("a layout read error FAILS OPEN (never false-blocks)", func(t *testing.T) {
		at := layoutFor(map[string]workspace.Layout{
			"spec/y": workspace.LayoutFlat, // bead/x missing → read error
		})
		if err := guardMergeLayout("bead/x", "spec/y", at, false); err != nil {
			t.Errorf("a layout read error must fail open, not block; got: %v", err)
		}
	})
}

// commitTreeOnBranch creates branch off base in the repo at dir, writes files,
// commits them, and returns the working tree to main.
func commitTreeOnBranch(t *testing.T, dir, branch, base string, files map[string]string) {
	t.Helper()
	runGitIn(t, dir, "checkout", "-q", "-b", branch, base)
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGitIn(t, dir, "add", "-A")
	runGitIn(t, dir, "commit", "-q", "-m", branch)
	runGitIn(t, dir, "checkout", "-q", "main")
}

// TestLayoutAtRef_ClassifiesRealBranches proves the production reader
// fingerprints each tree shape from a real git ref via TreeDirsAtRef +
// BlobExistsAtRef + the shared workspace signature helper (one source of
// truth, no drift). Bead 2 (spec 118) extends this with: unrelated wrappers
// that must NOT false-mark (AC-9, AC-11), all three context-map BLOB tiers
// (AC-16), and all three context-map TREE (directory) tiers, which must NOT
// mark and must NOT error (AC-23) — pinning the type-aware blob probe.
func TestLayoutAtRef_ClassifiesRealBranches(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	commitTreeOnBranch(t, dir, "canon", "main", map[string]string{
		".mindspec/docs/specs/106-x/spec.md": "# canonical\n",
	})
	commitTreeOnBranch(t, dir, "flat", "main", map[string]string{
		".mindspec/specs/106-x/spec.md": "# flat\n",
	})
	commitTreeOnBranch(t, dir, "legacy", "main", map[string]string{
		"docs/specs/106-x/spec.md": "# legacy\n",
	})
	// greenfield: main itself (no .mindspec, no docs/).

	// Unrelated wrappers (AC-9, AC-11): a wrapper dir exists and is
	// representable in git via an unrelated tracked file, but has no direct
	// lifecycle child and no context-map file. In isolation this must not
	// mark canonical/legacy at all (stays greenfield); alongside a flat
	// lifecycle tree it must not false-block/false-mix (stays flat).
	commitTreeOnBranch(t, dir, "canon-wrapper-unrelated", "main", map[string]string{
		".mindspec/docs/README.md": "unrelated canonical wrapper content\n",
	})
	commitTreeOnBranch(t, dir, "legacy-wrapper-unrelated", "main", map[string]string{
		"docs/README.md": "unrelated legacy wrapper content\n",
	})
	commitTreeOnBranch(t, dir, "flat-plus-canon-wrapper-unrelated", "main", map[string]string{
		".mindspec/specs/106-x/spec.md": "# flat\n",
		".mindspec/docs/README.md":      "unrelated canonical wrapper content\n",
	})
	commitTreeOnBranch(t, dir, "flat-plus-legacy-wrapper-unrelated", "main", map[string]string{
		".mindspec/specs/106-x/spec.md": "# flat\n",
		"docs/README.md":                "unrelated legacy wrapper content\n",
	})

	// Context-map BLOB tiers (AC-16): a regular context-map.md file at each
	// of the three tier paths, with no other marker present.
	commitTreeOnBranch(t, dir, "flat-contextmap-blob", "main", map[string]string{
		".mindspec/context-map.md": "# flat context map\n",
	})
	commitTreeOnBranch(t, dir, "canon-contextmap-blob", "main", map[string]string{
		".mindspec/docs/context-map.md": "# canonical context map\n",
	})
	commitTreeOnBranch(t, dir, "legacy-contextmap-blob", "main", map[string]string{
		"docs/context-map.md": "# legacy context map\n",
	})

	// Context-map TREE (directory) tiers (AC-23): context-map.md committed
	// as a DIRECTORY rather than a regular file at each of the three tier
	// paths. This must not mark the tier and must not error — the
	// type-aware probe (BlobExistsAtRef) must reject a tree at that path,
	// unlike a bare `git show`/FileAtRef existence check.
	commitTreeOnBranch(t, dir, "flat-contextmap-tree", "main", map[string]string{
		".mindspec/context-map.md/inner.txt": "not a blob\n",
	})
	commitTreeOnBranch(t, dir, "canon-contextmap-tree", "main", map[string]string{
		".mindspec/docs/context-map.md/inner.txt": "not a blob\n",
	})
	commitTreeOnBranch(t, dir, "legacy-contextmap-tree", "main", map[string]string{
		"docs/context-map.md/inner.txt": "not a blob\n",
	})

	cases := []struct {
		ref  string
		want workspace.Layout
	}{
		{"canon", workspace.LayoutCanonical},
		{"flat", workspace.LayoutFlat},
		{"legacy", workspace.LayoutLegacy},
		{"main", workspace.LayoutGreenfield},
		{"canon-wrapper-unrelated", workspace.LayoutGreenfield},
		{"legacy-wrapper-unrelated", workspace.LayoutGreenfield},
		{"flat-plus-canon-wrapper-unrelated", workspace.LayoutFlat},
		{"flat-plus-legacy-wrapper-unrelated", workspace.LayoutFlat},
		{"flat-contextmap-blob", workspace.LayoutFlat},
		{"canon-contextmap-blob", workspace.LayoutCanonical},
		{"legacy-contextmap-blob", workspace.LayoutLegacy},
		{"flat-contextmap-tree", workspace.LayoutGreenfield},
		{"canon-contextmap-tree", workspace.LayoutGreenfield},
		{"legacy-contextmap-tree", workspace.LayoutGreenfield},
	}
	for _, c := range cases {
		got, err := g.layoutAtRef(c.ref)
		if err != nil {
			t.Errorf("layoutAtRef(%q): %v", c.ref, err)
			continue
		}
		if got != c.want {
			t.Errorf("layoutAtRef(%q) = %q, want %q", c.ref, got, c.want)
		}
	}
}

// TestLayoutAtRef_FilesystemParity proves that, for every equivalent
// fixture, the real-git-ref classifier (layoutAtRef, via TreeDirsAtRef +
// BlobExistsAtRef) and the on-disk filesystem classifier
// (workspace.DetectLayout) agree — one source of truth, no drift — including
// the blob-versus-tree context-map variants that force the type-aware ref
// probe (Bead 2 / spec 118, B2-V4).
func TestLayoutAtRef_FilesystemParity(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	fixtures := []struct {
		name  string
		files map[string]string
		want  workspace.Layout
	}{
		{"flat", map[string]string{".mindspec/specs/106-x/spec.md": "# flat\n"}, workspace.LayoutFlat},
		{"canonical", map[string]string{".mindspec/docs/specs/106-x/spec.md": "# canonical\n"}, workspace.LayoutCanonical},
		{"legacy", map[string]string{"docs/specs/106-x/spec.md": "# legacy\n"}, workspace.LayoutLegacy},
		{"flat-contextmap-blob", map[string]string{".mindspec/context-map.md": "# ctx\n"}, workspace.LayoutFlat},
		{"canonical-contextmap-blob", map[string]string{".mindspec/docs/context-map.md": "# ctx\n"}, workspace.LayoutCanonical},
		{"legacy-contextmap-blob", map[string]string{"docs/context-map.md": "# ctx\n"}, workspace.LayoutLegacy},
		{"flat-contextmap-tree", map[string]string{".mindspec/context-map.md/inner.txt": "not a blob\n"}, workspace.LayoutGreenfield},
		{"canonical-contextmap-tree", map[string]string{".mindspec/docs/context-map.md/inner.txt": "not a blob\n"}, workspace.LayoutGreenfield},
		{"legacy-contextmap-tree", map[string]string{"docs/context-map.md/inner.txt": "not a blob\n"}, workspace.LayoutGreenfield},
		{"unrelated-canonical-wrapper", map[string]string{".mindspec/docs/README.md": "hi\n"}, workspace.LayoutGreenfield},
		{"unrelated-legacy-wrapper", map[string]string{"docs/README.md": "hi\n"}, workspace.LayoutGreenfield},
	}

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			branch := "parity-" + f.name
			commitTreeOnBranch(t, dir, branch, "main", f.files)
			refGot, err := g.layoutAtRef(branch)
			if err != nil {
				t.Fatalf("layoutAtRef(%q): %v", branch, err)
			}
			if refGot != f.want {
				t.Errorf("layoutAtRef(%q) = %q, want %q", branch, refGot, f.want)
			}

			fsRoot := t.TempDir()
			for rel, content := range f.files {
				p := filepath.Join(fsRoot, rel)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			fsGot, err := workspace.DetectLayout(fsRoot)
			if err != nil {
				t.Fatalf("DetectLayout(%q): %v", fsRoot, err)
			}
			if fsGot != f.want {
				t.Errorf("DetectLayout(%q) = %q, want %q", f.name, fsGot, f.want)
			}
			if refGot != fsGot {
				t.Errorf("ref/filesystem parity mismatch for %q: layoutAtRef=%q, DetectLayout=%q", f.name, refGot, fsGot)
			}
		})
	}
}

// TestGuardMergeLayout_CanonicalMixedSource is a real-git-ref integration
// (Bead 2 / spec 118, AC-10): a source ref carrying BOTH a flat lifecycle
// directory AND a canonical (.mindspec/docs/specs/...) lifecycle directory
// classifies mixed, and merging it onto a flat target reaches the local
// merge guard and is BLOCKED — independently derived, despite Flat already
// being set on the source.
func TestGuardMergeLayout_CanonicalMixedSource(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	commitTreeOnBranch(t, dir, "mixed-canonical-source", "main", map[string]string{
		".mindspec/specs/106-x/spec.md":      "# flat\n",
		".mindspec/docs/specs/106-x/spec.md": "# canonical\n",
	})
	commitTreeOnBranch(t, dir, "flat-target", "main", map[string]string{
		".mindspec/specs/106-x/spec.md": "# flat target\n",
	})

	if got, err := g.layoutAtRef("mixed-canonical-source"); err != nil || got != workspace.LayoutMixed {
		t.Fatalf("layoutAtRef(mixed-canonical-source) = (%q, %v), want (mixed, nil)", got, err)
	}

	err := guardMergeLayout("mixed-canonical-source", "flat-target", g.layoutAtRef, false)
	if err == nil {
		t.Fatal("a mixed (flat+canonical) source merging onto a flat target must be blocked")
	}
	if !strings.Contains(err.Error(), "layout regression") {
		t.Errorf("error must name the layout regression; got:\n%s", err.Error())
	}
}

// TestGuardMergeLayout_LegacyMixedSource is a real-git-ref integration
// (Bead 2 / spec 118, AC-12): a source ref carrying BOTH a flat lifecycle
// directory AND a legacy (root docs/specs/...) lifecycle directory
// classifies mixed, and merging it onto a flat target reaches the local
// merge guard and is BLOCKED — independently derived, despite Flat already
// being set on the source.
func TestGuardMergeLayout_LegacyMixedSource(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	commitTreeOnBranch(t, dir, "mixed-legacy-source", "main", map[string]string{
		".mindspec/specs/106-x/spec.md": "# flat\n",
		"docs/specs/106-x/spec.md":      "# legacy\n",
	})
	commitTreeOnBranch(t, dir, "flat-target-2", "main", map[string]string{
		".mindspec/specs/106-x/spec.md": "# flat target\n",
	})

	if got, err := g.layoutAtRef("mixed-legacy-source"); err != nil || got != workspace.LayoutMixed {
		t.Fatalf("layoutAtRef(mixed-legacy-source) = (%q, %v), want (mixed, nil)", got, err)
	}

	err := guardMergeLayout("mixed-legacy-source", "flat-target-2", g.layoutAtRef, false)
	if err == nil {
		t.Fatal("a mixed (flat+legacy) source merging onto a flat target must be blocked")
	}
	if !strings.Contains(err.Error(), "layout regression") {
		t.Errorf("error must name the layout regression; got:\n%s", err.Error())
	}
}

// TestGuardMergeLayout_LegacyContextMapMixedSource is a real-git-ref
// integration (Bead 2 / spec 118, AC-22): a source ref carrying a flat
// lifecycle directory AND ONLY a legacy root docs/context-map.md file (no
// legacy lifecycle directory at all) still classifies mixed via the
// independently derived legacy context-map marker, and merging it onto a
// flat target reaches the local merge guard and is BLOCKED.
func TestGuardMergeLayout_LegacyContextMapMixedSource(t *testing.T) {
	g, _, dir := newRepoExecutor(t)

	commitTreeOnBranch(t, dir, "mixed-legacy-contextmap-source", "main", map[string]string{
		".mindspec/specs/106-x/spec.md": "# flat\n",
		"docs/context-map.md":           "# legacy context map\n",
	})
	commitTreeOnBranch(t, dir, "flat-target-3", "main", map[string]string{
		".mindspec/specs/106-x/spec.md": "# flat target\n",
	})

	if got, err := g.layoutAtRef("mixed-legacy-contextmap-source"); err != nil || got != workspace.LayoutMixed {
		t.Fatalf("layoutAtRef(mixed-legacy-contextmap-source) = (%q, %v), want (mixed, nil)", got, err)
	}

	err := guardMergeLayout("mixed-legacy-contextmap-source", "flat-target-3", g.layoutAtRef, false)
	if err == nil {
		t.Fatal("a mixed (flat+legacy-context-map) source merging onto a flat target must be blocked")
	}
	if !strings.Contains(err.Error(), "layout regression") {
		t.Errorf("error must name the layout regression; got:\n%s", err.Error())
	}
}

// TestCompleteBead_LayoutRegressionBlocked is the bead→spec seam integration:
// a CANONICAL bead branch merging onto a FLAT spec target HARD-FAILS at
// CompleteBead's MergeInto seam with the rebase recovery line and mutates
// nothing (spec branch unchanged, no worktree removal).
func TestCompleteBead_LayoutRegressionBlocked(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	// Flat spec target; canonical bead source.
	commitTreeOnBranch(t, dir, "spec/106-x", "main", map[string]string{
		".mindspec/specs/106-x/spec.md": "# flat spec\n",
	})
	commitTreeOnBranch(t, dir, "bead/mindspec-106x.4", "main", map[string]string{
		".mindspec/docs/specs/106-x/spec.md": "# canonical bead\n",
	})

	// The spec worktree must exist on disk so CompleteBead reaches the merge
	// seam (the guard runs in front of MergeInto). The guard blocks before any
	// real merge, so a bare directory suffices.
	specWtPath := filepath.Join(dir, ".worktrees", "worktree-spec-106-x")
	if err := os.MkdirAll(specWtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	specHashBefore := refHash(t, dir, "spec/106-x")

	err := g.CompleteBead("mindspec-106x.4", "spec/106-x", "")
	if err == nil {
		t.Fatal("a canonical bead → flat spec merge must be blocked (layout regression)")
	}
	msg := err.Error()
	if !strings.Contains(msg, "layout regression") {
		t.Errorf("error must name the layout regression; got:\n%s", msg)
	}
	if !strings.Contains(msg, "rebase") {
		t.Errorf("error must carry the rebase recovery; got:\n%s", msg)
	}
	if got := refHash(t, dir, "spec/106-x"); got != specHashBefore {
		t.Errorf("spec branch must be unchanged (guard mutates nothing); was %s, now %s", specHashBefore, got)
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("no worktree removal may happen on a blocked merge; got %v", fake.removeCalls)
	}
	if branchExistsIn(t, dir, "bead/mindspec-106x.4") == false {
		t.Error("bead branch must be preserved on a blocked merge")
	}
}

// TestFinalizeEpic_DirectMergeLayoutRegressionBlocked is the spec→main seam
// integration: a CANONICAL spec branch direct-merging onto a FLAT main
// HARD-FAILS BEFORE any cleanup — main unchanged, spec branch preserved, no
// worktree removal.
func TestFinalizeEpic_DirectMergeLayoutRegressionBlocked(t *testing.T) {
	g, fake, dir := newRepoExecutor(t)

	// Make main FLAT.
	if err := os.MkdirAll(filepath.Join(dir, ".mindspec", "specs", "106-x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mindspec", "specs", "106-x", "spec.md"), []byte("# flat main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, dir, "add", "-A")
	runGitIn(t, dir, "commit", "-q", "-m", "flatten main")

	// Spec branch is CANONICAL, branched from the pre-flatten root so it is
	// ahead of main with a canonical tree.
	rootCommit := refHash(t, dir, "main~1")
	commitTreeOnBranch(t, dir, "spec/106-x", rootCommit, map[string]string{
		".mindspec/docs/specs/106-x/spec.md": "# canonical spec\n",
	})

	fake.listEntries = nil // no bead worktrees
	mainHashBefore := refHash(t, dir, "main")

	_, err := g.FinalizeEpic("epic-1", "106-x", "spec/106-x")
	if err == nil {
		t.Fatal("a canonical spec → flat main direct merge must be blocked (layout regression)")
	}
	msg := err.Error()
	if !strings.Contains(msg, "layout regression") {
		t.Errorf("error must name the layout regression; got:\n%s", msg)
	}
	if got := refHash(t, dir, "main"); got != mainHashBefore {
		t.Errorf("main must be unchanged (guard mutates nothing); was %s, now %s", mainHashBefore, got)
	}
	if !branchExistsIn(t, dir, "spec/106-x") {
		t.Error("spec branch must be preserved on a blocked merge")
	}
	if len(fake.removeCalls) != 0 {
		t.Errorf("no worktree removal may happen before a blocked direct merge; got %v", fake.removeCalls)
	}
}
