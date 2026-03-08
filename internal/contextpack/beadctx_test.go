package contextpack

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderBeadContext_FullFields(t *testing.T) {
	restore := SetBeadShowForTest(func(args ...string) ([]byte, error) {
		entry := []beadShowEntry{{
			ID:                 "bead-123",
			Title:              "[074-test] Bead 1: Widget Factory",
			Description:        "**Steps**\n1. Create widget.go\n2. Add tests\n\n**Verification**\n- [ ] `go test ./...` passes",
			AcceptanceCriteria: "- [ ] Widget frobs correctly\n- [ ] Widget grobs correctly",
			Design:             "## Requirements\n\n1. Widget must frob\n2. Widget must grob\n\n## ADR Decisions\n\n### ADR-0023\n\nUse beads as single state store.",
			Metadata: map[string]interface{}{
				"spec_id":    "074-test",
				"file_paths": []interface{}{"internal/widget/frob.go", "internal/widget/grob.go"},
			},
		}}
		return json.Marshal(entry)
	})
	defer restore()

	rendered, err := RenderBeadContext("bead-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Header
	if !strings.Contains(rendered, "# Bead Context: [074-test] Bead 1: Widget Factory") {
		t.Error("missing title header")
	}
	if !strings.Contains(rendered, "**Bead**: bead-123") {
		t.Error("missing bead ID")
	}

	// Design (requirements + ADR)
	if !strings.Contains(rendered, "Widget must frob") {
		t.Error("missing requirements in design")
	}
	if !strings.Contains(rendered, "ADR-0023") {
		t.Error("missing ADR decision in design")
	}

	// Acceptance Criteria
	if !strings.Contains(rendered, "## Acceptance Criteria") {
		t.Error("missing acceptance criteria section")
	}
	if !strings.Contains(rendered, "Widget frobs correctly") {
		t.Error("missing AC content")
	}

	// Work Chunk (description)
	if !strings.Contains(rendered, "## Work Chunk") {
		t.Error("missing work chunk section")
	}
	if !strings.Contains(rendered, "Create widget.go") {
		t.Error("missing work chunk content")
	}

	// File Paths
	if !strings.Contains(rendered, "## Key File Paths") {
		t.Error("missing file paths section")
	}
	if !strings.Contains(rendered, "internal/widget/frob.go") {
		t.Error("missing file path")
	}

	// Token estimate
	if !strings.Contains(rendered, "tokens") {
		t.Error("missing token estimate")
	}
}

func TestRenderBeadContext_EmptyFields(t *testing.T) {
	restore := SetBeadShowForTest(func(args ...string) ([]byte, error) {
		entry := []beadShowEntry{{
			ID:    "bead-empty",
			Title: "Empty Bead",
		}}
		return json.Marshal(entry)
	})
	defer restore()

	rendered, err := RenderBeadContext("bead-empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(rendered, "# Bead Context: Empty Bead") {
		t.Error("missing title")
	}
	// Should not contain sections for empty fields
	if strings.Contains(rendered, "## Work Chunk") {
		t.Error("should not have work chunk for empty description")
	}
	if strings.Contains(rendered, "## Acceptance Criteria") {
		t.Error("should not have AC for empty field")
	}
}

func TestRenderBeadContext_NotFound(t *testing.T) {
	restore := SetBeadShowForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	defer restore()

	_, err := RenderBeadContext("missing")
	if err == nil {
		t.Fatal("expected error for missing bead")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention not found: %v", err)
	}
}
