package approve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestUpdatePlanApproval_UpdatesFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	planContent := `---
status: Draft
spec_id: test
version: "0.1"
work_chunks:
  - id: 1
    title: "test"
    scope: "test.go"
    verify: []
    depends_on: []
---

# Plan body

Details here.
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	err := updatePlanApproval(planPath, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(planPath)
	content := string(data)

	// Should have approved status
	if !strings.Contains(content, "status: Approved") {
		t.Error("expected status: Approved in frontmatter")
	}

	// Should have approved_at
	if !strings.Contains(content, "approved_at:") {
		t.Error("expected approved_at in frontmatter")
	}

	// Should have approved_by
	if !strings.Contains(content, "approved_by: user") {
		t.Error("expected approved_by: user in frontmatter")
	}

	// Body preserved
	if !strings.Contains(content, "# Plan body") {
		t.Error("plan body lost")
	}
	if !strings.Contains(content, "Details here.") {
		t.Error("plan details lost")
	}

	// Original fields preserved
	if !strings.Contains(content, "spec_id: test") {
		t.Error("spec_id field lost")
	}
}

func TestUpdatePlanApproval_NoFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte("# No frontmatter\n"), 0644)

	err := updatePlanApproval(planPath, "user")
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestUpdatePlanApproval_PreservesGeneratedBlock(t *testing.T) {
	tmp := t.TempDir()
	planContent := `---
status: Draft
spec_id: test
version: "0.1"
generated:
  mol_parent_id: mol-123
  bead_ids:
    "1": bead-abc
work_chunks:
  - id: 1
    title: "test"
    scope: "test.go"
    verify: []
    depends_on: []
---

# Plan
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	err := updatePlanApproval(planPath, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(planPath)
	content := string(data)

	// Generated block should be preserved
	if !strings.Contains(content, "mol_parent_id") {
		t.Error("generated.mol_parent_id lost")
	}
	if !strings.Contains(content, "bead-abc") {
		t.Error("generated.bead_ids lost")
	}

	// Approval fields set
	if !strings.Contains(content, "status: Approved") {
		t.Error("expected status: Approved")
	}
}

func TestCreateImplementationBeads_CreatesAndWiresDeps(t *testing.T) {
	tmp := t.TempDir()
	planContent := `---
status: Approved
spec_id: "042-test"
version: "1.0"
---

# Plan

## Bead 1: First thing

**Steps**
1. Step one
2. Step two
3. Step three

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Depends on**
None

## Bead 2: Second thing

**Steps**
1. Step one
2. Step two
3. Step three

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Depends on**
Bead 1

## Bead 3: Third thing

**Steps**
1. Step one
2. Step two
3. Step three

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Depends on**
Bead 1, Bead 2
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	// Track bd calls
	var callCount atomic.Int32
	var depCalls []string

	orig := planRunBDFn
	defer func() { planRunBDFn = orig }()
	planRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			n := callCount.Add(1)
			id := fmt.Sprintf("test-bead-%d", n)
			return []byte(fmt.Sprintf(`{"id":"%s"}`, id)), nil
		}
		if len(args) > 0 && args[0] == "dep" {
			depCalls = append(depCalls, strings.Join(args, " "))
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected bd call: %v", args)
	}

	beadIDs, err := createImplementationBeads(planPath, "042-test", "parent-mol-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should create 3 beads
	if len(beadIDs) != 3 {
		t.Fatalf("expected 3 bead IDs, got %d: %v", len(beadIDs), beadIDs)
	}
	if beadIDs[0] != "test-bead-1" || beadIDs[1] != "test-bead-2" || beadIDs[2] != "test-bead-3" {
		t.Errorf("unexpected bead IDs: %v", beadIDs)
	}

	// Bead 2 depends on Bead 1 → dep add test-bead-2 test-bead-1
	// Bead 3 depends on Bead 1 and Bead 2 → dep add test-bead-3 test-bead-1, dep add test-bead-3 test-bead-2
	if len(depCalls) != 3 {
		t.Fatalf("expected 3 dep calls, got %d: %v", len(depCalls), depCalls)
	}
	if depCalls[0] != "dep add test-bead-2 test-bead-1" {
		t.Errorf("dep call 0: expected 'dep add test-bead-2 test-bead-1', got %q", depCalls[0])
	}
	if depCalls[1] != "dep add test-bead-3 test-bead-1" {
		t.Errorf("dep call 1: expected 'dep add test-bead-3 test-bead-1', got %q", depCalls[1])
	}
	if depCalls[2] != "dep add test-bead-3 test-bead-2" {
		t.Errorf("dep call 2: expected 'dep add test-bead-3 test-bead-2', got %q", depCalls[2])
	}
}

func TestCreateImplementationBeads_NoSections(t *testing.T) {
	tmp := t.TempDir()
	planContent := `---
status: Approved
spec_id: "042-test"
version: "1.0"
---

# Plan with no bead sections
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	beadIDs, err := createImplementationBeads(planPath, "042-test", "parent-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(beadIDs) != 0 {
		t.Errorf("expected 0 bead IDs for plan with no bead sections, got %d", len(beadIDs))
	}
}

