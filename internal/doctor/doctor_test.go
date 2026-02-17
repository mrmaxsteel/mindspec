package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasFailures(t *testing.T) {
	r := &Report{
		Checks: []Check{
			{Name: "test", Status: OK},
			{Name: "test2", Status: Warn},
		},
	}
	if r.HasFailures() {
		t.Error("expected no failures with OK and Warn only")
	}

	r.Checks = append(r.Checks, Check{Name: "test3", Status: Missing})
	if !r.HasFailures() {
		t.Error("expected failures with Missing status")
	}

	r2 := &Report{
		Checks: []Check{
			{Name: "test", Status: OK},
			{Name: "test2", Status: Error},
		},
	}
	if !r2.HasFailures() {
		t.Error("expected failures with Error status")
	}
}

// setupDocsFixture creates a project root with standard MindSpec doc structure.
func setupDocsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	dirs := []string{
		"docs/core",
		"docs/domains/core",
		"docs/domains/context-system",
		"docs/domains/workflow",
		"docs/specs",
		"architecture",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Domain files
	domainFiles := []string{
		"docs/domains/core/overview.md",
		"docs/domains/core/architecture.md",
		"docs/domains/core/interfaces.md",
		"docs/domains/core/runbook.md",
		"docs/domains/context-system/overview.md",
		"docs/domains/context-system/architecture.md",
		"docs/domains/context-system/interfaces.md",
		"docs/domains/context-system/runbook.md",
		"docs/domains/workflow/overview.md",
		"docs/domains/workflow/architecture.md",
		"docs/domains/workflow/interfaces.md",
		"docs/domains/workflow/runbook.md",
	}
	for _, f := range domainFiles {
		if err := os.WriteFile(filepath.Join(root, f), []byte("# placeholder"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// context-map.md
	if err := os.WriteFile(filepath.Join(root, "docs", "context-map.md"), []byte("# Context Map"), 0644); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestCheckDocs_AllPresent(t *testing.T) {
	root := setupDocsFixture(t)

	// Create a glossary with a valid link
	if err := os.WriteFile(filepath.Join(root, "docs/core/ARCHITECTURE.md"), []byte("# arch"), 0644); err != nil {
		t.Fatal(err)
	}
	glossary := `| Term | Target |
|:-----|:-------|
| **Bead** | [docs/core/ARCHITECTURE.md](docs/core/ARCHITECTURE.md) |
`
	if err := os.WriteFile(filepath.Join(root, "GLOSSARY.md"), []byte(glossary), 0644); err != nil {
		t.Fatal(err)
	}

	r := &Report{}
	checkDocs(r, root)

	for _, c := range r.Checks {
		if c.Status == Missing || c.Status == Error {
			t.Errorf("check %q: unexpected status %d (%s)", c.Name, c.Status, c.Message)
		}
	}
}

func TestCheckDocs_MissingDirs(t *testing.T) {
	root := t.TempDir()
	// Empty root — nothing exists

	r := &Report{}
	checkDocs(r, root)

	// Should have missing checks for required dirs
	missingCount := 0
	for _, c := range r.Checks {
		if c.Status == Missing {
			missingCount++
		}
	}
	if missingCount == 0 {
		t.Error("expected missing checks for empty root")
	}
}

func TestCheckGlossary_BrokenLinks(t *testing.T) {
	root := t.TempDir()
	glossary := `| Term | Target |
|:-----|:-------|
| **Missing** | [does-not-exist.md](does-not-exist.md) |
`
	if err := os.WriteFile(filepath.Join(root, "GLOSSARY.md"), []byte(glossary), 0644); err != nil {
		t.Fatal(err)
	}

	r := &Report{}
	checkGlossary(r, root)

	foundBroken := false
	for _, c := range r.Checks {
		if c.Name == "Glossary links" && c.Status == Error {
			foundBroken = true
		}
	}
	if !foundBroken {
		t.Error("expected broken link error")
	}
}

func TestCheckBeads_DirMissing(t *testing.T) {
	root := t.TempDir()

	r := &Report{}
	checkBeads(r, root)

	if len(r.Checks) == 0 {
		t.Fatal("expected at least one check")
	}
	if r.Checks[0].Status != Missing {
		t.Errorf("expected Missing status, got %d", r.Checks[0].Status)
	}
}

func TestCheckBeads_DurableState(t *testing.T) {
	root := t.TempDir()
	beadsDir := filepath.Join(root, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	r := &Report{}
	checkBeads(r, root)

	foundDurable := false
	for _, c := range r.Checks {
		if c.Name == "Beads durable state" && c.Status == OK {
			foundDurable = true
		}
	}
	if !foundDurable {
		t.Error("expected durable state OK check")
	}
}

func TestIsRuntimeArtifact(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"bd.sock", true},
		{"daemon.lock", true},
		{"bd.db", true},
		{"issues.jsonl", false},
		{"config.yaml", false},
		{"foo.db-wal", true},
		{"foo.db-shm", true},
		{"something.db", true},
		{"readme.md", false},
	}
	for _, tt := range tests {
		if got := isRuntimeArtifact(tt.name); got != tt.want {
			t.Errorf("isRuntimeArtifact(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestCheckDocs_CanonicalMigratedLayoutPasses(t *testing.T) {
	root := setupCanonicalMigratedFixture(t)

	r := &Report{}
	checkDocs(r, root)

	for _, c := range r.Checks {
		if c.Status == Missing || c.Status == Error {
			t.Fatalf("check %q: unexpected status %d (%s)", c.Name, c.Status, c.Message)
		}
	}
}

func TestCheckMigrationMetadata_MissingLineageManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs_archive", "run-1"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := &Report{}
	checkMigrationMetadata(r, root)

	foundMissing := false
	for _, c := range r.Checks {
		if c.Name == ".mindspec/lineage/manifest.json" && c.Status == Missing {
			foundMissing = true
		}
	}
	if !foundMissing {
		t.Fatal("expected missing lineage manifest check")
	}
}

func setupCanonicalMigratedFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	dirs := []string{
		".mindspec/docs/core",
		".mindspec/docs/domains/core",
		".mindspec/docs/domains/context-system",
		".mindspec/docs/domains/workflow",
		".mindspec/docs/specs",
		".mindspec/lineage",
		".mindspec/migrations/run-1",
		"docs_archive/run-1/docs/core",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	write := func(rel, content string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(".mindspec/docs/core/ARCHITECTURE.md", "# Architecture")
	write(".mindspec/docs/context-map.md", "# Context Map")
	write(".mindspec/docs/glossary.md", `| Term | Target |
|:-----|:-------|
| **Bead** | [.mindspec/docs/core/ARCHITECTURE.md](.mindspec/docs/core/ARCHITECTURE.md) |
`)

	domainFiles := []string{
		".mindspec/docs/domains/core/overview.md",
		".mindspec/docs/domains/core/architecture.md",
		".mindspec/docs/domains/core/interfaces.md",
		".mindspec/docs/domains/core/runbook.md",
		".mindspec/docs/domains/context-system/overview.md",
		".mindspec/docs/domains/context-system/architecture.md",
		".mindspec/docs/domains/context-system/interfaces.md",
		".mindspec/docs/domains/context-system/runbook.md",
		".mindspec/docs/domains/workflow/overview.md",
		".mindspec/docs/domains/workflow/architecture.md",
		".mindspec/docs/domains/workflow/interfaces.md",
		".mindspec/docs/domains/workflow/runbook.md",
	}
	for _, f := range domainFiles {
		write(f, "# placeholder")
	}

	write("docs_archive/run-1/docs/core/ARCHITECTURE.md", "# archived")
	write(".mindspec/lineage/manifest.json", `{
  "run_id": "run-1",
  "entries": [
    {
      "source": "docs/core/ARCHITECTURE.md",
      "canonical": ".mindspec/docs/core/ARCHITECTURE.md",
      "archive": "docs_archive/run-1/docs/core/ARCHITECTURE.md"
    }
  ]
}
`)
	write(".mindspec/migrations/run-1/inventory.json", `[
  {"path":"docs/core/ARCHITECTURE.md","sha256":"abc"}
]
`)
	write(".mindspec/migrations/run-1/classification.json", `[
  {
    "path":"docs/core/ARCHITECTURE.md",
    "sha256":"abc",
    "category":"core",
    "confidence":0.92,
    "rule":"path-contains-core",
    "requires_llm":false
  }
]
`)
	write(".mindspec/migrations/run-1/lineage.json", `[
  {
    "source":"docs/core/ARCHITECTURE.md",
    "source_sha256":"abc",
    "category":"core",
    "canonical":".mindspec/docs/core/ARCHITECTURE.md",
    "archive":"docs_archive/run-1/docs/core/ARCHITECTURE.md"
  }
]
`)
	write(".mindspec/migrations/run-1/state.json", `{
  "run_id":"run-1",
  "stage":"applied"
}
`)

	return root
}
