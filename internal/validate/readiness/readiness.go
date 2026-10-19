package readiness

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// --- bd-less hermeticity seam (spec 124 plan-gate G2-BDLESS-ENGINE-SEAM) ---
//
// The two package-level func vars below are the COMPLETE bd/lineage
// read-set EvaluateReadiness performs. Every one of MF-1..MF-4 reaches
// internal/bead only through these vars, so engine unit tests can swap
// both to deterministic in-memory fixtures and consult no real bd
// process — hermetic under `bd` absent from PATH, never a t.Skip (spec
// 124 plan-gate F3-1, the spec-119 lesson). MF-3's landed-merge leg
// (internal/lifecycle.FindLandedMerge) is a GIT read, not bd, and is
// exercised over real temp git repos in tests, not faked.
var (
	// findEpicForBeadFn resolves beadID's owning epic + spec via bd/phase
	// lineage (the internal/complete/complete.go:369 fail-closed pattern
	// — NEVER cwd). A real lookup error propagates as-is; this package
	// never degrades to a cwd-derived resolution.
	findEpicForBeadFn = phase.FindEpicForBead

	// fetchBeadRecordFn reads a bead's OWN bd record: its description
	// (MF-2's harvest context and MF-4's description scan) and its
	// "blocks" dependency edges, each carrying the dependency's own bd
	// status (MF-3). One `bd show <id> --json` read serves both needs.
	fetchBeadRecordFn = fetchBeadRecordReal

	// findLandedMergeFn is MF-3's landed-merge decision seam (FX-1). It
	// defaults to lifecycle.FindLandedMerge, which is a GIT read but ALSO
	// performs a TRANSITIVE bd read: FindLandedMerge -> landedBindingForBead
	// -> bead.GetMetadata (internal/lifecycle/landed.go), the merge-time
	// landed-binding corroboration leg. That transitive bd read is not
	// covered by the fetchBeadRecordFn/findEpicForBeadFn seams above, so
	// per the plan's F3-1 "route EVERY bd read through the injectable seam"
	// requirement the WHOLE landed-merge decision is seamed HERE — the
	// AC-3 real-repo tests keep the real function (git+bd both exercised
	// end-to-end over real temp repos), while a bd-less test that must
	// exercise the metadata-corroborated landed path swaps this var to a
	// deterministic in-memory return, consulting no real bd.
	findLandedMergeFn = lifecycle.FindLandedMerge
)

// dependencyEdge is one "blocks" dependency edge read off a bead's bd
// record: the dependency's bead ID and its own bd status.
type dependencyEdge struct {
	ID     string
	Status string
}

// beadRecord is the subset of a bead's bd record EvaluateReadiness needs.
type beadRecord struct {
	Description  string
	Dependencies []dependencyEdge
}

