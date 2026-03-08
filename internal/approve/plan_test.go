package approve

import (
	"encoding/json"
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

func TestCreateImplementationBeads_PopulatesFields(t *testing.T) {
	tmp := t.TempDir()

	// Create spec.md with Requirements, Acceptance Criteria, and ADR Touchpoints
	specContent := `---
status: Approved
---
# Spec 042-test

## Goal
Test the bead population.

## Impacted Domains
- core

## ADR Touchpoints
None applicable.

## Requirements
1. Widget must frob
2. Widget must grob

## Scope
### In Scope
- internal/widget/

### Out of Scope
- external stuff

## Acceptance Criteria
- [ ] Widget frobs correctly
- [ ] Widget grobs correctly

## Approval
- **Status**: APPROVED
`
	os.WriteFile(filepath.Join(tmp, "spec.md"), []byte(specContent), 0644)

	planContent := `---
status: Approved
spec_id: "042-test"
version: "1.0"
---

# Plan

## Bead 1: Implement widget frobbing

**Steps**
1. Create ` + "`internal/widget/frob.go`" + `
2. Add frob logic

**Verification**
- [ ] ` + "`go test ./internal/widget/...`" + ` passes

**Depends on**
None
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	var capturedArgs [][]string
	orig := planRunBDFn
	defer func() { planRunBDFn = orig }()
	planRunBDFn = func(args ...string) ([]byte, error) {
		argsCopy := make([]string, len(args))
		copy(argsCopy, args)
		capturedArgs = append(capturedArgs, argsCopy)
		return []byte(`{"id":"test-bead-1"}`), nil
	}

	beadIDs, err := createImplementationBeads(planPath, "042-test", "parent-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(beadIDs) != 1 {
		t.Fatalf("expected 1 bead, got %d", len(beadIDs))
	}

	// Find the create call
	if len(capturedArgs) == 0 {
		t.Fatal("no bd calls captured")
	}
	createArgs := capturedArgs[0]

	// Helper to find flag value
	findFlag := func(flag string) string {
		for i, a := range createArgs {
			if a == flag && i+1 < len(createArgs) {
				return createArgs[i+1]
			}
		}
		return ""
	}

	// Verify --description contains the work chunk
	desc := findFlag("--description")
	if desc == "" {
		t.Error("--description flag not passed")
	} else {
		if !strings.Contains(desc, "internal/widget/frob.go") {
			t.Error("description should contain file path from work chunk")
		}
		if !strings.Contains(desc, "frob logic") {
			t.Error("description should contain step content")
		}
	}

	// Verify --acceptance from spec
	ac := findFlag("--acceptance")
	if ac == "" {
		t.Error("--acceptance flag not passed")
	} else {
		if !strings.Contains(ac, "Widget frobs correctly") {
			t.Error("acceptance criteria should contain spec AC")
		}
	}

	// Verify --design contains requirements
	design := findFlag("--design")
	if design == "" {
		t.Error("--design flag not passed")
	} else {
		if !strings.Contains(design, "Widget must frob") {
			t.Error("design should contain spec requirements")
		}
	}

	// Verify --metadata contains spec_id and file_paths
	meta := findFlag("--metadata")
	if meta == "" {
		t.Error("--metadata flag not passed")
	} else {
		if !strings.Contains(meta, `"spec_id":"042-test"`) {
			t.Errorf("metadata should contain spec_id, got: %s", meta)
		}
		if !strings.Contains(meta, "internal/widget/frob.go") {
			t.Errorf("metadata should contain file_paths, got: %s", meta)
		}
	}
}

func TestCreateImplementationBeads_PerBeadAcceptanceCriteria(t *testing.T) {
	tmp := t.TempDir()

	specContent := `---
status: Approved
---
# Spec 042-test

## Requirements
1. Widget must frob
2. Widget must grob

## Acceptance Criteria
- [ ] Widget frobs correctly
- [ ] Widget grobs correctly
`
	os.WriteFile(filepath.Join(tmp, "spec.md"), []byte(specContent), 0644)

	planContent := `---
status: Approved
spec_id: "042-test"
version: "1.0"
---

# Plan

## Bead 1: Implement frobbing

**Steps**
1. Create frob.go
2. Add frob logic
3. Add frob tests

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Acceptance Criteria**
- [ ] Widget frobs correctly

**Depends on**
None

## Bead 2: Implement grobbing

**Steps**
1. Create grob.go
2. Add grob logic
3. Add grob tests

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Acceptance Criteria**
- [ ] Widget grobs correctly

**Depends on**
Bead 1
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	var capturedArgs [][]string
	orig := planRunBDFn
	defer func() { planRunBDFn = orig }()
	planRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			argsCopy := make([]string, len(args))
			copy(argsCopy, args)
			capturedArgs = append(capturedArgs, argsCopy)
			id := fmt.Sprintf("test-bead-%d", len(capturedArgs))
			return []byte(fmt.Sprintf(`{"id":"%s"}`, id)), nil
		}
		return nil, nil // dep calls
	}

	beadIDs, err := createImplementationBeads(planPath, "042-test", "parent-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(beadIDs) != 2 {
		t.Fatalf("expected 2 beads, got %d", len(beadIDs))
	}

	findFlag := func(args []string, flag string) string {
		for i, a := range args {
			if a == flag && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}

	// Bead 1 should get per-bead AC, not spec-level AC
	ac1 := findFlag(capturedArgs[0], "--acceptance")
	if !strings.Contains(ac1, "Widget frobs correctly") {
		t.Errorf("bead 1 AC should contain 'Widget frobs correctly', got: %q", ac1)
	}
	if strings.Contains(ac1, "Widget grobs correctly") {
		t.Errorf("bead 1 AC should NOT contain 'Widget grobs correctly', got: %q", ac1)
	}

	// Bead 2 should get its own per-bead AC
	ac2 := findFlag(capturedArgs[1], "--acceptance")
	if !strings.Contains(ac2, "Widget grobs correctly") {
		t.Errorf("bead 2 AC should contain 'Widget grobs correctly', got: %q", ac2)
	}
	if strings.Contains(ac2, "Widget frobs correctly") {
		t.Errorf("bead 2 AC should NOT contain 'Widget frobs correctly', got: %q", ac2)
	}
}

