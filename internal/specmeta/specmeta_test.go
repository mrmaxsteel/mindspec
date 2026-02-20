package specmeta

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractFrontmatter_WithFrontmatter(t *testing.T) {
	content := "---\nmolecule_id: mol-123\n---\n# Spec 001: Test\n\n## Goal\n"
	fm, body := extractFrontmatter(content)

	if fm != "molecule_id: mol-123" {
		t.Errorf("expected frontmatter 'molecule_id: mol-123', got %q", fm)
	}
	if !strings.Contains(body, "# Spec 001: Test") {
		t.Errorf("expected body to contain heading, got %q", body)
	}
}

func TestExtractFrontmatter_NoFrontmatter(t *testing.T) {
	content := "# Spec 001: Test\n\n## Goal\n"
	fm, body := extractFrontmatter(content)

	if fm != "" {
		t.Errorf("expected empty frontmatter, got %q", fm)
	}
	if body != content {
		t.Errorf("expected body to be full content")
	}
}

func TestExtractFrontmatter_EmptyFrontmatter(t *testing.T) {
	content := "---\n---\n# Spec\n"
	fm, body := extractFrontmatter(content)

	if fm != "" {
		t.Errorf("expected empty frontmatter string, got %q", fm)
	}
	if !strings.Contains(body, "# Spec") {
		t.Errorf("expected body to contain heading, got %q", body)
	}
}

func TestReadWrite_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")

	original := "# Spec 001: Test\n\n## Goal\n\nDo something.\n"
	os.WriteFile(specPath, []byte(original), 0644)

	// Write molecule binding
	m := &Meta{
		MoleculeID: "mol-abc",
		StepMapping: map[string]string{
			"spec":         "step-1",
			"spec-approve": "step-2",
		},
	}
	if err := Write(dir, m); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Read back
	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if got.MoleculeID != "mol-abc" {
		t.Errorf("MoleculeID = %q, want %q", got.MoleculeID, "mol-abc")
	}
	if got.StepMapping["spec"] != "step-1" {
		t.Errorf("StepMapping[spec] = %q, want %q", got.StepMapping["spec"], "step-1")
	}
	if got.StepMapping["spec-approve"] != "step-2" {
		t.Errorf("StepMapping[spec-approve] = %q, want %q", got.StepMapping["spec-approve"], "step-2")
	}

	// Verify the file still contains the original heading
	data, _ := os.ReadFile(specPath)
	content := string(data)
	if !strings.Contains(content, "# Spec 001: Test") {
		t.Errorf("original heading lost after write, content:\n%s", content)
	}
	if !strings.Contains(content, "## Goal") {
		t.Errorf("Goal section lost after write, content:\n%s", content)
	}
}

func TestRead_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	os.WriteFile(specPath, []byte("# Spec 001\n\n## Goal\n"), 0644)

	m, err := Read(dir)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if m.MoleculeID != "" {
		t.Errorf("expected empty MoleculeID, got %q", m.MoleculeID)
	}
}

func TestRead_EmptyMoleculeID(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	content := "---\nmolecule_id: \"\"\n---\n# Spec 001\n"
	os.WriteFile(specPath, []byte(content), 0644)

	m, err := Read(dir)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if m.MoleculeID != "" {
		t.Errorf("expected empty MoleculeID, got %q", m.MoleculeID)
	}
}

func TestWrite_PreservesExistingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")

	// Start with existing frontmatter
	original := "---\ncustom_field: value\n---\n# Spec 001\n"
	os.WriteFile(specPath, []byte(original), 0644)

	m := &Meta{MoleculeID: "mol-xyz"}
	if err := Write(dir, m); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	data, _ := os.ReadFile(specPath)
	content := string(data)

	if !strings.Contains(content, "custom_field: value") {
		t.Errorf("existing frontmatter field lost, content:\n%s", content)
	}
	if !strings.Contains(content, "molecule_id: mol-xyz") {
		t.Errorf("molecule_id not written, content:\n%s", content)
	}
}

func TestWrite_UpdatesExistingMoleculeID(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")

	original := "---\nmolecule_id: old-id\n---\n# Spec 001\n"
	os.WriteFile(specPath, []byte(original), 0644)

	m := &Meta{MoleculeID: "new-id"}
	if err := Write(dir, m); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if got.MoleculeID != "new-id" {
		t.Errorf("MoleculeID = %q, want %q", got.MoleculeID, "new-id")
	}
}

func TestRead_MissingFile(t *testing.T) {
	_, err := Read("/nonexistent")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestWrite_WritesApprovalFields(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	os.WriteFile(specPath, []byte("# Spec 001\n"), 0644)

	meta := &Meta{
		Status:     "Approved",
		ApprovedAt: "2026-02-20T15:00:00Z",
		ApprovedBy: "user",
	}
	if err := Write(dir, meta); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("reading spec: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "status: Approved") {
		t.Error("expected status field in frontmatter")
	}
	if !strings.Contains(content, "approved_at: \"2026-02-20T15:00:00Z\"") {
		t.Error("expected approved_at field in frontmatter")
	}
	if !strings.Contains(content, "approved_by: user") {
		t.Error("expected approved_by field in frontmatter")
	}
}

func TestEnsureFullyBound_RecoversMissingStepMapping(t *testing.T) {
	root := t.TempDir()
	specID := "010-test"
	specDir := filepath.Join(root, "docs", "specs", specID)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	spec := `---
molecule_id: mol-abc
---
# Spec 010
`
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	origRunBD := runBDFn
	defer func() { runBDFn = origRunBD }()

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 4 && args[0] == "mol" && args[1] == "show" && args[2] == "mol-abc" {
			payload := map[string]any{
				"issues": []map[string]string{
					{"id": "step-spec", "title": "Write spec 010-test"},
					{"id": "step-spec-approve", "title": "Approve spec 010-test"},
					{"id": "step-plan", "title": "Write plan 010-test"},
					{"id": "step-plan-approve", "title": "Approve plan 010-test"},
					{"id": "step-implement", "title": "Implement 010-test"},
					{"id": "step-review", "title": "Review 010-test"},
				},
			}
			return json.Marshal(payload)
		}
		return nil, fmt.Errorf("unexpected bd args: %v", args)
	}

	meta, err := EnsureFullyBound(root, specID)
	if err != nil {
		t.Fatalf("EnsureFullyBound() error: %v", err)
	}
	if meta.MoleculeID != "mol-abc" {
		t.Errorf("MoleculeID: got %q, want %q", meta.MoleculeID, "mol-abc")
	}
	if meta.StepMapping["spec"] != "step-spec" {
		t.Errorf("spec step not recovered: %v", meta.StepMapping)
	}
	if meta.StepMapping["review"] != "step-review" {
		t.Errorf("review step not recovered: %v", meta.StepMapping)
	}
	if meta.StepMapping["spec-lifecycle"] != "mol-abc" {
		t.Errorf("spec-lifecycle mapping: got %q, want %q", meta.StepMapping["spec-lifecycle"], "mol-abc")
	}
}