// fetchBeadRecordReal is the real, bd-backed default for fetchBeadRecordFn.
func fetchBeadRecordReal(beadID string) (*beadRecord, error) {
	// Class-2 consumer boundary (ADR-0042 Section1): beadID feeds a `bd
	// show` argv build directly — validated before any bd spawn.
	if err := idvalidate.BeadID(beadID); err != nil {
		return nil, fmt.Errorf("invalid bead id %s: %w", idrender.Bead(beadID), err)
	}
	out, err := bead.RunBD("show", beadID, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd show %s failed: %w", idrender.Bead(beadID), err)
	}
	var items []struct {
		Description  string `json:"description"`
		Dependencies []struct {
			ID             string `json:"id"`
			Status         string `json:"status"`
			DependencyType string `json:"dependency_type"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parsing bd show %s output: %w", idrender.Bead(beadID), err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("bead %s not found", idrender.Bead(beadID))
	}
	rec := &beadRecord{Description: items[0].Description}
	for _, d := range items[0].Dependencies {
		if strings.EqualFold(d.DependencyType, "blocks") {
			rec.Dependencies = append(rec.Dependencies, dependencyEdge{ID: d.ID, Status: d.Status})
		}
	}
	return rec, nil
}

// EvaluateReadiness evaluates the four mechanical-floor signals (MF-1..
// MF-4, spec 124 R2) for beadID and returns the resulting Report. It never
// mutates anything: no bd write, no git write, no file write on any path
// (spec 124 R1).
//
// Owning-spec resolution is LINEAGE-authoritative (bead -> epic -> spec via
// findEpicForBeadFn), never cwd-derived — a real lookup error refuses here
// rather than degrading to a cwd fallback.
func EvaluateReadiness(root, beadID string) (*Report, error) {
	if err := idvalidate.BeadID(beadID); err != nil {
		return nil, fmt.Errorf("invalid bead id %s: %w", idrender.Bead(beadID), err)
	}

	epicID, specID, err := findEpicForBeadFn(beadID)
	if err != nil {
		return nil, fmt.Errorf("resolving owning spec for %s: %w", idrender.Bead(beadID), err)
	}
	if specID == "" {
		return nil, fmt.Errorf("bead %s has no discoverable owning spec lineage", idrender.Bead(beadID))
	}

	n, err := beadIndex(beadID)
	if err != nil {
		return nil, &NonPlanFormError{BeadID: beadID, EpicID: epicID, SpecID: specID}
	}

	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return nil, fmt.Errorf("resolving spec dir for %s: %w", idrender.Spec(specID), err)
	}

	planContent, err := os.ReadFile(filepath.Join(specDir, "plan.md"))
	if err != nil {
		return nil, fmt.Errorf("reading plan.md for spec %s: %w", idrender.Spec(specID), err)
	}
	specContent, err := os.ReadFile(filepath.Join(specDir, "spec.md"))
	if err != nil {
		return nil, fmt.Errorf("reading spec.md for spec %s: %w", idrender.Spec(specID), err)
	}

	_, err = validate.ParsePlanFrontmatter(string(planContent))
	if err != nil {
		return nil, fmt.Errorf("parsing plan frontmatter for spec %s: %w", idrender.Spec(specID), err)
	}

	rec, err := fetchBeadRecordFn(beadID)
	if err != nil {
		return nil, fmt.Errorf("reading bd record for %s: %w", idrender.Bead(beadID), err)
	}

	sectionRaw, _ := extractBeadSectionRaw(string(planContent), n)

	ownSpecNum := leadingSpecNumber(specID)

	report := &Report{BeadID: beadID, EpicID: epicID, SpecID: specID}
	report.Signals = append(report.Signals, evaluateMF1(string(planContent), n))
	report.Signals = append(report.Signals, evaluateMF2(rec.Description, sectionRaw, string(specContent), ownSpecNum))
	report.Signals = append(report.Signals, evaluateMF3(root, specID, rec.Dependencies))
	report.Signals = append(report.Signals, evaluateMF4(rec.Description, sectionRaw))
	return report, nil
}

// beadIndexRe extracts the trailing dotted epic-child index (e.g. ".1" in
// "mindspec-8nhe.1") — the bead's N, mapped positionally to
// work_chunks[N-1] / the Nth "## Bead" section (spec 097 R3).
var beadIndexRe = regexp.MustCompile(`\.([0-9]+)$`)

// NonPlanFormError carries resolved lineage when a bead has no usable dotted
// index. It preserves beadIndex's diagnostic; claim callers can use the typed
// lineage before attempting any plan/file lookup (spec 129 R1.8).
type NonPlanFormError struct {
	// BeadID is the bead whose dotted index could not be parsed.
	BeadID string
	// EpicID is its resolved lineage epic.
	EpicID string
	// SpecID is its resolved lineage spec.
	SpecID string
}

// Error returns the existing dotted-index diagnostic byte for byte.
func (e *NonPlanFormError) Error() string {
	_, err := beadIndex(e.BeadID)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("bead %s has no dotted epic-child index (expected <epic>.<N>)", idrender.Bead(e.BeadID))
}

func beadIndex(beadID string) (int, error) {
	m := beadIndexRe.FindStringSubmatch(beadID)
	if m == nil {
		return 0, fmt.Errorf("bead %s has no dotted epic-child index (expected <epic>.<N>)", idrender.Bead(beadID))
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 {
		return 0, fmt.Errorf("bead %s has an invalid epic-child index", idrender.Bead(beadID))
	}
	return n, nil
}

// leadingSpecNumber parses the leading digit run of specID (e.g. 124 from
// "124-impl-readiness-gate") — MF-2's foreign-citation exclusion needs this
// to tell "the spec 123 AC-17 pattern" (a citation of a DIFFERENT spec)
// from a genuine same-spec reference.
func leadingSpecNumber(specID string) int {
	i := 0
	for i < len(specID) && specID[i] >= '0' && specID[i] <= '9' {
		i++
	}
	n, _ := strconv.Atoi(specID[:i])
	return n
}

// --- MF-1: plan section concrete-by-structure (spec 124 R2 MF-1) ---

const scaffoldACPlaceholder = "<Specific, measurable criterion for this bead>"
const scaffoldKeyFilePathPlaceholder = "path/to/file.go"

// PlanMapsBead reports whether the Nth positional Bead heading and a
// work_chunks id N both exist. Like extractBeadSectionRaw it counts every
// line beginning "## Bead ", even inside a fence, ignoring printed numbers.
// Content checks (ACs and paths) do not affect mapping. Invalid frontmatter
// maps nothing; callers that need read-error classification parse it first.
func PlanMapsBead(planContent string, n int) (mapped bool, why string) {
	if _, found := extractBeadSectionRaw(planContent, n); !found {
		count := 0
		for _, line := range strings.Split(planContent, "\n") {
			if strings.HasPrefix(line, "## Bead ") {
				count++
			}
		}
		return false, fmt.Sprintf("plan.md declares %d ## Bead sections", count)
	}
	fm, err := validate.ParsePlanFrontmatter(planContent)
	if err != nil {
		return false, "invalid plan.md frontmatter"
	}
	for _, chunk := range fm.WorkChunks {
		if chunk.ID == n {
			return true, ""
		}
	}
	return false, fmt.Sprintf("no work_chunks id %d", n)
}

func evaluateMF1(planContent string, n int) Signal {
	mapped, _ := PlanMapsBead(planContent, n)
	sectionRaw, sectionFound := extractBeadSectionRaw(planContent, n)
	fm, _ := validate.ParsePlanFrontmatter(planContent) // EvaluateReadiness validated it.
	var chunk *validate.WorkChunk
	if fm != nil {
		for i := range fm.WorkChunks {
			if fm.WorkChunks[i].ID == n {
				chunk = &fm.WorkChunks[i]
				break
			}
		}
	}
	s := Signal{ID: SignalPlanSection, Pass: true}
	var details []string
	add := func(key, detail, recovery string) {
		s.Pass = false
		s.Failures = append(s.Failures, fmt.Sprintf("%s:%d", key, n))
		details = append(details, detail)
		if s.Recovery == "" {
			s.Recovery = recovery
		}
	}
	if !mapped && !sectionFound {
		add("section-missing", fmt.Sprintf(`plan.md has no "## Bead %d" section`, n),
			fmt.Sprintf(`add a "## Bead %d" section to plan.md with a concrete Acceptance Criteria entry`, n))
	}
	acEntries, _ := extractNamedBlock(sectionRaw, "**Acceptance Criteria**")
	if sectionFound && !hasConcreteACEntry(acEntries) {
		add("ac-entry-missing", fmt.Sprintf(`"## Bead %d"'s Acceptance Criteria block has no entry beyond the scaffold placeholder %q`, n, scaffoldACPlaceholder),
			fmt.Sprintf(`edit plan.md's "## Bead %d" Acceptance Criteria block: replace the placeholder with a concrete, measurable criterion`, n))
	}
	if !mapped && chunk == nil {
		add("chunk-missing", fmt.Sprintf("plan.md frontmatter has no work_chunks entry with id: %d", n),
			fmt.Sprintf("add a work_chunks entry with id: %d and non-empty key_file_paths to plan.md's frontmatter", n))
	}
	if chunk != nil && len(chunk.KeyFilePaths) == 0 {
		add("key-file-paths-empty", fmt.Sprintf("work_chunks[%d].key_file_paths is empty", n),
			fmt.Sprintf("declare the files this bead touches in work_chunks id: %d's key_file_paths", n))
	}
	if chunk != nil {
		hasPlaceholder, hasBlank, hasConcrete := false, false, false
		for _, p := range chunk.KeyFilePaths {
			t := strings.TrimSpace(p)
			hasPlaceholder = hasPlaceholder || t == scaffoldKeyFilePathPlaceholder
			hasBlank = hasBlank || t == ""
			hasConcrete = hasConcrete || (t != "" && t != scaffoldKeyFilePathPlaceholder)
		}
		if hasPlaceholder {
			add("key-file-paths-placeholder", fmt.Sprintf("work_chunks[%d].key_file_paths still carries the scaffold placeholder %q", n, scaffoldKeyFilePathPlaceholder),
				fmt.Sprintf("replace the placeholder path in work_chunks id: %d's key_file_paths with real file paths", n))
		}
		if hasBlank {
			detail := fmt.Sprintf("work_chunks[%d].key_file_paths has no non-blank concrete path element", n)
			if hasConcrete {
				detail = fmt.Sprintf("work_chunks[%d].key_file_paths carries a blank path element", n)
			}
			add("key-file-paths-blank", detail,
				fmt.Sprintf("declare the files this bead touches in work_chunks id: %d's key_file_paths (blank entries do not count)", n))
		}
	}
	s.Failures = sortedUnique(s.Failures)
	s.Detail = strings.Join(details, "; ")
	return s
}

