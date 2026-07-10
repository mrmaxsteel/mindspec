package complete

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// Spec 106 Bead 4 (AC13): the AUTHORITATIVE panel gate's scan roots are
// LAYOUT-AWARE. On a canonical/legacy tree the gate honors BOTH the repo-root
// review/<slug> panels AND the co-located <spec-dir>/reviews/<slug> panels (the
// transition union); on a flat tree it honors the co-located reviews ONLY, and a
// leftover repo-root review/ panel no longer drives the gate.

// makeCanonicalSpecDir creates the canonical .mindspec/docs/specs/<id> dir so
// DetectLayout classifies the tree canonical and SpecDir resolves there.
func makeCanonicalSpecDir(t *testing.T, root, specID string) string {
	t.Helper()
	dir := filepath.Join(root, ".mindspec", "docs", "specs", specID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// makeFlatSpecDir creates the flat .mindspec/specs/<id> dir so DetectLayout
// classifies the tree flat and SpecDir resolves there.
func makeFlatSpecDir(t *testing.T, root, specID string) string {
	t.Helper()
	dir := filepath.Join(root, ".mindspec", "specs", specID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeCoLocatedPanel writes a panel under <spec-dir>/reviews/<slug>/ (the
// co-located convention) — the sibling of workspace.RecordingDir.
func writeCoLocatedPanel(t *testing.T, specDir, slug string, p panel.Panel, verdicts map[string]string) {
	t.Helper()
	dir := filepath.Join(specDir, "reviews", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(p)
	if err := os.WriteFile(filepath.Join(dir, "panel.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, v := range verdicts {
		vd, _ := json.Marshal(map[string]string{"verdict": v})
		if err := os.WriteFile(filepath.Join(dir, name), vd, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func containsRoot(roots []string, want string) bool {
	for _, r := range roots {
		if r == want {
			return true
		}
	}
	return false
}

// TestPanelGateRoots_LayoutAware pins the root-selection rule directly: a
// canonical tree includes BOTH the repo root and the co-located spec dir; a flat
// tree includes the spec dir ONLY and EXCLUDES the repo root.
func TestPanelGateRoots_LayoutAware(t *testing.T) {
	t.Run("canonical unions repo-root and spec-dir", func(t *testing.T) {
		root := t.TempDir()
		specDir := makeCanonicalSpecDir(t, root, "106-canon")

		roots := panelGateRoots(root, "", "106-canon")
		if !containsRoot(roots, root) {
			t.Errorf("canonical roots must include the repo root %s; got %v", root, roots)
		}
		if !containsRoot(roots, specDir) {
			t.Errorf("canonical roots must include the co-located spec dir %s; got %v", specDir, roots)
		}
	})

	t.Run("flat is co-located only, repo-root excluded", func(t *testing.T) {
		root := t.TempDir()
		specDir := makeFlatSpecDir(t, root, "106-flat")

		roots := panelGateRoots(root, "", "106-flat")
		if containsRoot(roots, root) {
			t.Errorf("a flat tree must NOT scan the repo root (root review/ ignored); got %v", roots)
		}
		if !containsRoot(roots, specDir) {
			t.Errorf("a flat tree must scan the co-located spec dir %s; got %v", specDir, roots)
		}
		if len(roots) != 1 {
			t.Errorf("flat roots should be the spec dir only; got %v", roots)
		}
	})
}

// TestPanelGate_CoLocatedCanonical_Blocks proves a sub-threshold panel under the
// co-located <spec-dir>/reviews/<slug> convention BLOCKS complete on a canonical
// tree — the new transition surface, exercised end-to-end through complete.Run.
func TestPanelGate_CoLocatedCanonical_Blocks(t *testing.T) {
	const specID, beadID = "106-canon", "mindspec-106c.4"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	specDir := makeCanonicalSpecDir(t, root, specID)
	writeCoLocatedPanel(t, specDir, specID+"-bd04", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA, // fresh → the BLOCK is the threshold clause
	}, subThresholdVerdicts())

	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	res, err := Run(root, beadID, specID, "", ex, CompleteOpts{})
	if err == nil {
		t.Fatal("a sub-threshold CO-LOCATED panel must BLOCK complete on a canonical tree")
	}
	if ex.completeCalled {
		t.Error("block must be PRE-merge: CompleteBead must not run")
	}
	if res != nil {
		t.Errorf("a blocked complete returns nil result; got %+v", res)
	}
	if !strings.Contains(err.Error(), "APPROVE") {
		t.Errorf("block message should name the threshold tally; got:\n%s", err.Error())
	}
}

// TestPanelGate_FlatIgnoresRootReview_CoLocatedDrives proves the flat-tree rule:
// a repo-root review/<slug> panel is IGNORED (does not drive the gate once the
// tree is flat), while the co-located <spec-dir>/reviews/<slug> panel DOES drive
// it. Exercised at the authoritative panelGate over the layout-aware roots.
func TestPanelGate_FlatIgnoresRootReview_CoLocatedDrives(t *testing.T) {
	const specID, beadID = "106-flat", "mindspec-106f.4"
	root, beadSHA := setupPanelGateRepo(t, specID, beadID)
	specDir := makeFlatSpecDir(t, root, specID)

	// A sub-threshold panel under the OLD repo-root review/ convention.
	writePanel(t, root, specID+"-root", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
	}, subThresholdVerdicts())

	roots := panelGateRoots(root, "", specID)

	// On a flat tree the repo-root panel is NOT scanned → no registration →
	// fail-open: the gate does NOT block.
	if _, err := panelGate(beadID, roots, "", true, nil); err != nil {
		t.Fatalf("a root review/ panel must be IGNORED on a flat tree (not block); got: %v", err)
	}

	// The co-located panel under the flat spec dir DOES drive the gate.
	writeCoLocatedPanel(t, specDir, specID+"-bd04", panel.Panel{
		BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
		ReviewedHeadSHA: beadSHA,
	}, subThresholdVerdicts())

	_, err := panelGate(beadID, roots, "", true, nil)
	if err == nil {
		t.Fatal("a sub-threshold CO-LOCATED panel must BLOCK on a flat tree")
	}
	if !strings.Contains(err.Error(), "APPROVE") {
		t.Errorf("co-located block should name the threshold tally; got:\n%s", err.Error())
	}
}
