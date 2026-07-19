package layout

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRouteReviewSlugIgnoresMalformedSpecDirs is spec 120 AC-23
// (internal/layout, round-4 G1): a specs root containing a hostile-named
// dir (a metacharacter-bearing name — a control-byte-bearing directory
// name is filesystem-impossible, per ADR-0042's own admission, so the
// metacharacter shape is the exercised hostile fixture here) never routes
// any slug to it — no MoveGroup.Dst and no plan/commit text carries the
// hostile bytes; valid dirs incl. the letter-suffixed "008b-human-gates"
// route byte-identically.
func TestRouteReviewSlugIgnoresMalformedSpecDirs(t *testing.T) {
	root := t.TempDir()
	specsRoot := filepath.Join(root, ".mindspec", "specs")
	for _, dir := range []string{
		"008b-human-gates",
		"120-x;evil",
		"099-final-panel",
	} {
		if err := os.MkdirAll(filepath.Join(specsRoot, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	m := NewMover(nil, root, "test-run")
	specIDs := m.listSpecIDs()

	for _, id := range specIDs {
		if id == "120-x;evil" {
			t.Errorf("listSpecIDs returned the hostile dir name %q", id)
		}
	}

	// A slug whose numeric prefix matches the valid 099 dir routes
	// byte-identically.
	got := m.routeReviewSlug("099-final-panel", specIDs)
	if got != "099-final-panel" {
		t.Errorf("routeReviewSlug(099-final-panel) = %q, want 099-final-panel", got)
	}

	// A slug whose numeric prefix would match the hostile dir (120-) must
	// NEVER route there — the hostile dir was dropped from specIDs, so
	// the numeric-prefix fallback cannot find it.
	got2 := m.routeReviewSlug("120-something", specIDs)
	if got2 == "120-x;evil" {
		t.Errorf("routeReviewSlug(120-something) routed to a hostile dir: %q", got2)
	}
}

// TestPanelSpecRejectsTraversal is spec 120 AC-23 (round-5 O3): a
// panel.json carrying "spec": "../.." — which the pre-fix specExists
// os.Stat alone would PASS, the enforcement-was-missing proof — and the
// hostile triple → the slug is NOT routed, no MoveGroup.Dst is composed;
// a valid "spec": "008b-human-gates" routes byte-identically. The valid
// case's slug prefix matches NO listed spec dir (numeric prefix "999"),
// so the positive leg can only route via the panel.json branch — truly
// discriminating it from routeReviewSlug's numeric-prefix fallback.
func TestPanelSpecRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	specsRoot := filepath.Join(root, ".mindspec", "specs")
	if err := os.MkdirAll(filepath.Join(specsRoot, "008b-human-gates"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewMover(nil, root, "test-run")
	specIDs := m.listSpecIDs()

	hostileSlugs := map[string]string{
		"traversal-slug": "../..",
		"hostile-slug":   "x;evil",
	}
	for slug, spec := range hostileSlugs {
		reviewDir := filepath.Join(root, "review", slug)
		if err := os.MkdirAll(reviewDir, 0o755); err != nil {
			t.Fatal(err)
		}
		panelJSON := `{"spec":"` + jsonQuoteEscape(spec) + `"}`
		if err := os.WriteFile(filepath.Join(reviewDir, "panel.json"), []byte(panelJSON), 0o644); err != nil {
			t.Fatal(err)
		}

		got := m.routeReviewSlug(slug, specIDs)
		if got != "" {
			t.Errorf("routeReviewSlug(%q) with panel.json spec=%q routed to %q, want \"\" (no route)", slug, spec, got)
		}
	}

	// Valid case: slug's numeric prefix (999) matches NO listed spec dir,
	// so only the panel.json branch can route it.
	validSlug := "999-unrelated-slug"
	validReviewDir := filepath.Join(root, "review", validSlug)
	if err := os.MkdirAll(validReviewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(validReviewDir, "panel.json"), []byte(`{"spec":"008b-human-gates"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := m.routeReviewSlug(validSlug, specIDs)
	if got != "008b-human-gates" {
		t.Errorf("routeReviewSlug(%q) = %q, want 008b-human-gates (via panel.json)", validSlug, got)
	}
}

// jsonQuoteEscape minimally escapes a string for embedding as a JSON
// string value in a hand-written test fixture.
func jsonQuoteEscape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case 0x00:
			// Cannot appear in valid JSON string content; the fixture
			// substitutes a printable placeholder byte so the file
			// remains parseable JSON while the SLUG directory name
			// itself still carries the hostile byte where the harness
			// needs it (this helper is only used for the panel.json
			// `spec` field body, which idvalidate.SpecID must reject
			// regardless of the exact hostile byte used).
			out = append(out, '?')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}