func TestCreateImplementationBeads_TitleIncludesSpecID(t *testing.T) {
	tmp := t.TempDir()
	planContent := `---
status: Approved
spec_id: "042-test"
version: "1.0"
---

# Plan

## Bead 1: Widget factory

**Steps**
1. Step one
2. Step two
3. Step three

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Depends on**
None
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	var capturedTitle string
	orig := planRunBDFn
	defer func() { planRunBDFn = orig }()
	planRunBDFn = func(args ...string) ([]byte, error) {
		for i, a := range args {
			if a == "--title" && i+1 < len(args) {
				capturedTitle = args[i+1]
			}
		}
		return []byte(`{"id":"test-1"}`), nil
	}

	_, err := createImplementationBeads(planPath, "042-test", "parent-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedTitle != "[042-test] Bead 1: Widget factory" {
		t.Errorf("expected title '[042-test] Bead 1: Widget factory', got %q", capturedTitle)
	}
}

func TestCreateImplementationBeads_BDCreateFails(t *testing.T) {
	tmp := t.TempDir()
	planContent := `---
status: Approved
spec_id: "042-test"
version: "1.0"
---

# Plan

## Bead 1: Widget factory

**Steps**
1. Step one

**Verification**
- [ ] Tests pass

**Depends on**
None
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	orig := planRunBDFn
	defer func() { planRunBDFn = orig }()
	planRunBDFn = func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("bd not available")
	}

	beadIDs, err := createImplementationBeads(planPath, "042-test", "parent-123")
	if err == nil {
		t.Fatal("expected error when bd create fails")
	}
	if len(beadIDs) != 0 {
		t.Errorf("expected 0 bead IDs on error, got %d", len(beadIDs))
	}
}

func TestWriteBeadIDsToFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	planContent := `---
status: Approved
spec_id: test
version: "1.0"
---

# Plan body
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	err := writeBeadIDsToFrontmatter(planPath, []string{"bead-aaa", "bead-bbb", "bead-ccc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(planPath)
	content := string(data)

	if !strings.Contains(content, "bead-aaa") {
		t.Error("expected bead-aaa in frontmatter")
	}
	if !strings.Contains(content, "bead-bbb") {
		t.Error("expected bead-bbb in frontmatter")
	}
	if !strings.Contains(content, "bead-ccc") {
		t.Error("expected bead-ccc in frontmatter")
	}
	if !strings.Contains(content, "bead_ids") {
		t.Error("expected bead_ids key in frontmatter")
	}
	// Body preserved
	if !strings.Contains(content, "# Plan body") {
		t.Error("plan body lost")
	}
}
