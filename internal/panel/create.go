package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// briefHeaderOpen/briefHeaderClose delimit the BRIEF.md region Create
// owns exclusively (Spec 110 R1). Everything outside this region —
// including everything below the closing marker — is skill-authored
// and Create never touches it.
const (
	briefHeaderOpen  = "<!-- mindspec:panel-header -->"
	briefHeaderClose = "<!-- /mindspec:panel-header -->"
)

// briefStubBody is the panel-specific skeleton a first `create` writes
// below the machine-managed header, mirroring the section headings
// /ms-panel-run step 3 fills in. It is written verbatim exactly once
// (the first Create for a slug); every later re-panel Create leaves it,
// and everything else the skill authored below it, untouched.
const briefStubBody = `## Summary

<!-- TODO(skill): one-paragraph summary of what this panel reviews -->

## Files in Scope

<!-- TODO(skill): the files/paths this review covers -->

## Prior-Round Asks

<!-- TODO(skill): concrete_changes_required from the previous round, if any -->

## Lens

<!-- TODO(skill): per-slot review lens assignments -->
`

// CreateInput carries the plain values Create writes into a panel
// directory's panel.json and BRIEF.md machine-managed header, in one
// atomic operation (Spec 110 R1). Every field is a plain value the
// CALLER (cmd/mindspec) resolves — the 109 config resolvers for
// ExpectedReviewers/ApproveThresholdExpr, the executor for
// ReviewedHeadSHA — so internal/panel stays an import-clean,
// config-free, git-free leaf (spec 109's "config reaches the leaf only
// as plain values" pattern, R7b).
type CreateInput struct {
	// BeadID is nil for a non-bead (final-review/PR) panel; it marshals
	// to panel.json's `bead_id: null`.
	BeadID *string
	// Spec is the owning spec ID.
	Spec string
	// Target is the reviewed ref (e.g. "bead/mindspec-x.1"), also
	// recorded verbatim as the BRIEF header's branch field.
	Target string
	// Round is the panel round, 1-based.
	Round int
	// ExpectedReviewers is the recorded reviewer count (109's
	// config.PanelExpectedReviewers(), read by the caller).
	ExpectedReviewers int
	// ApproveThresholdExpr is the optional recorded threshold override
	// (109's config.PanelApproveThresholdExpr(), read by the caller);
	// "" means "use the N-1 default" (see Panel.ApproveThreshold).
	ApproveThresholdExpr string
	// ReviewedHeadSHA is the target ref's resolved commit, captured by
	// the caller AT WRITE TIME. Never empty — Create does not validate
	// this (a leaf takes plain values on faith), but every caller must
	// supply it: panel.json must never omit reviewed_head_sha.
	ReviewedHeadSHA string
}

// Create writes dir/panel.json and rewrites dir/BRIEF.md's
// machine-managed header block in a single logical operation (Spec 110
// R1): Round and ReviewedHeadSHA land in the SAME Panel value, so no
// code path can write one without the other, and both files are
// computed in full — including validating any existing BRIEF.md's
// marker state — before either is touched on disk. Create never reads,
// writes, or deletes any `<slot>-round-<N>.json` verdict file.
//
// On a first create (no BRIEF.md yet at dir/BRIEF.md), the header is
// written followed by a fresh briefStubBody for the skill to fill in.
// On a re-panel (BRIEF.md already exists with exactly one matching
// marker pair), only the delimited region is replaced in place; every
// byte before the opening marker and after the closing marker —
// including the skill-authored body and any CRLF line endings in it —
// is preserved exactly. A BRIEF.md with no markers at all (legacy) gets
// a fresh header prepended, with the entire existing file kept
// byte-identical below it.
//
// If the existing BRIEF.md's markers are ambiguous or corrupt — an
// opening marker with no matching closing marker, a closing marker with
// no opening marker, or more than one opening/closing marker pair —
// Create returns a non-nil error and writes NEITHER panel.json NOR
// BRIEF.md: the marker-state validation happens before any write, so a
// corrupt BRIEF can never leave the two files disagreeing.
//
// internal/panel is a leaf: Create imports no internal/config and no
// git.
func Create(dir string, in CreateInput) error {
	p := Panel{
		BeadID:               in.BeadID,
		Spec:                 in.Spec,
		Target:               in.Target,
		Round:                in.Round,
		ExpectedReviewers:    in.ExpectedReviewers,
		ReviewedHeadSHA:      in.ReviewedHeadSHA,
		ApproveThresholdExpr: in.ApproveThresholdExpr,
	}
	panelData, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal panel.json: %w", err)
	}

	briefPath := filepath.Join(dir, "BRIEF.md")
	existing, readErr := os.ReadFile(briefPath)
	briefExists := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read %s: %w", briefPath, readErr)
	}

	slug := filepath.Base(dir)
	header := renderBriefHeader(slug, in.Round, in.Target, in.ReviewedHeadSHA)

	var newBrief string
	if briefExists {
		spliced, err := spliceBriefHeader(string(existing), header)
		if err != nil {
			return fmt.Errorf("BRIEF.md header in %s: %w", briefPath, err)
		}
		newBrief = spliced
	} else {
		newBrief = header + "\n\n" + briefStubBody
	}

	// Both files are fully computed above; only now do we touch disk,
	// and both writes happen unconditionally together.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create panel dir %s: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), panelData, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", FileName, err)
	}
	if err := os.WriteFile(briefPath, []byte(newBrief), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", briefPath, err)
	}
	return nil
}