var boldHeaderRe = regexp.MustCompile(`^\*\*[^*]+\*\*$`)

// extractBeadSectionRaw returns the raw text of the Nth "## Bead " section
// in content (the heading line through, but not including, the next H2
// heading), and whether such a section exists.
func extractBeadSectionRaw(content string, n int) (string, bool) {
	lines := strings.Split(content, "\n")
	var buf []string
	count := 0
	capturing := false
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## Bead ") {
			if capturing {
				break
			}
			count++
			if count == n {
				capturing = true
				found = true
			}
			if capturing {
				buf = append(buf, line)
			}
			continue
		}
		if capturing && strings.HasPrefix(line, "## ") {
			break
		}
		if capturing {
			buf = append(buf, line)
		}
	}
	return strings.Join(buf, "\n"), found
}

// extractNamedBlock returns the "- " list-item lines immediately following
// a bold header line matching headerText (e.g. "**Acceptance Criteria**"),
// stopping at the next bold header or H2/H3 heading.
func extractNamedBlock(sectionRaw, headerText string) ([]string, bool) {
	lines := strings.Split(sectionRaw, "\n")
	idx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == headerText {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, false
	}
	var out []string
	for _, l := range lines[idx+1:] {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") || boldHeaderRe.MatchString(trimmed) {
			break
		}
		if strings.HasPrefix(trimmed, "-") {
			out = append(out, trimmed)
		}
	}
	return out, true
}

