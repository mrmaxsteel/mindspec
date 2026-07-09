package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	// Gate records which of config.PanelGateKeys' five gate mixes this
	// panel was created from — plain, decision-inert recorded metadata
	// per the spec-112-R9 stable contract (Panel.Gate: name `gate`, type
	// string, `omitempty`, parse-lenient). "" means the caller omitted
	// --gate; via Panel.Gate's `omitempty` tag this writes NO `gate` key
	// at all, so an omitted Gate is byte-identical to pre-spec-113
	// output (spec 113 R3, completing 112's deferred writer). The
	// membership check against PanelGateKeys and the gate-scoped
	// ExpectedReviewers/ApproveThresholdExpr resolution both happen in
	// the caller (cmd/mindspec) — this leaf takes the plain value on
	// faith and never imports internal/config or git.
	Gate string
}

// Create writes dir/panel.json and rewrites dir/BRIEF.md's
// machine-managed header block in a single logical operation (Spec 110
// R1): Round and ReviewedHeadSHA land in the SAME Panel value, so no
// code path can write one without the other, and both files are
// computed in full — including validating any existing BRIEF.md's
// marker state — before either is touched on disk. Create never reads,
// writes, or deletes any `<slot>-round-<N>.json` verdict file.
//
// panel.json is written as a full-struct overwrite: Create never reads
// an existing panel.json first (unlike its read-before-splice
// BRIEF.md handling below). A re-panel of a directory whose panel.json
// carries a hand-set `abandoned`/`abandon_reason` (the /ms-panel-tally
// Abandon procedure) therefore CLEARS those fields and revives the
// panel into an active round — this is the known, intentional
// behavior of calling Create again, not an oversight
// (TestCreate_RepanelOfAbandonedPanelRevivesIt pins it).
//
// The two writes below (panel.json, then BRIEF.md) are sequential and
// not crash-atomic as a pair: a process death between them leaves
// panel.json bumped and BRIEF.md stale. The next Create call recomputes
// and rewrites both from scratch, so this is a self-healing bound, not
// a corruption risk — no temp-file+rename is used here.
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
		Gate:                 in.Gate,
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

// markerLookalikeRE loosely matches a line that RESEMBLES one of the
// two managed-region markers — the `<!--`/`-->` HTML-comment
// delimiters bracketing "mindspec:panel-header", optionally prefixed
// with "/" for the closing form — but tolerates internal whitespace
// drift (extra spaces, a tab) that keeps it from being an EXACT match
// of briefHeaderOpen/briefHeaderClose. It exists only to distinguish a
// mangled marker from genuinely marker-free legacy content (G1.2): a
// line that matches this pattern without exactly matching either
// marker constant is a corrupt marker, never legacy body text.
var markerLookalikeRE = regexp.MustCompile(`^<!--\s*/?\s*mindspec:panel-header\s*-->$`)

// scanNonFenceLines calls fn once per line of s, in file order,
// skipping every line inside a fenced code block — a region bracketed
// by lines whose trimmed content starts with "```" (a Markdown fence
// used, e.g., to quote the header syntax for documentation). offset is
// the byte offset of the line's first character in s; content is the
// line with its terminator removed (a trailing "\r" is stripped too,
// so CRLF files scan the same as LF ones).
func scanNonFenceLines(s string, fn func(offset int, content string)) {
	inFence := false
	start := 0
	for start <= len(s) {
		lineEnd := len(s)
		hasNL := false
		if i := strings.IndexByte(s[start:], '\n'); i >= 0 {
			lineEnd = start + i
			hasNL = true
		}
		content := strings.TrimSuffix(s[start:lineEnd], "\r")
		if strings.HasPrefix(strings.TrimSpace(content), "```") {
			inFence = !inFence
		} else if !inFence {
			fn(start, content)
		}
		if !hasNL {
			break
		}
		start = lineEnd + 1
	}
}

// findMarkerLines returns the byte offsets of every GENUINE occurrence
// of marker in s: a line, outside any fenced code block, whose content
// is EXACTLY marker and nothing else — no leading or trailing text on
// that line (G1.1). A marker string reproduced inside a fenced code
// block to document the header syntax, or quoted mid-line in prose or
// an inline code span, is not genuine and is never counted — this is
// what lets a BRIEF body quote the marker for documentation without
// jamming the next re-panel with a false duplicated-markers rejection.
func findMarkerLines(s, marker string) []int {
	var offsets []int
	scanNonFenceLines(s, func(offset int, content string) {
		if content == marker {
			offsets = append(offsets, offset)
		}
	})
	return offsets
}

// hasMarkerLookalike reports whether s contains a line, outside any
// fenced code block, that resembles a managed-region marker
// (markerLookalikeRE) without being an exact match — a marker mangled
// by stray whitespace (G1.2).
func hasMarkerLookalike(s string) bool {
	found := false
	scanNonFenceLines(s, func(_ int, content string) {
		if found {
			return
		}
		if markerLookalikeRE.MatchString(content) && content != briefHeaderOpen && content != briefHeaderClose {
			found = true
		}
	})
	return found
}

// spliceBriefHeader replaces the delimited machine-managed region in
// an existing BRIEF.md with header, preserving every other byte
// exactly. It returns an error — writing nothing — when the marker
// state is ambiguous or corrupt: more or fewer than one genuine
// opening/closing marker (findMarkerLines), a closing marker that
// precedes the opening one, or a marker mangled by stray whitespace
// where a naive scan would otherwise read the file as marker-free
// (hasMarkerLookalike, G1.2) — reject-as-corrupt rather than silently
// prepending a second header alongside the mangled one.
func spliceBriefHeader(existing, header string) (string, error) {
	opens := findMarkerLines(existing, briefHeaderOpen)
	closes := findMarkerLines(existing, briefHeaderClose)

	if len(opens) == 0 && len(closes) == 0 {
		if hasMarkerLookalike(existing) {
			return "", fmt.Errorf(
				"found a %s/%s marker mangled by stray whitespace, with no exact match — refusing to guess; fix or remove it by hand",
				briefHeaderOpen, briefHeaderClose)
		}
		// Legacy: no markers at all. Prepend a fresh region; the whole
		// existing file is kept byte-identical below it.
		return header + "\n\n" + existing, nil
	}
	if len(opens) != 1 || len(closes) != 1 {
		return "", fmt.Errorf(
			"expected exactly one genuine %q/%q marker pair (or neither), found %d opening and %d closing",
			briefHeaderOpen, briefHeaderClose, len(opens), len(closes))
	}

	openIdx := opens[0]
	closeIdx := closes[0]
	if closeIdx < openIdx {
		return "", fmt.Errorf(
			"closing marker %q precedes opening marker %q", briefHeaderClose, briefHeaderOpen)
	}
	closeEnd := closeIdx + len(briefHeaderClose)

	before := existing[:openIdx]
	after := existing[closeEnd:]
	return before + header + after, nil
}
