package approve

import (
	"encoding/json"
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateSpecApproval_UpdatesSection(t *testing.T) {
	tmp := t.TempDir()
	specContent := `# Spec 010: Test Feature

## Goal

Build a test feature.

## Impacted Domains

- **core**: test

## ADR Touchpoints

- ADR-0001

## Requirements

1. Requirement one
2. Requirement two

## Scope

### In Scope
- Things

### Out of Scope
- Other things

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Open Questions

None

## Approval

- **Status**: DRAFT
- **Approved By**: -
`
	specPath := filepath.Join(tmp, "spec.md")
	os.WriteFile(specPath, []byte(specContent), 0644)

	err := updateSpecApproval(specPath, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(specPath)
	content := string(data)

	if !strings.Contains(content, "status: Approved") {
		t.Error("expected status: Approved in frontmatter")
	}
	if !strings.Contains(content, "approved_at:") {
		t.Error("expected approved_at in frontmatter")
	}
	if !strings.Contains(content, "approved_by: user") {
		t.Error("expected approved_by in frontmatter")
	}
	if !strings.Contains(content, "**Status**: APPROVED") {
		t.Error("expected Status: APPROVED in output")
	}
	if !strings.Contains(content, "**Approved By**: user") {
		t.Error("expected Approved By: user in output")
	}
	if !strings.Contains(content, "**Approval Date**:") {
		t.Error("expected Approval Date in output")
	}
	// Original content preserved
	if !strings.Contains(content, "## Goal") {
		t.Error("Goal section lost")
	}
	if !strings.Contains(content, "Build a test feature.") {
		t.Error("Goal content lost")
	}
	// DRAFT should be gone
	if strings.Contains(content, "DRAFT") {
		t.Error("DRAFT should have been replaced")
	}
}

func TestUpdateSpecApproval_AddsApprovalSectionWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	specContent := "# Spec\n\n## Goal\n\nSomething.\n"
	specPath := filepath.Join(tmp, "spec.md")
	os.WriteFile(specPath, []byte(specContent), 0644)

	err := updateSpecApproval(specPath, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(specPath)
	content := string(data)
	if !strings.Contains(content, "## Approval") {
		t.Fatal("expected Approval section to be added")
	}
	if !strings.Contains(content, "status: Approved") {
		t.Error("expected approved status frontmatter")
	}
}

func TestUpdateSpecApproval_ApprovalNotLast(t *testing.T) {
	tmp := t.TempDir()
	specContent := `# Spec

## Approval

- **Status**: DRAFT

## Appendix

Extra stuff here.
`
	specPath := filepath.Join(tmp, "spec.md")
	os.WriteFile(specPath, []byte(specContent), 0644)

	err := updateSpecApproval(specPath, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(specPath)
	content := string(data)

	// Appendix should be preserved
	if !strings.Contains(content, "## Appendix") {
		t.Error("Appendix section lost")
	}
	if !strings.Contains(content, "Extra stuff here.") {
		t.Error("Appendix content lost")
	}
	if !strings.Contains(content, "status: Approved") {
		t.Error("expected approved status frontmatter")
	}
	if !strings.Contains(content, "APPROVED") {
		t.Error("expected APPROVED in output")
	}
}

// TestScaffoldPlanEmitsADRCitations (Spec 100 R4 AC1; strengthened by Spec
// 122 R5/AC-12b): the generated plan.md skeleton names the exact
// `adr_citations` frontmatter key the gate reads, so the author sees it up
// front. The key must appear inside the YAML frontmatter region (between
// the opening and closing `---`).
//
// Spec 122 AC-12b additionally pins the commented REMEDY-GUIDANCE sentence
// beside that key ("cite the Accepted ADRs whose Domain(s) cover" —
// `internal/approve/spec.go`'s `scaffoldPlan`) so the documented-key remedy
// text cannot silently drop; this half is red if that guidance sentence is
// removed even though the bare `adr_citations` key remains (a mutation
// probe recorded in review evidence: dropping the sentence while keeping
// the key turns this assertion, and only this assertion, red).
func TestScaffoldPlanEmitsADRCitations(t *testing.T) {
	out := scaffoldPlan("100-x")

	if !strings.Contains(out, "adr_citations") {
		t.Errorf("expected scaffoldPlan output to contain the literal adr_citations key, got:\n%s", out)
	}

	// Confirm the key lands inside the frontmatter region, not the body.
	const fence = "---"
	first := strings.Index(out, fence)
	if first < 0 {
		t.Fatalf("expected opening frontmatter fence, got:\n%s", out)
	}
	second := strings.Index(out[first+len(fence):], fence)
	if second < 0 {
		t.Fatalf("expected closing frontmatter fence, got:\n%s", out)
	}
	frontmatter := out[first : first+len(fence)+second]
	if !strings.Contains(frontmatter, "adr_citations") {
		t.Errorf("expected adr_citations within the frontmatter region, got frontmatter:\n%s", frontmatter)
	}

	// Spec 122 AC-12b: the commented remedy-guidance sentence beside the
	// key — the actual working remedy ("cite the Accepted ADRs whose
	// Domain(s) cover" this plan's impacted domains), not just the bare key
	// name.
	const guidanceSentence = "cite the Accepted ADRs whose Domain(s) cover"
	if !strings.Contains(frontmatter, guidanceSentence) {
		t.Errorf("expected the adr_citations remedy-guidance sentence %q within the frontmatter region, got frontmatter:\n%s", guidanceSentence, frontmatter)
	}
}

func TestSpecApprove_WritesSpecID(t *testing.T) {
	fakeDir := t.TempDir()
	logPath := filepath.Join(fakeDir, "bd.log")
	if err := os.WriteFile(filepath.Join(fakeDir, "bd"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$BD_INVOCATIONS\"\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BD_INVOCATIONS", logPath)
	t.Setenv("PATH", fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Cleanup(phase.SetRunBDForTest(func(...string) ([]byte, error) { return nil, nil }))
	root := t.TempDir()
	const specID = "129-linkage"
	dir := filepath.Join(root, ".mindspec", "specs", specID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(specE2EValidSpecMD(specID)), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(phase.SetListJSONForTest(func(...string) ([]byte, error) { return []byte(`[]`), nil }))
	creates := 0
	t.Cleanup(SetSpecRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			creates++
			var meta map[string]interface{}
			found := false
			epic := false
			for i, a := range args {
				if a == "--type=epic" {
					epic = true
				}
				if a == "--metadata" && i+1 < len(args) {
					found = true
					if err := json.Unmarshal([]byte(args[i+1]), &meta); err != nil {
						t.Fatal(err)
					}
				}
			}
			if !epic || !found || meta["spec_num"] != float64(129) {
				t.Fatalf("expected epic metadata, got %q", args)
			}
			if meta["spec_id"] != specID {
				t.Fatalf("epic must carry the full spec_id: %q", args)
			}
			return []byte(`{"id":"epic-129"}`), nil
		}
		if len(args) == 2 && args[0] == "dolt" && args[1] == "push" {
			return nil, nil
		}
		t.Fatalf("unexpected call %q", args)
		return nil, nil
	}))
	if _, err := ApproveSpec(root, specID, "fixture", &executor.MockExecutor{}); err != nil {
		t.Fatal(err)
	}
	if log, err := os.ReadFile(logPath); !os.IsNotExist(err) {
		t.Fatalf("approval pin must not invoke external bd: log=%q, err=%v", log, err)
	}
	if creates != 1 {
		t.Fatalf("expected one checked epic create, got %d", creates)
	}
}

func TestSpecApprove_DuplicateRefusesBeforeApproval(t *testing.T) {
	for _, specID := range []string{"129-linkage", "129b-linkage"} {
		t.Run(specID, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, ".mindspec/specs", specID)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "spec.md")
			before := specE2EValidSpecMD(specID)
			if err := os.WriteFile(path, []byte(before), 0644); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(phase.SetListJSONForTest(func(...string) ([]byte, error) {
				return []byte(fmt.Sprintf(`[{"id":"epic-a","metadata":{"spec_id":%q}},{"id":"epic-b","status":"closed","metadata":{"spec_id":%q}}]`, specID, specID)), nil
			}))
			mock := &executor.MockExecutor{}
			_, err := ApproveSpec(root, specID, "test", mock)
			if err == nil || !strings.Contains(err.Error(), "epic-a, epic-b") {
				t.Fatalf("duplicate approval: %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != before || len(mock.Calls) != 0 {
				t.Fatal("duplicate refusal changed approval or executor state")
			}
		})
	}
}
