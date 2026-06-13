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

// RenderBeadPrimer tests removed — primer.go deleted in Spec 074.
// See beadctx_test.go for the replacement tests.
