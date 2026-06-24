package panel

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a fixture helper: writes content at root/rel, creating
// parent dirs.
func writeFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const beadPanelJSON = `{
  "bead_id": "mindspec-x.1",
  "spec": "093-skills-thin-down",
  "target": "bead/mindspec-x.1",
  "round": 1,
  "expected_reviewers": 6,
  "reviewed_head_sha": "abc1234abc1234abc1234abc1234abc1234abc12"
}`

func TestScan_FindsRegisteredPanels(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/bead-x1-panel/panel.json", beadPanelJSON)
	writeFile(t, root, "review/other-panel/panel.json", `{"bead_id": null, "spec": "s", "target": "spec/s", "round": 2, "expected_reviewers": 6, "reviewed_head_sha": "ffff"}`)

	regs := Scan(root)
	if len(regs) != 2 {
		t.Fatalf("expected 2 registrations, got %d: %+v", len(regs), regs)
	}
	// Sorted by dir: bead-x1-panel < other-panel.
	if regs[0].Slug() != "bead-x1-panel" || regs[1].Slug() != "other-panel" {
		t.Errorf("unexpected order: %q, %q", regs[0].Slug(), regs[1].Slug())
	}
	if regs[0].Err != nil {
		t.Fatalf("unexpected parse error: %v", regs[0].Err)
	}
	p := regs[0].Panel
	if !p.IsBead() || *p.BeadID != "mindspec-x.1" {
		t.Errorf("bead_id not parsed: %+v", p)
	}
	if p.ExpectedReviewers != 6 || p.Round != 1 || p.ReviewedHeadSHA == "" {
		t.Errorf("schema fields not parsed: %+v", p)
	}
	if regs[1].Panel.IsBead() {
		t.Errorf("null bead_id must parse as non-bead: %+v", regs[1].Panel)
	}
}

// TestScan_FindsCoLocatedReviewsPanels (Spec 106 Bead 4, AC13): Scan globs the
// spec-scoped `reviews/<slug>/panel.json` convention as well as the repo-root
// `review/<slug>/panel.json` one, so handing it a spec-dir root surfaces the
// co-located panel. The literal `review/` must not substring-match `reviews/`.
func TestScan_FindsCoLocatedReviewsPanels(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "docs", "specs", "106-x")
	// Co-located panel under <spec-dir>/reviews/<slug>/.
	writeFile(t, specDir, "reviews/106-bead4/panel.json", beadPanelJSON)
	// A repo-root review/ panel under the repo root.
	writeFile(t, root, "review/106-root/panel.json", beadPanelJSON)

	// Scanning the spec dir finds ONLY the co-located reviews/ panel.
	specRegs := Scan(specDir)
	if len(specRegs) != 1 || specRegs[0].Slug() != "106-bead4" {
		t.Fatalf("spec-dir scan should find the co-located reviews/ panel; got %+v", specRegs)
	}

	// Scanning the repo root finds ONLY the root review/ panel (reviews/ lives
	// under the spec dir, not the repo root).
	rootRegs := Scan(root)
	if len(rootRegs) != 1 || rootRegs[0].Slug() != "106-root" {
		t.Fatalf("repo-root scan should find the root review/ panel; got %+v", rootRegs)
	}

	// The union (both roots) dedupes to both distinct panels.
	union := Scan(root, specDir)
	if len(union) != 2 {
		t.Fatalf("union scan should find both panels, got %d: %+v", len(union), union)
	}
}

func TestScan_LegacyBriefOnlyDirIsUnregistered(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/legacy-panel/BRIEF.md", "# brief")
	writeFile(t, root, "review/legacy-panel/claude-a-round-1.json", `{"verdict":"APPROVE"}`)

	if regs := Scan(root); len(regs) != 0 {
		t.Fatalf("legacy BRIEF-only dir must not register (HC-4 fail-open), got %+v", regs)
	}
}

func TestScan_MissingRootsAndEmptyRootSkipped(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/p/panel.json", beadPanelJSON)

	regs := Scan("", filepath.Join(root, "does-not-exist"), root)
	if len(regs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(regs))
	}
}

func TestScan_DedupesOverlappingRoots(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/p/panel.json", beadPanelJSON)

	// Same root twice, plus a non-clean alias of it.
	alias := filepath.Join(root, ".", "subdir", "..")
	if err := os.MkdirAll(filepath.Join(root, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	regs := Scan(root, root, alias)
	if len(regs) != 1 {
		t.Fatalf("expected deduped single registration, got %d: %+v", len(regs), regs)
	}
}

func TestScan_MalformedPanelJSONStillRegistersWithErr(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/broken/panel.json", `{not json`)

	regs := Scan(root)
	if len(regs) != 1 {
		t.Fatalf("malformed panel.json must still register, got %d", len(regs))
	}
	if regs[0].Err == nil {
		t.Fatal("expected Err on malformed registration")
	}
}

func TestForBead(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/mine/panel.json", beadPanelJSON)
	writeFile(t, root, "review/theirs/panel.json", `{"bead_id":"mindspec-y.9","spec":"s","target":"bead/mindspec-y.9","round":1,"expected_reviewers":6,"reviewed_head_sha":"a"}`)
	writeFile(t, root, "review/null-target/panel.json", `{"bead_id":null,"spec":"s","target":"spec/s","round":1,"expected_reviewers":6,"reviewed_head_sha":"a"}`)
	writeFile(t, root, "review/broken/panel.json", `{`)

	regs := Scan(root)
	got := ForBead(regs, "mindspec-x.1")
	if len(got) != 1 || got[0].Slug() != "mine" {
		t.Fatalf("ForBead mismatch: %+v", got)
	}
	if got := ForBead(regs, ""); got != nil {
		t.Fatalf("empty bead id must match nothing, got %+v", got)
	}
}

func TestApproveThreshold(t *testing.T) {
	cases := []struct {
		expected int
		want     int
	}{
		{6, 5},  // default panel: 5-of-6
		{3, 2},  // DQ5 parameterization fixture
		{1, 0},  // degenerate but well-defined
		{0, 0},  // malformed registration: never a free pass
		{-2, 0}, // malformed registration
	}
	for _, c := range cases {
		p := Panel{ExpectedReviewers: c.expected}
		if got := p.ApproveThreshold(); got != c.want {
			t.Errorf("ApproveThreshold(expected_reviewers=%d) = %d, want %d", c.expected, got, c.want)
		}
	}
}

func TestPanel_AbandonedFieldsParse(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "review/dead/panel.json", `{
	  "bead_id": "mindspec-x.2", "spec": "s", "target": "bead/mindspec-x.2",
	  "round": 3, "expected_reviewers": 6, "reviewed_head_sha": "beef",
	  "abandoned": true,
	  "abandon_reason": "max: superseded by spec rescope, see mindspec-x.3"
	}`)
	regs := Scan(root)
	if len(regs) != 1 || regs[0].Err != nil {
		t.Fatalf("scan failed: %+v", regs)
	}
	p := regs[0].Panel
	if !p.Abandoned || p.AbandonReason == "" {
		t.Errorf("abandoned/abandon_reason not parsed: %+v", p)
	}
}
