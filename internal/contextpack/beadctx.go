package contextpack

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

// beadShowFn is a package-level variable for testability.
var beadShowFn = bead.RunBD

// SetBeadShowForTest swaps beadShowFn for testing and returns a restore function.
func SetBeadShowForTest(fn func(args ...string) ([]byte, error)) func() {
	orig := beadShowFn
	beadShowFn = fn
	return func() { beadShowFn = orig }
}

// beadShowEntry represents the JSON structure returned by `bd show <id> --json`.
type beadShowEntry struct {
	ID                 string                 `json:"id"`
	Title              string                 `json:"title"`
	Description        string                 `json:"description"`
	AcceptanceCriteria string                 `json:"acceptance_criteria"`
	Design             string                 `json:"design"`
	Metadata           map[string]interface{} `json:"metadata"`
}

// RenderBeadContext fetches bead data via `bd show <beadID> --json` and renders
// a markdown context document. This replaces the old BuildBeadPrimer/RenderBeadPrimer
// flow — all context is pre-populated in the bead at plan approval time (Spec 074).
func RenderBeadContext(beadID string) (string, error) {
	out, err := beadShowFn("show", beadID, "--json")
	if err != nil {
		return "", fmt.Errorf("fetching bead %s: %w", beadID, err)
	}

	var entries []beadShowEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return "", fmt.Errorf("parsing bead %s: %w", beadID, err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("bead %s not found", beadID)
	}

	e := entries[0]
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Bead Context: %s\n", e.Title))
	b.WriteString(fmt.Sprintf("**Bead**: %s", e.ID))

	// Estimate tokens from total content
	totalLen := len(e.Description) + len(e.AcceptanceCriteria) + len(e.Design)
	if totalLen > 0 {
		b.WriteString(fmt.Sprintf(" | **~%d tokens**", totalLen/4))
	}
	b.WriteString("\n\n")

	if e.Design != "" {
		b.WriteString(e.Design)
		b.WriteString("\n\n")
	}

	if e.AcceptanceCriteria != "" {
		b.WriteString("## Acceptance Criteria\n\n")
		b.WriteString(e.AcceptanceCriteria)
		b.WriteString("\n\n")
	}

	if e.Description != "" {
		b.WriteString("## Work Chunk\n\n")
		b.WriteString(e.Description)
		b.WriteString("\n\n")
	}

	// Extract file_paths from metadata
	if e.Metadata != nil {
		if fpRaw, ok := e.Metadata["file_paths"]; ok {
			if fpList, ok := fpRaw.([]interface{}); ok && len(fpList) > 0 {
				b.WriteString("## Key File Paths\n\n")
				for _, fp := range fpList {
					if s, ok := fp.(string); ok {
						b.WriteString(fmt.Sprintf("- %s\n", s))
					}
				}
				b.WriteString("\n")
			}
		}
	}

	return b.String(), nil
}
