package contextpack

import (
	"testing"
)

func TestExtractSection(t *testing.T) {
	content := `# Spec

## Goal

Build a test feature.

## Requirements

1. First requirement
2. Second requirement

## Acceptance Criteria

- AC1: Thing works
- AC2: Other thing works

## Background

Some background.
`
	tests := []struct {
		heading string
		want    string
	}{
		{"Goal", "Build a test feature."},
		{"Requirements", "1. First requirement\n2. Second requirement"},
		{"Acceptance Criteria", "- AC1: Thing works\n- AC2: Other thing works"},
		{"Background", "Some background."},
		{"Nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.heading, func(t *testing.T) {
			got := ExtractSection(content, tt.heading)
			if got != tt.want {
				t.Errorf("ExtractSection(%q) = %q, want %q", tt.heading, got, tt.want)
			}
		})
	}
}

func TestExtractSection_CaseInsensitive(t *testing.T) {
	content := "## decision\n\nUse DDD.\n"
	got := ExtractSection(content, "Decision")
	if got != "Use DDD." {
		t.Errorf("got %q, want %q", got, "Use DDD.")
	}
}

func TestExtractFilePathsFromText(t *testing.T) {
	text := `
1. Modify internal/state/state.go to add the flag
2. Update cmd/mindspec/next.go and cmd/mindspec/state.go
3. Also check internal/complete/complete_test.go
`
	paths := ExtractFilePathsFromText(text)
	if len(paths) != 4 {
		t.Fatalf("expected 4 paths, got %d: %v", len(paths), paths)
	}
	expected := []string{
		"internal/state/state.go",
		"cmd/mindspec/next.go",
		"cmd/mindspec/state.go",
		"internal/complete/complete_test.go",
	}
	for i, want := range expected {
		if paths[i] != want {
			t.Errorf("path[%d] = %q, want %q", i, paths[i], want)
		}
	}
}

func TestExtractFilePathsFromText_BacktickWrapped(t *testing.T) {
	text := "Modify `internal/foo/bar.go` to add feature"
	paths := ExtractFilePathsFromText(text)
	if len(paths) != 1 || paths[0] != "internal/foo/bar.go" {
		t.Errorf("got %v", paths)
	}
}

func TestExtractFilePathsFromText_NoPaths(t *testing.T) {
	text := "No file paths here at all."
	paths := ExtractFilePathsFromText(text)
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %v", paths)
	}
}

func TestExtractFilePathsFromText_Dedup(t *testing.T) {
	text := "internal/foo.go and internal/foo.go again"
	paths := ExtractFilePathsFromText(text)
	if len(paths) != 1 {
		t.Errorf("expected 1 deduplicated path, got %v", paths)
	}
}

// RenderBeadPrimer tests removed — primer.go deleted in Spec 074.
// See beadctx_test.go for the replacement tests.