func stripListMarker(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "-")
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[ ]") || strings.HasPrefix(s, "[x]") || strings.HasPrefix(s, "[X]") {
		s = strings.TrimSpace(s[3:])
	}
	return s
}

func hasConcreteACEntry(entries []string) bool {
	for _, e := range entries {
		stripped := stripListMarker(e)
		if stripped != "" && stripped != scaffoldACPlaceholder {
			return true
		}
	}
	return false
}

// --- MF-2: claimed Rs/ACs resolve, by an exact rule (spec 124 R2 MF-2) ---

// claimTokenRe nominates a base, with an optional appended letter. Both
// bead harvesting and spec indexing use extractLineClaims and claimForms.
var claimTokenRe = regexp.MustCompile(`\b(R|AC-?)([0-9]+)([a-z])?\b`)

// claimForms is the shared form table, ordered most specific first. AC<n>
// normalizes to AC-<n> before these suffix rules. Dotted sub-numbers (R6.3)
// deliberately resolve at base granularity on both sides (spec 129 OQ-6).
var claimForms = []struct {
	name      string
	suffix    *regexp.Regexp
	subLetter bool
}{
	{"Roman parenthetical degrades to base", regexp.MustCompile(`^\((x{0,3}(?:ix|iv|v?i{0,3}))\)`), false},
	{"parenthetical sub-letter", regexp.MustCompile(`^\(([a-z])\)`), true},
	{"appended sub-letter", regexp.MustCompile(`^([a-z])\b`), true},
	{"bare base (including dotted sub-number)", regexp.MustCompile(`^`), false},
}

// foreignSpecRe matches a "spec <digits>" / "Spec <digits>" citation —
// MF-2's foreign-citation exclusion: a token on a line naming a DIFFERENT
// spec number is a citation, never a claim.
var foreignSpecRe = regexp.MustCompile(`(?i)\bspec\s+([0-9]+)\b`)

type claimToken struct {
	base        string
	letter      string
	subLettered bool
	raw         string
	// start/end are the token's byte-offset span within its source line
	// (raw's span, including any recognized parenthetical) — the
	// adjacency input for the foreign-citation SPAN exclusion below.
	start int
	end   int
}

// harvestClaims extracts the bead's OWN claimed R<n>/AC-<n> tokens from
// text (bd description or plan section), applying the three plan-level
// harvest rules: code-span/fence exclusion, foreign-citation SPAN
// exclusion, and clause-enumerator normalization.
func harvestClaims(text string, ownSpecNum int) []claimToken {
	stripped := stripCodeSpans(text)
	var out []claimToken
	for _, line := range strings.Split(stripped, "\n") {
		out = append(out, harvestLineClaims(line, ownSpecNum)...)
	}
	return out
}

