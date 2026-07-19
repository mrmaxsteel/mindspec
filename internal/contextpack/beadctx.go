package contextpack

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
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
//
// Deprecated: New callers should use BuildBead (internal/contextpack/budgeter.go)
// which honors a token budget, emits the spec 088 six-tier layout, and appends a
// SHA-256 Provenance block. RenderBeadContext is preserved for byte-identical
// output when `mindspec context bead <id>` is invoked without `--max-tokens`.
func RenderBeadContext(beadID string) (string, error) {
	// R3-gated CLI ingress + gate-all-ids (ADR-0042 §1, round 7): beadID
	// feeds a `bd show` argv build directly — validate BEFORE any bd
	// spawn.
	if err := idvalidate.BeadID(beadID); err != nil {
		// R4: the id just FAILED validation — render it forced-quoted.
		return "", fmt.Errorf("invalid bead id %s: %w", idrender.Bead(beadID), err)
	}
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

	// R4: Title is agent-writable single-line free text (termsafe.Escape);
	// ID is an ID-typed position (idrender.Bead). Design/AcceptanceCriteria/
	// Description stay untouched below — they are fenced multi-line
	// payload, not single-line render positions.
	b.WriteString(fmt.Sprintf("# Bead Context: %s\n", termsafe.Escape(e.Title)))
	b.WriteString(fmt.Sprintf("**Bead**: %s", idrender.Bead(e.ID)))

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
						// R4: each file_paths entry is agent-writable
						// metadata — escape per-line.
						b.WriteString(fmt.Sprintf("- %s\n", termsafe.Escape(s)))
					}
				}
				b.WriteString("\n")
			}
		}
	}

	return b.String(), nil
}