// renderBriefHeader renders the machine-managed BRIEF region: the
// panel-identifying fields plus a fixed, non-panel-specific "Your job"
// verdict-output contract. This is the ONE place a reviewer's
// verdict-JSON instructions are written — the skill (Bead 5) no longer
// re-authors a second copy.
func renderBriefHeader(slug string, round int, target, sha string) string {
	var b strings.Builder
	b.WriteString(briefHeaderOpen)
	b.WriteString("\n## Panel Registration\n\n")
	fmt.Fprintf(&b, "- **Slug**: `%s`\n", slug)
	fmt.Fprintf(&b, "- **Round**: %d\n", round)
	fmt.Fprintf(&b, "- **Branch**: `%s`\n", target)
	fmt.Fprintf(&b, "- **Reviewed commit**: `%s`\n", sha)
	b.WriteString("\n## Your job\n\n")
	b.WriteString("Review the diff at the reviewed commit above and write your verdict as\n")
	fmt.Fprintf(&b, "a single JSON file named `<your-slot>-round-%d.json` beside this BRIEF\n", round)
	b.WriteString("(the panel artifact schema — see `.mindspec/domains/workflow/interfaces.md`\n")
	b.WriteString("§ Panel Artifact Schema). Required and optional top-level fields:\n\n")
	b.WriteString("- `verdict` (required): one of `APPROVE`, `REQUEST_CHANGES`, `REJECT`.\n")
	b.WriteString("- `hard_block` (optional boolean, a top-level sibling of `verdict` — never\n")
	b.WriteString("  a per-finding field): set `true` only for an evidence-bearing halt that\n")
	b.WriteString("  no vote count may override.\n")
	b.WriteString("- `reviewer_id`, `confidence`, `rationale`, `concrete_changes_required`,\n")
	b.WriteString("  `findings`: reviewer-authored context, read presentation-only by\n")
	b.WriteString("  `mindspec panel tally` — not consumed by the gate decision.\n")
	b.WriteString(briefHeaderClose)
	return b.String()
}

// spliceBriefHeader replaces the delimited machine-managed region in
// an existing BRIEF.md with header, preserving every other byte
// exactly. It returns an error — writing nothing — when the marker
// state is ambiguous or corrupt.
func spliceBriefHeader(existing, header string) (string, error) {
	openCount := strings.Count(existing, briefHeaderOpen)
	closeCount := strings.Count(existing, briefHeaderClose)

	if openCount == 0 && closeCount == 0 {
		// Legacy: no markers at all. Prepend a fresh region; the whole
		// existing file is kept byte-identical below it.
		return header + "\n\n" + existing, nil
	}
	if openCount != 1 || closeCount != 1 {
		return "", fmt.Errorf(
			"expected exactly one matching %q/%q pair (or neither), found %d opening and %d closing",
			briefHeaderOpen, briefHeaderClose, openCount, closeCount)
	}

	openIdx := strings.Index(existing, briefHeaderOpen)
	closeIdx := strings.Index(existing, briefHeaderClose)
	if closeIdx < openIdx {
		return "", fmt.Errorf(
			"closing marker %q precedes opening marker %q", briefHeaderClose, briefHeaderOpen)
	}
	closeEnd := closeIdx + len(briefHeaderClose)

	before := existing[:openIdx]
	after := existing[closeEnd:]
	return before + header + after, nil
}