type citationSpan struct{ start, end int }

// foreignCitationSpans returns the byte-offset spans of every foreign-spec
// citation ("spec <digits>" where <digits> differs from the owning spec's
// number) on line.
func foreignCitationSpans(line string, ownSpecNum int) []citationSpan {
	var spans []citationSpan
	for _, m := range foreignSpecRe.FindAllStringSubmatchIndex(line, -1) {
		n, err := strconv.Atoi(line[m[2]:m[3]])
		if err == nil && n != ownSpecNum {
			spans = append(spans, citationSpan{start: m[0], end: m[1]})
		}
	}
	return spans
}

// citationGapRe: the characters that may separate a foreign-spec citation
// from the claim tokens belonging to that citation — whitespace and the
// token-chain separators `/`, `+`, `&` ("spec 097 R3/R4"). Clause
// punctuation (`,`, `;`, `:`, `.`, parens, dashes) deliberately BREAKS
// adjacency: a token in a different clause of the same line is the bead's
// own claim, not part of the citation.
var citationGapRe = regexp.MustCompile(`^[ \t/+&]*$`)

// harvestLineClaims extracts one line's claim tokens, excluding ONLY the
// tokens belonging to a foreign-spec citation's own span — the citation
// itself plus the chain of claim tokens strictly adjacent to it
// (separated by citationGapRe characters only), extended in both
// directions. Final-review r1 G1-MF2-MIXED-CITATION-LINE: the previous
// WHOLE-LINE exclusion dropped every token on a mixed line, so a dangling
// owning-spec token sharing a line with a benign foreign citation
// ("Preserve the spec 123 AC-17 pattern while satisfying AC-999") was
// never harvested — a false NEGATIVE the fixture pair now pins closed.
// "the spec 123 AC-17 pattern" still excludes AC-17 (adjacent to the
// citation); AC-999 elsewhere on the line is harvested and resolved.
func harvestLineClaims(line string, ownSpecNum int) []claimToken {
	tokens := extractLineClaims(line)
	spans := foreignCitationSpans(line, ownSpecNum)
	if len(spans) == 0 || len(tokens) == 0 {
		return tokens
	}

	adjacentGap := func(from, to int) bool {
		return from <= to && citationGapRe.MatchString(line[from:to])
	}
	excluded := make([]bool, len(tokens))
	// Forward pass (tokens arrive in left-to-right order): a token
	// directly after a citation span — or after an already-excluded
	// token — across a separator-only gap belongs to the citation.
	for i, tok := range tokens {
		for _, sp := range spans {
			if adjacentGap(sp.end, tok.start) {
				excluded[i] = true
			}
		}
		if !excluded[i] && i > 0 && excluded[i-1] && adjacentGap(tokens[i-1].end, tok.start) {
			excluded[i] = true
		}
	}
	// Backward pass: the mirror-image chain for tokens STRICTLY adjacent
	// before the citation ("AC-17 spec 123" shapes) — connector words
	// ("of", "per") deliberately break adjacency, keeping the exclusion
	// narrow: a doubtful token is harvested (visible, fixable) rather
	// than silently dropped.
	for i := len(tokens) - 1; i >= 0; i-- {
		if excluded[i] {
			continue
		}
		tok := tokens[i]
		for _, sp := range spans {
			if adjacentGap(tok.end, sp.start) {
				excluded[i] = true
			}
		}
		if !excluded[i] && i < len(tokens)-1 && excluded[i+1] && adjacentGap(tok.end, tokens[i+1].start) {
			excluded[i] = true
		}
	}

	out := make([]claimToken, 0, len(tokens))
	for i, tok := range tokens {
		if !excluded[i] {
			out = append(out, tok)
		}
	}
	return out
}

func extractLineClaims(line string) []claimToken {
	var out []claimToken
	for _, m := range claimTokenRe.FindAllStringSubmatchIndex(line, -1) {
		prefix := line[m[2]:m[3]]
		if prefix == "AC" {
			prefix = "AC-"
		}
		base := prefix + line[m[4]:m[5]]
		for _, form := range claimForms {
			match := form.suffix.FindStringSubmatchIndex(line[m[5]:])
			if match == nil {
				continue
			}
			// An empty () is not a Roman enumerator.
			if len(match) > 2 && match[2] == match[3] {
				continue
			}
			end := m[5] + match[1]
			c := claimToken{base: base, raw: line[m[0]:end], start: m[0], end: end}
			if form.subLetter {
				c.letter = line[m[5]+match[2] : m[5]+match[3]]
				c.subLettered = true
			}
			out = append(out, c)
			break
		}
	}
	return out
}