func TestCreateImplementationBeads_FallsBackToSpecAC(t *testing.T) {
	tmp := t.TempDir()

	specContent := `---
status: Approved
---
# Spec 042-test

## Requirements
1. Widget must frob

## Acceptance Criteria
- [ ] Widget frobs correctly
- [ ] Widget grobs correctly
`
	os.WriteFile(filepath.Join(tmp, "spec.md"), []byte(specContent), 0644)

	// Plan without per-bead acceptance criteria
	planContent := `---
status: Approved
spec_id: "042-test"
version: "1.0"
---

# Plan

## Bead 1: Implement widget

**Steps**
1. Create widget
2. Add logic
3. Add tests

**Verification**
- [ ] ` + "`go test ./...`" + ` passes

**Depends on**
None
`
	planPath := filepath.Join(tmp, "plan.md")
	os.WriteFile(planPath, []byte(planContent), 0644)

	var capturedArgs [][]string
	orig := planRunBDFn
	defer func() { planRunBDFn = orig }()
	planRunBDFn = func(args ...string) ([]byte, error) {
		argsCopy := make([]string, len(args))
		copy(argsCopy, args)
		capturedArgs = append(capturedArgs, argsCopy)
		return []byte(`{"id":"test-bead-1"}`), nil
	}

	beadIDs, err := createImplementationBeads(planPath, "042-test", "parent-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(beadIDs) != 1 {
		t.Fatalf("expected 1 bead, got %d", len(beadIDs))
	}

	// Should fall back to spec-level AC
	findFlag := func(args []string, flag string) string {
		for i, a := range args {
			if a == flag && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}

	ac := findFlag(capturedArgs[0], "--acceptance")
	if !strings.Contains(ac, "Widget frobs correctly") {
		t.Errorf("expected fallback to spec AC containing 'Widget frobs correctly', got: %q", ac)
	}
	if !strings.Contains(ac, "Widget grobs correctly") {
		t.Errorf("expected fallback to spec AC containing 'Widget grobs correctly', got: %q", ac)
	}
}

func TestExtractBeadSectionContents(t *testing.T) {
	content := `# Plan

## ADR Fitness
Some text.

## Bead 1: First

**Steps**
1. Do thing one
2. Do thing two

**Verification**
- [ ] Tests pass

**Depends on**
None

## Bead 2: Second

**Steps**
1. Do other thing

**Depends on**
Bead 1

## Provenance

Some provenance table.
`
	result := extractBeadSectionContents(content)

	if len(result) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(result))
	}

	bead1 := result["Bead 1: First"]
	if !strings.Contains(bead1, "Do thing one") {
		t.Error("bead 1 content should include steps")
	}
	if !strings.Contains(bead1, "Tests pass") {
		t.Error("bead 1 content should include verification")
	}

	bead2 := result["Bead 2: Second"]
	if !strings.Contains(bead2, "Do other thing") {
		t.Error("bead 2 content should include steps")
	}
}

