package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupCreateEnv(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	adrDir := filepath.Join(root, "docs", "adr")
	tmplDir := filepath.Join(root, "docs", "templates")
	os.MkdirAll(adrDir, 0o755)
	os.MkdirAll(tmplDir, 0o755)

	// Write existing ADRs
	os.WriteFile(filepath.Join(adrDir, "ADR-0001.md"), []byte(testADR1), 0o644)
	os.WriteFile(filepath.Join(adrDir, "ADR-0002.md"), []byte(testADR2), 0o644)

	// Write template
	tmpl := `# ADR-NNNN: <Title>

- **Date**: <YYYY-MM-DD>
- **Status**: Proposed
- **Domain(s)**: <comma-separated list>
- **Deciders**: <who decides>
- **Supersedes**: n/a
- **Superseded-by**: n/a

## Context

<What is the issue?>

## Decision

<What is the change?>
`
	os.WriteFile(filepath.Join(tmplDir, "adr.md"), []byte(tmpl), 0o644)

	return root
}

func TestCreate_HappyPath(t *testing.T) {
	root := setupCreateEnv(t)

	path, err := Create(root, "Use Redis for caching", CreateOpts{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// R5(a) (spec 123): create now emits a SLUGGED filename derived from
	// the title, not the bare ADR-0003.md the pre-123 behavior wrote.
	if !strings.HasSuffix(path, "ADR-0003-use-redis-for-caching.md") {
		t.Errorf("path = %q, want suffix ADR-0003-use-redis-for-caching.md", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "# ADR-0003: Use Redis for caching") {
		t.Error("expected title in heading")
	}
	if !strings.Contains(content, "**Status**: Proposed") {
		t.Error("expected Proposed status")
	}
}

func TestCreate_EmptyTitle(t *testing.T) {
	root := setupCreateEnv(t)

	_, err := Create(root, "", CreateOpts{})
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestCreate_WithDomains(t *testing.T) {
	root := setupCreateEnv(t)

	path, err := Create(root, "Test", CreateOpts{Domains: []string{"core", "workflow"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "core, workflow") {
		t.Errorf("expected domains in content, got:\n%s", string(data))
	}
}

func TestCreate_WithSupersedes(t *testing.T) {
	root := setupCreateEnv(t)

	path, err := Create(root, "New Approach", CreateOpts{Supersedes: "ADR-0001"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Check new ADR has Supersedes field
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "**Supersedes**: ADR-0001") {
		t.Error("new ADR should reference superseded ADR")
	}

	// Domains should be copied from old ADR
	if !strings.Contains(content, "core, context-system") {
		t.Errorf("expected inherited domains, got:\n%s", content)
	}

	// Check old ADR was updated
	oldData, _ := os.ReadFile(filepath.Join(root, "docs", "adr", "ADR-0001.md"))
	if !strings.Contains(string(oldData), "**Superseded-by**: ADR-0003") {
		t.Errorf("old ADR should reference new ADR, got:\n%s", string(oldData))
	}
}

func TestCreate_SupersedesNotFound(t *testing.T) {
	root := setupCreateEnv(t)

	_, err := Create(root, "Test", CreateOpts{Supersedes: "ADR-9999"})
	if err == nil {
		t.Error("expected error for nonexistent superseded ADR")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}

// TestCreateWithIDUsesSuppliedID asserts the Spec 087 Bead 3
// placeholder-creation helper writes to the user-supplied ID verbatim
// (revision 1: deterministic falsifiability for TestSupersedeUnblocks).
func TestCreateWithIDUsesSuppliedID(t *testing.T) {
	root := setupCreateEnv(t)

	path, err := CreateWithID(root, "ADR-0099", "Placeholder for ADR-0099", CreateOpts{
		Domains: []string{"core"},
	})
	if err != nil {
		t.Fatalf("CreateWithID: %v", err)
	}

	wantSuffix := filepath.Join("docs", "adr", "ADR-0099.md")
	if !strings.HasSuffix(path, wantSuffix) {
		t.Errorf("path = %q, want suffix %q", path, wantSuffix)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "# ADR-0099: Placeholder for ADR-0099") {
		t.Errorf("expected title heading with supplied ID, got:\n%s", content)
	}
	if !strings.Contains(content, "**Status**: Proposed") {
		t.Error("expected Status: Proposed in placeholder")
	}
	if !strings.Contains(content, "**Domain(s)**: core") {
		t.Errorf("expected Domain(s) to include seeded domain, got:\n%s", content)
	}
}

// TestCreateWithIDRejectsExisting asserts the collision path: invoking
// CreateWithID against an already-existing ADR id (exact filename match)
// returns an error containing the substring "already exists" and writes
// no file.
func TestCreateWithIDRejectsExisting(t *testing.T) {
	root := setupCreateEnv(t)

	// First call succeeds.
	if _, err := CreateWithID(root, "ADR-0099", "First", CreateOpts{}); err != nil {
		t.Fatalf("first CreateWithID: %v", err)
	}

	// Second call against the same ID must fail.
	_, err := CreateWithID(root, "ADR-0099", "Second", CreateOpts{})
	if err == nil {
		t.Fatal("expected error for existing ADR ID")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error must contain 'already exists', got: %v", err)
	}

	// The on-disk file must still be the "First" content — collision
	// must not overwrite.
	data, _ := os.ReadFile(filepath.Join(root, "docs", "adr", "ADR-0099.md"))
	if !strings.Contains(string(data), "First") {
		t.Errorf("collision must not overwrite original; got:\n%s", string(data))
	}
}

// TestCreateWithIDRejectsExistingSlug asserts collision detection also
// catches the slugged-filename shape (e.g. ADR-0099-foo.md) when the
// caller passes the bare "ADR-0099" id.
func TestCreateWithIDRejectsExistingSlug(t *testing.T) {
	root := setupCreateEnv(t)

	// Seed a slugged file.
	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-0099-existing-slug.md"), []byte("# existing\n"), 0o644); err != nil {
		t.Fatalf("seed slug file: %v", err)
	}

	_, err := CreateWithID(root, "ADR-0099", "Placeholder", CreateOpts{})
	if err == nil {
		t.Fatal("expected error for existing slugged ADR ID")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error must contain 'already exists', got: %v", err)
	}
}

// TestCreate_SupersedesResolvesSluggedPredecessor is the AC-9(ii) pin
// (spec 123 R5(c)): `--supersedes ADR-0001` must resolve a SLUGGED
// on-disk predecessor (ADR-0001-legacy-slug.md) through the exact-join
// --supersedes path, not just show's pre-existing glob fallback. RED on
// revert to the exact-join workspace.ADRFilePath (which only ever tried
// "ADR-0001.md" and reported "not found" against a slugged file).
func TestCreate_SupersedesResolvesSluggedPredecessor(t *testing.T) {
	root := setupCreateEnv(t)

	// Replace the bare ADR-0001.md fixture with a slugged sibling that
	// carries the SAME canonical number, mirroring a real slugged-create
	// predecessor.
	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.Remove(filepath.Join(adrDir, "ADR-0001.md")); err != nil {
		t.Fatalf("remove bare ADR-0001.md: %v", err)
	}
	sluggedPath := filepath.Join(adrDir, "ADR-0001-legacy-slug.md")
	if err := os.WriteFile(sluggedPath, []byte(testADR1), 0o644); err != nil {
		t.Fatalf("write slugged predecessor: %v", err)
	}

	path, err := Create(root, "Successor Decision", CreateOpts{Supersedes: "ADR-0001"})
	if err != nil {
		t.Fatalf("Create with --supersedes against a slugged predecessor: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "**Supersedes**: ADR-0001") {
		t.Error("new ADR should reference the superseded predecessor")
	}
	// Domains should be copied from the slugged predecessor (core, context-system).
	if !strings.Contains(string(data), "core, context-system") {
		t.Errorf("expected inherited domains from slugged predecessor, got:\n%s", string(data))
	}

	// The slugged predecessor itself must have been updated in place.
	oldData, err := os.ReadFile(sluggedPath)
	if err != nil {
		t.Fatalf("reading slugged predecessor: %v", err)
	}
	if !strings.Contains(string(oldData), "**Superseded-by**:") {
		t.Errorf("slugged predecessor should have been updated with Superseded-by, got:\n%s", string(oldData))
	}
}

// TestCreate_MixedDirectoryNumberingGuard is the AC-10 numbering-floor
// GUARD (spec 123 R5(d)): a directory holding a bare ADR-0001.md and a
// slugged ADR-0002-foo.md must allocate the NEXT create at 0003 — the
// numbering floor (maxADRNum, parse.go) is already slug-aware and must
// stay that way. Not a RED pin; this locks the guard against regression
// alongside the AC-9/AC-10 resolution changes.
func TestCreate_MixedDirectoryNumberingGuard(t *testing.T) {
	root := t.TempDir()
	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-0001.md"), []byte(testADR1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-0002-foo.md"), []byte(testADR2), 0o644); err != nil {
		t.Fatal(err)
	}

	adrs, err := ScanADRs(root)
	if err != nil {
		t.Fatalf("ScanADRs: %v", err)
	}
	gotIDs := map[string]bool{}
	for _, a := range adrs {
		gotIDs[a.ID] = true
	}
	if !gotIDs["ADR-0001"] || !gotIDs["ADR-0002"] {
		t.Fatalf("expected canonical IDs ADR-0001 and ADR-0002, got %v", adrs)
	}

	path, err := Create(root, "Next Decision", CreateOpts{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.Contains(path, "ADR-0003-") {
		t.Errorf("path = %q, want next allocation ADR-0003-* (numbering floor over mixed bare+slugged dir)", path)
	}
}

func TestCreate_SupersedesWithExplicitDomains(t *testing.T) {
	root := setupCreateEnv(t)

	path, err := Create(root, "Override Domains", CreateOpts{
		Supersedes: "ADR-0001",
		Domains:    []string{"new-domain"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	// Should use explicit domains, not inherited ones
	if !strings.Contains(content, "new-domain") {
		t.Error("expected explicit domain override")
	}
	if strings.Contains(content, "context-system") {
		t.Error("should not inherit domains when explicitly provided")
	}
}
