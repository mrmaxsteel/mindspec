package next

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// ResolvedWork holds the result of mode resolution for a claimed bead.
type ResolvedWork struct {
	Mode   string
	SpecID string
	Bead   BeadInfo
}

// ResolveMode maps a bead's type and artifact state to a MindSpec mode and spec ID.
//
// Mapping:
//   - task, bug → implement
//   - feature → spec (if no approved spec) or plan (if spec approved)
//
// Spec ID is parsed from the bead title prefix before the first colon.
func ResolveMode(root string, bead BeadInfo) ResolvedWork {
	specID := parseSpecID(bead.Title)

	switch bead.IssueType {
	case "task", "bug":
		return ResolvedWork{
			Mode:   state.ModeImplement,
			SpecID: specID,
			Bead:   bead,
		}
	case "feature":
		mode := resolveFeatureMode(root, specID)
		return ResolvedWork{
			Mode:   mode,
			SpecID: specID,
			Bead:   bead,
		}
	default:
		// Unknown type defaults to implement
		return ResolvedWork{
			Mode:   state.ModeImplement,
			SpecID: specID,
			Bead:   bead,
		}
	}
}

// parseSpecID extracts the spec ID from a bead title.
//
// Tries bracket-prefix convention first:
//
//	"[IMPL 009-feature.1] Chunk title" → "009-feature"
//	"[SPEC 008b-gates] Feature"        → "008b-gates"
//	"[PLAN 009-feature] Plan decomp"   → "009-feature"
//	"[049-hook-command] Bead 1: ..."    → "049-hook-command"
//
// Falls back to legacy colon convention:
//
//	"005-next: Implement work selection" → "005-next"
func parseSpecID(title string) string {
	// Try bracket-prefix: [TAG specID] or [TAG specID.chunk] or [specID]
	if strings.HasPrefix(title, "[") {
		closeBracket := strings.Index(title, "]")
		if closeBracket > 0 {
			inner := title[1:closeBracket] // e.g. "IMPL 009-feature.1"
			spaceIdx := strings.Index(inner, " ")
			if spaceIdx > 0 {
				slug := inner[spaceIdx+1:] // e.g. "009-feature.1"
				// Strip chunk suffix (everything from the last dot if it's followed by digits)
				slug = stripChunkSuffix(slug)
				return slug
			}
			// No space inside brackets: entire content is the spec ID
			// e.g. "[049-hook-command] Bead 1: ..." → "049-hook-command"
			return inner
		}
	}

	// Fallback: colon convention
	idx := strings.Index(title, ":")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(title[:idx])
}

// stripChunkSuffix removes a trailing ".N" chunk suffix from a slug.
// e.g. "009-feature.1" → "009-feature", "008b-gates" → "008b-gates"
func stripChunkSuffix(slug string) string {
	dotIdx := strings.LastIndex(slug, ".")
	if dotIdx <= 0 {
		return slug
	}
	suffix := slug[dotIdx+1:]
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return slug
		}
	}
	return slug[:dotIdx]
}

// resolveFeatureMode checks the spec's artifact state to determine if we're in
// spec mode (draft spec) or plan mode (approved spec with plan needed).
func resolveFeatureMode(root, specID string) string {
	if specID == "" {
		return state.ModeSpec
	}

	specPath := filepath.Join(workspace.SpecDir(root, specID), "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return state.ModeSpec
	}

	content := string(data)
	if strings.Contains(content, "Status: APPROVED") || strings.Contains(content, "**Status**: APPROVED") {
		return state.ModePlan
	}

	return state.ModeSpec
}
