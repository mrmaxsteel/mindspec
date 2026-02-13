package bead

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- ParsePlanMeta tests ---

func TestParsePlanMeta_WorkChunks(t *testing.T) {
	tmp := t.TempDir()
	planContent := `---
status: Approved
spec_id: 007-beads-tooling
work_chunks:
  - id: 1
    title: "bdcli wrapper"
    scope: "internal/bead/bdcli.go"
    verify:
      - "tests pass"
    depends_on: []
  - id: 2
    title: "spec bead"
    scope: "internal/bead/spec.go"
    verify:
      - "creates bead"
    depends_on: [1]
---

# Plan
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	meta, err := ParsePlanMeta(planPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.Status != "Approved" {
		t.Errorf("expected status Approved, got %q", meta.Status)
	}
	if len(meta.WorkChunks) != 2 {
		t.Fatalf("expected 2 work chunks, got %d", len(meta.WorkChunks))
	}
	if meta.WorkChunks[0].Title != "bdcli wrapper" {
		t.Errorf("expected first chunk title 'bdcli wrapper', got %q", meta.WorkChunks[0].Title)
	}
	if len(meta.WorkChunks[1].DependsOn) != 1 || meta.WorkChunks[1].DependsOn[0] != 1 {
		t.Errorf("expected chunk 2 depends_on [1], got %v", meta.WorkChunks[1].DependsOn)
	}
}

func TestParsePlanMeta_CommentedLines(t *testing.T) {
	tmp := t.TempDir()
	planContent := `---
status: Approved
spec_id: test
# This is a comment
# approved_at: not-yet
work_chunks:
  - id: 1
    title: "test"
    scope: "test.go"
    verify: []
    depends_on: []
---

body
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	meta, err := ParsePlanMeta(planPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Status != "Approved" {
		t.Errorf("expected status Approved, got %q", meta.Status)
	}
}

func TestParsePlanMeta_NoFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte("# Plan\nNo frontmatter here\n"), 0644)

	_, err := ParsePlanMeta(planPath)
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

// --- CreatePlanBeads tests ---

func TestCreatePlanBeads_UnapprovedPlan(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "test")
	os.MkdirAll(specDir, 0755)
	planContent := `---
status: Draft
spec_id: test
work_chunks:
  - id: 1
    title: "test"
    scope: "test.go"
    verify: []
    depends_on: []
---

# Plan
`
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(planContent), 0644)

	_, err := CreatePlanBeads(tmp, "test")
	if err == nil {
		t.Fatal("expected error for unapproved plan")
	}
	if !contains(err.Error(), "not approved") {
		t.Errorf("error should mention not approved: %v", err)
	}
}

func TestCreatePlanBeads_MissingWorkChunks(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "test")
	os.MkdirAll(specDir, 0755)
	planContent := `---
status: Approved
spec_id: test
---

# Plan
`
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(planContent), 0644)

	_, err := CreatePlanBeads(tmp, "test")
	if err == nil {
		t.Fatal("expected error for missing work_chunks")
	}
	if !contains(err.Error(), "no work_chunks") {
		t.Errorf("error should mention work_chunks: %v", err)
	}
}

