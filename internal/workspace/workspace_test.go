package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRoot_MindspecDir(t *testing.T) {
	tmp := t.TempDir()
	// Create .mindspec/ directory marker
	if err := os.Mkdir(filepath.Join(tmp, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRoot(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected root %q, got %q", tmp, root)
	}
}

func TestFindRoot_GitOnly(t *testing.T) {
	tmp := t.TempDir()
	// Create .git directory (no .mindspec/)
	if err := os.Mkdir(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRoot(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected root %q, got %q", tmp, root)
	}
}

func TestFindRoot_WalksUp(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmp, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRoot(nested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected root %q, got %q", tmp, root)
	}
}

func TestFindRoot_MindspecDirPriority(t *testing.T) {
	tmp := t.TempDir()
	// Both .mindspec/ and .git exist — .mindspec/ should be found first
	if err := os.Mkdir(filepath.Join(tmp, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRoot(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected root %q, got %q", tmp, root)
	}
}

func TestFindRoot_NoMarker(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "isolated")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := FindRoot(nested)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrNoRoot {
		t.Errorf("expected ErrNoRoot, got %v", err)
	}
}

func TestDocsDir(t *testing.T) {
	got := DocsDir("/project")
	want := filepath.Join("/project", "docs")
	if got != want {
		t.Errorf("DocsDir: got %q, want %q", got, want)
	}
}

func TestDocsDir_CanonicalPreferred(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}

	got := DocsDir(root)
	if got != canonical {
		t.Errorf("DocsDir canonical: got %q, want %q", got, canonical)
	}
}

func TestSpecDir_UsesDocsDir(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}

	got := SpecDir(root, "001-test")
	want := filepath.Join(canonical, "specs", "001-test")
	if got != want {
		t.Errorf("SpecDir canonical: got %q, want %q", got, want)
	}
}

func TestContextMapPath_UsesDocsDir(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ContextMapPath(root)
	want := filepath.Join(canonical, "context-map.md")
	if got != want {
		t.Errorf("ContextMapPath canonical: got %q, want %q", got, want)
	}
}

func TestADRDir_UsesDocsDir(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ADRDir(root)
	want := filepath.Join(canonical, "adr")
	if got != want {
		t.Errorf("ADRDir canonical: got %q, want %q", got, want)
	}
}

func TestDomainDir_UsesDocsDir(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}

	got := DomainDir(root, "core")
	want := filepath.Join(canonical, "domains", "core")
	if got != want {
		t.Errorf("DomainDir canonical: got %q, want %q", got, want)
	}
}

func TestRecordingDir_UsesSpecDir(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}

	got := RecordingDir(root, "001-test")
	want := filepath.Join(canonical, "specs", "001-test", "recording")
	if got != want {
		t.Errorf("RecordingDir canonical: got %q, want %q", got, want)
	}
}

func TestGlossaryPath(t *testing.T) {
	root := "/project"
	got := GlossaryPath(root)
	want := filepath.Join(root, "GLOSSARY.md")
	if got != want {
		t.Errorf("GlossaryPath: got %q, want %q", got, want)
	}
}

func TestGlossaryPath_CanonicalPreferred(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, ".mindspec", "docs")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}
	glossary := filepath.Join(canonical, "glossary.md")
	if err := os.WriteFile(glossary, []byte("# glossary"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := GlossaryPath(root)
	if got != glossary {
		t.Errorf("GlossaryPath canonical: got %q, want %q", got, glossary)
	}
}

func TestPoliciesPath(t *testing.T) {
	got := PoliciesPath("/project")
	want := filepath.Join("/project", ".mindspec", "policies.yml")
	if got != want {
		t.Errorf("PoliciesPath: got %q, want %q", got, want)
	}
}

func TestCanonicalAndLegacyDocsDir(t *testing.T) {
	root := "/project"
	if got := CanonicalDocsDir(root); got != filepath.Join(root, ".mindspec", "docs") {
		t.Errorf("CanonicalDocsDir: got %q", got)
	}
	if got := LegacyDocsDir(root); got != filepath.Join(root, "docs") {
		t.Errorf("LegacyDocsDir: got %q", got)
	}
}

func TestLegacyPoliciesPath(t *testing.T) {
	got := LegacyPoliciesPath("/project")
	want := filepath.Join("/project", "architecture", "policies.yml")
	if got != want {
		t.Errorf("LegacyPoliciesPath: got %q, want %q", got, want)
	}
}