// extractSpecTokens scans specContent for every R<n>/AC-<n> occurrence
// (bare or sub-lettered), returning the set of distinct base strings seen
// (for letterless-claim resolution: base equality only — "AC-1" and
// "AC-19" are distinct bases by construction, so a numeric-prefix claim
// never resolves via a longer number) and the set of exact base+letter
// strings seen (for sub-lettered-claim exact resolution).
func extractSpecTokens(specContent string) (bases map[string]bool, exact map[string]bool) {
	bases = map[string]bool{}
	exact = map[string]bool{}
	for _, c := range extractLineClaims(specContent) {
		bases[c.base] = true
		if c.subLettered {
			exact[c.base+c.letter] = true
		}
	}
	return bases, exact
}

func evaluateMF2(description, sectionRaw, specContent string, ownSpecNum int) Signal {
	claims := harvestClaims(description, ownSpecNum)
	claims = append(claims, harvestClaims(sectionRaw, ownSpecNum)...)

	specBases, specExact := extractSpecTokens(specContent)

	seen := map[string]bool{}
	var dangling []claimToken
	for _, c := range claims {
		key := c.base + "\x00" + c.letter
		if seen[key] {
			continue
		}
		seen[key] = true
		if c.subLettered {
			if !specExact[c.base+c.letter] {
				dangling = append(dangling, c)
			}
		} else if !specBases[c.base] {
			dangling = append(dangling, c)
		}
	}
	if len(dangling) > 0 {
		names := make([]string, 0, len(dangling))
		var failures []string
		for _, d := range dangling {
			names = append(names, d.raw)
			failures = append(failures, "dangling:"+d.base+d.letter)
		}
		sort.Strings(names)
		joined := strings.Join(names, ", ")
		return Signal{
			ID:       SignalTokens,
			Failures: sortedUnique(failures),
			Pass:     false,
			Detail:   fmt.Sprintf("claimed token(s) do not resolve in spec.md: %s", joined),
			Recovery: fmt.Sprintf("resolve the dangling claim(s) %s against spec.md, or remove them from the bead's plan section / bd description", joined),
		}
	}
	return Signal{ID: SignalTokens, Pass: true}
}

// --- MF-3: dependencies closed AND landed-merged (spec 124 R2 MF-3) ---