func TestCreatePlanBeads_CreatesMoleculeParent(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "test")
	os.MkdirAll(specDir, 0755)
	planContent := `---
status: Approved
spec_id: test
work_chunks:
  - id: 1
    title: "first chunk"
    scope: "first.go"
    verify:
      - "test passes"
    depends_on: []
  - id: 2
    title: "second chunk"
    scope: "second.go"
    verify:
      - "test passes"
    depends_on: [1]
---

# Plan
`
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(planContent), 0644)

	origExec := execCommand
	defer func() { execCommand = origExec }()

	var createCalls [][]string
	var depAddCalls [][]string

	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "bd" && len(args) > 0 {
			switch args[0] {
			case "search":
				// Gate searches and bead searches all return empty
				return exec.Command("echo", `[]`)
			case "create":
				createCalls = append(createCalls, args)
				title := args[1]
				var id string
				if strings.HasPrefix(title, "[PLAN") {
					id = "mol-parent-001"
				} else if strings.HasPrefix(title, "[GATE") {
					id = "plan-gate-001"
				} else {
					id = "bead-" + strings.Replace(title, " ", "", -1)[:10]
				}
				return exec.Command("echo", `{"id":"`+id+`","title":"`+title+`","description":"","status":"open","priority":2,"issue_type":"epic","owner":"","created_at":"","updated_at":""}`)
			case "dep":
				if len(args) >= 4 {
					depAddCalls = append(depAddCalls, args[1:])
				}
				return exec.Command("echo", "")
			case "gate":
				return exec.Command("echo", "")
			}
		}
		return exec.Command("echo", "")
	}

	result, err := CreatePlanBeads(tmp, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have molecule parent ID
	if result.MolParentID != "mol-parent-001" {
		t.Errorf("MolParentID: got %q, want %q", result.MolParentID, "mol-parent-001")
	}

	// Should have plan gate ID
	if result.PlanGateID != "plan-gate-001" {
		t.Errorf("PlanGateID: got %q, want %q", result.PlanGateID, "plan-gate-001")
	}

	// Should have 2 chunk beads
	if len(result.ChunkBeads) != 2 {
		t.Errorf("expected 2 chunk beads, got %d", len(result.ChunkBeads))
	}

	// Creates: [PLAN test] (epic), [GATE plan-approve test] (gate), [IMPL test.1], [IMPL test.2] = 4 creates
	if len(createCalls) != 4 {
		t.Fatalf("expected 4 create calls (1 epic + 1 gate + 2 tasks), got %d", len(createCalls))
	}

	// First create should be the molecule parent with type=epic
	molCreateArgs := createCalls[0]
	if !strings.HasPrefix(molCreateArgs[1], "[PLAN test]") {
		t.Errorf("first create should be [PLAN test], got title %q", molCreateArgs[1])
	}
	hasEpicType := false
	for _, arg := range molCreateArgs {
		if arg == "--type=epic" {
			hasEpicType = true
		}
	}
	if !hasEpicType {
		t.Error("molecule parent should be type=epic")
	}

	// At least one dep add call (chunk 2->chunk 1, plus gate deps)
	if len(depAddCalls) == 0 {
		t.Error("expected at least one dep add call")
	}
}

func TestCreatePlanBeads_IdempotentMolParent(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "test")
	os.MkdirAll(specDir, 0755)
	planContent := `---
status: Approved
spec_id: test
work_chunks:
  - id: 1
    title: "chunk one"
    scope: "one.go"
    verify: []
    depends_on: []
---

# Plan
`
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(planContent), 0644)

	origExec := execCommand
	defer func() { execCommand = origExec }()

	createCount := 0

	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "bd" && len(args) > 0 {
			switch args[0] {
			case "search":
				query := args[1]
				// [GATE spec-approve test] is resolved (closed)
				if strings.HasPrefix(query, "[GATE spec-approve") {
					return exec.Command("echo", `[{"id":"resolved-spec-gate","title":"[GATE spec-approve test]","description":"","status":"closed","priority":0,"issue_type":"gate","owner":"","created_at":"","updated_at":""}]`)
				}
				// [PLAN test] already exists
				if strings.HasPrefix(query, "[PLAN") {
					return exec.Command("echo", `[{"id":"existing-mol","title":"[PLAN test]","description":"","status":"open","priority":2,"issue_type":"epic","owner":"","created_at":"","updated_at":""}]`)
				}
				// [IMPL test.1] already exists
				if strings.HasPrefix(query, "[IMPL") {
					return exec.Command("echo", `[{"id":"existing-child","title":"[IMPL test.1]","description":"","status":"open","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":""}]`)
				}
				// [GATE plan-approve test] already exists (open)
				if strings.HasPrefix(query, "[GATE") {
					return exec.Command("echo", `[{"id":"existing-gate","title":"[GATE plan-approve test]","description":"","status":"open","priority":0,"issue_type":"gate","owner":"","created_at":"","updated_at":""}]`)
				}
				return exec.Command("echo", `[]`)
			case "create":
				createCount++
				return exec.Command("echo", `{"id":"new-bead","title":"","description":"","status":"open","priority":2,"issue_type":"task","owner":"","created_at":"","updated_at":""}`)
			case "dep":
				return exec.Command("echo", "")
			}
		}
		return exec.Command("echo", "")
	}

	result, err := CreatePlanBeads(tmp, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should reuse existing molecule parent
	if result.MolParentID != "existing-mol" {
		t.Errorf("MolParentID: got %q, want %q", result.MolParentID, "existing-mol")
	}

	// Should reuse existing gate
	if result.PlanGateID != "existing-gate" {
		t.Errorf("PlanGateID: got %q, want %q", result.PlanGateID, "existing-gate")
	}

	// Should reuse existing child
	if result.ChunkBeads[1] != "existing-child" {
		t.Errorf("ChunkBeads[1]: got %q, want %q", result.ChunkBeads[1], "existing-child")
	}

	// No new beads should have been created (all existing)
	if createCount != 0 {
		t.Errorf("expected 0 create calls (all existing), got %d", createCount)
	}
}

