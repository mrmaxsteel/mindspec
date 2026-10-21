package panel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
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

## Acceptance Criteria

Use the quoted acceptance criteria in the managed region above.

## Prior-Round Asks

<!-- TODO(skill): concrete_changes_required from the previous round, if any -->

## Lens

<!-- TODO(skill): per-slot review lens assignments -->
`

// CreateInput contains caller-resolved registration values. The caller resolves
// the roster from config and the reviewed commit and digest from the target.
type CreateInput struct {
	BriefRegions      BriefRegions
	BeadID            *string
	Spec              string
	Target            string
	Round             int
	ExpectedReviewers int
	// ApproveThresholdExpr is retained for source compatibility, but never written.
	ApproveThresholdExpr string
	ReviewedHeadSHA      string
	SubjectDigest        string
	Gate                 string
	Roster               []RosterSlot
	// ReviewersOverride replaces the previous roster when --reviewers was supplied.
	ReviewersOverride bool
	// ReportDropped receives each unbacked entry after the snapshot is written.
	ReportDropped func(string)
}

// CheckCreateRound refuses a round at or before the latest verdict round.
// Call it before scratch cleanup. Create also calls it before writing.
func CheckCreateRound(dir string, round int) error {
	res, err := Tally(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if res != nil && res.LatestRound > 0 && round <= res.LatestRound {
		return &CreateRoundConflict{Round: round, LatestRound: res.LatestRound}
	}
	return nil
}

// Create validates and computes the snapshot and BRIEF before writing. It
// archives the previous BRIEF without replacing an existing archive, appends
// the round event before the snapshot, then writes the new BRIEF. These writes
// are a single logical operation, not a crash-atomic multi-file transaction.
// A retry can repair a stale snapshot or BRIEF; verdict files are never changed.
func Create(dir string, in CreateInput) error {
	beadID := ""
	if in.BeadID != nil {
		beadID = *in.BeadID
	}
	binding, err := CreateBeadID(in.Gate, in.Target, beadID)
	if err != nil {
		return err
	}
	in.BeadID = binding
	var old Panel
	var telemetryPresent bool
	if _, err := os.Stat(filepath.Join(dir, FileName)); err == nil {
		reg := load(dir)
		if reg.Err != nil {
			return reg.Err
		}
		old = reg.Panel
		data, err := os.ReadFile(filepath.Join(dir, FileName))
		if err != nil {
			return err
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			return err
		}
		_, telemetryPresent = fields["telemetry"]
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read panel.json: %w", err)
	}
	if err := CheckCreateRound(dir, in.Round); err != nil {
		return err
	}
	lines, err := ReadLedger(dir)
	if err != nil {
		return err
	}
	var first, latest *RoundRecord
	backed := CarriedEntries{}
	abandonedAt := ""
	for _, line := range lines {
		var kind recordDiscriminator
		if err := json.Unmarshal(line, &kind); err != nil {
			return err
		}
		switch kind.Record {
		case RecordRound:
			var r RoundRecord
			if err := json.Unmarshal(line, &r); err != nil {
				return err
			}
			if first == nil {
				first = &r
				backed = r.Carried
			}
			latest = &r
			abandonedAt = ""
		case RecordRefutation:
			var r Refutation
			if err := json.Unmarshal(line, &r); err != nil {
				return err
			}
			backed.Refutations = append(backed.Refutations, r)
		case RecordDeferral:
			var d Deferral
			if err := json.Unmarshal(line, &d); err != nil {
				return err
			}
			backed.Deferrals = append(backed.Deferrals, d)
		case RecordRebind:
			var r Rebind
			if err := json.Unmarshal(line, &r); err != nil {
				return err
			}
			backed.Rebinds = append(backed.Rebinds, r)
		case RecordAbandon:
			var a AbandonRecord
			if err := json.Unmarshal(line, &a); err != nil {
				return err
			}
			abandonedAt = a.CreatedAt
		}
	}
	// A remaining marker still denotes a non-legacy registration. Create is the
	// recovery for hand edits, including removal of one of the markers.
	legacy := latest == nil && !telemetryPresent
	if legacy {
		checkEntry := func(entry interface{}, name string) error {
			data, err := json.Marshal(entry)
			if err != nil {
				return err
			}
			if err := HygienePredicate(data); err != nil {
				return &CreateLegacyPathError{Entry: name}
			}
			return nil
		}
		for i, r := range old.Refutations {
			if err := checkEntry(r, fmt.Sprintf("refutation[%d] slot %s finding %s", i, r.Slot, r.Finding)); err != nil {
				return err
			}
		}
		for i, d := range old.Deferrals {
			if err := checkEntry(d, fmt.Sprintf("deferral[%d] slot %s finding %s", i, d.Slot, d.Finding)); err != nil {
				return err
			}
		}
		for i, r := range old.Rebinds {
			if err := checkEntry(r, fmt.Sprintf("rebind[%d] round %d from %s to %s", i, r.Round, r.FromSHA, r.ToSHA)); err != nil {
				return err
			}
		}
	}
	carried := CarriedEntries{}
	var dropped []string
	keep := func(ok bool, name string) bool {
		if legacy || ok {
			return true
		}
		dropped = append(dropped, name)
		return false
	}
	for i, r := range old.Refutations {
		if keep(matchesRefutation(r, backed.Refutations), fmt.Sprintf("refutations[%d] slot %s round %d finding %s", i, r.Slot, r.Round, r.Finding)) {
			carried.Refutations = append(carried.Refutations, r)
		}
	}
	for i, d := range old.Deferrals {
		if keep(matchesDeferral(d, backed.Deferrals), fmt.Sprintf("deferrals[%d] slot %s round %d finding %s bead %s", i, d.Slot, d.Round, d.Finding, d.Bead)) {
			carried.Deferrals = append(carried.Deferrals, d)
		}
	}
	for i, r := range old.Rebinds {
		if keep(matchesRebind(r, backed.Rebinds), fmt.Sprintf("rebinds[%d] round %d from %s to %s", i, r.Round, r.FromSHA, r.ToSHA)) {
			carried.Rebinds = append(carried.Rebinds, r)
		}
	}
	roster := in.Roster
	if !in.ReviewersOverride {
		if latest != nil {
			roster = latest.Roster
		} else if legacy && len(old.Roster) > 0 {
			roster = old.Roster
		}
	}
	if len(roster) < 2 {
		return fmt.Errorf("a panel needs at least two reviewers; use --reviewers 2")
	}
	gate := in.Gate
	p := Panel{BeadID: in.BeadID, Spec: in.Spec, Target: in.Target, Round: in.Round,
		ExpectedReviewers: len(roster), Roster: roster, ReviewedHeadSHA: in.ReviewedHeadSHA,
		SubjectDigest: in.SubjectDigest, Gate: gate, Telemetry: "required",
		Refutations: carried.Refutations, Deferrals: carried.Deferrals, Rebinds: carried.Rebinds,
		AbandonHistory: old.AbandonHistory}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if old.Abandoned {
		p.AbandonHistory = append(p.AbandonHistory, AbandonHistory{Reason: old.AbandonReason, AbandonedAt: abandonedAt, RevivedAt: now})
	}
	baseline := CarriedEntries{}
	if legacy {
		baseline = carried
	}
	spec := in.Spec
	if spec == "" {
		spec = "adhoc"
	}
	event := RoundRecord{Record: RecordRound, Spec: spec, Panel: filepath.Base(dir), Round: p.Round,
		ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target,
		Gate: p.Gate, BeadID: p.BeadID, Roster: p.Roster, ExpectedReviewers: p.ExpectedReviewers,
		Telemetry: p.Telemetry, Carried: baseline, CreatedAt: now}
	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err := Validate(eventData); err != nil {
		return err
	}
	if err := HygienePredicate(eventData); err != nil {
		return err
	}
	briefPath := filepath.Join(dir, "BRIEF.md")
	existing, readErr := os.ReadFile(briefPath)
	briefExists := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read %s: %w", briefPath, readErr)
	}
	prior := "none recorded"
	priorData, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("consolidated-round-%d.md", in.Round-1)))
	if err == nil {
		prior = termsafe.Escape(string(priorData))
		// Quoted prose must not become region syntax or toggle fence scanning.
		trimmed := strings.TrimSpace(prior)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") || markerLookalikeRE.MatchString(trimmed) ||
			trimmed == "<!-- mindspec:prior-round -->" || trimmed == "<!-- /mindspec:prior-round -->" {
			prior = "> " + prior
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read prior round: %w", err)
	}
	header := renderCreateBriefHeader(filepath.Base(dir), p, prior, in.BriefRegions)
	var newBrief string
	if briefExists {
		newBrief, err = spliceBriefHeader(string(existing), header)
		if err != nil {
			return fmt.Errorf("BRIEF.md header in %s: %w", briefPath, err)
		}
	} else {
		newBrief = header + "\n\n" + briefStubBody
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if briefExists && old.Round > 0 && in.Round > old.Round {
		archive := filepath.Join(dir, fmt.Sprintf("BRIEF-round-%d.md", old.Round))
		f, err := os.OpenFile(archive, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("archive BRIEF: %w", err)
		}
		if err == nil {
			_, writeErr := f.Write(existing)
			closeErr := f.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	if err := AppendRecords(dir, eventData); err != nil {
		return err
	}
	if err := WriteSnapshot(dir, p); err != nil {
		return err
	}
	for _, name := range dropped {
		if in.ReportDropped != nil {
			in.ReportDropped("dropped unbacked " + termsafe.Escape(name))
		}
	}
	if err := os.WriteFile(briefPath, []byte(newBrief), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", briefPath, err)
	}
	return nil
}

func renderCreateBriefHeader(slug string, p Panel, prior string, regions BriefRegions) string {
	header := renderBriefHeader(slug, p.Round, p.Target, p.ReviewedHeadSHA)
	header = renderBriefRegions(header, regions)
	var b strings.Builder
	b.WriteString("## Reviewer roster\n\n")
	for _, slot := range p.Roster {
		fmt.Fprintf(&b, "%s %s %s\n", slot.Slot, termsafe.Escape(slot.Model), termsafe.Escape(slot.Lens))
	}
	b.WriteString("\n<!-- mindspec:prior-round -->\n")
	b.WriteString(prior)
	b.WriteString("\n<!-- /mindspec:prior-round -->\n")
	return strings.TrimSuffix(header, briefHeaderClose) + b.String() + briefHeaderClose
}

// CreateLegacyPathError identifies a legacy entry that cannot enter the ledger.
// Callers render Recovery with their normal failure format.
type CreateLegacyPathError struct{ Entry string }

func (e *CreateLegacyPathError) Error() string {
	return "cannot carry legacy " + termsafe.Escape(e.Entry) + " with an absolute path"
}

func (e *CreateLegacyPathError) Recovery() string {
	return "remove the absolute path from " + termsafe.Escape(e.Entry) + " in the legacy panel.json, then run create again"
}

// CreateRoundConflict lets callers render a recovery using the resolved binding.
type CreateRoundConflict struct{ Round, LatestRound int }

func (e *CreateRoundConflict) Error() string {
	return fmt.Sprintf("cannot create round %d: verdict files already reach round %d", e.Round, e.LatestRound)
}

// CreateBeadID derives and validates the binding before callers resolve git or
// write files. An omitted gate is non-bead, like every gate other than bead.
func CreateBeadID(gate, target, beadID string) (*string, error) {
	if gate != "bead" {
		if beadID != "" {
			return nil, fmt.Errorf("--bead requires --gate bead")
		}
		return nil, nil
	}
	if !strings.HasPrefix(target, "bead/") {
		return nil, fmt.Errorf("--gate bead requires --target bead/<id>")
	}
	derived := strings.TrimPrefix(target, "bead/")
	if err := idvalidate.BeadID(derived); err != nil {
		return nil, fmt.Errorf("invalid bead target: %w", err)
	}
	if beadID != "" && beadID != derived {
		return nil, fmt.Errorf("--bead %q does not match --target %q", beadID, target)
	}
	return &derived, nil
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
	b.WriteString("(the panel artifact schema). Required and optional top-level fields:\n\n")
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

// scanNonFenceLines visits lines outside code fences in file order.
// A shorter fence cannot close a longer fence. The offset is in bytes.
func scanNonFenceLines(s string, fn func(offset int, content string)) {
	scanMarkdownLines(s, fn)
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