func evaluateMF3(root, specID string, deps []dependencyEdge) Signal {
	if len(deps) == 0 {
		return Signal{ID: SignalDependencies, Pass: true}
	}
	specBranch, err := workspace.SpecBranch(specID)
	if err != nil {
		return Signal{
			ID:       SignalDependencies,
			Pass:     false,
			Detail:   fmt.Sprintf("could not resolve the spec branch for %s: %v", idrender.Spec(specID), err),
			Recovery: "fix the spec ID, then re-run `mindspec bead ready-check`",
		}
	}
	for _, dep := range deps {
		if err := idvalidate.BeadID(dep.ID); err != nil {
			return Signal{
				ID:       SignalDependencies,
				Pass:     false,
				Detail:   fmt.Sprintf("dependency id %s is malformed", idrender.Bead(dep.ID)),
				Recovery: fmt.Sprintf("fix the malformed dependency edge naming %s", idrender.Bead(dep.ID)),
			}
		}
		if !strings.EqualFold(strings.TrimSpace(dep.Status), "closed") {
			return Signal{
				ID:       SignalDependencies,
				Pass:     false,
				Detail:   fmt.Sprintf("dependency %s is not closed in bd (status=%s)", dep.ID, dep.Status),
				Recovery: fmt.Sprintf("mindspec complete %s", dep.ID),
			}
		}
		_, lmErr := findLandedMergeFn(root, specBranch, dep.ID)
		if lmErr == nil {
			continue
		}
		var noEvidence *lifecycle.LandedMergeNoEvidence
		switch {
		case errors.As(lmErr, &noEvidence):
			// Corroboration-unavailable (*lifecycle.LandedMergeNoEvidence):
			// a candidate merge EXISTS on the spec branch but no admissible
			// datum confirms it belongs to this bead (or owned candidates
			// conflict). The dependency is already complete, so `mindspec
			// complete` is the WRONG verb here; `mindspec reattest` is the
			// spec-125 verb built for exactly this state — it independently
			// git-corroborates the candidate merge and writes the missing
			// landed binding (final-review r2 G2-R2-MF3 / F2-1).
			return Signal{
				ID:       SignalDependencies,
				Pass:     false,
				Detail:   fmt.Sprintf("dependency %s: a candidate merge exists but no admissible datum corroborates it (%v)", dep.ID, lmErr),
				Recovery: fmt.Sprintf("verify the candidate merge, then run `mindspec reattest %s` to write the git-corroborated landed binding, and re-run `mindspec bead ready-check`", dep.ID),
			}
		case errors.Is(lmErr, lifecycle.ErrLandedMergeNotFound):
			// No candidate merge NAMES bead/<id> at all. Usually this is the
			// genuine closed-but-unmerged 2u0u split (recovery: `mindspec
			// complete`), but it is NOT proof the work never landed: spec
			// 125's FindLandedMerge also returns this sentinel for a REAL
			// merge whose subject names no bead (an anonymous hand-crafted
			// operator merge is never automatically identifiable, and
			// reattest's ownership nominator is the same subject parse — see
			// internal/lifecycle/landed.go). State the two possibilities
			// conservatively instead of asserting "not merged" as fact
			// (final-review r2 G2-R2-MF3-ANONYMOUS-CLASSIFICATION).
			return Signal{
				ID:       SignalDependencies,
				Pass:     false,
				Detail:   fmt.Sprintf("dependency %s is closed but no landed merge into %s names bead/%s — either it was never merged (the 2u0u split), or its merge subject does not name the bead", dep.ID, specBranch, dep.ID),
				Recovery: fmt.Sprintf("if %s was never merged, run `mindspec complete %s`; if a real merge exists under an anonymous subject, re-merge under a bead-naming subject or use the audited attested-restore (`mindspec reattest` cannot adopt an anonymous merge)", dep.ID, dep.ID),
			}
		default:
			return Signal{
				ID:       SignalDependencies,
				Pass:     false,
				Detail:   fmt.Sprintf("dependency %s: could not evaluate landed-merge state: %v", dep.ID, lmErr),
				Recovery: fmt.Sprintf("investigate the landed-merge evaluation failure for %s, then re-run `mindspec bead ready-check`", dep.ID),
			}
		}
	}
	return Signal{ID: SignalDependencies, Pass: true}
}

// --- MF-4: no genuine blocking marker (spec 124 R2 MF-4) ---

var fencedCodeBlockRe = regexp.MustCompile("(?s)```.*?```")

// stripCodeSpans blanks out fenced code blocks and inline code spans,
// preserving line breaks (so line-based scans downstream keep their line
// numbers/boundaries) and overall byte layout, so a token appearing only
// inside a code span/fence is invisible to the token and blocking-region
// scans (spec 124 R2 MF-2/MF-4's shared code-span exclusion rule).
func stripCodeSpans(s string) string {
	s = fencedCodeBlockRe.ReplaceAllStringFunc(s, blankPreservingNewlines)
	s = stripInlineCodeSpans(s)
	return s
}

// stripInlineCodeSpans blanks CommonMark inline code spans of ANY backtick
// run length (FX-1 regression: the prior single-backtick regex left the
// PAYLOAD of a multi-backtick span — e.g. a double-backtick “ “code“ “
// — visible, so a TBD / AC-<n> / unchecked-`[ ]` inside it was not
// excluded, producing a false MF-2/MF-4 refusal). Per CommonMark, an
// opening run of N backticks is closed by the next run of EXACTLY N
// backticks; an opening run with no matching closer is literal. The whole
// span (both delimiter runs + payload) is replaced with spaces, newlines
// preserved so line-based scans keep their boundaries.
func stripInlineCodeSpans(s string) string {
	b := []byte(s)
	out := make([]byte, len(b))
	copy(out, b)
	i := 0
	for i < len(b) {
		if b[i] != '`' {
			i++
			continue
		}
		// Measure the opening backtick run.
		openStart := i
		j := i
		for j < len(b) && b[j] == '`' {
			j++
		}
		runLen := j - openStart
		// Search for a closing run of EXACTLY runLen backticks.
		matchEnd := -1
		k := j
		for k < len(b) {
			if b[k] != '`' {
				k++
				continue
			}
			runStart := k
			for k < len(b) && b[k] == '`' {
				k++
			}
			if k-runStart == runLen {
				matchEnd = k
				break
			}
			// A different-length run cannot close this span; keep scanning
			// from AFTER it (k already advanced past it).
		}
		if matchEnd == -1 {
			// No closing run: the opening backticks are literal. Skip past
			// them so a later, longer/shorter run can still open a span.
			i = j
			continue
		}
		for p := openStart; p < matchEnd; p++ {
			if out[p] != '\n' {
				out[p] = ' '
			}
		}
		i = matchEnd
	}
	return string(out)
}