func TestCreatePlanBeads_RefusesUnresolvedSpecGate(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "test")
	os.MkdirAll(specDir, 0755)
	planContent := `---
status: Approved
spec_id: test
work_chunks:
  - id: 1
    title: "chunk"
    scope: "test.go"
    verify: []
    depends_on: []
---

# Plan
`
	os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(planContent), 0644)

	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "bd" && len(args) > 0 && args[0] == "search" {
			query := args[1]
			// Spec gate exists and is OPEN (not resolved)
			if strings.HasPrefix(query, "[GATE spec-approve") {
				return exec.Command("echo", `[{"id":"open-gate","title":"[GATE spec-approve test]","description":"","status":"open","priority":0,"issue_type":"gate","owner":"","created_at":"","updated_at":""}]`)
			}
			return exec.Command("echo", `[]`)
		}
		return exec.Command("echo", "")
	}

	_, err := CreatePlanBeads(tmp, "test")
	if err == nil {
		t.Fatal("expected error for unresolved spec gate")
	}
	if !contains(err.Error(), "spec gate is not resolved") {
		t.Errorf("error should mention spec gate: %v", err)
	}
}

// --- buildImplDescription tests ---

func TestBuildImplDescription_Format(t *testing.T) {
	chunk := WorkChunk{
		ID:    1,
		Title: "test chunk",
		Scope: "internal/bead/bdcli.go",
		Verify: []string{
			"tests pass",
			"preflight works",
		},
	}

	desc := buildImplDescription(chunk, "007-beads-tooling")
	if !contains(desc, "Scope: internal/bead/bdcli.go") {
		t.Errorf("missing Scope line: %s", desc)
	}
	if !contains(desc, "Verify:") {
		t.Errorf("missing Verify section: %s", desc)
	}
	if !contains(desc, "- tests pass") {
		t.Errorf("missing verify item: %s", desc)
	}
	if !contains(desc, "Plan: docs/specs/007-beads-tooling/plan.md") {
		t.Errorf("missing Plan line: %s", desc)
	}
}

func TestBuildImplDescription_Cap(t *testing.T) {
	chunk := WorkChunk{
		Scope:  strings.Repeat("x", 900),
		Verify: []string{"test"},
	}
	desc := buildImplDescription(chunk, "test")
	if len(desc) > 800 {
		t.Errorf("description exceeds 800 char cap: %d chars", len(desc))
	}
}

// --- WriteGeneratedBeadIDs tests ---

func TestWriteGeneratedBeadIDs_PreservesFields(t *testing.T) {
	tmp := t.TempDir()
	planContent := `---
status: Approved
spec_id: test
version: "1.0"
work_chunks:
  - id: 1
    title: "chunk one"
    scope: "test.go"
    verify: []
    depends_on: []
---

# Plan body content

This should be preserved.
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	result := &PlanBeadResult{
		MolParentID: "mol-parent-xyz",
		ChunkBeads: map[int]string{
			1: "bead-abc",
		},
	}

	err := WriteGeneratedBeadIDs(planPath, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read back
	data, _ := os.ReadFile(planPath)
	content := string(data)

	// Body should be preserved
	if !contains(content, "# Plan body content") {
		t.Error("body content was lost")
	}
	if !contains(content, "This should be preserved.") {
		t.Error("body detail was lost")
	}

	// Frontmatter should still have original fields
	if !contains(content, "status: Approved") {
		t.Error("status field was lost")
	}
	if !contains(content, "spec_id: test") {
		t.Error("spec_id field was lost")
	}

	// Should have generated.bead_ids
	if !contains(content, "bead_ids") {
		t.Error("generated bead_ids not found")
	}
	if !contains(content, "bead-abc") {
		t.Error("bead ID value not found")
	}

	// Should have generated.mol_parent_id
	if !contains(content, "mol_parent_id") {
		t.Error("generated mol_parent_id not found")
	}
	if !contains(content, "mol-parent-xyz") {
		t.Error("mol parent ID value not found")
	}
}

func TestWriteGeneratedBeadIDs_NoFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte("# No frontmatter\n"), 0644)

	result := &PlanBeadResult{
		MolParentID: "mol-x",
		ChunkBeads:  map[int]string{1: "bead-x"},
	}
	err := WriteGeneratedBeadIDs(planPath, result)
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}
