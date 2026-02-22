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
		"CLAUDE.md",
		".mindspec/state.json",
	}
	for _, f := range requiredFiles {
		p := filepath.Join(root, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Verify key dirs exist
	requiredDirs := []string{
		".mindspec/docs/domains",
		".mindspec/docs/specs",
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

	// Verify removed items are NOT created
	removedFiles := []string{
		"GLOSSARY.md",
		".mindspec/docs/context-map.md",
		".mindspec/policies.yml",
	}
	for _, f := range removedFiles {
		p := filepath.Join(root, f)
		if _, err := os.Stat(p); err == nil {
			t.Errorf("expected file %s to NOT exist (removed from bootstrap)", f)
		}
	}
	removedDirs := []string{
		".mindspec/docs/core",
		".mindspec/docs/adr",
	}
	for _, d := range removedDirs {
		p := filepath.Join(root, d)
		if _, err := os.Stat(p); err == nil {
			t.Errorf("expected dir %s to NOT exist (removed from bootstrap)", d)
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
	claudeBefore, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))

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
	claudeAfter, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(claudeBefore) != string(claudeAfter) {
		t.Error("CLAUDE.md content changed on second run")
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
	os.MkdirAll(filepath.Join(root, ".mindspec/docs/domains"), 0755)
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# Custom CLAUDE\n"), 0644)

	result, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify pre-existing items were skipped
	skipped := make(map[string]bool)
	for _, s := range result.Skipped {
		skipped[s] = true
	}
	if !skipped[".mindspec/docs/domains/"] {
		t.Error("expected .mindspec/docs/domains/ to be skipped")
	}

	// Verify pre-existing file was not overwritten
	content, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(content) == "" {
		t.Error("CLAUDE.md was emptied")
	}

	// Verify other items were created
	created := make(map[string]bool)
	for _, c := range result.Created {
		created[c] = true
	}
	if !created["AGENTS.md"] {
		t.Error("expected AGENTS.md to be created")
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

func TestRun_NoDomainScaffolding(t *testing.T) {
	root := t.TempDir()

	_, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Domains dir should exist but be empty — no default domains are scaffolded
	entries, err := os.ReadDir(filepath.Join(root, ".mindspec/docs/domains"))
	if err != nil {
		t.Fatalf("reading domains dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty domains dir, got %d entries", len(entries))
	}
}

func TestFormatSummary(t *testing.T) {
	r := &Result{
		Created: []string{"AGENTS.md", ".mindspec/docs/domains/"},
		Skipped: []string{"CLAUDE.md"},
		BeadsOK: false,
	}

	summary := r.FormatSummary()
	if !contains(summary, "+ AGENTS.md") {
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
		Created: []string{"AGENTS.md"},
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
