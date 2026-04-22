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
		".github/copilot-instructions.md",
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

func TestRun_CopilotInstructionsCreated(t *testing.T) {
	root := t.TempDir()

	_, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".github/copilot-instructions.md"))
	if err != nil {
		t.Fatalf("reading copilot-instructions.md: %v", err)
	}

	content := string(data)
	if !contains(content, "AGENTS.md") {
		t.Error("copilot-instructions.md should reference AGENTS.md")
	}
	if !contains(content, "mindspec instruct") {
		t.Error("copilot-instructions.md should reference mindspec instruct")
	}
	if !contains(content, mindspecMarkerBegin) {
		t.Error("copilot-instructions.md should contain BEGIN marker")
	}
	if !contains(content, mindspecMarkerEnd) {
		t.Error("copilot-instructions.md should contain END marker")
	}
}

func TestRun_CopilotInstructionsAppend(t *testing.T) {
	root := t.TempDir()

	// Pre-create .github/copilot-instructions.md without marker
	os.MkdirAll(filepath.Join(root, ".github"), 0755)
	os.WriteFile(filepath.Join(root, ".github/copilot-instructions.md"),
		[]byte("# Existing Copilot instructions\n\nCustom content here.\n"), 0644)

	result, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Should be in appended list
	appended := false
	for _, a := range result.Appended {
		if a == ".github/copilot-instructions.md" {
			appended = true
			break
		}
	}
	if !appended {
		t.Error("expected .github/copilot-instructions.md in Appended list")
	}

	data, _ := os.ReadFile(filepath.Join(root, ".github/copilot-instructions.md"))
	content := string(data)
	if !contains(content, "Existing Copilot instructions") {
		t.Error("original content should be preserved")
	}
	if !contains(content, mindspecMarkerBegin) {
		t.Error("appended block should contain BEGIN marker")
	}
	if !contains(content, mindspecMarkerEnd) {
		t.Error("appended block should contain END marker")
	}
	if !contains(content, "AGENTS.md") {
		t.Error("appended block should reference AGENTS.md")
	}
}

func TestRun_CopilotInstructionsIdempotent(t *testing.T) {
	root := t.TempDir()

	// Pre-create with marker already present
	os.MkdirAll(filepath.Join(root, ".github"), 0755)
	os.WriteFile(filepath.Join(root, ".github/copilot-instructions.md"),
		[]byte("# Custom\n"+mindspecMarkerBegin+"\nMindSpec block\n"+mindspecMarkerEnd+"\n"), 0644)

	result, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Should be in skipped list
	skipped := false
	for _, s := range result.Skipped {
		if contains(s, "copilot-instructions.md") {
			skipped = true
			break
		}
	}
	if !skipped {
		t.Error("expected copilot-instructions.md to be skipped when marker present")
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

func TestFormatSummary_MentionsCopilotSetup(t *testing.T) {
	r := &Result{
		Created: []string{"AGENTS.md"},
		BeadsOK: true,
	}

	summary := r.FormatSummary()
	if !contains(summary, "mindspec setup copilot") {
		t.Error("summary should mention 'mindspec setup copilot'")
	}
	if !contains(summary, "mindspec setup claude") {
		t.Error("summary should mention 'mindspec setup claude'")
	}
}

func TestRun_NoBeadsDir(t *testing.T) {
	root := t.TempDir()

	r, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// No .beads/ dir present → EnsureBeadsConfig is skipped.
	if r.BeadsConfig != nil {
		t.Errorf("expected BeadsConfig=nil when no .beads/ dir exists, got %+v", r.BeadsConfig)
	}
	if r.BeadsConfErr != "" {
		t.Errorf("expected no BeadsConfErr, got: %s", r.BeadsConfErr)
	}
}

func TestRun_PatchesExistingBeadsConfig(t *testing.T) {
	root := t.TempDir()

	// Simulate a project that ran `bd init` before `mindspec init`: .beads/
	// exists with a minimal user-authored config.
	beadsDir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	existing := "issue-prefix: \"myproj\"\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if r.BeadsConfig == nil {
		t.Fatalf("expected BeadsConfig to be populated when .beads/ exists")
	}
	if r.BeadsConfErr != "" {
		t.Fatalf("unexpected BeadsConfErr: %s", r.BeadsConfErr)
	}

	// mindspec-required keys should have been added.
	added := map[string]bool{}
	for _, k := range r.BeadsConfig.Added {
		added[k] = true
	}
	wantKeys := []string{"types.custom", "status.custom", "export.git-add"}
	for _, k := range wantKeys {
		if !added[k] {
			t.Errorf("expected %s in Added, got %v", k, r.BeadsConfig.Added)
		}
	}

	// Existing user-authored prefix must be preserved.
	data, err := os.ReadFile(filepath.Join(beadsDir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), `issue-prefix: "myproj"`) && !contains(string(data), "issue-prefix: myproj") {
		t.Errorf("user-authored issue-prefix not preserved; got:\n%s", string(data))
	}
}

func TestRun_BeadsConfigIdempotent(t *testing.T) {
	root := t.TempDir()

	// Seed with an already-mindspec-ready config so the first Run picks it up
	// via EnsureBeadsConfig as AlreadyCorrect.
	beadsDir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	ready := `issue-prefix: "proj"
types.custom: "gate"
status.custom: "resolved"
export.git-add: false
`
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(ready), 0o644); err != nil {
		t.Fatal(err)
	}

	before, _ := os.ReadFile(filepath.Join(beadsDir, "config.yaml"))

	if _, err := Run(root, false); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	r2, err := Run(root, false)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}

	// Nothing added, nothing drifted — already-correct is the expected state.
	if r2.BeadsConfig == nil {
		t.Fatal("expected BeadsConfig on second run")
	}
	if len(r2.BeadsConfig.Added) != 0 {
		t.Errorf("second run added keys: %v (want none)", r2.BeadsConfig.Added)
	}
	if len(r2.BeadsConfig.UserAuthored) != 0 {
		t.Errorf("second run reported drift: %+v", r2.BeadsConfig.UserAuthored)
	}

	after, _ := os.ReadFile(filepath.Join(beadsDir, "config.yaml"))
	if string(before) != string(after) {
		t.Errorf("config.yaml changed on idempotent run:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestRun_DryRunDoesNotPatchBeadsConfig(t *testing.T) {
	root := t.TempDir()

	// .beads/ exists but dry-run must not touch config.yaml.
	beadsDir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(beadsDir, "config.yaml")
	existing := "issue-prefix: \"myproj\"\n"
	if err := os.WriteFile(cfgPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Run(root, true)
	if err != nil {
		t.Fatalf("Run(dryRun=true) error: %v", err)
	}

	if r.BeadsConfig != nil {
		t.Errorf("dry-run should not call EnsureBeadsConfig, got BeadsConfig=%+v", r.BeadsConfig)
	}
	data, _ := os.ReadFile(cfgPath)
	if string(data) != existing {
		t.Errorf("dry-run modified config.yaml:\nwant: %q\ngot:  %q", existing, string(data))
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