func blankPreservingNewlines(match string) string {
	var b strings.Builder
	for _, r := range match {
		if r == '\n' {
			b.WriteRune('\n')
		} else {
			b.WriteRune(' ')
		}
	}
	return b.String()
}

var tbdTokenRe = regexp.MustCompile(`(?i)\bTBD\b`)
var openQuestionRe = regexp.MustCompile(`(?i)\bOPEN QUESTION\b`)

// blockingOccurrence preserves the original line for rendering and the
// heading path for canonical MF-4 identity.
type blockingOccurrence struct {
	line    string
	section string
}

var blockingHeaderRe = regexp.MustCompile(`^\*\*(Blocking Questions|Open Questions|Blocked on)\*\*$`)
var uncheckedItemRe = regexp.MustCompile(`^-\s*\[\s?\]`)
var headingRe = regexp.MustCompile(`^(#{1,6}) (.+)$`)

// findBlockingOccurrences scans every token or designated-region checklist
// occurrence outside code spans. Repeated tokens on one line stay distinct.
func findBlockingOccurrences(text string, checklist bool) []blockingOccurrence {
	strippedLines := strings.Split(stripCodeSpans(text), "\n")
	origLines := strings.Split(text, "\n")
	type heading struct {
		level int
		text  string
	}
	var headings []heading
	var out []blockingOccurrence
	inBlock := false
	for i, line := range strippedLines {
		trimmed := strings.TrimSpace(line)
		if m := headingRe.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			for len(headings) > 0 && headings[len(headings)-1].level >= level {
				headings = headings[:len(headings)-1]
			}
			headings = append(headings, heading{level, strings.TrimSpace(origLines[i])})
		}
		var path []string
		for _, h := range headings {
			path = append(path, h.text)
		}
		section := strings.Join(path, " / ")
		if section == "" {
			section = "-"
		}
		count := 0
		if checklist {
			if blockingHeaderRe.MatchString(trimmed) {
				inBlock = true
				continue
			}
			if boldHeaderRe.MatchString(trimmed) || headingRe.MatchString(line) {
				inBlock = false
			}
			if inBlock && uncheckedItemRe.MatchString(trimmed) {
				count = 1
			}
		} else {
			count = len(tbdTokenRe.FindAllStringIndex(line, -1)) + len(openQuestionRe.FindAllStringIndex(line, -1))
		}
		for j := 0; j < count; j++ {
			out = append(out, blockingOccurrence{strings.TrimSpace(origLines[i]), section})
		}
	}
	return out
}

func findBlockingTokens(text string) []blockingOccurrence {
	return findBlockingOccurrences(text, false)
}

func findBlockingChecklistItems(text string) []blockingOccurrence {
	return findBlockingOccurrences(text, true)
}

func evaluateMF4(description, sectionRaw string) Signal {
	s := Signal{ID: SignalBlocking, Pass: true}
	counts := map[string]int{}
	var details []string
	for _, source := range []struct{ name, text string }{{"description", description}, {"plan", sectionRaw}} {
		for _, kind := range []struct {
			name string
			scan func(string) []blockingOccurrence
		}{{"token", findBlockingTokens}, {"checklist", findBlockingChecklistItems}} {
			for _, occurrence := range kind.scan(source.text) {
				section := occurrence.section
				if source.name == "description" {
					section = "-"
				}
				s.Failures = append(s.Failures, occurrenceRecord(kind.name, source.name, section, occurrence.line, counts))
				if kind.name == "token" {
					details = append(details, fmt.Sprintf("a genuine blocking marker appears in the bead's plan section / bd description: %s", occurrence.line))
					if s.Recovery == "" {
						s.Recovery = "resolve the TBD/OPEN QUESTION marker (or remove it) in the bead's plan section / bd description"
					}
				} else {
					details = append(details, fmt.Sprintf("an unchecked item under a blocking-region header: %s", occurrence.line))
					if s.Recovery == "" {
						s.Recovery = "resolve (check off, or remove) the unchecked item under the blocking-region header, then re-run"
					}
				}
			}
		}
	}
	s.Failures = sortedUnique(s.Failures)
	s.Pass = len(s.Failures) == 0
	s.Detail = strings.Join(details, "; ")
	return s
}
