package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun_EmptyDir(t *testing.T) {
	root := t.TempDir()

	result, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(result.Created) == 0 {
		t.Fatal("expected items to be created, got none")
	}
	if len(result.Skipped) != 0 {
		t.Errorf("expected no skipped items, got %d", len(result.Skipped))
	}

	// Verify key files exist
	requiredFiles := []string{
		"GLOSSARY.md",
		"CLAUDE.md",
		"docs/context-map.md",
		".mindspec/policies.yml",
		".mindspec/state.json",
		"docs/templates/spec.md",
		"docs/templates/plan.md",
		"docs/templates/adr.md",
		"docs/domains/core/overview.md",
		"docs/domains/core/architecture.md",
		"docs/domains/core/interfaces.md",
		"docs/domains/core/runbook.md",
		"docs/domains/context-system/overview.md",
		"docs/domains/workflow/overview.md",
	}
	for _, f := range requiredFiles {
		p := filepath.Join(root, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Verify key dirs exist
	requiredDirs := []string{
		"docs/core",
		"docs/domains",
		"docs/specs",
		"docs/adr",
		".mindspec",
	}
	for _, d := range requiredDirs {
		p := filepath.Join(root, d)
		info, err := os.Stat(p)
		if os.IsNotExist(err) {
			t.Errorf("expected dir %s to exist", d)
		} else if err == nil && !info.IsDir() {
			t.Errorf("expected %s to be a directory", d)
		}
	}
}

func TestRun_Idempotent(t *testing.T) {
	root := t.TempDir()

	// First run
	r1, err := Run(root, false)
	if err != nil {
		t.Fatalf("first Run() error: %v", err)
	}
	if len(r1.Created) == 0 {
		t.Fatal("first run created nothing")
	}

	// Capture file content for comparison
	glossaryBefore, _ := os.ReadFile(filepath.Join(root, "GLOSSARY.md"))

	// Second run
	r2, err := Run(root, false)
	if err != nil {
		t.Fatalf("second Run() error: %v", err)
	}
	if len(r2.Created) != 0 {
		t.Errorf("second run created %d items, expected 0: %v", len(r2.Created), r2.Created)
	}
	if len(r2.Skipped) != len(r1.Created) {
		t.Errorf("second run skipped %d items, expected %d", len(r2.Skipped), len(r1.Created))
	}

	// Verify content unchanged
	glossaryAfter, _ := os.ReadFile(filepath.Join(root, "GLOSSARY.md"))
	if string(glossaryBefore) != string(glossaryAfter) {
		t.Error("GLOSSARY.md content changed on second run")
	}
}

func TestRun_DryRun(t *testing.T) {
	root := t.TempDir()

	result, err := Run(root, true)
	if err != nil {
		t.Fatalf("Run(dryRun=true) error: %v", err)
	}

	if len(result.Created) == 0 {
		t.Fatal("dry run reported nothing to create")
	}

	// Verify nothing was written
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Errorf("dry run wrote %d items to disk, expected 0", len(entries))
	}
}

func TestRun_PartialExists(t *testing.T) {
	root := t.TempDir()

	// Pre-create some files
	os.MkdirAll(filepath.Join(root, "docs/core"), 0755)
	os.WriteFile(filepath.Join(root, "GLOSSARY.md"), []byte("# Custom Glossary\n"), 0644)

	result, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify pre-existing items were skipped
	skipped := make(map[string]bool)
	for _, s := range result.Skipped {
		skipped[s] = true
	}
	if !skipped["docs/core/"] {
		t.Error("expected docs/core/ to be skipped")
	}
	if !skipped["GLOSSARY.md"] {
		t.Error("expected GLOSSARY.md to be skipped")
	}

	// Verify pre-existing file was not overwritten
	content, _ := os.ReadFile(filepath.Join(root, "GLOSSARY.md"))
	if string(content) != "# Custom Glossary\n" {
		t.Error("GLOSSARY.md was overwritten")
	}

	// Verify other items were created
	created := make(map[string]bool)
	for _, c := range result.Created {
		created[c] = true
	}
	if !created["CLAUDE.md"] {
		t.Error("expected CLAUDE.md to be created")
	}
}

func TestRun_StateFileContent(t *testing.T) {
	root := t.TempDir()

	_, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".mindspec/state.json"))
	if err != nil {
		t.Fatalf("reading state.json: %v", err)
	}

	content := string(data)
	if !contains(content, `"mode": "idle"`) {
		t.Error("state.json should contain mode=idle")
	}
	if !contains(content, `"activeSpec": ""`) {
		t.Error("state.json should contain empty activeSpec")
	}
}

func TestRun_DomainTemplateSubstitution(t *testing.T) {
	root := t.TempDir()

	_, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify domain name was substituted in scaffolded files
	data, err := os.ReadFile(filepath.Join(root, "docs/domains/context-system/overview.md"))
	if err != nil {
		t.Fatalf("reading domain overview: %v", err)
	}
	if !contains(string(data), "Context-System") {
		t.Error("domain overview should contain 'Context-System' display name")
	}
	if contains(string(data), "{{.DomainName}}") {
		t.Error("domain overview should not contain unreplaced template placeholder")
	}

	// Verify template files keep the placeholder
	tmpl, _ := os.ReadFile(filepath.Join(root, "docs/templates/domain/overview.md"))
	if !contains(string(tmpl), "{{.DomainName}}") {
		t.Error("template file should keep {{.DomainName}} placeholder")
	}
}

func TestFormatSummary(t *testing.T) {
	r := &Result{
		Created: []string{"GLOSSARY.md", "docs/core/"},
		Skipped: []string{"CLAUDE.md"},
		BeadsOK: false,
	}

	summary := r.FormatSummary()
	if !contains(summary, "+ GLOSSARY.md") {
		t.Error("summary should list created items with +")
	}
	if !contains(summary, "- CLAUDE.md") {
		t.Error("summary should list skipped items with -")
	}
	if !contains(summary, "not found in PATH") {
		t.Error("summary should include Beads advisory")
	}
}

func TestFormatSummary_BeadsOK(t *testing.T) {
	r := &Result{
		Created: []string{"GLOSSARY.md"},
		BeadsOK: true,
	}

	summary := r.FormatSummary()
	if contains(summary, "not found in PATH") {
		t.Error("summary should not include Beads advisory when BeadsOK=true")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