func TestParseADRIDs(t *testing.T) {
	touchpoints := `- [ADR-0023](../../adr/ADR-0023.md): Extends beads as state store
- [ADR-0012](../../adr/ADR-0012.md): Compose with external CLIs
`
	ids := parseADRIDs(touchpoints)
	if len(ids) != 2 {
		t.Fatalf("expected 2 ADR IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "ADR-0023" || ids[1] != "ADR-0012" {
		t.Errorf("unexpected ADR IDs: %v", ids)
	}

	// Dedup
	ids2 := parseADRIDs("ADR-0001 and ADR-0001 again")
	if len(ids2) != 1 {
		t.Errorf("expected dedup to 1, got %d", len(ids2))
	}

	// None
	ids3 := parseADRIDs("None applicable.")
	if len(ids3) != 0 {
		t.Errorf("expected 0 for 'None', got %d", len(ids3))
	}
}

func TestBuildBeadMetadata(t *testing.T) {
	meta := buildBeadMetadata("074-test", []string{"internal/foo.go", "cmd/bar.go"})
	if !strings.Contains(meta, `"spec_id":"074-test"`) {
		t.Errorf("metadata missing spec_id: %s", meta)
	}
	if !strings.Contains(meta, "internal/foo.go") {
		t.Errorf("metadata missing file_paths: %s", meta)
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

func TestHandleExistingBeads_NoChildren(t *testing.T) {
	// Stub ListJSON to return empty
	origList := planListJSONFn
	planListJSONFn = func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}
	defer func() { planListJSONFn = origList }()

	err := handleExistingBeads("epic-123", "version: 2\n")
	if err != nil {
		t.Fatalf("expected nil error for no children, got: %v", err)
	}
}

func TestHandleExistingBeads_AllOpen_ClosesAndProceeds(t *testing.T) {
	children := []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}{
		{ID: "bead-old-1", Status: "open"},
		{ID: "bead-old-2", Status: "open"},
	}
	childJSON, _ := json.Marshal(children)

	origList := planListJSONFn
	planListJSONFn = func(args ...string) ([]byte, error) {
		return childJSON, nil
	}
	defer func() { planListJSONFn = origList }()

	var closedArgs []string
	origCombined := planRunBDCombinedFn
	planRunBDCombinedFn = func(args ...string) ([]byte, error) {
		closedArgs = append(closedArgs, args...)
		return nil, nil
	}
	defer func() { planRunBDCombinedFn = origCombined }()

	err := handleExistingBeads("epic-123", "---\nversion: 2\n---\n# Plan\n")
	if err != nil {
		t.Fatalf("expected nil error for all-open children, got: %v", err)
	}

	// Should have closed both beads
	if len(closedArgs) == 0 {
		t.Fatal("expected close call, got none")
	}
	joined := strings.Join(closedArgs, " ")
	if !strings.Contains(joined, "bead-old-1") || !strings.Contains(joined, "bead-old-2") {
		t.Errorf("expected both bead IDs in close call, got: %s", joined)
	}
	if !strings.Contains(joined, "superseded by plan v2") {
		t.Errorf("expected supersede reason, got: %s", joined)
	}
}

func TestHandleExistingBeads_InProgress_ReturnsError(t *testing.T) {
	children := []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}{
		{ID: "bead-ok", Status: "open"},
		{ID: "bead-active", Status: "in_progress"},
	}
	childJSON, _ := json.Marshal(children)

	origList := planListJSONFn
	planListJSONFn = func(args ...string) ([]byte, error) {
		return childJSON, nil
	}
	defer func() { planListJSONFn = origList }()

	err := handleExistingBeads("epic-123", "version: 1\n")
	if err == nil {
		t.Fatal("expected error for in-progress bead")
	}
	if !strings.Contains(err.Error(), "bead-active") {
		t.Errorf("error should mention bead ID: %v", err)
	}
	if !strings.Contains(err.Error(), "in_progress") {
		t.Errorf("error should mention status: %v", err)
	}
}

func TestHandleExistingBeads_Closed_ReturnsError(t *testing.T) {
	children := []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}{
		{ID: "bead-done", Status: "closed"},
	}
	childJSON, _ := json.Marshal(children)

	origList := planListJSONFn
	planListJSONFn = func(args ...string) ([]byte, error) {
		return childJSON, nil
	}
	defer func() { planListJSONFn = origList }()

	err := handleExistingBeads("epic-123", "version: 1\n")
	if err == nil {
		t.Fatal("expected error for closed bead")
	}
	if !strings.Contains(err.Error(), "bead-done") {
		t.Errorf("error should mention bead ID: %v", err)
	}
}

func TestExtractPlanVersion(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{"version: 2\n", "2"},
		{"version: \"1.0\"\n", "1.0"},
		{"---\nstatus: Draft\nversion: 3\n---\n", "3"},
		{"no version here\n", "unknown"},
	}
	for _, tt := range tests {
		got := extractPlanVersion(tt.content)
		if got != tt.expected {
			t.Errorf("extractPlanVersion(%q) = %q, want %q", tt.content, got, tt.expected)
		}
	}
}
